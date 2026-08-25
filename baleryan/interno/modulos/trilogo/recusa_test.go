// rev 1 — os testes das recusas do Trílogo
//
// CADA UM DESTES RECOLOCA UM DEFEITO QUE JÁ ACONTECEU DE VERDADE (P-30)
//
//	As duas frases usadas aqui não foram inventadas: são as respostas literais que
//	o Trílogo devolveu em 25/08/2026, testando contra o sistema do cliente. Se
//	alguém "simplificar" o tratamento de erro, estes testes quebram com a mesma
//	mensagem que o servidor de verdade deu.
package trilogo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A frase real do 400 quando falta o anexo. Descoberta ao tentar criar custo com
// DocumentNumber e sem UploadedInvoiceFiles.
const frasesemNota = `{"message":"Necessário anexar a nota fiscal referente ao número informado. "}`

// A frase real do 400 por status. Repare no que ela NÃO diz: o ticket 126211
// estava ARQUIVADO e foi recusado, mas a mensagem só cita Aberto e Em execução.
const fraseStatus = `{"message":"Está ação não é permitida para tickets com o status 'Aberto' ou 'Em execução'"}`

func sessaoContra(t *testing.T, h http.HandlerFunc) (*Sessao, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	velha := base
	base = srv.URL
	return &Sessao{token: "t", http: srv.Client(), Conta: "teste"},
		func() { base = velha; srv.Close() }
}

// O DEFEITO: a frase deles era descartada e o erro virava "devolveu 400".
//
// Duas causas totalmente diferentes chegavam à tela com o mesmo texto, e nenhuma
// das duas dizia o que fazer.
func TestARecusaCarregaAFraseDoTrilogo(t *testing.T) {
	for _, caso := range []struct{ nome, corpo, esperado string }{
		{"falta a nota", frasesemNota, "Necessário anexar a nota fiscal"},
		{"status trava", fraseStatus, "não é permitida para tickets"},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			s, fechar := sessaoContra(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(caso.corpo))
			})
			defer fechar()

			err := s.pedir(context.Background(), http.MethodPost, "/api/Ticket/CreateTicketCost", []byte("{}"), nil)
			if err == nil {
				t.Fatal("400 passou como sucesso")
			}
			e, ok := Recusa(err)
			if !ok {
				t.Fatalf("o erro não é uma recusa do Trílogo: %v", err)
			}
			if e.Codigo != http.StatusBadRequest {
				t.Errorf("código: %d, esperava 400", e.Codigo)
			}
			if !strings.Contains(e.Corpo, caso.esperado) {
				t.Errorf("a frase se perdeu.\n  guardado: %q\n  esperava conter: %q", e.Corpo, caso.esperado)
			}
			// E ela também tem que aparecer no texto do erro, para quem só lê o log.
			if !strings.Contains(err.Error(), caso.esperado) {
				t.Errorf("a frase não chegou ao Error(): %q", err.Error())
			}
		})
	}
}

// O DEFEITO ORIGINAL: a conferência de corpo vazio vinha ANTES da de código, e
// devolvia nil. Um 4xx sem corpo passava como sucesso — o irmão gêmeo da
// armadilha do chamado de outra conta.
func TestErroSemCorpoNaoPassaComoSucesso(t *testing.T) {
	for _, codigo := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusInternalServerError} {
		s, fechar := sessaoContra(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(codigo)
		})
		err := s.pedir(context.Background(), http.MethodGet, "/api/Ticket/GetTicketDetail", nil, nil)
		fechar()
		if err == nil {
			t.Fatalf("%d com corpo vazio passou como sucesso", codigo)
		}
		if e, ok := Recusa(err); !ok || e.Codigo != codigo {
			t.Errorf("%d: não virou recusa com o código certo: %v", codigo, err)
		}
	}
}

// O 204 continua sendo resposta legítima e vazia — não pode virar erro.
func TestVazioComSucessoContinuaSendoSucesso(t *testing.T) {
	for _, codigo := range []int{http.StatusOK, http.StatusNoContent} {
		s, fechar := sessaoContra(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(codigo)
		})
		err := s.pedir(context.Background(), http.MethodGet, "/api/Ticket/GetTicketCosts/", nil, nil)
		fechar()
		if err != nil {
			t.Errorf("%d vazio virou erro: %v", codigo, err)
		}
	}
}

// A frase pode vir sem JSON. Melhor o texto cru do que nada — mas cortado, porque
// uma página de erro de 40 kB dentro de um log não ajuda ninguém.
func TestFraseCruaQuandoNaoEJson(t *testing.T) {
	longa := strings.Repeat("x", 1000)
	s, fechar := sessaoContra(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(longa))
	})
	defer fechar()
	err := s.pedir(context.Background(), http.MethodGet, "/api/Ticket/GetTicketDetail", nil, nil)
	e, ok := Recusa(err)
	if !ok {
		t.Fatalf("não virou recusa: %v", err)
	}
	if len(e.Corpo) > 420 {
		t.Errorf("a frase crua não foi cortada: %d caracteres", len(e.Corpo))
	}
	if !strings.HasPrefix(e.Corpo, "xxx") {
		t.Errorf("perdeu o texto cru: %q", e.Corpo)
	}
}
