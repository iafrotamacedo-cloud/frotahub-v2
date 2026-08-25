// rev 2 — as quatro frentes de Correções
//
// A tela abre em quatro quadrados, e cada um é uma pergunta diferente:
//
//	sem ticket           a nota foi lida, mas ninguém achou ticket nela
//	sem associação       o ticket foi escrito, mas não existe na nossa base
//	recusados            o Trílogo recusou o lançamento — e por quê
//	apagados             o que foi excluído, e pode voltar
//
// A DICA QUE MUDOU O DESENHO DE "SEM ASSOCIAÇÃO"
//
//	A primeira versão abria pedindo o número do ticket. O dono corrigiu: 90% das
//	vezes o ticket está certo na nossa base e ERRADO na nota — o fornecedor
//	digitou 13O328 com a letra O, ou trocou 130341 por 103341. O robô roda a cada
//	duas horas e não atendemos ticket anterior à data de corte, então "não está
//	na base" é o caso raro.
//
//	Por isso a tela sugere CANDIDATOS por semelhança de número antes de pedir
//	qualquer coisa. O usuário confere; não adivinha.
package orcamentos

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// GET /orcamentos/correcoes — as quatro listas de uma vez.
//
// De uma vez porque a tela mostra as quatro contagens ANTES de o usuário
// escolher uma. Quatro chamadas para pintar quatro números seria fazer o
// navegador esperar quatro vezes pelo mesmo motor que dorme.
func (m *Modulo) correcoes(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	cli := banco.Escapar(p.ClienteID)
	ctx := r.Context()

	// 2.4.1 — notas lidas, sem nenhum ticket amarrado.
	var semTicket []map[string]any
	_ = m.bd.Buscar(ctx, "documentos_lista?cliente_id=eq."+cli+
		"&oculto_em=is.null&status=eq.lido&tickets=eq.0"+
		"&order=inserido_em.desc&limit=500&select=*", &semTicket)

	// 2.4.2 — tickets escritos que não casaram com a nossa base.
	var semAssociacao []map[string]any
	_ = m.bd.Buscar(ctx, "documento_tickets?chamado_id=is.null"+
		"&select=id,ticket,documento_id,documentos!inner(id,nome_arquivo,numero,valor_total,cliente_id,oculto_em)"+
		"&documentos.cliente_id=eq."+cli+"&documentos.oculto_em=is.null"+
		"&order=ticket&limit=500", &semAssociacao)

	// 2.4.3 — os que FORAM TENTADOS e o Trílogo recusou.
	//
	// ANTES ISTO TRAZIA TODO ORÇAMENTO `gerado`, E ERA O DEFEITO DA TELA
	//
	//	Orçamento recém-nascido não é uma correção: é trabalho na fila. Trazendo
	//	os dois juntos, Correções vivia cheia e ninguém distinguia o que ali era
	//	problema de verdade — que é justamente a pergunta que a tela existe para
	//	responder.
	//
	//	`lancamento_bloqueio` só é escrito quando uma tentativa é RECUSADA. Então
	//	quem nunca foi tentado não aparece aqui por construção, não por um filtro
	//	que alguém precisa lembrar de manter.
	//
	//	Eles continuam na fila de lançar, com a marca à vista — o lugar de esperar
	//	é a fila, não a lista de defeitos.
	var recusados []map[string]any
	_ = m.bd.Buscar(ctx, "orcamentos_lista?cliente_id=eq."+cli+
		"&status=eq.gerado&lancamento_bloqueio=not.is.null"+
		"&order=lancamento_tentado_em.desc&limit=500&select=*", &recusados)

	// 2.4.4 — a lixeira que não é lixeira: nada foi apagado de verdade.
	var apagados []map[string]any
	_ = m.bd.Buscar(ctx, "orcamentos_lista?cliente_id=eq."+cli+
		"&status=eq.removido&order=criado_em.desc&limit=500&select=*", &apagados)

	// Aguardando aprovação não é uma das quatro frentes, mas é a fila que mais
	// trava dinheiro. Vai junto para a tela poder mostrá-la sem outra chamada.
	var aguardando []map[string]any
	_ = m.bd.Buscar(ctx, "orcamentos_lista?cliente_id=eq."+cli+
		"&status=eq.aguardando_aprovacao&order=criado_em&limit=500&select=*", &aguardando)

	web.Responder(w, http.StatusOK, map[string]any{
		"sem_ticket":     ouVazio(semTicket),
		"sem_associacao": ouVazio(semAssociacao),
		"recusados":      ouVazio(recusados),
		"apagados":       ouVazio(apagados),
		"aguardando":     ouVazio(aguardando),
	})
}

// ---------------------------------------------------------------------------
// GET /orcamentos/correcoes/candidatos?ticket=13O328
// ---------------------------------------------------------------------------

type candidato struct {
	Numero    int    `json:"numero"`
	Loja      string `json:"loja"`
	Descricao string `json:"descricao"`
	CriadoEm  string `json:"criado_em"`
	Conta     string `json:"conta"`
	// Por que este é candidato, escrito para o usuário ler.
	Porque string `json:"porque"`
	// Quanto mais perto de zero, mais parecido.
	Distancia int `json:"-"`
}

// candidatos sugere chamados parecidos com o número que o fornecedor escreveu.
//
// COMO ELE PROCURA
//
//	Primeiro o número exato — se existir, acabou, e a resposta diz que é só
//	confirmar. Depois as variações que a mão humana e o OCR realmente produzem:
//
//	  troca de um dígito       130328 -> 130828
//	  dois dígitos trocados    130341 -> 103341
//	  um dígito a mais/a menos 1303280 / 13032
//
//	Não é busca por proximidade numérica: 130328 e 130329 são vizinhos no
//	número e não têm nada a ver um com o outro. O que aproxima dois tickets é a
//	FORMA como um vira o outro ao ser digitado errado.
func (m *Modulo) candidatos(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	cru := strings.TrimSpace(r.URL.Query().Get("ticket"))
	// O que o fornecedor escreveu pode ter letra no meio — é justamente o caso
	// do "13O328". Trocamos as letras que o olho confunde e seguimos.
	digitos := desconfundir(cru)
	if len(digitos) < 4 || len(digitos) > 7 {
		web.Falhar(w, http.StatusBadRequest, "Informe um número de ticket com 5 ou 6 dígitos.")
		return
	}

	possiveis := variacoes(digitos)
	if len(possiveis) == 0 {
		web.Responder(w, http.StatusOK, map[string]any{"candidatos": []candidato{}})
		return
	}

	var achados []struct {
		Numero    int     `json:"numero"`
		Conta     string  `json:"conta"`
		Descricao *string `json:"descricao"`
		CriadoEm  *string `json:"criado_em"`
		Unidades  *struct {
			Nome string `json:"nome"`
		} `json:"unidades"`
	}
	_ = m.bd.Buscar(r.Context(), "chamados?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&numero=in.("+strings.Join(possiveis, ",")+")"+
		"&select=numero,conta,descricao,criado_em,unidades(nome)&limit=60", &achados)

	exato, _ := strconv.Atoi(digitos)
	saida := make([]candidato, 0, len(achados))
	for _, a := range achados {
		c := candidato{Numero: a.Numero, Conta: a.Conta}
		if a.Unidades != nil {
			c.Loja = a.Unidades.Nome
		}
		if a.Descricao != nil {
			c.Descricao = encurtar(*a.Descricao, 120)
		}
		if a.CriadoEm != nil {
			c.CriadoEm = *a.CriadoEm
		}
		switch {
		case a.Numero == exato:
			c.Porque = "é exatamente o número informado"
			c.Distancia = 0
		default:
			c.Porque = "parecido com " + cru + " — " + comoDifere(digitos, strconv.Itoa(a.Numero))
			c.Distancia = 1
		}
		saida = append(saida, c)
	}
	sort.SliceStable(saida, func(i, j int) bool { return saida[i].Distancia < saida[j].Distancia })

	web.Responder(w, http.StatusOK, map[string]any{
		"informado":  cru,
		"candidatos": saida,
		// Quando não achou nada, a tela precisa oferecer a OUTRA saída — e não
		// um beco. Atualizar a lista de chamados é o caso raro, mas existe.
		"dica": dicaDaBusca(len(saida)),
	})
}

func dicaDaBusca(quantos int) string {
	if quantos > 0 {
		return "Confira se algum destes é o chamado certo e confirme."
	}
	return "Nenhum chamado parecido na nossa base. " +
		"Se o ticket é recente, rode a atualização do Trílogo e tente de novo; " +
		"se for anterior à data de corte, ele não está no escopo."
}

// desconfundir troca as letras que o olho e o OCR confundem com dígitos. É
// exatamente o erro do "13O328" que o fornecedor manda.
var confusoes = strings.NewReplacer(
	"O", "0", "o", "0",
	"I", "1", "l", "1", "i", "1",
	"S", "5", "s", "5",
	"B", "8",
	"Z", "2", "z", "2",
	" ", "", ".", "", "-", "", "#", "",
)

func desconfundir(s string) string {
	s = confusoes.Replace(s)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// variacoes gera os números que um humano produz errando este.
func variacoes(n string) []string {
	visto := map[string]bool{n: true}
	saida := []string{n}

	guardar := func(v string) {
		v = strings.TrimLeft(v, "0")
		if len(v) < 5 || len(v) > 6 || visto[v] {
			return
		}
		visto[v] = true
		saida = append(saida, v)
	}

	// um dígito trocado
	for i := 0; i < len(n); i++ {
		for d := byte('0'); d <= '9'; d++ {
			if d == n[i] {
				continue
			}
			guardar(n[:i] + string(d) + n[i+1:])
		}
	}
	// dois dígitos vizinhos invertidos
	for i := 0; i+1 < len(n); i++ {
		if n[i] == n[i+1] {
			continue
		}
		guardar(n[:i] + string(n[i+1]) + string(n[i]) + n[i+2:])
	}
	// um dígito a menos
	for i := 0; i < len(n); i++ {
		guardar(n[:i] + n[i+1:])
	}
	// LIMITE
	//   Sem teto, um ticket de 6 dígitos gera ~70 variações — e um `in.(...)`
	//   com 70 números ainda é uma consulta barata. Mas 7 dígitos passariam de
	//   100 e a lista de sugestões deixaria de ajudar: sugestão demais é o mesmo
	//   que nenhuma.
	if len(saida) > 120 {
		saida = saida[:120]
	}
	return saida
}

// comoDifere explica em português por que dois números são parecidos.
func comoDifere(a, b string) string {
	if len(a) != len(b) {
		return "um dígito a mais ou a menos"
	}
	diferentes := 0
	primeiro := -1
	for i := range a {
		if a[i] != b[i] {
			diferentes++
			if primeiro < 0 {
				primeiro = i
			}
		}
	}
	switch diferentes {
	case 1:
		return "um dígito trocado na posição " + strconv.Itoa(primeiro+1)
	case 2:
		return "dois dígitos invertidos"
	default:
		return strconv.Itoa(diferentes) + " dígitos diferentes"
	}
}

// ---------------------------------------------------------------------------
// PATCH do ticket errado — a correção propriamente dita
// ---------------------------------------------------------------------------

type pedidoDeCorrecao struct {
	De   int `json:"de"`
	Para int `json:"para"`
}

// CorrigirTicket troca o número que o fornecedor errou pelo número certo.
//
// CORRIGIR É UPDATE, NUNCA LINHA NOVA
//
//	Regra herdada do sistema antigo, e das poucas que vieram inteiras: o
//	documento é governado pelo id, não pelo ticket. Criar uma linha nova ao
//	corrigir foi o que produziu, lá, o rombo de duplicidade que levou meses para
//	reconciliar.
func (m *Modulo) corrigirTicket(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaCorrecoes)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var pedido pedidoDeCorrecao
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido); err != nil || pedido.Para <= 0 {
		web.Falhar(w, http.StatusBadRequest, "Informe o número correto do ticket.")
		return
	}
	if _, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id&limit=1"); err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}

	// Some o errado, entra o certo — na mesma linha, pelo mesmo caminho que
	// resolve a associação com a nossa base.
	if pedido.De > 0 {
		if err := m.bd.Atualizar(r.Context(), "documento_tickets",
			"documento_id=eq."+id+"&ticket=eq."+strconv.Itoa(pedido.De),
			map[string]any{"ticket": pedido.Para}); err != nil {
			m.erro(w, "não consegui corrigir o ticket", err)
			return
		}
	}
	if err := m.amarrar(r.Context(), p, id, []int{pedido.Para}); err != nil {
		m.erro(w, "não consegui amarrar o ticket corrigido", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "corrigir_ticket", nil)

	var tickets []map[string]any
	_ = m.bd.Buscar(r.Context(), "documento_tickets?documento_id=eq."+id+"&order=ticket&select=*", &tickets)
	web.Responder(w, http.StatusOK, map[string]any{"tickets": ouVazio(tickets)})
}
