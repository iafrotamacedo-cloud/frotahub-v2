// rev 1 — o armazém: enviar arquivo para o Cloudflare R2
//
// O R2 fala o mesmo protocolo do S3 da Amazon, e a assinatura desse protocolo
// (Signature V4) está escrita aqui à mão. São umas cem linhas de HMAC — contra
// um SDK de dezenas de megabytes que traria consigo meia AWS. O motor não tem
// dependência externa nenhuma, e vai continuar assim.
//
// O CAMINHO DO ARQUIVO É O SHA256 DELE
//
//	Não é o nome, não é o número do chamado, não é a data. Dois arquivos com o
//	mesmo conteúdo caem no mesmo lugar, e duplicar vira impossível por
//	construção — não por disciplina de quem escreve o código (P-04).
//
//	O nome original não se perde: fica na tabela de aparições, junto de quem
//	subiu e quando.
package armazem

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

type Cliente struct {
	cfg  *config.Config
	http *http.Client
	// Endereço alternativo, usado pelos testes para apontar a um armazém de
	// mentira. Vazio em produção, que é o caso normal.
	endereco string
}

func (c *Cliente) base() string {
	if c.endereco != "" {
		return c.endereco
	}
	return c.cfg.R2.Endpoint()
}

func Novo(cfg *config.Config) *Cliente {
	return &Cliente{cfg: cfg, http: &http.Client{Timeout: 5 * time.Minute}}
}

// NovoEm aponta para outro endereço. Serve aos testes, que sobem um armazém de
// mentira; em produção ninguém chama isto.
func NovoEm(cfg *config.Config, endereco string) *Cliente {
	c := Novo(cfg)
	c.endereco = endereco
	return c
}

func (c *Cliente) Ligado() bool { return c.cfg.R2.Ligado() }

// Caminho monta a chave do objeto a partir do conteúdo.
//
// Os dois primeiros pares de caracteres viram pastas. Não é enfeite: um balde com
// centenas de milhares de objetos num prefixo só fica desagradável de listar e de
// navegar. Espalhar em 256 × 256 gavetas resolve isso de graça.
func Caminho(cliente, sha, extensao string) string {
	if len(sha) < 4 {
		return "clientes/" + cliente + "/arquivos/" + sha
	}
	ext := strings.TrimPrefix(strings.ToLower(extensao), ".")
	nome := sha
	if ext != "" {
		nome = sha + "." + ext
	}
	return "clientes/" + cliente + "/arquivos/" + sha[0:2] + "/" + sha[2:4] + "/" + nome
}

// Enviar grava o objeto.
//
// `sha` é o sha256 do conteúdo em hexadecimal — o mesmo valor que vira caminho e
// que o protocolo exige no cabeçalho. Quem chama já o calculou ao baixar; pedir
// de novo aqui seria ler o arquivo inteiro uma segunda vez (CORE-01).
func (c *Cliente) Enviar(ctx context.Context, chave string, corpo io.ReadSeeker, tamanho int64, sha, tipo string) error {
	if !c.Ligado() {
		return fmt.Errorf("o armazém não está configurado (R2_*)")
	}
	if tipo == "" {
		tipo = "application/octet-stream"
	}
	if _, err := corpo.Seek(0, io.SeekStart); err != nil {
		return err
	}

	agora := time.Now().UTC()
	carimbo := agora.Format("20060102T150405Z")
	dia := agora.Format("20060102")
	alvo := c.base()
	host := strings.TrimPrefix(strings.TrimPrefix(alvo, "https://"), "http://")
	caminho := "/" + c.cfg.R2.Bucket + "/" + escaparCaminho(chave)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, alvo+caminho, corpo)
	if err != nil {
		return err
	}
	req.ContentLength = tamanho
	req.Header.Set("Host", host)
	req.Header.Set("Content-Type", tipo)
	req.Header.Set("x-amz-content-sha256", sha)
	req.Header.Set("x-amz-date", carimbo)
	req.Header.Set("Authorization", c.assinar(dia, carimbo, host, caminho, tipo, sha))

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("não consegui falar com o armazém: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 600))
		return fmt.Errorf("o armazém recusou (%d): %s", resp.StatusCode, strings.TrimSpace(string(bruto)))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assinatura V4
//
// A receita é fixa e o protocolo é intolerante: qualquer diferença de espaço, de
// ordem de cabeçalho ou de maiúscula muda a assinatura e a resposta vira 403 sem
// explicação. Por isso cada etapa está separada e nomeada.
// ---------------------------------------------------------------------------

const regiao = "auto"
const servico = "s3"

func (c *Cliente) assinar(dia, carimbo, host, caminho, tipo, sha string) string {
	assinados := "content-type;host;x-amz-content-sha256;x-amz-date"

	canonica := strings.Join([]string{
		http.MethodPut,
		caminho,
		"", // sem parâmetros na URL
		"content-type:" + tipo,
		"host:" + host,
		"x-amz-content-sha256:" + sha,
		"x-amz-date:" + carimbo,
		"",
		assinados,
		sha,
	}, "\n")

	escopo := dia + "/" + regiao + "/" + servico + "/aws4_request"
	paraAssinar := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		carimbo,
		escopo,
		resumo(canonica),
	}, "\n")

	chave := hmacBytes(
		hmacBytes(
			hmacBytes(
				hmacBytes([]byte("AWS4"+c.cfg.R2.ChaveSecreta), dia),
				regiao),
			servico),
		"aws4_request")

	return "AWS4-HMAC-SHA256 " +
		"Credential=" + c.cfg.R2.ChaveID + "/" + escopo + ", " +
		"SignedHeaders=" + assinados + ", " +
		"Signature=" + hex.EncodeToString(hmacRaw(chave, paraAssinar))
}

func hmacRaw(chave []byte, dado string) []byte {
	m := hmac.New(sha256.New, chave)
	m.Write([]byte(dado))
	return m.Sum(nil)
}

func hmacBytes(chave []byte, dado string) []byte { return hmacRaw(chave, dado) }

func resumo(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// escaparCaminho codifica cada pedaço do caminho, preservando as barras.
//
// A regra do protocolo NÃO é a do `url.QueryEscape`: espaço é %20 e não "+", e
// os caracteres -_.~ passam intactos. Usar a função pronta aqui dá 403.
func escaparCaminho(chave string) string {
	var b strings.Builder
	for i := 0; i < len(chave); i++ {
		ch := chave[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~', ch == '/':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
