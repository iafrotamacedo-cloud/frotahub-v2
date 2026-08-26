// rev 1 — as regras do orçamento, em Go puro
//
// Este pacote não fala com o banco, não fala com a rede e não conhece HTTP. É de
// propósito: as três regras que decidem dinheiro — margem, teto e duplicidade —
// precisam ser testáveis sem subir nada.
//
// No sistema antigo elas viviam espalhadas: o `+20%` implementado onze vezes, o
// teto três, e a rotina de gerar orçamento copiada cinco. Corrigir uma regra era
// mexer em cinco lugares e torcer. Aqui cada uma existe uma vez (CORE-06).
package regras

import (
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------
// os parâmetros que o banco fornece
// ---------------------------------------------------------------------------

// Parametros são os números que o cliente decide, lidos da tabela `parametros`
// com a vigência do dia. Chegam prontos: este pacote não sabe de onde vieram.
type Parametros struct {
	// Margem em pontos-base: 2000 = 20%. Inteiro para não reintroduzir ponto
	// flutuante justamente no fator que multiplica todos os itens.
	MargemBP int64
	// Teto de lançamento, em centavos.
	Teto Dinheiro
	// Folga do teto, em pontos-base. Passando do teto em até isto, o valor é
	// REDUZIDO para o teto exato em vez de parar para aprovação.
	FolgaBP int64
}

// Padrao é o que valia em agosto de 2026. Existe para os testes e para o caso —
// que não deve acontecer — de o banco não ter parâmetro vigente. Se for usado em
// produção, quem usa avisa em log: silêncio aqui viraria regra fantasma.
var Padrao = Parametros{MargemBP: 2000, Teto: 60000, FolgaBP: 500}

// ---------------------------------------------------------------------------
// a margem
// ---------------------------------------------------------------------------

// ComMargem acrescenta a margem ao preço unitário.
//
// A margem entra no UNITÁRIO, não no total — é assim desde o sistema antigo, e
// muda o resultado: 3 unidades de R$ 10,00 com 20% no unitário dão 3 × R$ 12,00
// = R$ 36,00; com 20% no total dariam R$ 36,00 também, mas 7 unidades de
// R$ 0,33 divergem em centavos entre os dois caminhos.
//
// E ela NUNCA aparece no documento. O orçamento mostra o preço cobrado; de onde
// ele veio é assunto nosso.
func ComMargem(p Preco, margemBP int64) Preco {
	return Preco(dividirArredondando(int64(p)*(10000+margemBP), 10000))
}

// ---------------------------------------------------------------------------
// o teto
// ---------------------------------------------------------------------------

// Decisao é o que o teto responde. São três saídas, nunca duas: o meio-termo —
// "passou pouco, reduz" — é uma regra de verdade, não um caso especial.
type Decisao string

const (
	// Passa. O valor do orçamento é o valor calculado.
	Livre Decisao = "livre"
	// Passou do teto dentro da folga: o valor é REDUZIDO ao teto exato.
	Reduzido Decisao = "reduzido"
	// Passou demais: o orçamento nasce, mas parado, esperando o cliente.
	Aprovacao Decisao = "aprovacao"
)

// Veredito é a resposta completa do teto, com o número de antes e o de depois.
//
// Os dois ficam visíveis de propósito. Redução silenciosa é a diferença entre um
// sistema explicável e um sistema mágico: quem olhar o orçamento seis meses
// depois precisa conseguir ver que houve um corte, e de quanto.
type Veredito struct {
	Decisao Decisao
	// O valor que sai — já reduzido, quando foi o caso.
	Valor Dinheiro
	// O valor que entrou, antes de qualquer corte.
	ValorOriginal Dinheiro
	// Quanto o ticket já tinha de custo ANTES deste orçamento.
	JaNoTicket Dinheiro
	// O teto que valia no momento da decisão. Gravado junto com o orçamento —
	// sem ele, mudar o parâmetro reescreveria a história.
	Teto Dinheiro
}

// Reduziu é o atalho para quem só quer saber se houve corte.
func (v Veredito) Reduziu() bool { return v.Decisao == Reduzido }

// AplicarTeto decide o destino de um orçamento de `valor`, num ticket que já
// acumula `jaNoTicket`.
//
// O QUE `jaNoTicket` PRECISA SOMAR
//
//	TODOS os custos do ticket, de qualquer tipo — não só os que nós lançamos.
//	Um ticket com R$ 400 de Materiais antigo não pode aceitar mais R$ 600 de Mão
//	de obra: o teto é do ticket, não do nosso tipo de custo. Contar só o nosso
//	seria um teto que parece funcionar e vaza.
//
// POR QUE O CORTE É SOBRE O TOTAL DO TICKET
//
//	O que o cliente aprovou foi um limite por chamado. Se o ticket já tem R$ 200
//	e o orçamento novo é de R$ 450, o total daria R$ 650 — passou R$ 50, que
//	cabe na folga. O corte então tira R$ 50 DESTE orçamento, deixando-o em
//	R$ 400, e o ticket fecha exatamente no teto.
func AplicarTeto(valor, jaNoTicket Dinheiro, p Parametros) Veredito {
	v := Veredito{
		Decisao:       Livre,
		Valor:         valor,
		ValorOriginal: valor,
		JaNoTicket:    jaNoTicket,
		Teto:          p.Teto,
	}
	if p.Teto <= 0 {
		return v // teto desligado: passa tudo, e quem configurou sabe disso
	}

	total := jaNoTicket + valor
	if total <= p.Teto {
		return v
	}

	limiteDaFolga := p.Teto + Dinheiro(dividirArredondando(int64(p.Teto)*p.FolgaBP, 10000))
	if total <= limiteDaFolga {
		v.Decisao = Reduzido
		v.Valor = p.Teto - jaNoTicket
		// O ticket já estourou o teto sozinho, sem este orçamento. Reduzir daria
		// valor negativo — e orçamento negativo não existe. Vai para aprovação.
		if v.Valor <= 0 {
			v.Decisao = Aprovacao
			v.Valor = valor
		}
		return v
	}

	v.Decisao = Aprovacao
	return v
}

// ---------------------------------------------------------------------------
// os itens, e o corte que precisa fechar a conta
// ---------------------------------------------------------------------------

// Item é uma linha do orçamento, já com os dois preços lado a lado.
type Item struct {
	Descricao       string
	Unidade         string
	Quantidade      Quantidade
	UnitarioNota    Preco
	UnitarioCobrado Preco
	Total           Dinheiro
}

// MontarItens aplica a margem a cada linha e devolve os itens somados.
func MontarItens(descricoes []LinhaDaNota, margemBP int64) ([]Item, Dinheiro) {
	itens := make([]Item, 0, len(descricoes))
	var soma Dinheiro
	for _, d := range descricoes {
		cobrado := ComMargem(d.Unitario, margemBP)
		it := Item{
			Descricao:       d.Descricao,
			Unidade:         d.Unidade,
			Quantidade:      d.Quantidade,
			UnitarioNota:    d.Unitario,
			UnitarioCobrado: cobrado,
			Total:           Total(d.Quantidade, cobrado),
		}
		soma += it.Total
		itens = append(itens, it)
	}
	return itens, soma
}

// LinhaDaNota é o item como veio da nota, antes da margem.
type LinhaDaNota struct {
	Descricao  string
	Unidade    string
	Quantidade Quantidade
	Unitario   Preco
}

var ErrSemItens = errors.New("não dá para cortar um orçamento sem itens")

// Encaixar reduz os itens para que somem exatamente `alvo`.
//
// POR QUE O CORTE PRECISA CHEGAR ATÉ OS ITENS
//
//	O caminho fácil é gravar o total cortado e deixar os itens como estavam. Aí
//	o documento que vai para o cliente mostra 14 linhas que somam R$ 640 e um
//	total de R$ 600 — e a primeira pessoa que somar na calculadora descobre.
//	Documento que não fecha é documento que não passa em auditoria.
//
//	Então o corte é proporcional, item a item, e a sobra de centavos vai para a
//	ÚLTIMA linha. Assim a soma fecha no centavo, sempre.
func Encaixar(itens []Item, alvo Dinheiro) ([]Item, error) {
	if len(itens) == 0 {
		return nil, ErrSemItens
	}
	var soma Dinheiro
	for _, it := range itens {
		soma += it.Total
	}
	if soma == alvo || soma <= 0 {
		return itens, nil
	}

	saida := make([]Item, len(itens))
	copy(saida, itens)

	var acumulado Dinheiro
	for i := range saida {
		if i == len(saida)-1 {
			// A última linha absorve a sobra: é o que faz a soma fechar exata.
			saida[i].Total = alvo - acumulado
		} else {
			saida[i].Total = Dinheiro(dividirArredondando(int64(saida[i].Total)*int64(alvo), int64(soma)))
			acumulado += saida[i].Total
		}
		// O unitário cobrado acompanha o total, senão a linha se contradiz.
		if saida[i].Quantidade > 0 {
			saida[i].UnitarioCobrado = Preco(dividirArredondando(int64(saida[i].Total)*1_000_000, int64(saida[i].Quantidade)))
		}
	}
	return saida, nil
}

// ---------------------------------------------------------------------------
// duplicidade
// ---------------------------------------------------------------------------

// MesmaNota diz se duas notas são a mesma para efeito de duplicidade.
//
// A REGRA MUDOU EM AGOSTO DE 2026, E ESTA É A NOVA
//
//	Antes: um custo por ticket. Errado — um ticket pode ter várias notas
//	legítimas. Agora: a mesma NOTA não vira orçamento duas vezes no mesmo
//	ticket.
//
//	Onde existe chave de acesso (44 dígitos, única no Brasil), a comparação é
//	ela e acabou. Sem chave — DAV, nota sem número —, o par número+valor.
func MesmaNota(chaveA, numeroA string, valorA Dinheiro, chaveB, numeroB string, valorB Dinheiro) bool {
	if chaveA != "" && chaveB != "" {
		return chaveA == chaveB
	}
	if numeroA != "" && numeroA == numeroB {
		return true
	}
	return numeroA == "" && numeroB == "" && valorA == valorB && valorA > 0
}

// ---------------------------------------------------------------------------
// o desconto do fornecedor
// ---------------------------------------------------------------------------

// O PROBLEMA, COMO ELE APARECEU
//
//	`NF 9160`, de 26/08/2026: cinco itens somando R$ 514,60, e a nota dizendo
//	R$ 463,14. A diferença é exatamente 10% — desconto do fornecedor, não erro
//	de leitura.
//
//	Até aqui o sistema marcava a margem em cima dos ITENS. Ou seja: cobrava 20%
//	sobre R$ 514,60, um custo que a empresa não teve. E do outro lado, uma nota
//	cujo bruto passa do teto seria recusada mesmo quando o que se pagou por ela
//	cabia folgado.
//
// A LINHA DOS R$ 500 NÃO É UM NÚMERO ESCOLHIDO
//
//	Ela é o teto dividido pela margem: 600 / 1,20 = 500. É o maior custo que
//	ainda cabe no teto depois de marcado. Por isso ela é CALCULADA e não
//	escrita — no dia em que o teto ou a margem mudar, ela muda junto, sozinha.
//	Escrever 500 aqui seria plantar um número que envelhece em silêncio.

// DecisaoDoCusto é o que a regra do desconto responde.
type DecisaoDoCusto string

const (
	// A nota cabe inteira: o custo é o valor cheio dela.
	CustoCheio DecisaoDoCusto = "cheio"
	// O bruto passa da base máxima, mas o que se pagou cabe. O custo é
	// aparado para a base máxima e o orçamento fecha no teto exato.
	// É este o caso que ganha o carimbo "ajustada por conta do teto".
	CustoNoTeto DecisaoDoCusto = "no_teto"
	// Nem com o desconto a nota cabe. Não gera orçamento.
	CustoBloqueado DecisaoDoCusto = "bloqueado"
)

// Custo é a resposta completa da regra do desconto.
//
// Os três valores ficam visíveis de propósito: seis meses depois, alguém
// precisa conseguir ver que a nota valia R$ 700, que se pagou R$ 450, e que o
// orçamento saiu por R$ 600 porque era o teto — sem ter que reconstruir a
// conta de cabeça.
type Custo struct {
	Decisao DecisaoDoCusto
	// Base é o custo que vai virar orçamento, ANTES da margem.
	Base Dinheiro
	// Cheio é a soma dos itens, como estão na nota.
	Cheio Dinheiro
	// ComDesconto é o que a nota diz que se pagou.
	ComDesconto Dinheiro
	// BaseMaxima é a linha que valia na hora da decisão. Gravada junto, pelo
	// mesmo motivo que o teto é: mudar o parâmetro não pode reescrever o
	// passado.
	BaseMaxima Dinheiro
}

// Ajustada diz se a nota foi aparada — é o que acende o carimbo interno.
func (c Custo) Ajustada() bool { return c.Decisao == CustoNoTeto }

// Bloqueada diz se a nota não gera orçamento nenhum.
func (c Custo) Bloqueada() bool { return c.Decisao == CustoBloqueado }

// Desconto é quanto o fornecedor abateu. Zero quando não houve.
func (c Custo) Desconto() Dinheiro {
	if c.Cheio <= c.ComDesconto {
		return 0
	}
	return c.Cheio - c.ComDesconto
}

// BaseMaxima é o maior custo que ainda cabe no teto depois da margem.
//
// Com teto de R$ 600 e margem de 20%, dá R$ 500 — o número que o dono nomeou.
// Ele sai da conta, e não de uma constante, para que teto e margem continuem
// sendo os únicos parâmetros que alguém precisa mexer.
func BaseMaxima(p Parametros) Dinheiro {
	if p.Teto <= 0 {
		return 0
	}
	return Dinheiro(dividirArredondando(int64(p.Teto)*10000, 10000+p.MargemBP))
}

// AplicarDesconto decide qual valor da nota vira o custo do orçamento.
//
// AS TRÊS FAIXAS, COMO O DONO AS DITOU EM 26/08/2026
//
//	valor cheio abaixo da base máxima      entra o valor cheio, orçamento normal
//	cheio acima, mas o pago cabe           apara na base máxima -> orçamento no
//	                                       teto, com carimbo interno
//	nem o pago cabe                        bloqueia por limite de teto
//
// POR QUE O QUE DECIDE O BLOQUEIO É O VALOR PAGO
//
//	Porque é ele que diz o que a empresa realmente gastou. Uma nota de R$ 700
//	comprada por R$ 450 é uma compra de R$ 450 — recusá-la pelo bruto seria
//	recusar dinheiro que cabia no teto. Já uma compra de R$ 520 não cabe, tenha
//	a nota o desconto que tiver: acima da base máxima não existe orçamento
//	possível dentro do teto.
func AplicarDesconto(cheio, comDesconto Dinheiro, p Parametros) Custo {
	maxima := BaseMaxima(p)
	c := Custo{
		Decisao:     CustoCheio,
		Base:        cheio,
		Cheio:       cheio,
		ComDesconto: comDesconto,
		BaseMaxima:  maxima,
	}

	// NOTA SEM TOTAL LIDO NÃO É NOTA COM DESCONTO
	//   Total zero quer dizer "não consegui ler", e tratar isso como compra de
	//   graça faria a nota mais cara do mundo passar pela regra sorrindo. Sem
	//   o número, vale o bruto — que é o pior caso, e o pior caso é o seguro.
	if c.ComDesconto <= 0 {
		c.ComDesconto = cheio
	}

	// Teto desligado: passa tudo, e quem configurou sabe disso. Mesma decisão
	// que `AplicarTeto` toma, pelo mesmo motivo.
	if p.Teto <= 0 {
		return c
	}
	// Nota sem valor não vira orçamento. Não é bloqueio por teto — é ausência
	// de nota — mas o destino é o mesmo: pessoa olhando.
	if cheio <= 0 {
		c.Decisao = CustoBloqueado
		c.Base = 0
		return c
	}

	if c.ComDesconto > maxima {
		c.Decisao = CustoBloqueado
		c.Base = 0
		return c
	}
	if cheio > maxima {
		c.Decisao = CustoNoTeto
		c.Base = maxima
	}
	return c
}

// ---------------------------------------------------------------------------
// a nota inteira, quando ela se divide
// ---------------------------------------------------------------------------

// NotaPodeRodar aplica o tudo-ou-nada do rateio.
//
// UMA NOTA RATEADA É UMA DECISÃO SÓ
//
//	Ela vira vários orçamentos, um por ticket, e o teto vale para cada um
//	separadamente — R$ 600 por ticket, não R$ 600 pela nota. Mas se UM dos
//	pedaços não passa, a nota inteira para.
//
//	Não é preciosismo. Gerar quatro dos cinco orçamentos deixaria a nota
//	metade-lançada: o material do quinto ticket já foi comprado e entregue, e
//	não há onde cobrá-lo. Quem fosse tratar o caso teria que desfazer os quatro
//	primeiros para poder refazer a divisão — e desfazer orçamento lançado é
//	trabalho no Trílogo, na planilha e no faturamento.
//
//	Meia nota processada é pior que nenhuma. Esta é a regra que o dono deu em
//	26/08/2026: "caso algum dos orçamentos da nota esbarrar na regra, a nota não
//	é processada, ganha flag e vai para tratamento".
//
// A FOLGA DOS 5% NÃO É ESBARRAR
//
//	Passar do teto dentro da folga é caso previsto: o valor é aparado para o
//	teto exato e o orçamento roda normalmente, com carimbo. O que para a nota é
//	o que vai para APROVAÇÃO — aquilo que nem aparando cabe.
func NotaPodeRodar(c Custo, vereditos []Veredito) (bool, string) {
	if c.Bloqueada() {
		return false, fmt.Sprintf(
			"bloqueada por limite de teto: a nota custou %s, e o máximo que cabe é %s",
			c.ComDesconto.Reais(), c.BaseMaxima.Reais())
	}
	for _, v := range vereditos {
		if v.Decisao == Aprovacao {
			return false, fmt.Sprintf(
				"bloqueada por limite de teto: um dos orçamentos ficaria em %s num ticket que já tem %s, e o teto é %s",
				v.ValorOriginal.Reais(), v.JaNoTicket.Reais(), v.Teto.Reais())
		}
	}
	return true, ""
}
