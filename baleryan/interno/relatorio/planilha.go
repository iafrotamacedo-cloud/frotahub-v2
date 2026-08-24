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

	partes := []struct{ nome, corpo string }{
		{"[Content_Types].xml", tiposDeConteudo},
		{"_rels/.rels", relacoesRaiz},
		{"xl/workbook.xml", pastaDeTrabalho},
		{"xl/_rels/workbook.xml.rels", relacoesDaPasta},
		{"xl/styles.xml", estilos},
		{"xl/worksheets/sheet1.xml", t.folha()},
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
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (t Tabela) folha() string {
	var s strings.Builder
	s.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	s.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)

	// Larguras: o peso da coluna vira largura em caracteres.
	s.WriteString(`<cols>`)
	for i, c := range t.Colunas {
		largura := c.Peso * 1.6
		if largura < 8 {
			largura = 8
		}
		fmt.Fprintf(&s, `<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, i+1, i+1, largura)
	}
	s.WriteString(`</cols>`)

	s.WriteString(`<sheetData>`)

	// Linha 1: o cabeçalho.
	s.WriteString(`<row r="1" ht="20" customHeight="1">`)
	for i, c := range t.Colunas {
		s.WriteString(celulaTexto(coluna(i)+"1", c.Titulo, 1))
	}
	s.WriteString(`</row>`)

	for l := range t.Linhas {
		fmt.Fprintf(&s, `<row r="%d">`, l+2)
		for c, col := range t.Colunas {
			ref := coluna(c) + strconv.Itoa(l+2)
			s.WriteString(celula(ref, t.valor(l, c), col.Tipo))
		}
		s.WriteString(`</row>`)
	}
	s.WriteString(`</sheetData>`)

	// Congela o cabeçalho e liga o filtro: quem abrir a planilha já encontra as
	// duas coisas que ia procurar.
	fmt.Fprintf(&s, `<autoFilter ref="A1:%s%d"/>`, coluna(len(t.Colunas)-1), len(t.Linhas)+1)
	s.WriteString(`</worksheet>`)

	// O painel congelado precisa vir ANTES de sheetData na ordem do esquema; por
	// isso é montado no fim e enfiado no lugar certo.
	congelado := `<sheetViews><sheetView workbookViewId="0" tabSelected="1">` +
		`<pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/>` +
		`</sheetView></sheetViews>`
	saida := s.String()
	marca := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`
	return strings.Replace(saida, marca, marca+congelado, 1)
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
			return fmt.Sprintf(`<c r="%s" s="%d"><v>%s</v></c>`, ref, estilo,
				strconv.FormatFloat(serieDoExcel(d), 'f', 6, 64))
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

const pastaDeTrabalho = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"
 xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Chamados" sheetId="1" r:id="rId1"/></sheets>
</workbook>`

const relacoesDaPasta = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

// Os estilos, na ordem em que o código os usa:
//   0 comum · 1 cabeçalho · 2 data · 3 data e hora · 4 dinheiro
const estilos = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<numFmts count="3">
<numFmt numFmtId="164" formatCode="dd/mm/yyyy"/>
<numFmt numFmtId="165" formatCode="dd/mm/yyyy\ hh:mm"/>
<numFmt numFmtId="166" formatCode="#,##0.00"/>
</numFmts>
<fonts count="2">
<font><sz val="10"/><name val="Calibri"/></font>
<font><b/><sz val="10"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font>
</fonts>
<fills count="3">
<fill><patternFill patternType="none"/></fill>
<fill><patternFill patternType="gray125"/></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FF1E2227"/><bgColor indexed="64"/></patternFill></fill>
</fills>
<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="5">
<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment vertical="center"/></xf>
<xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment vertical="center"/></xf>
<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="165" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
<xf numFmtId="166" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
</cellXfs>
</styleSheet>`
