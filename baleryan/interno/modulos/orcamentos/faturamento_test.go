// rev 1 — o agrupamento que vira nota fiscal
//
// ONDE UM ERRO AQUI VAI PARAR
//
//	Cada célula loja×conta vira UM PCO do cliente, e cada PCO vira UMA nota. Um
//	orçamento na célula errada é uma loja cobrando o que é de outra — e o
//	cliente confere loja por loja.
//
// Os casos são reais: são os 39 orçamentos de 31/07 que ficaram de fora da
// fatura de julho e rolaram para agosto.
package orcamentos

import "testing"

func celulaDe(unidade, loja, conta string, ticket int, valor float64, quando string) linhaAFaturar {
	return linhaAFaturar{
		Ticket: ticket, Parte: 1, UnidadeID: ptrS(unidade), Loja: ptrS(loja),
		Conta: ptrS(conta), Valor: ptrF(valor), CriadoEm: quando, Status: "lancado",
	}
}

const oliveira = "aaaaaaaa-0000-0000-0000-000000000002"
const santos = "aaaaaaaa-0000-0000-0000-000000000013"

func TestMesmaLojaComDuasContasViraDuasFaturas(t *testing.T) {
	// Oliveira Paiva, na virada de julho: dois orçamentos de civil e dois de
	// instalações. São DUAS notas, não uma de R$ 222,48.
	linhas := []linhaAFaturar{
		celulaDe(oliveira, "LOJA 02 - OLIVEIRA PAIVA", "civil", 125409, 45.48, "2026-07-31T10:00:00Z"),
		celulaDe(oliveira, "LOJA 02 - OLIVEIRA PAIVA", "civil", 125964, 53.76, "2026-07-31T10:01:00Z"),
		celulaDe(oliveira, "LOJA 02 - OLIVEIRA PAIVA", "instalacoes", 126423, 104.28, "2026-07-31T10:02:00Z"),
		celulaDe(oliveira, "LOJA 02 - OLIVEIRA PAIVA", "instalacoes", 126658, 18.96, "2026-07-31T10:03:00Z"),
	}
	celulas := agruparPorLojaEConta(linhas)
	if len(celulas) != 2 {
		t.Fatalf("virou %d células, tinha que virar 2", len(celulas))
	}
	if celulas[0].Conta != "civil" || !perto(celulas[0].Valor, 99.24) {
		t.Errorf("civil: %s R$ %.2f, esperava civil R$ 99,24", celulas[0].Conta, celulas[0].Valor)
	}
	if celulas[1].Conta != "instalacoes" || !perto(celulas[1].Valor, 123.24) {
		t.Errorf("instalações: %s R$ %.2f, esperava instalacoes R$ 123,24", celulas[1].Conta, celulas[1].Valor)
	}
	if celulas[0].Orcamentos != 2 || celulas[1].Orcamentos != 2 {
		t.Errorf("contagem por célula: %d e %d", celulas[0].Orcamentos, celulas[1].Orcamentos)
	}
}

func TestLojasDiferentesNaoSomamJuntas(t *testing.T) {
	linhas := []linhaAFaturar{
		celulaDe(santos, "LOJA 13 - SANTOS DUMONT", "instalacoes", 126433, 120.54, "2026-07-31T10:00:00Z"),
		celulaDe(santos, "LOJA 13 - SANTOS DUMONT", "instalacoes", 126454, 43.79, "2026-07-31T10:01:00Z"),
		celulaDe(santos, "LOJA 13 - SANTOS DUMONT", "instalacoes", 126763, 243.48, "2026-07-31T10:02:00Z"),
		celulaDe(oliveira, "LOJA 02 - OLIVEIRA PAIVA", "instalacoes", 126423, 104.28, "2026-07-31T10:03:00Z"),
	}
	celulas := agruparPorLojaEConta(linhas)
	if len(celulas) != 2 {
		t.Fatalf("virou %d células, esperava 2 lojas", len(celulas))
	}
	// Ordem alfabética por loja: Oliveira (02) antes de Santos Dumont (13).
	if celulas[0].Loja != "LOJA 02 - OLIVEIRA PAIVA" {
		t.Errorf("a ordem saiu errada: primeira é %q", celulas[0].Loja)
	}
	if !perto(celulas[0].Valor, 104.28) || !perto(celulas[1].Valor, 407.81) {
		t.Errorf("as somas vazaram entre lojas: %.2f e %.2f", celulas[0].Valor, celulas[1].Valor)
	}
	total := celulas[0].Valor + celulas[1].Valor
	if !perto(total, 512.09) {
		t.Errorf("o total mudou no agrupamento: %.2f", total)
	}
}

// Sem loja ou sem conta não existe PCO possível. O orçamento não pode entrar
// numa célula qualquer — nem sumir sem ninguém ver.
func TestSemLojaOuSemContaNaoEntraEmCelulaNenhuma(t *testing.T) {
	linhas := []linhaAFaturar{
		celulaDe(oliveira, "LOJA 02 - OLIVEIRA PAIVA", "civil", 125409, 45.48, "2026-07-31T10:00:00Z"),
		{Ticket: 130320, Parte: 1, Valor: ptrF(100), CriadoEm: "2026-08-01T10:00:00Z", Status: "gerado"},
		{Ticket: 125691, Parte: 1, UnidadeID: ptrS(oliveira), Loja: ptrS("LOJA 02 - OLIVEIRA PAIVA"),
			Valor: ptrF(50), CriadoEm: "2026-08-02T10:00:00Z", Status: "gerado"},
	}
	celulas := agruparPorLojaEConta(linhas)
	if len(celulas) != 1 || celulas[0].Orcamentos != 1 {
		t.Fatalf("células: %+v — os sem destino entraram numa", celulas)
	}
	valor, gerados, semDestino := somarAFaturar(linhas)
	if !perto(valor, 195.48) {
		t.Errorf("o total tem que contar TODOS: %.2f", valor)
	}
	if semDestino != 2 {
		t.Errorf("sem destino: %d, esperava 2", semDestino)
	}
	if gerados != 2 {
		t.Errorf("não lançados: %d, esperava 2", gerados)
	}
}

func TestValorNuloNaoDerrubaACelula(t *testing.T) {
	linhas := []linhaAFaturar{
		{Ticket: 1, Parte: 1, UnidadeID: ptrS(oliveira), Loja: ptrS("L"), Conta: ptrS("civil"),
			Valor: nil, CriadoEm: "2026-08-01T00:00:00Z", Status: "lancado"},
		celulaDe(oliveira, "L", "civil", 2, 50, "2026-08-02T00:00:00Z"),
	}
	c := agruparPorLojaEConta(linhas)[0]
	if !perto(c.Valor, 50) {
		t.Errorf("soma: %.2f, esperava 50", c.Valor)
	}
	if c.Orcamentos != 2 {
		t.Errorf("o de valor nulo sumiu da contagem: %d", c.Orcamentos)
	}
}

// A planilha de julho vem por data e, dentro do dia, por chamado. O cliente já
// conferiu uma vez nessa ordem.
func TestAOrdemDaPlanilhaEDataDepoisChamado(t *testing.T) {
	linhas := []linhaAFaturar{
		celulaDe(oliveira, "L", "civil", 126658, 18.96, "2026-08-05T09:00:00Z"),
		celulaDe(oliveira, "L", "civil", 125409, 45.48, "2026-07-31T10:00:00Z"),
		celulaDe(oliveira, "L", "civil", 125964, 53.76, "2026-07-31T09:00:00Z"),
	}
	ordenadas := ordenarParaOCliente(linhas)
	querem := []int{125964, 125409, 126658}
	for i, q := range querem {
		if ordenadas[i].Ticket != q {
			t.Fatalf("posição %d: %d, esperava %d", i, ordenadas[i].Ticket, q)
		}
	}
	// E a original não pode ter sido remexida: quem chamou ainda usa a dela.
	if linhas[0].Ticket != 126658 {
		t.Error("ordenarParaOCliente mexeu na fatia de quem chamou")
	}
}

func TestCompetenciaValida(t *testing.T) {
	boas := []string{"2026-08", "2026-01", "2026-12"}
	ruins := []string{"", "2026-8", "2026-13", "agosto", "2026-08-26", "26-08"}
	for _, s := range boas {
		if !competenciaValida(s) {
			t.Errorf("%q devia passar", s)
		}
	}
	for _, s := range ruins {
		if competenciaValida(s) {
			t.Errorf("%q NÃO devia passar", s)
		}
	}
}

// Campo ausente não é campo vazio: é o que deixa marcar o recebimento sem
// apagar o número da nota que já estava lá.
func TestAusenteNaoMexeVazioApaga(t *testing.T) {
	campos := map[string]any{}
	guardarTexto(campos, "pco_numero", nil)
	if _, tem := campos["pco_numero"]; tem {
		t.Error("campo ausente não podia ter entrado")
	}
	vazio := "   "
	guardarTexto(campos, "nf_numero", &vazio)
	if v, tem := campos["nf_numero"]; !tem || v != nil {
		t.Errorf("texto vazio tinha que virar nulo: %v", v)
	}
	data := "2026-08-26"
	guardarData(campos, "recebido_em", &data)
	if campos["recebido_em"] != "2026-08-26" {
		t.Errorf("data boa: %v", campos["recebido_em"])
	}
	torta := "26/08/2026"
	guardarData(campos, "nf_em", &torta)
	if _, tem := campos["nf_em"]; tem {
		t.Error("data em formato brasileiro não pode virar valor de coluna date")
	}
}
