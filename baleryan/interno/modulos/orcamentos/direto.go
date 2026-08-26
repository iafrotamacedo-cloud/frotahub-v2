// rev 1 — faturamento direto: a nota que o fornecedor cobra do cliente
//
// O QUE ESTA FILA É, E O QUE ELA NÃO É
//
//	Há notas que o fornecedor fatura DIRETO ao cliente. Elas passam por nós só
//	para controle e estatística: nós lançamos o custo no Trílogo, porque o
//	chamado precisa saber quanto custou — e nunca as cobramos, porque não é
//	nosso o dinheiro que entra.
//
//	Por isso aqui NÃO existe margem, NÃO existe teto, NÃO existe rateio e NÃO
//	existe espelho de faturamento. O que sobe para o Trílogo é o arquivo
//	ORIGINAL da nota, e o valor é o dela, limpo.
//
// O TICKET VEM DO USUÁRIO, E ISSO É UMA DECISÃO
//
//	Decisão do dono em 26/08/2026: "não precisa conferir o trílogo nem ler o
//	ticket da nota, nesse caso é confiar no usuário". E é coerente: as travas de
//	ticket existem para proteger o que a gente COBRA — ticket errado num
//	orçamento nosso vira dinheiro cobrado da loja errada. Aqui não há cobrança
//	nossa; o pior caso é uma estatística no chamado errado, que se conserta
//	olhando.
//
//	Trocar rigor por velocidade só é aceitável quando se sabe o que se está
//	trocando. Aqui se sabe.
package orcamentos

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// FilaDireto é a terceira fila de entrada.
const FilaDireto = "direto"

// GET /orcamentos/direto — a lista, com os filtros que o dono pediu.
//
// FILTRO POR LOJA, TICKET E DATA
//
//	Esta fila não tem etapas: a nota entra, ganha o ticket e é lançada. O que
//	sobra depois é histórico — e histórico sem filtro é uma tabela que ninguém
//	abre pela segunda vez.
func (m *Modulo) listarDireto(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaNotas)
	if p == nil {
		return
	}
	q := r.URL.Query()
	pagina := umNumero(q.Get("pagina"), 1)
	por := umDosPermitidos(q.Get("por"))

	filtro := "documentos_lista?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&fila=eq." + FilaDireto

	if q.Get("ocultos") == "1" {
		filtro += "&oculto_em=not.is.null&order=oculto_em.desc"
	} else {
		filtro += "&oculto_em=is.null&order=inserido_em.desc"
	}
	// A data é a de INSERÇÃO, não a de emissão da nota: quem procura aqui
	// procura "o que entrou naquele dia", que é como o trabalho acontece.
	if de := q.Get("de"); de != "" {
		filtro += "&inserido_em=gte." + banco.Escapar(de)
	}
	if ate := q.Get("ate"); ate != "" {
		filtro += "&inserido_em=lte." + banco.Escapar(ate) + "T23:59:59"
	}
	if t := umNumero(q.Get("ticket"), 0); t > 0 {
		// `cs` é "contém": o ticket está num array na visão.
		filtro += "&ticket_numeros=cs.{" + strconv.Itoa(t) + "}"
	}
	if busca := strings.TrimSpace(q.Get("busca")); busca != "" {
		filtro += "&or=(nome_arquivo.ilike.*" + banco.Escapar(busca) +
			"*,numero.ilike.*" + banco.Escapar(busca) +
			"*,emitente_nome.ilike.*" + banco.Escapar(busca) + "*)"
	}

	var linhas []map[string]any
	total, err := m.bd.BuscarContando(r.Context(), filtro+"&select=*"+intervalo(pagina, por), &linhas)
	if err != nil {
		m.erro(w, "não consegui listar as notas de faturamento direto", err)
		return
	}

	// A LOJA NÃO ESTÁ NA NOTA — ELA VEM DO CHAMADO
	//   Filtrar por loja no banco exigiria uma visão nova só para isto. Como a
	//   página já vem cortada em cinquenta linhas, o cruzamento sai aqui, com
	//   uma consulta a mais em vez de uma migração a mais.
	m.enfeitarComLoja(r.Context(), linhas)
	if loja := strings.TrimSpace(q.Get("loja")); loja != "" {
		linhas = soDaLoja(linhas, loja)
	}

	web.Responder(w, http.StatusOK, montarPagina(linhas, total, pagina, por))
}

func soDaLoja(linhas []map[string]any, loja string) []map[string]any {
	alvo := strings.ToLower(loja)
	saida := make([]map[string]any, 0, len(linhas))
	for _, l := range linhas {
		if nome, _ := l["loja"].(string); strings.Contains(strings.ToLower(nome), alvo) {
			saida = append(saida, l)
		}
	}
	return saida
}

// enfeitarComLoja põe o nome da loja em cada linha, pelo primeiro ticket dela.
func (m *Modulo) enfeitarComLoja(ctx context.Context, linhas []map[string]any) {
	numeros := map[int]bool{}
	for _, l := range linhas {
		for _, t := range numerosDoCampo(l["ticket_numeros"]) {
			numeros[t] = true
		}
	}
	if len(numeros) == 0 {
		return
	}
	lista := make([]string, 0, len(numeros))
	for n := range numeros {
		lista = append(lista, strconv.Itoa(n))
	}
	var achados []struct {
		Numero  int `json:"numero"`
		Unidade struct {
			Nome string `json:"nome"`
		} `json:"unidades"`
	}
	_ = m.bd.Buscar(ctx, "chamados?numero=in.("+strings.Join(lista, ",")+
		")&select=numero,unidades(nome)", &achados)
	porTicket := map[int]string{}
	for _, a := range achados {
		porTicket[a.Numero] = a.Unidade.Nome
	}
	for _, l := range linhas {
		for _, t := range numerosDoCampo(l["ticket_numeros"]) {
			if nome := porTicket[t]; nome != "" {
				l["loja"] = nome
				break
			}
		}
	}
}

func numerosDoCampo(v any) []int {
	bruto, ok := v.([]any)
	if !ok {
		return nil
	}
	saida := make([]int, 0, len(bruto))
	for _, x := range bruto {
		if f, ok := x.(float64); ok {
			saida = append(saida, int(f))
		}
	}
	return saida
}

// POST /orcamentos/direto/{id}/lancar — põe a nota na fila de lançamento.
//
// POR QUE ELA VIRA UMA LINHA EM `orcamentos`
//
//	Porque a fila de lançar é essa, e a máquina que sobe arquivo para o Trílogo
//	é uma só. Duplicá-la para uma variante seria manter duas rotinas que fazem a
//	mesma coisa difícil — e uma das duas envelheceria.
//
//	O que a distingue é a coluna `faturamento_direto`, e ela faz UMA coisa:
//	mantém a linha fora do espelho de faturamento, para sempre. Sem ela, uma
//	nota que o cliente já pagou ao fornecedor seria cobrada por nós — todo mês,
//	porque `fatura_id` continuaria nulo.
func (m *Modulo) mandarParaLancar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaNotas)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	doc, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&fila=eq."+FilaDireto+"&select=*&limit=1")
	if err != nil {
		m.erro(w, "não achei esta nota na fila de faturamento direto", err)
		return
	}
	valor := regras.DinheiroDe(numeroDe(doc["valor_total"]))
	if valor <= 0 {
		web.Falhar(w, http.StatusBadRequest,
			"Esta nota está sem valor lido. Lançar zero no Trílogo seria pior que não lançar.")
		return
	}

	tickets, err := m.ticketsDo(r.Context(), id)
	if err != nil {
		m.erro(w, "não consegui ler os tickets", err)
		return
	}
	if len(tickets) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Informe o ticket antes de mandar para o lançamento.")
		return
	}

	// UMA NOTA, UM CUSTO
	//   Sem rateio: se o usuário puser dois tickets aqui, o custo vai no
	//   primeiro e a tela diz isso. Dividir valor entre chamados é a máquina de
	//   rateio, que esta fila justamente não usa.
	t := tickets[0]

	var chamadoID any
	unidade, conta := any(nil), any(nil)
	if t.ChamadoID != nil {
		chamadoID = *t.ChamadoID
		if c, err := m.contarUm(r.Context(), "chamados?id=eq."+*t.ChamadoID+
			"&select=unidade_id,conta&limit=1"); err == nil {
			unidade, conta = c["unidade_id"], c["conta"]
		}
	}

	parte, err := m.proximaParte(r.Context(), p.ClienteID, t.Ticket)
	if err != nil {
		m.erro(w, "não consegui numerar a parte", err)
		return
	}

	linha := map[string]any{
		"cliente_id": p.ClienteID,
		"ticket":     t.Ticket,
		"parte":      parte,
		"chamado_id": chamadoID,
		"unidade_id": unidade,
		"conta":      conta,
		// O VALOR É O DA NOTA, LIMPO
		//   Sem os 20%: a margem existe para o que a gente fatura ao cliente, e
		//   esta não é faturada por nós. O Trílogo recebe o custo real.
		"valor_nota":         valor.Float(),
		"valor_nota_cheio":   valor.Float(),
		"valor":              valor.Float(),
		"margem_aplicada":    0,
		"reduzido_pelo_teto": false,
		"ajustado_pelo_teto": false,
		"rateio":             false,
		"faturamento_direto": true,
		"status":             "gerado",
		"criado_por":         p.UserID,
	}

	var criados []map[string]any
	if err := m.bd.Inserir(r.Context(), "orcamentos", []map[string]any{linha}, &criados); err != nil {
		if banco.Duplicado(err) {
			web.Falhar(w, http.StatusConflict,
				"Já existe um lançamento com esta parte para o ticket "+strconv.Itoa(t.Ticket)+".")
			return
		}
		m.erro(w, "não consegui criar o lançamento", err)
		return
	}
	if len(criados) == 0 {
		m.erro(w, "criei o lançamento mas o banco não devolveu o id", nil)
		return
	}
	orcID := fmtID(criados[0]["id"])

	// O VÍNCULO É O QUE FAZ O ARQUIVO ORIGINAL SUBIR
	//   Na hora de lançar, é por ele que o motor acha a nota no armazém em vez
	//   de montar um PDF de orçamento que aqui não existe.
	if err := m.bd.Inserir(r.Context(), "orcamento_documentos", []map[string]any{{
		"orcamento_id": orcID,
		"documento_id": id,
		"parcela":      valor.Float(),
	}}, nil); err != nil {
		m.erro(w, "criei o lançamento mas não o vínculo com a nota", err)
		return
	}
	_ = m.bd.Atualizar(r.Context(), "documentos", "id=eq."+id, map[string]any{"status": "usado"})
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "faturamento_direto_lancar",
		map[string]historico.Mudanca{"ticket": {De: nil, Para: t.Ticket}})

	web.Responder(w, http.StatusOK, map[string]any{
		"ok": true, "orcamento": orcID, "ticket": t.Ticket,
		"aviso": avisoDeVariosTickets(len(tickets), t.Ticket),
	})
}

func avisoDeVariosTickets(quantos, usado int) string {
	if quantos <= 1 {
		return ""
	}
	return "Esta nota tem " + strconv.Itoa(quantos) + " tickets e o faturamento direto não rateia: " +
		"o custo foi lançado no ticket " + strconv.Itoa(usado) + "."
}

func fmtID(v any) string {
	s, _ := v.(string)
	return s
}
