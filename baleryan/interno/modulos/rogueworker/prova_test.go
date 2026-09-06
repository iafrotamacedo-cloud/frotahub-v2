package rogueworker

import (
	"net/http"
	"testing"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
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

func TestReconhecerChamadosAtendidos(t *testing.T) {
	for _, frase := range []string{
		"quantos chamados atendemos mes passado?",
		"quantos chamados atendemos no mes de agosto",
		"quantos tickets executamos este mês",
	} {
		if reconhecer(frase).comando != cmdChamadosAtendidos {
			t.Errorf("%q não reconheceu contagem de chamados", frase)
		}
	}
	if reconhecer("me mostra a estatística de chamados de todas as lojas").comando != cmdNavegar {
		t.Fatal("navegar para a tela de chamados tinha que continuar ganhando da contagem")
	}
}

func TestPeriodoDoPedido(t *testing.T) {
	inicio, fim, _ := periodoDoPedido("quantos chamados atendemos mes passado")
	agora := time.Now().In(relatorio.FusoDaCasa())
	quer := time.Date(agora.Year(), agora.Month(), 1, 0, 0, 0, 0, agora.Location()).AddDate(0, -1, 0)
	if inicio.Year() != quer.Year() || inicio.Month() != quer.Month() || inicio.Day() != 1 {
		t.Fatalf("mês passado virou %v, queria %v", inicio, quer)
	}
	if !fim.Equal(inicio.AddDate(0, 1, 0)) {
		t.Fatalf("fim %v", fim)
	}
	inicio, _, _ = periodoDoPedido("quantos chamados no mes de agosto de 2026")
	if inicio.Year() != 2026 || inicio.Month() != time.August {
		t.Fatalf("agosto/2026 virou %v", inicio)
	}
}

func TestPareceQueNaoEntendeu(t *testing.T) {
	if !pareceQueNaoEntendeu("Desculpe, não entendi.") {
		t.Fatal("tinha que reconhecer a recusa vazia do modelo")
	}
	if pareceQueNaoEntendeu("Em agosto atendemos 412 chamados.") {
		t.Fatal("resposta útil não é recusa")
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

func TestInferirFontes(t *testing.T) {
	casos := []struct{ frase, quer string }{
		{"documentos de funcionário vencendo", "funcionarios_vencendo"},
		{"quantos na fila de serviço", "servicos_painel"},
		{"notas da consolidação", "consolidacao"},
		{"quando rodou o robô do trilogo", "robos_trilogo"},
		{"status do kanban", "servicos_kanban"},
	}
	for _, c := range casos {
		ids := inferirFontes(c.frase)
		achou := false
		for _, id := range ids {
			if id == c.quer {
				achou = true
			}
		}
		if !achou {
			t.Errorf("%q → %v, faltou %s", c.frase, ids, c.quer)
		}
	}
}

func TestJuntarFontesCortaEIgnoraLixo(t *testing.T) {
	ids := juntarFontes([]string{"nao_existe", "servicos_painel", "servicos_painel", "orcamentos_painel"})
	if len(ids) != 2 || ids[0] != "servicos_painel" || ids[1] != "orcamentos_painel" {
		t.Fatalf("%v", ids)
	}
}

func TestCompactarListaGrande(t *testing.T) {
	xs := make([]any, 40)
	for i := range xs {
		xs[i] = map[string]any{"n": i}
	}
	v, ok := compactarValor(xs, 0).(map[string]any)
	if !ok {
		t.Fatalf("queria mapa com total, veio %T", compactarValor(xs, 0))
	}
	if v["total"] != 40 {
		t.Fatalf("total %v", v["total"])
	}
}
