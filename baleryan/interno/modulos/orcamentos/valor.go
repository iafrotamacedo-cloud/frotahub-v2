// rev 1 — a pergunta do valor
//
// POR QUE UMA PERGUNTA, E NÃO UM AVISO
//
//	A tela pedia "confira" e não dizia o quê. Quando passou a dizer, dizia "a
//	chave de acesso não foi lida" — específico e inútil: a chave tem 44 dígitos,
//	ninguém a digita, ela não entra no orçamento e não impede nada.
//
//	O que a pessoa consegue conferir olhando o papel é o VALOR. Então o sistema
//	pergunta o valor, e ela responde sim ou não.
//
// A RESPOSTA FICA GRAVADA
//
//	`valor_conferido_em` marca que alguém olhou. Pergunta respondida não volta —
//	senão a tela pediria a mesma conferência amanhã, e aí ninguém responde
//	nenhuma.
package orcamentos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

type respostaDoValor struct {
	// Confere é a resposta à pergunta: o valor lido está certo?
	Confere bool `json:"confere"`
	// Valor é o que a pessoa leu no papel, quando `confere` é falso.
	Valor float64 `json:"valor"`
}

func (m *Modulo) conferirValor(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaNotas)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	var pedido respostaDoValor
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi a resposta.")
		return
	}

	atual, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=valor_total,status&limit=1")
	if err != nil {
		m.erro(w, "não achei esta nota", err)
		return
	}
	// UMA NOTA JÁ USADA NÃO SE CORRIGE POR AQUI
	//   O orçamento dela já nasceu do valor antigo. Mudar o valor da nota agora
	//   faria o registro discordar do documento que já saiu — o conserto certo é
	//   apagar o orçamento, que devolve a nota para a fila.
	if fmt.Sprint(atual["status"]) == "usado" {
		web.Falhar(w, http.StatusConflict,
			"Esta nota já virou orçamento. Para mudar o valor, apague o orçamento primeiro — a nota volta para a fila.")
		return
	}

	campos := map[string]any{
		"valor_conferido_em":  time.Now().UTC().Format(time.RFC3339),
		"valor_conferido_por": p.UserID,
	}
	acao := "confirmar_valor"

	if !pedido.Confere {
		// VALOR CORRIGIDO É VALOR QUE A PESSOA LEU NO PAPEL
		//   Zero não é correção, é engano de digitação — e um total zerado faria
		//   a nota virar orçamento de graça.
		novo := regras.DinheiroDe(pedido.Valor)
		if novo <= 0 {
			web.Falhar(w, http.StatusBadRequest,
				"Diga qual é o valor certo da nota — ele precisa ser maior que zero.")
			return
		}
		campos["valor_total"] = novo.Float()
		acao = "corrigir_valor"
	}

	if err := m.bd.Atualizar(r.Context(), "documentos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID), campos); err != nil {
		m.erro(w, "não consegui gravar a conferência", err)
		return
	}

	// O QUE MUDOU FICA REGISTRADO COM O NOME DE QUEM MUDOU
	//   Valor de nota corrigido à mão é a coisa mais sensível que esta tela faz:
	//   ele vira o custo do orçamento. Daqui a seis meses alguém vai perguntar
	//   por que aquela nota vale isso.
	var mudancas map[string]historico.Mudanca
	if !pedido.Confere {
		mudancas = map[string]historico.Mudanca{
			"valor_total": {De: atual["valor_total"], Para: campos["valor_total"]},
		}
	}
	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, acao, mudancas)

	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}
