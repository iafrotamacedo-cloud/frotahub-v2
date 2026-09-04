// rev 1 — cotação e orçamento mexem no card, não só no Trílogo
//
// A LACUNA QUE ISTO FECHA
//
//	As primeiras rotas de cotação/orçamento (trilogo/rotas_servico.go, rev 1)
//	só falavam com o Trílogo — quem criasse uma cotação ou lançasse um
//	orçamento pela tela via o Trílogo confirmar, mas o card no nosso Kanban
//	continuava parado em "Aguardando orçamento", sem `cotacao_trilogo_id`,
//	sem `orcamento_trilogo_id`, sem `orcamento_valor`. Era a mesma classe de
//	problema do botão manual de fila (kanban.go, MarcarComoServico) — duas
//	ações fazendo metade do trabalho cada. Revisão do dono, 04/09/2026.
//
// CADA FUNÇÃO AQUI FAZ AS DUAS COISAS, NESTA ORDEM
//
//	Primeiro o Trílogo (a fonte da verdade), depois o card local. Se o
//	segundo passo falhar, o erro carrega o id que o Trílogo já criou — a
//	tela mostra "aconteceu, mas não gravei aqui", nunca finge que não
//	aconteceu nada (mesmo padrão de PromoverCandicato, kanban.go).
package servicos

import (
	"context"
	"errors"
	"fmt"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
)

var (
	// ErrSemCotacao — criar orçamento sem cotação não tem pra onde amarrar
	// (Orçamento.CreateBudget exige quotationId).
	ErrSemCotacao = errors.New("este card ainda não tem cotação — crie a cotação primeiro")

	// ErrSemOrcamento — excluir sem nunca ter lançado.
	ErrSemOrcamento = errors.New("este card não tem orçamento lançado")

	// ErrFornecedorNaoExiste — mesma ideia de trilogo.ErrResponsavelNaoExiste,
	// pro outro id que precisa estar configurado antes de lançar orçamento.
	ErrFornecedorNaoExiste = errors.New("fornecedor não existe")
)

// ErrOrcamentoJaExiste — CriarOrcamento recusa lançar um segundo orçamento
// por cima de um já rastreado. SEM ISSO, o `orcamento_trilogo_id` do card
// seria sobrescrito e o orçamento antigo ficaria esquecido, vivo no Trílogo,
// cobrando de alguém que não sabe mais que ele existe — o MESMO risco de
// "duas coisas mudando o mesmo estado" que o dono pediu pra revisar. A saída
// é sempre explícita: ExcluirOrcamento primeiro, depois lança de novo.
type ErrOrcamentoJaExiste struct {
	OrcamentoTrilogoID int
}

func (e *ErrOrcamentoJaExiste) Error() string {
	return fmt.Sprintf("este card já tem um orçamento lançado (id %d no Trílogo) — exclua antes de lançar outro", e.OrcamentoTrilogoID)
}

// ListarCotacoes repassa a leitura ao Trílogo — não grava nada local, é
// consulta, não evento.
func (s *Servico) ListarCotacoes(ctx context.Context, clienteID, itemID string) ([]trilogo.Cotacao, error) {
	sessao, item, err := s.sessaoDoItem(ctx, clienteID, itemID)
	if err != nil {
		return nil, err
	}
	return sessao.Cotacoes(ctx, item.Ticket)
}

// CriarCotacao cria a cotação no Trílogo e grava o vínculo no card. Avança
// o status para orcamento_feito só quando isso é o esperado — de
// aguardando_orcamento (o caminho normal) ou de orcamento_rejeitado (o
// REFAZER: uma cotação nova depois de um orçamento recusado). Em qualquer
// outro status só grava o id novo, sem empurrar o card pra frente ou pra
// trás — Trílogo nunca deixa apagar uma cotação (docs/api-trilogo-servicos.md,
// item 4), então criar uma nova aqui é sempre seguro, mesmo fora da ordem.
func (s *Servico) CriarCotacao(ctx context.Context, clienteID, itemID, descricao string) (cotacaoID int, err error) {
	sessao, item, err := s.sessaoDoItem(ctx, clienteID, itemID)
	if err != nil {
		return 0, err
	}
	cotacaoID, err = sessao.CriarCotacao(ctx, item.Ticket, descricao)
	if err != nil {
		return 0, err
	}

	campos := map[string]any{"cotacao_trilogo_id": cotacaoID}
	if item.Status == StatusAguardandoOrcamento || item.Status == StatusOrcamentoRejeitado {
		campos["status"] = StatusOrcamentoFeito
	}
	if err := s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos); err != nil {
		return cotacaoID, fmt.Errorf("criei a cotação no Trílogo (id %d), mas não consegui gravar no card: %w", cotacaoID, err)
	}
	return cotacaoID, nil
}

// CriarOrcamento cria o orçamento (mão de obra + material, sempre um só —
// ver trilogo.MontarOrcamentoServico) e grava id+valor no card. Avança
// orcamento_feito -> orcamento_lancado.
func (s *Servico) CriarOrcamento(ctx context.Context, clienteID, itemID string, maoDeObra, materiais []trilogo.ItemOrcamento, subidos []trilogo.Subido) (orcamentoID int, valor float64, err error) {
	sessao, item, err := s.sessaoDoItem(ctx, clienteID, itemID)
	if err != nil {
		return 0, 0, err
	}
	if item.CotacaoTrilogoID == nil {
		return 0, 0, ErrSemCotacao
	}
	if item.OrcamentoTrilogoID != nil {
		return 0, 0, &ErrOrcamentoJaExiste{OrcamentoTrilogoID: *item.OrcamentoTrilogoID}
	}
	supplierID := s.tri.IDFornecedorDaConta(item.Conta)
	if supplierID == 0 {
		return 0, 0, ErrFornecedorNaoExiste
	}

	orc := trilogo.MontarOrcamentoServico(*item.CotacaoTrilogoID, supplierID, maoDeObra, materiais, subidos)
	orcamentoID, err = sessao.CriarOrcamentoServico(ctx, orc)
	if err != nil {
		return 0, 0, err
	}
	valor = totalItens(maoDeObra) + totalItens(materiais)

	campos := map[string]any{"orcamento_trilogo_id": orcamentoID, "orcamento_valor": valor}
	if item.Status == StatusOrcamentoFeito {
		campos["status"] = StatusOrcamentoLancado
	}
	if err := s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos); err != nil {
		return orcamentoID, valor, fmt.Errorf("criei o orçamento no Trílogo (id %d), mas não consegui gravar no card: %w", orcamentoID, err)
	}
	return orcamentoID, valor, nil
}

// ExcluirOrcamento apaga o orçamento tracked no card, no Trílogo e aqui — e
// volta orcamento_lancado -> orcamento_feito: sem orçamento lançado nenhum,
// o card não pode continuar dizendo "lançado".
func (s *Servico) ExcluirOrcamento(ctx context.Context, clienteID, itemID string) error {
	sessao, item, err := s.sessaoDoItem(ctx, clienteID, itemID)
	if err != nil {
		return err
	}
	if item.OrcamentoTrilogoID == nil {
		return ErrSemOrcamento
	}
	if err := sessao.ExcluirOrcamentoServico(ctx, *item.OrcamentoTrilogoID); err != nil {
		return err
	}

	campos := map[string]any{"orcamento_trilogo_id": nil, "orcamento_valor": nil}
	if item.Status == StatusOrcamentoLancado {
		campos["status"] = StatusOrcamentoFeito
	}
	return s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos)
}

// sessaoDoItem acha o card ativo e já abre a sessão certa do Trílogo pra
// conta dele — toda ação de cotação/orçamento passa por aqui primeiro.
func (s *Servico) sessaoDoItem(ctx context.Context, clienteID, itemID string) (*trilogo.Sessao, ItemKanban, error) {
	item, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return nil, ItemKanban{}, err
	}
	sessao, err := s.tri.SessaoDaConta(ctx, item.Conta)
	if err != nil {
		return nil, ItemKanban{}, fmt.Errorf("entrando no Trílogo: %w", err)
	}
	return sessao, item, nil
}

func totalItens(itens []trilogo.ItemOrcamento) float64 {
	var total float64
	for _, it := range itens {
		total += it.Valor * it.Qtd
	}
	return total
}
