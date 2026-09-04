// rev 1 — a exportação da Planilha de controle
//
// MESMO PADRÃO DE trilogo/consulta.go (extrairPlanilha/extrairPDF)
//
//	relatorio.Tabela escrita à mão, sem dependência externa — ver o cabeçalho
//	de interno/relatorio/relatorio.go. A REGRA do que entra no relatório mora
//	num lugar só: servicos_lista (migração 054), a mesma fonte que as 9 telas
//	de lista já leem — tela e arquivo nunca podem discordar.
package servicos

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TetoDaExtracaoDeServico — mesma trava de trilogo.TetoDaExtracao, copiada e
// não importada: o volume de Serviço é uma fração do contrato (ver o
// comentário de ListarKanban), então este teto quase nunca aparece — mas a
// trava existe para não gerar um PDF gigante no dia em que aparecer.
const TetoDaExtracaoDeServico = 5000

var colunasDaPlanilhaDeServico = []relatorio.Coluna{
	{Titulo: "Ticket", Peso: 7, Tipo: relatorio.Numero},
	{Titulo: "Loja", Peso: 18, Tipo: relatorio.Texto},
	{Titulo: "Conta", Peso: 10, Tipo: relatorio.Texto},
	{Titulo: "Status", Peso: 16, Tipo: relatorio.Texto},
	{Titulo: "Descrição", Peso: 34, Tipo: relatorio.Texto},
	{Titulo: "Valor do orçamento", Peso: 12, Tipo: relatorio.Dinheiro},
	{Titulo: "PCO", Peso: 10, Tipo: relatorio.Texto},
	{Titulo: "Nota fiscal", Peso: 10, Tipo: relatorio.Texto},
	{Titulo: "Entrou em", Peso: 11, Tipo: relatorio.DataHora},
}

func contaPorExtensoServico(c string) string {
	if c == "civil" {
		return "Civil"
	}
	return "Instalações"
}

// rotuloDoStatus — os mesmos dez rótulos que o front usa (tipos.ts,
// STATUS_ROTULO) — duplicado de propósito, mesma razão de PROXIMOS_STATUS no
// front: o relatório não pode esperar o front carregar pra saber o nome de
// uma coluna.
var rotuloDoStatus = map[string]string{
	StatusAguardandoOrcamento:   "Aguardando orçamento",
	StatusOrcamentoFeito:        "Orçamento feito",
	StatusOrcamentoLancado:      "Orçamento lançado",
	StatusOrcamentoAprovado:     "Orçamento aprovado",
	StatusOrcamentoRejeitado:    "Orçamento rejeitado",
	StatusAprovadoExecucao:      "Aprovado p/ execução",
	StatusEmExecucao:            "Em execução",
	StatusFinalizado:            "Finalizado",
	StatusAguardandoFaturamento: "Aguardando faturamento",
	StatusFaturado:              "Faturado",
}

func rotuloStatusServico(status string) string {
	if r, ok := rotuloDoStatus[status]; ok {
		return r
	}
	return status
}

// montarRelatorioDeServico lê servicos_lista com o mesmo filtro da tela
// (ver caminhoDaLista) e monta a Tabela — chamado pelas duas rotas de
// extração, para as duas nunca discordarem do que a tela mostrou.
func (m *Modulo) montarRelatorioDeServico(w http.ResponseWriter, r *http.Request, clienteID string) (relatorio.Tabela, bool) {
	f := filtroDaQuery(r.URL.Query())
	f.Pagina = 0

	itens := []ItemLista{}
	caminho := caminhoDaLista(clienteID, f) +
		fmt.Sprintf("&select=*&order=atualizado_em.desc&limit=%d", TetoDaExtracaoDeServico)
	total, err := m.bd.BuscarContando(r.Context(), caminho, &itens)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os serviços para a extração.")
		return relatorio.Tabela{}, false
	}

	tab := relatorio.Tabela{
		Titulo:    "Serviço — planilha de controle",
		Subtitulo: descreverFiltroDeServico(f),
		Colunas:   colunasDaPlanilhaDeServico,
		Gerado:    time.Now(),
	}
	for _, it := range itens {
		tab.Linhas = append(tab.Linhas, []any{
			it.Ticket, textoOuTraco(it.Loja), contaPorExtensoServico(it.Conta), rotuloStatusServico(it.Status),
			textoOuTraco(it.ChamadoDescricao), valorOuNulo(it.OrcamentoValor),
			textoOuTraco(it.PCONumero), textoOuTraco(it.NFNumero),
			it.EntrouEm,
		})
	}
	if total > len(itens) {
		tab.Aviso = fmt.Sprintf("Mostrando os primeiros %d de %d — refine o filtro", len(itens), total)
	}
	return tab, true
}

func textoOuTraco(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}

func valorOuNulo(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func descreverFiltroDeServico(f FiltroLista) string {
	partes := []string{}
	if f.Status != "" {
		partes = append(partes, rotuloStatusServico(f.Status))
	}
	if f.Conta != "" {
		partes = append(partes, contaPorExtensoServico(f.Conta))
	}
	if f.Busca != "" {
		partes = append(partes, "busca: "+f.Busca)
	}
	if len(partes) == 0 {
		return "Todos os serviços"
	}
	s := partes[0]
	for _, p := range partes[1:] {
		s += " · " + p
	}
	return s
}

// GET /servicos/lista.xlsx
func (m *Modulo) extrairListaXLSX(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	tab, ok := m.montarRelatorioDeServico(w, r, p.ClienteID)
	if !ok {
		return
	}
	bytes, err := tab.Planilha()
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar a planilha.")
		return
	}
	entregarArquivoDeServico(w, bytes, "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

// GET /servicos/lista.pdf
func (m *Modulo) extrairListaPDF(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	tab, ok := m.montarRelatorioDeServico(w, r, p.ClienteID)
	if !ok {
		return
	}
	bytes, err := tab.PDF()
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar o PDF.")
		return
	}
	entregarArquivoDeServico(w, bytes, "pdf", "application/pdf")
}

func entregarArquivoDeServico(w http.ResponseWriter, corpo []byte, extensao, tipo string) {
	nome := fmt.Sprintf("servicos-%s.%s", time.Now().Format("2006-01-02"), extensao)
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+nome+"\"; filename*=UTF-8''"+url.PathEscape(nome))
	w.Header().Set("Content-Length", strconv.Itoa(len(corpo)))
	w.WriteHeader(http.StatusOK)
	w.Write(corpo)
}
