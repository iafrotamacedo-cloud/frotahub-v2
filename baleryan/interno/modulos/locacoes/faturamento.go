// rev 1 — Locações: o cálculo do faturamento (Fase 5, 17/09/2026)
//
// A REGRA, DO JEITO QUE O DONO FECHOU (17/09/2026)
//
//	Cada PERÍODO (locacoes_periodos — o original ou uma renovação) vira um
//	valor calculado, nunca a OC inteira: um equipamento pode ter 3 períodos
//	com regras diferentes se o fornecedor ou o próprio equipamento tiverem
//	override.
//
//	  - "padrao" (sem override): período 1 é MÊS CHEIO (valor_unit × qtd,
//	    não importa quantos dias durou); a partir do período 2 (primeira
//	    renovação em diante) é PROPORCIONAL.
//	  - "sempre_mes_cheio": todo período é mês cheio, incluindo renovações —
//	    o override vale DESDE o período 1.
//	  - "sempre_proporcional": todo período é proporcional, incluindo o 1º —
//	    o override vale DESDE o período 1.
//
//	PROPORCIONAL é sempre valor_unit ÷ 30 × dias do período × qtd — uma
//	diária fixa, a mesma pra mensal/quinzenal/semanal (decisão do dono:
//	simples e previsível, não pesa a diária pela periodicidade escolhida).
//
// A ORDEM DO OVERRIDE: EQUIPAMENTO VENCE FORNECEDOR VENCE PADRÃO
//
//	`locacoes_equipamentos.regra_faturamento` (um caso negociado à parte)
//	sobrepõe `fornecedores.regra_faturamento` (a regra geral daquela
//	locadora), que sobrepõe "padrao" quando nenhum dos dois foi setado.
//
// NADA AQUI É GRAVADO — É SEMPRE CALCULADO NA HORA
//
//	Mesma disciplina de `nf_progresso_ordens` (CORE-06): valor_unit e qtd já
//	estão congelados por período em `locacoes_periodos` desde que o período
//	nasceu (recebimento ou conclusão de renovação); o valor em reais é
//	derivado desses dois mais a regra, então não duplica estado — se a
//	regra mudar depois, o cálculo antigo muda junto, o que é o correto (a
//	regra é "como cobramos hoje", não uma foto do passado).
package locacoes

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// regraResolvida decide qual das três regras vale pra um período: o
// equipamento vence o fornecedor vence "padrao".
func regraResolvida(regraEquipamento, regraFornecedor string) string {
	if regraEquipamento != "" && regraEquipamento != "padrao" {
		return regraEquipamento
	}
	if regraFornecedor != "" && regraFornecedor != "padrao" {
		return regraFornecedor
	}
	return "padrao"
}

// calcularValorPeriodo devolve o valor em reais de um período — a única
// função que sabe a conta inteira, pra nunca ter duas contas discordando.
func calcularValorPeriodo(numero int, inicio, fim time.Time, qtd, valorUnit float64, regra string) float64 {
	mesCheio := regra == "sempre_mes_cheio" || (regra != "sempre_proporcional" && numero == 1)
	if mesCheio {
		return valorUnit * qtd
	}
	dias := fim.Sub(inicio).Hours() / 24
	if dias < 0 {
		dias = 0
	}
	return (valorUnit / 30) * dias * qtd
}

// ---------------------------------------------------------------------------
// GET /locacoes/faturamento — resumo do que está calculado hoje, por obra
// ---------------------------------------------------------------------------

func (m *Modulo) faturamento(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaMonitorar)
	if p == nil {
		return
	}

	var equipamentos []map[string]any
	if err := m.bd.Buscar(r.Context(), "locacoes_equipamentos?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,descricao,obra_centro_custo,regra_faturamento,fornecedores(razao_social,regra_faturamento)"+
		"&limit="+fmt.Sprint(TetoDaLista), &equipamentos); err != nil {
		m.erro(w, "não consegui listar os equipamentos para o faturamento", err)
		return
	}
	if len(equipamentos) == 0 {
		web.Responder(w, http.StatusOK, map[string]any{"total": 0.0, "por_obra": []any{}})
		return
	}

	regraPorEquipamento := make(map[string]string, len(equipamentos))
	obraPorEquipamento := make(map[string]string, len(equipamentos))
	ids := make([]string, 0, len(equipamentos))
	for _, e := range equipamentos {
		id := strCampo(e["id"])
		ids = append(ids, id)
		fornecedor, _ := e["fornecedores"].(map[string]any)
		regraFornecedor := ""
		if fornecedor != nil {
			regraFornecedor = strCampo(fornecedor["regra_faturamento"])
		}
		regraPorEquipamento[id] = regraResolvida(strCampo(e["regra_faturamento"]), regraFornecedor)
		obraPorEquipamento[id] = strCampo(e["obra_centro_custo"])
	}

	periodos, err := m.periodosDosEquipamentos(r.Context(), ids)
	if err != nil {
		m.erro(w, "não consegui listar os períodos para o faturamento", err)
		return
	}

	total := 0.0
	porObra := map[string]float64{}
	for _, per := range periodos {
		equipamentoID := strCampo(per["equipamento_id"])
		inicio, err1 := parseData(strCampo(per["inicio"]))
		fim, err2 := parseData(strCampo(per["fim"]))
		if err1 != nil || err2 != nil {
			continue
		}
		valor := calcularValorPeriodo(int(numCampo(per["numero"])), inicio, fim,
			numCampo(per["qtd"]), numCampo(per["valor_unit"]), regraPorEquipamento[equipamentoID])
		total += valor
		porObra[obraPorEquipamento[equipamentoID]] += valor
	}

	saida := make([]map[string]any, 0, len(porObra))
	for obra, valor := range porObra {
		if obra == "" {
			continue
		}
		saida = append(saida, map[string]any{"obra_centro_custo": obra, "total_calculado": valor})
	}

	web.Responder(w, http.StatusOK, map[string]any{"total": total, "por_obra": saida})
}

// periodosDosEquipamentos busca todos os períodos de uma lista de
// equipamentos numa consulta só (`in.(...)`).
func (m *Modulo) periodosDosEquipamentos(ctx context.Context, equipamentoIDs []string) ([]map[string]any, error) {
	if len(equipamentoIDs) == 0 {
		return nil, nil
	}
	lista := equipamentoIDs[0]
	for _, id := range equipamentoIDs[1:] {
		lista += "," + id
	}
	var periodos []map[string]any
	err := m.bd.Buscar(ctx, "locacoes_periodos?equipamento_id=in.("+lista+")"+
		"&select=equipamento_id,numero,inicio,fim,qtd,valor_unit", &periodos)
	return periodos, err
}
