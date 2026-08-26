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
