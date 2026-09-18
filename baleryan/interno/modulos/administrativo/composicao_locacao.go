// rev 1 — a composição de uma NF de locação: OC + romaneio + fotos (18/09/2026)
//
// O QUE O DONO PEDIU
//
//	Um link em cada linha de "Aguardando NF de locação" que abre, em tela
//	cheia, os três documentos que provam o recebimento — a OC, o romaneio
//	escaneado e as fotos do equipamento — como se fossem folhas A4
//	empilhadas. O desenho da folha (quantas fotos por página, retrato vs
//	paisagem) é todo do FRONT (`VisualizarComposicaoLocacao.tsx`); este
//	arquivo só entrega os três em endereços temporários, prontos pra
//	desenhar.
//
// POR QUE NÃO IMPORTA NADA DO MÓDULO locacoes (P-13)
//
//	`locacoes_recebimentos`/`locacoes_equipamentos`/`locacoes_fotos` são
//	tabelas do módulo Locações, mas nenhum tipo Go de lá é importado aqui —
//	mesma disciplina do resto do sistema: cada módulo lê a tabela do outro
//	direto pelo banco (`m.bd`), nunca pela função Go do vizinho.
package administrativo

import (
	"context"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// linkDoArquivo resolve um sha256 pra um endereço temporário do armazém —
// mesma receita usada em meia dúzia de lugares deste módulo
// (`arquivoDaOrdem`, `arquivoDaNF`...), só que como função à parte porque
// esta rota precisa chamá-la várias vezes (a OC, cada página do romaneio,
// cada foto) em vez de uma só.
func (m *Modulo) linkDoArquivo(ctx context.Context, sha string) (string, error) {
	if sha == "" {
		return "", nil
	}
	arq, err := m.contarUm(ctx, "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		return "", err
	}
	chave, _ := arq["chave_r2"].(string)
	if chave == "" {
		return "", nil
	}
	return m.arm.LinkTemporario(chave, ValidadeDoLink)
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/notas/{id}/composicao-locacao
// ---------------------------------------------------------------------------

func (m *Modulo) composicaoLocacaoDaNF(w http.ResponseWriter, r *http.Request) {
	// Mesma dupla rotina que já lê "Aguardando NF de locação" — nasce direto
	// no escritório, o almoxarife nunca vê esta fila (ver o cabeçalho de
	// `listarNF`).
	p := m.quemComQualquerRotina(w, r, RotinaNFEntregar, RotinaNFEnviarCliente)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	nf, err := m.contarUm(r.Context(), "notas_fiscais?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&origem=eq.locacao&select=id,ordem_compra_id&limit=1")
	if err != nil {
		m.erro(w, "não achei esta nota fiscal de locação", err)
		return
	}
	oid := strCampo(nf["ordem_compra_id"])
	if oid == "" {
		web.Falhar(w, http.StatusNotFound, "Esta nota fiscal não tem ordem de compra associada.")
		return
	}

	ctx := r.Context()

	// A OC — o mesmo PDF que Compras já mostra em qualquer outra tela.
	ordem, err := m.contarUm(ctx, "ordens_compra?id=eq."+banco.Escapar(oid)+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=nome_arquivo,arquivo_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei a ordem de compra desta locação", err)
		return
	}
	ocURL, err := m.linkDoArquivo(ctx, strCampo(ordem["arquivo_sha256"]))
	if err != nil {
		m.erro(w, "não consegui montar o endereço da OC", err)
		return
	}

	// O recebimento — de onde saem o romaneio e os equipamentos (pra achar
	// as fotos). `ordem_compra_id` é `unique` em `locacoes_recebimentos`,
	// então esta é sempre a linha certa.
	receb, err := m.contarUm(ctx, "locacoes_recebimentos?ordem_compra_id=eq."+banco.Escapar(oid)+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,romaneio_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei o recebimento desta locação", err)
		return
	}
	recebID := strCampo(receb["id"])

	// Romaneio: página 1 é `romaneio_sha256`, as seguintes vêm de
	// `locacoes_recebimentos_paginas`, na ordem — mesmo desenho de
	// `notas_fiscais`/`notas_fiscais_paginas` que este módulo já usa.
	var romaneioURLs []string
	if u, err := m.linkDoArquivo(ctx, strCampo(receb["romaneio_sha256"])); err == nil && u != "" {
		romaneioURLs = append(romaneioURLs, u)
	}
	var paginasRomaneio []map[string]any
	if recebID != "" {
		_ = m.bd.Buscar(ctx, "locacoes_recebimentos_paginas?recebimento_id=eq."+banco.Escapar(recebID)+
			"&order=pagina&select=arquivo_sha256", &paginasRomaneio)
	}
	for _, pg := range paginasRomaneio {
		if u, err := m.linkDoArquivo(ctx, strCampo(pg["arquivo_sha256"])); err == nil && u != "" {
			romaneioURLs = append(romaneioURLs, u)
		}
	}

	// Fotos — uma por equipamento recebido, evento "recebimento". Um
	// recebimento pode ter mais de um equipamento (uma OC com vários itens
	// locados), por isso o `in.(...)`.
	fotos := []map[string]any{}
	if recebID != "" {
		var equipamentos []map[string]any
		if err := m.bd.Buscar(ctx, "locacoes_equipamentos?recebimento_id=eq."+banco.Escapar(recebID)+"&select=id", &equipamentos); err == nil && len(equipamentos) > 0 {
			ids := make([]string, 0, len(equipamentos))
			for _, e := range equipamentos {
				if eid := strCampo(e["id"]); eid != "" {
					ids = append(ids, eid)
				}
			}
			if len(ids) > 0 {
				var linhas []map[string]any
				if err := m.bd.Buscar(ctx, "locacoes_fotos?equipamento_id=in.("+strings.Join(ids, ",")+")"+
					"&evento=eq.recebimento&order=criado_em&select=id,arquivo_sha256", &linhas); err == nil {
					for _, l := range linhas {
						u, err := m.linkDoArquivo(ctx, strCampo(l["arquivo_sha256"]))
						if err != nil || u == "" {
							continue
						}
						fotos = append(fotos, map[string]any{"id": l["id"], "url": u})
					}
				}
			}
		}
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"oc":       map[string]any{"url": ocURL, "nome": ordem["nome_arquivo"]},
		"romaneio": map[string]any{"paginas": romaneioURLs},
		"fotos":    fotos,
	})
}
