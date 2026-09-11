package administrativo

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

func TestExtrairCamposExtras_OCReal(t *testing.T) {
	e, err := ExtrairDoTexto(textoOCReal)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	casos := map[string]string{
		"emitente_razao":      e.EmitenteRazao,
		"emitente_cnpj":       e.EmitenteCNPJ,
		"responsavel_nome":    e.ResponsavelNome,
		"responsavel_email":   e.ResponsavelEmail,
		"fornecedor_email":    e.FornecedorEmail,
		"fornecedor_vendedor": e.FornecedorVendedor,
		"titulo":              e.TituloObra,
	}
	quer := map[string]string{
		"emitente_razao":      "FROTA MACEDO ENGENHARIA LTDA",
		"emitente_cnpj":       "27.363.223/0001-70",
		"responsavel_nome":    "FROTA MACEDO ENGENHARIA LTDA",
		"responsavel_email":   "compras@frotamacedo.com.br",
		"fornecedor_email":    "CONTABILIDADE@SVELETRICA.COM",
		"fornecedor_vendedor": "Contato principal",
		"titulo":              "MERCADINHOS SÃO LUIZ VILLAS - MSL VILLAS - AQUIRAZ",
	}
	for k, want := range quer {
		if casos[k] != want {
			t.Errorf("%s = %q, esperava %q", k, casos[k], want)
		}
	}
	if !strings.Contains(e.FaturamentoEndereco, "ROD. CE") {
		t.Errorf("endereço de faturamento = %q, deveria ter ROD. CE", e.FaturamentoEndereco)
	}
	if !strings.Contains(e.FornecedorEndereco, "Bezerra de Menezes") {
		t.Errorf("endereço do fornecedor = %q, deveria ter Bezerra de Menezes", e.FornecedorEndereco)
	}
	if !strings.Contains(e.EnderecoEntrega, "JACUND") && !strings.Contains(e.EnderecoEntrega, "CE 040") {
		t.Errorf("endereço de entrega = %q", e.EnderecoEntrega)
	}
	if e.EmitenteEndereco == "" {
		t.Error("endereço do emitente veio vazio")
	}
}

func TestExtrairDoTexto_ModeloDiferenteMesmasPalavras(t *testing.T) {
	// Outro layout, mesmas palavras-chave — sem colunas do Obra Prima.
	texto := `
ORDEM DE COMPRA 88001
LOJA DUNAS

DADOS DA ORDEM DE COMPRA 88001
Data: 01/09/2026
Previsao da entrega: 12/09/2026
Cond. pgto.: 2 parcelas
Forma pgto.: Pix
Observacao: urgente

RESPONSAVEL PELA COMPRA
Nome: FROTA MACEDO ENGENHARIA LTDA
Comprador: Igor Tostes
Email: compras@frotamacedo.com.br

DADOS DO FATURAMENTO
Nome: MERCADINHO DUNAS
CNPJ: 03720882000239
Endereco: Rua das Dunas, 10 - Fortaleza/CE

DADOS DO FORNECEDOR
Nome: FERRAGENS SANTA FE COMERCIO LTDA
CNPJ: 98765123000144
Telefone: 8533330000
E-mail: vendas@santafe.com
Endereco: Av. Central, 100

OBRA / CENTRO DE CUSTO: DUNAS - FORTALEZA
CNO:
ENDERECO ENTREGA:
Endereco: Rua das Dunas, 10
ENDERECO COBRANCA:
Endereco: Rua das Dunas, 10

N. Item                                                   Qtd.           Unit. (R$)       Subtotal (R$)       Desc. (R$)         Total (R$)
1     PARAFUSO FRANCES                                    10,00                   2,00              20,00            0,00               20,00
                                                          UN
                                                                   Subtotal                     20,00            0,00             20,00
                                                                   Frete                                                           0,00
                                                                   Total                                                          20,00
`
	e, err := ExtrairDoTexto(texto)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if e.Numero != "88001" {
		t.Errorf("numero = %q", e.Numero)
	}
	if e.CompradorCNPJ != "03720882000239" {
		t.Errorf("comprador_cnpj = %q", e.CompradorCNPJ)
	}
	if e.FornecedorCNPJ != "98765123000144" {
		t.Errorf("fornecedor_cnpj = %q", e.FornecedorCNPJ)
	}
	if e.FornecedorNome != "FERRAGENS SANTA FE COMERCIO LTDA" {
		t.Errorf("fornecedor_nome = %q", e.FornecedorNome)
	}
	if e.ObraCentroCusto != "DUNAS - FORTALEZA" {
		t.Errorf("obra = %q", e.ObraCentroCusto)
	}
	if e.CondPgto != "2 parcelas" {
		t.Errorf("cond = %q", e.CondPgto)
	}
	if e.FormaPgto != "Pix" {
		t.Errorf("forma = %q", e.FormaPgto)
	}
	if len(e.Itens) != 1 {
		t.Fatalf("itens = %d, %+v", len(e.Itens), e.Itens)
	}
	if e.Total.Float() != 20 {
		t.Errorf("total = %v", e.Total.Float())
	}
}

func TestRecalcularTotais(t *testing.T) {
	e := Extraida{
		Frete: regras.DinheiroDe(10),
		Itens: []ItemExtraido{
			{Descricao: "A", Qtd: 2, ValorUnit: regras.DinheiroDe(5), Desconto: regras.DinheiroDe(1)},
			{Descricao: "B", Qtd: 3, ValorUnit: regras.DinheiroDe(4), Desconto: 0},
		},
	}
	e.RecalcularTotais()
	if e.Itens[0].Total.Float() != 9 {
		t.Errorf("item0 total = %v, esperava 9", e.Itens[0].Total.Float())
	}
	if e.Subtotal.Float() != 22 {
		t.Errorf("subtotal = %v, esperava 22", e.Subtotal.Float())
	}
	if e.Desconto.Float() != 1 {
		t.Errorf("desconto = %v", e.Desconto.Float())
	}
	if e.Total.Float() != 31 {
		t.Errorf("total = %v, esperava 31", e.Total.Float())
	}
}

func TestDesenharOC_ContemCamposEFechaConta(t *testing.T) {
	e, err := ExtrairDoTexto(textoOCReal)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := desenharOC(e)
	if err != nil {
		t.Fatalf("desenhar: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("não saiu um PDF")
	}
	if !bytes.Contains(pdf, []byte("ORDEM DE COMPRA 019731")) {
		t.Error("o PDF não tem o número da OC")
	}
	if !bytes.Contains(pdf, []byte("DADOS DO FATURAMENTO")) {
		t.Error("o PDF não tem DADOS DO FATURAMENTO")
	}
	if !bytes.Contains(pdf, []byte("DADOS DO FORNECEDOR")) {
		t.Error("o PDF não tem DADOS DO FORNECEDOR")
	}
	if bytes.Contains(pdf, []byte("FrotaHub")) || bytes.Contains(pdf, []byte("gerado em")) {
		t.Error("o PDF da OC não pode levar marca FrotaHub")
	}
}

func TestDesenharOC_RoundTripLer(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext ausente — round-trip Ler() fica para o servidor")
	}
	e, err := ExtrairDoTexto(textoOCReal)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := desenharOC(e)
	if err != nil {
		t.Fatal(err)
	}
	lida, err := Ler(context.Background(), pdf)
	if err != nil {
		t.Fatalf("Ler do PDF gerado: %v", err)
	}
	if lida.Numero != e.Numero {
		t.Errorf("numero ida-e-volta: %q vs %q", lida.Numero, e.Numero)
	}
	if lida.CompradorCNPJ != e.CompradorCNPJ {
		t.Errorf("comprador_cnpj: %q vs %q", lida.CompradorCNPJ, e.CompradorCNPJ)
	}
	if lida.FornecedorCNPJ != e.FornecedorCNPJ {
		t.Errorf("fornecedor_cnpj: %q vs %q", lida.FornecedorCNPJ, e.FornecedorCNPJ)
	}
	if len(lida.Itens) != len(e.Itens) {
		t.Errorf("itens: %d vs %d", len(lida.Itens), len(e.Itens))
	}
	if lida.Total != e.Total {
		t.Errorf("total: %s vs %s", lida.Total.Reais(), e.Total.Reais())
	}
}

func TestJSONDocumento_IdaEVolta(t *testing.T) {
	e, err := ExtrairDoTexto(textoOCReal)
	if err != nil {
		t.Fatal(err)
	}
	e.RecalcularTotais()
	volta := jsonParaExtraida(extraidaParaJSON(e))
	if volta.Numero != e.Numero || volta.CompradorCNPJ != e.CompradorCNPJ || volta.FornecedorCNPJ != e.FornecedorCNPJ {
		t.Errorf("números/CNPJs divergiram: %+v vs origem %s/%s/%s", volta.Numero, e.Numero, e.CompradorCNPJ, e.FornecedorCNPJ)
	}
	if len(volta.Itens) != len(e.Itens) {
		t.Fatalf("itens %d vs %d", len(volta.Itens), len(e.Itens))
	}
	if volta.Total != e.Total {
		t.Errorf("total %s vs %s", volta.Total.Reais(), e.Total.Reais())
	}
}

func TestCNPJVazioNaoPegaCEP(t *testing.T) {
	texto := `
DADOS DA ORDEM DE COMPRA 20016
Data: 05/09/2026
Previsão da entrega: 15/09/2026
Cond. pgto.: 1 parcela
Forma pgto.: Boleto
Comprador: Igor Tostes

DADOS DO FATURAMENTO
Nome: MERCADINHOS SÃO LUIZ VILLAS
CNPJ: 03720882003920
Endereco: Aquiraz - CE, 61700-000

DADOS DO FORNECEDOR
Nome: S V COMERCIO DE MATERIAL ELETRICO LTDA
CNPJ:
Endereço: Avenida Bezerra de Menezes, 420 - Farias Brito, Fortaleza - CE, 60325-000

OBRA/CENTRO DE CUSTO: MSL VILLAS - AQUIRAZ

N. Item                                                   Qtd.           Unit. (R$)       Subtotal (R$)       Desc. (R$)         Total (R$)
1     PARAFUSO FRANCES                                    10,00                   2,00              20,00            0,00               20,00
                                                          UN
                                                                   Subtotal                     20,00            0,00             20,00
                                                                   Frete                                                           0,00
                                                                   Total                                                          20,00
`
	e, err := ExtrairDoTexto(texto)
	if err != nil {
		t.Fatal(err)
	}
	if e.FornecedorCNPJ != "" {
		t.Errorf("fornecedor_cnpj = %q, não podia pegar o CEP do endereço", e.FornecedorCNPJ)
	}
	if e.CompradorCNPJ != "03720882003920" {
		t.Errorf("comprador_cnpj = %q", e.CompradorCNPJ)
	}
}
