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

import "errors"

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
