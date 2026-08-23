// rev 2 — quem é você
//
// Esta é a ÚNICA porta de entrada do motor (P-14). Toda requisição passa por aqui e
// sai com um Principal — ou não passa.
//
// Dois tipos de visitante, e nada além disso:
//
//	usuario   uma pessoa logada, com token do Supabase no cabeçalho Authorization
//	robo      uma automação, com o segredo no cabeçalho X-Robot-Key
//
// O que cada um PODE fazer não se decide aqui — isso é do pacote `permissao`.
// Aqui só se responde "quem é".
package seguranca

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

type Tipo string

const (
	TipoUsuario Tipo = "usuario"
	TipoRobo    Tipo = "robo"
)

var (
	ErrSemCredencial = errors.New("é preciso entrar no sistema")
	ErrCredencialMa  = errors.New("sessão inválida ou expirada")
	ErrInativo       = errors.New("este login está desativado")
)

// Principal é quem está batendo na porta.
type Principal struct {
	Tipo          Tipo   `json:"tipo"`
	UserID        string `json:"user_id,omitempty"`
	Usuario       string `json:"usuario,omitempty"`
	Nome          string `json:"nome,omitempty"`
	ClienteID     string `json:"cliente_id,omitempty"`
	ClienteNome   string `json:"cliente_nome,omitempty"`
	CategoriaID   string `json:"categoria_id,omitempty"`
	CategoriaNome string `json:"categoria_nome,omitempty"`
	Nivel         string `json:"nivel,omitempty"`
	Ativo         bool   `json:"ativo"`
}

// Builder é a primeira das duas exceções do sistema de acesso: o builder passa
// sempre, aconteça o que acontecer com a matriz de permissões. É a garantia de
// que uma configuração errada nunca tranca o dono para fora do próprio sistema.
func (p *Principal) Builder() bool { return p != nil && p.Nivel == "builder" }

type Servico struct {
	cfg  *config.Config
	bd   *banco.Cliente
	http *http.Client
}

func Novo(cfg *config.Config, bd *banco.Cliente) *Servico {
	return &Servico{cfg: cfg, bd: bd, http: &http.Client{Timeout: 15 * time.Second}}
}

// ---------------------------------------------------------------------------

type usuarioSupabase struct {
	ID string `json:"id"`
}

// perfilBanco espelha exatamente o que a consulta abaixo devolve.
type perfilBanco struct {
	ID          string `json:"id"`
	Usuario     string `json:"usuario"`
	Nome        string `json:"nome"`
	Ativo       bool   `json:"ativo"`
	ClienteID   string `json:"cliente_id"`
	CategoriaID string `json:"categoria_id"`
	Clientes    *struct {
		Nome string `json:"nome"`
	} `json:"clientes"`
	Categorias *struct {
		Nome  string `json:"nome"`
		Nivel string `json:"nivel"`
	} `json:"categorias"`
}

// DaRequisicao descobre quem está chamando. É o começo de toda rota protegida.
func (s *Servico) DaRequisicao(r *http.Request) (*Principal, error) {
	if chave := r.Header.Get("X-Robot-Key"); chave != "" {
		if s.cfg.ChaveRobo == "" || subtle.ConstantTimeCompare([]byte(chave), []byte(s.cfg.ChaveRobo)) != 1 {
			return nil, ErrCredencialMa
		}
		return &Principal{Tipo: TipoRobo, Nome: "robô", Nivel: "robo", Ativo: true}, nil
	}

	token := ""
	if cab := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(cab), "bearer ") {
		token = strings.TrimSpace(cab[7:])
	}
	if token == "" {
		return nil, ErrSemCredencial
	}

	uid, err := s.usuarioDoToken(r.Context(), token)
	if err != nil {
		return nil, err
	}
	return s.PerfilDe(r.Context(), uid)
}

// usuarioDoToken pergunta ao Supabase de quem é este token. É o Supabase quem valida
// a assinatura e a validade — o motor não guarda segredo de sessão.
func (s *Servico) usuarioDoToken(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.Supabase.Auth()+"/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("apikey", s.cfg.Supabase.ChavePublica)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("não consegui confirmar a sessão: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ErrCredencialMa
	}
	corpo, _ := io.ReadAll(resp.Body)
	var u usuarioSupabase
	if err := json.Unmarshal(corpo, &u); err != nil || u.ID == "" {
		return "", ErrCredencialMa
	}
	return u.ID, nil
}

// PerfilDe busca no banco o que este usuário é dentro do FrotaHub.
// O nível vem da CATEGORIA, nunca da pessoa — assim os dois não podem discordar.
func (s *Servico) PerfilDe(ctx context.Context, uid string) (*Principal, error) {
	caminho := "perfis?id=eq." + banco.Escapar(uid) +
		"&select=id,usuario,nome,ativo,cliente_id,categoria_id,clientes(nome),categorias(nome,nivel)&limit=1"

	var linhas []perfilBanco
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		// Existe no login do Supabase mas não tem perfil no FrotaHub.
		return nil, ErrCredencialMa
	}

	p := linhas[0]
	if !p.Ativo {
		return nil, ErrInativo
	}

	principal := &Principal{
		Tipo:        TipoUsuario,
		UserID:      p.ID,
		Usuario:     p.Usuario,
		Nome:        p.Nome,
		ClienteID:   p.ClienteID,
		CategoriaID: p.CategoriaID,
		Ativo:       p.Ativo,
		Nivel:       "comum",
	}
	if p.Clientes != nil {
		principal.ClienteNome = p.Clientes.Nome
	}
	if p.Categorias != nil {
		principal.CategoriaNome = p.Categorias.Nome
		if p.Categorias.Nivel != "" {
			principal.Nivel = p.Categorias.Nivel
		}
	}
	return principal, nil
}

// StatusDoErro traduz os erros deste pacote em código HTTP.
func StatusDoErro(err error) int {
	switch {
	case errors.Is(err, ErrSemCredencial), errors.Is(err, ErrCredencialMa):
		return http.StatusUnauthorized
	case errors.Is(err, ErrInativo):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
