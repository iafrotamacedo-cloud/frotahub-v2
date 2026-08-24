// rev 2 — as rotas do robô no motor
//
// POR QUE A CHAMADA É SÍNCRONA E LIMITADA A UM LOTE
//
//	O caminho fácil seria disparar o trabalho numa rotina de fundo e responder
//	"comecei". Seria mentira num servidor que adormece: a rotina morreria com o
//	processo, e ninguém ficaria sabendo.
//
//	Aqui cada chamada faz UM LOTE e responde o que fez, com um `completo` dizendo
//	se sobrou trabalho. Quem chamou repete até acabar. O andamento mora no banco,
//	não na memória (P-03) — se o servidor cair no meio, o que foi gravado está
//	gravado e a próxima chamada continua de onde parou.
//
// QUEM PODE — E ISSO DEPENDE DO MODO
//
//	`atualizacao` é a leitura do dia a dia: lê só o que mudou, não apaga nada, e
//	repetir é inofensivo. Quem já enxerga a tela de Dados do Trílogo pode apertar
//	o botão. Foi decisão do dono, e é a que faz sentido para quem opera (P-29):
//	quem percebe que um chamado novo ainda não apareceu é justamente quem está
//	olhando a lista.
//
//	`levantamento` e `copia` são outra coisa. O primeiro relê a base inteira
//	desde a data de corte; o segundo baixa arquivo e escreve no armazém. Nenhum
//	dos dois é coisa para acontecer num clique distraído: continuam do builder.
//
//	O robô do agendador entra pela chave compartilhada, em qualquer modo.
//
// DUAS RODADAS AO MESMO TEMPO, NÃO
//
//	Com o botão liberado para todo mundo, três pessoas apertando no mesmo minuto
//	viram três leituras simultâneas do Trílogo. A trava está no BANCO, e não na
//	memória do motor, de propósito: assim ela também vale entre canais
//	diferentes — a rodada agendada roda no GitHub Actions, num processo que o
//	motor nunca vê, e mesmo assim o botão a enxerga e espera.
package trilogo

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// JanelaDaRodada — acima disto, uma rodada ainda "rodando" está travada, não
// trabalhando: o processo morreu antes de fechar a linha. Deixar de bloquear
// depois desse tempo evita que um tombo do Actions tranque o botão para sempre.
const JanelaDaRodada = 30 * time.Minute

// DescansoEntreLeituras — piso entre duas atualizações seguidas. Não protege o
// nosso banco, protege o Trílogo: sem isso, uma lista aberta em cinco máquinas
// vira cinco leituras completas da API deles a cada clique nervoso.
const DescansoEntreLeituras = 3 * time.Minute

type Modulo struct {
	svc  *Servico
	seg  *seguranca.Servico
	perm *permissao.Servico
	bd   *banco.Cliente
}

func NovoModulo(svc *Servico, seg *seguranca.Servico, perm *permissao.Servico, bd *banco.Cliente) *Modulo {
	return &Modulo{svc: svc, seg: seg, perm: perm, bd: bd}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("POST /robos/trilogo/{modo}", m.rodar)
	mux.HandleFunc("GET /robos/trilogo/rodadas", m.rodadas)
}

// DeBotao diz se este modo pode ser disparado por quem apenas enxerga a tela.
// É uma função, e não uma lista espalhada pelos `if`, para existir num lugar só.
func DeBotao(modo string) bool { return modo == ModoAtualizacao }

// quemPode aceita o robô com a chave certa; e, entre pessoas, cobra o que o modo
// exigir. `modo` vazio significa apenas LER (a lista de rodadas).
func (m *Modulo) quemPode(w http.ResponseWriter, r *http.Request, modo string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if p.Tipo == seguranca.TipoRobo {
		return p
	}

	// Ler as rodadas e disparar a atualização pedem a mesma coisa: enxergar a
	// tela. Os modos pesados pedem o builder.
	if modo == "" || DeBotao(modo) {
		if err := m.perm.Exige(r.Context(), p, RotinaDados); err != nil {
			web.Falhar(w, permissao.StatusDoErro(err), err.Error())
			return nil
		}
		return p
	}
	if err := permissao.ExigeBuilder(p); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err),
			"Esta rodada relê a base inteira e só o builder pode dispará-la.")
		return nil
	}
	return p
}

// cliente descobre de quem é o trabalho.
//
// Pessoa traz o cliente no próprio perfil. Robô não tem perfil: ele diz pelo
// endereço (?cliente=frota-macedo) e, havendo um cliente só, nem precisa dizer.
func (m *Modulo) cliente(r *http.Request, p *seguranca.Principal) (string, error) {
	if p.ClienteID != "" {
		return p.ClienteID, nil
	}
	slug := strings.TrimSpace(r.URL.Query().Get("cliente"))
	var linhas []struct {
		ID string `json:"id"`
	}
	caminho := "clientes?select=id&limit=2"
	if slug != "" {
		caminho = "clientes?slug=eq." + banco.Escapar(slug) + "&select=id&limit=2"
	}
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		return "", fmt.Errorf("não consegui descobrir o cliente")
	}
	switch len(linhas) {
	case 0:
		return "", fmt.Errorf("cliente não encontrado")
	case 1:
		return linhas[0].ID, nil
	default:
		return "", fmt.Errorf("existe mais de um cliente: diga qual em ?cliente=")
	}
}

// POST /robos/trilogo/{modo}   modo = levantamento | copia | atualizacao
func (m *Modulo) rodar(w http.ResponseWriter, r *http.Request) {
	modo := r.PathValue("modo")
	p := m.quemPode(w, r, modo)
	if p == nil {
		return
	}
	clienteID, err := m.cliente(r, p)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}

	// A trava vale para gente. O robô do agendador já é único pelo `concurrency`
	// do próprio workflow, e barrá-lo aqui faria uma rodada agendada desistir por
	// causa de um clique que aconteceu um minuto antes.
	if p.Tipo != seguranca.TipoRobo {
		if travada, motivo, err := m.travada(r.Context(), clienteID, modo); err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir se já existe leitura em andamento.")
			return
		} else if travada != nil {
			// 409, e não 500: não é defeito, é "já tem gente fazendo isso". A
			// rodada em curso vai junto para a tela poder mostrar o andamento
			// dela em vez de fingir que nada acontece (P-29).
			web.Responder(w, http.StatusConflict, map[string]any{
				"erro": motivo, "rodada": travada,
			})
			return
		}
	}

	quem := p.Usuario
	if p.Tipo == seguranca.TipoRobo {
		quem = "agendador"
	}

	res, err := m.svc.Rodar(r.Context(), modo, clienteID, quem)
	if err != nil {
		// A rodada pode ter falhado no meio DEPOIS de gravar coisa. Por isso vai
		// o resultado junto do erro: dizer só "falhou" esconderia o que já entrou.
		web.Responder(w, http.StatusInternalServerError, map[string]any{
			"erro": err.Error(), "rodada": res,
		})
		return
	}
	web.Responder(w, http.StatusOK, res)
}

// Rodada é o pouco que a tela precisa saber de uma execução para explicar por que
// o botão não fez nada agora.
type Rodada struct {
	ID         string `json:"id"`
	Modo       string `json:"modo"`
	Situacao   string `json:"situacao"`
	ComecouEm  string `json:"comecou_em"`
	TerminouEm string `json:"terminou_em"`
}

// travada responde se este pedido deve esperar, e por quê.
//
// Duas perguntas, nesta ordem:
//
//  1. tem alguma rodada em andamento? (vale para qualquer modo)
//  2. a última atualização terminou agora há pouco? (só para o modo de botão —
//     o builder que pede um levantamento sabe o que está fazendo)
func (m *Modulo) travada(ctx context.Context, clienteID, modo string) (*Rodada, string, error) {
	base := "robo_execucoes?cliente_id=eq." + banco.Escapar(clienteID) + "&robo=eq.trilogo"
	campos := "&select=id,modo,situacao,comecou_em,terminou_em"

	var rodando []Rodada
	if err := m.bd.Buscar(ctx, base+"&situacao=eq.rodando"+campos+"&order=comecou_em.desc&limit=1", &rodando); err != nil {
		return nil, "", err
	}
	if len(rodando) > 0 {
		if inicio, ok := instante(rodando[0].ComecouEm); ok {
			if desde := time.Since(inicio); desde < JanelaDaRodada {
				return &rodando[0], "Já tem uma leitura em andamento, começada " + haQuanto(desde) + ".", nil
			}
			// Passou da janela: a rodada não terminou nem vai terminar. Deixa
			// passar em vez de travar o botão para sempre.
		}
	}

	if !DeBotao(modo) {
		return nil, "", nil
	}

	var ultima []Rodada
	if err := m.bd.Buscar(ctx, base+"&modo=eq."+ModoAtualizacao+"&situacao=eq.concluida"+campos+
		"&order=terminou_em.desc&limit=1", &ultima); err != nil {
		return nil, "", err
	}
	if len(ultima) > 0 {
		if fim, ok := instante(ultima[0].TerminouEm); ok {
			if desde := time.Since(fim); desde < DescansoEntreLeituras {
				return &ultima[0], "A lista já foi atualizada " + haQuanto(desde) + ". Espere um instante antes de pedir de novo.", nil
			}
		}
	}
	return nil, "", nil
}

// GET /robos/trilogo/rodadas — as últimas, para a tela e para conferência.
func (m *Modulo) rodadas(w http.ResponseWriter, r *http.Request) {
	p := m.quemPode(w, r, "")
	if p == nil {
		return
	}
	clienteID, err := m.cliente(r, p)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}
	linhas := []map[string]any{}
	caminho := "robo_execucoes?cliente_id=eq." + banco.Escapar(clienteID) +
		"&robo=eq.trilogo&select=*&order=comecou_em.desc&limit=20"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as rodadas.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"rodadas": linhas})
}

// instante lê um horário do banco. O PostgREST manda ISO com T, mas já apareceu
// com espaço no meio dependendo de quem escreveu — aceita os dois em vez de
// devolver zero e fazer a trava sumir sem avisar.
func instante(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, f := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999-07",
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05-07",
	} {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// haQuanto escreve a duração como gente fala.
func haQuanto(d time.Duration) string {
	switch s := int(d.Seconds()); {
	case s < 5:
		return "agora mesmo"
	case s < 60:
		return fmt.Sprintf("há %d segundos", s)
	case s < 120:
		return "há um minuto"
	default:
		return fmt.Sprintf("há %d minutos", s/60)
	}
}
