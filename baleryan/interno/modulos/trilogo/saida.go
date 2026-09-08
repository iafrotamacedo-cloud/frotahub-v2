// rev 1 — o chamado que SAIU da lista do Trílogo
//
// O DEFEITO
//
//	O robô só sabia acrescentar. `ler()` pega a lista das duas contas, faz
//	upsert do que achou e pronto — ninguém nunca fez a pergunta inversa:
//	"o que estava aqui e não está mais lá?". Quando os Mercadinhos tiram um
//	chamado da nossa prestadora, ele some da lista das duas contas e continua
//	na nossa tela para sempre. A lista só crescia.
//
// O INDÍCIO É A LISTA; A PROVA É O TRÍLOGO
//
//	Sumir da lista já bastaria: ela vem ordenada pelo número (ver api.go), é
//	pedida com TODOS os status, e a paginação só para quando a página inteira
//	ficou antes da data de corte — então tudo que é nosso e nasceu depois do
//	corte TEM que estar nela.
//
//	Mesmo assim, não é assim que se decide aqui. Sumiço da lista é o INDÍCIO;
//	o que confirma é perguntar pelo chamado, um por um, em cada conta. O
//	motivo é o tamanho do estrago: uma lista que um dia volte truncada — um
//	`Offset` que a API passe a ignorar, um status que saia do filtro deles —
//	esconderia centenas de chamados de uma vez, calada. Com a pergunta direta,
//	esse caminho não existe: cada carimbo é uma resposta do Trílogo sobre
//	AQUELE chamado, não uma dedução sobre uma lista inteira.
//
// O CONTRATO DE `GetTicketDetail` NÃO FOI ADIVINHADO
//
//	Medido contra o Trílogo de verdade em 08/09/2026, nas duas contas, com o
//	ticket 132242 (que os Mercadinhos tinham acabado de tirar da nossa
//	prestadora), o 132243 (que continua nosso, na Instalações) e um número
//	inventado:
//
//	  200 + JSON do chamado ....... é da conta
//	  400 {"message":"Você não possui permissão para acessar este ticket"}
//	  400 {"message":"Ticket não encontrado"}
//
//	A PRIMEIRA VERSÃO DESTE ARQUIVO ERRAVA AQUI, e errava do jeito silencioso:
//	acreditou no comentário do `colher` — "chamado de outra conta responde
//	VAZIO, com 200" — e tratou todo erro como "não sei". Como a resposta real
//	é 400, todo chamado que saiu de verdade caía em "não sei", e o carimbo
//	nunca vinha. O robô rodaria bonito, sem erro nenhum, sem marcar UM
//	chamado. Foi pego rodando a pergunta na mão antes da primeira rodada.
//
//	(O corpo vazio com 200 continua tratado, logo abaixo, como "não alcança".
//	Ele acontece em outros endereços do Trílogo e não custa nada cobrir.)
//
// TRÊS RESPOSTAS, NÃO DUAS
//
//	é nossa · esta conta não alcança · não sei. E o "não sei" NUNCA carimba:
//	fica para a próxima rodada. Os dois 400 acima são "não alcança" — tanto
//	faz se perdemos a permissão ou se o chamado sumiu do Trílogo inteiro: nos
//	dois casos ele não é mais nosso, que é a pergunta que este arquivo faz.
//	Já 401, 403, 5xx e erro de rede não dizem NADA sobre o chamado, e viram
//	"não sei".
//
// A DECISÃO É PELO CÓDIGO HTTP, NUNCA PELA FRASE
//
//	`api.go` avisa, com razão, que a frase do Trílogo mente por omissão e não
//	serve para o programa ramificar. Aqui não se lê frase nenhuma: 400 é 400,
//	com qualquer texto dentro. As duas mensagens acima estão escritas só para
//	quem for ler este arquivo depois entender o que se mediu.
package trilogo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
)

// TetoDaConfirmacao — quantos sumiços se confirmam por rodada.
//
// Em regime, os suspeitos de uma rodada são um punhado: o que saiu do Trílogo
// nas últimas duas horas. O teto existe para a PRIMEIRA rodada depois desta
// mudança, que encontra de uma vez todo o acúmulo de meses — e para o dia em
// que o cliente devolver um lote inteiro de chamados a outra prestadora. Sem
// ele, uma rodada de dois minutos viraria uma de quarenta, e o corte de tempo
// do Actions mataria a leitura no meio.
//
// O que passar do teto não se perde: continua sem carimbo e é o primeiro da
// fila na rodada seguinte (a ordem é pelo número, do mais antigo para o mais
// novo, então a fronteira anda sempre para a frente).
const TetoDaConfirmacao = 200

// quantosPorFiltro — números por PATCH. O filtro do PostgREST vai na URL, e
// URL tem limite: mil e quatrocentos números de seis dígitos numa linha só é
// pedido que o servidor recusa antes de ler.
const quantosPorFiltro = 100

// espelho é o que a nossa base sabe de um chamado, para efeito desta conta.
type espelho struct {
	Numero int     `json:"numero"`
	Conta  string  `json:"conta"`
	SaiuEm *string `json:"saiu_em"`
}

// resposta é o que o Trílogo diz sobre um chamado.
type resposta int

const (
	aindaENossa    resposta = iota // 200 com o chamado: continua na conta
	naoEDessaConta                 // 400 (sem permissão / não encontrado), ou corpo vazio
	naoSei                         // rede, 401, 5xx — não diz nada sobre o chamado
)

// marcarQuemSaiu carimba `saiu_em` em quem sumiu da lista e o limpa em quem
// voltou.
//
// `naLista` é a UNIÃO dos números que as contas devolveram nesta rodada — a
// união, e não a lista de uma conta, porque um chamado que troca de prestadora
// entre Instalações e Civil continua sendo nosso (é a decisão 1 da migração
// 007: a conta é atributo, não identidade).
//
// POR QUE SÓ OS NASCIDOS DEPOIS DA DATA DE CORTE
//
//	Chamado anterior a 01/07/2026 chegou aqui por outro caminho — o modo
//	`alvos` ou a carga do legado (migração 013) — e a lista rotineira nem
//	tenta alcançá-lo: ela para de paginar ao passar do corte. Ausência dele na
//	lista não é sumiço, é o combinado. Carimbá-lo apagaria da tela justamente
//	o histórico que alguém pediu para trazer.
func (s *Servico) marcarQuemSaiu(ctx context.Context, clienteID string, naLista map[int]bool, sessoes []*Sessao, r *Resultado) error {
	if len(naLista) == 0 || len(sessoes) == 0 {
		// Lista vazia é o caso em que MAIS se erra: seria o Trílogo inteiro
		// sumindo de uma vez. Não se carimba nada com base em nada.
		return nil
	}

	nossos, err := s.espelhoDoCliente(ctx, clienteID)
	if err != nil {
		return err
	}

	var suspeitos []espelho
	var voltaram []int
	for _, c := range nossos {
		na := naLista[c.Numero]
		switch {
		case c.SaiuEm == nil && !na:
			suspeitos = append(suspeitos, c)
		case c.SaiuEm != nil && na:
			voltaram = append(voltaram, c.Numero)
		}
		// Carimbado e ainda fora da lista: não se pergunta de novo. Sem isto,
		// toda rodada gastaria uma consulta ao Trílogo por chamado que já saiu,
		// para sempre, e a conta só cresceria.
	}

	if len(voltaram) > 0 {
		if err := s.carimbar(ctx, clienteID, voltaram, nil); err != nil {
			return fmt.Errorf("limpando o carimbo de quem voltou: %w", err)
		}
		r.ChamadosQueVoltaram = len(voltaram)
		log.Printf("[trilogo] %d chamado(s) voltaram para a lista: %s",
			len(voltaram), listaDeNumeros(voltaram))
	}

	if len(suspeitos) == 0 {
		return nil
	}
	// Do mais antigo para o mais novo, e com teto: a fronteira anda sempre para
	// a frente, rodada após rodada.
	sort.Slice(suspeitos, func(i, j int) bool { return suspeitos[i].Numero < suspeitos[j].Numero })
	sobraram := 0
	if len(suspeitos) > TetoDaConfirmacao {
		sobraram = len(suspeitos) - TetoDaConfirmacao
		suspeitos = suspeitos[:TetoDaConfirmacao]
	}

	sairam, inconclusivos := s.confirmarSumico(ctx, sessoes, suspeitos)
	if len(sairam) > 0 {
		agora := time.Now().UTC().Format(time.RFC3339)
		if err := s.carimbar(ctx, clienteID, sairam, agora); err != nil {
			return fmt.Errorf("carimbando quem saiu: %w", err)
		}
		r.ChamadosQueSairam = len(sairam)
	}

	// O log diz as quatro coisas, porque as quatro são diferentes: quem era
	// suspeito, quem saiu de verdade, quem o Trílogo não soube responder agora
	// (volta na próxima) e quem ficou de fora pelo teto.
	log.Printf("[trilogo] saída · %d suspeito(s) · %d confirmado(s) · %d sem resposta · %d além do teto",
		len(suspeitos), len(sairam), inconclusivos, sobraram)
	if len(sairam) > 0 {
		log.Printf("[trilogo] saíram da lista: %s", listaDeNumeros(sairam))
	}
	return nil
}

// espelhoDoCliente lê o que a nossa base tem na janela da lista.
func (s *Servico) espelhoDoCliente(ctx context.Context, clienteID string) ([]espelho, error) {
	var nossos []espelho
	caminho := "chamados?cliente_id=eq." + banco.Escapar(clienteID) +
		"&criado_em=gte." + banco.Escapar(DataDeCorte.Format(time.RFC3339)) +
		"&select=numero,conta,saiu_em&order=numero"
	if err := s.bd.Buscar(ctx, caminho, &nossos); err != nil {
		return nil, fmt.Errorf("lendo os chamados que já temos: %w", err)
	}
	return nossos, nil
}

// confirmarSumico pergunta chamado por chamado, em cada conta, e devolve os que
// NENHUMA conta reconheceu — mais quantos ficaram sem resposta.
//
// A conta que a nossa base já atribui ao chamado é perguntada primeiro: no caso
// comum (o chamado continua onde estava) isso resolve na primeira pergunta, e a
// segunda conta nem é incomodada.
func (s *Servico) confirmarSumico(ctx context.Context, sessoes []*Sessao, suspeitos []espelho) (sairam []int, inconclusivos int) {
	veredicto := make([]resposta, len(suspeitos))
	vagas := make(chan struct{}, s.cfg.Trilogo.Paralelo)
	var espera sync.WaitGroup

	for i, c := range suspeitos {
		espera.Add(1)
		go func(i int, c espelho) {
			defer espera.Done()
			vagas <- struct{}{}
			defer func() { <-vagas }()
			veredicto[i] = aindaDeAlguem(ctx, ordemDeConsulta(sessoes, c.Conta), c.Numero)
		}(i, c)
	}
	espera.Wait()

	for i, c := range suspeitos {
		switch veredicto[i] {
		case naoEDessaConta:
			sairam = append(sairam, c.Numero)
		case naoSei:
			inconclusivos++
		}
	}
	return sairam, inconclusivos
}

// aindaDeAlguem pergunta a cada conta, na ordem dada, e para na primeira que
// reconhecer o chamado.
//
// Basta UMA conta reconhecê-lo para ele continuar sendo nosso. E basta UMA não
// responder direito para o veredicto virar "não sei": dizer que saiu sem ter
// perguntado a todas seria carimbar no escuro.
func aindaDeAlguem(ctx context.Context, sessoes []*Sessao, numero int) resposta {
	duvida := false
	for _, sessao := range sessoes {
		switch respostaDaConta(ctx, sessao, numero) {
		case aindaENossa:
			return aindaENossa
		case naoSei:
			duvida = true
		}
	}
	if duvida {
		return naoSei
	}
	return naoEDessaConta
}

// respostaDaConta traduz o que UMA conta respondeu sobre UM chamado.
func respostaDaConta(ctx context.Context, sessao *Sessao, numero int) resposta {
	d, err := sessao.Detalhe(ctx, numero)
	if err == nil {
		if d != nil && d.ID == numero {
			return aindaENossa
		}
		// Erro nulo sem o chamado dentro: corpo vazio. Esta conta não alcança.
		return naoEDessaConta
	}

	// 400 é a recusa do Trílogo a entregar o chamado para ESTA conta — sem
	// permissão, ou não existe mais. Nos dois casos ele não é mais nosso por
	// aqui. Qualquer outro erro (401 com o token vencido, 5xx deles, rede que
	// caiu) não fala do chamado, fala de nós: não decide nada.
	var doTrilogo *ErroDoTrilogo
	if errors.As(err, &doTrilogo) && doTrilogo.Codigo == http.StatusBadRequest {
		return naoEDessaConta
	}
	return naoSei
}

// ordemDeConsulta põe a conta do chamado na frente, mantendo as outras atrás.
func ordemDeConsulta(sessoes []*Sessao, conta string) []*Sessao {
	if conta == "" || len(sessoes) < 2 {
		return sessoes
	}
	ordem := make([]*Sessao, 0, len(sessoes))
	for _, s := range sessoes {
		if s.Conta == conta {
			ordem = append(ordem, s)
		}
	}
	if len(ordem) == 0 {
		return sessoes
	}
	for _, s := range sessoes {
		if s.Conta != conta {
			ordem = append(ordem, s)
		}
	}
	return ordem
}

// carimbar grava `saiu_em` (ou o limpa, com `quando` nulo) nos números dados.
//
// O cliente entra no filtro SEMPRE. Um PATCH em `chamados` filtrado só pelo
// número mexeria no chamado de outro cliente que tivesse o mesmo número — e
// número de ticket do Trílogo não é único entre clientes (CORE-11).
func (s *Servico) carimbar(ctx context.Context, clienteID string, numeros []int, quando any) error {
	for i := 0; i < len(numeros); i += quantosPorFiltro {
		fim := min(i+quantosPorFiltro, len(numeros))
		lote := numeros[i:fim]

		emTexto := make([]string, len(lote))
		for j, n := range lote {
			emTexto[j] = strconv.Itoa(n)
		}
		filtro := "cliente_id=eq." + banco.Escapar(clienteID) +
			"&numero=in.(" + strings.Join(emTexto, ",") + ")"
		if err := s.bd.Atualizar(ctx, "chamados", filtro, map[string]any{"saiu_em": quando}); err != nil {
			return err
		}
	}
	return nil
}
