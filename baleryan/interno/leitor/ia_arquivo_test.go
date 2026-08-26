// rev 1 — o arquivo indo inteiro para o modelo, e o ritmo que o segura
package leitor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// servidorDeMentira devolve sempre a mesma leitura e guarda o que recebeu.
func servidorDeMentira(t *testing.T, guardar func(pedidoIA)) *IA {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		var p pedidoIA
		if err := json.Unmarshal(corpo, &p); err != nil {
			t.Errorf("mandei um JSON que o próprio teste não lê: %v", err)
		}
		if guardar != nil {
			guardar(p)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":
			"{\"tipo\":\"nf\",\"numero\":\"000.009.160\",\"valor_total\":100,\"itens\":[]}"}]}}]}`))
	}))
	t.Cleanup(s.Close)
	ia := NovaIA("chave-de-mentira")
	ia.Base = s.URL
	ia.Intervalo = 0 // o ritmo tem teste próprio; aqui ele só atrapalharia
	return ia
}

// O ARQUIVO TEM QUE CHEGAR COMO ARQUIVO
//
//	Se ele virar texto no caminho, voltamos ao defeito que tirou o OCR do
//	sistema: o modelo recebendo uma fita achatada em vez da página.
func TestLerArquivoMandaOsBytesEOTipo(t *testing.T) {
	var visto pedidoIA
	ia := servidorDeMentira(t, func(p pedidoIA) { visto = p })

	bytesDoPNG := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3}
	if _, err := ia.LerArquivo(context.Background(), "image/png", bytesDoPNG); err != nil {
		t.Fatalf("LerArquivo falhou: %v", err)
	}

	if len(visto.Contents) != 1 {
		t.Fatalf("mandei %d conteúdos, esperava 1", len(visto.Contents))
	}
	var comArquivo *dadosEmLinha
	for _, p := range visto.Contents[0].Parts {
		if p.Inline != nil {
			comArquivo = p.Inline
		}
	}
	if comArquivo == nil {
		t.Fatal("o pedido foi só de texto — o arquivo não chegou ao modelo")
	}
	if comArquivo.Tipo != "image/png" {
		t.Errorf("declarei o tipo %q, e o arquivo era image/png", comArquivo.Tipo)
	}
	if comArquivo.Dados == "" {
		t.Error("o arquivo chegou vazio")
	}
}

// A leitura da IA passa por Arrumar como qualquer outra.
func TestLerArquivoNormalizaONumero(t *testing.T) {
	ia := servidorDeMentira(t, nil)
	l, err := ia.LerArquivo(context.Background(), "application/pdf", []byte("%PDF-1.4"))
	if err != nil {
		t.Fatalf("LerArquivo falhou: %v", err)
	}
	if l.Numero != "9160" {
		t.Errorf("a IA devolveu \"000.009.160\" e ficou gravado %q", l.Numero)
	}
}

// Arquivo grande é recusado ANTES de virar base64 — que engorda um terço.
func TestLerArquivoRecusaOQueNaoCabe(t *testing.T) {
	ia := servidorDeMentira(t, nil)
	gigante := make([]byte, TamanhoMaximoEmLinha+1)
	if _, err := ia.LerArquivo(context.Background(), "image/jpeg", gigante); err == nil {
		t.Error("mandou um arquivo acima do limite sem reclamar")
	}
}

// O RITMO É O QUE IMPEDE O ROBÔ DE SE AFOGAR
//
//	Sem espera, uma fila de oitenta notas gasta a cota do minuto em segundos e
//	as setenta restantes falham em sequência — cada uma com três tentativas,
//	cada tentativa queimando mais cota.
func TestChamadasSeguidasRespeitamOIntervalo(t *testing.T) {
	ia := servidorDeMentira(t, nil)
	ia.Intervalo = 120 * time.Millisecond

	comeco := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := ia.Ler(context.Background(), "qualquer texto"); err != nil {
			t.Fatalf("chamada %d falhou: %v", i+1, err)
		}
	}
	// Três chamadas = duas esperas. A primeira sai na hora.
	if passou := time.Since(comeco); passou < 240*time.Millisecond {
		t.Errorf("três chamadas levaram %v; com intervalo de 120ms teriam que levar 240ms ou mais", passou)
	}
}

// A primeira chamada não espera: fila que começa devagar é minuto de Actions
// pago para não fazer nada.
func TestAPrimeiraChamadaNaoEspera(t *testing.T) {
	ia := servidorDeMentira(t, nil)
	ia.Intervalo = 2 * time.Second

	comeco := time.Now()
	if _, err := ia.Ler(context.Background(), "qualquer texto"); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if passou := time.Since(comeco); passou > time.Second {
		t.Errorf("a primeira chamada esperou %v sem ter ninguém na frente", passou)
	}
}

// Espera é espera, mas não é sequestro: se o robô foi mandado parar, ele para.
func TestAEsperaObedeceOCancelamento(t *testing.T) {
	ia := servidorDeMentira(t, nil)
	ia.Intervalo = 10 * time.Second
	if _, err := ia.Ler(context.Background(), "primeira"); err != nil {
		t.Fatalf("falhou: %v", err)
	}

	ctx, parar := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer parar()
	comeco := time.Now()
	if _, err := ia.Ler(ctx, "segunda"); err == nil {
		t.Error("o contexto morreu e a chamada seguiu em frente")
	}
	if passou := time.Since(comeco); passou > 2*time.Second {
		t.Errorf("ficou %v presa na espera depois de o contexto morrer", passou)
	}
}

// ---------------------------------------------------------------------------
// sobrecarga do outro lado
// ---------------------------------------------------------------------------

// iaQueFalhaAntes recusa as `quantas` primeiras chamadas com `status`, e aceita
// a seguinte. Conta quantas vezes foi procurada.
func iaQueFalhaAntes(t *testing.T, quantas int, status int, recado string, voltas *int) *IA {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*voltas++
		w.Header().Set("Content-Type", "application/json")
		if *voltas <= quantas {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"` + recado + `"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":
			"{\"tipo\":\"nf\",\"numero\":\"1\",\"valor_total\":10,\"itens\":[]}"}]}}]}`))
	}))
	t.Cleanup(s.Close)
	ia := NovaIA("chave-de-mentira")
	ia.Base = s.URL
	ia.Intervalo = 0
	return ia
}

// SOBRECARGA NÃO É DEFEITO DA NOTA
//
//	Sem insistir, o robô queimava uma das três tentativas da nota e a mandava
//	para o fim da fila, onde ela reencontraria a mesma sobrecarga. Três voltas
//	depois ela ficava marcada como "falhou" — sem ter nada de errado.
func TestInsisteQuandoAIAEstaSobrecarregada(t *testing.T) {
	var voltas int
	ia := iaQueFalhaAntes(t, 2, http.StatusServiceUnavailable,
		"This model is currently experiencing high demand.", &voltas)

	if _, err := ia.Ler(context.Background(), "texto"); err != nil {
		t.Fatalf("desistiu de uma sobrecarga passageira: %v", err)
	}
	if voltas != 3 {
		t.Errorf("bateu %d vezes na porta, esperava 3 (duas recusas e o acerto)", voltas)
	}
}

func TestInsisteTambemNoLimitePorMinuto(t *testing.T) {
	var voltas int
	ia := iaQueFalhaAntes(t, 1, http.StatusTooManyRequests, "Resource exhausted", &voltas)
	if _, err := ia.Ler(context.Background(), "texto"); err != nil {
		t.Fatalf("desistiu de um 429, que é exatamente o caso de esperar: %v", err)
	}
}

// RECUSA DE VERDADE NÃO MELHORA COM INSISTÊNCIA
//
//	Modelo que não existe, arquivo que não serve: repetir só gastaria cota para
//	ouvir o mesmo "não". Foi o caso do gemini-2.5-flash aposentado.
func TestNaoInsisteQuandoARecusaEDefinitiva(t *testing.T) {
	var voltas int
	ia := iaQueFalhaAntes(t, 2, http.StatusBadRequest,
		"This model is no longer available to new users.", &voltas)

	if _, err := ia.Ler(context.Background(), "texto"); err == nil {
		t.Fatal("insistiu até dar certo numa recusa que nunca ia mudar")
	}
	if voltas != 1 {
		t.Errorf("bateu %d vezes numa porta que respondeu 'não existe' — esperava 1", voltas)
	}
}

// E se a sobrecarga não passar, ela vira erro — com o recado original.
func TestDesisteDepoisDasTentativas(t *testing.T) {
	var voltas int
	ia := iaQueFalhaAntes(t, 99, http.StatusServiceUnavailable, "high demand", &voltas)
	if _, err := ia.Ler(context.Background(), "texto"); err == nil {
		t.Fatal("a sobrecarga não passou e mesmo assim deu certo")
	}
	if voltas != TentativasNaSobrecarga {
		t.Errorf("insistiu %d vezes, e o teto é %d", voltas, TentativasNaSobrecarga)
	}
}
