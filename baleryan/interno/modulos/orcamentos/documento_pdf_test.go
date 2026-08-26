// rev 1 — o orçamento como documento
//
//	Estes testes não conferem beleza: conferem que o documento SAI, que ele fecha
//	a conta, e que os campos que faltam somem em vez de virarem rótulo vazio. A
//	aparência se confere com o olho — `olho_test.go` escreve o arquivo para isso.
package orcamentos

import (
	"bytes"
	"strings"
	"testing"
)

func emitenteDeTeste() *dadosDoEmitente {
	return &dadosDoEmitente{
		Razao: "FROTA MACEDO ENGENHARIA LTDA", CNPJ: "27.363.223/0001-70",
		Endereco: "Rua Tal, 295", Contato: "(85) 2181-1386",
		Pagamento: "Transferência Bancária 30 dias", Validade: 7,
		Observacao: []string{"Orçamento válido por 7 (sete) dias."},
	}
}

func cabecaDeTeste() map[string]any {
	return map[string]any{"ticket": 131088, "parte": 1, "loja": "Cocó",
		"criado_em": "2026-08-19T12:00:00Z"}
}

func umItem(desc string, qtd, unit, total float64) itemDoOrcamento {
	un := "UN"
	return itemDoOrcamento{Ordem: 1, Descricao: desc, Unidade: &un,
		Quantidade: qtd, Cobrado: unit, Total: total}
}

func TestODocumentoSaiEFecha(t *testing.T) {
	pdf, err := desenharOrcamento(cabecaDeTeste(),
		[]itemDoOrcamento{umItem("LAMPADA", 2, 27.48, 54.96)},
		emitenteDeTeste(), dadosDoTomador{Nome: "Mercadinhos São Luiz — Cocó"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Error("não começa como PDF")
	}
	// A tabela de posições no fim é a única coisa que, errada, faz o arquivo
	// não abrir em leitor nenhum.
	if !bytes.Contains(pdf, []byte("\nxref\n")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Error("o arquivo não fecha com xref e EOF")
	}
	if n := bytes.Count(pdf, []byte("/Type /Page /Parent")); n != 1 {
		t.Errorf("gerou %d páginas para um item, esperava 1", n)
	}
}

// O TOTAL É A SOMA DOS ITENS
//
//	Se ele viesse do campo `valor` do orçamento, uma divergência entre os dois
//	sairia escondida — e o documento mostraria linhas que não somam o total
//	impresso. É a primeira coisa que uma auditoria confere.
func TestOTotalVemDosItens(t *testing.T) {
	pdf, err := desenharOrcamento(cabecaDeTeste(), []itemDoOrcamento{
		umItem("A", 1, 10, 10.00),
		umItem("B", 1, 5.50, 5.50),
	}, emitenteDeTeste(), dadosDoTomador{Nome: "Loja"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	// 15,50 no número e por extenso — os dois nascem da mesma soma.
	if !bytes.Contains(pdf, []byte("15,50")) {
		t.Error("o total de 15,50 não aparece")
	}
	if !bytes.Contains(pdf, []byte("quinze reais e cinquenta centavos")) {
		t.Error("o extenso não bate com o total")
	}
}

// CAMPO QUE FALTA SOME — NÃO VIRA RÓTULO VAZIO
//
//	Nenhuma das 38 unidades tinha CNPJ em 26/08/2026. "CNPJ:" seguido de nada é
//	pior que a ausência da linha: parece defeito do documento, e quem recebe
//	desconfia do resto.
func TestTomadorSemCNPJNaoImprimeRotuloVazio(t *testing.T) {
	pdf, err := desenharOrcamento(cabecaDeTeste(),
		[]itemDoOrcamento{umItem("A", 1, 10, 10)},
		emitenteDeTeste(), dadosDoTomador{Nome: "Loja sem cadastro"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	// O do PRESTADOR existe sempre; o do tomador, não. Então "CNPJ:" tem que
	// aparecer uma vez só.
	if n := bytes.Count(pdf, []byte("(CNPJ:)")); n != 1 {
		t.Errorf("o rótulo CNPJ apareceu %d vezes, esperava 1 (só o do prestador)", n)
	}
	if bytes.Contains(pdf, []byte("(Cidade/Estado:)")) {
		t.Error("imprimiu Cidade/Estado sem ter cidade")
	}
}

// MARCA ILEGÍVEL NÃO DERRUBA O DOCUMENTO
//
//	O orçamento vale pelo conteúdo. Uma exceção aqui deixaria o cliente sem
//	documento nenhum por causa de um logotipo.
func TestMarcaQuebradaNaoDerrubaODocumento(t *testing.T) {
	emi := emitenteDeTeste()
	lixo := "isto não é base64 de JPEG nenhum"
	emi.MarcaB64 = &lixo
	if _, err := desenharOrcamento(cabecaDeTeste(),
		[]itemDoOrcamento{umItem("A", 1, 10, 10)}, emi, dadosDoTomador{Nome: "Loja"}); err != nil {
		t.Fatalf("a marca quebrada derrubou o documento: %v", err)
	}
}

// A REVISÃO SÓ APARECE QUANDO EXISTE
//
//	Escrever "REVISÃO 1" em todo orçamento tira o sentido da palavra — e ela
//	significa algo: é a segunda nota do mesmo ticket.
func TestRevisaoSoApareceDaSegundaParteEmDiante(t *testing.T) {
	itens := []itemDoOrcamento{umItem("A", 1, 10, 10)}
	primeira, _ := desenharOrcamento(cabecaDeTeste(), itens, emitenteDeTeste(), dadosDoTomador{Nome: "L"})
	if bytes.Contains(primeira, []byte("REVIS")) {
		t.Error("a parte 1 saiu marcada como revisão")
	}
	c := cabecaDeTeste()
	c["parte"] = 2
	segunda, _ := desenharOrcamento(c, itens, emitenteDeTeste(), dadosDoTomador{Nome: "L"})
	if !bytes.Contains(segunda, []byte("REVIS")) {
		t.Error("a parte 2 não diz que é revisão")
	}
}

// NOTA COMPRIDA NÃO TRANSBORDA A FOLHA
func TestNotaCompridaVirouPagina(t *testing.T) {
	itens := make([]itemDoOrcamento, 0, 40)
	for i := 0; i < 40; i++ {
		itens = append(itens, umItem("MATERIAL", 1, 10, 10))
	}
	pdf, err := desenharOrcamento(cabecaDeTeste(), itens, emitenteDeTeste(), dadosDoTomador{Nome: "L"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if n := bytes.Count(pdf, []byte("/Type /Page /Parent")); n < 2 {
		t.Fatalf("40 itens couberam em %d página — alguma linha saiu da folha", n)
	}
	// O cabeçalho da tabela se repete: uma continuação sem ele obriga a voltar a
	// folha para saber que coluna é qual.
	if n := strings.Count(string(pdf), "(DESCRI"); n < 2 {
		t.Error("a segunda página não repetiu o cabeçalho da tabela")
	}
}

// A DESCRIÇÃO LONGA QUEBRA — NÃO É CORTADA
//
//	Estes nomes carregam o código do produto no FIM: "TP 02 PVC 2P JUNTOS ENC 1
//	/2-3/4 BR TPG-02 (E018010132)". Cortar com reticências joga fora justamente a
//	parte que identifica a peça, e quem confere o orçamento contra a nota fica sem
//	o que comparar.
func TestDescricaoLongaQuebraEmVezDeSerCortada(t *testing.T) {
	longa := "TP 02 PVC 2P JUNTOS ENC 1 /2-3/4 BR TPG-02 (E018010132)"
	pdf, err := desenharOrcamento(cabecaDeTeste(),
		[]itemDoOrcamento{umItem(longa, 4, 5.10, 20.40)},
		emitenteDeTeste(), dadosDoTomador{Nome: "Loja"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	// O código do produto tem que estar no documento, em algum pedaço.
	if !bytes.Contains(pdf, []byte("E018010132")) {
		t.Error("o código do produto sumiu — a descrição foi cortada")
	}
}

// A ENTREGA APARECE COMO UMA UNIDADE
//
//	A regra mora em `regras` e é aplicada no `itensDo`; aqui a pergunta é só se o
//	documento mostra o resultado dela. Quem chega no PDF já vem invertido.
func TestOrcamentoMostraAEntregaComoUmaUnidade(t *testing.T) {
	pdf, err := desenharOrcamento(cabecaDeTeste(),
		[]itemDoOrcamento{umItem("SERVICO DE ENTREGA", 1, 18.00, 18.00)},
		emitenteDeTeste(), dadosDoTomador{Nome: "Loja"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !bytes.Contains(pdf, []byte("1,00")) || !bytes.Contains(pdf, []byte("18,00")) {
		t.Error("a entrega não saiu como 1 × R$ 18,00")
	}
}
