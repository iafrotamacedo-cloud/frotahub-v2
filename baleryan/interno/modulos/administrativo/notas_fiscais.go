// rev 1 — Notas Fiscais: o Bloco A (12/09/2026)
//
// O CICLO, DO JEITO QUE O DONO DESCREVEU
//
//	Toda OC que anda o ciclo inteiro (chega a "Enviados" em PCO e não é
//	excluída) espera uma ou mais notas fiscais. O almoxarife na obra escaneia
//	o material recebido — "recebida". Alguém leva a nota física até o
//	escritório, e quem recebe lá confirma — "entregue_escritorio" (essa
//	confirmação, com login de quem recebeu, é o protocolo de amanhã, Bloco
//	B). Por fim, o malote sai para o cliente — "enviada_cliente", fim do
//	ciclo OC-NF. Uma nota nunca cobre mais de uma OC, mas uma OC pode
//	receber várias notas em momentos diferentes (recebimento parcial) — ver
//	o cabeçalho da migração 064.
//
// A OC "COMPLETA" É CALCULADA, NUNCA GRAVADA
//
//	`nf_progresso_ordens` (migração 065) soma as notas não-canceladas de
//	cada OC e compara com o total — a mesma disciplina de nunca duplicar um
//	dado que já existe em outro lugar (CORE-06).
//
// SÓ QUEM TEM A OBRA LIBERADA RECEBE NOTA DELA
//
//	"o almoxarife só pode receber uma nota derivada de uma oc que foi
//	gerada" — e só das obras que alguém de nível superior concedeu a ele
//	(`centro_custo_acessos`, migração 064; concessão em `acessos_obra.go`).
//	O builder e quem tem `COMPRAS_NF_CONFIGURAR_ACESSO` (hoje só CEO) passam
//	direto, sem precisar de concessão — mesma exceção de sempre.
package administrativo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// porteiros — um por rotina, mesmo desenho de `quemPodeDestinatarios`
// ---------------------------------------------------------------------------

func (m *Modulo) quemPodeReceberNF(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	return m.quemComRotina(w, r, RotinaNFReceber)
}

func (m *Modulo) quemPodeEntregarNF(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	return m.quemComRotina(w, r, RotinaNFEntregar)
}

func (m *Modulo) quemComRotina(w http.ResponseWriter, r *http.Request, rotina string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, rotina); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// temAcessoAObra confere `centro_custo_acessos` — a concessão por obra que só
// quem tem `COMPRAS_NF_CONFIGURAR_ACESSO` (ou o builder) pode dar (ver
// `acessos_obra.go`). Quem já tem essa rotina de configurar passa direto: não
// faria sentido um CEO precisar conceder acesso a si mesmo.
func (m *Modulo) temAcessoAObra(ctx context.Context, p *seguranca.Principal, obraCentroCusto string) (bool, error) {
	if p.Builder() {
		return true, nil
	}
	if pode, err := m.perm.Pode(ctx, p, RotinaNFConfigurarAcesso); err == nil && pode {
		return true, nil
	}
	obra := strings.TrimSpace(obraCentroCusto)
	if obra == "" {
		return false, nil
	}
	centro, err := m.contarUm(ctx, "centros_custo?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&obra_centro_custo=eq."+banco.Escapar(obra)+"&select=id&limit=1")
	if err != nil {
		if err == errNaoAchei {
			return false, nil
		}
		return false, err
	}
	centroID := strCampo(centro["id"])
	if centroID == "" {
		return false, nil
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, "centro_custo_acessos?perfil_id=eq."+banco.Escapar(p.UserID)+
		"&centro_custo_id=eq."+banco.Escapar(centroID)+"&select=id&limit=1", &linhas); err != nil {
		return false, err
	}
	return len(linhas) > 0, nil
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/painel — os quatro cartões do hub
// ---------------------------------------------------------------------------

func (m *Modulo) painelDeNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemComRotina(w, r, RotinaNFReceber)
	if p == nil {
		p = m.quemComRotina(w, r, RotinaNFEntregar)
	}
	if p == nil {
		return
	}
	aguardando, err := m.bd.BuscarContando(r.Context(),
		"nf_progresso_ordens?cliente_id=eq."+banco.Escapar(p.ClienteID)+"&completa=eq.false&select=ordem_compra_id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as OCs aguardando nota fiscal", err)
		return
	}
	recebidas, err := m.bd.BuscarContando(r.Context(), filtroDasNF(p.ClienteID, "recebidas")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as notas recebidas", err)
		return
	}
	entregues, err := m.bd.BuscarContando(r.Context(), filtroDasNF(p.ClienteID, "entregues")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as notas entregues no escritório", err)
		return
	}
	enviadas, err := m.bd.BuscarContando(r.Context(), filtroDasNF(p.ClienteID, "enviadas")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as notas enviadas ao cliente", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"aguardando": aguardando,
		"recebidas":  recebidas,
		"entregues":  entregues,
		"enviadas":   enviadas,
	})
}

func filtroDasNF(clienteID, vista string) string {
	base := "notas_fiscais?cliente_id=eq." + banco.Escapar(clienteID) + "&cancelada=eq.false"
	switch vista {
	case "entregues":
		return base + "&status=eq.entregue_escritorio&order=entregue_escritorio_em.desc"
	case "enviadas":
		return base + "&status=eq.enviada_cliente&order=enviada_cliente_em.desc"
	default:
		return base + "&status=eq.recebida&order=recebida_em.desc"
	}
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/aguardando — OCs enviadas, ainda sem nota completa
// ---------------------------------------------------------------------------

func (m *Modulo) ordensAguardandoNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := "nf_progresso_ordens?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&completa=eq.false&order=numero" +
		"&select=ordem_compra_id,numero,obra_centro_custo,fornecedor_id,total,recebido,restante" +
		"&limit=" + fmt.Sprint(TetoDaLista)
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar as OCs aguardando nota fiscal", err)
		return
	}
	linhas = m.comAcessoNaObra(r.Context(), p, linhas)
	m.comNomeDoFornecedor(r.Context(), linhas)
	web.Responder(w, http.StatusOK, map[string]any{"ordens": ouVazio(linhas)})
}

// comAcessoNaObra filtra a lista para as obras que ESTE almoxarife pode
// receber — quem tem `COMPRAS_NF_CONFIGURAR_ACESSO` (ou o builder) já vê
// tudo, sem essa peneira (ver `temAcessoAObra`).
func (m *Modulo) comAcessoNaObra(ctx context.Context, p *seguranca.Principal, linhas []map[string]any) []map[string]any {
	if p.Builder() {
		return linhas
	}
	if pode, err := m.perm.Pode(ctx, p, RotinaNFConfigurarAcesso); err == nil && pode {
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

func (m *Modulo) comNomeDoFornecedor(ctx context.Context, linhas []map[string]any) {
	for _, l := range linhas {
		fid := strCampo(l["fornecedor_id"])
		if fid == "" {
			continue
		}
		if f, err := m.contarUm(ctx, "fornecedores?id=eq."+banco.Escapar(fid)+"&select=razao_social&limit=1"); err == nil {
			l["fornecedor_nome"] = f["razao_social"]
		}
	}
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/ordens/{id} — as notas já recebidas de uma OC
// ---------------------------------------------------------------------------

func (m *Modulo) notasDaOrdem(w http.ResponseWriter, r *http.Request) {
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
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,numero,obra_centro_custo,total&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	var notas []map[string]any
	if err := m.bd.Buscar(r.Context(), "notas_fiscais?ordem_compra_id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&order=recebida_em.desc"+
		"&select=id,numero,valor,status,cancelada,motivo_cancelamento,recebida_em,entregue_escritorio_em,enviada_cliente_em",
		&notas); err != nil {
		m.erro(w, "não consegui listar as notas fiscais desta ordem de compra", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ordem": ordem, "notas": ouVazio(notas)})
}

// ---------------------------------------------------------------------------
// POST /administrativo/nf/ordens/{id}/receber — o almoxarife escaneia a NF
// ---------------------------------------------------------------------------

func (m *Modulo) receberNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
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
			"O armazenamento de arquivos não está configurado. Sem ele, receber a nota seria perdê-la.")
		return
	}
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,status,numero,obra_centro_custo,pco_enviado_em&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if fmtStatus(ordem["status"]) != "lido" || !temPCOEnviado(ordem["pco_enviado_em"]) {
		web.Falhar(w, http.StatusConflict, "Esta ordem de compra ainda não foi enviada ao cliente — não há o que receber.")
		return
	}
	acesso, err := m.temAcessoAObra(r.Context(), p, strCampo(ordem["obra_centro_custo"]))
	if err != nil {
		m.erro(w, "não consegui conferir o acesso a esta obra", err)
		return
	}
	if !acesso {
		web.Falhar(w, http.StatusForbidden, "Você não tem acesso liberado para receber notas fiscais desta obra.")
		return
	}

	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o que foi enviado.")
		return
	}
	numero := strings.TrimSpace(r.FormValue("numero"))
	if numero == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o número da nota fiscal.")
		return
	}
	valor, verr := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(r.FormValue("valor")), ",", "."), 64)
	if verr != nil || valor <= 0 {
		web.Falhar(w, http.StatusBadRequest, "Informe o valor da nota fiscal.")
		return
	}
	cabecalhos := r.MultipartForm.File["arquivo"]
	if len(cabecalhos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escolha a foto ou o PDF da nota fiscal.")
		return
	}
	f, ferr := cabecalhos[0].Open()
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
	if len(conteudo) > TamanhoMaximo {
		web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("O arquivo passa de %d MB.", TamanhoMaximo>>20))
		return
	}
	sha, err := m.guardarArquivoNF(r.Context(), p, conteudo, cabecalhos[0].Filename)
	if err != nil {
		m.erro(w, "não consegui guardar o arquivo da nota fiscal", err)
		return
	}

	var criadas []map[string]any
	if err := m.bd.Inserir(r.Context(), "notas_fiscais", []map[string]any{{
		"cliente_id":      p.ClienteID,
		"ordem_compra_id": id,
		"numero":          numero,
		"valor":           valor,
		"arquivo_sha256":  sha,
		"status":          "recebida",
		"recebida_por":    p.UserID,
	}}, &criadas); err != nil {
		m.erro(w, "não consegui gravar a nota fiscal", err)
		return
	}
	if len(criadas) == 0 {
		m.erro(w, "gravei a nota fiscal mas o banco não devolveu o id", fmt.Errorf("insert sem retorno"))
		return
	}
	nfID := strCampo(criadas[0]["id"])
	_ = m.hist.Registrar(r.Context(), p, "administrativo", nfID, "receber_nota_fiscal", map[string]historico.Mudanca{
		"ordem_compra_id": {De: nil, Para: id},
		"numero":          {De: nil, Para: numero},
	})
	web.Responder(w, http.StatusOK, map[string]any{"id": nfID})
}

// guardarArquivoNF sobe a foto/PDF da nota — mesma receita de `guardarUma`
// (dedup por sha256, upsert em `arquivos`), sem criar linha em
// `ordens_compra`: aqui quem referencia o arquivo é `notas_fiscais`.
func (m *Modulo) guardarArquivoNF(ctx context.Context, p *seguranca.Principal, conteudo []byte, nome string) (string, error) {
	soma := sha256.Sum256(conteudo)
	sha := hex.EncodeToString(soma[:])
	ext := strings.TrimPrefix(strings.ToLower(nomeExtensao(nome)), ".")
	tipoMIME := tipoDoNome(nome)
	chave := armazem.Caminho(p.ClienteID, sha, ext)
	if err := m.arm.Enviar(ctx, chave, bytes.NewReader(conteudo), int64(len(conteudo)), sha, tipoMIME); err != nil {
		return "", fmt.Errorf("não consegui guardar no armazém: %w", err)
	}
	if err := m.bd.Upsert(ctx, "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": p.ClienteID,
		"tamanho":    len(conteudo),
		"tipo":       tipoMIME,
		"chave_r2":   chave,
	}}, nil); err != nil {
		return "", fmt.Errorf("guardei o arquivo mas não consegui registrá-lo: %w", err)
	}
	return sha, nil
}

func nomeExtensao(nome string) string {
	i := strings.LastIndex(nome, ".")
	if i < 0 {
		return ""
	}
	return nome[i:]
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/recebidas · /entregues · /enviadas
// ---------------------------------------------------------------------------

func (m *Modulo) listarNF(w http.ResponseWriter, r *http.Request, vista string) {
	p := m.quemPodeEntregarNF(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := filtroDasNF(p.ClienteID, vista) +
		"&select=id,numero,valor,ordem_compra_id,recebida_em,entregue_escritorio_em,enviada_cliente_em" +
		"&limit=" + fmt.Sprint(TetoDaLista)
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar as notas fiscais", err)
		return
	}
	m.comDadosDaOrdem(r.Context(), linhas)
	web.Responder(w, http.StatusOK, map[string]any{"notas": ouVazio(linhas)})
}

func (m *Modulo) listarNFRecebidas(w http.ResponseWriter, r *http.Request) { m.listarNF(w, r, "recebidas") }
func (m *Modulo) listarNFEntregues(w http.ResponseWriter, r *http.Request) { m.listarNF(w, r, "entregues") }
func (m *Modulo) listarNFEnviadas(w http.ResponseWriter, r *http.Request)  { m.listarNF(w, r, "enviadas") }

func (m *Modulo) comDadosDaOrdem(ctx context.Context, linhas []map[string]any) {
	for _, l := range linhas {
		oid := strCampo(l["ordem_compra_id"])
		if oid == "" {
			continue
		}
		if o, err := m.contarUm(ctx, "ordens_compra?id=eq."+banco.Escapar(oid)+
			"&select=numero,obra_centro_custo&limit=1"); err == nil {
			l["ordem_numero"] = o["numero"]
			l["obra_centro_custo"] = o["obra_centro_custo"]
		}
	}
}

// ---------------------------------------------------------------------------
// POST /administrativo/nf/{id}/entregar — chegou no escritório
// POST /administrativo/nf/{id}/enviar-cliente — saiu no malote
// POST /administrativo/nf/{id}/cancelar — o fornecedor cancelou a nota
// ---------------------------------------------------------------------------

func (m *Modulo) entregarNF(w http.ResponseWriter, r *http.Request) {
	m.avancarNF(w, r, "recebida", "entregue_escritorio", map[string]any{
		"status":                 "entregue_escritorio",
		"entregue_escritorio_em": time.Now().UTC().Format(time.RFC3339),
	}, "entregar_nota_fiscal_escritorio")
}

func (m *Modulo) enviarNFAoCliente(w http.ResponseWriter, r *http.Request) {
	m.avancarNF(w, r, "entregue_escritorio", "enviada_cliente", map[string]any{
		"status":             "enviada_cliente",
		"enviada_cliente_em": time.Now().UTC().Format(time.RFC3339),
	}, "enviar_nota_fiscal_cliente")
}

// avancarNF é o passo comum das duas etapas do escritório — bloqueio
// otimista no FILTRO (`status=eq.<deEsperado>`), mesma disciplina de
// `lerOrdem`: duas confirmações da mesma nota não colidem.
func (m *Modulo) avancarNF(w http.ResponseWriter, r *http.Request, deEsperado, paraStatus string, campos map[string]any, acao string) {
	p := m.quemPodeEntregarNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	quemConfirma := "entregue_confirmado_por"
	if paraStatus == "enviada_cliente" {
		quemConfirma = "enviada_confirmado_por"
	}
	campos[quemConfirma] = p.UserID

	var tomadas []map[string]any
	if err := m.bd.AtualizarDevolvendo(r.Context(), "notas_fiscais",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&cancelada=eq.false&status=eq."+deEsperado,
		campos, &tomadas); err != nil {
		m.erro(w, "não consegui atualizar esta nota fiscal", err)
		return
	}
	if len(tomadas) == 0 {
		web.Falhar(w, http.StatusConflict, "Esta nota fiscal não está na etapa esperada — a lista pode ter mudado, atualize a página.")
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, acao, map[string]historico.Mudanca{
		"status": {De: deEsperado, Para: paraStatus},
	})
	web.Responder(w, http.StatusOK, map[string]any{"status": paraStatus})
}

func (m *Modulo) cancelarNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeEntregarNF(w, r)
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
	motivo := strings.TrimSpace(corpo.Motivo)
	if motivo == "" {
		web.Falhar(w, http.StatusBadRequest, "Explique por que esta nota fiscal está sendo cancelada.")
		return
	}
	if err := m.bd.Atualizar(r.Context(), "notas_fiscais",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
		map[string]any{"cancelada": true, "motivo_cancelamento": motivo}); err != nil {
		m.erro(w, "não consegui cancelar esta nota fiscal", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "cancelar_nota_fiscal", map[string]historico.Mudanca{
		"cancelada": {De: false, Para: true},
	})
	web.Responder(w, http.StatusOK, map[string]any{"cancelada": true})
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/{id}/arquivo
// ---------------------------------------------------------------------------

func (m *Modulo) arquivoDaNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		p = m.quemPodeEntregarNF(w, r)
	}
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	nf, err := m.contarUm(r.Context(), "notas_fiscais?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=numero,arquivo_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei esta nota fiscal", err)
		return
	}
	sha := strCampo(nf["arquivo_sha256"])
	if sha == "" {
		web.Falhar(w, http.StatusNotFound, "Esta nota fiscal não tem arquivo guardado.")
		return
	}
	arq, err := m.contarUm(r.Context(), "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		m.erro(w, "não achei o arquivo", err)
		return
	}
	chave, _ := arq["chave_r2"].(string)
	link, err := m.arm.LinkTemporario(chave, ValidadeDoLink)
	if err != nil {
		m.erro(w, "não consegui montar o endereço do arquivo", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"url": link, "nome": nf["numero"]})
}
