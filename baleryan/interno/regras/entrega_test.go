// rev 1 — a linha de entrega
package regras

import "testing"

func linhaDe(desc string, qtd, unit float64) LinhaDaNota {
	return LinhaDaNota{Descricao: desc, Unidade: "UN",
		Quantidade: QuantidadeDe(qtd), Unitario: PrecoDe(unit)}
}

// O CASO REAL — 24 linhas assim no banco em 26/08/2026
func TestEntregaViraUmaUnidadePeloTotal(t *testing.T) {
	casos := []struct{ qtd, esperado float64 }{
		{10, 10}, {12, 12}, {15, 15}, {20, 20}, {40, 40},
	}
	for _, c := range casos {
		saida := NormalizarEntrega([]LinhaDaNota{linhaDe("SERVICO DE ENTREGA", c.qtd, 1)})
		if saida[0].Quantidade != QuantidadeDe(1) {
			t.Errorf("qtd %.0f: ficou com quantidade %v, esperava 1", c.qtd, saida[0].Quantidade.Float())
		}
		if saida[0].Unitario != PrecoDe(c.esperado) {
			t.Errorf("qtd %.0f: unitário ficou %v, esperava %.2f", c.qtd, saida[0].Unitario.Float(), c.esperado)
		}
	}
}

// O DINHEIRO NÃO PODE MUDAR
//
//	Esta regra é de APRESENTAÇÃO. Se ela mexesse no total, seria outra coisa — e
//	uma que ninguém autorizou.
func TestInverterNaoMudaOTotal(t *testing.T) {
	antes := []LinhaDaNota{
		linhaDe("PARAF MAD PHILIPS 4,8X50", 10, 0.30),
		linhaDe("SERVICO DE ENTREGA", 15, 1),
	}
	depois := NormalizarEntrega(antes)

	var somaAntes, somaDepois Dinheiro
	for i := range antes {
		somaAntes += Total(antes[i].Quantidade, antes[i].Unitario)
		somaDepois += Total(depois[i].Quantidade, depois[i].Unitario)
	}
	if somaAntes != somaDepois {
		t.Errorf("a soma mudou: %s viraram %s", somaAntes.Reais(), somaDepois.Reais())
	}
	// E com a margem por cima também: é assim que o orçamento sai.
	_, comAntes := MontarItens(antes, 2000)
	_, comDepois := MontarItens(depois, 2000)
	if comAntes != comDepois {
		t.Errorf("com margem a soma mudou: %s viraram %s", comAntes.Reais(), comDepois.Reais())
	}
}

// OS DOIS CRITÉRIOS PRECISAM VALER JUNTOS
func TestSoInverteOQueEhAConvencao(t *testing.T) {
	naoMexe := []LinhaDaNota{
		// material barato comprado às dúzias: o nome não é entrega
		linhaDe("BUCHA PLÁSTICA 8MM", 10, 1),
		// entrega cobrada de verdade por unidade: o unitário não é R$ 1,00
		linhaDe("SERVICO DE ENTREGA", 2, 8.50),
		// uma entrega de R$ 1,00: inverter não teria o que fazer
		linhaDe("SERVICO DE ENTREGA", 1, 1),
	}
	saida := NormalizarEntrega(naoMexe)
	for i := range naoMexe {
		if saida[i] != naoMexe[i] {
			t.Errorf("mexeu em %q: %v × %v virou %v × %v", naoMexe[i].Descricao,
				naoMexe[i].Quantidade.Float(), naoMexe[i].Unitario.Float(),
				saida[i].Quantidade.Float(), saida[i].Unitario.Float())
		}
	}
}

// ACENTO E CAIXA NÃO PODEM CRIAR UMA SEGUNDA REGRA
func TestAsGrafiasDaMesmaCoisa(t *testing.T) {
	for _, desc := range []string{
		"SERVICO DE ENTREGA", "SERVIÇO DE ENTREGA", "Serviço de Entrega",
		"TAXA DE ENTREGA", "ENTREGA",
	} {
		saida := NormalizarEntrega([]LinhaDaNota{linhaDe(desc, 12, 1)})
		if saida[0].Quantidade != QuantidadeDe(1) {
			t.Errorf("%q não foi reconhecida como entrega", desc)
		}
	}
}

// A FATIA ORIGINAL NÃO É TOCADA
//
//	O que está gravado em `documento_itens` é a nota do fornecedor, e ela
//	continua valendo o que o papel diz. Quem inverte é o orçamento.
func TestONormalizarNaoEstragaAEntrada(t *testing.T) {
	entrada := []LinhaDaNota{linhaDe("SERVICO DE ENTREGA", 12, 1)}
	_ = NormalizarEntrega(entrada)
	if entrada[0].Quantidade != QuantidadeDe(12) || entrada[0].Unitario != PrecoDe(1) {
		t.Error("a fatia de entrada foi alterada")
	}
}
