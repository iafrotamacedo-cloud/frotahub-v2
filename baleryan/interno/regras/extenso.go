// rev 1 — o valor por extenso
//
// POR QUE UM DOCUMENTO DE COBRANÇA ESCREVE O VALOR DUAS VEZES
//
//	Porque o número sozinho aceita um traço a mais. "R$ 54,96" com uma vírgula
//	deslocada vira R$ 549,60 e ninguém percebe olhando; "cinquenta e quatro reais
//	e noventa e seis centavos" não vira outra coisa por acidente. As duas formas
//	se conferem uma à outra — é a mesma ideia da soma dos itens contra o total da
//	nota, aplicada a quem lê.
package regras

import "strings"

var (
	ate19 = []string{
		"zero", "um", "dois", "três", "quatro", "cinco", "seis", "sete", "oito",
		"nove", "dez", "onze", "doze", "treze", "catorze", "quinze", "dezesseis",
		"dezessete", "dezoito", "dezenove",
	}
	dezenas = []string{
		"", "", "vinte", "trinta", "quarenta", "cinquenta", "sessenta",
		"setenta", "oitenta", "noventa",
	}
	centenas = []string{
		"", "cento", "duzentos", "trezentos", "quatrocentos", "quinhentos",
		"seiscentos", "setecentos", "oitocentos", "novecentos",
	}
)

// PorExtenso escreve o valor como se fala.
//
// Cobre até milhões, que é uma ordem de grandeza acima de qualquer orçamento
// que este sistema vá emitir — o teto de um ticket é R$ 600.
func PorExtenso(d Dinheiro) string {
	negativo := d < 0
	if negativo {
		d = -d
	}
	reais := int64(d) / 100
	centavos := int64(d) % 100

	var partes []string
	switch {
	case reais == 1:
		partes = append(partes, "um real")
	case reais > 1:
		// "dois milhões DE reais", mas "dois milhões e quinhentos mil reais".
		// O "de" só entra quando o número termina exatamente no milhão.
		if reais >= 1_000_000 && reais%1_000_000 == 0 {
			partes = append(partes, grupos(reais)+" de reais")
		} else {
			partes = append(partes, grupos(reais)+" reais")
		}
	}
	switch {
	case centavos == 1:
		partes = append(partes, "um centavo")
	case centavos > 1:
		partes = append(partes, grupos(centavos)+" centavos")
	}
	if len(partes) == 0 {
		return "zero real"
	}
	texto := strings.Join(partes, " e ")
	if negativo {
		return "menos " + texto
	}
	return texto
}

func grupos(n int64) string {
	switch {
	case n >= 1_000_000:
		milhoes := n / 1_000_000
		resto := n % 1_000_000
		nome := " milhões"
		if milhoes == 1 {
			nome = " milhão"
		}
		return juntar(grupos(milhoes)+nome, resto)
	case n >= 1000:
		mil := n / 1000
		resto := n % 1000
		// "mil", e não "um mil".
		cabeca := "mil"
		if mil > 1 {
			cabeca = grupos(mil) + " mil"
		}
		return juntar(cabeca, resto)
	default:
		return ate999(n)
	}
}

// juntar decide entre "e" e vírgula, que é onde o português tem regra própria:
// "mil e vinte", mas "mil, cento e vinte".
func juntar(cabeca string, resto int64) string {
	if resto == 0 {
		return cabeca
	}
	if resto < 100 || resto%100 == 0 {
		return cabeca + " e " + ate999(resto)
	}
	return cabeca + ", " + ate999(resto)
}

func ate999(n int64) string {
	switch {
	case n < 20:
		return ate19[n]
	case n < 100:
		d := dezenas[n/10]
		if n%10 == 0 {
			return d
		}
		return d + " e " + ate19[n%10]
	case n == 100:
		return "cem" // "cento" só existe acompanhado
	default:
		c := centenas[n/100]
		if n%100 == 0 {
			return c
		}
		return c + " e " + ate999(n%100)
	}
}
