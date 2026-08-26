// rev 1 — a nota que já estava aqui
//
// O DEFEITO QUE ESTE ARQUIVO GUARDA
//
//	`regras.MesmaNota` existia desde o começo do sistema, com teste, e NUNCA foi
//	chamada de lugar nenhum. O que protegia era só o sha256 do arquivo — que
//	cobre subir o mesmo arquivo duas vezes, e não cobre a mesma NOTA chegando
//	como arquivo diferente: a foto e o PDF dela, dois escaneamentos, o mesmo PDF
//	renomeado. Bytes diferentes, dois documentos, dois orçamentos, e a loja
//	pagando o mesmo material duas vezes.
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
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
)

// marcada é o que o PostgREST de mentira anotou: quem foi marcado como cópia,
// e de quem.
type marcada struct {
	quem string
	de   string
}

// bancoDeMentira responde às buscas com `achadas` e guarda os PATCH.
func bancoDeMentira(t *testing.T, achadas []map[string]any, anotar *[]marcada) *Leitor {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(achadas)
		case http.MethodPatch:
			var campos map[string]any
			_ = json.NewDecoder(r.Body).Decode(&campos)
			de, _ := campos["duplicada_de"].(string)
			id := strings.TrimPrefix(r.URL.Query().Get("id"), "eq.")
			*anotar = append(*anotar, marcada{quem: id, de: de})
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("a trava usou %s, e ela só lê e marca", r.Method)
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	t.Cleanup(s.Close)
	return &Leitor{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	}), quem: "teste"}
}

const (
	chaveA = "23260714788633000110550010000091601000055288"
	cedo   = "2026-08-26T10:00:00Z"
	tarde  = "2026-08-26T15:00:00Z"
)

// A QUE CHEGOU DEPOIS É A CÓPIA
func TestANotaQueChegouDepoisEAMarcada(t *testing.T) {
	var anotadas []marcada
	l := bancoDeMentira(t, []map[string]any{
		{"id": "a-original", "nome_arquivo": "NF.pdf", "chave_acesso": chaveA,
			"valor_total": 100.0, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "b-copia", Nome: "NF (1).pdf", Cliente: "c1", Inserido: tarde}
	if err := l.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ChaveAcesso: chaveA, ValorTotal: 100}); err != nil {
		t.Fatalf("falhou: %v", err)
	}

	if len(anotadas) != 1 {
		t.Fatalf("marcou %d notas, esperava 1", len(anotadas))
	}
	if anotadas[0].quem != "b-copia" || anotadas[0].de != "a-original" {
		t.Errorf("marcou %q como cópia de %q — a cópia é quem chegou depois",
			anotadas[0].quem, anotadas[0].de)
	}
}

// A ORDEM DE LEITURA NÃO É A ORDEM DE CHEGADA
//
//	Se as duas entram na mesma rodada e a segunda é lida primeiro, ela não acha
//	a primeira — que ainda não tem chave. Depois a primeira é lida e acha a
//	segunda. Se a regra fosse "marco a que estou lendo", a ORIGINAL viraria
//	cópia da cópia.
func TestLendoAOriginalDepois_AMarcaVaiNaOutra(t *testing.T) {
	var anotadas []marcada
	l := bancoDeMentira(t, []map[string]any{
		{"id": "b-copia", "nome_arquivo": "NF (1).pdf", "chave_acesso": chaveA,
			"valor_total": 100.0, "inserido_em": tarde},
	}, &anotadas)

	doc := &documento{ID: "a-original", Nome: "NF.pdf", Cliente: "c1", Inserido: cedo}
	if err := l.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ChaveAcesso: chaveA, ValorTotal: 100}); err != nil {
		t.Fatalf("falhou: %v", err)
	}

	if len(anotadas) != 1 {
		t.Fatalf("marcou %d notas, esperava 1", len(anotadas))
	}
	if anotadas[0].quem != "b-copia" {
		t.Errorf("marcou %q, e quem chegou depois foi b-copia — a original virou cópia da cópia",
			anotadas[0].quem)
	}
}

// SEM IDENTIDADE, NÃO SE ACUSA NINGUÉM
//
//	Duas notas de R$ 14,90 sem chave e sem número não são a mesma nota. Marcar
//	por valor igual criaria duplicidade onde não há, e o usuário perderia
//	confiança na marca — que é pior que não ter marca.
func TestNotaSemChaveESemNumeroNaoAcusa(t *testing.T) {
	var anotadas []marcada
	l := bancoDeMentira(t, []map[string]any{
		{"id": "outra", "nome_arquivo": "outra.jpg", "valor_total": 14.90, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "esta", Nome: "esta.jpg", Cliente: "c1", Inserido: tarde}
	if err := l.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ValorTotal: 14.90}); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if len(anotadas) != 0 {
		t.Errorf("acusou duplicidade sem chave e sem número: %+v", anotadas)
	}
}

// Notas diferentes com números parecidos não podem virar cópia uma da outra.
func TestNotasDiferentesNaoSaoMarcadas(t *testing.T) {
	var anotadas []marcada
	l := bancoDeMentira(t, []map[string]any{
		{"id": "outra", "nome_arquivo": "outra.pdf",
			"chave_acesso": "23260714788633000110550010000091601000055287",
			"valor_total":  100.0, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "esta", Nome: "esta.pdf", Cliente: "c1", Inserido: tarde}
	if err := l.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ChaveAcesso: chaveA, ValorTotal: 100}); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if len(anotadas) != 0 {
		t.Errorf("duas chaves diferentes viraram a mesma nota: %+v", anotadas)
	}
}

// O DAV NÃO TEM CHAVE, E MESMO ASSIM TEM IDENTIDADE
func TestDAVRepetidoEPegoPeloNumeroEValor(t *testing.T) {
	var anotadas []marcada
	numero := "19037"
	l := bancoDeMentira(t, []map[string]any{
		{"id": "a-original", "nome_arquivo": "dav.jpg", "numero": numero,
			"valor_total": 14.90, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "b-copia", Nome: "dav-foto.jpeg", Cliente: "c1", Inserido: tarde}
	if err := l.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{Numero: numero, ValorTotal: 14.90}); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if len(anotadas) != 1 || anotadas[0].quem != "b-copia" {
		t.Errorf("o mesmo DAV em dois arquivos não foi pego: %+v", anotadas)
	}
}
