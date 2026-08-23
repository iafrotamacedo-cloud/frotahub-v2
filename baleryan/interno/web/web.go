// rev 1 — as peças comuns de toda rota HTTP
//
// Respostas em JSON, erro traduzido para uma frase que o usuário entende, e CORS.
// Toda rota do motor passa por aqui — não existe rota escrevendo cabeçalho na mão.
package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// Responder devolve um JSON com o status pedido.
func Responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if corpo == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(corpo); err != nil {
		log.Printf("aviso: não consegui escrever a resposta: %v", err)
	}
}

// Falhar devolve um erro em JSON. A mensagem é escrita para ser LIDA por quem usa o
// sistema, não para o console: "sessão expirada", não "401 unauthorized".
func Falhar(w http.ResponseWriter, status int, mensagem string) {
	Responder(w, status, map[string]any{"erro": mensagem})
}

// CORS libera o front a conversar com o motor.
//
// Detalhe que já custou caro em sistema parecido: quando um erro escapa sem estes
// cabeçalhos, o navegador não mostra o erro de verdade — mostra "Failed to fetch",
// que não diz nada. Por isso o CORS entra ANTES de tudo e vale inclusive nas falhas.
func CORS(cfg *config.Config, proximo http.Handler) http.Handler {
	permitidas := map[string]bool{}
	livre := false
	for _, o := range cfg.Runtime.OrigensCORS {
		if o == "*" {
			livre = true
		}
		permitidas[strings.TrimRight(o, "/")] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origem := strings.TrimRight(r.Header.Get("Origin"), "/")
		switch {
		case livre:
			w.Header().Set("Access-Control-Allow-Origin", "*")
		case origem != "" && permitidas[origem]:
			w.Header().Set("Access-Control-Allow-Origin", origem)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Robot-Key, X-Pin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		proximo.ServeHTTP(w, r)
	})
}

// Recuperar impede que um erro inesperado derrube o motor inteiro.
func Recuperar(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("ERRO não tratado em %s %s: %v", r.Method, r.URL.Path, p)
				Falhar(w, http.StatusInternalServerError,
					"Alguma coisa deu errado do nosso lado. Tente de novo; se insistir, me avise.")
			}
		}()
		proximo.ServeHTTP(w, r)
	})
}

// Registrar escreve uma linha por requisição no console do servidor.
func Registrar(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		proximo.ServeHTTP(w, r)
	})
}
