// rev 1 — a linha da nota que não fecha
package orcamentos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

// moduloComItens responde `documento_itens` com as linhas dadas — e SÓ com as
// colunas que a busca pediu.
//
// O DUBLÊ É TÃO RIGOROSO QUANTO O ORIGINAL (P-30)
//
//	O PostgREST devolve exatamente as colunas do `select=`, nem uma a mais. Um
//	dublê que devolvesse a linha inteira esconderia o defeito de 26/08/2026:
//	o código não PEDIA `valor_total`, e por isso não o tinha. Aqui, quem não
//	pede não recebe — e o teste falha pela mesma razão que a produção falhou.
func moduloComItens(t *testing.T, linhas []map[string]any) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "documento_itens") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		pedidas := map[string]bool{}
		for _, c := range strings.Split(r.URL.Query().Get("select"), ",") {
			pedidas[strings.TrimSpace(c)] = true
		}
		saida := make([]map[string]any, 0, len(linhas))
		for _, l := range linhas {
			corte := map[string]any{}
			for k, v := range l {
				if pedidas[k] {
					corte[k] = v
				}
			}
			saida = append(saida, corte)
		}
		_ = json.NewEncoder(w).Encode(saida)
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// O CASO QUE CUSTOU DINHEIRO — DAV 19329, 26/08/2026
//
//	Item de R$ 225,90 lido com preço unitário zero. A conferência da leitura
//	passou, porque ela soma os TOTAIS e o total batia com a nota. Mas o
//	orçamento recalculava a linha por quantidade × unitário, obtinha zero, e o
//	item sumia: a nota de R$ 490,80 virou orçamento de R$ 317,88.
//
//	R$ 172,92 de prejuízo, sem uma linha de log.
func TestItemSemUnitarioNaoSomeDaConta(t *testing.T) {
	m := moduloComItens(t, []map[string]any{
		{"ordem": 1, "descricao": "DECORA SEDA ACET.LOC BASE MF", "unidade": "UN",
			"quantidade": 1.0, "valor_unitario": 0.0, "valor_total": 225.90},
		{"ordem": 2, "descricao": "DECORA SEDA ACETINADO BRANCO", "unidade": "UN",
			"quantidade": 1.0, "valor_unitario": 264.90, "valor_total": 264.90},
	})

	lidos, err := m.itensDo(context.Background(), "d1")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if lidos.Bloqueio != "" {
		t.Fatalf("bloqueou uma nota recuperável: %s", lidos.Bloqueio)
	}
	if soma := somaDasLinhas(lidos.Linhas); soma != regras.DinheiroDe(490.80) {
		t.Errorf("as linhas somam %s, a nota vale R$ 490,80", soma.Reais())
	}
	if lidos.Linhas[0].Unitario != regras.PrecoDe(225.90) {
		t.Errorf("o unitário do item 1 ficou em %v, esperava 225,90", lidos.Linhas[0].Unitario.Float())
	}
	// O VALOR CERTO NÃO BASTA
	//   A leitura errou um campo. Sair calado ensina a confiar numa leitura
	//   que não merece confiança.
	if lidos.Aviso == "" {
		t.Error("recuperou o preço e não avisou ninguém")
	}
}

// NÚMEROS QUE NÃO SE RECONCILIAM PARAM A NOTA
//
//	Sem quantidade não há divisão possível. Escolher um dos três números aqui
//	seria inventar preço — e inventar preço é pior que parar.
func TestItemQueNaoFechaBloqueiaAGeracao(t *testing.T) {
	m := moduloComItens(t, []map[string]any{
		{"ordem": 1, "descricao": "material", "unidade": "UN",
			"quantidade": 0.0, "valor_unitario": 0.0, "valor_total": 10.00},
	})

	lidos, err := m.itensDo(context.Background(), "d1")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if lidos.Bloqueio == "" {
		t.Fatal("uma linha que não fecha passou calada")
	}
	if !strings.Contains(lidos.Bloqueio, "material") {
		t.Errorf("o bloqueio não diz qual item: %q", lidos.Bloqueio)
	}
}

// A LINHA COERENTE PASSA INTACTA E SEM AVISO
//
//	O aviso tem que significar alguma coisa. Se toda nota vier com ele, ninguém
//	lê o de nenhuma.
func TestLinhaCoerentePassaSemAviso(t *testing.T) {
	m := moduloComItens(t, []map[string]any{
		{"ordem": 1, "descricao": "material", "unidade": "UN",
			"quantidade": 4.0, "valor_unitario": 83.90, "valor_total": 335.60},
	})

	lidos, err := m.itensDo(context.Background(), "d1")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if lidos.Aviso != "" || lidos.Bloqueio != "" {
		t.Errorf("reclamou de uma linha que fecha: aviso=%q bloqueio=%q", lidos.Aviso, lidos.Bloqueio)
	}
	if soma := somaDasLinhas(lidos.Linhas); soma != regras.DinheiroDe(335.60) {
		t.Errorf("somou %s, esperava R$ 335,60", soma.Reais())
	}
}
