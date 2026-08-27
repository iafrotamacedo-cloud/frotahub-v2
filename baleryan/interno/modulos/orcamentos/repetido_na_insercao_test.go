// rev 1 — o arquivo que já estava aqui
//
// O QUE ACONTECEU, EM 27/08/2026
//
//	O usuário mandou 10 arquivos e viu 9 na fila. Dois deles eram o MESMO
//	arquivo, byte a byte — a cópia "(1).jpg" que o navegador cria ao baixar duas
//	vezes. A trava do sha256 recusou o segundo, que é exatamente o que ela existe
//	para fazer.
//
//	Só que a tela dizia "1 já estava na lista". Com dez arquivos de nome UUID,
//	isso não identifica nada. Ele contou 10 contra 9 na mão e foi procurar
//	defeito no sistema — que estava certo, e calado.
//
//	A recusa não mudou. O que mudou é que agora ela diz QUAL, e de quem é cópia.
package orcamentos

import (
	"os"
	"strings"
	"testing"
)

// O TESTE OLHA A TELA QUE USA, NÃO O ARQUIVO DE TIPOS
//
//	A primeira versão procurava "ja_existia_como" no front INTEIRO — e passava
//	mesmo com a tela ignorando o campo, porque a declaração continua em
//	`tipos.ts`. Sabotei tirando o uso e o teste não reclamou: um campo declarado
//	e não lido é exatamente o defeito que ele deveria pegar.
func TestOFrontRecebeONomeDeQuemJaEstavaAqui(t *testing.T) {
	fonte, err := os.ReadFile("../../../../web/src/telas/orcamentos/Arquivos.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	if !strings.Contains(string(fonte), "a.ja_existia_como") {
		t.Error("a tela de inserção não lê `ja_existia_como` — ela volta a dizer só " +
			"quantos arquivos foram recusados, e o usuário fica sem saber quais. " +
			"Foi assim que 10 enviados e 9 na fila viraram caça a um defeito que não existia")
	}
}

// ouOutroNome NUNCA PODE DEVOLVER VAZIO
//
//	Vazio é o valor que significa "esta nota é nova". Uma linha antiga sem nome
//	no banco — ou com o `<nil>` que o `fmt.Sprint` produz de um campo nulo —
//	faria a nota repetida ser contada como nova, e a tela mostraria a fila com um
//	arquivo a menos e nenhuma explicação. É o defeito de 27/08, entrando por
//	outra porta.
func TestOuOutroNomeNuncaDevolveVazio(t *testing.T) {
	casos := []struct{ antigo, meu, quero string }{
		{"NF 19425.pdf", "copia.pdf", "NF 19425.pdf"},
		{"", "copia.pdf", "copia.pdf"},
		{"   ", "copia.pdf", "copia.pdf"},
		{"<nil>", "copia.pdf", "copia.pdf"},
	}
	for _, c := range casos {
		if got := ouOutroNome(c.antigo, c.meu); got != c.quero {
			t.Errorf("ouOutroNome(%q, %q) = %q, queria %q", c.antigo, c.meu, got, c.quero)
		}
		if ouOutroNome(c.antigo, c.meu) == "" {
			t.Errorf("ouOutroNome(%q, %q) devolveu vazio — a nota repetida seria contada como nova",
				c.antigo, c.meu)
		}
	}
}
