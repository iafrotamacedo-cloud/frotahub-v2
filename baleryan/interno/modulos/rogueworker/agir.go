package rogueworker

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/orcamentos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

func (m *Modulo) proporAcao(r *http.Request, p *seguranca.Principal, rec reconhecimento) Resposta {
	if !m.pode(r, p, orcamentos.RotinaLancar) {
		return Resposta{Texto: fraseRecusa}
	}
	switch rec.comando {
	case cmdGerarLote:
		ids, rotulo := m.notasDaFila(r)
		if len(ids) == 0 {
			return Resposta{Texto: "Não há nota pronta na fila para gerar orçamento agora."}
		}
		return Resposta{
			Texto:    fmt.Sprintf("Vou gerar orçamento para %s. Confirma?", rotulo),
			Pendente: &Pendente{Tipo: "acao", Comando: cmdGerarLote, IDs: ids, Rotulo: rotulo},
			Opcoes:   []string{"Sim", "Não"},
		}
	case cmdGerar:
		id := rec.id
		if id == "" && rec.ticket > 0 {
			id = m.documentoDoTicket(r, rec.ticket)
		}
		if id == "" {
			return Resposta{Texto: "Qual nota? Manda o número do ticket ou o identificador dela."}
		}
		rotulo := "esta nota"
		if rec.ticket > 0 {
			rotulo = fmt.Sprintf("a nota do ticket %d", rec.ticket)
		}
		return Resposta{
			Texto:    fmt.Sprintf("Vou gerar o orçamento de %s. Confirma?", rotulo),
			Pendente: &Pendente{Tipo: "acao", Comando: cmdGerar, IDs: []string{id}, Rotulo: rotulo},
			Opcoes:   []string{"Sim", "Não"},
		}
	case cmdLancarLote:
		ids, rotulo := m.orcamentosDaFilaDeLancar(r)
		if len(ids) == 0 {
			return Resposta{Texto: "Não há orçamento na fila de lançamento agora (só sobe o que o ticket deixa)."}
		}
		return Resposta{
			Texto:    fmt.Sprintf("Vou lançar no Trílogo %s. Confirma uma vez para o lote inteiro?", rotulo),
			Pendente: &Pendente{Tipo: "acao", Comando: cmdLancarLote, IDs: ids, Rotulo: rotulo},
			Opcoes:   []string{"Sim", "Não"},
		}
	case cmdLancar:
		id := rec.id
		if id == "" && rec.ticket > 0 {
			id = m.orcamentoDoTicket(r, rec.ticket)
		}
		if id == "" {
			return Resposta{Texto: "Qual orçamento? Manda o número do ticket ou o identificador dele."}
		}
		rotulo := "este orçamento"
		if rec.ticket > 0 {
			rotulo = fmt.Sprintf("o orçamento do ticket %d", rec.ticket)
		}
		return Resposta{
			Texto:    fmt.Sprintf("Vou lançar %s no Trílogo. Confirma?", rotulo),
			Pendente: &Pendente{Tipo: "acao", Comando: cmdLancar, IDs: []string{id}, Rotulo: rotulo},
			Opcoes:   []string{"Sim", "Não"},
		}
	}
	return Resposta{Texto: "Não sei executar essa ação."}
}

func (m *Modulo) executarAcao(r *http.Request, p *seguranca.Principal, pend *Pendente) Resposta {
	if !m.pode(r, p, orcamentos.RotinaLancar) {
		return Resposta{Texto: fraseRecusa}
	}
	if len(pend.IDs) == 0 {
		return Resposta{Texto: "Não restou item para executar."}
	}
	switch pend.Comando {
	case cmdGerar, cmdGerarLote:
		return m.gerarVarios(r, p, pend.IDs)
	case cmdLancar, cmdLancarLote:
		return m.lancarVarios(r, p, pend.IDs)
	default:
		return Resposta{Texto: "Não sei executar essa ação."}
	}
}

func (m *Modulo) gerarVarios(r *http.Request, p *seguranca.Principal, ids []string) Resposta {
	ok, falha := 0, 0
	var motivos []string
	for _, id := range ids {
		if _, ok := umUUID(id); !ok {
			falha++
			continue
		}
		var s struct {
			Resultados []map[string]any `json:"resultados"`
		}
		st, bruto, err := m.jsonInterno(r, http.MethodPost, "/orcamentos/gerar",
			map[string]any{"documentos": []string{id}}, &s)
		if err != nil || st != http.StatusOK {
			falha++
			if err == nil {
				motivos = appendUniq(motivos, m.erroInterno(st, bruto))
			}
			continue
		}
		if len(s.Resultados) == 0 {
			falha++
			motivos = appendUniq(motivos, "a nota não estava pronta para gerar")
			continue
		}
		umOk := false
		for _, res := range s.Resultados {
			if textoDe(res["erro"]) != "" {
				falha++
				motivos = appendUniq(motivos, textoDe(res["erro"]))
			} else {
				ok++
				umOk = true
			}
		}
		if umOk {
			_ = m.hist.Registrar(r.Context(), p, moduloHistorico, id, "gerar",
				map[string]historico.Mudanca{"via": {De: nil, Para: "rogueworker"}})
		}
	}
	return Resposta{Texto: resumoAcao("gerar orçamento", ok, falha, motivos)}
}

func (m *Modulo) lancarVarios(r *http.Request, p *seguranca.Principal, ids []string) Resposta {
	ok, falha := 0, 0
	var motivos []string
	for _, id := range ids {
		if _, good := umUUID(id); !good {
			falha++
			continue
		}
		st, bruto, err := m.chamarInterno(r, http.MethodPost, "/orcamentos/ficha/"+id+"/lancar", nil)
		if err != nil || st != http.StatusOK {
			falha++
			if err == nil {
				motivos = appendUniq(motivos, m.erroInterno(st, bruto))
			}
			continue
		}
		ok++
		_ = m.hist.Registrar(r.Context(), p, moduloHistorico, id, "lancar",
			map[string]historico.Mudanca{"via": {De: nil, Para: "rogueworker"}})
	}
	return Resposta{Texto: resumoAcao("lançar no Trílogo", ok, falha, motivos)}
}

func (m *Modulo) notasDaFila(r *http.Request) ([]string, string) {
	linhas, _, err := m.paginar(r, "/orcamentos/documentos?fila=orcamento")
	if err != nil {
		return nil, ""
	}
	var ids []string
	for _, n := range linhas {
		if pronto, _ := n["pronto_para_gerar"].(bool); !pronto {
			continue
		}
		id := textoDe(n["id"])
		if _, ok := umUUID(id); ok {
			ids = append(ids, id)
		}
	}
	return ids, fmt.Sprintf("estas %d nota(s) prontas da fila", len(ids))
}

func (m *Modulo) orcamentosDaFilaDeLancar(r *http.Request) ([]string, string) {
	linhas, _, err := m.paginar(r, "/orcamentos?status=gerado&destino=pode_lancar&decisao=nao")
	if err != nil {
		return nil, ""
	}
	var ids []string
	for _, o := range linhas {
		id := textoDe(o["id"])
		if _, ok := umUUID(id); ok {
			ids = append(ids, id)
		}
	}
	return ids, fmt.Sprintf("estes %d orçamento(s) da fila de lançamento", len(ids))
}

func (m *Modulo) orcamentoDoTicket(r *http.Request, ticket int) string {
	linhas, _, err := m.paginar(r, fmt.Sprintf("/orcamentos?ticket=%d&status=gerado", ticket))
	if err != nil || len(linhas) == 0 {
		return ""
	}
	return textoDe(linhas[0]["id"])
}

func resumoAcao(verbo string, ok, falha int, motivos []string) string {
	if ok == 0 && falha == 0 {
		return "Não havia o que " + verbo + "."
	}
	s := fmt.Sprintf("Pronto: %d deu certo ao %s", ok, verbo)
	if falha > 0 {
		s += fmt.Sprintf(", %d não passou", falha)
		if len(motivos) > 0 {
			s += " (" + strings.Join(motivos, "; ") + ")"
		}
	}
	return s + "."
}

func appendUniq(lista []string, s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return lista
	}
	for _, x := range lista {
		if x == s {
			return lista
		}
	}
	if len(lista) >= 4 {
		return lista
	}
	return append(lista, s)
}
