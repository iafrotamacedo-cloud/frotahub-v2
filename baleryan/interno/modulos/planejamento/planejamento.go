// rev 1 — Engenharia > Planejamento: contratantes, obras, calendários
//
// O QUE ESTE MÓDULO É
//
//	Cronograma de obra: quem contratou, onde é a obra, em que calendário ela
//	trabalha, e — em `eap_http.go` — a EAP (estrutura analítica do projeto) e as
//	dependências entre os itens dela.
//
// MODELO DE CLIENTES: TENANT E CONTRATANTE NA MESMA TABELA
//
//	`clientes` já tinha um papel — a empresa do grupo, dona do login
//	(`tipo='tenant'`, é para onde `perfis.cliente_id` aponta). Este módulo
//	introduz o segundo papel na MESMA tabela: `tipo='contratante'`, com
//	`tenant_id` apontando para o tenant dono do cadastro. É a mesma pessoa
//	jurídica de sempre — só que agora pode ser "quem contratou a obra" além de
//	"quem loga no sistema". Ver migração 056 para a policy de RLS que separa
//	os dois casos.
//
// QUEM PODE O QUÊ
//
//	PLANEJAMENTO_OBRAS_DADOS         ver obras e contratantes
//	PLANEJAMENTO_OBRAS_EDITAR        criar/editar obras e contratantes
//	PLANEJAMENTO_EAP_EDITAR          editar EAP, cronograma e dependências (eap_http.go)
//	PLANEJAMENTO_CALENDARIO_GERENCIAR calendários, feriados, exceções
//
//	Ainda sem rota (rotina já semeada, esperando a Fase 3): RDO_LANCAR,
//	RDO_APROVAR, EQUIPES_GERENCIAR.
package planejamento

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

const (
	RotinaObrasDados         = "PLANEJAMENTO_OBRAS_DADOS"
	RotinaObrasEditar        = "PLANEJAMENTO_OBRAS_EDITAR"
	RotinaEAPEditar          = "PLANEJAMENTO_EAP_EDITAR"
	RotinaCalendarioGerencia = "PLANEJAMENTO_CALENDARIO_GERENCIAR"
)

// O nome deste módulo dentro da tabela `historico`, que é compartilhada.
const moduloHistorico = "planejamento"

type Modulo struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
	hist *historico.Servico
}

func Novo(bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico, hist *historico.Servico) *Modulo {
	return &Modulo{bd: bd, seg: seg, perm: perm, hist: hist}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /contratantes", m.listarContratantes)
	mux.HandleFunc("POST /contratantes", m.criarContratante)
	mux.HandleFunc("GET /contratantes/{id}", m.obterContratante)
	mux.HandleFunc("PATCH /contratantes/{id}", m.editarContratante)

	mux.HandleFunc("GET /obras", m.listarObras)
	mux.HandleFunc("POST /obras", m.criarObra)
	mux.HandleFunc("GET /obras/{id}", m.obterObra)
	mux.HandleFunc("PATCH /obras/{id}", m.editarObra)

	mux.HandleFunc("GET /calendarios", m.listarCalendarios)
	mux.HandleFunc("POST /calendarios", m.criarCalendario)
	mux.HandleFunc("PATCH /calendarios/{id}", m.editarCalendario)
	mux.HandleFunc("GET /feriados", m.listarFeriados)
	mux.HandleFunc("POST /feriados", m.criarFeriado)
	mux.HandleFunc("GET /calendario-excecoes", m.listarExcecoes)
	mux.HandleFunc("POST /calendario-excecoes", m.criarExcecao)

	m.montarRotasEAP(mux)
}

// quem resolve as duas perguntas de toda rota daqui: quem é, e se alcança
// esta rotina. Mesmo desenho do módulo `funcionarios`.
func (m *Modulo) quem(w http.ResponseWriter, r *http.Request, rotina string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, rotina); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	return p
}

func semCancelar(r *http.Request) context.Context { return context.WithoutCancel(r.Context()) }

func (m *Modulo) registrar(r *http.Request, p *seguranca.Principal, registroID, acao string, mudancas map[string]historico.Mudanca) map[string]any {
	resposta := map[string]any{}
	if err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, registroID, acao, mudancas); err != nil {
		resposta["aviso"] = historico.Aviso
	}
	return resposta
}

func setPtr(row map[string]any, chave string, val *string) {
	if val != nil {
		row[chave] = *val
	}
}

func mesclar(a, b map[string]any) map[string]any {
	for k, v := range b {
		a[k] = v
	}
	return a
}

// ---------------------------------------------------------------------------
// contratantes — linhas de `clientes` com tipo='contratante'
// ---------------------------------------------------------------------------

type contratante struct {
	ID                string  `json:"id"`
	Nome              string  `json:"nome"`
	TipoPessoa        *string `json:"tipo_pessoa"`
	RazaoSocial       *string `json:"razao_social"`
	NomeFantasia      *string `json:"nome_fantasia"`
	CPFCNPJ           *string `json:"cpf_cnpj"`
	InscricaoEstadual *string `json:"inscricao_estadual"`
	Logradouro        *string `json:"logradouro"`
	Numero            *string `json:"numero"`
	Bairro            *string `json:"bairro"`
	Cidade            *string `json:"cidade"`
	UF                *string `json:"uf"`
	CEP               *string `json:"cep"`
	Ativo             bool    `json:"ativo"`
	Observacoes       *string `json:"observacoes"`
}

func (m *Modulo) listarContratantes(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	caminho := "clientes?tipo=eq.contratante&tenant_id=eq." + banco.Escapar(p.ClienteID) + "&order=nome.asc"
	var linhas []contratante
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os contratantes.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"contratantes": linhas})
}

func (m *Modulo) obterContratante(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	c, ok := m.carregarContratante(r.Context(), r.PathValue("id"), p.ClienteID)
	if !ok {
		web.Falhar(w, http.StatusNotFound, "Contratante não encontrado.")
		return
	}
	web.Responder(w, http.StatusOK, c)
}

func (m *Modulo) carregarContratante(ctx context.Context, id, tenantID string) (*contratante, bool) {
	var linhas []contratante
	caminho := "clientes?id=eq." + banco.Escapar(id) + "&tipo=eq.contratante&tenant_id=eq." + banco.Escapar(tenantID) + "&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil || len(linhas) == 0 {
		return nil, false
	}
	return &linhas[0], true
}

type pedidoContratante struct {
	Nome              string  `json:"nome"`
	TipoPessoa        *string `json:"tipo_pessoa"`
	RazaoSocial       *string `json:"razao_social"`
	NomeFantasia      *string `json:"nome_fantasia"`
	CPFCNPJ           *string `json:"cpf_cnpj"`
	InscricaoEstadual *string `json:"inscricao_estadual"`
	Logradouro        *string `json:"logradouro"`
	Numero            *string `json:"numero"`
	Bairro            *string `json:"bairro"`
	Cidade            *string `json:"cidade"`
	UF                *string `json:"uf"`
	CEP               *string `json:"cep"`
	Ativo             *bool   `json:"ativo"`
	Observacoes       *string `json:"observacoes"`
}

func montarContratante(p *seguranca.Principal, in pedidoContratante) map[string]any {
	row := map[string]any{
		"tipo":      "contratante",
		"tenant_id": p.ClienteID,
		"nome":      in.Nome,
		"contatos":  []any{},
	}
	setPtr(row, "tipo_pessoa", in.TipoPessoa)
	setPtr(row, "razao_social", in.RazaoSocial)
	setPtr(row, "nome_fantasia", in.NomeFantasia)
	setPtr(row, "cpf_cnpj", in.CPFCNPJ)
	setPtr(row, "inscricao_estadual", in.InscricaoEstadual)
	setPtr(row, "logradouro", in.Logradouro)
	setPtr(row, "numero", in.Numero)
	setPtr(row, "bairro", in.Bairro)
	setPtr(row, "cidade", in.Cidade)
	setPtr(row, "uf", in.UF)
	setPtr(row, "cep", in.CEP)
	setPtr(row, "observacoes", in.Observacoes)
	if in.Ativo != nil {
		row["ativo"] = *in.Ativo
	} else {
		row["ativo"] = true
	}
	return row
}

func (m *Modulo) criarContratante(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasEditar)
	if p == nil {
		return
	}
	var in pedidoContratante
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome do contratante.")
		return
	}
	row := montarContratante(p, in)
	var criados []contratante
	if err := m.bd.Inserir(r.Context(), "clientes", []map[string]any{row}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui cadastrar o contratante.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "O contratante foi criado, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID, "nome": criados[0].Nome},
		m.registrar(r, p, criados[0].ID, "cadastrou_contratante", map[string]historico.Mudanca{
			"nome": {De: nil, Para: in.Nome},
		}))
	web.Responder(w, http.StatusCreated, resposta)
}

func (m *Modulo) editarContratante(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasEditar)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	atual, ok := m.carregarContratante(r.Context(), id, p.ClienteID)
	if !ok {
		web.Falhar(w, http.StatusNotFound, "Contratante não encontrado.")
		return
	}
	var in pedidoContratante
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome do contratante.")
		return
	}
	campos := montarContratante(p, in)
	delete(campos, "tipo")
	delete(campos, "tenant_id")
	delete(campos, "contatos")
	filtro := "id=eq." + banco.Escapar(id) + "&tipo=eq.contratante&tenant_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "clientes", filtro, campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui salvar a alteração.")
		return
	}
	mudancas := map[string]historico.Mudanca{}
	if in.Nome != atual.Nome {
		mudancas["nome"] = historico.Mudanca{De: atual.Nome, Para: in.Nome}
	}
	resposta := mesclar(map[string]any{"ok": true}, m.registrar(r, p, id, "editou_contratante", mudancas))
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// obras
// ---------------------------------------------------------------------------

func (m *Modulo) listarObras(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	caminho := "obras?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=*,cliente_contratante:clientes!cliente_contratante_id(id,nome,cpf_cnpj),unidades(id,nome,cidade,uf)" +
		"&order=criado_em.desc"
	if v := r.URL.Query().Get("status"); v != "" {
		caminho += "&status=eq." + banco.Escapar(v)
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as obras.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"obras": linhas})
}

func (m *Modulo) obterObra(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasDados)
	if p == nil {
		return
	}
	caminho := "obras?id=eq." + banco.Escapar(r.PathValue("id")) + "&cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=*,cliente_contratante:clientes!cliente_contratante_id(id,nome,cpf_cnpj,contatos)," +
		"unidades(id,nome,endereco,cidade,uf,cnpj),calendarios(id,nome,dias_uteis)&limit=1"
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar a obra.")
		return
	}
	if len(linhas) == 0 {
		web.Falhar(w, http.StatusNotFound, "Obra não encontrada.")
		return
	}
	web.Responder(w, http.StatusOK, linhas[0])
}

type pedidoObra struct {
	ClienteContratanteID *string  `json:"cliente_contratante_id"`
	UnidadeID            *string  `json:"unidade_id"`
	Codigo               *string  `json:"codigo"`
	Nome                 string   `json:"nome"`
	Descricao            *string  `json:"descricao"`
	TipoObra             *string  `json:"tipo_obra"`
	Logradouro           *string  `json:"logradouro"`
	Numero               *string  `json:"numero"`
	Bairro               *string  `json:"bairro"`
	Cidade               *string  `json:"cidade"`
	UF                   *string  `json:"uf"`
	CEP                  *string  `json:"cep"`
	ResponsavelTecnicoID *string  `json:"responsavel_tecnico_id"`
	NumeroARTRRT         *string  `json:"numero_art_rrt"`
	ValorContrato        *float64 `json:"valor_contrato"`
	CalendarioID         *string  `json:"calendario_id"`
	DataInicioPrevista   *string  `json:"data_inicio_prevista"`
	DataFimPrevista      *string  `json:"data_fim_prevista"`
	Status               *string  `json:"status"`
	MotivoParalisacao    *string  `json:"motivo_paralisacao"`
}

func montarObra(p *seguranca.Principal, in pedidoObra) map[string]any {
	row := map[string]any{"cliente_id": p.ClienteID, "nome": in.Nome}
	setPtr(row, "cliente_contratante_id", in.ClienteContratanteID)
	setPtr(row, "unidade_id", in.UnidadeID)
	setPtr(row, "codigo", in.Codigo)
	setPtr(row, "descricao", in.Descricao)
	setPtr(row, "tipo_obra", in.TipoObra)
	setPtr(row, "logradouro", in.Logradouro)
	setPtr(row, "numero", in.Numero)
	setPtr(row, "bairro", in.Bairro)
	setPtr(row, "cidade", in.Cidade)
	setPtr(row, "uf", in.UF)
	setPtr(row, "cep", in.CEP)
	setPtr(row, "responsavel_tecnico_id", in.ResponsavelTecnicoID)
	setPtr(row, "numero_art_rrt", in.NumeroARTRRT)
	setPtr(row, "calendario_id", in.CalendarioID)
	setPtr(row, "data_inicio_prevista", in.DataInicioPrevista)
	setPtr(row, "data_fim_prevista", in.DataFimPrevista)
	setPtr(row, "motivo_paralisacao", in.MotivoParalisacao)
	if in.ValorContrato != nil {
		row["valor_contrato"] = *in.ValorContrato
	}
	if in.Status != nil {
		row["status"] = *in.Status
	} else {
		row["status"] = "planejamento"
	}
	return row
}

func (m *Modulo) criarObra(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasEditar)
	if p == nil {
		return
	}
	var in pedidoObra
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome da obra.")
		return
	}
	if in.ClienteContratanteID != nil {
		if _, ok := m.carregarContratante(r.Context(), *in.ClienteContratanteID, p.ClienteID); !ok {
			web.Falhar(w, http.StatusBadRequest, "Contratante inválido.")
			return
		}
	}
	row := montarObra(p, in)
	var criados []struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	}
	if err := m.bd.Inserir(r.Context(), "obras", []map[string]any{row}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui cadastrar a obra.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "A obra foi criada, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID, "nome": criados[0].Nome},
		m.registrar(r, p, criados[0].ID, "cadastrou_obra", map[string]historico.Mudanca{
			"nome": {De: nil, Para: in.Nome},
		}))
	web.Responder(w, http.StatusCreated, resposta)
}

func (m *Modulo) editarObra(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaObrasEditar)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	if !m.obraEhDoMeuCliente(p, id) {
		web.Falhar(w, http.StatusNotFound, "Obra não encontrada.")
		return
	}
	var in pedidoObra
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome da obra.")
		return
	}
	if in.ClienteContratanteID != nil {
		if _, ok := m.carregarContratante(r.Context(), *in.ClienteContratanteID, p.ClienteID); !ok {
			web.Falhar(w, http.StatusBadRequest, "Contratante inválido.")
			return
		}
	}
	campos := montarObra(p, in)
	delete(campos, "cliente_id")
	filtro := "id=eq." + banco.Escapar(id) + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "obras", filtro, campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui salvar a alteração.")
		return
	}
	resposta := mesclar(map[string]any{"ok": true}, m.registrar(r, p, id, "editou_obra", map[string]historico.Mudanca{
		"nome": {De: nil, Para: in.Nome},
	}))
	web.Responder(w, http.StatusOK, resposta)
}

func (m *Modulo) obraEhDoMeuCliente(p *seguranca.Principal, id string) bool {
	caminho := "obras?id=eq." + banco.Escapar(id) + "&cliente_id=eq." + banco.Escapar(p.ClienteID) + "&select=id&limit=1"
	var linhas []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Buscar(context.Background(), caminho, &linhas); err != nil {
		return false
	}
	return len(linhas) > 0
}

// ---------------------------------------------------------------------------
// calendários
// ---------------------------------------------------------------------------

func (m *Modulo) listarCalendarios(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCalendarioGerencia)
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := "calendarios?cliente_id=eq." + banco.Escapar(p.ClienteID) + "&order=padrao.desc,nome.asc"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os calendários.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"calendarios": linhas})
}

type pedidoCalendario struct {
	Nome      string `json:"nome"`
	DiasUteis []int  `json:"dias_uteis"`
	Padrao    *bool  `json:"padrao"`
}

func (m *Modulo) criarCalendario(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCalendarioGerencia)
	if p == nil {
		return
	}
	var in pedidoCalendario
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome do calendário.")
		return
	}
	dias := in.DiasUteis
	if len(dias) == 0 {
		dias = []int{1, 2, 3, 4, 5}
	}
	row := map[string]any{"cliente_id": p.ClienteID, "nome": in.Nome, "dias_uteis": dias, "padrao": false}
	if in.Padrao != nil {
		row["padrao"] = *in.Padrao
	}
	var criados []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Inserir(r.Context(), "calendarios", []map[string]any{row}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar o calendário.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "O calendário foi criado, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID}, m.registrar(r, p, criados[0].ID, "criou_calendario", map[string]historico.Mudanca{
		"nome": {De: nil, Para: in.Nome},
	}))
	web.Responder(w, http.StatusCreated, resposta)
}

func (m *Modulo) editarCalendario(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCalendarioGerencia)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var in pedidoCalendario
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome do calendário.")
		return
	}
	campos := map[string]any{"nome": in.Nome}
	if len(in.DiasUteis) > 0 {
		campos["dias_uteis"] = in.DiasUteis
	}
	if in.Padrao != nil {
		campos["padrao"] = *in.Padrao
	}
	filtro := "id=eq." + banco.Escapar(id) + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "calendarios", filtro, campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui salvar a alteração.")
		return
	}
	resposta := mesclar(map[string]any{"ok": true}, m.registrar(r, p, id, "editou_calendario", nil))
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// feriados
// ---------------------------------------------------------------------------

func (m *Modulo) listarFeriados(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCalendarioGerencia)
	if p == nil {
		return
	}
	caminho := "feriados?cliente_id=eq." + banco.Escapar(p.ClienteID) + "&order=data.asc"
	if v := r.URL.Query().Get("ano"); v != "" {
		caminho += "&data=gte." + banco.Escapar(v+"-01-01") + "&data=lte." + banco.Escapar(v+"-12-31")
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os feriados.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"feriados": linhas})
}

type pedidoFeriado struct {
	Data        string  `json:"data"`
	Nome        string  `json:"nome"`
	Abrangencia *string `json:"abrangencia"`
	UF          *string `json:"uf"`
	Municipio   *string `json:"municipio"`
	Recorrente  *bool   `json:"recorrente"`
}

func (m *Modulo) criarFeriado(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCalendarioGerencia)
	if p == nil {
		return
	}
	var in pedidoFeriado
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.Data == "" || in.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Data e nome são obrigatórios.")
		return
	}
	row := map[string]any{"cliente_id": p.ClienteID, "data": in.Data, "nome": in.Nome, "abrangencia": "nacional", "recorrente": false}
	if in.Abrangencia != nil {
		row["abrangencia"] = *in.Abrangencia
	}
	setPtr(row, "uf", in.UF)
	setPtr(row, "municipio", in.Municipio)
	if in.Recorrente != nil {
		row["recorrente"] = *in.Recorrente
	}
	var criados []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Inserir(r.Context(), "feriados", []map[string]any{row}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar o feriado.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "O feriado foi criado, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID}, m.registrar(r, p, criados[0].ID, "criou_feriado", map[string]historico.Mudanca{
		"nome": {De: nil, Para: in.Nome}, "data": {De: nil, Para: in.Data},
	}))
	web.Responder(w, http.StatusCreated, resposta)
}

// ---------------------------------------------------------------------------
// exceções de calendário
// ---------------------------------------------------------------------------

func (m *Modulo) listarExcecoes(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCalendarioGerencia)
	if p == nil {
		return
	}
	caminho := "calendario_excecoes?cliente_id=eq." + banco.Escapar(p.ClienteID) + "&order=data_inicio.asc"
	if v := r.URL.Query().Get("obra_id"); v != "" {
		caminho += "&obra_id=eq." + banco.Escapar(v)
	}
	if v := r.URL.Query().Get("calendario_id"); v != "" {
		caminho += "&calendario_id=eq." + banco.Escapar(v)
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as exceções de calendário.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"excecoes": linhas})
}

type pedidoExcecao struct {
	CalendarioID *string `json:"calendario_id"`
	ObraID       *string `json:"obra_id"`
	DataInicio   string  `json:"data_inicio"`
	DataFim      string  `json:"data_fim"`
	Motivo       *string `json:"motivo"`
	Tipo         *string `json:"tipo"`
}

func (m *Modulo) criarExcecao(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCalendarioGerencia)
	if p == nil {
		return
	}
	var in pedidoExcecao
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if in.DataInicio == "" || in.DataFim == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o início e o fim da exceção.")
		return
	}
	if in.CalendarioID == nil && in.ObraID == nil {
		web.Falhar(w, http.StatusBadRequest, "Informe o calendário ou a obra desta exceção.")
		return
	}
	row := map[string]any{"cliente_id": p.ClienteID, "data_inicio": in.DataInicio, "data_fim": in.DataFim, "tipo": "outro"}
	setPtr(row, "calendario_id", in.CalendarioID)
	setPtr(row, "obra_id", in.ObraID)
	setPtr(row, "motivo", in.Motivo)
	if in.Tipo != nil {
		row["tipo"] = *in.Tipo
	}
	var criados []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Inserir(r.Context(), "calendario_excecoes", []map[string]any{row}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar a exceção.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "A exceção foi criada, mas o banco não devolveu qual.")
		return
	}
	resposta := mesclar(map[string]any{"id": criados[0].ID}, m.registrar(r, p, criados[0].ID, "criou_excecao_calendario", map[string]historico.Mudanca{
		"data_inicio": {De: nil, Para: in.DataInicio}, "data_fim": {De: nil, Para: in.DataFim},
	}))
	web.Responder(w, http.StatusCreated, resposta)
}
