// rev 1 — o classificador de Candidatos, por API
//
// PADRÃO OpenAI, NÃO O DO GEMINI
//
//	`leitor/ia.go` fala com o Gemini (`x-goog-api-key` no header, corpo
//	`contents`/`systemInstruction`). O Groq segue outro contrato — o mesmo
//	formato "chat completions" da OpenAI: `Authorization: Bearer <chave>`,
//	corpo `{model, messages:[{role,content}]}`. São provedores diferentes,
//	de propósito (ver config.Groq) — não dá pra copiar ia.go linha a linha.
//
// SEM RETRY, DE PROPÓSITO (por enquanto)
//
//	O tier grátis do Groq é folgado (14.400 req/dia contra um teto nosso de
//	200/rodada) e o job roda em lote, uma vez a cada 2h: um chamado que
//	falhar hoje é reavaliado na PRÓXIMA rodada — a fila de "ainda não
//	classificado" (servicos_a_classificar) não esquece ninguém. Adicionar
//	retry aqui seria complexidade sem um problema real por trás.
package servicos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

var baseGroq = "https://api.groq.com/openai/v1"

type clienteGroq struct {
	chave  string
	modelo string
	http   *http.Client
}

func novoClienteGroq(cfg config.Groq) *clienteGroq {
	return &clienteGroq{
		chave:  cfg.Chave,
		modelo: cfg.Modelo,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *clienteGroq) ligado() bool { return g.chave != "" }

// promptCandidato explica, uma vez só, o critério que separa Serviço de
// Contrato — validado com o dono em 30 exemplos pareados em 03/09/2026:
// TODOS bateram com "conserto do que já existe = Contrato; instalar, criar
// ou ampliar algo que não existia = Serviço", sem exceção.
const promptCandidato = `Você classifica chamados de manutenção predial da Frota Macedo Engenharia, cliente Mercadinhos São Luiz.

Critério (confirmado com 30 exemplos reais, sem exceção):
- CONTRATO: conserto de algo que já existe e quebrou ou parou de funcionar. Volta o que já estava lá ao estado normal.
- SERVIÇO: instalar, construir, criar ou ampliar algo que NÃO existia antes — mesmo que pareça pequeno, mesmo que o texto não use a palavra "instalação" ou "serviço".

O cliente é leigo e raramente escreve "serviço" ou "instalação" explicitamente — julgue pela NATUREZA do pedido (conserta o que já existia, ou traz algo novo?), não pela palavra usada.

Responda SÓ com um objeto JSON, sem nenhum texto antes ou depois:
{"candidato": true ou false, "motivo": "até 12 palavras explicando por quê"}`

type respostaCandidato struct {
	Candidato bool   `json:"candidato"`
	Motivo    string `json:"motivo"`
}

// Classificar julga UM chamado pela descrição. Erro de rede ou de formato
// não trava a rodada inteira — ver candidatos.go, RodarClassificacao: o
// chamado que falhar aqui simplesmente não é decidido nesta rodada, e volta
// a aparecer em servicos_a_classificar na próxima.
func (g *clienteGroq) Classificar(ctx context.Context, descricao string) (respostaCandidato, error) {
	corpo, err := json.Marshal(map[string]any{
		"model": g.modelo,
		"messages": []map[string]string{
			{"role": "system", "content": promptCandidato},
			{"role": "user", "content": descricao},
		},
		"temperature":     0,
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return respostaCandidato{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseGroq+"/chat/completions", bytes.NewReader(corpo))
	if err != nil {
		return respostaCandidato{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.chave)

	resp, err := g.http.Do(req)
	if err != nil {
		return respostaCandidato{}, fmt.Errorf("groq: %w", err)
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return respostaCandidato{}, fmt.Errorf("groq devolveu %d: %s", resp.StatusCode, primeiraLinha(bruto))
	}

	var fora struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bruto, &fora); err != nil || len(fora.Choices) == 0 {
		return respostaCandidato{}, fmt.Errorf("groq: resposta sem os choices esperados")
	}

	var r respostaCandidato
	if err := json.Unmarshal([]byte(fora.Choices[0].Message.Content), &r); err != nil {
		return respostaCandidato{}, fmt.Errorf("groq: o conteúdo não é o JSON esperado: %w", err)
	}
	return r, nil
}

func primeiraLinha(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
