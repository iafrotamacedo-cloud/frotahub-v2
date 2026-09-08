package trilogo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Peças
// ---------------------------------------------------------------------------

// soODetalhe põe um chamado que EXISTE no Trílogo mas não aparece na lista.
//
// É o caso que separa o indício da prova: sumir da lista não é sumir do
// Trílogo, e é por isso que o robô pergunta antes de carimbar.
func (m *mundo) soODetalhe(numero, unidade int) {
	m.chamado(numero, unidade, "", nil, nil)
	m.lista = m.lista[:len(m.lista)-1]
}

// jaTemos monta uma linha do espelho — o que a nossa base já sabe.
func jaTemos(numero int, conta string, saiuEm any) map[string]any {
	return map[string]any{"numero": numero, "conta": conta, "saiu_em": saiuEm}
}

// carimbosDeSaida separa, entre os PATCH que a rodada mandou para `chamados`,
// os que puseram carimbo dos que o tiraram.
func carimbosDeSaida(m *mundo) (puseram, tiraram []string) {
	for _, p := range m.linhas("PATCH:chamados") {
		filtro, _ := p["__filtro"].(string)
		if v, tem := p["saiu_em"]; tem && v == nil {
			tiraram = append(tiraram, filtro)
		} else if tem {
			puseram = append(puseram, filtro)
		}
	}
	return puseram, tiraram
}

func algumContem(fs []string, s string) bool {
	for _, f := range fs {
		if strings.Contains(f, s) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// O defeito que isto conserta
// ---------------------------------------------------------------------------

// O chamado que o cliente tirou da nossa prestadora some da lista das duas
// contas E para de responder. Antes desta mudança ele ficava na nossa tela
// para sempre; agora ganha o carimbo e sai da vista.
func TestChamadoQueSaiuDoTrilogoGanhaOCarimbo(t *testing.T) {
	m := novoMundo(t)
	m.chamado(500, 82, "", nil, nil) // continua na lista
	m.nossaBase = []map[string]any{
		jaTemos(500, "instalacoes", nil),
		jaTemos(777, "instalacoes", nil), // nem na lista, nem no detalhe
	}

	r, err := m.servico().Rodar(context.Background(), ModoAtualizacao, cliente, "teste")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	puseram, _ := carimbosDeSaida(m)
	if !algumContem(puseram, "777") {
		t.Fatalf("o chamado que sumiu não foi carimbado: %v", puseram)
	}
	if algumContem(puseram, "500") {
		t.Errorf("carimbou um chamado que está na lista: %v", puseram)
	}
	if r.ChamadosQueSairam != 1 {
		t.Errorf("a rodada devia contar 1 saída, contou %d", r.ChamadosQueSairam)
	}
}

// O CARIMBO NÃO SAI DA LISTA, SAI DA RESPOSTA DO TRÍLOGO.
//
// Este é o teste que justifica a confirmação um a um. Se um dia a lista voltar
// truncada — um `Offset` que a API passe a ignorar, um status que saia do
// filtro deles —, o chamado some da lista SEM ter saído do Trílogo. Deduzindo
// pela lista, esconderíamos centenas de chamados de uma vez, calados.
func TestChamadoForaDaListaQueAindaRespondeNaoECarimbado(t *testing.T) {
	m := novoMundo(t)
	m.chamado(500, 82, "", nil, nil)
	m.soODetalhe(777, 82) // fora da lista, mas o Trílogo ainda o entrega
	m.nossaBase = []map[string]any{jaTemos(777, "instalacoes", nil)}

	r, err := m.servico().Rodar(context.Background(), ModoAtualizacao, cliente, "teste")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	puseram, _ := carimbosDeSaida(m)
	if len(puseram) > 0 {
		t.Fatalf("carimbou um chamado que o Trílogo ainda entrega: %v", puseram)
	}
	if r.ChamadosQueSairam != 0 {
		t.Errorf("contou %d saídas onde não houve nenhuma", r.ChamadosQueSairam)
	}
}

// Chamado que volta para a nossa prestadora reaparece — sem releitura de nada.
func TestChamadoQueVoltouPerdeOCarimbo(t *testing.T) {
	m := novoMundo(t)
	m.chamado(500, 82, "", nil, nil)
	m.nossaBase = []map[string]any{jaTemos(500, "instalacoes", "2026-09-01T10:00:00Z")}

	r, err := m.servico().Rodar(context.Background(), ModoAtualizacao, cliente, "teste")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	_, tiraram := carimbosDeSaida(m)
	if !algumContem(tiraram, "500") {
		t.Fatalf("o chamado voltou para a lista e o carimbo ficou: %v", tiraram)
	}
	if r.ChamadosQueVoltaram != 1 {
		t.Errorf("a rodada devia contar 1 volta, contou %d", r.ChamadosQueVoltaram)
	}
}

// O chamado que a rodada GRAVA acabou de responder no Trílogo — isso, por si
// só, já é prova de que ele não saiu. Sem esta linha no envio, um chamado que
// volta continuaria escondido até o diff da lista alcançá-lo, e o modo `alvos`
// (que não lê lista nenhuma) nunca o desenterraria.
func TestGravarOChamadoJaLimpaOCarimbo(t *testing.T) {
	m := novoMundo(t)
	m.chamado(500, 82, "", nil, nil)
	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	linhas := m.linhas("chamados")
	if len(linhas) == 0 {
		t.Fatal("nenhum chamado gravado")
	}
	v, tem := linhas[0]["saiu_em"]
	if !tem {
		t.Fatal("o envio de `chamados` não leva `saiu_em` — quem volta fica escondido")
	}
	if v != nil {
		t.Errorf("o envio devia zerar o carimbo, mandou %v", v)
	}
}

// LISTA VAZIA NÃO CARIMBA NINGUÉM.
//
// É o caso em que mais se erra: o Trílogo inteiro sumindo de uma vez (token
// recusado, endereço mudado, manutenção deles) esconderia a base toda. Não se
// decide nada com base em nada.
func TestListaVaziaNaoCarimbaNinguem(t *testing.T) {
	m := novoMundo(t)
	m.nossaBase = []map[string]any{
		jaTemos(500, "instalacoes", nil),
		jaTemos(777, "civil", nil),
	}

	if _, err := m.servico().Rodar(context.Background(), ModoAtualizacao, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	if puseram, _ := carimbosDeSaida(m); len(puseram) > 0 {
		t.Fatalf("uma lista vazia carimbou a base inteira: %v", puseram)
	}
}

// ---------------------------------------------------------------------------
// As três respostas
// ---------------------------------------------------------------------------

// trilogoQueResponde monta uma conta que responde SEMPRE o mesmo — código e
// corpo ditados pelo teste.
func trilogoQueResponde(t *testing.T, codigo int, corpo string) *Sessao {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(codigo)
		w.Write([]byte(corpo))
	}))
	t.Cleanup(srv.Close)
	anterior := base
	base = srv.URL
	t.Cleanup(func() { base = anterior })
	return &Sessao{Conta: "instalacoes", token: "faz-de-conta", http: srv.Client()}
}

// ESTE É O TESTE QUE A PRIMEIRA VERSÃO NÃO TINHA, E QUE TERIA PEGO O DEFEITO.
//
// O chamado que a conta não alcança volta 400, e não 200 vazio — medido no
// Trílogo de verdade em 08/09/2026 (ticket 132242, nas duas contas). Tratar
// esse 400 como "não sei" fazia o robô rodar limpo, sem erro nenhum, e não
// carimbar UM chamado. As duas frases que eles mandam entram aqui como estão,
// para o dia em que alguém trocar o texto: quem decide é o 400.
func TestARecusaDoTrilogoEUmSumico(t *testing.T) {
	for _, frase := range []string{
		`{"message":"Você não possui permissão para acessar este ticket"}`,
		`{"message":"Ticket não encontrado"}`,
	} {
		sessao := trilogoQueResponde(t, 400, frase)
		if r := aindaDeAlguem(context.Background(), []*Sessao{sessao}, 132242); r != naoEDessaConta {
			t.Errorf("400 %s não virou sumiço: %v", frase, r)
		}
	}
}

// Erro do Trílogo não é resposta sobre o chamado. Confundir os dois carimbaria
// no escuro a cada tosse da rede deles — ou, pior, a cada token vencido, que
// derrubaria a base inteira de uma vez.
func TestTrilogoQueNaoRespondeNaoViraSumico(t *testing.T) {
	for _, codigo := range []int{401, 403, 500, 502} {
		sessao := trilogoQueResponde(t, codigo, `{"message":"não foi dessa vez"}`)
		if r := aindaDeAlguem(context.Background(), []*Sessao{sessao}, 121413); r != naoSei {
			t.Errorf("HTTP %d virou veredicto sobre o chamado: %v", codigo, r)
		}
	}
}

// Uma conta recusando e outra tossindo dá "não sei" — não se carimba sem ter
// ouvido todas.
//
// `base` é global, então as duas sessões falam com o MESMO servidor; quem
// separa as contas aqui é o token, como no Trílogo de verdade.
func TestUmaContaMudaSeguraOCarimbo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Authorization"), "mudo") {
			w.WriteHeader(500)
			w.Write([]byte(`{"message":"deu ruim aqui"}`))
			return
		}
		w.WriteHeader(400)
		w.Write([]byte(`{"message":"Você não possui permissão para acessar este ticket"}`))
	}))
	t.Cleanup(srv.Close)
	anterior := base
	base = srv.URL
	t.Cleanup(func() { base = anterior })

	recusa := &Sessao{Conta: "instalacoes", token: "recusa", http: srv.Client()}
	mudo := &Sessao{Conta: "civil", token: "mudo", http: srv.Client()}

	if r := aindaDeAlguem(context.Background(), []*Sessao{recusa, mudo}, 132242); r != naoSei {
		t.Fatalf("uma conta sem resposta e mesmo assim carimbou: %v", r)
	}
	// E só com as duas recusando é que se carimba.
	outra := &Sessao{Conta: "civil", token: "recusa-tambem", http: srv.Client()}
	if r := aindaDeAlguem(context.Background(), []*Sessao{recusa, outra}, 132242); r != naoEDessaConta {
		t.Fatalf("as duas contas recusaram e não deu sumiço: %v", r)
	}
}

// Basta UMA conta reconhecer o chamado.
func TestChamadoDaOutraContaContinuaSendoNosso(t *testing.T) {
	instalacoes := trilogoDeMentira(t, map[int]bool{})
	civil := &Sessao{Conta: "civil", token: "faz-de-conta", http: instalacoes.http}

	// O de mentira acima não conhece ninguém; nenhuma conta reconhece.
	if r := aindaDeAlguem(context.Background(), []*Sessao{instalacoes, civil}, 121413); r != naoEDessaConta {
		t.Fatalf("ninguém reconheceu e mesmo assim não deu sumiço: %v", r)
	}

	// Agora o Trílogo passa a reconhecer: uma resposta basta.
	conhecido := trilogoDeMentira(t, map[int]bool{121413: true})
	if r := aindaDeAlguem(context.Background(), []*Sessao{conhecido}, 121413); r != aindaENossa {
		t.Fatalf("o chamado respondeu e mesmo assim foi dado como fora: %v", r)
	}
}

// A conta que a nossa base atribui ao chamado é a primeira a ser perguntada —
// no caso comum resolve na primeira pergunta, e a outra nem é incomodada.
func TestAContaDoChamadoEPerguntadaPrimeiro(t *testing.T) {
	a := &Sessao{Conta: "instalacoes"}
	b := &Sessao{Conta: "civil"}

	ordem := ordemDeConsulta([]*Sessao{a, b}, "civil")
	if len(ordem) != 2 || ordem[0] != b || ordem[1] != a {
		t.Fatalf("a conta do chamado não foi para a frente: %v", ordem)
	}
	// Conta desconhecida não pode fazer sumir ninguém da fila de perguntas.
	if ordem := ordemDeConsulta([]*Sessao{a, b}, "sei-la"); len(ordem) != 2 {
		t.Fatalf("uma conta que não existe encolheu a lista de perguntas: %v", ordem)
	}
}

// ---------------------------------------------------------------------------
// A lista, na tela
// ---------------------------------------------------------------------------

func filtroDe(t *testing.T, q map[string][]string) string {
	t.Helper()
	f, err := (&Consulta{}).montarFiltro("cli-1", q)
	if err != nil {
		t.Fatalf("filtro recusado: %v", err)
	}
	return f
}

func TestAListaNaoMostraQuemSaiuDoTrilogo(t *testing.T) {
	f := filtroDe(t, map[string][]string{})
	if !strings.Contains(f, "saiu_em=is.null") {
		t.Fatalf("a lista continua mostrando quem saiu: %q", f)
	}
}

// Quem digita o número tem o número na mão. Responder "não achei" para um
// ticket que está na base, inteiro, seria mentir — e mandaria a pessoa
// procurar em outro lugar o que está bem aqui.
func TestBuscaPorNumeroAchaAteQuemSaiu(t *testing.T) {
	f := filtroDe(t, map[string][]string{"ticket": {"130328"}})
	if strings.Contains(f, "saiu_em") {
		t.Fatalf("a busca por ticket herdou o filtro de saída: %q", f)
	}
	if !strings.Contains(f, "numero=eq.130328") {
		t.Fatalf("perdeu a busca por número: %q", f)
	}
}

func TestVistaDosQueSairam(t *testing.T) {
	f := filtroDe(t, map[string][]string{"saidos": {"sim"}})
	if !strings.Contains(f, "saiu_em=not.is.null") {
		t.Fatalf("a vista dos que saíram não filtra por eles: %q", f)
	}

	todos := filtroDe(t, map[string][]string{"saidos": {"todos"}})
	if strings.Contains(todos, "saiu_em") {
		t.Fatalf("`todos` devia trazer os dois juntos: %q", todos)
	}
}

// O cliente entra em TODO filtro, sempre — inclusive nos novos.
func TestOFiltroDeSaidaNaoDispensaOCliente(t *testing.T) {
	for _, q := range []map[string][]string{
		{}, {"saidos": {"sim"}}, {"saidos": {"todos"}}, {"ticket": {"1"}},
	} {
		if f := filtroDe(t, q); !strings.HasPrefix(f, "cliente_id=eq.cli-1") {
			t.Errorf("filtro sem cliente na frente: %q", f)
		}
	}
}
