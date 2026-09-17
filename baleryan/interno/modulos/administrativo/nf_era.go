// rev 2 — a leitura da nota fiscal no recebimento: sugestão rápida + ERA READ em fila
//
// DUAS LEITURAS, DOIS TEMPOS (15/09/2026)
//
//	O almoxarife está com o celular na mão, na obra. O que ele precisa em
//	segundos é o NÚMERO e o VALOR da nota para conferir e salvar. O que o
//	calibrador do ERA precisa é a leitura completa (texto com posição,
//	campos tipados) — e essa leitura custa de 30 s a minutos por página
//	(medido em 15/09/2026, ver o cabeçalho de eraleitura/motor.go).
//
//	Então são duas rotinas separadas de propósito:
//
//	  · POST /nf/ordens/{id}/escanear — a SUGESTÃO. Lê a primeira página
//	    com a IA (Gemini, camada 3 do `leitor`, a mesma que já lê nota
//	    para orçamento) e devolve número/valor em poucos segundos. Sem
//	    chave do Gemini, devolve vazio e o almoxarife digita — nunca é
//	    erro. Nada é gravado aqui.
//
//	  · A LEITURA ERA — entra numa fila em memória depois que a página já
//	    está salva no armazém e no banco. Um trabalhador só, uma página
//	    por vez, e o resultado (ou o erro) vai para `leitura_era` da
//	    própria página. Se o motor reiniciar no meio, a página fica com
//	    `leitura_era` nulo — o calibrador sabe reler o que falta; a nota
//	    em si nunca depende disso.
//
// POR QUE A LEITURA NÃO SEGURA MAIS O RECEBIMENTO
//
//	Na rev 1 o ERA rodava DENTRO de /receber: com placeholder nas
//	variáveis do Render, abrir o motor falhava e a nota era recusada com
//	422 — o almoxarife ficava sem conseguir receber nota nenhuma. Agora o
//	recebimento só depende do armazém e do banco, como qualquer outro
//	arquivo da casa.
package administrativo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/era-regen/integrar"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TempoDaSugestao é quanto a rota de escanear espera pela IA antes de
// devolver vazio. O celular está esperando com a tela aberta.
const TempoDaSugestao = 45 * time.Second

// TetoDaFilaERA é quantas páginas esperam leitura na memória do motor.
// Cada uma carrega o JPEG (1–3 MB); no plano gratuito do Render não cabe
// muito mais que isto. Passou do teto, a página fica com `leitura_era`
// nulo e o log avisa — a nota já está salva.
const TetoDaFilaERA = 12

// PaginasPorNota é o máximo de páginas num único recebimento.
const PaginasPorNota = 20

// ---------------------------------------------------------------------------
// POST /administrativo/nf/ordens/{id}/escanear — a sugestão de número e valor
// ---------------------------------------------------------------------------

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
	raw, err := m.bytesDoArquivo(r, "arquivo")
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}
	if !m.ia.Ligada() {
		web.Responder(w, http.StatusOK, map[string]any{
			"fonte":    "nenhuma",
			"era_read": m.era.Ligado(),
		})
		return
	}
	ctx, cancelar := context.WithTimeout(r.Context(), TempoDaSugestao)
	defer cancelar()
	l, err := m.ia.LerArquivo(ctx, tipoDaImagem(raw), raw)
	if err != nil {
		log.Printf("administrativo: sugestão de NF pela IA falhou: %v", err)
		web.Responder(w, http.StatusOK, map[string]any{
			"fonte":    "nenhuma",
			"era_read": m.era.Ligado(),
			"aviso":    "Não consegui ler a nota automaticamente — confira e preencha número e valor.",
		})
		return
	}
	numero := l.Numero
	if numero == "" && l.Tipo == "dav" {
		numero = l.DAV
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"fonte":    "ia",
		"tipo":     l.Tipo,
		"numero":   numero,
		"valor":    l.ValorTotal,
		"emitente": l.EmitenteNome,
		"era_read": m.era.Ligado(),
	})
}

// tipoDaImagem olha os primeiros bytes — o nome que vem do celular não é
// confiável (o scanner manda "nf-p1.jpg", uma foto de galeria pode vir PNG).
func tipoDaImagem(b []byte) string {
	if len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' {
		return "image/png"
	}
	if len(b) >= 4 && b[0] == '%' && b[1] == 'P' && b[2] == 'D' && b[3] == 'F' {
		return "application/pdf"
	}
	return "image/jpeg"
}

// ---------------------------------------------------------------------------
// POST /administrativo/nf/notas/{id}/paginas — acrescenta UMA página a uma NF
// já recebida (o recebimento normal manda todas de uma vez em /receber)
// ---------------------------------------------------------------------------

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
	if pagina > PaginasPorNota {
		web.Falhar(w, http.StatusConflict, fmt.Sprintf("Esta nota já tem %d páginas — é o máximo.", PaginasPorNota))
		return
	}
	sha, err := m.guardarArquivoNF(r.Context(), p, raw, fmt.Sprintf("nf-p%d.jpg", pagina))
	if err != nil {
		m.erro(w, "não consegui guardar esta página da nota", err)
		return
	}
	if err := m.bd.Inserir(r.Context(), "notas_fiscais_paginas", []map[string]any{{
		"cliente_id":     p.ClienteID,
		"nota_fiscal_id": nfID,
		"pagina":         pagina,
		"arquivo_sha256": sha,
	}}, nil); err != nil {
		m.erro(w, "não consegui gravar esta página da nota", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", nfID, "pagina_nota_fiscal", map[string]historico.Mudanca{
		"pagina": {De: nil, Para: pagina},
	})
	enfileirada := m.agendarLeituraERA(trabalhoERA{
		clienteID: p.ClienteID, userID: p.UserID, nfID: nfID, pagina: pagina, sha: sha, raw: raw,
	})
	web.Responder(w, http.StatusOK, map[string]any{
		"id":       nfID,
		"pagina":   pagina,
		"era_read": enfileirada,
	})
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/notas/{id}/paginas — todas as páginas, com endereço
// temporário para ver cada uma (a página 1 mora em notas_fiscais)
// ---------------------------------------------------------------------------

func (m *Modulo) paginasDaNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemComQualquerRotina(w, r, RotinaNFReceber, RotinaNFEntregar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	nf, err := m.contarUm(r.Context(), "notas_fiscais?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=numero,arquivo_sha256,leitura_era&limit=1")
	if err != nil {
		m.erro(w, "não achei esta nota fiscal", err)
		return
	}
	type pag struct {
		sha     string
		leitura any
	}
	paginas := []pag{{sha: strCampo(nf["arquivo_sha256"]), leitura: nf["leitura_era"]}}
	var mais []map[string]any
	if err := m.bd.Buscar(r.Context(), "notas_fiscais_paginas?nota_fiscal_id=eq."+id+
		"&select=pagina,arquivo_sha256,leitura_era&order=pagina", &mais); err != nil {
		m.erro(w, "não consegui listar as páginas desta nota", err)
		return
	}
	for _, l := range mais {
		paginas = append(paginas, pag{sha: strCampo(l["arquivo_sha256"]), leitura: l["leitura_era"]})
	}
	saida := make([]map[string]any, 0, len(paginas))
	for i, pg := range paginas {
		linha := map[string]any{"pagina": i + 1, "lida": pg.leitura != nil}
		if pg.sha != "" {
			if arq, err := m.contarUm(r.Context(), "arquivos?sha256=eq."+banco.Escapar(pg.sha)+"&select=chave_r2&limit=1"); err == nil {
				if chave, _ := arq["chave_r2"].(string); chave != "" {
					if link, err := m.arm.LinkTemporario(chave, ValidadeDoLink); err == nil {
						linha["url"] = link
					}
				}
			}
		}
		saida = append(saida, linha)
	}
	web.Responder(w, http.StatusOK, map[string]any{"numero": nf["numero"], "paginas": saida})
}

// ---------------------------------------------------------------------------
// a fila do ERA READ
// ---------------------------------------------------------------------------

type trabalhoERA struct {
	clienteID string
	userID    string
	nfID      string
	pagina    int
	sha       string
	raw       []byte
}

// agendarLeituraERA põe a página na fila. Devolve false (sem erro) quando
// o ERA está desligado ou a fila está cheia — nos dois casos a nota já
// está salva e nada mais depende disto.
func (m *Modulo) agendarLeituraERA(t trabalhoERA) bool {
	if m.era == nil || !m.era.Ligado() || m.filaERA == nil {
		return false
	}
	select {
	case m.filaERA <- t:
		return true
	default:
		log.Printf("administrativo: fila do ERA READ cheia (%d) — página %d da nota %s fica sem leitura por ora",
			TetoDaFilaERA, t.pagina, t.nfID)
		return false
	}
}

// trabalharERA é a goroutine única que lê as páginas, uma por vez, e grava
// `leitura_era`. Sobe em `Novo` e vive enquanto o motor viver.
func (m *Modulo) trabalharERA() {
	for t := range m.filaERA {
		m.lerUmaPaginaERA(t)
	}
}

func (m *Modulo) lerUmaPaginaERA(t trabalhoERA) {
	inicio := time.Now()
	meta := integrar.Lancamento{
		LeituraID:  novoIDLeitura(),
		NotaID:     t.nfID,
		ImagemURI:  "sha256:" + t.sha,
		LancadoPor: t.userID,
		LancadoEm:  inicio.UTC(),
	}
	var leitura map[string]any
	saida, err := m.era.Ler(t.raw, meta)
	if err != nil {
		log.Printf("administrativo: ERA READ falhou na página %d da nota %s (%v): %v",
			t.pagina, t.nfID, time.Since(inicio).Round(time.Second), err)
		leitura = map[string]any{"erro": err.Error()}
	} else {
		leitura = leituraParaMapa(saida.Leitura)
		if leitura == nil {
			leitura = map[string]any{}
		}
		sug := integrar.SugestaoDeSaida(saida)
		leitura["sugestao"] = map[string]any{"numero": sug.Numero, "valor": sug.Valor, "tipo": sug.Tipo}
		log.Printf("administrativo: ERA READ leu a página %d da nota %s em %v (%d linhas, tipo %s)",
			t.pagina, t.nfID, time.Since(inicio).Round(time.Second), len(saida.Scan.Lines), sug.Tipo)
	}
	leitura["lido_em"] = time.Now().UTC().Format(time.RFC3339)
	leitura["duracao_seg"] = int(time.Since(inicio).Seconds())

	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelar()
	var gravar error
	if t.pagina == 1 {
		gravar = m.bd.Atualizar(ctx, "notas_fiscais",
			"id=eq."+t.nfID+"&cliente_id=eq."+banco.Escapar(t.clienteID),
			map[string]any{"leitura_era": leitura})
	} else {
		gravar = m.bd.Atualizar(ctx, "notas_fiscais_paginas",
			"nota_fiscal_id=eq."+t.nfID+"&pagina=eq."+strconv.Itoa(t.pagina),
			map[string]any{"leitura_era": leitura})
	}
	if gravar != nil {
		log.Printf("administrativo: não consegui gravar a leitura ERA da página %d da nota %s: %v", t.pagina, t.nfID, gravar)
	}
}

// ---------------------------------------------------------------------------
// apoio
// ---------------------------------------------------------------------------

func (m *Modulo) ordemProntaParaReceber(r *http.Request, p *seguranca.Principal, ocID string) (map[string]any, error) {
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+ocID+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,status,numero,obra_centro_custo,pco_enviado_em,total,aguardando_correcao&limit=1")
	if err != nil {
		return nil, fmt.Errorf("não achei esta ordem de compra")
	}
	if ordem == nil {
		return nil, fmt.Errorf("não achei esta ordem de compra")
	}
	if fmtStatus(ordem["status"]) != "lido" || !temPCOEnviado(ordem["pco_enviado_em"]) {
		return nil, fmt.Errorf("esta ordem de compra ainda não foi enviada ao cliente")
	}
	// Migração 076 — enquanto está na fila de correção, ninguém recebe nota
	// pra esta OC: o RC precisa corrigi-la (ou voltá-la pra Aguardando NF)
	// primeiro, senão a nota nova entraria numa OC que já vai ser apagada.
	if b, _ := ordem["aguardando_correcao"].(bool); b {
		return nil, fmt.Errorf("esta ordem de compra está na fila de correção — fale com quem corrige OCs antes de receber mais notas")
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

// bytesDoArquivo lê o primeiro arquivo do campo `campo` do multipart.
func (m *Modulo) bytesDoArquivo(r *http.Request, campo string) ([]byte, error) {
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
			return nil, fmt.Errorf("não consegui ler o que foi enviado")
		}
	}
	cabs := r.MultipartForm.File[campo]
	if len(cabs) == 0 {
		return nil, fmt.Errorf("escaneie a nota fiscal")
	}
	return lerCabecalhoMultipart(cabs[0])
}

// lerCabecalhoMultipart lê um arquivo do multipart inteiro na memória,
// dentro do teto de tamanho. Erros viram frase pronta pra `web.Falhar`.
func lerCabecalhoMultipart(cab *multipart.FileHeader) ([]byte, error) {
	f, err := cab.Open()
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

func parseValorNF(s string) (float64, error) {
	s = strings.TrimSpace(s)
	// "1.234,56" (pt-BR) e "1234.56" (decimal) são as duas grafias que
	// chegam do celular; a primeira tem ponto de milhar que precisa sair.
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("informe o valor da nota fiscal")
	}
	return v, nil
}

// novaIA monta o leitor Gemini do módulo, com os mesmos ajustes que o
// módulo de Serviço usa (modelo e intervalo vêm do ambiente).
func novaIA(chave, modelo string, intervaloSeg int) *leitor.IA {
	ia := leitor.NovaIA(chave)
	if modelo != "" {
		ia.Modelo = modelo
	}
	if intervaloSeg > 0 {
		ia.Intervalo = time.Duration(intervaloSeg) * time.Second
	}
	return ia
}
