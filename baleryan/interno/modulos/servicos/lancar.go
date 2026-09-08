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
	"log"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
)

// ErrSemArquivoDeOrcamento — lançar sem o PDF anexado não tem o que subir
// pro Trílogo (ver InserirArquivoDeOrcamento, documentos.go).
var ErrSemArquivoDeOrcamento = errors.New("este card ainda não tem o arquivo do orçamento anexado")

// LancarNoTrilogo cria a cotação (se ainda não existir) e o orçamento no
// Trílogo, e grava os dois ids + o valor no card — Feitos -> Lançados.
//
// MÃO DE OBRA E MATERIAL, NO MESMO ORÇAMENTO
//
//	Decisão do dono em 04/09/2026 era só mão de obra; revista em 08/09/2026
//	para incluir material também — mesmo formulário, uma segunda lista.
//	trilogo.MontarOrcamentoServico já aceitava as duas desde sempre (é o
//	mesmo formato de cotacoes.go, CriarOrcamento); só este caminho mandava
//	materiais sempre vazio.
func (s *Servico) LancarNoTrilogo(ctx context.Context, clienteID, itemID, descricaoCotacao string, maoDeObra, materiais []trilogo.ItemOrcamento) (cotacaoID, orcamentoID int, valor float64, err error) {
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
		// GRAVA JÁ, ANTES DE TENTAR O ORÇAMENTO — NÃO NO FINAL
		//
		//	O BUG: se CriarOrcamentoServico falhar daqui pra frente, a
		//	cotação JÁ EXISTE de verdade no Trílogo — mas como o card só
		//	era atualizado no final (junto com o orçamento), a próxima
		//	tentativa não achava `CotacaoTrilogoID` nenhum e criava OUTRA
		//	cotação, deixando a primeira órfã pra sempre (Trílogo não deixa
		//	apagar cotação). Confirmado ao vivo em 08/09/2026: o ticket
		//	135698 acumulou duas cotações "TESTE" sem orçamento nenhum
		//	depois de duas tentativas que recusaram no passo seguinte.
		//
		//	Gravar aqui — mesmo que o resto falhe — faz a PRÓXIMA tentativa
		//	reaproveitar esta cotação em vez de criar mais uma.
		if err := s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID),
			map[string]any{"cotacao_trilogo_id": cotacaoID}); err != nil {
			log.Printf("servicos: criei a cotação %d no Trílogo (ticket %d) mas não gravei no card: %v — a próxima tentativa vai criar outra",
				cotacaoID, item.Ticket, err)
		}
	}

	supplierID := s.tri.IDFornecedorDaConta(item.Conta)
	if supplierID == 0 {
		return cotacaoID, 0, 0, ErrFornecedorNaoExiste
	}

	orc := trilogo.MontarOrcamentoServico(cotacaoID, supplierID, maoDeObra, materiais, []trilogo.Subido{subido})
	orcamentoID, err = sessao.CriarOrcamentoServico(ctx, orc)
	if err != nil {
		return cotacaoID, 0, 0, err
	}
	valor = totalItens(maoDeObra) + totalItens(materiais)

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
