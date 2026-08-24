package trilogo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// Um banco de mentira que só sabe de rodadas — e que, como o de verdade, exige
// que a consulta diga de qual cliente ela é.
func moduloComRodadas(t *testing.T, rodando, concluida *Rodada) *Modulo {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "cliente_id=eq.") {
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"consulta sem cliente"}`))
			return
		}
		saida := []Rodada{}
		switch {
		case strings.Contains(r.URL.RawQuery, "situacao=eq.rodando") && rodando != nil:
			saida = append(saida, *rodando)
		case strings.Contains(r.URL.RawQuery, "situacao=eq.concluida") && concluida != nil:
			saida = append(saida, *concluida)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(saida)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Supabase: config.Supabase{URL: srv.URL, ChaveServico: "servico"}}
	return NovoModulo(nil, nil, nil, banco.Novo(cfg))
}

func hoje(atras time.Duration) string {
	return time.Now().UTC().Add(-atras).Format(time.RFC3339Nano)
}

// O modo decide quem aperta. Esta é a regra inteira, num lugar só.
func TestSoAAtualizacaoEDeBotao(t *testing.T) {
	if !DeBotao(ModoAtualizacao) {
		t.Error("a atualização é o modo do botão e ficou de fora")
	}
	for _, pesado := range []string{ModoLevantamento, ModoCopia, "qualquer-outro"} {
		if DeBotao(pesado) {
			t.Errorf("%q não pode ser disparado por quem só enxerga a tela", pesado)
		}
	}
}

func TestLeituraEmAndamentoSeguraOBotao(t *testing.T) {
	m := moduloComRodadas(t, &Rodada{ID: "r1", Modo: ModoAtualizacao, Situacao: "rodando",
		ComecouEm: hoje(2 * time.Minute)}, nil)

	travada, motivo, err := m.travada(context.Background(), "cli-1", ModoAtualizacao)
	if err != nil {
		t.Fatal(err)
	}
	if travada == nil {
		t.Fatal("deixou passar uma segunda leitura com uma já rodando")
	}
	if travada.ID != "r1" {
		t.Errorf("a rodada em curso não voltou junto: %+v", travada)
	}
	if !strings.Contains(motivo, "em andamento") || !strings.Contains(motivo, "2 minutos") {
		t.Errorf("a mensagem não diz o que está acontecendo: %q", motivo)
	}
}

// Rodada pendurada não pode trancar o botão para sempre. Se o processo morreu
// antes de fechar a linha, ela fica "rodando" no banco até alguém arrumar — e
// nunca ninguém arruma.
func TestRodadaAbandonadaDeixaDeSegurar(t *testing.T) {
	m := moduloComRodadas(t, &Rodada{ID: "r-velha", Situacao: "rodando",
		ComecouEm: hoje(45 * time.Minute)}, nil)

	travada, _, err := m.travada(context.Background(), "cli-1", ModoAtualizacao)
	if err != nil {
		t.Fatal(err)
	}
	if travada != nil {
		t.Fatalf("uma rodada de 45 minutos atrás continuou travando o botão: %+v", travada)
	}
}

func TestDescansoEntreDuasAtualizacoes(t *testing.T) {
	m := moduloComRodadas(t, nil, &Rodada{ID: "r0", Modo: ModoAtualizacao,
		Situacao: "concluida", TerminouEm: hoje(30 * time.Second)})

	travada, motivo, err := m.travada(context.Background(), "cli-1", ModoAtualizacao)
	if err != nil {
		t.Fatal(err)
	}
	if travada == nil {
		t.Fatal("duas leituras em trinta segundos passaram")
	}
	if !strings.Contains(motivo, "já foi atualizada") {
		t.Errorf("a mensagem não explica: %q", motivo)
	}
}

// O descanso protege o Trílogo de clique nervoso. O builder que pede um
// levantamento sabe o que está fazendo, e não leva sermão.
func TestODescansoNaoValeParaOsModosPesados(t *testing.T) {
	m := moduloComRodadas(t, nil, &Rodada{ID: "r0", Modo: ModoAtualizacao,
		Situacao: "concluida", TerminouEm: hoje(10 * time.Second)})

	for _, modo := range []string{ModoLevantamento, ModoCopia} {
		travada, _, err := m.travada(context.Background(), "cli-1", modo)
		if err != nil {
			t.Fatal(err)
		}
		if travada != nil {
			t.Errorf("o modo %s foi barrado pelo descanso do botão", modo)
		}
	}
}

func TestCaminhoLivre(t *testing.T) {
	m := moduloComRodadas(t, nil, &Rodada{ID: "r0", Modo: ModoAtualizacao,
		Situacao: "concluida", TerminouEm: hoje(2 * time.Hour)})

	travada, motivo, err := m.travada(context.Background(), "cli-1", ModoAtualizacao)
	if err != nil {
		t.Fatal(err)
	}
	if travada != nil {
		t.Fatalf("travou sem motivo: %s", motivo)
	}
}

// Se o horário do banco não for entendido, a trava some sem avisar — e o botão
// volta a disparar leituras em paralelo. Os formatos que o PostgREST usa ficam
// travados aqui.
func TestHorarioDoBancoEEntendido(t *testing.T) {
	for _, bom := range []string{
		"2026-08-24T01:06:31.764959+00:00",
		"2026-08-24T01:06:31Z",
		"2026-08-24T01:06:31.764959-03",
		"2026-08-24 01:06:31.764959+00",
		"2026-08-24 01:06:31-03",
	} {
		if _, ok := instante(bom); !ok {
			t.Errorf("não entendeu o horário %q", bom)
		}
	}
	for _, ruim := range []string{"", "   ", "ontem", "24/08/2026"} {
		if _, ok := instante(ruim); ok {
			t.Errorf("aceitou %q como horário", ruim)
		}
	}
}

func TestDuracaoEscritaComoGenteFala(t *testing.T) {
	for d, quer := range map[time.Duration]string{
		2 * time.Second:  "agora mesmo",
		30 * time.Second: "há 30 segundos",
		90 * time.Second: "há um minuto",
		7 * time.Minute:  "há 7 minutos",
	} {
		if veio := haQuanto(d); veio != quer {
			t.Errorf("haQuanto(%s) = %q, esperava %q", d, veio, quer)
		}
	}
}
