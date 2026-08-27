// rev 1 — o relatório mensal que vai ao cliente
//
// O QUE ESTE ARQUIVO GUARDA
//
//	Um arquivo que sai da nossa mão e entra na planilha do CLIENTE. Errar aqui
//	não dá erro em lugar nenhum: dá uma cobrança no centro de custo errado, ou
//	uma margem revelada, e a conta chega semanas depois.
package orcamentos

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
)

// AS COLUNAS SÃO DO CLIENTE, E A ORDEM É CONTRATO
//
//	Este arquivo é lido do outro lado, provavelmente por PROCV. Renomear "Nº"
//	para "Item" ou trocar duas de lugar quebra a planilha de quem recebe — e o
//	sintoma aparece lá, não aqui.
func TestOModeloDoClienteTemAsOitoColunasNaOrdem(t *testing.T) {
	querido := []string{"Nº", "TICKET", "LOJA", "VALOR", "DATA", "ORÇAMENTO", "CONTA", "PCO"}
	if len(colunasDoRelatorioMensal) != len(querido) {
		t.Fatalf("o modelo tem %d colunas e o cliente espera %d",
			len(colunasDoRelatorioMensal), len(querido))
	}
	for i, q := range querido {
		if colunasDoRelatorioMensal[i].Titulo != q {
			t.Errorf("coluna %d é %q e deveria ser %q — o modelo do cliente não é nosso",
				i+1, colunasDoRelatorioMensal[i].Titulo, q)
		}
	}
	// DATA tem que ser data de verdade, não texto nem número solto.
	//   No modelo antigo ela saía como "46216" porque estava com formato Geral.
	if colunasDoRelatorioMensal[4].Tipo != relatorio.Data {
		t.Error("a coluna DATA não é do tipo Data — ela sai como número cru, " +
			"que foi o defeito do modelo antigo (aparecia 46216 em vez de 13/07/2026)")
	}
	for _, i := range []int{3, 5} {
		if colunasDoRelatorioMensal[i].Tipo != relatorio.Dinheiro {
			t.Errorf("a coluna %q não é dinheiro — sai sem somar no Excel",
				colunasDoRelatorioMensal[i].Titulo)
		}
	}
}

// A MARGEM NÃO SAI DAQUI
//
//	VALOR e ORÇAMENTO levam o MESMO número: o valor cobrado. No modelo do
//	cliente a segunda é a fórmula `=(D2)`, uma cópia da primeira. Pôr o valor da
//	NOTA numa e o COBRADO na outra entregaria a margem de 20% de bandeja — e o
//	sistema inteiro é construído para não mencioná-la.
func TestValorEOrcamentoLevamOMesmoNumeroECobrado(t *testing.T) {
	fonte, err := os.ReadFile("relatorio_mensal.go")
	if err != nil {
		t.Fatalf("não consegui ler o relatorio_mensal.go: %v", err)
	}
	texto := string(fonte)
	if strings.Contains(texto, "l.ValorNota") || strings.Contains(texto, "valor_nota") {
		t.Error("o relatório toca em `valor_nota` — a diferença entre ele e `valor` " +
			"É a margem, e este arquivo vai para as mãos do cliente")
	}
	// As duas colunas de dinheiro têm que sair da mesma variável.
	if strings.Count(texto, "l.Valor,") < 2 {
		t.Error("VALOR e ORÇAMENTO não saem do mesmo número — no modelo do cliente " +
			"a segunda é `=(D2)`, uma cópia da primeira")
	}
}

// A VIEW É A RÉGUA, E ELA É "AINDA NÃO COBRADO" — NÃO "DESTE MÊS"
//
//	*"todo orçamento inserido e não colocado em planilhas anteriores deve ser
//	colocado na planilha de agora"* — 27/08/2026.
//
//	Um filtro por mês faria um orçamento de julho que ninguém cobrou sumir para
//	sempre. Foi por isso que a aba "A faturar" também não tem filtro de mês.
func TestARegraEAindaNaoCobradoENaoDesteMes(t *testing.T) {
	sql, err := os.ReadFile("../../../../db/migrations/041_o_que_ainda_nao_foi_cobrado.sql")
	if err != nil {
		t.Fatalf("não achei a 041: %v", err)
	}
	regra := semComentarios(string(sql))

	// O TESTE OLHA O CORPO DA VIEW, NÃO O ARQUIVO INTEIRO
	//
	//	A primeira versão procurava `valor_nota` no arquivo todo e acusava o
	//	próprio `comment on view`, que EXPLICA por que a coluna não entra.
	//	Consertar seria apagar a explicação — o oposto do que se quer. O corpo
	//	vai do `create` até o `;` que o fecha.
	corpo := regra
	if i := strings.Index(corpo, "comment on view"); i > 0 {
		corpo = corpo[:i]
	}

	for _, exigido := range []struct{ trecho, porque string }{
		{"o.status = 'lancado'", "entraria orçamento que ainda nem subiu ao Trílogo"},
		{"o.fatura_id IS NULL", "a régua do que já foi cobrado sumiu — a planilha repetiria o mês passado"},
		{"o.faturamento_direto = false", "o direto é cobrado pelo fornecedor; cobrá-lo aqui é cobrar duas vezes"},
		{"security_invoker = true", "a view carrega dinheiro a cobrar e rodaria sem respeitar o login (P-35)"},
	} {
		if !strings.Contains(corpo, exigido.trecho) {
			t.Errorf("a 041 perdeu `%s`: %s", exigido.trecho, exigido.porque)
		}
	}
	// E o contrário: nada de janela de mês.
	for _, proibido := range []string{"date_trunc", "current_date", "interval"} {
		if strings.Contains(strings.ToLower(corpo), proibido) {
			t.Errorf("a 041 filtra por tempo (`%s`) — a régua é 'ainda não cobrado', "+
				"e um orçamento de julho não cobrado tem que entrar no relatório de dezembro", proibido)
		}
	}
	// `valor_nota` não pode nem existir no corpo da view.
	if strings.Contains(corpo, "valor_nota") {
		t.Error("a view expõe `valor_nota` — a diferença para `valor` é a margem")
	}
}

// A CONTA SAI COMO O CLIENTE A ESCREVE, ERRO DE DIGITAÇÃO INCLUÍDO
//
//	No cadastro dele está "MANUENÇÃO", sem o T. Corrigir a grafia aqui quebraria
//	o PROCV do outro lado — o texto é a chave, não uma legenda.
func TestAContaSaiComoOClienteAEscreve(t *testing.T) {
	casos := map[string]string{
		"civil":       "PREDIAL CIVIL - MANUENÇÃO CORRETIVA",
		"instalacoes": "PREDIAL INSTALAÇÕES - MANUENÇÃO CORRETIVA",
	}
	for conta, querido := range casos {
		if got := contaDoCliente(conta); got != querido {
			t.Errorf("contaDoCliente(%q) = %q, e o cadastro do cliente diz %q", conta, got, querido)
		}
	}
	// Conta que ninguém previu aparece como é, e não em branco: o desconhecido
	// tem que ser visível (P-38).
	if got := contaDoCliente("marketing"); got != "marketing" {
		t.Errorf("uma conta nova sumiu do relatório em vez de aparecer: %q", got)
	}
}

// A DATA VIRA DATA, E NÃO O NÚMERO DE SÉRIE CRU
//
//	A planilha que o dono enviou mostra "46216" na coluna DATA, porque a célula
//	ficou com formato Geral. Data em texto (ou em número cru) é a diferença
//	entre uma planilha que se ordena e uma que só se olha.
func TestADataViraDataDeVerdade(t *testing.T) {
	v := emData("2026-07-13")
	d, ok := v.(time.Time)
	if !ok {
		t.Fatalf("emData devolveu %T, e a planilha precisa de time.Time para gravar "+
			"a data como número de série COM formato", v)
	}
	if d.Format("2006-01-02") != "2026-07-13" {
		t.Errorf("emData mudou o dia: %s", d.Format("2006-01-02"))
	}
	// O DIA NÃO PODE ANDAR PARA TRÁS — O DEFEITO QUE SÓ O ARQUIVO MOSTROU
	//
	//	A primeira versão usava `time.Parse`, que devolve meia-noite em UTC. A
	//	planilha leva a data ao fuso da casa antes de contar os dias, e
	//	meia-noite UTC é 21h do dia ANTERIOR em Fortaleza. Gerei o .xlsx e todo
	//	13/07 saiu 12/07 — um dia a menos em cada linha do arquivo que vai ao
	//	cliente. Os testes concordavam consigo mesmos até eu abrir o arquivo.
	fuso := relatorio.FusoDaCasa()
	if d.In(fuso).Format("02/01/2006") != "13/07/2026" {
		t.Errorf("a data anda um dia no fuso da casa: %s — foi o defeito que o "+
			"arquivo gerado revelou", d.In(fuso).Format("02/01/2006 15:04"))
	}

	// Timestamp completo: o dia que vale é o de Fortaleza, não o de UTC.
	//   Um orçamento feito às 22h daqui chega como 01h UTC do dia seguinte, e o
	//   dia certo é o que a pessoa viu na tela quando o fez.
	tarde, ok := emData("2026-08-27T01:30:00+00:00").(time.Time)
	if !ok {
		t.Fatal("emData não aceita o timestamp que a view devolve")
	}
	if tarde.In(fuso).Format("02/01/2006") != "26/08/2026" {
		t.Errorf("01h30 UTC é 22h30 do dia 26 em Fortaleza, e a planilha diz %s",
			tarde.In(fuso).Format("02/01/2006"))
	}
	// E o que não é data não vira 30/12/1899.
	for _, lixo := range []string{"", "—", "2026"} {
		if emData(lixo) != nil {
			t.Errorf("emData(%q) inventou uma data", lixo)
		}
	}
}

// A TELA AVISA ANTES DE O ARQUIVO SAIR
//
//	Uma linha com a coluna LOJA em branco é uma linha que o cliente não consegue
//	lançar em centro de custo nenhum. O aviso tem que chegar com o botão ainda
//	na mão, não na devolutiva dele.
func TestATelaAvisaDaLojaSemNomeDoCliente(t *testing.T) {
	fonte, err := os.ReadFile("relatorio_mensal.go")
	if err != nil {
		t.Fatalf("não consegui ler o relatorio_mensal.go: %v", err)
	}
	if !strings.Contains(string(fonte), `"lojas_sem_nome"`) {
		t.Error("o motor não conta quais lojas estão sem o nome do cliente — elas " +
			"sairiam com a coluna LOJA em branco e ninguém saberia")
	}
	tela, err := os.ReadFile("../../../../web/src/telas/orcamentos/Faturamento.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	if !strings.Contains(string(tela), "lojas_sem_nome") {
		t.Error("a tela não lê `lojas_sem_nome` — o motor avisa e ninguém escuta")
	}
}

// SÓ EXCEL NESTA ABA
//
//	*"nesta tela, apenas a extração por excel"* — 27/08/2026. O arquivo É o
//	produto; um PDF seria uma segunda versão do mesmo documento, e duas versões
//	do que vai ao cliente divergem.
func TestORelatorioMensalNaoTemPDF(t *testing.T) {
	rotas, err := os.ReadFile("rotas.go")
	if err != nil {
		t.Fatalf("não consegui ler o rotas.go: %v", err)
	}
	texto := string(rotas)
	if !strings.Contains(texto, `"GET /orcamentos/relatorio-mensal.xlsx"`) {
		t.Error("a extração em Excel não está registrada")
	}
	if strings.Contains(texto, "relatorio-mensal.pdf") {
		t.Error("apareceu um PDF do relatório mensal — o dono pediu só Excel, e " +
			"duas versões do mesmo documento divergem")
	}
}

// A PRÉVIA E O ARQUIVO SAEM DA MESMA MONTAGEM
//
//	A tela mostra a planilha antes de ela sair. Se a prévia fosse montada por
//	outro caminho, um dia mostraria uma coisa e o arquivo levaria outra — e o
//	erro só apareceria do lado do cliente, que é o pior lugar possível.
//
//	*"a extracao deu certo, mas preciso colocar a planilha visivel na tela tb"*
//	— 27/08/2026.
func TestAPreviaDaTelaSaiDaMesmaMontagemQueOArquivo(t *testing.T) {
	fonte, err := os.ReadFile("relatorio_mensal.go")
	if err != nil {
		t.Fatalf("não consegui ler o relatorio_mensal.go: %v", err)
	}
	texto := string(fonte)

	// Uma função só, dois consumidores.
	if n := strings.Count(texto, "linhasDoModelo(linhas)"); n < 2 {
		t.Errorf("`linhasDoModelo` é chamada %d vez(es) — a prévia da tela e o "+
			"arquivo têm que sair dela, senão divergem", n)
	}
	if !strings.Contains(texto, `"linhas": linhasDoModelo(linhas)`) {
		t.Error("a rota da tela não devolve as linhas — a tela volta a mostrar " +
			"um retângulo com uma frase no meio, que não é a planilha")
	}
}

// A TELA DESENHA AS OITO COLUNAS, NA ORDEM
//
//	A prévia é a planilha. Mostrar sete colunas, ou trocar duas de lugar, faz a
//	pessoa conferir uma coisa e mandar outra.
func TestATelaDesenhaAsOitoColunasDoModelo(t *testing.T) {
	tela, err := os.ReadFile("../../../../web/src/telas/orcamentos/Faturamento.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	texto := string(tela)
	i := strings.Index(texto, `className="orc-tabela rel-mensal"`)
	if i < 0 {
		t.Fatal("a tela não desenha a tabela do relatório mensal")
	}
	cabeca := texto[i:]
	if j := strings.Index(cabeca, "</thead>"); j > 0 {
		cabeca = cabeca[:j]
	}
	for _, col := range []string{"Nº", "TICKET", "LOJA", "VALOR", "DATA", "ORÇAMENTO", "CONTA", "PCO"} {
		if !strings.Contains(cabeca, ">"+col+"<") {
			t.Errorf("a tabela da tela não tem a coluna %q — a prévia deixa de ser "+
				"a planilha que vai sair", col)
		}
	}
	// A LOJA QUE FALTA APARECE COMO FALTA
	//   Em branco ela some para o olho justamente na hora de conferir.
	if !strings.Contains(texto, "sem o nome do cliente") {
		t.Error("a loja sem `nome_cliente` sai como célula vazia na prévia — é a " +
			"linha que o cliente não conseguiria lançar, e ela precisa gritar")
	}
}

// O TEMA SEGUE A FRONTEIRA QUE O DONO DESENHOU
//
//	*"Todos os MENUS em tema escuro. O tema claro fica para a exibição das
//	LISTAS."* — 25/08/2026. A tabela usa as MESMAS classes das outras listas do
//	sistema; um estilo próprio aqui seria uma segunda aparência para a mesma
//	coisa, e ela divergiria no primeiro ajuste.
func TestATabelaUsaAsMesmasClassesDasOutrasListas(t *testing.T) {
	tela, err := os.ReadFile("../../../../web/src/telas/orcamentos/Faturamento.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	texto := string(tela)
	i := strings.Index(texto, `className="orc-tabela rel-mensal"`)
	if i < 0 {
		t.Fatal("a tabela do relatório mensal não usa `orc-tabela` — é a classe que " +
			"as outras listas usam, e sair dela cria uma segunda aparência para a " +
			"mesma coisa")
	}
	// A ROLAGEM É A QUE ENVOLVE ESTA TABELA, NÃO OUTRA QUALQUER DA TELA
	//   `orc-rolagem` aparece três vezes no arquivo, uma por aba. Procurar o
	//   nome solto deixaria passar justamente a aba que perdeu a sua.
	antes := texto[:i]
	if j := strings.LastIndex(antes, "<div"); j < 0 ||
		!strings.Contains(antes[j:], `className="orc-rolagem"`) {
		t.Error("a tabela do relatório mensal não está dentro de um " +
			"`orc-rolagem` — sem ele a tabela larga estoura a largura do cartão " +
			"em vez de rolar dentro dele")
	}
}
