//go:build integracao

package administrativo

import (
	"context"
	"fmt"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// TestLimparDadosTestePCO apaga OCs 20001–20030, itens, PDFs órfãos no R2 e
// linhas de arquivos — DELETE de verdade, não soft delete.
// TestEstadoComprasNoBanco lista quantas OCs existem por status (diagnóstico).
func TestEstadoComprasNoBanco(t *testing.T) {
	cfg, err := config.Carregar()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	bd := banco.Novo(cfg)
	var clientes []map[string]any
	if err := bd.Buscar(ctx, "clientes?select=id&limit=1", &clientes); err != nil {
		t.Fatalf("clientes: %v", err)
	}
	if len(clientes) == 0 {
		t.Fatal("nenhum cliente no banco")
	}
	clienteID, _ := clientes[0]["id"].(string)
	base := "ordens_compra?cliente_id=eq." + banco.Escapar(clienteID)
	for _, par := range []struct{ nome, filtro string }{
		{"inserido", base + "&status=eq.inserido&select=id&limit=1"},
		{"lendo", base + "&status=eq.lendo&select=id&limit=1"},
		{"lido", base + "&status=eq.lido&select=id&limit=1"},
		{"falhou", base + "&status=eq.falhou&select=id&limit=1"},
	} {
		n, err := bd.BuscarContando(ctx, par.filtro, nil)
		if err != nil {
			t.Fatalf("%s: %v", par.nome, err)
		}
		t.Logf("%s: %d", par.nome, n)
	}
	var presas []map[string]any
	if err := bd.Buscar(ctx, base+"&status=eq.lendo&select=nome_arquivo&order=criado_em", &presas); err != nil {
		t.Fatal(err)
	}
	for _, o := range presas {
		t.Logf("  presa: %s", o["nome_arquivo"])
	}
}

func TestLimparDadosTestePCO(t *testing.T) {
	cfg, err := config.Carregar()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	bd := banco.Novo(cfg)
	var arm *armazem.Cliente
	if cfg.R2.Ligado() {
		arm = armazem.Novo(cfg)
	}
	var clientes []map[string]any
	if err := bd.Buscar(ctx, "clientes?select=id&limit=1", &clientes); err != nil {
		t.Fatalf("clientes: %v", err)
	}
	if len(clientes) == 0 {
		t.Fatal("nenhum cliente no banco")
	}
	clienteID, _ := clientes[0]["id"].(string)
	n, err := limparDadosTestePCO(ctx, bd, arm, clienteID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("limpeza concluída: %d ordem(ns) de compra apagada(s)", n)
}

// limparDadosTestePCO remove rastros do teste das 30 OCs (números 20001–20030).
// Histórico: a tabela `historico` é imutável por design (migração 005) — o
// motor não consegue apagar essas linhas; ficam órfãs, só consulta de auditoria.
func limparDadosTestePCO(ctx context.Context, bd *banco.Cliente, arm *armazem.Cliente, clienteID string) (int, error) {
	// Por número (OCs que fecharam leitura) e por nome de arquivo (OCs que
	// ficaram presas em "lendo" quando o UPDATE falhou antes de gravar numero).
	caminhos := []string{
		"ordens_compra?cliente_id=eq." + banco.Escapar(clienteID) +
			"&numero=gte.20001&numero=lte.20030&select=id,arquivo_sha256",
		"ordens_compra?cliente_id=eq." + banco.Escapar(clienteID) +
			"&nome_arquivo=like.OC_200*&select=id,arquivo_sha256",
	}
	vistos := map[string]struct{}{}
	var ordens []map[string]any
	for _, caminho := range caminhos {
		var lote []map[string]any
		if err := bd.Buscar(ctx, caminho, &lote); err != nil {
			return 0, fmt.Errorf("buscar OCs de teste: %w", err)
		}
		for _, o := range lote {
			id, _ := o["id"].(string)
			if id == "" {
				continue
			}
			if _, ja := vistos[id]; ja {
				continue
			}
			vistos[id] = struct{}{}
			ordens = append(ordens, o)
		}
	}
	if len(ordens) == 0 {
		return 0, nil
	}

	shas := map[string]struct{}{}
	for _, o := range ordens {
		id, _ := o["id"].(string)
		if id == "" {
			continue
		}
		if sha, _ := o["arquivo_sha256"].(string); sha != "" {
			shas[sha] = struct{}{}
		}
		if err := bd.Apagar(ctx, "ordens_compra_itens", "ordem_compra_id=eq."+id); err != nil {
			return 0, fmt.Errorf("apagar itens da OC %s: %w", id, err)
		}
		if err := bd.Apagar(ctx, "ordens_compra", "id=eq."+id); err != nil {
			return 0, fmt.Errorf("apagar OC %s: %w", id, err)
		}
	}

	for sha := range shas {
		if err := apagarArquivoSeOrfao(ctx, bd, arm, sha); err != nil {
			return len(ordens), fmt.Errorf("arquivo %s: %w", sha[:8], err)
		}
	}
	return len(ordens), nil
}

func apagarArquivoSeOrfao(ctx context.Context, bd *banco.Cliente, arm *armazem.Cliente, sha string) error {
	esc := banco.Escapar(sha)
	var refs []map[string]any
	if err := bd.Buscar(ctx, "ordens_compra?arquivo_sha256=eq."+esc+"&select=id&limit=1", &refs); err != nil {
		return err
	}
	if len(refs) > 0 {
		return nil
	}
	if err := bd.Buscar(ctx, "documentos?arquivo_sha256=eq."+esc+"&select=id&limit=1", &refs); err != nil {
		return err
	}
	if len(refs) > 0 {
		return nil
	}

	var arqs []map[string]any
	if err := bd.Buscar(ctx, "arquivos?sha256=eq."+esc+"&select=chave_r2&limit=1", &arqs); err != nil {
		return err
	}
	if len(arqs) == 0 {
		return nil
	}
	chave, _ := arqs[0]["chave_r2"].(string)
	if arm != nil && chave != "" {
		_ = arm.Apagar(ctx, chave) // 404 no R2 não é erro
	}
	return bd.Apagar(ctx, "arquivos", "sha256=eq."+esc)
}
