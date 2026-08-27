// rev 1 — o ticket colado na palavra
//
//	"TICKET115716-SANTOS DUMONT-AMAURI" foi para SEM TICKET na leitura de
//	26/08/2026. O número está ali, legível, e o padrão era `\b(\d{5,6})\b` — só
//	que entre o "T" e o "1" não existe fronteira de palavra: as duas são letra
//	para o regex.
//
//	Tirar o `\b` da frente é seguro porque as duas defesas que importam continuam
//	de pé, e são melhores: a lista de palavras enganosas (DAV, NF, CEP, pedido) e
//	a checagem de dígito adjacente. Este arquivo prova as duas.
package leitor

import (
	"reflect"
	"testing"
)

func TestOsJeitosDeEscreverOTicketColado(t *testing.T) {
	casos := map[string][]int{
		"TICKET115716-SANTOS DUMONT-AMAURI":  {115716}, // o caso que falhou
		"TICKET-131372-ANTONIO SALES-AMAURI": {131372},
		"TICKET 115716-SANTOS DUMONT":        {115716},
		"131231 -VALECIO-MIGUEL DIAS":        {131231},
		"ticket115716":                       {115716},
	}
	for texto, esperado := range casos {
		if deu := Tickets(texto); !reflect.DeepEqual(deu, esperado) {
			t.Errorf("%q deu %v, esperava %v", texto, deu, esperado)
		}
	}
}

// AS DEFESAS QUE NÃO PODEM CAIR JUNTO
//
//	Soltar o `\b` da frente não pode abrir a porta para o que nunca foi ticket.
//	Um ticket inventado vira orçamento lançado no chamado errado — pior do que
//	não achar nenhum.
func TestSoltarOLimiteNaoDeixaEntrarOQueNaoEhTicket(t *testing.T) {
	for _, texto := range []string{
		"DAV: 92080",
		"DAV92080", // colado também não passa
		"NOSSO NUMERO: 45945",
		"NF 18452",
		"PEDIDO 123456",
		"Endereco CEP 60455-100",
		"chave 23260714788633000110550010000091601000055288",
	} {
		if deu := Tickets(texto); len(deu) > 0 {
			t.Errorf("%q inventou o ticket %v", texto, deu)
		}
	}
}
