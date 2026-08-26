// rev 1 — camada 2, segunda família: o DAV do SysPDV
//
// O SISTEMA SÓ CONHECIA UM TIPO DE DOCUMENTO
//
//	`texto.go` foi escrito para a DANFE: chave de acesso, CNPJ, "valor total da
//	nota". Medindo 35 documentos reais do contrato, 31 deles NÃO são DANFE — são
//	"DOCUMENTO AUXILIAR DE VENDA" emitido pelo SysPDV da Rodrigues, que é quem
//	fornece quase tudo. Ele não tem chave de acesso, não tem CNPJ rotulado do
//	jeito que o regex espera, e chama o total de "Total a pagar".
//
//	Resultado: para a maioria esmagadora das notas, a camada 2 devolvia nada, e a
//	leitura inteira ficava dependendo da IA estruturar texto de OCR. Este arquivo
//	é a outra família.
//
// E AQUI OS ITENS SÃO PARSEÁVEIS — NA DANFE NÃO ERAM
//
//	A ressalva de `texto.go` ("tabela de DANFE em texto de OCR é o inferno dos
//	regex") continua verdadeira. Mas o DAV do SysPDV é outra coisa: cada item
//	começa com um código de catorze dígitos, seguido de traço. Isso ancora a
//	linha. Medido: 10 de 10 itens lidos corretamente no documento 19058, sem IA
//	nenhuma.
//
// ITEM CANCELADO É O DETALHE QUE DECIDE A CONTA
//
//	O SysPDV imprime o item cancelado na lista, marcado "< Item ... Cancelado >",
//	e NÃO o inclui no total. Somar tudo daria mais que o documento — e a trava
//	aritmética recusaria uma leitura perfeita. Medido no documento 19072: bruto
//	268,97, cancelado 75,67, total 193,30.
package leitor

import (
	"regexp"
	"strings"
)

var (
	daFamiliaDAV = regexp.MustCompile(`(?i)documento\s+auxiliar\s+de\s+venda`)
	daFamiliaNFe = regexp.MustCompile(`(?i)danfe|nota\s+fiscal\s+eletr`)

	doNumeroDoc     = regexp.MustCompile(`(?i)n[ºo°.]{0,3}\s*do\s*documento\s*[:.]?\s*(\d{4,12})`)
	doTotalPagar    = regexp.MustCompile(`(?i)total\s*a\s*pagar\s*[:.]?\s*([\d.,]+)`)
	doValorProdutos = regexp.MustCompile(`(?i)valor\s+produtos\s*[:.]?\s*([\d.,]+)`)
	daObservacao    = regexp.MustCompile(`(?i)observa[çc][ãa]o\s*[:.]?\s*(.*)`)
	daEmissaoDAV    = regexp.MustCompile(`(?i)dt\.?\s*emis\.?\s*[:.]?\s*(\d{2})/(\d{2})/(\d{4})`)
	doCNPJdav       = regexp.MustCompile(`(?i)cnpj\s*[:.]?\s*(\d{14}|\d{2}[.\s]\d{3}[.\s]\d{3}[/\s]\d{4}[-\s]\d{2})`)

	// Um item começa com o código do produto e um traço. É a única âncora
	// confiável: a descrição quebra em várias linhas e as colunas escorregam.
	daLinhaDeItem = regexp.MustCompile(`^\s*(\d{8,20})\s*[-–—]\s*(.*)$`)
	doDecimal     = regexp.MustCompile(`\d[\d.]*,\d{2}|\b\d+\.\d{2}\b|\b\d+,\d{3}\b`)
	doCancelado   = regexp.MustCompile(`(?i)cancelad[oa]`)
)

// EhDAV diz se o texto é um Documento Auxiliar de Venda do SysPDV.
func EhDAV(texto string) bool {
	return daFamiliaDAV.MatchString(texto) && !daFamiliaNFe.MatchString(texto)
}

// DoDAV lê um Documento Auxiliar de Venda. Devolve nil se o texto não for um.
func DoDAV(texto string) *Leitura {
	if !EhDAV(texto) {
		return nil
	}
	l := &Leitura{Tipo: "dav", Camada: DoOCR}

	if m := doNumeroDoc.FindStringSubmatch(texto); m != nil {
		l.DAV = strings.TrimLeft(m[1], "0")
		l.Numero = l.DAV
	}
	// O TOTAL A PAGAR MANDA, NÃO O BRUTO
	//   Quando há item cancelado, o bruto inclui o que não vai ser pago.
	for _, re := range []*regexp.Regexp{doTotalPagar, doValorProdutos} {
		if m := re.FindStringSubmatch(texto); m != nil {
			if v, ok := Decimal(m[1]); ok && v > 0 {
				l.ValorTotal = v
				break
			}
		}
	}
	if m := daEmissaoDAV.FindStringSubmatch(texto); m != nil {
		l.Emissao = m[3] + "-" + m[2] + "-" + m[1]
	}
	if m := doCNPJdav.FindStringSubmatch(texto); m != nil {
		if d := SoDigitos(m[1]); len(d) == 14 {
			l.EmitenteCNPJ = d
		}
	}
	// A OBSERVAÇÃO É O CAMPO, NÃO O TEXTO INTEIRO
	//   `texto.go` guardava a página toda como observação, e quem caçasse
	//   ticket ali dentro tinha a nota inteira como palheiro. Aqui ela é o que
	//   está escrito depois de "Observação:", que é onde o fornecedor escreve.
	if m := daObservacao.FindStringSubmatch(texto); m != nil {
		if obs := Enxugar(primeiraLinha(m[1])); obs != "" {
			l.Observacao = obs
			l.ObservacaoDoCampo = true
		}
	}
	if l.Observacao == "" {
		l.Observacao = Enxugar(texto)
	}

	l.Itens = itensDoDAV(texto)
	l.Confianca = Conferir(l)
	return l
}

func primeiraLinha(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// itensDoDAV varre as linhas de produto.
//
// A descrição quebra em linhas seguintes que NÃO começam com código; elas são
// coladas na descrição do item aberto. Uma dessas continuações pode trazer a
// marca de cancelamento — por isso a marca é procurada no item inteiro, e não
// só na primeira linha.
func itensDoDAV(texto string) []Item {
	var itens []Item
	var atual *Item
	var cru strings.Builder

	fechar := func() {
		if atual == nil {
			return
		}
		if !doCancelado.MatchString(cru.String()) {
			itens = append(itens, *atual)
		}
		atual, cru = nil, strings.Builder{}
	}

	for _, linha := range strings.Split(texto, "\n") {
		m := daLinhaDeItem.FindStringSubmatch(linha)
		if m == nil {
			if atual != nil {
				if paraDeLerItens(linha) {
					fechar()
					continue
				}
				cru.WriteString(" " + linha)
				atual.Descricao = Enxugar(atual.Descricao + " " + soPalavras(linha))
			}
			continue
		}
		fechar()

		codigo, resto := m[1], m[2]
		numeros := doDecimal.FindAllString(resto, -1)
		// Sem pelo menos quantidade e total, a linha não é item: é rodapé com
		// número de documento, ou lixo do OCR.
		if len(numeros) < 3 {
			continue
		}
		qtd, _ := Decimal(numeros[0])
		unit, _ := Decimal(numeros[1])
		total, _ := Decimal(numeros[len(numeros)-1])
		if total <= 0 {
			continue
		}
		it := Item{
			Codigo:     strings.TrimLeft(codigo, "0"),
			Descricao:  Enxugar(soPalavras(resto)),
			Quantidade: qtd,
			Unitario:   unit,
			Total:      total,
		}
		atual = &it
		cru.Reset()
		cru.WriteString(linha)
	}
	fechar()
	return itens
}

// paraDeLerItens reconhece o rodapé — dali para baixo não há mais produto.
func paraDeLerItens(linha string) bool {
	l := strings.ToLower(linha)
	return strings.Contains(l, "total bruto") ||
		strings.Contains(l, "total a pagar") ||
		strings.Contains(l, "plano de pagamento") ||
		strings.Contains(l, "dados complementares")
}

// soPalavras tira da descrição a parte numérica das colunas.
func soPalavras(s string) string {
	var b strings.Builder
	for _, campo := range strings.Fields(s) {
		if temDigito(campo) || ehUnidade(campo) || strings.HasSuffix(campo, "%") {
			// A descrição de verdade acabou quando começam as colunas.
			break
		}
		b.WriteString(campo + " ")
	}
	saida := strings.TrimSpace(b.String())
	if saida == "" {
		return Enxugar(s)
	}
	return saida
}

func temDigito(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			return true
		}
	}
	return false
}

var unidades = map[string]bool{"UN": true, "UM": true, "M": true, "KG": true, "PC": true,
	"CX": true, "L": true, "MT": true, "PT": true, "SC": true, "JG": true, "PÇ": true}

func ehUnidade(s string) bool { return unidades[strings.ToUpper(s)] }

// ---------------------------------------------------------------------------
// a trava aritmética
// ---------------------------------------------------------------------------

// ContaFecha diz se a leitura se prova sozinha.
//
// É ESTA FUNÇÃO QUE DECIDE SE A IA É CHAMADA
//
//	A confiança do OCR não serve para isso: o Tesseract devolve 19960 no lugar
//	de 18860 com a mesma cara de quem acertou. Mas nota fiscal tem conferência
//	embutida, e ela é aritmética — a soma dos itens tem que dar o total. Uma
//	leitura que erra um dígito num item e mesmo assim fecha com o total é
//	coincidência que não acontece.
//
//	Um por cento de tolerância cobre arredondamento e desconto de subtotal sem
//	deixar passar item inventado.
func ContaFecha(l *Leitura) bool {
	if l == nil || l.ValorTotal <= 0 || len(l.Itens) == 0 {
		return false
	}
	var soma float64
	for _, it := range l.Itens {
		if it.Total <= 0 {
			return false
		}
		soma += it.Total
	}
	dif := soma - l.ValorTotal
	if dif < 0 {
		dif = -dif
	}
	return dif/l.ValorTotal <= 0.01
}

// TicketDaObservacao devolve o único ticket escrito na observação.
//
// POR QUE "O ÚNICO", E NÃO "OS TICKETS"
//
//	Porque duas leituras diferentes deste campo têm que poder ser comparadas, e
//	comparar listas de tamanhos diferentes não diz nada. Observação com mais de
//	um número de ticket não é caso de confiar em OCR: é caso de pessoa olhar.
func TicketDaObservacao(observacao string) (int, bool) {
	achados := Tickets(observacao)
	if len(achados) != 1 {
		return 0, false
	}
	return achados[0], true
}

// TicketConfiavel diz se dá para amarrar esta leitura a um chamado sozinho.
//
// Duas condições, e as duas são sobre a PROCEDÊNCIA do número, não sobre a
// confiança do OCR:
//
//  1. saiu do CAMPO de observação — onde o fornecedor digita o ticket — e não
//     de um lugar qualquer da página, onde pode ser rabisco à mão
//  2. o campo tem UM ticket, não dois nem nenhum
//
// Quem chama ainda precisa conferir que o número existe na base de chamados.
// Este é o primeiro filtro, não o único.
func TicketConfiavel(l *Leitura) (int, bool) {
	if l == nil || !l.ObservacaoDoCampo {
		return 0, false
	}
	return TicketDaObservacao(l.Observacao)
}
