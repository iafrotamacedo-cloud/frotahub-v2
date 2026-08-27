// rev 1 — o CORS conhece todos os métodos que o motor atende
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	Um método que o mux atende e o CORS não anuncia não dá erro em lugar nenhum:
//	o motor compila, sobe, responde no `curl`, e a tela do usuário diz "não
//	consegui falar com o servidor". O navegador recusa o preflight antes de o
//	pedido sair, e do lado do JavaScript isso é indistinguível de cabo
//	desligado.
//
//	Foi o que aconteceu com o `PUT` em 28/08/2026. Ele faltava desde o começo, e
//	a única rota `PUT` do sistema — a que grava a matriz de permissões — nunca
//	tinha sido chamada, porque o catálogo de rotinas ficou vazio até os módulos
//	de negócio existirem. Meses de silêncio, e o defeito apareceu no primeiro
//	clique em "Salvar permissões".
package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestOCORSAceitaTodosOsMetodosQueOMotorAtende(t *testing.T) {
	// O que o CORS anuncia.
	fonte, err := os.ReadFile("web.go")
	if err != nil {
		t.Fatalf("não consegui ler o web.go: %v", err)
	}
	m := regexp.MustCompile(`Allow-Methods",\s*"([^"]+)"`).FindStringSubmatch(string(fonte))
	if m == nil {
		t.Fatal("não achei o cabeçalho Access-Control-Allow-Methods")
	}
	anunciados := map[string]bool{}
	for _, x := range strings.Split(m[1], ",") {
		anunciados[strings.TrimSpace(x)] = true
	}

	// O que o motor de verdade atende, lido das rotas de todos os módulos.
	arquivos, err := filepath.Glob("../modulos/*/*.go")
	if err != nil || len(arquivos) == 0 {
		t.Skipf("não achei os módulos: %v", err)
	}
	rota := regexp.MustCompile(`HandleFunc\("(GET|POST|PUT|PATCH|DELETE) /`)
	usados := map[string]bool{}
	for _, a := range arquivos {
		if strings.HasSuffix(a, "_test.go") {
			continue
		}
		b, err := os.ReadFile(a)
		if err != nil {
			continue
		}
		for _, r := range rota.FindAllStringSubmatch(string(b), -1) {
			usados[r[1]] = true
		}
	}
	if len(usados) == 0 {
		t.Fatal("não achei rota nenhuma — o teste deixou de olhar o que devia")
	}

	for metodo := range usados {
		if !anunciados[metodo] {
			t.Errorf("o motor atende %s e o CORS não anuncia esse método — o navegador "+
				"recusa o preflight, e a tela mostra \"não consegui falar com o servidor\" "+
				"como se fosse falta de rede", metodo)
		}
	}

	// E o OPTIONS tem que estar lá, porque é ele que carrega a resposta do
	// preflight — nenhuma rota o registra, então nenhum laço acima o exigiria.
	if !anunciados["OPTIONS"] {
		t.Error("OPTIONS não está anunciado — é o método do próprio preflight")
	}
}
