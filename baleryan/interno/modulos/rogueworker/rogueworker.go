// rev 1 — a Rogue Worker: assistente dentro do FrotaHub
//
// Três capacidades, nesta ordem de peso: responder, agir, gerar relatório.
// Uma quarta, navegar, entra junto. Pergunta de leitura alcança qualquer
// módulo que este login já veria no menu — a conversa livre consulta os
// mesmos GET. Ação que muda dado (gerar, lançar) continua só em Orçamentos
// e sempre pede confirmação.
//
// ELA NÃO É UM SEGUNDO SISTEMA DE PERMISSÃO
//
//	Autentica como quem está conversando (o mesmo Bearer). Cada comando checa
//	a MESMA rotina que o handler original já exige. String solta de rotina
//	aqui é o mesmo furo de sempre — por isso as constantes vêm do pacote
//	`orcamentos`, não copiadas.
//
// ELA NÃO FURA O FLUXO
//
//	Ação (gerar, lançar) é chamada HTTP interna contra o mesmo mux, o mesmo
//	handler que o clique do usuário já chama. Escrever em `documentos` ou
//	`orcamentos` daqui pra simular um estado pulharia teto, ajuste e
//	validação — e é exatamente o que este módulo existe para não fazer.
package rogueworker

import (
	"log"
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// moduloHistorico é o nome que a Rogue Worker grava em `historico` quando
// executa uma ação — não quando só responde. O handler original também grava
// a ação de negócio; esta linha é o rastro de que FOI ELA quem pediu.
const moduloHistorico = "rogueworker"

// fraseRecusa é a recusa total de um comando de ação. Sem variação e sem
// expor rotina/categoria: o dono pediu esta frase, literal, em 05/09/2026.
const fraseRecusa = "infelizmente você não tem permissão para esta tarefa"

// limiarDePromocao é quantas pessoas DIFERENTES precisam confirmar um
// candidato antes de ele virar comando fixo. Um número pequeno já evita o
// pior caso (um erro do Groq confirmado por engano uma vez) sem travar a
// operação. Configurável aqui, não em variável de ambiente: não é
// infraestrutura, é regra da conversa.
const limiarDePromocao = 3

// inicioDoContrato ancora relatórios do tipo "desde o início". Fixo, decisão
// do dono: não vem de tabela.
const inicioDoContrato = "2026-07-01"

type Modulo struct {
	cfg  *config.Config
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
	hist *historico.Servico
	mux  http.Handler
	groq *clienteGroq
}

func Novo(cfg *config.Config, bd *banco.Cliente, seg *seguranca.Servico,
	perm *permissao.Servico, hist *historico.Servico, mux http.Handler) *Modulo {
	return &Modulo{
		cfg: cfg, bd: bd, seg: seg, perm: perm, hist: hist, mux: mux,
		groq: novoClienteGroq(cfg.Groq),
	}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("POST /rogueworker/conversar", m.conversar)
	mux.HandleFunc("GET /rogueworker/excel", m.excel)
}

// quem identifica a pessoa. Sem rotina na porta: cada comando checa a sua.
// Devolve nil quando já respondeu o erro.
func (m *Modulo) quem(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

func (m *Modulo) erro(w http.ResponseWriter, frase string, err error) {
	log.Printf("rogueworker: %s: %v", frase, err)
	web.Falhar(w, http.StatusInternalServerError,
		"Não consegui completar: "+frase+". Tente de novo em instantes.")
}

func (m *Modulo) pode(r *http.Request, p *seguranca.Principal, rotina string) bool {
	ok, err := m.perm.Pode(r.Context(), p, rotina)
	if err != nil {
		log.Printf("rogueworker: falha ao checar %s: %v", rotina, err)
		return false
	}
	return ok
}
