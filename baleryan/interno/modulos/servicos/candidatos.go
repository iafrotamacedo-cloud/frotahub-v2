// rev 2 — a pré-triagem de Serviço
//
// "SERÁ QUE ISTO É SERVIÇO?" — RESPOSTA DE MÁQUINA, PROVISÓRIA
//
//	RodarClassificacao é o coração do job agendado (cmd/servicos, modo
//	"candidatos"): lê `servicos_a_classificar` (a view da migração 050 — quem
//	ainda não passou por nenhuma decisão), pergunta pro Groq um por um, e
//	grava o palpite em servicos_candidatos. NADA aqui fala com o Trílogo —
//	é só sugestão, para um humano decidir na tela.
package servicos

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
)

// aClassificar é uma linha de `servicos_a_classificar`.
type aClassificar struct {
	ChamadoID string `json:"chamado_id"`
	ClienteID string `json:"cliente_id"`
	Ticket    int    `json:"ticket"`
	Descricao string `json:"descricao"`
	Conta     string `json:"conta"`
}

// Candidato é uma linha de servicos_candidatos, como a tela lê — já com
// loja/conta/descrição do chamado, pro mesmo formato de linha de
// trilogo/DadosTrilogo.tsx (a tela pede isso: "mesma formatação da lista do
// Trílogo", só sem status/prioridade/responsável, que aqui não fazem
// sentido — o candidato ainda nem entrou na fila de verdade).
type Candidato struct {
	ID        string `json:"id"`
	ChamadoID string `json:"chamado_id"`
	Ticket    int    `json:"ticket"`
	Loja      string `json:"loja"`
	Conta     string `json:"conta"`
	Descricao string `json:"descricao"`
	Motivo    string `json:"motivo"`
	Status    string `json:"status"`
	CriadoEm  string `json:"criado_em"`
}

// candidatoLinha é a forma bruta que o PostgREST devolve com o embed de
// chamados/unidades — a API pública (Candidato) fica achatada, sem o
// aninhamento, porque é isso que a tela espera.
type candidatoLinha struct {
	ID        string `json:"id"`
	ChamadoID string `json:"chamado_id"`
	Ticket    int    `json:"ticket"`
	Motivo    string `json:"motivo"`
	Status    string `json:"status"`
	CriadoEm  string `json:"criado_em"`
	Chamados  *struct {
		Conta     string `json:"conta"`
		Descricao string `json:"descricao"`
		Unidades  *struct {
			Nome string `json:"nome"`
		} `json:"unidades"`
	} `json:"chamados"`
}

func (l candidatoLinha) achatar() Candidato {
	c := Candidato{
		ID: l.ID, ChamadoID: l.ChamadoID, Ticket: l.Ticket,
		Motivo: l.Motivo, Status: l.Status, CriadoEm: l.CriadoEm,
	}
	if l.Chamados != nil {
		c.Conta = l.Chamados.Conta
		c.Descricao = l.Chamados.Descricao
		if l.Chamados.Unidades != nil {
			c.Loja = l.Chamados.Unidades.Nome
		}
	}
	return c
}

// RodarClassificacao avalia até LimitePorRodada chamados novos. Devolve
// quantos foram AVALIADOS (chamada ao Groq deu certo) e quantos, entre eles,
// viraram candidato — a diferença entre os dois é "quantos falharam", sem
// precisar de um terceiro número.
//
// UM CHAMADO POR VEZ, SEM PARAR NO PRIMEIRO ERRO
//
//	Uma chamada ao Groq falhando (rede, cota, formato) não pode derrubar a
//	rodada inteira: os outros 199 chamados não têm culpa. O que falhou
//	continua em `servicos_a_classificar` e é tentado de novo na próxima
//	rodada, sozinho — não existe estado parcial para desfazer.
func (s *Servico) RodarClassificacao(ctx context.Context, clienteID string) (avaliados, marcados int, err error) {
	if !s.groq.ligado() {
		return 0, 0, fmt.Errorf("GROQ_API_KEY não está configurada — o job de Candidatos não roda sem ela")
	}

	var fila []aClassificar
	caminho := "servicos_a_classificar?cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=chamado_id,cliente_id,ticket,descricao,conta" +
		"&order=criado_em&limit=" + itoa(s.cfg.Groq.LimitePorRodada)
	if err := s.bd.Buscar(ctx, caminho, &fila); err != nil {
		return 0, 0, fmt.Errorf("lendo a fila de classificação: %w", err)
	}

	for _, c := range fila {
		if ctx.Err() != nil {
			break
		}
		descricao := c.Descricao
		if descricao == "" {
			// Chamado sem descrição não tem o que julgar — marca como
			// resolvido é errado (não virou serviço) e pendente é mentira
			// (nunca vai ter mais informação). "Descartado" documenta que
			// a máquina olhou e não tinha como decidir.
			s.gravarCandidato(ctx, c, false, "sem descrição para avaliar")
			avaliados++
			continue
		}

		resp, erroGroq := s.groq.Classificar(ctx, descricao)
		if erroGroq != nil {
			log.Printf("servicos: candidato %s (ticket %d): %v — tento de novo na próxima rodada",
				c.ChamadoID, c.Ticket, erroGroq)
			continue
		}
		avaliados++
		if !resp.Candidato {
			continue
		}
		if err := s.gravarCandidato(ctx, c, true, resp.Motivo); err != nil {
			log.Printf("servicos: não gravei o candidato do ticket %d: %v", c.Ticket, err)
			continue
		}
		marcados++
	}
	return avaliados, marcados, nil
}

// gravarCandidato grava a decisão — 'pendente' quando é candidato de
// verdade, 'descartado' quando a própria máquina já sabe que não é (hoje, só
// o caso de descrição vazia) ou quando `candidato=false` e mesmo assim
// vale registrar que já foi avaliado (evita reavaliar o mesmo chamado a
// cada rodada, mordendo a cota do Groq à toa).
func (s *Servico) gravarCandidato(ctx context.Context, c aClassificar, candidato bool, motivo string) error {
	status := "descartado"
	if candidato {
		status = "pendente"
	}
	linha := map[string]any{
		"cliente_id": c.ClienteID,
		"chamado_id": c.ChamadoID,
		"ticket":     c.Ticket,
		"motivo":     motivo,
		"status":     status,
	}
	if !candidato {
		linha["decidido_em"] = time.Now().UTC().Format(time.RFC3339)
	}
	return s.bd.Inserir(ctx, "servicos_candidatos", []map[string]any{linha}, nil)
}

// ListarCandidatos devolve os pendentes, do mais novo para o mais velho — é
// o card de Candidatos.
func (s *Servico) ListarCandidatos(ctx context.Context, clienteID string) ([]Candidato, error) {
	brutas := []candidatoLinha{}
	caminho := "servicos_candidatos?cliente_id=eq." + banco.Escapar(clienteID) +
		"&status=eq.pendente" +
		"&select=id,chamado_id,ticket,motivo,status,criado_em,chamados(conta,descricao,unidades(nome))" +
		"&order=criado_em.desc"
	if err := s.bd.Buscar(ctx, caminho, &brutas); err != nil {
		return nil, err
	}
	candidatos := make([]Candidato, len(brutas))
	for i, l := range brutas {
		candidatos[i] = l.achatar()
	}
	return candidatos, nil
}

// ErrCandidatoNaoEstaPendente — só dá para descartar ou promover um
// candidato que ainda está pendente. Um já descartado é definitivo (o dono
// foi claro: "não volta pra fila de candidatos"); um já resolvido virou
// serviço de verdade e mexer nele aqui seria a ação errada — a reclassificação
// dele mora no Kanban (kanban.go, Reclassificar), não em Candidatos.
var ErrCandidatoNaoEstaPendente = fmt.Errorf("este candidato já foi decidido")

// candidatoPendente busca um candidato garantindo que está pendente — as
// duas ações (Descartar, Promover) começam pela mesma checagem.
func (s *Servico) candidatoPendente(ctx context.Context, clienteID, candidatoID string) (Candidato, error) {
	var linhas []Candidato
	caminho := "servicos_candidatos?id=eq." + banco.Escapar(candidatoID) +
		"&cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=id,chamado_id,ticket,motivo,status,criado_em&limit=1"
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return Candidato{}, err
	}
	if len(linhas) == 0 {
		return Candidato{}, fmt.Errorf("candidato não encontrado")
	}
	if linhas[0].Status != "pendente" {
		return Candidato{}, ErrCandidatoNaoEstaPendente
	}
	return linhas[0], nil
}

// DescartarCandidato marca "não é Serviço" — definitivo. `autorID` fica
// registrado; quem grava o histórico é a rota (rotas.go), que tem o
// Principal inteiro.
func (s *Servico) DescartarCandidato(ctx context.Context, clienteID, candidatoID, autorID string) error {
	if _, err := s.candidatoPendente(ctx, clienteID, candidatoID); err != nil {
		return err
	}
	campos := map[string]any{
		"status":       "descartado",
		"decidido_em":  time.Now().UTC().Format(time.RFC3339),
		"decidido_por": nuloSeVazio(autorID),
	}
	return s.bd.Atualizar(ctx, "servicos_candidatos", "id=eq."+banco.Escapar(candidatoID), campos)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func nuloSeVazio(s string) any {
	if s == "" {
		return nil
	}
	return s
}
