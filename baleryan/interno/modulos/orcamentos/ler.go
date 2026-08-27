// rev 1 — ler a nota na hora, sem esperar o robô
//
// O PROBLEMA QUE ISTO RESOLVE
//
//	A nota só pode virar orçamento depois de lida — a regra está na visão
//	(`pronto_para_gerar` exige `status = 'lido'`) e no filtro da geração. Até
//	aqui, quem lia era só o robô do GitHub, de trinta em trinta minutos, e nas
//	horas em que ele roda. Quem inseriu a nota ficava olhando uma fila cheia de
//	"na fila" sem nada para fazer a respeito.
//
//	Decisão do dono em 27/08/2026: "o motor le na hora, opcao A, e tira a
//	leitura de 30 em 30 minutos do git".
//
// CADA NOTA É LIDA UMA VEZ, E ISSO É GARANTIDO PELA FILA — NÃO PELA BOA VONTADE
//
//	O trabalho `ler_documento` continua sendo a única autorização para ler. Esta
//	rota o TOMA do mesmo jeito que o robô toma: alteração condicional com
//	`status=eq.na_fila` no filtro. Duas pontas que disputam a mesma linha, só
//	uma recebe a linha de volta.
//
//	Sem isso, um clique no botão enquanto o robô roda leria a mesma nota duas
//	vezes — duas chamadas de IA pagas, e a última a gravar decidindo o que a
//	nota diz.
//
// SEM `pdftotext` AQUI, E ISSO NÃO QUEBRA NADA
//
//	A camada 2 da cascata chama `pdftotext`, que existe no Actions e não no
//	contêiner do motor. `textoDePDF` devolve string vazia quando o programa não
//	está lá — o mesmo silêncio de um PDF escaneado — e a leitura segue para a
//	IA. XML continua exato dos dois lados. O que muda é a conta da cota: pelo
//	motor, o PDF digital também passa pelo modelo.
package orcamentos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitura"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TetoDaLeitura limita quantas notas o botão recebe de uma vez.
//
//	Não é medo do banco: é que a pessoa fica olhando a barra andar. Quinhentas
//	notas a uma chamada de IA cada são meia hora de tela parada. Quando o corte
//	acontece, a tela DIZ que aconteceu — corte silencioso é pior que corte.
const TetoDaLeitura = 500

// GET /orcamentos/documentos/porler — o que o botão "Ler notas" vai ler.
//
// POR QUE A LISTA VEM DO MOTOR, E NÃO DA PÁGINA QUE ESTÁ NA TELA
//
//	A tela mostra cem por vez. Se o botão lesse só o que está à vista, quem tem
//	noventa e uma notas leria cem, veria a fila continuar cheia e não teria como
//	saber que faltava paginar. O botão precisa saber o total DE VERDADE — é a
//	diferença entre "12 notas" e "12 nesta página".
func (m *Modulo) documentosPorLer(w http.ResponseWriter, r *http.Request) {
	fila := filaPedida(r)
	p := m.quem(w, r, rotinaDaFila(fila))
	if p == nil {
		return
	}
	var linhas []map[string]any
	total, err := m.bd.BuscarContando(r.Context(), "documentos?cliente_id=eq."+
		banco.Escapar(p.ClienteID)+"&fila=eq."+fila+"&oculto_em=is.null"+
		"&status=in.(inserido,falhou)&order=inserido_em"+
		"&select=id,nome_arquivo,status&limit="+strconv.Itoa(TetoDaLeitura), &linhas)
	if err != nil {
		m.erro(w, "não consegui listar as notas por ler", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"notas": ouVazio(linhas),
		"total": total,
		"teto":  TetoDaLeitura,
	})
}

// POST /orcamentos/documentos/{id}/ler
func (m *Modulo) lerDocumento(w http.ResponseWriter, r *http.Request) {
	fila := filaPedida(r)
	p := m.quem(w, r, rotinaDaFila(fila))
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	doc, err := m.contarUm(r.Context(), "documentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,fila,status,oculto_em,nome_arquivo&limit=1")
	if err != nil {
		m.erro(w, "não achei este documento", err)
		return
	}

	// A FILA DECLARADA TEM QUE SER A FILA DE VERDADE
	//   A permissão foi conferida pela fila que veio no endereço. Se a nota
	//   fosse de outra, quem tem só "Notas e DAVs" leria uma nota de rateio —
	//   pela porta dos fundos, com a permissão da porta da frente.
	if fmt.Sprint(doc["fila"]) != fila {
		web.Falhar(w, http.StatusBadRequest, "Esta nota não é desta fila.")
		return
	}
	if doc["oculto_em"] != nil {
		web.Falhar(w, http.StatusConflict, "Esta nota está na lixeira. Restaure-a antes de ler.")
		return
	}

	if recusa, pode := podeLerAgora(fmt.Sprint(doc["status"])); !pode {
		web.Falhar(w, http.StatusConflict, recusa)
		return
	}

	// A PERGUNTA VEM ANTES DE TOMAR O TRABALHO
	//
	//	Se a chave da IA não estiver configurada neste servidor, a cascata
	//	falharia lá dentro e a nota voltaria marcada `falhou` — noventa notas
	//	acusadas de um problema que é do servidor. Perguntando antes, ninguém é
	//	marcado e a mensagem diz onde está a saída.
	if falta := m.leitura.FaltaOLeitor(fmt.Sprint(doc["nome_arquivo"])); falta != "" {
		web.Falhar(w, http.StatusServiceUnavailable, "Não dá para ler por aqui: "+falta+
			". O robô do GitHub (Actions › Leitor de notas) continua dando conta.")
		return
	}

	trabalho, livre := m.tomarALeitura(r.Context(), id)
	if !livre {
		web.Falhar(w, http.StatusConflict,
			"Esta nota já está sendo lida agora. Espere terminar e atualize a lista.")
		return
	}

	res, err := m.leitura.Ler(r.Context(), id)
	if err != nil {
		m.leituraFalhou(r.Context(), id, trabalho, err)
		var temporaria leitura.FalhaTemporaria
		if errors.As(err, &temporaria) {
			web.Falhar(w, http.StatusServiceUnavailable,
				"Não consegui ler esta nota agora: "+leitor.EmPortugues(err.Error())+
					" Tente de novo em instantes.")
			return
		}
		web.Falhar(w, http.StatusUnprocessableEntity,
			"Não consegui ler esta nota: "+leitor.EmPortugues(err.Error()))
		return
	}
	m.fecharALeitura(r.Context(), trabalho)

	web.Responder(w, http.StatusOK, map[string]any{"leitura": res})
}

// podeLerAgora diz se a nota aceita uma leitura, e por que não quando não aceita.
//
// "CADA NOTA É LIDA APENAS UMA VEZ" — DECISÃO DO DONO, 27/08/2026
//
//	Ler de novo uma nota já lida não é só desperdício de cota: é apagar o que
//	alguém pode ter corrigido à mão em cima da leitura anterior. E `usado` é
//	pior — a nota já virou orçamento, e mudar os itens dela por baixo faria a
//	conta do orçamento discordar da nota que a originou.
//
//	`falhou` é o contrário: a leitura NÃO aconteceu. A nota continua sem número
//	e sem itens, e tentar de novo é o único caminho que existe para ela — é
//	inclusive o estado em que `leituraFalhou` deixa a nota logo ali embaixo.
//
// É função à parte porque é a decisão inteira desta rota, e decisão que só dá
// para testar subindo um banco não é testada (P-30).
func podeLerAgora(status string) (string, bool) {
	switch status {
	case "inserido", "falhou":
		return "", true
	case "lido":
		return "Esta nota já foi lida.", false
	case "usado":
		return "Esta nota já virou orçamento.", false
	default:
		return "Esta nota não está em condição de ser lida agora.", false
	}
}

// tomarALeitura pega o trabalho da fila, se ele estiver livre.
//
// O SEGUNDO RETORNO NÃO É "ACHEI O TRABALHO"
//
//	É "pode ler". Nota sem trabalho nenhum na fila — inserida antes de a fila
//	existir, ou com o trabalho já fechado por uma corrida que não gravou o
//	status — devolve `("", true)`: pode ler, e não há nada para fechar depois.
//	Só o trabalho que está `rodando` na mão de outra ponta devolve `false`.
func (m *Modulo) tomarALeitura(ctx context.Context, documentoID string) (string, bool) {
	type trabalho struct {
		ID string `json:"id"`
	}
	var tomados []trabalho
	err := m.bd.AtualizarDevolvendo(ctx, "jobs",
		"tipo=eq.ler_documento&alvo_id=eq."+documentoID+"&status=eq.na_fila",
		map[string]any{
			"status":     "rodando",
			"comecou_em": time.Now().UTC().Format(time.RFC3339),
			"tomado_por": "motor",
		}, &tomados)
	if err == nil && len(tomados) > 0 {
		return tomados[0].ID, true
	}
	// Não tomei. Ou não existe trabalho, ou ele está na mão de alguém.
	var rodando []trabalho
	if e := m.bd.Buscar(ctx, "jobs?tipo=eq.ler_documento&alvo_id=eq."+documentoID+
		"&status=eq.rodando&select=id&limit=1", &rodando); e == nil && len(rodando) > 0 {
		return "", false
	}
	return "", true
}

func (m *Modulo) fecharALeitura(ctx context.Context, trabalho string) {
	if trabalho == "" {
		return
	}
	_ = m.bd.Atualizar(ctx, "jobs", "id=eq."+trabalho, map[string]any{
		"status":      "concluido",
		"terminou_em": time.Now().UTC().Format(time.RFC3339),
		"erro":        nil,
	})
}

// leituraFalhou devolve o trabalho para a fila e conta o motivo na tela.
//
// POR QUE O TRABALHO VOLTA PARA A FILA EM VEZ DE MORRER
//
//	Porque quem clicou está ali, e vai clicar de novo. A contagem de tentativas
//	do robô existe para o lote, onde ninguém está olhando e uma nota ruim viraria
//	laço infinito; aqui o freio é a pessoa. Deixar o trabalho fechado como
//	"falhou" tiraria a nota do alcance dos dois — do botão e do robô.
//
//	A nota fica marcada `falhou` com o motivo em português, porque é isso que a
//	lista mostra. E `falhou` é justamente o estado que esta rota aceita de volta.
func (m *Modulo) leituraFalhou(ctx context.Context, documentoID, trabalho string, causa error) {
	if trabalho != "" {
		_ = m.bd.Atualizar(ctx, "jobs", "id=eq."+trabalho, map[string]any{
			"status":     "na_fila",
			"comecou_em": nil,
			"tomado_por": nil,
			"erro":       causa.Error(),
		})
	}
	_ = m.bd.Atualizar(ctx, "documentos", "id=eq."+documentoID, map[string]any{
		"status":       "falhou",
		"leitura_erro": leitor.EmPortugues(causa.Error()),
	})
}
