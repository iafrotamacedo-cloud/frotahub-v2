package regras

import "testing"

// A margem entra no unitário. Este teste existe porque no sistema antigo havia
// duas contas diferentes rodando ao mesmo tempo, e elas divergiam em centavos.
func TestMargemNoUnitario(t *testing.T) {
	casos := []struct {
		nome     string
		unitario Preco
		esperado Preco
	}{
		{"valor redondo", PrecoDe(10), PrecoDe(12)},
		{"o caso que divergia", PrecoDe(0.33), PrecoDe(0.396)},
		{"centavo quebrado arredonda meio para cima", PrecoDe(19), PrecoDe(22.8)},
		{"zero continua zero", 0, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := ComMargem(c.unitario, 2000); got != c.esperado {
				t.Fatalf("ComMargem(%v) = %v, queria %v", c.unitario.Float(), got.Float(), c.esperado.Float())
			}
		})
	}
}

// Sete unidades de R$ 0,33: o caso em que aplicar a margem no total e no
// unitário dão respostas diferentes. A nossa resposta é a do unitário.
func TestMargemNoUnitarioDivergeDoTotal(t *testing.T) {
	q := QuantidadeDe(7)
	unit := PrecoDe(0.33)

	peloUnitario := Total(q, ComMargem(unit, 2000))
	peloTotal := Dinheiro(dividirArredondando(int64(Total(q, unit))*12000, 10000))

	if peloUnitario == peloTotal {
		t.Skip("neste caso os dois caminhos coincidem; o teste não prova nada")
	}
	if peloUnitario != DinheiroDe(2.77) {
		t.Fatalf("pelo unitário = %s, queria 2,77", peloUnitario.Reais())
	}
}

func TestTotalNaoAcumulaErro(t *testing.T) {
	// 40 itens de R$ 0,10 têm que dar exatamente R$ 4,00. Em float64 esta soma
	// não fecha; é a razão de existir o int64.
	var soma Dinheiro
	for i := 0; i < 40; i++ {
		soma += Total(QuantidadeDe(1), PrecoDe(0.10))
	}
	if soma != DinheiroDe(4) {
		t.Fatalf("soma = %s, queria 4,00", soma.Reais())
	}
}

func TestReais(t *testing.T) {
	casos := map[Dinheiro]string{
		0:         "0,00",
		5:         "0,05",
		100:       "1,00",
		123456:    "1.234,56",
		100000000: "1.000.000,00",
		-25050:    "-250,50",
	}
	for v, esperado := range casos {
		if got := v.Reais(); got != esperado {
			t.Fatalf("Dinheiro(%d).Reais() = %q, queria %q", v, got, esperado)
		}
	}
}

// ---------------------------------------------------------------------------
// o teto
// ---------------------------------------------------------------------------

func TestTetoAsTresSaidas(t *testing.T) {
	p := Padrao // teto 600, folga 5% -> limite 630

	casos := []struct {
		nome       string
		valor      Dinheiro
		jaNoTicket Dinheiro
		decisao    Decisao
		valorFinal Dinheiro
	}{
		{"bem abaixo", DinheiroDe(149.42), 0, Livre, DinheiroDe(149.42)},
		{"exatamente no teto", DinheiroDe(600), 0, Livre, DinheiroDe(600)},
		{"um centavo acima entra na folga", DinheiroDe(600.01), 0, Reduzido, DinheiroDe(600)},
		{"no limite da folga", DinheiroDe(630), 0, Reduzido, DinheiroDe(600)},
		{"um centavo além da folga", DinheiroDe(630.01), 0, Aprovacao, DinheiroDe(630.01)},
		{"nasce grande", DinheiroDe(1425.30), 0, Aprovacao, DinheiroDe(1425.30)},

		// A soma com o que o ticket já tem — o caso que o sistema antigo só
		// conseguia ver na hora de lançar.
		{"soma estoura dentro da folga", DinheiroDe(420), DinheiroDe(200), Reduzido, DinheiroDe(400)},
		{"soma estoura muito", DinheiroDe(450), DinheiroDe(300), Aprovacao, DinheiroDe(450)},
		{"o ticket já passou sozinho", DinheiroDe(50), DinheiroDe(620), Aprovacao, DinheiroDe(50)},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			v := AplicarTeto(c.valor, c.jaNoTicket, p)
			if v.Decisao != c.decisao {
				t.Fatalf("decisão = %q, queria %q", v.Decisao, c.decisao)
			}
			if v.Valor != c.valorFinal {
				t.Fatalf("valor = %s, queria %s", v.Valor.Reais(), c.valorFinal.Reais())
			}
			if v.ValorOriginal != c.valor {
				t.Fatalf("o valor original se perdeu: %s", v.ValorOriginal.Reais())
			}
			if v.Teto != p.Teto {
				t.Fatalf("o teto do dia não foi gravado")
			}
		})
	}
}

// O ticket 130998 do teste real: já tinha R$ 66,96 e recebeu R$ 950. O Trílogo
// não barrou nada — quem tem que barrar somos nós.
func TestTetoNoCasoReal(t *testing.T) {
	v := AplicarTeto(DinheiroDe(950), DinheiroDe(66.96), Padrao)
	if v.Decisao != Aprovacao {
		t.Fatalf("R$ 950 sobre R$ 66,96 tinha que parar para aprovação, veio %q", v.Decisao)
	}
}

func TestTetoDesligadoDeixaPassar(t *testing.T) {
	v := AplicarTeto(DinheiroDe(5000), 0, Parametros{MargemBP: 2000, Teto: 0})
	if v.Decisao != Livre || v.Valor != DinheiroDe(5000) {
		t.Fatalf("teto zero devia deixar passar, veio %q %s", v.Decisao, v.Valor.Reais())
	}
}

// ---------------------------------------------------------------------------
// o corte que precisa fechar
// ---------------------------------------------------------------------------

func TestEncaixarFechaNoCentavo(t *testing.T) {
	linhas := []LinhaDaNota{
		{Descricao: "fita led", Quantidade: QuantidadeDe(50), Unitario: PrecoDe(19)},
		{Descricao: "conector", Quantidade: QuantidadeDe(3), Unitario: PrecoDe(7.77)},
		{Descricao: "fixador", Quantidade: QuantidadeDe(7), Unitario: PrecoDe(1.13)},
	}
	itens, soma := MontarItens(linhas, 2000)

	cortados, err := Encaixar(itens, DinheiroDe(600))
	if err != nil {
		t.Fatal(err)
	}
	var fecha Dinheiro
	for _, it := range cortados {
		fecha += it.Total
	}
	if fecha != DinheiroDe(600) {
		t.Fatalf("os itens somam %s, e o total diz 600,00 — documento que não fecha", fecha.Reais())
	}
	if soma <= DinheiroDe(600) {
		t.Fatalf("o caso de teste não estoura o teto (soma %s); ele não prova nada", soma.Reais())
	}
	// Nenhuma linha pode virar negativa no corte.
	for i, it := range cortados {
		if it.Total < 0 {
			t.Fatalf("linha %d ficou negativa: %s", i, it.Total.Reais())
		}
	}
}

func TestEncaixarSemItensRecusa(t *testing.T) {
	if _, err := Encaixar(nil, DinheiroDe(600)); err == nil {
		t.Fatal("cortar um orçamento vazio tinha que dar erro")
	}
}

func TestEncaixarQuandoJaFechaNaoMexe(t *testing.T) {
	itens := []Item{{Descricao: "x", Quantidade: QuantidadeDe(1), UnitarioCobrado: PrecoDe(600), Total: DinheiroDe(600)}}
	saida, err := Encaixar(itens, DinheiroDe(600))
	if err != nil {
		t.Fatal(err)
	}
	if saida[0].UnitarioCobrado != PrecoDe(600) {
		t.Fatal("mexeu num orçamento que já fechava")
	}
}

// ---------------------------------------------------------------------------
// duplicidade
// ---------------------------------------------------------------------------

func TestMesmaNota(t *testing.T) {
	chave := "23260814248351000120550010000197021394632944"

	if !MesmaNota(chave, "19702", DinheiroDe(950), chave, "outro", DinheiroDe(1)) {
		t.Fatal("a chave de acesso manda sozinha")
	}
	if MesmaNota(chave, "19702", DinheiroDe(950), "23260814248351000120550010000197021394632945", "19702", DinheiroDe(950)) {
		t.Fatal("chaves diferentes são notas diferentes, ainda que o número coincida")
	}
	if !MesmaNota("", "9921", DinheiroDe(300), "", "9921", DinheiroDe(999)) {
		t.Fatal("sem chave, o número decide")
	}
	if !MesmaNota("", "", DinheiroDe(300), "", "", DinheiroDe(300)) {
		t.Fatal("sem chave e sem número, o valor decide")
	}
	if MesmaNota("", "", 0, "", "", 0) {
		t.Fatal("duas notas sem nada não podem ser declaradas iguais")
	}
}
