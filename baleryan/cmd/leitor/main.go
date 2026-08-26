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
// A CASCATA, NA ORDEM — rev 3, depois da rodada de 26/08/2026
//
//  1. XML                     exato e de graça. Acaba a conversa.
//  2. PDF com camada de texto  `pdftotext` + regex. Também exato e de graça.
//     Se a soma dos itens fechar com o total, ACABOU.
//  3. tudo o mais -> IA        o ARQUIVO vai inteiro para o modelo: imagem e
//     PDF são entrada nativa dele.
//  4. a conta fecha?           agora é CONFERÊNCIA, não atalho: valida o que a
//     IA devolveu em vez de decidir se ela é chamada.
//
// POR QUE O OCR SAIU
//
//	Porque ele achatava a página. O Tesseract varre linha por linha e devolve
//	uma fita de texto onde a coluna "quantidade" virou vizinha da coluna "valor
//	unitário" — e a IA, recebendo só a fita, tinha que adivinhar onde uma
//	acabava e a outra começava.
//
//	`LEDS NF 19650`, de 26/08/2026, é a prova: PDF escaneado, o Tesseract
//	devolveu `LED SPOT SW EM QUAD SLEGI5A JO00R`, e a IA gravou um item com
//	quantidade zero e preço zero numa nota de R$ 112,60. O modelo leu certo um
//	texto que já estava errado.
//
//	Na mesma rodada, as duas notas cujo PDF era digital — texto de verdade, sem
//	OCR — saíram com 100% de confiança e a soma batendo no centavo. A diferença
//	nunca foi o modelo: era o que chegava até ele.
//
// O QUE ISSO CUSTA, DITO SEM ENFEITE
//
//	A trava aritmética deixa de POUPAR cota. Antes, o DAV que fechava a própria
//	conta nem chamava a IA — três das sete notas de 26/08 foram lidas de graça.
//	Agora toda nota que não seja XML nem PDF digital passa pelo modelo.
//
//	A trava continua existindo, e continua sendo a mesma prova: ela só mudou de
//	posto. Era porteiro, virou conferente.
//
// SEM IA, IMAGEM NÃO É LIDA
//
//	Com o OCR fora, uma foto sem a chave configurada não tem leitor nenhum. O
//	robô diz isso em voz alta e marca a nota, em vez de gravar uma leitura vazia
//	que parece nota sem item (CORE-23).
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
	if cfg.IA.IntervaloSegundos > 0 {
		ia.Intervalo = time.Duration(cfg.IA.IntervaloSegundos) * time.Second
	}
	if ia.Ligada() {
		log.Printf("IA: modelo %s · uma leitura a cada %s", ia.Modelo, ia.Intervalo)
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
	// SÓ O `pdftotext` CONTINUA SENDO PRECISO
	//   Saíram o Tesseract, o `pdftoppm` e o otimizador em Python junto com o
	//   OCR. Exigir ferramenta que o código não chama é fazer a corrida falhar
	//   por um motivo que não existe — e são dois minutos de `apt-get` por
	//   rodada, todas as rodadas.
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return fmt.Errorf("pdftotext (no Actions: apt-get install -y poppler-utils)")
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

	lida, err := l.interpretar(ctx, doc, bruto)
	if err != nil {
		l.desistirOuRepetir(ctx, t, doc.ID, err)
		return err
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

// interpretar escolhe quem lê o documento, e devolve a leitura.
func (l *Leitor) interpretar(ctx context.Context, doc *documento, bruto []byte) (*leitor.Leitura, error) {
	// CAMADA 1 — o XML é o documento fiscal, não uma interpretação dele.
	if strings.EqualFold(filepath.Ext(doc.Nome), ".xml") {
		return leitor.DoXML(bruto)
	}

	// CAMADA 2 — o PDF que já traz texto de verdade.
	//
	// Um em cada treze PDFs do contrato é digital. Ali o texto veio do arquivo,
	// não de um palpite sobre a imagem dele: se a soma dos itens fechar com o
	// total, a leitura se provou sozinha e não há o que a IA acrescente.
	var doTexto *leitor.Leitura
	if strings.EqualFold(filepath.Ext(doc.Nome), ".pdf") {
		if texto := textoDePDF(ctx, bruto); len(strings.TrimSpace(texto)) >= minimoDeTextoDePDF {
			if doTexto = leitor.DoDAV(texto); doTexto == nil {
				doTexto = leitor.DoTexto(texto)
			}
			doTexto.Camada = leitor.DoTextoCru
			if leitor.ContaFecha(doTexto) {
				log.Printf("documento %s · texto do próprio PDF · a conta fecha (%d itens somam %.2f) — sem IA",
					doc.Nome, len(doTexto.Itens), doTexto.ValorTotal)
				return doTexto, nil
			}
		}
	}

	// CAMADA 3 — o arquivo inteiro vai para o modelo.
	if !l.ia.Ligada() {
		if doTexto != nil {
			log.Printf("documento %s · sem GEMINI_API_KEY — fica só o que o regex achou no texto do PDF", doc.Nome)
			return doTexto, nil
		}
		// Nem XML, nem texto no PDF, nem IA. Não existe leitor para este
		// arquivo, e dizer isso é melhor que gravar uma nota vazia.
		return nil, fmt.Errorf("este arquivo só pode ser lido pela IA, e não há GEMINI_API_KEY configurada")
	}

	daIA, err := l.ia.LerArquivo(ctx, leitor.MimeDoNome(doc.Nome), bruto)
	if err != nil {
		if doTexto != nil {
			log.Printf("documento %s: a IA falhou (%v) — fica só o que o regex achou", doc.Nome, err)
			return doTexto, nil
		}
		return nil, err
	}

	// A TRAVA ARITMÉTICA MUDOU DE POSTO
	//   Antes ela decidia SE a IA seria chamada. Agora confere o que a IA
	//   devolveu — a mesma prova, no outro lado da porta. Item inventado
	//   continua não fechando com o total.
	melhor := leitor.Melhor(doTexto, daIA)
	if leitor.ContaFecha(melhor) {
		log.Printf("documento %s · a conta fecha (%d itens somam %.2f)",
			doc.Nome, len(melhor.Itens), melhor.ValorTotal)
	} else {
		log.Printf("documento %s · A CONTA NÃO FECHA: %d itens somam %.2f e a nota diz %.2f — confira",
			doc.Nome, len(melhor.Itens), leitor.SomaDosItens(melhor), melhor.ValorTotal)
	}
	return melhor, nil
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
	// Qual das duas entradas: `orcamento` ou `rateio`. Muda quem manda no
	// ticket — veja `amarraSozinho`.
	Fila string `json:"fila"`
}

func (l *Leitor) documento(ctx context.Context, id string) (*documento, error) {
	var d []documento
	if err := l.bd.Buscar(ctx, "documentos?id=eq."+id+
		"&select=id,nome_arquivo,arquivo_sha256,fila&limit=1", &d); err != nil {
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
	ticket, pode, porque := amarraSozinho(doc.Fila, lida)
	if !pode {
		log.Printf("documento %s · %s (campo=%v, observação=%q) — vai para SEM TICKET",
			doc.Nome, porque, lida.ObservacaoDoCampo, encurtar(lida.Observacao, 60))
		return nil
	}
	return l.amarrarTickets(ctx, doc.ID, []int{ticket})
}

// FilaRateio é a entrada em que o ticket NÃO sai da nota.
const FilaRateio = "rateio"

// amarraSozinho responde se o robô pode amarrar o ticket que leu.
//
// A FILA DE RATEIO É O CONTRÁRIO DA OUTRA
//
//	Na fila `orcamento`, a nota traz o próprio ticket na observação e ler esse
//	número é o serviço. Na fila `rateio`, a nota foi separada JUSTAMENTE porque
//	o material dela se divide entre vários chamados — quem dita os tickets é o
//	usuário, na tela.
//
//	O leitor não sabia disso: rodava a mesma regra nas duas. Uma nota de rateio
//	com um ticket escrito na observação seria amarrada sozinha, e aí o custo
//	inteiro cairia numa loja só — o rateio cancelado antes de alguém abrir a
//	tela para fazê-lo. E sem travar nada: gera, lança e cobra a loja errada, em
//	silêncio. É o mesmo modo de falha que a regra dos três critérios existe para
//	impedir, entrando por uma porta que ninguém tinha olhado.
//
//	Em 26/08/2026 as duas notas de rateio da fila escaparam por sorte: o campo
//	de observação não foi lido, porque a IA estava fora do ar. Sorte não é
//	regra.
//
// É função à parte, e não um `if` dentro do método, porque a decisão é o que
// importa aqui — e decisão que não dá para testar sem subir um banco não é
// testada (P-30).
func amarraSozinho(fila string, lida *leitor.Leitura) (int, bool, string) {
	if fila == FilaRateio {
		return 0, false, "é da fila de rateio, onde quem dita o ticket é o usuário"
	}
	ticket, confiavel := leitor.TicketConfiavel(lida)
	if !confiavel {
		return 0, false, "ticket não confiável"
	}
	return ticket, true, ""
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
//
// minimoDeTextoDePDF separa o PDF digital do escaneado.
//
// Não é zero de propósito: PDF de imagem às vezes traz um punhado de caracteres
// de metadado ou uma marca d'água em texto, e tratá-lo como digital devolveria
// três palavras no lugar da nota inteira.
const minimoDeTextoDePDF = 400

// textoDePDF devolve a camada de texto do arquivo, ou string vazia.
//
// Vazio aqui é resposta, não erro: PDF escaneado não TEM camada de texto, e é
// exatamente o caso normal. Quem chama decide o que fazer com o silêncio.
func textoDePDF(ctx context.Context, bruto []byte) string {
	pasta, err := os.MkdirTemp("", "leitor")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(pasta)

	caminho := filepath.Join(pasta, "nota.pdf")
	if err := os.WriteFile(caminho, bruto, 0o600); err != nil {
		return ""
	}
	fora, err := exec.CommandContext(ctx, "pdftotext", caminho, "-").Output()
	if err != nil {
		return ""
	}
	return string(fora)
}

func encurtar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
