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

// A FILA DE LANÇAR SÓ MOSTRA O QUE DÁ PARA LANÇAR
//
//	Medido em 27/08/2026: 93 orçamentos `gerado` e 5 podendo subir. A tela
//	mostrava os 93 e o "lançar todos" percorria os 93, para 88 levarem a mesma
//	recusa de sempre. O dono resumiu: *"pode clicar até morrer que não adianta
//	nada"*.
//
//	Uma fila em que a maioria dos itens não pode ser trabalhada não é fila: é
//	uma lista que a pessoa aprende a ignorar — e o que se aprende a ignorar um
//	dia esconde o que importava.
func TestAFilaDeLancarPedeSoOQuePodeSubir(t *testing.T) {
	fonte, err := os.ReadFile("../../../../web/src/telas/orcamentos/Lancar.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	texto := string(fonte)
	if !strings.Contains(texto, "'destino', 'pode_lancar'") {
		t.Error("a tela de Lançar não filtra por `destino=pode_lancar` — ela volta a " +
			"mostrar a fila inteira, e o botão de lote volta a tentar os travados")
	}
	// E o motor precisa aceitar o filtro, senão a tela pede e o banco devolve tudo.
	motor, err := os.ReadFile("gerar.go")
	if err != nil {
		t.Fatalf("não consegui ler o gerar.go: %v", err)
	}
	if !strings.Contains(string(motor), `q.Get("destino")`) {
		t.Error("o motor ignora o filtro `destino` — a tela pede e recebe a lista inteira")
	}
}

// NINGUÉM É CONTADO DUAS VEZES NO PAINEL
//
//	O cartão de Correções somava `recusados` junto com as notas travadas. Só que
//	orçamento recusado por status de chamado JÁ ESTÁ contado em Pendências: o
//	mesmo registro aparecia nos dois cartões, e 88 + 36 contava vinte e um deles
//	duas vezes. Era uma das razões de o painel parecer que não fechava.
func TestOPainelNaoContaOMesmoOrcamentoDuasVezes(t *testing.T) {
	fonte, err := os.ReadFile("../../../../web/src/telas/orcamentos/Orcamentos.tsx")
	if err != nil {
		t.Skipf("não achei o painel: %v", err)
	}
	texto := string(fonte)
	// Fora os comentários, que EXPLICAM o defeito citando a soma antiga.
	texto = regexp.MustCompile(`(?m)^\s*//[^\n]*`).ReplaceAllString(texto, "")
	if regexp.MustCompile(`numero:\s*d\.sem_ticket[^,]*d\.recusados`).MatchString(texto) {
		t.Error("o cartão de Correções voltou a somar `recusados` — esses orçamentos " +
			"já estão contados em Pendências ou na fila de Lançar, e a soma passa a " +
			"contar os mesmos registros duas vezes")
	}
}

// O LOTE NÃO PROMETE O QUE NÃO VAI ENTREGAR
//
//	Medido em 27/08/2026: a fila mostrava 5 "podem subir", o botão dizia
//	"Lançar todos que podem subir (5)", e o resultado foi **0 de 5 subiram** —
//	três barrados por duplicidade e dois pelo teto. As duas travas fizeram o que
//	deviam; quem errou foi o botão, que contou como pronto o que precisa de uma
//	pessoa decidindo.
//
//	`ticket_status` e `trilogo_fora` continuam entrando no lote de propósito:
//	aqueles destravam sozinhos quando o mundo anda, e tentar de novo é o certo.
//	Duplicidade e teto não andam sozinhos.
func TestOLoteNaoContaOQuePrecisaDeDecisao(t *testing.T) {
	fonte, err := os.ReadFile("../../../../web/src/telas/orcamentos/Lancar.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	texto := string(fonte)

	corpo := texto[strings.Index(texto, "function podeSubir("):]
	corpo = corpo[:strings.Index(corpo, "\n}")]
	if !strings.Contains(corpo, "precisaDeDecisao(o)") {
		t.Error("o lote voltou a contar os orçamentos travados por duplicidade ou teto — " +
			"o botão vai prometer um número e entregar zero, como em 27/08/2026")
	}

	decide := texto[strings.Index(texto, "function precisaDeDecisao("):]
	decide = decide[:strings.Index(decide, "\n}")]
	for _, flag := range []string{"possivel_duplicata", "teto"} {
		if !strings.Contains(decide, flag) {
			t.Errorf("`precisaDeDecisao` não cobre %q — o lote vai tentar e falhar sempre", flag)
		}
	}
	// E os que destravam sozinhos NÃO podem entrar aqui: tirá-los do lote os
	// deixaria parados para sempre esperando um clique manual.
	for _, flag := range []string{"ticket_status", "trilogo_fora"} {
		if strings.Contains(decide, flag) {
			t.Errorf("`precisaDeDecisao` inclui %q — esse bloqueio destrava sozinho quando "+
				"o chamado anda, e tirá-lo do lote é condená-lo a um clique manual eterno", flag)
		}
	}
}

// A TELA NÃO APONTA PARA UMA FRENTE QUE NÃO EXISTE MAIS
//
//	"Correções ▸ Recusados" saiu em 27/08/2026. Uma mensagem que manda a pessoa
//	a uma tela inexistente é pior que nenhuma mensagem: ela gasta a ida.
func TestNinguemMandaOUsuarioParaRecusados(t *testing.T) {
	arquivos, err := filepath.Glob("../../../../web/src/telas/orcamentos/*.tsx")
	if err != nil {
		t.Skipf("não achei as telas: %v", err)
	}
	for _, a := range arquivos {
		b, err := os.ReadFile(a)
		if err != nil {
			continue
		}
		// Fora os comentários, que REGISTRAM a remoção.
		texto := regexp.MustCompile(`(?m)^\s*//[^\n]*`).ReplaceAllString(string(b), "")
		texto = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(texto, "")
		if strings.Contains(texto, "Correções ▸ Recusados") || strings.Contains(texto, "Correções › Recusados") {
			t.Errorf("%s manda a pessoa para Correções ▸ Recusados, que não existe mais",
				filepath.Base(a))
		}
	}
}

// A REGRA DE "QUEM SÓ UMA PESSOA DESTRAVA" MORA NO BANCO
//
//	Ela nasceu no navegador em 27/08/2026. No mesmo dia o motor precisou dela
//	(para filtrar no banco, CORE-10) e o balanço também (para não contar como
//	"pode lançar" o que a tela mostra em Pendências). Três cópias da mesma lista
//	de dois nomes é como duas delas um dia discordam.
//
//	A migração 035 pôs a resposta numa coluna, `precisa_decisao`. Este teste
//	cobra que ninguém volte a escrever a lista à mão.
func TestQuemPrecisaDeDecisaoEhDecididoNoBanco(t *testing.T) {
	// 1) a coluna existe na migração
	var achou bool
	for _, caminho := range migracoes(t) {
		b, err := os.ReadFile(caminho)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "AS precisa_decisao") {
			achou = true
		}
	}
	if !achou {
		t.Fatal("nenhuma migração cria `precisa_decisao` — a regra voltou a viver só no código")
	}

	// 2) o motor filtra por ela, e não por uma lista própria
	motor, err := os.ReadFile("gerar.go")
	if err != nil {
		t.Fatalf("não consegui ler o gerar.go: %v", err)
	}
	corpo := string(motor)[strings.Index(string(motor), "func (m *Modulo) listarOrcamentos("):]
	corpo = corpo[:strings.Index(corpo, "\nfunc ")]
	if !strings.Contains(corpo, "precisa_decisao=is.") {
		t.Error("o motor não filtra por `precisa_decisao` — ele vai precisar da lista " +
			"de bloqueios escrita à mão, e vira a terceira cópia dela")
	}
	for _, flag := range []string{`"teto"`, `"possivel_duplicata"`} {
		if strings.Contains(corpo, flag) {
			t.Errorf("o `listarOrcamentos` escreve %s à mão — quem decide isso é a view", flag)
		}
	}

	// 3) a tela lê a coluna
	tela, err := os.ReadFile("../../../../web/src/telas/orcamentos/Lancar.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	if !strings.Contains(string(tela), "o.precisa_decisao") {
		t.Error("a tela de Lançar não lê `precisa_decisao` — ela mantém a própria lista, " +
			"e as duas vão divergir na primeira vez que um bloqueio novo aparecer")
	}
}

// O CARTÃO DO FECHAMENTO NÃO MOSTRA UM CONTADOR ERRADO
//
//	Ele exibia `notas_arquivos` sob o rótulo "notas na base". `notas_arquivos`
//	conta a FILA de notas, não a base: com a fila vazia, o painel anunciava
//	"0 notas na base" tendo 91. Num cartão que existe para provar que as contas
//	fecham, é o pior lugar possível para um número errado.
func TestOCartaoDoFechamentoNaoInventaNumero(t *testing.T) {
	fonte, err := os.ReadFile("../../../../web/src/telas/orcamentos/Orcamentos.tsx")
	if err != nil {
		t.Skipf("não achei o painel: %v", err)
	}
	texto := regexp.MustCompile(`(?m)^\s*//[^\n]*`).ReplaceAllString(string(fonte), "")
	i := strings.Index(texto, "chave: 'fechamento'")
	if i < 0 {
		t.Fatal("o cartão de Fechamento sumiu do painel")
	}
	cartao := texto[i:]
	if fim := strings.Index(cartao, "\n    },"); fim > 0 {
		cartao = cartao[:fim]
	}
	if strings.Contains(cartao, "numero:") {
		t.Error("o cartão de Fechamento voltou a mostrar um contador — os números desta " +
			"etapa são dois balanços com total e diferença, e resumi-los a um só foi " +
			"como ele passou a anunciar \"0 notas na base\" tendo 91")
	}
}
