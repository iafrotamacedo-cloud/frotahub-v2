// rev 1 — a trava de duplicidade no lançamento
//
// O QUE ESTE ARQUIVO PROTEGE
//
//	Um custo lançado duas vezes no mesmo ticket é dinheiro cobrado duas vezes do
//	cliente, e ninguém olhando o Trílogo consegue dizer qual dos dois é o certo.
//	Antes desta trava, o único obstáculo era o teto de R$ 60.000 — que uma
//	duplicata de R$ 143,28 atravessa sem encostar.
package orcamentos

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

func f(v float64) *float64 { return &v }

// O CASO MEDIDO NA BASE, EM 26/08/2026
//
//	Ticket 126998: custo nº 37671 de R$ 143,28 já lançado, orçamento gerado de
//	R$ 143,28 esperando o botão. Este teste é aquele ticket.
func TestCustoIdenticoNoMesmoTicketEhEncontrado(t *testing.T) {
	custos := []trilogo.Custo{
		{ID: 37671, Type: trilogo.TipoMaoDeObra, TotalValue: f(143.28)},
	}
	iguais := custosIguais(custos, regras.DinheiroDe(143.28))
	if len(iguais) != 1 || iguais[0].ID != 37671 {
		t.Fatalf("não achei o custo repetido do ticket 126998: %+v", iguais)
	}
}

// O VALOR PODE ESTAR EM QUALQUER UM DOS TRÊS CAMPOS
//
//	Custo antigo vem só com `serviceCost` ou só com `productCost`. Uma escada que
//	olhasse apenas `totalValue` acharia zero duplicata e passaria a impressão de
//	estar conferindo — que é pior do que não conferir.
func TestOValorEhAchadoNosTresCampos(t *testing.T) {
	casos := []struct {
		nome  string
		custo trilogo.Custo
	}{
		{"totalValue", trilogo.Custo{ID: 1, TotalValue: f(588.96)}},
		{"serviceCost", trilogo.Custo{ID: 2, ServiceCost: f(588.96)}},
		{"productCost", trilogo.Custo{ID: 3, ProductCost: f(588.96)}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if len(custosIguais([]trilogo.Custo{c.custo}, regras.DinheiroDe(588.96))) != 1 {
				t.Errorf("custo com o valor em %s passou despercebido", c.nome)
			}
		})
	}
}

// A ESCADA É UMA SÓ
//
//	Se `SomaDosCustos` e a trava lessem o valor de maneiras diferentes, um dia
//	uma acharia R$ 143,28 onde a outra achasse zero — e a que enxerga zero é a
//	que deixa passar o lançamento repetido.
func TestASomaDoTetoEATravaLeemOMesmoValor(t *testing.T) {
	custos := []trilogo.Custo{
		{ID: 1, ServiceCost: f(100)},
		{ID: 2, ProductCost: f(50)},
		{ID: 3, TotalValue: f(25), ServiceCost: f(999)},
	}
	if soma := trilogo.SomaDosCustos(custos); regras.DinheiroDe(soma) != regras.DinheiroDe(175) {
		t.Fatalf("a soma deu %v — a trava e o teto discordam sobre o que é o valor de um custo", soma)
	}
}

// CENTAVOS, NUNCA float
//
//	O 143,28 que vem do JSON deles e o que vem do nosso `numeric` não são
//	necessariamente o mesmo float64 — basta um dos dois ter passado por uma
//	conta. Comparar com `==` cru deixaria a duplicata escapar por uma diferença
//	invisível na décima terceira casa, e o cliente pagaria duas vezes por causa
//	de um bit.
func TestOSujeiraDoFloatNaoEscondeADuplicata(t *testing.T) {
	quase := 143.28 + 1e-12 // o mesmo dinheiro, outro float64
	if quase == 143.28 {
		t.Fatal("o valor de teste não é diferente de 143.28 — o teste não prova nada")
	}
	if len(custosIguais([]trilogo.Custo{{ID: 9, TotalValue: &quase}}, regras.DinheiroDe(143.28))) != 1 {
		t.Error("a duplicata escapou por diferença de float — a comparação não está em centavos")
	}
}

// ...MAS CENTAVO A MAIS É OUTRO VALOR
//
//	A régua tem que ser o centavo, e não "mais ou menos igual". Se ela fosse
//	frouxa, dois orçamentos legítimos de valores vizinhos viravam duplicata um do
//	outro e a fila travava inteira.
func TestUmCentavoDeDiferencaNaoEhDuplicata(t *testing.T) {
	if len(custosIguais([]trilogo.Custo{{ID: 9, TotalValue: f(143.29)}}, regras.DinheiroDe(143.28))) != 0 {
		t.Error("R$ 143,29 foi tratado como igual a R$ 143,28 — a régua não é o centavo")
	}
}

// VALOR DIFERENTE NÃO É DUPLICATA
//
//	A trava tem que ser estreita. Se ela barrasse qualquer custo existente, o
//	segundo orçamento legítimo de um ticket — que é rotina aqui — nunca subiria.
func TestValorDiferenteNaoTrava(t *testing.T) {
	custos := []trilogo.Custo{{ID: 1, TotalValue: f(243.85)}}
	if len(custosIguais(custos, regras.DinheiroDe(135.12))) != 0 {
		t.Error("travou um custo de valor diferente — o ticket 126406 nunca subiria")
	}
}

// AS DUAS, E NÃO A PRIMEIRA
//
//	Se o ticket já foi duplicado antes, quem abrir a resposta precisa ver os dois
//	para saber qual apagar.
func TestTodasAsRepetidasVoltamNaResposta(t *testing.T) {
	custos := []trilogo.Custo{
		{ID: 1, TotalValue: f(233.76)},
		{ID: 2, TotalValue: f(10)},
		{ID: 3, TotalValue: f(233.76)},
	}
	if len(custosIguais(custos, regras.DinheiroDe(233.76))) != 2 {
		t.Error("a resposta mostraria só uma das repetidas — a outra ficaria escondida")
	}
}

// A CONFIRMAÇÃO É UMA DECISÃO, NÃO UM PARÂMETRO QUE APARECEU
func TestSoAPalavraExataConfirma(t *testing.T) {
	casos := map[string]bool{
		"/x":                    false,
		"/x?duplicata=":         false,
		"/x?duplicata=1":        false,
		"/x?duplicata=true":     false,
		"/x?duplicata=confirmo": true,
		"/x?duplicata=CONFIRMO": false,
	}
	for url, esperado := range casos {
		r := httptest.NewRequest("POST", url, nil)
		if duplicataConfirmada(r) != esperado {
			t.Errorf("%s: esperava confirmação=%v", url, esperado)
		}
	}
}

// A TRAVA VEM ANTES DE QUALQUER GASTO
//
//	A ordem antiga já custou caro uma vez: o arquivo subia ao Trílogo antes da
//	conferência do status, e quando o custo era recusado o PDF ficava órfão no S3
//	do cliente, sem dono e sem endpoint para apagar. Uma trava que rodasse depois
//	do upload repetiria exatamente esse erro — e ainda deixaria um documento
//	pendurado num ticket que a gente decidiu não cobrar.
func TestATravaRodaAntesDeSubirArquivo(t *testing.T) {
	fonte, err := os.ReadFile("lancar.go")
	if err != nil {
		t.Fatalf("não consegui ler o arquivo: %v", err)
	}
	corpo := string(fonte)
	corpo = corpo[strings.Index(corpo, "func (m *Modulo) lancar("):]
	corpo = corpo[:strings.Index(corpo, "\nfunc ")]

	trava := strings.Index(corpo, "custosIguais(")
	if trava < 0 {
		t.Fatal("o `lancar` não confere duplicidade nenhuma")
	}
	for _, gasto := range []string{"m.arquivoParaLancar(", "m.guardarPDF(", "sessao.SubirArquivo(", "sessao.CriarCusto("} {
		if i := strings.Index(corpo, gasto); i >= 0 && i < trava {
			t.Errorf("%s acontece ANTES da trava de duplicidade — o arquivo subiria e ficaria órfão", gasto)
		}
	}
}

// A TRAVA FALA ANTES DO TETO
//
//	Uma duplicata quase sempre estoura o teto também — o valor repetido entra na
//	soma duas vezes. Se o teto respondesse primeiro, a tela ofereceria "aprovar
//	com a autorização do cliente" para uma cobrança repetida: a trava mandaria a
//	pessoa exatamente para o botão que não devia existir ali.
func TestADuplicataFalaAntesDoTeto(t *testing.T) {
	fonte, err := os.ReadFile("lancar.go")
	if err != nil {
		t.Fatalf("não consegui ler o arquivo: %v", err)
	}
	corpo := string(fonte)
	corpo = corpo[strings.Index(corpo, "func (m *Modulo) lancar("):]
	corpo = corpo[:strings.Index(corpo, "\nfunc ")]

	trava := strings.Index(corpo, "custosIguais(")
	teto := strings.Index(corpo, "regras.AplicarTeto(")
	if trava < 0 || teto < 0 {
		t.Fatalf("não achei as duas conferências (trava=%d teto=%d)", trava, teto)
	}
	if teto < trava {
		t.Error("o teto responde antes da trava — uma duplicata apareceria como " +
			"bloqueio de teto, oferecendo aprovar o que precisa ser apagado")
	}
}

// TODA FLAG DE BLOQUEIO TEM QUE CABER NO `check` DO BANCO
//
//	A coluna `lancamento_bloqueio` é FECHADA por um `check`. Uma flag nova no Go
//	sem a migração junto não dá erro visível: `bloquear` engole o próprio erro de
//	propósito — para não trocar a mensagem do Trílogo por uma mensagem do banco —
//	e a marca simplesmente não é gravada.
//
//	Foi o que aconteceu em 26/08/2026. A trava barrou o ticket 125199 de verdade,
//	a tela mostrou a frase certa, e o banco recusou a marca em silêncio: o
//	orçamento barrado não entrou em Correções › Recusados, e amanhã ninguém saberá
//	que ele já foi apontado. Uma conferência que barra sem registrar precisa ser
//	refeita toda vez, por alguém que talvez não esteja lá.
//
//	Este teste lê as DUAS fontes — as chamadas no `lancar.go` e a lista do `check`
//	na migração mais recente que a define — e quebra antes de subir se elas se
//	separarem outra vez.
func TestTodaFlagDeBloqueioEhPermitidaNoBanco(t *testing.T) {
	fonte, err := os.ReadFile("lancar.go")
	if err != nil {
		t.Fatalf("não consegui ler o lancar.go: %v", err)
	}
	// Casa pelo `orc, "..."`, que é a posição da flag na chamada — e não pela
	// primeira aspa depois do parêntese, que numa chamada com variável no lugar
	// da flag escorregaria para a primeira aspa do bloco SEGUINTE e acusaria uma
	// flag que ninguém escreveu.
	//
	// Só as flags escritas como texto. A chamada que passa a variável `flag`
	// (a recusa do Trílogo) usa 'desconhecido' e 'trilogo_fora', os dois já na
	// lista desde a 015 — um literal ali seria mentira, não segurança.
	usadas := regexp.MustCompile(`m\.bloquear\([^;]*?orc, "([a-z_]+)"`).FindAllStringSubmatch(string(fonte), -1)
	if len(usadas) == 0 {
		t.Fatal("não achei nenhuma chamada de bloquear — o teste deixou de olhar o que devia")
	}

	permitidas := flagsDoCheck(t)
	for _, m := range usadas {
		if !permitidas[m[1]] {
			t.Errorf("o lancar.go grava o bloqueio %q, e o `check` do banco não aceita esse valor — "+
				"a marca vai falhar em silêncio e o orçamento barrado não aparecerá em Recusados. "+
				"Falta a migração que acrescenta %q à lista.", m[1], m[1])
		}
	}
}

// flagsDoCheck lê a lista do `check` na migração MAIS RECENTE que o define — a
// que está valendo. Ler a 015 para sempre daria o resultado de antes de qualquer
// correção, que é o oposto do que este teste existe para pegar.
func flagsDoCheck(t *testing.T) map[string]bool {
	t.Helper()
	arquivos, err := filepath.Glob("../../../../db/migrations/*.sql")
	if err != nil || len(arquivos) == 0 {
		t.Skipf("não achei as migrações: %v", err)
	}
	sort.Strings(arquivos)

	for i := len(arquivos) - 1; i >= 0; i-- {
		bruto, err := os.ReadFile(arquivos[i])
		if err != nil {
			continue
		}
		texto := string(bruto)
		j := strings.Index(texto, "orcamentos_lancamento_bloqueio_check\n  check")
		if j < 0 {
			if j = strings.Index(texto, "add constraint orcamentos_lancamento_bloqueio_check"); j < 0 {
				continue
			}
		}
		lista := texto[j:]
		if fim := strings.Index(lista, "));"); fim > 0 {
			lista = lista[:fim]
		}
		fora := map[string]bool{}
		for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(lista, -1) {
			fora[m[1]] = true
		}
		if len(fora) > 0 {
			t.Logf("lista do `check` lida de %s: %d valores", filepath.Base(arquivos[i]), len(fora))
			return fora
		}
	}
	t.Fatal("nenhuma migração define orcamentos_lancamento_bloqueio_check")
	return nil
}

// TODA RECUSA DIZ O PRÓPRIO NOME
//
//	A tela escolhia o desenho pelo FORMATO da resposta — "tem `saidas`? é teto".
//	Três recusas têm `saidas`, e a do status do chamado caía na primeira. Num
//	lote de 56, em 26/08/2026, dezessete chamados Arquivados apareceram como
//	"passou do teto: – já no ticket + – deste, teto –": os travessões eram os
//	números do teto, que aquela resposta não traz. A tela mandou a pessoa caçar
//	um teto que nunca tinha estourado.
//
//	Uma recusa nova sem `bloqueio` volta a ser confundida com uma antiga por
//	acidente de formato — e o defeito reaparece calado, como este reapareceu.
func TestTodaRecusaMandaONomeDoBloqueio(t *testing.T) {
	fonte, err := os.ReadFile("lancar.go")
	if err != nil {
		t.Fatalf("não consegui ler o arquivo: %v", err)
	}
	texto := string(fonte)

	// Cada `web.Responder(..., http.StatusConflict, ...)` abre um mapa; o teste
	// olha da abertura até o fecho dele.
	corpos := regexp.MustCompile(`(?s)web\.Responder\(w, http\.StatusConflict, map\[string\]any\{(.*?)\n\t\t\}\)`).
		FindAllStringSubmatch(texto, -1)
	if len(corpos) == 0 {
		t.Fatal("não achei nenhuma resposta de conflito — o teste deixou de olhar o que devia")
	}
	for i, c := range corpos {
		if !strings.Contains(c[1], `"bloqueio"`) {
			t.Errorf("a %dª resposta de conflito não manda \"bloqueio\" — a tela vai ter que "+
				"adivinhar qual recusa é esta pelo formato do corpo, e foi assim que "+
				"dezessete chamados Arquivados viraram \"passou do teto\":\n%s", i+1, c[1])
		}
	}
}

// O LOTE NUNCA CONFIRMA POR NINGUÉM
//
//	"Lançar todos" percorre a fila sozinho. Se ele mandasse a confirmação, a
//	trava deixaria de existir justamente na hora em que ela importa — setenta e
//	dois orçamentos subindo sem uma pessoa olhando nenhum.
func TestOLoteNaoMandaConfirmacao(t *testing.T) {
	fonte, err := os.ReadFile("../../../../web/src/telas/orcamentos/Lancar.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	texto := string(fonte)
	corpo := texto[strings.Index(texto, "async function lancarTodos("):]
	corpo = corpo[:strings.Index(corpo, "\n  }")]
	if strings.Contains(corpo, "duplicata=confirmo") {
		t.Error("o lote manda a confirmação de duplicidade — a trava não valeria no lote")
	}
}
