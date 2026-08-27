// rev 1 — que erro vale tentar de novo
//
// POR QUE ISTO É UM TESTE E NÃO UM COMENTÁRIO
//
//	A fila do robô e o botão da tela decidem coisas DIFERENTES com a mesma
//	resposta: a fila escolhe entre devolver o trabalho e desistir de vez; o
//	botão escolhe entre "tente de novo em instantes" e "esta nota tem problema".
//	As duas decisões saem daqui.
//
//	Antes da separação, essa escolha era um `if` dentro do `main` do robô, e o
//	motor não tinha como fazê-la. Agora é o TIPO do erro — e tipo se testa.
package leitura

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// servicoDeMentira responde cada GET pela tabela que o caminho pedir.
func servicoDeMentira(t *testing.T, porTabela map[string][]map[string]any) *Servico {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tabela := strings.Trim(strings.SplitN(r.URL.Path, "?", 2)[0], "/")
		if i := strings.LastIndex(tabela, "/"); i >= 0 {
			tabela = tabela[i+1:]
		}
		linhas := porTabela[tabela]
		if linhas == nil {
			linhas = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(linhas)
	}))
	t.Cleanup(s.Close)
	cfg := &config.Config{Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"}}
	return Novo(banco.Novo(cfg), armazem.Novo(cfg), nil)
}

// NOTA QUE NÃO EXISTE NÃO MELHORA NA QUARTA TENTATIVA
//
//	Se este erro viesse marcado como temporário, o robô devolveria o trabalho
//	para a fila três vezes para receber o mesmo "não achei" — e o botão diria
//	"tente de novo em instantes" para uma nota que não vai aparecer.
func TestDocumentoInexistenteNaoEhFalhaTemporaria(t *testing.T) {
	s := servicoDeMentira(t, nil)

	_, err := s.Ler(context.Background(), "d-que-nao-existe")
	if err == nil {
		t.Fatal("ler um documento inexistente deu certo")
	}
	var temporaria FalhaTemporaria
	if errors.As(err, &temporaria) {
		t.Fatalf("o erro veio marcado como temporário: %v", err)
	}
}

// O ARMAZÉM QUE NÃO RESPONDE É OUTRA HISTÓRIA
//
//	A nota está inteira e a leitura nunca chegou a ser tentada. Tratar isso como
//	problema da nota a marcaria "falhou" por um minuto ruim da rede — e ninguém
//	depois saberia que não era culpa dela.
func TestFalhaAoBaixarEhTemporaria(t *testing.T) {
	s := servicoDeMentira(t, map[string][]map[string]any{
		"documentos": {{
			"id": "d1", "nome_arquivo": "NF.pdf", "arquivo_sha256": "abcdef0123456789",
			"fila": "orcamento", "inserido_em": "2026-08-27T10:00:00Z", "cliente_id": "c1",
		}},
		// `arquivos` volta vazio: o armazém não sabe deste sha.
	})

	_, err := s.Ler(context.Background(), "d1")
	if err == nil {
		t.Fatal("baixar um arquivo que não está registrado deu certo")
	}
	var temporaria FalhaTemporaria
	if !errors.As(err, &temporaria) {
		t.Fatalf("a falha ao baixar não foi marcada como temporária: %v", err)
	}
	if temporaria.Unwrap() == nil {
		t.Error("a marca engoliu a causa — quem lê o log fica sem o motivo")
	}
}

// SEM CHAVE E SEM `pdftotext`, A NOTA NÃO É CULPADA
//
//	Este é o caso do motor recém-subido com a chave da IA só no GitHub. Sem esta
//	pergunta, cada clique no botão mandaria a nota para a cascata, ela falharia
//	lá dentro e voltaria marcada `falhou` — noventa notas acusadas de um
//	problema do servidor.
func TestSemIANaoSeLeImagemNemPDF(t *testing.T) {
	semIA := servicoDeMentira(t, nil) // Novo(..., nil): sem IA nenhuma

	if falta := semIA.FaltaOLeitor("nota.xml"); falta != "" {
		t.Errorf("XML não precisa de IA e mesmo assim recusou: %q", falta)
	}
	if falta := semIA.FaltaOLeitor("foto.jpg"); falta == "" {
		t.Error("uma foto sem IA foi dada como legível — a nota levaria a culpa")
	}
	if falta := semIA.FaltaOLeitor("nota.PDF"); falta == "" && !temPdftotext() {
		t.Error("um PDF sem IA e sem pdftotext foi dado como legível")
	}
}

func temPdftotext() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}
