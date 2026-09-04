package trilogo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// servicoDeMentira é um Trílogo de mentira só com os endereços deste
// arquivo (servico.go) — não usa o `mundo` de prova_test.go de propósito:
// aquele existe para o robô inteiro (listar, detalhe, custos); aqui o alvo é
// só responsável, cotação e orçamento.
type servicoDeMentira struct {
	assignee   *int // nil = sem responsável
	quotations map[int]map[string]any
	budgets    map[int]map[string]any
	proxID     int
}

func novaSessaoDeServico(t *testing.T) (*Sessao, *servicoDeMentira) {
	t.Helper()
	m := &servicoDeMentira{
		quotations: map[int]map[string]any{},
		budgets:    map[int]map[string]any{},
		proxID:     270001,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/Login/SignIn", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"accessToken": "tok-teste", "authenticated": true})
	})

	mux.HandleFunc("PUT /api/Ticket/ChangeServiceCompanyAssignee", func(w http.ResponseWriter, r *http.Request) {
		var c struct {
			ServiceCompanyAssigneeID *int `json:"serviceCompanyAssigneeId"`
		}
		json.NewDecoder(r.Body).Decode(&c)
		m.assignee = c.ServiceCompanyAssigneeID
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/Quotation/CreateQuotation", func(w http.ResponseWriter, r *http.Request) {
		var corpo map[string]any
		json.NewDecoder(r.Body).Decode(&corpo)
		id := m.proxID
		m.proxID++
		corpo["id"] = id
		m.quotations[id] = corpo
		json.NewEncoder(w).Encode(map[string]any{"id": id})
	})

	mux.HandleFunc("POST /api/Budget/CreateBudget", func(w http.ResponseWriter, r *http.Request) {
		var corpo map[string]any
		json.NewDecoder(r.Body).Decode(&corpo)
		if _, ok := m.quotations[numeroDe(corpo["quotationId"])]; !ok {
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"cotação não existe"}`))
			return
		}
		id := m.proxID
		m.proxID++
		m.budgets[id] = corpo
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": id, "supplierId": corpo["supplierId"], "contact": nil})
	})

	mux.HandleFunc("DELETE /api/Budget/DeleteBudget/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := numeroDeTexto(r.PathValue("id"))
		if _, ok := m.budgets[id]; !ok {
			w.WriteHeader(404)
			return
		}
		delete(m.budgets, id)
		w.WriteHeader(200)
	})

	mux.HandleFunc("GET /api/Quotation/GetQuotationsTicketDetails", func(w http.ResponseWriter, r *http.Request) {
		var lista []map[string]any
		for _, q := range m.quotations {
			budgets := []map[string]any{}
			hasBudget := false
			for bid, b := range m.budgets {
				if numeroDe(b["quotationId"]) != numeroDe(q["id"]) {
					continue
				}
				hasBudget = true
				budgets = append(budgets, map[string]any{
					"id": bid, "totalValue": somaItens(b),
					"supplier":      map[string]any{"id": b["supplierId"], "name": "FROTA - INSTALAÇÕES"},
					"attachments":   []any{},
					"approvedValue": nil, "isWinning": false,
				})
			}
			copia := map[string]any{}
			for k, v := range q {
				copia[k] = v
			}
			copia["budgets"] = budgets
			copia["hasBudget"] = hasBudget
			if _, ok := copia["status"]; !ok {
				copia["status"] = 1
			}
			if hasBudget {
				copia["status"] = 2
			}
			copia["winningBudgetId"] = nil
			lista = append(lista, copia)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"quotations": lista})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	base = srv.URL
	t.Cleanup(func() { base = "https://web.api.trilogo.app" })

	s, err := Entrar(context.Background(), "instalacoes", "jonas@x", "certa")
	if err != nil {
		t.Fatalf("login de mentira falhou: %v", err)
	}
	return s, m
}

// numeroDe lê um id de dentro de um map[string]any — que tanto pode ter
// vindo de um json.Decode (todo número vira float64) quanto ter sido posto
// à mão pelo próprio dublê, como int puro (caso de "id" em CreateQuotation).
func numeroDe(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func numeroDeTexto(s string) int {
	var n int
	json.Unmarshal([]byte(s), &n)
	return n
}

func somaItens(b map[string]any) float64 {
	var total float64
	for _, chave := range []string{"products", "services"} {
		itens, _ := b[chave].([]any)
		for _, it := range itens {
			mi, _ := it.(map[string]any)
			preco, _ := mi["unitPrice"].(float64)
			qtd, _ := mi["amount"].(float64)
			total += preco * qtd
		}
	}
	return total
}

// ---------------------------------------------------------------------------

func TestMudarResponsavelERemover(t *testing.T) {
	s, m := novaSessaoDeServico(t)
	ctx := context.Background()

	if err := s.MudarResponsavel(ctx, 135613, 67277); err != nil {
		t.Fatalf("MudarResponsavel: %v", err)
	}
	if m.assignee == nil || *m.assignee != 67277 {
		t.Fatalf("responsável não ficou 67277: %v", m.assignee)
	}

	if err := s.RemoverResponsavel(ctx, 135613); err != nil {
		t.Fatalf("RemoverResponsavel: %v", err)
	}
	if m.assignee != nil {
		t.Fatalf("responsável deveria ter voltado a nil, ficou %v", *m.assignee)
	}
}

func TestMudarResponsavelRecusaIDZero(t *testing.T) {
	s, m := novaSessaoDeServico(t)

	err := s.MudarResponsavel(context.Background(), 135613, 0)
	if !errors.Is(err, ErrResponsavelNaoExiste) {
		t.Fatalf("esperava ErrResponsavelNaoExiste, veio %v", err)
	}
	if m.assignee != nil {
		t.Fatalf("não deveria ter chamado o Trílogo — responsável mudou pra %v", *m.assignee)
	}
}

func TestCriarOrcamentoServicoEExcluir(t *testing.T) {
	s, _ := novaSessaoDeServico(t)
	ctx := context.Background()

	cotID, err := s.CriarCotacao(ctx, 135613, "TESTE - captura")
	if err != nil {
		t.Fatalf("CriarCotacao: %v", err)
	}

	orc := MontarOrcamentoServico(cotID, 23927,
		[]ItemOrcamento{{Descricao: "MÃO DE OBRA", Valor: 0.01, Qtd: 1}},
		[]ItemOrcamento{{Descricao: "2 CFP (MATERIAL DE COLAGEM)", Valor: 0.01, Qtd: 1}},
		[]Subido{{Nome: "teste.pdf", Permalink: "https://s3/x/teste.pdf"}},
	)
	orcID, err := s.CriarOrcamentoServico(ctx, orc)
	if err != nil {
		t.Fatalf("CriarOrcamentoServico: %v", err)
	}
	if orcID == 0 {
		t.Fatalf("id do orçamento veio zero")
	}

	cotacoes, err := s.Cotacoes(ctx, 135613)
	if err != nil {
		t.Fatalf("Cotacoes: %v", err)
	}
	var achou *Cotacao
	for i := range cotacoes {
		if cotacoes[i].ID == cotID {
			achou = &cotacoes[i]
		}
	}
	if achou == nil {
		t.Fatalf("cotação %d não apareceu na releitura", cotID)
	}
	if !achou.HasBudget || len(achou.Budgets) != 1 {
		t.Fatalf("cotação deveria ter 1 orçamento, tem %d (hasBudget=%v)", len(achou.Budgets), achou.HasBudget)
	}
	if got := achou.Budgets[0].TotalValue; got != 0.02 {
		t.Fatalf("valor total esperado 0.02 (mão de obra + material), veio %v", got)
	}

	if err := s.ExcluirOrcamentoServico(ctx, orcID); err != nil {
		t.Fatalf("ExcluirOrcamentoServico: %v", err)
	}
	cotacoes, _ = s.Cotacoes(ctx, 135613)
	for i := range cotacoes {
		if cotacoes[i].ID == cotID && len(cotacoes[i].Budgets) != 0 {
			t.Fatalf("orçamento deveria ter sumido depois de excluir")
		}
	}
}
