// rev 1 — a mesma nota, em outro arquivo, não orça o mesmo ticket duas vezes
//
// O CASO REAL, QUE CUSTOU R$ 854,40
//
//	Em 26/08/2026 a NF 17936 entrou duas vezes, com três segundos de diferença:
//	`CCF17082026_0006.pdf`, num scan em que a IA não leu os 44 dígitos da chave
//	de acesso, e `nf frota macedo - 10.08_p3.pdf`, página de um PDF multipágina,
//	com a chave inteira.
//
//	São dois `documento_id`. A trava do ARQUIVO (`orcamento_documentos` único em
//	documento+ticket) não vê nada de errado — e não está errada: são arquivos
//	diferentes mesmo. A trava da NOTA, que roda na leitura, falhou por outro
//	defeito. Sem uma terceira pergunta, o ticket 126342 recebeu R$ 600,00 duas
//	vezes, e o 112449 recebeu R$ 254,40 duas vezes.
//
//	Esta é a pergunta que faltava, e ela é feita na hora de gastar dinheiro — não
//	na entrada, onde depende de a leitura ter acertado.
package orcamentos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// bancoComGemeas responde à busca de documentos com `gemeas` (aplicando o
// `or=(...)`, como o PostgREST faria) e à de orcamento_documentos com `choques`.
//
// O DUBLÊ APLICA O FILTRO DE PROPÓSITO (P-30)
//
//	Um dublê que devolve tudo para qualquer consulta faz o teste passar mesmo com
//	a busca sabotada — foi o que aconteceu no teste irmão, em `interno/leitura`,
//	e só apareceu porque a sabotagem foi feita.
func bancoComGemeas(t *testing.T, gemeas, choques []map[string]any, vistos *[]string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		caminho := r.URL.Path
		q := r.URL.Query()
		*vistos = append(*vistos, caminho+"?"+r.URL.RawQuery)

		switch {
		case strings.HasSuffix(caminho, "/documentos"):
			ou := strings.TrimSuffix(strings.TrimPrefix(q.Get("or"), "("), ")")
			if strings.TrimSpace(ou) == "" {
				t.Error("a busca das gêmeas foi sem `or=(...)` — o PostgREST devolveria " +
					"a base inteira e a trava acusaria notas que nada têm a ver")
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			var saida []map[string]any
			for _, linha := range gemeas {
				for _, cond := range strings.Split(ou, ",") {
					campo, valor, ok := strings.Cut(cond, ".eq.")
					if ok && linha[campo] != nil && fmt.Sprint(linha[campo]) == valor {
						saida = append(saida, linha)
						break
					}
				}
			}
			_ = json.NewEncoder(w).Encode(saida)
		case strings.HasSuffix(caminho, "/orcamento_documentos"):
			_ = json.NewEncoder(w).Encode(choques)
		default:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

func nf17936ComChave() documentoPronto {
	chave := "23260840579455000128550010000179361712764115"
	numero := "17936"
	return documentoPronto{ID: "b-com-chave", Nome: "nf frota macedo - 10.08_p3.pdf",
		Chave: &chave, Numero: &numero, Valor: 500}
}

// O CASO DE 26/08, EXATAMENTE COMO ELE ACONTECEU
func TestAMesmaNotaEmOutroArquivoNaoOrcaOMesmoTicket(t *testing.T) {
	var vistos []string
	m := bancoComGemeas(t,
		// a cópia que já está no banco: mesmo número, SEM chave de acesso
		[]map[string]any{{"id": "a-sem-chave", "nome_arquivo": "CCF17082026_0006.pdf",
			"numero": "17936", "chave_acesso": nil, "valor_total": 500.0}},
		// e ela já orçou o ticket 126342
		[]map[string]any{{"documento_id": "a-sem-chave", "ticket": 126342}},
		&vistos)

	recusa, err := m.mesmaNotaJaOrcou(context.Background(), "c1", nf17936ComChave(), []int{126342})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if recusa == "" {
		t.Fatal("a segunda cópia passou: é o defeito de 26/08, R$ 600,00 duas vezes no 126342")
	}
	for _, precisa := range []string{"CCF17082026_0006.pdf", "126342", "duas vezes"} {
		if !strings.Contains(recusa, precisa) {
			t.Errorf("a recusa não diz %q — quem lê precisa saber qual é a outra "+
				"e o que fazer: %q", precisa, recusa)
		}
	}
}

// NOTA DIFERENTE NO MESMO TICKET CONTINUA PASSANDO
//
//	Um ticket pode ter várias notas legítimas — foi por isso que a regra de "um
//	custo por ticket" foi abandonada em agosto. Se esta trava barrasse a segunda
//	nota de um mesmo ticket, ela quebraria o caso normal para consertar o raro.
func TestOutraNotaNoMesmoTicketContinuaPassando(t *testing.T) {
	var vistos []string
	m := bancoComGemeas(t,
		// existe uma nota no banco, mas com OUTRO número e OUTRA chave
		[]map[string]any{{"id": "outra", "nome_arquivo": "NF 11111.pdf",
			"numero": "11111", "chave_acesso": "9999", "valor_total": 500.0}},
		[]map[string]any{{"documento_id": "outra", "ticket": 126342}},
		&vistos)

	recusa, err := m.mesmaNotaJaOrcou(context.Background(), "c1", nf17936ComChave(), []int{126342})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if recusa != "" {
		t.Fatalf("barrou uma nota DIFERENTE no mesmo ticket — um ticket pode ter "+
			"várias notas legítimas: %q", recusa)
	}
}

// A MESMA NOTA EM OUTRO TICKET PASSA
//
//	É o rateio: uma nota atende dois chamados, e cada um recebe a sua parte. A
//	trava é por (nota, TICKET), não por nota.
func TestAMesmaNotaEmOutroTicketPassa(t *testing.T) {
	var vistos []string
	m := bancoComGemeas(t,
		[]map[string]any{{"id": "a-sem-chave", "nome_arquivo": "CCF17082026_0006.pdf",
			"numero": "17936", "chave_acesso": nil, "valor_total": 500.0}},
		// o choque seria em outro ticket, e o filtro do motor não o traria
		[]map[string]any{},
		&vistos)

	recusa, err := m.mesmaNotaJaOrcou(context.Background(), "c1", nf17936ComChave(), []int{999999})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if recusa != "" {
		t.Fatalf("barrou a mesma nota em OUTRO ticket — isso é rateio, e é legítimo: %q", recusa)
	}
	// e a consulta tem que ter mirado o ticket certo
	juntos := strings.Join(vistos, " ")
	if !strings.Contains(juntos, "ticket=in.(999999)") {
		t.Errorf("a busca dos choques não filtrou pelos tickets desta nota: %v", vistos)
	}
}

// NOTA SEM IDENTIDADE NÃO ACUSA NINGUÉM
//
//	Sem chave e sem número, inventar duplicidade a partir de valor igual acusaria
//	duas notas de R$ 14,90 que não têm nada a ver uma com a outra.
func TestNotaSemChaveESemNumeroNaoBarraNada(t *testing.T) {
	var vistos []string
	m := bancoComGemeas(t, nil, nil, &vistos)

	recusa, err := m.mesmaNotaJaOrcou(context.Background(), "c1",
		documentoPronto{ID: "x", Nome: "sem-identidade.pdf", Valor: 14.90}, []int{126342})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if recusa != "" {
		t.Errorf("acusou uma nota sem número e sem chave: %q", recusa)
	}
	if len(vistos) != 0 {
		t.Errorf("foi ao banco sem ter o que perguntar: %v", vistos)
	}
}

// A BARREIRA TEM QUE ESTAR NO CAMINHO, E ANTES DE GASTAR
//
//	Uma função de trava que ninguém chama é a mesma coisa que trava nenhuma — e
//	já aconteceu nesta obra: `regras.MesmaNota` existiu meses, com teste, sem ser
//	chamada de lugar nenhum. O teste olha a chamada, e olha a POSIÇÃO dela: antes
//	de `itensDo`, que é onde o orçamento começa a tomar forma.
func TestAGeracaoConsultaASegundaBarreiraAntesDeGastar(t *testing.T) {
	fonte, err := os.ReadFile("gerar.go")
	if err != nil {
		t.Fatalf("não consegui ler o gerar.go: %v", err)
	}
	texto := string(fonte)

	chamada := "m.mesmaNotaJaOrcou(ctx, p.ClienteID, d, numerosDe(tickets))"
	i := strings.Index(texto, chamada)
	if i < 0 {
		t.Fatal("a geração não pergunta se a MESMA nota já orçou este ticket — a " +
			"trava existe e ninguém a chama, que é o mesmo que não existir. " +
			"Foi assim que o ticket 126342 recebeu R$ 600,00 duas vezes")
	}
	j := strings.Index(texto, "lidos, err := m.itensDo(ctx, d.ID)")
	if j < 0 {
		t.Fatal("não achei o começo da montagem do orçamento em gerar.go")
	}
	if i > j {
		t.Error("a pergunta vem DEPOIS de montar os itens — a trava tem que barrar " +
			"antes de o orçamento tomar forma, não depois")
	}
}
