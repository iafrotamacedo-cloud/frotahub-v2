// rev 1 — o agrupamento das pendências
//
// É AQUI QUE UM ERRO SAI DE CASA
//
//	A lista do cliente é enviada POR E-MAIL para o cliente. Somar errado, contar
//	um ticket duas vezes ou perder uma parte não é um defeito de tela: é uma
//	cobrança errada mandada para fora.
package orcamentos

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func ptrS(s string) *string   { return &s }
func ptrF(f float64) *float64 { return &f }
func ptrB(b bool) *bool       { return &b }

func TestTresPartesDoMesmoTicketViramUmaLinhaSo(t *testing.T) {
	// O caso real: o ticket 126574 tem TRÊS orçamentos parados. O cliente tem que
	// receber uma linha com a soma, não três linhas perguntando o que são partes.
	linhas := []linhaDePendencia{
		{Ticket: 126574, Parte: 1, Valor: ptrF(202.80), CriadoEm: "2026-08-19T17:29:34Z",
			Loja: ptrS("LOJA 37 - BORGES DE MELO"), Conta: ptrS("civil"), TicketStatus: ptrS("Aberto")},
		{Ticket: 126574, Parte: 2, Valor: ptrF(251.04), CriadoEm: "2026-08-19T17:31:33Z",
			Loja: ptrS("LOJA 37 - BORGES DE MELO"), Conta: ptrS("civil"), TicketStatus: ptrS("Aberto")},
		{Ticket: 126574, Parte: 3, Valor: ptrF(45.48), CriadoEm: "2026-08-25T17:53:52Z",
			Loja: ptrS("LOJA 37 - BORGES DE MELO"), Conta: ptrS("civil"), TicketStatus: ptrS("Aberto")},
	}
	saida := agruparPorTicket(linhas)

	if len(saida) != 1 {
		t.Fatalf("virou %d linhas, tinha que virar 1", len(saida))
	}
	got := saida[0]
	if got.Orcamentos != 3 {
		t.Errorf("orçamentos: %d, esperava 3", got.Orcamentos)
	}
	if quer := 499.32; !perto(got.Valor, quer) {
		t.Errorf("soma: %.2f, esperava %.2f", got.Valor, quer)
	}
	// "Desde" é o MAIS ANTIGO: é há quanto tempo o dinheiro está parado. Pegar o
	// mais recente faria uma pendência de seis dias parecer de hoje.
	if got.DesdeEm != "2026-08-19T17:29:34Z" {
		t.Errorf("desde: %q, esperava a data da parte 1", got.DesdeEm)
	}
	if len(got.Partes) != 3 {
		t.Errorf("partes: %v, esperava as três", got.Partes)
	}
}

func TestTicketsDiferentesNaoSeMisturam(t *testing.T) {
	linhas := []linhaDePendencia{
		{Ticket: 130656, Parte: 1, Valor: ptrF(587.88), CriadoEm: "2026-08-14T11:31:02Z"},
		{Ticket: 130656, Parte: 2, Valor: ptrF(118.44), CriadoEm: "2026-08-17T14:04:13Z"},
		{Ticket: 130238, Parte: 1, Valor: ptrF(1.68), CriadoEm: "2026-08-12T21:00:47Z"},
	}
	saida := agruparPorTicket(linhas)
	if len(saida) != 2 {
		t.Fatalf("virou %d linhas, esperava 2 tickets", len(saida))
	}
	// Maior valor primeiro: quem lê resolve de cima para baixo.
	if saida[0].Ticket != 130656 {
		t.Errorf("a ordem está errada: o primeiro é %d, esperava o de maior valor", saida[0].Ticket)
	}
	if !perto(saida[0].Valor, 706.32) || !perto(saida[1].Valor, 1.68) {
		t.Errorf("as somas vazaram entre tickets: %.2f e %.2f", saida[0].Valor, saida[1].Valor)
	}
	if soma := saida[0].Valor + saida[1].Valor; !perto(soma, 708.00) {
		t.Errorf("o total mudou no agrupamento: %.2f", soma)
	}
}

func TestOMotivoDaReaberturaChegaNaLinha(t *testing.T) {
	// A frase que o cliente escreveu ao reabrir é o que muda a conversa: em vez
	// de "reabriram", a lista diz "reabriram porque não foi resolvido".
	linhas := []linhaDePendencia{{
		Ticket: 131768, Parte: 1, Valor: ptrF(123.24), CriadoEm: "2026-08-25T17:55:16Z",
		TicketStatus: ptrS("Aberto"), Reaberto: ptrB(true),
		MotivoReabertura: ptrS("Não foi resolvido"),
	}}
	got := agruparPorTicket(linhas)[0]
	if !got.Reaberto {
		t.Error("perdeu a marca de reaberto")
	}
	if got.Motivo != "Não foi resolvido" {
		t.Errorf("motivo: %q", got.Motivo)
	}
}

// Valor nulo não pode virar zero silencioso NEM derrubar a soma dos outros.
func TestValorNuloNaoQuebraASoma(t *testing.T) {
	linhas := []linhaDePendencia{
		{Ticket: 999001, Parte: 1, Valor: nil, CriadoEm: "2026-08-01T00:00:00Z"},
		{Ticket: 999001, Parte: 2, Valor: ptrF(50), CriadoEm: "2026-08-02T00:00:00Z"},
	}
	got := agruparPorTicket(linhas)[0]
	if !perto(got.Valor, 50) {
		t.Errorf("soma: %.2f, esperava 50", got.Valor)
	}
	if got.Orcamentos != 2 {
		t.Errorf("o de valor nulo sumiu da contagem: %d", got.Orcamentos)
	}
}

func perto(a, b float64) bool {
	d := a - b
	return d < 0.005 && d > -0.005
}

// A REGRA QUE NINGUÉM CHAMA NÃO EXISTE PARA QUEM TRABALHA
//
//	Estas quatro rotas foram escritas na 015, com PDF, planilha e registro de
//	cobrança — e ficaram um ano e meio sem uma tela que as chamasse. O efeito,
//	medido em 27/08/2026: 68 orçamentos parados por status de chamado, invisíveis
//	no sistema inteiro, enquanto a tela de Correções mostrava 23 e o usuário
//	perguntava por que os números não fechavam.
//
//	Nada quebrou. Os testes passavam, o motor respondia, a rota estava lá. Só não
//	havia porta. Este teste é a porta: se a tela sumir ou parar de chamar uma
//	destas rotas, ele fala antes de o buraco voltar.
//
// POR QUE SÓ AS DE PENDÊNCIAS
//
//	Nem toda rota do motor precisa de tela — há as que o robô chama e as que
//	existem para integração. Uma regra geral aqui viraria uma lista de exceções
//	que alguém mantém à mão, e lista de exceção mantida à mão é onde o próximo
//	buraco se esconde. Estas quatro têm dona: a tela de Pendências.
func TestAsTelasChamamAsRotasQueOMotorServe(t *testing.T) {
	rotas, err := os.ReadFile("rotas.go")
	if err != nil {
		t.Fatalf("não consegui ler o rotas.go: %v", err)
	}
	// Pendências e Fechamento: as duas frentes que nasceram de uma regra que já
	// existia no motor e não tinha porta na tela.
	registradas := regexp.MustCompile(`"(?:GET|POST) (/orcamentos/(?:pendencias|fechamento)[a-z./]*)"`).
		FindAllStringSubmatch(string(rotas), -1)
	if len(registradas) < 5 {
		t.Fatalf("achei só %d rotas de pendências/fechamento — o teste deixou de olhar o que devia",
			len(registradas))
	}

	front := lerOFront(t)
	for _, r := range registradas {
		if !strings.Contains(front, r[1]) {
			t.Errorf("o motor serve %s e nenhuma tela chama — a regra existe, roda, "+
				"e não tem porta. Foi assim que 68 orçamentos parados ficaram invisíveis.", r[1])
		}
	}
}

// lerOFront junta o código das telas num texto só. É busca por texto de
// propósito: entender import de TypeScript daqui seria construir meio compilador
// para responder uma pergunta de uma linha.
func lerOFront(t *testing.T) string {
	t.Helper()
	var tudo strings.Builder
	raiz := "../../../../web/src"
	err := filepath.Walk(raiz, func(caminho string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if e := filepath.Ext(caminho); e != ".ts" && e != ".tsx" {
			return nil
		}
		b, err := os.ReadFile(caminho)
		if err == nil {
			tudo.Write(b)
		}
		return nil
	})
	if err != nil || tudo.Len() == 0 {
		t.Skipf("não achei o código das telas em %s", raiz)
	}
	return tudo.String()
}
