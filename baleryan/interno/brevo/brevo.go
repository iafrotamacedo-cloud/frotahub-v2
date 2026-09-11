// rev 1 — o cliente do Brevo (11/09/2026)
//
// SÓ O QUE O PCO PRECISA, NADA A MAIS
//
//	A API do Brevo faz muito mais que e-mail transacional simples (SMS,
//	campanhas, contatos). Este cliente fala só com `POST /v3/smtp/email`, com
//	anexo — o suficiente para o pacote de PCO. Se outro módulo precisar de
//	e-mail um dia, ele cresce a partir daqui (CORE-06), não ganha um segundo
//	cliente HTTP para o mesmo provedor.
//
// SEM SDK
//
//	Mesma escolha de `interno/armazem` para o R2: a API é um POST com JSON,
//	um SDK inteiro para isto é peso morto. Ver o cabeçalho de `armazem/r2.go`.
package brevo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

const enderecoEnvio = "https://api.brevo.com/v3/smtp/email"

// Anexo é um arquivo para grudar no e-mail — sempre um .zip, no caso do PCO.
type Anexo struct {
	Nome     string
	Conteudo []byte
}

// Mensagem é o que `Enviar` manda. Sem CC de propósito: a lista de
// destinatários de PCO não distingue Para/Cc (ver a migração 061) — todo
// mundo ativo entra no mesmo "Para".
type Mensagem struct {
	Para    []string
	Assunto string
	HTML    string
	Anexos  []Anexo
}

type Cliente struct {
	cfg  config.Brevo
	http *http.Client
}

func Novo(cfg config.Brevo) *Cliente {
	return &Cliente{cfg: cfg, http: &http.Client{Timeout: 2 * time.Minute}}
}

func (c *Cliente) Ligado() bool { return c.cfg.Ligado() }

type destinatario struct {
	Email string `json:"email"`
}

type remetente struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type anexoAPI struct {
	Content string `json:"content"` // base64
	Name    string `json:"name"`
}

type corpoAPI struct {
	Sender      remetente      `json:"sender"`
	To          []destinatario `json:"to"`
	Subject     string         `json:"subject"`
	HTMLContent string         `json:"htmlContent"`
	Attachment  []anexoAPI     `json:"attachment,omitempty"`
}

// Enviar manda a mensagem. Erro sempre em português, com o corpo que o
// Brevo devolveu quando ele recusa — é o que explica "domínio do remetente
// não verificado" em vez de um genérico "falha ao enviar".
func (c *Cliente) Enviar(ctx context.Context, m Mensagem) error {
	if !c.Ligado() {
		return errors.New("o envio de e-mail não está configurado neste servidor (falta a chave do Brevo)")
	}
	if len(m.Para) == 0 {
		return errors.New("nenhum destinatário — não dá para enviar um e-mail sem ninguém para receber")
	}

	corpo := corpoAPI{
		Sender:      remetente{Name: c.cfg.NomeRemetente, Email: c.cfg.Remetente},
		Subject:     m.Assunto,
		HTMLContent: m.HTML,
	}
	for _, e := range m.Para {
		corpo.To = append(corpo.To, destinatario{Email: e})
	}
	for _, a := range m.Anexos {
		corpo.Attachment = append(corpo.Attachment, anexoAPI{
			Name:    a.Nome,
			Content: base64.StdEncoding.EncodeToString(a.Conteudo),
		})
	}

	bruto, err := json.Marshal(corpo)
	if err != nil {
		return fmt.Errorf("não consegui montar o e-mail: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, enderecoEnvio, bytes.NewReader(bruto))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", c.cfg.Chave)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("não consegui falar com o Brevo: %w", err)
	}
	defer resp.Body.Close()
	respostaBruta, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("o Brevo recusou o envio (%d): %s", resp.StatusCode, primeiraLinha(respostaBruta))
	}
	return nil
}

func primeiraLinha(b []byte) string {
	s := string(b)
	for i, r := range s {
		if r == '\n' {
			s = s[:i]
			break
		}
	}
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
