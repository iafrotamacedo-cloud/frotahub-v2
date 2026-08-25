package relatorio

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func tabelaDeTeste(linhas int) Tabela {
	quando := time.Date(2026, 8, 22, 15, 45, 0, 0, time.UTC)
	t := Tabela{
		Titulo:    "Dados do Trílogo — chamados",
		Subtitulo: "Loja: LOJA 29 - MONDUBIM · Status: Vistoriado · 01/07/2026 a 24/08/2026",
		Gerado:    time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		Colunas: []Coluna{
			{Titulo: "Ticket", Peso: 6, Tipo: Numero},
			{Titulo: "Loja", Peso: 16, Tipo: Texto},
			{Titulo: "Conta", Peso: 8, Tipo: Texto},
			{Titulo: "Status", Peso: 9, Tipo: Texto},
			{Titulo: "Prioridade", Peso: 8, Tipo: Texto},
			{Titulo: "Descrição", Peso: 34, Tipo: Texto},
			{Titulo: "Criado em", Peso: 11, Tipo: DataHora},
			{Titulo: "Custo", Peso: 8, Tipo: Dinheiro},
		},
	}
	for i := 0; i < linhas; i++ {
		t.Linhas = append(t.Linhas, []any{
			131712 - i, "LOJA 29 - MONDUBIM", "Instalações", "Vistoriado", "Média",
			"Solicito equipe para verificar as máquinas, principalmente de refrigeração da loja. As mesmas encontram-se alagando a laje TECK e não é comum.",
			quando.Add(time.Duration(-i) * time.Hour), 94.44 + float64(i),
		})
	}
	return t
}

// ---------------------------------------------------------------------------
// Planilha
// ---------------------------------------------------------------------------

// O Excel não explica por que recusa um arquivo: ele diz que está corrompido e
// pronto. Então a prova confere as peças obrigatórias uma a uma, e confere que
// cada XML é XML de verdade.
func TestPlanilhaTemAsPecasQueOExcelExige(t *testing.T) {
	bruto, err := tabelaDeTeste(3).Planilha()
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(bruto), int64(len(bruto)))
	if err != nil {
		t.Fatalf("não é um zip válido: %v", err)
	}

	dentro := map[string]string{}
	for _, f := range z.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		dentro[f.Name] = string(b)
		if err := xml.Unmarshal(b, new(any)); err != nil {
			t.Errorf("%s não é XML válido: %v", f.Name, err)
		}
	}

	for _, obrigatorio := range []string{
		"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml",
		"xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml",
	} {
		if _, tem := dentro[obrigatorio]; !tem {
			t.Errorf("faltou %s — o Excel recusa o arquivo inteiro", obrigatorio)
		}
	}

	folha := dentro["xl/worksheets/sheet1.xml"]
	for _, pedaco := range []string{
		"Ticket", "LOJA 29 - MONDUBIM", "Descri", // cabeçalho e conteúdo
		`<pane ySplit="1"`, // cabeçalho congelado
		"<autoFilter",      // filtro ligado
	} {
		if !strings.Contains(folha, pedaco) {
			t.Errorf("faltou %q na planilha", pedaco)
		}
	}
}

// Data como texto é a diferença entre uma planilha que se ordena e uma que só se
// olha. Se algum dia alguém "simplificar" mandando a data como string, cai aqui.
func TestDataEDinheiroVaoComoNumero(t *testing.T) {
	bruto, _ := tabelaDeTeste(1).Planilha()
	folha := abrirFolha(t, bruto)

	if strings.Contains(folha, "22/08/2026") {
		t.Error("a data foi escrita como texto; devia ser número com formato de data")
	}
	// 22/08/2026 15:45 UTC = 12:45 em Fortaleza. 12:45 é 0,53125 de um dia, e o
	// número tem que bater NA CASA DO MINUTO: um "46256.5" qualquer esconderia
	// um erro de meia hora, que foi exatamente o que aconteceu na primeira
	// versão desta conta.
	// A série agora é escrita com precisão total (`FormatFloat(..., -1)`), e não
	// com seis casas fixas: é o mesmo número, escrito sem o zero de enfeite.
	if !strings.Contains(folha, `s="3"><v>46256.53125<`) {
		t.Errorf("a data não virou número de série do Excel no fuso certo:\n%s", recorte(folha, "s=\"3\""))
	}
	if !strings.Contains(folha, `s="4"><v>94.44`) {
		t.Errorf("o dinheiro não veio como número: %s", recorte(folha, "s=\"4\""))
	}
}

// Texto vindo de outro sistema traz caractere de controle, e um byte inválido faz
// o Excel recusar o arquivo INTEIRO — não a célula.
func TestCaractereEstranhoNaoDerrubaAPlanilha(t *testing.T) {
	tab := tabelaDeTeste(1)
	tab.Linhas[0][1] = "LOJA\x07 29\nMONDUBIM & \"cia\" <teste>"
	bruto, err := tab.Planilha()
	if err != nil {
		t.Fatal(err)
	}
	folha := abrirFolha(t, bruto)
	if strings.Contains(folha, "\x07") {
		t.Error("o caractere de controle passou para dentro do XML")
	}
	if !strings.Contains(folha, "&amp;") || !strings.Contains(folha, "&lt;teste&gt;") {
		t.Error("os caracteres de XML não foram escapados")
	}
}

func TestColunasPassamDeZ(t *testing.T) {
	for i, quer := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"} {
		if veio := coluna(i); veio != quer {
			t.Errorf("coluna(%d) = %s, esperava %s", i, veio, quer)
		}
	}
}

// ---------------------------------------------------------------------------
// PDF
// ---------------------------------------------------------------------------

func TestPDFAbreEPagina(t *testing.T) {
	// 80 linhas não cabem numa folha: tem que virar mais de uma.
	bruto, err := tabelaDeTeste(80).PDF()
	if err != nil {
		t.Fatal(err)
	}
	s := string(bruto)

	if !strings.HasPrefix(s, "%PDF-1.4") {
		t.Fatal("não começa com a assinatura de PDF")
	}
	if !strings.HasSuffix(strings.TrimSpace(s), "%%EOF") {
		t.Error("não termina com o marcador de fim de arquivo")
	}
	if n := strings.Count(s, "/Type /Page\n") + strings.Count(s, "/Type /Page "); n < 2 {
		t.Errorf("80 linhas couberam numa folha só? páginas encontradas: %d", n)
	}
	if !strings.Contains(s, "/Count 3") {
		t.Errorf("esperava 3 folhas para 80 linhas; veio outro número:\n%s", recorte(s, "/Count"))
	}
	if !strings.Contains(s, "Helvetica-Bold") {
		t.Error("faltou a fonte do cabeçalho")
	}
}

// A tabela de posições no fim do arquivo é a única peça que, errada, faz o leitor
// recusar o PDF. Ela é conferida byte a byte: cada posição tem que cair
// exatamente no começo do objeto que ela promete.
//
// (Pegadinha que este teste já cobrou de si mesmo: as posições são gravadas com
// zeros à esquerda, e `fmt.Sscan` lê "0000000015" como OCTAL. A leitura tem que
// dizer a base, senão o teste acusa um defeito que não existe.)
func TestTabelaDePosicoesApontaParaOsObjetos(t *testing.T) {
	bruto, _ := tabelaDeTeste(5).PDF()
	s := string(bruto)

	inicio := strings.Index(s, "\nxref\n")
	if inicio < 0 {
		t.Fatal("não achei a tabela de posições")
	}
	// linhas[0] é a contagem e linhas[1] é a entrada livre obrigatória; os
	// objetos de verdade começam em linhas[2].
	linhas := strings.Split(s[inicio+6:], "\n")
	quantos := 0
	for _, l := range linhas[2:] {
		if !strings.HasSuffix(l, " n ") || len(l) < 10 {
			break
		}
		quantos++
		pos, err := strconv.Atoi(l[:10])
		if err != nil {
			t.Fatalf("posição ilegível: %q", l)
		}
		if pos <= 0 || pos >= len(bruto) {
			t.Fatalf("posição fora do arquivo: %d", pos)
		}
		esperado := strconv.Itoa(quantos) + " 0 obj"
		if !strings.HasPrefix(s[pos:], esperado) {
			t.Fatalf("a posição %d não aponta para o objeto %d, e sim para %q",
				pos, quantos, s[pos:min(pos+20, len(s))])
		}
	}
	if quantos < 5 {
		t.Errorf("poucos objetos na tabela: %d", quantos)
	}
	if !strings.Contains(s, "/Size "+strconv.Itoa(quantos+1)) {
		t.Errorf("o tamanho declarado no fim não bate com os %d objetos", quantos)
	}
}

func TestAcentoENoWinAnsi(t *testing.T) {
	if veio := escapar("Instalações"); !strings.Contains(veio, `\347`) { // ç
		t.Errorf("o ç não virou WinAnsi: %s", veio)
	}
	if veio := escapar("a(b)c\\d"); veio != `a\(b\)c\\d` {
		t.Errorf("os reservados não foram escapados: %s", veio)
	}
	if veio := escapar("日本"); veio != "??" {
		t.Errorf("fora do alfabeto devia virar ?, veio %q", veio)
	}
}

// O corte erra para cima de propósito: melhor cortar cedo do que invadir a
// coluna vizinha.
func TestTextoCortaAntesDeInvadirAColuna(t *testing.T) {
	longo := "LOJA 13 - SANTOS DUMONT E MAIS UM MONTE DE TEXTO QUE NÃO CABE"
	curto := cortar(longo, 60, 7.5)
	if largura(curto, 7.5) > 60 {
		t.Errorf("o corte passou da largura: %q", curto)
	}
	if !strings.HasSuffix(curto, "…") {
		t.Errorf("texto cortado devia terminar em reticências: %q", curto)
	}
	if cortar("curto", 200, 7.5) != "curto" {
		t.Error("cortou um texto que já cabia")
	}
}

func TestMoedaNoPadraoDaqui(t *testing.T) {
	for n, quer := range map[float64]string{
		94.44: "94,44", 1234.5: "1.234,50", 1234567.89: "1.234.567,89", 0.5: "0,50", -12.3: "-12,30",
	} {
		if veio := emMoeda(n); veio != quer {
			t.Errorf("emMoeda(%v) = %s, esperava %s", n, veio, quer)
		}
	}
}

// ---------------------------------------------------------------------------

func abrirFolha(t *testing.T, bruto []byte) string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(bruto), int64(len(bruto)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			r, _ := f.Open()
			defer r.Close()
			b, _ := io.ReadAll(r)
			return string(b)
		}
	}
	t.Fatal("a planilha não tem folha")
	return ""
}

func recorte(s, marca string) string {
	i := strings.Index(s, marca)
	if i < 0 {
		return "(não encontrado)"
	}
	return s[max(0, i-40):min(i+80, len(s))]
}

// Dumpar os arquivos para olhar de fora: `go test -run Dump -v`.
func TestDumpParaConferencia(t *testing.T) {
	if os.Getenv("DUMP") == "" {
		t.Skip("defina DUMP=1 para gravar os arquivos")
	}
	tab := tabelaDeTeste(80)
	tab.Aviso = "Mostrando as primeiras 80 de 1.377 — refine o filtro"
	x, _ := tab.Planilha()
	p, _ := tab.PDF()
	os.WriteFile("/tmp/relatorio.xlsx", x, 0o644)
	os.WriteFile("/tmp/relatorio.pdf", p, 0o644)
	t.Log("gravados em /tmp/relatorio.xlsx e /tmp/relatorio.pdf")
}

// A PRECISÃO DA DATA NA PLANILHA
//
//	09:05:00 saía do gerador como 09:04:59,981 porque a série era escrita com
//	seis casas — e seis casas de um DIA são 86 milésimos de segundo. O Excel
//	esconde isso ao exibir (o formato arredonda para o minuto), então o defeito
//	atravessava a conferência visual inteira.
//
//	Este teste desfaz a conta: pega o número escrito no XML e volta para hora,
//	minuto e segundo. É o mesmo caminho que um leitor de planilha de verdade
//	faz — e foi assim que o defeito apareceu (P-34).
func TestSerieDoExcelNaoPerdeOSegundo(t *testing.T) {
	casos := []struct{ h, m, s int }{
		{12, 45, 0},
		{9, 5, 0},
		{23, 59, 59},
		{0, 0, 1},
		{7, 33, 17},
	}
	for _, c := range casos {
		d := time.Date(2026, 8, 24, c.h, c.m, c.s, 0, FusoDaCasa())
		serie := serieDoExcel(d)

		// O caminho de volta, com o mesmo arredondamento que os leitores usam.
		segundos := int(math.Round((serie - math.Floor(serie)) * 86400))
		h, m, s := segundos/3600, (segundos/60)%60, segundos%60
		if h != c.h || m != c.m || s != c.s {
			t.Fatalf("%02d:%02d:%02d virou %02d:%02d:%02d (série %v)",
				c.h, c.m, c.s, h, m, s, serie)
		}
	}
}
