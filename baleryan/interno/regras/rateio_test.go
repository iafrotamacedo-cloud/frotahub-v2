package regras

import "testing"

// A nota que revelou o defeito: NF 660575, da SV, 26 itens, 14 tickets.
// Os três itens abaixo são os que mais doem — o de preço miúdo (a abraçadeira
// que saía a sete centavos), o de quantidade contínua (o cabo) e o que não
// alcança todos os tickets (o pacote).
func aNota() []LinhaDaNota {
	return []LinhaDaNota{
		{Descricao: "ABRACADEIRA NYLON 2,5X100", Unidade: "UN",
			Quantidade: QuantidadeDe(30), Unitario: PrecoDe(0.88)},
		{Descricao: "CABO FLEXIVEL 2,5MM", Unidade: "M",
			Quantidade: QuantidadeDe(300), Unitario: PrecoDe(2.35)},
		{Descricao: "PACOTE DE TERMINAIS", Unidade: "PC",
			Quantidade: QuantidadeDe(5), Unitario: PrecoDe(12.40)},
	}
}

// ---------------------------------------------------------------------------
// o grão
// ---------------------------------------------------------------------------

func TestOPassoSaiDaPropriaNota(t *testing.T) {
	casos := []struct {
		quanto float64
		passo  Quantidade
		porque string
	}{
		{300, casasDoPreco, "300 M é metro inteiro"},
		{30, casasDoPreco, "30 UN é unidade inteira"},
		{2.5, 1000, "2,5 KG só precisa de décimos"},
		{2.55, 100, "2,55 KG precisa de centésimos"},
		{0.125, 10, "0,125 precisa de milésimos"},
		{1.2345, 1, "1,2345 usa a última casa que existe"},
	}
	for _, c := range casos {
		if p := passoDe(QuantidadeDe(c.quanto)); p != c.passo {
			t.Errorf("%s: passoDe(%g) = %d, esperado %d", c.porque, c.quanto, p, c.passo)
		}
	}
}

// ---------------------------------------------------------------------------
// o que não se divide
// ---------------------------------------------------------------------------

// O DEFEITO EM UMA LINHA. Era isto que saía errado: a abraçadeira de R$ 0,88
// virava R$ 0,0753 nos catorze orçamentos. Se este teste voltar a falhar, o
// cliente voltou a receber um preço que não existe.
func TestOPrecoUnitarioNuncaMuda(t *testing.T) {
	nota := aNota()
	for _, n := range []int{1, 2, 5, 14, 27} {
		for i := 0; i < n; i++ {
			for _, l := range Repartir(nota, n, i) {
				for _, orig := range nota {
					if l.Descricao == orig.Descricao && l.Unitario != orig.Unitario {
						t.Fatalf("n=%d i=%d %s: unitário virou %d, era %d",
							n, i, l.Descricao, l.Unitario, orig.Unitario)
					}
				}
			}
		}
	}
}

func TestNaoExisteMeiaTomada(t *testing.T) {
	nota := []LinhaDaNota{
		{Descricao: "TOMADA 2P+T", Unidade: "UN",
			Quantidade: QuantidadeDe(3), Unitario: PrecoDe(9.90)},
	}
	vistos := map[Quantidade]int{}
	for i := 0; i < 2; i++ {
		for _, l := range Repartir(nota, 2, i) {
			if l.Quantidade%casasDoPreco != 0 {
				t.Fatalf("saiu fração de tomada: %d", l.Quantidade)
			}
			vistos[l.Quantidade]++
		}
	}
	if vistos[QuantidadeDe(2)] != 1 || vistos[QuantidadeDe(1)] != 1 {
		t.Fatalf("3 tomadas entre 2 tickets deviam sair 2 e 1, saiu %v", vistos)
	}
}

func TestCadaParteRespeitaOGraoDaLinha(t *testing.T) {
	nota := aNota()
	for _, n := range []int{2, 7, 14} {
		for i := 0; i < n; i++ {
			for _, l := range Repartir(nota, n, i) {
				for _, orig := range nota {
					if l.Descricao != orig.Descricao {
						continue
					}
					passo := passoDe(orig.Quantidade)
					if l.Quantidade%passo != 0 {
						t.Fatalf("n=%d i=%d %s: %d não é múltiplo do passo %d",
							n, i, l.Descricao, l.Quantidade, passo)
					}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// o que tem que fechar
// ---------------------------------------------------------------------------

// 300 metros entre 14 tickets tem que continuar sendo 300 metros. Sem a
// distribuição do resto sairiam 294, e seis metros comprados não seriam
// cobrados de ninguém.
func TestASomaDasPartesEAQuantidadeDaNota(t *testing.T) {
	nota := aNota()
	for _, n := range []int{1, 2, 3, 7, 13, 14, 29} {
		somas := map[string]Quantidade{}
		for i := 0; i < n; i++ {
			for _, l := range Repartir(nota, n, i) {
				somas[l.Descricao] += l.Quantidade
			}
		}
		for _, orig := range nota {
			if somas[orig.Descricao] != orig.Quantidade {
				t.Fatalf("n=%d %s: as partes somam %d e a nota diz %d",
					n, orig.Descricao, somas[orig.Descricao], orig.Quantidade)
			}
		}
	}
}

// 5 pacotes entre 14 tickets: cinco recebem um, nove não recebem nenhum. E os
// nove não recebem uma LINHA com zero — item que não foi entregue não é linha
// de documento.
func TestItemQueNaoAlcancaFicaDeFora(t *testing.T) {
	nota := []LinhaDaNota{
		{Descricao: "PACOTE DE TERMINAIS", Unidade: "PC",
			Quantidade: QuantidadeDe(5), Unitario: PrecoDe(12.40)},
	}
	com, sem := 0, 0
	for i := 0; i < 14; i++ {
		partes := Repartir(nota, 14, i)
		switch len(partes) {
		case 0:
			sem++
		case 1:
			if partes[0].Quantidade != QuantidadeDe(1) {
				t.Fatalf("i=%d levou %d pacotes", i, partes[0].Quantidade)
			}
			com++
		default:
			t.Fatalf("i=%d recebeu %d linhas de uma linha só", i, len(partes))
		}
	}
	if com != 5 || sem != 9 {
		t.Fatalf("5 pacotes entre 14: %d com e %d sem", com, sem)
	}
}

func TestNenhumaLinhaSaiComQuantidadeZero(t *testing.T) {
	nota := aNota()
	for _, n := range []int{2, 14, 31, 50} {
		for i := 0; i < n; i++ {
			for _, l := range Repartir(nota, n, i) {
				if l.Quantidade <= 0 {
					t.Fatalf("n=%d i=%d %s saiu com quantidade %d",
						n, i, l.Descricao, l.Quantidade)
				}
			}
		}
	}
}

func TestNotaDeUmTicketSoNaoEMexida(t *testing.T) {
	nota := aNota()
	partes := Repartir(nota, 1, 0)
	if len(partes) != len(nota) {
		t.Fatalf("nota de 1 ticket saiu com %d linhas de %d", len(partes), len(nota))
	}
	for k := range nota {
		if partes[k] != nota[k] {
			t.Fatalf("linha %d mudou: %+v virou %+v", k, nota[k], partes[k])
		}
	}
}

// Repartir recebe a fatia por VALOR de quem chama; se ela mexesse na fatia
// original, o segundo ticket receberia a nota já mutilada pelo primeiro.
func TestRepartirNaoEstragaANotaDeQuemChamou(t *testing.T) {
	nota := aNota()
	antes := append([]LinhaDaNota(nil), nota...)
	for i := 0; i < 14; i++ {
		Repartir(nota, 14, i)
	}
	for k := range antes {
		if nota[k] != antes[k] {
			t.Fatalf("a nota original mudou na linha %d: %+v virou %+v", k, antes[k], nota[k])
		}
	}
}

// ---------------------------------------------------------------------------
// o pago
// ---------------------------------------------------------------------------

// Um centavo de sobra multiplicado por 509 orçamentos é o tipo de divergência
// que ninguém consegue explicar seis meses depois.
func TestOPagoFechaNoCentavo(t *testing.T) {
	pago := DinheiroDe(1000.01)
	for _, n := range []int{1, 2, 3, 7, 13, 14} {
		cheios := make([]Dinheiro, n)
		for i := range cheios {
			cheios[i] = Dinheiro(100 + 37*i) // pedaços deliberadamente desiguais
		}
		var soma Dinheiro
		for _, v := range RepartirPago(pago, cheios) {
			soma += v
		}
		if soma != pago {
			t.Fatalf("n=%d: as partes somam %s e o pago é %s", n, soma.Reais(), pago.Reais())
		}
	}
}

// Quem levou mais material paga mais. É isto que faz o desconto da nota cair
// sobre cada ticket na medida do que ele recebeu.
func TestOPagoSegueOQueCadaUmLevou(t *testing.T) {
	pago := DinheiroDe(900)
	cheios := []Dinheiro{DinheiroDe(100), DinheiroDe(300), DinheiroDe(600)}
	fora := RepartirPago(pago, cheios)
	quer := []Dinheiro{DinheiroDe(90), DinheiroDe(270), DinheiroDe(540)}
	for i := range quer {
		if fora[i] != quer[i] {
			t.Fatalf("parte %d: %s, esperado %s", i, fora[i].Reais(), quer[i].Reais())
		}
	}
}

func TestPagoDeUmTicketSoEOPagoInteiro(t *testing.T) {
	pago := DinheiroDe(1234.56)
	fora := RepartirPago(pago, []Dinheiro{DinheiroDe(1000)})
	if len(fora) != 1 || fora[0] != pago {
		t.Fatalf("um ticket só devia levar a nota inteira, levou %v", fora)
	}
}

// Sem base para proporção — todos os pedaços zerados — ainda assim o dinheiro
// não pode sumir.
func TestSemBaseParaProporcaoODinheiroNaoSome(t *testing.T) {
	pago := DinheiroDe(100.01)
	fora := RepartirPago(pago, []Dinheiro{0, 0, 0})
	var soma Dinheiro
	for _, v := range fora {
		soma += v
	}
	if soma != pago {
		t.Fatalf("as partes somam %s e o pago é %s", soma.Reais(), pago.Reais())
	}
}

func TestPagoSemTicketNenhumNaoQuebra(t *testing.T) {
	if fora := RepartirPago(DinheiroDe(10), nil); len(fora) != 0 {
		t.Fatalf("sem ticket devia sair vazio, saiu %v", fora)
	}
}

// ---------------------------------------------------------------------------
// as duas metades juntas
// ---------------------------------------------------------------------------

// A prova que fecha o ciclo, na nota real: o que os catorze tickets recebem,
// somado ao preço da nota, é exatamente o que a nota vale.
func TestONF660575FechaItemAItem(t *testing.T) {
	nota := aNota()
	var cheio Dinheiro
	for _, l := range nota {
		cheio += Total(l.Quantidade, l.Unitario)
	}
	var soma Dinheiro
	for i := 0; i < 14; i++ {
		for _, l := range Repartir(nota, 14, i) {
			soma += Total(l.Quantidade, l.Unitario)
		}
	}
	// A diferença que sobra é só arredondamento de linha, e tem que ser miúda:
	// um centavo por linha por ticket, no pior caso.
	folga := soma - cheio
	if folga < 0 {
		folga = -folga
	}
	if folga > Dinheiro(len(nota)*14) {
		t.Fatalf("as partes somam %s e a nota vale %s", soma.Reais(), cheio.Reais())
	}
}
