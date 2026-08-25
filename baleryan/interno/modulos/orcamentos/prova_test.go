package orcamentos

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

// O caso que o dono descreveu: 90% das vezes o fornecedor digitou errado. Estes
// são os dois erros reais — a letra O no lugar do zero, e dígitos invertidos.
func TestDesconfundir(t *testing.T) {
	casos := map[string]string{
		"13O328":   "130328",
		"1303z8":   "130328",
		"130 328":  "130328",
		"#130.328": "130328",
		"l30328":   "130328",
		"130328":   "130328",
	}
	for entrada, esperado := range casos {
		if got := desconfundir(entrada); got != esperado {
			t.Fatalf("desconfundir(%q) = %q, queria %q", entrada, got, esperado)
		}
	}
}

func TestVariacoesAcertaOsErrosReais(t *testing.T) {
	v := variacoes("130341")
	tem := map[string]bool{}
	for _, x := range v {
		tem[x] = true
	}
	if !tem["103341"] {
		// dígitos vizinhos invertidos: 130341 -> 103341
		t.Fatal("a inversão de dois dígitos não foi gerada")
	}
	if !tem["130841"] {
		t.Fatal("a troca de um dígito não foi gerada")
	}
	if !tem["130341"] {
		t.Fatal("o próprio número tem que estar na lista — o caso mais comum é ele estar certo")
	}
	// Sugestão demais é o mesmo que nenhuma.
	if len(v) > 120 {
		t.Fatalf("%d variações é lista demais para o usuário conferir", len(v))
	}
}

func TestVariacoesNaoInventaTamanhoErrado(t *testing.T) {
	for _, x := range variacoes("130328") {
		if len(x) < 5 || len(x) > 6 {
			t.Fatalf("variação %q tem tamanho de coisa que não é ticket", x)
		}
	}
}

func TestComoDifere(t *testing.T) {
	if s := comoDifere("130328", "130828"); s != "um dígito trocado na posição 4" {
		t.Fatalf("comoDifere = %q", s)
	}
	if s := comoDifere("130341", "103341"); s != "dois dígitos invertidos" {
		t.Fatalf("comoDifere = %q", s)
	}
	if s := comoDifere("130328", "13032"); s != "um dígito a mais ou a menos" {
		t.Fatalf("comoDifere = %q", s)
	}
}

// O rateio: a soma das partes tem que dar exatamente o total. Um centavo de
// sobra multiplicado por 509 orçamentos é o tipo de divergência que ninguém
// consegue explicar seis meses depois.
func TestFatiarFechaNoCentavo(t *testing.T) {
	for _, n := range []int{2, 3, 7, 13} {
		total := regras.DinheiroDe(1000.01)
		var soma regras.Dinheiro
		for i := 0; i < n; i++ {
			soma += fatiar(total, n, i)
		}
		if soma != total {
			t.Fatalf("dividido em %d, a soma deu %s e o total é %s", n, soma.Reais(), total.Reais())
		}
	}
}

func TestUmUUIDRecusaLixo(t *testing.T) {
	bom := "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	if _, ok := umUUID(bom); !ok {
		t.Fatal("uuid válido recusado")
	}
	for _, ruim := range []string{
		"", "3f2504e0", "3f2504e0-4f89-11d3-9a0c-0305e82c330",
		"3f2504e0*4f89-11d3-9a0c-0305e82c3301",
		"' or 1=1 --                          ",
	} {
		if _, ok := umUUID(ruim); ok {
			t.Fatalf("%q passou por uuid", ruim)
		}
	}
}

// "até 25/08" precisa incluir o dia inteiro em Fortaleza. Sem o fuso, os
// lançamentos do fim da tarde sumiriam da planilha sem explicação.
func TestUmaDataFechaODiaNoFusoDaCasa(t *testing.T) {
	inicio := umaData("2026-08-25", false)
	fim := umaData("2026-08-25", true)
	if inicio != "2026-08-25T03:00:00Z" {
		t.Fatalf("início = %q", inicio)
	}
	if fim != "2026-08-26T02:59:59Z" {
		t.Fatalf("fim = %q — o dia não fechou às 23:59:59 de Fortaleza", fim)
	}
	if umaData("", false) != "" || umaData("ontem", false) != "" {
		t.Fatal("data inválida tinha que virar filtro vazio, não filtro estranho")
	}
}

func TestMarcaEhTracoOuCheck(t *testing.T) {
	if marca(false) != "–" {
		t.Fatal("não é traço")
	}
	if marca(true) == "✓" {
		t.Fatal("o símbolo de visto não existe nas fontes padrão do PDF: sairia quadrado vazio")
	}
}

func TestComPonto(t *testing.T) {
	casos := map[int]string{0: "0", 509: "509", 1377: "1.377", 1000000: "1.000.000"}
	for n, esperado := range casos {
		if got := comPonto(n); got != esperado {
			t.Fatalf("comPonto(%d) = %q, queria %q", n, got, esperado)
		}
	}
}

func TestUmDosPermitidos(t *testing.T) {
	if umDosPermitidos("500") != 500 {
		t.Fatal("500 é opção da tela")
	}
	if umDosPermitidos("999") != porPaginaPadrao {
		t.Fatal("valor fora da lista tinha que cair no padrão, não passar direto para o banco")
	}
}

func TestDescreverFiltroDizOQueFoiFiltrado(t *testing.T) {
	r := httptest.NewRequest("GET", "/orcamentos/planilhas.xlsx?conta=civil&de=2026-08-01&ate=2026-08-25", nil)
	s := descreverFiltro(r, 1377)
	for _, pedaco := range []string{"1.377", "Civil", "01/08/2026", "25/08/2026"} {
		if !contains(s, pedaco) {
			t.Fatalf("o subtítulo %q não diz %q — quem receber o arquivo não sabe o que está vendo", s, pedaco)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// as rotas
// ---------------------------------------------------------------------------

// ESTE TESTE EXISTE POR CAUSA DE UM MOTOR QUE NÃO SUBIU
//
//	O roteador do Go recusa dois padrões em que nenhum é mais específico que o
//	outro. `GET /orcamentos/{id}/pdf` e `GET /orcamentos/documentos/{id}` são
//	esse caso: "/orcamentos/documentos/pdf" casa com os dois.
//
//	O compilador não vê. `go vet` não vê. Os testes de unidade não viam, porque
//	nenhum deles montava as rotas. O erro só aparecia quando o processo subia —
//	e apareceu, em produção, dois segundos depois do deploy.
//
//	Montar num mux de verdade custa microssegundos e transforma uma queda de
//	produção numa linha vermelha aqui.
func TestMontarNaoEntraEmPanico(t *testing.T) {
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("as rotas se atropelam e o motor não sobe: %v", p)
		}
	}()
	// Os campos podem ser nulos: `Montar` só registra os métodos, não os chama.
	(&Modulo{}).Montar(http.NewServeMux())
}

// E este confere que cada endereço cai no lugar certo — porque "não entrou em
// pânico" não é o mesmo que "resolve como eu quis".
func TestCadaEnderecoCaiNoSeuLugar(t *testing.T) {
	mux := http.NewServeMux()
	(&Modulo{}).Montar(mux)

	const id = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	casos := []struct{ metodo, caminho, padrao string }{
		{"GET", "/orcamentos/painel", "GET /orcamentos/painel"},
		{"GET", "/orcamentos/documentos", "GET /orcamentos/documentos"},
		{"GET", "/orcamentos/documentos/" + id, "GET /orcamentos/documentos/{id}"},
		{"GET", "/orcamentos/documentos/" + id + "/arquivo", "GET /orcamentos/documentos/{id}/arquivo"},
		{"DELETE", "/orcamentos/documentos/" + id, "DELETE /orcamentos/documentos/{id}"},
		{"POST", "/orcamentos/documentos/" + id + "/tickets", "POST /orcamentos/documentos/{id}/tickets"},
		{"PATCH", "/orcamentos/documentos/" + id + "/tickets", "PATCH /orcamentos/documentos/{id}/tickets"},
		{"DELETE", "/orcamentos/documentos/" + id + "/tickets/130328", "DELETE /orcamentos/documentos/{id}/tickets/{ticket}"},
		{"GET", "/orcamentos", "GET /orcamentos"},
		{"POST", "/orcamentos/gerar", "POST /orcamentos/gerar"},
		{"GET", "/orcamentos/ficha/" + id + "/pdf", "GET /orcamentos/ficha/{id}/pdf"},
		{"POST", "/orcamentos/ficha/" + id + "/lancar", "POST /orcamentos/ficha/{id}/lancar"},
		{"POST", "/orcamentos/ficha/" + id + "/aprovar", "POST /orcamentos/ficha/{id}/aprovar"},
		{"POST", "/orcamentos/ficha/" + id + "/restaurar", "POST /orcamentos/ficha/{id}/restaurar"},
		{"DELETE", "/orcamentos/ficha/" + id, "DELETE /orcamentos/ficha/{id}"},
		{"GET", "/orcamentos/correcoes", "GET /orcamentos/correcoes"},
		{"GET", "/orcamentos/correcoes/candidatos", "GET /orcamentos/correcoes/candidatos"},
		{"GET", "/orcamentos/planilhas", "GET /orcamentos/planilhas"},
		{"GET", "/orcamentos/planilhas.xlsx", "GET /orcamentos/planilhas.xlsx"},
		{"GET", "/orcamentos/planilhas.pdf", "GET /orcamentos/planilhas.pdf"},
	}
	for _, c := range casos {
		_, padrao := mux.Handler(httptest.NewRequest(c.metodo, c.caminho, nil))
		if padrao != c.padrao {
			t.Errorf("%s %s caiu em %q, esperava %q", c.metodo, c.caminho, padrao, c.padrao)
		}
	}
}
