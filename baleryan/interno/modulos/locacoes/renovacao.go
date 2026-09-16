// rev 1 — Locações: renovar, em duas etapas (Fase 4, 16/09/2026)
//
// QUEM PEDE NÃO É QUEM CONCLUI
//
//	Decisão do dono: qualquer um da hierarquia da obra (LOCACOES_DECIDIR —
//	almoxarife, encarregado, engenheiro) PEDE a renovação de um equipamento
//	perto do vencimento. O pedido em si não cobre nada — só sinaliza o RC
//	(LOCACOES_RENOVAR_OC), que insere a OC de renovação de verdade (mesma
//	esteira de Compras: `POST /administrativo/compras/ordens` com
//	`destino_recebimento=locacao`, depois `ler`) e CONCLUI. É a conclusão
//	que cria o próximo período em `locacoes_periodos` e empurra
//	`vencimento_atual` — nunca o pedido.
//
// PERÍODOS CONTÍGUOS, SEM BURACO NEM SOBREPOSIÇÃO
//
//	O novo período começa exatamente onde o `vencimento_atual` de HOJE
//	termina — não na data em que o RC conclui. Um pedido feito 3 dias antes
//	do vencimento e concluído 2 dias depois não deveria "perder" nem
//	"ganhar" esses dias.
package locacoes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

func parseData(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// ---------------------------------------------------------------------------
// POST /locacoes/equipamentos/renovar — a hierarquia da obra PEDE
// ---------------------------------------------------------------------------

type corpoRenovar struct {
	Equipamentos  []string `json:"equipamentos"`
	Periodicidade string   `json:"periodicidade"`
}

func (m *Modulo) renovar(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaDecidir)
	if p == nil {
		return
	}
	var corpo corpoRenovar
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil || len(corpo.Equipamentos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Informe ao menos um equipamento.")
		return
	}
	if corpo.Periodicidade != "" && !periodicidadesValidas[corpo.Periodicidade] {
		web.Falhar(w, http.StatusBadRequest, "Prazo inválido — escolha mensal, quinzenal ou semanal.")
		return
	}

	pedidos := 0
	for _, equipamentoID := range corpo.Equipamentos {
		if _, ok := umUUID(equipamentoID); !ok {
			web.Falhar(w, http.StatusBadRequest, "Equipamento inválido na lista.")
			return
		}
		equip, err := m.contarUm(r.Context(), "locacoes_equipamentos?id=eq."+equipamentoID+
			"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,estado,periodicidade,obra_centro_custo&limit=1")
		if err != nil {
			web.Falhar(w, http.StatusNotFound, "Não achei um dos equipamentos.")
			return
		}
		if strCampo(equip["estado"]) != "ativo" {
			web.Falhar(w, http.StatusConflict, "Um dos equipamentos já está encerrado.")
			return
		}
		ok, err := m.temAcessoAObra(r.Context(), p, strCampo(equip["obra_centro_custo"]))
		if err != nil || !ok {
			web.Falhar(w, http.StatusForbidden, "Você não tem acesso a uma das obras.")
			return
		}
		pendentes, err := m.bd.BuscarContando(r.Context(), "locacoes_renovacoes?equipamento_id=eq."+equipamentoID+
			"&estado=eq.pendente&select=id&limit=1", nil)
		if err != nil {
			m.erro(w, "não consegui conferir se já existe um pedido pendente", err)
			return
		}
		if pendentes > 0 {
			// Não é erro — pedir de novo o que já está pedido é um não-evento
			// pro almoxarife, não um motivo pra travar o lote inteiro.
			continue
		}
		periodicidade := corpo.Periodicidade
		if periodicidade == "" {
			periodicidade = strCampo(equip["periodicidade"])
		}
		linha := map[string]any{
			"cliente_id":     p.ClienteID,
			"equipamento_id": equipamentoID,
			"periodicidade":  periodicidade,
			"pedida_por":     p.UserID,
		}
		if err := m.bd.Inserir(r.Context(), "locacoes_renovacoes", []map[string]any{linha}, nil); err != nil {
			m.erro(w, "não consegui registrar o pedido de renovação", err)
			return
		}
		pedidos++
		_ = m.hist.Registrar(r.Context(), p, "locacoes", equipamentoID, "pedir_renovacao", map[string]historico.Mudanca{
			"periodicidade": {De: nil, Para: periodicidade},
		})
	}

	web.Responder(w, http.StatusOK, map[string]any{"pedidos": pedidos})
}

// ---------------------------------------------------------------------------
// POST /locacoes/renovacoes/{id}/cancelar — desiste do pedido (libera Devolver)
// ---------------------------------------------------------------------------

func (m *Modulo) cancelarRenovacao(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaDecidir)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var corpo struct {
		Motivo string `json:"motivo"`
	}
	_ = json.NewDecoder(r.Body).Decode(&corpo)

	var tomadas []map[string]any
	if err := m.bd.AtualizarDevolvendo(r.Context(), "locacoes_renovacoes",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&estado=eq.pendente",
		map[string]any{"estado": "cancelada", "cancelada_em": time.Now().UTC().Format(time.RFC3339), "motivo": strings.TrimSpace(corpo.Motivo)},
		&tomadas); err != nil {
		m.erro(w, "não consegui cancelar este pedido de renovação", err)
		return
	}
	if len(tomadas) == 0 {
		web.Falhar(w, http.StatusConflict, "Este pedido não está mais pendente — a lista pode ter mudado, atualize a página.")
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "locacoes", id, "cancelar_renovacao", nil)
	web.Responder(w, http.StatusOK, map[string]any{"cancelada": true})
}

// ---------------------------------------------------------------------------
// GET /locacoes/renovacoes?estado=pendente — a fila do RC
// ---------------------------------------------------------------------------

func (m *Modulo) renovacoes(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaRenovarOC)
	if p == nil {
		return
	}
	estado := r.URL.Query().Get("estado")
	if estado == "" {
		estado = "pendente"
	}
	var linhas []map[string]any
	caminho := "locacoes_renovacoes?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&estado=eq." + banco.Escapar(estado) + "&order=pedida_em" +
		"&select=id,periodicidade,pedida_em,equipamento_id," +
		"locacoes_equipamentos(descricao,unidade,qtd_ativa,valor_unit,vencimento_atual,obra_centro_custo," +
		"fornecedores(razao_social))" +
		"&limit=" + fmt.Sprint(TetoDaLista)
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar os pedidos de renovação", err)
		return
	}

	saida := make([]map[string]any, 0, len(linhas))
	for _, l := range linhas {
		equip, _ := l["locacoes_equipamentos"].(map[string]any)
		if equip == nil {
			continue
		}
		saida = append(saida, map[string]any{
			"id":                l["id"],
			"equipamento_id":    l["equipamento_id"],
			"periodicidade":     l["periodicidade"],
			"pedida_em":         l["pedida_em"],
			"descricao":         equip["descricao"],
			"unidade":           equip["unidade"],
			"qtd_ativa":         equip["qtd_ativa"],
			"valor_unit":        equip["valor_unit"],
			"vencimento_atual":  equip["vencimento_atual"],
			"obra_centro_custo": equip["obra_centro_custo"],
			"fornecedor_nome":   nestedStr(equip["fornecedores"], "razao_social"),
		})
	}
	web.Responder(w, http.StatusOK, map[string]any{"renovacoes": saida})
}

// ---------------------------------------------------------------------------
// POST /locacoes/renovacoes/concluir — o RC amarra a OC de renovação
// ---------------------------------------------------------------------------

type corpoConcluir struct {
	OrdemCompraID string   `json:"ordem_compra_id"`
	Renovacoes    []string `json:"renovacoes"`
}

func (m *Modulo) concluirRenovacao(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaRenovarOC)
	if p == nil {
		return
	}
	var corpo corpoConcluir
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o pedido.")
		return
	}
	if _, ok := umUUID(corpo.OrdemCompraID); !ok {
		web.Falhar(w, http.StatusBadRequest, "Ordem de compra inválida.")
		return
	}
	if len(corpo.Renovacoes) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Selecione ao menos um pedido de renovação.")
		return
	}

	oc, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+corpo.OrdemCompraID+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,status,destino_recebimento&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if strCampo(oc["status"]) != "lido" {
		web.Falhar(w, http.StatusConflict, "Esta ordem de compra ainda não foi lida com sucesso.")
		return
	}
	if strCampo(oc["destino_recebimento"]) != "locacao" {
		web.Falhar(w, http.StatusConflict, "Esta ordem de compra não foi inserida como renovação de locação.")
		return
	}

	concluidas := 0
	for _, renovacaoID := range corpo.Renovacoes {
		if _, ok := umUUID(renovacaoID); !ok {
			web.Falhar(w, http.StatusBadRequest, "Pedido de renovação inválido na lista.")
			return
		}
		renov, err := m.contarUm(r.Context(), "locacoes_renovacoes?id=eq."+renovacaoID+
			"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&estado=eq.pendente&select=id,equipamento_id,periodicidade&limit=1")
		if err != nil {
			web.Falhar(w, http.StatusConflict, "Um dos pedidos de renovação não está mais pendente.")
			return
		}
		equipamentoID := strCampo(renov["equipamento_id"])
		periodicidade := strCampo(renov["periodicidade"])

		equip, err := m.contarUm(r.Context(), "locacoes_equipamentos?id=eq."+equipamentoID+
			"&select=vencimento_atual,qtd_ativa,valor_unit,estado&limit=1")
		if err != nil {
			m.erro(w, "não achei o equipamento de um dos pedidos", err)
			return
		}
		if strCampo(equip["estado"]) != "ativo" {
			web.Falhar(w, http.StatusConflict, "Um dos equipamentos já foi encerrado — cancele o pedido em vez de concluir.")
			return
		}
		inicio, err := parseData(strCampo(equip["vencimento_atual"]))
		if err != nil {
			m.erro(w, "não consegui ler o vencimento atual do equipamento", err)
			return
		}
		fim := somarPeriodo(inicio, periodicidade)

		ultimoNumero, err := m.contarUm(r.Context(), "locacoes_periodos?equipamento_id=eq."+equipamentoID+
			"&select=numero&order=numero.desc&limit=1")
		numero := 1
		if err == nil {
			numero = int(numCampo(ultimoNumero["numero"])) + 1
		}

		linhaPeriodo := map[string]any{
			"cliente_id":      p.ClienteID,
			"equipamento_id":  equipamentoID,
			"ordem_compra_id": corpo.OrdemCompraID,
			"numero":          numero,
			"tipo":            "renovacao",
			"inicio":          inicio.Format("2006-01-02"),
			"fim":             fim.Format("2006-01-02"),
			"qtd":             equip["qtd_ativa"],
			"valor_unit":      equip["valor_unit"],
			"criado_por":      p.UserID,
		}
		var criados []map[string]any
		if err := m.bd.Inserir(r.Context(), "locacoes_periodos", []map[string]any{linhaPeriodo}, &criados); err != nil {
			m.erro(w, "gravei parte do lote mas falhei ao registrar um período", err)
			return
		}
		if len(criados) == 0 {
			m.erro(w, "gravei um período mas o banco não devolveu o id", fmt.Errorf("insert sem retorno"))
			return
		}
		periodoID := strCampo(criados[0]["id"])

		if err := m.bd.Atualizar(r.Context(), "locacoes_equipamentos",
			"id=eq."+equipamentoID+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
			map[string]any{"vencimento_atual": fim.Format("2006-01-02"), "periodicidade": periodicidade}); err != nil {
			m.erro(w, "gravei o período mas não consegui atualizar o vencimento do equipamento", err)
			return
		}

		if err := m.bd.Atualizar(r.Context(), "locacoes_renovacoes",
			"id=eq."+renovacaoID+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
			map[string]any{
				"estado":          "concluida",
				"ordem_compra_id": corpo.OrdemCompraID,
				"periodo_id":      periodoID,
				"concluida_em":    time.Now().UTC().Format(time.RFC3339),
				"concluida_por":   p.UserID,
			}); err != nil {
			m.erro(w, "gravei o período mas não consegui concluir o pedido de renovação", err)
			return
		}

		concluidas++
		_ = m.hist.Registrar(r.Context(), p, "locacoes", equipamentoID, "concluir_renovacao", map[string]historico.Mudanca{
			"ordem_compra_id": {De: nil, Para: corpo.OrdemCompraID},
			"vencimento_novo": {De: nil, Para: fim.Format("2006-01-02")},
		})
	}

	web.Responder(w, http.StatusOK, map[string]any{"concluidas": concluidas})
}
