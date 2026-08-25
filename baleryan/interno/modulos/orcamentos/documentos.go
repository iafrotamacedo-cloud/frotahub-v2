// rev 1 — as duas filas de entrada: Notas e DAVs, e Notas para rateio
//
// São a MESMA tabela com um campo de fila diferente, e não duas tabelas. A
// diferença entre elas é só de trabalho: na fila `orcamento` a nota traz o
// próprio ticket nas observações; na fila `rateio` quem dita os tickets é o
// usuário. O arquivo, a leitura, a duplicidade e a exclusão são idênticos —
// duplicar a tabela duplicaria as quatro regras.
package orcamentos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// GET /orcamentos/painel — os números dos cinco botões
// ---------------------------------------------------------------------------

// O painel é UMA consulta, não cinco.
//
// A tela desenha cinco contadores e quatro sub-contadores. Buscar cada um por
// sua conta seriam nove viagens ao banco para pintar uma tela que ainda nem sabe
// o que o usuário quer. A visão `orcamentos_painel` responde tudo de uma vez.
func (m *Modulo) painel(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaModulo)
	if p == nil {
		return
	}
	linha, err := m.contarUm(r.Context(),
		"orcamentos_painel?cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=*&limit=1")
	if err != nil {
		m.erro(w, "não consegui ler o painel", err)
		return
	}

	// A última inserção serve de "há quanto tempo" na barra de cima, do mesmo
	// jeito que a última leitura serve na tela do Trílogo.
	var ultimas []map[string]any
	_ = m.bd.Buscar(r.Context(),
		"documentos?cliente_id=eq."+banco.Escapar(p.ClienteID)+
			"&order=inserido_em.desc&limit=1&select=inserido_em", &ultimas)
	if len(ultimas) > 0 {
		linha["ultima_insercao"] = ultimas[0]["inserido_em"]
	}

	// AS PRÉVIAS DAS BARRAS
	//
	//	Uma barra alta e vazia é um botão bonito. O que a transforma em painel de
	//	trabalho são as últimas linhas da própria etapa: o usuário lê o serviço do
	//	dia antes do primeiro clique.
	//
	//	Poderiam vir de três chamadas do navegador. Vêm daqui porque a tela
	//	desenha as cinco barras de uma vez — três viagens a mais para pintar a
	//	mesma tela é o motor do plano gratuito acordando três vezes.
	linha["previa"] = map[string]any{
		"notas": m.previa(r.Context(), "documentos_lista?cliente_id=eq."+cli(p)+
			"&fila=eq.orcamento&oculto_em=is.null&order=inserido_em.desc&limit=9"+
			"&select=nome_arquivo,inserido_em,valor_total"),
		"rateio": m.previa(r.Context(), "documentos_lista?cliente_id=eq."+cli(p)+
			"&fila=eq.rateio&oculto_em=is.null&tickets=eq.0&order=inserido_em.desc&limit=9"+
			"&select=nome_arquivo,inserido_em,valor_total"),
		"lancar": m.previa(r.Context(), "orcamentos_lista?cliente_id=eq."+cli(p)+
			"&status=eq.gerado&order=criado_em&limit=9&select=ticket,loja,valor"),
	}

	web.Responder(w, http.StatusOK, linha)
}

func cli(p *seguranca.Principal) string { return banco.Escapar(p.ClienteID) }

// previa devolve as linhas cruas da prévia. Erro aqui NÃO derruba o painel: uma
// barra sem prévia continua útil; um painel que não abre, não.
func (m *Modulo) previa(ctx context.Context, caminho string) []map[string]any {
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		log.Printf("orcamentos: prévia falhou (%s): %v", caminho, err)
		return []map[string]any{}
	}
	return ouVazio(linhas)
}

// ---------------------------------------------------------------------------
// GET /orcamentos/documentos — a lista
// ---------------------------------------------------------------------------

func (m *Modulo) listarDocumentos(w http.ResponseWriter, r *http.Request) {
	fila := filaPedida(r)
	p := m.quem(w, r, rotinaDaFila(fila))
	if p == nil {
		return
	}

	q := r.URL.Query()
	pagina := umNumero(q.Get("pagina"), 1)
	por := umDosPermitidos(q.Get("por"))

	filtro := "documentos_lista?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&fila=eq." + banco.Escapar(fila)

	// A área Hide é uma LISTA, não um limbo. Quem apagou por engano precisa
	// conseguir ver o que apagou — senão "desfazer" só funciona nos dez segundos
	// do aviso, e depois disso o arquivo virou fantasma.
	if q.Get("ocultos") == "1" {
		filtro += "&oculto_em=not.is.null&order=oculto_em.desc"
	} else {
		filtro += "&oculto_em=is.null&order=inserido_em.desc"
	}
	if busca := strings.TrimSpace(q.Get("busca")); busca != "" {
		filtro += "&or=(nome_arquivo.ilike.*" + banco.Escapar(busca) +
			"*,numero.ilike.*" + banco.Escapar(busca) +
			"*,emitente_nome.ilike.*" + banco.Escapar(busca) + "*)"
	}

	var linhas []map[string]any
	total, err := m.bd.BuscarContando(r.Context(), filtro+"&select=*"+intervalo(pagina, por), &linhas)
	if err != nil {
		m.erro(w, "não consegui listar os arquivos", err)
		return
	}
	web.Responder(w, http.StatusOK, montarPagina(linhas, total, pagina, por))
}

func filaPedida(r *http.Request) string {
	if r.URL.Query().Get("fila") == "rateio" {
		return "rateio"
	}
	return "orcamento"
}

func rotinaDaFila(fila string) string {
	if fila == "rateio" {
		return RotinaRateio
	}
	return RotinaNotas
}

// ---------------------------------------------------------------------------
// POST /orcamentos/documentos — inserir arquivos
// ---------------------------------------------------------------------------

type resultadoDaInsercao struct {
	Nome  string `json:"nome"`
	ID    string `json:"id,omitempty"`
	Erro  string `json:"erro,omitempty"`
	Igual bool   `json:"ja_existia,omitempty"`
}

// inserirDocumentos recebe os arquivos da barra de inserção.
//
// CADA ARQUIVO É INDEPENDENTE
//
//	Subir cinco notas e ter a terceira falhando não pode derrubar as outras
//	quatro. Cada uma responde por si, e a tela mostra a lista do que entrou e do
//	que não entrou. No sistema antigo um erro no meio do lote abortava tudo e o
//	usuário nunca sabia quais tinham passado.
func (m *Modulo) inserirDocumentos(w http.ResponseWriter, r *http.Request) {
	fila := filaPedida(r)
	p := m.quem(w, r, rotinaDaFila(fila))
	if p == nil {
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado. Sem ele, inserir uma nota seria perdê-la.")
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
		id, jaExistia, err := m.guardarUm(r.Context(), p, fila, cabecalho.Filename, cabecalho)
		switch {
		case err != nil:
			res.Erro = err.Error()
		default:
			res.ID = id
			res.Igual = jaExistia
		}
		saida = append(saida, res)
	}
	web.Responder(w, http.StatusOK, map[string]any{"arquivos": saida})
}

// guardarUm põe um arquivo no armazém e cria a linha do documento.
func (m *Modulo) guardarUm(ctx context.Context, p *seguranca.Principal, fila, nome string,
	cabecalho *multipart.FileHeader) (string, bool, error) {
	f, err := cabecalho.Open()
	if err != nil {
		return "", false, fmt.Errorf("não consegui abrir: %w", err)
	}
	defer f.Close()

	conteudo, err := io.ReadAll(io.LimitReader(f, TamanhoMaximo+1))
	if err != nil {
		return "", false, fmt.Errorf("não consegui ler: %w", err)
	}
	if len(conteudo) == 0 {
		return "", false, fmt.Errorf("o arquivo está vazio")
	}
	if len(conteudo) > TamanhoMaximo {
		return "", false, fmt.Errorf("passa de %d MB", TamanhoMaximo>>20)
	}

	soma := sha256.Sum256(conteudo)
	sha := hex.EncodeToString(soma[:])
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(nome)), ".")
	tipoMIME := tipoDoNome(nome)

	// O ARQUIVO NUNCA É APAGADO, ENTÃO ELE PODE JÁ ESTAR LÁ
	//   A chave no R2 é o sha256 do conteúdo. Subir a mesma nota duas vezes
	//   grava o mesmo lugar com os mesmos bytes — não duplica nada, e o segundo
	//   envio é idempotente por construção (P-04).
	chave := armazem.Caminho(p.ClienteID, sha, ext)
	if err := m.arm.Enviar(ctx, chave, bytes.NewReader(conteudo), int64(len(conteudo)), sha, tipoMIME); err != nil {
		return "", false, fmt.Errorf("não consegui guardar no armazém: %w", err)
	}

	if err := m.bd.Upsert(ctx, "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": p.ClienteID,
		"tamanho":    len(conteudo),
		"tipo":       tipoMIME,
		"chave_r2":   chave,
	}}, nil); err != nil {
		return "", false, fmt.Errorf("guardei o arquivo mas não consegui registrá-lo: %w", err)
	}

	// A MESMA NOTA JÁ ESTÁ NA FILA?
	//   Aqui o critério é o conteúdo, não a chave de acesso — que ainda não foi
	//   lida. Duas cópias do mesmo PDF são a mesma nota, e avisar disso na hora
	//   é melhor do que descobrir na geração.
	var iguais []map[string]any
	_ = m.bd.Buscar(ctx, "documentos?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&arquivo_sha256=eq."+sha+"&oculto_em=is.null&select=id&limit=1", &iguais)
	if len(iguais) > 0 {
		return fmt.Sprint(iguais[0]["id"]), true, nil
	}

	linha := map[string]any{
		"cliente_id":     p.ClienteID,
		"fila":           fila,
		"nome_arquivo":   nome,
		"arquivo_sha256": sha,
		"inserido_por":   p.UserID,
		"status":         "inserido",
	}

	// CAMADA 1 NA HORA
	//   Se o arquivo é o XML da NFe, não há nada para enfileirar: a leitura é
	//   exata, custa microssegundos e não depende de OCR nem de IA. Mandar isso
	//   para a fila seria fazer o usuário esperar por um trabalho que já está
	//   pronto.
	var lida *leitor.Leitura
	if ext == "xml" {
		if l, err := leitor.DoXML(conteudo); err == nil {
			lida = l
		}
	}
	if lida != nil {
		aplicarLeitura(linha, lida)
	}

	var criados []map[string]any
	if err := m.bd.Inserir(ctx, "documentos", []map[string]any{linha}, &criados); err != nil {
		return "", false, fmt.Errorf("não consegui registrar o documento: %w", err)
	}
	if len(criados) == 0 {
		return "", false, fmt.Errorf("registrei o documento mas o banco não devolveu o id")
	}
	id := fmt.Sprint(criados[0]["id"])

	if lida != nil {
		if err := m.gravarItens(ctx, id, lida); err != nil {
			log.Printf("orcamentos: itens do documento %s: %v", id, err)
		}
		m.amarrarTicketsLidos(ctx, p, id, lida)
	} else {
		// Sem XML, o trabalho é do robô: OCR e IA rodam onde há tempo e disco.
		if err := m.bd.Inserir(ctx, "jobs", []map[string]any{{
			"cliente_id": p.ClienteID,
			"tipo":       "ler_documento",
			"alvo_id":    id,
		}}, nil); err != nil {
			log.Printf("orcamentos: não consegui enfileirar a leitura de %s: %v", id, err)
		}
	}

	_ = m.hist.Registrar(ctx, p, "orcamentos", id, "inserir_documento", nil)
	return id, false, nil
}

// aplicarLeitura despeja a leitura nos campos da linha do documento.
func aplicarLeitura(linha map[string]any, l *leitor.Leitura) {
	linha["status"] = "lido"
	linha["tipo"] = ouNulo(l.Tipo)
	linha["numero"] = ouNulo(l.Numero)
	linha["serie"] = ouNulo(l.Serie)
	linha["chave_acesso"] = ouNulo(l.ChaveAcesso)
	linha["dav_numero"] = ouNulo(l.DAV)
	linha["emitente_cnpj"] = ouNulo(l.EmitenteCNPJ)
	linha["emitente_nome"] = ouNulo(l.EmitenteNome)
	linha["destinatario_cnpj"] = ouNulo(l.DestinatarioCNPJ)
	linha["emissao"] = ouNulo(l.Emissao)
	linha["valor_total"] = l.ValorTotal
	linha["valor_frete"] = l.ValorFrete
	linha["observacao"] = ouNulo(l.Observacao)
	linha["leitura_camada"] = string(l.Camada)
	linha["leitura_confianca"] = l.Confianca
	if bruto, err := json.Marshal(l); err == nil {
		linha["leitura_bruta"] = json.RawMessage(bruto)
	}
}

// ouNulo troca string vazia por nulo. É o que impede a coluna de guardar "" e
// depois a consulta `is.null` mentir sobre o que está preenchido.
func ouNulo(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (m *Modulo) gravarItens(ctx context.Context, documentoID string, l *leitor.Leitura) error {
	if len(l.Itens) == 0 {
		return nil
	}
	linhas := make([]map[string]any, 0, len(l.Itens))
	for i, it := range l.Itens {
		linhas = append(linhas, map[string]any{
			"documento_id":   documentoID,
			"ordem":          i + 1,
			"codigo":         ouNulo(it.Codigo),
			"descricao":      it.Descricao,
			"unidade":        ouNulo(it.Unidade),
			"quantidade":     it.Quantidade,
			"valor_unitario": it.Unitario,
			"valor_total":    it.Total,
		})
	}
	return m.bd.Upsert(ctx, "documento_itens?on_conflict=documento_id,ordem", linhas, nil)
}

// amarrarTicketsLidos escreve os tickets que estavam nas observações.
//
// Já resolve a associação com a nossa base aqui: o ticket que existe em
// `chamados` nasce ligado, e o que não existe nasce solto — e é exatamente essa
// diferença que alimenta a frente "sem associação" da tela de Correções.
func (m *Modulo) amarrarTicketsLidos(ctx context.Context, p *seguranca.Principal, documentoID string, l *leitor.Leitura) {
	tickets := leitor.Tickets(l.Observacao)
	if len(tickets) == 0 {
		return
	}
	if err := m.amarrar(ctx, p, documentoID, tickets); err != nil {
		log.Printf("orcamentos: tickets de %s: %v", documentoID, err)
	}
}

func tipoDoNome(nome string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(nome), ".")) {
	case "pdf":
		return "application/pdf"
	case "xml":
		return "application/xml"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

// ---------------------------------------------------------------------------
// GET /orcamentos/documentos/{id} e .../arquivo
// ---------------------------------------------------------------------------

func (m *Modulo) verDocumento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaNotas)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	doc, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=*&limit=1")
	if err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}
	var itens []map[string]any
	_ = m.bd.Buscar(r.Context(), "documento_itens?documento_id=eq."+id+"&order=ordem&select=*", &itens)
	var tickets []map[string]any
	_ = m.bd.Buscar(r.Context(), "documento_tickets?documento_id=eq."+id+"&order=ticket&select=*", &tickets)

	web.Responder(w, http.StatusOK, map[string]any{
		"documento": doc,
		"itens":     ouVazio(itens),
		"tickets":   ouVazio(tickets),
	})
}

func ouVazio(l []map[string]any) []map[string]any {
	if l == nil {
		return []map[string]any{}
	}
	return l
}

// arquivoDoDocumento devolve um endereço temporário para abrir o arquivo.
//
// O ENDEREÇO É ASSINADO, E CURTO
//
//	O arquivo não passa pelo motor: quem serve é a Cloudflare, direto. O motor só
//	assina um endereço que vale cinco minutos. É mais rápido, não gasta a banda
//	do plano gratuito, e o endereço que vazar já expirou.
func (m *Modulo) arquivoDoDocumento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaNotas)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	doc, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=nome_arquivo,arquivo_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}
	sha, _ := doc["arquivo_sha256"].(string)
	if sha == "" {
		web.Falhar(w, http.StatusNotFound, "Este documento não tem arquivo guardado.")
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
		"nome":    doc["nome_arquivo"],
		"valeAte": time.Now().Add(ValidadeDoLink).UTC().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// excluir e desfazer
// ---------------------------------------------------------------------------

// ocultarDocumento é o "excluir" da tela.
//
// NÃO APAGA NADA
//
//	Preenche `oculto_em` e pronto. O arquivo continua inteiro no R2, o registro
//	continua na tabela, e o "desfazer" do aviso é um update. Foi assim que o
//	sistema antigo recuperou 15 orçamentos perdidos — só que lá a rede de
//	segurança era a lixeira do Dropbox, e o Dropbox saiu do stack. Aqui a rede
//	é esta coluna.
func (m *Modulo) ocultarDocumento(w http.ResponseWriter, r *http.Request) {
	m.marcarOculto(w, r, true)
}

func (m *Modulo) restaurarDocumento(w http.ResponseWriter, r *http.Request) {
	m.marcarOculto(w, r, false)
}

func (m *Modulo) marcarOculto(w http.ResponseWriter, r *http.Request, ocultar bool) {
	p := m.quem(w, r, RotinaNotas)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	campos := map[string]any{"oculto_em": nil, "oculto_por": nil}
	acao := "restaurar_documento"
	if ocultar {
		campos["oculto_em"] = time.Now().UTC().Format(time.RFC3339)
		campos["oculto_por"] = p.UserID
		acao = "ocultar_documento"
	}

	filtro := "id=eq." + id + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "documentos", filtro, campos); err != nil {
		m.erro(w, "não consegui atualizar o documento", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, acao, nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "oculto": ocultar})
}

// ---------------------------------------------------------------------------
// os tickets de uma nota
// ---------------------------------------------------------------------------

type pedidoDeTickets struct {
	Tickets []int `json:"tickets"`
}

// amarrarTickets é o "+" da tela de rateio, e também a correção manual.
func (m *Modulo) amarrarTickets(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaRateio)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var pedido pedidoDeTickets
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi a lista de tickets.")
		return
	}
	if len(pedido.Tickets) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Informe pelo menos um ticket.")
		return
	}
	// Dez por linha antes de quebrar, disse a tela. Aqui o limite é outro: mais
	// de cem tickets numa nota é erro de digitação, não rateio.
	if len(pedido.Tickets) > 100 {
		web.Falhar(w, http.StatusBadRequest, "São tickets demais para uma nota só.")
		return
	}
	if err := m.amarrar(r.Context(), p, id, pedido.Tickets); err != nil {
		m.erro(w, "não consegui amarrar os tickets", err)
		return
	}
	var tickets []map[string]any
	_ = m.bd.Buscar(r.Context(), "documento_tickets?documento_id=eq."+id+"&order=ticket&select=*", &tickets)
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "amarrar_tickets", nil)
	web.Responder(w, http.StatusOK, map[string]any{"tickets": ouVazio(tickets)})
}

// amarrar grava os tickets JÁ RESOLVENDO a associação com a nossa base.
//
// O ticket que existe em `chamados` nasce com `chamado_id` preenchido; o que não
// existe nasce nulo. É essa coluna nula que a tela de Correções lista como "sem
// associação" — e, como o dono explicou, 90% das vezes é o fornecedor que
// digitou errado, não a nossa base que está desatualizada.
func (m *Modulo) amarrar(ctx context.Context, p *seguranca.Principal, documentoID string, tickets []int) error {
	numeros := make([]string, 0, len(tickets))
	for _, t := range tickets {
		if t > 0 {
			numeros = append(numeros, strconv.Itoa(t))
		}
	}
	if len(numeros) == 0 {
		return nil
	}

	var achados []struct {
		ID     string `json:"id"`
		Numero int    `json:"numero"`
	}
	if err := m.bd.Buscar(ctx, "chamados?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&numero=in.("+strings.Join(numeros, ",")+")&select=id,numero", &achados); err != nil {
		return err
	}
	naNossaBase := map[int]string{}
	for _, a := range achados {
		naNossaBase[a.Numero] = a.ID
	}

	linhas := make([]map[string]any, 0, len(tickets))
	for _, t := range tickets {
		if t <= 0 {
			continue
		}
		l := map[string]any{
			"documento_id": documentoID,
			"ticket":       t,
			"incluido_por": p.UserID,
		}
		if id, tem := naNossaBase[t]; tem {
			l["chamado_id"] = id
		} else {
			l["chamado_id"] = nil
		}
		linhas = append(linhas, l)
	}
	return m.bd.Upsert(ctx, "documento_tickets?on_conflict=documento_id,ticket", linhas, nil)
}

func (m *Modulo) soltarTicket(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaRateio)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ticket := umNumero(r.PathValue("ticket"), 0)
	if ticket == 0 {
		web.Falhar(w, http.StatusBadRequest, "Ticket inválido.")
		return
	}
	// Confere o titular antes de apagar: sem isto, o filtro por documento_id
	// sozinho deixaria um cliente mexer no documento de outro.
	if _, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id&limit=1"); err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}
	if err := m.bd.Atualizar(r.Context(), "documento_tickets",
		"documento_id=eq."+id+"&ticket=eq."+strconv.Itoa(ticket),
		map[string]any{"chamado_id": nil}); err != nil {
		m.erro(w, "não consegui soltar o ticket", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------

// erro traduz uma falha em resposta, sem despejar detalhe de banco na tela.
func (m *Modulo) erro(w http.ResponseWriter, frase string, err error) {
	if err == ErrNaoAchei {
		web.Falhar(w, http.StatusNotFound, "Não achei este registro.")
		return
	}
	log.Printf("orcamentos: %s: %v", frase, err)
	web.Falhar(w, http.StatusInternalServerError,
		"Não consegui completar: "+frase+". Tente de novo em instantes.")
}
