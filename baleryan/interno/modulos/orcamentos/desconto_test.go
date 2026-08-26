// rev 1 — a regra do desconto, já ligada na geração
//
// A regra pura tem teste próprio em `regras/desconto_test.go`. Este arquivo
// prova a LIGAÇÃO: que o valor certo chega nela, que o resultado vira o valor
// do orçamento, e que a nota inteira para quando um pedaço não passa.
package orcamentos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

// moduloComTicketVazio responde que o ticket não tem custo nenhum, para o teto
// do ticket não se misturar com a regra que estamos medindo.
func moduloComTicketVazio(t *testing.T) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// umaLinha monta uma linha de nota que soma exatamente `reais`.
func umaLinha(reais float64) regras.LinhaDaNota {
	return regras.LinhaDaNota{
		Descricao: "material", Unidade: "UN",
		Quantidade: regras.QuantidadeDe(1),
		Unitario:   regras.PrecoDe(reais),
	}
}

func tickets(n int) []ticketDoDocumento {
	id := "11111111-1111-1111-1111-111111111111"
	saida := make([]ticketDoDocumento, 0, n)
	for i := 0; i < n; i++ {
		saida = append(saida, ticketDoDocumento{Ticket: 130000 + i, ChamadoID: &id})
	}
	return saida
}

// FAIXA 1 — nota pequena entra inteira, com margem e sem carimbo.
func TestNotaPequenaGeraOrcamentoNormal(t *testing.T) {
	m := moduloComTicketVazio(t)
	// R$ 450 sem desconto → R$ 540 com os 20%.
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "nota.pdf", Valor: 450},
		tickets(1), []regras.LinhaDaNota{umaLinha(450)}, regras.Padrao)

	if err != nil || bloqueio != "" {
		t.Fatalf("erro=%v bloqueio=%q", err, bloqueio)
	}
	if len(partes) != 1 {
		t.Fatalf("planejou %d partes, esperava 1", len(partes))
	}
	if partes[0].veredito.Valor != 54000 {
		t.Errorf("o orçamento saiu %s, esperava R$ 540,00", partes[0].veredito.Valor.Reais())
	}
	if partes[0].custo.Ajustada() {
		t.Error("carimbou de ajustada uma nota que não foi tocada")
	}
}

// FAIXA 2 — o bruto passa da linha, o pago cabe: fecha no teto, com carimbo.
func TestNotaComDescontoFechaNoTetoEGanhaCarimbo(t *testing.T) {
	m := moduloComTicketVazio(t)
	// O caso real de NF 9160: itens somam R$ 514,60, a nota diz R$ 463,14.
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "NF 9160.pdf", Valor: 463.14},
		tickets(1), []regras.LinhaDaNota{umaLinha(514.60)}, regras.Padrao)

	if err != nil || bloqueio != "" {
		t.Fatalf("erro=%v bloqueio=%q", err, bloqueio)
	}
	if !partes[0].custo.Ajustada() {
		t.Error("aparou a nota e não carimbou — o carimbo é o que explica o corte depois")
	}
	if partes[0].veredito.Valor != regras.Padrao.Teto {
		t.Errorf("o orçamento saiu %s, e o teto é %s",
			partes[0].veredito.Valor.Reais(), regras.Padrao.Teto.Reais())
	}
	// Os itens do documento têm que somar o mesmo que o orçamento: um PDF que
	// mostra R$ 514,60 em itens e R$ 600 no total é um documento que não fecha.
	var soma regras.Dinheiro
	for _, it := range partes[0].itens {
		soma += it.Total
	}
	if soma != partes[0].veredito.Valor {
		t.Errorf("os itens somam %s e o orçamento diz %s", soma.Reais(), partes[0].veredito.Valor.Reais())
	}
}

// FAIXA 3 — nem o pago cabe.
func TestNotaCaraBloqueiaANota(t *testing.T) {
	m := moduloComTicketVazio(t)
	_, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "cara.pdf", Valor: 520},
		tickets(1), []regras.LinhaDaNota{umaLinha(700)}, regras.Padrao)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio == "" {
		t.Fatal("pagou R$ 520 e passou; acima de R$ 500 não existe orçamento dentro do teto")
	}
	if !strings.Contains(bloqueio, "teto") {
		t.Errorf("a frase do bloqueio não fala do teto: %q", bloqueio)
	}
}

// A REGRA É POR ORÇAMENTO, NÃO POR NOTA
//
//	Uma nota de R$ 900 dividida entre três tickets são três compras de R$ 300,
//	e as três cabem. Se a linha dos R$ 500 fosse aplicada à nota inteira, esta
//	seria bloqueada — e não há razão para isso: nenhum ticket passa do teto.
func TestNotaGrandeRateadaEntreVariosTicketsPassa(t *testing.T) {
	m := moduloComTicketVazio(t)
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "rateio.pdf", Fila: "rateio", Valor: 900},
		tickets(3), []regras.LinhaDaNota{umaLinha(900)}, regras.Padrao)

	if err != nil || bloqueio != "" {
		t.Fatalf("uma nota de R$ 900 entre três tickets foi bloqueada: erro=%v bloqueio=%q", err, bloqueio)
	}
	if len(partes) != 3 {
		t.Fatalf("planejou %d partes, esperava 3", len(partes))
	}
	for i, p := range partes {
		if p.custo.Ajustada() {
			t.Errorf("parte %d ganhou carimbo sem precisar", i+1)
		}
		// R$ 300 de custo com 20% = R$ 360.
		if p.veredito.Valor != 36000 {
			t.Errorf("parte %d saiu %s, esperava R$ 360,00", i+1, p.veredito.Valor.Reais())
		}
	}
}

// SE UM PEDAÇO NÃO PASSA, A NOTA INTEIRA PARA
//
//	Gerar os que cabem deixaria a nota metade-lançada: o material do ticket que
//	sobrou já foi comprado e entregue, e não há onde cobrá-lo.
func TestUmPedacoQueNaoCabeParaANotaInteira(t *testing.T) {
	m := moduloComTicketVazio(t)
	// R$ 1.100 entre dois tickets = R$ 550 cada, sem desconto: cada pedaço
	// passa dos R$ 500 e nenhum orçamento cabe no teto.
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "rateio.pdf", Fila: "rateio", Valor: 1100},
		tickets(2), []regras.LinhaDaNota{umaLinha(1100)}, regras.Padrao)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio == "" {
		t.Fatal("um dos pedaços não cabia e a nota rodou assim mesmo")
	}
	if len(partes) != 0 {
		t.Errorf("bloqueou e ainda devolveu %d partes para gravar", len(partes))
	}
	if !strings.Contains(bloqueio, "ticket ") {
		t.Errorf("a frase não diz QUAL ticket travou: %q", bloqueio)
	}
}

// O TETO DO TICKET CONTINUA VALENDO POR CIMA DE TUDO
//
//	Decisão do dono: "caso o novo orçamento for romper o custo total do ticket,
//	não lance e coloque flag" — exceto pela folga dos 5%, que apara e roda.
func TestTicketQueJaEstaCheioBloqueiaANota(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// O ticket já tem R$ 500 de outro custo.
		_, _ = w.Write([]byte(`[{"valor":500.00}]`))
	}))
	t.Cleanup(s.Close)
	m := &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}

	// R$ 400 de nota viram R$ 480; com os R$ 500 que já estão lá dá R$ 980,
	// muito além do teto e da folga.
	_, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "nota.pdf", Valor: 400},
		tickets(1), []regras.LinhaDaNota{umaLinha(400)}, regras.Padrao)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio == "" {
		t.Fatal("o ticket já estava cheio e o orçamento entrou por cima")
	}
}

// O CASO QUE SEPARA A REGRA DO DESCONTO DA FOLGA DOS 5%
//
//	Uma nota de R$ 514,60 disfarça as duas: com margem dá R$ 617,52, que cabe na
//	folga e seria aparado para R$ 600 de qualquer jeito. Já R$ 700 de bruto dão
//	R$ 840 — muito além da folga — e sem a regra do desconto a nota iria para
//	aprovação, quando a decisão do dono foi outra: "arredonda para quinhentos e
//	processa o orçamento com 600,00".
//
//	Sem este teste, tirar a regra do código não quebrava nada visível.
func TestBrutoBemAcimaDoTetoComPagoQueCabeAindaGeraOrcamento(t *testing.T) {
	m := moduloComTicketVazio(t)
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "nota.pdf", Valor: 450},
		tickets(1), []regras.LinhaDaNota{umaLinha(700)}, regras.Padrao)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio != "" {
		t.Fatalf("bloqueou uma nota de R$ 700 comprada por R$ 450: %s\n"+
			"a decisão do dono foi arredondar para R$ 500 e gerar R$ 600", bloqueio)
	}
	if len(partes) != 1 {
		t.Fatalf("planejou %d partes, esperava 1", len(partes))
	}
	if partes[0].veredito.Valor != regras.Padrao.Teto {
		t.Errorf("o orçamento saiu %s, esperava o teto de %s",
			partes[0].veredito.Valor.Reais(), regras.Padrao.Teto.Reais())
	}
	if !partes[0].custo.Ajustada() {
		t.Error("cortou R$ 240 do orçamento e não carimbou")
	}
	// E o veredito do teto NÃO deve ter mordido: quem aparou foi a regra do
	// desconto, antes. Se os dois aparassem, o corte apareceria duas vezes no
	// aviso e ninguém entenderia de onde veio o número.
	if partes[0].veredito.Decisao != regras.Livre {
		t.Errorf("o teto decidiu %q depois de a regra do desconto já ter aparado",
			partes[0].veredito.Decisao)
	}
}
