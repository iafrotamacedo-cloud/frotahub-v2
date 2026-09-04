package servicos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

func clienteGroqDeMentira(t *testing.T, resposta func(w http.ResponseWriter, r *http.Request)) *clienteGroq {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(resposta))
	t.Cleanup(srv.Close)
	base := baseGroq
	baseGroq = srv.URL
	t.Cleanup(func() { baseGroq = base })
	return novoClienteGroq(config.Groq{Chave: "chave-de-teste", Modelo: "modelo-de-teste"})
}

// O CONTEÚDO É UM JSON DENTRO DE OUTRO JSON
//
// A resposta do Groq (padrão OpenAI) devolve o JSON que a gente pediu como
// TEXTO dentro de choices[0].message.content — não como objeto direto. É
// fácil montar o dublê errado (devolver o objeto solto) e o parser passar
// no teste sem nunca ter testado esse desembrulho.
func TestClassificarDesembrulhaOConteudo(t *testing.T) {
	g := clienteGroqDeMentira(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer chave-de-teste" {
			t.Errorf("Authorization = %q", got)
		}
		var corpo map[string]any
		json.NewDecoder(r.Body).Decode(&corpo)
		if corpo["model"] != "modelo-de-teste" {
			t.Errorf("model = %v", corpo["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{
					"content": `{"candidato": true, "motivo": "instalação de câmera nova"}`,
				}},
			},
		})
	})

	r, err := g.Classificar(context.Background(), "Solicito instalação de câmera nova no estacionamento.")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Candidato {
		t.Error("esperava candidato=true")
	}
	if r.Motivo != "instalação de câmera nova" {
		t.Errorf("motivo = %q", r.Motivo)
	}
}

func TestClassificarStatusDiferenteDe200EhErro(t *testing.T) {
	g := clienteGroqDeMentira(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"rate limit"}}`))
	})
	_, err := g.Classificar(context.Background(), "qualquer coisa")
	if err == nil {
		t.Fatal("esperava erro com 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("erro não menciona o status: %v", err)
	}
}

// O MODELO NÃO OBEDECEU O FORMATO PEDIDO
//
//	Já aconteceu com o Gemini (ver leitor/ia.go) o modelo devolver texto
//	solto em vez do JSON pedido. Aqui não pode travar a rodada — só devolver
//	erro, pra candidatos.go pular este chamado e seguir os outros.
func TestClassificarConteudoNaoJSONEhErro(t *testing.T) {
	g := clienteGroqDeMentira(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "desculpe, não sei responder isso"}},
			},
		})
	})
	if _, err := g.Classificar(context.Background(), "x"); err == nil {
		t.Fatal("esperava erro com conteúdo que não é JSON")
	}
}

func TestClienteGroqLigado(t *testing.T) {
	if (&clienteGroq{}).ligado() {
		t.Error("sem chave não deveria estar ligado")
	}
	if !(&clienteGroq{chave: "x"}).ligado() {
		t.Error("com chave deveria estar ligado")
	}
}
