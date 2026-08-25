// rev 1 — o módulo Orçamentos
//
// Cinco telas, uma rotina de permissão para cada, e nenhuma regra de negócio
// dentro dos manipuladores: eles validam a entrada, chamam quem sabe fazer, e
// devolvem. A conta do dinheiro mora em `interno/regras`, os parâmetros em
// `interno/parametros`, a leitura da nota em `interno/leitor` e o lançamento em
// `interno/modulos/trilogo`.
//
// POR QUE UMA ROTINA POR BOTÃO
//
//	Porque "ver a planilha de controle" e "lançar custo no sistema do cliente"
//	são coisas de responsabilidade muito diferente, e na operação real quem faz
//	uma nem sempre é quem faz a outra. Uma rotina só para o módulo inteiro
//	obrigaria a liberar as duas juntas.
package orcamentos

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/parametros"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// As rotinas do catálogo, iguais às da migration 010. Escritas aqui como
// constante para o compilador acusar erro de digitação — string solta em rota é
// como se libera acesso sem querer.
const (
	RotinaModulo    = "CONTRATO_ORCAMENTOS"
	RotinaNotas     = "CONTRATO_ORCAMENTOS_NOTAS"
	RotinaRateio    = "CONTRATO_ORCAMENTOS_RATEIO"
	RotinaLancar    = "CONTRATO_ORCAMENTOS_LANCAR"
	RotinaCorrecoes = "CONTRATO_ORCAMENTOS_CORRECOES"
	RotinaPlanilhas = "CONTRATO_ORCAMENTOS_PLANILHAS"
)

// Quanto tempo vale o endereço temporário de um arquivo. Curto de propósito: o
// link é para abrir agora, não para guardar e passar adiante.
const ValidadeDoLink = 5 * time.Minute

// TamanhoMaximo de um arquivo aceito na inserção. Nota fiscal escaneada em 300
// dpi dá 1 a 2 MB; 25 MB é folga larga e ainda protege a memória do motor.
const TamanhoMaximo = 25 << 20

// PorPagina são as opções da lista, as mesmas da tela do Trílogo — a operação
// não deveria ter que aprender dois jeitos de paginar.
var PorPagina = []int{100, 250, 500}

const porPaginaPadrao = 100

// TetoDaExtracao limita o que sai em PDF e Excel. Corte silencioso é pior que
// corte: quando bate no teto, o documento diz que bateu.
const TetoDaExtracao = 5000

type Modulo struct {
	bd    *banco.Cliente
	seg   *seguranca.Servico
	perm  *permissao.Servico
	arm   *armazem.Cliente
	hist  *historico.Servico
	param *parametros.Servico
	cfg   *config.Config
}

func Novo(cfg *config.Config, bd *banco.Cliente, seg *seguranca.Servico,
	perm *permissao.Servico, arm *armazem.Cliente, hist *historico.Servico) *Modulo {
	return &Modulo{
		bd: bd, seg: seg, perm: perm, arm: arm, hist: hist,
		param: parametros.Novo(bd), cfg: cfg,
	}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	// o painel
	mux.HandleFunc("GET /orcamentos/painel", m.painel)

	// 2.1 e 2.2 — as duas filas de entrada
	mux.HandleFunc("GET /orcamentos/documentos", m.listarDocumentos)
	mux.HandleFunc("POST /orcamentos/documentos", m.inserirDocumentos)
	mux.HandleFunc("GET /orcamentos/documentos/{id}", m.verDocumento)
	mux.HandleFunc("GET /orcamentos/documentos/{id}/arquivo", m.arquivoDoDocumento)
	mux.HandleFunc("DELETE /orcamentos/documentos/{id}", m.ocultarDocumento)
	mux.HandleFunc("POST /orcamentos/documentos/{id}/restaurar", m.restaurarDocumento)

	// os tickets de uma nota (o "+" da tela de rateio, e a correção manual)
	mux.HandleFunc("POST /orcamentos/documentos/{id}/tickets", m.amarrarTickets)
	mux.HandleFunc("DELETE /orcamentos/documentos/{id}/tickets/{ticket}", m.soltarTicket)

	// 2.3 — gerar e lançar
	//
	// POR QUE UM ORÇAMENTO MORA EM /orcamentos/ficha/{id} E NÃO EM /orcamentos/{id}
	//
	//	Porque o segundo derruba o motor na subida. O roteador do Go recusa dois
	//	padrões em que nenhum é mais específico que o outro, e
	//	`GET /orcamentos/{id}/pdf` contra `GET /orcamentos/documentos/{id}` é
	//	exatamente isso: o endereço "/orcamentos/documentos/pdf" casa com os dois,
	//	e o Go não tem como escolher. Ele não avisa em compilação nem em `go vet`
	//	— entra em pânico no `Montar`, ou seja, no primeiro segundo do processo em
	//	produção.
	//
	//	A saída é a mesma que os documentos já usavam: um SUBSTANTIVO antes do id.
	//	Com "ficha" ali, todo segundo pedaço do caminho é palavra fixa —
	//	painel, documentos, gerar, ficha, correcoes, planilhas — e a ambiguidade
	//	deixa de ser possível, não só de acontecer.
	//
	//	"ficha" é a palavra que o sistema já usa para UM registro aberto (a ficha
	//	do chamado). Aqui é a ficha do orçamento.
	mux.HandleFunc("GET /orcamentos", m.listarOrcamentos)
	mux.HandleFunc("POST /orcamentos/gerar", m.gerar)
	mux.HandleFunc("POST /orcamentos/ficha/{id}/lancar", m.lancar)
	mux.HandleFunc("POST /orcamentos/ficha/{id}/aprovar", m.aprovar)
	mux.HandleFunc("DELETE /orcamentos/ficha/{id}", m.apagarOrcamento)
	mux.HandleFunc("POST /orcamentos/ficha/{id}/restaurar", m.restaurarOrcamento)
	mux.HandleFunc("GET /orcamentos/ficha/{id}/pdf", m.pdfDoOrcamento)

	// 2.4 — as quatro frentes
	mux.HandleFunc("GET /orcamentos/correcoes", m.correcoes)
	mux.HandleFunc("GET /orcamentos/correcoes/candidatos", m.candidatos)
	mux.HandleFunc("PATCH /orcamentos/documentos/{id}/tickets", m.corrigirTicket)

	// 2.5 — as planilhas de controle
	mux.HandleFunc("GET /orcamentos/planilhas", m.planilhas)
	mux.HandleFunc("GET /orcamentos/planilhas.xlsx", m.planilhaExcel)
	mux.HandleFunc("GET /orcamentos/planilhas.pdf", m.planilhaPDF)
}

// ---------------------------------------------------------------------------
// porteiro
// ---------------------------------------------------------------------------

// quem identifica e confere a rotina numa tacada. Devolve nil quando já
// respondeu o erro — quem chama só precisa testar o nil.
func (m *Modulo) quem(w http.ResponseWriter, r *http.Request, rotina string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, rotina); err != nil {
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

var ErrNaoAchei = errors.New("não achei este registro")

func umNumero(s string, padrao int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return padrao
	}
	return n
}

func umDosPermitidos(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return porPaginaPadrao
	}
	for _, p := range PorPagina {
		if p == n {
			return n
		}
	}
	return porPaginaPadrao
}

// intervalo transforma página e tamanho no par que o PostgREST entende.
func intervalo(pagina, porPagina int) string {
	return "&limit=" + strconv.Itoa(porPagina) + "&offset=" + strconv.Itoa((pagina-1)*porPagina)
}

// umUUID recusa qualquer coisa que não tenha cara de uuid ANTES de virar filtro.
// É a diferença entre um 404 limpo e um filtro estranho chegando no banco.
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

// Pagina é o envelope de toda lista deste módulo: as linhas e o total, para a
// tela escrever "1–100 de 1.377" sem uma segunda consulta.
type Pagina struct {
	Linhas    []map[string]any `json:"linhas"`
	Total     int              `json:"total"`
	Pagina    int              `json:"pagina"`
	PorPagina int              `json:"por_pagina"`
	Paginas   int              `json:"paginas"`
}

func montarPagina(linhas []map[string]any, total, pagina, por int) *Pagina {
	paginas := (total + por - 1) / por
	if paginas < 1 {
		paginas = 1
	}
	if linhas == nil {
		linhas = []map[string]any{}
	}
	return &Pagina{Linhas: linhas, Total: total, Pagina: pagina, PorPagina: por, Paginas: paginas}
}

// contarUm busca uma linha só e devolve ErrNaoAchei quando não vem nada — o
// caso mais comum de erro deste módulo, e o que mais merece uma frase clara.
func (m *Modulo) contarUm(ctx context.Context, caminho string) (map[string]any, error) {
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		return nil, ErrNaoAchei
	}
	return linhas[0], nil
}
