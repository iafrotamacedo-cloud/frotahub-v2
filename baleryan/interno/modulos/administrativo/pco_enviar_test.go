// rev 1 — o que dá para testar sem banco nem Brevo (P-30)
//
// O QUE FICA DE FORA
//
//	`enviarLote`/`enviarPorEmail`/`montarZip` precisam do banco (o CAS) e do
//	armazém (baixar o PDF) — não são testados aqui. O que ESTA prova cobre é
//	a parte pura: os campos nulos vindos do PostgREST virando texto sensato,
//	a tabela do e-mail fechando com o total certo, e o nome de pasta nunca
//	quebrando um .zip no Windows.
package administrativo

import (
	"strings"
	"testing"
)

func strPtr(s string) *string   { return &s }
func f64Ptr(f float64) *float64 { return &f }

func TestOrdemParaEnvio_CamposNulos(t *testing.T) {
	// Uma OC recém-lida pode não ter fornecedor casado (a leitura já teria
	// rejeitado, mas a função não pode presumir isso — ela lê o que veio).
	o := ordemParaEnvio{NomeArquivo: "OrdemDeCompra_019731.pdf"}
	if o.centro() != "SEM CENTRO DE CUSTO" {
		t.Errorf("centro() = %q, esperava o balde padrão", o.centro())
	}
	if o.numero() != "OrdemDeCompra_019731" {
		t.Errorf("numero() = %q, esperava o nome do arquivo sem .pdf", o.numero())
	}
	if o.valor() != 0 {
		t.Errorf("valor() = %v, esperava zero para Total nulo", o.valor())
	}
	if o.fornecedorNome() != "Não informado" || o.fornecedorCNPJ() != "Não informado" {
		t.Errorf("fornecedor sem embed deveria sair \"Não informado\", saiu %q/%q",
			o.fornecedorNome(), o.fornecedorCNPJ())
	}
	if o.chaveArquivo() != "" {
		t.Errorf("chaveArquivo() sem embed deveria ser vazio, saiu %q", o.chaveArquivo())
	}
}

func TestOrdemParaEnvio_CamposPreenchidos(t *testing.T) {
	o := ordemParaEnvio{
		NomeArquivo:     "arquivo.pdf",
		Numero:          strPtr("019731"),
		ObraCentroCusto: strPtr("  MSL VILLAS - AQUIRAZ  "),
		Total:           f64Ptr(415.90),
		Fornecedores:    &fornecedorEmbutido{RazaoSocial: "S V COMERCIO", CNPJ: "35088657000137"},
		Arquivos:        &arquivoEmbutido{ChaveR2: "clientes/x/y.pdf"},
	}
	if o.centro() != "MSL VILLAS - AQUIRAZ" {
		t.Errorf("centro() = %q, esperava aparado", o.centro())
	}
	if o.numero() != "019731" {
		t.Errorf("numero() = %q", o.numero())
	}
	if o.valor().Reais() != "415,90" {
		t.Errorf("valor().Reais() = %q, esperava 415,90", o.valor().Reais())
	}
	if o.fornecedorCNPJ() != "35.088.657/0001-37" {
		t.Errorf("fornecedorCNPJ() = %q", o.fornecedorCNPJ())
	}
	if o.chaveArquivo() != "clientes/x/y.pdf" {
		t.Errorf("chaveArquivo() = %q", o.chaveArquivo())
	}
}

func TestFormatarCNPJ(t *testing.T) {
	if got := formatarCNPJ("35088657000137"); got != "35.088.657/0001-37" {
		t.Errorf("formatarCNPJ(14 dígitos) = %q", got)
	}
	// CNPJ já pontuado também funciona — soDigitos tira a pontuação antes.
	if got := formatarCNPJ("35.088.657/0001-37"); got != "35.088.657/0001-37" {
		t.Errorf("formatarCNPJ(já pontuado) = %q", got)
	}
	// Menos de 14 dígitos: devolve como veio, em vez de um XX.XXX truncado
	// que pareceria um CNPJ válido sem ser.
	if got := formatarCNPJ("123"); got != "123" {
		t.Errorf("formatarCNPJ(curto) = %q, esperava passthrough", got)
	}
}

func TestPastaSegura(t *testing.T) {
	if got := pastaSegura("MSL VILLAS - AQUIRAZ"); got != "MSL VILLAS - AQUIRAZ" {
		t.Errorf("pastaSegura não deveria mexer num nome já limpo, saiu %q", got)
	}
	if got := pastaSegura("  espaço nas pontas  "); got != "espaço nas pontas" {
		t.Errorf("pastaSegura(%q) = %q, esperava aparado", "  espaço nas pontas  ", got)
	}
	// NENHUM DOS CARACTERES QUE O WINDOWS RECUSA EM NOME DE PASTA PODE SOBRAR
	//   Não conta dash a dash (frágil) — confere que a saída não tem mais
	//   nenhum dos oito caracteres proibidos, seja qual for a combinação.
	perigosos := `\/:*?"<>|`
	entrada := `LOJA 20 / SETOR: A*B?"<>|`
	saida := pastaSegura(entrada)
	if strings.ContainsAny(saida, perigosos) {
		t.Errorf("pastaSegura(%q) = %q ainda tem caractere proibido de pasta do Windows", entrada, saida)
	}
	if !strings.Contains(saida, "LOJA 20") || !strings.Contains(saida, "SETOR") {
		t.Errorf("pastaSegura(%q) = %q perdeu texto que devia sobreviver", entrada, saida)
	}
}

func TestPareceEmail(t *testing.T) {
	validos := []string{"ia.frotamacedo@gmail.com", "a@b.co", "nome.sobrenome@mslz.com.br"}
	for _, e := range validos {
		if !pareceEmail(e) {
			t.Errorf("pareceEmail(%q) = false, esperava true", e)
		}
	}
	invalidos := []string{"", "sem-arroba", "@semnome.com", "nome@", "com espaço@x.com", "nome@semponto"}
	for _, e := range invalidos {
		if pareceEmail(e) {
			t.Errorf("pareceEmail(%q) = true, esperava false", e)
		}
	}
}

func TestMontarHTMLDoEnvio(t *testing.T) {
	ordens := []ordemParaEnvio{
		{
			NomeArquivo: "a.pdf", Numero: strPtr("019731"),
			ObraCentroCusto: strPtr("MSL VILLAS - AQUIRAZ"), Total: f64Ptr(415.90),
			Fornecedores: &fornecedorEmbutido{RazaoSocial: "S V <Comércio> & Cia", CNPJ: "35088657000137"},
		},
		{
			NomeArquivo: "b.pdf", Numero: strPtr("019702"),
			ObraCentroCusto: strPtr("LOJA 20"), Total: f64Ptr(84.10),
			Fornecedores: &fornecedorEmbutido{RazaoSocial: "Outro Fornecedor", CNPJ: "11111111000191"},
		},
	}
	html := montarHTMLDoEnvio(ordens, "10-09-2026")

	if strings.Count(html, "<tr>") != 2 {
		t.Errorf("esperava 2 linhas de item, achei %d", strings.Count(html, "<tr>"))
	}
	if !strings.Contains(html, "500,00") {
		t.Errorf("total (415,90 + 84,10 = 500,00) não aparece no HTML")
	}
	if !strings.Contains(html, "Total (2 ordens)") {
		t.Errorf("o rodapé não diz quantas ordens — HTML: %s", html)
	}
	// O NOME DO FORNECEDOR TEM "<" E "&" — SE NÃO ESCAPAR, QUEBRA A TABELA
	//   Um fornecedor chamado "S V <Comércio> & Cia" sem escape faria o
	//   navegador (ou o cliente de e-mail) interpretar "<Comércio>" como uma
	//   tag HTML e sumir da tela — exatamente o tipo de defeito que só
	//   aparece com um nome de fornecedor "estranho" de verdade.
	if !strings.Contains(html, "S V &lt;Comércio&gt; &amp; Cia") {
		t.Error("o nome do fornecedor com < e & não foi escapado — quebraria o HTML do e-mail")
	}
	if strings.Contains(html, "<Comércio>") {
		t.Error("o HTML cru do fornecedor vazou sem escape")
	}
	if !strings.Contains(html, "Pedidos_PCO_10-09-2026.zip") {
		t.Error("o nome do anexo não bate com a data")
	}
}
