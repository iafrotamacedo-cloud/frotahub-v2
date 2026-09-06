package rogueworker

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
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
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

func (g *clienteGroq) ligado() bool { return g != nil && g.chave != "" }

const promptFallback = `Você é a Rogue Worker, assistente de dentro do FrotaHub (manutenção predial da Frota Macedo, cliente Mercadinhos São Luiz). Escopo desta versão: só o módulo Orçamentos.

Os comandos que o sistema já executa sozinho:
- pendencias: quantas notas/tickets estão pendentes
- status_nota: status de uma nota ou ticket (parametros.ticket ou parametros.id)
- extrapoladas: orçamentos/notas que passaram do teto
- bloqueadas: notas bloqueadas ou repetidas
- gerar: gerar orçamento de UMA nota (parametros.ticket ou id)
- gerar_lote: gerar orçamento de todas as notas da fila
- lancar: lançar UM orçamento no Trílogo
- lancar_lote: lançar todos da fila de lançamento
- pagamos_fornecedores: quanto pagamos / DAVs em aberto
- material_lancado: material lançado no contrato neste mês
- faturado: quanto já foi faturado / quanto falta faturar
- fechamento: fechamento do mês / relatório mensal
- navegar: levar o usuário a uma tela (parametros.tela: consolidacao, orcamentos, a-pagar, faturar, est-chamados, trilogo-dados, servicos-hub, funcionarios, minha-conta)
- desconhecido: não é nenhum desses

Responda SÓ com um objeto JSON, sem texto antes ou depois:
{"comando":"...","parametros":{"ticket":0,"id":"","tela":""},"formato_canonico":"frase curta canônica do pedido","resposta":"se comando=desconhecido, responda em português, curta, sem inventar número"}`

type interpretacaoGroq struct {
	Comando    string `json:"comando"`
	Parametros struct {
		Ticket int    `json:"ticket"`
		ID     string `json:"id"`
		Tela   string `json:"tela"`
	} `json:"parametros"`
	FormatoCanonico string `json:"formato_canonico"`
	Resposta        string `json:"resposta"`
}

func (g *clienteGroq) interpretar(ctx context.Context, pergunta string) (interpretacaoGroq, error) {
	corpo, err := json.Marshal(map[string]any{
		"model": g.modelo,
		"messages": []map[string]string{
			{"role": "system", "content": promptFallback},
			{"role": "user", "content": pergunta},
		},
		"temperature":     0,
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return interpretacaoGroq{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseGroq+"/chat/completions", bytes.NewReader(corpo))
	if err != nil {
		return interpretacaoGroq{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.chave)

	resp, err := g.http.Do(req)
	if err != nil {
		return interpretacaoGroq{}, fmt.Errorf("groq: %w", err)
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return interpretacaoGroq{}, fmt.Errorf("groq devolveu %d: %s", resp.StatusCode, primeiraLinha(bruto))
	}
	var fora struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bruto, &fora); err != nil || len(fora.Choices) == 0 {
		return interpretacaoGroq{}, fmt.Errorf("groq: resposta sem os choices esperados")
	}
	var r interpretacaoGroq
	if err := json.Unmarshal([]byte(fora.Choices[0].Message.Content), &r); err != nil {
		return interpretacaoGroq{}, fmt.Errorf("groq: o conteúdo não é o JSON esperado: %w", err)
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

func (m *Modulo) cairNoGroq(r *http.Request, p *seguranca.Principal, pergunta string) Resposta {
	if !m.groq.ligado() {
		return Resposta{Texto: "Não reconheci este pedido, e a conversa livre não está ligada neste servidor. Tente com uma das perguntas de orçamentos — pendências, teto, gerar, lançar, faturamento."}
	}
	interp, err := m.groq.interpretar(r.Context(), pergunta)
	if err != nil {
		return Resposta{Texto: "Não reconheci o pedido e não consegui consultar a conversa livre agora. Tente de outro jeito."}
	}
	comando := strings.TrimSpace(interp.Comando)
	if comando != "" && comando != cmdDesconhecido && comandoConhecido(comando) {
		rec := reconhecimento{
			comando: comando,
			ticket:  interp.Parametros.Ticket,
			id:      interp.Parametros.ID,
			tela:    interp.Parametros.Tela,
		}
		saida := m.executarComando(r, p, rec, pergunta)
		m.oferecerAprendizado(r, p, pergunta, rec, interp.FormatoCanonico, &saida)
		return saida
	}
	texto := strings.TrimSpace(interp.Resposta)
	if texto == "" {
		texto = "Não reconheci este pedido. Posso responder sobre pendências, notas, teto, gerar/lançar orçamento, faturamento e te levar até uma tela do menu."
	}
	saida := Resposta{Texto: texto + "\n\nIsso ajudou? posso lembrar da próxima vez?"}
	m.oferecerAprendizado(r, p, pergunta, reconhecimento{comando: cmdDesconhecido}, interp.FormatoCanonico, &saida)
	return saida
}

func comandoConhecido(c string) bool {
	switch c {
	case cmdPendencias, cmdStatusNota, cmdExtrapoladas, cmdBloqueadas,
		cmdGerar, cmdGerarLote, cmdLancar, cmdLancarLote,
		cmdPagamos, cmdMaterial, cmdFaturado, cmdFechamento, cmdNavegar:
		return true
	}
	return false
}
