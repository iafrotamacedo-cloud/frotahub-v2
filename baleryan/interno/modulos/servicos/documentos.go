// rev 1 — o arquivo do orçamento e da nota fiscal
//
// O MESMO PADRÃO DE orcamentos/documentos.go (guardarUm)
//
//	sha256 do conteúdo -> chave no R2 espalhada por armazem.Caminho -> upsert
//	em `arquivos` por sha256 (migração 007) -> o card guarda só a referência.
//	Sem fila, sem leitura automática (XML/OCR) — aqui o arquivo é só prova
//	documental, ninguém extrai dado dele.
package servicos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
)

// TamanhoMaximoDeArquivo — mesmo teto de orcamentos (20 MB): PDF de orçamento
// ou nota fiscal com anexo não é pequeno, mas também não precisa de mais.
const TamanhoMaximoDeArquivo = 20 << 20

// ValidadeDoLinkDeArquivo — mesmo teto de orcamentos.ValidadeDoLink: tempo
// suficiente pra tela abrir o PDF, curto o bastante pra não virar um link
// permanente por engano.
const ValidadeDoLinkDeArquivo = 5 * time.Minute

var ErrArquivoVazio = errors.New("o arquivo está vazio")

func tipoDoNome(nome string) string {
	if t := mime.TypeByExtension(filepath.Ext(nome)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// guardarArquivo sobe pro R2 e upserta em `arquivos` — devolve o sha256, que
// é a única coisa que os cards de Serviço precisam guardar.
func (s *Servico) guardarArquivo(ctx context.Context, clienteID, nome string, conteudo []byte) (sha string, err error) {
	if len(conteudo) == 0 {
		return "", ErrArquivoVazio
	}
	if len(conteudo) > TamanhoMaximoDeArquivo {
		return "", fmt.Errorf("passa de %d MB", TamanhoMaximoDeArquivo>>20)
	}

	soma := sha256.Sum256(conteudo)
	sha = hex.EncodeToString(soma[:])
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(nome)), ".")
	tipo := tipoDoNome(nome)

	// O ARQUIVO NUNCA É APAGADO, ENTÃO ELE PODE JÁ ESTAR LÁ — a chave é o
	// sha256 do conteúdo; subir o mesmo PDF duas vezes grava o mesmo lugar
	// com os mesmos bytes (P-04), sem duplicar nada.
	chave := armazem.Caminho(clienteID, sha, ext)
	if err := s.arm.Enviar(ctx, chave, bytes.NewReader(conteudo), int64(len(conteudo)), sha, tipo); err != nil {
		return "", fmt.Errorf("não consegui guardar no armazém: %w", err)
	}
	if err := s.bd.Upsert(ctx, "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": clienteID,
		"tamanho":    len(conteudo),
		"tipo":       tipo,
		"chave_r2":   chave,
	}}, nil); err != nil {
		return "", fmt.Errorf("guardei o arquivo mas não consegui registrá-lo: %w", err)
	}
	return sha, nil
}

// InserirArquivoDeOrcamento anexa o PDF do orçamento — Pendentes -> Feitos.
// O RASCUNHO, NÃO O LANÇAMENTO
//
//	Só guarda no R2 e avança o card; falar com o Trílogo é LancarNoTrilogo
//	(lancar.go), outro passo, outro clique. Permite reanexar enquanto o card
//	ainda está em orcamento_feito (corrigir o PDF antes de lançar) — só não
//	muda `status` de novo nesse caso, porque já está lá.
func (s *Servico) InserirArquivoDeOrcamento(ctx context.Context, clienteID, itemID, nome string, conteudo []byte) error {
	item, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return err
	}
	podeAnexar := item.Status == StatusAguardandoOrcamento || item.Status == StatusOrcamentoRejeitado || item.Status == StatusOrcamentoFeito
	if !podeAnexar {
		return &ErrTransicaoInvalida{De: item.Status, Para: StatusOrcamentoFeito}
	}

	sha, err := s.guardarArquivo(ctx, clienteID, nome, conteudo)
	if err != nil {
		return err
	}

	campos := map[string]any{
		"orcamento_arquivo_sha256": sha,
		"orcamento_arquivo_nome":   nome,
		"orcamento_arquivo_em":     time.Now().UTC().Format(time.RFC3339),
	}
	if item.Status == StatusAguardandoOrcamento || item.Status == StatusOrcamentoRejeitado {
		campos["status"] = StatusOrcamentoFeito
	}
	return s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos)
}

// ErrSemPCO — nota fiscal sem PCO preenchido não tem pra onde amarrar
// (a ordem do funil é sempre: PCO primeiro, nota fiscal depois).
var ErrSemPCO = errors.New("este card ainda não tem PCO — preencha o PCO primeiro")

// InserirArquivoDeNF anexa a nota fiscal — a ÚNICA transição de status real
// do cartão de Faturamento: aguardando_faturamento -> faturado.
func (s *Servico) InserirArquivoDeNF(ctx context.Context, clienteID, itemID, numero, nome string, conteudo []byte) error {
	item, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return err
	}
	if item.Status != StatusAguardandoFaturamento {
		return &ErrTransicaoInvalida{De: item.Status, Para: StatusFaturado}
	}
	if item.PCONumero == nil {
		return ErrSemPCO
	}

	sha, err := s.guardarArquivo(ctx, clienteID, nome, conteudo)
	if err != nil {
		return err
	}

	campos := map[string]any{
		"nf_numero":         nuloSeVazio(numero),
		"nf_arquivo_sha256": sha,
		"nf_arquivo_nome":   nome,
		"nf_arquivo_em":     time.Now().UTC().Format(time.RFC3339),
		"status":            StatusFaturado,
	}
	return s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos)
}

// ArquivoDoItem acha a chave no R2 do orçamento ou da nota fiscal de um
// card, para a rota HTTP pedir um link temporário (armazem.LinkTemporario) —
// mesmo padrão de orcamentos, o arquivo nunca passa pelo motor.
func (s *Servico) ArquivoDoItem(ctx context.Context, clienteID, itemID, tipo string) (chaveR2 string, err error) {
	item, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return "", err
	}
	var sha *string
	switch tipo {
	case "orcamento":
		sha = item.OrcamentoArquivoSHA256
	case "nf":
		sha = item.NFArquivoSHA256
	default:
		return "", fmt.Errorf("tipo de arquivo desconhecido: %q", tipo)
	}
	if sha == nil {
		return "", fmt.Errorf("este card não tem arquivo de %s", tipo)
	}

	var linhas []struct {
		ChaveR2 string `json:"chave_r2"`
	}
	caminho := "arquivos?sha256=eq." + banco.Escapar(*sha) + "&select=chave_r2&limit=1"
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return "", err
	}
	if len(linhas) == 0 {
		return "", fmt.Errorf("arquivo não encontrado no armazém")
	}
	return linhas[0].ChaveR2, nil
}
