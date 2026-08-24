// rev 1 — as rotas do robô no motor
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
// QUEM PODE
//
//	O builder, pela tela; e os robôs do agendador, pela chave compartilhada. Duas
//	portas, o mesmo caminho (CORE-06).
package trilogo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

type Modulo struct {
	svc *Servico
	seg *seguranca.Servico
	bd  *banco.Cliente
}

func NovoModulo(svc *Servico, seg *seguranca.Servico, bd *banco.Cliente) *Modulo {
	return &Modulo{svc: svc, seg: seg, bd: bd}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("POST /robos/trilogo/{modo}", m.rodar)
	mux.HandleFunc("GET /robos/trilogo/rodadas", m.rodadas)
}

// quemPode aceita o builder OU um robô com a chave certa.
func (m *Modulo) quemPode(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if p.Tipo == seguranca.TipoRobo {
		return p
	}
	if err := permissao.ExigeBuilder(p); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
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
	p := m.quemPode(w, r)
	if p == nil {
		return
	}
	clienteID, err := m.cliente(r, p)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}

	quem := p.Usuario
	if p.Tipo == seguranca.TipoRobo {
		quem = "agendador"
	}

	res, err := m.svc.Rodar(r.Context(), r.PathValue("modo"), clienteID, quem)
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

// GET /robos/trilogo/rodadas — as últimas, para a tela e para conferência.
func (m *Modulo) rodadas(w http.ResponseWriter, r *http.Request) {
	p := m.quemPode(w, r)
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
