// rev 1 — o rateio, ligado
//
// A regra pura tem teste próprio em `regras/rateio_test.go`. Este arquivo prova
// a LIGAÇÃO: que o preço que sai no documento do cliente é o da nota, que a
// nota para quando não tem o que dar a todo mundo, e que a tela do desconto e a
// geração repartem a nota do MESMO jeito.
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
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/parametros"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

// O ITEM QUE DEU NOME AO DEFEITO
//
//	NF 660575, da SV: 30 abraçadeiras a R$ 0,88, rateadas entre 14 tickets. O
//	sistema espremia os itens para caberem em 1/14 do valor, e o cliente recebia
//	catorze documentos dizendo "30 abraçadeiras a R$ 0,0753".
func TestOClienteVeOPrecoDaNota(t *testing.T) {
	m := moduloComTicketVazio(t)
	nota := []regras.LinhaDaNota{
		{Descricao: "ABRACADEIRA NYLON", Unidade: "UN",
			Quantidade: regras.QuantidadeDe(30), Unitario: regras.PrecoDe(0.88)},
		{Descricao: "CABO FLEXIVEL 2,5MM", Unidade: "M",
			Quantidade: regras.QuantidadeDe(300), Unitario: regras.PrecoDe(2.35)},
	}
	// R$ 26,40 + R$ 705,00 = R$ 731,40, sem desconto do fornecedor.
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "660575.pdf", Fila: "rateio", Valor: 731.40},
		tickets(14), nota, regras.Padrao)
	if err != nil || bloqueio != "" {
		t.Fatalf("a nota não rodou: erro=%v bloqueio=%q", err, bloqueio)
	}

	// Com 20% de margem: R$ 0,88 → R$ 1,06 (arredondado); R$ 2,35 → R$ 2,82.
	quer := map[string]regras.Preco{
		"ABRACADEIRA NYLON":   regras.PrecoDe(1.056),
		"CABO FLEXIVEL 2,5MM": regras.PrecoDe(2.82),
	}
	var abracadeiras, metros regras.Quantidade
	for _, p := range partes {
		for _, it := range p.itens {
			if it.UnitarioCobrado != quer[it.Descricao] {
				t.Fatalf("ticket %d — %s saiu a %d, o preço da nota com margem é %d",
					p.ticket.Ticket, it.Descricao, it.UnitarioCobrado, quer[it.Descricao])
			}
			switch it.Descricao {
			case "ABRACADEIRA NYLON":
				abracadeiras += it.Quantidade
			case "CABO FLEXIVEL 2,5MM":
				metros += it.Quantidade
			}
		}
	}
	// E o que foi comprado é o que foi cobrado — nem mais, nem menos.
	if abracadeiras != regras.QuantidadeDe(30) {
		t.Errorf("os catorze somam %d abraçadeiras; a nota traz 30", abracadeiras)
	}
	if metros != regras.QuantidadeDe(300) {
		t.Errorf("os catorze somam %d metros; a nota traz 300", metros)
	}
}

// NENHUM TICKET RECEBE A QUANTIDADE INTEIRA
//
//	Era o outro lado do defeito: cada um dos catorze documentos listava as 30
//	abraçadeiras, como se a loja tivesse recebido todas.
func TestNenhumTicketLevaANotaInteira(t *testing.T) {
	m := moduloComTicketVazio(t)
	nota := []regras.LinhaDaNota{
		{Descricao: "ABRACADEIRA NYLON", Unidade: "UN",
			Quantidade: regras.QuantidadeDe(30), Unitario: regras.PrecoDe(0.88)},
	}
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "660575.pdf", Fila: "rateio", Valor: 26.40},
		tickets(14), nota, regras.Padrao)
	if err != nil || bloqueio != "" {
		t.Fatalf("a nota não rodou: erro=%v bloqueio=%q", err, bloqueio)
	}
	for _, p := range partes {
		for _, it := range p.itens {
			if it.Quantidade >= regras.QuantidadeDe(30) {
				t.Fatalf("ticket %d levou %d das 30 abraçadeiras", p.ticket.Ticket, it.Quantidade)
			}
		}
	}
}

// UMA NOTA PODE NÃO TER O QUE DAR A TODO MUNDO
//
//	Um disjuntor para três tickets. Antes, o valor era dividido por igual e
//	todos recebiam um terço de um preço que não existe. Agora a nota para, com o
//	número do ticket, na tela onde se conserta a associação.
func TestNotaSemItemParaTodosParaComOMotivoCerto(t *testing.T) {
	m := moduloComTicketVazio(t)
	_, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "disjuntor.pdf", Fila: "rateio", Valor: 90},
		tickets(3), []regras.LinhaDaNota{umaLinha(90)}, regras.Padrao)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio == "" {
		t.Fatal("um disjuntor foi repartido entre três tickets")
	}
	if !strings.Contains(bloqueio, "ticket ") || !strings.Contains(bloqueio, "associação") {
		t.Errorf("a frase não diz qual ticket nem o que fazer: %q", bloqueio)
	}
}

// E VALE MESMO COM APROVAÇÃO PEDIDA
//
//	A aprovação leva o valor cheio ao cliente; ela não inventa material que a
//	nota não tem. Um orçamento sem item nenhum não é documento.
func TestNemComAprovacaoNasceOrcamentoVazio(t *testing.T) {
	m := moduloComTicketVazio(t)
	_, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "disjuntor.pdf", Fila: "rateio",
			Valor: 90, AprovacaoPedida: true},
		tickets(3), []regras.LinhaDaNota{umaLinha(90)}, regras.Padrao)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio == "" {
		t.Fatal("com aprovação pedida, dois tickets ganhariam orçamento sem itens")
	}
}

// ---------------------------------------------------------------------------
// a tela do desconto e a geração têm que concordar
// ---------------------------------------------------------------------------

// moduloRespondendo devolve um Modulo cujo banco de mentira responde por
// caminho. O que não estiver no mapa sai como lista vazia — que é o que as
// buscas deste teste esperam para custos e parâmetros.
func moduloRespondendo(t *testing.T, porCaminho map[string]string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		corpo := "[]"
		for pedaco, resposta := range porCaminho {
			if strings.Contains(r.URL.Path, pedaco) {
				corpo = resposta
				break
			}
		}
		_, _ = w.Write([]byte(corpo))
	}))
	t.Cleanup(s.Close)
	bd := banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})
	return &Modulo{bd: bd, param: parametros.Novo(bd)}
}

// O QUE ESTE TESTE IMPEDE
//
//	Até 31/08/2026 esta tela dividia o pago por IGUAL enquanto a geração já
//	repartia quantidade. Numa nota de itens desiguais as duas contas divergem: a
//	tela ofereceria um desconto que a geração continuaria recusando — com a
//	autorização já assinada e nada resolvido.
//
//	A nota abaixo é desigual de propósito: 2 tickets e um quadro que não se
//	divide. O primeiro leva R$ 610 de material, o segundo R$ 10. Pela divisão
//	antiga — R$ 310 para cada — a tela responderia "esta nota já cabe no teto,
//	não precisa de desconto", e a geração recusaria o primeiro ticket.
func TestATelaDoDescontoRepartAssimComoAGeracao(t *testing.T) {
	doc := "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	m := moduloRespondendo(t, map[string]string{
		"documentos": `[{"id":"` + doc + `","nome_arquivo":"n.pdf","fila":"rateio","valor_total":620}]`,
		"documento_tickets": `[{"ticket":130000,"chamado_id":"11111111-1111-1111-1111-111111111111"},
		                       {"ticket":130001,"chamado_id":"11111111-1111-1111-1111-111111111111"}]`,
		"documento_itens": `[{"ordem":1,"descricao":"QUADRO","unidade":"UN","quantidade":1,"valor_unitario":600,"valor_total":600},
		                     {"ordem":2,"descricao":"PARAFUSO","unidade":"UN","quantidade":2,"valor_unitario":10,"valor_total":20}]`,
	})
	p := &seguranca.Principal{ClienteID: "c1", Ativo: true}

	d, err := m.calcularDesconto(context.Background(), p, doc)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	// A prova: o mesmo módulo, a mesma nota, pela geração.
	lidos, err := m.itensDo(context.Background(), doc)
	if err != nil {
		t.Fatalf("erro lendo itens: %v", err)
	}
	docs, err := m.documentosProntos(context.Background(), "c1", []string{doc}, "")
	if err != nil || len(docs) == 0 {
		t.Fatalf("não achei a nota: %v", err)
	}
	partes, _, err := m.avaliarPartes(context.Background(), docs[0],
		[]ticketDoDocumento{{Ticket: 130000, ChamadoID: &doc}, {Ticket: 130001, ChamadoID: &doc}},
		lidos.Linhas, regras.Padrao)
	if err != nil {
		t.Fatalf("erro avaliando: %v", err)
	}

	// O pedaço que a tela chama de "pior" é o maior custo da geração.
	var maior regras.Dinheiro
	for _, parte := range partes {
		if parte.custo.Cheio > maior {
			maior = parte.custo.Cheio
		}
	}
	if maior != regras.DinheiroDe(610) {
		t.Fatalf("a geração deu %s ao pedaço grande; o quadro não se divide", maior.Reais())
	}
	if !d.Pode {
		t.Fatalf("a tela recusou o desconto que resolveria a nota: %q", d.Motivo)
	}
	// R$ 610 com 20% de margem = R$ 732 — e não R$ 372, que é o que sairia da
	// metade da nota.
	if regras.DinheiroDe(d.Original) != regras.DinheiroDe(732) {
		t.Fatalf("a tela partiu de %s; o pedaço grande com margem é R$ 732,00",
			regras.DinheiroDe(d.Original).Reais())
	}
	if regras.DinheiroDe(d.Final) != regras.DinheiroDe(600) {
		t.Fatalf("a tela fecharia em %s, e o teto é R$ 600,00",
			regras.DinheiroDe(d.Final).Reais())
	}
}

// Se NENHUM pedaço precisa de desconto, a resposta é "não precisa" — e não um
// desconto de zero por cento, que abriria um botão sem função.
func TestNotaQueJaCabeInteiraNaoOfereceDesconto(t *testing.T) {
	doc := "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	m := moduloRespondendo(t, map[string]string{
		"documentos": `[{"id":"` + doc + `","nome_arquivo":"n.pdf","fila":"rateio","valor_total":200}]`,
		"documento_tickets": `[{"ticket":130000,"chamado_id":"11111111-1111-1111-1111-111111111111"},
		                       {"ticket":130001,"chamado_id":"11111111-1111-1111-1111-111111111111"}]`,
		"documento_itens": `[{"ordem":1,"descricao":"PARAFUSO","unidade":"UN","quantidade":2,"valor_unitario":100,"valor_total":200}]`,
	})
	d, err := m.calcularDesconto(context.Background(),
		&seguranca.Principal{ClienteID: "c1", Ativo: true}, doc)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if d.Pode {
		t.Fatalf("ofereceu desconto a uma nota que já cabe: %+v", d)
	}
	if !strings.Contains(d.Motivo, "já cabe") {
		t.Errorf("a frase não explica que não precisa: %q", d.Motivo)
	}
}

// A tela também precisa parar quando a nota não tem itens legíveis — e parar
// como MOTIVO, não como erro de servidor.
func TestATelaDoDescontoParaSemItens(t *testing.T) {
	const doc = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	m := moduloRespondendo(t, map[string]string{
		"documentos":        `[{"id":"` + doc + `","nome_arquivo":"n.pdf","fila":"rateio","valor_total":800}]`,
		"documento_tickets": `[{"ticket":130000,"chamado_id":"11111111-1111-1111-1111-111111111111"}]`,
		"documento_itens":   `[]`,
	})
	d, err := m.calcularDesconto(context.Background(),
		&seguranca.Principal{ClienteID: "c1", Ativo: true}, doc)
	if err != nil {
		t.Fatalf("devolveu erro em vez de motivo: %v", err)
	}
	if d.Pode || d.Motivo == "" {
		t.Fatalf("nota sem itens passou: %+v", d)
	}
}

// Item que não fecha é coisa para a pessoa consertar. A tela do desconto tem
// que dizer isso, e não oferecer um desconto calculado sobre número inventado.
func TestATelaDoDescontoMostraOBloqueioDoItem(t *testing.T) {
	const doc = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	m := moduloRespondendo(t, map[string]string{
		"documentos":        `[{"id":"` + doc + `","nome_arquivo":"n.pdf","fila":"rateio","valor_total":800}]`,
		"documento_tickets": `[{"ticket":130000,"chamado_id":"11111111-1111-1111-1111-111111111111"}]`,
		// 3 × R$ 0,00 não dá os R$ 700 da linha, e não há unitário que feche.
		"documento_itens": `[{"ordem":1,"descricao":"QUADRO","unidade":"UN","quantidade":0,"valor_unitario":0,"valor_total":700}]`,
	})
	d, err := m.calcularDesconto(context.Background(),
		&seguranca.Principal{ClienteID: "c1", Ativo: true}, doc)
	if err != nil {
		t.Fatalf("devolveu erro em vez de motivo: %v", err)
	}
	if d.Pode || !strings.Contains(d.Motivo, "não fecham") {
		t.Fatalf("o bloqueio do item não chegou à tela: %+v", d)
	}
}

// A tela responde JSON, e o navegador lê `orcamento_original`. Um campo
// renomeado sem querer deixa a tela mostrando "R$ 0,00" sem erro nenhum.
func TestOJSONDoDescontoMantemOsNomes(t *testing.T) {
	b, err := json.Marshal(&descontoNaTela{Pode: true, BP: 600, Original: 840, Final: 500})
	if err != nil {
		t.Fatal(err)
	}
	for _, campo := range []string{`"pode"`, `"bp"`, `"orcamento_original"`, `"orcamento_final"`} {
		if !strings.Contains(string(b), campo) {
			t.Errorf("sumiu o campo %s de %s", campo, b)
		}
	}
}
