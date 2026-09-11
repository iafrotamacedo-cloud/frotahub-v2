package servicos

import "testing"

func chamado(codigo int, executado, vistoriado bool) struct {
	StatusCodigo *int    `json:"status_codigo"`
	ExecutadoEm  *string `json:"executado_em"`
	VistoriadoEm *string `json:"vistoriado_em"`
} {
	marca := "2026-09-08T10:00:00Z"
	c := struct {
		StatusCodigo *int    `json:"status_codigo"`
		ExecutadoEm  *string `json:"executado_em"`
		VistoriadoEm *string `json:"vistoriado_em"`
	}{StatusCodigo: &codigo}
	if executado {
		c.ExecutadoEm = &marca
	}
	if vistoriado {
		c.VistoriadoEm = &marca
	}
	return c
}

// ANDAMENTO DA EXECUÇÃO — O JOB SÓ ANDA PRA FRENTE
//
//	Aberto → Em execução → Executado → Aguardando faturamento. Sem marco,
//	não inventa avanço. Com os dois marcos de uma vez, a rodada encadeia
//	as colunas (ver RodarDeteccaoDeAvanco), cada uma no seu passo.
func TestProximoPorMarcoAndaSoComSinal(t *testing.T) {
	casos := []struct {
		status string
		c      struct {
			StatusCodigo *int    `json:"status_codigo"`
			ExecutadoEm  *string `json:"executado_em"`
			VistoriadoEm *string `json:"vistoriado_em"`
		}
		quer string
	}{
		{StatusAprovadoExecucao, chamado(1, false, false), ""},
		{StatusAprovadoExecucao, chamado(7, false, false), StatusEmExecucao},
		{StatusAprovadoExecucao, chamado(1, true, false), StatusEmExecucao},
		{StatusEmExecucao, chamado(7, false, false), ""},
		{StatusEmExecucao, chamado(5, false, false), StatusFinalizado},
		{StatusEmExecucao, chamado(7, true, false), StatusFinalizado},
		{StatusFinalizado, chamado(5, true, false), ""},
		{StatusFinalizado, chamado(6, true, false), StatusAguardandoFaturamento},
		{StatusFinalizado, chamado(5, true, true), StatusAguardandoFaturamento},
		{StatusOrcamentoLancado, chamado(7, true, true), ""},
	}
	for _, c := range casos {
		got := proximoPorMarco(c.status, c.c)
		if got != c.quer {
			t.Errorf("de %s com codigo=%v exec=%v vist=%v: got %q, quer %q",
				c.status, *c.c.StatusCodigo, c.c.ExecutadoEm != nil, c.c.VistoriadoEm != nil, got, c.quer)
		}
	}
}

func TestProximoPorMarcoNaoRegrediu(t *testing.T) {
	// Quem já está em Executado não volta para Aberto só porque o
	// status_codigo do Trílogo oscilou. O mapa é só pra frente.
	got := proximoPorMarco(StatusFinalizado, chamado(7, false, false))
	if got != "" {
		t.Errorf("Executado não deveria andar (nem voltar) só com codigo 7, andou para %q", got)
	}
}
