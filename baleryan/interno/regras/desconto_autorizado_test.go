// rev 1 — o desconto que alguém assina embaixo
//
// A regra é de DINHEIRO e o botão que a dispara pede senha. Se ela errar, erra
// no valor cobrado do cliente — e erra com aparência de autorização.
package regras

import "testing"

// O CASO NORMAL: a nota passa pouco, e o desconto é pequeno.
func TestDescontoLevaOOrcamentoAoTeto(t *testing.T) {
	// Pago R$ 520 → orçamento natural R$ 624. O teto é R$ 600.
	d := CalcularDesconto(52000, 0, Padrao)
	if !d.Pode {
		t.Fatalf("recusou um desconto de 24 reais: %s", d.Motivo)
	}
	if d.Original != 62400 {
		t.Errorf("o orçamento original saiu %s, esperava R$ 624,00", d.Original.Reais())
	}
	if d.Final != Padrao.Teto {
		t.Errorf("o final saiu %s, esperava o teto de %s", d.Final.Reais(), Padrao.Teto.Reais())
	}
	// 24 / 624 = 3,846% → 385 pontos-base.
	if d.BP != 385 {
		t.Errorf("o desconto saiu %d bp (%s), esperava 385 bp", d.BP, Porcentagem(d.BP))
	}
}

// O LIMITE DOS 20% É A MARGEM INTEIRA
//
//	Abrir mão de 20% é abrir mão de tudo o que se ganha. Abaixo disso a empresa
//	trabalharia pagando para trabalhar, e nenhuma autorização deveria poder
//	fazer isso com dois cliques.
func TestDescontoNaoPassaDaMargemInteira(t *testing.T) {
	// R$ 625 pagos → orçamento R$ 750 → desconto de R$ 150 = exatamente 20%.
	if d := CalcularDesconto(62500, 0, Padrao); !d.Pode || d.BP != 2000 {
		t.Errorf("R$ 625 deveria caber exatamente no limite; deu pode=%v bp=%d (%s)",
			d.Pode, d.BP, d.Motivo)
	}
	// Um real a mais e já não cabe.
	d := CalcularDesconto(62600, 0, Padrao)
	if d.Pode {
		t.Errorf("passou de 20%%: %d bp", d.BP)
	}
	if d.Motivo == "" {
		t.Error("recusou sem dizer por quê; é essa frase que a tela mostra no botão apagado")
	}
}

// O TETO É DO TICKET, NÃO DA NOTA
//
//	Se o ticket já tem custo, o que cabe nesta nota é menos — e existe o caso em
//	que NENHUM desconto resolve, porque quem estourou o teto não foi ela.
func TestDescontoConsideraOQueJaEstaNoTicket(t *testing.T) {
	// Ticket com R$ 200 lançados: cabe R$ 400 nesta nota.
	// Pago R$ 380 → orçamento R$ 456 → desconto de R$ 56 = 12,28%.
	d := CalcularDesconto(38000, 20000, Padrao)
	if !d.Pode {
		t.Fatalf("recusou: %s", d.Motivo)
	}
	if d.Final != 40000 {
		t.Errorf("o final saiu %s, esperava R$ 400,00 (teto menos o que já está lá)", d.Final.Reais())
	}
}

func TestQuandoOTicketJaEstaCheioNenhumDescontoResolve(t *testing.T) {
	d := CalcularDesconto(30000, 60000, Padrao)
	if d.Pode {
		t.Fatal("ofereceu desconto num ticket que já está no teto — o botão não consertaria nada")
	}
	if d.Motivo == "" {
		t.Error("não explicou que o problema não está nesta nota")
	}
}

// Nota que já cabe não precisa de desconto, e oferecer um seria dar dinheiro à toa.
func TestNotaQueJaCabeNaoGanhaDesconto(t *testing.T) {
	if d := CalcularDesconto(30000, 0, Padrao); d.Pode {
		t.Errorf("ofereceu %s de desconto numa nota que já cabia", Porcentagem(d.BP))
	}
}

func TestNotaSemValorNaoGanhaDesconto(t *testing.T) {
	if d := CalcularDesconto(0, 0, Padrao); d.Pode {
		t.Error("ofereceu desconto numa nota sem valor lido")
	}
}

// A AUTORIZAÇÃO NÃO MUDA A REGRA — ELA SÓ ASSINA EMBAIXO
func TestDescontoAutorizadoDestravaOCustoBloqueado(t *testing.T) {
	bloqueado := AplicarDesconto(52000, 52000, Padrao)
	if !bloqueado.Bloqueada() {
		t.Fatalf("o caso de partida não estava bloqueado: %q", bloqueado.Decisao)
	}
	c := ComDescontoAutorizado(bloqueado, Padrao)
	if c.Decisao != CustoNoTeto {
		t.Errorf("depois de autorizado deu %q, esperava fechar no teto", c.Decisao)
	}
	if c.Base != BaseMaxima(Padrao) {
		t.Errorf("a base saiu %s, esperava a base máxima %s",
			c.Base.Reais(), BaseMaxima(Padrao).Reais())
	}
}

// Autorizar não pode piorar uma nota que estava boa.
func TestAutorizarNaoMexeNoQueJaPassava(t *testing.T) {
	bom := AplicarDesconto(30000, 30000, Padrao)
	if c := ComDescontoAutorizado(bom, Padrao); c.Decisao != CustoCheio || c.Base != 30000 {
		t.Errorf("mexeu numa nota que já passava: %q base %s", c.Decisao, c.Base.Reais())
	}
}

func TestPorcentagemEscritaEmPortugues(t *testing.T) {
	casos := map[int64]string{2000: "20%", 385: "3,85%", 1250: "12,5%", 5: "0,05%", 100: "1%"}
	for bp, esperado := range casos {
		if veio := Porcentagem(bp); veio != esperado {
			t.Errorf("Porcentagem(%d) = %q, esperava %q", bp, veio, esperado)
		}
	}
}
