// rev 1 — as estatísticas do contrato
//
// UMA ROTA SÓ, E ELA DEVOLVE O RETRATO INTEIRO
//
//	A tela de Estatísticas nasceu fora do sistema, rodando offline sobre um
//	arquivo de 780 KB com nove tabelas dentro. A conta dela — mediana, tempo
//	entre marcos, índice contra o plano de preventiva — foi escrita e conferida
//	contra esse arquivo, e mora no JavaScript da tela, num lugar só (CORE-06).
//
//	Este módulo é o que aposenta o arquivo. Ele devolve as MESMAS nove tabelas,
//	com as mesmas colunas e na mesma ordem, lendo o banco vivo. A conta não
//	mudou uma linha na fusão, e é exatamente por isso que ela continua valendo:
//	o que foi conferido é o que está rodando.
//
// POR QUE NÃO UMA ROTA POR TELA
//
//	Porque as nove telas cruzam as mesmas tabelas o tempo todo — "Tempo de
//	atendimento" precisa dos chamados E da linha do tempo, "Balanço" precisa dos
//	orçamentos E das faturas. Nove rotas devolveriam as mesmas linhas nove
//	vezes, e cada uma teria a sua chance de discordar das outras sobre o que é
//	um chamado atendido.
//
//	Uma resposta, um retrato, uma verdade. O navegador filtra por período sem
//	voltar ao servidor, que é o que faz as pílulas de período responderem na
//	hora.
//
// O QUE ESTE MÓDULO NÃO FAZ
//
//	Não calcula estatística nenhuma, e não grava nada. É leitura pura de nove
//	views. Se aparecer aqui um `if` decidindo o que é atrasado, ele está no
//	lugar errado — a régua é da tela.
package estatisticas

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

// RotinaModulo é a rotina do catálogo, a mesma da migração 042. Escrita como
// constante para o compilador acusar erro de digitação — string solta em rota é
// como se libera acesso sem querer.
const RotinaModulo = "CONTRATO_ESTATISTICAS"

// Teto é o máximo de linhas que sai de UMA tabela do retrato.
//
// A maior é a linha do tempo: 6.290 eventos de mudança de status em 30/08/2026,
// crescendo perto de 25 por dia. 100.000 é folga de anos.
//
// E o teto não corta em silêncio: `carregar` compara o que veio com o total que
// o banco informa e RECUSA a resposta inteira se faltar linha. Estatística
// truncada não parece quebrada — parece uma queda no movimento.
const Teto = 100000

type Modulo struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
}

func Novo(bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico) *Modulo {
	return &Modulo{bd: bd, seg: seg, perm: perm}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /estatisticas", m.retrato)
}

// ---------------------------------------------------------------------------
// as nove tabelas
// ---------------------------------------------------------------------------

// tabela descreve uma das nove peças do retrato.
//
// `colunas` faz DUAS coisas de propósito: é o `select=` que vai ao banco e é a
// lista de nomes que sai no JSON. Uma lista só significa que o rótulo e o dado
// não têm como discordar — separadas, bastaria alguém acrescentar uma coluna no
// meio de uma delas para a tela passar a ler prioridade na casa do status, sem
// erro nenhum aparecer.
type tabela struct {
	nome    string
	view    string
	colunas []string
	ordem   string
}

// A ordem de saída é fixa e explícita em `ordem`. Sem ela o banco devolve na
// ordem que quiser, e dois retratos seguidos do mesmo dado sairiam diferentes —
// o que torna impossível conferir um contra o outro.
var doRetrato = []tabela{
	{"unidades", "estatisticas_unidades",
		[]string{"id", "nome", "cidade", "no_escopo"}, "id"},

	{"chamados", "estatisticas_chamados",
		[]string{"numero", "unidade", "conta", "status", "prioridade",
			"ambiente", "responsavel", "criado_em", "prazo"}, "numero"},

	{"eventos", "estatisticas_eventos",
		[]string{"chamado", "sc", "quando"}, "chamado,quando"},

	{"custos", "estatisticas_custos",
		[]string{"chamado", "tipo", "valor", "criado_em"}, "chamado"},

	{"orcamentos", "estatisticas_orcamentos",
		[]string{"ticket", "unidade", "conta", "valor_nota", "valor",
			"ajustado_pelo_teto", "rateio", "status", "lancado_em", "faturado",
			"pago", "pago_em", "criado_em", "lancamento_bloqueio",
			"faturamento_direto", "fatura_id"}, "ticket"},

	{"ciclos", "estatisticas_ciclos",
		[]string{"id", "competencia", "ate", "enviado_em", "fechado_em"}, "competencia"},

	{"faturas", "estatisticas_faturas",
		[]string{"id", "ciclo_id", "unidade", "conta", "pco_numero", "nf_numero",
			"nf_em", "recebido_em", "valor_recebido", "valor_faturado"}, "id"},

	{"documentos", "estatisticas_documentos",
		[]string{"fila", "tipo", "emissao", "valor_total", "status", "inserido_em",
			"oculto_em", "bloqueio_motivo", "duplicada_de", "tickets",
			"tickets_soltos", "orcamentos"}, "inserido_em"},

	{"parametros", "estatisticas_parametros",
		[]string{"chave", "valor", "vigencia_inicio", "vigencia_fim"}, "chave"},
}

// peca é uma tabela do retrato já no formato que a tela lê: os nomes uma vez, e
// os valores em array. Escrito assim, e não como lista de objetos, porque o
// nome de cada coluna repetido em cada linha era metade do peso do arquivo.
type peca struct {
	Colunas []string            `json:"colunas"`
	Linhas  [][]json.RawMessage `json:"linhas"`
}

// ---------------------------------------------------------------------------
// GET /estatisticas — o retrato
// ---------------------------------------------------------------------------

func (m *Modulo) retrato(w http.ResponseWriter, r *http.Request) {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return
	}
	if err := m.perm.Exige(r.Context(), p, RotinaModulo); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return
	}

	fora := map[string]any{"gerado_em": time.Now().UTC().Format(time.RFC3339)}
	for _, t := range doRetrato {
		pc, err := m.carregar(r.Context(), t, p.ClienteID)
		if err != nil {
			log.Printf("estatísticas: %v", err)
			web.Falhar(w, http.StatusInternalServerError,
				"Não consegui montar as estatísticas de "+t.nome+".")
			return
		}
		fora[t.nome] = pc
	}
	web.Responder(w, http.StatusOK, fora)
}

// carregar traz uma tabela e a põe no formato colunas+linhas.
func (m *Modulo) carregar(ctx context.Context, t tabela, clienteID string) (peca, error) {
	caminho := t.view +
		"?cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=" + selecao(t.colunas) +
		"&order=" + t.ordem +
		"&limit=" + strconv.Itoa(Teto)

	// json.RawMessage, e não `any`, de propósito: o valor atravessa o motor
	// byte a byte, do banco para a tela. Decodificado para `any`, todo número
	// viraria float64 no caminho — e um ticket de 8 dígitos que volta em
	// notação científica é um número que a tela não reconhece mais.
	var linhas []map[string]json.RawMessage
	total, err := m.bd.BuscarContando(ctx, caminho, &linhas)
	if err != nil {
		return peca{}, fmt.Errorf("lendo %s: %w", t.view, err)
	}
	// A CONFERÊNCIA É `!=`, E NÃO `>`
	//
	//	Sem offset, o banco devolve min(total, Teto) linhas — então `total` e o
	//	que veio são iguais sempre que couber. Diferente quer dizer uma de duas
	//	coisas, e as duas são graves: ou o teto cortou, ou o cabeçalho de
	//	contagem não veio e eu não tenho como afirmar que veio tudo.
	//
	//	Nos dois casos a resposta é recusar. Corte silencioso não parece
	//	defeito: parece uma queda no movimento, e alguém decide em cima disso.
	if total != len(linhas) {
		return peca{}, fmt.Errorf("%s: o banco diz %d linhas e chegaram %d (teto %d)",
			t.view, total, len(linhas), Teto)
	}

	fora := peca{Colunas: t.colunas, Linhas: make([][]json.RawMessage, 0, len(linhas))}
	for _, l := range linhas {
		linha := make([]json.RawMessage, len(t.colunas))
		for i, c := range t.colunas {
			v, tem := l[c]
			if !tem || len(v) == 0 {
				v = json.RawMessage("null")
			}
			linha[i] = v
		}
		fora.Linhas = append(fora.Linhas, linha)
	}
	return fora, nil
}

// selecao monta o `select=` a partir da MESMA lista que dá nome às colunas.
func selecao(colunas []string) string {
	fora := ""
	for i, c := range colunas {
		if i > 0 {
			fora += ","
		}
		fora += c
	}
	return fora
}
