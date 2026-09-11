// rev 1 — editor da OC: lê o PDF, devolve o documento, gera outro e substitui
//
// GET monta o JSON do editor a partir do PDF atual (palavras-chave). POST
// desenha um PDF novo, sobe no armazém, aponta a ordem para o sha novo e
// apaga o arquivo antigo se ninguém mais o usa. O nome do arquivo fica.
package administrativo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

type itemDocumento struct {
	Descricao string  `json:"descricao"`
	Qtd       float64 `json:"qtd"`
	Unidade   string  `json:"unidade"`
	ValorUnit float64 `json:"valor_unit"`
	Desconto  float64 `json:"desconto"`
	Total     float64 `json:"total"`
}

type documentoJSON struct {
	Numero              string          `json:"numero"`
	Data                string          `json:"data"`
	PrevisaoEntrega     string          `json:"previsao_entrega"`
	CondPgto            string          `json:"cond_pgto"`
	FormaPgto           string          `json:"forma_pgto"`
	Observacao          string          `json:"observacao"`
	Titulo              string          `json:"titulo"`
	DataImpressao       string          `json:"data_impressao"`
	EmitenteRazao       string          `json:"emitente_razao"`
	EmitenteEndereco    string          `json:"emitente_endereco"`
	EmitenteContato     string          `json:"emitente_contato"`
	EmitenteCNPJ        string          `json:"emitente_cnpj"`
	ResponsavelNome     string          `json:"responsavel_nome"`
	ResponsavelEmail    string          `json:"responsavel_email"`
	CompradorInterno    string          `json:"comprador_interno"`
	CompradorNome       string          `json:"comprador_nome"`
	CompradorCNPJ       string          `json:"comprador_cnpj"`
	FaturamentoIE       string          `json:"faturamento_ie"`
	FaturamentoEndereco string          `json:"faturamento_endereco"`
	FornecedorNome      string          `json:"fornecedor_nome"`
	FornecedorCNPJ      string          `json:"fornecedor_cnpj"`
	FornecedorTelefone  string          `json:"fornecedor_telefone"`
	FornecedorVendedor  string          `json:"fornecedor_vendedor"`
	FornecedorEmail     string          `json:"fornecedor_email"`
	FornecedorEndereco  string          `json:"fornecedor_endereco"`
	ObraCentroCusto     string          `json:"obra_centro_custo"`
	CNO                 string          `json:"cno"`
	EnderecoEntrega     string          `json:"endereco_entrega"`
	Recebedor           string          `json:"recebedor"`
	EnderecoCobranca    string          `json:"endereco_cobranca"`
	Subtotal            float64         `json:"subtotal"`
	Desconto            float64         `json:"desconto"`
	Frete               float64         `json:"frete"`
	Total               float64         `json:"total"`
	Itens               []itemDocumento `json:"itens"`
}

func extraidaParaJSON(e Extraida) documentoJSON {
	itens := make([]itemDocumento, 0, len(e.Itens))
	for _, it := range e.Itens {
		itens = append(itens, itemDocumento{
			Descricao: it.Descricao,
			Qtd:       it.Qtd,
			Unidade:   it.Unidade,
			ValorUnit: it.ValorUnit.Float(),
			Desconto:  it.Desconto.Float(),
			Total:     it.Total.Float(),
		})
	}
	return documentoJSON{
		Numero:              e.Numero,
		Data:                dataParaTela(e.Data),
		PrevisaoEntrega:     dataParaTela(e.PrevisaoEntrega),
		CondPgto:            e.CondPgto,
		FormaPgto:           e.FormaPgto,
		Observacao:          e.Observacao,
		Titulo:              e.TituloObra,
		DataImpressao:       e.DataImpressao,
		EmitenteRazao:       e.EmitenteRazao,
		EmitenteEndereco:    e.EmitenteEndereco,
		EmitenteContato:     e.EmitenteContato,
		EmitenteCNPJ:        e.EmitenteCNPJ,
		ResponsavelNome:     e.ResponsavelNome,
		ResponsavelEmail:    e.ResponsavelEmail,
		CompradorInterno:    e.CompradorInterno,
		CompradorNome:       e.CompradorNome,
		CompradorCNPJ:       e.CompradorCNPJ,
		FaturamentoIE:       e.FaturamentoIE,
		FaturamentoEndereco: e.FaturamentoEndereco,
		FornecedorNome:      e.FornecedorNome,
		FornecedorCNPJ:      e.FornecedorCNPJ,
		FornecedorTelefone:  e.FornecedorTelefone,
		FornecedorVendedor:  e.FornecedorVendedor,
		FornecedorEmail:     e.FornecedorEmail,
		FornecedorEndereco:  e.FornecedorEndereco,
		ObraCentroCusto:     e.ObraCentroCusto,
		CNO:                 e.CNO,
		EnderecoEntrega:     e.EnderecoEntrega,
		Recebedor:           e.Recebedor,
		EnderecoCobranca:    e.EnderecoCobranca,
		Subtotal:            e.Subtotal.Float(),
		Desconto:            e.Desconto.Float(),
		Frete:               e.Frete.Float(),
		Total:               e.Total.Float(),
		Itens:               itens,
	}
}

func jsonParaExtraida(d documentoJSON) Extraida {
	itens := make([]ItemExtraido, 0, len(d.Itens))
	for _, it := range d.Itens {
		itens = append(itens, ItemExtraido{
			Descricao: strings.TrimSpace(it.Descricao),
			Qtd:       it.Qtd,
			Unidade:   strings.TrimSpace(it.Unidade),
			ValorUnit: regras.DinheiroDe(it.ValorUnit),
			Desconto:  regras.DinheiroDe(it.Desconto),
			Total:     regras.DinheiroDe(it.Total),
		})
	}
	e := Extraida{
		Numero:              strings.TrimSpace(d.Numero),
		Data:                interpretarData(d.Data),
		PrevisaoEntrega:     interpretarData(d.PrevisaoEntrega),
		CondPgto:            strings.TrimSpace(d.CondPgto),
		FormaPgto:           strings.TrimSpace(d.FormaPgto),
		Observacao:          strings.TrimSpace(d.Observacao),
		TituloObra:          strings.TrimSpace(d.Titulo),
		DataImpressao:       strings.TrimSpace(d.DataImpressao),
		EmitenteRazao:       strings.TrimSpace(d.EmitenteRazao),
		EmitenteEndereco:    strings.TrimSpace(d.EmitenteEndereco),
		EmitenteContato:     strings.TrimSpace(d.EmitenteContato),
		EmitenteCNPJ:        strings.TrimSpace(d.EmitenteCNPJ),
		ResponsavelNome:     strings.TrimSpace(d.ResponsavelNome),
		ResponsavelEmail:    strings.TrimSpace(d.ResponsavelEmail),
		CompradorInterno:    strings.TrimSpace(d.CompradorInterno),
		CompradorNome:       strings.TrimSpace(d.CompradorNome),
		CompradorCNPJ:       soDigitos(d.CompradorCNPJ),
		FaturamentoIE:       strings.TrimSpace(d.FaturamentoIE),
		FaturamentoEndereco: strings.TrimSpace(d.FaturamentoEndereco),
		FornecedorNome:      strings.TrimSpace(d.FornecedorNome),
		FornecedorCNPJ:      soDigitos(d.FornecedorCNPJ),
		FornecedorTelefone:  strings.TrimSpace(d.FornecedorTelefone),
		FornecedorVendedor:  strings.TrimSpace(d.FornecedorVendedor),
		FornecedorEmail:     strings.TrimSpace(d.FornecedorEmail),
		FornecedorEndereco:  strings.TrimSpace(d.FornecedorEndereco),
		ObraCentroCusto:     strings.TrimSpace(d.ObraCentroCusto),
		CNO:                 strings.TrimSpace(d.CNO),
		EnderecoEntrega:     strings.TrimSpace(d.EnderecoEntrega),
		Recebedor:           strings.TrimSpace(d.Recebedor),
		EnderecoCobranca:    strings.TrimSpace(d.EnderecoCobranca),
		Frete:               regras.DinheiroDe(d.Frete),
		Itens:               itens,
	}
	e.RecalcularTotais()
	return e
}

func interpretarData(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) >= 10 && s[4] == '-' {
		return s[:10]
	}
	return dataBR(s)
}

func respostaDocumento(e Extraida, status string, pcoEnviado bool) map[string]any {
	motivos := e.MotivosDeRejeicao()
	forn, fat := errosDaRejeicao(strings.Join(motivos, "; "))
	return map[string]any{
		"documento":           extraidaParaJSON(e),
		"status":              status,
		"pco_enviado":         pcoEnviado,
		"motivos":             linhasMotivo(strings.Join(motivos, "; ")),
		"precisa_fornecedor":  forn,
		"precisa_faturamento": fat,
	}
}

// GET /administrativo/compras/ordens/{id}/documento
func (m *Modulo) verDocumento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ordem, err := m.ordemParaDocumento(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	ex, err := m.extraidaParaEditor(r.Context(), ordem)
	if err != nil {
		m.erro(w, "não consegui ler o PDF desta ordem de compra", err)
		return
	}
	web.Responder(w, http.StatusOK, respostaDocumento(ex, strCampo(ordem["status"]), temPCOEnviado(ordem["pco_enviado_em"])))
}

// POST /administrativo/compras/ordens/{id}/documento
func (m *Modulo) salvarDocumento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var corpo struct {
		Documento documentoJSON `json:"documento"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi o que veio no corpo.")
		return
	}
	ordem, err := m.ordemParaDocumento(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if fmtStatus(ordem["status"]) == "lido" {
		web.Falhar(w, http.StatusConflict, "Esta ordem de compra já foi processada e não pode ser editada.")
		return
	}

	ex := jsonParaExtraida(corpo.Documento)
	if ex.Numero == "" {
		web.Falhar(w, http.StatusBadRequest, "A ordem de compra precisa de número.")
		return
	}
	if len(ex.Itens) == 0 {
		web.Falhar(w, http.StatusBadRequest, "A ordem de compra precisa de ao menos um item.")
		return
	}
	for _, it := range ex.Itens {
		if strings.TrimSpace(it.Descricao) == "" {
			web.Falhar(w, http.StatusBadRequest, "Todo item precisa de descrição.")
			return
		}
	}

	if err := m.numeroLivre(r.Context(), p.ClienteID, id, ex.Numero); err != nil {
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	}

	pdf, err := desenharOC(ex)
	if err != nil {
		m.erro(w, "não consegui montar o PDF", err)
		return
	}

	shaAntigo, _ := ordem["arquivo_sha256"].(string)
	shaNovo, err := m.guardarPDFOrdem(r.Context(), p, pdf)
	if err != nil {
		m.erro(w, "não consegui guardar o PDF novo", err)
		return
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

	campos := camposLidos(ex, fornecedorID)
	campos["arquivo_sha256"] = shaNovo
	campos["status"] = status
	campos["erro_leitura"] = textoOuNil(motivo)
	if err := m.bd.Atualizar(r.Context(), "ordens_compra",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID), campos); err != nil {
		if banco.Duplicado(err) {
			web.Falhar(w, http.StatusConflict, "Já existe uma ordem de compra com este número.")
			return
		}
		m.erro(w, "não consegui gravar a ordem de compra", err)
		return
	}
	m.gravarItens(r.Context(), id, ex.Itens)

	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "editar_documento_oc", map[string]historico.Mudanca{
		"arquivo_sha256": {De: shaAntigo, Para: shaNovo},
		"status":         {De: strCampo(ordem["status"]), Para: status},
	})

	if shaAntigo != "" && shaAntigo != shaNovo {
		_ = apagarArquivoSeOrfao(r.Context(), m.bd, m.arm, shaAntigo)
	}

	saida := respostaDocumento(ex, status, temPCOEnviado(ordem["pco_enviado_em"]))
	saida["motivo"] = motivo
	web.Responder(w, http.StatusOK, saida)
}

func (m *Modulo) ordemParaDocumento(ctx context.Context, clienteID, id string) (map[string]any, error) {
	ordem, err := m.contarUm(ctx, "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(clienteID)+
		"&select=id,status,erro_leitura,numero,obra_centro_custo,comprador_nome,comprador_cnpj,"+
		"fornecedor_id,arquivo_sha256,comprador_interno,cond_pgto,forma_pgto,previsao_entrega,data,"+
		"subtotal,desconto,frete,total,pco_enviado_em,nome_arquivo&limit=1")
	if err != nil {
		return nil, err
	}
	if fid := fmt.Sprint(ordem["fornecedor_id"]); fid != "" && fid != "<nil>" {
		if f, err := m.contarUm(ctx, "fornecedores?id=eq."+banco.Escapar(fid)+"&select=cnpj,razao_social&limit=1"); err == nil {
			ordem["fornecedor_cnpj_sugerido"] = f["cnpj"]
			ordem["fornecedor_nome_sugerido"] = f["razao_social"]
		}
	}
	return ordem, nil
}

func (m *Modulo) extraidaParaEditor(ctx context.Context, ordem map[string]any) (Extraida, error) {
	ex, err := m.extraidaAtual(ctx, ordem)
	if err != nil {
		return Extraida{}, err
	}
	if ex.FornecedorNome == "" {
		ex.FornecedorNome = strCampo(ordem["fornecedor_nome_sugerido"])
	}
	if ex.FornecedorCNPJ == "" {
		ex.FornecedorCNPJ = soDigitos(strCampo(ordem["fornecedor_cnpj_sugerido"]))
	}
	if len(ex.Itens) == 0 {
		id := strCampo(ordem["id"])
		var linhas []map[string]any
		_ = m.bd.Buscar(ctx, "ordens_compra_itens?ordem_compra_id=eq."+id+"&select=*", &linhas)
		for _, ln := range linhas {
			ex.Itens = append(ex.Itens, ItemExtraido{
				Descricao: strCampo(ln["descricao"]),
				Qtd:       numeroDeJSON(ln["qtd"]),
				Unidade:   strCampo(ln["unidade"]),
				ValorUnit: regras.DinheiroDe(numeroDeJSON(ln["valor_unit"])),
				Desconto:  regras.DinheiroDe(numeroDeJSON(ln["desconto"])),
				Total:     regras.DinheiroDe(numeroDeJSON(ln["total"])),
			})
		}
	}
	if ex.Numero == "" || ex.Numero == "<nil>" {
		ex.Numero = strCampo(ordem["numero"])
	}
	return ex, nil
}

func (m *Modulo) numeroLivre(ctx context.Context, clienteID, id, numero string) error {
	var iguais []map[string]any
	if err := m.bd.Buscar(ctx, "ordens_compra?cliente_id=eq."+banco.Escapar(clienteID)+
		"&numero=eq."+banco.Escapar(numero)+"&id=neq."+id+"&select=id&limit=1", &iguais); err != nil {
		return fmt.Errorf("não consegui conferir o número da OC")
	}
	if len(iguais) > 0 {
		return fmt.Errorf("Já existe uma ordem de compra com o número %s.", numero)
	}
	return nil
}

func (m *Modulo) guardarPDFOrdem(ctx context.Context, p *seguranca.Principal, pdf []byte) (string, error) {
	if len(pdf) == 0 {
		return "", fmt.Errorf("o PDF saiu vazio")
	}
	soma := sha256.Sum256(pdf)
	sha := hex.EncodeToString(soma[:])
	chave := armazem.Caminho(p.ClienteID, sha, "pdf")
	if err := m.arm.Enviar(ctx, chave, bytes.NewReader(pdf), int64(len(pdf)), sha, "application/pdf"); err != nil {
		return "", err
	}
	if err := m.bd.Upsert(ctx, "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": p.ClienteID,
		"tamanho":    len(pdf),
		"tipo":       "application/pdf",
		"chave_r2":   chave,
	}}, nil); err != nil {
		return "", err
	}
	return sha, nil
}

func apagarArquivoSeOrfao(ctx context.Context, bd *banco.Cliente, arm *armazem.Cliente, sha string) error {
	esc := banco.Escapar(sha)
	var refs []map[string]any
	if err := bd.Buscar(ctx, "ordens_compra?arquivo_sha256=eq."+esc+"&select=id&limit=1", &refs); err != nil {
		return err
	}
	if len(refs) > 0 {
		return nil
	}
	if err := bd.Buscar(ctx, "documentos?arquivo_sha256=eq."+esc+"&select=id&limit=1", &refs); err != nil {
		return err
	}
	if len(refs) > 0 {
		return nil
	}
	var arqs []map[string]any
	if err := bd.Buscar(ctx, "arquivos?sha256=eq."+esc+"&select=chave_r2&limit=1", &arqs); err != nil {
		return err
	}
	if len(arqs) == 0 {
		return nil
	}
	chave, _ := arqs[0]["chave_r2"].(string)
	if arm != nil && chave != "" {
		_ = arm.Apagar(ctx, chave)
	}
	return bd.Apagar(ctx, "arquivos", "sha256=eq."+esc)
}

func temPCOEnviado(v any) bool {
	if v == nil {
		return false
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s != "" && s != "<nil>"
}

func numeroDeJSON(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		return numeroBR(n)
	default:
		return 0
	}
}
