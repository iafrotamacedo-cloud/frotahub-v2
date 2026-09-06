package rogueworker

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/estatisticas"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

// marcoExecutado é o código de status "Executado" na linha do tempo do Trílogo.
// A tela de Estatísticas usa o mesmo número (sc=5) — se alguém mudar um lado
// sem o outro, a conversa e o gráfico passam a contar coisas diferentes.
const marcoExecutado = 5

func (m *Modulo) cmdChamadosAtendidos(r *http.Request, p *seguranca.Principal, frase string) Resposta {
	if !m.pode(r, p, estatisticas.RotinaModulo) {
		return Resposta{Texto: "Não achei chamados com o que este login alcança."}
	}
	inicio, fim, rotulo := periodoDoPedido(frase)
	abertos, err := m.contarChamadosAbertos(r, p, inicio, fim)
	if err != nil {
		return Resposta{Texto: "Não consegui contar os chamados abertos agora."}
	}
	atendidos, err := m.contarChamadosAtendidos(r, p, inicio, fim)
	if err != nil {
		return Resposta{Texto: "Não consegui contar os chamados atendidos agora."}
	}
	return Resposta{Texto: fmt.Sprintf(
		"Em %s atendemos %d chamado(s) — primeira vez em Executado, a mesma conta da tela de Estatísticas. Abriram-se %d no mesmo mês.",
		rotulo, atendidos, abertos)}
}

func (m *Modulo) contarChamadosAbertos(r *http.Request, p *seguranca.Principal, inicio, fim time.Time) (int, error) {
	caminho := "estatisticas_chamados?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=numero,criado_em" +
		"&criado_em=gte." + banco.Escapar(inicio.UTC().Format(time.RFC3339)) +
		"&criado_em=lt." + banco.Escapar(fim.UTC().Format(time.RFC3339)) +
		"&limit=" + strconv.Itoa(estatisticas.Teto)
	var linhas []struct {
		Numero   int     `json:"numero"`
		CriadoEm *string `json:"criado_em"`
	}
	total, err := m.bd.BuscarContando(r.Context(), caminho, &linhas)
	if err != nil {
		return 0, err
	}
	if total != len(linhas) {
		return 0, fmt.Errorf("estatisticas_chamados: o banco diz %d e chegaram %d", total, len(linhas))
	}
	return len(linhas), nil
}

func (m *Modulo) contarChamadosAtendidos(r *http.Request, p *seguranca.Principal, inicio, fim time.Time) (int, error) {
	// TODAS as primeiras execuções, depois o recorte do mês.
	//
	// Filtrar `quando` no banco pegaria um retrabalho de agosto e contaria de
	// novo um chamado que já tinha sido executado em julho. A régua do dono
	// (30/08/2026): retrabalho não conta duas vezes — vale o primeiro instante
	// do marco Executado.
	caminho := "estatisticas_eventos?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=chamado,sc,quando" +
		"&sc=eq." + strconv.Itoa(marcoExecutado) +
		"&order=chamado,quando" +
		"&limit=" + strconv.Itoa(estatisticas.Teto)
	var linhas []struct {
		Chamado int     `json:"chamado"`
		SC      int     `json:"sc"`
		Quando  *string `json:"quando"`
	}
	total, err := m.bd.BuscarContando(r.Context(), caminho, &linhas)
	if err != nil {
		return 0, err
	}
	if total != len(linhas) {
		return 0, fmt.Errorf("estatisticas_eventos: o banco diz %d e chegaram %d", total, len(linhas))
	}
	primeiro := map[int]time.Time{}
	for _, l := range linhas {
		if l.Quando == nil {
			continue
		}
		t, ok := instanteDe(*l.Quando)
		if !ok {
			continue
		}
		if ant, tem := primeiro[l.Chamado]; !tem || t.Before(ant) {
			primeiro[l.Chamado] = t
		}
	}
	n := 0
	for _, t := range primeiro {
		if !t.Before(inicio) && t.Before(fim) {
			n++
		}
	}
	return n, nil
}

func instanteDe(iso string) (time.Time, bool) {
	iso = strings.TrimSpace(iso)
	if iso == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, iso); err == nil {
		return t.In(relatorio.FusoDaCasa()), true
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", strings.Replace(iso, " ", "T", 1), time.UTC); err == nil {
		return t.In(relatorio.FusoDaCasa()), true
	}
	if len(iso) >= 10 {
		if t, err := time.ParseInLocation("2006-01-02", iso[:10], relatorio.FusoDaCasa()); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

var nomesDeMes = []string{
	"janeiro", "fevereiro", "marco", "abril", "maio", "junho",
	"julho", "agosto", "setembro", "outubro", "novembro", "dezembro",
}

func periodoDoPedido(frase string) (inicio, fim time.Time, rotulo string) {
	agora := time.Now().In(relatorio.FusoDaCasa())
	ano, mes := agora.Year(), agora.Month()
	s := normalizar(frase)

	switch {
	case strings.Contains(s, "mes passado") || strings.Contains(s, "ultimo mes"):
		t := agora.AddDate(0, -1, 0)
		ano, mes = t.Year(), t.Month()
	case strings.Contains(s, "este mes") || strings.Contains(s, "neste mes") || strings.Contains(s, "esse mes"):
		// o mês corrente, Fortaleza
	default:
		achou := false
		for i, nome := range nomesDeMes {
			if strings.Contains(s, nome) {
				mes = time.Month(i + 1)
				achou = true
				break
			}
		}
		if !achou {
			t := agora.AddDate(0, -1, 0)
			ano, mes = t.Year(), t.Month()
			break
		}
		if mes > agora.Month() {
			ano--
		}
	}
	if i := strings.Index(s, "20"); i >= 0 && i+4 <= len(s) {
		if n, err := strconv.Atoi(s[i : i+4]); err == nil && n >= 2020 && n <= 2099 {
			ano = n
		}
	}
	inicio = time.Date(ano, mes, 1, 0, 0, 0, 0, agora.Location())
	fim = inicio.AddDate(0, 1, 0)
	rotulo = inicio.Format("01/2006")
	return
}

func pareceQueNaoEntendeu(s string) bool {
	s = normalizar(s)
	return strings.Contains(s, "nao entendi") || strings.Contains(s, "nao reconheci") ||
		s == "desculpe" || strings.HasPrefix(s, "desculpe")
}
