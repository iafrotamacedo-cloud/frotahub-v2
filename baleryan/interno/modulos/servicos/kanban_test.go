package servicos

import (
	"strings"
	"testing"
)

// O FLUXO TEM QUE FECHAR SEMPRE
//
//	Decisão do dono: todo status não-final tem que ter PELO MENOS um caminho
//	pra frente. Um status esquecido no mapa (sem entrada nenhuma) travaria
//	um cartão pra sempre, sem ninguém notar até um ticket real cair nele.
func TestTodoStatusNaoFinalTemCaminhoParaFrente(t *testing.T) {
	todos := []string{
		StatusAguardandoOrcamento, StatusOrcamentoFeito, StatusOrcamentoLancado,
		StatusOrcamentoAprovado, StatusOrcamentoRejeitado,
		StatusAprovadoExecucao, StatusEmExecucao, StatusFinalizado,
		StatusAguardandoFaturamento, StatusFaturado,
	}
	for _, status := range todos {
		_, existe := proximosStatus[status]
		if !existe {
			t.Errorf("status %q não está no mapa de transições — fica travado pra sempre", status)
		}
	}
	// Só o último da fila (Faturado) é becos sem saída — é o fim de linha de
	// verdade, não esquecimento.
	if len(proximosStatus[StatusFaturado]) != 0 {
		t.Errorf("Faturado deveria ser terminal, tem saída para %v", proximosStatus[StatusFaturado])
	}
	for status, saidas := range proximosStatus {
		if status == StatusFaturado {
			continue
		}
		if len(saidas) == 0 {
			t.Errorf("status %q não é o Faturado e não tem saída nenhuma — fluxo não fecha", status)
		}
	}
}

// REJEITADO NÃO ANDA SOZINHO, MAS TAMBÉM NÃO É BECO SEM SAÍDA
func TestRejeitadoVoltaParaOrcamentoFeito(t *testing.T) {
	saidas := proximosStatus[StatusOrcamentoRejeitado]
	if len(saidas) != 1 || saidas[0] != StatusOrcamentoFeito {
		t.Errorf("Rejeitado deveria voltar só para OrcamentoFeito, voltou para %v", saidas)
	}
}

// LANÇADO SÓ ANDA PRA EXECUÇÃO. Rejeitar não é status — sai do Kanban.
func TestLancadoVaiParaExecucao(t *testing.T) {
	saidas := proximosStatus[StatusOrcamentoLancado]
	if len(saidas) != 1 || saidas[0] != StatusAprovadoExecucao {
		t.Errorf("Lançado deveria ir só para Execução Aberto, foi para %v", saidas)
	}
}

func TestErrTransicaoInvalidaMensagem(t *testing.T) {
	err := &ErrTransicaoInvalida{De: StatusAguardandoOrcamento, Para: StatusFaturado}
	msg := err.Error()
	if !strings.Contains(msg, StatusOrcamentoFeito) {
		t.Errorf("a mensagem de erro não diz pra onde dava pra ir: %q", msg)
	}

	terminal := &ErrTransicaoInvalida{De: StatusFaturado, Para: StatusAguardandoOrcamento}
	if !strings.Contains(terminal.Error(), "final") {
		t.Errorf("a mensagem do status final deveria dizer isso: %q", terminal.Error())
	}
}

func TestListaEntreParenteses(t *testing.T) {
	got := listaEntreParenteses([]string{"Serviço Instalações", "Romario Santos"})
	// Cada valor entra entre aspas ESCAPADAS (banco.Escapar já cuida do
	// espaço e da cedilha) — só confere que as DUAS partes estão presentes
	// e separadas por vírgula, sem se prender ao detalhe do %-encoding.
	partes := strings.Split(got, ",")
	if len(partes) != 2 {
		t.Fatalf("esperava 2 partes separadas por vírgula, veio %d: %q", len(partes), got)
	}
}
