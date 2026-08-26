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
	cabeca := map[string]any{"ticket": 130832, "parte": 1, "loja": "Cocó", "criado_em": "2026-08-19T12:00:00Z"}
	// Os mesmos itens do orçamento 130832 do legado, com a linha de entrega —
	// para comparar lado a lado, inclusive a inversão.
	itens := []itemDoOrcamento{
		{1, "KLIN TOM 2P+T PB 20A RED S/PL BR (14017939)", &un, 4, 11.88, 47.52},
		{2, "TP 02 PVC 2P JUNTOS ENC 1 /2-3/4 BR TPG-02 (E018010132)", &un, 4, 5.10, 20.40},
		{3, "COND PVC 02 6 SAIDAS 1/2 , 3/4 BR LP6G-10 (E017210020)", &un, 4, 9.00, 36.00},
		{4, "ADAPT PVC 02 3/4 BR APIG -15 (E020710015)", &un, 7, 2.28, 15.96},
		{5, "PARAF MAD PHILIPS 4,8X50", &un, 10, 0.36, 3.60},
		{6, "BUCHA PLÁSTICA 8MM", &un, 10, 0.30, 3.00},
		// já invertida, como chega do `itensDo`: uma entrega, R$ 18,00
		{7, "SERVICO DE ENTREGA", &un, 1, 18.00, 18.00},
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
