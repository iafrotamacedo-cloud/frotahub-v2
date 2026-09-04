// rev 1 — o Kanban de verdade
//
// "EM QUE PÉ ESTÁ ESTE SERVIÇO?" — FATO, NÃO PALPITE
//
//	Uma linha de servicos_orcamentos só existe depois que o responsável FOI
//	trocado de verdade no Trílogo — pelo gatilho automático (detectado aqui,
//	RodarDeteccaoDeGatilho) ou pelo botão (MarcarComoServico, mais abaixo,
//	que já fala com o Trílogo direto). Este arquivo não decide SE um chamado é
//	Serviço — isso é candidatos.go e o humano que aprova. Aqui só se controla
//	POR ONDE ele anda depois de decidido.
package servicos

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
)

// As dez colunas — ver o comentário da migração 050 pro desenho completo.
const (
	StatusAguardandoOrcamento   = "aguardando_orcamento"
	StatusOrcamentoFeito        = "orcamento_feito"
	StatusOrcamentoLancado      = "orcamento_lancado"
	StatusOrcamentoAprovado     = "orcamento_aprovado"
	StatusOrcamentoRejeitado    = "orcamento_rejeitado"
	StatusAprovadoExecucao      = "aprovado_execucao"
	StatusEmExecucao            = "em_execucao"
	StatusFinalizado            = "finalizado"
	StatusAguardandoFaturamento = "aguardando_faturamento"
	StatusFaturado              = "faturado"
)

// proximosStatus é o mapa de transições válidas — a régua inteira do
// "fluxo tem que fechar sempre" num lugar só.
//
// StatusOrcamentoRejeitado -> StatusOrcamentoFeito (REFAZER) é a única volta
// atrás da tabela: rejeitado não anda sozinho (decisão do dono — "fica
// parado até ação manual"), mas também não é beco sem saída — alguém refaz
// o orçamento e o card volta a andar.
var proximosStatus = map[string][]string{
	StatusAguardandoOrcamento:   {StatusOrcamentoFeito},
	StatusOrcamentoFeito:        {StatusOrcamentoLancado},
	StatusOrcamentoLancado:      {StatusOrcamentoAprovado, StatusOrcamentoRejeitado},
	StatusOrcamentoAprovado:     {StatusAprovadoExecucao},
	StatusOrcamentoRejeitado:    {StatusOrcamentoFeito},
	StatusAprovadoExecucao:      {StatusEmExecucao},
	StatusEmExecucao:            {StatusFinalizado},
	StatusFinalizado:            {StatusAguardandoFaturamento},
	StatusAguardandoFaturamento: {StatusFaturado},
	StatusFaturado:              {},
}

// ItemKanban é uma linha de servicos_orcamentos, como a tela lê.
type ItemKanban struct {
	ID                 string   `json:"id"`
	ChamadoID          string   `json:"chamado_id"`
	Ticket             int      `json:"ticket"`
	Conta              string   `json:"conta"`
	Status             string   `json:"status"`
	CotacaoTrilogoID   *int     `json:"cotacao_trilogo_id"`
	OrcamentoTrilogoID *int     `json:"orcamento_trilogo_id"`
	OrcamentoValor     *float64 `json:"orcamento_valor"`
	Origem             string   `json:"origem"`
	EntrouEm           string   `json:"entrou_em"`
	AtualizadoEm       string   `json:"atualizado_em"`

	// O rascunho local do orçamento (migração 052) — o PDF anexado em
	// Pendentes, antes de existir no Trílogo.
	OrcamentoArquivoSHA256 *string `json:"orcamento_arquivo_sha256"`
	OrcamentoArquivoNome   *string `json:"orcamento_arquivo_nome"`

	// Faturamento por ticket (migração 053).
	PCONumero       *string `json:"pco_numero"`
	NFArquivoSHA256 *string `json:"nf_arquivo_sha256"`
}

// ListarKanban devolve todos os cartões ativos — a tela agrupa por `status`
// nas dez colunas; não há razão para paginar aqui (o funil inteiro de
// Serviço, medido em 04/09/2026, é uma fração dos ~1.900 chamados totais).
func (s *Servico) ListarKanban(ctx context.Context, clienteID string) ([]ItemKanban, error) {
	itens := []ItemKanban{}
	caminho := "servicos_orcamentos?cliente_id=eq." + banco.Escapar(clienteID) +
		"&removido_em=is.null" +
		"&select=id,chamado_id,ticket,conta,status,cotacao_trilogo_id,orcamento_trilogo_id," +
		"orcamento_valor,origem,entrou_em,atualizado_em" +
		"&order=atualizado_em.desc"
	if err := s.bd.Buscar(ctx, caminho, &itens); err != nil {
		return nil, err
	}
	return itens, nil
}

// ErrTransicaoInvalida — P-18: nunca só "não pode". A mensagem carrega pra
// onde dava pra ir, porque quem programou o front vai perguntar exatamente
// isso no primeiro teste manual.
type ErrTransicaoInvalida struct {
	De   string
	Para string
}

func (e *ErrTransicaoInvalida) Error() string {
	validos := proximosStatus[e.De]
	if len(validos) == 0 {
		return fmt.Sprintf("%q é status final — não anda mais", e.De)
	}
	return fmt.Sprintf("de %q só dá para ir para %v, não %q", e.De, validos, e.Para)
}

// MudarStatus move um cartão para a próxima coluna — só se a transição for
// uma das previstas em proximosStatus.
func (s *Servico) MudarStatus(ctx context.Context, clienteID, itemID, novoStatus string) error {
	atual, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return err
	}
	permitido := false
	for _, v := range proximosStatus[atual.Status] {
		if v == novoStatus {
			permitido = true
			break
		}
	}
	if !permitido {
		return &ErrTransicaoInvalida{De: atual.Status, Para: novoStatus}
	}
	return s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID),
		map[string]any{"status": novoStatus})
}

func (s *Servico) itemAtivo(ctx context.Context, clienteID, itemID string) (ItemKanban, error) {
	var itens []ItemKanban
	caminho := "servicos_orcamentos?id=eq." + banco.Escapar(itemID) +
		"&cliente_id=eq." + banco.Escapar(clienteID) +
		"&removido_em=is.null&select=id,chamado_id,ticket,conta,status," +
		"cotacao_trilogo_id,orcamento_trilogo_id,orcamento_valor," +
		"orcamento_arquivo_sha256,orcamento_arquivo_nome,pco_numero,nf_arquivo_sha256&limit=1"
	if err := s.bd.Buscar(ctx, caminho, &itens); err != nil {
		return ItemKanban{}, err
	}
	if len(itens) == 0 {
		return ItemKanban{}, fmt.Errorf("não achei este item no Kanban (ou já foi reclassificado pro contrato)")
	}
	return itens[0], nil
}

// ErrJaFaturado — o único ponto onde "sempre pode voltar pro contrato" para:
// depurar uma fatura emitida seria mexer em dinheiro que já saiu.
var ErrJaFaturado = fmt.Errorf("este serviço já foi faturado — não dá para reclassificar")

// Reclassificar volta um chamado pra fila do contrato. Não é só tirar o
// responsável: o PDF do orçamento sai do R2, o Budget lançado (se houver)
// some no Trílogo, e o card local perde os vínculos. A cotação em si o
// Trílogo não deixa apagar — só o orçamento.
//
// ORDEM
//
//	1. Apaga o Budget no Trílogo enquanto o chamado ainda é Serviço.
//	2. Apaga o PDF no R2 enquanto o card ainda está ativo — se o armazém
//	   recusar, o usuário aperta de novo e itemAtivo ainda acha a linha.
//	3. Tira o responsável (o chamado volta pra fila do contrato).
//	4. Fecha a linha (removido_em) e zera os vínculos do orçamento.
//
//	A fonte da verdade do "é Serviço?" continua sendo o Trílogo: se 4
//	falhar depois de 3, a próxima leitura do robô corrige a nossa tabela.
func (s *Servico) Reclassificar(ctx context.Context, clienteID, itemID, autorID, motivo string) error {
	item, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return err
	}
	if item.Status == StatusFaturado {
		return ErrJaFaturado
	}

	sessao, err := s.tri.SessaoDaConta(ctx, item.Conta)
	if err != nil {
		return fmt.Errorf("entrando no Trílogo: %w", err)
	}

	if item.OrcamentoTrilogoID != nil {
		if err := sessao.ExcluirOrcamentoServico(ctx, *item.OrcamentoTrilogoID); err != nil {
			return fmt.Errorf("excluindo o orçamento no Trílogo: %w", err)
		}
	}

	var shaDoArquivo string
	apagouDoArmazem := false
	if item.OrcamentoArquivoSHA256 != nil {
		shaDoArquivo = *item.OrcamentoArquivoSHA256
		apagou, err := s.apagarArquivoDeOrcamento(ctx, item.ID, shaDoArquivo)
		if err != nil {
			return fmt.Errorf("apagando o orçamento do armazém: %w", err)
		}
		apagouDoArmazem = apagou
	}

	if err := sessao.RemoverResponsavel(ctx, item.Ticket); err != nil {
		return fmt.Errorf("removendo o responsável no Trílogo: %w", err)
	}

	campos := map[string]any{
		"removido_em":              time.Now().UTC().Format(time.RFC3339),
		"removido_por":             nuloSeVazio(autorID),
		"removido_motivo":          nuloSeVazio(motivo),
		"orcamento_arquivo_sha256": nil,
		"orcamento_arquivo_nome":   nil,
		"orcamento_arquivo_em":     nil,
		"orcamento_trilogo_id":     nil,
		"orcamento_valor":          nil,
	}
	if err := s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos); err != nil {
		return err
	}
	if apagouDoArmazem && shaDoArquivo != "" {
		s.soltarRegistroDeArquivo(ctx, shaDoArquivo)
	}
	return nil
}

// PreencherPCO grava o pedido de compra que o cliente abriu pra este ticket
// faturar — mesmo conceito de faturas.pco_numero (migração 017), por ticket
// em vez de por ciclo (ver migração 053). NÃO muda `status`: o card já está
// em aguardando_faturamento, e é só o preenchimento de pco_numero que separa
// "Aguardando PCO" de "A faturar" na tela — dois sub-cards, um status só.
//
// PODE SER REESCRITO
//
//	Digitar errado e corrigir é o caso comum — travar a correção geraria mais
//	atrito que valor (ao contrário da nota fiscal, que dispara uma mudança de
//	status de verdade e por isso não tem "desfazer" nesta entrega).
func (s *Servico) PreencherPCO(ctx context.Context, clienteID, itemID, autorID, pco string) error {
	item, err := s.itemAtivo(ctx, clienteID, itemID)
	if err != nil {
		return err
	}
	if item.Status != StatusAguardandoFaturamento {
		return &ErrTransicaoInvalida{De: item.Status, Para: StatusAguardandoFaturamento}
	}
	campos := map[string]any{
		"pco_numero":         pco,
		"pco_preenchido_em":  time.Now().UTC().Format(time.RFC3339),
		"pco_preenchido_por": nuloSeVazio(autorID),
	}
	return s.bd.Atualizar(ctx, "servicos_orcamentos", "id=eq."+banco.Escapar(itemID), campos)
}

// ---------------------------------------------------------------------------
// Entrada manual — o botão "mandar para fila de serviços"
// ---------------------------------------------------------------------------
//
// DUAS PORTAS PARA O MESMO CORREDOR
//
//	Um chamado pode entrar no Kanban por dois caminhos: este (MarcarComoServico,
//	o clique) e RodarDeteccaoDeGatilho (a varredura agendada, que roda a cada
//	2h e alcança tanto quem clicou aqui quanto quem teve o responsável trocado
//	direto no Trílogo, por fora do nosso sistema). Os dois terminam escrevendo
//	a MESMA linha em servicos_orcamentos — por isso o cuidado com duplicação
//	mora nos dois: MarcarComoServico confere antes de agir (evita repetir a
//	troca no Trílogo), e o índice único parcial servicos_orcamentos_ativo
//	(migração 050) é quem garante, no banco, que só uma linha ativa sobrevive
//	mesmo se as duas entradas colidirem na mesma fração de segundo.

// chamadoPorTicket acha o id interno e a conta atual de um chamado pelo
// número do Trílogo — é como o botão (que só conhece o número da tela)
// alcança o chamado sem o front ter que saber nosso uuid.
func (s *Servico) chamadoPorTicket(ctx context.Context, clienteID string, ticket int) (chamadoID, conta string, err error) {
	var linhas []struct {
		ID    string `json:"id"`
		Conta string `json:"conta"`
	}
	caminho := "chamados?cliente_id=eq." + banco.Escapar(clienteID) +
		"&numero=eq." + strconv.Itoa(ticket) +
		"&select=id,conta&limit=1"
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return "", "", err
	}
	if len(linhas) == 0 {
		return "", "", fmt.Errorf("não achei o chamado %d", ticket)
	}
	return linhas[0].ID, linhas[0].Conta, nil
}

func (s *Servico) itemAtivoPorChamado(ctx context.Context, clienteID, chamadoID string) (*ItemKanban, error) {
	var itens []ItemKanban
	caminho := "servicos_orcamentos?chamado_id=eq." + banco.Escapar(chamadoID) +
		"&cliente_id=eq." + banco.Escapar(clienteID) +
		"&removido_em=is.null&select=id,chamado_id,ticket,conta,status&limit=1"
	if err := s.bd.Buscar(ctx, caminho, &itens); err != nil {
		return nil, err
	}
	if len(itens) == 0 {
		return nil, nil
	}
	return &itens[0], nil
}

// MarcarComoServico é a entrada MANUAL no Kanban: muda o responsável no
// Trílogo E grava a linha em servicos_orcamentos na hora — sem esperar a
// próxima rodada de RodarDeteccaoDeGatilho alcançar sozinha. A `conta` não
// vem do pedido: já está em chamados.conta (qual empresa atende esse chamado
// hoje), então não existe como o usuário escolher a conta errada.
//
// jaEstava=true quer dizer "não fiz nada, já estava na fila" — dois casos:
// achei uma linha ativa já gravada (não mexe no Trílogo de novo, evitando
// uma troca redundante e um rastro de histórico duplicado), ou o INSERT
// esbarrou no índice único da corrida rara com a varredura automática (aí o
// Trílogo já tinha sido mudado por quem venceu a corrida).
func (s *Servico) MarcarComoServico(ctx context.Context, clienteID string, ticket int) (rotulo string, jaEstava bool, err error) {
	chamadoID, conta, err := s.chamadoPorTicket(ctx, clienteID, ticket)
	if err != nil {
		return "", false, err
	}

	if ativo, err := s.itemAtivoPorChamado(ctx, clienteID, chamadoID); err != nil {
		return "", false, err
	} else if ativo != nil {
		rotulo, _ = s.tri.RotuloResponsavelServico(s.tri.IDResponsavelServicoDaConta(ativo.Conta))
		return rotulo, true, nil
	}

	assigneeID := s.tri.IDResponsavelServicoDaConta(conta)
	if assigneeID == 0 {
		return "", false, trilogo.ErrResponsavelNaoExiste
	}
	rotulo, _ = s.tri.RotuloResponsavelServico(assigneeID)

	sessao, err := s.tri.SessaoDaConta(ctx, conta)
	if err != nil {
		return "", false, fmt.Errorf("entrando no Trílogo: %w", err)
	}
	if err := sessao.MudarResponsavel(ctx, ticket, assigneeID); err != nil {
		return "", false, err
	}

	linha := map[string]any{
		"cliente_id": clienteID,
		"chamado_id": chamadoID,
		"ticket":     ticket,
		"conta":      conta,
		"status":     StatusAguardandoOrcamento,
		"origem":     "manual",
	}
	if err := s.bd.Inserir(ctx, "servicos_orcamentos", []map[string]any{linha}, nil); err != nil {
		if banco.Duplicado(err) {
			// A varredura automática venceu a corrida entre o clique e o
			// próprio Inserir — o Trílogo já está certo, a linha já existe.
			return rotulo, true, nil
		}
		return rotulo, false, fmt.Errorf("mudei o responsável no Trílogo, mas não consegui gravar no Kanban (a próxima rodada alcança): %w", err)
	}
	_ = s.bd.Atualizar(ctx, "servicos_candidatos",
		"chamado_id=eq."+banco.Escapar(chamadoID)+"&status=eq.pendente",
		map[string]any{"status": "resolvido", "decidido_em": time.Now().UTC().Format(time.RFC3339)})

	return rotulo, false, nil
}

// ---------------------------------------------------------------------------
// O gatilho automático
// ---------------------------------------------------------------------------

type chamadoComResponsavel struct {
	ID          string `json:"id"`
	Ticket      int    `json:"numero"`
	Conta       string `json:"conta"`
	Responsavel string `json:"responsavel"`
}

// RodarDeteccaoDeGatilho é a outra metade do job agendado: varre os
// chamados cujo `responsavel` (já lido pelo robô do Trílogo — trilogo.Rodar)
// bate com um dos dois nomes especiais, e insere no Kanban quem ainda não
// está lá. Cobre os DOIS caminhos de entrada: alguém trocou o responsável
// direto no Trílogo, OU pelo botão desta tela (que já mudou o Trílogo —
// esta varredura só precisa ALCANÇAR, não decidir de novo).
//
// O FILTRO DE DATA É POR `alterado_em`, NÃO POR `criado_em`
//
//	CorteServico não é "chamado tem que ser novo" — é "a troca de
//	responsável pra Serviço tem que ser recente" (decisão do dono,
//	04/09/2026). `chamados.alterado_em` é o `dateOfLastChange` do Trílogo, e
//	ele muda toda vez que o responsável é trocado — então um chamado de 2025
//	entra aqui normalmente, contanto que a mudança de responsável (a marcação
//	manual pelo botão, ou uma troca direta no Trílogo) tenha acontecido
//	depois do corte. Um chamado antigo cujo responsável nunca mudou desde
//	então fica de fora, porque `alterado_em` dele continua velho.
func (s *Servico) RodarDeteccaoDeGatilho(ctx context.Context, clienteID string) (detectados int, err error) {
	nomes := []string{}
	if n := s.cfg.Trilogo.NomeResponsavelServicoInstalacoes; n != "" {
		nomes = append(nomes, n)
	}
	if n := s.cfg.Trilogo.NomeResponsavelServicoCivil; n != "" {
		nomes = append(nomes, n)
	}
	if len(nomes) == 0 {
		return 0, nil
	}

	var comResponsavel []chamadoComResponsavel
	caminho := "chamados?cliente_id=eq." + banco.Escapar(clienteID) +
		"&alterado_em=gte." + CorteServico +
		"&responsavel=in.(" + listaEntreParenteses(nomes) + ")" +
		"&select=id,numero,conta,responsavel"
	if err := s.bd.Buscar(ctx, caminho, &comResponsavel); err != nil {
		return 0, fmt.Errorf("lendo chamados com responsável de Serviço: %w", err)
	}
	if len(comResponsavel) == 0 {
		return 0, nil
	}

	ids := make([]string, len(comResponsavel))
	for i, c := range comResponsavel {
		ids[i] = c.ID
	}
	var ativos []struct {
		ChamadoID string `json:"chamado_id"`
	}
	caminhoAtivos := "servicos_orcamentos?chamado_id=in.(" + listaEntreParenteses(ids) + ")" +
		"&removido_em=is.null&select=chamado_id"
	if err := s.bd.Buscar(ctx, caminhoAtivos, &ativos); err != nil {
		return 0, fmt.Errorf("conferindo quem já está no Kanban: %w", err)
	}
	jaAtivo := map[string]bool{}
	for _, a := range ativos {
		jaAtivo[a.ChamadoID] = true
	}

	for _, c := range comResponsavel {
		if jaAtivo[c.ID] {
			continue
		}
		linha := map[string]any{
			"cliente_id": clienteID,
			"chamado_id": c.ID,
			"ticket":     c.Ticket,
			"conta":      c.Conta,
			"status":     StatusAguardandoOrcamento,
			"origem":     "gatilho",
		}
		if err := s.bd.Inserir(ctx, "servicos_orcamentos", []map[string]any{linha}, nil); err != nil {
			if banco.Duplicado(err) {
				// Alguém marcou este chamado manualmente (MarcarComoServico)
				// entre a checagem de jaAtivo, ali em cima, e este INSERT —
				// o Trílogo já está certo, a linha já existe, não é erro.
				continue
			}
			return detectados, fmt.Errorf("gravando o ticket %d no Kanban: %w", c.Ticket, err)
		}
		// O candidato correspondente (se veio desse caminho) fecha o ciclo —
		// deixa de aparecer como "pendente" na tela de Candidatos, mesmo que
		// ninguém tenha clicado em nada lá: o gatilho direto no Trílogo
		// resolveu por fora.
		_ = s.bd.Atualizar(ctx, "servicos_candidatos",
			"chamado_id=eq."+banco.Escapar(c.ID)+"&status=eq.pendente",
			map[string]any{"status": "resolvido", "decidido_em": time.Now().UTC().Format(time.RFC3339)})
		detectados++
	}
	return detectados, nil
}

// PromoverCandidato é o atalho da tela de Candidatos: aprova o palpite do
// Groq de uma vez — muda o responsável no Trílogo E já cria a linha do
// Kanban, sem esperar a próxima rodada do gatilho automático encontrar
// sozinha.
func (s *Servico) PromoverCandidato(ctx context.Context, clienteID, candidatoID, conta, autorID string) (rotulo string, err error) {
	cand, err := s.candidatoPendente(ctx, clienteID, candidatoID)
	if err != nil {
		return "", err
	}

	assigneeID := s.tri.IDResponsavelServicoDaConta(conta)
	if assigneeID == 0 {
		return "", trilogo.ErrResponsavelNaoExiste
	}
	rotulo, _ = s.tri.RotuloResponsavelServico(assigneeID)

	sessao, err := s.tri.SessaoDaConta(ctx, conta)
	if err != nil {
		return "", fmt.Errorf("entrando no Trílogo: %w", err)
	}
	if err := sessao.MudarResponsavel(ctx, cand.Ticket, assigneeID); err != nil {
		return "", fmt.Errorf("mudando o responsável no Trílogo: %w", err)
	}

	agora := time.Now().UTC().Format(time.RFC3339)
	linha := map[string]any{
		"cliente_id": clienteID,
		"chamado_id": cand.ChamadoID,
		"ticket":     cand.Ticket,
		"conta":      conta,
		"status":     StatusAguardandoOrcamento,
		"origem":     "candidato",
	}
	if err := s.bd.Inserir(ctx, "servicos_orcamentos", []map[string]any{linha}, nil); err != nil && !banco.Duplicado(err) {
		// O Trílogo JÁ mudou — não desfaz sozinho (ver Reclassificar: a fonte
		// da verdade é o Trílogo, e a próxima varredura de gatilho alcança
		// mesmo se esta gravação falhar aqui). banco.Duplicado quer dizer que
		// o gatilho automático (ou o botão) já ganhou a corrida e gravou a
		// linha primeiro — não é erro, só segue e fecha o candidato.
		return rotulo, fmt.Errorf("mudei o responsável no Trílogo, mas não consegui gravar no Kanban (a próxima rodada alcança): %w", err)
	}
	_ = s.bd.Atualizar(ctx, "servicos_candidatos", "id=eq."+banco.Escapar(candidatoID),
		map[string]any{"status": "resolvido", "decidido_em": agora, "decidido_por": nuloSeVazio(autorID)})

	return rotulo, nil
}

// listaEntreParenteses monta o `in.(a,b,c)` do PostgREST com cada item entre
// aspas ESCAPADAS — mesmo padrão de acesso.go/listaPara: sem o escape, "Serviço
// Instalações" (espaço, cedilha) e qualquer valor com vírgula quebrariam o
// filtro depois de montado na URL.
func listaEntreParenteses(valores []string) string {
	partes := make([]string, 0, len(valores))
	for _, v := range valores {
		partes = append(partes, banco.Escapar(`"`+v+`"`))
	}
	return strings.Join(partes, ",")
}
