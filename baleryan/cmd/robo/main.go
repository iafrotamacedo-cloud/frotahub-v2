// rev 3 — o robô fora do motor
//
// Este programa existe para as duas passadas pesadas — levantamento e cópia —
// que rodam no GitHub Actions, no botão, e não no motor.
//
// POR QUE NÃO NO MOTOR
//
//	São trabalhos de minutos a horas. O motor roda num plano que adormece; um
//	trabalho longo lá seria interrompido no meio. No Actions há tempo, disco para
//	os vídeos e minutos livres — o repositório é público.
//
// É O MESMO CÓDIGO do robô que roda dentro do motor. Muda só quem chama (CORE-06).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
)

func main() {
	log.SetFlags(log.Ltime)

	modo := strings.TrimSpace(os.Getenv("MODO"))
	if modo == "" {
		log.Fatal("diga o MODO: levantamento, copia ou atualizacao")
	}
	slug := os.Getenv("CLIENTE_SLUG")
	if slug == "" {
		slug = "frota-macedo"
	}

	cfg, err := config.Carregar()
	if err != nil {
		log.Fatal(err)
	}
	bd := banco.Novo(cfg)
	svc := trilogo.Novo(cfg, bd, armazem.Novo(cfg))

	// Ctrl+C e o cancelamento do Actions param entre lotes, não no meio de um.
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	clienteID, err := clienteDoSlug(ctx, bd, slug)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("robô trílogo · modo %s · cliente %s", modo, slug)
	comeco := time.Now()
	var total trilogo.Resultado

	// Repete enquanto sobrar trabalho. Cada volta é uma rodada registrada no
	// banco.
	//
	// O TETO DE VOLTAS NÃO É UM GUARDA-CHUVA SUFICIENTE
	//
	//	Ele existia — 500 voltas — e mesmo assim o robô girou nove vezes lendo a
	//	base inteira sem sair do lugar, até o corte de tempo do Actions matar o
	//	processo. Teto alto só transforma laço infinito em laço demorado.
	//
	//	O guarda que faltava é este: se DUAS voltas seguidas não gravam nada, a
	//	próxima faria o mesmo trabalho pela terceira vez. Melhor parar e dizer.
	semProgresso := 0
	for volta := 1; volta <= 500; volta++ {
		r, err := svc.Rodar(ctx, modo, clienteID, "actions")
		if r != nil {
			total.ChamadosLidos += r.ChamadosLidos
			total.ChamadosGravados += r.ChamadosGravados
			total.EventosGravados += r.EventosGravados
			total.ArquivosVistos += r.ArquivosVistos
			total.BytesVistos += r.BytesVistos
			total.ArquivosSoLink += r.ArquivosSoLink
			total.BytesSoLink += r.BytesSoLink
			total.ArquivosCopiados += r.ArquivosCopiados
			total.BytesCopiados += r.BytesCopiados
			log.Printf("volta %d · %s", volta, resumo(r))
		}
		if err != nil {
			log.Fatalf("parei: %v", err)
		}
		if r == nil || r.Completo {
			break
		}
		if r.ChamadosGravados == 0 && r.ArquivosCopiados == 0 {
			semProgresso++
			if semProgresso >= 2 {
				log.Printf("parei na volta %d: duas voltas seguidas sem gravar nada, "+
					"e a rodada continua dizendo que falta trabalho. "+
					"A próxima faria o mesmo — isto é sintoma de cursor travado.", volta)
				break
			}
		} else {
			semProgresso = 0
		}
		if ctx.Err() != nil {
			log.Print("cancelado; paro entre lotes, sem deixar nada pela metade")
			break
		}
	}

	log.Printf("=== FIM em %s ===", time.Since(comeco).Round(time.Second))
	log.Print(resumo(&total))
	if total.BytesVistos > 0 {
		// Os dois números aparecem SEPARADOS de propósito: a diferença entre eles
		// é a decisão que o dono tomou, e ela precisa estar visível toda vez.
		log.Printf("catalogado: %s no total", tamanhoLegivel(total.BytesVistos))
		log.Printf("  A COPIAR : %d arquivos · %s",
			total.ArquivosVistos-total.ArquivosSoLink, tamanhoLegivel(total.BytesVistos-total.BytesSoLink))
		log.Printf("  SÓ O LINK: %d arquivos · %s (ficam no Trílogo, por decisão)",
			total.ArquivosSoLink, tamanhoLegivel(total.BytesSoLink))
	}
	if total.BytesCopiados > 0 {
		log.Printf("copiados para o armazém: %s", tamanhoLegivel(total.BytesCopiados))
	}
}

func resumo(r *trilogo.Resultado) string {
	return fmt.Sprintf("chamados lidos %d · gravados %d · eventos %d · arquivos vistos %d (%s) · copiados %d (%s)",
		r.ChamadosLidos, r.ChamadosGravados, r.EventosGravados,
		r.ArquivosVistos, tamanhoLegivel(r.BytesVistos),
		r.ArquivosCopiados, tamanhoLegivel(r.BytesCopiados))
}

func tamanhoLegivel(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f kB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func clienteDoSlug(ctx context.Context, bd *banco.Cliente, slug string) (string, error) {
	var linhas []struct {
		ID string `json:"id"`
	}
	if err := bd.Buscar(ctx, "clientes?slug=eq."+banco.Escapar(slug)+"&select=id&limit=1", &linhas); err != nil {
		return "", err
	}
	if len(linhas) == 0 {
		return "", fmt.Errorf("não achei o cliente %q", slug)
	}
	return linhas[0].ID, nil
}
