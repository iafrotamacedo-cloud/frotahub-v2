// rev 1 — o avanço automático de Execução e Vistoria
//
// PARTE DO JOB AGENDADO (cmd/servicos), NÃO DO ROBÔ DO TRÍLOGO
//
//	Roda 5 minutos depois de robo-trilogo.yml (ver .github/workflows/
//	servicos.yml) — os marcos que esta função olha (status_codigo,
//	executado_em, vistoriado_em) já foram lidos e gravados em `chamados` pela
//	rodada do contrato. Nenhuma sessão no Trílogo é aberta aqui.
//
// POR MARCO PERSISTIDO, NÃO SÓ POR status_codigo
//
//	`status_codigo` é "onde o chamado está AGORA" — pode voltar a mudar. Os
//	marcos (`executado_em`, `vistoriado_em`) são carimbos que o Trílogo grava
//	uma vez e não regridem (ver trilogo/robo.go, comentário de `alterado_em`).
//	São o sinal PRIMÁRIO; `status_codigo` entra como sinal secundário só onde
//	não existe marco próprio (7 = "Em execução" não tem um `_em` dedicado).
package servicos

import (
	"context"
	"fmt"
	"log"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
)

type itemDeExecucao struct {
	ID       string `json:"id"`
	Ticket   int    `json:"ticket"`
	Status   string `json:"status"`
	Chamados struct {
		StatusCodigo *int    `json:"status_codigo"`
		ExecutadoEm  *string `json:"executado_em"`
		VistoriadoEm *string `json:"vistoriado_em"`
	} `json:"chamados"`
}

// proximoPorMarco decide se um item, no status em que está, já tem o marco
// que manda ele andar — devolve "" quando não tem nada novo.
func proximoPorMarco(status string, c struct {
	StatusCodigo *int    `json:"status_codigo"`
	ExecutadoEm  *string `json:"executado_em"`
	VistoriadoEm *string `json:"vistoriado_em"`
}) string {
	codigo := 0
	if c.StatusCodigo != nil {
		codigo = *c.StatusCodigo
	}
	switch status {
	case StatusAprovadoExecucao:
		if codigo == 7 || c.ExecutadoEm != nil {
			return StatusEmExecucao
		}
	case StatusEmExecucao:
		if c.ExecutadoEm != nil || codigo == 5 {
			return StatusFinalizado
		}
	case StatusFinalizado:
		if c.VistoriadoEm != nil || codigo == 6 {
			return StatusAguardandoFaturamento
		}
	}
	return ""
}

// RodarDeteccaoDeAvanco varre os cards de Execução e avança sozinho quem já
// tem o marco correspondente lido pelo robô do contrato. Um item pode andar
// MAIS DE UMA coluna na mesma rodada — se a leitura de 2h em 2h só alcançou
// o chamado depois de ele já estar executado E vistoriado, ele passa pelas
// três transições aqui, cada uma virando sua própria linha de histórico
// (não um buraco na auditoria).
func (s *Servico) RodarDeteccaoDeAvanco(ctx context.Context, clienteID string) (avancados int, err error) {
	var itens []itemDeExecucao
	caminho := "servicos_orcamentos?cliente_id=eq." + banco.Escapar(clienteID) +
		"&status=in.(" + listaEntreParenteses([]string{StatusAprovadoExecucao, StatusEmExecucao, StatusFinalizado}) + ")" +
		"&removido_em=is.null" +
		"&select=id,ticket,status,chamados(status_codigo,executado_em,vistoriado_em)"
	if err := s.bd.Buscar(ctx, caminho, &itens); err != nil {
		return 0, fmt.Errorf("lendo os cards de execução: %w", err)
	}

	for _, it := range itens {
		atual := it.Status
		for {
			proximo := proximoPorMarco(atual, it.Chamados)
			if proximo == "" {
				break
			}

			// CONDICIONADO AO STATUS QUE ACABAMOS DE LER
			//   Se alguém mudou o card manualmente no meio desta rodada, o
			//   filtro não bate — devolve zero linhas, e a rodada segue sem
			//   erro (a próxima passada alcança, ou não precisa mais).
			var devolvido []struct {
				ID string `json:"id"`
			}
			filtro := "id=eq." + banco.Escapar(it.ID) + "&status=eq." + atual
			if err := s.bd.AtualizarDevolvendo(ctx, "servicos_orcamentos", filtro,
				map[string]any{"status": proximo}, &devolvido); err != nil {
				log.Printf("servicos: avançando o ticket %d de %q pra %q: %v", it.Ticket, atual, proximo, err)
				break
			}
			if len(devolvido) == 0 {
				break
			}
			avancados++
			atual = proximo
		}
	}
	return avancados, nil
}
