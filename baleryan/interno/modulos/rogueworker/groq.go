package rogueworker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
		http:   &http.Client{Timeout: 45 * time.Second},
	}
}

func (g *clienteGroq) ligado() bool { return g != nil && g.chave != "" }

const promptFallback = `Você é a Rogue Worker, assistente de dentro do FrotaHub (manutenção predial da Frota Macedo, cliente Mercadinhos São Luiz). Você responde sobre QUALQUER módulo que o login alcance: orçamentos, chamados/Trílogo, estatísticas, serviços, consolidação, funcionários/SESMT, usuários.

Os comandos que o sistema já executa sozinho (use um destes quando casar):
- pendencias, status_nota, extrapoladas, bloqueadas
- gerar, gerar_lote, lancar, lancar_lote (ações: o motor confirma antes)
- pagamos_fornecedores, material_lancado, faturado, fechamento
- chamados_atendidos
- navegar (parametros.tela: consolidacao, orcamentos, a-pagar, faturar, est-chamados, trilogo-dados, servicos-hub, funcionarios, minha-conta)
- desconhecido: não é um comando acima — aí preencha fontes

fontes (1 a 4, só leitura, GET que o sistema já tem):
orcamentos_painel, orcamentos_pendencias, orcamentos_pedido, orcamentos_faturamento, orcamentos_fechamento,
servicos_painel, servicos_lista, servicos_kanban,
trilogo_chamados, trilogo_chamado,
estatisticas_resumo, consolidacao,
funcionarios_conformidade, funcionarios_vencendo, funcionarios_lista,
robos_trilogo, usuarios

Responda SÓ com um objeto JSON:
{"comando":"...","fontes":["..."],"parametros":{"ticket":0,"id":"","tela":""},"formato_canonico":"frase curta canônica","resposta":""}`

const promptNarrar = `Você é a Rogue Worker. Responda em português, curto (no máximo 8 frases), só com o que está em DADOS. Não invente número, nome, status nem data. Se DADOS não cobre a pergunta, diga o que falta e ofereça ir à tela certa. Sem markdown, sem lista de fontes.

Responda SÓ com JSON: {"resposta":"..."}`

type interpretacaoGroq struct {
	Comando    string   `json:"comando"`
	Fontes     []string `json:"fontes"`
	Parametros struct {
		Ticket int    `json:"ticket"`
		ID     string `json:"id"`
		Tela   string `json:"tela"`
	} `json:"parametros"`
	FormatoCanonico string `json:"formato_canonico"`
	Resposta        string `json:"resposta"`
}

func (g *clienteGroq) interpretar(ctx context.Context, pergunta string) (interpretacaoGroq, error) {
	return g.jsonChat(ctx, promptFallback, pergunta)
}

func (g *clienteGroq) narrar(ctx context.Context, pergunta string, dados map[string]any) (string, error) {
	bruto, err := json.Marshal(dados)
	if err != nil {
		return "", err
	}
	if len(bruto) > tetoDoRetrato {
		bruto = bruto[:tetoDoRetrato]
	}
	user := "PERGUNTA:\n" + pergunta + "\n\nDADOS:\n" + string(bruto)
	interp, err := g.jsonChat(ctx, promptNarrar, user)
	if err != nil {
		return "", err
	}
	texto := strings.TrimSpace(interp.Resposta)
	if texto == "" || pareceQueNaoEntendeu(texto) {
		return "", fmt.Errorf("groq: narração vazia")
	}
	return texto, nil
}

func (g *clienteGroq) jsonChat(ctx context.Context, sistema, usuario string) (interpretacaoGroq, error) {
	corpo, err := json.Marshal(map[string]any{
		"model": g.modelo,
		"messages": []map[string]string{
			{"role": "system", "content": sistema},
			{"role": "user", "content": usuario},
		},
		"temperature":     0,
		"response_format": map[string]string{"type": "json_object"},
		// gpt-oss é modelo de raciocínio: sem isto ele gasta os tokens de
		// saída "pensando" e devolve conteúdo vazio — confirmado em teste
		// real (08/09/2026), Groq responde 400 json_validate_failed com
		// failed_generation vazio em prompt não trivial.
		"reasoning_effort":      "low",
		"max_completion_tokens": 4000,
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
	conteudo := strings.TrimSpace(fora.Choices[0].Message.Content)
	conteudo = strings.TrimPrefix(conteudo, "```json")
	conteudo = strings.TrimPrefix(conteudo, "```")
	conteudo = strings.TrimSuffix(conteudo, "```")
	conteudo = strings.TrimSpace(conteudo)
	var r interpretacaoGroq
	if err := json.Unmarshal([]byte(conteudo), &r); err != nil {
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
		return Resposta{Texto: "Não reconheci este pedido, e a conversa livre não está ligada neste servidor. Tente de outro jeito, ou avise o responsável pela chave do Groq."}
	}
	interp, err := m.groq.interpretar(r.Context(), pergunta)
	if err != nil {
		log.Printf("rogueworker: groq falhou (%s): %v", m.groq.modelo, err)
		return m.consultarComDados(r, p, pergunta, interpretacaoGroq{})
	}
	comando := strings.TrimSpace(interp.Comando)
	if comando != "" && comando != cmdDesconhecido && comandoConhecido(comando) {
		if comando == cmdNavegar && telaPorNome(interp.Parametros.Tela) == nil {
			return m.consultarComDados(r, p, pergunta, interp)
		}
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
	return m.consultarComDados(r, p, pergunta, interp)
}

func (m *Modulo) consultarComDados(r *http.Request, p *seguranca.Principal, pergunta string, interp interpretacaoGroq) Resposta {
	ids := juntarFontes(interp.Fontes, inferirFontes(pergunta))
	if len(ids) == 0 {
		ids = fontesDoRetratoGeral()
	}
	dados := m.buscarFontes(r, p, pergunta, ids)
	texto, err := m.groq.narrar(r.Context(), pergunta, dados)
	if err != nil {
		log.Printf("rogueworker: groq não narrou: %v", err)
		texto = textoSemModelo(dados)
	}
	saida := Resposta{Texto: texto}
	m.oferecerAprendizado(r, p, pergunta, reconhecimento{comando: cmdDesconhecido}, interp.FormatoCanonico, &saida)
	return saida
}

func textoSemModelo(dados map[string]any) string {
	ok := 0
	for _, v := range dados {
		m, eh := v.(map[string]any)
		if !eh {
			ok++
			continue
		}
		if _, tem := m["erro"]; tem && len(m) == 1 {
			continue
		}
		ok++
	}
	if ok == 0 {
		return "Não achei isso com o que este login alcança, ou o sistema não devolveu o dado agora."
	}
	return "Consultei o sistema, mas não consegui redigir a resposta agora. Pergunte de novo em instantes, ou abra a tela no menu."
}

func comandoConhecido(c string) bool {
	switch c {
	case cmdPendencias, cmdStatusNota, cmdExtrapoladas, cmdBloqueadas,
		cmdGerar, cmdGerarLote, cmdLancar, cmdLancarLote,
		cmdPagamos, cmdMaterial, cmdFaturado, cmdFechamento,
		cmdChamadosAtendidos, cmdNavegar:
		return true
	}
	return false
}
