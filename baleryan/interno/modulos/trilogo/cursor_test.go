package trilogo

import (
	"context"
	"testing"
	"time"
)

// muitosChamados enche o Trílogo de mentira com chamados de horários de alteração
// DIFERENTES, do mais antigo para o mais novo. É o que permite ver a fronteira
// andar — com todos no mesmo horário, não há como distinguir progresso de
// repetição.
func muitosChamados(m *mundo, quantos int) {
	inicio := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	for i := 0; i < quantos; i++ {
		numero := 200000 + i
		m.chamado(numero, 13, "", nil, nil)
		quando := inicio.Add(time.Duration(i) * time.Minute).Format("2006-01-02T15:04:05.000")
		m.lista[len(m.lista)-1]["dateOfLastChange"] = quando
		m.detalhes[numero]["dateOfLastChange"] = quando
	}
}

// ---------------------------------------------------------------------------
// O defeito que este arquivo existe para impedir
// ---------------------------------------------------------------------------
//
// Em 24/08/2026 o robô agendado girou NOVE vezes seguidas lendo 1.600 chamados e
// gravando os mesmos 150, até o corte de tempo do Actions matar o processo. A
// marca d'água só era gravada quando a rodada varria tudo — e, como cada rodada
// processa um lote, ela nunca varria tudo. O cursor nunca saía do lugar.

func TestAMarcaAvancaMesmoQuandoFaltaTrabalho(t *testing.T) {
	m := novoMundo(t)
	muitosChamados(m, 300)
	svc := m.servico()

	r, err := svc.Rodar(context.Background(), ModoAtualizacao, cliente, "teste")
	if err != nil {
		t.Fatal(err)
	}
	if r.Completo {
		t.Fatal("300 chamados couberam num lote só? o teste perdeu o sentido")
	}
	if m.marcaDagua == "" {
		t.Fatal("a rodada não foi completa e a marca d'água NÃO foi gravada — " +
			"é exatamente o laço infinito de 24/08: a próxima volta recomeça do zero")
	}
}

func TestAAtualizacaoTerminaEmVezDeGirar(t *testing.T) {
	m := novoMundo(t)
	muitosChamados(m, 300)
	svc := m.servico()
	ctx := context.Background()

	var voltas int
	var marcas []string
	for volta := 1; volta <= 30; volta++ {
		voltas = volta
		r, err := svc.Rodar(ctx, ModoAtualizacao, cliente, "teste")
		if err != nil {
			t.Fatalf("volta %d: %v", volta, err)
		}
		marcas = append(marcas, m.marcaDagua)
		if volta > 1 && marcas[volta-1] == marcas[volta-2] && !r.Completo {
			t.Fatalf("volta %d: a marca não andou (%s) e a rodada diz que falta trabalho — está girando",
				volta, m.marcaDagua)
		}
		if r.Completo {
			break
		}
	}
	if voltas >= 30 {
		t.Fatalf("não terminou em 30 voltas; marcas: %v", marcas)
	}
	// 300 chamados, 75 por conta: o mínimo teórico é 4 voltas. O teto de 8 dá
	// espaço para a folga do relógio e denuncia se ela voltar a comer o lote —
	// com uma hora de folga, isto levava 16.
	if voltas > 8 {
		t.Errorf("terminou em %d voltas; devia levar perto de 4. A folga do relógio "+
			"está refazendo trabalho demais a cada lote.", voltas)
	}
	t.Logf("terminou em %d voltas", voltas)

	// E terminou tendo gravado TODOS os chamados, não só os do primeiro lote.
	vistos := map[any]bool{}
	for _, l := range m.linhas("chamados") {
		vistos[l["numero"]] = true
	}
	if len(vistos) != 300 {
		t.Errorf("gravou %d chamados distintos, esperava 300", len(vistos))
	}
}

// O limite era um só, descontado na primeira conta e chegando ZERADO na segunda.
// Efeito: a conta Civil rodava sem limite nenhum e, quando a Instalações consumia
// tudo, a Civil não andava.
func TestCadaContaTemOSeuOrcamento(t *testing.T) {
	m := novoMundo(t)
	muitosChamados(m, 300)
	svc := m.servico()

	r, err := svc.Rodar(context.Background(), ModoAtualizacao, cliente, "teste")
	if err != nil {
		t.Fatal(err)
	}
	// Duas contas, lote de 150 → 75 para cada. As duas contas do mundo de mentira
	// enxergam a mesma lista, então a primeira volta processa 75 de cada uma.
	if r.ChamadosGravados > 160 {
		t.Errorf("a primeira volta gravou %d: uma conta passou do seu orçamento", r.ChamadosGravados)
	}
	if r.ChamadosGravados < 100 {
		t.Errorf("a primeira volta gravou só %d: alguma conta não andou", r.ChamadosGravados)
	}
}

// A marca é a MENOR fronteira entre as contas. Guardar a maior faria a conta
// atrasada pular para sempre o que ela ainda não leu.
func TestAMarcaEODaContaQueMenosAndou(t *testing.T) {
	agora := time.Now()
	casos := []struct {
		nome  string
		dados []time.Time
		quer  time.Time
	}{
		{"duas contas", []time.Time{agora.Add(-2 * time.Hour), agora}, agora.Add(-2 * time.Hour)},
		{"conta vazia não segura", []time.Time{{}, agora}, agora},
		{"tudo vazio", []time.Time{{}, {}}, time.Time{}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if veio := menorInstante(c.dados); !veio.Equal(c.quer) {
				t.Errorf("veio %v, esperava %v", veio, c.quer)
			}
		})
	}
}

// Prova de que o mundo de mentira guarda a marca de verdade — sem isto, os testes
// acima passariam por acidente.
func TestODubleGuardaAMarca(t *testing.T) {
	m := novoMundo(t)
	muitosChamados(m, 300)
	svc := m.servico()

	_, _ = svc.Rodar(context.Background(), ModoAtualizacao, cliente, "teste")
	primeira := m.marcaDagua
	if primeira == "" {
		t.Fatal("nada foi gravado")
	}
	marca, err := svc.ultimaMarca(context.Background(), cliente)
	if err != nil {
		t.Fatal(err)
	}
	if marca.IsZero() {
		t.Fatal("o dublê guardou a marca mas não a devolve na leitura")
	}
}
