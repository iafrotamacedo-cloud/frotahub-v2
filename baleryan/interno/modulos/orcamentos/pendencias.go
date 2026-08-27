// rev 1 — as pendências: quem tem que agir para o orçamento poder subir
//
// SÃO DUAS LISTAS, E ELAS VÃO PARA PESSOAS DIFERENTES
//
//	encarregados  o chamado está Aberto ou Em execução — é serviço NOSSO a
//	              terminar. Enquanto ele não for marcado como executado, o
//	              Trílogo não aceita o custo.
//
//	cliente       o chamado está Arquivado, ou foi Reaberto. Nenhuma das duas
//	              coisas nós destravamos: só o cliente reabre um arquivado, e um
//	              reaberto é decisão dele.
//
// A LISTA É DE TICKET, NÃO DE ORÇAMENTO
//
//	Um ticket com três orçamentos parados vai ao cliente como UMA linha, com a
//	soma. Mandar três faria ele perguntar por que três — e a resposta ("são
//	partes") é detalhe nosso, não problema dele.
//
// O DESTINO NÃO É GRAVADO, É CALCULADO
//
//	Ele sai da view `orcamentos_lista`, do status do ticket AGORA. Medimos 7 de
//	26 bloqueados destravando sozinhos em seis dias — e hoje mesmo o ticket
//	131768 saiu de "pronto para lançar" para "cliente" entre uma medição e a
//	seguinte, porque o cliente reabriu dizendo "não foi resolvido". Gravado, o
//	destino envelheceria mentindo.
//
//	O que É gravado é o aviso: `ticket_avisos` diz o que já foi cobrado e quando.
//	É o que separa "já mandei e não veio" de "nunca mandei".
package orcamentos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// Os dois destinos que geram lista. `pode_lancar` e `sem_chamado` não geram:
// o primeiro não é pendência de ninguém, e o segundo é pendência do robô.
var listasDePendencia = map[string]string{
	"cliente":      "Chamados que só o cliente destrava",
	"encarregados": "Chamados que a nossa equipe precisa concluir",
}

type linhaDePendencia struct {
	ID               string   `json:"id"`
	Ticket           int      `json:"ticket"`
	Parte            int      `json:"parte"`
	Loja             *string  `json:"loja"`
	Conta            *string  `json:"conta"`
	Valor            *float64 `json:"valor"`
	CriadoEm         string   `json:"criado_em"`
	TicketStatus     *string  `json:"ticket_status"`
	Reaberto         *bool    `json:"reaberto"`
	MotivoReabertura *string  `json:"motivo_reabertura"`
	ChamadoDescricao *string  `json:"chamado_descricao"`
	AvisadoEm        *string  `json:"avisado_em"`
	Destino          *string  `json:"destino"`
	Bloqueio         *string  `json:"lancamento_bloqueio"`
	BloqueioDetalhe  *string  `json:"lancamento_bloqueio_detalhe"`
	TentadoEm        *string  `json:"lancamento_tentado_em"`
	Tentativas       int      `json:"lancamento_tentativas"`
}

// Pendencia é um TICKET, com os orçamentos dele dentro.
type Pendencia struct {
	Ticket int `json:"ticket"`
	// UM ORÇAMENTO DAQUELE TICKET — qualquer um serve, e isso não é desleixo.
	//
	//	O status é do CHAMADO, não do orçamento: reconferir um destrava todos os
	//	do mesmo ticket. A tela precisa de um id para chamar a rota, e mandar os
	//	três de um ticket de três partes seriam três idas ao Trílogo para
	//	responder a mesma pergunta.
	Orcamento  string   `json:"orcamento"`
	Loja       string   `json:"loja"`
	Conta      string   `json:"conta"`
	Status     string   `json:"ticket_status"`
	Reaberto   bool     `json:"reaberto"`
	Motivo     string   `json:"motivo"`
	Descricao  string   `json:"descricao"`
	Orcamentos int      `json:"orcamentos"`
	Partes     []string `json:"partes"`
	Valor      float64  `json:"valor"`
	DesdeEm    string   `json:"desde_em"`
	AvisadoEm  string   `json:"avisado_em"`
	// A ÚLTIMA TENTATIVA, QUANDO HOUVE UMA
	//
	//	Ela mudou de lugar: até 27/08/2026 morava numa frente própria em
	//	Correções, que mostrava o motivo e não oferecia conserto nenhum — e ainda
	//	contava de novo orçamentos que já estavam contados aqui. O motivo passa a
	//	aparecer ONDE o trabalho é feito, ao lado do botão que o desfaz.
	//
	//	É do ticket, e não de um orçamento: se três partes foram tentadas, a
	//	recusa é a mesma. Vale a mais recente.
	Recusa     string `json:"recusa"`
	RecusaEm   string `json:"recusa_em"`
	Tentativas int    `json:"tentativas"`
}

// GET /orcamentos/pendencias?destino=cliente|encarregados
func (m *Modulo) pendencias(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	destino := strings.TrimSpace(r.URL.Query().Get("destino"))
	if _, ok := listasDePendencia[destino]; !ok {
		web.Falhar(w, http.StatusBadRequest,
			"Diga a lista: destino=cliente ou destino=encarregados.")
		return
	}
	lista, err := m.juntarPendencias(r, p.ClienteID, destino)
	if err != nil {
		m.erro(w, "não consegui montar a lista", err)
		return
	}

	var soma float64
	var orcamentos int
	for _, t := range lista {
		soma += t.Valor
		orcamentos += t.Orcamentos
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"destino":    destino,
		"titulo":     listasDePendencia[destino],
		"tickets":    lista,
		"orcamentos": orcamentos,
		"valor":      soma,
	})
}

// juntarPendencias lê a view e AGRUPA por ticket.
//
// O agrupamento é aqui e não no banco de propósito: a view devolve orçamento a
// orçamento porque é assim que as outras telas usam, e uma segunda view só para
// somar seria uma segunda verdade sobre o mesmo dado (CORE-06).
func (m *Modulo) juntarPendencias(r *http.Request, clienteID, destino string) ([]Pendencia, error) {
	var linhas []linhaDePendencia
	err := m.bd.Buscar(r.Context(), "orcamentos_lista?cliente_id=eq."+banco.Escapar(clienteID)+
		"&status=eq.gerado&destino=eq."+destino+
		"&order=ticket&limit="+strconv.Itoa(TetoDaExtracao)+"&select=*", &linhas)
	if err != nil {
		return nil, err
	}
	return agruparPorTicket(linhas), nil
}

// agruparPorTicket é a parte que decide o que o cliente vê — e por isso ela é
// separada da leitura: assim dá para prová-la sem banco nenhum, com os casos
// reais que já apareceram (o ticket de três partes, o de valor nulo).
func agruparPorTicket(linhas []linhaDePendencia) []Pendencia {
	porTicket := map[int]*Pendencia{}
	for _, l := range linhas {
		t, existe := porTicket[l.Ticket]
		if !existe {
			t = &Pendencia{
				Ticket:    l.Ticket,
				Orcamento: l.ID,
				Loja:      texto(l.Loja),
				Conta:     contaPorExtenso(texto(l.Conta)),
				Status:    texto(l.TicketStatus),
				Reaberto:  l.Reaberto != nil && *l.Reaberto,
				Motivo:    texto(l.MotivoReabertura),
				Descricao: encurtar(texto(l.ChamadoDescricao), 140),
				DesdeEm:   l.CriadoEm,
				AvisadoEm: texto(l.AvisadoEm),
			}
			porTicket[l.Ticket] = t
		}
		// A recusa mais recente do ticket. As linhas vêm ordenadas por ticket, não
		// por data, então a comparação é explícita.
		if l.TentadoEm != nil && *l.TentadoEm > t.RecusaEm {
			t.RecusaEm = *l.TentadoEm
			t.Recusa = texto(l.BloqueioDetalhe)
			if t.Recusa == "" {
				t.Recusa = texto(l.Bloqueio)
			}
		}
		if l.Tentativas > t.Tentativas {
			t.Tentativas = l.Tentativas
		}
		t.Orcamentos++
		t.Partes = append(t.Partes, strconv.Itoa(l.Parte))
		if l.Valor != nil {
			t.Valor += *l.Valor
		}
		// O mais antigo é o que conta: é há quanto tempo aquele ticket trava
		// dinheiro nosso.
		if l.CriadoEm < t.DesdeEm {
			t.DesdeEm = l.CriadoEm
		}
	}

	saida := make([]Pendencia, 0, len(porTicket))
	for _, t := range porTicket {
		saida = append(saida, *t)
	}
	// Maior valor primeiro: quem lê a lista resolve de cima para baixo, e o de
	// cima é o que mais pesa.
	sort.SliceStable(saida, func(i, j int) bool {
		if saida[i].Valor != saida[j].Valor {
			return saida[i].Valor > saida[j].Valor
		}
		return saida[i].Ticket < saida[j].Ticket
	})
	return saida
}

// As colunas de cada lista são DIFERENTES, e é de propósito.
//
//	O cliente precisa saber o chamado, a loja, o que foi pedido e quanto está
//	parado. "Parte 2 de 3" não diz nada para ele.
//
//	O encarregado precisa saber onde ir e o que terminar — por isso a descrição
//	do chamado é a coluna larga da lista dele.
var colunasCliente = []relatorio.Coluna{
	{Titulo: "Chamado", Peso: 1, Tipo: relatorio.Numero},
	{Titulo: "Loja", Peso: 2.2, Tipo: relatorio.Texto},
	{Titulo: "Conta", Peso: 1.2, Tipo: relatorio.Texto},
	{Titulo: "Situação", Peso: 1.6, Tipo: relatorio.Texto},
	{Titulo: "Material comprado", Peso: 1.3, Tipo: relatorio.Dinheiro},
	{Titulo: "Desde", Peso: 1.2, Tipo: relatorio.Data},
	{Titulo: "Observação", Peso: 3, Tipo: relatorio.Texto},
}

var colunasEncarregados = []relatorio.Coluna{
	{Titulo: "Chamado", Peso: 1, Tipo: relatorio.Numero},
	{Titulo: "Loja", Peso: 2.2, Tipo: relatorio.Texto},
	{Titulo: "Conta", Peso: 1.2, Tipo: relatorio.Texto},
	{Titulo: "Situação", Peso: 1.6, Tipo: relatorio.Texto},
	{Titulo: "O que foi pedido", Peso: 4, Tipo: relatorio.Texto},
	{Titulo: "Material parado", Peso: 1.3, Tipo: relatorio.Dinheiro},
	{Titulo: "Desde", Peso: 1.2, Tipo: relatorio.Data},
}

func (m *Modulo) tabelaDePendencias(w http.ResponseWriter, r *http.Request) (relatorio.Tabela, string, bool) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return relatorio.Tabela{}, "", false
	}
	destino := strings.TrimSpace(r.URL.Query().Get("destino"))
	if _, ok := listasDePendencia[destino]; !ok {
		web.Falhar(w, http.StatusBadRequest, "Diga a lista: destino=cliente ou destino=encarregados.")
		return relatorio.Tabela{}, "", false
	}
	lista, err := m.juntarPendencias(r, p.ClienteID, destino)
	if err != nil {
		m.erro(w, "não consegui montar a lista", err)
		return relatorio.Tabela{}, "", false
	}

	colunas := colunasEncarregados
	if destino == "cliente" {
		colunas = colunasCliente
	}

	var soma float64
	linhas := make([][]any, 0, len(lista))
	for _, t := range lista {
		soma += t.Valor
		situacao := t.Status
		if t.Reaberto {
			situacao += " (reaberto)"
		}
		if destino == "cliente" {
			// A observação é o que o CLIENTE escreveu ao reabrir. Devolver a
			// frase dele muda a conversa: em vez de "reabriram", é "reabriram
			// dizendo que o serviço não foi executado".
			linhas = append(linhas, []any{
				t.Ticket, t.Loja, t.Conta, situacao, t.Valor, instante(&t.DesdeEm), t.Motivo,
			})
		} else {
			linhas = append(linhas, []any{
				t.Ticket, t.Loja, t.Conta, situacao, t.Descricao, t.Valor, instante(&t.DesdeEm),
			})
		}
	}

	sub := fmt.Sprintf("%d chamados · %d orçamentos · R$ %.2f parados",
		len(lista), contarOrcamentos(lista), soma)

	return relatorio.Tabela{
		Titulo:    listasDePendencia[destino],
		Subtitulo: sub,
		Colunas:   colunas,
		Linhas:    linhas,
		Gerado:    time.Now(),
	}, destino, true
}

func contarOrcamentos(l []Pendencia) int {
	n := 0
	for _, t := range l {
		n += t.Orcamentos
	}
	return n
}

// GET /orcamentos/pendencias.xlsx?destino=…
func (m *Modulo) pendenciasExcel(w http.ResponseWriter, r *http.Request) {
	tab, destino, ok := m.tabelaDePendencias(w, r)
	if !ok {
		return
	}
	corpo, err := tab.Planilha()
	if err != nil {
		m.erro(w, "não consegui montar a planilha", err)
		return
	}
	entregar(w, corpo, "pendencias-"+destino, "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

// GET /orcamentos/pendencias.pdf?destino=…
func (m *Modulo) pendenciasPDF(w http.ResponseWriter, r *http.Request) {
	tab, destino, ok := m.tabelaDePendencias(w, r)
	if !ok {
		return
	}
	corpo, err := tab.PDF()
	if err != nil {
		m.erro(w, "não consegui montar o PDF", err)
		return
	}
	entregar(w, corpo, "pendencias-"+destino, "pdf", "application/pdf")
}

// POST /orcamentos/pendencias/avisar  {destino, tickets:[…]}
//
// Marca que aqueles tickets foram cobrados. Não manda e-mail nenhum: quem manda
// é a pessoa, com a planilha na mão. Isto aqui só registra que saiu — e um
// registro de envio que mente é pior que nenhum.
func (m *Modulo) avisarPendencias(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	var pedido struct {
		Destino string `json:"destino"`
		Tickets []int  `json:"tickets"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi o pedido.")
		return
	}
	if _, ok := listasDePendencia[pedido.Destino]; !ok {
		web.Falhar(w, http.StatusBadRequest, "Diga a lista: cliente ou encarregados.")
		return
	}
	if len(pedido.Tickets) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Nenhum chamado informado.")
		return
	}

	// O estado de AGORA vira o motivo gravado. O destino muda sozinho; o aviso
	// precisa dizer por que foi mandado naquele dia, senão daqui a três meses
	// ninguém entende um aviso ao cliente num ticket que hoje está aberto.
	atuais, err := m.juntarPendencias(r, p.ClienteID, pedido.Destino)
	if err != nil {
		m.erro(w, "não consegui conferir a lista antes de marcar", err)
		return
	}
	porTicket := map[int]Pendencia{}
	for _, t := range atuais {
		porTicket[t.Ticket] = t
	}

	linhas := make([]map[string]any, 0, len(pedido.Tickets))
	for _, n := range pedido.Tickets {
		t, ok := porTicket[n]
		if !ok {
			// Saiu da lista entre a tela abrir e o clique — provavelmente porque o
			// chamado andou. Não é erro, e marcar seria gravar uma cobrança que
			// não faz mais sentido.
			continue
		}
		motivo := t.Status
		if t.Reaberto {
			motivo += " (reaberto)"
		}
		linhas = append(linhas, map[string]any{
			"cliente_id":  p.ClienteID,
			"ticket":      n,
			"lista":       pedido.Destino,
			"motivo":      motivo,
			"quantos":     t.Orcamentos,
			"valor":       t.Valor,
			"avisado_por": p.UserID,
		})
	}
	if len(linhas) == 0 {
		web.Falhar(w, http.StatusConflict,
			"Nenhum destes chamados ainda está nesta lista — eles andaram desde que a tela abriu.")
		return
	}
	if err := m.bd.Inserir(r.Context(), "ticket_avisos", linhas, nil); err != nil {
		m.erro(w, "não consegui registrar o aviso", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"ok":        true,
		"marcados":  len(linhas),
		"ignorados": len(pedido.Tickets) - len(linhas),
		"quando":    time.Now().UTC().Format(time.RFC3339),
	})
}
