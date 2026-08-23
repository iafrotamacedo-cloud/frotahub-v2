// rev 2 — baleryan, o motor do FrotaHub
//
// Este arquivo faz três coisas e só:
//
//  1. lê a configuração (e recusa subir se estiver faltando alguma coisa)
//  2. monta as peças — banco, segurança, permissão — e os módulos
//  3. registra as rotas comuns e fica escutando
//
// NENHUMA regra de negócio mora aqui. Este arquivo não sabe o que é orçamento,
// nota ou ticket. Cada módulo traz as suas próprias rotas e este arquivo só o
// monta — ele não cresce junto com o sistema (P-13).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/usuarios"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// Revisao aparece em /saude, para conferir o que está no ar sem abrir o servidor.
const Revisao = "2"

type motor struct {
	cfg  *config.Config
	seg  *seguranca.Servico
	perm *permissao.Servico
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[baleryan] ")

	cfg, err := config.Carregar()
	if err != nil {
		// Falha na largada, com a lista completa do que falta (P-09).
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	bd := banco.Novo(cfg)
	seg := seguranca.Novo(cfg, bd)
	m := &motor{cfg: cfg, seg: seg, perm: permissao.Novo(bd)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /saude", m.saude)
	mux.HandleFunc("GET /api/eu", m.eu)

	// Os módulos trazem as suas rotas.
	usuarios.Novo(cfg, bd, seg).Montar(mux)

	// A ordem importa: CORS por fora de tudo, para que até um erro inesperado
	// chegue ao navegador como erro de verdade, e não como "Failed to fetch".
	manipulador := web.CORS(cfg, web.Recuperar(web.Registrar(mux)))

	servidor := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Runtime.Porta),
		Handler:           manipulador,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Encerramento educado: para de aceitar requisição nova e espera as que estão
	// em andamento terminarem.
	parar := make(chan os.Signal, 1)
	signal.Notify(parar, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("rev %s · ambiente %s · escutando na porta %d",
			Revisao, cfg.Runtime.Ambiente, cfg.Runtime.Porta)
		if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("não consegui subir: %v", err)
		}
	}()

	<-parar
	log.Println("encerrando...")
	ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	if err := servidor.Shutdown(ctx); err != nil {
		log.Printf("aviso ao encerrar: %v", err)
	}
	log.Println("encerrado.")
}

// GET /saude — o motor está vivo? Não pede login: é o que o Render e o front
// consultam para saber se o serviço respondeu. Não revela nenhuma chave.
func (m *motor) saude(w http.ResponseWriter, r *http.Request) {
	web.Responder(w, http.StatusOK, map[string]any{
		"ok":      true,
		"motor":   "baleryan",
		"revisao": Revisao,
		"ligado":  m.cfg.Resumo(),
	})
}

// GET /api/eu — quem sou eu e o que eu posso.
//
// É a primeira chamada do front depois do login, e é dela que sai o menu: o front
// recebe a lista de rotinas permitidas e esconde o que não estiver nela (P-17).
func (m *motor) eu(w http.ResponseWriter, r *http.Request) {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return
	}

	rotinas, err := m.perm.Rotinas(r.Context(), p)
	if err != nil {
		log.Printf("erro lendo permissões de %s: %v", p.Usuario, err)
		web.Falhar(w, http.StatusInternalServerError,
			"Não consegui carregar as suas permissões. Tente de novo em instantes.")
		return
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"principal": p,
		"rotinas":   rotinas,
	})
}
