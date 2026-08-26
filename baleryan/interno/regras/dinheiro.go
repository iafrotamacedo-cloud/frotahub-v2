// rev 1 — dinheiro em inteiro, nunca em ponto flutuante (P-12)
//
// POR QUE NÃO float64
//
//	0.1 + 0.2 não dá 0.3 em ponto flutuante, e um orçamento com 40 itens soma 40
//	desses errinhos. No sistema antigo o `+20%` tinha ONZE implementações com
//	DOIS arredondamentos diferentes — e elas divergiam em centavos. Divergência
//	em centavo, num documento que vai para o cliente, é o tipo de defeito que
//	ninguém acha porque ninguém procura.
//
// DUAS ESCALAS, E SÓ DUAS
//
//	Dinheiro    centavos          (2 casas) — o que aparece no documento
//	Preco       décimos de milésimo (4 casas) — o que a nota traz por unidade
//
//	São as mesmas duas escalas do banco (`numeric(14,2)` e `numeric(14,4)`). Ter
//	as mesmas escalas dos dois lados evita a conversão silenciosa que arredonda
//	sem avisar.
package regras

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Dinheiro é um valor em centavos. R$ 1,00 = 100.
type Dinheiro int64

// Preco é um valor unitário em décimos de milésimo. R$ 1,00 = 10000.
type Preco int64

// Quantidade tem as mesmas 4 casas do preço unitário.
type Quantidade int64

const (
	casasDoDinheiro = 100   // 2 casas
	casasDoPreco    = 10000 // 4 casas
)

// ---------------------------------------------------------------------------
// entrada
// ---------------------------------------------------------------------------

// DinheiroDe converte um número decimal (como o que vem do JSON do banco) em
// centavos, arredondando meio para cima.
func DinheiroDe(v float64) Dinheiro { return Dinheiro(arredondar(v * casasDoDinheiro)) }

// PrecoDe converte um número decimal em décimos de milésimo.
func PrecoDe(v float64) Preco { return Preco(arredondar(v * casasDoPreco)) }

// QuantidadeDe converte um número decimal em quantidade.
func QuantidadeDe(v float64) Quantidade { return Quantidade(arredondar(v * casasDoPreco)) }

// arredondar é o ÚNICO arredondamento do sistema.
//
// Meio para cima, sempre, dos dois lados do zero. `math.Round` já faz isso; o
// que esta função garante é que ninguém escreva outro em outro lugar.
func arredondar(v float64) int64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return int64(math.Round(v))
}

// ---------------------------------------------------------------------------
// saída
// ---------------------------------------------------------------------------

// Float devolve o valor para gravar no banco. O banco guarda `numeric`, então a
// conversão acontece uma vez, na borda, e não no meio da conta.
func (d Dinheiro) Float() float64   { return float64(d) / casasDoDinheiro }
func (p Preco) Float() float64      { return float64(p) / casasDoPreco }
func (q Quantidade) Float() float64 { return float64(q) / casasDoPreco }

// Reais escreve "1.234,56" — sem o "R$", que quem monta o documento acrescenta.
func (d Dinheiro) Reais() string {
	sinal := ""
	n := int64(d)
	if n < 0 {
		sinal, n = "-", -n
	}
	inteiro := strconv.FormatInt(n/casasDoDinheiro, 10)
	centavos := fmt.Sprintf("%02d", n%casasDoDinheiro)

	var b strings.Builder
	b.WriteString(sinal)
	for i, c := range inteiro {
		if i > 0 && (len(inteiro)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	b.WriteByte(',')
	b.WriteString(centavos)
	return b.String()
}

// ---------------------------------------------------------------------------
// contas
// ---------------------------------------------------------------------------

// Total multiplica quantidade por preço unitário e devolve centavos.
//
// A multiplicação é feita em inteiro de 64 bits e só o resultado é arredondado.
// Arredondar antes — no unitário, no subtotal — é como nascem as divergências
// de centavo entre duas telas que deviam mostrar o mesmo número.
func Total(q Quantidade, p Preco) Dinheiro {
	// q e p têm 4 casas cada; o produto tem 8. Para chegar a 2 casas dividimos
	// por 10^6, arredondando meio para cima.
	produto := int64(q) * int64(p)
	return Dinheiro(dividirArredondando(produto, 1_000_000))
}

// PrecoQueFecha devolve o preço unitário que reproduz EXATAMENTE o total já
// gravado da linha.
//
// POR QUE ISTO PRECISA EXISTIR
//
//	A leitura de uma nota devolve TRÊS números por item: quantidade, preço
//	unitário e total da linha. Só dois deles se provam. O total da linha entra
//	na soma que `leitor.ContaFecha` confere contra o total impresso da nota —
//	errar um item ali derruba a leitura inteira. O preço unitário não entra em
//	conferência nenhuma: ninguém nunca o confrontou com nada.
//
//	Em 26/08/2026 isso custou dinheiro de verdade. A DAV 19329 tinha um item de
//	R$ 225,90 cujo unitário o OCR leu como zero. A conferência passou, porque a
//	soma dos TOTAIS batia com a nota. Mas quem monta o orçamento recalculava a
//	linha por quantidade × unitário, obteve 1 × R$ 0,00 = zero, e o item sumiu
//	da conta sem uma palavra: a nota de R$ 490,80 virou orçamento de R$ 317,88.
//
//	A lição não é "conferir o unitário também". É que o total da linha é o
//	número que se provou, e é dele que o unitário se recupera.
//
// Devolve `false` quando não há recuperação honesta: sem quantidade não se
// divide, e um unitário que não reproduz o total de volta não serve para nada.
func PrecoQueFecha(q Quantidade, total Dinheiro) (Preco, bool) {
	if q <= 0 || total <= 0 {
		return 0, false
	}
	// Total() divide o produto por 10^6; para desfazer, multiplica-se antes.
	p := Preco(dividirArredondando(int64(total)*1_000_000, int64(q)))
	if Total(q, p) != total {
		return 0, false
	}
	return p, true
}

// dividirArredondando faz a divisão inteira arredondando meio PARA LONGE DO
// ZERO, que é a mesma regra de `arredondar` — as duas precisam concordar, senão
// o sistema teria dois arredondamentos de novo, que é exatamente o defeito que
// este pacote existe para não repetir.
func dividirArredondando(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	if b < 0 {
		a, b = -a, -b
	}
	if a >= 0 {
		return (a + b/2) / b
	}
	return -((-a + b/2) / b)
}
