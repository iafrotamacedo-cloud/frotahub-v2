// rev 1 — correção manual de OCs rejeitadas (Reparar › EDITAR)
//
// A leitura automática grava o que o PDF trouxe e aplica os dois filtros.
// Aqui a pessoa corrige o que faltou ou veio errado — CNPJ do fornecedor,
// obra/centro e CNPJ de faturamento — sem depender de reler o PDF do zero.
// Quando passa nos filtros, a OC vira `lido` e entra em Processadas + PCO
// pendentes (mesma linha, `pco_enviado_em` continua nulo).
package administrativo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// GET /administrativo/compras/obras-centro?q=
// ---------------------------------------------------------------------------

func (m *Modulo) buscarObrasCentro(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		web.Responder(w, http.StatusOK, map[string]any{"obras": []any{}})
		return
	}
	padrao := "*" + banco.Escapar(q) + "*"
	var linhas []map[string]any
	caminho := "ordens_compra?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&status=eq.lido&obra_centro_custo=not.is.null" +
		"&obra_centro_custo=ilike." + padrao +
		"&select=obra_centro_custo,comprador_cnpj,comprador_nome&order=obra_centro_custo&limit=40"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui buscar obras/centro de custo", err)
		return
	}
	vistas := map[string]bool{}
	saida := make([]map[string]any, 0, 12)
	for _, lin := range linhas {
		obra, _ := lin["obra_centro_custo"].(string)
		obra = strings.TrimSpace(obra)
		if obra == "" || vistas[obra] {
			continue
		}
		vistas[obra] = true
		saida = append(saida, map[string]any{
			"obra_centro_custo": obra,
			"comprador_cnpj":    lin["comprador_cnpj"],
			"comprador_nome":    lin["comprador_nome"],
		})
		if len(saida) >= 12 {
			break
		}
	}
	web.Responder(w, http.StatusOK, map[string]any{"obras": ouVazio(saida)})
}

// ---------------------------------------------------------------------------
// GET /administrativo/compras/ordens/{id}/reparo
// ---------------------------------------------------------------------------

func (m *Modulo) estadoReparo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ordem, err := m.ordemParaReparo(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if fmtStatus(ordem["status"]) != "falhou" {
		web.Falhar(w, http.StatusConflict, "Só dá para editar uma OC rejeitada.")
		return
	}
	motivo, _ := ordem["erro_leitura"].(string)
	forn, fat := errosDaRejeicao(motivo)
	web.Responder(w, http.StatusOK, map[string]any{
		"precisa_fornecedor":  forn,
		"precisa_faturamento": fat,
		"motivos":             linhasMotivo(motivo),
		"fornecedor_cnpj":     ordem["fornecedor_cnpj_sugerido"],
		"obra_centro_custo":   ordem["obra_centro_custo"],
		"comprador_cnpj":      ordem["comprador_cnpj"],
		"comprador_nome":      ordem["comprador_nome"],
	})
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/ordens/{id}/reparo
// ---------------------------------------------------------------------------

type corpoReparo struct {
	FornecedorCNPJ  string `json:"fornecedor_cnpj"`
	ObraCentroCusto string `json:"obra_centro_custo"`
	CompradorCNPJ   string `json:"comprador_cnpj"`
	CompradorNome   string `json:"comprador_nome"`
}

func (m *Modulo) aplicarReparo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var corpo corpoReparo
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi o que veio no corpo.")
		return
	}

	ordem, err := m.ordemParaReparo(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if fmtStatus(ordem["status"]) != "falhou" {
		web.Falhar(w, http.StatusConflict, "Esta ordem de compra não está rejeitada.")
		return
	}

	ex, err := m.extraidaAtual(r.Context(), ordem)
	if err != nil {
		m.erro(w, "não consegui montar os dados desta ordem de compra", err)
		return
	}

	motivoAntigo, _ := ordem["erro_leitura"].(string)
	precisaForn, precisaFat := errosDaRejeicao(motivoAntigo)

	if precisaForn {
		cnpj := soDigitos(strings.TrimSpace(corpo.FornecedorCNPJ))
		if len(cnpj) != 14 {
			web.Falhar(w, http.StatusBadRequest, "Informe o CNPJ do fornecedor com 14 dígitos.")
			return
		}
		ex.FornecedorCNPJ = cnpj
		if strings.TrimSpace(ex.FornecedorNome) == "" {
			ex.FornecedorNome = "Fornecedor CNPJ " + cnpj
		}
	}
	if precisaFat {
		obra := strings.TrimSpace(corpo.ObraCentroCusto)
		if obra == "" {
			web.Falhar(w, http.StatusBadRequest, "Informe a obra/centro de custo.")
			return
		}
		ex.ObraCentroCusto = obra
		cnpjFat := soDigitos(strings.TrimSpace(corpo.CompradorCNPJ))
		if cnpjFat != "" {
			ex.CompradorCNPJ = cnpjFat
		}
		if n := strings.TrimSpace(corpo.CompradorNome); n != "" {
			ex.CompradorNome = n
		}
	}

	fornecedorID, ferr := m.resolverFornecedor(r.Context(), p.ClienteID, ex)
	if ferr != nil {
		m.erro(w, "não consegui gravar o fornecedor", ferr)
		return
	}

	status, motivo := "lido", ""
	if motivos := ex.MotivosDeRejeicao(); len(motivos) > 0 {
		status, motivo = "falhou", strings.Join(motivos, "; ")
	}

	if err := m.terminarLeitura(r.Context(), p, id, status, motivo, camposLidos(ex, fornecedorID), &ex); err != nil {
		m.erro(w, "não consegui gravar a correção", err)
		return
	}

	acao := "corrigir_ordem_compra"
	if status == "lido" {
		acao = "reparar_ordem_compra"
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, acao, map[string]historico.Mudanca{
		"status": {De: "falhou", Para: status},
	})

	pf, pft := errosDaRejeicao(motivo)
	web.Responder(w, http.StatusOK, map[string]any{
		"status":              status,
		"motivo":              motivo,
		"motivos":             linhasMotivo(motivo),
		"precisa_fornecedor":  pf,
		"precisa_faturamento": pft,
	})
}

func (m *Modulo) ordemParaReparo(ctx context.Context, clienteID, id string) (map[string]any, error) {
	ordem, err := m.contarUm(ctx, "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(clienteID)+
		"&select=id,status,erro_leitura,numero,obra_centro_custo,comprador_nome,comprador_cnpj,"+
		"fornecedor_id,arquivo_sha256,comprador_interno,cond_pgto,forma_pgto,previsao_entrega,data,"+
		"subtotal,desconto,frete,total&limit=1")
	if err != nil {
		return nil, err
	}
	if fid := fmt.Sprint(ordem["fornecedor_id"]); fid != "" && fid != "<nil>" {
		if f, err := m.contarUm(ctx, "fornecedores?id=eq."+banco.Escapar(fid)+"&select=cnpj&limit=1"); err == nil {
			ordem["fornecedor_cnpj_sugerido"] = f["cnpj"]
		}
	}
	return ordem, nil
}

// extraidaAtual monta o que temos hoje: o PDF relido + o que já está no banco.
func (m *Modulo) extraidaAtual(ctx context.Context, ordem map[string]any) (Extraida, error) {
	ex := extraidaDoBanco(ordem)
	sha, _ := ordem["arquivo_sha256"].(string)
	if sha == "" {
		return ex, nil
	}
	arq, err := m.contarUm(ctx, "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		return ex, nil
	}
	chave, _ := arq["chave_r2"].(string)
	pdf, err := m.arm.Baixar(ctx, chave)
	if err != nil {
		return ex, nil
	}
	lida, err := Ler(ctx, pdf)
	if err != nil {
		return ex, nil
	}
	ex = mesclarExtraida(lida, ex)
	return ex, nil
}

func extraidaDoBanco(ordem map[string]any) Extraida {
	var ex Extraida
	ex.Numero = fmt.Sprint(ordem["numero"])
	if ex.Numero == "<nil>" {
		ex.Numero = ""
	}
	ex.ObraCentroCusto = strCampo(ordem["obra_centro_custo"])
	ex.CompradorNome = strCampo(ordem["comprador_nome"])
	ex.CompradorCNPJ = strCampo(ordem["comprador_cnpj"])
	ex.CompradorInterno = strCampo(ordem["comprador_interno"])
	ex.CondPgto = strCampo(ordem["cond_pgto"])
	ex.FormaPgto = strCampo(ordem["forma_pgto"])
	ex.Data = interpretarData(strCampo(ordem["data"]))
	ex.PrevisaoEntrega = interpretarData(strCampo(ordem["previsao_entrega"]))
	ex.Subtotal = regras.DinheiroDe(numeroDeJSON(ordem["subtotal"]))
	ex.Desconto = regras.DinheiroDe(numeroDeJSON(ordem["desconto"]))
	ex.Frete = regras.DinheiroDe(numeroDeJSON(ordem["frete"]))
	ex.Total = regras.DinheiroDe(numeroDeJSON(ordem["total"]))
	return ex
}

func mesclarExtraida(lida, banco Extraida) Extraida {
	if banco.Numero != "" {
		lida.Numero = banco.Numero
	}
	if banco.ObraCentroCusto != "" {
		lida.ObraCentroCusto = banco.ObraCentroCusto
	}
	if banco.CompradorNome != "" {
		lida.CompradorNome = banco.CompradorNome
	}
	if banco.CompradorCNPJ != "" {
		lida.CompradorCNPJ = banco.CompradorCNPJ
	}
	return lida
}

func strCampo(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func fmtStatus(v any) string {
	return strings.TrimSpace(fmt.Sprint(v))
}

// errosDaRejeicao lê o texto gravado em `erro_leitura` e diz o que falta corrigir.
func errosDaRejeicao(motivo string) (fornecedor, faturamento bool) {
	m := strings.ToLower(motivo)
	if strings.Contains(m, "fornecedor") {
		fornecedor = true
	}
	if strings.Contains(m, "faturamento") || strings.Contains(m, "03720882") {
		faturamento = true
	}
	return
}

// linhasMotivo quebra o motivo em linhas curtas para a barra do visor.
func linhasMotivo(motivo string) []string {
	if strings.TrimSpace(motivo) == "" {
		return []string{}
	}
	partes := strings.Split(motivo, ";")
	saida := make([]string, 0, len(partes))
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if p != "" {
			saida = append(saida, simplificarMotivoAPI(p))
		}
	}
	return saida
}

func simplificarMotivoAPI(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "fornecedor"):
		return "Fornecedor sem CNPJ"
	case strings.Contains(lower, "faturamento") && strings.Contains(lower, "não achei"):
		return "Faturamento sem CNPJ"
	case strings.Contains(lower, "faturamento"):
		return "CNPJ de faturamento errado"
	default:
		return strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return ' '
			}
			return r
		}, s))
	}
}
