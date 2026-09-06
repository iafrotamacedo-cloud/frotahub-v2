package rogueworker

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/orcamentos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

func (m *Modulo) proporOuEntregarRelatorio(r *http.Request, p *seguranca.Principal, rec reconhecimento, frase string) Resposta {
	s := normalizar(frase)
	// Só existe conta de Manutenção nestes relatórios. Perguntar "Manutenção
	// ou Serviços?" e depois recusar Serviços era dois turnos para um número
	// que já dava para devolver.
	if ehServicos(s) {
		return Resposta{
			Texto:    "Nesta versão eu só consulto o módulo de Orçamentos (Manutenção). Quer que eu calcule pela Manutenção?",
			Pendente: &Pendente{Tipo: "escopo", Comando: rec.comando},
			Opcoes:   []string{"Manutenção", "Agora não"},
		}
	}
	return m.entregarRelatorio(r, p, rec.comando, frase)
}

func (m *Modulo) entregarRelatorio(r *http.Request, p *seguranca.Principal, comando, frase string) Resposta {
	switch comando {
	case cmdPagamos:
		return m.relPagamos(r, p)
	case cmdMaterial:
		return m.relMaterial(r, p, frase)
	case cmdFaturado:
		return m.relFaturado(r, p)
	case cmdFechamento:
		return m.relFechamento(r, p)
	default:
		return Resposta{Texto: "Não sei montar esse relatório ainda."}
	}
}

func (m *Modulo) relPagamos(r *http.Request, p *seguranca.Principal) Resposta {
	if !m.pode(r, p, orcamentos.RotinaPagar) {
		return Resposta{Texto: "Não achei pagamento a fornecedor com o que este login alcança."}
	}
	var s struct {
		Quantas int     `json:"quantas"`
		Valor   float64 `json:"valor"`
	}
	st, bruto, err := m.jsonInterno(r, http.MethodGet, "/orcamentos/pedido", nil, &s)
	if err != nil {
		return Resposta{Texto: "Não consegui montar o pedido ao fornecedor agora."}
	}
	if st != http.StatusOK {
		return Resposta{Texto: m.erroInterno(st, bruto)}
	}
	texto := fmt.Sprintf("Há %d DAV(s) em aberto com o fornecedor, somando R$ %s. É o que ainda falta pedir faturamento — o que já foi pedido saiu desta lista.",
		s.Quantas, emReais(s.Valor))
	return m.comExcel(texto, "/orcamentos/pedido.xlsx")
}

func (m *Modulo) relMaterial(r *http.Request, p *seguranca.Principal, frase string) Resposta {
	if !m.pode(r, p, orcamentos.RotinaLancar) {
		return Resposta{Texto: "Não achei material lançado com o que este login alcança."}
	}
	linhas, _, err := m.paginar(r, "/orcamentos?status=lancado")
	if err != nil {
		return Resposta{Texto: "Não consegui listar os orçamentos lançados agora."}
	}
	inicio, fim, rotulo := periodoDoPedido(frase)
	var soma float64
	quantos := 0
	for _, l := range linhas {
		quando := textoDe(l["lancado_em"])
		if quando == "" {
			quando = textoDe(l["criado_em"])
		}
		if !noIntervalo(quando, inicio, fim) {
			continue
		}
		soma += numeroDe(l["valor"])
		quantos++
	}
	texto := fmt.Sprintf("Em %s foram lançados %d orçamento(s) no contrato, somando R$ %s de material.",
		rotulo, quantos, emReais(soma))
	return m.comExcel(texto, "/rogueworker/excel?o=material&mes="+inicio.Format("2006-01"))
}

func (m *Modulo) relFaturado(r *http.Request, p *seguranca.Principal) Resposta {
	if !m.pode(r, p, orcamentos.RotinaFaturar) {
		return Resposta{Texto: "Não achei faturamento com o que este login alcança."}
	}
	var s struct {
		Orcamentos int     `json:"orcamentos"`
		Valor      float64 `json:"valor"`
		Faturas    int     `json:"faturas"`
	}
	st, bruto, err := m.jsonInterno(r, http.MethodGet, "/orcamentos/faturamento", nil, &s)
	if err != nil {
		return Resposta{Texto: "Não consegui montar o faturamento agora."}
	}
	if st != http.StatusOK {
		return Resposta{Texto: m.erroInterno(st, bruto)}
	}
	texto := fmt.Sprintf("Ainda faltam faturar ao cliente %d orçamento(s) em %d fatura(s) possível(is), somando R$ %s. O que já foi faturado saiu desta fila.",
		s.Orcamentos, s.Faturas, emReais(s.Valor))
	return m.comExcel(texto, "/orcamentos/faturamento.xlsx")
}

func (m *Modulo) relFechamento(r *http.Request, p *seguranca.Principal) Resposta {
	if !m.pode(r, p, orcamentos.RotinaFaturar) {
		return Resposta{Texto: "Não achei fechamento do mês com o que este login alcança."}
	}
	var s struct {
		Quantos int     `json:"quantos"`
		Valor   float64 `json:"valor"`
	}
	st, bruto, err := m.jsonInterno(r, http.MethodGet, "/orcamentos/relatorio-mensal", nil, &s)
	if err != nil {
		return Resposta{Texto: "Não consegui montar o fechamento agora."}
	}
	if st != http.StatusOK {
		return Resposta{Texto: m.erroInterno(st, bruto)}
	}
	texto := fmt.Sprintf("Fechamento: %d orçamento(s) lançados e ainda não cobrados, somando R$ %s. É o relatório mensal que vai ao cliente.",
		s.Quantos, emReais(s.Valor))
	return m.comExcel(texto, "/orcamentos/relatorio-mensal.xlsx")
}

func (m *Modulo) comExcel(texto, caminho string) Resposta {
	return Resposta{
		Texto:    texto + "\n\nQuer a planilha Excel?",
		Pendente: &Pendente{Tipo: "excel", Excel: caminho},
		Excel:    &OfertaExcel{Caminho: caminho},
		Opcoes:   []string{"Sim, baixar", "Agora não"},
	}
}

func mesCorrente() (time.Time, time.Time) {
	agora := time.Now().In(relatorio.FusoDaCasa())
	inicio := time.Date(agora.Year(), agora.Month(), 1, 0, 0, 0, 0, agora.Location())
	fim := inicio.AddDate(0, 1, 0)
	return inicio, fim
}

func noIntervalo(iso string, inicio, fim time.Time) bool {
	iso = strings.TrimSpace(iso)
	if iso == "" {
		return false
	}
	if t, err := time.Parse(time.RFC3339, iso); err == nil {
		return !t.Before(inicio) && t.Before(fim)
	}
	if len(iso) >= 10 {
		if t, err := time.ParseInLocation("2006-01-02", iso[:10], relatorio.FusoDaCasa()); err == nil {
			return !t.Before(inicio) && t.Before(fim)
		}
	}
	return false
}
