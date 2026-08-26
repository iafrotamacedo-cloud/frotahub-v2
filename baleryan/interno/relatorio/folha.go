// rev 1 — a folha em branco, para documentos que não são tabela
//
// POR QUE ISTO EXISTE AO LADO DE `Tabela`
//
//	`Tabela` desenha uma listagem: cabeçalho, linhas, paginação. É o certo para
//	uma extração de planilha, e foi usada — por erro meu — para o ORÇAMENTO que
//	vai ao cliente. O resultado era o que se esperaria: A4 deitada, cabeçalho de
//	relatório, rodapé "gerado em". Um despejo de dados onde deveria haver um
//	documento comercial.
//
//	Um orçamento tem blocos, faixas, quadros lado a lado, valor por extenso e
//	linha de aceite. Nada disso cabe numa tabela paginada, e forçar caberia só
//	deformando as duas coisas.
//
//	Então: `Tabela` continua sendo a listagem, intacta. `Folha` é papel em
//	branco com régua — quem desenha em cima é quem sabe o que o documento diz.
//	As duas dividem o encanamento: escapar texto, medir largura, montar o
//	arquivo. Uma implementação de PDF só (CORE-06).
package relatorio

import (
	"bytes"
	"fmt"
	"strings"
)

// A4 em pé, em pontos. A deitada continua em `pdf.go`, para a `Tabela`.
const (
	LarguraRetrato = 595.0
	AlturaRetrato  = 842.0
)

// Cinzas e azuis do documento. Ficam nomeados porque a mesma cor aparece em
// cinco lugares — e cor repetida à mão é cor que um dia diverge.
const (
	CorTexto  = "0.13 0.15 0.18"
	CorMuda   = "0.42 0.45 0.50"
	CorClara  = "0.62 0.65 0.70"
	CorFaixa  = "0.16 0.22 0.34" // o azul-marinho das faixas
	CorFundo  = "0.94 0.95 0.96"
	CorLinha  = "0.85 0.87 0.89"
	CorBranco = "1 1 1"
)

// Folha acumula o desenho de uma ou mais páginas do mesmo tamanho.
type Folha struct {
	Largura, Altura float64

	paginas []string
	atual   strings.Builder
	imagens []imagem
}

type imagem struct {
	nome  string
	jpeg  []byte
	larg  int
	alt   int
	cinza bool
}

// NovaFolha abre um documento em pé.
func NovaFolha() *Folha {
	return &Folha{Largura: LarguraRetrato, Altura: AlturaRetrato}
}

// Pagina fecha a página corrente e começa outra.
func (f *Folha) Pagina() {
	f.paginas = append(f.paginas, f.atual.String())
	f.atual.Reset()
}

// Texto escreve a partir de x. `y` é medido do PÉ da folha, como o PDF conta.
func (f *Folha) Texto(x, y, tam float64, negrito bool, cor, txt string) {
	texto(&f.atual, fonteDe(negrito), tam, x, y, cor, txt)
}

// Direita escreve terminando em xFim — para colunas de dinheiro, onde o que
// alinha é a última casa.
func (f *Folha) Direita(xFim, y, tam float64, negrito bool, cor, txt string) {
	direita(&f.atual, fonteDe(negrito), tam, xFim, y, cor, txt)
}

// TextoCortado escreve sem invadir a coluna vizinha.
func (f *Folha) TextoCortado(x, y, tam, disponivel float64, negrito bool, cor, txt string) {
	texto(&f.atual, fonteDe(negrito), tam, x, y, cor, cortar(txt, disponivel, tam))
}

// Caixa pinta um retângulo cheio.
func (f *Folha) Caixa(x, y, w, h float64, cor string) {
	retangulo(&f.atual, x, y, w, h, cor)
}

// Moldura desenha só o contorno. Espessura fixa de meio ponto: é o traço que
// some no papel e aparece na tela, que é onde este documento vive.
func (f *Folha) Moldura(x, y, w, h float64, cor string) {
	fmt.Fprintf(&f.atual, "%s RG 0.5 w %.2f %.2f %.2f %.2f re S\n", cor, x, y, w, h)
}

// Linha desenha um traço horizontal.
func (f *Folha) Linha(x1, x2, y float64, cor string) {
	fmt.Fprintf(&f.atual, "%s RG 0.5 w %.2f %.2f m %.2f %.2f l S\n", cor, x1, y, x2, y)
}

// Medir devolve a largura estimada do texto, para quem precisa centralizar.
func (f *Folha) Medir(txt string, tam float64) float64 { return largura(txt, tam) }

// Imagem embute um JPEG **baseline** e o desenha na posição dada.
//
// POR QUE SÓ JPEG, E POR QUE ISSO BASTA
//
//	O PDF aceita um JPEG inteiro como está: o filtro `DCTDecode` diz ao leitor
//	"estes bytes já são um JPEG, decodifique você". Não há o que converter, e
//	por isso o arquivo não cresce nem um byte além da própria imagem.
//
//	Um PNG exigiria descompactar o zlib, desfazer os filtros por linha e
//	recompactar — código de imagem dentro de um gerador de documento, para
//	guardar a mesma marca. A logo da casa é um JPEG de 3,8 KB; a troca não se
//	justifica.
func (f *Folha) Imagem(x, y, w, h float64, jpeg []byte) error {
	larg, alt, comp, err := medirJPEG(jpeg)
	if err != nil {
		return err
	}
	nome := fmt.Sprintf("Im%d", len(f.imagens)+1)
	f.imagens = append(f.imagens, imagem{nome: nome, jpeg: jpeg, larg: larg, alt: alt, cinza: comp == 1})
	// `cm` põe a matriz de desenho no lugar e na escala; `q`/`Q` guardam e
	// devolvem o estado, para o resto da página não herdar a transformação.
	fmt.Fprintf(&f.atual, "q %.2f 0 0 %.2f %.2f %.2f cm /%s Do Q\n", w, h, x, y, nome)
	return nil
}

// PDF fecha o documento.
func (f *Folha) PDF() ([]byte, error) {
	if f.atual.Len() > 0 {
		f.Pagina()
	}
	if len(f.paginas) == 0 {
		return nil, fmt.Errorf("documento sem página")
	}
	return f.montar(), nil
}

func fonteDe(negrito bool) string {
	if negrito {
		return "F2"
	}
	return "F1"
}

// medirJPEG lê largura, altura e número de componentes do cabeçalho.
//
// Precisa dos três: o PDF exige declarar as dimensões e o espaço de cor do
// objeto, e errar o espaço de cor faz a imagem sair com as cores trocadas em
// vez de falhar — o pior tipo de erro, o que parece que funcionou.
func medirJPEG(d []byte) (larg, alt, componentes int, err error) {
	if len(d) < 4 || d[0] != 0xFF || d[1] != 0xD8 {
		return 0, 0, 0, fmt.Errorf("isto não é um JPEG")
	}
	for i := 2; i+9 < len(d); {
		if d[i] != 0xFF {
			i++
			continue
		}
		marca := d[i+1]
		switch {
		case marca == 0xD8 || marca == 0xD9 || (marca >= 0xD0 && marca <= 0xD7):
			i += 2
			continue
		// SOF0/1/2 — baseline e progressivo. Os outros SOF (arit., lossless)
		// o `DCTDecode` não garante, e é melhor recusar do que gerar um arquivo
		// que abre em um leitor e não em outro.
		case marca == 0xC0 || marca == 0xC1 || marca == 0xC2:
			alt = int(d[i+5])<<8 | int(d[i+6])
			larg = int(d[i+7])<<8 | int(d[i+8])
			componentes = int(d[i+9])
			if larg == 0 || alt == 0 {
				return 0, 0, 0, fmt.Errorf("JPEG sem dimensão")
			}
			return larg, alt, componentes, nil
		default:
			i += 2 + (int(d[i+2])<<8 | int(d[i+3]))
		}
	}
	return 0, 0, 0, fmt.Errorf("JPEG sem cabeçalho de dimensão")
}

// montar escreve os objetos e a tabela de posições.
//
// É primo do `montar` de `pdf.go` e não o mesmo porque aqui há imagens e folha
// em pé. Fundir os dois exigiria um terceiro com condicionais nos dois eixos —
// mais difícil de ler que as duas versões, e é a tabela de bytes do fim que não
// perdoa engano.
func (f *Folha) montar() []byte {
	// 1 catálogo · 2 páginas · 3 e 4 fontes · depois as imagens · depois os
	// pares (página, conteúdo).
	primeiraImagem := 5
	primeiraPagina := primeiraImagem + len(f.imagens)

	var recursos strings.Builder
	recursos.WriteString("/Font << /F1 3 0 R /F2 4 0 R >>")
	if len(f.imagens) > 0 {
		recursos.WriteString(" /XObject <<")
		for i, im := range f.imagens {
			fmt.Fprintf(&recursos, " /%s %d 0 R", im.nome, primeiraImagem+i)
		}
		recursos.WriteString(" >>")
	}

	filhos := make([]string, 0, len(f.paginas))
	for i := range f.paginas {
		filhos = append(filhos, fmt.Sprintf("%d 0 R", primeiraPagina+i*2))
	}

	objetos := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(filhos, " "), len(f.paginas)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
	}
	for _, im := range f.imagens {
		cor := "/DeviceRGB"
		if im.cinza {
			cor = "/DeviceGray"
		}
		objetos = append(objetos, fmt.Sprintf(
			"<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s "+
				"/BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n%s\nendstream",
			im.larg, im.alt, cor, len(im.jpeg), im.jpeg))
	}
	for i, fluxo := range f.paginas {
		objetos = append(objetos, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << %s >> /Contents %d 0 R >>",
			f.Largura, f.Altura, recursos.String(), primeiraPagina+i*2+1))
		objetos = append(objetos, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(fluxo), fluxo))
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
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
