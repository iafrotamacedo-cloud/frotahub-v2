// rev 1 — o trabalho que ficou na mão de uma máquina morta
//
// O BURACO QUE ISTO TAPA
//
//	`tomarTrabalho` marca o trabalho como "rodando" e só o fecha no fim. Se a
//	corrida morre no meio — o Actions cai, o tempo estoura, alguém cancela — o
//	trabalho fica "rodando" para sempre, e nenhuma corrida futura o pega, porque
//	todas procuram por `na_fila`.
//
//	Aconteceu em 26/08/2026. O documento não some, não dá erro e não aparece
//	como falhado: ele simplesmente nunca mais é lido. É a falha mais difícil de
//	notar que existe — a que não deixa rastro em lugar nenhum.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

type chamada struct {
	metodo string
	alvo   string
	campos map[string]any
}

func leitorQueEscuta(t *testing.T, anotar *[]chamada) *Leitor {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := chamada{metodo: r.Method, alvo: r.URL.Path + "?" + r.URL.RawQuery}
		if r.Method == http.MethodPatch {
			_ = json.NewDecoder(r.Body).Decode(&c.campos)
		}
		*anotar = append(*anotar, c)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"j1","cliente_id":"c1","alvo_id":"d1","tentativas":1}]`))
	}))
	t.Cleanup(s.Close)
	return &Leitor{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	}), quem: "teste"}
}

func TestRecolheOTrabalhoPresoEmUmaCorridaMorta(t *testing.T) {
	var vistas []chamada
	l := leitorQueEscuta(t, &vistas)
	l.recolherOrfaos(context.Background())

	if len(vistas) != 1 {
		t.Fatalf("fez %d chamadas, esperava 1", len(vistas))
	}
	c := vistas[0]

	// SÓ ALTERAÇÃO CONDICIONAL SERVE
	//   Um PATCH sem filtro devolveria para a fila o trabalho que a máquina AO
	//   LADO está rodando neste segundo — e as duas leriam a mesma nota.
	if c.metodo != http.MethodPatch {
		t.Errorf("recolheu com %s; só a alteração condicional (PATCH) serve", c.metodo)
	}
	if !strings.Contains(c.alvo, "status=eq.rodando") {
		t.Errorf("não filtrou por status=rodando: %s", c.alvo)
	}
	if !strings.Contains(c.alvo, "comecou_em=lt.") {
		t.Errorf("não filtrou por tempo — isso devolveria para a fila o trabalho "+
			"que outra máquina está rodando AGORA: %s", c.alvo)
	}
	if !strings.Contains(c.alvo, "tipo=eq.ler_documento") {
		t.Errorf("não filtrou pelo tipo; recolheria trabalho de outro robô: %s", c.alvo)
	}

	if c.campos["status"] != "na_fila" {
		t.Errorf("devolveu com status %v, esperava na_fila", c.campos["status"])
	}
	// NÃO É FALHA, É INTERRUPÇÃO
	//   A nota está inteira e a leitura nunca chegou ao fim. Marcar "falhou"
	//   seria acusar a nota de um problema que foi da máquina.
	if c.campos["status"] == "falhou" {
		t.Error("marcou a nota como falhada; quem morreu foi a máquina")
	}
	// O dono anterior tem que sair, senão a linha continua contando a história
	// de uma máquina que não existe mais.
	if v, tem := c.campos["tomado_por"]; !tem || v != nil {
		t.Errorf("tomado_por ficou %v; tinha que ser nulo", v)
	}
}

// A tentativa NÃO volta: um trabalho que mata a máquina toda vez precisa parar
// de ser tentado em algum momento, senão vira laço infinito.
func TestRecolherNaoDevolveATentativaGasta(t *testing.T) {
	var vistas []chamada
	l := leitorQueEscuta(t, &vistas)
	l.recolherOrfaos(context.Background())

	if _, mexeu := vistas[0].campos["tentativas"]; mexeu {
		t.Error("mexeu no contador de tentativas ao recolher — uma nota que derruba " +
			"a máquina toda vez seria tentada para sempre")
	}
}
