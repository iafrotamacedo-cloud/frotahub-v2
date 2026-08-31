// rev 1 — o rateio reparte QUANTIDADE, nunca preço
//
// A REGRA, NAS PALAVRAS DO DONO (31/08/2026)
//
//	"é só dividir a quantidade de metros de cabo entre os chamados... o que não
//	 pode ser dividido é o valor unitário e a unidade mínima citada"
//
// O QUE ESTAVA ERRADO ANTES
//
//	O rateio dividia o VALOR da nota por igual entre os tickets e depois
//	espremia todos os itens para caberem naquela fatia — o que fazia o preço
//	unitário mudar. Medido na NF 660575 (SV, 26 itens, 14 tickets): a
//	abraçadeira de R$ 0,88 saía a R$ 0,0753 nos catorze orçamentos, cada um
//	deles listando as 30 unidades inteiras.
//
//	O cliente recebia um documento dizendo "30 abraçadeiras a sete centavos" —
//	um preço que não existe, numa quantidade que não foi entregue naquela loja.
//	A conta do dinheiro fechava no centavo; o que mentia era a composição, que é
//	justamente a parte que alguém lê.
//
// O QUE NÃO SE DIVIDE
//
//	O PREÇO. Ele é o da nota, e sai igual em todos os tickets.
//
//	A UNIDADE MÍNIMA CITADA. Não existe meia tomada. O passo de cada linha sai
//	da própria quantidade da nota: 300 M divide de metro em metro; 2,55 KG
//	divide de centésimo em centésimo. A nota diz qual é o grão, e o rateio o
//	respeita — ver `passoDe`.
//
// O QUE ACONTECE COM ITEM MENOR QUE O NÚMERO DE TICKETS
//
//	5 pacotes de abraçadeira entre 14 tickets: cinco tickets recebem um pacote,
//	nove não recebem nenhum. É o certo. Dar 0,36 pacote a cada um seria voltar
//	a inventar o que não foi comprado — e a linha nem existiria no documento.
package regras

// passoDe descobre a menor fração que a quantidade da nota usa.
//
// `Quantidade` guarda 4 casas, então 300 unidades são 3.000.000. Se o número
// divide certo por 10.000, a nota falou em unidades inteiras e é de unidade em
// unidade que se reparte. Se só divide por 1.000, ela falou em décimos. E assim
// por diante, até o milésimo de milésimo.
//
// É isto que a frase "a unidade mínima citada" quer dizer: quem define o grão é
// a nota, não uma regra escrita aqui.
func passoDe(q Quantidade) Quantidade {
	for _, passo := range []Quantidade{casasDoPreco, 1000, 100, 10} {
		if q%passo == 0 {
			return passo
		}
	}
	return 1
}

// Repartir devolve as linhas que cabem ao ticket `i` de `n`.
//
// A soma das n partes é EXATAMENTE a quantidade da nota, linha por linha: o
// resto da divisão é distribuído de um em um pelos primeiros tickets, e não
// jogado fora nem empilhado no último. Sem isso, uma nota de 300 metros entre
// 14 tickets entregaria 294 e sumiria com 6.
//
// Linha que não alcança este ticket sai de fora — não entra com quantidade zero,
// porque item que não foi entregue não é linha de documento.
func Repartir(linhas []LinhaDaNota, n, i int) []LinhaDaNota {
	if n <= 1 {
		return linhas
	}
	fora := make([]LinhaDaNota, 0, len(linhas))
	for _, l := range linhas {
		passo := passoDe(l.Quantidade)
		graos := int64(l.Quantidade) / int64(passo)
		base := graos / int64(n)
		resto := graos % int64(n)

		meus := base
		if int64(i) < resto {
			meus++
		}
		if meus <= 0 {
			continue
		}
		l.Quantidade = Quantidade(meus) * passo
		fora = append(fora, l)
	}
	return fora
}

// RepartirPago devolve, de uma vez, o pago de CADA ticket.
//
// De uma vez, e não um por chamada, porque o último depende de todos os
// outros: é ele que absorve a sobra dos arredondamentos. Uma função que
// devolvesse um valor por vez teria que refazer a conta inteira a cada chamada
// para saber quanto já foi — e duas contas iguais em lugares diferentes é como
// nascem as divergências de centavo.
func RepartirPago(pago Dinheiro, cheios []Dinheiro) []Dinheiro {
	n := len(cheios)
	fora := make([]Dinheiro, n)
	if n == 0 {
		return fora
	}
	if n == 1 {
		fora[0] = pago
		return fora
	}
	var total Dinheiro
	for _, c := range cheios {
		total += c
	}
	if total <= 0 {
		// Sem base para proporção, cai no antigo: por igual, sobra no último.
		parte := pago / Dinheiro(n)
		for i := range fora {
			fora[i] = parte
		}
		fora[n-1] = pago - parte*Dinheiro(n-1)
		return fora
	}
	var dados Dinheiro
	for i := 0; i < n-1; i++ {
		fora[i] = Dinheiro(dividirArredondando(int64(pago)*int64(cheios[i]), int64(total)))
		dados += fora[i]
	}
	fora[n-1] = pago - dados
	return fora
}
