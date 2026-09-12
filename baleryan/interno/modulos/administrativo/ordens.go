// rev 2 — Compras: inserir, listar e ler Ordens de Compra
//
// TRÊS VISTAS, MESMO DESENHO DE `orcamentos.filtroDosDocumentos`
//
//	`fila` (padrão) — ainda não lida, ou sendo lida agora: `inserido`/`lendo`.
//	`processadas` — passou nos dois filtros de negócio: `status = lido`.
//	`rejeitadas` — falhou algum dos dois, ou o PDF não deu para ler: `falhou`.
//
//	Um lugar só decide a consulta por vista (`filtroDasOrdens`), como a
//	mesma ideia já provada em Orçamentos — CORE-06.
//
// POR QUE `nome_arquivo` (migração 060, e não a 059)
//
//	`numero` só existe depois de lida. Até lá, o único jeito de alguém
//	reconhecer "essa é a OC que acabei de subir" numa lista é o nome do
//	arquivo escolhido no PC — mesmo papel de `documentos.nome_arquivo` em
//	Orçamentos. A 059 tinha copiado o resto do padrão de `documentos` e
//	esquecido esta coluna; a 060 conserta antes que o primeiro insert precise
//	dela.
package administrativo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ValidadeDoLink é quanto tempo vale o endereço temporário do PDF — mesmo
// prazo de Orçamentos: para abrir agora, não para guardar.
const ValidadeDoLink = 5 * time.Minute

// ---------------------------------------------------------------------------
// GET /administrativo/compras/ordens — a fila
// ---------------------------------------------------------------------------

func (m *Modulo) listarOrdens(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := filtroDasOrdens(p.ClienteID, r.URL.Query().Get("vista")) +
		"&select=id,nome_arquivo,status,erro_leitura,numero,obra_centro_custo,comprador_nome,comprador_cnpj,fornecedor_id,total,criado_em" +
		"&limit=" + fmt.Sprint(TetoDaLista)
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar as ordens de compra", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ordens": ouVazio(linhas)})
}

// ---------------------------------------------------------------------------
// GET /administrativo/compras/ordens/painel — o hub de Compras
// ---------------------------------------------------------------------------

// painelDeOrdens alimenta os cartões do painel de Compras: "Inserir OC" (o
// que está na fila) e "OCs Inseridas", que se abre em Processadas/
// Rejeitadas — uma consulta por contador, com `count=exact` (Content-Range),
// mais uma prévia pequena de cada uma das duas vistas já lidas. Mesma ideia
// de `orcamentos.painel`: "o painel é UMA pergunta ao banco por cartão, não
// uma leitura da lista inteira".
func (m *Modulo) painelDeOrdens(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	fila, err := m.bd.BuscarContando(r.Context(),
		filtroDasOrdens(p.ClienteID, "fila")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as ordens na fila", err)
		return
	}
	processadas, err := m.bd.BuscarContando(r.Context(),
		filtroDasOrdens(p.ClienteID, "processadas")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as ordens processadas", err)
		return
	}
	rejeitadas, err := m.bd.BuscarContando(r.Context(),
		filtroDasOrdens(p.ClienteID, "rejeitadas")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as ordens rejeitadas", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"fila":        fila,
		"processadas": processadas,
		"rejeitadas":  rejeitadas,
		"previa": map[string]any{
			"processadas": m.previaDasOrdens(r.Context(), p.ClienteID, "processadas"),
			"rejeitadas":  m.previaDasOrdens(r.Context(), p.ClienteID, "rejeitadas"),
		},
	})
}

// ---------------------------------------------------------------------------
// GET /administrativo/compras/pco/painel — o hub de PCO
// ---------------------------------------------------------------------------

// painelDoPCO alimenta os dois cartões de PCO: "Pendentes de envio" e
// "Enviados" — a mesma ideia de `painelDeOrdens`, uma pergunta por contador.
func (m *Modulo) painelDoPCO(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	pendentes, err := m.bd.BuscarContando(r.Context(),
		filtroDasOrdens(p.ClienteID, "pco-pendentes")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as OCs pendentes de envio", err)
		return
	}
	enviados, err := m.bd.BuscarContando(r.Context(),
		filtroDasOrdens(p.ClienteID, "pco-enviados")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as OCs enviadas", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"pendentes": pendentes,
		"enviados":  enviados,
		"previa": map[string]any{
			"pendentes": m.previaDasOrdens(r.Context(), p.ClienteID, "pco-pendentes"),
			"enviados":  m.previaDasOrdens(r.Context(), p.ClienteID, "pco-enviados"),
		},
	})
}

// previaDasOrdens busca só o suficiente para as últimas linhas do cartão —
// falhar aqui não derruba o painel (os contadores já responderam): o cartão
// fica sem prévia, não sem número.
func (m *Modulo) previaDasOrdens(ctx context.Context, clienteID, vista string) []map[string]any {
	var linhas []map[string]any
	caminho := filtroDasOrdens(clienteID, vista) + "&select=nome_arquivo,numero,erro_leitura,criado_em&limit=6"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return []map[string]any{}
	}
	return ouVazio(linhas)
}

// filtroDasOrdens decide a consulta a partir da vista pedida — o mesmo
// desenho de "um lugar só decide a vista" que `orcamentos.filtroDosDocumentos`
// já usa (CORE-06).
//
// "pco-pendentes"/"pco-enviados" NÃO SÃO UMA SEGUNDA CÓPIA DA OC
//
//	É a mesma linha de `ordens_compra`, filtrada por `pco_enviado_em` — a
//	coluna que a migração 059 já reservou para isto ("nasce vazia, a rotina
//	agendada preenche depois"). Uma OC processada nasce em "pendente de
//	envio" e, quando o envio existir (fase futura, ainda não construída),
//	passa para "enviada" só por `pco_enviado_em` deixar de ser nulo — sem
//	sair de "Processadas" em Compras, que continua mostrando todas, para
//	sempre (é a planilha de controle do comprador).
func filtroDasOrdens(clienteID, vista string) string {
	base := "ordens_compra?cliente_id=eq." + banco.Escapar(clienteID)
	switch vista {
	case "processadas":
		return base + "&status=eq.lido&order=criado_em.desc"
	case "rejeitadas":
		return base + "&status=eq.falhou&order=criado_em.desc"
	case "pco-pendentes":
		return base + "&status=eq.lido&pco_enviado_em=is.null&order=criado_em.desc"
	case "pco-enviados":
		return base + "&status=eq.lido&pco_enviado_em=not.is.null&order=pco_enviado_em.desc"
	default:
		return base + "&status=in.(inserido,lendo)&order=criado_em.desc"
	}
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/ordens — inserir OCs
// ---------------------------------------------------------------------------

type resultadoDaInsercao struct {
	Nome   string `json:"nome"`
	ID     string `json:"id,omitempty"`
	Erro   string `json:"erro,omitempty"`
	Igual  bool   `json:"ja_existia,omitempty"`
	IgualA string `json:"ja_existia_como,omitempty"`
}

// inserirOrdens recebe os PDFs da barra de inserção.
//
// CADA ARQUIVO É INDEPENDENTE — mesma regra de Orçamentos: um erro no meio do
// lote não derruba os outros.
func (m *Modulo) inserirOrdens(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado. Sem ele, inserir uma OC seria perdê-la.")
		return
	}
	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler os arquivos enviados.")
		return
	}
	arquivos := r.MultipartForm.File["arquivos"]
	if len(arquivos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escolha pelo menos um arquivo.")
		return
	}

	saida := make([]resultadoDaInsercao, 0, len(arquivos))
	for _, cabecalho := range arquivos {
		res := resultadoDaInsercao{Nome: cabecalho.Filename}
		id, igualA, err := m.guardarUma(r.Context(), p, cabecalho)
		switch {
		case err != nil:
			res.Erro = err.Error()
		default:
			res.ID = id
			res.Igual = igualA != ""
			res.IgualA = igualA
		}
		saida = append(saida, res)
	}
	web.Responder(w, http.StatusOK, map[string]any{"arquivos": saida})
}

// ouOutroNome devolve o nome de quem já estava, ou o do próprio arquivo
// quando a linha antiga não tem nome (mesma função de Orçamentos).
func ouOutroNome(antigo, meu string) string {
	if a := strings.TrimSpace(antigo); a != "" && a != "<nil>" {
		return a
	}
	return meu
}

// guardarUma põe um PDF no armazém e cria a linha da OC — só o arquivo. Todo
// o resto (número, fornecedor, itens, totais) fica nulo até a leitura
// existir (ver cabeçalho do arquivo).
func (m *Modulo) guardarUma(ctx context.Context, p *seguranca.Principal,
	cabecalho *multipart.FileHeader) (string, string, error) {
	f, err := cabecalho.Open()
	if err != nil {
		return "", "", fmt.Errorf("não consegui abrir: %w", err)
	}
	defer f.Close()

	conteudo, err := io.ReadAll(io.LimitReader(f, TamanhoMaximo+1))
	if err != nil {
		return "", "", fmt.Errorf("não consegui ler: %w", err)
	}
	if len(conteudo) == 0 {
		return "", "", fmt.Errorf("o arquivo está vazio")
	}
	if len(conteudo) > TamanhoMaximo {
		return "", "", fmt.Errorf("passa de %d MB", TamanhoMaximo>>20)
	}

	soma := sha256.Sum256(conteudo)
	sha := hex.EncodeToString(soma[:])
	nome := cabecalho.Filename
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(nome)), ".")
	tipoMIME := tipoDoNome(nome)

	// O ARQUIVO NUNCA É APAGADO, ENTÃO ELE PODE JÁ ESTAR LÁ (mesma regra de
	// Orçamentos): a chave no R2 é o sha256 do conteúdo, e subir o mesmo PDF
	// duas vezes grava o mesmo lugar com os mesmos bytes.
	chave := armazem.Caminho(p.ClienteID, sha, ext)
	if err := m.arm.Enviar(ctx, chave, bytes.NewReader(conteudo), int64(len(conteudo)), sha, tipoMIME); err != nil {
		return "", "", fmt.Errorf("não consegui guardar no armazém: %w", err)
	}

	if err := m.bd.Upsert(ctx, "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": p.ClienteID,
		"tamanho":    len(conteudo),
		"tipo":       tipoMIME,
		"chave_r2":   chave,
	}}, nil); err != nil {
		return "", "", fmt.Errorf("guardei o arquivo mas não consegui registrá-lo: %w", err)
	}

	// A MESMA OC JÁ ESTÁ NA FILA?
	//   Critério é o conteúdo (sha256), igual a Orçamentos: dois PDFs iguais
	//   byte a byte são a mesma OC mandada duas vezes, não duas OCs.
	var iguais []map[string]any
	_ = m.bd.Buscar(ctx, "ordens_compra?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&arquivo_sha256=eq."+sha+"&select=id,nome_arquivo&limit=1", &iguais)
	if len(iguais) > 0 {
		return fmt.Sprint(iguais[0]["id"]),
			ouOutroNome(fmt.Sprint(iguais[0]["nome_arquivo"]), nome), nil
	}

	linha := map[string]any{
		"cliente_id":     p.ClienteID,
		"nome_arquivo":   nome,
		"arquivo_sha256": sha,
		"criado_por":     p.UserID,
		"status":         "inserido",
	}
	var criados []map[string]any
	if err := m.bd.Inserir(ctx, "ordens_compra", []map[string]any{linha}, &criados); err != nil {
		return "", "", fmt.Errorf("não consegui registrar a ordem de compra: %w", err)
	}
	if len(criados) == 0 {
		return "", "", fmt.Errorf("registrei a OC mas o banco não devolveu o id")
	}
	id := fmt.Sprint(criados[0]["id"])

	_ = m.hist.Registrar(ctx, p, "administrativo", id, "inserir_ordem_compra", nil)
	return id, "", nil
}

func tipoDoNome(nome string) string {
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
// GET /administrativo/compras/ordens/{id} e .../arquivo
// ---------------------------------------------------------------------------

func (m *Modulo) verOrdem(w http.ResponseWriter, r *http.Request) {
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
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=*&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	var itens []map[string]any
	_ = m.bd.Buscar(r.Context(), "ordens_compra_itens?ordem_compra_id=eq."+id+"&select=*", &itens)

	web.Responder(w, http.StatusOK, map[string]any{
		"ordem": ordem,
		"itens": ouVazio(itens),
	})
}

// arquivoDaOrdem devolve um endereço temporário para abrir o PDF — o arquivo
// não passa pelo motor, quem serve é a Cloudflare direto (mesma razão de
// Orçamentos).
func (m *Modulo) arquivoDaOrdem(w http.ResponseWriter, r *http.Request) {
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
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=nome_arquivo,arquivo_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	sha, _ := ordem["arquivo_sha256"].(string)
	if sha == "" {
		web.Falhar(w, http.StatusNotFound, "Esta ordem de compra não tem arquivo guardado.")
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
		"url":     link,
		"nome":    ordem["nome_arquivo"],
		"valeAte": time.Now().Add(ValidadeDoLink).UTC().Format(time.RFC3339),
	})
}
