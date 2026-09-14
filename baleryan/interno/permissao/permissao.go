// rev 2 — o que você pode
//
// A regra do FrotaHub cabe em quatro linhas:
//
//	Cada login pertence a uma categoria.
//	A categoria tem uma lista de rotinas liberadas.
//	Abrir uma rotina significa que a sua categoria a tem marcada.
//	Duas exceções, e só duas: o builder passa sempre, e só o builder mexe em login.
//
// Nada além disso decide acesso (P-14). Um segundo mecanismo de permissão é como
// se acumula sete jeitos diferentes de autenticar sem ninguém perceber.
package permissao

import (
	"context"
	"errors"
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

var (
	ErrNegado         = errors.New("você não tem acesso a esta rotina")
	ErrSoBuilder      = errors.New("apenas o builder pode fazer isto")
	ErrSoBuilderOuCEO = errors.New("apenas o builder ou o CEO podem fazer isto")
)

type Servico struct{ bd *banco.Cliente }

func Novo(bd *banco.Cliente) *Servico { return &Servico{bd: bd} }

type linhaPermissao struct {
	Rotina string `json:"rotina"`
}

// Pode responde se este principal alcança a rotina.
//
// Em caso de falha ao consultar, devolve erro — e quem chama traduz em acesso
// NEGADO. Na dúvida, nega (CORE-08): é melhor uma tela de erro do que um acesso
// indevido.
func (s *Servico) Pode(ctx context.Context, p *seguranca.Principal, rotina string) (bool, error) {
	if p == nil {
		return false, nil
	}

	// Exceção 1: o builder passa sempre. Garantia anti-tranca — uma matriz mal
	// configurada nunca deixa o dono do sistema do lado de fora.
	if p.Builder() {
		return true, nil
	}

	// O robô não navega no sistema: ele alcança apenas as rotas próprias, que já
	// são protegidas pelo seu segredo. Não entra na matriz.
	if p.Tipo == seguranca.TipoRobo {
		return false, nil
	}

	if p.CategoriaID == "" {
		return false, nil
	}

	// categoria_rotinas_efetivas é a UNIÃO da matriz de sempre com o bypass de
	// módulo do CEO (ver 067_categoria_modulos_e_bypass.sql) — nunca mais
	// consultar categoria_permissoes direto, ou o bypass fica invisível aqui.
	caminho := "categoria_rotinas_efetivas?categoria_id=eq." + banco.Escapar(p.CategoriaID) +
		"&rotina=eq." + banco.Escapar(rotina) + "&select=rotina&limit=1"

	var linhas []linhaPermissao
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return false, err
	}
	return len(linhas) > 0, nil
}

// Exige devolve erro se o principal não alcançar a rotina. É o que as rotas usam.
func (s *Servico) Exige(ctx context.Context, p *seguranca.Principal, rotina string) error {
	ok, err := s.Pode(ctx, p, rotina)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNegado
	}
	return nil
}

// ExigeBuilder é a segunda exceção: mexer em login é exclusividade do builder.
// Assim ninguém consegue se promover nem criar outro dono para o sistema.
func ExigeBuilder(p *seguranca.Principal) error {
	if !p.Builder() {
		return ErrSoBuilder
	}
	return nil
}

// ExigeBuilderOuCEO é a mesma trava anti-circular de ExigeBuilder, alargada
// um degrau: mexer nas categorias abaixo do CEO (e no vínculo hierárquico) é
// dele também, não só do builder — mas continua sendo trava de NÍVEL, não de
// rotina. Uma rotina que controlasse quem edita a matriz seria circular:
// quem tivesse essa rotina poderia se dar qualquer outra.
func ExigeBuilderOuCEO(p *seguranca.Principal) error {
	if !p.Builder() && p.Nivel != "ceo" {
		return ErrSoBuilderOuCEO
	}
	return nil
}

// Rotinas devolve os códigos que este principal alcança. É o que o front usa para
// montar o menu — o menu se ajusta sozinho ao login (P-17).
func (s *Servico) Rotinas(ctx context.Context, p *seguranca.Principal) ([]string, error) {
	if p == nil {
		return []string{}, nil
	}

	// O builder alcança tudo que existe no catálogo.
	if p.Builder() {
		var todas []struct {
			Codigo string `json:"codigo"`
		}
		if err := s.bd.Buscar(ctx, "rotinas?select=codigo&order=codigo", &todas); err != nil {
			return nil, err
		}
		saida := make([]string, 0, len(todas))
		for _, r := range todas {
			saida = append(saida, r.Codigo)
		}
		return saida, nil
	}

	if p.CategoriaID == "" {
		return []string{}, nil
	}

	var linhas []struct {
		Rotina string `json:"rotina"`
	}
	caminho := "categoria_rotinas_efetivas?categoria_id=eq." + banco.Escapar(p.CategoriaID) +
		"&select=rotina&order=rotina"
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	saida := make([]string, 0, len(linhas))
	for _, l := range linhas {
		saida = append(saida, l.Rotina)
	}
	return saida, nil
}

// StatusDoErro traduz os erros deste pacote em código HTTP.
func StatusDoErro(err error) int {
	switch {
	case errors.Is(err, ErrNegado), errors.Is(err, ErrSoBuilder), errors.Is(err, ErrSoBuilderOuCEO):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
