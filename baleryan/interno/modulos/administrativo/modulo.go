// rev 1 — o módulo Administrativo: Compras
//
// PRIMEIRO PASSO SÓ: INSERIR E LISTAR
//
//	O pedido do dono (10/09/2026) foi específico: uma tela de inserção com um
//	botão para escolher o arquivo, um para inserir, e uma lista da fila embaixo.
//	Não entra aqui a leitura do PDF (extrair número, fornecedor, itens) — isso
//	depende de um leitor próprio para o formato do Obra Prima, que é passo
//	seguinte, não este (ver migração 059). Por isso este módulo não tem rota de
//	"ler": a OC nasce e fica em `status = 'inserido'` até esse leitor existir.
//
// PCO, Notas fiscais e Locações ainda não têm código nenhum — continuam em
// `<EmBreve>` no front até ganharem a vez (ver `claude/fase5-administrativo-
// planejamento.md` no Projeto).
package administrativo

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// RotinaOrdens é a única rotina deste primeiro passo — migração 059.
const RotinaOrdens = "COMPRAS_ORDENS_GERENCIAR"

// TamanhoMaximo de um arquivo de OC aceito na inserção. Mesmo teto de
// Orçamentos: PDF de OC digital não passa de poucos MB.
const TamanhoMaximo = 25 << 20

// TetoDaLista limita quantas ordens a fila devolve de uma vez. Não é
// paginação de verdade — não foi pedida — é o mesmo cuidado de
// `TetoDaLeitura` em Orçamentos: um número, e não "tudo o que existir".
const TetoDaLista = 500

type Modulo struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
	arm  *armazem.Cliente
	hist *historico.Servico
}

func Novo(bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico,
	arm *armazem.Cliente, hist *historico.Servico) *Modulo {
	return &Modulo{bd: bd, seg: seg, perm: perm, arm: arm, hist: hist}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /administrativo/compras/ordens", m.listarOrdens)
	mux.HandleFunc("POST /administrativo/compras/ordens", m.inserirOrdens)
	mux.HandleFunc("GET /administrativo/compras/ordens/{id}", m.verOrdem)
	mux.HandleFunc("GET /administrativo/compras/ordens/{id}/arquivo", m.arquivoDaOrdem)
}

// ---------------------------------------------------------------------------
// porteiro
// ---------------------------------------------------------------------------

// quem identifica e confere a rotina numa tacada — mesmo desenho de
// Orçamentos. Devolve nil quando já respondeu o erro.
func (m *Modulo) quem(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, RotinaOrdens); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// ---------------------------------------------------------------------------
// utilidades pequenas
// ---------------------------------------------------------------------------

var errNaoAchei = errors.New("não achei este registro")

// umUUID recusa qualquer coisa que não tenha cara de uuid ANTES de virar
// filtro — a diferença entre um 404 limpo e um filtro estranho chegando no
// banco (mesma função de Orçamentos).
func umUUID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return "", false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return "", false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", false
		}
	}
	return s, true
}

func ouVazio(l []map[string]any) []map[string]any {
	if l == nil {
		return []map[string]any{}
	}
	return l
}

// contarUm busca uma linha só e devolve errNaoAchei quando não vem nada.
func (m *Modulo) contarUm(ctx context.Context, caminho string) (map[string]any, error) {
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		return nil, errNaoAchei
	}
	return linhas[0], nil
}

// erro traduz uma falha em resposta, sem despejar detalhe de banco na tela.
func (m *Modulo) erro(w http.ResponseWriter, frase string, err error) {
	if err == errNaoAchei {
		web.Falhar(w, http.StatusNotFound, "Não achei este registro.")
		return
	}
	log.Printf("administrativo: %s: %v", frase, err)
	web.Falhar(w, http.StatusInternalServerError,
		"Não consegui completar: "+frase+". Tente de novo em instantes.")
}
