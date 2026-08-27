// rev 1 — coluna lida pela negativa não pode ser nula
//
// O DEFEITO QUE ESTE ARQUIVO GUARDA
//
//	Em 27/08/2026 o dono gerou mais de 40 orçamentos e o painel continuou
//	dizendo "1 PODEM SUBIR". Os 39 novos não apareceram nem no cartão nem na
//	fila. Nenhum estava travado: eles não eram VISTOS.
//
//	`precisa_decisao` era escrita assim, na 035:
//
//	  o.status = 'gerado' AND (o.lancamento_bloqueio = ANY (ARRAY['teto', ...]))
//
//	Em SQL, comparar com NULL não devolve `false` — devolve NULL. Todo orçamento
//	recém-gerado tem `lancamento_bloqueio` nulo, então recebia NULL. E quem lê a
//	coluna pergunta pela negativa: `NOT precisa_decisao` no painel,
//	`precisa_decisao=is.false` na tela. `NOT NULL` é NULL, e NULL não passa em
//	`WHERE` nem casa com `is.false`.
//
//	O orçamento sumia dos dois ao mesmo tempo. Sem erro, sem log, sem nada na
//	tela.
//
// POR QUE DEMOROU UM DIA PARA APARECER
//
//	`lancamento_bloqueio` deixa de ser nulo na primeira tentativa de lançamento.
//	Todos os orçamentos que existiam quando a 035 subiu já tinham sido tentados.
//	O defeito exigia alguém gerar e olhar o painel ANTES de tentar lançar.
package orcamentos

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A EXPRESSÃO DE `precisa_decisao` NÃO PODE DEIXAR NULL ESCAPAR
//
//	O teste olha a expressão de verdade — o `SELECT ... AS precisa` do LATERAL —
//	e não a menção da palavra COALESCE em algum lugar do arquivo. Casar menção
//	em vez de uso é o vício que já deixou duas sabotagens passarem hoje (P-42).
func TestPrecisaDecisaoNuncaPodeSerNulo(t *testing.T) {
	sql, err := os.ReadFile("../../../../db/migrations/039_precisa_decisao_nunca_e_nulo.sql")
	if err != nil {
		t.Fatalf("não achei a 039: %v", err)
	}
	regra := semComentarios(string(sql))

	fim := strings.Index(regra, "AS precisa)")
	if fim < 0 {
		t.Fatal("a 039 não define a expressão `precisa`")
	}
	ini := strings.LastIndex(regra[:fim], "SELECT o.status")
	if ini < 0 {
		t.Fatal("não achei o começo da expressão `precisa`")
	}
	expr := regra[ini:fim]

	if !strings.Contains(expr, "COALESCE(o.lancamento_bloqueio") {
		t.Errorf("a expressão de `precisa` compara `lancamento_bloqueio` sem "+
			"COALESCE. Bloqueio nulo — todo orçamento recém-gerado — vira NULL, e "+
			"a linha some do painel E da tela ao mesmo tempo.\nexpressão: %s", expr)
	}
}

// A TELA CONTINUA PERGUNTANDO PELA NEGATIVA — E ESTÁ CERTA
//
//	`is.false` é a pergunta certa: quem pode subir é quem NÃO precisa de decisão.
//	O que estava errado era a coluna responder NULL. Este teste existe para o
//	conserto não ser "trocar a pergunta" — trocar para `is.not.true` esconderia o
//	defeito em vez de resolvê-lo, e a próxima coluna nula repetiria a história.
func TestAFilaDeLancarContinuaPerguntandoPelaNegativa(t *testing.T) {
	fonte, err := os.ReadFile("gerar.go")
	if err != nil {
		t.Fatalf("não consegui ler o gerar.go: %v", err)
	}
	if !strings.Contains(string(fonte), `"&precisa_decisao=is.false"`) {
		t.Error("a fila de lançar deixou de perguntar `precisa_decisao=is.false` — " +
			"se o conserto foi mudar a pergunta em vez da coluna, o defeito só mudou de lugar")
	}
}

// NENHUMA MIGRAÇÃO NOVA PODE REPETIR O PADRÃO
//
//	`<coluna> = ANY (ARRAY[...])` sobre coluna que aceita nulo é a forma exata do
//	defeito. Vale para as migrações da 039 em diante: as anteriores já rodaram no
//	banco de verdade, e reescrever arquivo aplicado faz o repositório mentir
//	sobre o que foi executado (é a mesma linha de corte de `TodaMigracaoSeRegistra`).
func TestMigracaoNovaNaoComparaColunaNulavelComANY(t *testing.T) {
	arquivos, err := os.ReadDir("../../../../db/migrations")
	if err != nil {
		t.Skipf("não achei as migrações: %v", err)
	}
	// `o.lancamento_bloqueio = ANY (` e parentes: coluna qualificada, sem COALESCE.
	perigo := regexp.MustCompile(`(?i)\b([a-z_]+\.[a-z_]+)\s*=\s*ANY\s*\(\s*ARRAY\[\s*'`)

	for _, a := range arquivos {
		nome := a.Name()
		if !strings.HasSuffix(nome, ".sql") || nome < "039" {
			continue
		}
		b, err := os.ReadFile("../../../../db/migrations/" + nome)
		if err != nil {
			continue
		}
		for _, m := range perigo.FindAllStringSubmatch(semComentarios(string(b)), -1) {
			t.Errorf("%s compara %s com `= ANY (ARRAY[...])` sem COALESCE. Se essa "+
				"coluna aceitar nulo, a expressão devolve NULL em vez de false — e "+
				"quem a ler pela negativa perde a linha em silêncio (P-44).",
				nome, m[1])
		}
	}
}
