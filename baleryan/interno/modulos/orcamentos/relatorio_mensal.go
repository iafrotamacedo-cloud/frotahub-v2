// rev 1 — o relatório mensal que vai ao cliente
//
// O QUE ELE É
//
//	A relação de tudo que já foi LANÇADO no Trílogo e ainda NÃO foi cobrado do
//	cliente. É o documento que fecha o mês: sai daqui, vai para ele, e o que
//	estiver aqui hoje não pode estar no do mês que vem.
//
// A REGRA, NAS PALAVRAS DO DONO (27/08/2026)
//
//	"todo orçamento inserido e não colocado em planilhas anteriores deve ser
//	 colocado na planilha de agora"
//
//	Em SQL: `status = 'lancado' and fatura_id is null`. Não é uma janela de
//	datas — é o que ficou para trás, venha de quando vier. Um orçamento lançado
//	em julho que ninguém cobrou aparece no relatório de dezembro, e tem que
//	aparecer: a alternativa é ele nunca ser cobrado.
//
// POR QUE A RÉGUA É `fatura_id`, E POR QUE ELA JÁ ESTAVA CERTA
//
//	Medido em 27/08/2026 contra a planilha de julho que o dono enviou: dos 247
//	tickets dela, 245 já tinham `fatura_id` no sistema. Os que faltavam não eram
//	buraco — eram PARTES 2 e 3 de tickets cuja parte 1 fora cobrada (93523,
//	120586, 124822, 126400, 126474). A planilha lista o valor da PARTE, não a
//	soma do ticket. A régua está certa parte a parte, e essas seis entram agora.
//
//	Os outros 2 (120530 e 126119) são os que ficaram de fora da carga do legado
//	por não terem custo no Trílogo — já registrados como exceção.
//
// VALOR E ORÇAMENTO SÃO O MESMO NÚMERO, E ISSO É DE PROPÓSITO
//
//	No modelo do cliente a coluna ORÇAMENTO é a fórmula `=(D2)`: uma cópia de
//	VALOR. Pôr o valor da NOTA numa e o COBRADO na outra revelaria a margem de
//	20% ao cliente, que é justamente o que o sistema inteiro evita dizer. As
//	duas levam o valor cobrado.
//
// A DATA É A DA CRIAÇÃO DO ORÇAMENTO
//
//	Conferido contra a planilha de julho, seis tickets, todos batendo no dia:
//	`criado_em`. `lancado_em` não serve — nos 564 do legado ele é 15/08, a data
//	da carga, e não a do lançamento de verdade.
package orcamentos

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TetoDoRelatorio — o mesmo teto das outras extrações. Corte silencioso é pior
// que corte: quando bate, o documento diz que bateu.
const TetoDoRelatorio = TetoDaExtracao

// AsColunas do modelo que o cliente recebe, na ordem exata dele.
//
// A ORDEM E OS NOMES NÃO SÃO NOSSOS
//
//	Este arquivo é lido do outro lado, provavelmente por PROCV. Mudar "Nº" para
//	"Item" ou trocar duas colunas de lugar quebra a planilha de quem recebe, e o
//	sintoma aparece lá, não aqui.
var colunasDoRelatorioMensal = []relatorio.Coluna{
	{Titulo: "Nº", Peso: 5, Tipo: relatorio.Numero},
	{Titulo: "TICKET", Peso: 5, Tipo: relatorio.Numero},
	{Titulo: "LOJA", Peso: 24, Tipo: relatorio.Texto},
	{Titulo: "VALOR", Peso: 8, Tipo: relatorio.Dinheiro},
	{Titulo: "DATA", Peso: 9, Tipo: relatorio.Data},
	{Titulo: "ORÇAMENTO", Peso: 8, Tipo: relatorio.Dinheiro},
	{Titulo: "CONTA", Peso: 26, Tipo: relatorio.Texto},
	// PCO fica VAZIA, e a coluna existe por isso mesmo.
	//
	//	Decisão do dono em 27/08/2026: "a coluna PCO do modelo sera uma variavel
	//	que usaremos nos proximos passos, mas hoje, por enquanto, nao eh util".
	//
	//	Tirar a coluna faria a planilha deixar de casar com o modelo dele; e
	//	quando o PCO chegar, quem recebe teria que mudar a leitura. Ela vai
	//	vazia, no lugar certo, esperando.
	{Titulo: "PCO", Peso: 14, Tipo: relatorio.Texto},
}

// aCobrar é uma linha do relatório, já com o nome que o CLIENTE usa.
type aCobrar struct {
	Ticket    int     `json:"ticket"`
	Parte     int     `json:"parte"`
	Valor     float64 `json:"valor"`
	CriadoEm  string  `json:"criado_em"`
	Conta     string  `json:"conta"`
	Loja      *string `json:"loja_cliente"`
	LojaNossa *string `json:"loja"`
	Unidade   *string `json:"unidade_id"`
}

// AsContasComoOClienteAsChama.
//
//	No banco a conta é `civil` ou `instalacoes`. No cadastro do cliente ela é o
//	centro de custo por extenso — e com "MANUENÇÃO" sem o T, que é como está
//	escrito lá. Corrigir a grafia aqui quebraria o PROCV dele.
func contaDoCliente(conta string) string {
	switch conta {
	case "civil":
		return "PREDIAL CIVIL - MANUENÇÃO CORRETIVA"
	case "instalacoes":
		return "PREDIAL INSTALAÇÕES - MANUENÇÃO CORRETIVA"
	}
	// Conta que ninguém previu aparece como é, e não em branco: o desconhecido
	// tem que ser visível (P-38).
	return conta
}

// orcamentosACobrar são os lançados que nenhuma planilha anterior levou.
func (m *Modulo) orcamentosACobrar(ctx context.Context, clienteID string) ([]aCobrar, error) {
	var linhas []aCobrar
	err := m.bd.Buscar(ctx, "orcamentos_a_cobrar?cliente_id=eq."+banco.Escapar(clienteID)+
		"&order=criado_em,ticket,parte&limit="+fmt.Sprint(TetoDoRelatorio+1)+"&select=*", &linhas)
	return linhas, err
}

// GET /orcamentos/relatorio-mensal — o que a tela mostra antes de extrair.
//
// A tela não lista os 408: ela diz QUANTOS e QUANTO, e o que está sem o nome do
// cliente. Uma lista de quatrocentas linhas na tela não ajuda a decidir nada — a
// decisão é uma só, "mando ou não mando", e ela se toma com dois números.
func (m *Modulo) relatorioMensal(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaFaturar)
	if p == nil {
		return
	}
	linhas, err := m.orcamentosACobrar(r.Context(), p.ClienteID)
	if err != nil {
		m.erro(w, "não consegui montar o relatório", err)
		return
	}

	var soma float64
	var semNome []string
	de, ate := "", ""
	for _, l := range linhas {
		soma += l.Valor
		if l.Loja == nil || *l.Loja == "" {
			semNome = append(semNome, ouVazioTexto(l.LojaNossa))
		}
		if de == "" || l.CriadoEm < de {
			de = l.CriadoEm
		}
		if l.CriadoEm > ate {
			ate = l.CriadoEm
		}
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"quantos": len(linhas),
		"valor":   soma,
		"de":      de,
		"ate":     ate,
		// AS LOJAS SEM O NOME DO CLIENTE APARECEM ANTES DE O ARQUIVO SAIR
		//   Sair com a coluna LOJA em branco é mandar ao cliente uma linha que
		//   ele não consegue lançar em centro de custo nenhum. Melhor ele saber
		//   aqui, com o botão ainda na mão.
		"lojas_sem_nome": semDuplicatas(semNome),
		"teto":           TetoDoRelatorio,
	})
}

// GET /orcamentos/relatorio-mensal.xlsx
func (m *Modulo) relatorioMensalExcel(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaFaturar)
	if p == nil {
		return
	}
	linhas, err := m.orcamentosACobrar(r.Context(), p.ClienteID)
	if err != nil {
		m.erro(w, "não consegui montar o relatório", err)
		return
	}

	aviso := ""
	if len(linhas) > TetoDoRelatorio {
		linhas = linhas[:TetoDoRelatorio]
		aviso = fmt.Sprintf("A relação foi cortada em %d linhas. Há mais orçamentos "+
			"a cobrar do que cabe num arquivo — feche este e gere de novo.", TetoDoRelatorio)
	}

	var soma float64
	corpo := make([][]any, 0, len(linhas))
	for i, l := range linhas {
		soma += l.Valor
		corpo = append(corpo, []any{
			i + 1,
			l.Ticket,
			ouVazioTexto(l.Loja),
			l.Valor,
			emData(l.CriadoEm),
			// ORÇAMENTO repete VALOR — ver o cabeçalho do arquivo.
			l.Valor,
			contaDoCliente(l.Conta),
			// PCO, vazia por enquanto.
			"",
		})
	}

	tab := relatorio.Tabela{
		Titulo:    "Orçamentos de materiais",
		Aba:       "Orçamentos",
		Subtitulo: fmt.Sprintf("%d orçamentos · R$ %.2f · ainda não cobrados", len(linhas), soma),
		Colunas:   colunasDoRelatorioMensal,
		Linhas:    corpo,
		Aviso:     aviso,
		Gerado:    time.Now(),
	}
	corpoXLSX, err := tab.Planilha()
	if err != nil {
		m.erro(w, "não consegui montar a planilha", err)
		return
	}
	entregar(w, corpoXLSX, "orcamentos-materiais", "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

// emData transforma o carimbo do banco no `time.Time` que a planilha grava como
// data de verdade — número de série com formato — e não como texto.
//
// Data em texto é a diferença entre uma planilha que se ordena e uma que só se
// olha. E era assim que o modelo antigo saía: a coluna DATA mostrava "46216".
//
// O DIA É O DE FORTALEZA, E ISSO CUSTOU UM DIA INTEIRO NA PRIMEIRA VERSÃO
//
//	A primeira versão fazia `time.Parse`, que devolve MEIA-NOITE EM UTC. A
//	planilha converte para o fuso da casa antes de contar os dias — e meia-noite
//	UTC é 21h do dia ANTERIOR em Fortaleza. Gerei o arquivo e o 13/07 saiu como
//	12/07: um dia a menos em toda linha.
//
//	Os testes do código concordavam consigo mesmos; só abrir o .xlsx mostrou. É
//	a mesma armadilha que o comentário do `serieDoExcel` já descrevia, e eu caí
//	nela do outro lado da fronteira.
//
//	Aqui a data é montada NO FUSO DA CASA. `criado_em` é timestamptz: um
//	orçamento feito às 22h de Fortaleza chega como 01h UTC do dia seguinte, e o
//	dia que vale é o que a pessoa viu na tela quando o fez.
func emData(iso string) any {
	if len(iso) < 10 {
		return nil
	}
	fuso := relatorio.FusoDaCasa()

	// Com hora: leva ao fuso da casa e fica com o DIA de lá.
	if len(iso) > 10 {
		if t, err := time.Parse(time.RFC3339, iso); err == nil {
			l := t.In(fuso)
			return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, fuso)
		}
	}
	// Só o dia: já é o dia local, e nasce no fuso da casa.
	t, err := time.ParseInLocation("2006-01-02", iso[:10], fuso)
	if err != nil {
		return nil
	}
	return t
}

func ouVazioTexto(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func semDuplicatas(v []string) []string {
	visto := map[string]bool{}
	saida := make([]string, 0, len(v))
	for _, s := range v {
		if s == "" || visto[s] {
			continue
		}
		visto[s] = true
		saida = append(saida, s)
	}
	return saida
}
