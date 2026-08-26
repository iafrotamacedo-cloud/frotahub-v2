// rev 1 — camada 2: o que dá para arrancar do texto sem inteligência nenhuma
//
// O texto chega do OCR. Esta camada não tenta ler a nota inteira — tenta pegar
// os campos que têm forma FIXA e não admitem interpretação:
//
//	chave de acesso   44 dígitos, e um dígito verificador que confere
//	CNPJ              14 dígitos, com verificador
//	valor total       vem rotulado
//	data de emissão   vem rotulada
//
// Os ITENS ficam para a camada 3. Tabela de DANFE em texto de OCR é o inferno
// dos regex: coluna que escorrega, descrição que quebra em duas linhas, ponto
// decimal que vira vírgula. Fingir que dá para pegar com expressão regular é
// como se produz um sistema que erra 5% das notas e ninguém sabe quais.
//
// O GANHO DE PARAR AQUI
//
//	A chave de acesso sozinha já resolve DUPLICIDADE — e duplicidade é o defeito
//	mais caro do sistema antigo. Mesmo quando a camada 3 precisa entrar, esta
//	aqui entrega de graça a identidade do documento.
package leitor

import (
	"regexp"
	"strings"
)

var (
	// A chave aparece pontuada de todo jeito: em blocos de quatro, com espaço,
	// com ponto. Pegamos o trecho e limpamos depois.
	daChave = regexp.MustCompile(`(?:\d[\s.\-]?){43}\d`)

	doCNPJ = regexp.MustCompile(`\d{2}[.\s]?\d{3}[.\s]?\d{3}[/\s]?\d{4}[-\s]?\d{2}`)

	doValorTotal = regexp.MustCompile(`(?i)valor\s+total\s+d[ao]\s+nota[^\d]{0,40}([\d.,]+)`)
	// O RÓTULO VEM DEPOIS DO VALOR, E NÃO ANTES
	//   Numa DANFE, "VALOR TOTAL DA NOTA" e o número dele estão em células
	//   vizinhas de uma tabela. Ao achatar o PDF em texto, o `pdftotext` põe o
	//   número numa linha e o rótulo na SEGUINTE. Medido no documento 17934: a
	//   linha 169 é "500,00" e a 170 é "VALOR TOTAL DA NOTA" — o regex que só
	//   olha para frente devolvia zero numa nota perfeitamente legível.
	doValorAntes = regexp.MustCompile(`(?im)^\s*([\d.,]+)\s*$\n^\s*valor\s+total\s+d[ao]\s+nota\s*$`)
	doValorNF    = regexp.MustCompile(`(?i)\bv(?:alor)?\.?\s*(?:total\s+)?nf\b[^\d]{0,20}([\d.,]+)`)
	doFrete      = regexp.MustCompile(`(?i)valor\s+d[oe]\s+frete[^\d]{0,40}([\d.,]+)`)

	daEmissao = regexp.MustCompile(`(?i)(?:data\s+d[ae]\s+emiss[ãa]o|emiss[ãa]o)[^\d]{0,20}(\d{2})[/.\-](\d{2})[/.\-](\d{4})`)
	doNumero  = regexp.MustCompile(`(?i)n[ºo°.]{0,2}\s*(\d{3}\.?\d{3}\.?\d{3})`)
	doDAV     = regexp.MustCompile(`(?i)\bdav\s*[:.\-]?\s*(\d{3,8})`)
)

// DoTexto tenta estruturar o texto de uma DANFE. Devolve o que conseguiu e um
// aviso de que a leitura está incompleta — nunca finge que terminou.
func DoTexto(texto string) *Leitura {
	l := &Leitura{Tipo: "nf", Camada: DoOCR}
	if texto == "" {
		return l
	}

	if m := daChave.FindString(texto); m != "" {
		if d := SoDigitos(m); len(d) == 44 && ChaveValida(d) {
			l.ChaveAcesso = d
			// A chave carrega o número da nota nas posições 25–33 e a série nas
			// 22–24. Melhor tirar dali do que caçar o rótulo no texto torto.
			l.Serie = strings.TrimLeft(d[22:25], "0")
			l.Numero = strings.TrimLeft(d[25:34], "0")
			l.EmitenteCNPJ = d[6:20]
		}
	}
	if l.Numero == "" {
		if m := doNumero.FindStringSubmatch(texto); m != nil {
			l.Numero = strings.TrimLeft(SoDigitos(m[1]), "0")
		}
	}
	if l.EmitenteCNPJ == "" {
		if m := doCNPJ.FindString(texto); m != "" {
			if d := SoDigitos(m); len(d) == 14 {
				l.EmitenteCNPJ = d
			}
		}
	}

	for _, re := range []*regexp.Regexp{doValorTotal, doValorAntes, doValorNF} {
		if m := re.FindStringSubmatch(texto); m != nil {
			if v, ok := Decimal(m[1]); ok && v > 0 {
				l.ValorTotal = v
				break
			}
		}
	}
	if m := doFrete.FindStringSubmatch(texto); m != nil {
		l.ValorFrete, _ = Decimal(m[1])
	}
	if m := daEmissao.FindStringSubmatch(texto); m != nil {
		l.Emissao = m[3] + "-" + m[2] + "-" + m[1]
	}
	if m := doDAV.FindStringSubmatch(texto); m != nil {
		l.DAV = m[1]
	}

	l.Observacao = Enxugar(texto)
	l.Arrumar()
	l.Confianca = confiancaDoTexto(l)
	return l
}

// confiancaDoTexto é deliberadamente pessimista.
//
// Nunca passa de 0,6 — porque esta camada NÃO leu os itens, e uma leitura sem
// itens não é uma nota lida. O número alto fica reservado para quem leu tudo.
func confiancaDoTexto(l *Leitura) float64 {
	c := 0.0
	if l.ChaveAcesso != "" {
		c += 0.35 // conferida pelo dígito verificador: é quase certeza
	}
	if l.ValorTotal > 0 {
		c += 0.15
	}
	if l.Emissao != "" {
		c += 0.05
	}
	if l.EmitenteCNPJ != "" {
		c += 0.05
	}
	return arredondarConfianca(c)
}

// ChaveValida confere o dígito verificador da chave de acesso (módulo 11).
//
// POR QUE CONFERIR
//
//	O OCR troca 8 por B, 1 por l, 0 por O. Uma chave lida errado é 44 dígitos
//	perfeitamente plausíveis apontando para uma nota que não existe — e, pior,
//	quebrando a trava de duplicidade justamente onde ela deveria funcionar. O
//	verificador custa dez linhas e transforma um palpite em certeza.
func ChaveValida(chave string) bool {
	if len(chave) != 44 {
		return false
	}
	pesos := []int{2, 3, 4, 5, 6, 7, 8, 9}
	soma, p := 0, 0
	for i := 42; i >= 0; i-- {
		d := int(chave[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		soma += d * pesos[p]
		p = (p + 1) % len(pesos)
	}
	resto := soma % 11
	dv := 11 - resto
	if dv >= 10 {
		dv = 0
	}
	return dv == int(chave[43]-'0')
}
