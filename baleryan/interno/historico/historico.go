// rev 1 — o rastro do sistema
//
// POR QUE ESTA PEÇA EXISTE AGORA, E NÃO ANTES
//
//	Na revisão anterior, gravar histórico era uma função privada dentro do módulo
//	de usuários, porque era o único que gravava. Desenhar a peça compartilhada
//	com um usuário só é adivinhar como o segundo vai ser.
//
//	O segundo chegou: a matriz de permissões (MOD-ACESSO-01). Com dois, o formato
//	parou de ser suposição — então a função sobe para cá e os dois módulos usam a
//	mesma (CORE-06).
//
// O QUE ESTA PEÇA NÃO DECIDE
//
//	Ela não obriga ninguém a gravar. Quem obriga é a tenet do módulo. Esta peça
//	só garante que, quando um módulo grava, grava do mesmo jeito que os outros.
package historico

import (
	"context"
	"log"
	"strconv"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

// Mudanca é o par de/para de um campo. Os dois lados aparecem sempre, inclusive
// na criação (onde o "de" é nulo), para a tela ter um formato só para desenhar.
type Mudanca struct {
	De   any `json:"de"`
	Para any `json:"para"`
}

// Aviso é a frase que vai para a tela quando a operação deu certo mas o rastro
// não foi gravado.
//
// Não é erro de HTTP de propósito: a operação ACONTECEU. Responder "deu errado"
// faria a pessoa tentar de novo — criando um segundo registro, ou desfazendo o
// que ela acabou de fazer. Mentir sobre o resultado causa mais dano do que avisar.
const Aviso = "Feito, mas o histórico NÃO foi gravado. Anote o que você fez e avise o responsável pelo sistema."

type Servico struct{ bd *banco.Cliente }

func Novo(bd *banco.Cliente) *Servico { return &Servico{bd: bd} }

// Linha é um evento, como a tela lê.
type Linha struct {
	ID           int64              `json:"id"`
	Acao         string             `json:"acao"`
	AutorUsuario string             `json:"autor_usuario"`
	Quando       string             `json:"quando"`
	Mudancas     map[string]Mudanca `json:"mudancas"`
}

// Registrar grava um evento.
//
// Devolve o erro em vez de engoli-lo: quem chama decide o que dizer na tela, mas
// ninguém tem licença para fingir que gravou.
func (s *Servico) Registrar(ctx context.Context, p *seguranca.Principal, modulo, registroID, acao string, mudancas map[string]Mudanca) error {
	linha := map[string]any{
		"cliente_id":    p.ClienteID,
		"modulo":        modulo,
		"registro_id":   registroID,
		"acao":          acao,
		"autor_id":      p.UserID,
		"autor_usuario": p.Usuario,
	}
	if len(mudancas) > 0 {
		linha["mudancas"] = mudancas
	}

	if err := s.bd.Inserir(ctx, "historico", []map[string]any{linha}, nil); err != nil {
		// Segunda trilha. Se o banco recusou, ao menos o console do servidor
		// guarda o que aconteceu — é de onde a informação será resgatada.
		log.Printf("[historico] FALHOU: %s %s/%s por %s: %v", acao, modulo, registroID, p.Usuario, err)
		return err
	}
	return nil
}

// Listar devolve o rastro de um registro, do mais novo para o mais velho.
//
// A ordem das condições acompanha o índice `historico_por_registro`. O desempate
// por id existe porque dois eventos da mesma ação nascem com o mesmo carimbo de
// tempo — sem ele, a ordem entre os dois seria sorteio.
func (s *Servico) Listar(ctx context.Context, clienteID, modulo, registroID string, limite, inicio int) ([]Linha, error) {
	caminho := "historico?cliente_id=eq." + banco.Escapar(clienteID) +
		"&modulo=eq." + banco.Escapar(modulo) +
		"&registro_id=eq." + banco.Escapar(registroID) +
		"&select=id,acao,autor_usuario,quando,mudancas" +
		"&order=quando.desc,id.desc" +
		"&limit=" + strconv.Itoa(limite) + "&offset=" + strconv.Itoa(inicio)

	linhas := []Linha{}
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	return linhas, nil
}
