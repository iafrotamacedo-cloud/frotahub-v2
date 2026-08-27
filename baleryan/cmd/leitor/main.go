// rev 2 — a fila de leitura de notas
//
// O QUE MUDOU NA rev 2
//
//	A cascata de leitura saiu daqui e foi para `interno/leitura`, porque o motor
//	da web precisava dela: quem insere a nota está olhando para ela e quer o
//	orçamento agora, não daqui a meia hora. Pacote `main` não se importa — era
//	essa a única razão de a leitura ser exclusiva do robô.
//
//	Aqui ficou a FILA: tomar trabalho sem duas máquinas pegarem o mesmo,
//	recolher órfão, contar tentativa, desistir depois de três. A descrição da
//	cascata continua abaixo porque é ela que este programa executa — só que
//	agora pelo mesmo código que a tela usa (CORE-06).
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
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitura"
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
	servico := leitura.NovoDaConfig(cfg, bd, arm)
	ia := servico.IA()
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

	l := &Leitor{bd: bd, leitura: servico, quem: quemSou()}
	l.recolherOrfaos(ctx)

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
	bd      *banco.Cliente
	leitura *leitura.Servico
	quem    string
}

type Trabalho struct {
	ID         string `json:"id"`
	ClienteID  string `json:"cliente_id"`
	Alvo       string `json:"alvo_id"`
	Tentativas int    `json:"tentativas"`
}

// TempoDeOrfandade é quanto um trabalho pode ficar "rodando" antes de a gente
// concluir que ninguém está rodando ele.
//
// Trinta minutos é o teto do próprio workflow: passando disso, a máquina que
// tomou o trabalho não existe mais — o GitHub a desligou. Um pouco de margem
// para o relógio das duas pontas não bater, e chega.
const TempoDeOrfandade = 40 * time.Minute

// recolherOrfaos devolve para a fila o trabalho que ficou na mão de uma máquina
// que morreu.
//
// O BURACO QUE ISTO TAPA
//
//	`tomarTrabalho` marca o trabalho como "rodando" e só o fecha no fim. Se a
//	corrida morre no meio — o Actions cai, o tempo estoura, alguém cancela — o
//	trabalho fica "rodando" para sempre. Nenhuma corrida futura o pega, porque
//	todas procuram por `na_fila`.
//
//	Aconteceu em 26/08/2026, num dia em que o próprio GitHub teve incidente. O
//	documento não some, não dá erro e não aparece como falhado: ele simplesmente
//	nunca mais é lido. É a falha mais difícil de notar que existe — a que não
//	deixa rastro em lugar nenhum.
//
// POR QUE VOLTAR PARA A FILA E NÃO MARCAR COMO FALHADO
//
//	Porque não houve falha: houve interrupção. A nota está inteira, o arquivo
//	está no armazém, e a leitura nunca chegou a ser tentada até o fim. Marcar
//	"falhou" seria acusar a nota de um problema que foi da máquina.
//
//	A tentativa NÃO é devolvida de propósito: ela já foi gasta, e um trabalho
//	que morre a máquina toda vez precisa parar de ser tentado em algum momento —
//	senão uma nota que trava o processo vira laço infinito.
func (l *Leitor) recolherOrfaos(ctx context.Context) {
	limite := time.Now().UTC().Add(-TempoDeOrfandade).Format(time.RFC3339)

	var recolhidos []Trabalho
	err := l.bd.AtualizarDevolvendo(ctx, "jobs",
		"status=eq.rodando&tipo=eq.ler_documento&comecou_em=lt."+limite,
		map[string]any{
			"status":     "na_fila",
			"comecou_em": nil,
			"tomado_por": nil,
			"erro":       "a máquina que tinha este trabalho não terminou; devolvido para a fila",
		}, &recolhidos)
	if err != nil {
		// Não derruba a corrida: recolher órfão é limpeza, e a fila normal
		// continua funcionando sem ela.
		log.Printf("não consegui recolher trabalho órfão: %v", err)
		return
	}
	if len(recolhidos) > 0 {
		log.Printf("%d trabalho(s) tinham ficado presos em uma corrida que morreu — devolvidos para a fila",
			len(recolhidos))
	}
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

// ler entrega a nota ao serviço de leitura e cuida só do trabalho na fila.
//
// A CASCATA NÃO MORA MAIS AQUI
//
//	Baixar, interpretar, gravar e conferir duplicidade vivem em
//	`interno/leitura`, porque o motor da web precisa fazer exatamente o mesmo
//	quando alguém clica em "ler notas". O que sobrou aqui é o que só existe em
//	lote: fechar o trabalho, contar tentativa, decidir entre repetir e desistir.
//
// QUEM DECIDE ENTRE REPETIR E DESISTIR É O TIPO DO ERRO
//
//	`leitura.FalhaTemporaria` marca o que vale tentar de novo — rede, cota da
//	IA, um minuto ruim do armazém. O resto — "não achei o documento", "esta nota
//	não tem arquivo guardado" — é problema da nota, e a quarta tentativa recebe
//	o mesmo não que a primeira.
func (l *Leitor) ler(ctx context.Context, t *Trabalho) error {
	_, err := l.leitura.Ler(ctx, t.Alvo)
	if err != nil {
		var temporaria leitura.FalhaTemporaria
		if errors.As(err, &temporaria) {
			l.desistirOuRepetir(ctx, t, t.Alvo, err)
		} else {
			l.concluir(ctx, t.ID, err.Error())
		}
		return err
	}
	l.concluir(ctx, t.ID, "")
	return nil
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
	// O MOTIVO VAI PARA A TELA, ENTÃO VAI EM PORTUGUÊS
	//   Quem lê `leitura_erro` é quem precisa DECIDIR o que fazer com a nota, e
	//   não quem escreveu o programa. A frase crua do Google — "This model is no
	//   longer available to new users" — não diz nada a essa pessoa. O detalhe
	//   técnico continua lá, entre parênteses.
	_ = l.bd.Atualizar(ctx, "documentos", "id=eq."+documentoID, map[string]any{
		"status":       "falhou",
		"leitura_erro": leitor.EmPortugues(causa.Error()),
	})
}
