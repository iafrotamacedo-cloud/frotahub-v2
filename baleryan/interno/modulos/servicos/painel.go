// rev 1 — o hub de 6 cards e a Planilha de controle
//
// DUAS PERGUNTAS, DUAS VIEWS (migração 054)
//
//	Painel responde "quantos em cada fila?" — os números e a prévia dos 6
//	cards. Lista responde "quais, exatamente?" — a fonte única das 9 telas de
//	lista E da exportação da Planilha de controle, para as duas nunca
//	discordarem sobre o que é "com PCO" ou "atrasado".
package servicos

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
)

// Painel é uma linha de servicos_painel — os números dos 6 cards do hub.
type Painel struct {
	CandidatosPendentes      int `json:"candidatos_pendentes"`
	OrcamentosPendentes      int `json:"orcamentos_pendentes"`
	OrcamentoFeito           int `json:"orcamento_feito"`
	OrcamentoLancado         int `json:"orcamento_lancado"`
	ExecucaoAberto           int `json:"execucao_aberto"`
	ExecucaoEmCurso          int `json:"execucao_em_curso"`
	ExecucaoExecutado        int `json:"execucao_executado"`
	FaturamentoAguardandoPCO int `json:"faturamento_aguardando_pco"`
	FaturamentoAFaturar      int `json:"faturamento_a_faturar"`
	FaturamentoFaturado      int `json:"faturamento_faturado"`
	TotalAtivos              int `json:"total_ativos"`
}

// LerPainel busca a linha de servicos_painel deste cliente. A view faz LEFT
// JOIN a partir de `clientes`, então toda conta tem uma linha — mesmo sem
// nenhum serviço ainda, ela vem zerada, não ausente.
func (s *Servico) LerPainel(ctx context.Context, clienteID string) (Painel, error) {
	var linhas []Painel
	caminho := "servicos_painel?cliente_id=eq." + banco.Escapar(clienteID) + "&limit=1"
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return Painel{}, err
	}
	if len(linhas) == 0 {
		return Painel{}, nil
	}
	return linhas[0], nil
}

// ItemLista é uma linha de servicos_lista — as 9 telas de lista e a
// exportação da Planilha de controle leem daqui, e só daqui.
type ItemLista struct {
	ID        string `json:"id"`
	ChamadoID string `json:"chamado_id"`
	Ticket    int    `json:"ticket"`
	Conta     string `json:"conta"`
	Status    string `json:"status"`

	ChamadoDescricao    *string `json:"chamado_descricao"`
	ChamadoStatusCodigo *int    `json:"chamado_status_codigo"`
	ChamadoStatus       *string `json:"chamado_status"`
	ExecutadoEm         *string `json:"executado_em"`
	VistoriadoEm        *string `json:"vistoriado_em"`
	Loja                *string `json:"loja"`

	CotacaoTrilogoID       *int     `json:"cotacao_trilogo_id"`
	OrcamentoTrilogoID     *int     `json:"orcamento_trilogo_id"`
	OrcamentoValor         *float64 `json:"orcamento_valor"`
	OrcamentoArquivoSHA256 *string  `json:"orcamento_arquivo_sha256"`
	OrcamentoArquivoNome   *string  `json:"orcamento_arquivo_nome"`
	OrcamentoArquivoEm     *string  `json:"orcamento_arquivo_em"`
	OrcamentoAprovadoEm    *string  `json:"orcamento_aprovado_em"`
	OrcamentoRejeitadoEm   *string  `json:"orcamento_rejeitado_em"`

	PCONumero       *string `json:"pco_numero"`
	PCOPreenchidoEm *string `json:"pco_preenchido_em"`
	NFNumero        *string `json:"nf_numero"`
	NFArquivoSHA256 *string `json:"nf_arquivo_sha256"`
	NFArquivoNome   *string `json:"nf_arquivo_nome"`
	NFArquivoEm     *string `json:"nf_arquivo_em"`

	Origem         string  `json:"origem"`
	EntrouEm       string  `json:"entrou_em"`
	AtualizadoEm   string  `json:"atualizado_em"`
	RemovidoEm     *string `json:"removido_em"`
	RemovidoMotivo *string `json:"removido_motivo"`

	ComOrcamento  bool `json:"com_orcamento"`
	ComPCO        bool `json:"com_pco"`
	ComNF         bool `json:"com_nf"`
	EstaAprovado  bool `json:"esta_aprovado"`
	EstaRejeitado bool `json:"esta_rejeitado"`
}

// FiltroLista é o que as 9 telas (e a exportação) mandam — todo campo é
// opcional, vazio quer dizer "não filtra por isso".
type FiltroLista struct {
	Status    string
	Conta     string
	Busca     string
	// PCO separa os dois sub-cards de Faturamento, que vivem no MESMO status
	// (aguardando_faturamento) — "sim" só quem já tem pco_numero, "nao" só
	// quem não tem, vazio não filtra (ver migração 053).
	PCO       string
	SoAtivos  bool
	Pagina    int
	PorPagina int
}

// caminhoDaLista monta o filtro comum entre a leitura paginada (Lista) e a
// exportação (sem paginação) — um lugar só, pra tela e PDF/Excel nunca
// discordarem do que foi pedido.
func caminhoDaLista(clienteID string, f FiltroLista) string {
	caminho := "servicos_lista?cliente_id=eq." + banco.Escapar(clienteID)
	if f.SoAtivos {
		caminho += "&removido_em=is.null"
	}
	if f.Status != "" {
		caminho += "&status=eq." + banco.Escapar(f.Status)
	}
	if f.Conta != "" {
		caminho += "&conta=eq." + banco.Escapar(f.Conta)
	}
	switch f.PCO {
	case "sim":
		caminho += "&pco_numero=not.is.null"
	case "nao":
		caminho += "&pco_numero=is.null"
	}
	if b := strings.TrimSpace(f.Busca); b != "" {
		partes := []string{"loja.ilike." + banco.Escapar("*"+b+"*")}
		if numero, err := strconv.Atoi(b); err == nil {
			partes = append(partes, "ticket.eq."+strconv.Itoa(numero))
		}
		caminho += "&or=(" + strings.Join(partes, ",") + ")"
	}
	return caminho
}

const porPaginaPadraoDaLista = 100
const porPaginaMaximaDaLista = 500

// Lista devolve uma página de servicos_lista, mais o total (sem o limite da
// página) — mesma dupla resposta de qualquer tela paginada do sistema.
func (s *Servico) Lista(ctx context.Context, clienteID string, f FiltroLista) (itens []ItemLista, total int, err error) {
	itens = []ItemLista{}
	pagina := f.Pagina
	if pagina < 1 {
		pagina = 1
	}
	porPagina := f.PorPagina
	if porPagina < 1 || porPagina > porPaginaMaximaDaLista {
		porPagina = porPaginaPadraoDaLista
	}

	caminho := caminhoDaLista(clienteID, f) +
		"&select=*&order=atualizado_em.desc" +
		fmt.Sprintf("&limit=%d&offset=%d", porPagina, (pagina-1)*porPagina)

	total, err = s.bd.BuscarContando(ctx, caminho, &itens)
	if err != nil {
		return nil, 0, err
	}
	return itens, total, nil
}
