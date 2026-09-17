// rev 1 — Locações: o monitoramento (Fase 2, 16/09/2026)
//
// POR QUE O CONTADOR VIRA "DESCOBERTO" ANTES DE VIRAR "VERMELHO"
//
//	`descoberto` é o equipamento que já venceu (vencimento_atual < hoje) sem
//	nenhuma decisão — dinheiro vazando de verdade, porque a locadora segue
//	cobrando um equipamento sem OC cobrindo o período. `vermelho` (faixa) já
//	inclui o dia do vencimento em diante — é o aviso; `descoberto` é o que já
//	passou do aviso sem ninguém agir. Como a lista ordena por vencimento_atual
//	crescente, os descobertos (as datas mais antigas) já nascem no topo — não
//	precisa de um segundo critério de ordenação.
//
// "TOTAL MENSAL" É UMA SOMA SIMPLES, DE PROPÓSITO
//
//	`qtd_ativa × valor_unit`, sem normalizar quinzenal/semanal para um
//	equivalente mensal — é a mesma simplificação que o plano do módulo
//	descreveu (soma direta). Vira uma conta mais precisa quando a Fase 5
//	(faturamento) chegar; até lá é só um indicador de grandeza, não uma
//	conta fechada.
package locacoes

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

const TetoDaLista = 500

// ---------------------------------------------------------------------------
// GET /locacoes/painel — os cartões do hub
// ---------------------------------------------------------------------------

func (m *Modulo) painel(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaMonitorar)
	if p == nil {
		return
	}
	hoje := time.Now().UTC().Truncate(24 * time.Hour)
	daqui7 := hoje.AddDate(0, 0, 7)

	var ativos []map[string]any
	if err := m.bd.Buscar(r.Context(), "locacoes_equipamentos?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&estado=eq.ativo&select=id,qtd_ativa,valor_unit,vencimento_atual,obra_centro_custo&limit="+fmt.Sprint(TetoDaLista),
		&ativos); err != nil {
		m.erro(w, "não consegui contar os equipamentos ativos", err)
		return
	}

	descobertos, vencendo, totalMensal := 0, 0, 0.0
	porObra := map[string]float64{}
	for _, l := range ativos {
		venc, err := time.Parse("2006-01-02", strCampo(l["vencimento_atual"]))
		if err == nil {
			if venc.Before(hoje) {
				descobertos++
			} else if !venc.After(daqui7) {
				vencendo++
			}
		}
		v := numCampo(l["qtd_ativa"]) * numCampo(l["valor_unit"])
		totalMensal += v
		porObra[strCampo(l["obra_centro_custo"])] += v
	}

	encerrados, err := m.bd.BuscarContando(r.Context(), "locacoes_equipamentos?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&estado=eq.encerrado&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar os equipamentos encerrados", err)
		return
	}
	renovacoesPendentes, err := m.bd.BuscarContando(r.Context(), "locacoes_renovacoes?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&estado=eq.pendente&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as renovações pendentes", err)
		return
	}

	obras := make([]map[string]any, 0, len(porObra))
	for obra, total := range porObra {
		if obra == "" {
			continue
		}
		obras = append(obras, map[string]any{"obra_centro_custo": obra, "total_mensal": total})
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"ativos":               len(ativos),
		"descobertos":          descobertos,
		"vencendo":             vencendo,
		"encerrados":           encerrados,
		"renovacoes_pendentes": renovacoesPendentes,
		"total_mensal":         totalMensal,
		"por_obra":             obras,
	})
}

// ---------------------------------------------------------------------------
// GET /locacoes/equipamentos?vista=ativos|descobertos|encerrados
// ---------------------------------------------------------------------------

func (m *Modulo) equipamentos(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaMonitorar)
	if p == nil {
		return
	}
	vista := r.URL.Query().Get("vista")

	caminho := "locacoes_equipamentos?cliente_id=eq." + banco.Escapar(p.ClienteID)
	hoje := time.Now().UTC().Truncate(24 * time.Hour)
	switch vista {
	case "encerrados":
		caminho += "&estado=eq.encerrado&order=atualizado_em.desc"
	case "descobertos":
		caminho += "&estado=eq.ativo&vencimento_atual=lt." + hoje.Format("2006-01-02") + "&order=vencimento_atual"
	default: // "ativos" — todo mundo ainda ativo, vencido ou não, vencimento mais próximo primeiro
		caminho += "&estado=eq.ativo&order=vencimento_atual"
	}
	caminho += "&select=id,descricao,unidade,qtd_ativa,valor_unit,periodicidade,data_inicio,vencimento_atual,estado," +
		"obra_centro_custo,ordem_compra_id,fornecedores(razao_social),ordens_compra(numero)" +
		"&limit=" + fmt.Sprint(TetoDaLista)

	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar os equipamentos locados", err)
		return
	}
	linhas = m.comAcessoNasObras(r.Context(), p, linhas)
	pendentes := m.equipamentosComRenovacaoPendente(r.Context(), p, linhas)

	saida := make([]map[string]any, 0, len(linhas))
	for _, l := range linhas {
		dias := 0
		if venc, err := time.Parse("2006-01-02", strCampo(l["vencimento_atual"])); err == nil {
			dias = int(venc.Sub(hoje).Hours() / 24)
		}
		saida = append(saida, map[string]any{
			"id":                 l["id"],
			"descricao":          l["descricao"],
			"unidade":            l["unidade"],
			"qtd_ativa":          l["qtd_ativa"],
			"valor_unit":         l["valor_unit"],
			"periodicidade":      l["periodicidade"],
			"data_inicio":        l["data_inicio"],
			"vencimento_atual":   l["vencimento_atual"],
			"estado":             l["estado"],
			"obra_centro_custo":  l["obra_centro_custo"],
			"ordem_compra_id":    l["ordem_compra_id"],
			"ordem_numero":       nestedStr(l["ordens_compra"], "numero"),
			"fornecedor_nome":    nestedStr(l["fornecedores"], "razao_social"),
			"dias_para_vencer":   dias,
			"faixa":              faixaDoVencimento(dias),
			"descoberto":         dias < 0,
			"renovacao_pendente": pendentes[strCampo(l["id"])],
		})
	}
	web.Responder(w, http.StatusOK, map[string]any{"equipamentos": saida})
}

func faixaDoVencimento(dias int) string {
	switch {
	case dias > 7:
		return "verde"
	case dias >= 3:
		return "amarelo"
	case dias >= 1:
		return "laranja"
	default:
		return "vermelho"
	}
}

// ---------------------------------------------------------------------------
// GET /locacoes/equipamentos/{id} — o detalhe: períodos, romaneio, fotos
// ---------------------------------------------------------------------------

func (m *Modulo) equipamento(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaMonitorar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	equip, err := m.contarUm(r.Context(), "locacoes_equipamentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,descricao,unidade,qtd_recebida,qtd_ativa,valor_unit,periodicidade,data_inicio,"+
		"vencimento_atual,estado,obra_centro_custo,recebimento_id,regra_faturamento,"+
		"fornecedores(razao_social,regra_faturamento),ordens_compra(numero)&limit=1")
	if err != nil {
		m.erro(w, "não achei este equipamento", err)
		return
	}
	if ok, err := m.temAcessoAObra(r.Context(), p, strCampo(equip["obra_centro_custo"])); err != nil || !ok {
		web.Falhar(w, http.StatusForbidden, "Você não tem acesso a esta obra.")
		return
	}
	fornecedor, _ := equip["fornecedores"].(map[string]any)
	regraFornecedor := ""
	if fornecedor != nil {
		regraFornecedor = strCampo(fornecedor["regra_faturamento"])
	}
	regra := regraResolvida(strCampo(equip["regra_faturamento"]), regraFornecedor)

	var periodosBrutos []map[string]any
	if err := m.bd.Buscar(r.Context(), "locacoes_periodos?equipamento_id=eq."+id+
		"&order=numero&select=id,numero,tipo,inicio,fim,qtd,valor_unit,ordens_compra(numero)", &periodosBrutos); err != nil {
		m.erro(w, "não consegui listar os períodos deste equipamento", err)
		return
	}
	periodos := make([]map[string]any, 0, len(periodosBrutos))
	totalCalculado := 0.0
	for _, per := range periodosBrutos {
		valor := 0.0
		if inicio, err1 := parseData(strCampo(per["inicio"])); err1 == nil {
			if fim, err2 := parseData(strCampo(per["fim"])); err2 == nil {
				valor = calcularValorPeriodo(int(numCampo(per["numero"])), inicio, fim,
					numCampo(per["qtd"]), numCampo(per["valor_unit"]), regra)
			}
		}
		totalCalculado += valor
		periodos = append(periodos, map[string]any{
			"id":              per["id"],
			"numero":          per["numero"],
			"tipo":            per["tipo"],
			"inicio":          per["inicio"],
			"fim":             per["fim"],
			"qtd":             per["qtd"],
			"valor_unit":      per["valor_unit"],
			"ordens_compra":   per["ordens_compra"],
			"valor_calculado": valor,
		})
	}

	var fotos []map[string]any
	if err := m.bd.Buscar(r.Context(), "locacoes_fotos?equipamento_id=eq."+id+
		"&evento=eq.recebimento&select=id,arquivo_sha256", &fotos); err != nil {
		m.erro(w, "não consegui listar as fotos deste equipamento", err)
		return
	}

	recebimento, err := m.contarUm(r.Context(), "locacoes_recebimentos?id=eq."+banco.Escapar(strCampo(equip["recebimento_id"]))+
		"&select=data_recebimento,romaneio_sha256,nf_numero,nf_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei o recebimento deste equipamento", err)
		return
	}

	var devolucoesBrutas []map[string]any
	if err := m.bd.Buscar(r.Context(), "locacoes_devolucoes?equipamento_id=eq."+id+
		"&order=criado_em.desc&select=id,data_devolucao,qtd,romaneio_sha256,frete:ordens_compra!ordem_compra_frete_id(numero)",
		&devolucoesBrutas); err != nil {
		m.erro(w, "não consegui listar as devoluções deste equipamento", err)
		return
	}
	devolucoes := make([]map[string]any, 0, len(devolucoesBrutas))
	for _, d := range devolucoesBrutas {
		devolucoes = append(devolucoes, map[string]any{
			"id":                 d["id"],
			"data_devolucao":     d["data_devolucao"],
			"qtd":                d["qtd"],
			"romaneio_sha256":    d["romaneio_sha256"],
			"frete_ordem_numero": nestedStr(d["frete"], "numero"),
		})
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"equipamento": map[string]any{
			"id":                equip["id"],
			"descricao":         equip["descricao"],
			"unidade":           equip["unidade"],
			"qtd_recebida":      equip["qtd_recebida"],
			"qtd_ativa":         equip["qtd_ativa"],
			"valor_unit":        equip["valor_unit"],
			"periodicidade":     equip["periodicidade"],
			"data_inicio":       equip["data_inicio"],
			"vencimento_atual":  equip["vencimento_atual"],
			"estado":            equip["estado"],
			"obra_centro_custo": equip["obra_centro_custo"],
			"fornecedor_nome":   nestedStr(equip["fornecedores"], "razao_social"),
			"ordem_numero":      nestedStr(equip["ordens_compra"], "numero"),
			"regra_faturamento": regra,
			"total_calculado":   totalCalculado,
		},
		"periodos":    ouVazio(periodos),
		"fotos":       ouVazio(fotos),
		"recebimento": recebimento,
		"devolucoes":  ouVazio(devolucoes),
	})
}

// ---------------------------------------------------------------------------
// utilidades
// ---------------------------------------------------------------------------

// comAcessoNasObras — mesma peneira de `administrativo.comAcessoNaObra`,
// aplicada a uma lista inteira de uma vez.
func (m *Modulo) comAcessoNasObras(ctx context.Context, p *seguranca.Principal, linhas []map[string]any) []map[string]any {
	if p.Builder() {
		return linhas
	}
	if pode, err := m.perm.Pode(ctx, p, rotinaConfigurarAcessoObra); err == nil && pode {
		return linhas
	}
	saida := make([]map[string]any, 0, len(linhas))
	for _, l := range linhas {
		ok, err := m.temAcessoAObra(ctx, p, strCampo(l["obra_centro_custo"]))
		if err == nil && ok {
			saida = append(saida, l)
		}
	}
	return saida
}

// equipamentosComRenovacaoPendente devolve, num mapa, quais dos IDs da lista
// têm um pedido de renovação pendente — uma consulta só (`in.(...)`), não
// uma por equipamento.
func (m *Modulo) equipamentosComRenovacaoPendente(ctx context.Context, p *seguranca.Principal, linhas []map[string]any) map[string]bool {
	pendentes := make(map[string]bool, len(linhas))
	if len(linhas) == 0 {
		return pendentes
	}
	ids := make([]string, 0, len(linhas))
	for _, l := range linhas {
		ids = append(ids, strCampo(l["id"]))
	}
	var achados []map[string]any
	if err := m.bd.Buscar(ctx, "locacoes_renovacoes?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&estado=eq.pendente&equipamento_id=in.("+strings.Join(ids, ",")+")&select=equipamento_id", &achados); err != nil {
		return pendentes
	}
	for _, a := range achados {
		pendentes[strCampo(a["equipamento_id"])] = true
	}
	return pendentes
}

func nestedStr(v any, campo string) string {
	mapa, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	return strCampo(mapa[campo])
}

func numCampo(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	default:
		return 0
	}
}
