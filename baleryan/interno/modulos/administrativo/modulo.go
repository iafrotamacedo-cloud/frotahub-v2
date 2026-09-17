// rev 2 — o módulo Administrativo: Compras
//
// PASSO 1: INSERIR E LISTAR. PASSO 2 (10/09/2026): LER
//
//	O pedido do dono (10/09/2026) foi específico: uma tela de inserção com um
//	botão para escolher o arquivo, um para inserir, e uma lista da fila
//	embaixo — isso é o Passo 1. O Passo 2 acrescenta a leitura determinística
//	do PDF (`leitura.go`/`ler.go`): número, comprador, fornecedor, itens e
//	totais, mais os dois filtros de negócio que decidem `lido` ou `falhou`
//	(ver o cabeçalho de `leitura.go`). As três vistas (fila/processadas/
//	rejeitadas) seguem o mesmo desenho de Orçamentos — `filtroDasOrdens` em
//	`ordens.go` é quem decide a consulta.
//
// PCO, Notas fiscais e Locações ainda não têm código nenhum — continuam em
// `<EmBreve>` no front até ganharem a vez (ver `claude/fase5-administrativo-
// planejamento.md` no Projeto).
package administrativo

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/brevo"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/eraleitura"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// RotinaOrdens é a única rotina deste primeiro passo — migração 059.
const RotinaOrdens = "COMPRAS_ORDENS_GERENCIAR"

// As duas rotinas do PCO (migração 061) — SEPARADAS de propósito: editar
// quem recebe o e-mail é uma coisa, apertar o botão de enviar é outra (ver
// o cabeçalho da migração).
const (
	RotinaPCODestinatarios = "COMPRAS_PCO_DESTINATARIOS"
	RotinaPCOEnviar        = "COMPRAS_PCO_ENVIAR"
)

// As três rotinas de Notas Fiscais (migração 064, 12/09/2026) — SEPARADAS
// pelo mesmo motivo do PCO: receber a nota (almoxarife, na obra) e confirmar
// entrega/envio (escritório) são responsabilidades diferentes, e conceder
// acesso por obra é coisa de nível superior — ver o cabeçalho de
// `notas_fiscais.go` e `acessos_obra.go`.
//
// MAIS DUAS, MIGRAÇÃO 075 (17/09/2026, obra piloto MSL Fátima)
//
//	COMPRAS_NF_RECEBER_PDF: subir a nota já em PDF (veio por e-mail do
//	fornecedor) em vez de escanear pela câmera — capacidade À PARTE de
//	RECEBER, não incluída nela. O dono pediu por categoria (não por login),
//	sem trava de nível — "compras e adm tb devem poder fazer isso".
//
//	COMPRAS_NF_ENVIAR_CLIENTE: ENTREGAR virava sem querer duas
//	responsabilidades na mesma rotina — confirmar que a nota chegou fisicamente
//	no escritório, e marcar que ela SAIU no malote pro cliente. O dono pediu
//	pra "deixar bem definida" a permissão de recebimento no escritório —
//	ENTREGAR agora é só isso; enviar ao cliente ganha rotina própria, do
//	mesmo jeito que RECEBER/ENTREGAR já eram separadas uma da outra.
const (
	RotinaNFReceber          = "COMPRAS_NF_RECEBER"
	RotinaNFReceberPDF       = "COMPRAS_NF_RECEBER_PDF"
	RotinaNFEntregar         = "COMPRAS_NF_ENTREGAR"
	RotinaNFEnviarCliente    = "COMPRAS_NF_ENVIAR_CLIENTE"
	RotinaNFConfigurarAcesso = "COMPRAS_NF_CONFIGURAR_ACESSO"
)

// TamanhoMaximo de um arquivo de OC aceito na inserção. Mesmo teto de
// Orçamentos: PDF de OC digital não passa de poucos MB.
const TamanhoMaximo = 25 << 20

// TetoDaLista limita quantas ordens a fila devolve de uma vez. Não é
// paginação de verdade — não foi pedida — é o mesmo cuidado de
// `TetoDaLeitura` em Orçamentos: um número, e não "tudo o que existir".
const TetoDaLista = 500

type Modulo struct {
	cfg   *config.Config
	bd    *banco.Cliente
	seg   *seguranca.Servico
	perm  *permissao.Servico
	arm   *armazem.Cliente
	hist  *historico.Servico
	brevo *brevo.Cliente
	era   *eraleitura.Motor
	// A IA (Gemini) que sugere número e valor da NF na hora do escaneamento
	// — ver o cabeçalho de nf_era.go. Sem chave, `Ligada()` é falso e a
	// sugestão simplesmente não vem.
	ia *leitor.IA
	// A fila do ERA READ: uma goroutine só lê as páginas, uma por vez, e
	// grava `leitura_era`. Nula quando o ERA está desligado.
	filaERA chan trabalhoERA
}

func Novo(cfg *config.Config, bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico,
	arm *armazem.Cliente, hist *historico.Servico, era *eraleitura.Motor) *Modulo {
	m := &Modulo{
		cfg: cfg, bd: bd, seg: seg, perm: perm, arm: arm, hist: hist, era: era,
		brevo: brevo.Novo(cfg.Brevo),
		ia:    novaIA(cfg.IA.Chave, cfg.IA.Modelo, cfg.IA.IntervaloSegundos),
	}
	if era != nil && era.Ligado() {
		m.filaERA = make(chan trabalhoERA, TetoDaFilaERA)
		go m.trabalharERA()
	}
	return m
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /administrativo/compras/ordens", m.listarOrdens)
	mux.HandleFunc("GET /administrativo/compras/ordens/painel", m.painelDeOrdens)
	mux.HandleFunc("POST /administrativo/compras/ordens", m.inserirOrdens)
	mux.HandleFunc("GET /administrativo/compras/ordens/{id}", m.verOrdem)
	mux.HandleFunc("GET /administrativo/compras/ordens/{id}/arquivo", m.arquivoDaOrdem)
	// A leitura da OC (Passo 2, 10/09/2026) — ver o cabeçalho de `ler.go`.
	mux.HandleFunc("GET /administrativo/compras/ordens/porler", m.ordensPorLer)
	mux.HandleFunc("POST /administrativo/compras/ordens/{id}/ler", m.lerOrdem)
	mux.HandleFunc("GET /administrativo/compras/ordens/{id}/documento", m.verDocumento)
	mux.HandleFunc("POST /administrativo/compras/ordens/{id}/documento", m.salvarDocumento)
	// Excluir e substituir (11/09/2026) — ver o cabeçalho de `substituicao.go`.
	mux.HandleFunc("DELETE /administrativo/compras/ordens/{id}", m.excluirOrdem)
	mux.HandleFunc("POST /administrativo/compras/ordens/{id}/substituir", m.substituirOrdem)
	// Correção de OC — NF com valor divergente (migração 076, 17/09/2026) —
	// ver o cabeçalho de `correcao_oc.go`.
	mux.HandleFunc("POST /administrativo/compras/ordens/{id}/voltar-aguardando", m.voltarOCParaAguardando)
	mux.HandleFunc("POST /administrativo/compras/ordens/{id}/corrigir", m.corrigirOrdemComDivergencia)
	// O hub de PCO (10/09/2026) — ver o cabeçalho de `painelDoPCO` em ordens.go.
	mux.HandleFunc("GET /administrativo/compras/pco/painel", m.painelDoPCO)
	// O envio por e-mail (11/09/2026) — ver o cabeçalho de `pco_enviar.go`.
	mux.HandleFunc("POST /administrativo/compras/pco/enviar", m.enviarPCO)
	mux.HandleFunc("POST /administrativo/compras/pco/ordens/{id}/enviar", m.enviarUmaPCO)
	// Substituir (qualquer OC, sem exigir o mesmo número) e o registro de
	// excluídas/substituídas depois do envio (12/09/2026) — ver o
	// cabeçalho de `cancelamento.go`.
	mux.HandleFunc("POST /administrativo/compras/pco/ordens/{id}/substituir", m.substituirOrdemPCO)
	mux.HandleFunc("GET /administrativo/compras/pco/canceladas", m.listarCanceladas)
	mux.HandleFunc("GET /administrativo/compras/pco/canceladas/{id}/arquivo", m.arquivoDaCancelada)
	// Os destinatários — ver o cabeçalho de `destinatarios.go`.
	mux.HandleFunc("GET /administrativo/compras/pco/destinatarios", m.listarDestinatarios)
	mux.HandleFunc("POST /administrativo/compras/pco/destinatarios", m.criarDestinatario)
	mux.HandleFunc("PATCH /administrativo/compras/pco/destinatarios/{id}", m.alterarDestinatario)
	mux.HandleFunc("GET /administrativo/compras/pco/destinatarios/{id}/historico", m.historicoDestinatario)
	// Notas Fiscais — Bloco A (12/09/2026) — ver o cabeçalho de `notas_fiscais.go`.
	mux.HandleFunc("GET /administrativo/nf/painel", m.painelDeNF)
	mux.HandleFunc("GET /administrativo/nf/aguardando", m.ordensAguardandoNF)
	mux.HandleFunc("GET /administrativo/nf/ordens/{id}", m.notasDaOrdem)
	mux.HandleFunc("GET /administrativo/nf/ordens/{id}/arquivo", m.arquivoDaOrdemNF)
	mux.HandleFunc("POST /administrativo/nf/ordens/{id}/escanear", m.escanearNF)
	mux.HandleFunc("POST /administrativo/nf/ordens/{id}/receber", m.receberNF)
	mux.HandleFunc("POST /administrativo/nf/ordens/{id}/marcar-correcao", m.marcarCorrecaoManual)
	mux.HandleFunc("POST /administrativo/nf/notas/{id}/paginas", m.paginaNF)
	mux.HandleFunc("GET /administrativo/nf/notas/{id}/paginas", m.paginasDaNF)
	mux.HandleFunc("GET /administrativo/nf/recebidas", m.listarNFRecebidas)
	mux.HandleFunc("GET /administrativo/nf/entregues", m.listarNFEntregues)
	mux.HandleFunc("GET /administrativo/nf/enviadas", m.listarNFEnviadas)
	// "notas/{id}/..." e não "{id}/...": `{id}/arquivo` colidiria com
	// `ordens/{id}` de cima (nenhum dos dois é mais específico que o outro,
	// e o ServeMux do Go 1.22+ recusa registrar isso — pane em runtime).
	mux.HandleFunc("POST /administrativo/nf/notas/{id}/entregar", m.entregarNF)
	mux.HandleFunc("POST /administrativo/nf/notas/{id}/enviar-cliente", m.enviarNFAoCliente)
	mux.HandleFunc("POST /administrativo/nf/notas/{id}/cancelar", m.cancelarNF)
	mux.HandleFunc("POST /administrativo/nf/notas/{id}/trocar", m.trocarNF)
	mux.HandleFunc("GET /administrativo/nf/notas/{id}/arquivo", m.arquivoDaNF)
	// A configuração de acesso por obra (12/09/2026) — ver o cabeçalho de `acessos_obra.go`.
	mux.HandleFunc("GET /administrativo/nf/obras", m.listarObrasNF)
	mux.HandleFunc("GET /administrativo/nf/perfis", m.listarPerfisNF)
	mux.HandleFunc("GET /administrativo/nf/acessos", m.listarAcessosNF)
	mux.HandleFunc("POST /administrativo/nf/acessos", m.concederAcessoNF)
	mux.HandleFunc("DELETE /administrativo/nf/acessos/{id}", m.revogarAcessoNF)
}

// ---------------------------------------------------------------------------
// porteiro
// ---------------------------------------------------------------------------

// quem identifica e confere a rotina numa tacada — mesmo desenho de
// Orçamentos. Devolve nil quando já respondeu o erro.
func (m *Modulo) quem(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, RotinaOrdens); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// ---------------------------------------------------------------------------
// utilidades pequenas
// ---------------------------------------------------------------------------

var errNaoAchei = errors.New("não achei este registro")

// umUUID recusa qualquer coisa que não tenha cara de uuid ANTES de virar
// filtro — a diferença entre um 404 limpo e um filtro estranho chegando no
// banco (mesma função de Orçamentos).
func umUUID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return "", false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return "", false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", false
		}
	}
	return s, true
}

func ouVazio(l []map[string]any) []map[string]any {
	if l == nil {
		return []map[string]any{}
	}
	return l
}

// contarUm busca uma linha só e devolve errNaoAchei quando não vem nada.
func (m *Modulo) contarUm(ctx context.Context, caminho string) (map[string]any, error) {
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		return nil, errNaoAchei
	}
	return linhas[0], nil
}

// erro traduz uma falha em resposta, sem despejar detalhe de banco na tela.
func (m *Modulo) erro(w http.ResponseWriter, frase string, err error) {
	if err == errNaoAchei {
		web.Falhar(w, http.StatusNotFound, "Não achei este registro.")
		return
	}
	log.Printf("administrativo: %s: %v", frase, err)
	web.Falhar(w, http.StatusInternalServerError,
		"Não consegui completar: "+frase+". Tente de novo em instantes.")
}
