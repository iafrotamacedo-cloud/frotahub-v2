// rev 1 — recuperar o unitário a partir do total da linha
package regras

import "testing"

func TestPrecoQueFechaRecuperaOUnitario(t *testing.T) {
	casos := []struct {
		nome     string
		qtd      float64
		total    float64
		esperado float64
	}{
		{"o item da DAV 19329", 1, 225.90, 225.90},
		{"o item da DAV 18317", 2, 8.50, 4.25},
		{"quantidade fracionada", 2.5, 25.00, 10.00},
		{"divisão que não é exata mas volta", 3, 10.00, 3.3333},
	}
	for _, c := range casos {
		p, deu := PrecoQueFecha(QuantidadeDe(c.qtd), DinheiroDe(c.total))
		if !deu {
			t.Errorf("%s: desistiu de um caso recuperável", c.nome)
			continue
		}
		if p != PrecoDe(c.esperado) {
			t.Errorf("%s: devolveu %v, esperava %v", c.nome, p.Float(), c.esperado)
		}
		// A PROVA É O CAMINHO DE VOLTA
		//   Um unitário que não reconstrói o total não serve para nada: seria
		//   trocar um número errado por outro.
		if v := Total(QuantidadeDe(c.qtd), p); v != DinheiroDe(c.total) {
			t.Errorf("%s: %v × %v deu %s, esperava %.2f", c.nome, c.qtd, p.Float(), v.Reais(), c.total)
		}
	}
}

func TestPrecoQueFechaDesisteQuandoNaoDa(t *testing.T) {
	casos := []struct {
		nome  string
		qtd   float64
		total float64
	}{
		{"sem quantidade não se divide", 0, 10.00},
		{"quantidade negativa", -1, 10.00},
		{"total zerado não prova nada", 1, 0},
	}
	for _, c := range casos {
		if _, deu := PrecoQueFecha(QuantidadeDe(c.qtd), DinheiroDe(c.total)); deu {
			t.Errorf("%s: inventou um preço", c.nome)
		}
	}
}
