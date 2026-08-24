// rev 1 — o PDF, escrito à mão
//
// Um PDF é mais simples do que a fama: uma lista de objetos numerados, um fluxo
// de texto com comandos de posição, e uma tabela no fim dizendo em que byte cada
// objeto começa. Errar essa tabela é o único jeito de o arquivo não abrir — e é
// por isso que ela é montada por medição, e não por conta de cabeça.
//
// A FONTE NÃO VAI DENTRO DO ARQUIVO
//
//	O formato tem catorze fontes que todo leitor de PDF já tem, e a Helvetica é
//	uma delas. Usá-la dispensa embutir um arquivo de fonte — o que evita megabytes
//	e evita a discussão de licença. O preço é ficar no alfabeto ocidental, que é
//	exatamente onde a gente está.
//
// A LARGURA DO TEXTO É ESTIMADA, COM VIÉS
//
//	Medir texto de verdade exigiria a tabela de larguras da fonte. Aqui a conta é
//	por classe de caractere, e ela erra PARA CIMA de propósito: maiúscula conta
//	como 0,70 do tamanho, minúscula 0,52, dígito 0,556. Errando para cima, o
//	texto é cortado um pouco antes do necessário; errando para baixo, ele
//	invadiria a coluna vizinha. Entre as duas falhas, a primeira é a educada.
package relatorio

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// A4 deitada, em pontos.
const (
	larguraFolha = 842.0
	alturaFolha  = 595.0
	margem       = 28.0
	alturaLinha  = 14.0
	corpoFonte   = 7.5
)

// PDF devolve os bytes de um arquivo .pdf com a tabela paginada.
func (t Tabela) PDF() ([]byte, error) {
	if len(t.Colunas) == 0 {
		return nil, fmt.Errorf("relatório sem colunas")
	}
	larguras := t.largurasEmPontos()
	// O que sobra da folha depois do cabeçalho da página, do cabeçalho da tabela
	// e do rodapé, dividido pela altura da linha.
	sobra := alturaFolha - margem - 92 - 26
	porPagina := int(sobra / alturaLinha)
	if porPagina < 1 {
		porPagina = 1
	}
	paginas := (len(t.Linhas) + porPagina - 1) / porPagina
	if paginas == 0 {
		paginas = 1
	}

	fluxos := make([]string, 0, paginas)
	for p := 0; p < paginas; p++ {
		inicio := p * porPagina
		fim := inicio + porPagina
		if fim > len(t.Linhas) {
			fim = len(t.Linhas)
		}
		fluxos = append(fluxos, t.desenhar(inicio, fim, p+1, paginas, larguras))
	}
	return montar(fluxos), nil
}

// largurasEmPontos reparte a folha entre as colunas, na proporção dos pesos.
func (t Tabela) largurasEmPontos() []float64 {
	total := 0.0
	for _, c := range t.Colunas {
		total += c.Peso
	}
	util := larguraFolha - 2*margem
	larguras := make([]float64, len(t.Colunas))
	for i, c := range t.Colunas {
		larguras[i] = util * c.Peso / total
	}
	return larguras
}

func (t Tabela) desenhar(inicio, fim, pagina, paginas int, larguras []float64) string {
	var s strings.Builder
	y := alturaFolha - margem

	// ---- cabeçalho da página
	y -= 14
	texto(&s, "F2", 13, margem, y, "0.118 0.133 0.153", t.Titulo)
	direita(&s, "F1", 8, larguraFolha-margem, y+2, "0.41 0.44 0.47",
		fmt.Sprintf("Página %d de %d", pagina, paginas))

	if t.Subtitulo != "" {
		y -= 12
		texto(&s, "F1", 8, margem, y, "0.41 0.44 0.47", t.Subtitulo)
	}
	y -= 8
	retangulo(&s, margem, y, larguraFolha-2*margem, 1, "0.631 0.122 0.133")

	// ---- cabeçalho da tabela
	y -= 18
	retangulo(&s, margem, y-4, larguraFolha-2*margem, 17, "0.118 0.133 0.153")
	x := margem
	for i, c := range t.Colunas {
		alinhaDireita := c.Tipo == Numero || c.Tipo == Dinheiro
		escreverNaColuna(&s, "F2", 7, x, y+0.5, larguras[i], strings.ToUpper(c.Titulo), "1 1 1", alinhaDireita)
		x += larguras[i]
	}

	// ---- as linhas
	y -= 4
	for l := inicio; l < fim; l++ {
		y -= alturaLinha
		if (l-inicio)%2 == 1 {
			retangulo(&s, margem, y-3.5, larguraFolha-2*margem, alturaLinha, "0.976 0.980 0.984")
		}
		x = margem
		for c, col := range t.Colunas {
			alinhaDireita := col.Tipo == Numero || col.Tipo == Dinheiro
			escreverNaColuna(&s, "F1", corpoFonte, x, y, larguras[c],
				emTexto(t.valor(l, c), col.Tipo), "0.204 0.227 0.259", alinhaDireita)
			x += larguras[c]
		}
		retangulo(&s, margem, y-4, larguraFolha-2*margem, 0.4, "0.898 0.910 0.925")
	}

	// ---- rodapé
	rodape := "FrotaHub · gerado em " + t.Gerado.In(FusoDaCasa()).Format("02/01/2006 15:04")
	texto(&s, "F1", 7, margem, margem-4, "0.54 0.57 0.60", rodape)
	if t.Aviso != "" && pagina == paginas {
		direita(&s, "F2", 7, larguraFolha-margem, margem-4, "0.631 0.122 0.133", t.Aviso)
	}
	return s.String()
}

func emTexto(v any, tipo Tipo) string {
	if v == nil {
		return ""
	}
	switch tipo {
	case Dinheiro:
		if n, ok := comoNumero(v); ok {
			if n == 0 {
				return "—"
			}
			return emMoeda(n)
		}
	case Data:
		if d, ok := v.(time.Time); ok && !d.IsZero() {
			return d.In(FusoDaCasa()).Format("02/01/2006")
		}
		return "—"
	case DataHora:
		if d, ok := v.(time.Time); ok && !d.IsZero() {
			return d.In(FusoDaCasa()).Format("02/01/2006 15:04")
		}
		return "—"
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" {
		return "—"
	}
	return strings.Join(strings.Fields(s), " ")
}

// emMoeda escreve no padrão daqui: ponto no milhar, vírgula no centavo.
func emMoeda(n float64) string {
	s := fmt.Sprintf("%.2f", n)
	inteiro, centavos, _ := strings.Cut(s, ".")
	negativo := strings.HasPrefix(inteiro, "-")
	inteiro = strings.TrimPrefix(inteiro, "-")
	var partes []string
	for len(inteiro) > 3 {
		partes = append([]string{inteiro[len(inteiro)-3:]}, partes...)
		inteiro = inteiro[:len(inteiro)-3]
	}
	partes = append([]string{inteiro}, partes...)
	saida := strings.Join(partes, ".") + "," + centavos
	if negativo {
		return "-" + saida
	}
	return saida
}

// ---------------------------------------------------------------------------
// Desenho
// ---------------------------------------------------------------------------

func escreverNaColuna(s *strings.Builder, fonte string, tam, x, y, largura float64, txt, cor string, aDireita bool) {
	txt = cortar(txt, largura-8, tam)
	if aDireita {
		direita(s, fonte, tam, x+largura-4, y, cor, txt)
		return
	}
	texto(s, fonte, tam, x+4, y, cor, txt)
}

func texto(s *strings.Builder, fonte string, tam, x, y float64, cor, txt string) {
	if txt == "" {
		return
	}
	fmt.Fprintf(s, "BT %s rg /%s %.1f Tf %.2f %.2f Td (%s) Tj ET\n", cor, fonte, tam, x, y, escapar(txt))
}

func direita(s *strings.Builder, fonte string, tam, xFim, y float64, cor, txt string) {
	texto(s, fonte, tam, xFim-largura(txt, tam), y, cor, txt)
}

func retangulo(s *strings.Builder, x, y, w, h float64, cor string) {
	fmt.Fprintf(s, "%s rg %.2f %.2f %.2f %.2f re f\n", cor, x, y, w, h)
}

// largura estima quanto o texto ocupa. Erra para cima — ver o comentário do topo.
func largura(txt string, tam float64) float64 {
	var w float64
	for _, r := range txt {
		switch {
		case r == ' ':
			w += 0.278
		case r >= '0' && r <= '9':
			w += 0.556
		case r >= 'A' && r <= 'Z':
			w += 0.70
		case r >= 'a' && r <= 'z':
			w += 0.52
		default:
			w += 0.58
		}
	}
	return w * tam
}

func cortar(txt string, disponivel, tam float64) string {
	if disponivel <= 0 || largura(txt, tam) <= disponivel {
		return txt
	}
	runas := []rune(txt)
	for len(runas) > 1 {
		runas = runas[:len(runas)-1]
		if largura(string(runas)+"…", tam) <= disponivel {
			return string(runas) + "…"
		}
	}
	return ""
}

// escapar prepara o texto para o fluxo do PDF: os três caracteres reservados
// viram sequências de escape, e o resto vai em WinAnsi — que é Latin-1 para tudo
// que interessa aqui, acentos do português inclusive. O que não couber vira "?",
// e não um byte inválido que faz o leitor recusar a página.
func escapar(txt string) string {
	var b strings.Builder
	for _, r := range txt {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		case '…':
			b.WriteString(`\205`) // reticências, em WinAnsi
		case '—', '–':
			b.WriteString(`\226`)
		default:
			switch {
			case r < 32:
				b.WriteByte(' ')
			case r < 127:
				// ASCII imprimível vai cru: em octal, o fluxo ficaria quatro
				// vezes maior sem nenhum ganho.
				b.WriteRune(r)
			case r < 256:
				fmt.Fprintf(&b, `\%03o`, r)
			default:
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// A montagem do arquivo
// ---------------------------------------------------------------------------

func montar(fluxos []string) []byte {
	// Numeração: 1 catálogo, 2 páginas, 3 e 4 as fontes, e daí em diante um par
	// (página, conteúdo) para cada folha.
	const primeiraPagina = 5
	objetos := make([]string, 0, 4+2*len(fluxos))

	var filhos []string
	for i := range fluxos {
		filhos = append(filhos, fmt.Sprintf("%d 0 R", primeiraPagina+i*2))
	}

	objetos = append(objetos,
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(filhos, " "), len(fluxos)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
	)

	for i, fluxo := range fluxos {
		objetos = append(objetos, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] "+
				"/Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",
			larguraFolha, alturaFolha, primeiraPagina+i*2+1))
		objetos = append(objetos, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(fluxo), fluxo))
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	// Um comentário com bytes altos avisa aos leitores que o arquivo é binário.
	buf.Write([]byte{'%', 0xE2, 0xE3, 0xCF, 0xD3, '\n'})

	posicoes := make([]int, len(objetos)+1)
	for i, o := range objetos {
		posicoes[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}

	inicioTabela := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objetos)+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objetos); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", posicoes[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objetos)+1, inicioTabela)
	return buf.Bytes()
}
