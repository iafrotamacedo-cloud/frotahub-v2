// rev 1 — faturamento direto
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	Uma nota de faturamento direto o cliente JÁ PAGOU ao fornecedor. Se ela
//	entrar num espelho de cobrança, a gente cobra de novo — e como `fatura_id`
//	fica nulo para sempre, ela entraria em TODO espelho, todo mês. É o tipo de
//	erro que o cliente descobre antes da gente.
package orcamentos

import (
	"os"
	"strings"
	"testing"
)

// A FILA DE FATURAR TEM QUE EXCLUIR AS DIRETAS, NOS DOIS LUGARES
//
//	O filtro aparece duas vezes no arquivo — a lista e a extração. Um teste que
//	olhasse só uma passaria com a outra furada.
func TestAFilaDeFaturarNuncaPegaFaturamentoDireto(t *testing.T) {
	fonte, err := os.ReadFile("faturamento.go")
	if err != nil {
		t.Fatalf("não consegui ler o arquivo: %v", err)
	}
	texto := string(fonte)

	comFatura := strings.Count(texto, "fatura_id=is.null&status=neq.removido")
	comFiltro := strings.Count(texto, "fatura_id=is.null&status=neq.removido&faturamento_direto=is.false")
	if comFatura != comFiltro {
		t.Errorf("a fila de faturar é montada %d vezes e só %d excluem o faturamento direto — "+
			"a que sobrou cobraria de novo uma nota que o cliente já pagou ao fornecedor",
			comFatura, comFiltro)
	}
	if comFiltro == 0 {
		t.Error("nenhuma consulta exclui o faturamento direto")
	}
}

// O LANÇAMENTO SOBE A NOTA ORIGINAL, NÃO UM PDF NOSSO
//
//	Montar um orçamento aqui seria fabricar um documento que afirma uma cobrança
//	que não existe — anexado a um chamado, com cara de custo nosso, e ninguém
//	olhando o Trílogo teria como saber.
func TestOLancamentoDiretoNaoMontaOrcamento(t *testing.T) {
	fonte, err := os.ReadFile("lancar.go")
	if err != nil {
		t.Fatalf("não consegui ler o arquivo: %v", err)
	}
	texto := string(fonte)

	if !strings.Contains(texto, "arquivoParaLancar") {
		t.Fatal("o lançamento não passa por arquivoParaLancar — ele monta o PDF direto, " +
			"e no faturamento direto isso fabricaria um documento falso")
	}
	// `montarPDF` só pode ser chamado de DENTRO da escolha, no ramo do fluxo
	// normal. Se ele voltar a aparecer no corpo de `lancar`, a escolha foi
	// contornada.
	corpoDoLancar := texto[strings.Index(texto, "func (m *Modulo) lancar("):]
	corpoDoLancar = corpoDoLancar[:strings.Index(corpoDoLancar, "\nfunc ")]
	if strings.Contains(corpoDoLancar, "m.montarPDF(") {
		t.Error("o `lancar` voltou a montar o PDF direto, pulando a escolha do documento")
	}
}
