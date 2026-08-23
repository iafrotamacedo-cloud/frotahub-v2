// rev 1 — usuários e logins
//
// A primeira função de verdade do baleryan, e ela existe aqui por um motivo técnico:
// criar login exige a API de administração do Supabase, que exige a chave de serviço.
// Essa chave não pode existir no navegador (CORE-09). Então esta rotina é do servidor,
// e só pode ser do servidor.
//
// QUEM PODE: só o builder (a segunda exceção do modelo de acesso). Assim ninguém
// consegue se promover nem criar outro dono para o sistema.
//
// SOBRE CRIAR EM DOIS LUGARES: um login vive em dois sítios — a `auth.users` do
// Supabase, que guarda a senha, e a `perfis`, que diz quem a pessoa é aqui dentro.
// Se o segundo passo falhar, o primeiro é desfeito. Meio login é pior que nenhum.
package usuarios

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// Tamanho mínimo de senha. O Supabase também tem o seu; este é o nosso piso.
const senhaMinima = 8

// Quantas linhas uma página traz. Nenhuma lista devolve tudo (CORE-10) — nem hoje,
// com três usuários, nem daqui a três anos.
const porPaginaPadrao = 25
const porPaginaMaximo = 100

type Modulo struct {
	cfg  *config.Config
	bd   *banco.Cliente
	seg  *seguranca.Servico
	http *http.Client
}

func Novo(cfg *config.Config, bd *banco.Cliente, seg *seguranca.Servico) *Modulo {
	return &Modulo{cfg: cfg, bd: bd, seg: seg, http: &http.Client{Timeout: 20 * time.Second}}
}

// Montar registra as rotas deste módulo. O arquivo principal não conhece nenhuma
// delas — ele só monta o módulo (P-13).
func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /usuarios", m.listar)
	mux.HandleFunc("POST /usuarios", m.criar)
	mux.HandleFunc("PATCH /usuarios/{id}", m.editar)
	mux.HandleFunc("POST /usuarios/{id}/senha", m.trocarSenha)
}

// quemEBuilder resolve as duas perguntas de toda rota daqui: quem é, e se pode.
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
// listar
// ---------------------------------------------------------------------------

type linhaUsuario struct {
	ID         string `json:"id"`
	Usuario    string `json:"usuario"`
	Nome       string `json:"nome"`
	Ativo      bool   `json:"ativo"`
	CriadoEm   string `json:"criado_em"`
	Categorias *struct {
		Codigo string `json:"codigo"`
		Nome   string `json:"nome"`
		Nivel  string `json:"nivel"`
	} `json:"categorias"`
}

// GET /usuarios?pagina=1&por_pagina=25&busca=igor
func (m *Modulo) listar(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}

	pagina := max(1, inteiro(r.URL.Query().Get("pagina"), 1))
	porPagina := inteiro(r.URL.Query().Get("por_pagina"), porPaginaPadrao)
	if porPagina < 1 || porPagina > porPaginaMaximo {
		porPagina = porPaginaPadrao
	}
	inicio := (pagina - 1) * porPagina

	// O filtro por cliente é escrito aqui TAMBÉM, apesar de a política do banco já
	// filtrar. Não é redundância inútil: a chave de serviço passa por cima das
	// políticas, então no servidor o filtro tem que ser explícito.
	caminho := "perfis?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&select=id,usuario,nome,ativo,criado_em,categorias(codigo,nome,nivel)" +
		"&order=usuario.asc" +
		"&limit=" + strconv.Itoa(porPagina) + "&offset=" + strconv.Itoa(inicio)

	if busca := strings.TrimSpace(r.URL.Query().Get("busca")); busca != "" {
		caminho += "&or=(usuario.ilike.*" + banco.Escapar(busca) + "*,nome.ilike.*" + banco.Escapar(busca) + "*)"
	}

	var linhas []linhaUsuario
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os usuários. Tente de novo em instantes.")
		return
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"usuarios":   linhas,
		"pagina":     pagina,
		"por_pagina": porPagina,
		// Uma linha a mais que o pedido significa que existe próxima página.
		"tem_mais": len(linhas) == porPagina,
	})
}

// ---------------------------------------------------------------------------
// criar
// ---------------------------------------------------------------------------

type pedidoCriar struct {
	Usuario     string `json:"usuario"`
	Nome        string `json:"nome"`
	Senha       string `json:"senha"`
	CategoriaID string `json:"categoria_id"`
}

type respostaAdmin struct {
	ID  string `json:"id"`
	Msg string `json:"msg"`
}

func (m *Modulo) criar(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}

	var pedido pedidoCriar
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}

	pedido.Usuario = strings.ToLower(strings.TrimSpace(pedido.Usuario))
	pedido.Nome = strings.TrimSpace(pedido.Nome)

	if problema := validar(pedido); problema != "" {
		web.Falhar(w, http.StatusBadRequest, problema)
		return
	}

	// A categoria tem que ser do MESMO cliente de quem está criando. Sem esta
	// checagem, um builder poderia colocar alguém numa categoria de outro cliente.
	if err := m.categoriaDoCliente(r.Context(), pedido.CategoriaID, p.ClienteID); err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}

	email := pedido.Usuario + "@" + m.cfg.DominioLogin

	// Passo 1: cria o login no Supabase (é aqui que a senha nasce e fica).
	uid, err := m.criarNoSupabase(r.Context(), email, pedido.Senha)
	if err != nil {
		web.Falhar(w, http.StatusBadGateway, err.Error())
		return
	}

	// Passo 2: cria o perfil. Se falhar, desfaz o passo 1 — meio login é pior
	// que nenhum, porque o e-mail fica ocupado e ninguém entende por quê.
	perfil := map[string]any{
		"id": uid, "usuario": pedido.Usuario, "nome": pedido.Nome,
		"cliente_id": p.ClienteID, "categoria_id": pedido.CategoriaID, "ativo": true,
	}
	if err := m.bd.Inserir(r.Context(), "perfis", []map[string]any{perfil}, nil); err != nil {
		m.apagarNoSupabase(context.WithoutCancel(r.Context()), uid)
		web.Falhar(w, http.StatusInternalServerError,
			"Não consegui criar o perfil, e desfiz o login para não deixar nada pela metade. Tente de novo.")
		return
	}

	web.Responder(w, http.StatusCreated, map[string]any{"id": uid, "usuario": pedido.Usuario})
}

func validar(p pedidoCriar) string {
	switch {
	case p.Usuario == "":
		return "Informe o usuário."
	case strings.ContainsAny(p.Usuario, "@ "):
		return "O usuário não pode ter espaço nem @ — é um nome curto, como \"joao\"."
	case len(p.Usuario) < 3:
		return "O usuário precisa de pelo menos 3 letras."
	case p.Nome == "":
		return "Informe o nome de quem vai usar este login."
	case len(p.Senha) < senhaMinima:
		return fmt.Sprintf("A senha precisa de pelo menos %d caracteres.", senhaMinima)
	case p.CategoriaID == "":
		return "Escolha a categoria deste login."
	}
	return ""
}

func (m *Modulo) categoriaDoCliente(ctx context.Context, categoriaID, clienteID string) error {
	var linhas []struct {
		ID string `json:"id"`
	}
	caminho := "categorias?id=eq." + banco.Escapar(categoriaID) +
		"&cliente_id=eq." + banco.Escapar(clienteID) + "&select=id&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return fmt.Errorf("não consegui conferir a categoria")
	}
	if len(linhas) == 0 {
		return fmt.Errorf("categoria não encontrada")
	}
	return nil
}

// criarNoSupabase usa a API de administração. É o único lugar do sistema que a toca.
func (m *Modulo) criarNoSupabase(ctx context.Context, email, senha string) (string, error) {
	corpo := map[string]any{
		"email":    email,
		"password": senha,
		// Confirmado na hora: não existe e-mail de verdade para receber a
		// confirmação, já que o domínio é interno.
		"email_confirm": true,
	}
	b, _ := json.Marshal(corpo)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.cfg.Supabase.Auth()+"/admin/users", strings.NewReader(string(b)))
	if err != nil {
		return "", err
	}
	req.Header.Set("apikey", m.cfg.Supabase.ChaveServico)
	req.Header.Set("Authorization", "Bearer "+m.cfg.Supabase.ChaveServico)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("não consegui falar com o serviço de login")
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		if strings.Contains(strings.ToLower(string(bruto)), "already been registered") {
			return "", fmt.Errorf("já existe um login com este usuário")
		}
		return "", fmt.Errorf("o serviço de login recusou: %s", strings.TrimSpace(string(bruto)))
	}

	var out respostaAdmin
	if err := json.Unmarshal(bruto, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("o serviço de login respondeu de um jeito que eu não entendi")
	}
	return out.ID, nil
}

func (m *Modulo) apagarNoSupabase(ctx context.Context, uid string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		m.cfg.Supabase.Auth()+"/admin/users/"+uid, nil)
	if err != nil {
		return
	}
	req.Header.Set("apikey", m.cfg.Supabase.ChaveServico)
	req.Header.Set("Authorization", "Bearer "+m.cfg.Supabase.ChaveServico)
	if resp, err := m.http.Do(req); err == nil {
		resp.Body.Close()
	}
}

// ---------------------------------------------------------------------------
// editar e trocar senha
// ---------------------------------------------------------------------------

type pedidoEditar struct {
	Nome        *string `json:"nome"`
	CategoriaID *string `json:"categoria_id"`
	Ativo       *bool   `json:"ativo"`
}

func (m *Modulo) editar(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	alvo := r.PathValue("id")

	var pedido pedidoEditar
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}

	// Ninguém se desativa. Sem isto, um clique distraído tranca o dono para fora.
	if pedido.Ativo != nil && !*pedido.Ativo && alvo == p.UserID {
		web.Falhar(w, http.StatusBadRequest, "Você não pode desativar o seu próprio login.")
		return
	}

	campos := map[string]any{}
	if pedido.Nome != nil {
		nome := strings.TrimSpace(*pedido.Nome)
		if nome == "" {
			web.Falhar(w, http.StatusBadRequest, "O nome não pode ficar vazio.")
			return
		}
		campos["nome"] = nome
	}
	if pedido.CategoriaID != nil {
		if err := m.categoriaDoCliente(r.Context(), *pedido.CategoriaID, p.ClienteID); err != nil {
			web.Falhar(w, http.StatusBadRequest, err.Error())
			return
		}
		campos["categoria_id"] = *pedido.CategoriaID
	}
	if pedido.Ativo != nil {
		campos["ativo"] = *pedido.Ativo
	}
	if len(campos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Não veio nada para alterar.")
		return
	}

	filtro := "id=eq." + banco.Escapar(alvo) + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "perfis", filtro, campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui salvar a alteração.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

func (m *Modulo) trocarSenha(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	alvo := r.PathValue("id")

	var pedido struct {
		Senha string `json:"senha"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	if len(pedido.Senha) < senhaMinima {
		web.Falhar(w, http.StatusBadRequest,
			fmt.Sprintf("A senha precisa de pelo menos %d caracteres.", senhaMinima))
		return
	}

	// Confere que o alvo é do mesmo cliente ANTES de tocar na senha.
	var linhas []struct {
		ID string `json:"id"`
	}
	caminho := "perfis?id=eq." + banco.Escapar(alvo) + "&cliente_id=eq." + banco.Escapar(p.ClienteID) + "&select=id&limit=1"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil || len(linhas) == 0 {
		web.Falhar(w, http.StatusNotFound, "Login não encontrado.")
		return
	}

	corpo, _ := json.Marshal(map[string]any{"password": pedido.Senha})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPut,
		m.cfg.Supabase.Auth()+"/admin/users/"+alvo, strings.NewReader(string(corpo)))
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar o pedido.")
		return
	}
	req.Header.Set("apikey", m.cfg.Supabase.ChaveServico)
	req.Header.Set("Authorization", "Bearer "+m.cfg.Supabase.ChaveServico)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.http.Do(req)
	if err != nil {
		web.Falhar(w, http.StatusBadGateway, "Não consegui falar com o serviço de login.")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bruto, _ := io.ReadAll(resp.Body)
		web.Falhar(w, http.StatusBadGateway, "O serviço de login recusou: "+strings.TrimSpace(string(bruto)))
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------

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
