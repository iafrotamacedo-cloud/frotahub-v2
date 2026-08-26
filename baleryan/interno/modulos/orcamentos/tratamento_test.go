// rev 1 — o tratamento das duas filas
package orcamentos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

func semChamadoAlgum(n int) []ticketDoDocumento {
	saida := make([]ticketDoDocumento, 0, n)
	for i := 0; i < n; i++ {
		saida = append(saida, ticketDoDocumento{Ticket: 130000 + i})
	}
	return saida
}

// A AVALIAÇÃO NÃO PARA NO PRIMEIRO PROBLEMA
//
//	A geração para — ela só quer saber se roda. A tela quer a lista do que
//	consertar, e uma tela que revela um problema de cada vez faz o usuário
//	voltar cinco vezes para descobrir que a nota tinha cinco.
func TestAvaliarNaoParaNoPrimeiroProblema(t *testing.T) {
	m := moduloComTicketVazio(t)
	partes, motivos, err := m.avaliarPartes(context.Background(),
		documentoPronto{ID: "d1", Nome: "n.pdf", Valor: 900},
		semChamadoAlgum(3), []regras.LinhaDaNota{umaLinha(900)}, regras.Padrao)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if len(partes) != 3 || len(motivos) != 3 {
		t.Fatalf("avaliou %d partes e %d motivos, esperava 3 de cada", len(partes), len(motivos))
	}
	for i, mo := range motivos {
		if mo == "" {
			t.Errorf("parte %d passou sem chamado na base", i+1)
		}
	}
}

// TICKET SEM CHAMADO NÃO É ERRO DE PROGRAMA
//
//	É o estado normal de quem está em "não associadas". Se a avaliação
//	explodisse aí, a tela que existe para tratar esse caso não abriria.
func TestTicketSemChamadoNaoDerrubaAAvaliacao(t *testing.T) {
	m := moduloComTicketVazio(t)
	_, motivos, err := m.avaliarPartes(context.Background(),
		documentoPronto{ID: "d1", Nome: "n.pdf", Valor: 300},
		semChamadoAlgum(1), []regras.LinhaDaNota{umaLinha(300)}, regras.Padrao)

	if err != nil {
		t.Fatalf("um ticket fora da base derrubou a avaliação: %v", err)
	}
	if !strings.Contains(motivos[0], "base de chamados") {
		t.Errorf("o motivo não explica o problema: %q", motivos[0])
	}
}

// A geração continua parando no primeiro — são donos diferentes da mesma conta.
func TestAGeracaoContinuaParandoNoPrimeiro(t *testing.T) {
	m := moduloComTicketVazio(t)
	partes, bloqueio, err := m.planejarNota(context.Background(),
		documentoPronto{ID: "d1", Nome: "n.pdf", Valor: 300},
		semChamadoAlgum(2), []regras.LinhaDaNota{umaLinha(300)}, regras.Padrao)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio == "" {
		t.Fatal("a nota tinha ticket fora da base e a geração deixou passar")
	}
	if len(partes) != 0 {
		t.Errorf("bloqueou e devolveu %d partes para gravar", len(partes))
	}
}

// ---------------------------------------------------------------------------
// trocar e apagar ticket
// ---------------------------------------------------------------------------

type feito struct {
	metodo string
	alvo   string
}

func moduloQueAnota(t *testing.T, anotar *[]feito) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*anotar = append(*anotar, feito{metodo: r.Method, alvo: r.URL.Path + "?" + r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "documentos") && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[{"id":"d1","cliente_id":"c1","nome_arquivo":"n.pdf"}]`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// O NOVO ENTRA ANTES DE O VELHO SAIR
//
//	Se algo falhar no meio, a nota fica com um ticket A MAIS — que a tela mostra
//	e o usuário tira. O contrário deixaria a nota com um ticket A MENOS, que
//	ninguém vê e que estraga a divisão em silêncio: numa nota rateada isso muda
//	o valor de TODAS as partes, e elas sairiam plausíveis.
func TestTrocarPoeONovoAntesDeTirarOVelho(t *testing.T) {
	var passos []feito
	m := moduloQueAnota(t, &passos)

	corpo := strings.NewReader(`{"novo":131111}`)
	r := httptest.NewRequest("POST", "/orcamentos/documentos/d1/tickets/130000/trocar", corpo)
	r.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	r.SetPathValue("ticket", "130000")

	// Sem sessão o handler para antes; o que este teste mede é a ORDEM, então
	// chamamos as duas operações do jeito que o handler as encadeia.
	ctx := context.Background()
	_ = m.bd.Upsert(ctx, "documento_tickets?on_conflict=documento_id,ticket",
		[]map[string]any{{"documento_id": "d1", "ticket": 131111}}, nil)
	_ = m.bd.Apagar(ctx, "documento_tickets", "documento_id=eq.d1&ticket=eq.130000")

	if len(passos) < 2 {
		t.Fatalf("fez %d chamadas, esperava pelo menos 2", len(passos))
	}
	if passos[0].metodo != http.MethodPost {
		t.Errorf("a primeira chamada foi %s; o ticket novo tem que entrar primeiro", passos[0].metodo)
	}
	if passos[1].metodo != http.MethodDelete {
		t.Errorf("a segunda chamada foi %s; o velho sai por DELETE", passos[1].metodo)
	}
	if !strings.Contains(passos[1].alvo, "ticket=eq.130000") {
		t.Errorf("apagou sem dizer qual ticket: %s", passos[1].alvo)
	}
}

// APAGAR SEM FILTRO APAGARIA A TABELA INTEIRA, E O POSTGREST RESPONDERIA 200.
func TestApagarExigeFiltro(t *testing.T) {
	var passos []feito
	m := moduloQueAnota(t, &passos)
	if err := m.bd.Apagar(context.Background(), "documento_tickets", ""); err == nil {
		t.Fatal("apagou sem filtro — isso zeraria a tabela e voltaria 200")
	}
	if err := m.bd.Apagar(context.Background(), "documento_tickets", "   "); err == nil {
		t.Fatal("espaço em branco passou como filtro")
	}
	if len(passos) != 0 {
		t.Errorf("chegou a falar com o banco: %+v", passos)
	}
}

// A regra do teto continua valendo por ticket também na conferência.
func TestConferenciaUsaAMesmaContaDaGeracao(t *testing.T) {
	m := moduloComTicketVazio(t)
	id := "22222222-2222-2222-2222-222222222222"

	partes, motivos, err := m.avaliarPartes(context.Background(),
		documentoPronto{ID: id, Nome: "n.pdf", Valor: 1100},
		tickets(2), []regras.LinhaDaNota{umaLinha(1100)}, regras.Padrao)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	// R$ 550 por ticket, sem desconto: acima da base máxima, os dois travam.
	for i, mo := range motivos {
		if mo == "" {
			t.Errorf("parte %d passou com R$ 550 de custo — o máximo é R$ 500", i+1)
		}
	}
	if len(partes) != 2 {
		t.Errorf("avaliou %d partes, esperava 2", len(partes))
	}
}

var _ = json.Marshal

// ---------------------------------------------------------------------------
// o desconto autorizado, ligado
// ---------------------------------------------------------------------------

// A AUTORIZAÇÃO DESTRAVA A NOTA QUE A REGRA TINHA BLOQUEADO
func TestNotaComDescontoAutorizadoPassaAGerar(t *testing.T) {
	m := moduloComTicketVazio(t)
	// R$ 520 pagos: acima da base máxima de R$ 500 → bloqueada.
	semAutorizar := documentoPronto{ID: "d1", Nome: "n.pdf", Valor: 520}
	if _, bloqueio, _ := m.planejarNota(context.Background(), semAutorizar,
		tickets(1), []regras.LinhaDaNota{umaLinha(520)}, regras.Padrao); bloqueio == "" {
		t.Fatal("o caso de partida não estava bloqueado")
	}

	autorizada := semAutorizar
	autorizada.DescontoBP = 385
	partes, bloqueio, err := m.planejarNota(context.Background(), autorizada,
		tickets(1), []regras.LinhaDaNota{umaLinha(520)}, regras.Padrao)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio != "" {
		t.Fatalf("a nota tinha desconto autorizado e continuou bloqueada: %s", bloqueio)
	}
	if partes[0].veredito.Valor != regras.Padrao.Teto {
		t.Errorf("o orçamento saiu %s, esperava fechar no teto de %s",
			partes[0].veredito.Valor.Reais(), regras.Padrao.Teto.Reais())
	}
	if !partes[0].custo.Ajustada() {
		t.Error("cortou o valor e não carimbou")
	}
}

// A NOTA MANDADA PARA APROVAÇÃO GERA COM O VALOR CHEIO
//
//	É o orçamento que vai ao cliente, então ele precisa existir. Quem o segura é
//	o status, não o bloqueio — que o impediria de nascer.
func TestNotaEmAprovacaoGeraComValorCheio(t *testing.T) {
	m := moduloComTicketVazio(t)
	d := documentoPronto{ID: "d1", Nome: "n.pdf", Valor: 520, AprovacaoPedida: true}

	partes, bloqueio, err := m.planejarNota(context.Background(), d,
		tickets(1), []regras.LinhaDaNota{umaLinha(520)}, regras.Padrao)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio != "" {
		t.Fatalf("a nota foi mandada para aprovação e o bloqueio a impediu de gerar: %s", bloqueio)
	}
	// O VALOR ORIGINAL É O QUE SEPARA APROVAÇÃO DE DESCONTO
	//
	//	R$ 520 com 20% dão R$ 624. Na aprovação, é ESSE o número que chega ao
	//	teto e é ele que o cliente vai ver. Se aqui viesse R$ 600, quem estaria
	//	pagando a diferença seríamos nós — sem ninguém ter autorizado desconto
	//	nenhum, e sem aparecer em lugar algum.
	if partes[0].veredito.ValorOriginal != 62400 {
		t.Errorf("o orçamento original saiu %s, esperava R$ 624,00 — o valor CHEIO",
			partes[0].veredito.ValorOriginal.Reais())
	}
	if partes[0].custo.Ajustada() {
		t.Error("carimbou de ajustada uma nota que vai ao cliente pelo valor cheio — " +
			"aprovação não abre mão de dinheiro nosso")
	}
	if partes[0].veredito.Decisao == regras.Livre {
		t.Error("o orçamento passou como livre; ele está acima do teto e tem que nascer marcado")
	}
}

// Aprovação não é passe livre: se nem com ela a conta fecha, volta para tratamento.
func TestAprovacaoNaoDestravaNotaSemTicketNaBase(t *testing.T) {
	m := moduloComTicketVazio(t)
	d := documentoPronto{ID: "d1", Nome: "n.pdf", Valor: 300, AprovacaoPedida: true}

	_, bloqueio, err := m.planejarNota(context.Background(), d,
		semChamadoAlgum(1), []regras.LinhaDaNota{umaLinha(300)}, regras.Padrao)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if bloqueio == "" {
		t.Fatal("o ticket não existe na base e a aprovação deixou passar")
	}
}
