package rogueworker

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// reconhecer tenta casar a frase com um dos comandos desta leva, SEM Groq.
// Ordem importa: lote antes do item, navegar com nome de tela antes do resto.
func reconhecer(frase string) reconhecimento {
	s := normalizar(frase)
	if s == "" {
		return reconhecimento{comando: cmdDesconhecido}
	}

	if r := reconhecerNavegar(s); r.comando != "" {
		return r
	}

	switch {
	case contemTodos(s, "todas", "notas") && (strings.Contains(s, "gere") || strings.Contains(s, "gerar") || strings.Contains(s, "orçamento") || strings.Contains(s, "orcamento")):
		return reconhecimento{comando: cmdGerarLote}
	case (strings.Contains(s, "gere") || strings.Contains(s, "gerar")) &&
		(strings.Contains(s, "fila") || strings.Contains(s, "todas") || strings.Contains(s, "todos")):
		return reconhecimento{comando: cmdGerarLote}
	case (strings.Contains(s, "lance") || strings.Contains(s, "lancar") || strings.Contains(s, "lançar")) &&
		(strings.Contains(s, "todos") || strings.Contains(s, "todas") || strings.Contains(s, "fila")):
		return reconhecimento{comando: cmdLancarLote}
	case strings.Contains(s, "gere") || strings.Contains(s, "gerar orçamento") || strings.Contains(s, "gerar orcamento") ||
		(strings.Contains(s, "gerar") && strings.Contains(s, "orcamento")):
		r := reconhecimento{comando: cmdGerar, id: extrairUUID(frase), ticket: extrairTicket(frase)}
		return r
	case strings.Contains(s, "lance") || strings.Contains(s, "lancar") || strings.Contains(s, "lançar") ||
		(strings.Contains(s, "trilog") && (strings.Contains(s, "lanc") || strings.Contains(s, "orçamento") || strings.Contains(s, "orcamento"))):
		return reconhecimento{comando: cmdLancar, id: extrairUUID(frase), ticket: extrairTicket(frase)}
	}

	switch {
	case strings.Contains(s, "pendente"):
		return reconhecimento{comando: cmdPendencias}
	case strings.Contains(s, "extrapolad") || (strings.Contains(s, "teto") && (strings.Contains(s, "pass") || strings.Contains(s, "estour") || strings.Contains(s, "acima") || strings.Contains(s, "quais"))):
		return reconhecimento{comando: cmdExtrapoladas}
	case strings.Contains(s, "bloquead") || strings.Contains(s, "repetid") || strings.Contains(s, "duplicad"):
		return reconhecimento{comando: cmdBloqueadas}
	case strings.Contains(s, "status") || strings.Contains(s, "como esta a nota") || strings.Contains(s, "como esta o ticket") ||
		strings.Contains(s, "como esta essa nota") || strings.Contains(s, "situacao da nota") || strings.Contains(s, "situacao do ticket"):
		return reconhecimento{comando: cmdStatusNota, id: extrairUUID(frase), ticket: extrairTicket(frase)}
	}

	switch {
	case strings.Contains(s, "pagamos") || strings.Contains(s, "pagar a fornecedor") ||
		(strings.Contains(s, "fornecedor") && (strings.Contains(s, "quanto") || strings.Contains(s, "pag"))):
		return reconhecimento{comando: cmdPagamos}
	case strings.Contains(s, "fechamento") || strings.Contains(s, "relatorio mensal") || strings.Contains(s, "relatório mensal"):
		return reconhecimento{comando: cmdFechamento}
	case strings.Contains(s, "falta faturar") || strings.Contains(s, "ja foi faturado") || strings.Contains(s, "já foi faturado") ||
		strings.Contains(s, "faturado ao cliente") || strings.Contains(s, "quanto falta") ||
		(strings.Contains(s, "fatur") && strings.Contains(s, "quanto")):
		return reconhecimento{comando: cmdFaturado}
	case strings.Contains(s, "material") && (strings.Contains(s, "lancad") || strings.Contains(s, "lançad") || strings.Contains(s, "contrato")):
		return reconhecimento{comando: cmdMaterial}
	}

	if r := reconhecerChamados(s); r.comando != "" {
		return r
	}

	return reconhecimento{comando: cmdDesconhecido}
}

// reconhecerChamados pega "quantos chamados atendemos mês passado" e afins.
// Navegar para a tela de estatística já rodou antes — aqui só entra contagem.
func reconhecerChamados(s string) reconhecimento {
	if !strings.Contains(s, "chamado") && !strings.Contains(s, "ticket") {
		return reconhecimento{}
	}
	if strings.Contains(s, "status") {
		return reconhecimento{}
	}
	pede := strings.Contains(s, "quant") || strings.Contains(s, "atend") ||
		strings.Contains(s, "execut") || strings.Contains(s, "abrimos")
	if !pede {
		return reconhecimento{}
	}
	return reconhecimento{comando: cmdChamadosAtendidos}
}

func reconhecerNavegar(s string) reconhecimento {
	pede := strings.Contains(s, "me mostra") || strings.Contains(s, "mostra a tela") ||
		strings.Contains(s, "abre a tela") || strings.Contains(s, "abrir a tela") ||
		strings.Contains(s, "vai para") || strings.Contains(s, "vai pra") ||
		strings.Contains(s, "leva pra") || strings.Contains(s, "leva para") ||
		strings.Contains(s, "quero ir") || strings.Contains(s, "onde fica")
	if !pede && !strings.Contains(s, "tela de") && !strings.Contains(s, "estatistica") {
		return reconhecimento{}
	}
	melhor := reconhecimento{}
	melhorLen := 0
	for _, t := range catalogoDeTelas {
		for _, n := range t.nomes {
			if strings.Contains(s, n) && len(n) > melhorLen {
				melhor = reconhecimento{comando: cmdNavegar, tela: t.tela}
				melhorLen = len(n)
			}
		}
	}
	if melhor.comando != "" {
		return melhor
	}
	if pede {
		return reconhecimento{comando: cmdNavegar}
	}
	return reconhecimento{}
}

func contemTodos(s string, partes ...string) bool {
	for _, p := range partes {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}

func ehSim(s string) bool {
	s = strings.TrimSpace(normalizar(s))
	switch s {
	case "sim", "s", "ss", "ok", "okay", "confirmo", "confirma", "pode", "pode ser",
		"vai", "vai la", "vai lá", "isso", "claro", "positivo", "yes", "y":
		return true
	}
	return strings.HasPrefix(s, "sim ") || strings.HasPrefix(s, "pode ") || strings.HasPrefix(s, "confirmo")
}

func ehNao(s string) bool {
	s = strings.TrimSpace(normalizar(s))
	switch s {
	case "nao", "n", "no", "cancela", "cancelar", "deixa", "deixa quieto", "negativo", "agora nao":
		return true
	}
	return strings.HasPrefix(s, "nao ") || strings.HasPrefix(s, "não ")
}

func ehManutencao(s string) bool {
	s = normalizar(s)
	return s == "manutencao" || s == "manutenção" || strings.Contains(s, "manutencao") || strings.Contains(s, "manutenção") ||
		s == "contrato" || s == "orcamentos" || s == "orçamentos"
}

func ehServicos(s string) bool {
	s = normalizar(s)
	return s == "servicos" || s == "serviços" || s == "servico" || s == "serviço" ||
		strings.Contains(s, "servicos") || strings.Contains(s, "serviços") || strings.Contains(s, "instalac")
}

var reUUID = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
var reTicket = regexp.MustCompile(`\b(\d{4,7})\b`)

func extrairUUID(s string) string {
	return strings.ToLower(reUUID.FindString(s))
}

func extrairTicket(s string) int {
	m := reTicket.FindString(s)
	if m == "" {
		return 0
	}
	n, _ := strconv.Atoi(m)
	return n
}

func umUUID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return "", false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return "", false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", false
		}
	}
	return s, true
}

// normalizar deixa a frase comparável: minúscula, sem acento, espaços simples.
// Sem biblioteca: o motor não tem dependência externa, e não vai ter por causa
// de um ã.
func normalizar(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	ultimoEspaco := true
	for _, r := range s {
		if n := semAcento[r]; n != 0 {
			r = n
		}
		if unicode.IsSpace(r) || r == '?' || r == '!' || r == '.' || r == ',' || r == ';' || r == ':' {
			if !ultimoEspaco {
				b.WriteByte(' ')
				ultimoEspaco = true
			}
			continue
		}
		b.WriteRune(r)
		ultimoEspaco = false
	}
	return strings.TrimSpace(b.String())
}

var semAcento = map[rune]rune{
	'á': 'a', 'à': 'a', 'ã': 'a', 'â': 'a', 'ä': 'a',
	'é': 'e', 'ê': 'e', 'è': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'õ': 'o', 'ô': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c',
}

func tokensDe(s string) []string {
	s = normalizar(s)
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

// pareceCom mede se a pergunta nova é a mesma coisa que o formato canônico
// aprendido. Não é regex escrita à mão: é sobreposição de tokens, o bastante
// para "quantas notas pendentes" casar com "qtas notas estão pendentes agora"
// e não casar com "lance o orçamento".
func pareceCom(pergunta, canonico string) bool {
	a := tokensDe(pergunta)
	b := tokensDe(canonico)
	if len(b) == 0 || len(a) == 0 {
		return false
	}
	set := map[string]bool{}
	for _, t := range a {
		if len(t) < 3 {
			continue
		}
		set[t] = true
	}
	batem := 0
	uteis := 0
	for _, t := range b {
		if len(t) < 3 {
			continue
		}
		uteis++
		if set[t] {
			batem++
		}
	}
	if uteis == 0 {
		return false
	}
	return batem*2 >= uteis && batem >= 2
}
