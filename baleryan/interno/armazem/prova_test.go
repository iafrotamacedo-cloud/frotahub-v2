package armazem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// A assinatura do S3 é intolerante: qualquer diferença de espaço, de ordem de
// cabeçalho ou de maiúscula muda o resultado, e o servidor responde 403 sem
// explicar nada. Este teste compara o que o Go produz com o que uma
// implementação INDEPENDENTE, escrita em Python, produziu para as mesmas
// entradas. Duas implementações concordando é prova; uma sozinha é esperança.
func TestAssinaturaBateComImplementacaoIndependente(t *testing.T) {
	cfg := &config.Config{R2: config.R2{
		ContaID: "conta", Bucket: "frotahub",
		ChaveID: "chave-de-teste", ChaveSecreta: "segredo-de-teste",
	}}
	c := Novo(cfg)

	soma := sha256.Sum256([]byte("conteudo do orcamento"))
	sha := hex.EncodeToString(soma[:])
	const shaEsperado = "aa5a5c8418f66dc1f519b135079728951516fb465a253c9029238882fa7c65c5"
	if sha != shaEsperado {
		t.Fatalf("o sha256 do conteúdo mudou: %s", sha)
	}

	cab := c.assinar(
		"20260824", "20260824T101112Z",
		"conta.r2.cloudflarestorage.com",
		"/frotahub/clientes/frota-macedo/arquivos/ab/cd/abcd.pdf",
		"application/pdf", sha)

	const esperada = "165aa287935770387b0b8bd2c69c2458fc33c9d7f7f2bf65ac1056f9014ddae6"
	if !strings.HasSuffix(cab, "Signature="+esperada) {
		t.Fatalf("assinatura diferente da calculada em Python.\nveio: %s\nesperava terminar em: Signature=%s", cab, esperada)
	}
	for _, pedaco := range []string{
		"AWS4-HMAC-SHA256 ",
		"Credential=chave-de-teste/20260824/auto/s3/aws4_request",
		"SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date",
	} {
		if !strings.Contains(cab, pedaco) {
			t.Errorf("faltou %q no cabeçalho: %s", pedaco, cab)
		}
	}
}

func TestCaminhoVemDoConteudo(t *testing.T) {
	sha := strings.Repeat("a", 60) + "beef"
	got := Caminho("frota-macedo", sha, "JPG")
	quer := "clientes/frota-macedo/arquivos/aa/aa/" + sha + ".jpg"
	if got != quer {
		t.Fatalf("esperava %s, veio %s", quer, got)
	}
	// mesmo conteúdo, nome diferente -> MESMO lugar. É o que impede duplicar.
	if Caminho("frota-macedo", sha, ".jpg") != quer {
		t.Fatal("o ponto na extensão não pode mudar o caminho")
	}
}

// O protocolo NÃO usa a codificação de query string: espaço é %20, nunca "+",
// e -_.~ passam intactos. Errar isto dá 403 sem explicação.
func TestCaminhoEscapadoDoJeitoDoProtocolo(t *testing.T) {
	got := escaparCaminho("clientes/a b/c+d/e~f.-_")
	quer := "clientes/a%20b/c%2Bd/e~f.-_"
	if got != quer {
		t.Fatalf("esperava %s, veio %s", quer, got)
	}
}

func TestEnviaOObjetoComOsCabecalhosCertos(t *testing.T) {
	var recebido struct {
		metodo, caminho, tipo, sha, autoriza string
		corpo                                []byte
		tamanho                              int64
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recebido.metodo = r.Method
		recebido.caminho = r.URL.Path
		recebido.tipo = r.Header.Get("Content-Type")
		recebido.sha = r.Header.Get("x-amz-content-sha256")
		recebido.autoriza = r.Header.Get("Authorization")
		recebido.tamanho = r.ContentLength
		recebido.corpo, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &config.Config{R2: config.R2{ContaID: "conta", Bucket: "frotahub", ChaveID: "id", ChaveSecreta: "s"}}
	c := NovoEm(cfg, srv.URL)

	dado := []byte("uma foto qualquer")
	soma := sha256.Sum256(dado)
	sha := hex.EncodeToString(soma[:])

	err := c.Enviar(context.Background(), Caminho("frota-macedo", sha, "jpg"),
		bytes.NewReader(dado), int64(len(dado)), sha, "image/jpeg")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if recebido.metodo != http.MethodPut {
		t.Errorf("método %s", recebido.metodo)
	}
	if !strings.HasPrefix(recebido.caminho, "/frotahub/clientes/frota-macedo/arquivos/") {
		t.Errorf("caminho %s", recebido.caminho)
	}
	if !strings.Contains(recebido.caminho, sha) {
		t.Error("o caminho tinha que conter o sha256 do conteúdo")
	}
	if recebido.sha != sha {
		t.Errorf("o cabeçalho x-amz-content-sha256 tinha que ser o sha do conteúdo")
	}
	if recebido.tamanho != int64(len(dado)) || !bytes.Equal(recebido.corpo, dado) {
		t.Error("o corpo chegou diferente")
	}
	if recebido.tipo != "image/jpeg" || recebido.autoriza == "" {
		t.Error("faltou tipo ou autorização")
	}
}

func TestArmazemDesligadoFalaClaro(t *testing.T) {
	c := Novo(&config.Config{})
	err := c.Enviar(context.Background(), "x", bytes.NewReader(nil), 0, strings.Repeat("0", 64), "")
	if err == nil || !strings.Contains(err.Error(), "não está configurado") {
		t.Fatalf("esperava recusa clara, veio: %v", err)
	}
}
