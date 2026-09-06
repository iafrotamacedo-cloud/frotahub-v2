package rogueworker

import (
	"net/http"
	"testing"
)

func TestReconhecerPendencias(t *testing.T) {
	if reconhecer("quantas notas estão pendentes agora?").comando != cmdPendencias {
		t.Fatal("não reconheceu pendências")
	}
}

func TestReconhecerLoteAntesDoItem(t *testing.T) {
	if reconhecer("gere orçamento de todas as notas na fila").comando != cmdGerarLote {
		t.Fatal("lote de gerar tinha que ganhar do item")
	}
	if reconhecer("lance todos os orçamentos da fila de lançamento").comando != cmdLancarLote {
		t.Fatal("lote de lançar tinha que ganhar do item")
	}
}

func TestReconhecerGerarComTicket(t *testing.T) {
	r := reconhecer("gere orçamento da nota do ticket 131768")
	if r.comando != cmdGerar || r.ticket != 131768 {
		t.Fatalf("gerar item: %+v", r)
	}
}

func TestReconhecerExtrapoladas(t *testing.T) {
	if reconhecer("quais orçamentos passaram do teto?").comando != cmdExtrapoladas {
		t.Fatal("não reconheceu extrapoladas")
	}
}

func TestReconhecerBloqueadas(t *testing.T) {
	if reconhecer("quais notas estão bloqueadas ou repetidas?").comando != cmdBloqueadas {
		t.Fatal("não reconheceu bloqueadas")
	}
}

func TestReconhecerRelatorios(t *testing.T) {
	casos := map[string]string{
		"quanto pagamos a fornecedores este mês?":                 cmdPagamos,
		"quanto de material já foi lançado no contrato este mês?": cmdMaterial,
		"quanto já foi faturado ao cliente?":                      cmdFaturado,
		"quanto falta faturar?":                                   cmdFaturado,
		"fechamento do mês":                                       cmdFechamento,
	}
	for frase, cmd := range casos {
		if reconhecer(frase).comando != cmd {
			t.Errorf("%q → %s, queria %s", frase, reconhecer(frase).comando, cmd)
		}
	}
}

func TestReconhecerNavegar(t *testing.T) {
	r := reconhecer("me mostra a tela de consolidação financeira")
	if r.comando != cmdNavegar || r.tela != "consolidacao" {
		t.Fatalf("navegar consolidação: %+v", r)
	}
	r = reconhecer("me mostra a estatística de chamados de todas as lojas")
	if r.comando != cmdNavegar || r.tela != "est-chamados" {
		t.Fatalf("navegar chamados: %+v", r)
	}
}

func TestEhSimNao(t *testing.T) {
	if !ehSim("Sim") || !ehSim("pode") || ehSim("talvez") {
		t.Fatal("sim")
	}
	if !ehNao("não") || !ehNao("cancela") || ehNao("sim") {
		t.Fatal("não")
	}
}

func TestPareceCom(t *testing.T) {
	if !pareceCom("qtas notas estão pendentes agora", "quantas notas pendentes") {
		t.Fatal("tinha que casar o formato canônico")
	}
	if pareceCom("lance o orçamento 12", "quantas notas pendentes") {
		t.Fatal("não podia casar ação com pergunta de leitura")
	}
}

func TestMontarNaoPânico(t *testing.T) {
	(&Modulo{}).Montar(http.NewServeMux())
}

func TestFraseRecusaEFixa(t *testing.T) {
	if fraseRecusa != "infelizmente você não tem permissão para esta tarefa" {
		t.Fatal("a frase de recusa não pode variar")
	}
}

func TestUmUUID(t *testing.T) {
	if _, ok := umUUID("3d8f1c2a-4b5e-6789-abcd-ef0123456789"); !ok {
		t.Fatal("uuid válido recusado")
	}
	if _, ok := umUUID("nao-e-uuid"); ok {
		t.Fatal("lixo passou por uuid")
	}
}
