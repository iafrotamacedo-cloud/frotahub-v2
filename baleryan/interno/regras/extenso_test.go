// rev 1 — o valor por extenso
package regras

import "testing"

func TestPorExtenso(t *testing.T) {
	casos := map[float64]string{
		54.96:   "cinquenta e quatro reais e noventa e seis centavos", // o caso do legado
		588.96:  "quinhentos e oitenta e oito reais e noventa e seis centavos",
		88.68:   "oitenta e oito reais e sessenta e oito centavos",
		1:       "um real",
		2:       "dois reais",
		0.01:    "um centavo",
		0.02:    "dois centavos",
		0:       "zero real",
		100:     "cem reais",  // "cem", e não "cento"
		101:     "cento e um reais",
		1000:    "mil reais",  // "mil", e não "um mil"
		1020:    "mil e vinte reais",
		1120:    "mil, cento e vinte reais", // vírgula, e não "e"
		1454.22: "mil, quatrocentos e cinquenta e quatro reais e vinte e dois centavos",
		15.9:    "quinze reais e noventa centavos",
		2000000: "dois milhões de reais",
	}
	for v, esperado := range casos {
		if deu := PorExtenso(DinheiroDe(v)); deu != esperado {
			t.Errorf("%.2f virou %q, esperava %q", v, deu, esperado)
		}
	}
}

// O EXTENSO EXISTE PARA CONFERIR O NÚMERO
//
//	Se ele fosse derivado de outro lugar que não o próprio total, os dois
//	poderiam discordar — e aí o documento teria dois valores diferentes, que é
//	pior do que ter um só.
func TestExtensoNasceDoMesmoNumero(t *testing.T) {
	d := DinheiroDe(317.88)
	if PorExtenso(d) != "trezentos e dezessete reais e oitenta e oito centavos" {
		t.Errorf("deu %q", PorExtenso(d))
	}
	if d.Reais() != "317,88" {
		t.Errorf("o número deu %q", d.Reais())
	}
}
