// rev 2 — Candidatos, o gatilho automático e o avanço de Execução/Vistoria
//
// MESMO PADRÃO do cmd/robo: um binário que roda direto no GitHub Actions, sem
// passar pelo motor HTTP — trabalho de rodada agendada, não de clique (ver o
// cabeçalho de cmd/robo/main.go). Três coisas por rodada, nesta ordem:
//
//  1. RodarDeteccaoDeGatilho — alcança quem já entrou na fila de Serviço de
//     verdade (responsável trocado no Trílogo, direto ou pelo botão) e ainda
//     não tem cartão no Kanban.
//  2. RodarClassificacao — pergunta pro Groq sobre quem é chamado NOVO e
//     ainda não foi nem candidato nem serviço.
//  3. RodarDeteccaoDeAvanco — avança sozinho quem já está em Execução e o
//     robô do Trílogo (rodado 5 minutos antes, ver .github/workflows/
//     servicos.yml) já leu o marco que manda andar (status_codigo,
//     executado_em, vistoriado_em — ver deteccao.go).
//
// A ORDEM IMPORTA: o gatilho primeiro fecha os candidatos que já foram
// resolvidos por fora (ver kanban.go), então a classificação não desperdiça
// uma chamada ao Groq num chamado que já tem resposta definitiva. O avanço de
// Execução vem por último porque não depende de nenhum dos dois — só dos
// dados que o robô do contrato já deixou prontos em `chamados`.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/servicos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
)

func main() {
	log.SetFlags(log.Ltime)

	slug := os.Getenv("CLIENTE_SLUG")
	if slug == "" {
		slug = "frota-macedo"
	}

	cfg, err := config.Carregar()
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.Groq.Ligado() {
		// Não é fatal: a detecção do gatilho não depende do Groq. Só avisa e
		// segue — RodarClassificacao recusa sozinha, mais abaixo.
		log.Print("aviso: GROQ_API_KEY não configurada — só a detecção de gatilho vai rodar")
	}

	bd := banco.Novo(cfg)
	arm := armazem.Novo(cfg)
	tri := trilogo.Novo(cfg, bd, arm)
	svc := servicos.Novo(cfg, bd, tri, arm)

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	clienteID, err := clienteDoSlug(ctx, bd, slug)
	if err != nil {
		log.Fatal(err)
	}

	execID, err := abrirExecucao(ctx, bd, clienteID)
	if err != nil {
		// Não sobe: um rastro perdido não é motivo para deixar de classificar
		// candidato nenhum.
		log.Printf("aviso: não consegui abrir a rodada em robo_execucoes: %v", err)
	}

	comeco := time.Now()
	detectados, err := svc.RodarDeteccaoDeGatilho(ctx, clienteID)
	if err != nil {
		fecharExecucao(ctx, bd, execID, "falhou", err.Error())
		log.Fatalf("detecção do gatilho: %v", err)
	}
	log.Printf("gatilho automático: %d chamado(s) novo(s) no Kanban", detectados)

	var avaliados, marcados int
	if cfg.Groq.Ligado() {
		avaliados, marcados, err = svc.RodarClassificacao(ctx, clienteID)
		if err != nil {
			fecharExecucao(ctx, bd, execID, "falhou", err.Error())
			log.Fatalf("classificação de candidatos: %v", err)
		}
	}
	log.Printf("candidatos: %d avaliado(s), %d virou(ram) candidato", avaliados, marcados)

	avancados, err := svc.RodarDeteccaoDeAvanco(ctx, clienteID)
	if err != nil {
		fecharExecucao(ctx, bd, execID, "falhou", err.Error())
		log.Fatalf("avanço de execução: %v", err)
	}
	log.Printf("execução: %d card(s) avançou(aram) sozinho(s)", avancados)

	fecharExecucao(ctx, bd, execID, "concluida", "")
	log.Printf("=== FIM em %s === gatilho %d · avaliados %d · candidatos %d · avançados %d",
		time.Since(comeco).Round(time.Second), detectados, avaliados, marcados, avancados)
}

func clienteDoSlug(ctx context.Context, bd *banco.Cliente, slug string) (string, error) {
	var linhas []struct {
		ID string `json:"id"`
	}
	if err := bd.Buscar(ctx, "clientes?slug=eq."+banco.Escapar(slug)+"&select=id&limit=1", &linhas); err != nil {
		return "", err
	}
	if len(linhas) == 0 {
		return "", fmt.Errorf("não achei o cliente %q", slug)
	}
	return linhas[0].ID, nil
}

// abrirExecucao / fecharExecucao deixam rastro em robo_execucoes, com
// robo='servicos' — a mesma tabela do robô do Trílogo (migração 007),
// discriminada pela coluna `robo`. Sem trava de concorrência aqui: esta
// rodada não compete com clique nenhum de usuário (ao contrário do robô do
// Trílogo, que tem o botão "Atualizar agora") — a proteção contra duas
// rodadas ao mesmo tempo já vem do `concurrency` do próprio workflow.
func abrirExecucao(ctx context.Context, bd *banco.Cliente, clienteID string) (string, error) {
	var fora []struct {
		ID string `json:"id"`
	}
	linha := map[string]any{
		"cliente_id":    clienteID,
		"robo":          "servicos",
		"modo":          "candidatos",
		"situacao":      "rodando",
		"disparado_por": "actions",
	}
	if err := bd.Inserir(ctx, "robo_execucoes", []map[string]any{linha}, &fora); err != nil || len(fora) == 0 {
		return "", err
	}
	return fora[0].ID, nil
}

func fecharExecucao(ctx context.Context, bd *banco.Cliente, execID, situacao, erro string) {
	if execID == "" {
		return
	}
	campos := map[string]any{
		"situacao":    situacao,
		"terminou_em": time.Now().UTC().Format(time.RFC3339),
	}
	if erro != "" {
		campos["erro"] = erro
	}
	if err := bd.Atualizar(ctx, "robo_execucoes", "id=eq."+banco.Escapar(execID), campos); err != nil {
		log.Printf("aviso: não consegui fechar a rodada em robo_execucoes: %v", err)
	}
}
