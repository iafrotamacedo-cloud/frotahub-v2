// rev 1 — as travas das migrações
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	Duas classes de defeito que já aconteceram neste projeto, as duas em
//	silêncio — sem erro, sem aviso, e sem nada quebrar na hora:
//
//	  1. Uma migração que roda e não se registra. A 019 criou coluna e índices,
//	     ficou de pé, e nunca escreveu a própria linha em `schema_migrations`.
//	     Resultado: a tabela que existe para responder "o que já rodou?" mentia,
//	     e a única forma de descobrir era conferir objeto por objeto.
//
//	  2. Um `create or replace view` que derruba a tranca da view. As views
//	     nasceram com `with (security_invoker = true)`, que manda a consulta
//	     rodar com os privilégios de QUEM PERGUNTA. `create or replace` não
//	     preserva essa opção: sem repetir a cláusula, ela volta ao padrão e a
//	     view passa a rodar como dona das tabelas — ignorando as políticas.
//
//	     Da 019 até a 031, oito migrações reescreveram essas views sem a
//	     cláusula. Medido em 27/08/2026, rodando como `anon` (o papel de quem
//	     NÃO fez login): as tabelas devolviam 0 linhas e as views devolviam 91
//	     notas e 703 orçamentos, com valor, fornecedor e loja. A chave que o
//	     navegador usa viaja dentro do JavaScript do site.
//
//	Nenhum teste do sistema pegava nem um nem outro, porque os dois deixam o
//	banco funcionando perfeitamente — só que errado.
package orcamentos

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A LINHA DE CORTE
//
//	As migrações até a 032 são HISTÓRIA: já rodaram no banco de verdade, e
//	reescrever o arquivo de uma migração aplicada faz o repositório mentir sobre
//	o que foi executado. Elas ficam como estão, com os defeitos que tiveram —
//	que a 033 conserta no banco, e este comentário registra.
//
//	Da 033 em diante a regra vale, e vale por arquivo. Um NÚMERO com motivo
//	escrito é honesto; uma lista de arquivos perdoados, mantida à mão, é onde o
//	próximo defeito se esconde — e seria a mesma armadilha que estes testes
//	existem para fechar.
const primeiraCobrada = "033"

func migracoes(t *testing.T) []string {
	t.Helper()
	arquivos, err := filepath.Glob("../../../../db/migrations/*.sql")
	if err != nil || len(arquivos) == 0 {
		t.Skipf("não achei as migrações: %v", err)
	}
	sort.Strings(arquivos)
	return arquivos
}

// TODA MIGRAÇÃO SE REGISTRA
//
//	A regra da casa (P-05) é que o banco guarda o que já rodou. Uma migração que
//	não se registra quebra essa promessa para todas as outras: a partir dela,
//	`schema_migrations` deixa de ser resposta e vira palpite.
func TestTodaMigracaoSeRegistra(t *testing.T) {
	for _, caminho := range migracoes(t) {
		nome := filepath.Base(caminho)
		if nome[:3] < primeiraCobrada {
			continue
		}
		bruto, err := os.ReadFile(caminho)
		if err != nil {
			t.Errorf("%s: não consegui ler: %v", nome, err)
			continue
		}
		texto := string(bruto)
		if !strings.Contains(texto, "insert into schema_migrations") {
			t.Errorf("%s não escreve a própria linha em `schema_migrations` — "+
				"ela vai rodar e a tabela que responde \"o que já rodou?\" não vai saber. "+
				"Foi o que aconteceu com a 019.", nome)
			continue
		}
		// E registra o número DELA, não o de outra.
		versao := nome[:3]
		if !strings.Contains(texto, "'"+versao+"'") {
			t.Errorf("%s registra uma versão que não é a dela (esperava '%s') — "+
				"copiar o rodapé da migração anterior e esquecer de trocar o número "+
				"faz duas migrações brigarem pela mesma linha", nome, versao)
		}
	}
}

// A TRANCA DA VIEW NÃO PODE CAIR NUMA REESCRITA
//
//	`create or replace view` substitui as opções da view pelo que vier no
//	`with`. Sem a cláusula, `security_invoker` volta a `false` e a view passa a
//	ignorar as políticas de quem pergunta.
//
//	O teste olha as views que carregam DADO DE NEGÓCIO. `chamados_lista` e as
//	outras entram pela mesma régua: se um dia alguém as reescrever, a cláusula
//	tem que ir junto.
func TestReescreverViewNaoDerrubaOSecurityInvoker(t *testing.T) {
	// Casa `create [or replace] view <nome>` e captura o que vem até o `as`.
	criacao := regexp.MustCompile(`(?is)create\s+(?:or\s+replace\s+)?view\s+([a-z_]+)(.*?)\bas\b`)

	for _, caminho := range migracoes(t) {
		nome := filepath.Base(caminho)
		if nome[:3] < primeiraCobrada {
			continue
		}
		bruto, err := os.ReadFile(caminho)
		if err != nil {
			continue
		}
		// Fora os comentários: a 033 e a 034 EXPLICAM a armadilha citando o
		// comando sem a cláusula, e explicar não é cometer.
		texto := regexp.MustCompile(`(?m)^\s*--[^\n]*`).ReplaceAllString(string(bruto), "")

		for _, m := range criacao.FindAllStringSubmatch(texto, -1) {
			view, meio := m[1], m[2]
			if !strings.Contains(strings.ToLower(meio), "security_invoker") {
				t.Errorf("%s: `create view %s` sem `with (security_invoker = true)` — "+
					"a view volta a rodar como dona das tabelas e ignora as políticas. "+
					"Rodando como `anon`, isso entregou 91 notas e 703 orçamentos "+
					"a quem não fez login (ver migração 033).", nome, view)
			}
		}
	}
}

// A COLUNA NOVA VAI NO FIM — e o teste sabe ler a ordem
//
//	`create or replace view` congela nome, ordem e tipo do que já existe. Pôr
//	coluna nova no meio faz o Postgres recusar com "cannot change name of view
//	column" — que foi como a primeira 031 morreu, depois de a regra estar
//	escrita no comentário da 029.
//
//	Este teste não substitui rodar a migração num Postgres de verdade (P-24);
//	ele pega o caso óbvio antes disso: a mesma view reescrita duas vezes no
//	repositório, com a segunda encurtando a lista da primeira.
func TestViewReescritaNuncaPerdeColuna(t *testing.T) {
	ultima := map[string][]string{}
	corpo := regexp.MustCompile(`(?is)create\s+or\s+replace\s+view\s+([a-z_]+).*?\bas\b(.*?);\s*$`)

	for _, caminho := range migracoes(t) {
		nome := filepath.Base(caminho)
		bruto, err := os.ReadFile(caminho)
		if err != nil {
			continue
		}
		texto := regexp.MustCompile(`(?m)^\s*--[^\n]*`).ReplaceAllString(string(bruto), "")
		for _, m := range corpo.FindAllStringSubmatch(texto+"\n", -1) {
			view := m[1]
			cols := colunasDoSelect(m[2])
			if len(cols) == 0 {
				continue
			}
			antes, tinha := ultima[view]
			if tinha {
				for k := range antes {
					if k >= len(cols) {
						t.Errorf("%s: a view %s perdeu a coluna %q, que existia antes — "+
							"`create or replace view` não deixa remover coluna", nome, view, antes[k])
						break
					}
					if antes[k] != cols[k] {
						t.Errorf("%s: a view %s trocou a coluna da posição %d de %q para %q — "+
							"nome e ordem do que já existe são congelados; coluna nova só no fim",
							nome, view, k+1, antes[k], cols[k])
						break
					}
				}
			}
			ultima[view] = cols
		}
	}
}

// colunasDoSelect extrai os apelidos do `select` de nível zero.
//
// É um leitor de SQL bem burro, e de propósito: ele só precisa acertar a LISTA
// de colunas. Qualquer coisa que ele não entenda vira uma lista vazia, e o
// teste pula — errar para o lado de não acusar é o certo aqui, porque a prova
// de verdade é a migração rodando num Postgres (P-24).
func colunasDoSelect(sql string) []string {
	i := strings.Index(strings.ToUpper(sql), "SELECT")
	if i < 0 {
		return nil
	}
	sql = sql[i+6:]

	prof, fim := 0, -1
	for j := 0; j < len(sql)-4; j++ {
		switch sql[j] {
		case '(':
			prof++
		case ')':
			prof--
		}
		if prof == 0 && strings.EqualFold(sql[j:j+4], "FROM") &&
			(j == 0 || !ehPalavra(sql[j-1])) && !ehPalavra(sql[j+4]) {
			fim = j
		}
		if fim >= 0 {
			break
		}
	}
	if fim < 0 {
		return nil
	}

	var partes []string
	prof, atual := 0, strings.Builder{}
	for _, c := range sql[:fim] {
		switch c {
		case '(':
			prof++
		case ')':
			prof--
		}
		if c == ',' && prof == 0 {
			partes = append(partes, atual.String())
			atual.Reset()
			continue
		}
		atual.WriteRune(c)
	}
	if strings.TrimSpace(atual.String()) != "" {
		partes = append(partes, atual.String())
	}

	apelido := regexp.MustCompile(`(?i)\bAS\s+([a-z_][a-z0-9_]*)\s*$`)
	fora := make([]string, 0, len(partes))
	for _, p := range partes {
		p = strings.Join(strings.Fields(p), " ")
		if m := apelido.FindStringSubmatch(p); m != nil {
			fora = append(fora, strings.ToLower(m[1]))
			continue
		}
		pedaco := p[strings.LastIndex(p, ".")+1:]
		fora = append(fora, strings.ToLower(strings.TrimSpace(pedaco)))
	}
	return fora
}

func ehPalavra(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// A 033 TEM QUE CONSERTAR AS QUATRO VIEWS QUE FICARAM ABERTAS
//
//	A linha de corte acima perdoa o passado. Só que o passado deixou quatro
//	views rodando como dona das tabelas — e perdoar o arquivo não pode virar
//	perdoar o banco. Este teste é a outra metade: a 033 precisa existir e
//	precisa nomear as quatro.
//
//	Se alguém apagar a 033 achando que é redundante, o buraco volta calado, e
//	nenhum outro teste percebe.
func TestA033ConsertaAsViewsQueFicaramAbertas(t *testing.T) {
	var texto string
	for _, caminho := range migracoes(t) {
		if strings.HasPrefix(filepath.Base(caminho), "033") {
			b, err := os.ReadFile(caminho)
			if err != nil {
				t.Fatalf("não consegui ler a 033: %v", err)
			}
			texto = string(b)
		}
	}
	if texto == "" {
		t.Fatal("a migração 033 sumiu — sem ela, as views abertas pela 019 " +
			"continuam entregando nota e orçamento a quem não fez login")
	}
	for _, view := range []string{
		"documentos_lista", "orcamentos_lista",
		"orcamentos_painel", "pedidos_faturamento_lista",
	} {
		if !regexp.MustCompile(`alter\s+view\s+` + view + `\s+set\s*\(\s*security_invoker`).MatchString(texto) {
			t.Errorf("a 033 não tranca a view %s", view)
		}
	}
}
