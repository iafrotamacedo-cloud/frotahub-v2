// rev 1 — a consolidação: nota × ticket, nos dois sentidos
//
// O QUE ESTA TELA RESPONDE
//
//	Duas perguntas que hoje ninguém consegue responder sem abrir três telas e
//	uma planilha:
//
//	  "Comprei esta nota. Quanto dela já voltou do cliente?"     → aba das NOTAS
//	  "Cobrei este ticket. De que nota saiu, e já foi paga?"     → aba dos TICKETS
//
// UMA ROTA, DUAS TABELAS, NENHUMA CONTA
//
//	As duas listas se leem juntas — a pessoa clica de um lado e confere do
//	outro — então voltar ao servidor para trocar de aba seria uma espera sem
//	motivo. E as duas precisam concordar sobre o mesmo dinheiro: pedidas na
//	mesma resposta, elas saem do mesmo instante do banco e não têm como
//	discordar por causa de um lançamento que entrou no meio.
//
//	Este módulo não calcula nada. Soma, filtro e junção moram nas duas views da
//	migração 043; totalizar e formatar mora na tela. Se aparecer aqui um `if`
//	decidindo o que é pendente, ele está no lugar errado.
package consolidacao

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// RotinaModulo é a rotina do catálogo, a mesma da migração 043. Constante para
// o compilador acusar erro de digitação — string solta em rota é como se libera
// acesso sem querer.
const RotinaModulo = "CONTRATO_FINANCEIRO_CONSOLIDACAO"

// Teto é o máximo de linhas que sai de UMA das duas listas.
//
// Em 01/09/2026: 252 notas e 882 orçamentos. 20.000 é folga de anos — e o teto
// não corta em silêncio: `carregar` compara o que veio com o total que o banco
// informa e RECUSA a resposta inteira se faltar linha. Lista truncada não
// parece quebrada; parece que a nota não existe.
const Teto = 20000

type Modulo struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
}

func Novo(bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico) *Modulo {
	return &Modulo{bd: bd, seg: seg, perm: perm}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /consolidacao", m.consolidacao)
}

// ---------------------------------------------------------------------------
// o que sai
// ---------------------------------------------------------------------------

// O `select=` é escrito uma vez e serve de contrato: o que não está aqui não
// chega à tela. Pedir `*` traria a coluna nova de amanhã sem ninguém decidir
// que ela deve sair — e numa tela de dinheiro isso é como um número aparece sem
// dono.
const colunasDaNota = "documento_id,nf,tipo,fornecedor,emissao,valor,fila,status," +
	"tickets,ticket_numeros,orcamentos,orcado,recebido,pendente,lancado,inserido_em"

const colunasDoTicket = "orcamento_id,ticket,parte,loja,conta,valor,valor_nota,nfs," +
	"nota_excluida,no_relatorio,faturado,pago,status,lancado_em,rateio,legado,criado_em"

// ---------------------------------------------------------------------------
// GET /consolidacao
// ---------------------------------------------------------------------------

func (m *Modulo) consolidacao(w http.ResponseWriter, r *http.Request) {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return
	}
	if err := m.perm.Exige(r.Context(), p, RotinaModulo); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return
	}
	// SEM CLIENTE NÃO HÁ CONSOLIDAÇÃO
	//   O filtro por `cliente_id` é o que separa um contrato do outro. Sem ele
	//   a consulta traria a base inteira, e a tela mostraria a nota de um
	//   cliente na consolidação de outro.
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return
	}

	notas, err := m.carregar(r.Context(), "consolidacao_notas", colunasDaNota,
		"inserido_em.desc", p.ClienteID)
	if err != nil {
		log.Printf("consolidação: %v", err)
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar a lista de notas.")
		return
	}
	tickets, err := m.carregar(r.Context(), "consolidacao_tickets", colunasDoTicket,
		"criado_em.desc", p.ClienteID)
	if err != nil {
		log.Printf("consolidação: %v", err)
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar a lista de tickets.")
		return
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"gerado_em": time.Now().UTC().Format(time.RFC3339),
		"notas":     notas,
		"tickets":   tickets,
	})
}

// carregar traz uma das duas listas inteira, ou não traz nenhuma.
//
// O VALOR ATRAVESSA O MOTOR BYTE A BYTE
//
//	`json.RawMessage`, e não `any`, de propósito. Decodificado para `any`, todo
//	número viraria float64 no caminho — e um ticket de seis dígitos que volta em
//	notação científica é um número que a tela não reconhece mais. Dinheiro tem o
//	problema irmão: 587.98 vira 587.98000000000002 e a soma da tela passa a
//	fechar com um centavo de diferença que ninguém explica.
func (m *Modulo) carregar(ctx context.Context, view, colunas, ordem, clienteID string) (
	[]map[string]json.RawMessage, error) {

	caminho := view +
		"?cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=" + colunas +
		"&order=" + ordem +
		"&limit=" + strconv.Itoa(Teto)

	var linhas []map[string]json.RawMessage
	total, err := m.bd.BuscarContando(ctx, caminho, &linhas)
	if err != nil {
		return nil, fmt.Errorf("lendo %s: %w", view, err)
	}
	// A CONFERÊNCIA É `!=`, E NÃO `>`
	//
	//	Sem offset o banco devolve min(total, Teto) linhas — então `total` e o
	//	que veio são iguais sempre que couber. Diferente quer dizer uma de duas
	//	coisas, e as duas são graves: ou o teto cortou, ou o cabeçalho de
	//	contagem não veio e eu não tenho como afirmar que veio tudo.
	//
	//	Nos dois casos a resposta é recusar. Numa tela de conferência, meia
	//	lista é pior que lista nenhuma: ela fecha uma conta errada com ar de
	//	certa.
	if total != len(linhas) {
		return nil, fmt.Errorf("%s: o banco diz %d linhas e chegaram %d (teto %d)",
			view, total, len(linhas), Teto)
	}
	// Lista vazia sai como `[]`, nunca como `null`: a tela faz `.map` em cima, e
	// `null` derrubaria a página inteira em vez de mostrar "nada por aqui".
	if linhas == nil {
		linhas = []map[string]json.RawMessage{}
	}
	return linhas, nil
}
