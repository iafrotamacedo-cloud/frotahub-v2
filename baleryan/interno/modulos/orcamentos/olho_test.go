package orcamentos

import (
	"encoding/base64"
	"os"
	"testing"
)

// TestOlho não afirma nada: escreve o PDF em disco para conferência humana.
// Só roda quando alguém pede, com ORC_OLHO=1.
func TestOlho(t *testing.T) {
	if os.Getenv("ORC_OLHO") == "" {
		t.Skip("conferência visual: rode com ORC_OLHO=1")
	}
	jpg, err := os.ReadFile("/tmp/marca.jpg")
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(jpg)
	un := "UN"
	cnpj := "03.720.882/0011-20"
	end := "Av. Eng. Santana Jr., 2977 - Cocó"
	cid, uf := "Fortaleza", "CE"

	emi := &dadosDoEmitente{
		Razao:     "FROTA MACEDO ENGENHARIA LTDA",
		CNPJ:      "27.363.223/0001-70",
		Endereco:  "Eng. Heitor de Oliveira Albuquerque, 295 — Cidade dos Funcionários — Fortaleza/CE",
		Contato:   "(85) 2181-1386 • frotamacedoengenharia@gmail.com",
		Pagamento: "Transferência Bancária 30 dias",
		Validade:  7,
		Observacao: []string{
			"Orçamento válido por 7 (sete) dias corridos a partir da data de emissão.",
			"Os valores acima incluem material e serviço de entrega.",
		},
		MarcaB64: &b64,
	}
	tom := dadosDoTomador{Nome: "Mercadinhos São Luiz — Cocó", CNPJ: &cnpj, Endereco: &end, Cidade: &cid, UF: &uf}
	cabeca := map[string]any{"ticket": 131088, "parte": 1, "loja": "Cocó", "criado_em": "2026-08-19T12:00:00Z"}
	itens := []itemDoOrcamento{
		{1, "LAMPADA TUBO LED T8 120CM 4000K 18W", &un, 2, 27.48, 54.96},
	}

	pdf, err := desenharOrcamento(cabeca, itens, emi, tom)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("/tmp/orc.pdf", pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("escrito /tmp/orc.pdf (%d bytes)", len(pdf))

	// E um segundo, com muitos itens e corte pelo teto, para ver o documento
	// cheio — é onde as alturas se atropelam, se forem se atropelar.
	itens2 := make([]itemDoOrcamento, 0, 32)
	for i := 1; i <= 32; i++ {
		itens2 = append(itens2, itemDoOrcamento{i,
			"MATERIAL DE TESTE COM DESCRIÇÃO LONGA PARA VER ATÉ ONDE CORTA " + string(rune('A'+i)),
			&un, float64(i), 12.3, 12.3 * float64(i)})
	}
	cabeca2 := map[string]any{"ticket": 130409, "parte": 2, "loja": "Miguel Dias",
		"criado_em": "2026-08-26T18:21:00Z", "reduzido_pelo_teto": true, "valor_antes_do_teto": 812.4}
	pdf2, err := desenharOrcamento(cabeca2, itens2, emi, tom)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile("/tmp/orc_cheio.pdf", pdf2, 0o644)
	t.Logf("escrito /tmp/orc_cheio.pdf (%d bytes)", len(pdf2))
}
