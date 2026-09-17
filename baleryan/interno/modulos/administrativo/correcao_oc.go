// rev 1 — Correção de OC: NF com valor divergente (migração 076, 17/09/2026)
//
// O CICLO, DO JEITO QUE O DONO EXPLICOU
//
//	NF com valor diferente do total da OC tem três causas: item faltando (ou,
//	raro, sobrando), a nota é parcial mesmo (já resolvido, ver o cabeçalho de
//	`notas_fiscais.go`), ou o produto é cobrado por peso e a balança do
//	fornecedor não bate exatamente com a OC. Nos dois primeiros — faltando/
//	sobrando e peso — quem resolve é o RC, corrigindo a OC no Obra Prima e
//	subindo o PDF certo aqui.
//
//	Um percentual (`LimiarDivergenciaOCPercent`) decide o automático: dentro
//	da faixa, a OC sai IMEDIATAMENTE de "Aguardando NF" pra "Correção de OC"
//	assim que a NF é recebida — sem ninguém apertar nada
//	(`avaliarDivergenciaOC`, chamada de `receberNF` e de `trocarNF`). Fora da
//	faixa, a OC continua em Aguardando NF até a equipe da obra (RO) decidir
//	mandar pra correção mesmo assim (`marcarCorrecaoManual`) — nunca sai
//	sozinha quando a diferença é grande demais pra presumir.
//
// POR QUE `correcao_origem` EXISTE
//
//	Pra `avaliarDivergenciaOC` nunca desmarcar uma correção que uma PESSOA
//	decidiu (manual) — só o "voltar pra fila inicial" (RC,
//	`voltarOCParaAguardando`) ou um encaixe exato de valor desmarcam. Ver o
//	cabeçalho da migração 076 pro raciocínio completo.
//
// "CORRIGIR" REAPROVEITA O SUBSTITUIR QUE JÁ EXISTE, COM UM PASSO A MAIS
//
//	`substituirOrdemPCO` (substituicao.go) já sabe ler um PDF novo, criar a
//	OC nova do zero e apagar a velha — é o mesmo "a nota veio com valor
//	diferente do previsto" que o cabeçalho daquele arquivo já cita. A
//	diferença aqui é que a OC velha pode já ter nota(s) fiscal(is)
//	recebida(s): `notas_fiscais.ordem_compra_id` tem `on delete restrict`,
//	então apagar a OC velha sem primeiro MIGRAR essas notas pra OC nova
//	quebraria a chave estrangeira. `corrigirOrdemComDivergencia` faz
//	exatamente isso — migra as notas (voltam pra `recebida`, nunca mais à
//	frente disso: a nota nova sempre precisa ser entregue no escritório de
//	novo) — e só então apaga a OC velha.
package administrativo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// LimiarDivergenciaOCPercent — "um percentual de 2-3% cobre 95% dos casos"
// (o dono). Fixo no código por ora; se um dia precisar ser editável por
// tela, é só um campo a mais em algum lugar de configuração.
const LimiarDivergenciaOCPercent = 3

// dentroDoLimiarDivergencia decide se `recebido` está perto o bastante de
// `total` pra ser um desvio "normal" (peso, item a menos) e não uma OC
// errada de verdade. Tudo em centavos (regras.Dinheiro), nunca em float —
// dinheiro não divide em ponto flutuante (P-12).
func dentroDoLimiarDivergencia(total, recebido regras.Dinheiro) bool {
	if total <= 0 {
		return false
	}
	diff := recebido - total
	if diff < 0 {
		diff = -diff
	}
	if diff == 0 {
		return false // bate exato — não é divergência nenhuma, é o caso normal
	}
	return int64(diff)*100 <= int64(LimiarDivergenciaOCPercent)*int64(total)
}

// avaliarDivergenciaOC roda depois de QUALQUER mudança no valor recebido de
// uma OC (uma NF nova, ou uma NF trocada) — nunca sobrescreve uma marcação
// manual, e só desmarca sozinha quando o valor bate exato (ver o cabeçalho
// deste arquivo).
func (m *Modulo) avaliarDivergenciaOC(ctx context.Context, p *seguranca.Principal, ordemID string) {
	progresso, err := m.contarUm(ctx, "nf_progresso_ordens?ordem_compra_id=eq."+banco.Escapar(ordemID)+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=total,recebido,aguardando_correcao&limit=1")
	if err != nil {
		log.Printf("administrativo: não consegui reavaliar a divergência da OC %s: %v", ordemID, err)
		return
	}
	total := regras.DinheiroDe(numeroDeJSON(progresso["total"]))
	recebido := regras.DinheiroDe(numeroDeJSON(progresso["recebido"]))
	jaMarcada, _ := progresso["aguardando_correcao"].(bool)

	if recebido == total {
		if jaMarcada {
			m.definirCorrecao(ctx, p, ordemID, false, "")
		}
		return
	}
	if jaMarcada {
		return // já está na fila (automática ou manual) — não mexe sozinho
	}
	if dentroDoLimiarDivergencia(total, recebido) {
		m.definirCorrecao(ctx, p, ordemID, true, "automatica")
	}
}

// definirCorrecao é o único lugar que grava `aguardando_correcao` — usado
// pelo cálculo automático e pelos dois botões manuais (marcar/voltar).
func (m *Modulo) definirCorrecao(ctx context.Context, p *seguranca.Principal, ordemID string, marcada bool, origem string) error {
	campos := map[string]any{"aguardando_correcao": marcada}
	if marcada {
		campos["correcao_origem"] = origem
		campos["correcao_marcada_em"] = time.Now().UTC().Format(time.RFC3339)
		campos["correcao_marcada_por"] = p.UserID
	} else {
		campos["correcao_origem"] = nil
		campos["correcao_marcada_em"] = nil
		campos["correcao_marcada_por"] = nil
	}
	if err := m.bd.Atualizar(ctx, "ordens_compra",
		"id=eq."+ordemID+"&cliente_id=eq."+banco.Escapar(p.ClienteID), campos); err != nil {
		log.Printf("administrativo: não consegui gravar a correção da OC %s: %v", ordemID, err)
		return err
	}
	acao := "desmarcar_oc_aguardando_correcao"
	if marcada {
		acao = "marcar_oc_aguardando_correcao"
	}
	_ = m.hist.Registrar(ctx, p, "administrativo", ordemID, acao, map[string]historico.Mudanca{
		"aguardando_correcao": {De: !marcada, Para: marcada},
	})
	return nil
}

// ---------------------------------------------------------------------------
// POST /administrativo/nf/ordens/{id}/marcar-correcao — o RO força manualmente
// ---------------------------------------------------------------------------

func (m *Modulo) marcarCorrecaoManual(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,obra_centro_custo,aguardando_correcao&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	acesso, err := m.temAcessoAObra(r.Context(), p, strCampo(ordem["obra_centro_custo"]))
	if err != nil {
		m.erro(w, "não consegui conferir o acesso a esta obra", err)
		return
	}
	if !acesso {
		web.Falhar(w, http.StatusForbidden, "Você não tem esta obra liberada.")
		return
	}
	if b, _ := ordem["aguardando_correcao"].(bool); b {
		web.Falhar(w, http.StatusConflict, "Esta OC já está na fila de correção.")
		return
	}
	if err := m.definirCorrecao(r.Context(), p, id, true, "manual"); err != nil {
		m.erro(w, "não consegui mandar esta OC para a correção", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"aguardando_correcao": true})
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/ordens/{id}/voltar-aguardando — o RC desiste
// ---------------------------------------------------------------------------

func (m *Modulo) voltarOCParaAguardando(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,aguardando_correcao&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if b, _ := ordem["aguardando_correcao"].(bool); !b {
		web.Falhar(w, http.StatusConflict, "Esta OC não está na fila de correção.")
		return
	}
	if err := m.definirCorrecao(r.Context(), p, id, false, ""); err != nil {
		m.erro(w, "não consegui voltar esta OC para Aguardando NF", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"aguardando_correcao": false})
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/ordens/{id}/corrigir — o RC sobe a OC certa
// ---------------------------------------------------------------------------

func (m *Modulo) corrigirOrdemComDivergencia(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
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
			"O armazenamento de arquivos não está configurado. Sem ele, corrigir perderia o arquivo.")
		return
	}
	ordem, err := m.ordemParaCancelamento(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if b, _ := ordem["aguardando_correcao"].(bool); !b {
		web.Falhar(w, http.StatusConflict, "Esta OC não está na fila de correção.")
		return
	}

	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo enviado.")
		return
	}
	cabecalhos := r.MultipartForm.File["arquivo"]
	if len(cabecalhos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escolha o PDF da OC corrigida.")
		return
	}
	cabecalho := cabecalhos[0]

	f, ferr := cabecalho.Open()
	if ferr != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui abrir o arquivo enviado.")
		return
	}
	conteudo, rerr := io.ReadAll(io.LimitReader(f, TamanhoMaximo+1))
	f.Close()
	if rerr != nil || len(conteudo) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo enviado.")
		return
	}
	soma := sha256.Sum256(conteudo)
	shaNovo := hex.EncodeToString(soma[:])
	shaAntigo := strCampo(ordem["arquivo_sha256"])
	if shaNovo == shaAntigo {
		web.Falhar(w, http.StatusBadRequest,
			"Este é o mesmo arquivo de antes — escolha o PDF já corrigido no Obra Prima.")
		return
	}

	// Só confere que É uma OC de verdade — como em `substituirOrdemPCO`, o
	// número pode ser outro (a compra certa pode nem ter o mesmo número no
	// Obra Prima).
	novaLeitura, lerErr := Ler(r.Context(), conteudo)
	if lerErr != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler este PDF como uma Ordem de Compra: "+lerErr.Error())
		return
	}

	novoID, _, err := m.guardarUma(r.Context(), p, cabecalho, "")
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}

	// MIGRA AS NOTAS FISCAIS ANTES DE APAGAR A OC VELHA — na ordem certa: se
	// isto falhar, a OC velha continua com as notas dela, presa na fila de
	// correção, em vez de a exclusão quebrar a chave estrangeira no meio do
	// caminho (`notas_fiscais.ordem_compra_id` é `on delete restrict`).
	//
	// "nunca à frente" (pedido do dono): a nota volta sempre pra `recebida`,
	// mesmo se já tinha sido entregue no escritório ou enviada ao cliente —
	// a nota nova sempre precisa passar pelas duas etapas de novo, porque é
	// uma OC diferente que está sendo entregue agora.
	if err := m.bd.Atualizar(r.Context(), "notas_fiscais",
		"ordem_compra_id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID), map[string]any{
			"ordem_compra_id":         novoID,
			"status":                  "recebida",
			"entregue_escritorio_em":  nil,
			"entregue_confirmado_por": nil,
			"enviada_cliente_em":      nil,
			"enviada_confirmado_por":  nil,
		}); err != nil {
		m.erro(w, "guardei a OC corrigida mas não consegui migrar a(s) nota(s) fiscal(is) pra ela", err)
		return
	}

	if temPCOEnviado(ordem["pco_enviado_em"]) {
		if err := m.registrarCancelamento(r.Context(), p, ordem, tipoSubstituida, novoID, novaLeitura.Numero); err != nil {
			m.erro(w, "migrei as notas fiscais mas não consegui registrar a substituição da OC antiga", err)
			return
		}
	}

	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "corrigir_ordem_compra", map[string]historico.Mudanca{
		"arquivo_sha256": {De: shaAntigo, Para: shaNovo},
	})
	if err := m.apagarOrdemDeVez(r.Context(), p.ClienteID, id, shaAntigo); err != nil {
		m.erro(w, "migrei as notas fiscais mas não consegui remover a ordem de compra antiga", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"nova_ordem_id": novoID})
}
