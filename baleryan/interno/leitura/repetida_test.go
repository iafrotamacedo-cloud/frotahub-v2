// rev 1 — a nota que já estava aqui
//
// O DEFEITO QUE ESTE ARQUIVO GUARDA
//
//	`regras.MesmaNota` existia desde o começo do sistema, com teste, e NUNCA foi
//	chamada de lugar nenhum. O que protegia era só o sha256 do arquivo — que
//	cobre subir o mesmo arquivo duas vezes, e não cobre a mesma NOTA chegando
//	como arquivo diferente: a foto e o PDF dela, dois escaneamentos, o mesmo PDF
//	renomeado. Bytes diferentes, dois documentos, dois orçamentos, e a loja
//	pagando o mesmo material duas vezes.
package leitura

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
)

// marcada é o que o PostgREST de mentira anotou: quem foi marcado como cópia,
// e de quem.
type marcada struct {
	quem string
	de   string
}

// bancoDeMentira responde às buscas com `achadas` e guarda os PATCH.
//
// O DUBLÊ APLICA O FILTRO — E ISSO NÃO É CAPRICHO (P-30)
//
//	A primeira versão devolvia `achadas` inteiro para QUALQUER busca. Com isso,
//	o teste do caso real de 26/08 passava mesmo com o defeito de volta no código:
//	a consulta procurava só por chave, a candidata sem chave não deveria
//	aparecer — e aparecia, porque o dublê não sabia filtrar. Sabotei e o teste
//	não reclamou.
//
//	Um dublê mais permissivo que o original transforma o teste em enfeite: ele
//	prova que o código funciona num banco que não existe.
func bancoDeMentira(t *testing.T, achadas []map[string]any, anotar *[]marcada) *Servico {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(filtrarComo(t, r.URL.Query().Get("or"), achadas))
		case http.MethodPatch:
			var campos map[string]any
			_ = json.NewDecoder(r.Body).Decode(&campos)
			de, _ := campos["duplicada_de"].(string)
			id := strings.TrimPrefix(r.URL.Query().Get("id"), "eq.")
			*anotar = append(*anotar, marcada{quem: id, de: de})
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("a trava usou %s, e ela só lê e marca", r.Method)
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	t.Cleanup(s.Close)
	return &Servico{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// filtrarComo faz o que o PostgREST faria com `or=(campo.eq.valor,...)`:
// devolve só as linhas que casam com PELO MENOS UMA das condições.
//
// Sem `or` na busca, nada casa — que é o comportamento honesto: uma consulta que
// não filtra nada é uma consulta que o PostgREST recusaria, não uma que devolve
// tudo.
func filtrarComo(t *testing.T, ou string, linhas []map[string]any) []map[string]any {
	t.Helper()
	ou = strings.TrimSuffix(strings.TrimPrefix(ou, "("), ")")
	if strings.TrimSpace(ou) == "" {
		t.Errorf("a busca das candidatas foi sem `or=(...)` — o PostgREST devolveria " +
			"a base inteira, e a trava passaria a acusar notas que nada têm a ver")
		return nil
	}

	var saida []map[string]any
	for _, linha := range linhas {
		for _, cond := range strings.Split(ou, ",") {
			campo, valor, ok := strings.Cut(cond, ".eq.")
			if !ok {
				t.Fatalf("condição que o dublê não entende: %q", cond)
			}
			// `url.QueryEscape` do lado de cá; o valor chega decodificado.
			if fmt.Sprint(linha[campo]) == valor && linha[campo] != nil {
				saida = append(saida, linha)
				break
			}
		}
	}
	return saida
}

const (
	chaveA = "23260714788633000110550010000091601000055288"
	cedo   = "2026-08-26T10:00:00Z"
	tarde  = "2026-08-26T15:00:00Z"
)

// A QUE CHEGOU DEPOIS É A CÓPIA
func TestANotaQueChegouDepoisEAMarcada(t *testing.T) {
	var anotadas []marcada
	s := bancoDeMentira(t, []map[string]any{
		{"id": "a-original", "nome_arquivo": "NF.pdf", "chave_acesso": chaveA,
			"valor_total": 100.0, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "b-copia", Nome: "NF (1).pdf", Cliente: "c1", Inserido: tarde}
	if err := s.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ChaveAcesso: chaveA, ValorTotal: 100}); err != nil {
		t.Fatalf("falhou: %v", err)
	}

	if len(anotadas) != 1 {
		t.Fatalf("marcou %d notas, esperava 1", len(anotadas))
	}
	if anotadas[0].quem != "b-copia" || anotadas[0].de != "a-original" {
		t.Errorf("marcou %q como cópia de %q — a cópia é quem chegou depois",
			anotadas[0].quem, anotadas[0].de)
	}
}

// A ORDEM DE LEITURA NÃO É A ORDEM DE CHEGADA
//
//	Se as duas entram na mesma rodada e a segunda é lida primeiro, ela não acha
//	a primeira — que ainda não tem chave. Depois a primeira é lida e acha a
//	segunda. Se a regra fosse "marco a que estou lendo", a ORIGINAL viraria
//	cópia da cópia.
func TestLendoAOriginalDepois_AMarcaVaiNaOutra(t *testing.T) {
	var anotadas []marcada
	s := bancoDeMentira(t, []map[string]any{
		{"id": "b-copia", "nome_arquivo": "NF (1).pdf", "chave_acesso": chaveA,
			"valor_total": 100.0, "inserido_em": tarde},
	}, &anotadas)

	doc := &documento{ID: "a-original", Nome: "NF.pdf", Cliente: "c1", Inserido: cedo}
	if err := s.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ChaveAcesso: chaveA, ValorTotal: 100}); err != nil {
		t.Fatalf("falhou: %v", err)
	}

	if len(anotadas) != 1 {
		t.Fatalf("marcou %d notas, esperava 1", len(anotadas))
	}
	if anotadas[0].quem != "b-copia" {
		t.Errorf("marcou %q, e quem chegou depois foi b-copia — a original virou cópia da cópia",
			anotadas[0].quem)
	}
}

// SEM IDENTIDADE, NÃO SE ACUSA NINGUÉM
//
//	Duas notas de R$ 14,90 sem chave e sem número não são a mesma nota. Marcar
//	por valor igual criaria duplicidade onde não há, e o usuário perderia
//	confiança na marca — que é pior que não ter marca.
func TestNotaSemChaveESemNumeroNaoAcusa(t *testing.T) {
	var anotadas []marcada
	s := bancoDeMentira(t, []map[string]any{
		{"id": "outra", "nome_arquivo": "outra.jpg", "valor_total": 14.90, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "esta", Nome: "esta.jpg", Cliente: "c1", Inserido: tarde}
	if err := s.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ValorTotal: 14.90}); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if len(anotadas) != 0 {
		t.Errorf("acusou duplicidade sem chave e sem número: %+v", anotadas)
	}
}

// Notas diferentes com números parecidos não podem virar cópia uma da outra.
func TestNotasDiferentesNaoSaoMarcadas(t *testing.T) {
	var anotadas []marcada
	s := bancoDeMentira(t, []map[string]any{
		{"id": "outra", "nome_arquivo": "outra.pdf",
			"chave_acesso": "23260714788633000110550010000091601000055287",
			"valor_total":  100.0, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "esta", Nome: "esta.pdf", Cliente: "c1", Inserido: tarde}
	if err := s.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ChaveAcesso: chaveA, ValorTotal: 100}); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if len(anotadas) != 0 {
		t.Errorf("duas chaves diferentes viraram a mesma nota: %+v", anotadas)
	}
}

// O DAV NÃO TEM CHAVE, E MESMO ASSIM TEM IDENTIDADE
func TestDAVRepetidoEPegoPeloNumeroEValor(t *testing.T) {
	var anotadas []marcada
	numero := "19037"
	s := bancoDeMentira(t, []map[string]any{
		{"id": "a-original", "nome_arquivo": "dav.jpg", "numero": numero,
			"valor_total": 14.90, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "b-copia", Nome: "dav-foto.jpeg", Cliente: "c1", Inserido: tarde}
	if err := s.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{Numero: numero, ValorTotal: 14.90}); err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if len(anotadas) != 1 || anotadas[0].quem != "b-copia" {
		t.Errorf("o mesmo DAV em dois arquivos não foi pego: %+v", anotadas)
	}
}

// A CÓPIA SEM CHAVE E A ORIGINAL COM CHAVE SÃO A MESMA NOTA
//
// O CASO REAL, E ELE CUSTOU R$ 854,40
//
//	Em 26/08/2026 a NF 17936 entrou duas vezes, com TRÊS SEGUNDOS de diferença:
//	`CCF17082026_0006.pdf`, cujo scan não deixou a IA ler os 44 dígitos da chave,
//	e `nf frota macedo - 10.08_p3.pdf`, página de um PDF multipágina, com a chave
//	inteira. Mesmo número, mesmo valor, a mesma nota.
//
//	A busca das candidatas era exclusiva: quem tinha chave procurava SÓ por
//	chave. A cópia sem chave nunca entrava na lista, e `MesmaNota` — que teria
//	dito "é a mesma" pelo número — nunca era chamada para aquele par.
//
//	As duas viraram orçamento de R$ 600,00 no MESMO ticket 126342. E a NF 17937
//	repetiu no 112449, com R$ 254,40 cada.
//
//	O teste sabota o defeito ao contrário: a nota lida TEM chave, a que já estava
//	no banco NÃO tem. Com a busca antiga, `achadas` viria vazia e nada seria
//	marcado.
func TestNotaComChaveAchaACopiaSemChave(t *testing.T) {
	var anotadas []marcada
	s := bancoDeMentira(t, []map[string]any{
		// A que já estava aqui: mesmo número, mesmo valor, SEM chave de acesso.
		{"id": "a-sem-chave", "nome_arquivo": "CCF17082026_0006.pdf", "chave_acesso": nil,
			"numero": "17936", "valor_total": 500.0, "inserido_em": cedo},
	}, &anotadas)

	doc := &documento{ID: "b-com-chave", Nome: "nf frota macedo - 10.08_p3.pdf",
		Cliente: "c1", Inserido: tarde}

	if err := s.conferirRepetida(context.Background(), doc,
		&leitor.Leitura{ChaveAcesso: chaveA, Numero: "17936", ValorTotal: 500}); err != nil {
		t.Fatalf("erro: %v", err)
	}

	if len(anotadas) != 1 {
		t.Fatalf("a cópia sem chave passou em branco — é o defeito de 26/08, que "+
			"pôs a mesma nota duas vezes no ticket 126342. marcadas: %v", anotadas)
	}
	if anotadas[0].quem != "b-com-chave" {
		t.Errorf("marcou %q; quem chegou depois foi a b-com-chave", anotadas[0].quem)
	}
	if anotadas[0].de != "a-sem-chave" {
		t.Errorf("apontou para %q; a original é a a-sem-chave", anotadas[0].de)
	}
}

// A BUSCA PESCA LARGO — CHAVE **OU** NÚMERO
//
//	Se a consulta voltar a decidir sozinha, `MesmaNota` deixa de receber os pares
//	que precisa julgar, e a regra vira enfeite. É o defeito acima, na origem.
func TestABuscaDeCandidatasNaoDecideSozinha(t *testing.T) {
	f := ouEntao(chaveA, "17936")
	if !strings.Contains(f, "chave_acesso.eq.") || !strings.Contains(f, "numero.eq.") {
		t.Errorf("com chave E número, a busca tem que procurar pelos DOIS: %q", f)
	}
	if !strings.HasPrefix(f, "or=(") {
		t.Errorf("a busca deixou de ser um OU: %q", f)
	}
	// E cada um sozinho continua funcionando.
	if soChave := ouEntao(chaveA, ""); strings.Contains(soChave, "numero.eq.") {
		t.Errorf("sem número, não há o que procurar por número: %q", soChave)
	}
	if soNumero := ouEntao("", "17936"); strings.Contains(soNumero, "chave_acesso.eq.") {
		t.Errorf("sem chave, não há o que procurar por chave: %q", soNumero)
	}
}
