// rev 1 — nada sai da fila antes de gerar
//
// A REGRA, DITA PELO DONO EM 27/08/2026 DEPOIS DE EU ERRAR TRÊS VEZES
//
//	"nao pode sair nada da fila quando inseri ou depois que le....nada"
//
//	Ele inseriu 9 notas, mandou ler, e a lista caiu para 8. Nenhuma sumiu do
//	sistema: a DAV 19425 não tem ticket escrito na observação, e a fila filtrava
//	por `onde=eq.fila`, que manda a nota sem ticket para Correções.
//
//	Do lado de dentro é uma classificação correta. Do lado de fora é uma nota que
//	evaporou depois de uma ação que deveria apenas LER — e uma fila que muda de
//	tamanho sozinha ensina o usuário a desconfiar dela.
package orcamentos

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// A FILA MOSTRA TUDO O QUE NÃO VIROU ORÇAMENTO
func TestAFilaNaoFiltraPorOnde(t *testing.T) {
	f := filtroDosDocumentos("c1", "orcamento", url.Values{})

	if strings.Contains(f, "onde=eq.fila") {
		t.Fatal("a fila voltou a filtrar por `onde` — a nota sem ticket e a não " +
			"associada somem da lista depois de lidas, que é o defeito de 27/08")
	}
	if !strings.Contains(f, "na_fila=is.true") {
		t.Error("a fila não pergunta `na_fila` — a regra dos três momentos " +
			"(inserir, ler, gerar) deixou de valer")
	}
	if !strings.Contains(f, "oculto_em=is.null") {
		t.Error("a fila passou a mostrar o que foi tirado dela")
	}
}

// OS TRÊS MOMENTOS, ESCRITOS NA REGRA DO BANCO
//
//	A regra do dono tem três momentos, e a diferença entre eles não é o ESTADO
//	da nota — é se ela já foi TENTADA:
//
//	  insere  -> tudo na fila
//	  lê      -> tudo continua na fila
//	  gera    -> vira orçamento, vai para Correções, ou FICA (duplicidade e
//	             falta de confiança)
//
//	Sem `geracao_tentada_em` na condição, a coluna `na_fila` só sabe olhar o
//	estado — e aí ou a nota sem ticket some ao ser lida (o defeito de manhã), ou
//	fica presa para sempre (o defeito da tarde). Este teste guarda as duas
//	pontas ao mesmo tempo.
func TestARegraDaFilaSeparaOsTresMomentos(t *testing.T) {
	sql, err := os.ReadFile("../../../../db/migrations/038_a_fila_esvazia_quando_voce_gera.sql")
	if err != nil {
		t.Fatalf("não achei a 038: %v", err)
	}
	regra := semComentarios(string(sql))

	i := strings.Index(regra, ") AS na_fila")
	if i < 0 {
		t.Fatal("a 038 não define a coluna `na_fila`")
	}
	// A condição inteira: do último `(d.oculto_em` até o `) AS na_fila`.
	j := strings.LastIndex(regra[:i], "(d.oculto_em IS NULL")
	if j < 0 {
		t.Fatal("não achei a condição de `na_fila`")
	}
	cond := regra[j:i]

	for _, exigido := range []struct{ trecho, porque string }{
		{"d.status <> 'usado'", "a nota que virou orçamento continuaria na fila — arquivo morto"},
		{"d.geracao_tentada_em IS NULL", "sem isto a nota sem ticket some ao ser LIDA, que foi o defeito da manhã"},
		{"'sem-ticket'", "a nota sem ticket não iria para Correções depois de tentada"},
		{"'sem-associacao'", "a nota com ticket fora da base não iria para Correções depois de tentada"},
	} {
		if !strings.Contains(cond, exigido.trecho) {
			t.Errorf("`na_fila` perdeu %s: %s\ncondição: %s", exigido.trecho, exigido.porque, cond)
		}
	}

	// E o contrário, que é o pedido literal do dono: duplicidade e falta de
	// confiança FICAM. Se alguém as puser na lista de saída, elas somem da
	// única tela onde a decisão é tomada.
	for _, proibido := range []string{"'repetida'", "'a-conferir'"} {
		if strings.Contains(cond, proibido) {
			t.Errorf("`na_fila` manda %s embora da fila — o dono pediu o oposto: "+
				"duplicidade e falta de confiança PERMANECEM", proibido)
		}
	}
}

// A TENTATIVA É CARIMBADA EM TODAS, INCLUSIVE NAS QUE FALHARAM
//
//	Marcar só as que deram certo seria não marcar nada de útil: essas já saem
//	por `status = 'usado'`. Quem precisa do carimbo é exatamente quem falhou —
//	é ele que manda a nota para Correções em vez de deixá-la na fila para
//	sempre.
func TestGerarCarimbaATentativaEmTodasAsNotasDoLote(t *testing.T) {
	fonte, err := os.ReadFile("gerar.go")
	if err != nil {
		t.Fatalf("não consegui ler o gerar.go: %v", err)
	}
	texto := string(fonte)
	if !strings.Contains(texto, "m.marcarTentativa(r.Context(), docs)") {
		t.Fatal("`gerar` não carimba a tentativa — a fila nunca esvazia, e as " +
			"notas que não passaram ficam ali para sempre")
	}
	// `docs` é o lote inteiro; `saida` é o resultado. Carimbar a saída marcaria
	// só quem gerou.
	if strings.Contains(texto, "m.marcarTentativa(r.Context(), saida)") {
		t.Error("o carimbo foi para o resultado em vez do lote — quem falhou " +
			"ficaria sem marca, que é justamente quem precisa dela")
	}
}

// AS OUTRAS DUAS VISTAS NÃO MUDAM
//
//	"Já viraram orçamento" e "Fora da fila" são justamente as duas listas cujo
//	critério É a saída. Se a mesma constante vazasse para elas, as três vistas
//	mostrariam a mesma coisa.
func TestAsOutrasVistasContinuamSendoOQueEram(t *testing.T) {
	processadas := filtroDosDocumentos("c1", "orcamento", url.Values{"vista": {"processadas"}})
	if !strings.Contains(processadas, "onde=eq.usada") {
		t.Error("a vista `processadas` deixou de mostrar o que virou orçamento")
	}
	fora := filtroDosDocumentos("c1", "orcamento", url.Values{"vista": {"fora"}})
	if !strings.Contains(fora, "oculto_em=not.is.null") {
		t.Error("a vista `fora` deixou de mostrar o que foi tirado da fila")
	}
}

// O PAINEL CONTA A MESMA FILA QUE A LISTA MOSTRA
//
//	Era este o defeito irmão: a lista passaria a mostrar 12 e o cartão do painel
//	continuaria dizendo 8, porque `orcamentos_painel.notas_arquivos` tinha a sua
//	própria régua (`onde = 'fila'`). Duas réguas para a mesma fila é como a tela
//	e o botão discordam — e já discordaram antes, em 93 contra 5.

func TestOPainelContaAMesmaFilaQueAListaMostra(t *testing.T) {
	// A PRÉVIA SAI DA MESMA CONSTANTE QUE A LISTA
	//
	//	A primeira versão contava quantas vezes "NaFila" aparecia no arquivo — e
	//	passava com a prévia sabotada, porque o comentário que explica a
	//	constante já bastava para o mínimo. Contar menção não é conferir uso.
	//	Agora o teste olha a LINHA da prévia.
	fonte, err := os.ReadFile("documentos.go")
	if err != nil {
		t.Fatalf("não consegui ler o documentos.go: %v", err)
	}
	if !strings.Contains(string(fonte), `"&fila=eq.orcamento"+NaFila+`) {
		t.Error("a prévia do painel ganhou régua própria — o cartão vai dizer um " +
			"número e a lista vai mostrar outro. Já discordaram antes, em 93 contra 5")
	}

	// E a migração 037 tem que ter tirado o `onde` do contador do banco.
	sql, err := os.ReadFile("../../../../db/migrations/037_a_fila_nao_perde_nota.sql")
	if err != nil {
		t.Fatalf("não achei a 037: %v", err)
	}
	// O COMENTÁRIO NÃO É CÓDIGO
	//   A 037 CITA a régua antiga para explicar o que mudou. Um teste que casa
	//   texto cru acusaria a própria explicação — e o conserto seria apagar o
	//   comentário, que é o oposto do que se quer. Ficam só as linhas de SQL.
	texto := semComentarios(string(sql))
	if strings.Contains(texto, "d.onde = 'fila'") {
		t.Error("a 037 deixou `notas_arquivos` contando por `onde` — o cartão do " +
			"painel vai dizer um número e a lista vai mostrar outro")
	}
	// P-35: reescrever a view apaga as reloptions.
	if !strings.Contains(texto, "security_invoker = true") {
		t.Error("a 037 reescreve `orcamentos_painel` sem repetir o " +
			"`with (security_invoker = true)` — a tranca cai em silêncio (P-35)")
	}
}

// A NOTA QUE FICA TEM QUE TER O QUE FAZER
//
//	Metade da regra é a nota não sumir. A outra metade é ela ser resolvível ali:
//	uma linha que fica na lista sem nada para fazer nela some da vista do mesmo
//	jeito, e ainda ocupa espaço.
func TestAFilaOfereceConsertoParaQuemPrecisa(t *testing.T) {
	tela, err := os.ReadFile("../../../../web/src/telas/orcamentos/Arquivos.tsx")
	if err != nil {
		t.Skipf("não achei a tela: %v", err)
	}
	// A EXPRESSÃO INTEIRA, E NÃO O NOME DA FUNÇÃO
	//
	//	`precisaDeGente(d)` também aparece no contador do cabeçalho. Procurar só
	//	o nome fazia o teste passar com o botão sabotado — foi o que a sabotagem
	//	mostrou. O que importa é a decisão do botão.
	const decisao = "(d.motivo_conferencia || (fila === 'orcamento' && precisaDeGente(d)))"
	if !strings.Contains(string(tela), decisao) {
		t.Error("a fila voltou a decidir o botão só por `motivo_conferencia` — a " +
			"nota sem ticket fica na lista mostrando apenas `ver`, sem como consertar")
	}
}

// semComentarios tira as linhas iniciadas por `--`, para o teste ler o que o
// Postgres vai executar e não o que a migração explica.
func semComentarios(sql string) string {
	var b strings.Builder
	for _, linha := range strings.Split(sql, "\n") {
		if strings.HasPrefix(strings.TrimSpace(linha), "--") {
			continue
		}
		b.WriteString(linha)
		b.WriteByte('\n')
	}
	return b.String()
}
