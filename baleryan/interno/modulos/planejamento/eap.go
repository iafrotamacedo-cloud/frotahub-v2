// rev 1 — a árvore da EAP
//
// Grupo/Fase/Item/Subitem é VISÃO de negócio (o nível 1..4), não são tabelas
// separadas — todo nó mora em `eap_nos`, auto-referenciada por `pai_id`. Este
// arquivo só sabe montar/ordenar a árvore e calcular `codigo_eap`; quem chama
// o banco é `eap_http.go`.
package planejamento

import (
	"strconv"
	"strings"
)

func codigoEAPRaiz(ordem int) string {
	return strconv.Itoa(ordem)
}

func codigoEAPFilho(codigoPai string, ordem int) string {
	if codigoPai == "" {
		return codigoEAPRaiz(ordem)
	}
	return codigoPai + "." + strconv.Itoa(ordem)
}

func nivelFilho(nivelPai int) int {
	if nivelPai <= 0 {
		return 1
	}
	return nivelPai + 1
}

func asInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// montarArvore transforma a lista plana que o PostgREST devolve numa árvore
// ordenada por ordem/codigo_eap. O grão de entrada é map[string]any porque a
// lista vem de um `select=*` genérico — a EAP tem colunas demais para valer a
// pena um struct aqui, e a tela só reempacota o que já está no JSON.
func montarArvore(nos []map[string]any) []map[string]any {
	porID := map[string]map[string]any{}
	raizes := []map[string]any{}
	for _, n := range nos {
		id := asString(n["id"])
		copia := make(map[string]any, len(n)+1)
		for k, v := range n {
			copia[k] = v
		}
		copia["filhos"] = []map[string]any{}
		porID[id] = copia
	}
	for _, n := range porID {
		paiID := asString(n["pai_id"])
		if paiID == "" {
			raizes = append(raizes, n)
			continue
		}
		if pai, ok := porID[paiID]; ok {
			filhos := pai["filhos"].([]map[string]any)
			pai["filhos"] = append(filhos, n)
		} else {
			raizes = append(raizes, n)
		}
	}
	ordenarArvore(raizes)
	return raizes
}

func ordenarArvore(nos []map[string]any) {
	ordenarNivel(nos)
	for _, n := range nos {
		if filhos, ok := n["filhos"].([]map[string]any); ok && len(filhos) > 0 {
			ordenarArvore(filhos)
		}
	}
}

// ordenarNivel usa inserção simples — a EAP de uma obra tem dezenas de itens,
// não milhares; clareza vale mais que um algoritmo mais rápido aqui.
func ordenarNivel(nos []map[string]any) {
	for i := 1; i < len(nos); i++ {
		j := i
		for j > 0 && menor(nos[j], nos[j-1]) {
			nos[j], nos[j-1] = nos[j-1], nos[j]
			j--
		}
	}
}

func menor(a, b map[string]any) bool {
	oa, ob := asInt(a["ordem"]), asInt(b["ordem"])
	if oa != ob {
		return oa < ob
	}
	return asString(a["codigo_eap"]) < asString(b["codigo_eap"])
}

// parseOrdemCodigo extrai o último segmento numérico do codigo_eap ("1.2.3" → 3).
// Hoje não é usada pelas rotas (a ordem vem de `proximaOrdem`), mas fica junto
// das outras contas da EAP para quem precisar decompor um código existente.
func parseOrdemCodigo(codigo string) int {
	partes := strings.Split(codigo, ".")
	if len(partes) == 0 {
		return 1
	}
	n, _ := strconv.Atoi(partes[len(partes)-1])
	if n <= 0 {
		return 1
	}
	return n
}
