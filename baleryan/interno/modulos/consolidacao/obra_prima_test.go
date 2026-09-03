// rev 1 — as travas do leitor do Obra Prima
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	O formato do CSV não é escolha nossa (Latin-1, ";", número brasileiro, sete
//	linhas de lixo antes do cabeçalho de verdade) — é o que o Obra Prima
//	exporta, e a amostra real (Documentos_a_pagar_7.csv, 02/09/2026, 157
//	linhas) é o que moldou cada teste aqui. A garantia mais importante não é de
//	formato: é que uma nota parcelada não pode virar dinheiro em dobro (ver
//	TestParseRecusaBrutoInconsistente e TestResumoSomaPorNotaNaoPorLinha).
package consolidacao

import (
	"strings"
	"testing"
)

// paraLatin1 é o inverso de deLatin1: codifica cada rune num byte só. Existe
// para o teste montar um arquivo de amostra sem depender de o editor salvar
// em Latin-1 — os testes escrevem UTF-8 normal, e esta função converte antes
// de entregar a `parseObraPrima`, exatamente como ela vai chegar de verdade.
func paraLatin1(t *testing.T, s string) []byte {
	t.Helper()
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 255 {
			t.Fatalf("caractere %q não cabe em Latin-1 — ajuste a amostra do teste", r)
		}
		b = append(b, byte(r))
	}
	return b
}

// amostra monta um CSV no formato real do Obra Prima: sete linhas de
// cabeçalho de empresa, a linha de colunas, e as linhas de dados dadas.
func amostra(linhasDeDados ...string) string {
	cabecalho := []string{
		";FROTA MACEDO ENGENHARIA LTDA;;;;;;;02/09/2026;;;;;;;",
		";Rua Exemplo, 295;;;;;;;;;;;;;;",
		";85 0000-0000 - CNPJ: 00.000.000/0001-00;;;;;;;;;;;;;;",
		";;;;;;;;;;;;;;;",
		";Relatório lista de documentos a pagar;;;;;;;;;;;;;;",
		";;;;;;;;;;;;;;;",
		"Empresa;;Doc.;Tipo;Núm.;Parc.;Obra;Fornecedor;;Venc.;Bruto (R$);Líquido (R$);Data Pgto.;Vlr. pago (R$);Situação;Desc.",
	}
	todas := append(append([]string{}, cabecalho...), linhasDeDados...)
	return strings.Join(todas, "\r\n") + "\r\n"
}

func linhaCSV(doc, num, parc, fornecedor, venc, bruto, liquido, dataPgto, valorPago, situacao, desc string) string {
	return strings.Join([]string{
		"FROTA MACEDO ENGENHARIA LTDA", "", doc, "Provisão", num, parc,
		"GASTOS MATERIAIS CONTRATO MANUTENÇÃO", fornecedor, "", venc, bruto, liquido,
		dataPgto, valorPago, situacao, desc,
	}, ";")
}

func TestParseLeUmaNotaSimples(t *testing.T) {
	csv := amostra(
		linhaCSV("087056", "9220", "1/1", "Rodrigues Material de Construção Ltda",
			"28/08/2026", "15.424,86", "15.424,86", "28/08/2026", "15.424,86", "Liquidado",
			"GASTOS MATERIAIS DIA 23/07 ATÉ DIA 31/07/2026"),
	)
	linhas, err := parseObraPrima(paraLatin1(t, csv))
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if len(linhas) != 1 {
		t.Fatalf("esperava 1 linha, vieram %d", len(linhas))
	}
	l := linhas[0]
	if l.Doc != "087056" || l.Num != "9220" || l.Parc != "1/1" {
		t.Errorf("doc/num/parc saíram %q/%q/%q", l.Doc, l.Num, l.Parc)
	}
	if l.Fornecedor != "Rodrigues Material de Construção Ltda" {
		t.Errorf("fornecedor saiu %q — o acento sobreviveu à conversão de Latin-1?", l.Fornecedor)
	}
	if l.Bruto.Float() != 15424.86 {
		t.Errorf("bruto saiu %v, esperava 15424.86", l.Bruto.Float())
	}
	if l.Vencimento != "2026-08-28" {
		t.Errorf("vencimento saiu %q, esperava 2026-08-28", l.Vencimento)
	}
	if l.Situacao != "Liquidado" {
		t.Errorf("situação saiu %q", l.Situacao)
	}
}

// O OBRA PRIMA ÀS VEZES EXPORTA "9214'" EM VEZ DE "9214"
//
//	Medido na amostra real (Documentos_a_pagar_7.csv, 02/09/2026, doc. 087456):
//	o "Núm." veio "9214'", com um apóstrofo grudado no fim — defeito da
//	exportação, confirmado pelo dono em 03/09/2026 (é a mesma nota 9214 da
//	associação manual de tickets). Sem limpar, essa nota nunca bateria com
//	`obra_prima_ticket` por causa de um caractere.
func TestParseTiraApostrofoDoFinalDoNumero(t *testing.T) {
	csv := amostra(
		linhaCSV("087456", "9214'", "1/1", "Rodrigues Material de Construção Ltda",
			"28/08/2026", "1.119,06", "1.119,06", "28/08/2026", "1.119,06", "Liquidado",
			"REF A PREVENTIVA"),
	)
	linhas, err := parseObraPrima(paraLatin1(t, csv))
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if linhas[0].Num != "9214" {
		t.Errorf("num saiu %q, esperava 9214 (apóstrofo do fim deveria ter sumido)", linhas[0].Num)
	}
}

// A AMOSTRA REAL TEM DUAS NOTAS PARCELADAS QUE SE COMPORTAM ASSIM
//
//	660575 (3 parcelas) e 19702 (2 parcelas): o `Bruto` vem IDÊNTICO em cada
//	parcela — é o valor do documento inteiro, repetido, não dividido. Somar por
//	linha inflaria a nota por 2x/3x.
func TestParseNotaParceladaTemBrutoRepetidoNaoSomado(t *testing.T) {
	csv := amostra(
		linhaCSV("087536", "660575", "1/3", "S V Comércio", "29/08/2026", "2.100,00", "2.100,00", "", "0,00", "Aberto", ""),
		linhaCSV("087536", "660575", "2/3", "S V Comércio", "29/09/2026", "2.100,00", "2.100,00", "", "0,00", "Aberto", ""),
		linhaCSV("087536", "660575", "3/3", "S V Comércio", "29/10/2026", "2.100,00", "2.100,00", "", "0,00", "Aberto", ""),
	)
	linhas, err := parseObraPrima(paraLatin1(t, csv))
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if len(linhas) != 3 {
		t.Fatalf("esperava 3 linhas (uma por parcela), vieram %d", len(linhas))
	}
	resumo := resumoDaImportacao("teste.csv", linhas)
	if resumo.Notas != 1 {
		t.Errorf("esperava 1 nota distinta, saiu %d", resumo.Notas)
	}
	if resumo.Valor != 2100.00 {
		t.Errorf("o resumo somou %v — a nota de 3 parcelas de R$2.100 virou R$%.2f, "+
			"deveria continuar R$2.100 (bruto é do documento, não da parcela)", resumo.Valor, resumo.Valor)
	}
}

// A GARANTIA CENTRAL: BRUTO DIVERGENTE NA MESMA NOTA DERRUBA O ARQUIVO INTEIRO
func TestParseRecusaBrutoInconsistente(t *testing.T) {
	csv := amostra(
		linhaCSV("087536", "660575", "1/2", "S V Comércio", "29/08/2026", "2.100,00", "2.100,00", "", "0,00", "Aberto", ""),
		linhaCSV("087536", "660575", "2/2", "S V Comércio", "29/09/2026", "1.050,00", "1.050,00", "", "0,00", "Aberto", ""),
	)
	_, err := parseObraPrima(paraLatin1(t, csv))
	if err == nil {
		t.Fatal("a nota 660575 discorda de si mesma (2.100,00 vs 1.050,00) e o parser aceitou")
	}
	if !strings.Contains(err.Error(), "660575") {
		t.Errorf("o erro não cita a nota culpada: %v", err)
	}
}

func TestParseRecusaColunaFaltando(t *testing.T) {
	semNum := strings.Replace(amostra(), "Núm.", "Numero da Nota", 1)
	_, err := parseObraPrima(paraLatin1(t, semNum))
	if err == nil {
		t.Fatal("o cabeçalho não tem \"Núm.\" e o parser aceitou mesmo assim")
	}
	if !strings.Contains(err.Error(), "Núm.") {
		t.Errorf("o erro não diz qual coluna faltou: %v", err)
	}
}

func TestParseRecusaArquivoSemLinhaDeCabecalho(t *testing.T) {
	_, err := parseObraPrima(paraLatin1(t, "isto;nao;e;um;csv;do;obra;prima\r\n"))
	if err == nil {
		t.Fatal("não existe linha \"Empresa;...;Doc.;...;Núm.\" nenhuma e o parser aceitou")
	}
}

func TestParseRecusaArquivoSemLinhaDeDados(t *testing.T) {
	_, err := parseObraPrima(paraLatin1(t, amostra()))
	if err == nil {
		t.Fatal("o arquivo só tem cabeçalho, nenhuma nota — deveria recusar, não devolver lista vazia")
	}
}

func TestParseIgnoraLinhaEmBrancoNoFim(t *testing.T) {
	csv := amostra(
		linhaCSV("087056", "9220", "1/1", "Rodrigues", "28/08/2026", "15.424,86", "15.424,86", "28/08/2026", "15.424,86", "Liquidado", ""),
		";;;;;;;;;;;;;;;",
		"",
	)
	linhas, err := parseObraPrima(paraLatin1(t, csv))
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if len(linhas) != 1 {
		t.Fatalf("a linha em branco no fim não é dado — esperava 1 linha, vieram %d", len(linhas))
	}
}

func TestParseRecusaLinhaSemNumeroDaNota(t *testing.T) {
	semNum := linhaCSV("087056", "", "1/1", "Rodrigues", "28/08/2026", "15.424,86", "15.424,86", "", "0,00", "Aberto", "")
	_, err := parseObraPrima(paraLatin1(t, amostra(semNum)))
	if err == nil {
		t.Fatal("a linha não tem número de nota — sem ele não dá para casar com ticket nenhum, e o parser aceitou")
	}
}

func TestParseRecusaBrutoInvalido(t *testing.T) {
	brutoRuim := linhaCSV("087056", "9220", "1/1", "Rodrigues", "28/08/2026", "quinze mil", "15.424,86", "", "0,00", "Aberto", "")
	_, err := parseObraPrima(paraLatin1(t, amostra(brutoRuim)))
	if err == nil {
		t.Fatal("\"quinze mil\" não é um valor em reais, e o parser aceitou")
	}
}

func TestDinheiroBRConverteFormatoBrasileiro(t *testing.T) {
	casos := []struct {
		texto   string
		reais   float64
		comErro bool
	}{
		{texto: "15.424,86", reais: 15424.86},
		{texto: "0,00", reais: 0},
		{texto: "5,40", reais: 5.40},
		{texto: "1.234.567,89", reais: 1234567.89},
		{texto: "abc", comErro: true},
		{texto: "", comErro: true},
	}
	for _, c := range casos {
		d, err := dinheiroBR(c.texto)
		if c.comErro {
			if err == nil {
				t.Errorf("dinheiroBR(%q): esperava erro, saiu %v", c.texto, d.Float())
			}
			continue
		}
		if err != nil {
			t.Errorf("dinheiroBR(%q): erro inesperado: %v", c.texto, err)
			continue
		}
		if d.Float() != c.reais {
			t.Errorf("dinheiroBR(%q) = %v, esperava %v", c.texto, d.Float(), c.reais)
		}
	}
}

func TestDataBRConverteParaISO(t *testing.T) {
	iso, err := dataBR("28/08/2026")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if iso != "2026-08-28" {
		t.Errorf("saiu %q, esperava 2026-08-28", iso)
	}
	if _, err := dataBR("32/13/2026"); err == nil {
		t.Error("32/13/2026 não é uma data e foi aceita")
	}
}

// paraGravar É QUEM DECIDE O CLIENTE — NÃO O CSV
//
//	O arquivo não traz cliente nenhum; quem grava sabe de quem é o login que
//	mandou o upload. Uma linha gravada com o `cliente_id` errado é a nota de um
//	contrato aparecendo na consolidação de outro.
func TestParaGravarUsaClienteDaRequisicaoNaoDoArquivo(t *testing.T) {
	csv := amostra(
		linhaCSV("087056", "9220", "1/1", "Rodrigues", "28/08/2026", "15.424,86", "15.424,86", "28/08/2026", "15.424,86", "Liquidado", ""),
	)
	linhas, err := parseObraPrima(paraLatin1(t, csv))
	if err != nil {
		t.Fatal(err)
	}
	gravar := paraGravar("cliente-do-sao-luiz", "Documentos_a_pagar_7.csv", linhas)
	if len(gravar) != 1 {
		t.Fatalf("esperava 1 linha para gravar, saiu %d", len(gravar))
	}
	if gravar[0].ClienteID != "cliente-do-sao-luiz" {
		t.Errorf("cliente_id saiu %q", gravar[0].ClienteID)
	}
	if gravar[0].Arquivo != "Documentos_a_pagar_7.csv" {
		t.Errorf("arquivo de origem saiu %q — some a auditoria de qual carga trouxe a linha", gravar[0].Arquivo)
	}
	if gravar[0].Vencimento == nil || *gravar[0].Vencimento != "2026-08-28" {
		t.Errorf("vencimento não atravessou paraGravar corretamente: %+v", gravar[0].Vencimento)
	}
}

// UMA LINHA SEM DATA DE PAGAMENTO VIRA NIL, NÃO STRING VAZIA
//
//	`""` gravaria uma data vazia no banco (erro de tipo); nil grava NULL, que é
//	o que "ainda não foi paga" realmente significa.
func TestParaGravarDataEmBrancoViraNilNaoStringVazia(t *testing.T) {
	aberta := linhaCSV("087385", "9270", "1/1", "Rodrigues", "15/09/2026", "12.816,33", "12.816,33", "", "0,00", "Aberto", "")
	linhas, err := parseObraPrima(paraLatin1(t, amostra(aberta)))
	if err != nil {
		t.Fatal(err)
	}
	gravar := paraGravar("c1", "arq.csv", linhas)
	if gravar[0].DataPagamento != nil {
		t.Errorf("data de pagamento em branco no CSV virou %q — deveria ser nil (NULL)", *gravar[0].DataPagamento)
	}
	if gravar[0].ValorPago == nil || *gravar[0].ValorPago != 0 {
		t.Errorf("vlr. pago \"0,00\" deveria virar 0, saiu %+v", gravar[0].ValorPago)
	}
}
