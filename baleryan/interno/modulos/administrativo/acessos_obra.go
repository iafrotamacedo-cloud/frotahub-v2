// rev 1 — quem pode receber nota fiscal de qual obra (12/09/2026)
//
// "deve haver uma configuração onde auths de nível superior decidem quais
// obras ele pode ver/editar" — o dono, sobre o almoxarife. `centro_custo_
// acessos` (migração 064) é o vínculo perfil↔obra; este arquivo é a tela de
// quem concede. Só quem tem `COMPRAS_NF_CONFIGURAR_ACESSO` (hoje só CEO, e
// o builder sempre) mexe aqui — ver `temAcessoAObra` em `notas_fiscais.go`
// para quem CONSOME esta concessão.
package administrativo

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

func (m *Modulo) quemPodeConfigurarAcessoNF(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	return m.quemComRotina(w, r, RotinaNFConfigurarAcesso)
}

func decodificarCorpoJSON(r *http.Request, destino any) error {
	return json.NewDecoder(r.Body).Decode(destino)
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/obras — o catálogo de obras conhecido (`centros_custo`)
// ---------------------------------------------------------------------------

func (m *Modulo) listarObrasNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeConfigurarAcessoNF(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), "centros_custo?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&order=obra_centro_custo&select=id,obra_centro_custo,comprador_nome", &linhas); err != nil {
		m.erro(w, "não consegui listar as obras conhecidas", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"obras": ouVazio(linhas)})
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/perfis — quem pode ganhar acesso a uma obra
// ---------------------------------------------------------------------------

func (m *Modulo) listarPerfisNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeConfigurarAcessoNF(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), "perfis?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&ativo=eq.true&order=nome&select=id,nome,usuario,categorias(nome)", &linhas); err != nil {
		m.erro(w, "não consegui listar os perfis", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"perfis": ouVazio(linhas)})
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/acessos — os vínculos já concedidos
// ---------------------------------------------------------------------------

func (m *Modulo) listarAcessosNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeConfigurarAcessoNF(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := "centro_custo_acessos?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&order=criado_em.desc&select=id,perfil_id,centro_custo_id,criado_em," +
		"perfis(nome),centros_custo(obra_centro_custo)&limit=" + fmt.Sprint(TetoDaLista)
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar os acessos concedidos", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"acessos": ouVazio(linhas)})
}

// ---------------------------------------------------------------------------
// POST /administrativo/nf/acessos — conceder
// ---------------------------------------------------------------------------

func (m *Modulo) concederAcessoNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeConfigurarAcessoNF(w, r)
	if p == nil {
		return
	}
	var corpo struct {
		PerfilID      string `json:"perfil_id"`
		CentroCustoID string `json:"centro_custo_id"`
	}
	if err := decodificarCorpoJSON(r, &corpo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi o que veio no corpo.")
		return
	}
	perfilID, ok1 := umUUID(corpo.PerfilID)
	centroID, ok2 := umUUID(corpo.CentroCustoID)
	if !ok1 || !ok2 {
		web.Falhar(w, http.StatusBadRequest, "Escolha um perfil e uma obra.")
		return
	}
	var criados []map[string]any
	if err := m.bd.Upsert(r.Context(), "centro_custo_acessos?on_conflict=perfil_id,centro_custo_id", []map[string]any{{
		"cliente_id":      p.ClienteID,
		"perfil_id":       perfilID,
		"centro_custo_id": centroID,
		"concedido_por":   p.UserID,
	}}, &criados); err != nil {
		m.erro(w, "não consegui conceder este acesso", err)
		return
	}
	id := ""
	if len(criados) > 0 {
		id = strCampo(criados[0]["id"])
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "conceder_acesso_obra_nf", map[string]historico.Mudanca{
		"perfil_id":       {De: nil, Para: perfilID},
		"centro_custo_id": {De: nil, Para: centroID},
	})
	web.Responder(w, http.StatusOK, map[string]any{"id": id})
}

// ---------------------------------------------------------------------------
// DELETE /administrativo/nf/acessos/{id} — revogar
// ---------------------------------------------------------------------------

func (m *Modulo) revogarAcessoNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeConfigurarAcessoNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if err := m.bd.Apagar(r.Context(), "centro_custo_acessos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID)); err != nil {
		m.erro(w, "não consegui revogar este acesso", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "revogar_acesso_obra_nf", nil)
	web.Responder(w, http.StatusOK, map[string]any{"revogado": true})
}
