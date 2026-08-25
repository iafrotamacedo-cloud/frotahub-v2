// rev 1 — as planilhas de controle
//
// SÃO DUAS VISÕES DA MESMA TABELA
//
//	5.1  todos os orçamentos gerados, com a coluna "lançado" (traço ou check)
//	5.2  só os lançados, com a data do lançamento e SEM a coluna "lançado" —
//	     que ali seria uma coluna com o mesmo valor em todas as linhas
//
// SOBRE `faturado` E `pago`
//
//	As duas colunas existem e hoje são sempre falso, porque quem as escreve é o
//	módulo financeiro, que ainda não foi construído. Isso está dito na tela e no
//	comentário da tabela — e é uma diferença importante em relação ao sistema
//	antigo, onde as mesmas colunas existiam, nunca eram escritas por ninguém, e
//	a planilha reportava "não" para tudo como se fosse informação.
package orcamentos

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

func (m *Modulo) planilhas(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaPlanilhas)
	if p == nil {
		return
	}
	q := r.URL.Query()
	pagina := umNumero(q.Get("pagina"), 1)
	por := umDosPermitidos(q.Get("por"))

	var linhas []map[string]any
	total, err := m.bd.BuscarContando(r.Context(),
		m.filtroDaPlanilha(p.ClienteID, r)+"&select=*"+intervalo(pagina, por), &linhas)
	if err != nil {
		m.erro(w, "não consegui montar a planilha", err)
		return
	}
	web.Responder(w, http.StatusOK, montarPagina(linhas, total, pagina, por))
}

// filtroDaPlanilha é o ÚNICO lugar onde o filtro vira consulta.
//
// A tela, o Excel e o PDF chamam esta função. Se fossem três montagens, bastaria
// alguém corrigir uma para a extração passar a discordar do que está na tela —
// que é o defeito mais difícil de perceber, porque os dois números parecem
// certos separados.
func (m *Modulo) filtroDaPlanilha(clienteID string, r *http.Request) string {
	q := r.URL.Query()
	f := "orcamentos_lista?cliente_id=eq." + banco.Escapar(clienteID)

	if lancadas(r) {
		f += "&status=eq.lancado&order=lancado_em.desc"
	} else {
		f += "&status=neq.removido&order=criado_em.desc"
	}
	if t := umNumero(q.Get("ticket"), 0); t > 0 {
		f += "&ticket=eq." + strconv.Itoa(t)
	}
	if c := q.Get("conta"); c == "instalacoes" || c == "civil" {
		f += "&conta=eq." + c
	}
	if de := umaData(q.Get("de"), false); de != "" {
		f += "&criado_em=gte." + de
	}
	if ate := umaData(q.Get("ate"), true); ate != "" {
		f += "&criado_em=lte." + ate
	}
	return f
}

func lancadas(r *http.Request) bool { return r.URL.Query().Get("tipo") == "lancados" }

// umaData aceita "2026-08-25" e devolve o instante no fuso da casa.
//
// Sem o fuso, "até 25/08" significaria "até 25/08 21:00 em Fortaleza" — e os
// lançamentos do fim do dia sumiriam da planilha sem explicação.
func umaData(s string, fimDoDia bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	d, err := time.ParseInLocation("2006-01-02", s, fusoDaCasa())
	if err != nil {
		return ""
	}
	if fimDoDia {
		d = d.Add(24*time.Hour - time.Second)
	}
	return d.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// extração
// ---------------------------------------------------------------------------

var colunasGerados = []relatorio.Coluna{
	{Titulo: "Ticket", Peso: 1, Tipo: relatorio.Texto},
	{Titulo: "Nota", Peso: 1.1, Tipo: relatorio.Texto},
	{Titulo: "DAV", Peso: 1, Tipo: relatorio.Texto},
	{Titulo: "Loja", Peso: 2, Tipo: relatorio.Texto},
	{Titulo: "Valor", Peso: 1.2, Tipo: relatorio.Dinheiro},
	{Titulo: "Gerado em", Peso: 1.4, Tipo: relatorio.DataHora},
	{Titulo: "Conta", Peso: 1, Tipo: relatorio.Texto},
	{Titulo: "Lançado", Peso: 0.8, Tipo: relatorio.Texto},
	{Titulo: "Faturado", Peso: 0.9, Tipo: relatorio.Texto},
	{Titulo: "Pago", Peso: 0.8, Tipo: relatorio.Texto},
}

var colunasLancados = []relatorio.Coluna{
	{Titulo: "Ticket", Peso: 1, Tipo: relatorio.Texto},
	{Titulo: "Nota", Peso: 1.1, Tipo: relatorio.Texto},
	{Titulo: "DAV", Peso: 1, Tipo: relatorio.Texto},
	{Titulo: "Loja", Peso: 2.2, Tipo: relatorio.Texto},
	{Titulo: "Valor", Peso: 1.2, Tipo: relatorio.Dinheiro},
	{Titulo: "Lançado em", Peso: 1.5, Tipo: relatorio.DataHora},
	{Titulo: "Conta", Peso: 1, Tipo: relatorio.Texto},
	{Titulo: "Faturado", Peso: 0.9, Tipo: relatorio.Texto},
	{Titulo: "Pago", Peso: 0.8, Tipo: relatorio.Texto},
}

type linhaDaPlanilha struct {
	Ticket    int      `json:"ticket"`
	Parte     int      `json:"parte"`
	Notas     *string  `json:"notas"`
	Davs      *string  `json:"davs"`
	Loja      *string  `json:"loja"`
	Valor     *float64 `json:"valor"`
	CriadoEm  *string  `json:"criado_em"`
	LancadoEm *string  `json:"lancado_em"`
	Conta     *string  `json:"conta"`
	Status    string   `json:"status"`
	Faturado  bool     `json:"faturado"`
	Pago      bool     `json:"pago"`
}

func (m *Modulo) montarRelatorio(w http.ResponseWriter, r *http.Request) (relatorio.Tabela, bool) {
	p := m.quem(w, r, RotinaPlanilhas)
	if p == nil {
		return relatorio.Tabela{}, false
	}
	so := lancadas(r)

	var linhas []linhaDaPlanilha
	total, err := m.bd.BuscarContando(r.Context(),
		m.filtroDaPlanilha(p.ClienteID, r)+"&select=*&limit="+strconv.Itoa(TetoDaExtracao), &linhas)
	if err != nil {
		m.erro(w, "não consegui montar a extração", err)
		return relatorio.Tabela{}, false
	}

	colunas := colunasGerados
	titulo := "Orçamentos gerados"
	if so {
		colunas = colunasLancados
		titulo = "Orçamentos lançados"
	}

	saida := make([][]any, 0, len(linhas))
	for _, l := range linhas {
		ticket := strconv.Itoa(l.Ticket)
		if l.Parte > 1 {
			ticket += "-" + strconv.Itoa(l.Parte)
		}
		base := []any{ticket, ouTraco(texto(l.Notas)), ouTraco(texto(l.Davs)), texto(l.Loja), valorOuNada(l.Valor)}
		if so {
			base = append(base, instante(l.LancadoEm), contaPorExtenso(texto(l.Conta)),
				marca(l.Faturado), marca(l.Pago))
		} else {
			base = append(base, instante(l.CriadoEm), contaPorExtenso(texto(l.Conta)),
				marca(l.Status == "lancado"), marca(l.Faturado), marca(l.Pago))
		}
		saida = append(saida, base)
	}

	tab := relatorio.Tabela{
		Titulo:    titulo,
		Subtitulo: descreverFiltro(r, total),
		Colunas:   colunas,
		Linhas:    saida,
		Gerado:    time.Now().In(fusoDaCasa()),
	}
	// CORTE SILENCIOSO É PIOR QUE CORTE
	//   Quando bate no teto, o documento DIZ que bateu. Sem esta linha, quem
	//   recebe a planilha acha que está vendo tudo.
	if total > len(linhas) {
		tab.Aviso = "Mostrando as " + comPonto(len(linhas)) + " primeiras de " +
			comPonto(total) + ". Aperte o filtro para levar o resto."
	}
	return tab, true
}

// marca é o traço e o check que o dono pediu: "traço para não, check para sim".
func marca(v bool) string {
	if v {
		// Um "x" em vez do símbolo de visto: as catorze fontes padrão do PDF não
		// têm o caractere ✓, e ele sairia como um quadrado vazio no arquivo.
		return "x"
	}
	return "–"
}

// ouTraco escreve o traço no lugar do vazio. Célula em branco na planilha lê
// como "esqueceram de preencher"; o traço lê como "não tem".
func ouTraco(s string) string {
	if strings.TrimSpace(s) == "" {
		return "–"
	}
	return s
}

func texto(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func valorOuNada(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func instante(p *string) any {
	if p == nil || *p == "" {
		return nil
	}
	for _, f := range []string{time.RFC3339, "2006-01-02T15:04:05.999999", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(f, *p); err == nil {
			return t.In(fusoDaCasa())
		}
	}
	return *p
}

func contaPorExtenso(c string) string {
	switch c {
	case "instalacoes":
		return "Instalações"
	case "civil":
		return "Civil"
	default:
		return c
	}
}

func descreverFiltro(r *http.Request, total int) string {
	q := r.URL.Query()
	partes := []string{comPonto(total) + " orçamentos"}
	if t := q.Get("ticket"); t != "" {
		partes = append(partes, "ticket "+t)
	}
	if c := q.Get("conta"); c != "" {
		partes = append(partes, "conta "+contaPorExtenso(c))
	}
	de, ate := q.Get("de"), q.Get("ate")
	switch {
	case de != "" && ate != "":
		partes = append(partes, "de "+diaBonito(de)+" a "+diaBonito(ate))
	case de != "":
		partes = append(partes, "a partir de "+diaBonito(de))
	case ate != "":
		partes = append(partes, "até "+diaBonito(ate))
	}
	return strings.Join(partes, " · ")
}

func diaBonito(s string) string {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("02/01/2006")
	}
	return s
}

func comPonto(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func (m *Modulo) planilhaExcel(w http.ResponseWriter, r *http.Request) {
	tab, ok := m.montarRelatorio(w, r)
	if !ok {
		return
	}
	corpo, err := tab.Planilha()
	if err != nil {
		m.erro(w, "não consegui montar a planilha", err)
		return
	}
	entregar(w, corpo, "orcamentos", "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func (m *Modulo) planilhaPDF(w http.ResponseWriter, r *http.Request) {
	tab, ok := m.montarRelatorio(w, r)
	if !ok {
		return
	}
	corpo, err := tab.PDF()
	if err != nil {
		m.erro(w, "não consegui montar o PDF", err)
		return
	}
	entregar(w, corpo, "orcamentos", "pdf", "application/pdf")
}
