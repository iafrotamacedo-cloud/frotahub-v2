// rev 2 — usuários e logins
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
//
// O QUE MUDOU NA REVISÃO 2 --------------------------------------------------
// Entrou a MOD-USUARIOS-01: criar, alterar, mudar situação e trocar senha passam
// a gravar histórico. Junto vieram duas consequências:
//
//  1. Alterar agora LÊ o registro antes de gravar. Precisa disso para saber o
//     "de" de cada campo — e, de brinde, corrigiu um defeito silencioso: antes,
//     alterar um id que não existe respondia "ok", porque o PATCH sem linha
//     correspondente não é erro para o banco.
//
//  2. Nasceu a rota GET /usuarios/{id}/historico.
package usuarios

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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

// O nome deste módulo dentro da tabela `historico`, que é compartilhada.
const moduloHistorico = "usuarios"

var erroNaoEncontrado = errors.New("Login não encontrado.")

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
	mux.HandleFunc("GET /usuarios/{id}/historico", m.historico)
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
// MOD-USUARIOS-01 — histórico
//
// Toda criação, alteração, mudança de situação e troca de senha grava uma linha.
// Senha nunca entra no conteúdo — nem cifrada, nem em pedaço.
// ---------------------------------------------------------------------------

// mudanca é o par de/para de um campo. Os dois lados aparecem sempre, inclusive
// na criação (onde o "de" é nulo), para a tela ter um formato só para desenhar.
type mudanca struct {
	De   any `json:"de"`
	Para any `json:"para"`
}

// A frase que vai para a tela quando a alteração deu certo mas o histórico não.
// Não é erro de HTTP de propósito: a operação ACONTECEU, e responder "deu errado"
// faria o usuário tentar de novo, o que criaria um segundo login ou desfaria o
// que ele acabou de fazer. Mentir sobre o resultado é pior do que avisar.
const avisoHistorico = "Feito, mas o histórico NÃO foi gravado. Anote o que você fez e avise o responsável pelo sistema."

// registrar grava uma linha de histórico.
//
// Devolve o erro em vez de engoli-lo: quem chama decide o que dizer na tela, mas
// ninguém tem licença para fingir que gravou.
func (m *Modulo) registrar(ctx context.Context, p *seguranca.Principal, registroID, acao string, mudancas map[string]mudanca) error {
	linha := map[string]any{
		"cliente_id":    p.ClienteID,
		"modulo":        moduloHistorico,
		"registro_id":   registroID,
		"acao":          acao,
		"autor_id":      p.UserID,
		"autor_usuario": p.Usuario,
	}
	if len(mudancas) > 0 {
		linha["mudancas"] = mudancas
	}

	if err := m.bd.Inserir(ctx, "historico", []map[string]any{linha}, nil); err != nil {
		// Segunda trilha. Se o banco recusou, pelo menos o console do servidor
		// guarda o que aconteceu — é de onde a informação será resgatada.
		log.Printf("[historico] FALHOU: %s %s/%s por %s: %v", acao, moduloHistorico, registroID, p.Usuario, err)
		return err
	}
	return nil
}

// semCancelar desliga o histórico do tempo de vida da requisição. Se o navegador
// desistir no meio, a alteração já foi feita — o registro dela não pode ir junto.
func semCancelar(r *http.Request) context.Context {
	return context.WithoutCancel(r.Context())
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

	pagina, porPagina, inicio := paginacao(r)

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
		// Uma página cheia significa que pode existir próxima.
		"tem_mais": len(linhas) == porPagina,
	})
}

// ---------------------------------------------------------------------------
// ler o registro atual
//
// Serve a três coisas: conferir que o login existe, conferir que é do mesmo
// cliente, e saber o "de" de cada campo que vai mudar.
// ---------------------------------------------------------------------------

type perfilAtual struct {
	ID          string `json:"id"`
	Usuario     string `json:"usuario"`
	Nome        string `json:"nome"`
	CategoriaID string `json:"categoria_id"`
	Ativo       bool   `json:"ativo"`
	Categorias  *struct {
		Nome string `json:"nome"`
	} `json:"categorias"`
}

func (a *perfilAtual) categoriaNome() string {
	if a.Categorias == nil {
		return ""
	}
	return a.Categorias.Nome
}

func (m *Modulo) perfilDoCliente(ctx context.Context, id, clienteID string) (*perfilAtual, error) {
	var linhas []perfilAtual
	caminho := "perfis?id=eq." + banco.Escapar(id) +
		"&cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=id,usuario,nome,categoria_id,ativo,categorias(nome)&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, fmt.Errorf("Não consegui carregar este login.")
	}
	if len(linhas) == 0 {
		return nil, erroNaoEncontrado
	}
	return &linhas[0], nil
}

func responderErroPerfil(w http.ResponseWriter, err error) {
	if errors.Is(err, erroNaoEncontrado) {
		web.Falhar(w, http.StatusNotFound, err.Error())
		return
	}
	web.Falhar(w, http.StatusInternalServerError, err.Error())
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
	categoriaNome, err := m.categoriaDoCliente(r.Context(), pedido.CategoriaID, p.ClienteID)
	if err != nil {
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
		m.apagarNoSupabase(semCancelar(r), uid)
		web.Falhar(w, http.StatusInternalServerError,
			"Não consegui criar o perfil, e desfiz o login para não deixar nada pela metade. Tente de novo.")
		return
	}

	// Passo 3: o histórico. Note que a categoria entra pelo NOME, não pelo id:
	// histórico é para ser lido por gente, e daqui a dois anos o id não diz nada.
	// A senha, obviamente, não entra de forma nenhuma.
	resposta := map[string]any{"id": uid, "usuario": pedido.Usuario}
	err = m.registrar(semCancelar(r), p, uid, "criou", map[string]mudanca{
		"usuario":   {De: nil, Para: pedido.Usuario},
		"nome":      {De: nil, Para: pedido.Nome},
		"categoria": {De: nil, Para: categoriaNome},
	})
	if err != nil {
		resposta["aviso"] = avisoHistorico
	}

	web.Responder(w, http.StatusCreated, resposta)
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

// categoriaDoCliente confere que a categoria existe e é do cliente, e devolve o
// nome dela — que é o que vai para o histórico.
func (m *Modulo) categoriaDoCliente(ctx context.Context, categoriaID, clienteID string) (string, error) {
	var linhas []struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	}
	caminho := "categorias?id=eq." + banco.Escapar(categoriaID) +
		"&cliente_id=eq." + banco.Escapar(clienteID) + "&select=id,nome&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return "", fmt.Errorf("Não consegui conferir a categoria.")
	}
	if len(linhas) == 0 {
		return "", fmt.Errorf("Categoria não encontrada.")
	}
	return linhas[0].Nome, nil
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
// editar
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

	// Lê o estado atual ANTES de mexer. É o que dá o "de" do histórico — e é
	// também o que faz um id inexistente virar 404 em vez de um "ok" mentiroso.
	atual, err := m.perfilDoCliente(r.Context(), alvo, p.ClienteID)
	if err != nil {
		responderErroPerfil(w, err)
		return
	}

	campos := map[string]any{}
	mudancasDados := map[string]mudanca{}

	if pedido.Nome != nil {
		nome := strings.TrimSpace(*pedido.Nome)
		if nome == "" {
			web.Falhar(w, http.StatusBadRequest, "O nome não pode ficar vazio.")
			return
		}
		// Campo reenviado igual não é alteração. Não vira gravação nem linha de
		// histórico: rastro cheio de "mudou de X para X" é rastro que ninguém lê.
		if nome != atual.Nome {
			campos["nome"] = nome
			mudancasDados["nome"] = mudanca{De: atual.Nome, Para: nome}
		}
	}

	if pedido.CategoriaID != nil && *pedido.CategoriaID != atual.CategoriaID {
		nomeNovo, err := m.categoriaDoCliente(r.Context(), *pedido.CategoriaID, p.ClienteID)
		if err != nil {
			web.Falhar(w, http.StatusBadRequest, err.Error())
			return
		}
		campos["categoria_id"] = *pedido.CategoriaID
		mudancasDados["categoria"] = mudanca{De: atual.categoriaNome(), Para: nomeNovo}
	}

	situacaoMudou := pedido.Ativo != nil && *pedido.Ativo != atual.Ativo
	if situacaoMudou {
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

	// Duas linhas quando as duas coisas mudam na mesma vez, e não uma linha com
	// tudo dentro. Assim cada linha tem UM significado: "alterou" é mexer em
	// dados, "desativou" é tirar do ar. Um rastro que mistura os dois obriga
	// quem lê a abrir o conteúdo para descobrir o que de fato aconteceu.
	ctx := semCancelar(r)
	falhou := false

	if len(mudancasDados) > 0 {
		if err := m.registrar(ctx, p, alvo, "alterou", mudancasDados); err != nil {
			falhou = true
		}
	}
	if situacaoMudou {
		acao := "desativou"
		if *pedido.Ativo {
			acao = "reativou"
		}
		if err := m.registrar(ctx, p, alvo, acao, map[string]mudanca{
			"ativo": {De: atual.Ativo, Para: *pedido.Ativo},
		}); err != nil {
			falhou = true
		}
	}

	resposta := map[string]any{"ok": true}
	if falhou {
		resposta["aviso"] = avisoHistorico
	}
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// trocar senha
// ---------------------------------------------------------------------------

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

	// Confere que o alvo existe e é do mesmo cliente ANTES de tocar na senha.
	if _, err := m.perfilDoCliente(r.Context(), alvo, p.ClienteID); err != nil {
		responderErroPerfil(w, err)
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

	// Registra o FATO, e só o fato. Nenhum pedaço da senha, nem antiga nem nova,
	// nem tamanho, nem resumo criptográfico. Por isso `mudancas` vai vazio.
	resposta := map[string]any{"ok": true}
	if err := m.registrar(semCancelar(r), p, alvo, "trocou_senha", nil); err != nil {
		resposta["aviso"] = avisoHistorico
	}
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// histórico
// ---------------------------------------------------------------------------

type linhaHistorico struct {
	ID           int64           `json:"id"`
	Acao         string          `json:"acao"`
	AutorUsuario string          `json:"autor_usuario"`
	Quando       string          `json:"quando"`
	Mudancas     json.RawMessage `json:"mudancas"`
}

// GET /usuarios/{id}/historico?pagina=1&por_pagina=25
func (m *Modulo) historico(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	alvo := r.PathValue("id")

	// O histórico de um login só se abre para quem pode ver aquele login.
	if _, err := m.perfilDoCliente(r.Context(), alvo, p.ClienteID); err != nil {
		responderErroPerfil(w, err)
		return
	}

	pagina, porPagina, inicio := paginacao(r)

	// A ordem das condições acompanha o índice `historico_por_registro`.
	// O desempate por id existe porque dois eventos da mesma alteração nascem
	// com o mesmo carimbo de tempo — sem ele, a ordem entre eles seria sorteio.
	caminho := "historico?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&modulo=eq." + moduloHistorico +
		"&registro_id=eq." + banco.Escapar(alvo) +
		"&select=id,acao,autor_usuario,quando,mudancas" +
		"&order=quando.desc,id.desc" +
		"&limit=" + strconv.Itoa(porPagina) + "&offset=" + strconv.Itoa(inicio)

	var linhas []linhaHistorico
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
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

// paginacao lê pagina/por_pagina da URL e devolve os três números já corrigidos.
// Está num lugar só porque toda lista do sistema pagina do mesmo jeito (CORE-10).
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
