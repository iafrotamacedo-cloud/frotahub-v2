// rev 1 — lançar no Trílogo, o segundo passo do orçamento
//
// A SEGUNDA METADE DE "CRIAR ORÇAMENTO" (ver documentos.go pra primeira)
//
//	Pendentes -> Feitos anexa o PDF, só pro nosso R2 (documentos.go,
//	InserirArquivoDeOrcamento). Feitos -> Lançados é este arquivo: cria a
//	cotação (se ainda não tiver) e o orçamento no Trílogo DE VERDADE, subindo
//	o MESMO PDF já anexado — ninguém pede o arquivo duas vezes.
package servicos

import (
	"context"
	"errors"
	"fmt"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
)

// ErrSemArquivoDeOrcamento — lançar sem o PDF anexado não tem o que subir
// pro Trílogo (ver InserirArquivoDeOrcamento, documentos.go).
var ErrSemArquivoDeOrcamento = errors.New("este card ainda não tem o arquivo do orçamento anexado")

// LancarNoTrilogo cria a cotação (se ainda não existir) e o orçamento no
// Trílogo, e grava os dois ids + o valor no card — Feitos -> Lançados.
//
// SEMPRE MÃO DE OBRA
//
//	Serviço não usa a categoria "material" do Trílogo (decisão do dono,
//	04/09/2026) — só `services`. trilogo.MontarOrcamentoServico aceita as
//	duas listas; aqui a de materiais é sempre vazia.
func (s *Servico) LancarNoTrilogo(ctx context.Context, clienteID, itemID, descricaoCotacao string, itens []trilogo.ItemOrcamento) (cotacaoID, orcamentoID int, valor float64, err error) {
	item, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return 0, 0, 0, err
	}
	if item.Status != StatusOrcamentoFeito {
		return 0, 0, 0, &ErrTransicaoInvalida{De: item.Status, Para: StatusOrcamentoLancado}
	}
	if item.OrcamentoArquivoSHA256 == nil {
		return 0, 0, 0, ErrSemArquivoDeOrcamento
	}
	if item.OrcamentoTrilogoID != nil {
		return 0, 0, 0, &ErrOrcamentoJaExiste{OrcamentoTrilogoID: *item.OrcamentoTrilogoID}
	}

	chaveR2, err := s.ArquivoDoItem(ctx, clienteID, itemID, "orcamento")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("achando o arquivo do orçamento: %w", err)
	}
	conteudo, err := s.arm.Baixar(ctx, chaveR2)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("baixando o arquivo do orçamento: %w", err)
	}

	sessao, err := s.tri.SessaoDaConta(ctx, item.Conta)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("entrando no Trílogo: %w", err)
	}

	nome := "orcamento.pdf"
	if item.OrcamentoArquivoNome != nil && *item.OrcamentoArquivoNome != "" {
		nome = *item.OrcamentoArquivoNome
	}
	subido, err := sessao.SubirArquivo(ctx, nome, conteudo)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("subindo o anexo pro Trílogo: %w", err)
	}

	if item.CotacaoTrilogoID != nil {
		cotacaoID = *item.CotacaoTrilogoID
	} else {
		cotacaoID, err = sessao.CriarCotacao(ctx, item.Ticket, descricaoCotacao)
		if err != nil {
			return 0, 0, 0, err
		}
	}

	supplierID := s.tri.IDFornecedorDaConta(item.Conta)
	if supplierID == 0 {
		return cotacaoID, 0, 0, ErrFornecedorNaoExiste
	}

	orc := trilogo.MontarOrcamentoServico(cotacaoID, supplierID, itens, nil, []trilogo.Subido{subido})
	orcamentoID, err = sessao.CriarOrcamentoServico(ctx, orc)
	if err != nil {
		return cotacaoID, 0, 0, err
	}
	valor = totalItens(itens)

	campos := map[string]any{
		"cotacao_trilogo_id":   cotacaoID,
		"orcamento_trilogo_id": orcamentoID,
		"orcamento_valor":      valor,
		"status":               StatusOrcamentoLancado,
	}
	if err := s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos); err != nil {
		return cotacaoID, orcamentoID, valor, fmt.Errorf(
			"lancei no Trílogo (cotação %d, orçamento %d), mas não consegui gravar no card: %w",
			cotacaoID, orcamentoID, err)
	}
	return cotacaoID, orcamentoID, valor, nil
}
