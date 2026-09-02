// rev 1 — as travas da consolidação
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	A consolidação é um contrato entre TRÊS lugares que não se conhecem: as duas
//	views da migração 043, as duas listas de colunas aqui no Go, e a tela em
//	JavaScript. Os três falam em nome de coluna, e nenhum avisa o outro quando
//	muda.
//
//	O modo de falhar é o pior possível numa tela de dinheiro: ela abre, responde
//	rápido, e fecha uma conta errada com ar de certa.
package consolidacao

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

const arquivoDaMigracao = "043_a_consolidacao.sql"

func migracao(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join("..", "..", "..", "..", "db", "migrations", arquivoDaMigracao)
	bruto, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("não achei a migração da consolidação em %s: %v", caminho, err)
	}
	return string(bruto)
}

// corpoDaView devolve o SQL da view, sem os comentários.
//
// OS COMENTÁRIOS SAEM ANTES DE QUALQUER COISA
//
//	A primeira versão deste ajudante cortava no primeiro `;` do texto — e o
//	primeiro `;` da migração está DENTRO de um comentário ("corrige a
//	associação; `distinct` é o que impede"). O corpo saía com três linhas, cinco
//	testes falhavam, e o defeito estava no teste.
//
//	Pior: se o comentário citasse `security_invoker`, o teste da tranca passaria
//	sem a tranca existir. Comentário não é código, e não pode responder por ele.
func corpoDaView(t *testing.T, sql, nome string) string {
	t.Helper()
	abre := strings.Index(sql, "create view "+nome+"\n")
	if abre < 0 {
		t.Fatalf("a view %s não existe na migração %s", nome, arquivoDaMigracao)
	}
	limpo := semComentarios(sql[abre:])
	fim := strings.Index(limpo, ";")
	if fim < 0 {
		t.Fatalf("a view %s não termina em ';' — não consigo ler as colunas dela", nome)
	}
	return limpo[:fim]
}

// semComentarios apaga tudo de `--` até o fim da linha, preservando as quebras.
func semComentarios(sql string) string {
	var b strings.Builder
	for _, linha := range strings.Split(sql, "\n") {
		if i := strings.Index(linha, "--"); i >= 0 {
			linha = linha[:i]
		}
		b.WriteString(linha)
		b.WriteByte('\n')
	}
	return b.String()
}

// entrega diz se a view expõe a coluna com ESTE nome.
//
// A conferência é por fim de item — `as nome` ou `tabela.nome`, seguido de
// vírgula ou fim de linha. Um `strings.Contains` cru daria falso positivo em
// toda coluna cujo nome é prefixo de outra: `pago` casaria dentro de `pago_em`,
// e a tela pediria uma coluna que não existe achando que o teste conferiu.
func entrega(corpo, col string) bool {
	return regexp.MustCompile(`(?m)(?:\bas\s+|\.)` + regexp.QuoteMeta(col) + `\b\s*(?:,|$)`).
		MatchString(corpo)
}

func colunas(lista string) []string {
	fora := []string{}
	for _, c := range strings.Split(lista, ",") {
		if c = strings.TrimSpace(c); c != "" {
			fora = append(fora, c)
		}
	}
	return fora
}

// ---------------------------------------------------------------------------
// o contrato com o banco
// ---------------------------------------------------------------------------

// PEDIR COLUNA QUE A VIEW NÃO TEM DEVOLVE 400 NA CARA DO USUÁRIO
//
//	O `select=` vai montado daqui para o PostgREST. Um nome errado não quebra
//	compilação, não quebra teste nenhum que não chegue ao banco, e só aparece em
//	produção. Este teste lê as duas fontes e as compara antes disso (P-36).
func TestTodaColunaPedidaExisteNaView(t *testing.T) {
	sql := migracao(t)
	casos := []struct{ view, lista string }{
		{"consolidacao_notas", colunasDaNota},
		{"consolidacao_tickets", colunasDoTicket},
	}
	for _, c := range casos {
		corpo := corpoDaView(t, sql, c.view)
		for _, col := range colunas(c.lista) {
			if !entrega(corpo, col) {
				t.Errorf("%s: a view não entrega a coluna %q que o motor pede", c.view, col)
			}
		}
	}
}

// A TRANCA PRECISA ESTAR NAS DUAS (P-35)
//
//	Sem `security_invoker`, a view roda com os direitos de quem a criou e passa
//	por cima do RLS de todo mundo. Numa tela que mostra o que se pagou e o que
//	se cobrou, isso é o contrato de um cliente aparecendo na conta de outro.
func TestAsDuasViewsRodamComoQuemPergunta(t *testing.T) {
	sql := migracao(t)
	for _, view := range []string{"consolidacao_notas", "consolidacao_tickets"} {
		corpo := corpoDaView(t, sql, view)
		if !strings.Contains(corpo, "security_invoker = true") {
			t.Errorf("%s foi criada SEM security_invoker — ela fura o RLS", view)
		}
	}
}

// A NOTA OCULTA E A REPETIDA NÃO PODEM SOMAR
//
//	A primeira foi descartada por alguém; a segunda é a mesma nota contada duas
//	vezes. Qualquer uma das duas entrando na soma faz o total do período não
//	bater com nada, e ninguém descobre olhando.
func TestAAbaDasNotasIgnoraOcultaERepetida(t *testing.T) {
	corpo := corpoDaView(t, migracao(t), "consolidacao_notas")
	for _, trava := range []string{"d.oculto_em is null", "d.duplicada_de is null"} {
		if !strings.Contains(corpo, trava) {
			t.Errorf("a aba das notas perdeu a trava %q", trava)
		}
	}
}

// O ORÇAMENTO APAGADO NÃO É DINHEIRO
//
//	`removido_em` marca o orçamento desfeito. Somá-lo cobraria do cliente uma
//	coisa que alguém já decidiu que não existe.
func TestOOrcamentoRemovidoNaoEntraEmNenhumaDasAbas(t *testing.T) {
	sql := migracao(t)
	if !strings.Contains(corpoDaView(t, sql, "consolidacao_notas"), "orc.removido_em is null") {
		t.Error("a aba das notas soma orçamento apagado")
	}
	if !strings.Contains(corpoDaView(t, sql, "consolidacao_tickets"), "orc.removido_em is null") {
		t.Error("a aba dos tickets lista orçamento apagado")
	}
}

// O TICKET CONTA UMA VEZ SÓ
//
//	`documento_tickets` pode ter o mesmo ticket em duas linhas quando alguém
//	corrige a associação. Sem `distinct`, a nota diz que cobre três chamados
//	quando cobre dois.
func TestOTicketRepetidoNaoInflaAContagem(t *testing.T) {
	corpo := corpoDaView(t, migracao(t), "consolidacao_notas")
	if !strings.Contains(corpo, "count(distinct dt.ticket)") {
		t.Error("a contagem de tickets da nota não é distinta — ticket corrigido conta duas vezes")
	}
}

// A JUNÇÃO NÃO PODE MULTIPLICAR
//
//	Foi o erro medido na primeira versão desta tela: R$ 220.787,51 de pendente
//	onde o certo era R$ 59.185,21, porque dois `left join` viraram produto. A
//	forma que impede isso é sub-consulta por linha — `left join lateral` — e não
//	junção direta com `group by`.
func TestASomaDaNotaNaoMultiplicaPorTicket(t *testing.T) {
	corpo := corpoDaView(t, migracao(t), "consolidacao_notas")
	if strings.Contains(corpo, "group by") {
		t.Error("a aba das notas usa group by — é assim que a soma multiplica por ticket")
	}
	if strings.Count(corpo, "left join lateral") != 2 {
		t.Errorf("esperava as duas sub-consultas laterais, achei %d",
			strings.Count(corpo, "left join lateral"))
	}
}

// O ORÇAMENTO CUJA NOTA FOI JOGADA FORA PRECISA APARECER
//
//	Dois deles existem hoje (R$ 525,48, um já lançado no Trílogo). Eles não
//	entram na aba das notas — e está certo. Se também não fossem marcados na aba
//	dos tickets, seriam dinheiro cobrado que nenhuma tela mostra.
func TestOOrcamentoDeNotaExcluidaSaiMarcado(t *testing.T) {
	corpo := corpoDaView(t, migracao(t), "consolidacao_tickets")
	if !strings.Contains(corpo, "nota_excluida") {
		t.Fatal("a aba dos tickets não marca o orçamento de nota excluída")
	}
	if !strings.Contains(corpo, "bool_and(") {
		t.Error("a marca precisa valer só quando TODAS as notas do orçamento foram excluídas")
	}
	if !strings.Contains(colunasDoTicket, "nota_excluida") {
		t.Error("a view marca, mas o motor não pede a coluna — a tela nunca vai ver")
	}
}

// ---------------------------------------------------------------------------
// o que o motor faz com o que chega
// ---------------------------------------------------------------------------

// bancoQueResponde devolve um cliente cujo PostgREST de mentira responde o
// corpo dado, com o cabeçalho de contagem que `contagem` pedir.
func bancoQueResponde(t *testing.T, corpo, contagem string) *banco.Cliente {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if contagem != "" {
			w.Header().Set("Content-Range", contagem)
		}
		_, _ = w.Write([]byte(corpo))
	}))
	t.Cleanup(s.Close)
	return banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})
}

// LISTA CORTADA É PIOR QUE LISTA NENHUMA
//
//	Meia lista numa tela de conferência fecha uma conta errada com ar de certa.
//	Se o banco diz que há mais linhas do que chegaram, a resposta inteira é
//	recusada.
func TestListaCortadaPeloTetoERecusada(t *testing.T) {
	m := &Modulo{bd: bancoQueResponde(t, `[{"nf":"1"},{"nf":"2"}]`, "0-1/900")}
	_, err := m.carregar(context.Background(), "consolidacao_notas", colunasDaNota,
		"inserido_em.desc", "c1")
	if err == nil {
		t.Fatal("o banco disse 900 linhas, chegaram 2, e o motor aceitou")
	}
	if !strings.Contains(err.Error(), "900") || !strings.Contains(err.Error(), "chegaram 2") {
		t.Errorf("o erro não diz o tamanho do estrago: %v", err)
	}
}

// Lista vazia é resposta legítima — nota nenhuma no período, por exemplo — e
// tem que sair como `[]`. `null` derruba o `.map` da tela e leva a página
// inteira junto, que foi como a tela de Estatísticas caiu em 31/08/2026.
func TestListaVaziaSaiComoListaENaoComoNulo(t *testing.T) {
	m := &Modulo{bd: bancoQueResponde(t, `[]`, "*/0")}
	fora, err := m.carregar(context.Background(), "consolidacao_notas", colunasDaNota,
		"inserido_em.desc", "c1")
	if err != nil {
		t.Fatalf("lista vazia virou erro: %v", err)
	}
	if fora == nil {
		t.Fatal("lista vazia saiu como nil — a tela quebra ao fazer .map")
	}
	b, _ := json.Marshal(fora)
	if string(b) != "[]" {
		t.Errorf("saiu %s, esperava []", b)
	}
}

// E A LISTA VAZIA PRECISA SOBREVIVER TAMBÉM AO CORPO `null`
//
//	`[]` já chega como fatia vazia sozinho — este caso não exercita guarda
//	nenhuma. O que exercita é o corpo que decodifica para nil, e é ele que
//	derrubaria a tela: `null.map` não existe, e o React leva a árvore inteira
//	junto. Foi assim que a tela de Estatísticas caiu em 31/08/2026.
func TestCorpoNuloTambemSaiComoLista(t *testing.T) {
	m := &Modulo{bd: bancoQueResponde(t, `null`, "*/0")}
	fora, err := m.carregar(context.Background(), "consolidacao_notas", colunasDaNota,
		"inserido_em.desc", "c1")
	if err != nil {
		t.Fatalf("corpo nulo virou erro: %v", err)
	}
	b, _ := json.Marshal(fora)
	if string(b) != "[]" {
		t.Errorf("saiu %s, esperava []", b)
	}
}

// O NÚMERO ATRAVESSA O MOTOR COMO VEIO
//
//	`numeric` do Postgres não cabe em float64. Nos valores do dia a dia a volta
//	é inofensiva — 587,98 continua 587,98 —, e é justamente por isso que o
//	defeito não aparece em teste feito com número pequeno: ele espera a soma de
//	um período grande, ou um id longo, para virar notação científica.
//
//	Por isso a medida aqui é com um número que float64 NÃO representa. Se este
//	teste passar, o caminho é byte a byte de verdade; se ele começar a falhar,
//	alguém trocou `json.RawMessage` por `any` em algum lugar.
func TestONumeroNaoPassaPorFloat(t *testing.T) {
	const exato = "12345678901234567890.12"
	m := &Modulo{bd: bancoQueResponde(t,
		`[{"ticket":126834,"valor":`+exato+`,"nfs":"662491"}]`, "0-0/1")}
	fora, err := m.carregar(context.Background(), "consolidacao_tickets", colunasDoTicket,
		"criado_em.desc", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(fora) != 1 {
		t.Fatalf("vieram %d linhas", len(fora))
	}
	if s := string(fora[0]["valor"]); s != exato {
		t.Errorf("o valor virou %s no caminho — esperava %s", s, exato)
	}
	if s := string(fora[0]["ticket"]); s != "126834" {
		t.Errorf("o ticket virou %s no caminho", s)
	}
}

// O FILTRO POR CLIENTE É O QUE SEPARA UM CONTRATO DO OUTRO. Sem ele a consulta
// traz a base inteira, e a nota de um cliente aparece na consolidação de outro.
func TestAConsultaVaiSempreFiltradaPorCliente(t *testing.T) {
	var visto string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Range", "*/0")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	m := &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
	if _, err := m.carregar(context.Background(), "consolidacao_notas", colunasDaNota,
		"inserido_em.desc", "c-do-sao-luiz"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(visto, "cliente_id=eq.c-do-sao-luiz") {
		t.Errorf("a consulta saiu sem o filtro de cliente: %s", visto)
	}
	if !strings.Contains(visto, "limit=20000") {
		t.Errorf("a consulta saiu sem teto: %s", visto)
	}
}

// A rota é uma só, e é de leitura. Um POST aqui seria uma tela de conferência
// que altera o que está conferindo.
func TestARotaEUmaSoEDeLeitura(t *testing.T) {
	mux := http.NewServeMux()
	(&Modulo{}).Montar(mux)
	if _, padrao := mux.Handler(httptest.NewRequest("GET", "/consolidacao", nil)); padrao != "GET /consolidacao" {
		t.Errorf("GET /consolidacao não está montado (achei %q)", padrao)
	}
	if _, padrao := mux.Handler(httptest.NewRequest("POST", "/consolidacao", nil)); padrao == "POST /consolidacao" {
		t.Error("existe um POST em /consolidacao — esta tela não grava nada")
	}
}
