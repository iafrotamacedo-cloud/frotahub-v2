// rev 1 — o DAV do SysPDV, provado contra o formato real
//
// O TEXTO DOS TESTES É FIEL AO FORMATO, MAS NÃO É DE NINGUÉM
//
//	Os documentos que originaram estes casos são notas de fornecedor de um
//	cliente, e este repositório é público. O layout, os rótulos, o jeito que a
//	descrição quebra em duas linhas e a marca de item cancelado são reproduzidos
//	exatamente como o OCR os devolve — os nomes, códigos e valores foram
//	trocados. O que o teste exercita é a forma, e a forma está intacta.
//
//	Os CASOS, esses são reais: cada um saiu de um erro medido em documento de
//	verdade em 26/08/2026.
package leitor

import "testing"

// O caso simples: cinco itens, nenhum cancelado, a conta fecha.
const davInteiro = `Relatorio SysPDV
DOCUMENTO AUXILIAR DE VENDA - PEDIDO
NÃO É DOCUMENTO FISCAL - NÃO E VÁLIDO COMO RECIBO E COMO GARANTIA DE
Identificação do Estabelecimento Emitente
Razão Social: FORNECEDOR EXEMPLO LTDA-ME (EXEMPLO) CNPJ: 11222333000181
Nº do Documento: 0000012345
Produto/Endereço Embalagem Quantidade Preço Unitário Desc. % Desc. R$ Acresc. % Acresc. R$ Valor Total
00000000000111 - TINTA EXEMPLO BRANCO UN 2,000 100,00 0,00% 0,00 0,00 % 0,00 200,00
15L
00000000000222 - ROLO EXEMPLO 23CM UN 3,000 10,00 0,00% 0,00 0,00 % 0,00 30,00
00000000000333 - FITA EXEMPLO 48MM UN 1,000 20,50 0,00% 0,00 0,00 % 0,00 20,50
Total Bruto Produtos: 250,50 Valor Produtos: 250,50
Total a pagar: 250,50
Plano de Pagamento
Dados Complementares
Endereço: RUA EXEMPLO, 100 - Bairro: CENTRO
Vendedor: FULANO Dt. Prev: 14/08/2026 Hr. Prev: 0807
Fale Conosco: 8500000000 Dt. Emis: 14/08/2026
Observação: TICKET-130559-COCO-AMAURI`

// O CASO QUE QUEBRARIA A TRAVA ARITMÉTICA
//
//	O SysPDV IMPRIME o item cancelado e NÃO o soma no total. Medido no documento
//	real: bruto 268,97, cancelado 75,67, total a pagar 193,30. Somar tudo daria
//	268,97 contra 193,30 — e a trava recusaria uma leitura perfeita, mandando
//	para a IA uma nota que o OCR já tinha lido certo.
const davComCancelado = `DOCUMENTO AUXILIAR DE VENDA - ORÇAMENTO
Nº do Documento: 0000019072
Produto/Endereço Embalagem Quantidade Preço Unitário Desc. % Desc. R$ Acresc. % Acresc. R$ Valor Total
00000000000210 - PISO EXEMPLO AURORA BR PI [à] 2,300 32,90 0,00% 0,00 0,00 % 0,00 75,67
4 46X46 C2,30 < Item 00000000000210 Cancelado >
00000000001795 - ARGAMASSA EXEMPLO 15KG UM 3,000 31,90 0,00% 0,00 0,00 % 0,00 95,70
CINZA EXEMPLO
00000000004423 - TALHADEIRA EXEMPLO 14 X UN 2,000 20,90 0,00% 0,00 0,00 % 0,00 41,80
40 X 250MH HTX
00000000002923 - FITA ZEBRADA EXEMPLO A UN 1,000 29,90 0,00% 0,00 0,00 % 0,00 29,90
00000000004575 - FITA CREPE EXEMPLO 711 A UM 1,000 13,90 0,00% 0,00 0,00 % 0,00 13,90
DELBRAS
00000000004200 - PANO EXEMPLO BRANCO UN 2,000 6.00 0,00% 0,00 0,00 % 0,00 12,00
Total Bruto Produtos: 268,97 Valor Produtos: 193,30
Total a pagar: 193,30
Observação: %120586 - EXEMPLO`

func TestDAVInteiroFechaAConta(t *testing.T) {
	l := DoDAV(davInteiro)
	if l == nil {
		t.Fatal("não reconheceu o documento como DAV")
	}
	if l.DAV != "12345" {
		t.Errorf("número do documento: %q, esperava 12345", l.DAV)
	}
	if l.ValorTotal != 250.50 {
		t.Errorf("total: %.2f, esperava 250,50", l.ValorTotal)
	}
	if len(l.Itens) != 3 {
		t.Fatalf("itens: %d, esperava 3 — %+v", len(l.Itens), l.Itens)
	}
	if l.Emissao != "2026-08-14" {
		t.Errorf("emissão: %q", l.Emissao)
	}
	if l.EmitenteCNPJ != "11222333000181" {
		t.Errorf("cnpj: %q", l.EmitenteCNPJ)
	}
	if !ContaFecha(l) {
		t.Error("a conta tinha que fechar: 200 + 30 + 20,50 = 250,50")
	}
	// A observação é o CAMPO, não a página inteira. Guardar a página toda faz
	// de qualquer número de cinco dígitos um candidato a ticket.
	if l.Observacao != "TICKET-130559-COCO-AMAURI" {
		t.Errorf("observação: %q", l.Observacao)
	}
	tk, ok := TicketDaObservacao(l.Observacao)
	if !ok || tk != 130559 {
		t.Errorf("ticket: %d (%v), esperava 130559", tk, ok)
	}
}

func TestItemCanceladoNaoEntraNaSoma(t *testing.T) {
	l := DoDAV(davComCancelado)
	if l == nil {
		t.Fatal("não reconheceu o documento como DAV")
	}
	if l.ValorTotal != 193.30 {
		t.Errorf("total: %.2f, esperava 193,30 (o TOTAL A PAGAR, não o bruto)", l.ValorTotal)
	}
	// Seis linhas de produto, uma cancelada: cinco itens.
	if len(l.Itens) != 5 {
		t.Fatalf("itens: %d, esperava 5 (a de 75,67 foi cancelada) — %+v", len(l.Itens), l.Itens)
	}
	for _, it := range l.Itens {
		if it.Total == 75.67 {
			t.Error("o item cancelado entrou na lista")
		}
	}
	if !ContaFecha(l) {
		var soma float64
		for _, it := range l.Itens {
			soma += it.Total
		}
		t.Errorf("a conta tinha que fechar: soma %.2f contra total %.2f", soma, l.ValorTotal)
	}
}

// Se a conta NÃO fecha, é aí que a IA precisa entrar — e só aí.
func TestContaQueNaoFechaPedeIA(t *testing.T) {
	l := &Leitura{ValorTotal: 100, Itens: []Item{{Total: 40}, {Total: 30}}}
	if ContaFecha(l) {
		t.Error("70 contra 100 não pode fechar")
	}
	l.Itens = append(l.Itens, Item{Total: 30})
	if !ContaFecha(l) {
		t.Error("100 contra 100 tinha que fechar")
	}
	// Um centavo de diferença em cem reais é arredondamento, não erro.
	l.Itens[2].Total = 30.01
	if !ContaFecha(l) {
		t.Error("um centavo de folga tinha que passar")
	}
	// Item sem valor invalida a conta inteira: não dá para provar o que não soma.
	l.Itens = append(l.Itens, Item{Total: 0})
	if ContaFecha(l) {
		t.Error("item com total zero não pode fechar conta")
	}
}

func TestDANFENaoEConfundidaComDAV(t *testing.T) {
	if DoDAV("DANFE DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRÔNICA") != nil {
		t.Error("uma DANFE não pode ser lida como DAV")
	}
	if DoDAV("texto qualquer sem cabeçalho") != nil {
		t.Error("texto solto não é DAV")
	}
}

// Observação com dois tickets não é caso de OCR: é caso de gente.
func TestObservacaoComDoisTicketsNaoDaTicket(t *testing.T) {
	if _, ok := TicketDaObservacao("TICKET 130559 e TICKET 126574"); ok {
		t.Error("com dois números não dá para escolher um")
	}
	if _, ok := TicketDaObservacao("sem numero nenhum"); ok {
		t.Error("sem número não dá ticket")
	}
}

// O TICKET SÓ VALE SE VEIO DO CAMPO
//
//	Sem esta regra, qualquer número de cinco dígitos perdido na página vira
//	ticket — e ticket errado não falha alto: gera, lança e cobra a loja errada.
func TestTicketSoValeQuandoVemDoCampo(t *testing.T) {
	daPagina := &Leitura{Observacao: "algum texto 130559 no meio da página", ObservacaoDoCampo: false}
	if _, ok := TicketConfiavel(daPagina); ok {
		t.Error("número achado solto na página não pode virar ticket")
	}

	doCampo := &Leitura{Observacao: "TICKET-130559-COCO-AMAURI", ObservacaoDoCampo: true}
	tk, ok := TicketConfiavel(doCampo)
	if !ok || tk != 130559 {
		t.Errorf("do campo: %d (%v), esperava 130559", tk, ok)
	}

	dois := &Leitura{Observacao: "TICKET 130559 e 126574", ObservacaoDoCampo: true}
	if _, ok := TicketConfiavel(dois); ok {
		t.Error("dois números no campo não dão um ticket")
	}
	if _, ok := TicketConfiavel(nil); ok {
		t.Error("leitura nula não dá ticket")
	}
}

// Casos de observação vindos dos documentos reais medidos em 26/08/2026 — o
// OCR escreve o rótulo de jeitos diferentes, e o número tem que sair de todos.
func TestOsJeitosReaisDeEscreverOTicket(t *testing.T) {
	casos := map[string]int{
		"TICKET-130559-COCO-AMAURI":    130559,
		"%120586 - EDILSON":            120586,
		"130366 - MIGUEL DIAS":         130366,
		"TICKER:124766 PADARIA AMAURI": 124766,
		"TICKET/ 131382/ AMAURI":       131382,
		"OBS:126114- OLIVEIRA -AMAURI": 126114,
	}
	for obs, quer := range casos {
		tk, ok := TicketConfiavel(&Leitura{Observacao: obs, ObservacaoDoCampo: true})
		if !ok || tk != quer {
			t.Errorf("%q => %d (%v), esperava %d", obs, tk, ok, quer)
		}
	}
}

// A CONFIANÇA DO DAV NÃO PODE EMPACAR EM "CONFIRA"
//
//	A tela diz "leitura boa" a partir de 0,85 e "confira" a partir de 0,6. Como
//	0,35 da conta vem do dígito verificador da chave de acesso — que o DAV não
//	tem —, um DAV perfeito parava em 0,65 e pedia conferência para sempre.
func TestDAVQueFechaAContaChegaEmLeituraBoa(t *testing.T) {
	l := DoDAV(davInteiro)
	if l == nil {
		t.Fatal("não leu")
	}
	if !ContaFecha(l) {
		t.Fatal("este documento fecha a conta")
	}
	if l.Confianca < 0.85 {
		t.Errorf("confiança %.2f — um DAV que fecha a própria conta tem que passar de 0,85", l.Confianca)
	}
	// E a DANFE não pode ganhar os pontos do DAV por tabela.
	danfe := &Leitura{Tipo: "nf", DAV: "123", EmitenteCNPJ: "11222333000181",
		ValorTotal: 10, Itens: []Item{{Total: 10, Descricao: "x", Quantidade: 1, Unitario: 10}}}
	antes := Conferir(danfe)
	if antes >= 0.85 {
		t.Errorf("uma nota sem chave de acesso não pode chegar a %.2f só por ter número", antes)
	}
}
