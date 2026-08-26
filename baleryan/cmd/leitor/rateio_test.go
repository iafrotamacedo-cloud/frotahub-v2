// rev 1 — a fila de rateio não deixa o robô escolher o ticket
//
// O DEFEITO QUE ESTE ARQUIVO GUARDA
//
//	O leitor rodava a mesma regra de ticket nas duas filas. Numa nota de
//	rateio — que existe porque o material se divide entre vários chamados —
//	amarrar o ticket que estava escrito na observação joga o custo inteiro numa
//	loja só, cancela o rateio antes de alguém abrir a tela, e não trava nada:
//	gera, lança e cobra errado em silêncio.
//
//	Em 26/08/2026 as duas notas de rateio da fila escaparam por sorte (a IA
//	estava fora do ar e a observação não foi lida). Este teste troca a sorte
//	por uma regra.
package main

import (
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
)

// leituraComTicket é o pior caso possível para a fila de rateio: uma leitura
// PERFEITA, com o ticket no campo certo. Se a trava dependesse de a leitura
// ser ruim, ela não seria trava.
func leituraComTicket() *leitor.Leitura {
	return &leitor.Leitura{
		Tipo:              "dav",
		DAV:               "19702",
		Observacao:        "TICKET 130773",
		ObservacaoDoCampo: true,
	}
}

func TestRateioNuncaAmarraSozinho(t *testing.T) {
	ticket, pode, porque := amarraSozinho(FilaRateio, leituraComTicket())
	if pode {
		t.Fatalf("a nota é de rateio e o robô amarrou o ticket %d sozinho — "+
			"o custo inteiro acabou de cair numa loja só", ticket)
	}
	if porque == "" {
		t.Error("recusou sem dizer por quê; é isso que o log mostra ao usuário")
	}
}

func TestOrcamentoAmarraQuandoOTicketEConfiavel(t *testing.T) {
	ticket, pode, porque := amarraSozinho("orcamento", leituraComTicket())
	if !pode {
		t.Fatalf("a nota é da fila comum, o ticket veio do campo de observação "+
			"e mesmo assim não amarrou: %s", porque)
	}
	if ticket != 130773 {
		t.Errorf("amarrou o ticket %d, e o que estava escrito era 130773", ticket)
	}
}

// A trava do rateio NÃO substitui a regra dos três critérios: ela se soma.
func TestOrcamentoContinuaRecusandoTicketDuvidoso(t *testing.T) {
	// Observação lida da página inteira, e não do campo: procedência ruim.
	fora := leituraComTicket()
	fora.ObservacaoDoCampo = false

	if _, pode, _ := amarraSozinho("orcamento", fora); pode {
		t.Error("o número não saiu do campo de observação e mesmo assim virou ticket")
	}
}

// Fila vazia é fila comum. O robô lê documento antigo, gravado antes de a
// coluna existir, e não pode travar por causa disso.
func TestFilaVaziaSeComportaComoOrcamento(t *testing.T) {
	if _, pode, porque := amarraSozinho("", leituraComTicket()); !pode {
		t.Errorf("documento sem fila parou de amarrar ticket: %s", porque)
	}
}
