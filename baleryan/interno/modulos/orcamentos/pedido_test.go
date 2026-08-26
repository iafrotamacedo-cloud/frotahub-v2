// rev 1 — o pedido de faturamento ao fornecedor
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	Uma DAV que entra em dois pedidos faz a Rodrigues emitir DUAS notas
//	cobrando o mesmo material. E uma DAV que nunca entra em pedido nenhum é
//	material que a gente comprou e que ninguém vai cobrar — até aparecer, meses
//	depois, numa conferência de conta.
package orcamentos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

func moduloQueGuardaAConsulta(t *testing.T, guardar *string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*guardar = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// A FILA É `pedido_id is null`, E SÓ DAV
func TestAFilaSoPegaDAVAindaNaoPedida(t *testing.T) {
	var pergunta string
	m := moduloQueGuardaAConsulta(t, &pergunta)

	if _, err := m.davsEmAberto(context.Background(),
		"11111111-1111-1111-1111-111111111111", ""); err != nil {
		t.Fatalf("erro: %v", err)
	}
	for _, exigido := range []string{"tipo=eq.dav", "pedido_id=is.null", "oculto_em=is.null"} {
		if !strings.Contains(pergunta, exigido) {
			t.Errorf("a consulta não tem %q — sem isso a mesma DAV entraria em dois pedidos:\n%s",
				exigido, pergunta)
		}
	}
}

// DAV SEM DATA DE EMISSÃO NÃO PODE SUMIR DO CORTE
//
//	Ela existe, foi comprada e é devida. Se o filtro a excluísse por não ter
//	data, ela ficaria invisível para sempre — nunca entraria em pedido nenhum,
//	porque todo pedido tem corte. Três das onze DAVs no banco de 26/08/2026
//	estão exatamente assim.
func TestDAVSemDataEntraNoCorte(t *testing.T) {
	var pergunta string
	m := moduloQueGuardaAConsulta(t, &pergunta)

	if _, err := m.davsEmAberto(context.Background(),
		"11111111-1111-1111-1111-111111111111", "2026-08-26"); err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !strings.Contains(pergunta, "emissao.is.null") {
		t.Errorf("o corte por data exclui a DAV sem emissão — ela ficaria invisível para sempre:\n%s",
			pergunta)
	}
	if !strings.Contains(pergunta, "emissao.lte.2026-08-26") {
		t.Errorf("o corte não foi aplicado:\n%s", pergunta)
	}
}

// SEM CORTE, TUDO ENTRA — e nenhum filtro de data aparece.
func TestSemCorteNaoHaFiltroDeData(t *testing.T) {
	var pergunta string
	m := moduloQueGuardaAConsulta(t, &pergunta)
	if _, err := m.davsEmAberto(context.Background(),
		"11111111-1111-1111-1111-111111111111", ""); err != nil {
		t.Fatalf("erro: %v", err)
	}
	if strings.Contains(pergunta, "emissao.lte") {
		t.Errorf("filtrou por data sem ninguém pedir corte:\n%s", pergunta)
	}
}

// O aviso da relação conta o que ela não diz sozinha.
func TestOAvisoContaOQueFaltaNaRelacao(t *testing.T) {
	num := "19387"
	data := "2026-08-25"
	davs := []davEmAberto{
		{DAV: &num, Emissao: &data},
		{},          // sem número e sem data
		{DAV: &num}, // sem data
	}
	aviso := avisoDoPedido(davs)
	if !strings.Contains(aviso, "1 sem número") {
		t.Errorf("não contou as sem número: %q", aviso)
	}
	if !strings.Contains(aviso, "2 sem data") {
		t.Errorf("não contou as sem data: %q", aviso)
	}
}

func TestRelacaoCompletaNaoGeraAviso(t *testing.T) {
	num := "19387"
	data := "2026-08-25"
	if aviso := avisoDoPedido([]davEmAberto{{DAV: &num, Emissao: &data}}); aviso != "" {
		t.Errorf("inventou aviso numa relação completa: %q", aviso)
	}
}

// O número mostrado cai para o campo alternativo antes de virar traço: uma DAV
// tem o número em `dav_numero`, e o leitor às vezes o põe também em `numero`.
func TestONumeroDaDAVUsaOQueTiver(t *testing.T) {
	dav, numero := "19387", "19387"
	if v := numeroDaDAV(davEmAberto{DAV: &dav}); v != "19387" {
		t.Errorf("com dav_numero deu %q", v)
	}
	if v := numeroDaDAV(davEmAberto{Numero: &numero}); v != "19387" {
		t.Errorf("só com numero deu %q — a DAV ficaria sem identificação na relação", v)
	}
	if v := numeroDaDAV(davEmAberto{}); v != "—" {
		t.Errorf("sem nenhum dos dois deu %q", v)
	}
}

func TestDataEscritaComoGenteLe(t *testing.T) {
	if v := emDiaMesAno("2026-08-25"); v != "25/08/2026" {
		t.Errorf("emDiaMesAno = %q", v)
	}
}
