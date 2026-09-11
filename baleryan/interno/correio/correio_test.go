package correio

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

// TestMontarMIME confere que a mensagem que construímos à mão é um e-mail
// de verdade — abrimos com net/mail (cabeçalho) e mime/multipart (corpo),
// as mesmas bibliotecas que um servidor de e-mail usaria para ler o que
// mandamos. É o jeito de testar a montagem sem precisar de credencial SMTP
// nenhuma.
func TestMontarMIME(t *testing.T) {
	m := Mensagem{
		Para:    []string{"cliente@exemplo.com", "outro@exemplo.com"},
		Assunto: "Ordens de Compra (PCO) - 11-09-2026 - Frota Macedo Engenharia",
		HTML:    "<p>Olá, ç ã é — acentos de propósito.</p>",
		Anexos: []Anexo{
			{Nome: "Pedidos_PCO_11-09-2026.zip", Conteudo: []byte("conteudo-fake-do-zip-0123456789")},
		},
	}

	bruto, err := montarMIME("Frota Macedo Engenharia", "pco@frotamacedo.com.br", m)
	if err != nil {
		t.Fatalf("montarMIME: %v", err)
	}

	msg, err := mail.ReadMessage(bytes.NewReader(bruto))
	if err != nil {
		t.Fatalf("net/mail não conseguiu ler a mensagem: %v", err)
	}

	de := msg.Header.Get("From")
	if !strings.Contains(de, "pco@frotamacedo.com.br") {
		t.Errorf("From não tem o remetente: %q", de)
	}
	assunto, err := (&mime.WordDecoder{}).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("decodificando Subject: %v", err)
	}
	if assunto != m.Assunto {
		t.Errorf("Subject: esperava %q, veio %q", m.Assunto, assunto)
	}

	_, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("Content-Type inválido: %v", err)
	}
	if !strings.HasPrefix(msg.Header.Get("Content-Type"), "multipart/mixed") {
		t.Fatalf("Content-Type não é multipart/mixed: %q", msg.Header.Get("Content-Type"))
	}

	leitor := multipart.NewReader(msg.Body, params["boundary"])
	var partes int
	var achouHTML, achouAnexo bool
	for {
		parte, err := leitor.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("lendo parte %d: %v", partes, err)
		}
		partes++
		conteudo, err := io.ReadAll(parte)
		if err != nil {
			t.Fatalf("lendo corpo da parte %d: %v", partes, err)
		}
		decodificado, err := base64.StdEncoding.DecodeString(string(conteudo))
		if err != nil {
			t.Fatalf("parte %d não é base64 válido: %v", partes, err)
		}

		switch {
		case strings.HasPrefix(parte.Header.Get("Content-Type"), "text/html"):
			achouHTML = true
			if string(decodificado) != m.HTML {
				t.Errorf("HTML da parte não bate:\nesperava %q\nveio     %q", m.HTML, string(decodificado))
			}
		case strings.Contains(parte.Header.Get("Content-Disposition"), "attachment"):
			achouAnexo = true
			if !bytes.Equal(decodificado, m.Anexos[0].Conteudo) {
				t.Errorf("conteúdo do anexo não bate: %d bytes esperados, %d vieram", len(m.Anexos[0].Conteudo), len(decodificado))
			}
			if !strings.Contains(parte.Header.Get("Content-Disposition"), m.Anexos[0].Nome) {
				t.Errorf("nome do anexo não aparece no Content-Disposition: %q", parte.Header.Get("Content-Disposition"))
			}
		}
	}
	if partes != 2 {
		t.Errorf("esperava 2 partes (HTML + anexo), vieram %d", partes)
	}
	if !achouHTML {
		t.Error("não achei a parte text/html")
	}
	if !achouAnexo {
		t.Error("não achei a parte com Content-Disposition: attachment")
	}
}

func TestMontarMIME_SemAnexo(t *testing.T) {
	m := Mensagem{Para: []string{"a@b.com"}, Assunto: "sem acento", HTML: "<p>oi</p>"}
	bruto, err := montarMIME("", "pco@frotamacedo.com.br", m)
	if err != nil {
		t.Fatalf("montarMIME: %v", err)
	}
	if _, err := mail.ReadMessage(bytes.NewReader(bruto)); err != nil {
		t.Fatalf("net/mail não conseguiu ler a mensagem: %v", err)
	}
}
