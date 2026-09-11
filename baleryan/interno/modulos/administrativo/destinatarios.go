// rev 1 — PCO: os destinatários do e-mail (migração 061)
//
// ROTINA PRÓPRIA, SEPARADA DE "ENVIAR" (ver o cabeçalho de `pco_enviar.go`)
//
//	Quem edita a lista fixa de e-mails do cliente e quem aperta o botão de
//	enviar podem ser pessoas diferentes — pedido explícito do dono. Por isso
//	`RotinaPCODestinatarios` é o único filtro destas rotas, nunca
//	`RotinaPCOEnviar`.
//
// NUNCA APAGA — SÓ ATIVA/DESATIVA (CORE-05)
//
//	Um destinatário tirado de circulação continua na tabela, com `ativo =
//	false`. É o que faz "quem recebia isso em março" ser uma pergunta
//	respondível, e não um registro que sumiu sem deixar rastro.
package administrativo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

const moduloHistoricoDestinatarios = "pco_destinatarios"

func (m *Modulo) quemPodeDestinatarios(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, RotinaPCODestinatarios); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// destinatariosAtivos é o que `pco_enviar.go` usa de verdade — só os e-mails,
// só os ativos. Sem checar rotina: o motor já é quem decide enviar; esta
// função só lê o que ele precisa para montar o "Para".
func (m *Modulo) destinatariosAtivos(ctx context.Context, clienteID string) ([]string, error) {
	var linhas []struct {
		Email string `json:"email"`
	}
	caminho := "pco_destinatarios?cliente_id=eq." + banco.Escapar(clienteID) +
		"&ativo=eq.true&select=email&order=email"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	email := make([]string, 0, len(linhas))
	for _, l := range linhas {
		email = append(email, l.Email)
	}
	return email, nil
}

// ---------------------------------------------------------------------------
// GET /administrativo/compras/pco/destinatarios
// ---------------------------------------------------------------------------

func (m *Modulo) listarDestinatarios(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeDestinatarios(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := "pco_destinatarios?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=id,email,ativo,criado_em&order=email"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar os destinatários", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"destinatarios": ouVazio(linhas)})
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/pco/destinatarios
// ---------------------------------------------------------------------------

func (m *Modulo) criarDestinatario(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeDestinatarios(w, r)
	if p == nil {
		return
	}
	var pedido struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o pedido.")
		return
	}
	email := strings.TrimSpace(strings.ToLower(pedido.Email))
	if !pareceEmail(email) {
		web.Falhar(w, http.StatusBadRequest, "Isto não parece um e-mail válido.")
		return
	}

	var criados []map[string]any
	if err := m.bd.Inserir(r.Context(), "pco_destinatarios", []map[string]any{{
		"cliente_id": p.ClienteID,
		"email":      email,
		"criado_por": p.UserID,
	}}, &criados); err != nil {
		if banco.Duplicado(err) {
			web.Falhar(w, http.StatusConflict, "Este e-mail já está cadastrado.")
			return
		}
		m.erro(w, "não consegui cadastrar o destinatário", err)
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "Cadastrei mas o banco não devolveu o registro.")
		return
	}
	id := fmt.Sprint(criados[0]["id"])
	if err := m.hist.Registrar(r.Context(), p, moduloHistoricoDestinatarios, id, "cadastrou", map[string]historico.Mudanca{
		"email": {De: nil, Para: email},
	}); err != nil {
		web.Responder(w, http.StatusOK, map[string]any{"destinatario": criados[0], "aviso": historico.Aviso})
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"destinatario": criados[0]})
}

// ---------------------------------------------------------------------------
// PATCH /administrativo/compras/pco/destinatarios/{id} — só liga/desliga
// ---------------------------------------------------------------------------

func (m *Modulo) alterarDestinatario(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeDestinatarios(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var pedido struct {
		Ativo *bool `json:"ativo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil || pedido.Ativo == nil {
		web.Falhar(w, http.StatusBadRequest, "Diga o novo estado (\"ativo\").")
		return
	}

	atual, err := m.contarUm(r.Context(), "pco_destinatarios?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=ativo&limit=1")
	if err != nil {
		m.erro(w, "não achei este destinatário", err)
		return
	}
	antes, _ := atual["ativo"].(bool)
	if antes == *pedido.Ativo {
		web.Responder(w, http.StatusOK, map[string]any{"ok": true}) // nada mudou — não gera rastro (P-02)
		return
	}
	if err := m.bd.Atualizar(r.Context(), "pco_destinatarios", "id=eq."+id,
		map[string]any{"ativo": *pedido.Ativo}); err != nil {
		m.erro(w, "não consegui alterar o destinatário", err)
		return
	}
	acao := "desativou"
	if *pedido.Ativo {
		acao = "ativou"
	}
	if err := m.hist.Registrar(r.Context(), p, moduloHistoricoDestinatarios, id, acao, map[string]historico.Mudanca{
		"ativo": {De: antes, Para: *pedido.Ativo},
	}); err != nil {
		web.Responder(w, http.StatusOK, map[string]any{"ok": true, "aviso": historico.Aviso})
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// GET /administrativo/compras/pco/destinatarios/{id}/historico
// ---------------------------------------------------------------------------

func (m *Modulo) historicoDestinatario(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeDestinatarios(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	// A lista é curta (cadastros e trocas de estado, poucos por ano) — 100
	// linhas nunca vão faltar, então não precisa de paginação de verdade.
	linhas, err := m.hist.Listar(r.Context(), p.ClienteID, moduloHistoricoDestinatarios, id, 100, 0)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar o histórico.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"historico": linhas})
}

// pareceEmail é uma conferência rasa de propósito — só "tem @, tem algo dos
// dois lados, sem espaço". Quem confirma que o e-mail EXISTE é a primeira
// mensagem que chegar (ou não) nele; validação forte demais aqui só
// rejeitaria endereço válido incomum.
func pareceEmail(s string) bool {
	arroba := strings.IndexByte(s, '@')
	if arroba <= 0 || arroba == len(s)-1 {
		return false
	}
	if strings.ContainsAny(s, " \t\n") {
		return false
	}
	return strings.Contains(s[arroba+1:], ".")
}
