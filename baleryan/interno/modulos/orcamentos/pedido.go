// rev 1 — o pedido de faturamento ao fornecedor
//
// O LADO DE CÁ DO BALCÃO
//
//	`faturamento.go` trata do que a gente COBRA do cliente. Este arquivo trata
//	do que a gente DEVE ao fornecedor — e o mecanismo dele é diferente do nosso.
//
//	A Rodrigues não emite nota a cada compra: ela emite um DAV (documento
//	auxiliar de venda, do SysPDV) e vai acumulando. De tempos em tempos nós
//	mandamos a relação das DAVs em aberto, e ela emite UMA nota fiscal cobrindo
//	todas. Sem esse pedido, o DAV fica solto e ninguém cobra ninguém.
//
// SE É DAV, É DELA
//
//	Decisão do dono em 26/08/2026, e ela evita um erro caro. Filtrar por
//	emitente pareceria mais rigoroso — mas três das onze DAVs no banco entraram
//	antes de o leitor ler emitente, e ficariam de fora do pedido sem ninguém
//	perceber. São R$ 601,90 que a Rodrigues não cobraria e que a gente
//	descobriria devendo meses depois.
//
//	DAV é o formato do sistema DELA. Se aparecer um segundo fornecedor de DAV
//	um dia, aí sim vale um cadastro com CNPJ — e aí a mudança é aqui, num lugar
//	só.
package orcamentos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// RotinaPagar protege o menu A pagar.
const RotinaPagar = "CONTRATO_FINANCEIRO_PAGAR"

type davEmAberto struct {
	ID       string   `json:"id"`
	Numero   *string  `json:"numero"`
	DAV      *string  `json:"dav_numero"`
	Emissao  *string  `json:"emissao"`
	Valor    *float64 `json:"valor_total"`
	Nome     string   `json:"nome_arquivo"`
	Inserido string   `json:"inserido_em"`
}

// GET /orcamentos/pedido — as DAVs em aberto até a data escolhida.
//
// O CORTE É POR EMISSÃO, E NÃO POR INSERÇÃO
//
//	Quem confere do outro lado é a Rodrigues, e o que ela tem é a data em que
//	emitiu o DAV. Cortar pela data em que NÓS inserimos no sistema produziria
//	uma relação que não bate com a lista dela — e a conversa viraria "esse aqui
//	é de agosto" contra "não, entrou em setembro".
func (m *Modulo) pedidoEmAberto(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaPagar)
	if p == nil {
		return
	}
	ate := strings.TrimSpace(r.URL.Query().Get("ate"))
	davs, err := m.davsEmAberto(r.Context(), p.ClienteID, ate)
	if err != nil {
		m.erro(w, "não consegui listar as DAVs em aberto", err)
		return
	}
	var soma float64
	semData := 0
	for _, d := range davs {
		if d.Valor != nil {
			soma += *d.Valor
		}
		if d.Emissao == nil || *d.Emissao == "" {
			semData++
		}
	}

	var pedidos []map[string]any
	_ = m.bd.Buscar(r.Context(), "pedidos_faturamento_lista?cliente_id=eq."+
		banco.Escapar(p.ClienteID)+"&order=numero.desc&limit=24&select=*", &pedidos)

	web.Responder(w, http.StatusOK, map[string]any{
		"davs":     ouVaziaDeDAV(davs),
		"quantas":  len(davs),
		"valor":    soma,
		"sem_data": semData,
		"ate":      ate,
		"pedidos":  ouVazio(pedidos),
	})
}

func ouVaziaDeDAV(v []davEmAberto) []davEmAberto {
	if v == nil {
		return []davEmAberto{}
	}
	return v
}

func (m *Modulo) davsEmAberto(ctx context.Context, clienteID, ate string) ([]davEmAberto, error) {
	caminho := "documentos?cliente_id=eq." + banco.Escapar(clienteID) +
		"&tipo=eq.dav&pedido_id=is.null&oculto_em=is.null" +
		"&order=emissao,dav_numero&limit=" + fmt.Sprint(TetoDaExtracao) +
		"&select=id,numero,dav_numero,emissao,valor_total,nome_arquivo,inserido_em"
	if ate != "" {
		// DAV SEM DATA DE EMISSÃO NÃO PODE SUMIR DO CORTE
		//   Ela existe, foi comprada e é devida. Se o filtro a excluísse por
		//   não ter data, ela ficaria invisível para sempre — nunca entraria em
		//   pedido nenhum, porque todo pedido tem corte.
		caminho += "&or=(emissao.lte." + banco.Escapar(ate) + ",emissao.is.null)"
	}
	var davs []davEmAberto
	if err := m.bd.Buscar(ctx, caminho, &davs); err != nil {
		return nil, err
	}
	return davs, nil
}

// ---------------------------------------------------------------------------
// a relação que vai para a Rodrigues
// ---------------------------------------------------------------------------

var colunasDoPedido = []relatorio.Coluna{
	{Titulo: "Nº", Peso: 0.6, Tipo: relatorio.Numero},
	{Titulo: "DAV", Peso: 1.2, Tipo: relatorio.Texto},
	{Titulo: "Emissão", Peso: 1.1, Tipo: relatorio.Data},
	{Titulo: "Valor", Peso: 1.4, Tipo: relatorio.Dinheiro},
	{Titulo: "Arquivo", Peso: 3, Tipo: relatorio.Texto},
}

func (m *Modulo) tabelaDoPedido(w http.ResponseWriter, r *http.Request) (relatorio.Tabela, bool) {
	p := m.quem(w, r, RotinaPagar)
	if p == nil {
		return relatorio.Tabela{}, false
	}
	ate := strings.TrimSpace(r.URL.Query().Get("ate"))
	davs, err := m.davsEmAberto(r.Context(), p.ClienteID, ate)
	if err != nil {
		m.erro(w, "não consegui montar a relação", err)
		return relatorio.Tabela{}, false
	}

	var soma float64
	corpo := make([][]any, 0, len(davs))
	for i, d := range davs {
		if d.Valor != nil {
			soma += *d.Valor
		}
		corpo = append(corpo, []any{
			i + 1, numeroDaDAV(d), texto(d.Emissao), d.Valor, d.Nome,
		})
	}

	subtitulo := fmt.Sprintf("%d DAVs · R$ %.2f", len(davs), soma)
	if ate != "" {
		subtitulo += " · emitidas até " + emDiaMesAno(ate)
	}

	return relatorio.Tabela{
		Titulo:    "Pedido de faturamento",
		Aba:       "DAVs em aberto",
		Subtitulo: subtitulo,
		Colunas:   colunasDoPedido,
		Linhas:    corpo,
		Aviso:     avisoDoPedido(davs),
		Gerado:    time.Now(),
	}, true
}

// avisoDoPedido conta o que a relação NÃO diz sozinha.
//
// Uma DAV sem número ou sem data continua na lista — ela é devida do mesmo
// jeito. Mas quem receber a relação vai reparar, e é melhor que a explicação
// esteja no papel do que numa ligação depois.
func avisoDoPedido(davs []davEmAberto) string {
	semNumero, semData := 0, 0
	for _, d := range davs {
		if numeroDaDAV(d) == "—" {
			semNumero++
		}
		if d.Emissao == nil || *d.Emissao == "" {
			semData++
		}
	}
	partes := []string{}
	if semNumero > 0 {
		partes = append(partes, fmt.Sprintf("%d sem número de DAV legível", semNumero))
	}
	if semData > 0 {
		partes = append(partes, fmt.Sprintf("%d sem data de emissão", semData))
	}
	if len(partes) == 0 {
		return ""
	}
	return "Nesta relação: " + strings.Join(partes, " e ") + ". Confira o arquivo original."
}

func numeroDaDAV(d davEmAberto) string {
	if d.DAV != nil && *d.DAV != "" {
		return *d.DAV
	}
	if d.Numero != nil && *d.Numero != "" {
		return *d.Numero
	}
	return "—"
}

func emDiaMesAno(iso string) string {
	if len(iso) < 10 {
		return iso
	}
	return iso[8:10] + "/" + iso[5:7] + "/" + iso[0:4]
}

// GET /orcamentos/pedido.xlsx
func (m *Modulo) pedidoExcel(w http.ResponseWriter, r *http.Request) {
	tab, ok := m.tabelaDoPedido(w, r)
	if !ok {
		return
	}
	corpo, err := tab.Planilha()
	if err != nil {
		m.erro(w, "não consegui montar a planilha", err)
		return
	}
	entregar(w, corpo, "pedido-de-faturamento", "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

// GET /orcamentos/pedido.pdf
func (m *Modulo) pedidoPDF(w http.ResponseWriter, r *http.Request) {
	tab, ok := m.tabelaDoPedido(w, r)
	if !ok {
		return
	}
	corpo, err := tab.PDF()
	if err != nil {
		m.erro(w, "não consegui montar o PDF", err)
		return
	}
	entregar(w, corpo, "pedido-de-faturamento", "pdf", "application/pdf")
}

// ---------------------------------------------------------------------------
// fechar
// ---------------------------------------------------------------------------

// POST /orcamentos/pedido/fechar
//
// FECHAR É O QUE TIRA AS DAVs DA FILA
//
//	Antes disso, a tela é só uma consulta: dá para gerar a relação quantas vezes
//	quiser sem mexer em nada. Depois, aquelas DAVs têm dono — e não entram no
//	pedido da semana que vem.
//
//	Sem esse passo, a mesma DAV entraria em dois pedidos e a Rodrigues emitiria
//	duas notas cobrando o mesmo material. É o espelho exato do que a 017 evita
//	do lado do cliente.
func (m *Modulo) fecharPedido(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaPagar)
	if p == nil {
		return
	}
	var pedido struct {
		Ate        string `json:"ate"`
		Observacao string `json:"observacao"`
	}
	corpo, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if len(corpo) > 0 {
		_ = json.Unmarshal(corpo, &pedido)
	}
	ate := strings.TrimSpace(pedido.Ate)
	if ate == "" {
		ate = time.Now().In(fusoDaCasa()).Format("2006-01-02")
	}
	if !dataValida(ate) {
		web.Falhar(w, http.StatusBadRequest, "A data do corte tem que ser AAAA-MM-DD.")
		return
	}

	davs, err := m.davsEmAberto(r.Context(), p.ClienteID, ate)
	if err != nil {
		m.erro(w, "não consegui ler as DAVs em aberto", err)
		return
	}
	if len(davs) == 0 {
		web.Falhar(w, http.StatusConflict,
			"Não há DAV em aberto até esta data. Não vou criar um pedido vazio.")
		return
	}

	numero, err := m.proximoNumeroDePedido(r.Context(), p.ClienteID)
	if err != nil {
		m.erro(w, "não consegui numerar o pedido", err)
		return
	}

	var criados []map[string]any
	if err := m.bd.Inserir(r.Context(), "pedidos_faturamento", []map[string]any{{
		"cliente_id": p.ClienteID,
		"numero":     numero,
		"ate":        ate,
		"fechado_em": time.Now().UTC().Format(time.RFC3339),
		"observacao": ouNulo(strings.TrimSpace(pedido.Observacao)),
		"criado_por": p.UserID,
	}}, &criados); err != nil {
		m.erro(w, "não consegui criar o pedido", err)
		return
	}
	if len(criados) == 0 {
		m.erro(w, "criei o pedido mas o banco não devolveu o id", nil)
		return
	}
	pedidoID := fmtID(criados[0]["id"])

	// A MARCA VAI COM O MESMO FILTRO QUE MONTOU A LISTA
	//
	//	E `pedido_id=is.null` continua no filtro de propósito: se outra pessoa
	//	fechou um pedido nos segundos entre a leitura e esta gravação, as DAVs
	//	dela não são roubadas para este.
	filtro := "cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&tipo=eq.dav&pedido_id=is.null&oculto_em=is.null" +
		"&or=(emissao.lte." + banco.Escapar(ate) + ",emissao.is.null)"
	var marcadas []map[string]any
	if err := m.bd.AtualizarDevolvendo(r.Context(), "documentos", filtro,
		map[string]any{"pedido_id": pedidoID}, &marcadas); err != nil {
		m.erro(w, "criei o pedido mas não consegui marcar as DAVs", err)
		return
	}

	_ = m.hist.Registrar(r.Context(), p, "orcamentos", pedidoID, "fechar_pedido_faturamento",
		map[string]historico.Mudanca{"davs": {De: nil, Para: len(marcadas)}})

	web.Responder(w, http.StatusOK, map[string]any{
		"ok": true, "pedido": pedidoID, "numero": numero,
		"davs": len(marcadas), "ate": ate,
	})
}

func (m *Modulo) proximoNumeroDePedido(ctx context.Context, clienteID string) (int, error) {
	var ultimos []struct {
		Numero int `json:"numero"`
	}
	if err := m.bd.Buscar(ctx, "pedidos_faturamento?cliente_id=eq."+banco.Escapar(clienteID)+
		"&order=numero.desc&limit=1&select=numero", &ultimos); err != nil {
		return 0, err
	}
	if len(ultimos) == 0 {
		return 1, nil
	}
	return ultimos[0].Numero + 1, nil
}

// POST /orcamentos/pedido/{id}/reabrir — devolve as DAVs para a fila.
//
// PORQUE FECHAR CEDO DEMAIS ACONTECE
//
//	Alguém fecha o pedido, e aí aparece uma DAV que estava na mesa. Sem
//	reabertura, a saída seria mexer no banco à mão — e mexer no banco à mão é
//	como uma DAV acaba em dois pedidos.
func (m *Modulo) reabrirPedido(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaPagar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	dono, err := m.contarUm(r.Context(), "pedidos_faturamento?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,enviado_em&limit=1")
	if err != nil {
		m.erro(w, "não achei este pedido", err)
		return
	}
	if dono["enviado_em"] != nil {
		web.Falhar(w, http.StatusConflict,
			"Este pedido já foi marcado como enviado ao fornecedor. Reabrir agora deixaria a relação que ele tem em mãos diferente da nossa.")
		return
	}

	if err := m.bd.Atualizar(r.Context(), "documentos", "pedido_id=eq."+id,
		map[string]any{"pedido_id": nil}); err != nil {
		m.erro(w, "não consegui devolver as DAVs para a fila", err)
		return
	}
	if err := m.bd.Apagar(r.Context(), "pedidos_faturamento", "id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)); err != nil {
		m.erro(w, "devolvi as DAVs mas não consegui apagar o pedido", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "reabrir_pedido_faturamento", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /orcamentos/pedido/{id}/enviado — carimba que a relação foi mandada.
func (m *Modulo) marcarPedidoEnviado(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaPagar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	filtro := "id=eq." + id + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "pedidos_faturamento", filtro, map[string]any{
		"enviado_em": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		m.erro(w, "não consegui marcar como enviado", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "pedido_enviado", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

func dataValida(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
