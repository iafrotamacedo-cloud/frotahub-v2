// rev 1 — o PDF do orçamento
//
// NÃO PRECISA MAIS SAIR EM WORD
//
//	Decisão do dono em 25/08/2026. No sistema antigo o Word era o original e o
//	PDF uma conversão; cada orçamento nascia duas vezes, em dois formatos que
//	podiam divergir. Agora nasce um só.
//
// REAPROVEITA O `relatorio`
//
//	O gerador de PDF do FrotaHub já existe, escrito à mão, sem dependência
//	externa, com teste. Escrever um segundo para o orçamento seria ter dois
//	geradores de PDF divergindo com o tempo — que é o mesmo defeito do Word.
//
//	O documento é uma tabela de itens com um cabeçalho e uma linha de TOTAL
//	GERAL. É o que o orçamento sempre foi.
//
// A MARGEM NÃO APARECE
//
//	O documento mostra o preço cobrado, e só. De onde ele veio é assunto nosso —
//	é assim desde o sistema antigo, e mudar isso agora seria mudar o que o
//	cliente vê.
package orcamentos

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

var colunasDoOrcamento = []relatorio.Coluna{
	{Titulo: "Item", Peso: 0.5, Tipo: relatorio.Numero},
	{Titulo: "Descrição", Peso: 6, Tipo: relatorio.Texto},
	{Titulo: "Un.", Peso: 0.7, Tipo: relatorio.Texto},
	{Titulo: "Qtd.", Peso: 0.9, Tipo: relatorio.Numero},
	{Titulo: "Valor unit.", Peso: 1.3, Tipo: relatorio.Dinheiro},
	{Titulo: "Valor total", Peso: 1.4, Tipo: relatorio.Dinheiro},
}

// montarPDF arma o documento do orçamento.
func (m *Modulo) montarPDF(ctx context.Context, clienteID, orcamentoID string) ([]byte, error) {
	cabeca, err := m.contarUm(ctx, "orcamentos_lista?id=eq."+orcamentoID+
		"&cliente_id=eq."+banco.Escapar(clienteID)+"&select=*&limit=1")
	if err != nil {
		return nil, err
	}

	var itens []struct {
		Ordem      int     `json:"ordem"`
		Descricao  string  `json:"descricao"`
		Unidade    *string `json:"unidade"`
		Quantidade float64 `json:"quantidade"`
		Cobrado    float64 `json:"valor_unitario_cobrado"`
		Total      float64 `json:"valor_total"`
	}
	if err := m.bd.Buscar(ctx, "orcamento_itens?orcamento_id=eq."+orcamentoID+
		"&order=ordem&select=ordem,descricao,unidade,quantidade,valor_unitario_cobrado,valor_total",
		&itens); err != nil {
		return nil, err
	}
	if len(itens) == 0 {
		return nil, fmt.Errorf("este orçamento não tem itens")
	}

	linhas := make([][]any, 0, len(itens)+2)
	var soma regras.Dinheiro
	for _, it := range itens {
		un := ""
		if it.Unidade != nil {
			un = *it.Unidade
		}
		soma += regras.DinheiroDe(it.Total)
		linhas = append(linhas, []any{
			it.Ordem, it.Descricao, un, it.Quantidade, it.Cobrado, it.Total,
		})
	}
	// Linha em branco e o total. A soma vem dos ITENS, não do campo `valor` do
	// orçamento — se os dois divergirem, o documento mostra a divergência em vez
	// de escondê-la, e alguém conserta.
	//
	// As células vazias vão como `nil`, e não como string vazia: o gerador
	// escreve travessão onde o texto está vazio (que é o certo para uma célula
	// sem dado), mas aqui a célula não tem dado NENHUM — é separador. Com string
	// vazia, a linha em branco saía com dois travessões soltos no meio do
	// documento.
	linhas = append(linhas, []any{nil, nil, nil, nil, nil, nil})
	linhas = append(linhas, []any{nil, "TOTAL GERAL", nil, nil, nil, soma.Float()})

	ticket := fmt.Sprint(cabeca["ticket"])
	parte := fmt.Sprint(cabeca["parte"])
	titulo := "ORÇAMENTO " + ticket
	if parte != "1" {
		titulo += "-" + parte
	}

	sub := []string{}
	if loja, ok := cabeca["loja"].(string); ok && loja != "" {
		sub = append(sub, loja)
	}
	sub = append(sub, "Ticket "+ticket)
	if notas, ok := cabeca["notas"].(string); ok && notas != "" {
		sub = append(sub, "Nota "+notas)
	}
	if d, ok := cabeca["chamado_descricao"].(string); ok && d != "" {
		sub = append(sub, encurtar(d, 90))
	}

	tab := relatorio.Tabela{
		Titulo:    titulo,
		Subtitulo: strings.Join(sub, " · "),
		Colunas:   colunasDoOrcamento,
		Linhas:    linhas,
		Gerado:    time.Now().In(fusoDaCasa()),
	}
	// Um orçamento reduzido pelo teto diz isso no próprio documento. Corte que
	// não aparece é corte que ninguém consegue explicar depois.
	if reduzido, _ := cabeca["reduzido_pelo_teto"].(bool); reduzido {
		antes := regras.DinheiroDe(numeroDe(cabeca["valor_antes_do_teto"]))
		tab.Aviso = "Valor ajustado ao limite contratado (de R$ " + antes.Reais() + ")."
	}
	return tab.PDF()
}

func encurtar(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}

// GET /orcamentos/{id}/pdf — o documento, para conferir antes de lançar.
func (m *Modulo) pdfDoOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	pdf, err := m.montarPDF(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não consegui montar o PDF", err)
		return
	}
	entregar(w, pdf, "orcamento", "pdf", "application/pdf")
}

// entregar manda o arquivo com o nome certo.
//
// O Content-Disposition só chega ao navegador porque `web.CORS` o expõe. Sem
// aquela linha, o arquivo baixa com o nome da rota — e ninguém entende por quê.
func entregar(w http.ResponseWriter, corpo []byte, base, extensao, tipo string) {
	nome := base + "-" + time.Now().In(fusoDaCasa()).Format("2006-01-02-1504") + "." + extensao
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`"`)
	w.Header().Set("Content-Length", fmt.Sprint(len(corpo)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(corpo)
}
