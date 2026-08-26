// rev 1 — a tela de Notas e DAVs é uma fila, não um arquivo morto
//
//	A nota que já virou orçamento sai da vista padrão. Sem isso a lista cresce
//	para sempre: em dois anos seriam milhares de linhas resolvidas escondendo as
//	dez que precisam de gente.
//
//	O teste olha o FILTRO, e não o resultado, porque é no filtro que a proteção
//	mora — e o sintoma de ela sumir é mudo: a fila volta a mostrar tudo e
//	ninguém nota até estar com mil linhas.
package orcamentos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

func filtroDaVista(vista string) string {
	q := url.Values{}
	if vista != "" {
		q.Set("vista", vista)
	}
	return filtroDosDocumentos("11111111-1111-1111-1111-111111111111", "orcamento", q)
}

func TestAFilaNaoMostraOQueJaVirouOrcamento(t *testing.T) {
	f := filtroDaVista("")
	if !strings.Contains(f, "onde=eq.fila") {
		t.Errorf("a vista padrão não usa a regra de onde a nota mora: %q", f)
	}
	if !strings.Contains(f, "oculto_em=is.null") {
		t.Errorf("a vista padrão perdeu o filtro das tiradas da fila: %q", f)
	}
}

func TestAsProcessadasContinuamAlcancaveis(t *testing.T) {
	// Sair da fila não é sumir. Se não houvesse esta vista, "tirar da lista"
	// viraria "esconder para sempre" — e aí a queixa seria a oposta.
	f := filtroDaVista("processadas")
	if !strings.Contains(f, "onde=eq.usada") {
		t.Errorf("não dá para ver o que já virou orçamento: %q", f)
	}
}

func TestForaDaFilaMostraAsTiradas(t *testing.T) {
	f := filtroDaVista("fora")
	if !strings.Contains(f, "oculto_em=not.is.null") {
		t.Errorf("a vista de fora da fila não filtra pelas tiradas: %q", f)
	}
	// Aqui o status NÃO entra: tirada da fila é tirada, tenha virado orçamento
	// ou não. Filtrar por status faria a nota resolvida E tirada não aparecer em
	// vista nenhuma — sumida de verdade, que é o que este desenho evita.
	if strings.Contains(f, "onde=") {
		t.Errorf("a vista de fora da fila filtra por `onde` e esconde casos: %q", f)
	}
}

// A BUSCA VALE EM QUALQUER VISTA
//
//	Procurar pelo nome do arquivo é justamente o que a pessoa faz quando a nota
//	não está onde ela esperava.
func TestABuscaValeEmQualquerVista(t *testing.T) {
	for _, vista := range []string{"", "processadas", "fora"} {
		q := url.Values{"busca": {"19329"}}
		if vista != "" {
			q.Set("vista", vista)
		}
		f := filtroDosDocumentos("11111111-1111-1111-1111-111111111111", "orcamento", q)
		if !strings.Contains(f, "19329") {
			t.Errorf("vista %q perdeu a busca: %q", vista, f)
		}
	}
}

// O CONTADOR E A PRÉVIA CONTAM A MESMA COISA QUE A LISTA
//
//	A tela mostrava 31 e o botão dizia 77 — as 46 resolvidas entravam no número e
//	não na lista. Contador em que não se confia é pior que contador nenhum: quem
//	o vê passa a conferir tudo na mão.
func TestAPreviaDoPainelNaoMostraOQueJaFoi(t *testing.T) {
	var perguntou []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perguntou = append(perguntou, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	m := &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}

	m.previa(context.Background(), "documentos_lista?cliente_id=eq.x"+
		"&fila=eq.orcamento&oculto_em=is.null&onde=eq.fila"+
		"&order=inserido_em.desc&limit=9&select=nome_arquivo")

	if len(perguntou) != 1 || !strings.Contains(perguntou[0], "onde=eq.fila") {
		t.Errorf("a prévia não filtra o que já virou orçamento: %v", perguntou)
	}
}

// A REGRA DE ONDE A NOTA MORA É UMA SÓ
//
//	Antes, a lista, o contador e as telas de Correções perguntavam cada uma à sua
//	maneira "esta nota é minha?". Três respostas para a mesma pergunta foi como
//	nasceu a tela que mostrava 31 com o botão dizendo 77.
//
//	Agora quem responde é a coluna `onde` da visão. O que este teste protege é a
//	consulta CONTINUAR perguntando a ela, em vez de alguém reescrever as
//	condições aqui "para não depender da view".
func TestAListaPerguntaOndeAoInvesDeAdivinhar(t *testing.T) {
	f := filtroDaVista("")
	for _, proibido := range []string{"duplicada_de", "bloqueio_motivo", "ticket_soltos", "tickets="} {
		if strings.Contains(f, proibido) {
			t.Errorf("a lista voltou a repetir a regra (%s): %q", proibido, f)
		}
	}
	if !strings.Contains(f, "onde=eq.fila") {
		t.Errorf("a lista não pergunta onde a nota mora: %q", f)
	}
}
