// rev 1 — a planilha (.xlsx), escrita à mão
//
// Um `.xlsx` é um zip com meia dúzia de XMLs dentro. O mínimo que o Excel aceita
// é: os tipos de conteúdo, duas listas de relações, a pasta de trabalho, uma
// planilha e os estilos. Falta um deles e o Excel não abre o arquivo — ele diz
// que está corrompido, sem explicar o que faltou.
//
// AS DATAS E OS VALORES SÃO NÚMEROS, NÃO TEXTO
//
//	Data escrita como texto é a diferença entre uma planilha que se ordena e uma
//	que só se olha. Aqui a data vira o número de série que o Excel usa (dias
//	desde 30/12/1899) com um formato de exibição, e o dinheiro vira número com
//	duas casas. Quem receber o arquivo consegue somar, filtrar e ordenar.
package relatorio

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Planilha devolve os bytes de um arquivo .xlsx.
func (t Tabela) Planilha() ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)

	d := t.dispor()
	partes := []struct{ nome, corpo string }{
		{"[Content_Types].xml", tiposDeConteudoCom(t.Capa != nil)},
		{"_rels/.rels", relacoesRaiz},
		{"xl/workbook.xml", t.pasta(d)},
		{"xl/_rels/workbook.xml.rels", relacoesDaPasta},
		{"xl/styles.xml", estilos},
		{"xl/worksheets/sheet1.xml", t.folha(d)},
	}
	// A CAPA TRAZ QUATRO PARTES A MAIS, E AS QUATRO SÃO OBRIGATÓRIAS
	//
	//	O desenho, a relação dele com a imagem, a relação da folha com o
	//	desenho, e a imagem. Faltando qualquer uma, o Excel não avisa o que
	//	faltou: diz que o arquivo está corrompido e se oferece para recuperá-lo
	//	— e a recuperação joga a marca fora em silêncio.
	if t.Capa != nil {
		partes = append(partes,
			struct{ nome, corpo string }{"xl/drawings/drawing1.xml", t.desenho(d)},
			struct{ nome, corpo string }{"xl/drawings/_rels/drawing1.xml.rels", relacoesDoDesenho},
			struct{ nome, corpo string }{"xl/worksheets/_rels/sheet1.xml.rels", relacoesDaFolha},
		)
	}
	for _, p := range partes {
		w, err := z.Create(p.nome)
		if err != nil {
			return nil, fmt.Errorf("não consegui montar a planilha (%s): %w", p.nome, err)
		}
		if _, err := w.Write([]byte(p.corpo)); err != nil {
			return nil, err
		}
	}
	if t.Capa != nil {
		w, err := z.Create("xl/media/marca.png")
		if err != nil {
			return nil, fmt.Errorf("não consegui montar a planilha (marca): %w", err)
		}
		if _, err := w.Write(marcaPNG); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t Tabela) folha(d disposicao) string {
	if t.Capa != nil {
		return t.folhaComCapa(d)
	}
	return t.folhaCrua(d)
}

// folhaCrua é a planilha de sempre: cabeçalho na linha 1, dados a partir da 2.
// Nenhuma das outras extrações do sistema mudou de aparência por causa da capa.
func (t Tabela) folhaCrua(d disposicao) string {
	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	s.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	fmt.Fprintf(&s, `<sheetViews><sheetView workbookViewId="0" tabSelected="1">`+
		`<pane ySplit="%d" topLeftCell="A%d" activePane="bottomLeft" state="frozen"/>`+
		`</sheetView></sheetViews>`, d.cabecalho, d.primeira)

	s.WriteString(`<cols>`)
	for i, l := range t.larguras() {
		fmt.Fprintf(&s, `<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, i+1, i+1, l)
	}
	s.WriteString(`</cols>`)

	s.WriteString(`<sheetData>`)
	fmt.Fprintf(&s, `<row r="%d" ht="20" customHeight="1">`, d.cabecalho)
	for i, c := range t.Colunas {
		s.WriteString(celulaTexto(coluna(i)+strconv.Itoa(d.cabecalho), c.Titulo, 1))
	}
	s.WriteString(`</row>`)

	for l := range t.Linhas {
		fmt.Fprintf(&s, `<row r="%d">`, d.primeira+l)
		for c, col := range t.Colunas {
			ref := coluna(c) + strconv.Itoa(d.primeira+l)
			s.WriteString(celula(ref, t.valor(l, c), col.Tipo))
		}
		s.WriteString(`</row>`)
	}
	s.WriteString(`</sheetData>`)

	fmt.Fprintf(&s, `<autoFilter ref="A%d:%s%d"/>`,
		d.cabecalho, coluna(len(t.Colunas)-1), d.ultima)
	s.WriteString(`</worksheet>`)
	return s.String()
}

// folhaComCapa é o modelo que vai ao cliente: faixa escura com a marca, tabela
// clara, total no pé, cabeçalho repetindo em toda página impressa.
//
// A ORDEM DOS BLOCOS NÃO É ESTILO, É EXIGÊNCIA
//
//	`sheetPr`, `sheetViews`, `cols`, `sheetData`, `autoFilter`, `mergeCells`,
//	`pageMargins`, `pageSetup`, `headerFooter`, `drawing` — nesta ordem. Fora
//	dela o Excel recusa o arquivo sem dizer qual bloco estava no lugar errado.
func (t Tabela) folhaComCapa(d disposicao) string {
	ultimaCol := coluna(len(t.Colunas) - 1)
	alturas := t.alturas(d)

	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	s.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	s.WriteString(`<sheetPr><tabColor rgb="` + corCasa + `"/><pageSetUpPr fitToPage="1"/></sheetPr>`)

	// A GRADE SAI DE CENA
	//   Com a grade ligada, o desenho vira uma faixa bonita colada numa folha
	//   quadriculada. Sem ela o arquivo lê como documento, que é o que é.
	fmt.Fprintf(&s, `<sheetViews><sheetView workbookViewId="0" tabSelected="1" showGridLines="0">`+
		`<pane ySplit="%d" topLeftCell="A%d" activePane="bottomLeft" state="frozen"/>`+
		`</sheetView></sheetViews>`, d.cabecalho, d.primeira)

	s.WriteString(`<cols>`)
	for i, l := range t.larguras() {
		fmt.Fprintf(&s, `<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, i+1, i+1, l)
	}
	s.WriteString(`</cols>`)

	s.WriteString(`<sheetData>`)
	t.escreverCapa(&s, d, alturas)
	t.escreverTabela(&s, d)
	s.WriteString(`</sheetData>`)

	fmt.Fprintf(&s, `<autoFilter ref="A%d:%s%d"/>`, d.cabecalho, ultimaCol, d.ultima)
	t.escreverMesclas(&s, d, ultimaCol)

	s.WriteString(`<pageMargins left="0.3" right="0.3" top="0.45" bottom="0.45" header="0.2" footer="0.2"/>`)
	s.WriteString(`<pageSetup paperSize="9" orientation="landscape" fitToWidth="1" fitToHeight="0"/>`)
	s.WriteString(`<headerFooter><oddFooter>&amp;L&amp;8Frota Macedo Engenharia` +
		`&amp;R&amp;8P&#225;gina &amp;P de &amp;N</oddFooter></headerFooter>`)
	s.WriteString(`<drawing xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1"/>`)
	s.WriteString(`</worksheet>`)
	return s.String()
}

// escreverCapa desenha a faixa escura: as células pintadas de ponta a ponta, o
// texto por cima, o fio vermelho e o respiro branco.
func (t Tabela) escreverCapa(s *strings.Builder, d disposicao, alturas map[int]float64) {
	// A faixa é pintada em TODAS as colunas, não só nas que têm texto: célula
	// sem preenchimento no meio de um fundo escuro aparece como um buraco
	// branco, e aparece exatamente na largura que ninguém testou.
	faixa := func(linha int, estilo int) {
		fmt.Fprintf(s, `<row r="%d" ht="%.1f" customHeight="1">`, linha, alturas[linha])
		for c := range t.Colunas {
			fmt.Fprintf(s, `<c r="%s%d" s="%d"/>`, coluna(c), linha, estilo)
		}
		s.WriteString(`</row>`)
	}
	// Uma linha da faixa com texto: pinta tudo e reescreve as células que falam.
	comTexto := func(linha int, textos map[int]struct {
		valor  string
		estilo int
	}) {
		fmt.Fprintf(s, `<row r="%d" ht="%.1f" customHeight="1">`, linha, alturas[linha])
		for c := range t.Colunas {
			ref := coluna(c) + strconv.Itoa(linha)
			if x, ok := textos[c]; ok && x.valor != "" {
				s.WriteString(celulaTexto(ref, x.valor, x.estilo))
				continue
			}
			estilo := estiloFaixa
			if x, ok := textos[c]; ok {
				estilo = x.estilo
			}
			fmt.Fprintf(s, `<c r="%s" s="%d"/>`, ref, estilo)
		}
		s.WriteString(`</row>`)
	}
	type fala = struct {
		valor  string
		estilo int
	}

	esquerda := colunaDoTexto
	if esquerda >= len(t.Colunas) {
		esquerda = 0
	}
	// O bloco da direita — contagem e total — só existe se sobrarem colunas
	// para ele. Numa tabela estreita ele simplesmente não aparece, em vez de
	// escrever por cima do título.
	direita := len(t.Colunas) - 2
	if direita <= esquerda {
		direita = -1
	}

	comTexto(d.faixaDe, map[int]fala{esquerda: {t.Titulo, estiloTitulo}})
	comTexto(d.faixaDe+1, map[int]fala{esquerda: {t.Capa.Chapeu, estiloChapeu}})

	linha3 := map[int]fala{esquerda: {t.Capa.Periodo, estiloFaixaTexto}}
	linha4 := map[int]fala{esquerda: {t.Capa.Assinatura, estiloFaixaTexto}}
	if direita >= 0 {
		linha3[direita] = fala{t.Capa.Resumo, estiloResumo}
		linha4[direita] = fala{t.Capa.Destaque, estiloDestaque}
	}
	comTexto(d.faixaDe+2, linha3)
	comTexto(d.faixaDe+3, linha4)

	if d.aviso > 0 {
		comTexto(d.aviso, map[int]fala{esquerda: {t.Aviso, estiloAviso}})
	}
	faixa(d.fio, estiloFio)
	faixa(d.respiro, estiloRespiro)
}

// escreverTabela desenha o cabeçalho vermelho, os dados e o total.
func (t Tabela) escreverTabela(s *strings.Builder, d disposicao) {
	fmt.Fprintf(s, `<row r="%d" ht="22" customHeight="1">`, d.cabecalho)
	for i, c := range t.Colunas {
		estilo := estiloCabecalhoCentro
		if c.Tipo == Texto {
			estilo = estiloCabecalhoEsquerda
		}
		s.WriteString(celulaTexto(coluna(i)+strconv.Itoa(d.cabecalho), c.Titulo, estilo))
	}
	s.WriteString(`</row>`)

	for l := range t.Linhas {
		linha := d.primeira + l
		fmt.Fprintf(s, `<row r="%d" ht="15.5" customHeight="1">`, linha)
		for c, col := range t.Colunas {
			ref := coluna(c) + strconv.Itoa(linha)
			s.WriteString(celulaCom(ref, t.valor(l, c), col.Tipo,
				estiloDoDado(col.Tipo, c == 0 && col.Tipo == Numero, l%2 == 1)))
		}
		s.WriteString(`</row>`)
	}

	if len(t.Linhas) == 0 {
		return
	}
	t.escreverTotal(s, d)
}

// escreverTotal fecha a lista somando as colunas de dinheiro.
//
// A SOMA É `SUBTOTAL`, NÃO `SUM`
//
//	A planilha vai com filtro ligado. Com `SUM`, quem filtrar por uma loja vê o
//	total de TODAS embaixo das linhas de uma só — e é justamente esse número que
//	a pessoa vai copiar. `SUBTOTAL(109,…)` acompanha o filtro.
//
// O VALOR VAI CALCULADO JUNTO COM A FÓRMULA
//
//	Fórmula sem valor em cache abre como zero em leitor que não recalcula. O
//	Excel recalcula; o visualizador do celular do cliente, nem sempre.
func (t Tabela) escreverTotal(s *strings.Builder, d disposicao) {
	fmt.Fprintf(s, `<row r="%d" ht="22" customHeight="1">`, d.total)
	primeiraDinheiro := -1
	for c, col := range t.Colunas {
		if col.Tipo == Dinheiro {
			primeiraDinheiro = c
			break
		}
	}
	for c, col := range t.Colunas {
		ref := coluna(c) + strconv.Itoa(d.total)
		switch {
		case c == 0:
			// O rótulo diz TOTAL antes da contagem. "60 orçamentos" sozinho no
			// pé de uma lista de 60 linhas é ambíguo: pode ser mais uma linha.
			rotulo := "TOTAL"
			if t.Capa.Resumo != "" {
				rotulo += "  ·  " + t.Capa.Resumo
			}
			s.WriteString(celulaTexto(ref, rotulo, estiloTotalRotulo))
		case col.Tipo == Dinheiro:
			var soma float64
			for l := range t.Linhas {
				if n, ok := comoNumero(t.valor(l, c)); ok {
					soma += n
				}
			}
			letra := coluna(c)
			fmt.Fprintf(s, `<c r="%s" s="%d"><f>SUBTOTAL(109,%s%d:%s%d)</f><v>%s</v></c>`,
				ref, estiloTotalDinheiro, letra, d.primeira, letra, d.ultima,
				strconv.FormatFloat(soma, 'f', 2, 64))
		case primeiraDinheiro > 0 && c < primeiraDinheiro:
			// Faz parte da mescla do rótulo: a célula existe só para o
			// preenchimento e a borda não terem falha.
			fmt.Fprintf(s, `<c r="%s" s="%d"/>`, ref, estiloTotalRotulo)
		default:
			fmt.Fprintf(s, `<c r="%s" s="%d"/>`, ref, estiloTotalVazio)
		}
	}
	s.WriteString(`</row>`)
}

// escreverMesclas junta as células do texto da faixa e do rótulo do total.
func (t Tabela) escreverMesclas(s *strings.Builder, d disposicao, ultimaCol string) {
	esquerda := colunaDoTexto
	if esquerda >= len(t.Colunas) {
		esquerda = 0
	}
	direita := len(t.Colunas) - 2
	if direita <= esquerda {
		direita = -1
	}
	letraEsq := coluna(esquerda)

	var refs []string
	junta := func(r string) { refs = append(refs, r) }

	junta(fmt.Sprintf("%s%d:%s%d", letraEsq, d.faixaDe, ultimaCol, d.faixaDe))
	junta(fmt.Sprintf("%s%d:%s%d", letraEsq, d.faixaDe+1, ultimaCol, d.faixaDe+1))
	if direita >= 0 {
		fimEsq := coluna(direita - 1)
		junta(fmt.Sprintf("%s%d:%s%d", letraEsq, d.faixaDe+2, fimEsq, d.faixaDe+2))
		junta(fmt.Sprintf("%s%d:%s%d", letraEsq, d.faixaDe+3, fimEsq, d.faixaDe+3))
		junta(fmt.Sprintf("%s%d:%s%d", coluna(direita), d.faixaDe+2, ultimaCol, d.faixaDe+2))
		junta(fmt.Sprintf("%s%d:%s%d", coluna(direita), d.faixaDe+3, ultimaCol, d.faixaDe+3))
	} else {
		junta(fmt.Sprintf("%s%d:%s%d", letraEsq, d.faixaDe+2, ultimaCol, d.faixaDe+2))
		junta(fmt.Sprintf("%s%d:%s%d", letraEsq, d.faixaDe+3, ultimaCol, d.faixaDe+3))
	}
	if d.aviso > 0 {
		junta(fmt.Sprintf("%s%d:%s%d", letraEsq, d.aviso, ultimaCol, d.aviso))
	}
	if len(t.Linhas) > 0 {
		primeiraDinheiro := -1
		for c, col := range t.Colunas {
			if col.Tipo == Dinheiro {
				primeiraDinheiro = c
				break
			}
		}
		if primeiraDinheiro > 1 {
			junta(fmt.Sprintf("A%d:%s%d", d.total, coluna(primeiraDinheiro-1), d.total))
		}
	}

	fmt.Fprintf(s, `<mergeCells count="%d">`, len(refs))
	for _, r := range refs {
		fmt.Fprintf(s, `<mergeCell ref="%s"/>`, r)
	}
	s.WriteString(`</mergeCells>`)
}

func celula(ref string, v any, tipo Tipo) string {
	if v == nil {
		return ""
	}
	switch tipo {
	case Numero:
		if n, ok := comoNumero(v); ok {
			return fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, strconv.FormatFloat(n, 'f', -1, 64))
		}
	case Dinheiro:
		if n, ok := comoNumero(v); ok {
			return fmt.Sprintf(`<c r="%s" s="4"><v>%s</v></c>`, ref, strconv.FormatFloat(n, 'f', 2, 64))
		}
	case Data, DataHora:
		if d, ok := v.(time.Time); ok && !d.IsZero() {
			estilo := 2
			if tipo == DataHora {
				estilo = 3
			}
			// PRECISÃO TOTAL, E NÃO SEIS CASAS
			//
			//	Seis casas de um DIA são 86 milésimos de segundo de resolução.
			//	Parece bastante até alguém abrir o arquivo com um leitor de
			//	verdade: 09:05:00 saía como 09:04:59,981. O Excel esconde isso ao
			//	exibir, porque o formato arredonda para o minuto — então o
			//	defeito atravessaria a conferência visual inteira e só apareceria
			//	numa fórmula que compara horários.
			//
			//	`-1` escreve a representação mais curta que volta ao mesmo
			//	float64. É o número que a gente calculou, e não uma versão
			//	aparada dele.
			return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, estilo,
				strconv.FormatFloat(serieDoExcel(d), 'f', -1, 64))
		}
		return ""
	}
	return celulaTexto(ref, fmt.Sprint(v), 0)
}

func celulaTexto(ref, texto string, estilo int) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(limpar(texto)))
	return fmt.Sprintf(`<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`,
		ref, estilo, b.String())
}

// celulaCom é a célula da planilha com capa: ela SEMPRE sai escrita, com o
// estilo dado, mesmo vazia.
//
// POR QUE VAZIA TAMBÉM
//
//	Na planilha crua, célula sem valor não é escrita e ninguém percebe. Aqui a
//	linha tem zebra e fio embaixo: a célula que não sai vira uma falha branca no
//	meio da faixa cinza, e a falha aparece justamente na coluna que ninguém
//	preencheu — a PCO, que vai vazia de propósito.
func celulaCom(ref string, v any, tipo Tipo, estilo int) string {
	if v == nil {
		return fmt.Sprintf(`<c r="%s" s="%d"/>`, ref, estilo)
	}
	switch tipo {
	case Numero, Dinheiro:
		if n, ok := comoNumero(v); ok {
			casas := -1
			if tipo == Dinheiro {
				casas = 2
			}
			return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, estilo,
				strconv.FormatFloat(n, 'f', casas, 64))
		}
	case Data, DataHora:
		if dt, ok := v.(time.Time); ok && !dt.IsZero() {
			return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, estilo,
				strconv.FormatFloat(serieDoExcel(dt), 'f', -1, 64))
		}
		return fmt.Sprintf(`<c r="%s" s="%d"/>`, ref, estilo)
	}
	texto := fmt.Sprint(v)
	if texto == "" {
		return fmt.Sprintf(`<c r="%s" s="%d"/>`, ref, estilo)
	}
	return celulaTexto(ref, texto, estilo)
}

// Os estilos da capa, pelo número que eles ocupam na folha de estilos.
const (
	estiloFaixa             = 5
	estiloTitulo            = 6
	estiloChapeu            = 7
	estiloFaixaTexto        = 8
	estiloResumo            = 9
	estiloDestaque          = 10
	estiloFio               = 11
	estiloRespiro           = 12
	estiloCabecalhoCentro   = 13
	estiloCabecalhoEsquerda = 14
	// Os doze estilos dos dados começam aqui: seis para a linha clara e, seis
	// adiante, os mesmos seis com a zebra.
	estiloDadoBase      = 15
	saltoDaZebra        = 6
	estiloTotalRotulo   = 27
	estiloTotalDinheiro = 28
	estiloTotalVazio    = 29
	estiloAviso         = 30
)

// estiloDoDado escolhe o estilo da célula pelo TIPO da coluna — não por uma
// lista de alinhamentos escrita à mão ao lado da lista de colunas.
//
//	Duas listas em paralelo é o começo de toda planilha com a data alinhada à
//	direita e o dinheiro no centro: alguém acrescenta a coluna numa e esquece a
//	outra. Aqui o alinhamento é consequência do tipo, e coluna nova já nasce
//	certa.
func estiloDoDado(tipo Tipo, contador, zebra bool) int {
	e := estiloDadoBase
	switch {
	case contador:
		e += 5
	case tipo == Texto:
		e++
	case tipo == Dinheiro:
		e += 2
	case tipo == Data:
		e += 3
	case tipo == DataHora:
		e += 4
	}
	if zebra {
		e += saltoDaZebra
	}
	return e
}

func comoNumero(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(n, ",", "."), 64)
		return f, err == nil
	}
	return 0, false
}

// serieDoExcel converte um instante no número que o Excel usa: dias desde
// 30/12/1899.
//
// A CONTA É FEITA COM O RELÓGIO DE PAREDE, NÃO COM UMA SUBTRAÇÃO DE INSTANTES
//
//	O caminho direto — levar a data para o fuso da casa e subtrair 30/12/1899
//	naquele mesmo fuso — erra por 26 minutos. Em 1899 Fortaleza não estava em
//	UTC−3: estava no horário do meridiano local, UTC−2:34. A subtração carrega
//	essa diferença para dentro do resultado, e a planilha mostra 13:11 onde o
//	sistema mostra 12:45.
//
//	Isso só apareceu abrindo o arquivo com um leitor de planilha de verdade — os
//	testes do próprio código concordavam consigo mesmos.
//
//	Aqui a data é lida como as pessoas a leem: dia, hora e minuto no fuso da
//	casa, e a contagem de dias feita entre duas datas no MESMO fuso neutro.
func serieDoExcel(d time.Time) float64 {
	local := d.In(FusoDaCasa())
	meiaNoite := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	origem := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	dias := meiaNoite.Sub(origem).Hours() / 24
	fracao := float64(local.Hour()*3600+local.Minute()*60+local.Second()) / 86400
	return dias + fracao
}

// limpar tira os caracteres de controle que o XML recusa. Texto vindo de outro
// sistema traz coisa estranha, e um único byte inválido faz o Excel recusar o
// arquivo inteiro.
func limpar(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, s)
}

// coluna traduz 0,1,2... em A,B,C... e segue depois de Z (AA, AB...).
func coluna(i int) string {
	nome := ""
	for i >= 0 {
		nome = string(rune('A'+i%26)) + nome
		i = i/26 - 1
	}
	return nome
}

const tiposDeConteudo = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const relacoesRaiz = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

func (t Tabela) pasta(d disposicao) string {
	// O CABEÇALHO DA TABELA SE REPETE EM TODA PÁGINA IMPRESSA
	//
	//	Isto não é atributo da folha: é um nome definido reservado, guardado na
	//	pasta. Sem ele, a página 2 de sete chega ao cliente como um bloco de
	//	números sem nome de coluna nenhum.
	titulos := ""
	if t.Capa != nil {
		titulos = fmt.Sprintf(`<definedNames><definedName name="_xlnm.Print_Titles" `+
			`localSheetId="0">'%s'!$%d:$%d</definedName></definedNames>`,
			nomeDaAba(t), d.cabecalho, d.cabecalho)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
 xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="` + nomeDaAba(t) + `" sheetId="1" r:id="rId1"/></sheets>` + titulos + `
</workbook>`
}

// nomeDaAba obedece às regras do Excel: até 31 caracteres, sem : \\ / ? * [ ],
// nunca em branco. Um nome fora dessas regras não vira aviso — o arquivo
// simplesmente não abre.
func nomeDaAba(t Tabela) string {
	bruto := t.Aba
	if strings.TrimSpace(bruto) == "" {
		bruto = t.Titulo
	}
	// O travessão e o que vem depois dele são subtítulo disfarçado de título:
	// "Orçamentos montados — 2026-08" vira "Orçamentos montados".
	if i := strings.Index(bruto, " — "); i > 0 {
		bruto = bruto[:i]
	}
	var s []rune
	for _, r := range bruto {
		if strings.ContainsRune(`:\/?*[]`, r) {
			continue
		}
		s = append(s, r)
		if len(s) == 31 {
			break
		}
	}
	limpo := strings.TrimSpace(strings.Trim(string(s), "'"))
	if limpo == "" {
		return "Planilha"
	}
	// O nome vira ATRIBUTO XML. Um "&" cru aqui não dá aviso nenhum: derruba o
	// arquivo inteiro na abertura.
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(limpar(limpo)))
	return b.String()
}

const relacoesDaPasta = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

// Os estilos, na ordem em que o código os usa:
//
//	0 comum · 1 cabeçalho · 2 data · 3 data e hora · 4 dinheiro
//
// Do 5 em diante são os da capa, e os nomes deles estão nas constantes
// `estilo*` acima. Os índices são POSIÇÃO nesta lista: acrescentar um estilo no
// meio renumera todos os seguintes e repinta a planilha inteira em silêncio.
// Estilo novo entra no FIM.
const estilos = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<numFmts count="4">
<numFmt numFmtId="164" formatCode="dd/mm/yyyy"/>
<numFmt numFmtId="165" formatCode="dd/mm/yyyy\ hh:mm"/>
<numFmt numFmtId="166" formatCode="#,##0.00"/>
<numFmt numFmtId="167" formatCode="&quot;R$&quot;\ #,##0.00"/>
</numFmts>
<fonts count="11">
<font><sz val="10"/><name val="Calibri"/></font>
<font><b/><sz val="10"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font>
<font><b/><sz val="15"/><color rgb="FFEFEBEC"/><name val="Calibri"/></font>
<font><b/><sz val="9"/><color rgb="FFD2494C"/><name val="Calibri"/></font>
<font><sz val="9.5"/><color rgb="FFB9B1B3"/><name val="Calibri"/></font>
<font><b/><sz val="9.5"/><color rgb="FFD2494C"/><name val="Calibri"/></font>
<font><b/><sz val="13"/><color rgb="FFEFEBEC"/><name val="Calibri"/></font>
<font><b/><sz val="9.5"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font>
<font><sz val="9.5"/><color rgb="FF26201F"/><name val="Calibri"/></font>
<font><sz val="9"/><color rgb="FF9A8F90"/><name val="Calibri"/></font>
<font><b/><sz val="10"/><color rgb="FF7A1517"/><name val="Calibri"/></font>
</fonts>
<fills count="9">
<fill><patternFill patternType="none"/></fill>
<fill><patternFill patternType="gray125"/></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FF1E2227"/><bgColor indexed="64"/></patternFill></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FF100D0E"/><bgColor indexed="64"/></patternFill></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FFA11F22"/><bgColor indexed="64"/></patternFill></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FF7A1517"/><bgColor indexed="64"/></patternFill></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FFFAF6F6"/><bgColor indexed="64"/></patternFill></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FFF4EDED"/><bgColor indexed="64"/></patternFill></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FFFFFFFF"/><bgColor indexed="64"/></patternFill></fill>
</fills>
<borders count="3">
<border><left/><right/><top/><bottom/><diagonal/></border>
<border><left/><right/><top/><bottom style="thin"><color rgb="FFE8DFDF"/></bottom><diagonal/></border>
<border><left/><right/><top style="medium"><color rgb="FF7A1517"/></top><bottom/><diagonal/></border>
</borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="31">
<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment vertical="center"/></xf>
<xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment vertical="center"/></xf>
<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="165" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="166" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="0" fontId="0" fillId="3" borderId="0" xfId="0" applyFill="1"/>
<xf numFmtId="0" fontId="2" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="left" vertical="bottom"/></xf>
<xf numFmtId="0" fontId="3" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="left" vertical="top"/></xf>
<xf numFmtId="0" fontId="4" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="left" vertical="center"/></xf>
<xf numFmtId="0" fontId="5" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="right" vertical="center" indent="1"/></xf>
<xf numFmtId="0" fontId="6" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="right" vertical="center" indent="1"/></xf>
<xf numFmtId="0" fontId="0" fillId="4" borderId="0" xfId="0" applyFill="1"/>
<xf numFmtId="0" fontId="0" fillId="8" borderId="0" xfId="0" applyFill="1"/>
<xf numFmtId="0" fontId="7" fillId="5" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="0" fontId="7" fillId="5" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="left" vertical="center" indent="1"/></xf>
<xf numFmtId="0" fontId="8" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="0" fontId="8" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="left" vertical="center" indent="1"/></xf>
<xf numFmtId="167" fontId="8" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right" vertical="center" indent="1"/></xf>
<xf numFmtId="164" fontId="8" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="165" fontId="8" fillId="0" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="0" fontId="9" fillId="0" borderId="1" xfId="0" applyFont="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="0" fontId="8" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="0" fontId="8" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="left" vertical="center" indent="1"/></xf>
<xf numFmtId="167" fontId="8" fillId="6" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right" vertical="center" indent="1"/></xf>
<xf numFmtId="164" fontId="8" fillId="6" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="165" fontId="8" fillId="6" borderId="1" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="0" fontId="9" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
<xf numFmtId="0" fontId="10" fillId="7" borderId="2" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="left" vertical="center" indent="1"/></xf>
<xf numFmtId="167" fontId="10" fillId="7" borderId="2" xfId="0" applyNumberFormat="1" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="right" vertical="center" indent="1"/></xf>
<xf numFmtId="0" fontId="0" fillId="7" borderId="2" xfId="0" applyFill="1" applyBorder="1"/>
<xf numFmtId="0" fontId="5" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="left" vertical="center"/></xf>
</cellXfs>
</styleSheet>`
