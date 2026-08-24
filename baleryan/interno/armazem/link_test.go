package armazem

import (
	"strings"
	"testing"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

func armazemDeTeste() *Cliente {
	return Novo(&config.Config{R2: config.R2{
		ContaID: "conta", Bucket: "frotahub",
		ChaveID: "chave-de-teste", ChaveSecreta: "segredo-de-teste",
	}})
}

// Mesma disciplina do teste do envio: a assinatura é comparada com a de uma
// implementação INDEPENDENTE, escrita em Python. O nome do arquivo tem um espaço
// de propósito — é onde a codificação erra, e o erro aparece como 403 mudo.
func TestLinkTemporarioBateComImplementacaoIndependente(t *testing.T) {
	c := armazemDeTeste()
	quando := time.Date(2026, 8, 24, 10, 11, 12, 0, time.UTC)

	link, err := c.linkEm("clientes/frota-macedo/arquivos/ab/cd/abcd ef.jpeg", 5*time.Minute, quando)
	if err != nil {
		t.Fatalf("não assinou: %v", err)
	}

	const esperado = "https://conta.r2.cloudflarestorage.com" +
		"/frotahub/clientes/frota-macedo/arquivos/ab/cd/abcd%20ef.jpeg" +
		"?X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=chave-de-teste%2F20260824%2Fauto%2Fs3%2Faws4_request" +
		"&X-Amz-Date=20260824T101112Z" +
		"&X-Amz-Expires=300" +
		"&X-Amz-SignedHeaders=host" +
		"&X-Amz-Signature=dbad5e49a194e7df59c1f8a1135bbdbb4d9b5c4fed9b34333caf0bb921cb050b"

	if link != esperado {
		t.Fatalf("link diferente do calculado em Python.\nveio:    %s\nesperava: %s", link, esperado)
	}
}

// O segredo assina, mas não viaja. Se um dia alguém "simplificar" a assinatura
// mandando a chave secreta na URL, este teste cai antes de o link chegar a um
// navegador (CORE-09).
func TestLinkNaoCarregaOSegredo(t *testing.T) {
	c := armazemDeTeste()
	link, err := c.LinkTemporario("clientes/frota-macedo/arquivos/aa/bb/aabb.jpg", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(link, "segredo-de-teste") {
		t.Fatalf("a chave secreta vazou no link: %s", link)
	}
	if !strings.Contains(link, "X-Amz-Expires=60") {
		t.Errorf("o prazo pedido não foi respeitado: %s", link)
	}
}

// Prazo é prazo: nem zero, nem eterno.
func TestPrazoDoLinkFicaDentroDosLimites(t *testing.T) {
	c := armazemDeTeste()
	for _, caso := range []struct {
		nome     string
		validade time.Duration
		quer     string
	}{
		{"zero vira um segundo", 0, "X-Amz-Expires=1"},
		{"negativo vira um segundo", -time.Hour, "X-Amz-Expires=1"},
		{"acima do teto vira uma semana", 30 * 24 * time.Hour, "X-Amz-Expires=604800"},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			link, err := c.LinkTemporario("clientes/x/arquivos/aa/bb/aabb.jpg", caso.validade)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(link, caso.quer) {
				t.Errorf("esperava %s em %s", caso.quer, link)
			}
		})
	}
}

// Sem armazém configurado, não existe link — e o erro diz o que falta, em vez de
// devolver uma URL quebrada que só falha no navegador.
func TestSemArmazemNaoInventaLink(t *testing.T) {
	c := Novo(&config.Config{})
	if _, err := c.LinkTemporario("qualquer", time.Minute); err == nil {
		t.Fatal("assinou sem configuração nenhuma")
	}
}
