// rev 1 — o e-mail sai pela caixa do próprio domínio (HostGator/cPanel), não por terceiro
//
// POR QUE TROCAR O BREVO (11/09/2026)
//
//	O Brevo exige validação de remetente OU autenticação de domínio inteira
//	(SPF/DKIM/DMARC) antes de aceitar um envio — pco@frotamacedo.com.br
//	nunca foi validado lá, e frotamacedo.com.br aparece "Não autenticado"
//	(ver DIARIOS_CLAUDE/diario_110926_1045_pco.md, o diagnóstico completo).
//	Mandar pela caixa de e-mail do próprio domínio (SMTP do HostGator/
//	cPanel) não tem essa barreira: o e-mail já sai de dentro da conta dona
//	do domínio. `brevo/brevo.go` continua no repositório, sem ninguém
//	chamando — troca fácil de desfazer se o SMTP do HostGator surpreender
//	(limite de envio por hora mais baixo, sem log de entrega como o Brevo
//	tinha — foi o log dele que achou o bug original).
//
// SEM SDK, IGUAL AO RESTO DA CASA
//
//	`net/smtp` da biblioteca padrão dá conta — autenticação PLAIN e
//	STARTTLS (porta 587) ou TLS direto (porta 465, o padrão mais comum em
//	hospedagem compartilhada) é o suficiente para uma caixa de e-mail
//	comum. Nenhuma dependência nova.
package correio

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

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
	cfg config.SMTP
}

func Novo(cfg config.SMTP) *Cliente {
	return &Cliente{cfg: cfg}
}

func (c *Cliente) Ligado() bool { return c.cfg.Ligado() }

// Enviar manda a mensagem pela caixa configurada (SMTP_USUARIO). Porta 465
// usa TLS desde o primeiro byte (o padrão de hospedagem compartilhada);
// qualquer outra porta (587, em geral) negocia STARTTLS — net/smtp faz
// isso sozinho dentro de SendMail.
func (c *Cliente) Enviar(ctx context.Context, m Mensagem) error {
	if !c.Ligado() {
		return errors.New("o envio de e-mail não está configurado neste servidor (falta o SMTP do HostGator)")
	}
	if len(m.Para) == 0 {
		return errors.New("nenhum destinatário — não dá para enviar um e-mail sem ninguém para receber")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	corpo, err := montarMIME(c.cfg.NomeRemetente, c.cfg.Usuario, m)
	if err != nil {
		return fmt.Errorf("não consegui montar o e-mail: %w", err)
	}

	if c.cfg.Porta == 465 {
		err = enviarComTLSImediato(c.cfg, m.Para, corpo)
	} else {
		endereco := fmt.Sprintf("%s:%d", c.cfg.Servidor, c.cfg.Porta)
		auth := smtp.PlainAuth("", c.cfg.Usuario, c.cfg.Senha, c.cfg.Servidor)
		err = smtp.SendMail(endereco, auth, c.cfg.Usuario, m.Para, corpo)
	}
	if err != nil {
		return fmt.Errorf("o servidor de e-mail (%s) recusou o envio: %w", c.cfg.Servidor, err)
	}
	return nil
}

// enviarComTLSImediato é o caminho da porta 465: TLS já na conexão, sem
// STARTTLS — net/smtp.SendMail não sabe fazer isso sozinho, então a
// conversa SMTP é feita à mão aqui, igual ao exemplo da documentação do
// pacote.
func enviarComTLSImediato(cfg config.SMTP, para []string, corpo []byte) error {
	endereco := fmt.Sprintf("%s:%d", cfg.Servidor, cfg.Porta)
	conexao, err := tls.Dial("tcp", endereco, &tls.Config{ServerName: cfg.Servidor})
	if err != nil {
		return fmt.Errorf("não consegui conectar: %w", err)
	}
	defer conexao.Close()

	cliente, err := smtp.NewClient(conexao, cfg.Servidor)
	if err != nil {
		return err
	}
	defer cliente.Close()

	auth := smtp.PlainAuth("", cfg.Usuario, cfg.Senha, cfg.Servidor)
	if err := cliente.Auth(auth); err != nil {
		return fmt.Errorf("autenticação recusada: %w", err)
	}
	if err := cliente.Mail(cfg.Usuario); err != nil {
		return err
	}
	for _, dest := range para {
		if err := cliente.Rcpt(dest); err != nil {
			return fmt.Errorf("destinatário %s recusado: %w", dest, err)
		}
	}
	escritor, err := cliente.Data()
	if err != nil {
		return err
	}
	if _, err := escritor.Write(corpo); err != nil {
		return err
	}
	if err := escritor.Close(); err != nil {
		return err
	}
	return cliente.Quit()
}

// montarMIME monta a mensagem RFC 5322 inteira: cabeçalho + corpo HTML +
// anexos, em multipart/mixed. net/smtp manda bytes crus — quem monta a
// estrutura MIME somos nós.
func montarMIME(nomeRemetente, remetente string, m Mensagem) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	de := remetente
	if nomeRemetente != "" {
		de = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", nomeRemetente), remetente)
	}
	fmt.Fprintf(&buf, "From: %s\r\n", de)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(m.Para, ", "))
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", m.Assunto))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n", w.Boundary())
	fmt.Fprintf(&buf, "\r\n")

	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", `text/html; charset="UTF-8"`)
	htmlHeader.Set("Content-Transfer-Encoding", "base64")
	htmlParte, err := w.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	if err := escreverBase64(htmlParte, []byte(m.HTML)); err != nil {
		return nil, err
	}

	for _, a := range m.Anexos {
		anexoHeader := textproto.MIMEHeader{}
		anexoHeader.Set("Content-Type", fmt.Sprintf("application/octet-stream; name=%q", a.Nome))
		anexoHeader.Set("Content-Transfer-Encoding", "base64")
		anexoHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.Nome))
		anexoParte, err := w.CreatePart(anexoHeader)
		if err != nil {
			return nil, err
		}
		if err := escreverBase64(anexoParte, a.Conteudo); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// escreverBase64 quebra em linhas de 76 colunas — o limite que a RFC 2045
// pede para base64 em e-mail (linha maior derruba alguns servidores).
func escreverBase64(destino interface{ Write([]byte) (int, error) }, dados []byte) error {
	codificado := base64.StdEncoding.EncodeToString(dados)
	for i := 0; i < len(codificado); i += 76 {
		fim := i + 76
		if fim > len(codificado) {
			fim = len(codificado)
		}
		if _, err := destino.Write([]byte(codificado[i:fim] + "\r\n")); err != nil {
			return err
		}
	}
	return nil
}
