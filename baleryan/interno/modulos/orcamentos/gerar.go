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
	// "orcamento" | "rateio" | vazio (as duas). É o que permite ter o botão na
	// própria tela de notas: quem está no rateio gera o rateio, e não mexe na
	// outra fila sem querer.
	Fila string `json:"fila"`
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

	docs, err := m.documentosProntos(r.Context(), p.ClienteID, pedido.Documentos, pedido.Fila)
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

	// ---- a 022 ----
	// Desconto autorizado, em pontos-base do orçamento. Zero é o normal.
	DescontoBP int64 `json:"desconto_bp"`
	// A nota foi mandada para aprovação do cliente: o orçamento nasce com o
	// valor CHEIO e parado, porque é ele que vai ao cliente.
	AprovacaoPedida bool `json:"aprovacao_pedida"`
}

// documentosProntos são as notas lidas, ainda não usadas e não ocultas.
func (m *Modulo) documentosProntos(ctx context.Context, clienteID string, apenas []string, fila string) ([]documentoPronto, error) {
	// A REPETIDA NÃO ENTRA, NEM QUANDO ALGUÉM A ESCOLHE NA MÃO
	//
	//	O filtro vale também para o caminho `apenas` — o "gerar estas aqui" da
	//	tela. Uma trava que o usuário desliga clicando não é trava: é aviso. E o
	//	custo de furá-la é a loja pagando o mesmo material duas vezes, que é a
	//	espécie de erro que ninguém descobre olhando.
	//
	//	Quem quiser gerar mesmo assim tem o caminho certo: abrir a nota, ver as
	//	duas lado a lado, e desfazer a marca com conhecimento de causa.
	filtro := "documentos?cliente_id=eq." + banco.Escapar(clienteID) +
		"&oculto_em=is.null&status=eq.lido&duplicada_de=is.null" +
		"&select=id,nome_arquivo,fila,numero,dav_numero,chave_acesso,emissao,observacao,valor_total,desconto_bp,aprovacao_pedida" +
		"&order=inserido_em"

	if fila == "orcamento" || fila == "rateio" {
		filtro += "&fila=eq." + fila
	}

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

	// A NOTA SÓ PASSA COM TODOS OS TICKETS RESOLVIDOS
	//
	//	Numa nota rateada o valor de cada orçamento depende de QUANTOS tickets a
	//	nota atende. Gerar só para os que casaram e pular o que não casou faria
	//	todos os orçamentos daquela nota saírem com o valor errado — e saírem
	//	parecendo certos, porque cada um fecha a própria conta.
	//
	//	Antes daqui a geração fazia isso: um erro por ticket, e os outros
	//	seguiam. Era o defeito mais caro possível: silencioso, plausível, e só
	//	descoberto conferindo nota por nota meses depois.
	//
	//	A recusa diz QUAIS tickets travaram e para onde ir. Bloqueio sem saída
	//	visível é o que faz o usuário criar planilha paralela.
	if soltos := semChamado(tickets); len(soltos) > 0 {
		base.Erro = fmt.Sprintf(
			"travada: %s não %s na nossa base de chamados. "+
				"Corrija em Correções › Sem associação — enquanto um ticket estiver solto, "+
				"o valor de cada parte desta nota sairia errado.",
			listaDeTickets(soltos), concordancia(len(soltos)))
		return []saidaDaGeracao{base}
	}

	// O QUE JÁ FOI ORÇADO NÃO SE ORÇA DE NOVO — E ISSO NÃO É ERRO
	//
	//	`orcamento_documentos` tem índice único em (documento_id, ticket): é a
	//	trava que impede a loja de pagar o mesmo material duas vezes. Até
	//	26/08/2026 a geração não a consultava, só esbarrava nela — o banco
	//	devolvia 409, o orçamento já tinha nascido, e sobrava um órfão sem
	//	vínculo para alguém limpar na mão.
	//
	//	Perguntar antes custa uma consulta. Descobrir depois custa um registro
	//	pela metade.
	jaOrcados, err := m.ticketsJaOrcados(ctx, d.ID)
	if err != nil {
		base.Erro = "não consegui ver o que esta nota já orçou: " + err.Error()
		return []saidaDaGeracao{base}
	}
	if faltam := semOsJaOrcados(tickets, jaOrcados); len(faltam) != len(tickets) {
		if len(faltam) == 0 {
			base.Erro = "esta nota já virou orçamento em todos os seus tickets"
			return []saidaDaGeracao{base}
		}
		// UMA NOTA RATEADA NÃO SE COMPLETA PELA METADE
		//   O valor de cada parte depende de QUANTOS tickets a nota atende.
		//   Gerar só os que faltam faria essas partes saírem com um valor que
		//   não conversa com as que já existem — e cada uma fecharia a própria
		//   conta, parecendo certa.
		base.Erro = fmt.Sprintf(
			"esta nota já virou orçamento em %s e ainda falta%s %s. "+
				"Apague o que já existe antes de gerar de novo — gerar só o que falta "+
				"daria valores que não conversam entre si.",
			listaDeTickets(jaOrcados), plural(len(faltam), "", "m"), listaDeTickets(numerosDe(faltam)))
		return []saidaDaGeracao{base}
	}

	lidos, err := m.itensDo(ctx, d.ID)
	if err != nil || len(lidos.Linhas) == 0 {
		base.Erro = "esta nota não tem itens lidos"
		return []saidaDaGeracao{base}
	}
	if lidos.Bloqueio != "" {
		if err := m.bd.Atualizar(ctx, "documentos", "id=eq."+d.ID,
			map[string]any{"bloqueio_motivo": lidos.Bloqueio}); err != nil {
			log.Printf("orcamentos: não consegui marcar o bloqueio de %s: %v", d.Nome, err)
		}
		base.Erro = lidos.Bloqueio
		return []saidaDaGeracao{base}
	}
	linhas := lidos.Linhas

	// DECIDIR A NOTA INTEIRA ANTES DE GRAVAR QUALQUER PEDAÇO
	//
	//	O plano calcula, para cada ticket, quanto sairia e se cabe — sem tocar
	//	no banco. Só depois de a nota inteira passar é que o primeiro orçamento
	//	nasce. É o que torna o tudo-ou-nada possível: uma vez gravado, desfazer
	//	orçamento é trabalho no Trílogo, na planilha e no faturamento.
	partes, bloqueio, err := m.planejarNota(ctx, d, tickets, linhas, par)
	if err != nil {
		base.Erro = err.Error()
		return []saidaDaGeracao{base}
	}
	if bloqueio != "" {
		// A frase fica NA NOTA, e não só nesta resposta: a tela de quem vai
		// tratar é outra, e um erro que só existe no retorno de uma chamada
		// morre quando a página recarrega.
		if err := m.bd.Atualizar(ctx, "documentos", "id=eq."+d.ID,
			map[string]any{"bloqueio_motivo": bloqueio}); err != nil {
			log.Printf("orcamentos: não consegui marcar o bloqueio de %s: %v", d.Nome, err)
		}
		base.Erro = bloqueio
		return []saidaDaGeracao{base}
	}

	// A nota passou: se ela estava marcada de antes, a marca sai. Condição
	// muda — o ticket é lançado, o valor é corrigido — e bloqueio que não
	// desaparece sozinho vira entulho que ninguém confia.
	_ = m.bd.Atualizar(ctx, "documentos", "id=eq."+d.ID+"&bloqueio_motivo=not.is.null",
		map[string]any{"bloqueio_motivo": nil})

	saida := make([]saidaDaGeracao, 0, len(partes))
	for _, parte := range partes {
		res := base
		res.Ticket = parte.ticket.Ticket

		id, aviso, err := m.criarOrcamento(ctx, p, d, parte, par)
		switch {
		case err != nil:
			res.Erro = err.Error()
		default:
			res.Orcamento = id
			res.Aviso = juntarAvisos(lidos.Aviso, aviso)
		}
		saida = append(saida, res)
	}
	return saida
}

// ---------------------------------------------------------------------------
// o plano da nota
// ---------------------------------------------------------------------------

// parteDaNota é o que UM ticket vai receber, já decidido.
type parteDaNota struct {
	ticket   ticketDoDocumento
	custo    regras.Custo
	itens    []regras.Item
	veredito regras.Veredito
}

// planejarNota calcula o destino de cada pedaço da nota sem gravar nada.
//
// A REGRA DO DESCONTO VALE POR ORÇAMENTO, NÃO POR NOTA
//
//	O teto é do TICKET — R$ 600 em cada um, não R$ 600 na nota. Então a linha
//	dos R$ 500 também é por ticket: uma nota de R$ 900 dividida entre três
//	tickets são três compras de R$ 300, e as três cabem. Decisão do dono em
//	26/08/2026: "a regra vale para cada orçamento gerado".
//
// E SE UM PEDAÇO NÃO PASSA, A NOTA INTEIRA PARA
//
//	Gerar quatro dos cinco deixaria a nota metade-lançada: o material do quinto
//	ticket já foi comprado e entregue, e não há onde cobrá-lo. Quem fosse tratar
//	teria que desfazer os quatro primeiros para poder refazer a divisão.
func (m *Modulo) planejarNota(ctx context.Context, d documentoPronto,
	tickets []ticketDoDocumento, linhas []regras.LinhaDaNota,
	par regras.Parametros) ([]parteDaNota, string, error) {

	partes, motivos, err := m.avaliarPartes(ctx, d, tickets, linhas, par)
	if err != nil {
		return nil, "", err
	}
	// A PRIMEIRA RECUSA PARA A NOTA INTEIRA
	//   Aqui basta saber que não dá. A tela de tratamento usa a MESMA avaliação
	//   e mostra as recusas todas, porque lá a pergunta é outra: não é "posso
	//   gerar?", é "o que eu preciso consertar?".
	for _, motivo := range motivos {
		if motivo != "" {
			return nil, motivo, nil
		}
	}
	return partes, "", nil
}

// avaliarPartes calcula o destino de CADA pedaço da nota, sem parar no primeiro
// problema e sem gravar nada.
//
// POR QUE ELA NÃO PARA NA PRIMEIRA RECUSA
//
//	Porque tem dois donos. A geração quer saber se a nota roda — e para no
//	primeiro "não". A tela de tratamento quer a lista do que consertar, e uma
//	tela que revela um problema de cada vez faz o usuário voltar cinco vezes
//	para descobrir que a nota tinha cinco.
//
//	As duas leem a mesma avaliação, escrita uma vez (CORE-06). No sistema antigo
//	a conferência da tela e a da geração eram códigos diferentes, e discordavam:
//	a tela dizia "pronta" e a geração recusava, sem ninguém entender por quê.
//
// `motivos` tem o mesmo tamanho de `partes`: vazio quer dizer que aquele pedaço
// passa.
func (m *Modulo) avaliarPartes(ctx context.Context, d documentoPronto,
	tickets []ticketDoDocumento, linhas []regras.LinhaDaNota,
	par regras.Parametros) ([]parteDaNota, []string, error) {

	// O BRUTO SAI DOS ITENS; O PAGO SAI DA NOTA
	//   São dois números diferentes quando há desconto, e é justamente a
	//   diferença entre eles que esta regra existe para tratar.
	comMargem, totalComMargem := regras.MontarItens(linhas, par.MargemBP)
	cheio := somaDasLinhas(linhas)
	pago := regras.DinheiroDe(d.Valor)

	n := len(tickets)
	partes := make([]parteDaNota, 0, n)
	motivos := make([]string, 0, n)

	for i, t := range tickets {
		custo := regras.AplicarDesconto(fatiar(cheio, n, i), fatiar(pago, n, i), par)

		// ALGUÉM ASSINOU EMBAIXO
		//   A autorização não muda a regra: ela diz que, para ESTA nota, se
		//   abre mão da diferença para fechar no teto. O teto do ticket
		//   continua sendo conferido logo abaixo, como para qualquer outra.
		if d.DescontoBP > 0 {
			custo = regras.ComDescontoAutorizado(custo, par)
		}
		// A APROVAÇÃO NÃO ABRE MÃO DE NADA
		//   Ela leva o valor CHEIO ao cliente e pergunta se ele aceita pagar
		//   acima do limite que ele mesmo contratou. O desconto, ao contrário,
		//   é dinheiro nosso. São caminhos diferentes de propósito.
		if d.AprovacaoPedida {
			custo = regras.ComAprovacaoDoCliente(custo)
		}

		// O ALVO DESTE PEDAÇO
		//   Quando o custo foi aparado, o orçamento fecha no TETO exato — é o
		//   que o dono pediu com "ajusta orçamento para o teto, ou seja, 600".
		//   Quando não foi, é a fatia normal do total com margem.
		alvo := fatiar(totalComMargem, n, i)
		if custo.Ajustada() {
			alvo = par.Teto
		}

		itensDaParte := comMargem
		if n > 1 || custo.Ajustada() {
			var errCorte error
			if itensDaParte, errCorte = regras.Encaixar(comMargem, alvo); errCorte != nil {
				return nil, nil, errCorte
			}
		}

		// TICKET SEM CHAMADO NÃO TEM CUSTO PARA SOMAR
		//   E não é caso de erro: é o estado normal de uma nota que está em
		//   "não associadas". Ela precisa ser AVALIADA assim mesmo, para a tela
		//   poder dizer que o problema é a associação e não o teto.
		var jaNoTicket regras.Dinheiro
		if t.ChamadoID != nil {
			var err error
			if jaNoTicket, err = m.custosDoTicket(ctx, *t.ChamadoID); err != nil {
				return nil, nil, fmt.Errorf("não consegui somar os custos do ticket %d: %w", t.Ticket, err)
			}
		}
		veredito := regras.AplicarTeto(alvo, jaNoTicket, par)

		motivo := ""
		switch {
		case t.ChamadoID == nil:
			motivo = fmt.Sprintf("ticket %d — não existe na nossa base de chamados", t.Ticket)

		// A NOTA QUE VAI AO CLIENTE PASSA DE PROPÓSITO
		//
		//	Ela foi mandada para aprovação: o orçamento TEM que nascer, com o
		//	valor cheio, porque é ele o documento que o cliente recebe. Quem o
		//	segura é o status `aguardando_aprovacao`, logo adiante — não este
		//	bloqueio, que o impediria de existir.
		//
		//	O bloqueio por DESCONTO continua valendo: se nem com aprovação a
		//	conta fecha, o problema é outro e a nota volta para tratamento.
		case d.AprovacaoPedida:
			motivo = ""

		default:
			if pode, porque := regras.NotaPodeRodar(custo, []regras.Veredito{veredito}); !pode {
				motivo = fmt.Sprintf("ticket %d — %s", t.Ticket, porque)
			}
		}

		// A folga dos 5% apara aqui, e o orçamento roda com carimbo.
		if motivo == "" && veredito.Reduziu() {
			var errCorte error
			if itensDaParte, errCorte = regras.Encaixar(itensDaParte, veredito.Valor); errCorte != nil {
				return nil, nil, errCorte
			}
		}

		partes = append(partes, parteDaNota{
			ticket: t, custo: custo, itens: itensDaParte, veredito: veredito,
		})
		motivos = append(motivos, motivo)
	}
	return partes, motivos, nil
}

// somaDasLinhas é o bruto da nota: o que os itens dizem que ela vale, antes de
// qualquer desconto.
func somaDasLinhas(linhas []regras.LinhaDaNota) regras.Dinheiro {
	var soma regras.Dinheiro
	for _, l := range linhas {
		soma += regras.Total(l.Quantidade, l.Unitario)
	}
	return soma
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

// semChamado devolve os tickets que não casaram com a nossa base.
func semChamado(tickets []ticketDoDocumento) []int {
	var soltos []int
	for _, t := range tickets {
		if t.ChamadoID == nil {
			soltos = append(soltos, t.Ticket)
		}
	}
	return soltos
}

// listaDeTickets escreve "130899" ou "130708, 130720 e 130899" — porque a
// mensagem é para ser lida por gente (P-18).
func listaDeTickets(ns []int) string {
	txt := make([]string, 0, len(ns))
	for _, n := range ns {
		txt = append(txt, strconv.Itoa(n))
	}
	switch len(txt) {
	case 0:
		return ""
	case 1:
		return "o ticket " + txt[0]
	default:
		return "os tickets " + strings.Join(txt[:len(txt)-1], ", ") + " e " + txt[len(txt)-1]
	}
}

func concordancia(n int) string {
	if n == 1 {
		return "existe"
	}
	return "existem"
}

// ticketsJaOrcados diz em quais tickets esta nota JÁ tem vínculo.
//
// A pergunta é sobre a linha de `orcamento_documentos`, não sobre o status do
// orçamento. É a linha que ocupa a chave única — enquanto ela existir, aquele
// par (nota, ticket) não aceita outro orçamento, mesmo que o dono dela esteja
// marcado como removido. Quem apaga um orçamento tem que soltar o vínculo; é
// exatamente isso que `apagarOrcamento` faz.
func (m *Modulo) ticketsJaOrcados(ctx context.Context, documentoID string) ([]int, error) {
	var linhas []struct {
		Ticket *int `json:"ticket"`
	}
	if err := m.bd.Buscar(ctx, "orcamento_documentos?documento_id=eq."+documentoID+
		"&removido_em=is.null&order=ticket&select=ticket", &linhas); err != nil {
		return nil, err
	}
	saida := make([]int, 0, len(linhas))
	for _, l := range linhas {
		if l.Ticket != nil {
			saida = append(saida, *l.Ticket)
		}
	}
	return saida, nil
}

func semOsJaOrcados(tickets []ticketDoDocumento, jaOrcados []int) []ticketDoDocumento {
	tem := make(map[int]bool, len(jaOrcados))
	for _, t := range jaOrcados {
		tem[t] = true
	}
	faltam := make([]ticketDoDocumento, 0, len(tickets))
	for _, t := range tickets {
		if !tem[t.Ticket] {
			faltam = append(faltam, t)
		}
	}
	return faltam
}

func numerosDe(tickets []ticketDoDocumento) []int {
	ns := make([]int, 0, len(tickets))
	for _, t := range tickets {
		ns = append(ns, t.Ticket)
	}
	return ns
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

// itensLidos é o que a nota gravou, já conferido linha a linha.
//
// A CONFERÊNCIA ACONTECE AQUI, NA FRONTEIRA
//
//	Daqui para dentro tudo é `regras.LinhaDaNota`, e lá a linha vale
//	quantidade × unitário — uma conta só, num lugar só. Para isso valer, o que
//	entra tem que ser coerente. Este é o último ponto em que ainda existem os
//	três números do banco para comparar entre si.
type itensLidos struct {
	Linhas []regras.LinhaDaNota
	// Bloqueio impede a geração: os números da linha não se reconciliam.
	Bloqueio string
	// Aviso gerou, mas alguém precisa saber: o unitário foi recuperado.
	Aviso string
}

func (m *Modulo) itensDo(ctx context.Context, documentoID string) (itensLidos, error) {
	var linhas []struct {
		Ordem      int     `json:"ordem"`
		Descricao  string  `json:"descricao"`
		Unidade    *string `json:"unidade"`
		Quantidade float64 `json:"quantidade"`
		Unitario   float64 `json:"valor_unitario"`
		Total      float64 `json:"valor_total"`
	}
	// O `valor_total` VAI NA BUSCA
	//   Ele é o número que a leitura provou contra o total da nota. Buscar só
	//   quantidade e unitário — como se fazia até 26/08/2026 — é jogar fora a
	//   única linha conferida e recalculá-la a partir de um campo que ninguém
	//   nunca conferiu.
	if err := m.bd.Buscar(ctx, "documento_itens?documento_id=eq."+documentoID+
		"&order=ordem&select=ordem,descricao,unidade,quantidade,valor_unitario,valor_total",
		&linhas); err != nil {
		return itensLidos{}, err
	}

	lidos := itensLidos{Linhas: make([]regras.LinhaDaNota, 0, len(linhas))}
	var recuperados []string
	for _, l := range linhas {
		un := ""
		if l.Unidade != nil {
			un = *l.Unidade
		}
		q := regras.QuantidadeDe(l.Quantidade)
		unitario := regras.PrecoDe(l.Unitario)
		total := regras.DinheiroDe(l.Total)

		if regras.Total(q, unitario) != total {
			certo, deu := regras.PrecoQueFecha(q, total)
			if !deu {
				// SEM PALPITE
				//   Três números que não se reconciliam e nenhum jeito de
				//   recuperar o unitário. Escolher um deles aqui seria inventar
				//   preço. A nota para e alguém olha o papel.
				lidos.Bloqueio = fmt.Sprintf(
					"o item %d (%s) foi lido com números que não fecham: %g × R$ %.2f não dá "+
						"os R$ %.2f da linha. Confira o item na nota antes de gerar.",
					l.Ordem, l.Descricao, l.Quantidade, l.Unitario, l.Total)
				return lidos, nil
			}
			unitario = certo
			recuperados = append(recuperados, strconv.Itoa(l.Ordem))
		}

		lidos.Linhas = append(lidos.Linhas, regras.LinhaDaNota{
			Descricao:  l.Descricao,
			Unidade:    un,
			Quantidade: q,
			Unitario:   unitario,
		})
	}

	if len(recuperados) > 0 {
		// O AVISO NÃO É DECORAÇÃO
		//   O orçamento saiu com o valor certo, mas a leitura errou um campo.
		//   Quem confere a nota precisa saber qual linha veio torta do papel.
		lidos.Aviso = fmt.Sprintf(
			"o preço unitário d%s ite%s %s foi recuperado do total da linha — a leitura não o trouxe",
			plural(len(recuperados), "o", "os"), plural(len(recuperados), "m", "ns"),
			strings.Join(recuperados, ", "))
	}
	return lidos, nil
}

// juntarAvisos não deixa um aviso apagar o outro: os dois falam de coisas
// diferentes e quem lê precisa dos dois.
func juntarAvisos(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " · " + b
}

func plural(n int, um, varios string) string {
	if n == 1 {
		return um
	}
	return varios
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
	d documentoPronto, parte parteDaNota, par regras.Parametros) (string, string, error) {

	t, v, custo := parte.ticket, parte.veredito, parte.custo

	aviso := ""
	if v.Reduziu() {
		aviso = fmt.Sprintf("reduzido de %s para %s: o ticket já tinha %s e o teto é %s",
			v.ValorOriginal.Reais(), v.Valor.Reais(), v.JaNoTicket.Reais(), v.Teto.Reais())
	}
	// O CARIMBO EXPLICA O CORTE QUE NÃO TEM OUTRA EXPLICAÇÃO
	//   Sem esta frase, alguém abre um orçamento de R$ 600 para uma nota de
	//   R$ 700 e não tem como saber se foi regra, erro de digitação ou fraude.
	if custo.Ajustada() {
		aviso = fmt.Sprintf(
			"ajustada por conta do teto: a nota soma %s, pagamos %s, e o máximo que cabe no teto de %s é %s",
			custo.Cheio.Reais(), custo.ComDesconto.Reais(), par.Teto.Reais(), custo.BaseMaxima.Reais())
	}

	// O ORÇAMENTO ACIMA DO TETO NASCE PARADO
	//
	//	Eu tinha tirado esta decisão quando `planejarNota` passou a barrar antes
	//	de chegar aqui — e estava certo enquanto NADA podia passar do teto. Com
	//	o "mandar para aprovação", volta a haver um caminho legítimo até este
	//	ponto com o valor cheio, e sem isto o orçamento nasceria `gerado`:
	//	pronto para ser lançado sem ninguém ter aprovado nada.
	status := "gerado"
	if v.Decisao == regras.Aprovacao {
		status = "aguardando_aprovacao"
		aviso = fmt.Sprintf("parado para aprovação: %s neste orçamento mais %s já no ticket passam do teto de %s",
			v.ValorOriginal.Reais(), v.JaNoTicket.Reais(), v.Teto.Reais())
	}

	parteN, err := m.proximaParte(ctx, p.ClienteID, t.Ticket)
	if err != nil {
		return "", "", err
	}

	chamado, err := m.contarUm(ctx, "chamados?id=eq."+*t.ChamadoID+
		"&select=unidade_id,conta&limit=1")
	if err != nil {
		return "", "", err
	}

	linha := map[string]any{
		"cliente_id": p.ClienteID,
		"ticket":     t.Ticket,
		"parte":      parteN,
		"chamado_id": *t.ChamadoID,
		"unidade_id": chamado["unidade_id"],
		"conta":      chamado["conta"],
		// O QUE SE PAGOU E O QUE A NOTA SOMA, LADO A LADO
		//   Iguais quando não houve desconto. Diferentes, contam a história
		//   inteira sem ninguém precisar reconstruir a conta.
		"valor_nota":         custo.ComDesconto.Float(),
		"valor_nota_cheio":   custo.Cheio.Float(),
		"valor":              v.Valor.Float(),
		"margem_aplicada":    float64(par.MargemBP) / 10000,
		"teto_aplicado":      v.Teto.Float(),
		"reduzido_pelo_teto": v.Reduziu(),
		"ajustado_pelo_teto": custo.Ajustada(),
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

	if err := m.gravarItensDoOrcamento(ctx, id, parte.itens); err != nil {
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
	// A JUSTIFICATIVA DEIXOU DE SER OBRIGATÓRIA EM 26/08/2026
	//
	//	Decisão do dono: registrar quem e quando basta. O campo continua
	//	existindo e continua sendo gravado quando alguém escreve — tirar a
	//	COLUNA seria destruir a defesa dos orçamentos que já a têm, e a de quem
	//	quiser escrever amanhã.
	//
	//	Vale dizer o que se perde: sem o texto, seis meses depois o registro diz
	//	que fulano clicou, não em que fulano se baseou. Se um dia a discussão
	//	for com o cliente, é a diferença entre "foi aprovado" e "foi aprovado
	//	por e-mail do Beltrano em tal dia".

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

	// QUAL NOTA ESTE ORÇAMENTO PRENDE — PERGUNTADO ANTES DE SOLTAR
	//   Depois de apagar o vínculo não há mais como descobrir, e a nota ficaria
	//   presa em `usado` sem ninguém saber qual era.
	notas, err := m.notasDoOrcamento(r.Context(), id)
	if err != nil {
		m.erro(w, "não consegui ver a que nota este orçamento pertence", err)
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

	// SOLTAR O VÍNCULO É PARTE DE APAGAR, NÃO UM EXTRA
	//
	//	O índice único de `orcamento_documentos` é (documento_id, ticket) e não
	//	olha o status do orçamento. Marcar o orçamento como removido e deixar o
	//	vínculo de pé tornava aquela nota impossível de reorçar NAQUELE ticket
	//	para sempre — o banco recusava com 409, e o orçamento novo já tinha
	//	nascido quando a recusa chegava. Sobrava um órfão.
	//
	//	Aconteceu em 26/08/2026, ao refazer o orçamento da DAV 19329.
	//
	//	A linha é MARCADA, não apagada (migração 025): o índice só enxerga as
	//	vivas, então a chave se libera, e restaurar continua sabendo de qual
	//	nota este orçamento veio.
	if err := m.bd.Atualizar(r.Context(), "orcamento_documentos",
		"orcamento_id=eq."+id+"&removido_em=is.null",
		map[string]any{"removido_em": time.Now().UTC().Format(time.RFC3339)}); err != nil {
		m.erro(w, "apaguei o orçamento mas não consegui soltar a nota", err)
		return
	}

	// E A NOTA VOLTA PARA A FILA
	//   Gerar virou o documento de `lido` para `usado`, e é o `usado` que o tira
	//   da lista de prontas. Apagar sem desfazer isso deixa a nota invisível:
	//   sem orçamento e fora da fila, que é o pior dos dois mundos.
	for _, n := range notas {
		if err := m.bd.Atualizar(r.Context(), "documentos",
			"id=eq."+n+"&status=eq.usado", map[string]any{"status": "lido"}); err != nil {
			log.Printf("orcamentos: apaguei %s mas não devolvi a nota %s para a fila: %v", id, n, err)
		}
	}

	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "apagar", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "notas": notas})
}

// notasDoOrcamento lista as notas que este orçamento consumiu.
//
// É plural porque a tabela permite: um orçamento pode nascer de mais de uma
// nota. Hoje nenhum nasce, mas escrever no singular aqui seria embutir na
// escrita de um lado uma regra que o outro não tem.
func (m *Modulo) notasDoOrcamento(ctx context.Context, orcamentoID string) ([]string, error) {
	var linhas []struct {
		Documento string `json:"documento_id"`
	}
	if err := m.bd.Buscar(ctx, "orcamento_documentos?orcamento_id=eq."+orcamentoID+
		"&select=documento_id", &linhas); err != nil {
		return nil, err
	}
	notas := make([]string, 0, len(linhas))
	for _, l := range linhas {
		notas = append(notas, l.Documento)
	}
	return notas, nil
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
	// O VÍNCULO VOLTA PRIMEIRO — É ELE QUE PODE RECUSAR
	//
	//	Enquanto o orçamento esteve apagado, a nota pode ter virado outro
	//	orçamento no mesmo ticket. Aí a chave está ocupada e restaurar este aqui
	//	deixaria DOIS orçamentos vivos para o mesmo par — a loja pagando duas
	//	vezes, que é exatamente o que o índice existe para impedir.
	//
	//	Tentando o vínculo antes do status, uma recusa deixa tudo como estava.
	//	Na ordem inversa, o orçamento voltaria a valer sem nota atrás.
	notas, err := m.notasDoOrcamento(r.Context(), id)
	if err != nil {
		m.erro(w, "não consegui ver a que nota este orçamento pertence", err)
		return
	}
	if err := m.bd.Atualizar(r.Context(), "orcamento_documentos",
		"orcamento_id=eq."+id, map[string]any{"removido_em": nil}); err != nil {
		web.Falhar(w, http.StatusConflict,
			"Não dá para restaurar: a nota já virou outro orçamento neste ticket. "+
				"Apague o novo antes, ou deixe este como está.")
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

	// E A NOTA SAI DA FILA DE NOVO
	//   Apagar devolveu ela para `lido`. Restaurar sem desfazer isso a deixaria
	//   pronta para gerar um segundo orçamento que o vínculo agora recusaria.
	for _, n := range notas {
		if err := m.bd.Atualizar(r.Context(), "documentos",
			"id=eq."+n+"&status=eq.lido", map[string]any{"status": "usado"}); err != nil {
			log.Printf("orcamentos: restaurei %s mas não tirei a nota %s da fila: %v", id, n, err)
		}
	}

	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "restaurar", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}
