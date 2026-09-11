// rev 1 — a leitura da OC provada contra um PDF real
//
// `testdata/oc_real_019731.txt` NÃO é sintético: é a saída literal de
// `pdftotext -layout -enc UTF-8 -eol unix` (poppler 25.07.0, o mesmo
// binário que `apk add poppler-utils` instala) rodada sobre uma Ordem de
// Compra de verdade (019731, Obra Prima, 10/09/2026, 2 páginas, 10 itens).
// Os valores esperados abaixo foram conferidos à mão contra o PDF original
// — inclusive a soma dos 10 itens batendo com o Subtotal/Total impressos.
package administrativo

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

//go:embed testdata/oc_real_019731.txt
var textoOCReal string

func TestExtrairDoTexto_OCReal(t *testing.T) {
	e, err := ExtrairDoTexto(textoOCReal)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}

	casos := map[string]struct{ got, want string }{
		"numero":            {e.Numero, "019731"},
		"data":              {e.Data, "2026-09-02"},
		"previsao_entrega":  {e.PrevisaoEntrega, "2026-09-10"},
		"cond_pgto":         {e.CondPgto, "1 parcela"},
		"forma_pgto":        {e.FormaPgto, "Boleto"},
		"comprador_interno": {e.CompradorInterno, "Nadyson Ferreira"},
		"comprador_nome":    {e.CompradorNome, "MERCADINHOS SÃO LUIZ VILLAS"},
		"comprador_cnpj":    {e.CompradorCNPJ, "03720882003920"},
		"fornecedor_nome":   {e.FornecedorNome, "S V COMERCIO DE MATERIAL ELETRICO LTDA"},
		"fornecedor_cnpj":   {e.FornecedorCNPJ, "35088657000137"},
		"obra_centro_custo": {e.ObraCentroCusto, "MSL VILLAS - AQUIRAZ"},
	}
	for campo, c := range casos {
		if c.got != c.want {
			t.Errorf("%s = %q, esperava %q", campo, c.got, c.want)
		}
	}

	if e.Subtotal.Float() != 415.90 {
		t.Errorf("subtotal = %v, esperava 415.90", e.Subtotal.Float())
	}
	if e.Desconto.Float() != 0 {
		t.Errorf("desconto = %v, esperava 0", e.Desconto.Float())
	}
	if e.Frete.Float() != 0 {
		t.Errorf("frete = %v, esperava 0", e.Frete.Float())
	}
	if e.Total.Float() != 415.90 {
		t.Errorf("total = %v, esperava 415.90", e.Total.Float())
	}

	if len(e.Itens) != 10 {
		t.Fatalf("esperava 10 itens, vieram %d: %+v", len(e.Itens), e.Itens)
	}

	// OS 10 ITENS, UM A UM — NÃO SÓ UMA AMOSTRA
	//
	//	Uma primeira versão deste teste conferia só os itens 1, 6 e 10, mais a
	//	soma total. Ela passou com dois defeitos reais escondidos nos itens do
	//	meio: a unidade "UN" quando vem SOZINHA na linha de continuação (não
	//	grudada com texto, como no item 1) virava parte da descrição; e o item
	//	8, o último antes da quebra de página, engolia o cabeçalho inteiro
	//	repetido no topo da página 2. A soma dos totais continuava batendo os
	//	dois jeitos — Total não depende de descrição — por isso o teste
	//	"passava" com a leitura errada. Cada item, individualmente, é o que
	//	teria pegado isso de cara.
	esperados := []ItemExtraido{
		{Descricao: `ABRAÇADEIRA PVC WETZEL 3/4" - BRANCO`, Qtd: 20, Unidade: "UN"},
		{Descricao: `BUCHA NYLON S-8 COM PARAFUSO`, Qtd: 50, Unidade: "UN"},
		{Descricao: `CONDULETE PVC 3/4" BRANCO - WETZEL`, Qtd: 3, Unidade: "UN"},
		{Descricao: `CONECTOR BOX PVC 3/4" BRANCO - WETZEL`, Qtd: 6, Unidade: "UN"},
		{Descricao: `CURVA 90º WETZEL BRANCO 3/4`, Qtd: 2, Unidade: "UN"},
		{Descricao: `DISJUNTOR DR 25A - DR bipolar sheneider 25A`, Qtd: 2, Unidade: "UN"},
		{Descricao: `DISJUNTOR MONOPOLAR 20A - sheneider`, Qtd: 2, Unidade: "UN"},
		{Descricao: `ELETRODUTO DE PVC RIGIDO WETZEL BRANCO 3/4''`, Qtd: 5, Unidade: "UN"},
		{Descricao: `LUVA PVC 3/4" BRANCO - WETZEL`, Qtd: 5, Unidade: "UN"},
		{Descricao: `TOMADA BRANCA 2P + T DE 20A PARA CONDULETE BRANCA - com tampa para condulete pvc Branco wetzel3/4`, Qtd: 2, Unidade: "Unidades"},
	}
	valoresUnit := []float64{0.77, 0.19, 4.38, 0.97, 2.19, 125.53, 9.15, 12.98, 1.16, 13.80}
	totais := []float64{15.40, 9.50, 13.14, 5.82, 4.38, 251.06, 18.30, 64.90, 5.80, 27.60}

	for i, esperado := range esperados {
		got := e.Itens[i]
		n := i + 1
		if got.Descricao != esperado.Descricao {
			t.Errorf("item %d: descrição = %q, esperava %q", n, got.Descricao, esperado.Descricao)
		}
		if got.Qtd != esperado.Qtd {
			t.Errorf("item %d: qtd = %v, esperava %v", n, got.Qtd, esperado.Qtd)
		}
		if got.Unidade != esperado.Unidade {
			t.Errorf("item %d: unidade = %q, esperava %q", n, got.Unidade, esperado.Unidade)
		}
		if got.ValorUnit.Float() != valoresUnit[i] {
			t.Errorf("item %d: valor unit = %v, esperava %v", n, got.ValorUnit.Float(), valoresUnit[i])
		}
		if got.Total.Float() != totais[i] {
			t.Errorf("item %d: total = %v, esperava %v", n, got.Total.Float(), totais[i])
		}
		if got.Desconto.Float() != 0 {
			t.Errorf("item %d: desconto = %v, esperava 0", n, got.Desconto.Float())
		}
	}

	// A SOMA DOS 10 ITENS TEM QUE BATER COM O SUBTOTAL IMPRESSO
	//
	//	Não é coincidência — é a mesma garantia que `leitor.ContaFecha` cobra
	//	das notas fiscais. Uma leitura de itens que não fecha com o total da
	//	OC é uma leitura errada, mesmo que cada item pareça razoável sozinho.
	//	A soma é em CENTAVOS (regras.Dinheiro), não em float64 — comparar
	//	float diretamente é o mesmo erro de arredondamento que o pacote
	//	`regras` existe para evitar (P-12).
	var soma regras.Dinheiro
	for _, it := range e.Itens {
		soma += it.Total
	}
	if soma != e.Subtotal {
		t.Errorf("soma dos itens = %s, subtotal impresso = %s — não fecha", soma.Reais(), e.Subtotal.Reais())
	}

	if motivos := e.MotivosDeRejeicao(); len(motivos) != 0 {
		t.Errorf("esta OC deveria passar nos dois filtros, mas foi rejeitada: %v", motivos)
	}
}

func TestMotivosDeRejeicao_CNPJDeFaturamentoErrado(t *testing.T) {
	// A mesma OC real, só que faturando para outro grupo — não a raiz fixa
	// que a migração 059 exige.
	texto := strings.Replace(textoOCReal, "03720882003920", "12345678000199", 1)
	e, err := ExtrairDoTexto(texto)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	motivos := e.MotivosDeRejeicao()
	if len(motivos) != 1 {
		t.Fatalf("esperava 1 motivo de rejeição, vieram %d: %v", len(motivos), motivos)
	}
	if !strings.Contains(motivos[0], "12345678000199") {
		t.Errorf("o motivo não cita o CNPJ errado: %q", motivos[0])
	}
}

func TestMotivosDeRejeicao_FornecedorSemCNPJ(t *testing.T) {
	// Tira só o número do CNPJ do fornecedor (token único no documento — não
	// se confunde com o CNPJ do faturamento) — o resto continua igual.
	texto := strings.Replace(textoOCReal, "35088657000137", "", 1)
	e, err := ExtrairDoTexto(texto)
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if e.FornecedorCNPJ != "" {
		t.Fatalf("a troca não tirou o CNPJ do fornecedor do texto — ajuste o teste (achei %q)", e.FornecedorCNPJ)
	}
	motivos := e.MotivosDeRejeicao()
	if len(motivos) != 1 {
		t.Fatalf("esperava 1 motivo de rejeição, vieram %d: %v", len(motivos), motivos)
	}
	if !strings.Contains(motivos[0], "fornecedor") {
		t.Errorf("o motivo não fala do fornecedor: %q", motivos[0])
	}
}

func TestExtrairDoTexto_SemNumeroRecusaAltoLogo(t *testing.T) {
	texto := strings.Replace(textoOCReal, "DADOS DA ORDEM DE COMPRA", "ISTO NAO E UMA OC", 1)
	_, err := ExtrairDoTexto(texto)
	if err == nil {
		t.Fatal("o texto não tem \"DADOS DA ORDEM DE COMPRA\" nenhum, e a leitura aceitou mesmo assim")
	}
}

func TestExtrairDoTexto_TextoVazio(t *testing.T) {
	if _, err := ExtrairDoTexto(""); err == nil {
		t.Fatal("texto vazio deveria recusar, não devolver uma OC zerada")
	}
	if _, err := ExtrairDoTexto("   \n\n  "); err == nil {
		t.Fatal("texto só com espaço deveria recusar")
	}
}

// FRETE E DESCONTO DIFERENTES DE ZERO
//
//	A amostra real tem os dois em 0,00 — o caso mais comum, mas não prova que
//	o regex do rodapé lê um valor de verdade quando ele existe. O rodapé é
//	testado isolado, como as 3 linhas de verdade que `pdftotext -layout`
//	produziu (copiadas do fixture real), só com os números trocados — não
//	precisa do documento inteiro para provar o parser do rodapé.
func TestExtrairTotais_FreteEDescontoDiferentesDeZero(t *testing.T) {
	rodape := "                                                                   Subtotal                    415,90           20,00            395,90\n" +
		"                                                                   Frete                                                          10,00\n" +
		"                                                                   Total                                                         405,90\n"
	var e Extraida
	extrairTotais(rodape, &e)
	if e.Subtotal.Float() != 415.90 {
		t.Errorf("subtotal = %v, esperava 415.90", e.Subtotal.Float())
	}
	if e.Desconto.Float() != 20.00 {
		t.Errorf("desconto = %v, esperava 20.00", e.Desconto.Float())
	}
	if e.Frete.Float() != 10.00 {
		t.Errorf("frete = %v, esperava 10.00", e.Frete.Float())
	}
	if e.Total.Float() != 405.90 {
		t.Errorf("total = %v, esperava 405.90", e.Total.Float())
	}
}

func TestDataBR(t *testing.T) {
	if got := dataBR("02/09/2026"); got != "2026-09-02" {
		t.Errorf("dataBR(02/09/2026) = %q", got)
	}
	if got := dataBR("data inválida"); got != "" {
		t.Errorf("dataBR de lixo deveria devolver vazio, deu %q", got)
	}
}

func TestDinheiroBR(t *testing.T) {
	casos := map[string]float64{
		"1.234,56": 1234.56,
		"0,77":     0.77,
		"415,90":   415.90,
	}
	for texto, esperado := range casos {
		if got := dinheiroBR(texto).Float(); got != esperado {
			t.Errorf("dinheiroBR(%q) = %v, esperava %v", texto, got, esperado)
		}
	}
}

func TestCompradorCNPJParaBanco(t *testing.T) {
	if got := compradorCNPJParaBanco(""); got != nil {
		t.Errorf("vazio deveria ser nil, veio %v", got)
	}
	if got := compradorCNPJParaBanco("03720882003920"); got != "03720882003920" {
		t.Errorf("CNPJ válido deveria gravar, veio %v", got)
	}
	if got := compradorCNPJParaBanco("45612378000123"); got != nil {
		t.Errorf("CNPJ fora da raiz deve ser nil (CHECK do banco), veio %v", got)
	}
}
