// rev 1 — o tratamento das duas filas: sem ticket e sem associação
//
// A REGRA DE OURO DESTA TELA: A CONFERÊNCIA É A MESMA DA GERAÇÃO
//
//	No sistema antigo a tela tinha a própria conferência e a geração tinha
//	outra. As duas discordavam: a tela dizia "pronta" e a geração recusava, sem
//	ninguém entender por quê — e a saída de todo mundo era mandar gerar para
//	descobrir. Aqui as duas chamam `avaliarPartes`, escrita uma vez (CORE-06).
//
// A EDIÇÃO É LIVRE; QUEM JULGA É A CONFERÊNCIA
//
//	Seria tentador recusar a alteração que deixa a nota inválida — trocar um
//	ticket que estoura o teto, tirar um que deixa a fatia grande demais. Mas
//	consertar uma nota costuma passar por um estado inválido: tirar um ticket
//	errado ANTES de pôr o certo deixa a nota, por um instante, com a divisão
//	toda errada.
//
//	Travar o passo intermediário prenderia o usuário num beco: ele não pode
//	tirar sem pôr, e não pode pôr sem tirar. Então a alteração passa, e a
//	conferência diz a verdade na hora seguinte — verde processa, vermelho não.
//	É a mesma diferença entre um formulário que recusa a tecla e um que recusa
//	o envio.
package orcamentos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// conferir
// ---------------------------------------------------------------------------

// Conferencia é o que a tela precisa saber para pintar a linha de verde ou de
// vermelho — e, quando for vermelho, o que dizer ao usuário.
type Conferencia struct {
	Documento string           `json:"documento"`
	Nome      string           `json:"nome"`
	Fila      string           `json:"fila"`
	Valor     float64          `json:"valor_total"`
	Pronta    bool             `json:"pronta"`
	Motivos   []string         `json:"motivos,omitempty"`
	Partes    []ParteConferida `json:"partes"`
}

// ParteConferida é um ticket da nota, já com a conta feita.
type ParteConferida struct {
	Ticket int    `json:"ticket"`
	NaBase bool   `json:"na_base"`
	Loja   string `json:"loja,omitempty"`
	/** Quanto desta nota cabe a este ticket, já descontado. */
	Custo float64 `json:"custo"`
	/** Quanto o ticket já tem de custo lançado, de qualquer tipo. */
	JaNoTicket float64 `json:"ja_no_ticket"`
	/** O orçamento que sairia daqui. */
	Orcamento float64 `json:"orcamento"`
	/** livre · reduzido · aprovacao */
	Decisao string `json:"decisao"`
	/** Aparada pela regra do desconto — o carimbo interno. */
	Ajustada bool   `json:"ajustada"`
	Motivo   string `json:"motivo,omitempty"`
}

// GET /orcamentos/documentos/{id}/conferir
func (m *Modulo) conferirDocumento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	c, err := m.conferirUm(r.Context(), p, id)
	if err != nil {
		m.erro(w, "não consegui conferir esta nota", err)
		return
	}
	web.Responder(w, http.StatusOK, c)
}

// POST /orcamentos/correcoes/conferir — a lista inteira de uma vez.
//
// É o que o botão "atualizar" chama depois de mexer no Trílogo. De uma vez
// porque a tela pinta TODAS as linhas: conferir uma por uma seriam quarenta
// viagens ao motor para pintar uma tela só.
func (m *Modulo) conferirVarios(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	var pedido struct {
		Documentos []string `json:"documentos"`
	}
	corpo, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if len(corpo) > 0 {
		_ = json.Unmarshal(corpo, &pedido)
	}

	ids := pedido.Documentos
	if len(ids) == 0 {
		// Sem lista, confere tudo o que está em "não associadas".
		var linhas []struct {
			ID string `json:"documento_id"`
		}
		_ = m.bd.Buscar(r.Context(), "documento_tickets?chamado_id=is.null"+
			"&select=documento_id,documentos!inner(cliente_id,oculto_em)"+
			"&documentos.cliente_id=eq."+banco.Escapar(p.ClienteID)+
			"&documentos.oculto_em=is.null&limit=500", &linhas)
		visto := map[string]bool{}
		for _, l := range linhas {
			if l.ID != "" && !visto[l.ID] {
				visto[l.ID] = true
				ids = append(ids, l.ID)
			}
		}
	}

	saida := make([]Conferencia, 0, len(ids))
	for _, bruto := range ids {
		id, ok := umUUID(bruto)
		if !ok {
			continue
		}
		c, err := m.conferirUm(r.Context(), p, id)
		if err != nil {
			// Uma nota que não confere não pode derrubar a conferência das
			// outras trinta e nove.
			saida = append(saida, Conferencia{
				Documento: id, Pronta: false,
				Motivos: []string{"não consegui conferir: " + err.Error()},
			})
			continue
		}
		saida = append(saida, *c)
	}
	web.Responder(w, http.StatusOK, map[string]any{"notas": saida})
}

func (m *Modulo) conferirUm(ctx context.Context, p *seguranca.Principal, id string) (*Conferencia, error) {
	docs, err := m.documentosProntos(ctx, p.ClienteID, []string{id}, "")
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		// Pode ser repetida, bloqueada, oculta ou já usada — todas razões
		// legítimas para ela não estar na fila. A tela precisa da frase.
		return &Conferencia{Documento: id, Pronta: false,
			Motivos: []string{"esta nota não está na fila de geração"}}, nil
	}
	d := docs[0]

	c := &Conferencia{Documento: d.ID, Nome: d.Nome, Fila: d.Fila, Valor: d.Valor}

	tickets, err := m.ticketsDo(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		c.Motivos = []string{"esta nota não tem ticket"}
		return c, nil
	}
	lidos, err := m.itensDo(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if len(lidos.Linhas) == 0 {
		c.Motivos = []string{"esta nota não tem itens lidos"}
		return c, nil
	}
	// A TELA DE TRATAMENTO MOSTRA O MOTIVO, NÃO UM ERRO DE SERVIDOR
	//   Item que não fecha é coisa para a pessoa consertar, como as outras
	//   recusas desta lista. Devolver 500 aqui esconderia justamente o que ela
	//   veio ver.
	if lidos.Bloqueio != "" {
		c.Motivos = []string{lidos.Bloqueio}
		return c, nil
	}
	linhas := lidos.Linhas

	par := m.parametros(ctx, p.ClienteID)
	partes, motivos, err := m.avaliarPartes(ctx, d, tickets, linhas, par)
	if err != nil {
		return nil, err
	}

	lojas := m.lojasDosTickets(ctx, tickets)
	c.Pronta = true
	for i, parte := range partes {
		pc := ParteConferida{
			Ticket:     parte.ticket.Ticket,
			NaBase:     parte.ticket.ChamadoID != nil,
			Loja:       lojas[parte.ticket.Ticket],
			Custo:      parte.custo.ComDesconto.Float(),
			JaNoTicket: parte.veredito.JaNoTicket.Float(),
			Orcamento:  parte.veredito.Valor.Float(),
			Decisao:    string(parte.veredito.Decisao),
			Ajustada:   parte.custo.Ajustada(),
			Motivo:     motivos[i],
		}
		if motivos[i] != "" {
			c.Pronta = false
			c.Motivos = append(c.Motivos, motivos[i])
			pc.Orcamento = 0
		}
		c.Partes = append(c.Partes, pc)
	}
	return c, nil
}

// lojasDosTickets é enfeite útil: sem o nome da loja, a tela mostra seis números
// e quem está tratando não sabe qual é qual.
func (m *Modulo) lojasDosTickets(ctx context.Context, tickets []ticketDoDocumento) map[int]string {
	saida := map[int]string{}
	ids := make([]string, 0, len(tickets))
	for _, t := range tickets {
		if t.ChamadoID != nil {
			ids = append(ids, *t.ChamadoID)
		}
	}
	if len(ids) == 0 {
		return saida
	}
	var linhas []struct {
		Numero  int `json:"numero"`
		Unidade struct {
			Nome string `json:"nome"`
		} `json:"unidades"`
	}
	_ = m.bd.Buscar(ctx, "chamados?id=in.("+juntar(ids)+")&select=numero,unidades(nome)", &linhas)
	for _, l := range linhas {
		saida[l.Numero] = l.Unidade.Nome
	}
	return saida
}

func juntar(ids []string) string {
	saida := ""
	for i, id := range ids {
		if i > 0 {
			saida += ","
		}
		saida += id
	}
	return saida
}

// ---------------------------------------------------------------------------
// mudar de fila
// ---------------------------------------------------------------------------

// POST /orcamentos/documentos/{id}/fila — o botão "nota rateada".
//
// A NOTA NÃO ESTÁ PERDIDA, ESTÁ NA GAVETA ERRADA
//
//	O que faz uma nota ser "de rateio" é o material dela servir a vários
//	chamados — e isso está na cabeça do usuário, não na página. Nenhum campo do
//	documento diz isso, então nenhuma leitura vai adivinhar. O conserto é mover,
//	e mover é trocar um campo.
func (m *Modulo) trocarFila(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var pedido struct {
		Fila string `json:"fila"`
	}
	corpo, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = json.Unmarshal(corpo, &pedido)

	if pedido.Fila != FilaOrcamento && pedido.Fila != FilaRateio {
		web.Falhar(w, http.StatusBadRequest,
			"A fila tem que ser \"orcamento\" ou \"rateio\".")
		return
	}

	filtro := "id=eq." + id + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "documentos", filtro,
		map[string]any{"fila": pedido.Fila}); err != nil {
		m.erro(w, "não consegui mudar a nota de fila", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "trocar_fila",
		map[string]historico.Mudanca{"fila": {De: nil, Para: pedido.Fila}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "fila": pedido.Fila})
}

// As duas filas de entrada, escritas uma vez.
const (
	FilaOrcamento = "orcamento"
	FilaRateio    = "rateio"
)

// ---------------------------------------------------------------------------
// trocar um ticket
// ---------------------------------------------------------------------------

// POST /orcamentos/documentos/{id}/tickets/{ticket}/trocar
//
// POR QUE UM ENDEREÇO SÓ, E NÃO "APAGA E DEPOIS INSERE"
//
//	Porque entre as duas chamadas a nota fica sem aquele ticket. Se a segunda
//	falhar — rede, aba fechada, motor dormindo —, a nota perdeu um ticket e
//	ninguém sabe. Numa nota rateada isso muda o valor de TODAS as partes, e
//	elas sairiam plausíveis: cada uma fecha a própria conta.
func (m *Modulo) trocarTicket(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	velho := umNumero(r.PathValue("ticket"), 0)
	var pedido struct {
		Novo int `json:"novo"`
	}
	corpo, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = json.Unmarshal(corpo, &pedido)

	if velho <= 0 || pedido.Novo <= 0 {
		web.Falhar(w, http.StatusBadRequest, "Preciso do ticket que sai e do que entra.")
		return
	}
	if velho == pedido.Novo {
		web.Responder(w, http.StatusOK, map[string]any{"ok": true, "ticket": velho})
		return
	}
	if err := m.donoDoDocumento(r.Context(), p, id); err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}

	// O NOVO ENTRA ANTES DE O VELHO SAIR
	//   Assim, se algo falhar no meio, a nota fica com um ticket A MAIS — que a
	//   tela mostra e o usuário tira. O contrário deixaria a nota com um ticket
	//   A MENOS, que ninguém vê e que estraga a divisão em silêncio.
	if err := m.amarrar(r.Context(), p, id, []int{pedido.Novo}); err != nil {
		m.erro(w, "não consegui amarrar o ticket novo", err)
		return
	}
	if err := m.bd.Apagar(r.Context(), "documento_tickets",
		"documento_id=eq."+id+"&ticket=eq."+strconv.Itoa(velho)); err != nil {
		m.erro(w, "amarrei o ticket novo mas não consegui tirar o antigo", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "trocar_ticket",
		map[string]historico.Mudanca{"ticket": {De: velho, Para: pedido.Novo}})

	c, err := m.conferirUm(r.Context(), p, id)
	if err != nil {
		web.Responder(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "conferencia": c})
}

func (m *Modulo) donoDoDocumento(ctx context.Context, p *seguranca.Principal, id string) error {
	_, err := m.contarUm(ctx, "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id&limit=1")
	return err
}

// parametros devolve o que vale hoje, caindo no padrão quando o banco não tem.
//
// Cair no padrão em SILÊNCIO seria regra fantasma — por isso quem gera avisa em
// log. Aqui é só conferência: um número aproximado numa tela é melhor que uma
// tela que não abre.
func (m *Modulo) parametros(ctx context.Context, clienteID string) regras.Parametros {
	par, _, err := m.param.Do(ctx, clienteID)
	if err != nil {
		return regras.Padrao
	}
	return par
}

// POST /orcamentos/documentos/{id}/tickets/{ticket}/apagar
//
// POR QUE APAGAR DE VERDADE, SE QUASE NADA AQUI É APAGADO
//
//	Porque isto é uma LIGAÇÃO, não um fato. A nota continua inteira, o arquivo
//	continua no armazém, o chamado continua existindo — o que sai é a linha que
//	dizia "esta nota atende aquele ticket", e ela nunca deveria ter existido.
//	Guardar uma associação errada com data de remoção seria guardar um erro de
//	digitação como se fosse história.
//
//	É POST e não DELETE porque o DELETE deste mesmo caminho já significa outra
//	coisa desde antes: soltar o ticket do chamado, mantendo o ticket na nota.
//	Duas operações parecidas com nomes iguais é como se apaga o errado.
func (m *Modulo) apagarTicket(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ticket := umNumero(r.PathValue("ticket"), 0)
	if ticket <= 0 {
		web.Falhar(w, http.StatusBadRequest, "Ticket inválido.")
		return
	}
	if err := m.donoDoDocumento(r.Context(), p, id); err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}

	// NOTA QUE JÁ VIROU ORÇAMENTO NÃO TEM TICKET TIRADO POR AQUI
	//
	//	O ticket é o chamado que recebeu o custo. Se a nota já gerou orçamento,
	//	tirar o ticket dela deixa os dois contando histórias diferentes: o
	//	orçamento diz "vim da nota Y, para o ticket X" e a nota Y não conhece
	//	mais o ticket X. A ponte entre as duas contas continua fechando — o
	//	vínculo é carimbado a partir do orçamento — mas a resposta para "de onde
	//	veio este custo?" deixa de existir.
	//
	//	E a saída certa já existe e desfaz tudo na ordem: apagar o orçamento
	//	devolve a nota para a fila, solta o vínculo e libera a chave. Depois
	//	disso, mexer nos tickets é seguro.
	//
	//	A frase diz o caminho, e não só o "não" (P-18): recusa que não ensina o
	//	que fazer é a recusa que faz a pessoa procurar contorno.
	doc, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=status&limit=1")
	if err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}
	if fmt.Sprint(doc["status"]) == "usado" {
		web.Falhar(w, http.StatusConflict,
			"Esta nota já virou orçamento, então os tickets dela não podem mudar aqui. "+
				"Apague o orçamento primeiro — a nota volta para a fila e os tickets ficam livres.")
		return
	}

	if err := m.bd.Apagar(r.Context(), "documento_tickets",
		"documento_id=eq."+id+"&ticket=eq."+strconv.Itoa(ticket)); err != nil {
		m.erro(w, "não consegui tirar o ticket desta nota", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "apagar_ticket",
		map[string]historico.Mudanca{"ticket": {De: ticket, Para: nil}})

	// A conferência volta junto: tirar um ticket muda a fatia de TODOS os
	// outros, e a tela precisa repintar a nota inteira, não só a linha tocada.
	c, err := m.conferirUm(r.Context(), p, id)
	if err != nil {
		web.Responder(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "conferencia": c})
}

// ---------------------------------------------------------------------------
// as notas que não cabem no teto
// ---------------------------------------------------------------------------

// GET /orcamentos/correcoes/extrapoladas
//
// São as notas que a geração recusou por limite de teto. Elas já carregam a
// frase do motivo desde a 020 — esta lista só as junta, e a tela pergunta, para
// cada uma, se existe desconto que resolva.
func (m *Modulo) extrapoladas(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), "documentos_lista?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&oculto_em=is.null&bloqueio_motivo=not.is.null"+
		"&order=inserido_em.desc&limit=500&select=*", &linhas); err != nil {
		m.erro(w, "não consegui listar as notas que passam do teto", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"notas": ouVazio(linhas)})
}

type descontoNaTela struct {
	Pode          bool    `json:"pode"`
	BP            int64   `json:"bp"`
	Porcentagem   string  `json:"porcentagem"`
	Original      float64 `json:"orcamento_original"`
	Final         float64 `json:"orcamento_final"`
	Motivo        string  `json:"motivo,omitempty"`
	JaTemDesconto bool    `json:"ja_tem_desconto"`
}

// GET /orcamentos/documentos/{id}/desconto — quanto seria, e se dá.
//
// A TELA PERGUNTA ANTES DE MOSTRAR O BOTÃO
//
//	O usuário não escolhe o número: o sistema calcula o único desconto que
//	resolve. Mas ele precisa VER o número antes de confirmar, e o botão precisa
//	saber se abre — oferecer "dar desconto" numa nota que nenhum desconto
//	conserta é pedir para a pessoa tentar o caminho que não leva a lugar nenhum.
func (m *Modulo) verDesconto(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	d, err := m.calcularDesconto(r.Context(), p, id)
	if err != nil {
		m.erro(w, "não consegui calcular o desconto", err)
		return
	}
	web.Responder(w, http.StatusOK, d)
}

func (m *Modulo) calcularDesconto(ctx context.Context, p *seguranca.Principal, id string) (*descontoNaTela, error) {
	docs, err := m.documentosProntos(ctx, p.ClienteID, []string{id}, "")
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return &descontoNaTela{Motivo: "esta nota não está na fila de geração"}, nil
	}
	d := docs[0]

	// UMA NOTA NÃO GANHA DESCONTO DUAS VEZES
	//   Empilhar autorizações é como se chega a 40% sem ninguém ter autorizado
	//   40%: cada clique parecia pequeno.
	if d.DescontoBP > 0 {
		return &descontoNaTela{
			JaTemDesconto: true, BP: d.DescontoBP,
			Porcentagem: regras.Porcentagem(d.DescontoBP),
			Motivo:      "esta nota já teve desconto de " + regras.Porcentagem(d.DescontoBP) + " autorizado",
		}, nil
	}

	tickets, err := m.ticketsDo(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		return &descontoNaTela{Motivo: "esta nota não tem ticket"}, nil
	}

	// A DIVISÃO AQUI É A MESMA DA GERAÇÃO (31/08/2026)
	//
	//	Esta tela responde "quanto de desconto resolve esta nota". A geração
	//	responde "esta nota roda". As duas perguntas só têm resposta compatível
	//	se as duas repartirem a nota do mesmo jeito.
	//
	//	Até aqui esta linha dividia o pago por igual (`fatiar`) enquanto a
	//	geração já repartia QUANTIDADE. Numa nota com itens de tamanhos
	//	diferentes as duas contas divergem: a tela ofereceria um desconto de 6%
	//	e a geração continuaria recusando o ticket que levou o pedaço maior —
	//	com a autorização já assinada e nada resolvido.
	lidos, err := m.itensDo(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if lidos.Bloqueio != "" {
		return &descontoNaTela{Motivo: lidos.Bloqueio}, nil
	}
	if len(lidos.Linhas) == 0 {
		return &descontoNaTela{Motivo: "esta nota não tem itens lidos"}, nil
	}

	par := m.parametros(ctx, p.ClienteID)
	pago := regras.DinheiroDe(d.Valor)
	n := len(tickets)

	cheios := make([]regras.Dinheiro, n)
	for i := range tickets {
		cheios[i] = somaDasLinhas(regras.Repartir(lidos.Linhas, n, i))
	}
	pagos := regras.RepartirPago(pago, cheios)

	// NUMA NOTA RATEADA, O PIOR PEDAÇO MANDA
	//   O desconto é da nota inteira, e só serve se resolver TODOS os tickets.
	//   Calcular pelo primeiro deixaria a nota autorizada e ainda travada — com
	//   a assinatura já dada e nada resolvido.
	// PEDAÇO QUE JÁ CABE NÃO É RECUSA
	//   Numa nota rateada os pedaços são desiguais. Ler "não precisa de
	//   desconto" como "não dá desconto" faria a nota inteira travar por causa
	//   do ticket que recebeu dois parafusos.
	pior := regras.Desconto{}
	algumPrecisa := false
	for i, t := range tickets {
		var jaNoTicket regras.Dinheiro
		if t.ChamadoID != nil {
			if jaNoTicket, err = m.custosDoTicket(ctx, *t.ChamadoID); err != nil {
				return nil, err
			}
		}
		este := regras.CalcularDesconto(pagos[i], jaNoTicket, par)
		if este.Desnecessario {
			continue
		}
		if !este.Pode {
			return &descontoNaTela{
				Motivo: fmt.Sprintf("ticket %d — %s", t.Ticket, este.Motivo),
			}, nil
		}
		algumPrecisa = true
		if este.BP > pior.BP {
			pior = este
		}
	}
	if !algumPrecisa {
		return &descontoNaTela{
			Motivo: "esta nota já cabe no teto — não precisa de desconto",
		}, nil
	}

	return &descontoNaTela{
		Pode: true, BP: pior.BP, Porcentagem: regras.Porcentagem(pior.BP),
		Original: pior.Original.Float(), Final: pior.Final.Float(),
	}, nil
}

// POST /orcamentos/documentos/{id}/desconto — autoriza.
//
// A SENHA NÃO É CONFERIDA AQUI, E ISSO PRECISA ESTAR ESCRITO
//
//	A confirmação por senha acontece no navegador, contra o mesmo serviço de
//	autenticação que faz o login. Ela é confirmação de INTENÇÃO — o equivalente
//	a "digite APAGAR para confirmar" —, não trava de servidor: quem chamasse
//	este endereço por fora não passaria por ela.
//
//	O que o servidor garante é o resto, e o resto é o que importa: o desconto é
//	CALCULADO aqui, nunca recebido do navegador; não passa de 20%; e fica
//	registrado quem autorizou e quando. A trava de verdade será a permissão por
//	nível, que o dono deixou para depois.
func (m *Modulo) autorizarDesconto(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	// O NAVEGADOR NÃO DIZ QUANTO É O DESCONTO
	//   Ele pede a autorização; o número sai da mesma conta que a tela mostrou.
	//   Aceitar um valor de fora seria aceitar 40% de quem soubesse escrever a
	//   chamada à mão.
	d, err := m.calcularDesconto(r.Context(), p, id)
	if err != nil {
		m.erro(w, "não consegui calcular o desconto", err)
		return
	}
	if !d.Pode {
		web.Falhar(w, http.StatusConflict, ouEntao(d.Motivo, "esta nota não aceita desconto."))
		return
	}

	// `desconto_bp=eq.0` no filtro é a segunda trava contra empilhar: se dois
	// cliques chegarem juntos, o segundo não encontra linha para alterar.
	filtro := "id=eq." + id + "&cliente_id=eq." + banco.Escapar(p.ClienteID) + "&desconto_bp=eq.0"
	if err := m.bd.Atualizar(r.Context(), "documentos", filtro, map[string]any{
		"desconto_bp":     d.BP,
		"desconto_em":     time.Now().UTC().Format(time.RFC3339),
		"desconto_por":    p.UserID,
		"bloqueio_motivo": nil,
	}); err != nil {
		m.erro(w, "não consegui gravar o desconto", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "autorizar_desconto",
		map[string]historico.Mudanca{"desconto_bp": {De: 0, Para: d.BP}})

	web.Responder(w, http.StatusOK, map[string]any{
		"ok": true, "bp": d.BP, "porcentagem": d.Porcentagem})
}

// POST /orcamentos/documentos/{id}/aprovacao — manda ao cliente.
//
// O QUE VAI AO CLIENTE É O ORÇAMENTO, NÃO A NOTA
//
//	Correção do dono, e ela muda o desenho: o orçamento precisa EXISTIR para ser
//	mostrado. Então esta marca não guarda a nota numa gaveta — ela libera a nota
//	a gerar com o valor CHEIO, e quem segura o resultado é o status
//	`aguardando_aprovacao` do orçamento, que o sistema já sabia fazer.
func (m *Modulo) pedirAprovacao(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	filtro := "id=eq." + id + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Atualizar(r.Context(), "documentos", filtro, map[string]any{
		"aprovacao_pedida":     true,
		"aprovacao_pedida_em":  time.Now().UTC().Format(time.RFC3339),
		"aprovacao_pedida_por": p.UserID,
		"bloqueio_motivo":      nil,
	}); err != nil {
		m.erro(w, "não consegui marcar a nota para aprovação", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "pedir_aprovacao", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

func ouEntao(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
