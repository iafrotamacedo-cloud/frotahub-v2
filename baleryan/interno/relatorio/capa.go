// rev 1 — a capa da planilha: a faixa escura com a marca
//
// O QUE ELA É
//
//	O cabeçalho gráfico que abre o arquivo que vai AO CLIENTE: faixa preta com a
//	marca à esquerda, título, período e o total à direita, fechada por um fio
//	vermelho. Da linha do cabeçalho da tabela para baixo é tudo claro.
//
// POR QUE A FRONTEIRA PASSA EXATAMENTE AÍ
//
//	É a mesma regra das telas, dita pelo dono em 25/08/2026: *"Todos os MENUS em
//	tema escuro. O tema claro fica para a exibição das LISTAS."* Aqui a faixa é
//	o "menu" do documento — quem fez, quando, de quando a quando, quanto — e a
//	tabela é a lista. Trezentas linhas em fundo escuro cansam a vista de quem
//	confere; a faixa escura faz o total saltar em vez de competir com o branco.
//
// TABELA SEM CAPA CONTINUA COMO ERA
//
//	`Tabela.Capa` nil significa a planilha crua de sempre — cabeçalho na linha 1,
//	dados a partir da 2. As outras extrações do sistema não mudaram de aparência
//	por causa desta.
//
// A POSIÇÃO DA MARCA É MEDIDA, NÃO CHUTADA
//
//	Era esse o defeito do modelo antigo: a âncora dizia 1,5" × 0,67" para uma
//	imagem 2:1, e o Excel esticava. Aqui a caixa da marca sai da soma das
//	larguras REAIS das primeiras colunas e das alturas REAIS das linhas da
//	faixa; a imagem é encaixada dentro dela mantendo a proporção do arquivo e
//	centrada nos dois eixos. Mudar a largura de uma coluna reposiciona a marca
//	sozinho.
package relatorio

import (
	_ "embed"
	"fmt"
	"strings"
)

// A marca da casa, o MESMO arquivo que o front serve em /marca.png.
//
// SÃO DUAS CÓPIAS, E EXISTE UM TESTE PARA ISSO
//
//	`go:embed` não atravessa a raiz do módulo, então o arquivo do `web/` não
//	pode ser embutido daqui. A cópia é obrigatória; o que não pode é ela
//	divergir em silêncio, e é o que `TestAMarcaEmbutidaEAMesmaDoFront` guarda.
//
//go:embed marca.png
var marcaPNG []byte

// MarcaEmbutida devolve os bytes da marca. Existe para o teste poder compará-la
// com o arquivo do front.
func MarcaEmbutida() []byte { return marcaPNG }

// As proporções do arquivo da marca. Elas entram na conta da âncora; se a
// imagem for trocada por outra de proporção diferente, este par muda junto — e
// o teste da proporção acusa se não mudar.
const (
	marcaLargura = 710
	marcaAltura  = 640
)

// Capa é o cabeçalho gráfico da planilha. Nil = planilha crua.
type Capa struct {
	// Chapeu é a linha pequena e vermelha embaixo do título: quem assina o
	// documento e sob qual contrato.
	Chapeu string
	// Periodo é a janela que a pessoa escolheu, escrita para ser lida. É o
	// FILTRO, não o intervalo dos dados: se nenhum registro caiu no último dia,
	// o cabeçalho ainda diz até onde ela mandou olhar.
	Periodo string
	// Assinatura é o "Gerado em … por … através do FrotaHub®".
	Assinatura string
	// Resumo e Destaque ocupam a direita da faixa: a contagem e o total.
	Resumo   string
	Destaque string
}

// As cores, tiradas do tema escuro do front (`web/src/estilos/escuro.css`).
//
// A planilha que sai do FrotaHub tem que parecer que saiu do FrotaHub. Os
// valores estão repetidos aqui porque Go não lê CSS — e é por isso que cada um
// carrega o nome da variável de origem ao lado.
const (
	corFaixa    = "FF100D0E" // --d-bg
	corTitulo   = "FFEFEBEC" // --d-txt
	corApagada  = "FFB9B1B3" // --d-txt2
	corRealce   = "FFD2494C" // --red-lift, o vermelho clareado que se lê no preto
	corMarca    = "FFA11F22" // o vermelho da marca, no fio que fecha a faixa
	corCasa     = "FF7A1517" // o vermelho da casa, no cabeçalho da tabela
	corTinta    = "FF26201F" // o texto dos dados
	corContador = "FF9A8F90" // a coluna Nº, que não deve competir com o conteúdo
	corZebra    = "FFFAF6F6"
	corFio      = "FFE8DFDF"
	corTotal    = "FFF4EDED"
	corAviso    = "FFFDF3F3"
)

// colunaDoTexto é onde o bloco de texto da faixa começa: a coluna C, 0-based 2.
// As duas primeiras ficam para a marca — é o mesmo desenho do modelo que o
// cliente já recebe.
const colunaDoTexto = 2

// disposicao é o mapa de linhas da planilha. TODO índice de linha sai daqui:
// número de linha escrito à mão em três lugares é como o cabeçalho e os dados
// se desencontram no dia em que a faixa ganha uma linha.
type disposicao struct {
	faixaDe, faixaAte int // a faixa escura
	aviso             int // dentro da faixa, quando existe; 0 = não existe
	fio               int // o fio vermelho que fecha a faixa
	respiro           int // a linha branca entre a faixa e a tabela
	cabecalho         int // o cabeçalho da tabela
	primeira, ultima  int // os dados
	total             int // a linha de total; 0 = não existe
}

// dispor monta o mapa. Sem capa, é o desenho de sempre.
func (t Tabela) dispor() disposicao {
	if t.Capa == nil {
		return disposicao{cabecalho: 1, primeira: 2, ultima: 1 + len(t.Linhas)}
	}
	d := disposicao{faixaDe: 1, faixaAte: 4}
	if t.Aviso != "" {
		// O AVISO DE CORTE MORA NO ALTO, NÃO NO PÉ
		//   Ele é a frase mais importante de um documento cortado: diz que o
		//   que está ali não é tudo. No pé de sete páginas, ninguém lê.
		d.faixaAte = 5
		d.aviso = 5
	}
	d.fio = d.faixaAte + 1
	d.respiro = d.fio + 1
	d.cabecalho = d.respiro + 1
	d.primeira = d.cabecalho + 1
	d.ultima = d.primeira + len(t.Linhas) - 1
	if len(t.Linhas) == 0 {
		d.ultima = d.primeira
	}
	d.total = d.ultima + 1
	return d
}

// alturas devolve a altura, em pontos, de cada linha da capa.
func (t Tabela) alturas(d disposicao) map[int]float64 {
	a := map[int]float64{
		d.faixaDe:     30, // o título
		d.faixaDe + 1: 21, // o chapéu
		d.faixaDe + 2: 15, // o período
		d.faixaDe + 3: 17, // a assinatura e o total
		d.fio:         4.5,
		d.respiro:     9,
		d.cabecalho:   22,
	}
	if d.aviso > 0 {
		a[d.aviso] = 17
	}
	return a
}

// larguras traduz o peso de cada coluna em largura de caractere do Excel.
//
// É a MESMA conta de antes da capa: um peso só governa o PDF e a planilha, e
// largura separada por formato seriam duas verdades sobre a mesma coluna.
func (t Tabela) larguras() []float64 {
	w := make([]float64, len(t.Colunas))
	for i, c := range t.Colunas {
		l := c.Peso * 1.6
		if l < 8 {
			l = 8
		}
		w[i] = l
	}
	return w
}

// As unidades do OOXML. O Excel mede posição de imagem em EMU — 914.400 por
// polegada, 9.525 por pixel de tela, 12.700 por ponto tipográfico.
const (
	emuPorPixel = 9525
	emuPorPonto = 12700
)

// pixelsDaColuna converte a largura em caracteres na largura em pixels que o
// Excel realmente usa para a fonte padrão.
func pixelsDaColuna(largura float64) int { return int(largura*7) + 5 }

// caixaDaMarca calcula onde a imagem entra, em EMU: deslocamento a partir de A1
// e tamanho. A imagem é encaixada na caixa formada pelas colunas que sobram à
// esquerda do texto e pelas linhas da faixa, com folga, mantendo a proporção.
func (t Tabela) caixaDaMarca(d disposicao) (colOff, rowOff, cx, cy int) {
	larguras := t.larguras()
	ate := colunaDoTexto
	if ate > len(larguras) {
		ate = len(larguras)
	}
	var caixaLarg int
	for i := 0; i < ate; i++ {
		caixaLarg += pixelsDaColuna(larguras[i]) * emuPorPixel
	}
	alturas := t.alturas(d)
	var caixaAlt int
	for r := d.faixaDe; r <= d.faixaAte; r++ {
		caixaAlt += int(alturas[r] * emuPorPonto)
	}

	// A altura manda; a largura corrige se a marca encostar nas bordas. Os dois
	// fatores são a folga: a marca respira dentro da faixa em vez de tocá-la.
	cy = int(float64(caixaAlt) * 0.58)
	cx = cy * marcaLargura / marcaAltura
	if limite := int(float64(caixaLarg) * 0.72); cx > limite {
		cx = limite
		cy = cx * marcaAltura / marcaLargura
	}
	return (caixaLarg - cx) / 2, (caixaAlt - cy) / 2, cx, cy
}

// ---------------------------------------------------------------------------
// As partes extras do zip quando existe capa
// ---------------------------------------------------------------------------

func (t Tabela) desenho(d disposicao) string {
	colOff, rowOff, cx, cy := t.caixaDaMarca(d)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<xdr:oneCellAnchor>` +
		fmt.Sprintf(`<xdr:from><xdr:col>0</xdr:col><xdr:colOff>%d</xdr:colOff>`+
			`<xdr:row>0</xdr:row><xdr:rowOff>%d</xdr:rowOff></xdr:from>`, colOff, rowOff) +
		fmt.Sprintf(`<xdr:ext cx="%d" cy="%d"/>`, cx, cy) +
		`<xdr:pic><xdr:nvPicPr>` +
		`<xdr:cNvPr id="1" name="Marca" descr="Frota Macedo Engenharia"/>` +
		`<xdr:cNvPicPr><a:picLocks noChangeAspect="1"/></xdr:cNvPicPr>` +
		`</xdr:nvPicPr>` +
		`<xdr:blipFill>` +
		`<a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="rId1"/>` +
		`<a:stretch><a:fillRect/></a:stretch>` +
		`</xdr:blipFill>` +
		`<xdr:spPr><a:xfrm><a:off x="0" y="0"/>` +
		fmt.Sprintf(`<a:ext cx="%d" cy="%d"/></a:xfrm>`, cx, cy) +
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr>` +
		`</xdr:pic><xdr:clientData/></xdr:oneCellAnchor></xdr:wsDr>`
}

const relacoesDoDesenho = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/marca.png"/>
</Relationships>`

const relacoesDaFolha = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>
</Relationships>`

// tiposDeConteudoCom monta o [Content_Types].xml. Sem capa ele é o de sempre;
// com capa ganha o PNG e o desenho.
//
// UM TIPO FALTANDO NÃO DÁ AVISO
//
//	O Excel não diz "faltou declarar o png". Ele diz que o arquivo está
//	corrompido e se oferece para recuperá-lo — e a recuperação joga a imagem
//	fora em silêncio.
func tiposDeConteudoCom(capa bool) string {
	if !capa {
		return tiposDeConteudo
	}
	return strings.Replace(tiposDeConteudo,
		`<Default Extension="xml" ContentType="application/xml"/>`,
		`<Default Extension="xml" ContentType="application/xml"/>`+"\n"+
			`<Default Extension="png" ContentType="image/png"/>`+"\n"+
			`<Override PartName="/xl/drawings/drawing1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`,
		1)
}
