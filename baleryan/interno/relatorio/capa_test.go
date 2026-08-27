package relatorio

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// as ferramentas
// ---------------------------------------------------------------------------

func tabelaDoModelo(linhas int) Tabela {
	fuso := FusoDaCasa()
	cols := []Coluna{
		{Titulo: "Nº", Peso: 5, Tipo: Numero},
		{Titulo: "TICKET", Peso: 5, Tipo: Numero},
		{Titulo: "LOJA", Peso: 24, Tipo: Texto},
		{Titulo: "VALOR", Peso: 8, Tipo: Dinheiro},
		{Titulo: "DATA", Peso: 9, Tipo: Data},
		{Titulo: "ORÇAMENTO", Peso: 8, Tipo: Dinheiro},
		{Titulo: "CONTA", Peso: 26, Tipo: Texto},
		{Titulo: "PCO", Peso: 14, Tipo: Texto},
	}
	var corpo [][]any
	for i := 0; i < linhas; i++ {
		v := 100.0 + float64(i)
		corpo = append(corpo, []any{
			i + 1, 126498 + i, "LJ-13 - 13.2(SANTOS DUMMONT)", v,
			time.Date(2026, 7, 13, 0, 0, 0, 0, fuso), v,
			"PREDIAL CIVIL - MANUENÇÃO CORRETIVA", "",
		})
	}
	return Tabela{
		Titulo:  "CUSTOS DE MATERIAIS DOS CHAMADOS DE MANUTENÇÃO",
		Aba:     "Orçamentos",
		Colunas: cols,
		Linhas:  corpo,
		Gerado:  time.Date(2026, 8, 27, 9, 0, 0, 0, fuso),
		Capa: &Capa{
			Chapeu:     "FROTA MACEDO ENGENHARIA  ·  CONTRATO DE MANUTENÇÃO PREDIAL",
			Periodo:    "Orçamentos lançados e ainda não cobrados  ·  13/07/2026 a 14/08/2026",
			Assinatura: "Gerado em 27/08/2026 por Francisco Jailton Barbosa Silva através do FrotaHub®",
			Resumo:     fmt.Sprintf("%d orçamentos", linhas),
			Destaque:   "R$ 1.234,56",
		},
	}
}

// abrir devolve as partes do .xlsx por nome.
func abrir(t *testing.T, tab Tabela) map[string]string {
	t.Helper()
	b, err := tab.Planilha()
	if err != nil {
		t.Fatalf("não montou a planilha: %v", err)
	}
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("o .xlsx não é um zip válido: %v", err)
	}
	partes := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatalf("não abriu %s: %v", f.Name, err)
		}
		dados, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatalf("não leu %s: %v", f.Name, err)
		}
		partes[f.Name] = string(dados)
	}
	return partes
}

// ---------------------------------------------------------------------------
// TODA PARTE DO ARQUIVO É XML QUE ABRE
//
//	O Excel não diz qual parte estava malformada: diz que o arquivo está
//	corrompido e se oferece para recuperá-lo. A recuperação joga fora o que não
//	entendeu — em silêncio, e geralmente é a marca.
// ---------------------------------------------------------------------------

func TestTodaParteDaPlanilhaEXMLQueAbre(t *testing.T) {
	for nome, corpo := range abrir(t, tabelaDoModelo(5)) {
		if !strings.HasSuffix(nome, ".xml") && !strings.HasSuffix(nome, ".rels") {
			continue
		}
		if err := xml.Unmarshal([]byte(corpo), new(any)); err != nil {
			t.Errorf("%s não é XML válido: %v", nome, err)
		}
	}
}

// A PLANILHA COM CAPA TRAZ AS QUATRO PARTES DA MARCA
//
//	Desenho, relação do desenho com a imagem, relação da folha com o desenho, e
//	a imagem. Faltando uma, o arquivo abre "recuperado" e sem a marca.
func TestACapaTrazTodasAsPartesDaMarca(t *testing.T) {
	partes := abrir(t, tabelaDoModelo(3))
	for _, p := range []string{
		"xl/drawings/drawing1.xml",
		"xl/drawings/_rels/drawing1.xml.rels",
		"xl/worksheets/_rels/sheet1.xml.rels",
		"xl/media/marca.png",
	} {
		if _, tem := partes[p]; !tem {
			t.Errorf("faltou %s — o Excel abre o arquivo como recuperado e a marca some", p)
		}
	}
	// O tipo do PNG tem que estar declarado, senão a imagem é descartada.
	if !strings.Contains(partes["[Content_Types].xml"], `Extension="png"`) {
		t.Error("o [Content_Types].xml não declara o png — a marca é jogada fora na abertura")
	}
	if !strings.Contains(partes["[Content_Types].xml"], "drawing1.xml") {
		t.Error("o [Content_Types].xml não declara o desenho")
	}
	if !bytes.Equal([]byte(partes["xl/media/marca.png"]), marcaPNG) {
		t.Error("a imagem gravada não é a marca embutida")
	}
}

// SEM CAPA, NADA DA MARCA ENTRA NO ARQUIVO
//
//	As outras extrações do sistema não podem engordar 130 KB nem mudar de
//	aparência por causa desta.
func TestSemCapaAPlanilhaContinuaCrua(t *testing.T) {
	tab := tabelaDoModelo(3)
	tab.Capa = nil
	partes := abrir(t, tab)
	for _, p := range []string{
		"xl/drawings/drawing1.xml", "xl/media/marca.png",
		"xl/worksheets/_rels/sheet1.xml.rels",
	} {
		if _, tem := partes[p]; tem {
			t.Errorf("%s entrou numa planilha SEM capa", p)
		}
	}
	if strings.Contains(partes["[Content_Types].xml"], "png") {
		t.Error("o [Content_Types].xml declara png numa planilha sem marca")
	}
	folha := partes["xl/worksheets/sheet1.xml"]
	if strings.Contains(folha, "<drawing") {
		t.Error("a folha sem capa referencia um desenho que não existe — o arquivo não abre")
	}
	// O cabeçalho continua na linha 1 e os dados na 2.
	if !strings.Contains(folha, `<row r="1" ht="20" customHeight="1">`) {
		t.Error("a planilha crua mudou de desenho — o cabeçalho não está mais na linha 1")
	}
	if !strings.Contains(folha, `ySplit="1" topLeftCell="A2"`) {
		t.Error("a planilha crua mudou o congelamento")
	}
}

// A PROPORÇÃO DA MARCA É A DO ARQUIVO
//
//	Foi ESTE o defeito do modelo antigo: a âncora pedia 1,5" × 0,67" para uma
//	imagem 2:1 e o Excel esticava. O teste compara a proporção da âncora com a
//	do arquivo e não aceita mais que meio por cento de diferença.
//	A CONFERÊNCIA É EM VÁRIAS LARGURAS, E ISSO NÃO É ZELO — É CORREÇÃO
//
//	A primeira versão media UMA tabela só. Sabotei a conta da proporção e o
//	teste passou: naquela largura o limite de segurança recalculava a altura a
//	partir da largura já cortada e devolvia a proporção certa por acidente. O
//	defeito só aparece onde o limite não morde. Colunas estreitas e largas,
//	então, todas.
func TestAMarcaNaoSaiEsticada(t *testing.T) {
	dela := float64(marcaLargura) / float64(marcaAltura)
	for _, peso := range []float64{5, 8, 14, 30, 60} {
		tab := tabelaDoModelo(4)
		tab.Colunas[0].Peso = peso
		d := tab.dispor()
		_, _, cx, cy := tab.caixaDaMarca(d)
		if cx <= 0 || cy <= 0 {
			t.Fatalf("peso %.0f: caixa da marca sem tamanho: %dx%d", peso, cx, cy)
		}
		nossa := float64(cx) / float64(cy)
		if erro := (nossa - dela) / dela; erro > 0.005 || erro < -0.005 {
			t.Errorf("com a coluna A de peso %.0f a marca sai esticada: proporção "+
				"%.4f contra %.4f do arquivo (%.2f%% de diferença) — é o defeito "+
				"que este modelo veio consertar", peso, nossa, dela, erro*100)
		}
	}
}

// A MARCA CABE DENTRO DA FAIXA, COM FOLGA
//
//	Imagem maior que a faixa vaza por cima da tabela; encostada nas bordas fica
//	pior que centrada errado, porque parece corte.
func TestAMarcaCabeNaFaixaComFolga(t *testing.T) {
	tab := tabelaDoModelo(4)
	d := tab.dispor()
	colOff, rowOff, cx, cy := tab.caixaDaMarca(d)

	var caixaLarg int
	for i := 0; i < colunaDoTexto; i++ {
		caixaLarg += pixelsDaColuna(tab.larguras()[i]) * emuPorPixel
	}
	alturas := tab.alturas(d)
	var caixaAlt int
	for r := d.faixaDe; r <= d.faixaAte; r++ {
		caixaAlt += int(alturas[r] * emuPorPonto)
	}

	if colOff <= 0 || rowOff <= 0 {
		t.Errorf("a marca encosta na borda da faixa (colOff=%d rowOff=%d)", colOff, rowOff)
	}
	if colOff+cx > caixaLarg {
		t.Errorf("a marca vaza para a direita: %d > %d — ela entra por cima do título",
			colOff+cx, caixaLarg)
	}
	if rowOff+cy > caixaAlt {
		t.Errorf("a marca vaza para baixo: %d > %d — ela entra por cima da tabela",
			rowOff+cy, caixaAlt)
	}
}

// A MARCA SE MOVE QUANDO A COLUNA MUDA DE LARGURA
//
//	É a diferença entre posição MEDIDA e posição chutada. Com número fixo, mexer
//	na largura da coluna desloca a marca e ninguém percebe até o cliente abrir.
func TestAMarcaAcompanhaALarguraDaColuna(t *testing.T) {
	estreita := tabelaDoModelo(4)
	larga := tabelaDoModelo(4)
	larga.Colunas[0].Peso = 30 // coluna A bem mais larga

	a, _, _, _ := estreita.caixaDaMarca(estreita.dispor())
	b, _, _, _ := larga.caixaDaMarca(larga.dispor())
	if a == b {
		t.Errorf("a marca ficou no mesmo lugar (%d EMU) com a coluna A muito mais "+
			"larga — a posição está fixa no código, não medida da folha", a)
	}
}

// O AVISO DE CORTE VAI PARA O ALTO, E EMPURRA O RESTO
//
//	Ele é a frase mais importante de um documento cortado. E, quando entra, o
//	mapa inteiro desce uma linha: se algum índice estivesse escrito à mão, o
//	cabeçalho e os dados se separariam.
func TestOAvisoDeCorteEntraNaFaixaEEmpurraOResto(t *testing.T) {
	sem := tabelaDoModelo(3)
	com := tabelaDoModelo(3)
	com.Aviso = "A relação foi cortada em 2000 linhas."

	ds, dc := sem.dispor(), com.dispor()
	if dc.faixaAte != ds.faixaAte+1 || dc.aviso == 0 {
		t.Fatalf("o aviso não entrou na faixa: sem=%+v com=%+v", ds, dc)
	}
	if dc.cabecalho != ds.cabecalho+1 || dc.primeira != ds.primeira+1 || dc.total != ds.total+1 {
		t.Errorf("a tabela não desceu junto com a faixa: sem=%+v com=%+v", ds, dc)
	}
	folha := abrir(t, com)["xl/worksheets/sheet1.xml"]
	if !strings.Contains(folha, "cortada em 2000 linhas") {
		t.Error("o aviso não foi escrito no arquivo — o cliente recebe uma lista " +
			"cortada achando que é a lista inteira")
	}
	// E o congelamento e o filtro acompanharam.
	if !strings.Contains(folha, fmt.Sprintf(`ySplit="%d" topLeftCell="A%d"`, dc.cabecalho, dc.primeira)) {
		t.Error("o congelamento ficou na linha antiga")
	}
	if !strings.Contains(folha, fmt.Sprintf(`<autoFilter ref="A%d:H%d"/>`, dc.cabecalho, dc.ultima)) {
		t.Error("o filtro ficou na linha antiga")
	}
}

// O TOTAL SOMA O QUE ESTÁ FILTRADO
//
//	A planilha vai com filtro ligado. Com SUM, quem filtrar por uma loja lê o
//	total de TODAS embaixo das linhas de uma só — e é esse número que ele copia.
func TestOTotalAcompanhaOFiltro(t *testing.T) {
	tab := tabelaDoModelo(4)
	d := tab.dispor()
	folha := abrir(t, tab)["xl/worksheets/sheet1.xml"]

	alvo := fmt.Sprintf(`<f>SUBTOTAL(109,D%d:D%d)</f>`, d.primeira, d.ultima)
	if !strings.Contains(folha, alvo) {
		t.Errorf("a coluna VALOR não soma com SUBTOTAL sobre %s", alvo)
	}
	if strings.Contains(folha, "<f>SUM(") {
		t.Error("alguma soma virou SUM — ela ignora o filtro e mente para quem filtrar")
	}
	// 100 + 101 + 102 + 103 = 406,00 — e o valor vai em cache, porque leitor
	// que não recalcula mostraria zero.
	if !strings.Contains(folha, `<v>406.00</v>`) {
		t.Error("o total não foi calculado junto com a fórmula — em leitor que não " +
			"recalcula, o cliente vê R$ 0,00 no pé do documento")
	}
}

// UMA LISTA VAZIA NÃO GANHA LINHA DE TOTAL
//
//	"TOTAL · 0 orçamentos · R$ 0,00" com uma fórmula somando um intervalo que
//	não existe é #REF! na cara do cliente.
func TestListaVaziaNaoTemTotal(t *testing.T) {
	tab := tabelaDoModelo(0)
	tab.Capa.Resumo = "0 orçamentos"
	folha := abrir(t, tab)["xl/worksheets/sheet1.xml"]
	if strings.Contains(folha, "SUBTOTAL") {
		t.Error("uma lista vazia ganhou linha de total — a fórmula aponta para um " +
			"intervalo que não existe")
	}
}

// O CABEÇALHO SE REPETE EM TODA PÁGINA IMPRESSA
//
//	São sete páginas. Sem isto, a página 2 chega ao cliente como um bloco de
//	números sem nome de coluna nenhum.
func TestOCabecalhoSeRepeteAoImprimir(t *testing.T) {
	tab := tabelaDoModelo(5)
	d := tab.dispor()
	pasta := abrir(t, tab)["xl/workbook.xml"]
	alvo := fmt.Sprintf(`'Orçamentos'!$%d:$%d`, d.cabecalho, d.cabecalho)
	if !strings.Contains(pasta, "_xlnm.Print_Titles") || !strings.Contains(pasta, alvo) {
		t.Errorf("a pasta não repete a linha %d ao imprimir; ela diz: %s", d.cabecalho, pasta)
	}
}

// O ALINHAMENTO NASCE DO TIPO DA COLUNA
//
//	Se ele viesse de uma segunda lista escrita ao lado das colunas, uma coluna
//	nova entraria numa lista e não na outra — e a planilha sairia com a data à
//	direita e o dinheiro no centro.
func TestOAlinhamentoNasceDoTipoDaColuna(t *testing.T) {
	// Colunas do mesmo tipo têm que dar o mesmo estilo, e tipos diferentes
	// têm que dar estilos diferentes.
	if estiloDoDado(Dinheiro, false, false) == estiloDoDado(Data, false, false) {
		t.Error("dinheiro e data caem no mesmo estilo")
	}
	if estiloDoDado(Texto, false, false) == estiloDoDado(Numero, false, false) {
		t.Error("texto e número caem no mesmo estilo")
	}
	// A zebra é um salto constante, e nunca colide com a linha clara.
	vistos := map[int]bool{}
	for _, tipo := range []Tipo{Texto, Numero, Dinheiro, Data, DataHora} {
		for _, contador := range []bool{false, true} {
			for _, zebra := range []bool{false, true} {
				e := estiloDoDado(tipo, contador, zebra)
				if e < estiloDadoBase || e > estiloDadoBase+11 {
					t.Errorf("estilo %d fora da faixa dos dados (%d..%d)",
						e, estiloDadoBase, estiloDadoBase+11)
				}
				vistos[e] = true
			}
		}
	}
	if len(vistos) != 12 {
		t.Errorf("os dados usam %d estilos; a folha declara 12", len(vistos))
	}
}

// TODO ESTILO USADO EXISTE NA FOLHA DE ESTILOS
//
//	Um `s="31"` numa folha com 31 estilos (0 a 30) é a célula que o Excel pinta
//	de um jeito que ninguém escolheu — ou o arquivo que ele recusa. Acrescentar
//	um estilo no MEIO da lista renumera todos os seguintes, e é isso que este
//	teste pega.
func TestTodoEstiloUsadoExisteNaFolha(t *testing.T) {
	partes := abrir(t, tabelaDoModelo(6))
	var quantos int
	if _, err := fmt.Sscanf(recorteEntre(partes["xl/styles.xml"], `<cellXfs count="`, `"`),
		"%d", &quantos); err != nil {
		t.Fatalf("não achei a contagem de estilos: %v", err)
	}
	if reais := strings.Count(partes["xl/styles.xml"], "<xf numFmtId=") - 1; reais != quantos {
		t.Errorf("a folha diz ter %d estilos e tem %d — o Excel recusa o arquivo",
			quantos, reais)
	}
	folha := partes["xl/worksheets/sheet1.xml"]
	for _, pedaco := range strings.Split(folha, ` s="`)[1:] {
		var n int
		fmt.Sscanf(pedaco, "%d", &n)
		if n >= quantos {
			t.Errorf(`a folha usa s="%d" e só existem %d estilos (0 a %d)`,
				n, quantos, quantos-1)
		}
	}
}

// NENHUMA MESCLA CRUZA COM OUTRA
//
//	Duas mesclas sobrepostas fazem o Excel recusar o arquivo. Como as faixas da
//	capa são calculadas a partir do número de colunas, uma tabela mais estreita
//	é exatamente onde elas se encavalariam.
func TestAsMesclasNaoSeCruzam(t *testing.T) {
	for _, colunas := range []int{4, 5, 6, 8} {
		tab := tabelaDoModelo(3)
		tab.Colunas = tab.Colunas[:colunas]
		for i := range tab.Linhas {
			tab.Linhas[i] = tab.Linhas[i][:colunas]
		}
		folha := abrir(t, tab)["xl/worksheets/sheet1.xml"]

		ocupada := map[string]string{}
		for _, ref := range refsDasMesclas(folha) {
			for _, cel := range celulasDe(ref) {
				if antes, tem := ocupada[cel]; tem {
					t.Errorf("com %d colunas, as mesclas %s e %s dividem a célula %s "+
						"— o Excel recusa o arquivo", colunas, antes, ref, cel)
				}
				ocupada[cel] = ref
			}
		}
	}
}

// A MARCA EMBUTIDA É A MESMA DO FRONT
//
//	São duas cópias do arquivo porque `go:embed` não atravessa a raiz do módulo.
//	Cópia que ninguém compara é cópia que diverge: um dia o front ganha uma
//	marca nova e a planilha continua mandando a velha ao cliente.
func TestAMarcaEmbutidaEAMesmaDoFront(t *testing.T) {
	caminho := filepath.Join("..", "..", "..", "web", "public", "marca.png")
	doFront, err := os.ReadFile(caminho)
	if err != nil {
		t.Skipf("não achei a marca do front: %v", err)
	}
	if !bytes.Equal(doFront, marcaPNG) {
		t.Errorf("a marca embutida no motor (%d bytes) não é a do front (%d bytes) — "+
			"a planilha vai ao cliente com uma marca que o sistema não usa mais. "+
			"Copie web/public/marca.png para interno/relatorio/marca.png",
			len(marcaPNG), len(doFront))
	}
}

// ---------------------------------------------------------------------------
// pedacinhos
// ---------------------------------------------------------------------------

func recorteEntre(s, de, ate string) string {
	i := strings.Index(s, de)
	if i < 0 {
		return ""
	}
	resto := s[i+len(de):]
	j := strings.Index(resto, ate)
	if j < 0 {
		return resto
	}
	return resto[:j]
}

func refsDasMesclas(folha string) []string {
	bloco := recorteEntre(folha, "<mergeCells", "</mergeCells>")
	var refs []string
	for _, p := range strings.Split(bloco, `<mergeCell ref="`)[1:] {
		if j := strings.Index(p, `"`); j > 0 {
			refs = append(refs, p[:j])
		}
	}
	return refs
}

// celulasDe abre "C3:F3" em C3, D3, E3, F3. As mesclas da capa são sempre de
// uma linha só, o que mantém isto simples de propósito.
func celulasDe(ref string) []string {
	partes := strings.Split(ref, ":")
	if len(partes) != 2 {
		return []string{ref}
	}
	colA, linA := parteRef(partes[0])
	colB, linB := parteRef(partes[1])
	var saida []string
	for l := linA; l <= linB; l++ {
		for c := colA; c <= colB; c++ {
			saida = append(saida, fmt.Sprintf("%s%d", coluna(c), l))
		}
	}
	return saida
}

func parteRef(ref string) (col, lin int) {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A') + 1
		i++
	}
	fmt.Sscanf(ref[i:], "%d", &lin)
	return col - 1, lin
}
