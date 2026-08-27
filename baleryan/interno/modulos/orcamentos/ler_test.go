// rev 1 — a leitura na hora
//
// O QUE ESTE ARQUIVO GUARDA
//
//	Duas garantias, e as duas são sobre LER UMA VEZ SÓ:
//	  · a nota já lida (ou já usada) não volta para a leitura;
//	  · o trabalho da fila é TOMADO, não só olhado — se o robô do GitHub já está
//	    com ele na mão, o botão recusa em vez de ler por cima.
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
)

// NOTA LIDA NÃO SE LÊ DE NOVO
//
//	Não é economia de cota: é não apagar o que alguém corrigiu à mão em cima da
//	leitura anterior. E `usado` é pior ainda — o orçamento já existe, e mexer
//	nos itens por baixo faria a conta dele discordar da nota.
func TestNotaJaLidaNaoVolta(t *testing.T) {
	for _, caso := range []struct {
		status string
		pode   bool
	}{
		{"inserido", true},
		{"falhou", true}, // a leitura NÃO aconteceu; tentar de novo é o caminho
		{"lido", false},
		{"usado", false},
		{"", false}, // status que ninguém previu não vira permissão
	} {
		recusa, pode := podeLerAgora(caso.status)
		if pode != caso.pode {
			t.Errorf("status %q: pode=%v, esperava %v", caso.status, pode, caso.pode)
		}
		if !pode && recusa == "" {
			t.Errorf("status %q recusou sem dizer por quê", caso.status)
		}
		if pode && recusa != "" {
			t.Errorf("status %q liberou e ainda assim reclamou: %q", caso.status, recusa)
		}
	}
}

// filaDeMentira responde ao PATCH condicional com `tomados`, e ao GET de
// trabalho `rodando` com `rodando`. Guarda os filtros para a conferência.
func filaDeMentira(t *testing.T, tomados, rodando []map[string]any, filtros *[]string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		*filtros = append(*filtros, r.Method+" "+r.URL.RawQuery)
		switch r.Method {
		case http.MethodPatch:
			_ = json.NewEncoder(w).Encode(tomados)
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(rodando)
		default:
			t.Errorf("tomar trabalho usou %s", r.Method)
		}
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// A DISPUTA É RESOLVIDA PELO BANCO, NÃO POR UMA OLHADA ANTES
//
//	Ler o trabalho, ver que está `na_fila` e depois marcá-lo é uma corrida: o
//	robô pode marcar entre as duas coisas. Por isso `status=eq.na_fila` vai no
//	FILTRO do PATCH — quem não receber linha de volta perdeu, e sabe disso.
func TestTomarALeituraExigeQueOTrabalhoEstejaNaFila(t *testing.T) {
	var filtros []string
	m := filaDeMentira(t, []map[string]any{{"id": "j1"}}, nil, &filtros)

	id, livre := m.tomarALeitura(context.Background(), "d1")
	if !livre || id != "j1" {
		t.Fatalf("tomou %q livre=%v, esperava j1/true", id, livre)
	}
	if len(filtros) == 0 || !strings.Contains(filtros[0], "status=eq.na_fila") {
		t.Fatalf("o PATCH não exigiu na_fila no filtro: %v", filtros)
	}
	if !strings.Contains(filtros[0], "alvo_id=eq.d1") || !strings.Contains(filtros[0], "tipo=eq.ler_documento") {
		t.Errorf("o PATCH não foi mirado neste trabalho: %v", filtros)
	}
}

// SE O ROBÔ ESTÁ COM O TRABALHO, O BOTÃO RECUSA
//
//	Ler por cima seria a segunda chamada de IA paga na mesma nota, e a última a
//	gravar decidiria o que a nota diz.
func TestTrabalhoNaMaoDeOutroRecusaALeitura(t *testing.T) {
	var filtros []string
	m := filaDeMentira(t, []map[string]any{}, []map[string]any{{"id": "j1"}}, &filtros)

	if _, livre := m.tomarALeitura(context.Background(), "d1"); livre {
		t.Fatal("o trabalho estava rodando em outra máquina e a leitura foi liberada")
	}
}

// NOTA SEM TRABALHO NENHUM AINDA PODE SER LIDA
//
//	Nota inserida antes de a fila existir, ou com o trabalho fechado por uma
//	corrida que não gravou o status, não tem trabalho para tomar. Recusar aqui
//	deixaria a nota presa para sempre — sem robô que a leia e sem botão que a
//	destranque.
func TestNotaSemTrabalhoNaFilaAindaPodeSerLida(t *testing.T) {
	var filtros []string
	m := filaDeMentira(t, []map[string]any{}, []map[string]any{}, &filtros)

	id, livre := m.tomarALeitura(context.Background(), "d1")
	if !livre {
		t.Fatal("nota sem trabalho na fila ficou presa")
	}
	if id != "" {
		t.Errorf("inventou o trabalho %q para fechar depois", id)
	}
}

// A ROTA LITERAL NÃO PODE SER COMIDA PELO CURINGA
//
//	`/orcamentos/documentos/{id}` e `/orcamentos/documentos/porler` disputam o
//	mesmo lugar no caminho. O ServeMux do Go dá preferência ao literal, e é por
//	isso que as duas convivem — mas essa é uma regra que uma renomeação futura
//	pode quebrar em silêncio. O preço seria o botão "Ler notas" respondendo
//	"Endereço inválido" (a nota `porler` não é um UUID) sem ninguém entender por
//	quê.
func TestPorlerNaoCaiNoCuringaDoId(t *testing.T) {
	mux := http.NewServeMux()
	(&Modulo{}).Montar(mux)

	_, padrao := mux.Handler(httptest.NewRequest(http.MethodGet, "/orcamentos/documentos/porler", nil))
	if !strings.Contains(padrao, "porler") {
		t.Fatalf("quem atendeu /documentos/porler foi %q", padrao)
	}
}
