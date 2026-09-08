// rev 1 — SESMT e DP: funcionário da obra e a documentação dele
//
// O QUE ESTE MÓDULO É
//
//	Cadastro de funcionário (não é login — ver `perfis` para quem entra no
//	sistema) e o controle dos documentos que a lei exige dele: pessoais,
//	admissionais, ASO (PCMSO) e certificados de NR. Ver a migração 044 para o
//	desenho do esquema e o porquê de cada tabela.
//
// QUEM PODE O QUÊ
//
//	CONTRATO_FUNCIONARIOS_DADOS         — ver a lista e a ficha (CPF mascarado)
//	CONTRATO_FUNCIONARIOS_DOCUMENTOS    — enviar documento, ver a pilha
//	CONTRATO_FUNCIONARIOS_APROVAR       — aprovar ou reprovar um documento enviado
//	CONTRATO_FUNCIONARIOS_DADO_COMPLETO — ver CPF/RG sem máscara; cadastrar
//	                                       funcionário; criar função; definir
//	                                       requisito
//
//	A quarta é a mais forte: quem cadastra um funcionário digita o CPF inteiro,
//	então cadastrar já exige o mesmo acesso de ver sem máscara.
//
// ESTE ARQUIVO TRAZ: funcionário, função e o catálogo de tipos de documento.
// `documentos.go` traz o envio, a aprovação e a conformidade.
package funcionarios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

const (
	RotinaDados        = "CONTRATO_FUNCIONARIOS_DADOS"
	RotinaDocumentos   = "CONTRATO_FUNCIONARIOS_DOCUMENTOS"
	RotinaAprovar      = "CONTRATO_FUNCIONARIOS_APROVAR"
	RotinaDadoCompleto = "CONTRATO_FUNCIONARIOS_DADO_COMPLETO"
)

// O nome deste módulo dentro da tabela `historico`, que é compartilhada.
const moduloHistorico = "funcionarios"

const porPaginaPadrao = 25
const porPaginaMaximo = 100

// TamanhoMaximo é o teto de um documento enviado — mesmo teto que Orçamentos
// usa para nota fiscal (20 MB). Um documento de RH é PDF ou foto de página
// única; nunca chega perto disso.
const TamanhoMaximo = 20 << 20

var erroNaoEncontrado = fmt.Errorf("Funcionário não encontrado.")

type Modulo struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
	hist *historico.Servico
	arm  *armazem.Cliente
}

func Novo(bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico, hist *historico.Servico, arm *armazem.Cliente) *Modulo {
	return &Modulo{bd: bd, seg: seg, perm: perm, hist: hist, arm: arm}
}

// Montar registra as rotas deste módulo. O arquivo principal não conhece
// nenhuma delas — ele só monta o módulo (P-13).
func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /funcionarios", m.listar)
	mux.HandleFunc("POST /funcionarios", m.criar)
	mux.HandleFunc("GET /funcionarios/conformidade", m.conformidade)
	mux.HandleFunc("GET /funcionarios/vencendo", m.vencendo)
	mux.HandleFunc("GET /funcionarios/{id}", m.obter)
	mux.HandleFunc("GET /funcionarios/{id}/documentos", m.listarDocumentos)
	mux.HandleFunc("POST /funcionarios/{id}/documentos", m.enviarDocumento)
	mux.HandleFunc("POST /documentos/{id}/aprovar", m.aprovar)
	mux.HandleFunc("POST /documentos/{id}/reprovar", m.reprovar)

	mux.HandleFunc("GET /funcoes", m.listarFuncoes)
	mux.HandleFunc("POST /funcoes", m.criarFuncao)
	mux.HandleFunc("PUT /funcoes/{id}/requisitos", m.definirRequisitos)

	mux.HandleFunc("GET /tipos-documento", m.listarTiposDocumento)
}

// quem resolve as duas perguntas de toda rota daqui: quem é, e se alcança
// esta rotina. Mesmo desenho de `quemEBuilder` em usuarios/acesso — só que
// aqui a permissão é por rotina, não exclusiva do builder.
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

// semCancelar desliga o histórico do tempo de vida da requisição. Se o
// navegador desistir no meio, a alteração já foi feita — o registro dela não
// pode ir junto.
func semCancelar(r *http.Request) context.Context { return context.WithoutCancel(r.Context()) }

func paginacao(r *http.Request) (pagina, porPagina, inicio int) {
	pagina = max(1, inteiro(r.URL.Query().Get("pagina"), 1))
	porPagina = inteiro(r.URL.Query().Get("por_pagina"), porPaginaPadrao)
	if porPagina < 1 || porPagina > porPaginaMaximo {
		porPagina = porPaginaPadrao
	}
	return pagina, porPagina, (pagina - 1) * porPagina
}

func inteiro(s string, padrao int) int {
	if s == "" {
		return padrao
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return padrao
	}
	return n
}

// ---------------------------------------------------------------------------
// máscara de CPF/RG
//
// Some para quem não tem CONTRATO_FUNCIONARIOS_DADO_COMPLETO. "123.456.789-00"
// vira "123.***.**9-00" — dá para conferir que é a pessoa certa (começo e
// fim batem) sem expor o documento inteiro para quem só precisa conferir
// certificado de NR.
// ---------------------------------------------------------------------------

func mascararCPF(cpf string) string {
	if len(cpf) < 6 {
		return "***"
	}
	return cpf[:3] + "***" + cpf[len(cpf)-4:]
}

func mascararRG(rg string) string {
	if rg == "" || len(rg) <= 2 {
		return rg
	}
	return "***" + rg[len(rg)-2:]
}

// ---------------------------------------------------------------------------
// listar
// ---------------------------------------------------------------------------

type linhaFuncionario struct {
	ID           string  `json:"id"`
	NomeCompleto string  `json:"nome_completo"`
	CPF          string  `json:"cpf"`
	RG           *string `json:"rg"`
	Status       string  `json:"status"`
	FuncaoID     *string `json:"funcao_id"`
	UnidadeID    *string `json:"unidade_id"`
	Funcoes      *struct {
		Nome string `json:"nome"`
	} `json:"funcoes"`
	Unidades *struct {
		Nome string `json:"nome"`
	} `json:"unidades"`
}

// GET /funcionarios?pagina=1&por_pagina=25&busca=joao&status=ativo&funcao_id=...
func (m *Modulo) listar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDados)
	if p == nil {
		return
	}

	pagina, porPagina, inicio := paginacao(r)

	// O filtro por cliente vai aqui TAMBÉM, apesar de a política do banco já
	// filtrar: a chave de serviço passa por cima das políticas, então no
	// servidor o filtro tem que ser explícito.
	caminho := "funcionarios?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=id,nome_completo,cpf,rg,status,funcao_id,unidade_id,funcoes(nome),unidades(nome)" +
		"&order=nome_completo.asc" +
		"&limit=" + strconv.Itoa(porPagina) + "&offset=" + strconv.Itoa(inicio)

	if busca := strings.TrimSpace(r.URL.Query().Get("busca")); busca != "" {
		caminho += "&or=(nome_completo.ilike.*" + banco.Escapar(busca) + "*,cpf.ilike.*" + banco.Escapar(busca) + "*)"
	}
	if status := r.URL.Query().Get("status"); status != "" {
		caminho += "&status=eq." + banco.Escapar(status)
	}
	if funcaoID := r.URL.Query().Get("funcao_id"); funcaoID != "" {
		caminho += "&funcao_id=eq." + banco.Escapar(funcaoID)
	}

	var linhas []linhaFuncionario
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os funcionários. Tente de novo em instantes.")
		return
	}

	completo, err := m.perm.Pode(r.Context(), p, RotinaDadoCompleto)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir a sua permissão.")
		return
	}
	if !completo {
		for i := range linhas {
			linhas[i].CPF = mascararCPF(linhas[i].CPF)
			if linhas[i].RG != nil {
				m := mascararRG(*linhas[i].RG)
				linhas[i].RG = &m
			}
		}
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"funcionarios": linhas,
		"pagina":       pagina,
		"por_pagina":   porPagina,
		"tem_mais":     len(linhas) == porPagina,
	})
}

// ---------------------------------------------------------------------------
// obter
// ---------------------------------------------------------------------------

func (m *Modulo) funcionarioDoCliente(ctx context.Context, id, clienteID string) (*linhaFuncionario, error) {
	var linhas []linhaFuncionario
	caminho := "funcionarios?id=eq." + banco.Escapar(id) +
		"&cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=id,nome_completo,cpf,rg,status,funcao_id,unidade_id,funcoes(nome),unidades(nome)&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, fmt.Errorf("Não consegui carregar este funcionário.")
	}
	if len(linhas) == 0 {
		return nil, erroNaoEncontrado
	}
	return &linhas[0], nil
}

func responderErroFuncionario(w http.ResponseWriter, err error) {
	if err == erroNaoEncontrado {
		web.Falhar(w, http.StatusNotFound, err.Error())
		return
	}
	web.Falhar(w, http.StatusInternalServerError, err.Error())
}

// GET /funcionarios/{id}
func (m *Modulo) obter(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDados)
	if p == nil {
		return
	}
	linha, err := m.funcionarioDoCliente(r.Context(), r.PathValue("id"), p.ClienteID)
	if err != nil {
		responderErroFuncionario(w, err)
		return
	}

	completo, err := m.perm.Pode(r.Context(), p, RotinaDadoCompleto)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir a sua permissão.")
		return
	}
	if !completo {
		linha.CPF = mascararCPF(linha.CPF)
		if linha.RG != nil {
			m := mascararRG(*linha.RG)
			linha.RG = &m
		}
	}
	web.Responder(w, http.StatusOK, linha)
}

// ---------------------------------------------------------------------------
// criar
// ---------------------------------------------------------------------------

type pedidoCriar struct {
	NomeCompleto   string  `json:"nome_completo"`
	CPF            string  `json:"cpf"`
	RG             *string `json:"rg"`
	PISNIS         *string `json:"pis_nis"`
	CTPSNumero     *string `json:"ctps_numero"`
	FuncaoID       *string `json:"funcao_id"`
	UnidadeID      *string `json:"unidade_id"`
	DataNascimento *string `json:"data_nascimento"`
	DataAdmissao   *string `json:"data_admissao"`
}

func (m *Modulo) criar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDadoCompleto)
	if p == nil {
		return
	}

	var pedido pedidoCriar
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	pedido.NomeCompleto = strings.TrimSpace(pedido.NomeCompleto)
	pedido.CPF = strings.TrimSpace(pedido.CPF)

	if problema := validarCriar(pedido); problema != "" {
		web.Falhar(w, http.StatusBadRequest, problema)
		return
	}

	nova := map[string]any{
		"cliente_id":      p.ClienteID,
		"nome_completo":   pedido.NomeCompleto,
		"cpf":             pedido.CPF,
		"rg":              pedido.RG,
		"pis_nis":         pedido.PISNIS,
		"ctps_numero":     pedido.CTPSNumero,
		"funcao_id":       pedido.FuncaoID,
		"unidade_id":      pedido.UnidadeID,
		"data_nascimento": pedido.DataNascimento,
		"data_admissao":   pedido.DataAdmissao,
		"status":          "ativo",
	}

	var criados []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Inserir(r.Context(), "funcionarios", []map[string]any{nova}, &criados); err != nil {
		if banco.Duplicado(err) {
			web.Falhar(w, http.StatusConflict, "Já existe um funcionário com este CPF.")
			return
		}
		web.Falhar(w, http.StatusInternalServerError, "Não consegui cadastrar o funcionário.")
		return
	}
	if len(criados) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "O funcionário foi criado, mas o banco não devolveu qual.")
		return
	}
	id := criados[0].ID

	resposta := map[string]any{"id": id, "nome_completo": pedido.NomeCompleto}
	err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, id, "cadastrou", map[string]historico.Mudanca{
		"nome_completo": {De: nil, Para: pedido.NomeCompleto},
		"cpf":           {De: nil, Para: pedido.CPF},
	})
	if err != nil {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusCreated, resposta)
}

func validarCriar(p pedidoCriar) string {
	switch {
	case p.NomeCompleto == "":
		return "Informe o nome completo."
	case p.CPF == "":
		return "Informe o CPF."
	case len(p.CPF) < 11:
		return "O CPF parece incompleto."
	}
	return ""
}

// ---------------------------------------------------------------------------
// funções (cargos)
// ---------------------------------------------------------------------------

type funcao struct {
	ID       string `json:"id"`
	Nome     string `json:"nome"`
	CriadoEm string `json:"criado_em"`
}

// GET /funcoes — junto com o que cada uma exige, para a tela montar o
// checklist sem uma segunda viagem por função.
func (m *Modulo) listarFuncoes(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDados)
	if p == nil {
		return
	}
	caminho := "funcoes?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=id,nome,criado_em,funcao_documento_requisitos(tipo_documento_id,obrigatorio,tipos_documento(nome))" +
		"&order=nome.asc"

	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as funções.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"funcoes": linhas})
}

func (m *Modulo) criarFuncao(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDadoCompleto)
	if p == nil {
		return
	}
	var pedido struct {
		Nome string `json:"nome"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	pedido.Nome = strings.TrimSpace(pedido.Nome)
	if pedido.Nome == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o nome da função.")
		return
	}

	var criadas []funcao
	nova := map[string]any{"cliente_id": p.ClienteID, "nome": pedido.Nome}
	if err := m.bd.Inserir(r.Context(), "funcoes", []map[string]any{nova}, &criadas); err != nil {
		if banco.Duplicado(err) {
			web.Falhar(w, http.StatusConflict, "Já existe uma função com este nome.")
			return
		}
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar a função.")
		return
	}
	if len(criadas) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "A função foi criada, mas o banco não devolveu qual.")
		return
	}

	resposta := map[string]any{"id": criadas[0].ID, "nome": criadas[0].Nome}
	err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, criadas[0].ID, "criou_funcao", map[string]historico.Mudanca{
		"nome": {De: nil, Para: pedido.Nome},
	})
	if err != nil {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusCreated, resposta)
}

// PUT /funcoes/{id}/requisitos  {"requisitos": [{"tipo_documento_id": "...", "obrigatorio": true}]}
//
// A tela manda a lista COMPLETA do que a função exige; o motor substitui.
// Mesmo desenho de `gravarPermissoes` em acesso.go: mais simples de acertar
// do que "marque isto, desmarque aquilo".
func (m *Modulo) definirRequisitos(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDadoCompleto)
	if p == nil {
		return
	}
	funcaoID := r.PathValue("id")

	// Confere que a função é do mesmo cliente antes de mexer nela.
	var funcoesDoCliente []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Buscar(r.Context(), "funcoes?id=eq."+banco.Escapar(funcaoID)+"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id", &funcoesDoCliente); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir a função.")
		return
	}
	if len(funcoesDoCliente) == 0 {
		web.Falhar(w, http.StatusNotFound, "Função não encontrada.")
		return
	}

	var pedido struct {
		Requisitos []struct {
			TipoDocumentoID string `json:"tipo_documento_id"`
			Obrigatorio     bool   `json:"obrigatorio"`
		} `json:"requisitos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}

	if err := m.bd.Apagar(r.Context(), "funcao_documento_requisitos", "funcao_id=eq."+banco.Escapar(funcaoID)); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui limpar os requisitos anteriores.")
		return
	}

	linhas := make([]map[string]any, 0, len(pedido.Requisitos))
	for _, req := range pedido.Requisitos {
		if req.TipoDocumentoID == "" {
			continue
		}
		linhas = append(linhas, map[string]any{
			"funcao_id": funcaoID, "tipo_documento_id": req.TipoDocumentoID, "obrigatorio": req.Obrigatorio,
		})
	}
	if len(linhas) > 0 {
		if err := m.bd.Inserir(r.Context(), "funcao_documento_requisitos", linhas, nil); err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui gravar os requisitos.")
			return
		}
	}

	resposta := map[string]any{"ok": true, "total": len(linhas)}
	err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, funcaoID, "definiu_requisitos", map[string]historico.Mudanca{
		"total_de_documentos_exigidos": {De: nil, Para: len(linhas)},
	})
	if err != nil {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// catálogo de tipos de documento (só leitura por enquanto — nasce pela
// migração 044; cadastro de tipo novo pela tela vem quando alguém precisar)
// ---------------------------------------------------------------------------

func (m *Modulo) listarTiposDocumento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDados)
	if p == nil {
		return
	}
	caminho := "tipos_documento?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=id,nome,categoria,tem_validade,validade_dias_padrao,dias_alerta" +
		"&order=categoria.asc,nome.asc"
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar o catálogo de documentos.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"tipos_documento": linhas})
}
