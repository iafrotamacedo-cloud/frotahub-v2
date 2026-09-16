// rev 1 — GET .../itens e POST .../receber: a bifurcação vira dado
//
// UMA OC, UM RECEBIMENTO DE LOCAÇÃO, N EQUIPAMENTOS
//
//	`itensDaOrdem` pré-preenche a tela (fornecedor, obra, itens da OC —
//	mesmos que `ordens_compra_itens` já tem da leitura normal, ver a 059).
//	`receber` sobe TODOS os arquivos antes de qualquer INSERT (mesma
//	disciplina de `receberNF`): se uma foto falhar no meio, nada foi criado.
//	Cada item da OC vira um `locacoes_equipamentos` com o período 1 em
//	`locacoes_periodos`, e só no fim a OC ganha `destino_recebimento =
//	'locacao'` — é essa coluna que a tira de "Aguardando NF" (view 065/072).
package locacoes

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// GET /locacoes/ordens/{id}/itens
// ---------------------------------------------------------------------------

func (m *Modulo) itensDaOrdem(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaReceber)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	oc, err := m.ordemProntaParaLocacao(r, p, id)
	if err != nil {
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	}

	var itens []map[string]any
	if err := m.bd.Buscar(r.Context(), "ordens_compra_itens?ordem_compra_id=eq."+id+
		"&select=id,descricao,unidade,qtd,valor_unit&order=descricao", &itens); err != nil {
		m.erro(w, "não consegui listar os itens desta ordem de compra", err)
		return
	}

	fornecedorNome := ""
	if fid := strCampo(oc["fornecedor_id"]); fid != "" {
		if f, err := m.contarUm(r.Context(), "fornecedores?id=eq."+banco.Escapar(fid)+"&select=razao_social&limit=1"); err == nil {
			fornecedorNome = strCampo(f["razao_social"])
		}
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"ordem": map[string]any{
			"id":                oc["id"],
			"numero":            oc["numero"],
			"obra_centro_custo": oc["obra_centro_custo"],
			"fornecedor_nome":   fornecedorNome,
		},
		"itens": ouVazio(itens),
	})
}

// ordemProntaParaLocacao confere: existe, é do meu cliente, já passou pela
// leitura e pelo PCO, ainda não foi recebida (nem como NF nem como locação),
// e o login tem acesso à obra dela. Mesmo bloqueio otimista em espírito de
// `ordemProntaParaReceber` em administrativo/nf_era.go.
func (m *Modulo) ordemProntaParaLocacao(r *http.Request, p *seguranca.Principal, id string) (map[string]any, error) {
	oc, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,numero,obra_centro_custo,fornecedor_id,status,pco_enviado_em,destino_recebimento&limit=1")
	if err != nil {
		return nil, fmt.Errorf("não achei esta ordem de compra")
	}
	if strCampo(oc["destino_recebimento"]) != "" {
		return nil, fmt.Errorf("esta OC já foi recebida")
	}
	if strCampo(oc["status"]) != "lido" || oc["pco_enviado_em"] == nil {
		return nil, fmt.Errorf("esta OC ainda não está pronta para ser recebida")
	}
	ok, err := m.temAcessoAObra(r.Context(), p, strCampo(oc["obra_centro_custo"]))
	if err != nil {
		return nil, fmt.Errorf("não consegui conferir seu acesso a esta obra")
	}
	if !ok {
		return nil, fmt.Errorf("você não tem acesso a esta obra")
	}
	return oc, nil
}

// ---------------------------------------------------------------------------
// POST /locacoes/ordens/{id}/receber
// ---------------------------------------------------------------------------

type itemRecebido struct {
	ItemID      string  `json:"item_id"`
	Descricao   string  `json:"descricao"`
	Unidade     string  `json:"unidade"`
	QtdRecebida float64 `json:"qtd_recebida"`
	ValorUnit   float64 `json:"valor_unit"`
}

var periodicidadesValidas = map[string]bool{"mensal": true, "quinzenal": true, "semanal": true}

func somarPeriodo(inicio time.Time, periodicidade string) time.Time {
	switch periodicidade {
	case "quinzenal":
		return inicio.AddDate(0, 0, 15)
	case "semanal":
		return inicio.AddDate(0, 0, 7)
	default: // "mensal"
		return inicio.AddDate(0, 1, 0)
	}
}

func (m *Modulo) receber(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaReceber)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado. Sem ele, receber a locação seria perdê-la.")
		return
	}
	if _, err := m.ordemProntaParaLocacao(r, p, id); err != nil {
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	}

	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o que foi enviado.")
		return
	}

	periodicidade := strings.TrimSpace(r.FormValue("periodicidade"))
	if periodicidade == "" {
		periodicidade = "mensal"
	}
	if !periodicidadesValidas[periodicidade] {
		web.Falhar(w, http.StatusBadRequest, "Prazo inválido — escolha mensal, quinzenal ou semanal.")
		return
	}

	var itens []itemRecebido
	if err := json.Unmarshal([]byte(r.FormValue("itens")), &itens); err != nil || len(itens) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Informe ao menos um equipamento recebido.")
		return
	}
	for i, it := range itens {
		if _, ok := umUUID(it.ItemID); !ok {
			web.Falhar(w, http.StatusBadRequest, "Item inválido na lista recebida.")
			return
		}
		if it.QtdRecebida <= 0 {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Informe a quantidade recebida de %s.", descricaoOuItem(it, i)))
			return
		}
	}

	// O ROMANEIO — PROVA DE ENTRADA, OBRIGATÓRIO (o equivalente à NF aqui)
	primeira := r.MultipartForm.File["romaneio"]
	if len(primeira) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escaneie o romaneio de entrega.")
		return
	}
	cabecalhosRomaneio := append([]*multipart.FileHeader{primeira[0]}, r.MultipartForm.File["romaneio_paginas"]...)
	type paginaLida struct{ sha string }
	paginasRomaneio := make([]paginaLida, 0, len(cabecalhosRomaneio))
	for i, cab := range cabecalhosRomaneio {
		sha, err := m.lerEGuardarArquivo(r.Context(), p, cab)
		if err != nil {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Romaneio, página %d: %s.", i+1, err.Error()))
			return
		}
		paginasRomaneio = append(paginasRomaneio, paginaLida{sha: sha})
	}

	// A NF — OPCIONAL AQUI (pode vir junto com o equipamento, mesmo sem ser
	// o documento de entrada; ver o cabeçalho da migração 072).
	nfNumero := strings.TrimSpace(r.FormValue("nf_numero"))
	nfSha := ""
	if cabs := r.MultipartForm.File["nf"]; len(cabs) > 0 {
		sha, err := m.lerEGuardarArquivo(r.Context(), p, cabs[0])
		if err != nil {
			web.Falhar(w, http.StatusBadRequest, "Nota fiscal: "+err.Error())
			return
		}
		nfSha = sha
	}

	// FOTOS POR ITEM — PELO MENOS UMA CADA (decisão fechada com o dono)
	fotosPorItem := make(map[string][]string, len(itens))
	for i, it := range itens {
		cabs := r.MultipartForm.File["fotos_"+it.ItemID]
		if len(cabs) == 0 {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Tire ao menos uma foto de %s.", descricaoOuItem(it, i)))
			return
		}
		shas := make([]string, 0, len(cabs))
		for j, cab := range cabs {
			sha, err := m.lerEGuardarArquivo(r.Context(), p, cab)
			if err != nil {
				web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Foto %d de %s: %s.", j+1, descricaoOuItem(it, i), err.Error()))
				return
			}
			shas = append(shas, sha)
		}
		fotosPorItem[it.ItemID] = shas
	}

	// A DATA É SEMPRE HOJE — SEM CAMPO NA TELA
	//
	//	Decisão do dono: só o builder muda isto, direto pelo backend
	//	(PATCH /locacoes/recebimentos/{id}, fase futura) — a tela não tem
	//	campo de data de propósito, pra não virar hábito editar o dia do
	//	recebimento.
	hoje := time.Now().UTC().Truncate(24 * time.Hour)
	vencimento := somarPeriodo(hoje, periodicidade)

	oc, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&select=fornecedor_id,obra_centro_custo&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}

	linhaRecebimento := map[string]any{
		"cliente_id":       p.ClienteID,
		"ordem_compra_id":  id,
		"data_recebimento": hoje.Format("2006-01-02"),
		"recebido_por":     p.UserID,
		"romaneio_sha256":  paginasRomaneio[0].sha,
	}
	if nfNumero != "" {
		linhaRecebimento["nf_numero"] = nfNumero
	}
	if nfSha != "" {
		linhaRecebimento["nf_sha256"] = nfSha
	}
	var criados []map[string]any
	if err := m.bd.Inserir(r.Context(), "locacoes_recebimentos", []map[string]any{linhaRecebimento}, &criados); err != nil {
		m.erro(w, "não consegui gravar o recebimento", err)
		return
	}
	if len(criados) == 0 {
		m.erro(w, "gravei o recebimento mas o banco não devolveu o id", fmt.Errorf("insert sem retorno"))
		return
	}
	recebimentoID := strCampo(criados[0]["id"])

	if len(paginasRomaneio) > 1 {
		linhasPaginas := make([]map[string]any, 0, len(paginasRomaneio)-1)
		for i := 1; i < len(paginasRomaneio); i++ {
			linhasPaginas = append(linhasPaginas, map[string]any{
				"cliente_id":     p.ClienteID,
				"recebimento_id": recebimentoID,
				"pagina":         i + 1,
				"arquivo_sha256": paginasRomaneio[i].sha,
			})
		}
		if err := m.bd.Inserir(r.Context(), "locacoes_recebimentos_paginas", linhasPaginas, nil); err != nil {
			m.erro(w, "gravei o recebimento mas não consegui registrar as páginas seguintes do romaneio", err)
			return
		}
	}

	equipamentosCriados := 0
	for _, it := range itens {
		linhaEquipamento := map[string]any{
			"cliente_id":           p.ClienteID,
			"recebimento_id":       recebimentoID,
			"ordem_compra_id":      id,
			"ordem_compra_item_id": it.ItemID,
			"fornecedor_id":        oc["fornecedor_id"],
			"obra_centro_custo":    oc["obra_centro_custo"],
			"descricao":            it.Descricao,
			"unidade":              it.Unidade,
			"qtd_recebida":         it.QtdRecebida,
			"qtd_ativa":            it.QtdRecebida,
			"valor_unit":           it.ValorUnit,
			"periodicidade":        periodicidade,
			"data_inicio":          hoje.Format("2006-01-02"),
			"vencimento_atual":     vencimento.Format("2006-01-02"),
			"estado":               "ativo",
		}
		var criadosEquip []map[string]any
		if err := m.bd.Inserir(r.Context(), "locacoes_equipamentos", []map[string]any{linhaEquipamento}, &criadosEquip); err != nil {
			m.erro(w, fmt.Sprintf("gravei parte do recebimento mas falhei no equipamento %q", it.Descricao), err)
			return
		}
		if len(criadosEquip) == 0 {
			m.erro(w, "gravei um equipamento mas o banco não devolveu o id", fmt.Errorf("insert sem retorno"))
			return
		}
		equipamentoID := strCampo(criadosEquip[0]["id"])
		equipamentosCriados++

		linhaPeriodo := map[string]any{
			"cliente_id":      p.ClienteID,
			"equipamento_id":  equipamentoID,
			"ordem_compra_id": id,
			"numero":          1,
			"tipo":            "original",
			"inicio":          hoje.Format("2006-01-02"),
			"fim":             vencimento.Format("2006-01-02"),
			"qtd":             it.QtdRecebida,
			"valor_unit":      it.ValorUnit,
			"criado_por":      p.UserID,
		}
		if err := m.bd.Inserir(r.Context(), "locacoes_periodos", []map[string]any{linhaPeriodo}, nil); err != nil {
			m.erro(w, "gravei o equipamento mas não consegui registrar o período de cobertura", err)
			return
		}

		linhasFotos := make([]map[string]any, 0, len(fotosPorItem[it.ItemID]))
		for _, sha := range fotosPorItem[it.ItemID] {
			linhasFotos = append(linhasFotos, map[string]any{
				"cliente_id":     p.ClienteID,
				"equipamento_id": equipamentoID,
				"evento":         "recebimento",
				"arquivo_sha256": sha,
			})
		}
		if err := m.bd.Inserir(r.Context(), "locacoes_fotos", linhasFotos, nil); err != nil {
			m.erro(w, "gravei o equipamento mas não consegui registrar as fotos", err)
			return
		}
	}

	if err := m.bd.Atualizar(r.Context(), "ordens_compra",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
		map[string]any{"destino_recebimento": "locacao"}); err != nil {
		m.erro(w, "gravei o recebimento mas não consegui atualizar a ordem de compra", err)
		return
	}

	_ = m.hist.Registrar(r.Context(), p, "locacoes", recebimentoID, "receber_locacao", map[string]historico.Mudanca{
		"ordem_compra_id": {De: nil, Para: id},
		"periodicidade":   {De: nil, Para: periodicidade},
		"equipamentos":    {De: nil, Para: equipamentosCriados},
	})

	web.Responder(w, http.StatusOK, map[string]any{
		"id":           recebimentoID,
		"equipamentos": equipamentosCriados,
		"vencimento":   vencimento.Format("2006-01-02"),
	})
}

func descricaoOuItem(it itemRecebido, i int) string {
	if strings.TrimSpace(it.Descricao) != "" {
		return it.Descricao
	}
	return fmt.Sprintf("item %d", i+1)
}
