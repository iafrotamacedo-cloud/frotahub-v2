package rogueworker

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/orcamentos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// GET /rogueworker/excel — planilha dos relatórios desta leva que NÃO têm
// extração própria no módulo original. Pedido, faturamento e relatório mensal
// apontam para o .xlsx que orçamentos já entrega, para não nascer um segundo
// gerador com outra capa.
func (m *Modulo) excel(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	switch r.URL.Query().Get("o") {
	case "material":
		m.excelMaterial(w, r, p)
	default:
		web.Falhar(w, http.StatusBadRequest, "Diga qual planilha: o=material.")
	}
}

func (m *Modulo) excelMaterial(w http.ResponseWriter, r *http.Request, p *seguranca.Principal) {
	if !m.pode(r, p, orcamentos.RotinaLancar) {
		web.Falhar(w, http.StatusForbidden, fraseRecusa)
		return
	}
	linhas, _, err := m.paginar(r, "/orcamentos?status=lancado")
	if err != nil {
		m.erro(w, "não consegui listar os orçamentos lançados", err)
		return
	}
	inicio, fim := mesCorrente()
	corpo := make([][]any, 0)
	var soma float64
	for _, l := range linhas {
		quando := textoDe(l["lancado_em"])
		if quando == "" {
			quando = textoDe(l["criado_em"])
		}
		if !noIntervalo(quando, inicio, fim) {
			continue
		}
		valor := numeroDe(l["valor"])
		soma += valor
		corpo = append(corpo, []any{
			inteiroDe(l["ticket"]),
			textoDe(l["loja"]),
			valor,
			quando,
		})
	}
	agora := time.Now().In(relatorio.FusoDaCasa())
	mes := agora.Format("01/2006")
	tab := relatorio.Tabela{
		Titulo:    "Material lançado no contrato",
		Aba:       "Lançados",
		Subtitulo: fmt.Sprintf("%d orçamentos · R$ %s · %s", len(corpo), emReais(soma), mes),
		Colunas: []relatorio.Coluna{
			{Titulo: "Ticket", Peso: 1, Tipo: relatorio.Numero},
			{Titulo: "Loja", Peso: 2.4, Tipo: relatorio.Texto},
			{Titulo: "Valor", Peso: 1.3, Tipo: relatorio.Dinheiro},
			{Titulo: "Lançado em", Peso: 1.4, Tipo: relatorio.DataHora},
		},
		Linhas: corpo,
		Gerado: agora,
		Capa: &relatorio.Capa{
			Chapeu:     "FROTA MACEDO ENGENHARIA  ·  CONTRATO DE MANUTENÇÃO PREDIAL",
			Periodo:    "Lançados em " + mes,
			Assinatura: "Gerado em " + agora.Format("02/01/2006 15:04") + " por " + p.Nome + " através do FrotaHub",
			Resumo:     fmt.Sprintf("%d orçamentos", len(corpo)),
			Destaque:   "R$ " + emReais(soma),
		},
	}
	bytesXLSX, err := tab.Planilha()
	if err != nil {
		m.erro(w, "não consegui montar a planilha", err)
		return
	}
	nome := fmt.Sprintf("material-lancado-%s.xlsx", agora.Format("200601021504"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`"`)
	w.Header().Set("Content-Length", fmt.Sprint(len(bytesXLSX)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytesXLSX)
}
