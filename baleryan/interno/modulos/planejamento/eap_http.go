// rev 1 — cronogramas, nós da EAP e dependências
//
// SEM CPM AINDA
//
//	As datas dos nós são digitadas à mão; folga e `eh_critico` existem como
//	coluna (migração 057... na verdade a 056 só cuidou de RLS/permissão, o
//	schema já veio pronto da fase 2 original) mas ninguém calcula ainda. Isso é
//	Fase 3, junto com RDO e equipes — as rotinas já estão semeadas esperando.
package planejamento

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

func (m *Modulo) montarRotasEAP(mux *http.ServeMux) {
	mux.HandleFunc("GET /obras/{obraId}/cronogramas", m.listarCronogramas)
	mux.HandleFunc("POST /obras/{obraId}/cronogramas", m.criarCronograma)
	mux.HandleFunc("GET /obras/{obraId}/cronograma-atual", m.obterCronogramaAtual)

	mux.HandleFunc("GET /cronogramas/{cronogramaId}/eap", m.listarEAP)
	mux.HandleFunc("POST /cronogramas/{cronogramaId}/eap", m.criarEAPNo)
	mux.HandleFunc("PATCH /eap/{id}", m.editarEAPNo)
	mux.HandleFunc("POST /eap/{id}/mover", m.moverEAPNo)

	mux.HandleFunc("GET /cronogramas/{cronogramaId}/dependencias", m.listarDependencias)
	mux.HandleFunc("POST /cronogramas/{cronogramaId}/dependencias", m.criarDependencia)
	mux.HandleFunc("DELETE /dependencias/{id}", m.removerDependencia)
}

// ---------------------------------------------------------------------------
// cronogramas
// ---------------------------------------------------------------------------

func (m *Modulo) listarCronogramas(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	obraID := r.PathValue("obraId")
	if !m.obraEhDoMeuCliente(p, obraID) {
		web.Falhar(w, http.StatusNotFound, "Obra não encontrada.")
		return
	}
	var linhas []map[string]any
	caminho := "cronogramas?obra_id=eq." + banco.Escapar(obraID) + "&order=versao.desc"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os cronogramas.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"cronogramas": linhas})
}

type pedidoCronograma struct {
	Tipo          string  `json:"tipo"`
	MotivoRevisao *string `json:"motivo_revisao"`
}

func (m *Modulo) criarCronograma(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaEAPEditar)
	if p == nil {
		return
	}
	obraID := r.PathValue("obraId")
	if !m.obraEhDoMeuCliente(p, obraID) {
		web.Falhar(w, http.StatusNotFound, "Obra não encontrada.")
		return
	}
	var in pedidoCronograma
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	tipo := in.Tipo
	if tipo == "" {
		tipo = "atual"
	}
	if tipo != "baseline" && tipo != "atual" && tipo != "revisao" {
		web.Falhar(w, http.StatusBadRequest, "Tipo de cronograma inválido.")
		return
	}

	versao := 1
	var ultimos []struct {
		Versao int `json:"versao"`
	}
	if err := m.bd.Buscar(r.Context(), "cronogramas?obra_id=eq."+banco.Escapar(obraID)+"&select=versao&order=versao.desc&limit=1", &ultimos); err == nil && len(ultimos) > 0 {
		versao = ultimos[0].Versao + 1
	}

	if tipo == "atual" {
		// Só um cronograma "atual" fica ativo por obra — o anterior sai de cena,
		// não é apagado (CORE-05): continua na tabela, só com `ativo=false`.
		_ = m.bd.Atualizar(r.Context(), "cronogramas",
			"obra_id=eq."+banco.Escapar(obraID)+"&tipo=eq.atual", map[string]any{"ativo": false})
	}

	row := map[string]any{
		"obra_id":    obraID,
		"versao":     versao,
		"tipo":       tipo,
		"ativo":      tipo == "atual",
		"criado_por": p.UserID,
	}
	if in.MotivoRevisao != nil {
		row["motivo_revisao"] = *in.MotivoRevisao
	}
	var criados []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Inserir(r.Context(), "cronogramas", []map[string]any{row}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar o cronograma.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "O cronograma foi criado, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID, "versao": versao}, m.registrar(r, p, criados[0].ID, "criou_cronograma", map[string]historico.Mudanca{
		"versao": {De: nil, Para: versao},
	}))
	web.Responder(w, http.StatusCreated, resposta)
}

func (m *Modulo) obterCronogramaAtual(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	obraID := r.PathValue("obraId")
	if !m.obraEhDoMeuCliente(p, obraID) {
		web.Falhar(w, http.StatusNotFound, "Obra não encontrada.")
		return
	}
	var linhas []map[string]any
	caminho := "cronogramas?obra_id=eq." + banco.Escapar(obraID) + "&tipo=eq.atual&ativo=eq.true&order=versao.desc&limit=1"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar o cronograma.")
		return
	}
	if len(linhas) == 0 {
		web.Falhar(w, http.StatusNotFound, "Esta obra ainda não tem cronograma. Crie um com POST /obras/{id}/cronogramas.")
		return
	}
	web.Responder(w, http.StatusOK, linhas[0])
}

// ---------------------------------------------------------------------------
// nós da EAP
// ---------------------------------------------------------------------------

func (m *Modulo) listarEAP(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	cronID := r.PathValue("cronogramaId")
	if _, ok := m.carregarCronograma(p, cronID); !ok {
		web.Falhar(w, http.StatusNotFound, "Cronograma não encontrado.")
		return
	}
	var nos []map[string]any
	caminho := "eap_nos?cronograma_id=eq." + banco.Escapar(cronID) + "&order=ordem.asc,codigo_eap.asc"
	if err := m.bd.Buscar(r.Context(), caminho, &nos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar a EAP.")
		return
	}
	if r.URL.Query().Get("formato") == "arvore" {
		web.Responder(w, http.StatusOK, montarArvore(nos))
		return
	}
	web.Responder(w, http.StatusOK, nos)
}

type pedidoEAPNo struct {
	PaiID              *string  `json:"pai_id"`
	Nome               string   `json:"nome"`
	Descricao          *string  `json:"descricao"`
	UnidadeMedida      *string  `json:"unidade_medida"`
	Quantidade         *float64 `json:"quantidade"`
	DuracaoDias        *float64 `json:"duracao_dias"`
	DuracaoManual      *bool    `json:"duracao_manual"`
	DataInicioPrevista *string  `json:"data_inicio_prevista"`
	DataFimPrevista    *string  `json:"data_fim_prevista"`
	Peso               *float64 `json:"peso"`
	CustoPrevisto      *float64 `json:"custo_previsto"`
	IsMarco            *bool    `json:"is_marco"`
	Status             *string  `json:"status"`
	Ordem              *int     `json:"ordem"`
}

func (m *Modulo) criarEAPNo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaEAPEditar)
	if p == nil {
		return
	}
	cronID := r.PathValue("cronogramaId")
	cron, ok := m.carregarCronograma(p, cronID)
	if !ok {
		web.Falhar(w, http.StatusNotFound, "Cronograma não encontrado.")
		return
	}
	obraID := asString(cron["obra_id"])

	var in pedidoEAPNo
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome do item.")
		return
	}

	nivel := 1
	codigoPai := ""
	if in.PaiID != nil && *in.PaiID != "" {
		pai, ok := m.carregarEAPNo(p, *in.PaiID, cronID)
		if !ok {
			web.Falhar(w, http.StatusBadRequest, "O item-pai informado não existe neste cronograma.")
			return
		}
		nivel = nivelFilho(asInt(pai["nivel"]))
		codigoPai = asString(pai["codigo_eap"])
	}

	ordem := m.proximaOrdem(r.Context(), cronID, in.PaiID)
	if in.Ordem != nil && *in.Ordem > 0 {
		ordem = *in.Ordem
	}

	row := map[string]any{
		"obra_id": obraID, "cronograma_id": cronID, "nome": in.Nome,
		"nivel": nivel, "ordem": ordem, "codigo_eap": codigoEAPFilho(codigoPai, ordem),
		"duracao_manual": false, "is_marco": false, "status": "nao_iniciado", "peso": 1,
	}
	if in.PaiID != nil && *in.PaiID != "" {
		row["pai_id"] = *in.PaiID
	}
	setPtr(row, "descricao", in.Descricao)
	setPtr(row, "unidade_medida", in.UnidadeMedida)
	setPtr(row, "data_inicio_prevista", in.DataInicioPrevista)
	setPtr(row, "data_fim_prevista", in.DataFimPrevista)
	if in.Quantidade != nil {
		row["quantidade"] = *in.Quantidade
	}
	if in.DuracaoDias != nil {
		row["duracao_dias"] = *in.DuracaoDias
	}
	if in.DuracaoManual != nil {
		row["duracao_manual"] = *in.DuracaoManual
	}
	if in.Peso != nil {
		row["peso"] = *in.Peso
	}
	if in.CustoPrevisto != nil {
		row["custo_previsto"] = *in.CustoPrevisto
	}
	if in.IsMarco != nil {
		row["is_marco"] = *in.IsMarco
		if *in.IsMarco {
			row["duracao_dias"] = 0
		}
	}
	if in.Status != nil {
		row["status"] = *in.Status
	}

	var criados []struct {
		ID        string `json:"id"`
		CodigoEAP string `json:"codigo_eap"`
	}
	if err := m.bd.Inserir(r.Context(), "eap_nos", []map[string]any{row}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar o item da EAP.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "O item foi criado, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID, "codigo_eap": criados[0].CodigoEAP},
		m.registrar(r, p, criados[0].ID, "criou_item_eap", map[string]historico.Mudanca{
			"nome": {De: nil, Para: in.Nome}, "codigo_eap": {De: nil, Para: criados[0].CodigoEAP},
		}))
	web.Responder(w, http.StatusCreated, resposta)
}

func (m *Modulo) editarEAPNo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaEAPEditar)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	no, ok := m.carregarEAPNoPorID(p, id)
	if !ok {
		web.Falhar(w, http.StatusNotFound, "Item da EAP não encontrado.")
		return
	}
	cronID := asString(no["cronograma_id"])

	var in pedidoEAPNo
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome do item.")
		return
	}

	campos := map[string]any{"nome": in.Nome}
	setPtr(campos, "descricao", in.Descricao)
	setPtr(campos, "unidade_medida", in.UnidadeMedida)
	setPtr(campos, "data_inicio_prevista", in.DataInicioPrevista)
	setPtr(campos, "data_fim_prevista", in.DataFimPrevista)
	if in.Quantidade != nil {
		campos["quantidade"] = *in.Quantidade
	}
	if in.DuracaoDias != nil {
		campos["duracao_dias"] = *in.DuracaoDias
	}
	if in.DuracaoManual != nil {
		campos["duracao_manual"] = *in.DuracaoManual
	}
	if in.Peso != nil {
		campos["peso"] = *in.Peso
	}
	if in.CustoPrevisto != nil {
		campos["custo_previsto"] = *in.CustoPrevisto
	}
	if in.IsMarco != nil {
		campos["is_marco"] = *in.IsMarco
		if *in.IsMarco {
			campos["duracao_dias"] = 0
		}
	}
	if in.Status != nil {
		campos["status"] = *in.Status
	}

	filtro := "id=eq." + banco.Escapar(id) + "&cronograma_id=eq." + banco.Escapar(cronID)
	if err := m.bd.Atualizar(r.Context(), "eap_nos", filtro, campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui salvar a alteração.")
		return
	}
	resposta := mesclar(map[string]any{"ok": true}, m.registrar(r, p, id, "editou_item_eap", map[string]historico.Mudanca{
		"nome": {De: no["nome"], Para: in.Nome},
	}))
	web.Responder(w, http.StatusOK, resposta)
}

type pedidoMoverEAP struct {
	PaiID *string `json:"pai_id"`
	Ordem int     `json:"ordem"`
}

// POST /eap/{id}/mover — trocar de pai e/ou de posição. É a única rota da EAP
// que reescreve `codigo_eap` em cascata: mover um grupo move a numeração de
// tudo que está dentro dele (recalcularSubarvore).
func (m *Modulo) moverEAPNo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaEAPEditar)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	no, ok := m.carregarEAPNoPorID(p, id)
	if !ok {
		web.Falhar(w, http.StatusNotFound, "Item da EAP não encontrado.")
		return
	}
	cronID := asString(no["cronograma_id"])

	var in pedidoMoverEAP
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Ordem <= 0 {
		web.Falhar(w, http.StatusBadRequest, "A ordem tem que ser 1 ou mais.")
		return
	}
	if in.PaiID != nil && *in.PaiID != "" {
		if *in.PaiID == id {
			web.Falhar(w, http.StatusBadRequest, "Um item não pode ser pai de si mesmo.")
			return
		}
		if m.ehDescendente(r.Context(), cronID, id, *in.PaiID) {
			web.Falhar(w, http.StatusBadRequest, "O destino é descendente do item que está sendo movido.")
			return
		}
	}

	nivel := 1
	campos := map[string]any{"ordem": in.Ordem}
	var codigoNovo string
	if in.PaiID == nil || *in.PaiID == "" {
		campos["pai_id"] = nil
		campos["nivel"] = 1
		codigoNovo = codigoEAPRaiz(in.Ordem)
	} else {
		pai, ok := m.carregarEAPNo(p, *in.PaiID, cronID)
		if !ok {
			web.Falhar(w, http.StatusBadRequest, "O item-pai informado não existe neste cronograma.")
			return
		}
		nivel = nivelFilho(asInt(pai["nivel"]))
		codigoNovo = codigoEAPFilho(asString(pai["codigo_eap"]), in.Ordem)
		campos["pai_id"] = *in.PaiID
		campos["nivel"] = nivel
	}
	campos["codigo_eap"] = codigoNovo

	filtro := "id=eq." + banco.Escapar(id) + "&cronograma_id=eq." + banco.Escapar(cronID)
	var atualizados []map[string]any
	if err := m.bd.AtualizarDevolvendo(r.Context(), "eap_nos", filtro, campos, &atualizados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui mover o item.")
		return
	}
	if len(atualizados) == 0 {
		web.Falhar(w, http.StatusNotFound, "Item da EAP não encontrado.")
		return
	}
	m.recalcularSubarvore(r.Context(), cronID, id, codigoNovo, nivel)
	resposta := mesclar(map[string]any{"ok": true}, m.registrar(r, p, id, "moveu_item_eap", map[string]historico.Mudanca{
		"codigo_eap": {De: no["codigo_eap"], Para: codigoNovo},
	}))
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// dependências
// ---------------------------------------------------------------------------

func (m *Modulo) listarDependencias(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	cronID := r.PathValue("cronogramaId")
	if _, ok := m.carregarCronograma(p, cronID); !ok {
		web.Falhar(w, http.StatusNotFound, "Cronograma não encontrado.")
		return
	}
	var linhas []map[string]any
	caminho := "eap_dependencias?cronograma_id=eq." + banco.Escapar(cronID) + "&order=criado_em.asc"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as dependências.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"dependencias": linhas})
}

type pedidoDependencia struct {
	PredecessorID string `json:"predecessor_id"`
	SucessorID    string `json:"sucessor_id"`
	Tipo          string `json:"tipo"`
	LagDias       *int   `json:"lag_dias"`
}

func (m *Modulo) criarDependencia(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaEAPEditar)
	if p == nil {
		return
	}
	cronID := r.PathValue("cronogramaId")
	if _, ok := m.carregarCronograma(p, cronID); !ok {
		web.Falhar(w, http.StatusNotFound, "Cronograma não encontrado.")
		return
	}
	var in pedidoDependencia
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.PredecessorID == "" || in.SucessorID == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o predecessor e o sucessor.")
		return
	}
	if in.PredecessorID == in.SucessorID {
		web.Falhar(w, http.StatusBadRequest, "O predecessor e o sucessor precisam ser itens diferentes.")
		return
	}
	tipo := in.Tipo
	if tipo == "" {
		tipo = "FS"
	}
	if tipo != "FS" && tipo != "SS" && tipo != "FF" && tipo != "SF" {
		web.Falhar(w, http.StatusBadRequest, "Tipo de dependência inválido — use FS, SS, FF ou SF.")
		return
	}
	if _, ok := m.carregarEAPNo(p, in.PredecessorID, cronID); !ok {
		web.Falhar(w, http.StatusBadRequest, "Predecessor inválido.")
		return
	}
	if _, ok := m.carregarEAPNo(p, in.SucessorID, cronID); !ok {
		web.Falhar(w, http.StatusBadRequest, "Sucessor inválido.")
		return
	}
	lag := 0
	if in.LagDias != nil {
		lag = *in.LagDias
	}
	row := map[string]any{
		"cronograma_id": cronID, "predecessor_id": in.PredecessorID, "sucessor_id": in.SucessorID,
		"tipo": tipo, "lag_dias": lag,
	}
	var criados []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Inserir(r.Context(), "eap_dependencias", []map[string]any{row}, &criados); err != nil {
		if banco.Duplicado(err) {
			web.Falhar(w, http.StatusConflict, "Já existe uma dependência entre estes dois itens.")
			return
		}
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar a dependência.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "A dependência foi criada, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID}, m.registrar(r, p, criados[0].ID, "criou_dependencia", map[string]historico.Mudanca{
		"tipo": {De: nil, Para: tipo},
	}))
	web.Responder(w, http.StatusCreated, resposta)
}

// removerDependencia é uma das poucas linhas que este sistema de fato apaga
// (CORE-05 abre exceção para LIGAÇÃO, não FATO): uma dependência errada não
// tem história para contar.
func (m *Modulo) removerDependencia(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaEAPEditar)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var linhas []struct {
		ID           string `json:"id"`
		CronogramaID string `json:"cronograma_id"`
	}
	if err := m.bd.Buscar(r.Context(), "eap_dependencias?id=eq."+banco.Escapar(id)+"&select=id,cronograma_id&limit=1", &linhas); err != nil || len(linhas) == 0 {
		web.Falhar(w, http.StatusNotFound, "Dependência não encontrada.")
		return
	}
	if _, ok := m.carregarCronograma(p, linhas[0].CronogramaID); !ok {
		web.Falhar(w, http.StatusNotFound, "Dependência não encontrada.")
		return
	}
	if err := m.bd.Apagar(r.Context(), "eap_dependencias", "id=eq."+banco.Escapar(id)); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui remover a dependência.")
		return
	}
	resposta := mesclar(map[string]any{"ok": true}, m.registrar(r, p, id, "removeu_dependencia", nil))
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// auxiliares — todos conferem o dono via join até `obras.cliente_id`, porque
// nem cronograma nem nó de EAP carregam cliente_id direto (P-14: a checagem
// mora toda no motor, o banco não filtra nada sozinho para a service_role).
// ---------------------------------------------------------------------------

func (m *Modulo) carregarCronograma(p *seguranca.Principal, cronID string) (map[string]any, bool) {
	var linhas []map[string]any
	caminho := "cronogramas?id=eq." + banco.Escapar(cronID) +
		"&select=*,obras!inner(cliente_id)&obras.cliente_id=eq." + banco.Escapar(p.ClienteID) + "&limit=1"
	if err := m.bd.Buscar(context.Background(), caminho, &linhas); err != nil || len(linhas) == 0 {
		return nil, false
	}
	return linhas[0], true
}

func (m *Modulo) carregarEAPNo(p *seguranca.Principal, id, cronID string) (map[string]any, bool) {
	var linhas []map[string]any
	caminho := "eap_nos?id=eq." + banco.Escapar(id) + "&cronograma_id=eq." + banco.Escapar(cronID) +
		"&select=*,obras!inner(cliente_id)&obras.cliente_id=eq." + banco.Escapar(p.ClienteID) + "&limit=1"
	if err := m.bd.Buscar(context.Background(), caminho, &linhas); err != nil || len(linhas) == 0 {
		return nil, false
	}
	return linhas[0], true
}

func (m *Modulo) carregarEAPNoPorID(p *seguranca.Principal, id string) (map[string]any, bool) {
	var linhas []map[string]any
	caminho := "eap_nos?id=eq." + banco.Escapar(id) +
		"&select=*,obras!inner(cliente_id)&obras.cliente_id=eq." + banco.Escapar(p.ClienteID) + "&limit=1"
	if err := m.bd.Buscar(context.Background(), caminho, &linhas); err != nil || len(linhas) == 0 {
		return nil, false
	}
	return linhas[0], true
}

func (m *Modulo) proximaOrdem(ctx context.Context, cronID string, paiID *string) int {
	caminho := "eap_nos?cronograma_id=eq." + banco.Escapar(cronID) + "&select=ordem&order=ordem.desc&limit=1"
	if paiID == nil || *paiID == "" {
		caminho += "&pai_id=is.null"
	} else {
		caminho += "&pai_id=eq." + banco.Escapar(*paiID)
	}
	var linhas []struct {
		Ordem int `json:"ordem"`
	}
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil || len(linhas) == 0 {
		return 1
	}
	return linhas[0].Ordem + 1
}

// ehDescendente impede o laço clássico de árvore: mover um grupo para dentro
// do seu próprio neto criaria um ciclo sem fim.
func (m *Modulo) ehDescendente(ctx context.Context, cronID, ancestralID, candidatoID string) bool {
	var nos []struct {
		ID    string `json:"id"`
		PaiID string `json:"pai_id"`
	}
	if err := m.bd.Buscar(ctx, "eap_nos?cronograma_id=eq."+banco.Escapar(cronID)+"&select=id,pai_id", &nos); err != nil {
		return false
	}
	pais := map[string]string{}
	for _, n := range nos {
		pais[n.ID] = n.PaiID
	}
	atual := candidatoID
	for atual != "" {
		if atual == ancestralID {
			return true
		}
		atual = pais[atual]
	}
	return false
}

// recalcularSubarvore reescreve codigo_eap/nivel/ordem de toda a descendência
// depois que o pai mudou de lugar — é o que mantém "1.2.3" batendo com a
// posição de verdade na árvore, e não com onde o item nasceu.
func (m *Modulo) recalcularSubarvore(ctx context.Context, cronID, paiID, codigoPai string, nivelPai int) {
	var filhos []map[string]any
	caminho := "eap_nos?cronograma_id=eq." + banco.Escapar(cronID) + "&pai_id=eq." + banco.Escapar(paiID) + "&order=ordem.asc"
	if err := m.bd.Buscar(ctx, caminho, &filhos); err != nil {
		return
	}
	for i, f := range filhos {
		fid := asString(f["id"])
		ordem := i + 1
		if o := asInt(f["ordem"]); o > 0 {
			ordem = o
		}
		novoCodigo := codigoEAPFilho(codigoPai, ordem)
		nivel := nivelFilho(nivelPai)
		campos := map[string]any{"codigo_eap": novoCodigo, "nivel": nivel, "ordem": ordem}
		_ = m.bd.Atualizar(ctx, "eap_nos", "id=eq."+banco.Escapar(fid), campos)
		m.recalcularSubarvore(ctx, cronID, fid, novoCodigo, nivel)
	}
}
