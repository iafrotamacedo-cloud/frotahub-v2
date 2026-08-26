// rev 1 — o leitor de notas, fora do motor
//
// POR QUE NÃO NO MOTOR
//
//	Porque OCR não é Go puro. O Tesseract é um programa, e converter PDF em
//	imagem é outro. Instalar os dois dentro do motor derrubaria a regra de zero
//	dependência e engordaria o contêiner que roda num plano que adormece.
//
//	No GitHub Actions há tempo, disco e liberdade para instalar o que for. É o
//	mesmo desenho do robô do Trílogo: o trabalho pesado roda lá, o motor só lê o
//	resultado. E é o mesmo código de leitura dos dois lados (CORE-06) — muda
//	quem chama.
//
// A CASCATA, NA ORDEM — rev 2, depois de medir 35 documentos reais
//
//  1. XML                    exato, de graça. Acaba a conversa.
//  2. PDF com camada de texto  `pdftotext`. Também exato e de graça. Um em cada
//     treze PDFs do contrato é digital; ler o texto dele
//     por OCR seria estragar de propósito o que já vem
//     pronto.
//  3. imagem -> OTIMIZADOR -> OCR
//  4. texto -> regex         DAV do SysPDV ou DANFE, conforme a família
//  5. A CONTA FECHA?         se a soma dos itens bate com o total, ACABOU: a
//     leitura se provou sozinha e a IA não é chamada.
//  6. só se não fechar -> IA
//
// POR QUE O OTIMIZADOR MUDA TUDO
//
//	As notas deste contrato não são fotos de papel: são CAPTURAS DE TELA do
//	sistema do fornecedor. O documento ocupa uns 530x330 de uma tela de
//	1440x900, então a letra tem 6 a 8 pixels — metade do que o Tesseract precisa.
//	Medido nos mesmos 35 documentos, só ampliando e binarizando:
//
//	                     antes      depois
//	  valor total        10/35      31/35
//	  nº do documento    11/35      31/35
//	  ticket             12/35      29/35
//
// POR QUE A TRAVA É ARITMÉTICA, E NÃO A CONFIANÇA DO OCR
//
//	Porque o Tesseract não sabe quando errou: ele devolve 19960 no lugar de
//	18860 com a mesma cara de quem acertou. Já a soma dos itens contra o total
//	é prova, não placar. Ela também é o que economiza a cota da IA — 500
//	leituras por dia no plano atual.
//
// SE A IA NÃO ESTIVER CONFIGURADA, os passos 1 a 5 continuam valendo.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
)

// Quantas voltas sem achar trabalho antes de encerrar. O Actions cobra por
// minuto; ficar acordado esperando fila vazia é dinheiro no lixo.
const voltasVazias = 2

// TetoDeTentativas — uma nota que falhou três vezes não vai passar na quarta.
// Ela para de rodar e aparece na tela como "falhou", com o motivo.
const TetoDeTentativas = 3

func main() {
	log.SetFlags(log.Ltime)

	cfg, err := config.Carregar()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	bd := banco.Novo(cfg)
	arm := armazem.Novo(cfg)
	ia := leitor.NovaIA(cfg.IA.Chave)
	if cfg.IA.Modelo != "" {
		ia.Modelo = cfg.IA.Modelo
	}

	if err := conferirFerramentas(); err != nil {
		// Dizer o que falta ANTES de tomar o primeiro trabalho. Tomar e falhar
		// deixaria a nota marcada como "rodando" sem ninguém rodando.
		log.Fatalf("faltam ferramentas nesta máquina: %v", err)
	}
	if !ia.Ligada() {
		log.Print("aviso: sem GEMINI_API_KEY — as notas entram sem itens, para conferência manual")
	}

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	l := &Leitor{bd: bd, arm: arm, ia: ia, quem: quemSou()}
	vazias, lidas, falhas := 0, 0, 0

	for volta := 1; volta <= 2000 && ctx.Err() == nil; volta++ {
		job, err := l.tomarTrabalho(ctx)
		if err != nil {
			log.Fatalf("parei: %v", err)
		}
		if job == nil {
			vazias++
			if vazias >= voltasVazias {
				break
			}
			time.Sleep(2 * time.Second)
			continue
		}
		vazias = 0
		if err := l.ler(ctx, job); err != nil {
			falhas++
			log.Printf("documento %s: %v", job.Alvo, err)
			continue
		}
		lidas++
	}

	log.Printf("=== FIM === %d lidas · %d falhas", lidas, falhas)
}

func quemSou() string {
	if s := os.Getenv("GITHUB_RUN_ID"); s != "" {
		return "actions-" + s
	}
	nome, _ := os.Hostname()
	return nome
}

// conferirFerramentas falha na largada, com a lista completa (P-09).
func conferirFerramentas() error {
	faltam := []string{}
	for _, f := range []string{"pdftoppm", "pdftotext", "tesseract", "python3"} {
		if _, err := exec.LookPath(f); err != nil {
			faltam = append(faltam, f)
		}
	}
	if len(faltam) > 0 {
		return fmt.Errorf("%s (no Actions: apt-get install -y poppler-utils tesseract-ocr tesseract-ocr-por · pip install opencv-python-headless)",
			strings.Join(faltam, ", "))
	}
	return nil
}

// ---------------------------------------------------------------------------

type Leitor struct {
	bd   *banco.Cliente
	arm  *armazem.Cliente
	ia   *leitor.IA
	quem string
}

type Trabalho struct {
	ID         string `json:"id"`
	ClienteID  string `json:"cliente_id"`
	Alvo       string `json:"alvo_id"`
	Tentativas int    `json:"tentativas"`
}

// tomarTrabalho pega uma linha da fila SEM correr o risco de duas máquinas
// pegarem a mesma.
//
// COMO A CORRIDA É EVITADA
//
//	A alteração é condicional: `status=eq.na_fila` vai no FILTRO. Duas máquinas
//	que leem a mesma linha disputam essa alteração, e só uma recebe a linha de
//	volta — a outra recebe zero e vai buscar a próxima. Sem a condição no
//	filtro, as duas trabalhariam a mesma nota e o resultado dependeria de quem
//	gravasse por último.
//
// ISTO JÁ FOI UM UPSERT, E POR ISSO O ROBÔ NUNCA PEGOU TRABALHO
//
//	O comentário acima sempre descreveu uma alteração condicional; o código
//	fazia um upsert. O PostgREST monta o upsert como `insert ... on conflict do
//	update`, e o insert é avaliado antes de o conflito ser resolvido — sem
//	`cliente_id` e `tipo`, que são obrigatórios na fila, o banco recusava a
//	linha por violação de not-null. O erro caía no `continue`, as cinco linhas
//	da fila eram descartadas em silêncio, e a rodada terminava com
//	"0 lidas · 0 falhas" — a cara de fila vazia.
//
//	Medido em 26/08/2026: 7 documentos na fila, 7 trabalhos `na_fila`, e o robô
//	dizendo que não havia nada para fazer.
func (l *Leitor) tomarTrabalho(ctx context.Context) (*Trabalho, error) {
	var fila []Trabalho
	if err := l.bd.Buscar(ctx, "jobs?status=eq.na_fila&tipo=eq.ler_documento"+
		"&order=criado_em&limit=5&select=id,cliente_id,alvo_id,tentativas", &fila); err != nil {
		return nil, err
	}
	for _, t := range fila {
		var tomados []Trabalho
		err := l.bd.AtualizarDevolvendo(ctx, "jobs",
			"id=eq."+t.ID+"&status=eq.na_fila",
			map[string]any{
				"status":     "rodando",
				"comecou_em": time.Now().UTC().Format(time.RFC3339),
				"tomado_por": l.quem,
				"tentativas": t.Tentativas + 1,
			}, &tomados)
		if err != nil {
			// Erro aqui não é "alguém pegou primeiro" — é problema de verdade,
			// e engoli-lo foi exatamente o que escondeu o defeito do upsert.
			log.Printf("não consegui tomar o trabalho %s: %v", t.ID, err)
			continue
		}
		if len(tomados) == 0 {
			continue // outra máquina pegou esta linha; segue para a próxima
		}
		trabalho := t
		trabalho.Tentativas++
		return &trabalho, nil
	}
	return nil, nil
}

func (l *Leitor) concluir(ctx context.Context, id string, erro string) {
	campos := map[string]any{
		"status":      "concluido",
		"terminou_em": time.Now().UTC().Format(time.RFC3339),
		"erro":        nil,
	}
	if erro != "" {
		campos["status"] = "falhou"
		campos["erro"] = erro
	}
	if err := l.bd.Atualizar(ctx, "jobs", "id=eq."+id, campos); err != nil {
		log.Printf("não consegui fechar o trabalho %s: %v", id, err)
	}
}

// ler é a cascata inteira para um documento.
func (l *Leitor) ler(ctx context.Context, t *Trabalho) error {
	doc, err := l.documento(ctx, t.Alvo)
	if err != nil {
		l.concluir(ctx, t.ID, err.Error())
		return err
	}

	bruto, err := l.baixar(ctx, doc.SHA)
	if err != nil {
		l.desistirOuRepetir(ctx, t, doc.ID, err)
		return err
	}

	var lida *leitor.Leitura

	if strings.EqualFold(filepath.Ext(doc.Nome), ".xml") {
		lida, err = leitor.DoXML(bruto)
		if err != nil {
			l.desistirOuRepetir(ctx, t, doc.ID, err)
			return err
		}
	} else {
		texto, via, err := extrair(ctx, doc.Nome, bruto)
		if err != nil {
			l.desistirOuRepetir(ctx, t, doc.ID, err)
			return err
		}
		if strings.TrimSpace(texto) == "" {
			err := fmt.Errorf("não consegui tirar texto nenhum deste arquivo (%s)", via)
			l.desistirOuRepetir(ctx, t, doc.ID, err)
			return err
		}

		// A FAMÍLIA DECIDE O LEITOR
		//   São dois documentos diferentes com nomes parecidos: a DANFE tem
		//   chave de acesso e "valor total da nota"; o DAV do SysPDV não tem
		//   chave nenhuma e chama o total de "total a pagar". Um regex só para
		//   os dois não lê nem um.
		doRegex := leitor.DoDAV(texto)
		if doRegex == nil {
			doRegex = leitor.DoTexto(texto)
		}
		lida = doRegex
		if via == viaTextoDoPDF {
			// Não passou por OCR: o texto é o do arquivo.
			lida.Camada = leitor.DoTextoCru
		}

		// A CONTA FECHOU: A LEITURA SE PROVOU SOZINHA
		//   Chamar a IA aqui seria gastar uma das 500 leituras diárias para
		//   confirmar o que a aritmética já confirmou.
		if leitor.ContaFecha(lida) {
			log.Printf("documento %s · %s · a conta fecha (%d itens somam %.2f) — sem IA",
				doc.Nome, via, len(lida.Itens), lida.ValorTotal)
		} else if l.ia.Ligada() {
			// A IA pode falhar (cota, rede, JSON torto) sem derrubar a leitura:
			// o que o regex achou continua valendo. Meia leitura registrada é
			// melhor que nenhuma.
			if daIA, err := l.ia.Ler(ctx, texto); err != nil {
				log.Printf("documento %s: a IA falhou (%v) — fica só o que o regex achou", doc.ID, err)
			} else {
				lida = leitor.Melhor(doRegex, daIA)
			}
		}
	}

	if err := l.gravar(ctx, doc, lida); err != nil {
		l.desistirOuRepetir(ctx, t, doc.ID, err)
		return err
	}
	l.concluir(ctx, t.ID, "")
	log.Printf("documento %s · %s · camada %s · confiança %.0f%% · %d itens",
		doc.Nome, ouTraco(lida.Numero), lida.Camada, lida.Confianca*100, len(lida.Itens))
	return nil
}

func ouTraco(s string) string {
	if s == "" {
		return "sem número"
	}
	return "nº " + s
}

// desistirOuRepetir decide se a nota volta para a fila ou para de vez.
func (l *Leitor) desistirOuRepetir(ctx context.Context, t *Trabalho, documentoID string, causa error) {
	if t.Tentativas < TetoDeTentativas {
		_ = l.bd.Atualizar(ctx, "jobs", "id=eq."+t.ID, map[string]any{
			"status": "na_fila",
			"erro":   causa.Error(),
		})
		return
	}
	l.concluir(ctx, t.ID, causa.Error())
	// A NOTA PRECISA APARECER NA TELA COMO FALHADA
	//   Ficar como "inserido" para sempre a esconderia: ninguém procura o que
	//   parece estar em ordem. Marcada como falhou, com o motivo, ela entra na
	//   lista de quem precisa de conferência manual.
	_ = l.bd.Atualizar(ctx, "documentos", "id=eq."+documentoID, map[string]any{
		"status":       "falhou",
		"leitura_erro": causa.Error(),
	})
}

type documento struct {
	ID   string `json:"id"`
	Nome string `json:"nome_arquivo"`
	SHA  string `json:"arquivo_sha256"`
}

func (l *Leitor) documento(ctx context.Context, id string) (*documento, error) {
	var d []documento
	if err := l.bd.Buscar(ctx, "documentos?id=eq."+id+
		"&select=id,nome_arquivo,arquivo_sha256&limit=1", &d); err != nil {
		return nil, err
	}
	if len(d) == 0 {
		return nil, fmt.Errorf("não achei o documento %s", id)
	}
	if d[0].SHA == "" {
		return nil, fmt.Errorf("o documento %s não tem arquivo guardado", id)
	}
	return &d[0], nil
}

// baixar traz o arquivo do armazém pelo endereço assinado.
func (l *Leitor) baixar(ctx context.Context, sha string) ([]byte, error) {
	var arq []struct {
		Chave string `json:"chave_r2"`
	}
	if err := l.bd.Buscar(ctx, "arquivos?sha256=eq."+sha+"&select=chave_r2&limit=1", &arq); err != nil {
		return nil, err
	}
	if len(arq) == 0 {
		return nil, fmt.Errorf("o arquivo %s não está registrado", sha[:8])
	}
	url, err := l.arm.LinkTemporario(arq[0].Chave, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("o armazém respondeu %d ao baixar o arquivo", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func (l *Leitor) gravar(ctx context.Context, doc *documento, lida *leitor.Leitura) error {
	bruto, _ := json.Marshal(lida)
	campos := map[string]any{
		"status":            "lido",
		"tipo":              vazioVira(lida.Tipo, "nf"),
		"numero":            nuloSeVazio(lida.Numero),
		"serie":             nuloSeVazio(lida.Serie),
		"chave_acesso":      nuloSeVazio(lida.ChaveAcesso),
		"dav_numero":        nuloSeVazio(lida.DAV),
		"emitente_cnpj":     nuloSeVazio(lida.EmitenteCNPJ),
		"emitente_nome":     nuloSeVazio(lida.EmitenteNome),
		"destinatario_cnpj": nuloSeVazio(lida.DestinatarioCNPJ),
		"emissao":           nuloSeVazio(lida.Emissao),
		"valor_total":       lida.ValorTotal,
		"valor_frete":       lida.ValorFrete,
		"observacao":        nuloSeVazio(lida.Observacao),
		"leitura_camada":    string(lida.Camada),
		"leitura_confianca": lida.Confianca,
		"leitura_bruta":     json.RawMessage(bruto),
		"leitura_erro":      nil,
	}
	if err := l.bd.Atualizar(ctx, "documentos", "id=eq."+doc.ID, campos); err != nil {
		return err
	}

	if len(lida.Itens) > 0 {
		linhas := make([]map[string]any, 0, len(lida.Itens))
		for i, it := range lida.Itens {
			linhas = append(linhas, map[string]any{
				"documento_id":   doc.ID,
				"ordem":          i + 1,
				"codigo":         nuloSeVazio(it.Codigo),
				"descricao":      it.Descricao,
				"unidade":        nuloSeVazio(it.Unidade),
				"quantidade":     it.Quantidade,
				"valor_unitario": it.Unitario,
				"valor_total":    it.Total,
			})
		}
		if err := l.bd.Upsert(ctx, "documento_itens?on_conflict=documento_id,ordem", linhas, nil); err != nil {
			return err
		}
	}

	return l.amarrarOTicket(ctx, doc, lida)
}

// amarrarOTicket liga a nota ao chamado — ou deixa de propósito sem ticket.
//
// TRÊS CONDIÇÕES, E TODAS PRECISAM VALER
//
//  1. o número saiu do CAMPO de observação, não de um lugar qualquer da página
//  2. o campo tem UM número de ticket, não dois nem nenhum
//  3. esse número existe na nossa base de chamados
//
// Falhando qualquer uma, a nota entra SEM ticket e vai para a fila de quem
// digita o número na mão. É trabalho — e é muito mais barato que o contrário:
// um ticket errado não trava nada, ele gera, lança e cobra a loja errada, e
// ninguém descobre.
//
// Medido em 35 documentos reais: o campo de observação foi lido em 29 deles, e
// os 29 tickets estavam CERTOS. Nenhum ticket errado. Os 6 que ficaram de fora
// caem na fila manual, que é exatamente onde deveriam cair.
func (l *Leitor) amarrarOTicket(ctx context.Context, doc *documento, lida *leitor.Leitura) error {
	ticket, confiavel := leitor.TicketConfiavel(lida)
	if !confiavel {
		log.Printf("documento %s · ticket não confiável (campo=%v, observação=%q) — vai para SEM TICKET",
			doc.Nome, lida.ObservacaoDoCampo, encurtar(lida.Observacao, 60))
		return nil
	}
	return l.amarrarTickets(ctx, doc.ID, []int{ticket})
}

func (l *Leitor) amarrarTickets(ctx context.Context, documentoID string, tickets []int) error {
	numeros := make([]string, 0, len(tickets))
	for _, t := range tickets {
		numeros = append(numeros, fmt.Sprint(t))
	}
	var achados []struct {
		ID     string `json:"id"`
		Numero int    `json:"numero"`
	}
	if err := l.bd.Buscar(ctx, "chamados?numero=in.("+strings.Join(numeros, ",")+
		")&select=id,numero", &achados); err != nil {
		return err
	}
	naBase := map[int]string{}
	for _, a := range achados {
		naBase[a.Numero] = a.ID
	}

	linhas := make([]map[string]any, 0, len(tickets))
	for _, t := range tickets {
		linha := map[string]any{"documento_id": documentoID, "ticket": t, "chamado_id": nil}
		if id, tem := naBase[t]; tem {
			linha["chamado_id"] = id
		}
		linhas = append(linhas, linha)
	}
	return l.bd.Upsert(ctx, "documento_tickets?on_conflict=documento_id,ticket", linhas, nil)
}

func nuloSeVazio(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func vazioVira(s, padrao string) string {
	if s == "" {
		return padrao
	}
	return s
}

// ---------------------------------------------------------------------------
// OCR
// ---------------------------------------------------------------------------

// extrair transforma o arquivo em texto, pelo caminho mais barato que servir.
//
// A ORDEM É A DO CUSTO, E TAMBÉM A DA QUALIDADE
//
//	texto do PDF     exato e instantâneo. Quando existe, não há o que melhorar.
//	imagem + OCR     tudo o mais. E aqui a imagem passa antes pelo otimizador.
const (
	viaTextoDoPDF = "texto-do-pdf"
	viaOCR        = "ocr"
)

func extrair(ctx context.Context, nome string, bruto []byte) (string, string, error) {
	pasta, err := os.MkdirTemp("", "leitor")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(pasta)

	ext := strings.ToLower(filepath.Ext(nome))
	var imagens []string

	if ext == ".pdf" {
		entrada := filepath.Join(pasta, "nota.pdf")
		if err := os.WriteFile(entrada, bruto, 0o600); err != nil {
			return "", "", err
		}
		// PDF COM CAMADA DE TEXTO NÃO PRECISA DE OCR
		//   Medido: 1 em 13 PDFs do contrato é digital. Mandá-lo para o OCR
		//   seria trocar o texto exato do arquivo por um palpite sobre a
		//   imagem dele.
		if texto := textoDoPDF(ctx, entrada); len(strings.TrimSpace(texto)) >= minimoDeTextoDePDF {
			return texto, viaTextoDoPDF, nil
		}
		saida := filepath.Join(pasta, "pagina")
		cmd := exec.CommandContext(ctx, "pdftoppm", "-r", "300", "-png", entrada, saida)
		if fora, err := cmd.CombinedOutput(); err != nil {
			return "", "", fmt.Errorf("pdftoppm falhou: %s", strings.TrimSpace(string(fora)))
		}
		achadas, _ := filepath.Glob(saida + "*.png")
		sort.Strings(achadas)
		imagens = achadas
	} else {
		entrada := filepath.Join(pasta, "nota"+ext)
		if err := os.WriteFile(entrada, bruto, 0o600); err != nil {
			return "", "", err
		}
		imagens = []string{entrada}
	}

	if len(imagens) == 0 {
		return "", "", fmt.Errorf("não consegui transformar o arquivo em imagem")
	}

	var tudo strings.Builder
	for i, img := range imagens {
		pronta := otimizar(ctx, pasta, i, img)
		// `-l por` usa o dicionário em português: sem ele o Tesseract lê
		// "MANUTENÇÃO" como "MANUTENCAG" e a observação vira ruído.
		cmd := exec.CommandContext(ctx, "tesseract", pronta, "stdout", "-l", "por", "--psm", "6")
		fora, err := cmd.Output()
		if err != nil {
			return "", "", fmt.Errorf("tesseract falhou em %s: %w", filepath.Base(img), err)
		}
		tudo.Write(fora)
		tudo.WriteByte('\n')
	}
	return tudo.String(), viaOCR, nil
}

// minimoDeTextoDePDF separa o PDF digital do escaneado.
//
// Não é zero de propósito: PDF de imagem às vezes traz um punhado de
// caracteres de metadado ou uma marca d'água em texto, e tratá-lo como digital
// devolveria três palavras no lugar da nota inteira.
const minimoDeTextoDePDF = 400

func textoDoPDF(ctx context.Context, caminho string) string {
	fora, err := exec.CommandContext(ctx, "pdftotext", caminho, "-").Output()
	if err != nil {
		return ""
	}
	return string(fora)
}

// otimizar chama o preparador de imagem. Se ele falhar, seguimos com a imagem
// crua: leitura pior é melhor que nenhuma leitura.
func otimizar(ctx context.Context, pasta string, i int, img string) string {
	saida := filepath.Join(pasta, fmt.Sprintf("otim-%d.png", i))
	cmd := exec.CommandContext(ctx, "python3", scriptDoOtimizador(), img, saida)
	if fora, err := cmd.CombinedOutput(); err != nil {
		log.Printf("otimizador falhou em %s (%v) — sigo com a imagem crua: %s",
			filepath.Base(img), err, strings.TrimSpace(string(fora)))
		return img
	}
	if _, err := os.Stat(saida); err != nil {
		return img
	}
	return saida
}

// scriptDoOtimizador acha o programa ao lado do código, sem depender de onde o
// robô foi chamado.
func scriptDoOtimizador() string {
	if s := os.Getenv("OTIMIZADOR"); s != "" {
		return s
	}
	return filepath.Join("ferramentas", "otimizar_imagem.py")
}

func encurtar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
