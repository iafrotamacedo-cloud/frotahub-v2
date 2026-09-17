// rev 1 — o módulo Locações: recebimento de equipamento (Fase 1, 16/09/2026)
//
// POR QUE MÓDULO PRÓPRIO, E NÃO DENTRO DE administrativo
//
//	Locação vive na mesma esteira de Compras até o recebimento — a OC, a
//	leitura, o PCO, tudo idêntico, zero mudança em administrativo por causa
//	disto. O que vem DEPOIS do recebimento (vencimento, renovação,
//	devolução) não tem nada a ver com o ciclo de NF, então cresce à parte
//	(P-13): este módulo replica as poucas receitas de que precisa
//	(armazém, leitura de multipart) em vez de importar `administrativo`.
//
// FASE 1: SÓ RECEBER
//
//	A bifurcação mora em `administrativo/AguardandoNF.tsx` (front): em vez
//	de escanear a NF, o almoxarife escolhe "Locação". Isto cria os
//	equipamentos (um por item da OC, com vencimento) e marca a OC com
//	`destino_recebimento = 'locacao'` — é o que a tira de "Aguardando NF"
//	(ver a view `nf_progresso_ordens`, migração 072). Monitoramento,
//	devolução e renovação são as próximas fases do plano salvo em Locações.
package locacoes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// As quatro rotinas do módulo (migração 072) — separadas porque são quatro
// responsabilidades diferentes: o almoxarife recebe, qualquer um da
// hierarquia da obra decide (devolver/pedir renovação), só o RC conclui uma
// renovação inserindo a OC, e "monitorar" é só ver.
const (
	RotinaReceber   = "LOCACOES_RECEBER"
	RotinaMonitorar = "LOCACOES_MONITORAR"
	RotinaDecidir   = "LOCACOES_DECIDIR"
	RotinaRenovarOC = "LOCACOES_RENOVAR_OC"
)

// A mesma rotina que administrativo usa para "acesso por obra" — não
// importada (P-13), só o código, que é o dado estável (o catálogo de
// rotinas é o mesmo banco para todos os módulos).
const rotinaConfigurarAcessoObra = "COMPRAS_NF_CONFIGURAR_ACESSO"

const TamanhoMaximo = 25 << 20

type Modulo struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
	arm  *armazem.Cliente
	hist *historico.Servico
}

func Novo(bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico, arm *armazem.Cliente, hist *historico.Servico) *Modulo {
	return &Modulo{bd: bd, seg: seg, perm: perm, arm: arm, hist: hist}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /locacoes/ordens/{id}/itens", m.itensDaOrdem)
	mux.HandleFunc("POST /locacoes/ordens/{id}/receber", m.receber)
	mux.HandleFunc("GET /locacoes/arquivos/{sha}", m.arquivo)
	// O monitoramento (Fase 2, 16/09/2026) — ver o cabeçalho de monitoramento.go.
	mux.HandleFunc("GET /locacoes/painel", m.painel)
	mux.HandleFunc("GET /locacoes/equipamentos", m.equipamentos)
	mux.HandleFunc("GET /locacoes/equipamentos/{id}", m.equipamento)
	// Devolver e reabrir (Fase 3, 16/09/2026) — ver o cabeçalho de decisao.go.
	mux.HandleFunc("POST /locacoes/equipamentos/devolver", m.devolver)
	mux.HandleFunc("POST /locacoes/equipamentos/{id}/reabrir", m.reabrir)
	// Renovar, em duas etapas (Fase 4, 16/09/2026) — ver o cabeçalho de renovacao.go.
	mux.HandleFunc("POST /locacoes/equipamentos/renovar", m.renovar)
	mux.HandleFunc("POST /locacoes/renovacoes/{id}/cancelar", m.cancelarRenovacao)
	mux.HandleFunc("GET /locacoes/renovacoes", m.renovacoes)
	mux.HandleFunc("POST /locacoes/renovacoes/concluir", m.concluirRenovacao)
	// O cálculo do faturamento (Fase 5, 17/09/2026) — ver o cabeçalho de faturamento.go.
	mux.HandleFunc("GET /locacoes/faturamento", m.faturamento)
	// A busca de OC pra vincular como frete de desmobilização — ver decisao.go.
	mux.HandleFunc("GET /locacoes/ordens/buscar", m.buscarOrdemPorNumero)
}

// ---------------------------------------------------------------------------
// porteiro
// ---------------------------------------------------------------------------

func (m *Modulo) quemComRotina(w http.ResponseWriter, r *http.Request, rotina string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, rotina); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// quemEBuilder — mesmo desenho de `usuarios.quemEBuilder`: reabrir uma
// decisão (Fase 3) é a única ação do módulo travada no nível, não na
// matriz de rotinas — nem o RC nem o CEO desfazem sozinhos, só o builder.
func (m *Modulo) quemEBuilder(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := permissao.ExigeBuilder(p); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

func (m *Modulo) quemComQualquerRotina(w http.ResponseWriter, r *http.Request, rotinas ...string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	for _, rotina := range rotinas {
		pode, err := m.perm.Pode(r.Context(), p, rotina)
		if err != nil {
			m.erro(w, "não consegui conferir sua permissão", err)
			return nil
		}
		if !pode {
			continue
		}
		if p.ClienteID == "" {
			web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
			return nil
		}
		return p
	}
	web.Falhar(w, http.StatusForbidden, "Você não tem acesso a esta rotina.")
	return nil
}

// temAcessoAObra — mesma peneira de `administrativo.temAcessoAObra`: quem tem
// COMPRAS_NF_CONFIGURAR_ACESSO (ou é builder) vê qualquer obra; os demais só
// as que alguém de nível superior concedeu em `centro_custo_acessos`
// (migração 064 — a mesma tabela, não uma cópia para Locações).
func (m *Modulo) temAcessoAObra(ctx context.Context, p *seguranca.Principal, obraCentroCusto string) (bool, error) {
	if p.Builder() {
		return true, nil
	}
	if pode, err := m.perm.Pode(ctx, p, rotinaConfigurarAcessoObra); err == nil && pode {
		return true, nil
	}
	obra := strings.TrimSpace(obraCentroCusto)
	if obra == "" {
		return false, nil
	}
	centro, err := m.contarUm(ctx, "centros_custo?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&obra_centro_custo=eq."+banco.Escapar(obra)+"&select=id&limit=1")
	if err != nil {
		if err == errNaoAchei {
			return false, nil
		}
		return false, err
	}
	centroID := strCampo(centro["id"])
	if centroID == "" {
		return false, nil
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, "centro_custo_acessos?perfil_id=eq."+banco.Escapar(p.UserID)+
		"&centro_custo_id=eq."+banco.Escapar(centroID)+"&select=id&limit=1", &linhas); err != nil {
		return false, err
	}
	return len(linhas) > 0, nil
}

// ---------------------------------------------------------------------------
// utilidades pequenas — mesmo desenho de administrativo/modulo.go
// ---------------------------------------------------------------------------

var errNaoAchei = fmt.Errorf("não achei este registro")

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

func strCampo(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

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

func (m *Modulo) erro(w http.ResponseWriter, frase string, err error) {
	if err == errNaoAchei {
		web.Falhar(w, http.StatusNotFound, "Não achei este registro.")
		return
	}
	log.Printf("locacoes: %s: %v", frase, err)
	web.Falhar(w, http.StatusInternalServerError,
		"Não consegui completar: "+frase+". Tente de novo em instantes.")
}
