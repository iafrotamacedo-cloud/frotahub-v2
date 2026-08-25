// rev 1 — gerar o orçamento
//
// UM PIPELINE, NÃO CINCO
//
//	No sistema antigo esta rotina existia cinco vezes quase idêntica:
//	`_orc_job_run`, `_reproc_gerar`, `_corr_job_run`, `_txt_job_run` e
//	`_rat_job_run`. Corrigir uma regra era mexer em cinco lugares e torcer para
//	não esquecer nenhum — e o `+20%` acabou com onze implementações e dois
//	arredondamentos diferentes.
//
//	Aqui existe UM caminho. Rateio não é outro caminho: é o mesmo, com a lista
//	de tickets vindo do usuário em vez de vir da observação da nota.
package orcamentos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// GET /orcamentos — a lista
// ---------------------------------------------------------------------------

func (m *Modulo) listarOrcamentos(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}
	q := r.URL.Query()
	pagina := umNumero(q.Get("pagina"), 1)
	por := umDosPermitidos(q.Get("por"))

	filtro := "orcamentos_lista?cliente_id=eq." + banco.Escapar(p.ClienteID)
	if s := q.Get("status"); s != "" && statusConhecido(s) {
		filtro += "&status=eq." + s
	} else {
		filtro += "&status=neq.removido"
	}
	if t := umNumero(q.Get("ticket"), 0); t > 0 {
		filtro += "&ticket=eq." + strconv.Itoa(t)
	}
	filtro += "&order=criado_em.desc"

	var linhas []map[string]any
	total, err := m.bd.BuscarContando(r.Context(), filtro+"&select=*"+intervalo(pagina, por), &linhas)
	if err != nil {
		m.erro(w, "não consegui listar os orçamentos", err)
		return
	}
	web.Responder(w, http.StatusOK, montarPagina(linhas, total, pagina, por))
}

func statusConhecido(s string) bool {
	switch s {
	case "gerado", "aguardando_aprovacao", "lancado", "removido":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// POST /orcamentos/gerar
// ---------------------------------------------------------------------------

type pedidoDeGeracao struct {
	// Vazio = gera para tudo que está lido e amarrado. Com lista, gera só esses.
	Documentos []string `json:"documentos"`
}

type saidaDaGeracao struct {
	Documento string `json:"documento"`
	Nome      string `json:"nome"`
	Ticket    int    `json:"ticket,omitempty"`
	Orcamento string `json:"orcamento,omitempty"`
	Valor     string `json:"valor,omitempty"`
	Status    string `json:"status,omitempty"`
	Aviso     string `json:"aviso,omitempty"`
	Erro      string `json:"erro,omitempty"`
}

func (m *Modulo) gerar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}

	var pedido pedidoDeGeracao
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido)
	}

	par, completo, err := m.param.Do(r.Context(), p.ClienteID)
	if err != nil {
		m.erro(w, "não consegui ler os parâmetros", err)
		return
	}
	if !completo {
		// Não trava a geração, mas grita. Regra fantasma é pior que regra
		// errada: a errada alguém corrige, a fantasma ninguém vê.
		log.Printf("orcamentos: ATENÇÃO — parâmetros incompletos para o cliente %s; usando o padrão (teto %s, margem %d bp)",
			p.ClienteID, par.Teto.Reais(), par.MargemBP)
	}

	docs, err := m.documentosProntos(r.Context(), p.ClienteID, pedido.Documentos)
	if err != nil {
		m.erro(w, "não consegui achar as notas prontas", err)
		return
	}

	saida := make([]saidaDaGeracao, 0, len(docs))
	for _, d := range docs {
		saida = append(saida, m.gerarDeUmDocumento(r.Context(), p, d, par)...)
	}
	web.Responder(w, http.StatusOK, map[string]any{"resultados": saida})
}

type documentoPronto struct {
	ID         string  `json:"id"`
	Nome       string  `json:"nome_arquivo"`
	Fila       string  `json:"fila"`
	Numero     *string `json:"numero"`
	DAV        *string `json:"dav_numero"`
	Chave      *string `json:"chave_acesso"`
	Emissao    *string `json:"emissao"`
	Observacao *string `json:"observacao"`
	Valor      float64 `json:"valor_total"`
}

// documentosProntos são as notas lidas, ainda não usadas e não ocultas.
func (m *Modulo) documentosProntos(ctx context.Context, clienteID string, apenas []string) ([]documentoPronto, error) {
	filtro := "documentos?cliente_id=eq." + banco.Escapar(clienteID) +
		"&oculto_em=is.null&status=eq.lido" +
		"&select=id,nome_arquivo,fila,numero,dav_numero,chave_acesso,emissao,observacao,valor_total" +
		"&order=inserido_em"

	if len(apenas) > 0 {
		ids := make([]string, 0, len(apenas))
		for _, a := range apenas {
			if id, ok := umUUID(a); ok {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return nil, nil
		}
		filtro += "&id=in.(" + strings.Join(ids, ",") + ")"
	}

	var docs []documentoPronto
	if err := m.bd.Buscar(ctx, filtro, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// gerarDeUmDocumento produz um orçamento POR TICKET amarrado à nota.
//
// UMA NOTA, VÁRIOS TICKETS: É O RATEIO
//
//	Quando a nota atende três tickets, cada um recebe a sua parte — e a soma das
//	partes é o valor da nota, no centavo. A divisão é por igual entre os tickets
//	porque é o que a operação faz hoje; se um dia virar por item, muda aqui e em
//	nenhum outro lugar.
func (m *Modulo) gerarDeUmDocumento(ctx context.Context, p *seguranca.Principal,
	d documentoPronto, par regras.Parametros) []saidaDaGeracao {

	base := saidaDaGeracao{Documento: d.ID, Nome: d.Nome}

	tickets, err := m.ticketsDo(ctx, d.ID)
	if err != nil {
		base.Erro = "não consegui ler os tickets: " + err.Error()
		return []saidaDaGeracao{base}
	}
	if len(tickets) == 0 {
		base.Erro = "esta nota não tem ticket"
		return []saidaDaGeracao{base}
	}

	itens, err := m.itensDo(ctx, d.ID)
	if err != nil || len(itens) == 0 {
		base.Erro = "esta nota não tem itens lidos"
		return []saidaDaGeracao{base}
	}

	// A margem entra no unitário, uma vez só, em regras.MontarItens.
	comMargem, totalComMargem := regras.MontarItens(itens, par.MargemBP)

	saida := make([]saidaDaGeracao, 0, len(tickets))
	for i, t := range tickets {
		res := base
		res.Ticket = t.Ticket

		if t.ChamadoID == nil {
			res.Erro = "o ticket não existe na nossa base de chamados"
			saida = append(saida, res)
			continue
		}

		// A parcela deste ticket. A última fica com a sobra dos centavos, para a
		// soma das partes fechar exatamente com o total.
		parcela := totalComMargem
		itensDaParte := comMargem
		if len(tickets) > 1 {
			parcela = fatiar(totalComMargem, len(tickets), i)
			var errCorte error
			itensDaParte, errCorte = regras.Encaixar(comMargem, parcela)
			if errCorte != nil {
				res.Erro = errCorte.Error()
				saida = append(saida, res)
				continue
			}
		}

		id, aviso, err := m.criarOrcamento(ctx, p, d, t, itensDaParte, parcela, par)
		switch {
		case err != nil:
			res.Erro = err.Error()
		default:
			res.Orcamento = id
			res.Aviso = aviso
		}
		saida = append(saida, res)
	}
	return saida
}

// fatiar divide um valor em n partes iguais, jogando a sobra na última.
func fatiar(total regras.Dinheiro, n, i int) regras.Dinheiro {
	if n <= 1 {
		return total
	}
	parte := regras.Dinheiro(int64(total) / int64(n))
	if i == n-1 {
		return total - parte*regras.Dinheiro(n-1)
	}
	return parte
}

type ticketDoDocumento struct {
	Ticket    int     `json:"ticket"`
	ChamadoID *string `json:"chamado_id"`
}

func (m *Modulo) ticketsDo(ctx context.Context, documentoID string) ([]ticketDoDocumento, error) {
	var t []ticketDoDocumento
	err := m.bd.Buscar(ctx, "documento_tickets?documento_id=eq."+documentoID+
		"&order=ticket&select=ticket,chamado_id", &t)
	return t, err
}

func (m *Modulo) itensDo(ctx context.Context, documentoID string) ([]regras.LinhaDaNota, error) {
	var linhas []struct {
		Descricao  string  `json:"descricao"`
		Unidade    *string `json:"unidade"`
		Quantidade float64 `json:"quantidade"`
		Unitario   float64 `json:"valor_unitario"`
	}
	if err := m.bd.Buscar(ctx, "documento_itens?documento_id=eq."+documentoID+
		"&order=ordem&select=descricao,unidade,quantidade,valor_unitario", &linhas); err != nil {
		return nil, err
	}
	saida := make([]regras.LinhaDaNota, 0, len(linhas))
	for _, l := range linhas {
		un := ""
		if l.Unidade != nil {
			un = *l.Unidade
		}
		saida = append(saida, regras.LinhaDaNota{
			Descricao:  l.Descricao,
			Unidade:    un,
			Quantidade: regras.QuantidadeDe(l.Quantidade),
			Unitario:   regras.PrecoDe(l.Unitario),
		})
	}
	return saida, nil
}

// criarOrcamento grava o orçamento, já com o teto decidido.
//
// O TETO É CONFERIDO NA GERAÇÃO, E NÃO SÓ NO LANÇAMENTO
//
//	No sistema antigo só dava para conferir na hora de lançar, porque a soma dos
//	custos do ticket só existia lá no Trílogo. Hoje o robô já leu esses custos
//	para dentro de `chamado_custos` — então dá para avisar no momento em que o
//	orçamento nasce que aquele ticket já tem R$ 480 gastos.
//
//	Descobrir cedo custa um clique. Descobrir tarde custa retrabalho.
//
//	Isso NÃO dispensa a conferência no lançamento: entre gerar e lançar, alguém
//	pode ter lançado outra coisa. São duas conferências de propósito.
func (m *Modulo) criarOrcamento(ctx context.Context, p *seguranca.Principal,
	d documentoPronto, t ticketDoDocumento, itens []regras.Item,
	valor regras.Dinheiro, par regras.Parametros) (string, string, error) {

	jaNoTicket, err := m.custosDoTicket(ctx, *t.ChamadoID)
	if err != nil {
		return "", "", fmt.Errorf("não consegui somar os custos do ticket: %w", err)
	}

	v := regras.AplicarTeto(valor, jaNoTicket, par)

	itensFinais := itens
	aviso := ""
	if v.Reduziu() {
		itensFinais, err = regras.Encaixar(itens, v.Valor)
		if err != nil {
			return "", "", err
		}
		aviso = fmt.Sprintf("reduzido de %s para %s: o ticket já tinha %s e o teto é %s",
			v.ValorOriginal.Reais(), v.Valor.Reais(), v.JaNoTicket.Reais(), v.Teto.Reais())
	}

	status := "gerado"
	if v.Decisao == regras.Aprovacao {
		status = "aguardando_aprovacao"
		aviso = fmt.Sprintf("parado para aprovação: %s neste orçamento mais %s já no ticket passam do teto de %s",
			v.ValorOriginal.Reais(), v.JaNoTicket.Reais(), v.Teto.Reais())
	}

	parte, err := m.proximaParte(ctx, p.ClienteID, t.Ticket)
	if err != nil {
		return "", "", err
	}

	chamado, err := m.contarUm(ctx, "chamados?id=eq."+*t.ChamadoID+
		"&select=unidade_id,conta&limit=1")
	if err != nil {
		return "", "", err
	}

	linha := map[string]any{
		"cliente_id":         p.ClienteID,
		"ticket":             t.Ticket,
		"parte":              parte,
		"chamado_id":         *t.ChamadoID,
		"unidade_id":         chamado["unidade_id"],
		"conta":              chamado["conta"],
		"valor_nota":         d.Valor,
		"valor":              v.Valor.Float(),
		"margem_aplicada":    float64(par.MargemBP) / 10000,
		"teto_aplicado":      v.Teto.Float(),
		"reduzido_pelo_teto": v.Reduziu(),
		"rateio":             d.Fila == "rateio",
		"status":             status,
		"criado_por":         p.UserID,
	}
	if v.Reduziu() {
		linha["valor_antes_do_teto"] = v.ValorOriginal.Float()
	}

	var criados []map[string]any
	if err := m.bd.Inserir(ctx, "orcamentos", []map[string]any{linha}, &criados); err != nil {
		if banco.Duplicado(err) {
			return "", "", fmt.Errorf("já existe um orçamento com esta parte para o ticket %d", t.Ticket)
		}
		return "", "", err
	}
	if len(criados) == 0 {
		return "", "", fmt.Errorf("gravei o orçamento mas o banco não devolveu o id")
	}
	id := fmt.Sprint(criados[0]["id"])

	if err := m.gravarItensDoOrcamento(ctx, id, itensFinais); err != nil {
		return id, aviso, fmt.Errorf("gravei o orçamento mas não os itens: %w", err)
	}

	// O vínculo com a nota é o que impede a MESMA nota de virar orçamento duas
	// vezes no mesmo ticket — o índice único faz o resto.
	if err := m.bd.Inserir(ctx, "orcamento_documentos", []map[string]any{{
		"orcamento_id": id,
		"documento_id": d.ID,
		"parcela":      v.Valor.Float(),
	}}, nil); err != nil {
		return id, aviso, fmt.Errorf("gravei o orçamento mas não o vínculo com a nota: %w", err)
	}

	_ = m.bd.Atualizar(ctx, "documentos", "id=eq."+d.ID, map[string]any{"status": "usado"})
	_ = m.hist.Registrar(ctx, p, "orcamentos", id, "gerar", nil)
	return id, aviso, nil
}

func (m *Modulo) gravarItensDoOrcamento(ctx context.Context, orcamentoID string, itens []regras.Item) error {
	linhas := make([]map[string]any, 0, len(itens))
	for i, it := range itens {
		linhas = append(linhas, map[string]any{
			"orcamento_id":           orcamentoID,
			"ordem":                  i + 1,
			"descricao":              it.Descricao,
			"unidade":                ouNulo(it.Unidade),
			"quantidade":             it.Quantidade.Float(),
			"valor_unitario_nota":    it.UnitarioNota.Float(),
			"valor_unitario_cobrado": it.UnitarioCobrado.Float(),
			"valor_total":            it.Total.Float(),
		})
	}
	if len(linhas) == 0 {
		return nil
	}
	return m.bd.Upsert(ctx, "orcamento_itens?on_conflict=orcamento_id,ordem", linhas, nil)
}

// custosDoTicket soma o que o chamado JÁ tem lançado, de todos os tipos.
//
// De TODOS: um ticket com R$ 400 de Materiais antigo não pode aceitar mais
// R$ 600 de Mão de obra. Contar só o nosso tipo seria um teto que vaza.
func (m *Modulo) custosDoTicket(ctx context.Context, chamadoID string) (regras.Dinheiro, error) {
	var custos []struct {
		Valor *float64 `json:"valor"`
	}
	if err := m.bd.Buscar(ctx, "chamado_custos?chamado_id=eq."+chamadoID+"&select=valor", &custos); err != nil {
		return 0, err
	}
	var soma regras.Dinheiro
	for _, c := range custos {
		if c.Valor != nil {
			soma += regras.DinheiroDe(*c.Valor)
		}
	}
	return soma, nil
}

// proximaParte descobre o próximo número de parte para este ticket.
//
// POR QUE NÃO `max(parte)+1` NO GO
//
//	Porque é exatamente a corrida que o sistema antigo tinha com `PF-{max(id)+1}`:
//	duas requisições simultâneas leem o mesmo máximo e gravam a mesma parte. Aqui
//	o índice único (cliente, ticket, parte) recusa a segunda, e quem chamou
//	recebe erro em vez de gravar por cima. A leitura abaixo é só para acertar na
//	primeira tentativa; a garantia é do banco.
func (m *Modulo) proximaParte(ctx context.Context, clienteID string, ticket int) (int, error) {
	var linhas []struct {
		Parte int `json:"parte"`
	}
	if err := m.bd.Buscar(ctx, "orcamentos?cliente_id=eq."+banco.Escapar(clienteID)+
		"&ticket=eq."+strconv.Itoa(ticket)+"&order=parte.desc&limit=1&select=parte", &linhas); err != nil {
		return 0, err
	}
	if len(linhas) == 0 {
		return 1, nil
	}
	return linhas[0].Parte + 1, nil
}

// ---------------------------------------------------------------------------
// aprovar, apagar, restaurar
// ---------------------------------------------------------------------------

type pedidoDeAprovacao struct {
	Nota string `json:"nota"`
}

// aprovar libera um orçamento que estava parado no teto.
//
// EXIGE UMA JUSTIFICATIVA
//
//	Não por burocracia: porque a aprovação é do CLIENTE, e seis meses depois
//	alguém vai perguntar quem autorizou R$ 1.425 num ticket com teto de R$ 600.
//	Um campo de texto obrigatório é o que transforma "foi aprovado" em "foi
//	aprovado por e-mail do Fulano em tal dia".
func (m *Modulo) aprovar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var pedido pedidoDeAprovacao
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido)
	if strings.TrimSpace(pedido.Nota) == "" {
		web.Falhar(w, http.StatusBadRequest,
			"Escreva onde está a aprovação do cliente (e-mail, print, conversa). Sem isso o orçamento fica sem defesa.")
		return
	}

	filtro := "id=eq." + id + "&cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&status=eq.aguardando_aprovacao"
	if err := m.bd.Atualizar(r.Context(), "orcamentos", filtro, map[string]any{
		"status":         "gerado",
		"aprovado_por":   p.UserID,
		"aprovado_em":    time.Now().UTC().Format(time.RFC3339),
		"aprovacao_nota": strings.TrimSpace(pedido.Nota),
	}); err != nil {
		m.erro(w, "não consegui aprovar", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "aprovar", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

func (m *Modulo) apagarOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	motivo := strings.TrimSpace(r.URL.Query().Get("motivo"))

	// Um orçamento LANÇADO não pode ser apagado pela tela: apagar aqui deixaria
	// o custo vivo no Trílogo e a nossa base dizendo que não existe. Para tirar
	// um lançado é preciso primeiro removê-lo de lá.
	atual, err := m.contarUm(r.Context(), "orcamentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=status&limit=1")
	if err != nil {
		m.erro(w, "não achei este orçamento", err)
		return
	}
	if fmt.Sprint(atual["status"]) == "lancado" {
		web.Falhar(w, http.StatusConflict,
			"Este orçamento já está lançado no Trílogo. Remova o custo de lá antes de apagá-lo aqui.")
		return
	}

	if err := m.bd.Atualizar(r.Context(), "orcamentos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID), map[string]any{
			"status":          "removido",
			"removido_em":     time.Now().UTC().Format(time.RFC3339),
			"removido_por":    p.UserID,
			"removido_motivo": ouNulo(motivo),
		}); err != nil {
		m.erro(w, "não consegui apagar", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "apagar", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// restaurarOrcamento é a frente 2.4.4 da tela de Correções.
//
// Ela é trivial justamente porque nada foi apagado de verdade: o orçamento, os
// itens e o vínculo com a nota continuam na tabela. Restaurar é trocar o status
// de volta.
func (m *Modulo) restaurarOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if err := m.bd.Atualizar(r.Context(), "orcamentos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&status=eq.removido",
		map[string]any{
			"status":          "gerado",
			"removido_em":     nil,
			"removido_por":    nil,
			"removido_motivo": nil,
		}); err != nil {
		m.erro(w, "não consegui restaurar", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "restaurar", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}
