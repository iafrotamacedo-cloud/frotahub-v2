// rev 1 — o motivo da falha, escrito para gente
package leitor

import (
	"strings"
	"testing"
)

// O CASO QUE APARECEU NA TELA EM 26/08/2026
func TestOModeloAposentadoVirouInstrucao(t *testing.T) {
	bruto := "a IA recusou: This model models/gemini-2.5-flash-lite is no longer " +
		"available to new users. Please update your code to use models/gemini-3.5-flash-lite " +
		"for the latest features and improvements."
	deu := EmPortugues(bruto)

	if !strings.Contains(deu, "aposentado") || !strings.Contains(deu, "GEMINI_MODELO") {
		t.Errorf("não diz o que fazer: %q", deu)
	}
	// O DETALHE NÃO SE PERDE
	//   Quem for investigar precisa da frase original; quem opera lê só a
	//   primeira parte. Traduzir jogando fora o original troca um problema por
	//   outro.
	if !strings.Contains(deu, "gemini-2.5-flash-lite") {
		t.Errorf("jogou fora o detalhe técnico: %q", deu)
	}
}

func TestCadaFalhaDizOQueFazer(t *testing.T) {
	casos := map[string]string{
		"a IA recusou: You exceeded your current quota": "meia-noite",
		"a IA está sobrecarregada: model is overloaded": "passageiro",
		"a IA recusou: API key not valid":               "GEMINI_API_KEY",
		"a IA recusou: request payload is too large":    "grande demais",
		"context deadline exceeded":                     "demorou demais",
	}
	for bruto, esperado := range casos {
		if deu := EmPortugues(bruto); !strings.Contains(deu, esperado) {
			t.Errorf("%q virou %q — esperava falar em %q", bruto, deu, esperado)
		}
	}
}

// FRASE JÁ EM PORTUGUÊS NÃO É TRADUZIDA DUAS VEZES
//
//	As recusas do próprio sistema já foram escritas para gente. Passá-las de novo
//	pelo tradutor produziria "a leitura falhou (a leitura falhou)".
func TestOQueJaEstaEmPortuguesPassaLimpo(t *testing.T) {
	deu := EmPortugues("a conta não fecha: 3 itens somam 514,60 e a nota diz 463,14")
	if strings.Count(deu, "a conta não fecha") > 1 {
		t.Errorf("repetiu a frase: %q", deu)
	}
	if !strings.Contains(deu, "Confira o papel") && !strings.Contains(deu, "confira o papel") {
		t.Errorf("perdeu a instrução: %q", deu)
	}
}

// SEM MOTIVO REGISTRADO, A TELA AINDA DIZ ALGUMA COISA
//
//	Campo vazio na tela lê-se como defeito do programa, e não como falta de dado.
func TestVazioNaoViraTelaVazia(t *testing.T) {
	if deu := EmPortugues("   "); deu == "" || !strings.Contains(deu, "falhou") {
		t.Errorf("deu %q", deu)
	}
}

// O DESCONHECIDO NÃO SAI EM INGLÊS CRU
func TestErroDesconhecidoGanhaMolduraEmPortugues(t *testing.T) {
	deu := EmPortugues("some brand new failure nobody mapped yet")
	if !strings.HasPrefix(deu, "a leitura desta nota falhou") {
		t.Errorf("saiu sem moldura: %q", deu)
	}
	if !strings.Contains(deu, "brand new failure") {
		t.Errorf("jogou fora o detalhe: %q", deu)
	}
}
