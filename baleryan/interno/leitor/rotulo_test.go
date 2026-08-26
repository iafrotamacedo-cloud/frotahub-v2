// rev 1 — de qual caixa saiu a observação
//
// O DEFEITO QUE ESTE ARQUIVO GUARDA
//
//	Em 26/08/2026, depois que o arquivo passou a ir inteiro para o modelo,
//	NENHUM documento lido pela IA conseguia mais amarrar ticket: o JSON pedido
//	não tinha o campo de procedência, então ele ficava falso sempre. O amarre
//	automático — que estava em 29 de 32 documentos, com zero erro — tinha ido a
//	zero sem ninguém notar, porque "vai para SEM TICKET" é uma frase que também
//	aparece quando está tudo certo.
package leitor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRotuloDeCampoDeObservacaoVale(t *testing.T) {
	bons := []string{
		"INFORMAÇÕES COMPLEMENTARES",
		"informacoes complementares",
		"Informações Complementares:",
		"DADOS ADICIONAIS",
		"Observação:",
		"OBSERVACOES",
		"Obs.",
		"INFORMAÇÕES ADICIONAIS",
		// O emissor às vezes acrescenta o resto do título da caixa.
		"INFORMAÇÕES COMPLEMENTARES DE INTERESSE DO CONTRIBUINTE",
	}
	for _, r := range bons {
		if !RotuloConfiavel(r) {
			t.Errorf("%q é caixa de observação e foi recusada — a nota vai virar trabalho manual à toa", r)
		}
	}
}

// O CANHOTO NÃO É OBSERVAÇÃO
//
//	O caso real de NF 9160: a IA devolveu o texto do canhoto do topo da DANFE,
//	que o entregador destaca e o cliente assina. Número achado ali não é ticket
//	digitado por ninguém.
func TestRotuloDeOutraCaixaNaoVale(t *testing.T) {
	ruins := []string{
		"RECEBEMOS DE RODRIGUES MATERIAL DE CONSTRUCOES LTDA-ME",
		"NATUREZA DA OPERAÇÃO",
		"DESTINATÁRIO / REMETENTE",
		"CÁLCULO DO IMPOSTO",
		"DADOS DO PRODUTO/SERVIÇO",
		"TRANSPORTADOR / VOLUMES TRANSPORTADOS",
		"", // o modelo não achou caixa nenhuma: junta de página solta
		"   ",
	}
	for _, r := range ruins {
		if RotuloConfiavel(r) {
			t.Errorf("%q foi aceita como campo de observação — daí sai ticket errado em silêncio", r)
		}
	}
}

// A regra do ticket tem que continuar exigindo procedência DEPOIS da mudança.
func TestTicketSoValeComRotuloBom(t *testing.T) {
	comCaixa := &Leitura{
		Observacao:        "MANUTENÇÃO PREVENTIVA — TICKET 130773",
		ObservacaoDoCampo: RotuloConfiavel("INFORMAÇÕES COMPLEMENTARES"),
	}
	if ticket, ok := TicketConfiavel(comCaixa); !ok || ticket != 130773 {
		t.Errorf("ticket do campo certo saiu %d/%v, esperava 130773/true", ticket, ok)
	}

	doCanhoto := &Leitura{
		Observacao:        "MANUTENÇÃO PREVENTIVA — TICKET 130773",
		ObservacaoDoCampo: RotuloConfiavel("RECEBEMOS DE FULANO LTDA"),
	}
	if _, ok := TicketConfiavel(doCanhoto); ok {
		t.Error("o número veio do canhoto e virou ticket confiável")
	}
}

// A MARCA VIAJA COM O TEXTO
//
//	`Melhor` troca a observação pela mais longa. Se a marca de procedência não
//	for junto, a procedência de um texto passa a valer para outro.
func TestMelhorLevaAProcedenciaJuntoComAObservacao(t *testing.T) {
	curta := &Leitura{
		Confianca: 0.9, Observacao: "obs", ObservacaoDoCampo: true,
		ObservacaoRotulo: "OBSERVAÇÃO",
	}
	longaSemCaixa := &Leitura{
		Confianca: 0.5, Observacao: "um texto bem mais longo, tirado da página solta",
		ObservacaoDoCampo: false,
	}

	junta := Melhor(curta, longaSemCaixa)
	if junta.Observacao != longaSemCaixa.Observacao {
		t.Fatalf("a observação mais longa não ganhou: %q", junta.Observacao)
	}
	if junta.ObservacaoDoCampo {
		t.Error("ficou com a observação da página solta e a marca de campo da outra leitura — " +
			"é assim que um número solto vira ticket confiável")
	}

	// E o caminho contrário: a longa é a boa, e a marca tem que ir junto.
	longaComCaixa := &Leitura{
		Confianca: 0.5, Observacao: "um texto bem mais longo, tirado da caixa certa",
		ObservacaoDoCampo: true, ObservacaoRotulo: "DADOS ADICIONAIS",
	}
	curtaSemCaixa := &Leitura{Confianca: 0.9, Observacao: "obs", ObservacaoDoCampo: false}

	junta = Melhor(curtaSemCaixa, longaComCaixa)
	if !junta.ObservacaoDoCampo {
		t.Error("a observação boa perdeu a marca no caminho e virou trabalho manual à toa")
	}
}

// A ponta a ponta: o modelo diz o rótulo, o nosso código decide a marca.
func TestAIADerivaAMarcaDoRotuloQueDevolveu(t *testing.T) {
	casos := []struct {
		rotulo   string
		confiado bool
	}{
		{"INFORMAÇÕES COMPLEMENTARES", true},
		{"DADOS ADICIONAIS", true},
		{"RECEBEMOS DE FULANO LTDA", false},
		{"", false},
	}
	for _, k := range casos {
		ia := iaQueResponde(t, `{"tipo":"nf","numero":"1","valor_total":10,
			"observacao":"TICKET 130773","observacao_rotulo":`+aspas(k.rotulo)+`,"itens":[]}`)
		l, err := ia.LerArquivo(context.Background(), "application/pdf", []byte("%PDF"))
		if err != nil {
			t.Fatalf("rótulo %q: %v", k.rotulo, err)
		}
		if l.ObservacaoDoCampo != k.confiado {
			t.Errorf("rótulo %q deu marca %v, esperava %v", k.rotulo, l.ObservacaoDoCampo, k.confiado)
		}
		if _, ok := TicketConfiavel(l); ok != k.confiado {
			t.Errorf("rótulo %q: ticket confiável = %v, esperava %v", k.rotulo, ok, k.confiado)
		}
	}
}

func aspas(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func iaQueResponde(t *testing.T, jsonDaLeitura string) *IA {
	t.Helper()
	envelope, _ := json.Marshal(map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{map[string]any{"text": jsonDaLeitura}}},
		}},
	})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(envelope)
	}))
	t.Cleanup(s.Close)
	ia := NovaIA("chave-de-mentira")
	ia.Base = s.URL
	ia.Intervalo = 0
	return ia
}

// O RÓTULO DO DAV DO SysPDV
//
//	Em 26/08/2026 esta única linha faltando na lista custou o ticket de TODOS os
//	DAVs lidos pela IA — inclusive um com "TICKET-131372-ANTONIO SALES-AMAURI"
//	escrito na caixa certa, por extenso. Rótulo legítimo faltando não gera erro:
//	gera trabalho manual silencioso, que é a forma mais cara de estar errado.
func TestDadosComplementaresEhCampoDeObservacao(t *testing.T) {
	for _, r := range []string{"Dados Complementares", "DADOS COMPLEMENTARES", "dados complementares:"} {
		if !RotuloConfiavel(r) {
			t.Errorf("%q é a caixa do DAV do SysPDV e foi recusada", r)
		}
	}

	// O caso real, ponta a ponta.
	l := &Leitura{
		Observacao:        "TICKET-131372-ANTONIO SALES-AMAURI",
		ObservacaoDoCampo: RotuloConfiavel("Dados Complementares"),
	}
	if ticket, ok := TicketConfiavel(l); !ok || ticket != 131372 {
		t.Errorf("o ticket do DAV saiu %d/%v, esperava 131372/true", ticket, ok)
	}
}

// NO DAV, O NÚMERO DO DAV É O NÚMERO DA NOTA
//
//	A IA preenche `dav_numero` e às vezes deixa `numero` vazio. Como o DAV não
//	tem chave de acesso, a identidade dele para duplicidade é NÚMERO e valor —
//	sem número, a trava fica inerte justamente na família onde as repetições
//	acontecem.
func TestDAVSemNumeroHerdaONumeroDoDAV(t *testing.T) {
	l := &Leitura{Tipo: "dav", DAV: "19387", ValorTotal: 98.10}
	l.Arrumar()
	if l.Numero != "19387" {
		t.Errorf("o DAV 19387 ficou com número %q — sem número, ele não é comparável com ninguém", l.Numero)
	}
}

func TestNumeroDaNotaNaoEAtropeladoPeloDAV(t *testing.T) {
	// NF 19702: número 19702 e um número de pedido 92080 impresso na DANFE.
	l := &Leitura{Tipo: "nf", Numero: "19702", DAV: "92080"}
	l.Arrumar()
	if l.Numero != "19702" {
		t.Errorf("o número da nota virou %q — o pedido impresso ao lado não é a nota", l.Numero)
	}
}

// CEP NÃO É TICKET
//
//	A caixa do DAV às vezes recebe o endereço de entrega digitado pelo operador,
//	e foi o que apareceu em 26/08/2026. "60455-100" tem cinco dígitos e uma
//	fronteira de palavra no hífen.
func TestCEPNoEnderecoNaoViraTicket(t *testing.T) {
	obs := "Endereço: AVENIDA ENGENHEIRO HEITOR DE OLIVEIRA ALBUQUERQUE, 295 - Bairro Papicu - 60455-100"
	if achados := Tickets(obs); len(achados) > 0 {
		t.Errorf("o endereço virou os tickets %v", achados)
	}
}

// E o ticket de verdade continua sendo achado no meio do mesmo tipo de texto.
func TestTicketNoMeioDoEnderecoAindaEAchado(t *testing.T) {
	obs := "131231 -VALECIO-MIGUEL DIAS"
	achados := Tickets(obs)
	if len(achados) != 1 || achados[0] != 131231 {
		t.Errorf("achou %v, esperava só o 131231", achados)
	}
}
