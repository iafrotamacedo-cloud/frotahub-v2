package rogueworker

import (
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/orcamentos"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

// telaConhecida espelha a árvore de `web/src/menu/arvore.ts`. A fonte da
// verdade da navegação CONTINUA sendo a árvore do front: daqui só sai o
// caminho de `rota` para o Casca chamar a mesma `navegar()` do clique.
// Permissão é a rotina da folha — se o menu esconde, a Worker também recusa.
type telaConhecida struct {
	rotas  []string
	tela   string
	rotina string
	nomes  []string
}

var catalogoDeTelas = []telaConhecida{
	{
		rotas: []string{"manutencao", "dados-trilogo"},
		tela:  "trilogo-dados", rotina: "CONTRATO_TRILOGO_DADOS",
		nomes: []string{"dados do trilogo", "trilog", "chamados do contrato"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "orcamentos"},
		tela:  "orcamentos", rotina: orcamentos.RotinaModulo,
		nomes: []string{"orcamento", "notas fiscais", "fila de notas"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "financeiro", "a-pagar"},
		tela:  "a-pagar", rotina: orcamentos.RotinaPagar,
		nomes: []string{"a pagar", "pagar fornecedor", "pedido de faturamento", "davs"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "financeiro", "a-receber"},
		tela:  "faturar", rotina: orcamentos.RotinaFaturar,
		nomes: []string{"a receber", "faturar ao cliente", "faturamento ao cliente"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "financeiro", "consolidacao"},
		tela:  "consolidacao", rotina: "CONTRATO_FINANCEIRO_CONSOLIDACAO",
		nomes: []string{"consolidacao", "consolidacao financeira", "obra prima"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "estatisticas"},
		tela:  "est-raiz", rotina: "CONTRATO_ESTATISTICAS",
		nomes: []string{"estatistica", "estatisticas"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "estatisticas", "operacionais", "chamados"},
		tela:  "est-chamados", rotina: "CONTRATO_ESTATISTICAS",
		nomes: []string{"estatistica de chamados", "chamados de todas as lojas", "chamados de todas lojas"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "estatisticas", "operacionais", "onde"},
		tela:  "est-onde", rotina: "CONTRATO_ESTATISTICAS",
		nomes: []string{"onde estao os chamados", "chamados por loja"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "estatisticas", "operacionais", "fila"},
		tela:  "est-fila", rotina: "CONTRATO_ESTATISTICAS",
		nomes: []string{"fila de hoje"},
	},
	{
		rotas: []string{"manutencao", "contrato-sao-luiz", "estatisticas", "financeiras", "faturamento"},
		tela:  "est-faturamento", rotina: "CONTRATO_ESTATISTICAS",
		nomes: []string{"estatistica de faturamento"},
	},
	{
		rotas: []string{"manutencao", "servicos"},
		tela:  "servicos-hub", rotina: "CONTRATO_SERVICO_GERENCIAR",
		nomes: []string{"hub de servicos", "kanban de servico", "servicos novos"},
	},
	{
		rotas: []string{"sesmt-dp", "funcionarios"},
		tela:  "funcionarios", rotina: "CONTRATO_FUNCIONARIOS_DADOS",
		nomes: []string{"funcionarios", "sesmt", "documentos de funcionario"},
	},
	{
		rotas: []string{"configuracoes", "minha-conta"},
		tela:  "minha-conta", rotina: "",
		nomes: []string{"minha conta", "minhas configuracoes", "trocar senha"},
	},
}

func telaPorNome(tela string) *telaConhecida {
	for i := range catalogoDeTelas {
		if catalogoDeTelas[i].tela == tela {
			return &catalogoDeTelas[i]
		}
	}
	return nil
}

func (m *Modulo) executarNavegar(r *http.Request, p *seguranca.Principal, rec reconhecimento) Resposta {
	t := telaPorNome(rec.tela)
	if t == nil {
		return Resposta{Texto: "Não achei essa tela no menu. Diga o nome como ele aparece na barra lateral — por exemplo, consolidação financeira, orçamentos, estatística de chamados."}
	}
	if t.rotina != "" && !m.pode(r, p, t.rotina) {
		return Resposta{Texto: fraseRecusa}
	}
	return Resposta{
		Texto:   "Abrindo " + t.nomes[0] + ".",
		Navegar: &DestinoNav{Rotas: t.rotas, Tela: t.tela, Rotina: t.rotina},
	}
}
