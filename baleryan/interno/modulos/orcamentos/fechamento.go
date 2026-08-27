// rev 1 — o fechamento: as duas contas, e a prova de que elas fecham
//
// A PERGUNTA QUE ESTA TELA RESPONDE
//
//	"tem 93 não lançados, 23 de pendência, 68 parados. nada bate com nada."
//
//	Os três números estavam certos. Eram respostas para três perguntas
//	diferentes, e nenhuma tela mostrava as três juntas — então a única coisa que
//	dava para concluir, olhando de fora, era que o sistema não fechava.
//
//	93 = a fila inteira · 68 = travados por status de chamado · 23 = já tentados
//	e recusados. E 93 = 68 + 22 + 3.
//
// POR QUE SÃO DUAS CONTAS, E NÃO UMA
//
//	A conta que o dono escreveu misturava as duas unidades — notas de um lado,
//	orçamentos do outro — e por isso nunca fecharia: uma nota vira VÁRIOS
//	orçamentos (as partes), e um orçamento de rateio nasce de VÁRIAS notas.
//
//	O que fecha, e é o que ele quer de verdade, são duas contas ligadas por uma
//	ponte: toda nota em um destino, todo orçamento em um estado, e nada perdido
//	entre uma coisa e outra.
//
// ESTA ROTA NÃO CALCULA NADA
//
//	As três views fazem a conta. Se o motor somasse de novo aqui, teríamos duas
//	respostas para a mesma pergunta — que é o defeito que este arquivo existe
//	para acabar (CORE-06). Ele lê, junta e devolve.
package orcamentos

import (
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

type linhaDoFechamento struct {
	Ordem   int     `json:"ordem"`
	Chave   string  `json:"chave"`
	Rotulo  string  `json:"rotulo"`
	Quantos int     `json:"quantos"`
	Valor   float64 `json:"valor"`
}

// GET /orcamentos/fechamento
func (m *Modulo) fechamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaModulo)
	if p == nil {
		return
	}
	cli := banco.Escapar(p.ClienteID)

	var notas []struct {
		Ordem   int     `json:"ordem"`
		Destino string  `json:"destino"`
		Rotulo  string  `json:"rotulo"`
		Quantas int     `json:"quantas"`
		Valor   float64 `json:"valor"`
	}
	if err := m.bd.Buscar(r.Context(),
		"fechamento_notas?cliente_id=eq."+cli+"&order=ordem&select=*", &notas); err != nil {
		m.erro(w, "não consegui ler o fechamento das notas", err)
		return
	}

	var orcs []struct {
		Ordem   int     `json:"ordem"`
		Estado  string  `json:"estado"`
		Rotulo  string  `json:"rotulo"`
		Quantos int     `json:"quantos"`
		Valor   float64 `json:"valor"`
	}
	if err := m.bd.Buscar(r.Context(),
		"fechamento_orcamentos?cliente_id=eq."+cli+"&order=ordem&select=*", &orcs); err != nil {
		m.erro(w, "não consegui ler o fechamento dos orçamentos", err)
		return
	}

	ponte, err := m.contarUm(r.Context(),
		"fechamento_ponte?cliente_id=eq."+cli+"&select=*&limit=1")
	if err != nil {
		// A ponte é conferência, não conteúdo: sem ela as duas contas continuam
		// legíveis, e derrubar a tela inteira por causa dela seria trocar o
		// principal pelo acessório.
		ponte = map[string]any{}
	}

	linhasN := make([]linhaDoFechamento, 0, len(notas))
	var totalN int
	var somaN float64
	for _, l := range notas {
		linhasN = append(linhasN, linhaDoFechamento{l.Ordem, l.Destino, l.Rotulo, l.Quantas, l.Valor})
		totalN += l.Quantas
		somaN += l.Valor
	}
	linhasO := make([]linhaDoFechamento, 0, len(orcs))
	var totalO int
	var somaO float64
	for _, l := range orcs {
		linhasO = append(linhasO, linhaDoFechamento{l.Ordem, l.Estado, l.Rotulo, l.Quantos, l.Valor})
		totalO += l.Quantos
		somaO += l.Valor
	}

	// O UNIVERSO VEM DA TABELA, NÃO DA SOMA DAS LINHAS
	//
	//	Se ele viesse da soma, a conta fecharia sempre — inclusive quando alguma
	//	nota escapasse da classificação. Contando a tabela por fora, a diferença
	//	entre os dois números é justamente o que ninguém viu.
	var vazio []map[string]any
	universoN, _ := m.bd.BuscarContando(r.Context(),
		"documentos?cliente_id=eq."+cli+"&select=id&limit=1", &vazio)
	universoO, _ := m.bd.BuscarContando(r.Context(),
		"orcamentos?cliente_id=eq."+cli+"&select=id&limit=1", &vazio)

	web.Responder(w, http.StatusOK, map[string]any{
		"notas": map[string]any{
			"linhas":    linhasN,
			"total":     totalN,
			"valor":     somaN,
			"universo":  universoN,
			"diferenca": universoN - totalN,
		},
		"orcamentos": map[string]any{
			"linhas":    linhasO,
			"total":     totalO,
			"valor":     somaO,
			"universo":  universoO,
			"diferenca": universoO - totalO,
		},
		"ponte": ponte,
	})
}
