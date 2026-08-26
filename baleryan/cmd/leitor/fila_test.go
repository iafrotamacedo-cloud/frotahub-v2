// rev 1 — a fila: tomar um trabalho sem duas máquinas pegarem o mesmo
//
// ESTE TESTE NASCEU DE UM DEFEITO QUE NÃO APARECIA EM LUGAR NENHUM
//
//	Em 26/08/2026 o robô rodou com 7 documentos na fila e 7 trabalhos
//	`na_fila`, e encerrou dizendo "0 lidas · 0 falhas" — a cara exata de fila
//	vazia. A causa: o código tomava a linha com um UPSERT, que o PostgREST
//	monta como `insert ... on conflict do update`. O insert é avaliado antes de
//	o conflito ser resolvido, e sem `cliente_id` e `tipo` — obrigatórios na
//	fila — o banco recusava a linha inteira. O erro caía num `continue` mudo.
//
//	Nada disso quebrava a compilação, nada aparecia no log. Por isso o teste
//	abaixo não confere só o resultado: ele confere o MÉTODO da requisição.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// filaDeMentira responde como o PostgREST: a busca devolve as linhas, e a
// alteração condicional devolve a linha só para quem chegar primeiro.
func filaDeMentira(t *testing.T, jaTomados map[string]bool) (*Leitor, *[]string) {
	t.Helper()
	var metodos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metodos = append(metodos, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]Trabalho{
				{ID: "aaa", ClienteID: "cli", Alvo: "doc-1", Tentativas: 0},
				{ID: "bbb", ClienteID: "cli", Alvo: "doc-2", Tentativas: 1},
			})

		case http.MethodPatch:
			// A CONDIÇÃO TEM QUE ESTAR NO FILTRO
			//   Sem `status=eq.na_fila` ali, duas máquinas tomam a mesma linha.
			if !strings.Contains(r.URL.RawQuery, "status=eq.na_fila") {
				t.Error("a alteração foi feita SEM a condição de corrida no filtro")
			}
			id := ""
			for _, p := range strings.Split(r.URL.RawQuery, "&") {
				if strings.HasPrefix(p, "id=eq.") {
					id = strings.TrimPrefix(p, "id=eq.")
				}
			}
			if jaTomados[id] {
				json.NewEncoder(w).Encode([]Trabalho{}) // outra máquina chegou antes
				return
			}
			json.NewEncoder(w).Encode([]Trabalho{{ID: id, ClienteID: "cli", Alvo: "doc"}})

		default:
			// É AQUI QUE O DEFEITO VOLTARIA A ENTRAR
			t.Errorf("a fila foi tomada com %s — só a alteração condicional (PATCH) serve", r.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Supabase: config.Supabase{URL: srv.URL, ChaveServico: "servico"}}
	return &Leitor{bd: banco.Novo(cfg), quem: "teste"}, &metodos
}

func TestTomarTrabalhoPegaAPrimeiraLinhaLivre(t *testing.T) {
	l, metodos := filaDeMentira(t, nil)
	trabalho, err := l.tomarTrabalho(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if trabalho == nil {
		t.Fatal("havia trabalho na fila e ele voltou vazio — foi este o defeito de 26/08")
	}
	if trabalho.ID != "aaa" {
		t.Errorf("pegou %q, esperava a primeira da fila", trabalho.ID)
	}
	// A tentativa é contada no ato de tomar, não depois: nota que derruba o
	// processo no meio não pode ficar valendo zero tentativas para sempre.
	if trabalho.Tentativas != 1 {
		t.Errorf("tentativas: %d, esperava 1", trabalho.Tentativas)
	}
	if len(*metodos) < 2 || !strings.HasPrefix((*metodos)[1], "PATCH") {
		t.Errorf("a tomada não foi um PATCH: %v", *metodos)
	}
}

// Quem perde a corrida não trava: vai para a próxima linha.
func TestQuemPerdeACorridaPegaAProxima(t *testing.T) {
	l, _ := filaDeMentira(t, map[string]bool{"aaa": true})
	trabalho, err := l.tomarTrabalho(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if trabalho == nil {
		t.Fatal("a segunda linha estava livre e ele desistiu")
	}
	if trabalho.ID != "bbb" {
		t.Errorf("pegou %q, esperava a segunda", trabalho.ID)
	}
}

// Fila realmente vazia devolve nil sem erro — é o que encerra a rodada.
func TestFilaVaziaDevolveNada(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	cfg := &config.Config{Supabase: config.Supabase{URL: srv.URL, ChaveServico: "servico"}}
	l := &Leitor{bd: banco.Novo(cfg), quem: "teste"}
	trabalho, err := l.tomarTrabalho(t.Context())
	if err != nil || trabalho != nil {
		t.Errorf("fila vazia: trabalho=%v erro=%v", trabalho, err)
	}
}
