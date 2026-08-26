// rev 1 — as faixas do desconto, uma a uma
//
// Este arquivo existe porque a regra é de DINHEIRO e as faixas se tocam. Errar
// a borda de R$ 500 por um centavo é a diferença entre um orçamento que roda e
// um que para — e nenhum dos dois erros grita.
package regras

import "testing"

// oPadrao é o que valia em agosto de 2026: teto R$ 600, margem 20%, folga 5%.
// A base máxima que sai daí é R$ 500 — o número que o dono nomeou.
var oPadrao = Padrao

func TestBaseMaximaSaiDoTetoEDaMargem(t *testing.T) {
	if b := BaseMaxima(oPadrao); b != 50000 {
		t.Errorf("BaseMaxima = %s, e teto 600 com margem 20%% dá R$ 500,00", b.Reais())
	}
	// Se o teto mudar, a linha muda junto — é o motivo de ela ser calculada.
	if b := BaseMaxima(Parametros{Teto: 120000, MargemBP: 2000}); b != 100000 {
		t.Errorf("com teto de R$ 1.200 a base máxima teria que ser R$ 1.000, e deu %s", b.Reais())
	}
	// E se a margem mudar, também.
	if b := BaseMaxima(Parametros{Teto: 60000, MargemBP: 5000}); b != 40000 {
		t.Errorf("com margem de 50%% a base máxima teria que ser R$ 400, e deu %s", b.Reais())
	}
}

// FAIXA 1 — abaixo da base máxima, entra o valor cheio.
func TestNotaPequenaEntraInteira(t *testing.T) {
	c := AplicarDesconto(45000, 45000, oPadrao) // R$ 450, sem desconto
	if c.Decisao != CustoCheio {
		t.Fatalf("R$ 450 deu %q, esperava passar inteira", c.Decisao)
	}
	if c.Base != 45000 {
		t.Errorf("a base saiu %s, esperava R$ 450,00", c.Base.Reais())
	}
	if c.Ajustada() {
		t.Error("carimbou de ajustada uma nota que não foi tocada")
	}
}

// FAIXA 2 — o bruto passa, o pago cabe: apara e carimba.
func TestNotaComDescontoEAparadaNoTeto(t *testing.T) {
	// O caso do dono: cheio entre 500 e 600.
	c := AplicarDesconto(55000, 48000, oPadrao) // cheio R$ 550, pago R$ 480
	if c.Decisao != CustoNoTeto {
		t.Fatalf("cheio R$ 550 com pago R$ 480 deu %q, esperava ajuste pelo teto", c.Decisao)
	}
	if c.Base != 50000 {
		t.Errorf("a base saiu %s, esperava a base máxima de R$ 500,00", c.Base.Reais())
	}
	if !c.Ajustada() {
		t.Error("aparou a nota e não carimbou — o carimbo é o que explica o corte depois")
	}
	if c.Desconto() != 7000 {
		t.Errorf("o desconto saiu %s, esperava R$ 70,00", c.Desconto().Reais())
	}
}

// FAIXA 2, o caso que o dono decidiu em 26/08: bruto MUITO acima, pago cabe.
func TestNotaBemAcimaDoTetoComPagoQueCabe(t *testing.T) {
	c := AplicarDesconto(70000, 45000, oPadrao) // cheio R$ 700, pago R$ 450
	if c.Decisao != CustoNoTeto {
		t.Fatalf("cheio R$ 700 com pago R$ 450 deu %q — a decisão do dono foi arredondar para R$ 500", c.Decisao)
	}
	if c.Base != 50000 {
		t.Errorf("a base saiu %s, esperava R$ 500,00", c.Base.Reais())
	}
}

// FAIXA 3 — nem o pago cabe.
func TestNotaCaraBloqueia(t *testing.T) {
	c := AplicarDesconto(70000, 52000, oPadrao) // cheio R$ 700, pago R$ 520
	if !c.Bloqueada() {
		t.Fatalf("pago de R$ 520 passou; acima de R$ 500 não existe orçamento dentro do teto")
	}
	if c.Base != 0 {
		t.Errorf("bloqueou e ainda deixou base de %s", c.Base.Reais())
	}
}

// AS BORDAS. É aqui que regra de dinheiro erra.
func TestAsBordasDosQuinhentos(t *testing.T) {
	casos := []struct {
		nome            string
		cheio, desconto Dinheiro
		esperada        DecisaoDoCusto
		base            Dinheiro
	}{
		// Exatamente na base máxima: 500 x 1,20 = 600 = o teto. Cabe SEM corte,
		// então não leva carimbo — nada foi ajustado.
		{"cheio exatamente R$ 500", 50000, 50000, CustoCheio, 50000},
		// Um centavo acima, e sem desconto: o pago também passou de R$ 500.
		{"cheio R$ 500,01 sem desconto", 50001, 50001, CustoBloqueado, 0},
		// Um centavo acima, mas o pago recuou para a linha: apara.
		{"cheio R$ 500,01 e pago R$ 500", 50001, 50000, CustoNoTeto, 50000},
		// O pago um centavo acima da linha já não cabe.
		{"pago R$ 500,01", 60000, 50001, CustoBloqueado, 0},
		{"um centavo abaixo da linha", 49999, 49999, CustoCheio, 49999},
	}
	for _, k := range casos {
		c := AplicarDesconto(k.cheio, k.desconto, oPadrao)
		if c.Decisao != k.esperada {
			t.Errorf("%s: deu %q, esperava %q", k.nome, c.Decisao, k.esperada)
		}
		if c.Base != k.base {
			t.Errorf("%s: base %s, esperava %s", k.nome, c.Base.Reais(), k.base.Reais())
		}
	}
}

// NOTA SEM TOTAL LIDO NÃO É NOTA DE GRAÇA
//
//	Total zero quer dizer "não consegui ler". Tratá-lo como compra por R$ 0
//	faria a nota mais cara do mundo passar sorrindo pela regra.
func TestTotalNaoLidoValeOBruto(t *testing.T) {
	c := AplicarDesconto(70000, 0, oPadrao)
	if !c.Bloqueada() {
		t.Errorf("nota de R$ 700 com total ilegível deu %q — sem o número, vale o bruto", c.Decisao)
	}
	c = AplicarDesconto(30000, 0, oPadrao)
	if c.Decisao != CustoCheio || c.Base != 30000 {
		t.Errorf("nota de R$ 300 com total ilegível deu %q com base %s", c.Decisao, c.Base.Reais())
	}
}

func TestNotaSemValorNaoViraOrcamento(t *testing.T) {
	if c := AplicarDesconto(0, 0, oPadrao); !c.Bloqueada() {
		t.Error("uma nota de R$ 0,00 gerou orçamento")
	}
}

// Teto desligado passa tudo — a mesma decisão que AplicarTeto toma.
func TestTetoDesligadoNaoBloqueiaNada(t *testing.T) {
	sem := Parametros{MargemBP: 2000, Teto: 0}
	c := AplicarDesconto(900000, 800000, sem)
	if c.Decisao != CustoCheio || c.Base != 900000 {
		t.Errorf("com teto desligado deu %q com base %s", c.Decisao, c.Base.Reais())
	}
}

// ---------------------------------------------------------------------------
// o tudo-ou-nada do rateio
// ---------------------------------------------------------------------------

func TestNotaRateadaParaInteiraSeUmPedacoNaoCabe(t *testing.T) {
	c := AplicarDesconto(45000, 45000, oPadrao)
	vereditos := []Veredito{
		{Decisao: Livre, Valor: 20000},
		{Decisao: Reduzido, Valor: 30000, ValorOriginal: 31000},
		{Decisao: Aprovacao, Valor: 40000, ValorOriginal: 40000, JaNoTicket: 55000, Teto: 60000},
	}
	pode, motivo := NotaPodeRodar(c, vereditos)
	if pode {
		t.Fatal("um dos pedaços ia para aprovação e a nota rodou assim mesmo — " +
			"meia nota processada é pior que nenhuma")
	}
	if motivo == "" {
		t.Error("parou a nota sem dizer por quê; é isso que a tela mostra a quem for tratar")
	}
}

// A FOLGA DOS 5% NÃO É ESBARRAR
//
//	Ela é caso previsto: apara para o teto e roda. Se ela parasse a nota, toda
//	nota que encostasse no teto viraria trabalho manual.
func TestAFolgaDeCincoPorCentoNaoParaANota(t *testing.T) {
	c := AplicarDesconto(45000, 45000, oPadrao)
	vereditos := []Veredito{
		{Decisao: Livre, Valor: 20000},
		{Decisao: Reduzido, Valor: 25000, ValorOriginal: 26000},
	}
	if pode, motivo := NotaPodeRodar(c, vereditos); !pode {
		t.Errorf("a folga dos 5%% parou a nota: %s", motivo)
	}
}

func TestNotaBloqueadaNoDescontoNaoRodaNemComTicketVazio(t *testing.T) {
	c := AplicarDesconto(70000, 52000, oPadrao)
	if pode, _ := NotaPodeRodar(c, []Veredito{{Decisao: Livre, Valor: 52000}}); pode {
		t.Error("a nota estava bloqueada pelo desconto e rodou porque o ticket estava vazio")
	}
}

func TestNotaSimplesQueCabeRoda(t *testing.T) {
	c := AplicarDesconto(30000, 30000, oPadrao)
	if pode, motivo := NotaPodeRodar(c, []Veredito{{Decisao: Livre, Valor: 36000}}); !pode {
		t.Errorf("nota de R$ 300 num ticket vazio não rodou: %s", motivo)
	}
}

// A REGRA INTEIRA, DE PONTA A PONTA
//
//	Da nota ao valor que sai, passando pela margem e pelo teto. É o teste que
//	pega o dia em que uma das três partes mudar sozinha.
func TestDaNotaAoOrcamentoNasTresFaixas(t *testing.T) {
	casos := []struct {
		nome            string
		cheio, desconto Dinheiro
		orcamento       Dinheiro
		carimbo         bool
		roda            bool
	}{
		{"R$ 450 sem desconto", 45000, 45000, 54000, false, true},
		{"R$ 550 pagos R$ 480", 55000, 48000, 60000, true, true},
		{"R$ 700 pagos R$ 450", 70000, 45000, 60000, true, true},
		{"R$ 700 pagos R$ 520", 70000, 52000, 0, false, false},
	}
	for _, k := range casos {
		c := AplicarDesconto(k.cheio, k.desconto, oPadrao)
		if c.Ajustada() != k.carimbo {
			t.Errorf("%s: carimbo %v, esperava %v", k.nome, c.Ajustada(), k.carimbo)
		}
		if c.Bloqueada() {
			if k.roda {
				t.Errorf("%s: bloqueou e devia rodar", k.nome)
			}
			continue
		}
		// A margem entra no unitário na geração de verdade; aqui basta o total
		// ideal, que é o que o teto vai olhar. Ticket vazio: nada acumulado.
		comMargem := Dinheiro(dividirArredondando(int64(c.Base)*(10000+oPadrao.MargemBP), 10000))
		v := AplicarTeto(comMargem, 0, oPadrao)
		if pode, motivo := NotaPodeRodar(c, []Veredito{v}); pode != k.roda {
			t.Errorf("%s: rodar=%v (%s), esperava %v", k.nome, pode, motivo, k.roda)
		}
		if v.Valor != k.orcamento {
			t.Errorf("%s: o orçamento saiu %s, esperava %s", k.nome, v.Valor.Reais(), k.orcamento.Reais())
		}
	}
}
