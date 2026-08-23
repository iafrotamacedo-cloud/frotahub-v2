// rev 1 — categorias e matriz de permissões
//
// Este módulo é o painel de controle do acesso: cria os grupos e marca, para cada
// grupo, o que ele alcança.
//
// QUEM PODE: só o builder. É a mesma exceção que vale para logins — quem pode
// mexer na matriz pode se dar qualquer acesso, então mexer na matriz é do dono.
//
// A REGRA QUE ESTE MÓDULO SERVE
//
//	Cada login pertence a uma categoria. Cada categoria tem uma lista de rotinas
//	liberadas. Duas exceções, e só duas: o builder passa sempre, e só o builder
//	mexe em login e em permissão.
//
// O QUE ESTE MÓDULO NÃO FAZ
//
//	Não apaga categoria (CORE-05) e não cadastra rotina. O catálogo de rotinas se
//	preenche sozinho: cada módulo registra as suas na própria migração, quando é
//	construído. Enquanto não houver módulo de negócio, a matriz abre vazia — e
//	isso é o desenho funcionando, não uma tela quebrada.
package acesso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// O nome deste módulo dentro da tabela `historico`, que é compartilhada.
const moduloHistorico = "acesso"

const porPaginaPadrao = 25
const porPaginaMaximo = 100

// Os níveis que a TELA pode escolher.
//
// `builder` está fora de propósito. A categoria builder é única, nasce protegida
// pela migração e é a trava anti-tranca do sistema inteiro. Se ela virasse opção
// de formulário, um clique distraído criaria um segundo dono — e o segundo dono
// pode desativar o primeiro.
var niveisPermitidos = map[string]bool{"ceo": true, "gerente": true, "comum": true}

type Modulo struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	hist *historico.Servico
}

func Novo(bd *banco.Cliente, seg *seguranca.Servico, hist *historico.Servico) *Modulo {
	return &Modulo{bd: bd, seg: seg, hist: hist}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /categorias", m.listar)
	mux.HandleFunc("POST /categorias", m.criar)
	mux.HandleFunc("PATCH /categorias/{id}", m.editar)
	mux.HandleFunc("GET /categorias/{id}/permissoes", m.lerPermissoes)
	mux.HandleFunc("PUT /categorias/{id}/permissoes", m.gravarPermissoes)
	mux.HandleFunc("GET /categorias/{id}/historico", m.verHistorico)
}

func (m *Modulo) quemEBuilder(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := permissao.ExigeBuilder(p); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	return p
}

// ---------------------------------------------------------------------------
// a categoria como ela está hoje
// ---------------------------------------------------------------------------

type categoria struct {
	ID        string `json:"id"`
	Codigo    string `json:"codigo"`
	Nome      string `json:"nome"`
	Nivel     string `json:"nivel"`
	Protegida bool   `json:"protegida"`
	Ativo     bool   `json:"ativo"`
	CriadoEm  string `json:"criado_em"`
}

func (m *Modulo) categoriaDoCliente(ctx context.Context, id, clienteID string) (*categoria, error) {
	var linhas []categoria
	caminho := "categorias?id=eq." + banco.Escapar(id) +
		"&cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=id,codigo,nome,nivel,protegida,ativo,criado_em&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, fmt.Errorf("Não consegui carregar a categoria.")
	}
	if len(linhas) == 0 {
		return nil, erroNaoEncontrada
	}
	return &linhas[0], nil
}

var erroNaoEncontrada = fmt.Errorf("Categoria não encontrada.")

func (m *Modulo) acharOuFalhar(w http.ResponseWriter, r *http.Request, p *seguranca.Principal) *categoria {
	cat, err := m.categoriaDoCliente(r.Context(), r.PathValue("id"), p.ClienteID)
	if err != nil {
		if err == erroNaoEncontrada {
			web.Falhar(w, http.StatusNotFound, err.Error())
		} else {
			web.Falhar(w, http.StatusInternalServerError, err.Error())
		}
		return nil
	}
	return cat
}

// ---------------------------------------------------------------------------
// listar
// ---------------------------------------------------------------------------

// GET /categorias?incluir_inativas=1
func (m *Modulo) listar(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}

	caminho := "categorias?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=id,codigo,nome,nivel,protegida,ativo,criado_em&order=nome.asc"

	// O formulário de criar login só deve oferecer categoria em circulação.
	// Quem quiser ver as arquivadas pede de propósito.
	if r.URL.Query().Get("incluir_inativas") == "" {
		caminho += "&ativo=is.true"
	}

	linhas := []categoria{}
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as categorias.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"categorias": linhas})
}

// ---------------------------------------------------------------------------
// criar
// ---------------------------------------------------------------------------

type pedidoCategoria struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
	Nivel  string `json:"nivel"`
}

func (m *Modulo) criar(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}

	var pedido pedidoCategoria
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	pedido.Codigo = strings.ToLower(strings.TrimSpace(pedido.Codigo))
	pedido.Nome = strings.TrimSpace(pedido.Nome)
	pedido.Nivel = strings.ToLower(strings.TrimSpace(pedido.Nivel))

	if problema := validar(pedido); problema != "" {
		web.Falhar(w, http.StatusBadRequest, problema)
		return
	}

	nova := map[string]any{
		"cliente_id": p.ClienteID,
		"codigo":     pedido.Codigo,
		"nome":       pedido.Nome,
		"nivel":      pedido.Nivel,
		"protegida":  false,
		"ativo":      true,
	}

	var criadas []categoria
	if err := m.bd.Inserir(r.Context(), "categorias", []map[string]any{nova}, &criadas); err != nil {
		if banco.Duplicado(err) {
			web.Falhar(w, http.StatusConflict, "Já existe uma categoria com este código.")
			return
		}
		web.Falhar(w, http.StatusInternalServerError, "Não consegui criar a categoria.")
		return
	}
	if len(criadas) == 0 {
		web.Falhar(w, http.StatusInternalServerError, "A categoria foi criada, mas o banco não devolveu qual.")
		return
	}
	cat := criadas[0]

	resposta := map[string]any{"id": cat.ID, "codigo": cat.Codigo}
	err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, cat.ID, "criou", map[string]historico.Mudanca{
		"codigo": {De: nil, Para: cat.Codigo},
		"nome":   {De: nil, Para: cat.Nome},
		"nivel":  {De: nil, Para: cat.Nivel},
	})
	if err != nil {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusCreated, resposta)
}

func validar(p pedidoCategoria) string {
	switch {
	case p.Codigo == "":
		return "Informe o código da categoria."
	case strings.ContainsAny(p.Codigo, " @/"):
		return "O código não pode ter espaço nem barra — é um nome curto, como \"administrativo\"."
	case len(p.Codigo) < 3:
		return "O código precisa de pelo menos 3 letras."
	case p.Codigo == "builder":
		return "\"builder\" é reservado para a categoria do dono do sistema."
	case p.Nome == "":
		return "Informe o nome que aparece na tela."
	case !niveisPermitidos[p.Nivel]:
		return "Escolha um nível: comum, gerente ou ceo."
	}
	return ""
}

// ---------------------------------------------------------------------------
// editar
// ---------------------------------------------------------------------------

type pedidoEditar struct {
	Nome  *string `json:"nome"`
	Nivel *string `json:"nivel"`
	Ativo *bool   `json:"ativo"`
}

func (m *Modulo) editar(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	atual := m.acharOuFalhar(w, r, p)
	if atual == nil {
		return
	}

	// A categoria protegida não se edita. Ela é a trava anti-tranca: se desse para
	// renomear, rebaixar ou desativar, daria para trancar o dono para fora.
	if atual.Protegida {
		web.Falhar(w, http.StatusBadRequest,
			"Esta é a categoria do dono do sistema. Ela não se edita nem sai de circulação, de propósito.")
		return
	}

	var pedido pedidoEditar
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}

	campos := map[string]any{}
	mudancasDados := map[string]historico.Mudanca{}

	if pedido.Nome != nil {
		nome := strings.TrimSpace(*pedido.Nome)
		if nome == "" {
			web.Falhar(w, http.StatusBadRequest, "O nome não pode ficar vazio.")
			return
		}
		if nome != atual.Nome {
			campos["nome"] = nome
			mudancasDados["nome"] = historico.Mudanca{De: atual.Nome, Para: nome}
		}
	}

	if pedido.Nivel != nil {
		nivel := strings.ToLower(strings.TrimSpace(*pedido.Nivel))
		if !niveisPermitidos[nivel] {
			web.Falhar(w, http.StatusBadRequest, "Escolha um nível: comum, gerente ou ceo.")
			return
		}
		if nivel != atual.Nivel {
			campos["nivel"] = nivel
			mudancasDados["nivel"] = historico.Mudanca{De: atual.Nivel, Para: nivel}
		}
	}

	situacaoMudou := pedido.Ativo != nil && *pedido.Ativo != atual.Ativo
	if situacaoMudou && !*pedido.Ativo {
		// Tirar de circulação uma categoria que ainda tem gente dentro deixaria
		// essas pessoas num grupo que a tela não mostra mais. Melhor recusar e
		// dizer quantas são do que criar um problema que ninguém vê.
		quantos, err := m.loginsAtivos(r.Context(), atual.ID, p.ClienteID)
		if err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir quem está nesta categoria.")
			return
		}
		if quantos > 0 {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf(
				"Esta categoria ainda tem %d login(s) ativo(s). Mova essas pessoas para outra categoria antes de tirá-la de circulação.", quantos))
			return
		}
	}
	if situacaoMudou {
		campos["ativo"] = *pedido.Ativo
	}

	if len(campos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Não veio nada para alterar.")
		return
	}

	filtro := "id=eq." + banco.Escapar(atual.ID) + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "categorias", filtro, campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui salvar a alteração.")
		return
	}

	ctx := semCancelar(r)
	falhou := false
	if len(mudancasDados) > 0 {
		if err := m.hist.Registrar(ctx, p, moduloHistorico, atual.ID, "alterou", mudancasDados); err != nil {
			falhou = true
		}
	}
	if situacaoMudou {
		acao := "desativou"
		if *pedido.Ativo {
			acao = "reativou"
		}
		if err := m.hist.Registrar(ctx, p, moduloHistorico, atual.ID, acao, map[string]historico.Mudanca{
			"ativo": {De: atual.Ativo, Para: *pedido.Ativo},
		}); err != nil {
			falhou = true
		}
	}

	resposta := map[string]any{"ok": true}
	if falhou {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusOK, resposta)
}

func (m *Modulo) loginsAtivos(ctx context.Context, categoriaID, clienteID string) (int, error) {
	var linhas []struct {
		ID string `json:"id"`
	}
	caminho := "perfis?categoria_id=eq." + banco.Escapar(categoriaID) +
		"&cliente_id=eq." + banco.Escapar(clienteID) + "&ativo=is.true&select=id"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return 0, err
	}
	return len(linhas), nil
}

// ---------------------------------------------------------------------------
// a matriz
// ---------------------------------------------------------------------------

type rotina struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
	Modulo string `json:"modulo"`
	Ordem  int    `json:"ordem"`
}

type linhaMatriz struct {
	Rotina string `json:"rotina"`
	Pode   bool   `json:"pode"`
}

// GET /categorias/{id}/permissoes
//
// Devolve o catálogo INTEIRO mais o que esta categoria alcança. A tela precisa
// dos dois: as rotinas desmarcadas são metade da informação.
func (m *Modulo) lerPermissoes(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	cat := m.acharOuFalhar(w, r, p)
	if cat == nil {
		return
	}

	catalogo := []rotina{}
	if err := m.bd.Buscar(r.Context(), "rotinas?select=codigo,nome,modulo,ordem&order=modulo.asc,ordem.asc,nome.asc", &catalogo); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar o catálogo de rotinas.")
		return
	}

	marcadas, err := m.marcadas(r.Context(), cat.ID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as permissões desta categoria.")
		return
	}
	permitidas := []string{}
	for codigo := range marcadas {
		permitidas = append(permitidas, codigo)
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"categoria":  cat,
		"rotinas":    catalogo,
		"permitidas": permitidas,
		// O builder não passa pela matriz — a tela precisa saber disso para
		// explicar por que o quadro está desabilitado em vez de parecer quebrado.
		"ignora_matriz": cat.Protegida,
	})
}

func (m *Modulo) marcadas(ctx context.Context, categoriaID string) (map[string]bool, error) {
	linhas := []linhaMatriz{}
	caminho := "categoria_permissoes?categoria_id=eq." + banco.Escapar(categoriaID) + "&select=rotina,pode"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	fora := map[string]bool{}
	for _, l := range linhas {
		if l.Pode {
			fora[l.Rotina] = true
		}
	}
	return fora, nil
}

// PUT /categorias/{id}/permissoes  {"rotinas": ["CONFIG_USUARIOS", ...]}
//
// A tela manda a lista COMPLETA do que deve ficar marcado; o motor calcula a
// diferença. É mais simples de acertar do que mandar "marque isto, desmarque
// aquilo": se dois builders salvarem quase junto, o último salva um estado
// inteiro coerente, em vez de metade de dois estados.
func (m *Modulo) gravarPermissoes(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	cat := m.acharOuFalhar(w, r, p)
	if cat == nil {
		return
	}

	if cat.Protegida {
		web.Falhar(w, http.StatusBadRequest,
			"A categoria do dono do sistema não passa pela matriz: ela alcança tudo por construção. Marcar rotinas aqui não teria efeito.")
		return
	}

	var pedido struct {
		Rotinas []string `json:"rotinas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}

	desejadas := map[string]bool{}
	for _, c := range pedido.Rotinas {
		c = strings.TrimSpace(c)
		if c != "" {
			desejadas[c] = true
		}
	}

	// Nenhuma rotina inventada entra. A chave estrangeira do banco também barra,
	// mas o erro dela não explica nada para quem está na tela.
	if len(desejadas) > 0 {
		catalogo := []rotina{}
		if err := m.bd.Buscar(r.Context(), "rotinas?select=codigo", &catalogo); err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir o catálogo de rotinas.")
			return
		}
		existe := map[string]bool{}
		for _, rt := range catalogo {
			existe[rt.Codigo] = true
		}
		for c := range desejadas {
			if !existe[c] {
				web.Falhar(w, http.StatusBadRequest, "Rotina desconhecida: "+c)
				return
			}
		}
	}

	atuais, err := m.marcadas(r.Context(), cat.ID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as permissões atuais.")
		return
	}

	mudancas := map[string]historico.Mudanca{}
	aMarcar := []map[string]any{}
	aDesmarcar := []string{}

	for c := range desejadas {
		if !atuais[c] {
			aMarcar = append(aMarcar, map[string]any{"categoria_id": cat.ID, "rotina": c, "pode": true})
			mudancas[c] = historico.Mudanca{De: false, Para: true}
		}
	}
	for c := range atuais {
		if !desejadas[c] {
			aDesmarcar = append(aDesmarcar, c)
			mudancas[c] = historico.Mudanca{De: true, Para: false}
		}
	}

	if len(mudancas) == 0 {
		web.Responder(w, http.StatusOK, map[string]any{"ok": true, "sem_mudanca": true})
		return
	}

	if len(aMarcar) > 0 {
		if err := m.bd.Gravar(r.Context(), "categoria_permissoes", aMarcar); err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui liberar as rotinas novas.")
			return
		}
	}
	if len(aDesmarcar) > 0 {
		// Desmarcar é gravar `pode = false`, não apagar a linha (CORE-05). A linha
		// que fica é o registro de que aquela rotina já esteve liberada.
		filtro := "categoria_id=eq." + banco.Escapar(cat.ID) + "&rotina=in.(" + listaPara(aDesmarcar) + ")"
		if err := m.bd.Atualizar(r.Context(), "categoria_permissoes", filtro, map[string]any{"pode": false}); err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui retirar as rotinas.")
			return
		}
	}

	resposta := map[string]any{"ok": true, "liberadas": len(aMarcar), "retiradas": len(aDesmarcar)}
	// MOD-ACESSO-01: mexer em permissão deixa rastro. Uma linha por salvamento,
	// listando só o que mudou — não trinta linhas quando alguém marca trinta.
	if err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, cat.ID, "alterou_permissoes", mudancas); err != nil {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusOK, resposta)
}

// listaPara monta o `in.(a,b,c)` do PostgREST com cada item entre aspas, para um
// código com vírgula ou parêntese não quebrar o filtro.
func listaPara(itens []string) string {
	partes := make([]string, 0, len(itens))
	for _, i := range itens {
		partes = append(partes, banco.Escapar("\""+i+"\""))
	}
	return strings.Join(partes, ",")
}

// ---------------------------------------------------------------------------
// histórico
// ---------------------------------------------------------------------------

func (m *Modulo) verHistorico(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	cat := m.acharOuFalhar(w, r, p)
	if cat == nil {
		return
	}

	pagina, porPagina, inicio := paginacao(r)
	linhas, err := m.hist.Listar(r.Context(), p.ClienteID, moduloHistorico, cat.ID, porPagina, inicio)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar o histórico.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"historico":  linhas,
		"pagina":     pagina,
		"por_pagina": porPagina,
		"tem_mais":   len(linhas) == porPagina,
	})
}

// ---------------------------------------------------------------------------

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
