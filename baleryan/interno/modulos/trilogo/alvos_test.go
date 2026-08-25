package trilogo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// trilogoDeMentira responde como o de verdade: o chamado que é desta conta vem
// inteiro; o que não é vem VAZIO, com 200 — que é justamente a resposta que
// engana.
func trilogoDeMentira(t *testing.T, meus map[int]bool) *Sessao {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			id = r.URL.Query().Get("ticketId")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "GetTicketDetail"):
			if meus[atoi(id)] {
				w.Write([]byte(`{"id":` + id + `,"description":"consertar a luz",
					"serviceCompany":{"name":"FROTA - INSTALAÇÕES"},"company":{"id":9,"name":"LOJA 09"}}`))
				return
			}
			// Corpo vazio com 200: o Trílogo faz isso, e é o caso que importa.
			w.Write([]byte(``))
		case strings.Contains(r.URL.Path, "GetTicketCosts"):
			w.Write([]byte(`[]`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)

	anterior := base
	base = srv.URL
	t.Cleanup(func() { base = anterior })

	return &Sessao{Conta: "instalacoes", token: "faz-de-conta", http: srv.Client()}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func servicoDeTeste() *Servico {
	return Novo(&config.Config{Trilogo: config.Trilogo{Paralelo: 4}}, nil, nil)
}

// ESTE É O TESTE QUE JUSTIFICA A CONFERÊNCIA DO `d.ID`.
//
// Sem ela, o chamado da outra conta volta como um Detalhe zerado SEM ERRO, e o
// caminho de gravação escreve um chamado de número 0. Como o modo `alvos`
// pergunta às duas contas de propósito, isso aconteceria em metade das
// perguntas — e a primeira gravação criaria a linha número 0 no banco.
func TestChamadoDeOutraContaNaoViraChamadoZero(t *testing.T) {
	sessao := trilogoDeMentira(t, map[int]bool{121413: true})
	svc := servicoDeTeste()

	colhidos, faltaram, err := svc.colher(context.Background(), sessao, []int{121413, 67295}, true)
	if err != nil {
		t.Fatalf("no modo tolerante, chamado de outra conta não é erro: %v", err)
	}
	if colhidos[0] == nil || colhidos[0].detalhe.ID != 121413 {
		t.Fatalf("o chamado desta conta tinha que vir inteiro, veio %v", colhidos[0])
	}
	if colhidos[1] != nil {
		t.Fatalf("o chamado da outra conta virou %#v — devia ter ficado de fora",
			colhidos[1].detalhe)
	}
	if len(faltaram) != 1 || faltaram[0] != 67295 {
		t.Fatalf("os ausentes deviam ser [67295], vieram %v", faltaram)
	}
}

// No caminho normal os números vêm da lista da própria conta. Ali, não responder
// é falha de verdade, e a rodada tem que parar em vez de gravar pela metade.
func TestForaDoModoAlvosChamadoMudoDerrubaARodada(t *testing.T) {
	sessao := trilogoDeMentira(t, map[int]bool{121413: true})
	svc := servicoDeTeste()

	_, _, err := svc.colher(context.Background(), sessao, []int{121413, 67295}, false)
	if err == nil {
		t.Fatal("chamado mudo passou batido; no caminho normal isso é falha")
	}
	if !strings.Contains(err.Error(), "67295") {
		t.Errorf("o erro tem que dizer QUAL chamado: %v", err)
	}
}

// O modo alvos sem lista não roda. Recusar é melhor que rodar sem alvo nenhum e
// devolver "concluída, 0 chamados" — que parece sucesso e não é.
func TestAlvosSemListaRecusa(t *testing.T) {
	svc := servicoDeTeste()
	r := &Resultado{}
	err := svc.lerAlvos(context.Background(), "um-cliente", r)
	if err == nil {
		t.Fatal("rodou sem lista de alvos")
	}
	if !strings.Contains(err.Error(), "TRILOGO_ALVOS") {
		t.Errorf("o erro tem que dizer o que falta preencher: %v", err)
	}
	if r.ChamadosGravados != 0 {
		t.Errorf("não podia ter gravado nada, gravou %d", r.ChamadosGravados)
	}
}

// O modo tem que ser conhecido pelo robô E pelo CHECK da tabela (migração 012).
// Este teste guarda a metade que mora no código; a outra metade foi provada num
// Postgres de verdade antes de a migração sair da mão.
func TestModosConhecidos(t *testing.T) {
	for _, m := range []string{ModoLevantamento, ModoCopia, ModoAtualizacao, ModoAlvos} {
		if !modoConhecido(m) {
			t.Errorf("%q é modo de verdade e o robô não reconhece", m)
		}
	}
	if modoConhecido("carga-maior") {
		t.Error("modo inventado passou; o CHECK da tabela recusaria e a rodada morreria na primeira linha")
	}
}

// alvos é passada de dono, pelo Actions — nunca de botão na tela.
func TestAlvosNaoEDeBotao(t *testing.T) {
	if DeBotao(ModoAlvos) {
		t.Error("o modo alvos escreve no banco a partir de uma lista digitada; não é botão de tela")
	}
}
