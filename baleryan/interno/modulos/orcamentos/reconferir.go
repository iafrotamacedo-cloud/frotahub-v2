// rev 1 — reconferir o status do chamado no Trílogo
//
// POR QUE ESTE BOTÃO PRECISA EXISTIR
//
//	A marca de bloqueio descreve a ÚLTIMA tentativa, e o mundo anda depois dela.
//	Medimos: dos 26 orçamentos que o sistema antigo marcou como travados por
//	status em 19/08, SETE já estavam liberados seis dias depois — o chamado foi
//	executado e ninguém percebeu, porque nada aqui olhava de novo.
//
//	Sem este botão, a marca velha vira mentira: a tela diz "travado" para um
//	orçamento que subiria na primeira tentativa.
//
// ELE RECONFERE O TICKET, NÃO O ORÇAMENTO
//
//	O status é do chamado. Um ticket com três orçamentos parados destrava os três
//	de uma vez — reconferir um por um seria três idas ao Trílogo para responder
//	a mesma pergunta.
//
// E ELE NÃO LANÇA
//
//	Reconferir é ler. Se o chamado andou, a marca some e o orçamento volta a
//	aparecer como pronto; quem manda subir continua sendo a pessoa. Um botão que
//	diz "conferir" e lança seria um botão que mente sobre o que faz.
package orcamentos

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// POST /orcamentos/ficha/{id}/reconferir
func (m *Modulo) reconferir(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if !m.cfg.Trilogo.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"As contas do Trílogo não estão configuradas no servidor.")
		return
	}

	orc, err := m.contarUm(r.Context(), "orcamentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=*&limit=1")
	if err != nil {
		m.erro(w, "não achei este orçamento", err)
		return
	}
	ticket := int(numeroDe(orc["ticket"]))
	conta := fmt.Sprint(orc["conta"])

	sessao, err := m.entrarNoTrilogo(r.Context(), conta)
	if err != nil {
		m.erro(w, "não consegui entrar no Trílogo", err)
		return
	}
	d, err := sessao.Detalhe(r.Context(), ticket)
	if err != nil {
		m.erro(w, "não consegui ler o chamado no Trílogo", err)
		return
	}
	if d == nil || d.ID != ticket {
		web.Falhar(w, http.StatusConflict, fmt.Sprintf(
			"A conta %s não devolveu o chamado %d. Confira o número e a conta.", conta, ticket))
		return
	}

	// O ESPELHO É ATUALIZADO JUNTO
	//
	//	As listas e o `destino` da view saem de `chamados`, não do Trílogo. Se só
	//	a marca do orçamento sumisse, a tela continuaria mandando aquele ticket
	//	para a lista do cliente — dois lugares com respostas diferentes sobre a
	//	mesma pergunta, que é o defeito que o CORE-06 existe para evitar.
	//
	//	Falhar aqui não interrompe: a resposta ao usuário é sobre o Trílogo, e
	//	trocá-la por um erro de banco esconderia o diagnóstico verdadeiro.
	espelho := map[string]any{
		"status_codigo": d.Status,
		"status":        rotuloDoStatus(d.Status),
		"lido_em":       time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.bd.Atualizar(r.Context(), "chamados",
		"numero=eq."+strconv.Itoa(ticket)+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
		espelho); err != nil {
		log.Printf("orcamentos: reconferir não atualizou o espelho do chamado %d: %v", ticket, err)
	}

	nome, travado := statusQueRecusa[d.Status]
	if travado {
		// Continua travado. A marca é REESCRITA, não mantida: o motivo pode ter
		// mudado de Aberto para Arquivado, e a data tem que dizer que a conferência
		// é de agora.
		det := fmt.Sprintf("o Trílogo não aceita custo em chamado %s (status %d)", nome, d.Status)
		campos := map[string]any{
			"lancamento_bloqueio":         "ticket_status",
			"lancamento_bloqueio_detalhe": det,
			"lancamento_tentado_em":       time.Now().UTC().Format(time.RFC3339),
		}
		_ = m.bd.Atualizar(r.Context(), "orcamentos",
			"cliente_id=eq."+banco.Escapar(p.ClienteID)+"&ticket=eq."+strconv.Itoa(ticket)+
				"&status=eq.gerado", campos)
		web.Responder(w, http.StatusOK, map[string]any{
			"liberou":       false,
			"ticket":        ticket,
			"ticket_status": nome,
			"mensagem": fmt.Sprintf("O chamado %d continua %s. Quem resolve: %s.",
				ticket, nome, quemResolve(d.Status)),
		})
		return
	}

	// Liberou. A marca some de TODOS os orçamentos gerados daquele ticket, e as
	// tentativas ficam — elas são o histórico de quantas vezes esbarramos.
	limpar := map[string]any{
		"lancamento_bloqueio":         nil,
		"lancamento_bloqueio_detalhe": nil,
	}
	if err := m.bd.Atualizar(r.Context(), "orcamentos",
		"cliente_id=eq."+banco.Escapar(p.ClienteID)+"&ticket=eq."+strconv.Itoa(ticket)+
			"&status=eq.gerado", limpar); err != nil {
		m.erro(w, "o chamado liberou, mas não consegui tirar a marca", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"liberou":       true,
		"ticket":        ticket,
		"ticket_status": rotuloDoStatus(d.Status),
		"mensagem": fmt.Sprintf("O chamado %d está %s. Pode lançar.",
			ticket, rotuloDoStatus(d.Status)),
	})
}

// rotuloDoStatus traduz o código do Trílogo.
//
// CÓDIGO DESCONHECIDO NÃO VIRA VAZIO
//
//	Ele aparece como "status 9", visível, para alguém perceber. É a mesma regra
//	do robô — e foi assim que a prioridade invertida do sistema antigo sobreviveu
//	tanto tempo: traduzida para nada, ninguém via.
func rotuloDoStatus(codigo int) string {
	switch codigo {
	case 1:
		return "Aberto"
	case 3:
		return "Arquivado"
	case 5:
		return "Executado"
	case 6:
		return "Vistoriado"
	case 7:
		return "Em execução"
	}
	return "status " + strconv.Itoa(codigo)
}
