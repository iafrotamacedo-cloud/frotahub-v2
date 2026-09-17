// rev 1 — Locações: devolver e reabrir (Fase 3, 16/09/2026)
//
// UM ROMANEIO, VÁRIAS DEVOLUÇÕES
//
//	`locacoes_devolucoes` guarda uma linha POR EQUIPAMENTO (é ele quem
//	encerra, não a OC — mesmo raciocínio do recebimento). Mas fisicamente um
//	único romaneio de devolução costuma cobrir vários equipamentos de uma
//	vez — por isso o lote sobe o romaneio UMA VEZ (dedup por sha256, mesma
//	receita de sempre) e cada linha de `locacoes_devolucoes` criada aponta
//	para o mesmo `romaneio_sha256`. As páginas 2+ são replicadas por linha
//	(mesmo arquivo, referência própria) — cada devolução fica autossuficiente
//	pra quem só olhar aquele equipamento.
//
// O INTERTRAVAMENTO (OPÇÃO A, JÁ FECHADA COM O DONO)
//
//	Um equipamento com renovação PENDENTE (`locacoes_renovacoes`, Fase 4) não
//	pode ser devolvido — o pedido em aberto trava a outra ponta da decisão.
//	A tabela já existe desde a migração 072; a Fase 4 é quem vai criar
//	pedidos de verdade, mas a trava já vale desde agora.
//
// REABRIR É SÓ BUILDER, E SÓ DESFAZ A ÚLTIMA
//
//	Decisão do dono: ninguém edita uma devolução — desfaz e refaz. Apaga a
//	devolução mais recente daquele equipamento (fotos e páginas do romaneio
//	junto, o arquivo em si fica no armazém — outra linha pode referenciá-lo
//	de novo), recalcula qtd_ativa pela soma do que sobrou, e o equipamento
//	volta a `ativo`.
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

type itemDevolvido struct {
	EquipamentoID string  `json:"equipamento_id"`
	Qtd           float64 `json:"qtd"`
}

// equipamentoParaDevolucao é o que a validação carrega de cada item antes de
// gravar qualquer coisa — nada é escrito até o lote inteiro passar.
type equipamentoParaDevolucao struct {
	qtdAtiva float64
}

func (m *Modulo) devolver(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaDecidir)
	if p == nil {
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado. Sem ele, devolver seria perder a prova.")
		return
	}
	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o que foi enviado.")
		return
	}

	var itens []itemDevolvido
	if err := json.Unmarshal([]byte(r.FormValue("itens")), &itens); err != nil || len(itens) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Informe ao menos um equipamento devolvido.")
		return
	}
	for i, it := range itens {
		if _, ok := umUUID(it.EquipamentoID); !ok {
			web.Falhar(w, http.StatusBadRequest, "Equipamento inválido na lista.")
			return
		}
		if it.Qtd <= 0 {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Informe a quantidade devolvida do item %d.", i+1))
			return
		}
	}

	// VALIDA TUDO ANTES DE SUBIR QUALQUER ARQUIVO
	//
	//	Cada equipamento precisa existir, ser do meu cliente, estar ativo,
	//	não ter renovação pendente (intertravamento) e a obra dele precisa
	//	estar liberada pra este login — tudo isso é barato de conferir e
	//	evita subir romaneio/fotos pra um lote que ia falhar de qualquer jeito.
	validados := make(map[string]equipamentoParaDevolucao, len(itens))
	for _, it := range itens {
		if _, jaVisto := validados[it.EquipamentoID]; jaVisto {
			web.Falhar(w, http.StatusBadRequest, "O mesmo equipamento aparece duas vezes na lista.")
			return
		}
		equip, err := m.contarUm(r.Context(), "locacoes_equipamentos?id=eq."+it.EquipamentoID+
			"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,qtd_ativa,estado,obra_centro_custo&limit=1")
		if err != nil {
			web.Falhar(w, http.StatusNotFound, "Não achei um dos equipamentos.")
			return
		}
		if strCampo(equip["estado"]) != "ativo" {
			web.Falhar(w, http.StatusConflict, "Um dos equipamentos já está encerrado.")
			return
		}
		qtdAtiva := numCampo(equip["qtd_ativa"])
		if it.Qtd > qtdAtiva {
			web.Falhar(w, http.StatusBadRequest, "A quantidade devolvida de um item é maior do que a quantidade ainda ativa.")
			return
		}
		pendentes, err := m.bd.BuscarContando(r.Context(), "locacoes_renovacoes?equipamento_id=eq."+it.EquipamentoID+
			"&estado=eq.pendente&select=id&limit=1", nil)
		if err != nil {
			m.erro(w, "não consegui conferir se há renovação pendente", err)
			return
		}
		if pendentes > 0 {
			web.Falhar(w, http.StatusConflict,
				"Um dos equipamentos tem um pedido de renovação pendente — cancele o pedido antes de devolver.")
			return
		}
		ok, err := m.temAcessoAObra(r.Context(), p, strCampo(equip["obra_centro_custo"]))
		if err != nil || !ok {
			web.Falhar(w, http.StatusForbidden, "Você não tem acesso a uma das obras deste lote.")
			return
		}
		validados[it.EquipamentoID] = equipamentoParaDevolucao{qtdAtiva: qtdAtiva}
	}

	// O ROMANEIO DE DEVOLUÇÃO — UM SÓ, OBRIGATÓRIO, VALE PRO LOTE INTEIRO
	primeira := r.MultipartForm.File["romaneio"]
	if len(primeira) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escaneie o romaneio de devolução.")
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

	// FOTOS DO ESTADO NA SAÍDA — PELO MENOS UMA POR EQUIPAMENTO
	fotosPorItem := make(map[string][]string, len(itens))
	for i, it := range itens {
		cabs := r.MultipartForm.File["fotos_"+it.EquipamentoID]
		if len(cabs) == 0 {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Tire ao menos uma foto do item %d na devolução.", i+1))
			return
		}
		shas := make([]string, 0, len(cabs))
		for j, cab := range cabs {
			sha, err := m.lerEGuardarArquivo(r.Context(), p, cab)
			if err != nil {
				web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Foto %d do item %d: %s.", j+1, i+1, err.Error()))
				return
			}
			shas = append(shas, sha)
		}
		fotosPorItem[it.EquipamentoID] = shas
	}

	// A OC DE DESMOBILIZAÇÃO (FRETE DE VOLTA) — OPCIONAL, UMA SÓ PRO LOTE
	//
	//	Decisão do dono (17/09/2026): quando existe uma OC de frete pra
	//	trazer o equipamento de volta, ela é vinculada aqui — já precisa
	//	estar inserida (a tela resolve o id pelo número, ver
	//	`buscarOrdemPorNumero`). Sem vínculo automático de "qual frete é de
	//	qual locação": quem devolve escolhe, porque o sistema não tem como
	//	adivinhar isso a partir de um PDF solto do Obra Prima.
	ordemFreteID := strings.TrimSpace(r.FormValue("ordem_compra_frete_id"))
	if ordemFreteID != "" {
		if _, ok := umUUID(ordemFreteID); !ok {
			web.Falhar(w, http.StatusBadRequest, "OC de frete inválida.")
			return
		}
		if _, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+ordemFreteID+
			"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id&limit=1"); err != nil {
			web.Falhar(w, http.StatusBadRequest, "Não achei a OC de frete informada.")
			return
		}
	}

	hoje := time.Now().UTC().Truncate(24 * time.Hour).Format("2006-01-02")
	encerrados := 0
	for _, it := range itens {
		linha := map[string]any{
			"cliente_id":      p.ClienteID,
			"equipamento_id":  it.EquipamentoID,
			"data_devolucao":  hoje,
			"qtd":             it.Qtd,
			"romaneio_sha256": paginasRomaneio[0].sha,
			"registrado_por":  p.UserID,
		}
		if ordemFreteID != "" {
			linha["ordem_compra_frete_id"] = ordemFreteID
		}
		var criados []map[string]any
		if err := m.bd.Inserir(r.Context(), "locacoes_devolucoes", []map[string]any{linha}, &criados); err != nil {
			m.erro(w, "gravei parte do lote mas falhei ao registrar uma devolução", err)
			return
		}
		if len(criados) == 0 {
			m.erro(w, "gravei uma devolução mas o banco não devolveu o id", fmt.Errorf("insert sem retorno"))
			return
		}
		devolucaoID := strCampo(criados[0]["id"])

		if len(paginasRomaneio) > 1 {
			linhasPaginas := make([]map[string]any, 0, len(paginasRomaneio)-1)
			for i := 1; i < len(paginasRomaneio); i++ {
				linhasPaginas = append(linhasPaginas, map[string]any{
					"cliente_id":     p.ClienteID,
					"devolucao_id":   devolucaoID,
					"pagina":         i + 1,
					"arquivo_sha256": paginasRomaneio[i].sha,
				})
			}
			if err := m.bd.Inserir(r.Context(), "locacoes_devolucoes_paginas", linhasPaginas, nil); err != nil {
				m.erro(w, "gravei a devolução mas não consegui registrar as páginas do romaneio", err)
				return
			}
		}

		linhasFotos := make([]map[string]any, 0, len(fotosPorItem[it.EquipamentoID]))
		for _, sha := range fotosPorItem[it.EquipamentoID] {
			linhasFotos = append(linhasFotos, map[string]any{
				"cliente_id":     p.ClienteID,
				"equipamento_id": it.EquipamentoID,
				"evento":         "devolucao",
				"devolucao_id":   devolucaoID,
				"arquivo_sha256": sha,
			})
		}
		if err := m.bd.Inserir(r.Context(), "locacoes_fotos", linhasFotos, nil); err != nil {
			m.erro(w, "gravei a devolução mas não consegui registrar as fotos", err)
			return
		}

		novaQtdAtiva := validados[it.EquipamentoID].qtdAtiva - it.Qtd
		novoEstado := "ativo"
		if novaQtdAtiva <= 0 {
			novaQtdAtiva = 0
			novoEstado = "encerrado"
			encerrados++
		}
		if err := m.bd.Atualizar(r.Context(), "locacoes_equipamentos",
			"id=eq."+it.EquipamentoID+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
			map[string]any{"qtd_ativa": novaQtdAtiva, "estado": novoEstado}); err != nil {
			m.erro(w, "gravei a devolução mas não consegui atualizar o equipamento", err)
			return
		}

		_ = m.hist.Registrar(r.Context(), p, "locacoes", devolucaoID, "devolver_locacao", map[string]historico.Mudanca{
			"equipamento_id": {De: nil, Para: it.EquipamentoID},
			"qtd":            {De: nil, Para: it.Qtd},
			"estado_novo":    {De: nil, Para: novoEstado},
		})
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"devolvidos": len(itens),
		"encerrados": encerrados,
	})
}

// ---------------------------------------------------------------------------
// GET /locacoes/ordens/buscar?numero=X — acha uma OC já inserida pelo
// número, pra vincular como frete de desmobilização na devolução. Não exige
// que a OC seja de locação — frete é uma OC de compra comum.
// ---------------------------------------------------------------------------

func (m *Modulo) buscarOrdemPorNumero(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaDecidir)
	if p == nil {
		return
	}
	numero := strings.TrimSpace(r.URL.Query().Get("numero"))
	if numero == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o número da OC.")
		return
	}
	oc, err := m.contarUm(r.Context(), "ordens_compra?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&numero=eq."+banco.Escapar(numero)+"&select=id,numero,fornecedor_id,total,fornecedores(razao_social)&limit=1")
	if err != nil {
		web.Falhar(w, http.StatusNotFound, "Não achei nenhuma OC com este número.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"id":              oc["id"],
		"numero":          oc["numero"],
		"total":           oc["total"],
		"fornecedor_nome": nestedStr(oc["fornecedores"], "razao_social"),
	})
}

// ---------------------------------------------------------------------------
// POST /locacoes/equipamentos/{id}/reabrir — só builder, desfaz A ÚLTIMA
// decisão deste equipamento — devolução ou renovação, o que aconteceu
// depois (comparando `criado_em` dos dois candidatos).
// ---------------------------------------------------------------------------

func (m *Modulo) reabrir(w http.ResponseWriter, r *http.Request) {
	p := m.quemEBuilder(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if _, err := m.contarUm(r.Context(), "locacoes_equipamentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id&limit=1"); err != nil {
		m.erro(w, "não achei este equipamento", err)
		return
	}

	ultimaDevolucao, errDev := m.contarUm(r.Context(), "locacoes_devolucoes?equipamento_id=eq."+id+
		"&order=criado_em.desc&select=id,criado_em&limit=1")
	ultimoPeriodo, errPer := m.contarUm(r.Context(), "locacoes_periodos?equipamento_id=eq."+id+
		"&tipo=eq.renovacao&order=criado_em.desc&select=id,criado_em&limit=1")

	temDevolucao, temPeriodo := errDev == nil, errPer == nil
	if !temDevolucao && !temPeriodo {
		web.Falhar(w, http.StatusConflict, "Este equipamento não tem devolução nem renovação para reabrir.")
		return
	}

	desfazerRenovacao := temPeriodo && (!temDevolucao || maisRecente(ultimoPeriodo["criado_em"], ultimaDevolucao["criado_em"]))
	if desfazerRenovacao {
		m.reabrirRenovacao(w, r, p, id, strCampo(ultimoPeriodo["id"]))
		return
	}
	m.reabrirDevolucao(w, r, p, id, strCampo(ultimaDevolucao["id"]))
}

// maisRecente compara dois timestamps vindos do PostgREST (RFC3339). Em caso
// de erro de parse (não deveria acontecer — o banco sempre devolve o mesmo
// formato), trata como NÃO mais recente, pra nunca desfazer a coisa errada
// por engano.
func maisRecente(a, b any) bool {
	ta, erra := time.Parse(time.RFC3339, strCampo(a))
	tb, errb := time.Parse(time.RFC3339, strCampo(b))
	if erra != nil || errb != nil {
		return false
	}
	return ta.After(tb)
}

func (m *Modulo) reabrirDevolucao(w http.ResponseWriter, r *http.Request, p *seguranca.Principal, id, devolucaoID string) {
	// Fotos primeiro — `locacoes_fotos.devolucao_id` não tem `on delete
	// cascade` (uma foto pode, no futuro, sobreviver à devolução que a
	// gerou se alguém quiser guardar histórico à parte); aqui, reabrindo,
	// ela precisa ir junto.
	if err := m.bd.Apagar(r.Context(), "locacoes_fotos", "devolucao_id=eq."+banco.Escapar(devolucaoID)); err != nil {
		m.erro(w, "não consegui apagar as fotos da devolução", err)
		return
	}
	if err := m.bd.Apagar(r.Context(), "locacoes_devolucoes", "id=eq."+banco.Escapar(devolucaoID)); err != nil {
		m.erro(w, "não consegui apagar a devolução", err)
		return
	}

	var restantes []map[string]any
	if err := m.bd.Buscar(r.Context(), "locacoes_devolucoes?equipamento_id=eq."+id+"&select=qtd", &restantes); err != nil {
		m.erro(w, "apaguei a devolução mas não consegui recalcular a quantidade ativa", err)
		return
	}
	equip, err := m.contarUm(r.Context(), "locacoes_equipamentos?id=eq."+id+"&select=qtd_recebida&limit=1")
	if err != nil {
		m.erro(w, "apaguei a devolução mas não consegui reler o equipamento", err)
		return
	}
	qtdAtiva := numCampo(equip["qtd_recebida"])
	for _, d := range restantes {
		qtdAtiva -= numCampo(d["qtd"])
	}
	if qtdAtiva < 0 {
		qtdAtiva = 0
	}

	if err := m.bd.Atualizar(r.Context(), "locacoes_equipamentos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
		map[string]any{"qtd_ativa": qtdAtiva, "estado": "ativo"}); err != nil {
		m.erro(w, "apaguei a devolução mas não consegui reabrir o equipamento", err)
		return
	}

	_ = m.hist.Registrar(r.Context(), p, "locacoes", id, "reabrir_locacao_devolucao", map[string]historico.Mudanca{
		"devolucao_desfeita": {De: nil, Para: devolucaoID},
		"qtd_ativa_nova":     {De: nil, Para: qtdAtiva},
	})

	web.Responder(w, http.StatusOK, map[string]any{"desfeito": "devolucao", "qtd_ativa": qtdAtiva, "estado": "ativo"})
}

// reabrirRenovacao desfaz o período mais recente (tipo="renovacao") e devolve
// o pedido que o originou pra "pendente" — o RC vê de novo na fila e pode
// concluir de novo, desta vez direito. O `on delete` de
// `locacoes_renovacoes.periodo_id` é NO ACTION (padrão), então o vínculo
// precisa ser desfeito ANTES de apagar o período, nunca depois.
func (m *Modulo) reabrirRenovacao(w http.ResponseWriter, r *http.Request, p *seguranca.Principal, id, periodoID string) {
	if err := m.bd.Atualizar(r.Context(), "locacoes_renovacoes",
		"periodo_id=eq."+banco.Escapar(periodoID)+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
		map[string]any{"estado": "pendente", "ordem_compra_id": nil, "periodo_id": nil, "concluida_em": nil, "concluida_por": nil}); err != nil {
		m.erro(w, "não consegui reabrir o pedido de renovação ligado a este período", err)
		return
	}
	if err := m.bd.Apagar(r.Context(), "locacoes_periodos", "id=eq."+banco.Escapar(periodoID)); err != nil {
		m.erro(w, "reabri o pedido mas não consegui apagar o período de renovação", err)
		return
	}

	restante, err := m.contarUm(r.Context(), "locacoes_periodos?equipamento_id=eq."+id+
		"&select=fim&order=numero.desc&limit=1")
	if err != nil {
		m.erro(w, "apaguei o período mas não consegui recalcular o vencimento do equipamento", err)
		return
	}
	novoVencimento := strCampo(restante["fim"])

	if err := m.bd.Atualizar(r.Context(), "locacoes_equipamentos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
		map[string]any{"vencimento_atual": novoVencimento}); err != nil {
		m.erro(w, "apaguei o período mas não consegui atualizar o vencimento do equipamento", err)
		return
	}

	_ = m.hist.Registrar(r.Context(), p, "locacoes", id, "reabrir_locacao_renovacao", map[string]historico.Mudanca{
		"periodo_desfeito": {De: nil, Para: periodoID},
		"vencimento_novo":  {De: nil, Para: novoVencimento},
	})

	web.Responder(w, http.StatusOK, map[string]any{"desfeito": "renovacao", "vencimento_atual": novoVencimento, "estado": "ativo"})
}
