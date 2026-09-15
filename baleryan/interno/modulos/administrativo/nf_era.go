// ERA READ no recebimento de NF — escaneamento pagina a pagina.
package administrativo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/era-regen/integrar"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// resultadoScan e o que o ERA READ devolve ao escanear uma pagina.
type resultadoScan struct {
	SHA       string
	Leitura   map[string]any
	Sugestao  integrar.SugestaoNF
	ERAAtivo  bool
}

// POST /administrativo/nf/ordens/{id}/escanear — le a pagina sem gravar (preview).
func (m *Modulo) escanearNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if _, err := m.ordemProntaParaReceber(r, p, id); err != nil {
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	}
	if m.era == nil || !m.era.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O ERA READ não está configurado neste servidor — preencha número e valor manualmente.")
		return
	}
	raw, err := m.bytesDoArquivo(r, "arquivo")
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := m.lerPaginaNF(r.Context(), p, id, "", 1, raw)
	if err != nil {
		web.Falhar(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"numero":      res.Sugestao.Numero,
		"valor":       res.Sugestao.Valor,
		"tipo":        res.Sugestao.Tipo,
		"leitura_era": res.Leitura,
		"era_read":    true,
	})
}

// POST /administrativo/nf/notas/{id}/paginas — salva pagina 2+ de uma NF ja criada.
func (m *Modulo) paginaNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	nfID, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado.")
		return
	}
	nota, err := m.contarUm(r.Context(), "notas_fiscais?id=eq."+nfID+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,status,ordem_compra_id,cancelada&limit=1")
	if err != nil {
		m.erro(w, "não achei esta nota fiscal", err)
		return
	}
	if nota == nil || fmtStatus(nota["status"]) != "recebida" || boolCampo(nota["cancelada"]) {
		web.Falhar(w, http.StatusConflict, "Só é possível acrescentar páginas a uma nota recebida e ativa.")
		return
	}
	ocID := strCampo(nota["ordem_compra_id"])
	if _, err := m.ordemProntaParaReceber(r, p, ocID); err != nil {
		web.Falhar(w, http.StatusForbidden, err.Error())
		return
	}

	raw, err := m.bytesDoArquivo(r, "arquivo")
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}
	pagina, err := m.proximaPaginaNF(r.Context(), nfID)
	if err != nil {
		m.erro(w, "não consegui contar as páginas desta nota", err)
		return
	}
	res, err := m.lerPaginaNF(r.Context(), p, ocID, nfID, pagina, raw)
	if err != nil {
		web.Falhar(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := m.bd.Inserir(r.Context(), "notas_fiscais_paginas", []map[string]any{{
		"cliente_id":     p.ClienteID,
		"nota_fiscal_id": nfID,
		"pagina":         pagina,
		"arquivo_sha256": res.SHA,
		"leitura_era":    res.Leitura,
	}}, nil); err != nil {
		m.erro(w, "não consegui gravar esta página da nota", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", nfID, "pagina_nota_fiscal", map[string]historico.Mudanca{
		"pagina": {De: nil, Para: pagina},
	})
	web.Responder(w, http.StatusOK, map[string]any{
		"id":          nfID,
		"pagina":      pagina,
		"era_read":    res.ERAAtivo,
		"leitura_era": res.Leitura,
	})
}

func (m *Modulo) ordemProntaParaReceber(r *http.Request, p *seguranca.Principal, ocID string) (map[string]any, error) {
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+ocID+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,status,numero,obra_centro_custo,pco_enviado_em&limit=1")
	if err != nil {
		return nil, fmt.Errorf("não achei esta ordem de compra")
	}
	if ordem == nil {
		return nil, fmt.Errorf("não achei esta ordem de compra")
	}
	if fmtStatus(ordem["status"]) != "lido" || !temPCOEnviado(ordem["pco_enviado_em"]) {
		return nil, fmt.Errorf("esta ordem de compra ainda não foi enviada ao cliente")
	}
	acesso, err := m.temAcessoAObra(r.Context(), p, strCampo(ordem["obra_centro_custo"]))
	if err != nil {
		return nil, fmt.Errorf("não consegui conferir o acesso a esta obra")
	}
	if !acesso {
		return nil, fmt.Errorf("você não tem acesso liberado para receber notas fiscais desta obra")
	}
	return ordem, nil
}

func (m *Modulo) proximaPaginaNF(ctx context.Context, nfID string) (int, error) {
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, "notas_fiscais_paginas?nota_fiscal_id=eq."+nfID+"&select=pagina", &linhas); err != nil {
		return 0, err
	}
	return len(linhas) + 2, nil
}

func (m *Modulo) lerPaginaNF(ctx context.Context, p *seguranca.Principal, ocID, nfID string, pagina int, raw []byte) (resultadoScan, error) {
	out := resultadoScan{ERAAtivo: m.era != nil && m.era.Ligado()}
	bytesGuardar := raw

	if out.ERAAtivo {
		meta := integrar.Lancamento{
			LeituraID:  novoIDLeitura(),
			NotaID:     nfID,
			LancadoPor: p.UserID,
			LancadoEm:  time.Now().UTC(),
		}
		if meta.NotaID == "" {
			meta.NotaID = ocID
		}
		saida, err := m.era.Ler(raw, meta)
		if err != nil {
			return out, fmt.Errorf("não consegui ler a nota com o ERA READ: %v", err)
		}
		out.Sugestao = integrar.SugestaoDeSaida(saida)
		if len(saida.Preparo.ColorJPEG) > 0 {
			bytesGuardar = saida.Preparo.ColorJPEG
		}
		out.Leitura = leituraParaMapa(saida.Leitura)
	}

	nome := fmt.Sprintf("nf-p%d.jpg", pagina)
	sha, err := m.guardarArquivoNF(ctx, p, bytesGuardar, nome)
	if err != nil {
		return out, err
	}
	out.SHA = sha
	return out, nil
}

func leituraParaMapa(l any) map[string]any {
	b, err := json.Marshal(l)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

func (m *Modulo) bytesDoArquivo(r *http.Request, campo string) ([]byte, error) {
	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		return nil, fmt.Errorf("não consegui ler o que foi enviado")
	}
	cabs := r.MultipartForm.File[campo]
	if len(cabs) == 0 {
		return nil, fmt.Errorf("escaneie a nota fiscal")
	}
	f, err := cabs[0].Open()
	if err != nil {
		return nil, fmt.Errorf("não consegui abrir o arquivo enviado")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, TamanhoMaximo+1))
	if err != nil || len(b) == 0 {
		return nil, fmt.Errorf("não consegui ler o arquivo enviado")
	}
	if len(b) > TamanhoMaximo {
		return nil, fmt.Errorf("o arquivo passa de %d MB", TamanhoMaximo>>20)
	}
	return b, nil
}

func novoIDLeitura() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func boolCampo(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "t"
	default:
		return false
	}
}

func aplicarSugestao(numero string, valor float64, sug integrar.SugestaoNF) (string, float64) {
	if strings.TrimSpace(numero) == "" && sug.Numero != "" {
		numero = sug.Numero
	}
	if valor <= 0 && sug.Valor > 0 {
		valor = sug.Valor
	}
	return numero, valor
}

func parseValorNF(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("informe o valor da nota fiscal")
	}
	return v, nil
}
