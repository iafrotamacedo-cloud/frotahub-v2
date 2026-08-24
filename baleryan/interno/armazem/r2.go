// rev 2 — o armazém: enviar arquivo para o Cloudflare R2, e deixar ver de volta
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
//
// COMO A FOTO CHEGA NA TELA SEM O BALDE SER PÚBLICO
//
//	O balde não tem acesso público, e não vai ter (CORE-07). Para a tela mostrar
//	uma foto, o motor assina um endereço temporário — a assinatura vai na própria
//	URL e vence em minutos. Passou o prazo, o mesmo endereço devolve erro.
//
//	É exatamente o contrário do que o sistema antigo faz, onde o endereço do
//	arquivo é público, não vence nunca e vale para quem quer que o tenha recebido.
package armazem

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
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

// LinkTemporario devolve um endereço assinado que abre o objeto e vence sozinho.
//
// A diferença para o Enviar é onde a assinatura viaja: lá, num cabeçalho; aqui,
// dentro da própria URL — é o que permite jogar o endereço num <img> e o
// navegador buscar sozinho, sem saber de chave nenhuma. O segredo NUNCA sai do
// motor (CORE-09): o que sai é uma assinatura que só serve para este objeto e
// só durante este prazo.
//
// O corpo não é assinado (UNSIGNED-PAYLOAD): numa leitura não existe corpo.
func (c *Cliente) LinkTemporario(chave string, validade time.Duration) (string, error) {
	return c.linkEm(chave, validade, time.Now().UTC())
}

// linkEm é o LinkTemporario com o relógio por fora. Existe para o teste poder
// fixar o instante e comparar a assinatura com uma implementação independente —
// uma assinatura que só a si mesma confirma não prova nada.
func (c *Cliente) linkEm(chave string, validade time.Duration, agora time.Time) (string, error) {
	if !c.Ligado() {
		return "", fmt.Errorf("o armazém não está configurado (R2_*)")
	}
	segundos := int(validade.Seconds())
	switch {
	case segundos < 1:
		segundos = 1
	case segundos > 7*24*3600: // o teto do protocolo é uma semana
		segundos = 7 * 24 * 3600
	}

	agora = agora.UTC()
	carimbo := agora.Format("20060102T150405Z")
	dia := agora.Format("20060102")
	alvo := c.base()
	host := strings.TrimPrefix(strings.TrimPrefix(alvo, "https://"), "http://")
	caminho := "/" + c.cfg.R2.Bucket + "/" + escaparCaminho(chave)
	escopo := dia + "/" + regiao + "/" + servico + "/aws4_request"

	// A ordem dos parâmetros na consulta canônica é alfabética, e cada valor vai
	// codificado. A barra dentro do Credential vira %2F — se ficar barra crua, a
	// assinatura fecha aqui e o servidor recusa lá, sem dizer por quê.
	partes := []string{
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		"X-Amz-Credential=" + escaparValor(c.cfg.R2.ChaveID+"/"+escopo),
		"X-Amz-Date=" + carimbo,
		"X-Amz-Expires=" + strconv.Itoa(segundos),
		"X-Amz-SignedHeaders=host",
	}
	sort.Strings(partes)
	consulta := strings.Join(partes, "&")

	canonica := strings.Join([]string{
		http.MethodGet,
		caminho,
		consulta,
		"host:" + host,
		"",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")

	paraAssinar := strings.Join([]string{
		"AWS4-HMAC-SHA256", carimbo, escopo, resumo(canonica),
	}, "\n")

	assinatura := hex.EncodeToString(hmacRaw(c.chaveDoDia(dia), paraAssinar))
	return alvo + caminho + "?" + consulta + "&X-Amz-Signature=" + assinatura, nil
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

	return "AWS4-HMAC-SHA256 " +
		"Credential=" + c.cfg.R2.ChaveID + "/" + escopo + ", " +
		"SignedHeaders=" + assinados + ", " +
		"Signature=" + hex.EncodeToString(hmacRaw(c.chaveDoDia(dia), paraAssinar))
}

// chaveDoDia deriva a chave de assinatura. São quatro HMAC encadeados, sempre na
// mesma ordem — data, região, serviço, encerramento — e a chave resultante só
// vale para aquele dia. Os dois caminhos (envio e link temporário) usam esta, em
// vez de cada um repetir a escada (CORE-06).
func (c *Cliente) chaveDoDia(dia string) []byte {
	return hmacBytes(
		hmacBytes(
			hmacBytes(
				hmacBytes([]byte("AWS4"+c.cfg.R2.ChaveSecreta), dia),
				regiao),
			servico),
		"aws4_request")
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
// escaparValor codifica um valor de parâmetro. É o escaparCaminho SEM a exceção
// da barra: aqui ela também vira %2F, porque não separa pastas — faz parte do
// valor.
func escaparValor(v string) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		ch := v[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

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
