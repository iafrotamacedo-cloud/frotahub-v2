// rev 1 — a nota que já virou orçamento naquele ticket
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	`orcamento_documentos` tem índice único em (documento_id, ticket). Até
//	26/08/2026 a geração não perguntava por ele: só esbarrava. O banco devolvia
//	409 DEPOIS que o orçamento já tinha nascido, e sobrava um registro sem
//	vínculo — vivo na lista, órfão na tabela.
//
//	Perguntar antes é a correção. Estes testes existem para que a pergunta não
//	desapareça numa edição distraída, porque o sintoma dela sumir é caro e
//	silencioso.
package orcamentos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

// moduloQueResponde devolve um corpo por tabela e guarda tudo que foi
// perguntado. Tabela sem resposta combinada devolve `[]`, como o PostgREST
// faria com um filtro que não acha nada.
func moduloQueResponde(t *testing.T, corpos map[string]string, perguntas *[]string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tabela := strings.TrimPrefix(r.URL.Path, "/rest/v1/")
		if i := strings.Index(tabela, "?"); i >= 0 {
			tabela = tabela[:i]
		}
		if perguntas != nil {
			*perguntas = append(*perguntas, r.Method+" "+tabela+"?"+r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		if corpo, ok := corpos[tabela]; ok {
			_, _ = w.Write([]byte(corpo))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

func alguem() *seguranca.Principal {
	return &seguranca.Principal{
		UserID: "11111111-1111-1111-1111-111111111111", Usuario: "teste",
		ClienteID: "22222222-2222-2222-2222-222222222222", Ativo: true,
	}
}

// A NOTA JÁ ORÇADA NÃO TENTA DE NOVO
//
//	Sem esta recusa, a geração ia até o fim, criava o orçamento e só então o
//	banco recusava o vínculo — deixando o órfão.
func TestNotaJaOrcadaNaoGeraDeNovo(t *testing.T) {
	m := moduloQueResponde(t, map[string]string{
		"documento_tickets":    `[{"ticket":131382,"chamado_id":"33333333-3333-3333-3333-333333333333"}]`,
		"orcamento_documentos": `[{"ticket":131382}]`,
	}, nil)

	saida := m.gerarDeUmDocumento(context.Background(), alguem(),
		documentoPronto{ID: "d1", Nome: "nota.jpeg", Valor: 490.80}, regras.Padrao)

	if len(saida) != 1 {
		t.Fatalf("devolveu %d resultados, esperava 1", len(saida))
	}
	if saida[0].Erro == "" {
		t.Fatal("gerou de novo uma nota que já virou orçamento neste ticket")
	}
	if saida[0].Orcamento != "" {
		t.Errorf("chegou a criar orçamento antes de recusar: %s", saida[0].Orcamento)
	}
	if !strings.Contains(saida[0].Erro, "já virou orçamento") {
		t.Errorf("a recusa não explica o que houve: %q", saida[0].Erro)
	}
}

// UMA NOTA RATEADA NÃO SE COMPLETA PELA METADE
//
//	O valor de cada parte depende de QUANTOS tickets a nota atende. Gerar só os
//	que faltam daria partes que não conversam com as que já existem — e cada uma
//	fecharia a própria conta, parecendo certa. É o defeito mais caro que existe:
//	silencioso e plausível.
func TestNotaOrcadaPelaMetadeNaoCompletaSozinha(t *testing.T) {
	m := moduloQueResponde(t, map[string]string{
		"documento_tickets": `[{"ticket":130708,"chamado_id":"33333333-3333-3333-3333-333333333333"},
		                       {"ticket":130720,"chamado_id":"44444444-4444-4444-4444-444444444444"}]`,
		"orcamento_documentos": `[{"ticket":130708}]`,
	}, nil)

	saida := m.gerarDeUmDocumento(context.Background(), alguem(),
		documentoPronto{ID: "d1", Nome: "nota.pdf", Valor: 300}, regras.Padrao)

	if saida[0].Erro == "" {
		t.Fatal("completou pela metade uma nota rateada")
	}
	for _, n := range []string{"130708", "130720"} {
		if !strings.Contains(saida[0].Erro, n) {
			t.Errorf("a recusa não diz qual ticket é qual — falta %s em %q", n, saida[0].Erro)
		}
	}
}

// O VÍNCULO MARCADO NÃO CONTA
//
//	Apagar um orçamento marca o vínculo (`removido_em`) em vez de apagá-lo, para
//	restaurar continuar sabendo de qual nota ele veio. Se a consulta esquecer o
//	filtro, a nota apagada continua "já orçada" e nunca mais gera nada — que é
//	exatamente o defeito que a 025 foi escrita para tirar.
func TestOVinculoSoltoNaoImpedeNovaGeracao(t *testing.T) {
	var perguntas []string
	m := moduloQueResponde(t, nil, &perguntas)

	if _, err := m.ticketsJaOrcados(context.Background(), "d1"); err != nil {
		t.Fatalf("erro: %v", err)
	}
	var achou string
	for _, p := range perguntas {
		if strings.Contains(p, "orcamento_documentos") {
			achou = p
		}
	}
	if achou == "" {
		t.Fatal("não perguntou nada a orcamento_documentos")
	}
	if !strings.Contains(achou, "removido_em=is.null") {
		t.Errorf("a consulta conta vínculos já soltos: %q", achou)
	}
}

func TestSemOsJaOrcadosTiraSoOQueJaFoi(t *testing.T) {
	todos := []ticketDoDocumento{{Ticket: 1}, {Ticket: 2}, {Ticket: 3}}
	faltam := semOsJaOrcados(todos, []int{2})
	if len(faltam) != 2 || faltam[0].Ticket != 1 || faltam[1].Ticket != 3 {
		t.Errorf("sobrou %v, esperava os tickets 1 e 3", numerosDe(faltam))
	}
	if n := len(semOsJaOrcados(todos, nil)); n != 3 {
		t.Errorf("sem nada orçado sobraram %d, esperava 3", n)
	}
	if n := len(semOsJaOrcados(todos, []int{1, 2, 3})); n != 0 {
		t.Errorf("com tudo orçado sobraram %d, esperava 0", n)
	}
}
