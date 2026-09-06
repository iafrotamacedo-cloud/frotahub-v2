package rogueworker

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/orcamentos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

func (m *Modulo) cmdPendencias(r *http.Request, p *seguranca.Principal) Resposta {
	if !m.pode(r, p, orcamentos.RotinaCorrecoes) {
		return Resposta{Texto: "Não achei pendência à vista com o que este login alcança."}
	}
	type saida struct {
		Tickets    []map[string]any `json:"tickets"`
		Orcamentos int              `json:"orcamentos"`
		Valor      float64          `json:"valor"`
		Titulo     string           `json:"titulo"`
	}
	var cliente, equipe saida
	st1, b1, err := m.jsonInterno(r, http.MethodGet, "/orcamentos/pendencias?destino=cliente", nil, &cliente)
	if err != nil {
		return Resposta{Texto: "Não consegui ler as pendências agora."}
	}
	if st1 != http.StatusOK {
		return Resposta{Texto: m.erroInterno(st1, b1)}
	}
	st2, b2, err := m.jsonInterno(r, http.MethodGet, "/orcamentos/pendencias?destino=encarregados", nil, &equipe)
	if err != nil {
		return Resposta{Texto: "Não consegui ler as pendências agora."}
	}
	if st2 != http.StatusOK {
		return Resposta{Texto: m.erroInterno(st2, b2)}
	}
	total := len(cliente.Tickets) + len(equipe.Tickets)
	if total == 0 {
		return Resposta{Texto: "Nenhuma pendência agora — nem do cliente, nem da equipe."}
	}
	return Resposta{Texto: fmt.Sprintf(
		"Há %d ticket(s) pendente(s) agora: %d esperando o cliente (R$ %s) e %d com a nossa equipe (R$ %s).",
		total, len(cliente.Tickets), emReais(cliente.Valor), len(equipe.Tickets), emReais(equipe.Valor))}
}

func (m *Modulo) cmdStatusNota(r *http.Request, p *seguranca.Principal, rec reconhecimento) Resposta {
	if !m.pode(r, p, orcamentos.RotinaNotas) {
		return Resposta{Texto: "Não achei essa nota com o que este login alcança."}
	}
	id := rec.id
	if id == "" && rec.ticket > 0 {
		id = m.documentoDoTicket(r, rec.ticket)
	}
	if id == "" {
		return Resposta{Texto: "Qual nota ou ticket? Manda o número do ticket ou o identificador da nota."}
	}
	type saida struct {
		Documento map[string]any   `json:"documento"`
		Tickets   []map[string]any `json:"tickets"`
	}
	var s saida
	st, bruto, err := m.jsonInterno(r, http.MethodGet, "/orcamentos/documentos/"+id, nil, &s)
	if err != nil {
		return Resposta{Texto: "Não consegui abrir essa nota agora."}
	}
	if st == http.StatusNotFound {
		return Resposta{Texto: "Não achei essa nota."}
	}
	if st != http.StatusOK {
		return Resposta{Texto: m.erroInterno(st, bruto)}
	}
	doc := s.Documento
	numero := textoDe(doc["numero"])
	if numero == "" {
		numero = textoDe(doc["dav_numero"])
	}
	if numero == "" {
		numero = textoDe(doc["nome_arquivo"])
	}
	status := textoDe(doc["status"])
	fila := textoDe(doc["fila"])
	tickets := make([]string, 0, len(s.Tickets))
	for _, t := range s.Tickets {
		if n := inteiroDe(t["ticket"]); n > 0 {
			tickets = append(tickets, fmt.Sprintf("%d", n))
		}
	}
	linha := fmt.Sprintf("Nota %s · status %s · fila %s.", numero, status, fila)
	if textoDe(doc["duplicada_de"]) != "" {
		linha += " Está marcada como repetida."
	}
	if textoDe(doc["bloqueio_motivo"]) != "" {
		linha += " Bloqueio: " + textoDe(doc["bloqueio_motivo"]) + "."
	}
	if len(tickets) > 0 {
		linha += " Ticket(s): " + strings.Join(tickets, ", ") + "."
	}
	return Resposta{Texto: linha}
}

func (m *Modulo) documentoDoTicket(r *http.Request, ticket int) string {
	var linhas []map[string]any
	_ = m.bd.Buscar(r.Context(),
		"documento_tickets?ticket=eq."+fmt.Sprint(ticket)+
			"&select=documento_id&limit=1", &linhas)
	if len(linhas) == 0 {
		return ""
	}
	return textoDe(linhas[0]["documento_id"])
}

func (m *Modulo) cmdExtrapoladas(r *http.Request, p *seguranca.Principal) Resposta {
	if !m.pode(r, p, orcamentos.RotinaCorrecoes) {
		return Resposta{Texto: "Não achei orçamento extrapolado com o que este login alcança."}
	}
	var s struct {
		Notas []map[string]any `json:"notas"`
	}
	st, bruto, err := m.jsonInterno(r, http.MethodGet, "/orcamentos/correcoes/extrapoladas", nil, &s)
	if err != nil {
		return Resposta{Texto: "Não consegui listar as extrapoladas agora."}
	}
	if st != http.StatusOK {
		return Resposta{Texto: m.erroInterno(st, bruto)}
	}
	if len(s.Notas) == 0 {
		return Resposta{Texto: "Nenhuma nota extrapolada (acima do teto) neste momento."}
	}
	amostra := make([]string, 0, 5)
	for i, n := range s.Notas {
		if i >= 5 {
			break
		}
		nome := textoDe(n["numero"])
		if nome == "" {
			nome = textoDe(n["nome_arquivo"])
		}
		amostra = append(amostra, nome)
	}
	extra := ""
	if len(s.Notas) > 5 {
		extra = fmt.Sprintf(" As primeiras: %s.", strings.Join(amostra, ", "))
	} else if len(amostra) > 0 {
		extra = " " + strings.Join(amostra, ", ") + "."
	}
	return Resposta{Texto: fmt.Sprintf("%d nota(s) passaram do teto.%s", len(s.Notas), extra)}
}

func (m *Modulo) cmdBloqueadas(r *http.Request, p *seguranca.Principal) Resposta {
	if !m.pode(r, p, orcamentos.RotinaNotas) {
		return Resposta{Texto: "Não achei nota bloqueada ou repetida com o que este login alcança."}
	}
	linhas, _, err := m.paginar(r, "/orcamentos/documentos?fila=orcamento")
	if err != nil {
		return Resposta{Texto: "Não consegui listar as notas agora."}
	}
	if m.pode(r, p, orcamentos.RotinaRateio) {
		rateio, _, err := m.paginar(r, "/orcamentos/documentos?fila=rateio")
		if err == nil {
			linhas = append(linhas, rateio...)
		}
	}
	var bloqueadas, repetidas int
	for _, n := range linhas {
		if textoDe(n["duplicada_de"]) != "" {
			repetidas++
		}
		if textoDe(n["bloqueio_motivo"]) != "" {
			bloqueadas++
		}
	}
	if bloqueadas == 0 && repetidas == 0 {
		return Resposta{Texto: "Nenhuma nota bloqueada ou repetida na fila agora."}
	}
	return Resposta{Texto: fmt.Sprintf(
		"Na fila: %d bloqueada(s) e %d repetida(s).", bloqueadas, repetidas)}
}

func textoDe(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func inteiroDe(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case jsonNumber:
		n, _ := t.Int64()
		return int(n)
	case string:
		n, _ := fmt.Sscanf(t, "%d", new(int))
		_ = n
	}
	var n int
	_, _ = fmt.Sscanf(fmt.Sprint(v), "%d", &n)
	return n
}

type jsonNumber interface{ Int64() (int64, error) }

func numeroDe(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		var f float64
		fmt.Sscanf(strings.ReplaceAll(t, ",", "."), "%f", &f)
		return f
	}
	return 0
}
