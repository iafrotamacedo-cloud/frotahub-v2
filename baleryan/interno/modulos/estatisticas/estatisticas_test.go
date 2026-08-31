// rev 1 — as travas do retrato
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	O retrato é um contrato entre TRÊS lugares que não se conhecem: as nove
//	views da migração 042, a lista `doRetrato` aqui no Go, e a conta da tela em
//	JavaScript. Os três falam em nome de coluna, e nenhum dos três avisa o outro
//	quando muda.
//
//	O modo de falhar não é um erro na tela. É pior: a coluna sai de lugar e a
//	tela desenha prioridade na casa do status, ou uma tabela chega cortada e o
//	gráfico mostra uma queda no movimento que nunca houve. Nos dois casos a tela
//	fica bonita, responde rápido, e está errada.
package estatisticas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

const arquivoDaMigracao = "042_as_estatisticas_do_contrato.sql"

func migracao(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join("..", "..", "..", "..", "db", "migrations", arquivoDaMigracao)
	bruto, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("não achei a migração das estatísticas em %s: %v", caminho, err)
	}
	return string(bruto)
}

// corpoDaView devolve o texto entre `create or replace view <nome>` e o `;` que
// fecha o select. É nele que os nomes de coluna moram.
func corpoDaView(t *testing.T, sql, nome string) string {
	t.Helper()
	abre := strings.Index(sql, "create or replace view "+nome+"\n")
	if abre < 0 {
		t.Fatalf("a view %s não existe na migração %s", nome, arquivoDaMigracao)
	}
	resto := sql[abre:]
	fim := strings.Index(resto, ";")
	if fim < 0 {
		t.Fatalf("a view %s não termina em ';' — não consigo ler as colunas dela", nome)
	}
	return resto[:fim]
}

// ---------------------------------------------------------------------------
// o contrato com o banco
// ---------------------------------------------------------------------------

// A LISTA DO GO E A VIEW DO BANCO TÊM QUE FALAR A MESMA COISA
//
//	`doRetrato` monta o `select=` que vai ao PostgREST. Pedir uma coluna que a
//	view não tem devolve erro 400 — e devolve na CARA DO USUÁRIO, em produção,
//	porque nenhum teste chega ao banco. Este teste lê as duas fontes e as
//	compara antes disso (P-36).
func TestTodaColunaDoRetratoExisteNaView(t *testing.T) {
	sql := migracao(t)
	for _, tab := range doRetrato {
		corpo := corpoDaView(t, sql, tab.view)
		for _, c := range tab.colunas {
			// A view pode entregar a coluna com o nome de origem (`u.nome`), com
			// apelido (`as unidade`) ou como expressão apelidada. Em todos os
			// casos o NOME QUE SAI aparece no corpo.
			if !strings.Contains(corpo, c) {
				t.Errorf("%s: o Go pede a coluna %q e a view %s não a entrega",
					tab.nome, c, tab.view)
			}
		}
	}
}

// A ORDEM DE CADA TABELA É OBRIGATÓRIA
//
//	Sem `order=`, o banco devolve na ordem que quiser. Dois retratos do mesmo
//	dado sairiam diferentes, e conferir um contra o outro — que é como toda esta
//	fusão foi validada — deixaria de ser possível.
func TestTodaTabelaDoRetratoTemOrdemFixa(t *testing.T) {
	for _, tab := range doRetrato {
		if strings.TrimSpace(tab.ordem) == "" {
			t.Errorf("%s sai do banco sem ordem definida", tab.nome)
		}
	}
}

// A TRANCA TEM QUE ESTAR EM TODAS AS NOVE (P-35)
//
//	Estas views entregam a operação inteira do contrato: chamados, custos,
//	margem e faturamento. Uma sem `security_invoker` roda como dona das tabelas
//	e ignora toda política — foi o buraco que a migração 033 fechou.
func TestTodaViewDoRetratoCarregaATranca(t *testing.T) {
	sql := migracao(t)
	for _, tab := range doRetrato {
		corpo := corpoDaView(t, sql, tab.view)
		if !strings.Contains(corpo, "with (security_invoker = true)") {
			t.Errorf("a view %s não repete `with (security_invoker = true)` — "+
				"ela passa a rodar como dona das tabelas", tab.view)
		}
	}
}

// ROTINA QUE O MOTOR EXIGE E O BANCO NÃO CONHECE É UMA TELA QUE NÃO ABRE PARA
// NINGUÉM — e não dá erro: `Exige` simplesmente nunca acha a permissão.
func TestARotinaDoModuloEstaNoCatalogo(t *testing.T) {
	sql := migracao(t)
	if !strings.Contains(sql, "'"+RotinaModulo+"'") {
		t.Fatalf("a rotina %s não é cadastrada em %s", RotinaModulo, arquivoDaMigracao)
	}
	if !strings.Contains(sql, "insert into rotinas") {
		t.Error("a migração não insere nada no catálogo de rotinas")
	}
	if !strings.Contains(sql, "insert into categoria_permissoes") {
		t.Error("a rotina nasce sem ninguém marcado — o menu some para todo mundo")
	}
}

// A DECISÃO DE 30/08/2026: A DESCRIÇÃO DO CHAMADO NÃO ATRAVESSA
//
//	Ela pesava 114,4 KB — 14% do retrato — para desenhar dez linhas, que hoje
//	levam para Dados do Trílogo. É o tipo de coluna que volta sozinha no dia em
//	que alguém quiser um texto na tela e achar mais fácil pedir de novo.
func TestADescricaoDoChamadoNaoSaiNoRetrato(t *testing.T) {
	for _, tab := range doRetrato {
		if tab.nome != "chamados" {
			continue
		}
		for _, c := range tab.colunas {
			if c == "descricao" {
				t.Fatal("a descrição do chamado voltou ao retrato: são 114 KB " +
					"para dez linhas que já levam a Dados do Trílogo")
			}
		}
		corpo := corpoDaView(t, migracao(t), tab.view)
		if strings.Contains(corpo, "descricao") {
			t.Error("a view estatisticas_chamados voltou a entregar a descrição")
		}
		return
	}
	t.Fatal("a tabela `chamados` sumiu do retrato")
}

// UMA LISTA SÓ DÁ NOME E PEDE O DADO
//
//	Se o `select=` e os rótulos saíssem de listas diferentes, bastaria alguém
//	acrescentar uma coluna no meio de UMA delas para a tela ler o valor errado
//	debaixo do nome certo — sem erro nenhum aparecer.
func TestOSelectPedeExatamenteAsColunasQueSaem(t *testing.T) {
	for _, tab := range doRetrato {
		if got, quer := selecao(tab.colunas), strings.Join(tab.colunas, ","); got != quer {
			t.Errorf("%s: o select pede %q e a tela recebe os nomes %q", tab.nome, got, quer)
		}
	}
}

// ---------------------------------------------------------------------------
// a montagem da resposta
// ---------------------------------------------------------------------------

// bancoDeMentira responde o JSON dado, com o Content-Range que o `count=exact`
// produziria.
func bancoDeMentira(t *testing.T, corpo, contentRange string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Range", contentRange)
		_, _ = w.Write([]byte(corpo))
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

var tabelaDeProva = tabela{
	nome: "prova", view: "estatisticas_prova",
	colunas: []string{"numero", "unidade", "conta"}, ordem: "numero",
}

// O BANCO NÃO PROMETE ORDEM DE CHAVE NENHUMA
//
//	O JSON do PostgREST é um objeto por linha, e a ordem das chaves dentro dele
//	não é contrato. Quem manda é `colunas` — e é isso que faz a tela poder ler
//	`linha[1]` sabendo que ali está a loja.
func TestALinhaSaiNaOrdemDasColunas(t *testing.T) {
	m := bancoDeMentira(t, `[{"conta":"civil","numero":131808,"unidade":26}]`, "0-0/1")
	pc, err := m.carregar(context.Background(), tabelaDeProva, "cliente-1")
	if err != nil {
		t.Fatalf("não deveria falhar: %v", err)
	}
	if len(pc.Linhas) != 1 {
		t.Fatalf("esperava 1 linha, vieram %d", len(pc.Linhas))
	}
	quero := []string{"131808", "26", `"civil"`}
	for i, q := range quero {
		if got := string(pc.Linhas[0][i]); got != q {
			t.Errorf("coluna %q (posição %d): esperava %s e veio %s",
				tabelaDeProva.colunas[i], i, q, got)
		}
	}
}

// COLUNA VAZIA VIRA NULO NO LUGAR DELA, NÃO UM BURACO
//
//	Se a coluna que falta apenas deixasse de entrar, todas as seguintes andariam
//	uma casa para trás — e a tela leria a conta na posição da loja. Um `null` na
//	casa certa é o único jeito de a linha continuar significando o que diz.
func TestColunaQueFaltaViraNuloEmVezDeDeslocarALinha(t *testing.T) {
	m := bancoDeMentira(t, `[{"numero":9,"conta":"civil"}]`, "0-0/1")
	pc, err := m.carregar(context.Background(), tabelaDeProva, "cliente-1")
	if err != nil {
		t.Fatalf("não deveria falhar: %v", err)
	}
	linha := pc.Linhas[0]
	if string(linha[1]) != "null" {
		t.Errorf("a coluna ausente devia virar null e virou %s", string(linha[1]))
	}
	if string(linha[2]) != `"civil"` {
		t.Errorf("a conta saiu do lugar: esperava \"civil\" na posição 2 e veio %s", string(linha[2]))
	}
}

// RETRATO CORTADO É RECUSADO, NÃO ENTREGUE PELA METADE
//
//	Uma tabela cortada não parece defeito na tela: parece uma queda no
//	movimento. E alguém decide em cima disso.
func TestRetratoCortadoEhRecusado(t *testing.T) {
	// O banco diz que tem 5 linhas e mandou 1.
	m := bancoDeMentira(t, `[{"numero":1,"unidade":2,"conta":"civil"}]`, "0-0/5")
	if _, err := m.carregar(context.Background(), tabelaDeProva, "cliente-1"); err == nil {
		t.Fatal("aceitou um retrato cortado — a tela desenharia o gráfico a menos")
	}
}

// CONTAGEM QUE NÃO VEIO TAMBÉM É RECUSA
//
//	Sem o Content-Range não há como afirmar que veio tudo. Servir mesmo assim é
//	trocar uma incerteza por um número que parece certo.
func TestSemContagemOModuloNaoAfirmaQueVeioTudo(t *testing.T) {
	m := bancoDeMentira(t, `[{"numero":1,"unidade":2,"conta":"civil"}]`, "")
	if _, err := m.carregar(context.Background(), tabelaDeProva, "cliente-1"); err == nil {
		t.Fatal("serviu a tabela sem poder provar que ela veio inteira")
	}
}

// O VALOR ATRAVESSA O MOTOR BYTE A BYTE
//
//	É o motivo de as linhas serem `json.RawMessage` e não `any`. Decodificado
//	para `any`, todo número passa por float64 e é REESCRITO na saída: o
//	`52.70` que o banco mandou vira `52.7`, e um `1.0` vira `1`.
//
//	Em JavaScript os dois valem o mesmo, e é por isso que este defeito não
//	apareceria na tela. Ele aparece em outro lugar: esta fusão inteira foi
//	conferida comparando a resposta do motor com o retrato offline, número
//	contra número. Um valor que muda de forma no caminho não quebra a tela —
//	quebra a única prova de que a tela continua certa.
func TestOValorChegaComoOBancoMandou(t *testing.T) {
	m := bancoDeMentira(t, `[{"numero":1,"unidade":52.70,"conta":"civil"}]`, "0-0/1")
	pc, err := m.carregar(context.Background(), tabelaDeProva, "cliente-1")
	if err != nil {
		t.Fatalf("não deveria falhar: %v", err)
	}
	if got := string(pc.Linhas[0][1]); got != "52.70" {
		t.Errorf("o banco mandou 52.70 e o motor entregou %s — a conferência "+
			"contra o retrato deixa de fechar", got)
	}
}

// A RESPOSTA TEM QUE SER LEGÍVEL DO OUTRO LADO
//
//	`peca` é o que a tela desempacota. Se ela deixar de sair como
//	`{colunas, linhas}`, a tela não acha nada — e o erro aparece no navegador,
//	não aqui.
func TestAPecaSaiComoColunasELinhas(t *testing.T) {
	m := bancoDeMentira(t, `[{"numero":1,"unidade":2,"conta":"civil"}]`, "0-0/1")
	pc, err := m.carregar(context.Background(), tabelaDeProva, "cliente-1")
	if err != nil {
		t.Fatalf("não deveria falhar: %v", err)
	}
	bruto, err := json.Marshal(pc)
	if err != nil {
		t.Fatalf("a peça não vira JSON: %v", err)
	}
	var lido struct {
		Colunas []string            `json:"colunas"`
		Linhas  [][]json.RawMessage `json:"linhas"`
	}
	if err := json.Unmarshal(bruto, &lido); err != nil {
		t.Fatalf("a tela não conseguiria ler a peça: %v", err)
	}
	if len(lido.Colunas) != 3 {
		t.Errorf("esperava 3 colunas nomeadas e vieram %d", len(lido.Colunas))
	}
}

// O CLIENTE DA REQUISIÇÃO FILTRA A CONSULTA
//
//	Sem o filtro por cliente, o retrato de um cliente entregaria a operação de
//	outro — e este módulo é leitura de tudo o que o contrato tem (CORE-11).
func TestOFiltroPorClienteVaiNaConsulta(t *testing.T) {
	visto := make(chan string, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case visto <- r.URL.RequestURI():
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Range", "*/0")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	m := &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}

	if _, err := m.carregar(context.Background(), tabelaDeProva, "cliente-42"); err != nil {
		t.Fatalf("não deveria falhar: %v", err)
	}
	uri := <-visto
	if !strings.Contains(uri, "cliente_id=eq.cliente-42") {
		t.Errorf("a consulta foi ao banco sem filtrar por cliente: %s", uri)
	}
}
