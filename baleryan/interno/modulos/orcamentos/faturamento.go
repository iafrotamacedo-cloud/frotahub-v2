// rev 1 — o que ainda não foi cobrado do cliente, e como vira nota
//
// O LADO QUE FALTAVA
//
//	`orcamentos.faturado` e `orcamentos.pago` são do FORNECEDOR: a nota que
//	compramos, o dinheiro que saiu. Aqui é o outro lado — a planilha que vai ao
//	cliente e o dinheiro que volta. Enquanto os dois não existirem separados, a
//	pergunta "quanto o cliente me deve" não tem resposta.
//
// A FILA É UMA COLUNA VAZIA, NÃO UMA DATA
//
//	O que entra na próxima planilha é `fatura_id is null`. Nada de "orçamentos
//	de agosto": em julho a planilha fechou no dia 29 e a leva do dia 31 rolou
//	para o mês seguinte — se o critério fosse o mês, aqueles 39 orçamentos
//	(R$ 4.463,41) teriam sumido do faturamento para sempre.
//
//	Por isso o corte fica gravado no ciclo (`ate`), e não no código. Ele é o que
//	explica, daqui a um ano, por que um orçamento de 31/07 foi cobrado em agosto.
//
// O FECHAMENTO É REENTRANTE DE PROPÓSITO
//
//	Não há transação: o PostgREST faz uma chamada por vez, e são até 59 células.
//	Então a ordem foi escolhida para que uma queda no meio seja retomável —
//	criar o ciclo, criar as faturas, e só então carimbar os orçamentos, cada
//	passo ignorando o que já existe. Rodar de novo termina o serviço; não o
//	duplica.
package orcamentos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

type linhaAFaturar struct {
	ID        string   `json:"id"`
	Ticket    int      `json:"ticket"`
	Parte     int      `json:"parte"`
	UnidadeID *string  `json:"unidade_id"`
	Loja      *string  `json:"loja"`
	Conta     *string  `json:"conta"`
	Valor     *float64 `json:"valor"`
	CriadoEm  string   `json:"criado_em"`
	Status    string   `json:"status"`
}

// Celula é um par loja×conta: exatamente uma nota do cliente.
//
// "Célula" é a palavra dele. O cliente monta um PCO por conta/loja/período —
// até 34×2 = 68 por mês — e célula vazia não gera fatura. Em julho foram 54.
type Celula struct {
	UnidadeID  string  `json:"unidade_id"`
	Loja       string  `json:"loja"`
	Conta      string  `json:"conta"`
	Orcamentos int     `json:"orcamentos"`
	Valor      float64 `json:"valor"`
}

// agruparPorLojaEConta é separado da leitura para poder ser provado sem banco.
// Ele decide quantas notas o cliente vai emitir — errar aqui é cobrar a loja
// errada, e isso sai de casa.
func agruparPorLojaEConta(linhas []linhaAFaturar) []Celula {
	tipo := map[string]*Celula{}
	for _, l := range linhas {
		// Sem loja ou sem conta não há nota possível: não existe PCO para
		// "ninguém". Some da contagem de células e aparece como pendência.
		if l.UnidadeID == nil || l.Conta == nil || *l.UnidadeID == "" || *l.Conta == "" {
			continue
		}
		chave := *l.UnidadeID + "|" + *l.Conta
		c, existe := tipo[chave]
		if !existe {
			c = &Celula{UnidadeID: *l.UnidadeID, Loja: texto(l.Loja), Conta: *l.Conta}
			tipo[chave] = c
		}
		c.Orcamentos++
		if l.Valor != nil {
			c.Valor += *l.Valor
		}
	}
	saida := make([]Celula, 0, len(tipo))
	for _, c := range tipo {
		saida = append(saida, *c)
	}
	// Loja e depois conta: é a ordem em que ele confere contra os PCOs.
	sort.SliceStable(saida, func(i, j int) bool {
		if saida[i].Loja != saida[j].Loja {
			return saida[i].Loja < saida[j].Loja
		}
		return saida[i].Conta < saida[j].Conta
	})
	return saida
}

// ordenarParaOCliente devolve a mesma ordem da planilha de julho: por data, e
// dentro do dia por chamado. Mudar a ordem de um documento que o cliente já
// conferiu uma vez é criar trabalho para ele sem motivo.
func ordenarParaOCliente(linhas []linhaAFaturar) []linhaAFaturar {
	saida := append([]linhaAFaturar(nil), linhas...)
	sort.SliceStable(saida, func(i, j int) bool {
		if saida[i].CriadoEm != saida[j].CriadoEm {
			return saida[i].CriadoEm < saida[j].CriadoEm
		}
		if saida[i].Ticket != saida[j].Ticket {
			return saida[i].Ticket < saida[j].Ticket
		}
		return saida[i].Parte < saida[j].Parte
	})
	return saida
}

func somarAFaturar(linhas []linhaAFaturar) (valor float64, gerados int, semDestino int) {
	for _, l := range linhas {
		if l.Valor != nil {
			valor += *l.Valor
		}
		if l.Status == "gerado" {
			gerados++
		}
		if l.UnidadeID == nil || l.Conta == nil || *l.UnidadeID == "" || *l.Conta == "" {
			semDestino++
		}
	}
	return
}

// aFaturar lê a fila. `ate` vazio = tudo que ainda não foi cobrado.
func (m *Modulo) aFaturar(ctx context.Context, clienteID, ate string) ([]linhaAFaturar, error) {
	caminho := "orcamentos_lista?cliente_id=eq." + banco.Escapar(clienteID) +
		"&fatura_id=is.null&status=neq.removido" +
		"&order=criado_em,ticket,parte&limit=" + fmt.Sprint(TetoDaExtracao) + "&select=*"
	if ate != "" {
		caminho += "&criado_em=lt." + banco.Escapar(ate)
	}
	var linhas []linhaAFaturar
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	return linhas, nil
}

// competenciaDeHoje é o mês corrente em Fortaleza — não em UTC, que às 21h já
// virou amanhã (e no dia 31 já virou o mês seguinte).
func competenciaDeHoje() string {
	return time.Now().In(fusoDaCasa()).Format("2006-01")
}

// GET /orcamentos/faturamento
func (m *Modulo) faturamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaFaturar)
	if p == nil {
		return
	}
	linhas, err := m.aFaturar(r.Context(), p.ClienteID, "")
	if err != nil {
		m.erro(w, "não consegui montar o faturamento", err)
		return
	}
	celulas := agruparPorLojaEConta(linhas)
	valor, gerados, semDestino := somarAFaturar(linhas)

	desde, ateData := "", ""
	if len(linhas) > 0 {
		ordenadas := ordenarParaOCliente(linhas)
		desde = ordenadas[0].CriadoEm
		ateData = ordenadas[len(ordenadas)-1].CriadoEm
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"competencia": competenciaDeHoje(),
		"orcamentos":  len(linhas),
		"valor":       valor,
		"celulas":     celulas,
		"faturas":     len(celulas),
		// Quantos ainda não subiram para o Trílogo. Não impede faturar — em
		// julho dois orçamentos sem custo lá foram cobrados e pagos — mas quem
		// assina a planilha merece saber.
		"nao_lancados": gerados,
		// Sem loja ou sem conta não existe PCO possível. Estes NÃO entram em
		// nenhuma célula, e por isso precisam aparecer sozinhos.
		"sem_destino": semDestino,
		"desde":       desde,
		"ate":         ateData,
	})
}

// As colunas que o cliente recebe. As cinco primeiras são as da planilha de
// julho, na mesma ordem. "Conta" é a única novidade — ele abre um PCO por
// conta e loja, e sem essa coluna a separação era trabalho dele.
var colunasDoCliente = []relatorio.Coluna{
	{Titulo: "Nº", Peso: 0.6, Tipo: relatorio.Numero},
	{Titulo: "Ticket", Peso: 1, Tipo: relatorio.Numero},
	{Titulo: "Loja", Peso: 2.6, Tipo: relatorio.Texto},
	{Titulo: "Conta", Peso: 1.2, Tipo: relatorio.Texto},
	{Titulo: "Valor total", Peso: 1.3, Tipo: relatorio.Dinheiro},
	{Titulo: "Data", Peso: 1.1, Tipo: relatorio.Data},
}

func (m *Modulo) tabelaDoFaturamento(w http.ResponseWriter, r *http.Request) (relatorio.Tabela, bool) {
	p := m.quem(w, r, RotinaFaturar)
	if p == nil {
		return relatorio.Tabela{}, false
	}
	linhas, err := m.aFaturar(r.Context(), p.ClienteID, "")
	if err != nil {
		m.erro(w, "não consegui montar a planilha do cliente", err)
		return relatorio.Tabela{}, false
	}
	linhas = ordenarParaOCliente(linhas)

	var soma float64
	corpo := make([][]any, 0, len(linhas))
	for i, l := range linhas {
		if l.Valor != nil {
			soma += *l.Valor
		}
		corpo = append(corpo, []any{
			i + 1, l.Ticket, texto(l.Loja), contaPorExtenso(texto(l.Conta)),
			l.Valor, instante(&l.CriadoEm),
		})
	}

	aviso := ""
	if len(linhas) >= TetoDaExtracao {
		aviso = "A lista bateu no teto da extração e foi cortada."
	}
	return relatorio.Tabela{
		Titulo: "Orçamentos montados — " + competenciaDeHoje(),
		// A guia com o mesmo nome da planilha de julho. O cliente abre este
		// arquivo do lado do anterior.
		Aba:       "Orçamentos",
		Subtitulo: fmt.Sprintf("%d orçamentos · %d faturas · R$ %.2f", len(linhas), len(agruparPorLojaEConta(linhas)), soma),
		Colunas:   colunasDoCliente,
		Linhas:    corpo,
		Aviso:     aviso,
		Gerado:    time.Now(),
	}, true
}

// GET /orcamentos/faturamento.xlsx — o arquivo que vai para o cliente
func (m *Modulo) faturamentoExcel(w http.ResponseWriter, r *http.Request) {
	tab, ok := m.tabelaDoFaturamento(w, r)
	if !ok {
		return
	}
	corpo, err := tab.Planilha()
	if err != nil {
		m.erro(w, "não consegui montar a planilha", err)
		return
	}
	entregar(w, corpo, "orcamentos-montados-"+competenciaDeHoje(), "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

// GET /orcamentos/faturamento.pdf
func (m *Modulo) faturamentoPDF(w http.ResponseWriter, r *http.Request) {
	tab, ok := m.tabelaDoFaturamento(w, r)
	if !ok {
		return
	}
	corpo, err := tab.PDF()
	if err != nil {
		m.erro(w, "não consegui montar o PDF", err)
		return
	}
	entregar(w, corpo, "orcamentos-montados-"+competenciaDeHoje(), "pdf", "application/pdf")
}

// POST /orcamentos/faturamento/fechar  {competencia}
//
// Fecha o mês: cria o ciclo com o corte de AGORA, abre uma fatura por célula e
// carimba os orçamentos. Depois disto eles somem da fila — é o que impede
// cobrar o mesmo orçamento duas vezes.
func (m *Modulo) fecharFaturamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaFaturar)
	if p == nil {
		return
	}
	var pedido struct {
		Competencia string `json:"competencia"`
		Observacao  string `json:"observacao"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi o pedido.")
		return
	}
	competencia := strings.TrimSpace(pedido.Competencia)
	if competencia == "" {
		competencia = competenciaDeHoje()
	}
	if !competenciaValida(competencia) {
		web.Falhar(w, http.StatusBadRequest, "A competência tem que ser no formato AAAA-MM.")
		return
	}
	ctx := r.Context()

	// 1. O ciclo. Se já existe (retomada de um fechamento que caiu no meio),
	//    o corte dele é o que vale — inventar um novo mudaria quem entra.
	ciclo, err := m.cicloDaCompetencia(ctx, p.ClienteID, competencia)
	if err != nil {
		m.erro(w, "não consegui abrir o ciclo", err)
		return
	}
	if ciclo == nil {
		agora := time.Now().UTC().Format(time.RFC3339)
		linha := map[string]any{
			"cliente_id":  p.ClienteID,
			"competencia": competencia,
			"ate":         agora,
			"fechado_em":  agora,
			"criado_por":  p.UserID,
		}
		if o := strings.TrimSpace(pedido.Observacao); o != "" {
			linha["observacao"] = o
		}
		if err := m.bd.Inserir(ctx, "faturamento_ciclos", []map[string]any{linha}, nil); err != nil &&
			!banco.Duplicado(err) {
			m.erro(w, "não consegui abrir o ciclo", err)
			return
		}
		if ciclo, err = m.cicloDaCompetencia(ctx, p.ClienteID, competencia); err != nil || ciclo == nil {
			m.erro(w, "não consegui reler o ciclo que acabei de abrir", err)
			return
		}
	}

	// 2. As células dentro do corte.
	linhas, err := m.aFaturar(ctx, p.ClienteID, ciclo.Ate)
	if err != nil {
		m.erro(w, "não consegui ler a fila", err)
		return
	}
	celulas := agruparPorLojaEConta(linhas)
	if len(celulas) == 0 {
		web.Falhar(w, http.StatusConflict,
			"Não há nada para faturar: todo orçamento desta janela já está numa fatura.")
		return
	}

	// 3. Uma fatura por célula. Ignora as que já existirem.
	novas := make([]map[string]any, 0, len(celulas))
	for _, c := range celulas {
		novas = append(novas, map[string]any{
			"cliente_id": p.ClienteID,
			"ciclo_id":   ciclo.ID,
			"unidade_id": c.UnidadeID,
			"conta":      c.Conta,
		})
	}
	if err := m.bd.InserirIgnorando(ctx, "faturas?on_conflict=ciclo_id,unidade_id,conta", novas); err != nil {
		m.erro(w, "não consegui abrir as faturas", err)
		return
	}

	// 4. Reler para saber o id de cada uma — inclusive das que já existiam.
	var faturas []struct {
		ID        string `json:"id"`
		UnidadeID string `json:"unidade_id"`
		Conta     string `json:"conta"`
	}
	if err := m.bd.Buscar(ctx, "faturas?ciclo_id=eq."+banco.Escapar(ciclo.ID)+
		"&limit=500&select=id,unidade_id,conta", &faturas); err != nil {
		m.erro(w, "não consegui reler as faturas", err)
		return
	}

	// 5. O carimbo, fatura por fatura. Só toca em quem ainda não tem uma.
	carimbados := 0
	for _, f := range faturas {
		filtro := "cliente_id=eq." + banco.Escapar(p.ClienteID) +
			"&unidade_id=eq." + banco.Escapar(f.UnidadeID) +
			"&conta=eq." + banco.Escapar(f.Conta) +
			"&fatura_id=is.null&status=neq.removido" +
			"&criado_em=lt." + banco.Escapar(ciclo.Ate)
		if err := m.bd.Atualizar(ctx, "orcamentos", filtro, map[string]any{"fatura_id": f.ID}); err != nil {
			// Parar aqui deixaria o resto para a próxima chamada, que é
			// justamente o que o desenho reentrante permite. Mas quem clicou
			// precisa saber que ficou pela metade.
			m.erro(w, fmt.Sprintf("carimbei %d de %d faturas e parei", carimbados, len(faturas)), err)
			return
		}
		carimbados++
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"ok":          true,
		"competencia": competencia,
		"ciclo_id":    ciclo.ID,
		"corte":       ciclo.Ate,
		"faturas":     len(faturas),
		"orcamentos":  len(linhas),
	})
}

type cicloDeFaturamento struct {
	ID          string `json:"id"`
	Competencia string `json:"competencia"`
	Ate         string `json:"ate"`
	EnviadoEm   string `json:"enviado_em"`
	FechadoEm   string `json:"fechado_em"`
}

func (m *Modulo) cicloDaCompetencia(ctx context.Context, clienteID, competencia string) (*cicloDeFaturamento, error) {
	var linhas []cicloDeFaturamento
	err := m.bd.Buscar(ctx, "faturamento_ciclos?cliente_id=eq."+banco.Escapar(clienteID)+
		"&competencia=eq."+banco.Escapar(competencia)+"&limit=1&select=*", &linhas)
	if err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		return nil, nil
	}
	return &linhas[0], nil
}

// competenciaValida é AAAA-MM, com mês entre 01 e 12. Uma competência torta
// vira um ciclo que ninguém acha depois.
func competenciaValida(s string) bool {
	if len(s) != 7 || s[4] != '-' {
		return false
	}
	_, err := time.Parse("2006-01", s)
	return err == nil
}

// GET /orcamentos/faturas?competencia=AAAA-MM
//
// O controle: o que foi cobrado, o que voltou, e a diferença entre os dois.
func (m *Modulo) listarFaturas(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaFaturar)
	if p == nil {
		return
	}
	caminho := "faturas_lista?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&order=competencia.desc,loja,conta&limit=1000&select=*"
	if c := strings.TrimSpace(r.URL.Query().Get("competencia")); c != "" {
		if !competenciaValida(c) {
			web.Falhar(w, http.StatusBadRequest, "A competência tem que ser no formato AAAA-MM.")
			return
		}
		caminho += "&competencia=eq." + banco.Escapar(c)
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui ler as faturas", err)
		return
	}
	if linhas == nil {
		linhas = []map[string]any{}
	}
	var faturado, recebido float64
	for _, l := range linhas {
		faturado += numeroDe(l["valor"])
		recebido += numeroDe(l["valor_recebido"])
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"faturas":   linhas,
		"faturado":  faturado,
		"recebido":  recebido,
		"a_receber": faturado - recebido,
	})
}

// POST /orcamentos/faturas/{id}  — anota PCO, nota e recebimento
//
// Campo ausente não é campo vazio: só o que vier no corpo é gravado. É o que
// permite marcar o recebimento sem reescrever o número da nota.
func (m *Modulo) anotarFatura(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaFaturar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusNotFound, "Não achei esta fatura.")
		return
	}
	var pedido struct {
		PCO           *string  `json:"pco_numero"`
		NF            *string  `json:"nf_numero"`
		NFEm          *string  `json:"nf_em"`
		RecebidoEm    *string  `json:"recebido_em"`
		ValorRecebido *float64 `json:"valor_recebido"`
		Observacao    *string  `json:"observacao"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi o pedido.")
		return
	}
	campos := map[string]any{}
	guardarTexto(campos, "pco_numero", pedido.PCO)
	guardarTexto(campos, "nf_numero", pedido.NF)
	guardarData(campos, "nf_em", pedido.NFEm)
	guardarData(campos, "recebido_em", pedido.RecebidoEm)
	guardarTexto(campos, "observacao", pedido.Observacao)
	if pedido.ValorRecebido != nil {
		campos["valor_recebido"] = *pedido.ValorRecebido
	}
	if len(campos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Não veio nada para anotar.")
		return
	}

	// O cliente_id no filtro não é decoração: sem ele, um id adivinhado de
	// outro cliente seria alterável por esta rota.
	filtro := "id=eq." + banco.Escapar(id) + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "faturas", filtro, campos); err != nil {
		m.erro(w, "não consegui anotar a fatura", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// Texto vazio apaga o campo; ausente não mexe nele.
func guardarTexto(campos map[string]any, nome string, v *string) {
	if v == nil {
		return
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		campos[nome] = nil
		return
	}
	campos[nome] = s
}

// A data chega AAAA-MM-DD. Qualquer outra coisa é recusada em silêncio em vez
// de virar um texto estranho numa coluna `date`.
func guardarData(campos map[string]any, nome string, v *string) {
	if v == nil {
		return
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		campos[nome] = nil
		return
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return
	}
	campos[nome] = s
}
