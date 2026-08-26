// rev 1 — a repetida não entra na geração
//
// POR QUE ESTE TESTE OLHA O FILTRO, E NÃO O RESULTADO
//
//	A proteção contra gerar duas vezes a mesma nota mora numa condição da
//	consulta. Uma condição que ninguém confere é uma condição que some numa
//	edição distraída — e o sintoma, meses depois, é a loja pagando o mesmo
//	material duas vezes, sem nada apitar.
//
//	Então o teste levanta um PostgREST de mentira e lê o que foi perguntado a
//	ele. É a mesma disciplina do `fila_test.go`: quando o que protege é o
//	pedido, o teste tem que olhar o pedido.
package orcamentos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

func moduloQueEscuta(t *testing.T, guardar *string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*guardar = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

func TestAGeracaoNuncaPegaNotaRepetida(t *testing.T) {
	var perguntou string
	m := moduloQueEscuta(t, &perguntou)

	if _, err := m.documentosProntos(context.Background(),
		"11111111-1111-1111-1111-111111111111", nil, "orcamento"); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if !strings.Contains(perguntou, "duplicada_de=is.null") {
		t.Errorf("a geração pediu notas sem excluir as repetidas:\n%s", perguntou)
	}
}

// A TRAVA VALE TAMBÉM PARA O "GERAR ESTAS AQUI"
//
//	O caminho `apenas` é o da tela, quando o usuário escolhe as notas na mão.
//	Trava que o usuário desliga clicando não é trava, é aviso.
func TestEscolherNaMaoNaoFuraATrava(t *testing.T) {
	var perguntou string
	m := moduloQueEscuta(t, &perguntou)

	if _, err := m.documentosProntos(context.Background(),
		"11111111-1111-1111-1111-111111111111",
		[]string{"22222222-2222-2222-2222-222222222222"}, "orcamento"); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if !strings.Contains(perguntou, "duplicada_de=is.null") {
		t.Errorf("escolher a nota na mão pulou a trava de duplicidade:\n%s", perguntou)
	}
}
