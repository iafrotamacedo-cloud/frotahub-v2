// rev 1 — o relatório mensal de Serviço, no mesmo modelo do de materiais
//
// O MESMO CONCEITO DE orcamentos/relatorio_mensal.go, DO OUTRO LADO
//
//	Lá a fila é "lançado no Trílogo e nenhuma planilha anterior levou"
//	(`fatura_id is null`). Aqui não existe fatura por ciclo — o card de
//	Faturamento já separa "Aguardando PCO" de "A faturar" só pelo preenchimento
//	de `pco_numero` (migração 053). A fila deste relatório é exatamente o
//	sub-card "Aguardando PCO": vistoriado, sem PCO ainda. Ver a view
//	`servicos_a_cobrar` (migração 073) para a régua completa.
//
// SÓ EXCEL, MESMA RAZÃO DA TELA DE MATERIAIS
//
//	Decisão do dono, mesma lógica de 27/08/2026 aplicada aqui: o arquivo É o
//	produto — sai daqui, entra na planilha do cliente, ele preenche o PCO e
//	devolve. Um PDF seria uma segunda versão do mesmo documento.
package servicos

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TetoDoRelatorioMensalDeServico — mesma trava de TetoDaExtracaoDeServico
// (relatorio.go): corte silencioso é pior que corte.
const TetoDoRelatorioMensalDeServico = TetoDaExtracaoDeServico

// colunasDoRelatorioMensalDeServico — as mesmas OITO colunas, na mesma ordem,
// do modelo de materiais (orcamentos/relatorio_mensal.go): Nº, TICKET, LOJA,
// VALOR, DATA, ORÇAMENTO, CONTA, PCO. O dono pediu explicitamente o mesmo
// modelo de saída para Serviço — mudar a forma aqui quebraria a promessa de
// "é a mesma planilha" que o pedido fez.
var colunasDoRelatorioMensalDeServico = []relatorio.Coluna{
	{Titulo: "Nº", Peso: 5, Tipo: relatorio.Numero},
	{Titulo: "TICKET", Peso: 5, Tipo: relatorio.Numero},
	{Titulo: "LOJA", Peso: 24, Tipo: relatorio.Texto},
	{Titulo: "VALOR", Peso: 8, Tipo: relatorio.Dinheiro},
	{Titulo: "DATA", Peso: 9, Tipo: relatorio.Data},
	{Titulo: "ORÇAMENTO", Peso: 8, Tipo: relatorio.Dinheiro},
	{Titulo: "CONTA", Peso: 26, Tipo: relatorio.Texto},
	// PCO fica VAZIA: é exatamente o que falta para a linha sair desta fila
	// (ver o filtro de servicos_a_cobrar). Preenchida pelo cliente e
	// devolvida, ela entra em servicos_orcamentos.pco_numero pela tela de
	// Faturamento (PreencherPCO, kanban.go) — o mesmo lugar de sempre.
	{Titulo: "PCO", Peso: 14, Tipo: relatorio.Texto},
}

// aCobrarServico é uma linha de servicos_a_cobrar.
type aCobrarServico struct {
	Ticket    int     `json:"ticket"`
	Conta     string  `json:"conta"`
	Valor     float64 `json:"valor"`
	Data      string  `json:"data_relatorio"`
	Loja      *string `json:"loja_cliente"`
	LojaNossa *string `json:"loja"`
}

// servicosACobrar são os vistoriados que ainda não têm PCO do cliente.
func (m *Modulo) servicosACobrar(ctx context.Context, clienteID string) ([]aCobrarServico, error) {
	var linhas []aCobrarServico
	err := m.bd.Buscar(ctx, "servicos_a_cobrar?cliente_id=eq."+banco.Escapar(clienteID)+
		"&order=data_relatorio,ticket&limit="+fmt.Sprint(TetoDoRelatorioMensalDeServico+1)+"&select=*", &linhas)
	return linhas, err
}

// GET /servicos/relatorio-mensal — o que a tela mostra antes de extrair.
//
// A TELA MOSTRA A PLANILHA, NÃO UM RESUMO DELA — mesma razão do relatório de
// materiais: as linhas que a tela recebe são as MESMAS que vão no arquivo,
// montadas pelo mesmo caminho (linhasDoModeloDeServico).
func (m *Modulo) relatorioMensal(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	linhas, err := m.servicosACobrar(r.Context(), p.ClienteID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar o relatório.")
		return
	}

	var soma float64
	var semNome []string
	de, ate := "", ""
	for _, l := range linhas {
		soma += l.Valor
		if l.Loja == nil || *l.Loja == "" {
			semNome = append(semNome, ouVazioTexto(l.LojaNossa))
		}
		if de == "" || l.Data < de {
			de = l.Data
		}
		if l.Data > ate {
			ate = l.Data
		}
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"quantos":        len(linhas),
		"valor":          soma,
		"de":             de,
		"ate":            ate,
		"linhas":         linhasDoModeloDeServico(linhas),
		"lojas_sem_nome": semDuplicatasDeServico(semNome),
		"teto":           TetoDoRelatorioMensalDeServico,
	})
}

// GET /servicos/relatorio-mensal.xlsx
func (m *Modulo) relatorioMensalExcel(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	linhas, err := m.servicosACobrar(r.Context(), p.ClienteID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar o relatório.")
		return
	}

	aviso := ""
	if len(linhas) > TetoDoRelatorioMensalDeServico {
		linhas = linhas[:TetoDoRelatorioMensalDeServico]
		aviso = fmt.Sprintf("A relação foi cortada em %d linhas. Há mais serviços "+
			"a cobrar do que cabe num arquivo — feche este e gere de novo.", TetoDoRelatorioMensalDeServico)
	}

	tab := tabelaDoRelatorioDeServico(linhas, aviso, p.Nome, time.Now().In(relatorio.FusoDaCasa()))
	corpoXLSX, err := tab.Planilha()
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar a planilha.")
		return
	}
	entregarArquivoDeServico(w, corpoXLSX, "servicos-relatorio-mensal", "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

// tabelaDoRelatorioDeServico monta o documento inteiro — a lista e a capa.
// Separada da rota de propósito, mesma razão da irmã de materiais: o que vai
// ao cliente tem que poder ser conferido sem subir servidor.
func tabelaDoRelatorioDeServico(linhas []aCobrarServico, aviso, quem string, gerado time.Time) relatorio.Tabela {
	var soma float64
	de, ate := "", ""
	for _, l := range linhas {
		soma += l.Valor
		if de == "" || l.Data < de {
			de = l.Data
		}
		if l.Data > ate {
			ate = l.Data
		}
	}
	return relatorio.Tabela{
		Titulo:    tituloDoModeloDeServico,
		Aba:       "Serviços",
		Subtitulo: fmt.Sprintf("%d serviços · R$ %s · aguardando PCO", len(linhas), emReaisServico(soma)),
		Colunas:   colunasDoRelatorioMensalDeServico,
		Linhas:    linhasDoModeloDeServico(linhas),
		Aviso:     aviso,
		Gerado:    gerado,
		Capa: &relatorio.Capa{
			Chapeu:     chapeuDoModeloDeServico,
			Periodo:    periodoDoModeloDeServico(de, ate),
			Assinatura: assinaturaDeServico(quem, gerado),
			Resumo:     fmt.Sprintf("%d serviços", len(linhas)),
			Destaque:   "R$ " + emReaisServico(soma),
		},
	}
}

// O que a faixa escura diz — textos do DOCUMENTO, não do sistema.
const (
	tituloDoModeloDeServico = "CUSTOS DE SERVIÇOS DOS CHAMADOS"
	// "Fora do contrato de manutenção" é a fronteira que o próprio módulo
	// desenha (ver menu/modulos/servicos.ts) — instalação, obra, ampliação.
	// Chamar isso de "manutenção" no documento que vai ao cliente confundiria
	// a conta certa com a errada.
	chapeuDoModeloDeServico = "FROTA MACEDO ENGENHARIA  ·  SERVIÇOS — INSTALAÇÃO, OBRA E AMPLIAÇÃO"
)

func periodoDoModeloDeServico(de, ate string) string {
	d, a := soODiaServico(de), soODiaServico(ate)
	if d == "" || a == "" {
		return "Serviços vistoriados, aguardando PCO do cliente"
	}
	return "Serviços vistoriados, aguardando PCO do cliente  ·  " + d + " a " + a
}

func soODiaServico(iso string) string {
	d, ok := emDataServico(iso).(time.Time)
	if !ok {
		return ""
	}
	return d.Format("02/01/2006")
}

func assinaturaDeServico(nome string, quando time.Time) string {
	quem := ""
	if n := strings.TrimSpace(nome); n != "" {
		quem = " por " + n
	}
	return "Gerado em " + quando.Format("02/01/2006") + quem + " através do FrotaHub®"
}

func emReaisServico(v float64) string { return regras.DinheiroDe(v).Reais() }

// linhasDoModeloDeServico monta as oito colunas, na ordem do modelo — mesma
// função para a tela e para o arquivo (ver o comentário do topo).
func linhasDoModeloDeServico(linhas []aCobrarServico) [][]any {
	corpo := make([][]any, 0, len(linhas))
	for i, l := range linhas {
		corpo = append(corpo, []any{
			i + 1,
			l.Ticket,
			ouVazioTexto(l.Loja),
			l.Valor,
			emDataServico(l.Data),
			// ORÇAMENTO repete VALOR — mesma razão do modelo de materiais: as
			// duas colunas levam o valor cobrado, nunca a margem.
			l.Valor,
			contaPorExtensoServico(l.Conta),
			// PCO, vazia por enquanto — ver o comentário da coluna.
			"",
		})
	}
	return corpo
}

// emDataServico — mesma lógica de emData (orcamentos/relatorio_mensal.go):
// data de VERDADE (serial do Excel), no fuso da casa, não texto.
func emDataServico(iso string) any {
	if len(iso) < 10 {
		return nil
	}
	fuso := relatorio.FusoDaCasa()
	if len(iso) > 10 {
		if t, err := time.Parse(time.RFC3339, iso); err == nil {
			l := t.In(fuso)
			return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, fuso)
		}
	}
	t, err := time.ParseInLocation("2006-01-02", iso[:10], fuso)
	if err != nil {
		return nil
	}
	return t
}

func ouVazioTexto(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func semDuplicatasDeServico(v []string) []string {
	visto := map[string]bool{}
	saida := make([]string, 0, len(v))
	for _, s := range v {
		if s == "" || visto[s] {
			continue
		}
		visto[s] = true
		saida = append(saida, s)
	}
	return saida
}
