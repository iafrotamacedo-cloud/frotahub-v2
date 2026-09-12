// rev 1 — o retrato de OCs excluídas/substituídas depois do envio (12/09/2026)
//
// O PROCESSO DE NEGÓCIO, DO JEITO QUE O DONO EXPLICOU
//
//	Uma OC pode ser excluída por desistência da compra a qualquer momento,
//	MESMO depois do PCO já ter sido enviado ao cliente — e não é problema:
//	o PCO não faz fechamento financeiro, é só um gabarito. A regra do
//	cliente é "pode sobrar, não pode faltar" — um PCO sem nota nunca chega a
//	incomodar ninguém; uma nota sem PCO, sim. Por isso `excluirOrdem`
//	(substituicao.go) não bloqueia mais depois do envio.
//
//	Também existe a substituição de uma OC por outra DIFERENTE (nota veio
//	com valor diferente do previsto, fornecedor mudou o CNPJ na hora de
//	faturar, o Obra Prima pediu reaprovação, etc.) — ao contrário do
//	`substituirOrdem` de faturamento errado, aqui NÃO precisa ser o mesmo
//	número: o sistema aceita "a mais" tranquilamente.
//
// SÓ REGISTRA O QUE JÁ FOI ENVIADO
//
//	Excluir/substituir uma OC que nunca chegou a ser enviada continua sem
//	rastro nenhum (o ID morre ali, como já combinado) — este arquivo só
//	entra em ação quando `pco_enviado_em` já estava preenchido.
package administrativo

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

type tipoCancelamento string

const (
	tipoExcluida    tipoCancelamento = "excluida"
	tipoSubstituida tipoCancelamento = "substituida"
)

// ordemParaCancelamento busca só o que o retrato precisa — chamado ANTES de
// apagar a OC de vez, tanto por excluir quanto por substituir.
func (m *Modulo) ordemParaCancelamento(ctx context.Context, clienteID, id string) (map[string]any, error) {
	return m.contarUm(ctx, "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(clienteID)+
		"&select=id,status,numero,obra_centro_custo,comprador_nome,comprador_cnpj,"+
		"fornecedor_id,total,nome_arquivo,arquivo_sha256,pco_enviado_em&limit=1")
}

// registrarCancelamento grava o retrato em `ordens_compra_canceladas`. Só
// deve ser chamado quando a OC já tinha sido enviada — quem chama já
// conferiu isso (`temPCOEnviado`).
func (m *Modulo) registrarCancelamento(ctx context.Context, p *seguranca.Principal,
	ordem map[string]any, tipo tipoCancelamento, substitutaID, substitutaNumero string) error {
	fornecedorNome, fornecedorCNPJ := "", ""
	if fid := strCampo(ordem["fornecedor_id"]); fid != "" {
		if f, err := m.contarUm(ctx, "fornecedores?id=eq."+banco.Escapar(fid)+
			"&select=razao_social,cnpj&limit=1"); err == nil {
			fornecedorNome = strCampo(f["razao_social"])
			fornecedorCNPJ = strCampo(f["cnpj"])
		}
	}
	linha := map[string]any{
		"cliente_id":        p.ClienteID,
		"tipo":              string(tipo),
		"numero":            textoOuNil(strCampo(ordem["numero"])),
		"obra_centro_custo": textoOuNil(strCampo(ordem["obra_centro_custo"])),
		"comprador_nome":    textoOuNil(strCampo(ordem["comprador_nome"])),
		"comprador_cnpj":    textoOuNil(strCampo(ordem["comprador_cnpj"])),
		"fornecedor_nome":   textoOuNil(fornecedorNome),
		"fornecedor_cnpj":   textoOuNil(fornecedorCNPJ),
		"valor":             numeroDeJSON(ordem["total"]),
		"nome_arquivo":      strCampo(ordem["nome_arquivo"]),
		"arquivo_sha256":    textoOuNil(strCampo(ordem["arquivo_sha256"])),
		"enviado_em":        ordem["pco_enviado_em"],
		"removida_por":      textoOuNil(p.UserID),
	}
	if substitutaID != "" {
		linha["substituta_id"] = substitutaID
		linha["substituta_numero"] = textoOuNil(substitutaNumero)
	}
	return m.bd.Inserir(ctx, "ordens_compra_canceladas", []map[string]any{linha}, nil)
}

// ---------------------------------------------------------------------------
// GET /administrativo/compras/pco/canceladas
// ---------------------------------------------------------------------------

func (m *Modulo) listarCanceladas(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	caminho := "ordens_compra_canceladas?cliente_id=eq." + banco.Escapar(p.ClienteID)
	if tipo := strings.TrimSpace(r.URL.Query().Get("tipo")); tipo == "excluida" || tipo == "substituida" {
		caminho += "&tipo=eq." + tipo
	}
	if desde := strings.TrimSpace(r.URL.Query().Get("desde")); desde != "" {
		caminho += "&removida_em=gte." + banco.Escapar(desde)
	}
	if ate := strings.TrimSpace(r.URL.Query().Get("ate")); ate != "" {
		caminho += "&removida_em=lte." + banco.Escapar(ate)
	}
	caminho += "&order=removida_em.desc" +
		"&select=id,tipo,numero,obra_centro_custo,comprador_nome,comprador_cnpj," +
		"fornecedor_nome,fornecedor_cnpj,valor,nome_arquivo,enviado_em,removida_em," +
		"substituta_numero,substituta_id&limit=" + fmt.Sprint(TetoDaLista)
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar as OCs excluídas/substituídas", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ordens": ouVazio(linhas)})
}

// ---------------------------------------------------------------------------
// GET /administrativo/compras/pco/canceladas/{id}/arquivo
// ---------------------------------------------------------------------------

func (m *Modulo) arquivoDaCancelada(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	linha, err := m.contarUm(r.Context(), "ordens_compra_canceladas?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=nome_arquivo,arquivo_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei este registro", err)
		return
	}
	sha := strCampo(linha["arquivo_sha256"])
	if sha == "" {
		web.Falhar(w, http.StatusNotFound, "Este registro não tem arquivo guardado.")
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
	web.Responder(w, http.StatusOK, map[string]any{
		"url":  link,
		"nome": linha["nome_arquivo"],
	})
}
