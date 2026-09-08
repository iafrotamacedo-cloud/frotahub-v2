// rev 1 — o envio, a aprovação e a conformidade
//
// O ARQUIVO É SÓ MAIS UM ARQUIVO
//
//	O upload aqui é o MESMO desenho de `orcamentos/documentos.go`: lê o corpo
//	inteiro, calcula o sha256, grava no armazém pela chave derivada do
//	conteúdo, registra em `arquivos`. Nenhuma peça nova nasceu para isto — é
//	reaproveitar `armazem.Enviar`, que já existe e já é testado (CORE-06).
//
// REENVIAR SUBSTITUI, NÃO ACUMULA
//
//	`funcionario_documentos` tem UNIQUE (funcionario_id, tipo_documento_id):
//	só existe um "ASO Periódico" vivo por funcionário. Enviar de novo troca o
//	sha256 e volta o status para `enviado` — quem aprovou o antigo precisa
//	aprovar o novo, porque um documento reenviado é, por definição, ainda não
//	conferido.
package funcionarios

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

var (
	erroDocumentoNaoEncontrado = fmt.Errorf("Documento não encontrado.")
)

// ---------------------------------------------------------------------------
// listar os documentos de um funcionário
// ---------------------------------------------------------------------------

// GET /funcionarios/{id}/documentos
func (m *Modulo) listarDocumentos(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDocumentos)
	if p == nil {
		return
	}
	funcID := r.PathValue("id")
	if _, err := m.funcionarioDoCliente(r.Context(), funcID, p.ClienteID); err != nil {
		responderErroFuncionario(w, err)
		return
	}

	caminho := "funcionario_documentos?funcionario_id=eq." + banco.Escapar(funcID) +
		"&select=id,tipo_documento_id,status,data_emissao,data_validade,motivo_reprovacao,aprovado_em," +
		"tipos_documento(nome,categoria,tem_validade,dias_alerta)" +
		"&order=criado_em.desc"

	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os documentos.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"documentos": linhas})
}

// ---------------------------------------------------------------------------
// enviar um documento
// ---------------------------------------------------------------------------

// POST /funcionarios/{id}/documentos
// multipart: campo "arquivos" (um arquivo), "tipo_documento_id", opcionais
// "data_emissao" e "data_validade" (AAAA-MM-DD).
func (m *Modulo) enviarDocumento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDocumentos)
	if p == nil {
		return
	}
	funcID := r.PathValue("id")
	if _, err := m.funcionarioDoCliente(r.Context(), funcID, p.ClienteID); err != nil {
		responderErroFuncionario(w, err)
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado. Sem ele, enviar um documento seria perdê-lo.")
		return
	}

	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo enviado.")
		return
	}
	arquivos := r.MultipartForm.File["arquivos"]
	if len(arquivos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escolha o arquivo do documento.")
		return
	}
	tipoDocumentoID := strings.TrimSpace(r.FormValue("tipo_documento_id"))
	if tipoDocumentoID == "" {
		web.Falhar(w, http.StatusBadRequest, "Diga de qual documento se trata.")
		return
	}
	dataEmissao := nuloSeVazio(r.FormValue("data_emissao"))
	dataValidade := nuloSeVazio(r.FormValue("data_validade"))

	cabecalho := arquivos[0]
	f, err := cabecalho.Open()
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui abrir o arquivo enviado.")
		return
	}
	defer f.Close()

	conteudo, err := io.ReadAll(io.LimitReader(f, TamanhoMaximo+1))
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo enviado.")
		return
	}
	if len(conteudo) == 0 {
		web.Falhar(w, http.StatusBadRequest, "O arquivo está vazio.")
		return
	}
	if len(conteudo) > TamanhoMaximo {
		web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("O arquivo passa de %d MB.", TamanhoMaximo>>20))
		return
	}

	soma := sha256.Sum256(conteudo)
	sha := hex.EncodeToString(soma[:])
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(cabecalho.Filename)), ".")
	tipoMIME := tipoDoArquivo(cabecalho.Filename)

	chave := armazem.Caminho(p.ClienteID, sha, ext)
	if err := m.arm.Enviar(r.Context(), chave, bytes.NewReader(conteudo), int64(len(conteudo)), sha, tipoMIME); err != nil {
		web.Falhar(w, http.StatusBadGateway, "Não consegui guardar o arquivo no armazém.")
		return
	}
	if err := m.bd.Upsert(r.Context(), "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": p.ClienteID,
		"tamanho":    len(conteudo),
		"tipo":       tipoMIME,
		"chave_r2":   chave,
	}}, nil); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Guardei o arquivo mas não consegui registrá-lo.")
		return
	}

	// Upsert por (funcionario_id, tipo_documento_id): reenviar substitui a
	// linha, e volta para "enviado" — quem aprovou o antigo precisa aprovar
	// o novo.
	doc := map[string]any{
		"funcionario_id":    funcID,
		"tipo_documento_id": tipoDocumentoID,
		"arquivo_sha256":    sha,
		"status":            "enviado",
		"data_emissao":      dataEmissao,
		"data_validade":     dataValidade,
		"aprovado_por":      nil,
		"aprovado_em":       nil,
		"motivo_reprovacao": nil,
		"criado_por":        p.UserID,
	}
	var criados []struct {
		ID string `json:"id"`
	}
	if err := m.bd.Upsert(r.Context(), "funcionario_documentos?on_conflict=funcionario_id,tipo_documento_id",
		[]map[string]any{doc}, &criados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Guardei o arquivo mas não consegui registrar o documento.")
		return
	}

	resposta := map[string]any{"ok": true, "sha256": sha}
	if len(criados) > 0 {
		resposta["id"] = criados[0].ID
		err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, criados[0].ID, "enviou_documento", map[string]historico.Mudanca{
			"tipo_documento_id": {De: nil, Para: tipoDocumentoID},
		})
		if err != nil {
			resposta["aviso"] = historico.Aviso
		}
	}
	web.Responder(w, http.StatusOK, resposta)
}

func nuloSeVazio(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

func tipoDoArquivo(nome string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(nome), ".")) {
	case "pdf":
		return "application/pdf"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

// ---------------------------------------------------------------------------
// aprovar / reprovar
// ---------------------------------------------------------------------------

type documentoAtual struct {
	ID              string `json:"id"`
	FuncionarioID   string `json:"funcionario_id"`
	TipoDocumentoID string `json:"tipo_documento_id"`
	Status          string `json:"status"`
}

func (m *Modulo) documentoDoCliente(ctx context.Context, id, clienteID string) (*documentoAtual, error) {
	var linhas []documentoAtual
	caminho := "funcionario_documentos?id=eq." + banco.Escapar(id) +
		"&select=id,funcionario_id,tipo_documento_id,status,funcionarios!inner(cliente_id)" +
		"&funcionarios.cliente_id=eq." + banco.Escapar(clienteID) + "&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, fmt.Errorf("Não consegui carregar este documento.")
	}
	if len(linhas) == 0 {
		return nil, erroDocumentoNaoEncontrado
	}
	return &linhas[0], nil
}

func responderErroDocumento(w http.ResponseWriter, err error) {
	if err == erroDocumentoNaoEncontrado {
		web.Falhar(w, http.StatusNotFound, err.Error())
		return
	}
	web.Falhar(w, http.StatusInternalServerError, err.Error())
}

// POST /documentos/{id}/aprovar
func (m *Modulo) aprovar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaAprovar)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	atual, err := m.documentoDoCliente(r.Context(), id, p.ClienteID)
	if err != nil {
		responderErroDocumento(w, err)
		return
	}
	if atual.Status == "aprovado" {
		web.Falhar(w, http.StatusBadRequest, "Este documento já está aprovado.")
		return
	}

	campos := map[string]any{
		"status":            "aprovado",
		"aprovado_por":      p.UserID,
		"aprovado_em":       time.Now().UTC().Format(time.RFC3339),
		"motivo_reprovacao": nil,
	}
	if err := m.bd.Atualizar(r.Context(), "funcionario_documentos", "id=eq."+banco.Escapar(id), campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui aprovar o documento.")
		return
	}

	resposta := map[string]any{"ok": true}
	if err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, atual.FuncionarioID, "aprovou_documento", map[string]historico.Mudanca{
		"status": {De: atual.Status, Para: "aprovado"},
	}); err != nil {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusOK, resposta)
}

// POST /documentos/{id}/reprovar  {"motivo": "..."}
func (m *Modulo) reprovar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaAprovar)
	if p == nil {
		return
	}
	id := r.PathValue("id")

	var pedido struct {
		Motivo string `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	pedido.Motivo = strings.TrimSpace(pedido.Motivo)
	if pedido.Motivo == "" {
		web.Falhar(w, http.StatusBadRequest, "Diga o motivo da reprovação — quem enviou precisa saber o que corrigir.")
		return
	}

	atual, err := m.documentoDoCliente(r.Context(), id, p.ClienteID)
	if err != nil {
		responderErroDocumento(w, err)
		return
	}

	campos := map[string]any{
		"status":            "reprovado",
		"motivo_reprovacao": pedido.Motivo,
		"aprovado_por":      nil,
		"aprovado_em":       nil,
	}
	if err := m.bd.Atualizar(r.Context(), "funcionario_documentos", "id=eq."+banco.Escapar(id), campos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui reprovar o documento.")
		return
	}

	resposta := map[string]any{"ok": true}
	if err := m.hist.Registrar(semCancelar(r), p, moduloHistorico, atual.FuncionarioID, "reprovou_documento", map[string]historico.Mudanca{
		"status": {De: atual.Status, Para: "reprovado"},
		"motivo": {De: nil, Para: pedido.Motivo},
	}); err != nil {
		resposta["aviso"] = historico.Aviso
	}
	web.Responder(w, http.StatusOK, resposta)
}

// ---------------------------------------------------------------------------
// conformidade — o retrato de quem está em dia e quem não está
//
// UMA ROTA SÓ, MESMO DESENHO DE `estatisticas` E `consolidacao`
//
//	A tela precisa cruzar três coisas — funcionário, o que a função dele exige,
//	e o que já foi aprovado — e fazer isso em três consultas ao banco é mais
//	barato e mais simples do que inventar uma view para um cruzamento que só
//	esta tela usa.
// ---------------------------------------------------------------------------

type funcionarioAtivo struct {
	ID           string `json:"id"`
	NomeCompleto string `json:"nome_completo"`
	FuncaoID     string `json:"funcao_id"`
}

type requisito struct {
	FuncaoID        string `json:"funcao_id"`
	TipoDocumentoID string `json:"tipo_documento_id"`
	TiposDocumento  *struct {
		Nome string `json:"nome"`
	} `json:"tipos_documento"`
}

type documentoResumo struct {
	FuncionarioID   string  `json:"funcionario_id"`
	TipoDocumentoID string  `json:"tipo_documento_id"`
	Status          string  `json:"status"`
	DataValidade    *string `json:"data_validade"`
}

type pendenciaFuncionario struct {
	FuncionarioID string   `json:"funcionario_id"`
	NomeCompleto  string   `json:"nome_completo"`
	Faltando      []string `json:"faltando"`
	Vencidos      []string `json:"vencidos"`
}

// GET /funcionarios/conformidade
func (m *Modulo) conformidade(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDados)
	if p == nil {
		return
	}

	var funcionarios []funcionarioAtivo
	caminho := "funcionarios?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&status=eq.ativo&select=id,nome_completo,funcao_id"
	if err := m.bd.Buscar(r.Context(), caminho, &funcionarios); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os funcionários.")
		return
	}

	var requisitos []requisito
	if err := m.bd.Buscar(r.Context(),
		"funcao_documento_requisitos?obrigatorio=eq.true&select=funcao_id,tipo_documento_id,tipos_documento(nome)",
		&requisitos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os requisitos das funções.")
		return
	}
	porFuncao := map[string][]requisito{}
	for _, req := range requisitos {
		porFuncao[req.FuncaoID] = append(porFuncao[req.FuncaoID], req)
	}

	ids := make([]string, 0, len(funcionarios))
	for _, f := range funcionarios {
		ids = append(ids, f.ID)
	}

	docsPorFuncionario := map[string]map[string]documentoResumo{}
	if len(ids) > 0 {
		var docs []documentoResumo
		caminho := "funcionario_documentos?funcionario_id=in.(" + strings.Join(ids, ",") +
			")&select=funcionario_id,tipo_documento_id,status,data_validade"
		if err := m.bd.Buscar(r.Context(), caminho, &docs); err == nil {
			for _, d := range docs {
				if docsPorFuncionario[d.FuncionarioID] == nil {
					docsPorFuncionario[d.FuncionarioID] = map[string]documentoResumo{}
				}
				docsPorFuncionario[d.FuncionarioID][d.TipoDocumentoID] = d
			}
		}
	}

	hoje := time.Now().Format("2006-01-02")
	var comPendencia []pendenciaFuncionario
	conformes := 0

	for _, f := range funcionarios {
		var faltando, vencidos []string
		for _, req := range porFuncao[f.FuncaoID] {
			nomeTipo := ""
			if req.TiposDocumento != nil {
				nomeTipo = req.TiposDocumento.Nome
			}
			doc, existe := docsPorFuncionario[f.ID][req.TipoDocumentoID]
			if !existe || doc.Status != "aprovado" {
				faltando = append(faltando, nomeTipo)
				continue
			}
			if doc.DataValidade != nil && *doc.DataValidade != "" && *doc.DataValidade < hoje {
				vencidos = append(vencidos, nomeTipo)
			}
		}
		if len(faltando) == 0 && len(vencidos) == 0 {
			conformes++
		} else {
			comPendencia = append(comPendencia, pendenciaFuncionario{
				FuncionarioID: f.ID, NomeCompleto: f.NomeCompleto, Faltando: faltando, Vencidos: vencidos,
			})
		}
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"total_funcionarios":         len(funcionarios),
		"conformes":                  conformes,
		"com_pendencia":              len(comPendencia),
		"funcionarios_com_pendencia": comPendencia,
	})
}

// GET /funcionarios/vencendo?dias=30
func (m *Modulo) vencendo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaDados)
	if p == nil {
		return
	}
	dias := 30
	if v := r.URL.Query().Get("dias"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dias = n
		}
	}
	limite := time.Now().AddDate(0, 0, dias).Format("2006-01-02")

	caminho := "funcionario_documentos?status=eq.aprovado&data_validade=not.is.null&data_validade=lte." + limite +
		"&select=id,status,data_validade,funcionario_id,funcionarios!inner(nome_completo,cliente_id),tipos_documento(nome)" +
		"&funcionarios.cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&order=data_validade.asc"

	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os documentos vencendo.")
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"vencendo": linhas, "dias": dias})
}
