package rogueworker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/consolidacao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/estatisticas"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/orcamentos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/servicos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

// tetoDoRetrato é o máximo de JSON que vai para o Groq numa rodada. Acima
// disso o modelo começa a inventar, ou a chamada estoura o tempo. Listas
// grandes viram total + amostra — o número certo mora no `total` do handler,
// não na soma das linhas que couberam.
const tetoDoRetrato = 20 << 10

const maxFontesPorTurno = 4

// rotinaFuncionariosDados é o código da migração 044. Não importamos o
// pacote funcionarios: ele ainda entra e sai do working tree, e a Rogue
// Worker não pode deixar de compilar por causa disso. O handler original
// continua sendo o portão — se a rota não existir, o GET volta 404.
const rotinaFuncionariosDados = "CONTRATO_FUNCIONARIOS_DADOS"

// fonteLeitura é um GET que o clique do usuário já faz. A conversa livre
// não lê tabela direto: pede o mesmo handler, com o mesmo Bearer, e a
// rotina original recusa se este login não alcança.
type fonteLeitura struct {
	id     string
	rotina string
	// caminho vazio = esta fonte não é HTTP (resumo montado aqui, da mesma
	// view que a tela de Estatísticas já usa).
	caminho func(frase string) string
}

func catalogoDeFontes() []fonteLeitura {
	return []fonteLeitura{
		{id: "orcamentos_painel", rotina: orcamentos.RotinaModulo,
			caminho: fixo("/orcamentos/painel")},
		{id: "orcamentos_pendencias", rotina: orcamentos.RotinaCorrecoes,
			caminho: fixo("/orcamentos/pendencias?destino=cliente")},
		{id: "orcamentos_pedido", rotina: orcamentos.RotinaPagar,
			caminho: fixo("/orcamentos/pedido")},
		{id: "orcamentos_faturamento", rotina: orcamentos.RotinaFaturar,
			caminho: fixo("/orcamentos/faturamento")},
		{id: "orcamentos_fechamento", rotina: orcamentos.RotinaFaturar,
			caminho: fixo("/orcamentos/relatorio-mensal")},
		{id: "servicos_painel", rotina: servicos.RotinaGerenciar,
			caminho: fixo("/servicos/painel")},
		{id: "servicos_lista", rotina: servicos.RotinaGerenciar,
			caminho: fixo("/servicos/lista?pagina=1&por_pagina=20")},
		{id: "servicos_kanban", rotina: servicos.RotinaGerenciar,
			caminho: fixo("/servicos/kanban")},
		{id: "trilogo_chamados", rotina: trilogo.RotinaDados,
			caminho: func(frase string) string {
				c := "/trilogo/chamados?pagina=1&por_pagina=100"
				if n := extrairTicket(frase); n > 0 {
					return c + "&ticket=" + strconv.Itoa(n)
				}
				return c
			}},
		{id: "trilogo_chamado", rotina: trilogo.RotinaDados,
			caminho: func(frase string) string {
				n := extrairTicket(frase)
				if n == 0 {
					return ""
				}
				return "/trilogo/chamados/" + strconv.Itoa(n)
			}},
		{id: "estatisticas_resumo", rotina: estatisticas.RotinaModulo},
		{id: "consolidacao", rotina: consolidacao.RotinaModulo,
			caminho: fixo("/consolidacao")},
		{id: "funcionarios_conformidade", rotina: rotinaFuncionariosDados,
			caminho: fixo("/funcionarios/conformidade")},
		{id: "funcionarios_vencendo", rotina: rotinaFuncionariosDados,
			caminho: fixo("/funcionarios/vencendo")},
		{id: "funcionarios_lista", rotina: rotinaFuncionariosDados,
			caminho: fixo("/funcionarios?pagina=1&por_pagina=25")},
		{id: "robos_trilogo", rotina: trilogo.RotinaDados,
			caminho: fixo("/robos/trilogo/rodadas")},
		{id: "usuarios", rotina: "",
			caminho: fixo("/usuarios?pagina=1&por_pagina=25")},
	}
}

func fixo(c string) func(string) string {
	return func(string) string { return c }
}

func fontePorID(id string) (fonteLeitura, bool) {
	for _, f := range catalogoDeFontes() {
		if f.id == id {
			return f, true
		}
	}
	return fonteLeitura{}, false
}

// inferirFontes escolhe o que ler quando o modelo não aponta, ou aponta pouco.
// Palavra da pergunta, não regex de comando: o comando fixo já rodou antes.
func inferirFontes(frase string) []string {
	s := normalizar(frase)
	var ids []string
	poe := func(id string) {
		for _, x := range ids {
			if x == id {
				return
			}
		}
		ids = append(ids, id)
	}
	switch {
	case strings.Contains(s, "funcionario") || strings.Contains(s, "sesmt") ||
		strings.Contains(s, "aso") || strings.Contains(s, "nr-") ||
		strings.Contains(s, "nr ") || strings.Contains(s, "conformidade") ||
		strings.Contains(s, "documento venc"):
		poe("funcionarios_conformidade")
		if strings.Contains(s, "venc") {
			poe("funcionarios_vencendo")
		} else {
			poe("funcionarios_lista")
		}
	case strings.Contains(s, "servico") || strings.Contains(s, "kanban") ||
		strings.Contains(s, "candidato") || strings.Contains(s, "pco") ||
		strings.Contains(s, "cotacao"):
		poe("servicos_painel")
		if strings.Contains(s, "kanban") {
			poe("servicos_kanban")
		} else {
			poe("servicos_lista")
		}
	case strings.Contains(s, "consolida") || strings.Contains(s, "obra prima") ||
		strings.Contains(s, "intrusa"):
		poe("consolidacao")
	case strings.Contains(s, "usuario") || strings.Contains(s, "login") ||
		strings.Contains(s, "permiss") || strings.Contains(s, "categoria"):
		poe("usuarios")
	case strings.Contains(s, "robo") || strings.Contains(s, "sincron") ||
		strings.Contains(s, "ultima leitura") || strings.Contains(s, "rodada"):
		poe("robos_trilogo")
	case strings.Contains(s, "material") || strings.Contains(s, "gasta"):
		poe("orcamentos_painel")
	case strings.Contains(s, "chamado") || strings.Contains(s, "ticket") ||
		strings.Contains(s, "atend") || strings.Contains(s, "execut"):
		poe("estatisticas_resumo")
		if extrairTicket(frase) > 0 {
			poe("trilogo_chamado")
		} else {
			poe("trilogo_chamados")
		}
	case strings.Contains(s, "fatur") || strings.Contains(s, "fechamento"):
		poe("orcamentos_faturamento")
		poe("orcamentos_fechamento")
	case strings.Contains(s, "pagamos") || strings.Contains(s, "fornecedor") || strings.Contains(s, "dav"):
		poe("orcamentos_pedido")
	case strings.Contains(s, "pendente"):
		poe("orcamentos_pendencias")
	case strings.Contains(s, "orcamento") || strings.Contains(s, "nota"):
		poe("orcamentos_painel")
	}
	return ids
}

func fontesDoRetratoGeral() []string {
	return []string{"orcamentos_painel", "servicos_painel", "estatisticas_resumo", "funcionarios_conformidade"}
}

func juntarFontes(partes ...[]string) []string {
	var fora []string
	visto := map[string]bool{}
	for _, p := range partes {
		for _, id := range p {
			id = strings.TrimSpace(id)
			if id == "" || visto[id] {
				continue
			}
			if _, ok := fontePorID(id); !ok {
				continue
			}
			visto[id] = true
			fora = append(fora, id)
			if len(fora) >= maxFontesPorTurno {
				return fora
			}
		}
	}
	return fora
}

func (m *Modulo) buscarFontes(r *http.Request, p *seguranca.Principal, frase string, ids []string) map[string]any {
	fora := map[string]any{}
	for _, id := range ids {
		f, ok := fontePorID(id)
		if !ok {
			continue
		}
		if f.rotina != "" && !m.pode(r, p, f.rotina) {
			fora[id] = map[string]any{"erro": "este login não alcança"}
			continue
		}
		if id == "estatisticas_resumo" {
			resumo, err := m.resumoEstatisticas(r, p, frase)
			if err != nil {
				fora[id] = map[string]any{"erro": err.Error()}
				continue
			}
			fora[id] = resumo
			continue
		}
		if f.caminho == nil {
			continue
		}
		caminho := f.caminho(frase)
		if caminho == "" {
			continue
		}
		st, bruto, err := m.chamarInterno(r, http.MethodGet, caminho, nil)
		if err != nil {
			fora[id] = map[string]any{"erro": "não consegui consultar agora"}
			continue
		}
		if st == http.StatusForbidden {
			fora[id] = map[string]any{"erro": "este login não alcança"}
			continue
		}
		if st != http.StatusOK {
			fora[id] = map[string]any{"erro": m.erroInterno(st, bruto)}
			continue
		}
		fora[id] = compactarBruto(bruto)
	}
	return fora
}

func (m *Modulo) resumoEstatisticas(r *http.Request, p *seguranca.Principal, frase string) (map[string]any, error) {
	inicio, fim, rotulo := periodoDoPedido(frase)
	abertos, err := m.contarChamadosAbertos(r, p, inicio, fim)
	if err != nil {
		return nil, err
	}
	atendidos, err := m.contarChamadosAtendidos(r, p, inicio, fim)
	if err != nil {
		return nil, err
	}
	agora, err := m.contarChamadosAbertosAgora(r, p)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"periodo":              rotulo,
		"abertos_no_periodo":   abertos,
		"atendidos_no_periodo": atendidos,
		"abertos_agora":        agora,
		"o_que_e_atendido":     "primeira vez em Executado na linha do tempo (a mesma conta da tela de Estatísticas)",
	}, nil
}

func (m *Modulo) contarChamadosAbertosAgora(r *http.Request, p *seguranca.Principal) (map[string]int, error) {
	caminho := "estatisticas_chamados?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=status&limit=" + strconv.Itoa(estatisticas.Teto)
	var linhas []struct {
		Status *string `json:"status"`
	}
	total, err := m.bd.BuscarContando(r.Context(), caminho, &linhas)
	if err != nil {
		return nil, err
	}
	if total != len(linhas) {
		return nil, fmt.Errorf("estatisticas_chamados: o banco diz %d e chegaram %d", total, len(linhas))
	}
	contagem := map[string]int{}
	abertos := 0
	for _, l := range linhas {
		st := "sem status"
		if l.Status != nil && *l.Status != "" {
			st = *l.Status
		}
		contagem[st]++
		if st == "Aberto" || st == "Em execução" {
			abertos++
		}
	}
	contagem["_abertos_ou_em_execucao"] = abertos
	contagem["_total"] = len(linhas)
	return contagem, nil
}

func compactarBruto(bruto []byte) any {
	if len(bytes.TrimSpace(bruto)) == 0 {
		return map[string]any{}
	}
	if len(bruto) <= tetoDoRetrato {
		return json.RawMessage(append([]byte(nil), bruto...))
	}
	dec := json.NewDecoder(bytes.NewReader(bruto))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		s := strings.TrimSpace(string(bruto))
		if len(s) > 400 {
			s = s[:400] + "…"
		}
		return s
	}
	c := compactarValor(v, 0)
	saiu, err := json.Marshal(c)
	if err != nil || len(saiu) <= tetoDoRetrato {
		return c
	}
	return map[string]any{"aviso": "recorte: o dado é maior do que cabe na conversa", "amostra": compactarValor(v, 1)}
}

func compactarValor(v any, aperto int) any {
	limite := 12
	if aperto > 0 {
		limite = 6
	}
	switch x := v.(type) {
	case map[string]any:
		fora := map[string]any{}
		for k, val := range x {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "sha256") || strings.Contains(lk, "arquivo") ||
				strings.Contains(lk, "pdf") || strings.Contains(lk, "token") {
				continue
			}
			fora[k] = compactarValor(val, aperto)
		}
		return fora
	case []any:
		if len(x) <= limite {
			amostra := make([]any, len(x))
			for i := range x {
				amostra[i] = compactarValor(x[i], aperto+1)
			}
			return amostra
		}
		amostra := make([]any, limite)
		for i := 0; i < limite; i++ {
			amostra[i] = compactarValor(x[i], aperto+1)
		}
		return map[string]any{"total": len(x), "amostra": amostra}
	default:
		return x
	}
}
