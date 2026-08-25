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
// A CASCATA, NA ORDEM
//
//  1. o arquivo é XML?           camada 1 resolve sozinha, e nem chega aqui
//  2. PDF -> imagem -> texto     pdftoppm + tesseract
//  3. texto -> regex             pega chave de acesso, valor, data
//  4. texto -> IA                estrutura os itens
//  5. junta as duas leituras     cada camada acerta onde a outra erra
//
// SE A IA NÃO ESTIVER CONFIGURADA, os passos 1 a 3 continuam valendo. A nota
// entra com chave de acesso e valor, sem itens, e aparece na tela pedindo
// conferência — que é melhor do que não entrar.
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
	for _, f := range []string{"pdftoppm", "tesseract"} {
		if _, err := exec.LookPath(f); err != nil {
			faltam = append(faltam, f)
		}
	}
	if len(faltam) > 0 {
		return fmt.Errorf("%s (no Actions: apt-get install -y poppler-utils tesseract-ocr tesseract-ocr-por)",
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
//	O update é condicional: `status=eq.na_fila`. Duas máquinas que leem a mesma
//	linha disputam esse update, e só uma o consegue — a outra recebe zero linhas
//	afetadas e vai buscar a próxima. Sem a condição no WHERE, as duas
//	trabalhariam a mesma nota e o resultado dependeria de quem gravasse por
//	último.
func (l *Leitor) tomarTrabalho(ctx context.Context) (*Trabalho, error) {
	var fila []Trabalho
	if err := l.bd.Buscar(ctx, "jobs?status=eq.na_fila&tipo=eq.ler_documento"+
		"&order=criado_em&limit=5&select=id,cliente_id,alvo_id,tentativas", &fila); err != nil {
		return nil, err
	}
	for _, t := range fila {
		var tomados []map[string]any
		err := l.bd.Upsert(ctx, "jobs?on_conflict=id", []map[string]any{{
			"id":         t.ID,
			"status":     "rodando",
			"comecou_em": time.Now().UTC().Format(time.RFC3339),
			"tomado_por": l.quem,
			"tentativas": t.Tentativas + 1,
		}}, &tomados)
		if err != nil {
			continue
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
		texto, err := ocr(ctx, doc.Nome, bruto)
		if err != nil {
			l.desistirOuRepetir(ctx, t, doc.ID, err)
			return err
		}
		if strings.TrimSpace(texto) == "" {
			err := fmt.Errorf("o OCR não achou texto nenhum neste arquivo")
			l.desistirOuRepetir(ctx, t, doc.ID, err)
			return err
		}

		doRegex := leitor.DoTexto(texto)
		lida = doRegex

		if l.ia.Ligada() {
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

	// Os tickets das observações, já resolvidos contra a nossa base de chamados.
	tickets := leitor.Tickets(lida.Observacao)
	if len(tickets) == 0 {
		return nil
	}
	return l.amarrarTickets(ctx, doc.ID, tickets)
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

// ocr converte o arquivo em texto.
//
// PDF vira imagem antes, uma página por vez, a 300 dpi — que é a resolução em
// que as notas do São Luiz foram escaneadas. Subir mais não melhora a leitura e
// multiplica o tempo; descer perde os dígitos pequenos da chave de acesso.
func ocr(ctx context.Context, nome string, bruto []byte) (string, error) {
	pasta, err := os.MkdirTemp("", "leitor")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(pasta)

	ext := strings.ToLower(filepath.Ext(nome))
	var imagens []string

	if ext == ".pdf" {
		entrada := filepath.Join(pasta, "nota.pdf")
		if err := os.WriteFile(entrada, bruto, 0o600); err != nil {
			return "", err
		}
		saida := filepath.Join(pasta, "pagina")
		cmd := exec.CommandContext(ctx, "pdftoppm", "-r", "300", "-png", entrada, saida)
		if fora, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("pdftoppm falhou: %s", strings.TrimSpace(string(fora)))
		}
		achadas, _ := filepath.Glob(saida + "*.png")
		imagens = achadas
	} else {
		entrada := filepath.Join(pasta, "nota"+ext)
		if err := os.WriteFile(entrada, bruto, 0o600); err != nil {
			return "", err
		}
		imagens = []string{entrada}
	}

	if len(imagens) == 0 {
		return "", fmt.Errorf("não consegui transformar o arquivo em imagem")
	}

	var tudo strings.Builder
	for _, img := range imagens {
		// `-l por` usa o dicionário em português: sem ele o Tesseract lê
		// "MANUTENÇÃO" como "MANUTENCAG" e a observação vira ruído.
		cmd := exec.CommandContext(ctx, "tesseract", img, "stdout", "-l", "por", "--psm", "6")
		fora, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("tesseract falhou em %s: %w", filepath.Base(img), err)
		}
		tudo.Write(fora)
		tudo.WriteByte('\n')
	}
	return tudo.String(), nil
}
