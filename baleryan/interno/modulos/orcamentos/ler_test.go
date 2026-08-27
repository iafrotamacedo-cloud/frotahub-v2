// rev 1 — a leitura na hora
//
// O QUE ESTE ARQUIVO GUARDA
//
//	Duas garantias, e as duas são sobre LER UMA VEZ SÓ:
//	  · a nota já lida (ou já usada) não volta para a leitura;
//	  · o trabalho da fila é TOMADO, não só olhado — se o robô do GitHub já está
//	    com ele na mão, o botão recusa em vez de ler por cima.
package orcamentos

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitura"
)

// NOTA LIDA NÃO SE LÊ DE NOVO
//
//	Não é economia de cota: é não apagar o que alguém corrigiu à mão em cima da
//	leitura anterior. E `usado` é pior ainda — o orçamento já existe, e mexer
//	nos itens por baixo faria a conta dele discordar da nota.
func TestNotaJaLidaNaoVolta(t *testing.T) {
	for _, caso := range []struct {
		status string
		pode   bool
	}{
		{"inserido", true},
		{"falhou", true}, // a leitura NÃO aconteceu; tentar de novo é o caminho
		{"lido", false},
		{"usado", false},
		{"", false}, // status que ninguém previu não vira permissão
	} {
		recusa, pode := podeLerAgora(caso.status)
		if pode != caso.pode {
			t.Errorf("status %q: pode=%v, esperava %v", caso.status, pode, caso.pode)
		}
		if !pode && recusa == "" {
			t.Errorf("status %q recusou sem dizer por quê", caso.status)
		}
		if pode && recusa != "" {
			t.Errorf("status %q liberou e ainda assim reclamou: %q", caso.status, recusa)
		}
	}
}

// filaDeMentira responde ao PATCH condicional com `tomados`, e ao GET de
// trabalho `rodando` com `rodando`. Guarda os filtros para a conferência.
func filaDeMentira(t *testing.T, tomados, rodando []map[string]any, filtros *[]string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		*filtros = append(*filtros, r.Method+" "+r.URL.RawQuery)
		switch r.Method {
		case http.MethodPatch:
			_ = json.NewEncoder(w).Encode(tomados)
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(rodando)
		default:
			t.Errorf("tomar trabalho usou %s", r.Method)
		}
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// A DISPUTA É RESOLVIDA PELO BANCO, NÃO POR UMA OLHADA ANTES
//
//	Ler o trabalho, ver que está `na_fila` e depois marcá-lo é uma corrida: o
//	robô pode marcar entre as duas coisas. Por isso `status=eq.na_fila` vai no
//	FILTRO do PATCH — quem não receber linha de volta perdeu, e sabe disso.
func TestTomarALeituraExigeQueOTrabalhoEstejaNaFila(t *testing.T) {
	var filtros []string
	m := filaDeMentira(t, []map[string]any{{"id": "j1"}}, nil, &filtros)

	id, livre := m.tomarALeitura(context.Background(), "d1")
	if !livre || id != "j1" {
		t.Fatalf("tomou %q livre=%v, esperava j1/true", id, livre)
	}
	if len(filtros) == 0 || !strings.Contains(filtros[0], "status=eq.na_fila") {
		t.Fatalf("o PATCH não exigiu na_fila no filtro: %v", filtros)
	}
	if !strings.Contains(filtros[0], "alvo_id=eq.d1") || !strings.Contains(filtros[0], "tipo=eq.ler_documento") {
		t.Errorf("o PATCH não foi mirado neste trabalho: %v", filtros)
	}
}

// SE O ROBÔ ESTÁ COM O TRABALHO, O BOTÃO RECUSA
//
//	Ler por cima seria a segunda chamada de IA paga na mesma nota, e a última a
//	gravar decidiria o que a nota diz.
func TestTrabalhoNaMaoDeOutroRecusaALeitura(t *testing.T) {
	var filtros []string
	m := filaDeMentira(t, []map[string]any{}, []map[string]any{{"id": "j1"}}, &filtros)

	if _, livre := m.tomarALeitura(context.Background(), "d1"); livre {
		t.Fatal("o trabalho estava rodando em outra máquina e a leitura foi liberada")
	}
}

// NOTA SEM TRABALHO NENHUM AINDA PODE SER LIDA
//
//	Nota inserida antes de a fila existir, ou com o trabalho fechado por uma
//	corrida que não gravou o status, não tem trabalho para tomar. Recusar aqui
//	deixaria a nota presa para sempre — sem robô que a leia e sem botão que a
//	destranque.
func TestNotaSemTrabalhoNaFilaAindaPodeSerLida(t *testing.T) {
	var filtros []string
	m := filaDeMentira(t, []map[string]any{}, []map[string]any{}, &filtros)

	id, livre := m.tomarALeitura(context.Background(), "d1")
	if !livre {
		t.Fatal("nota sem trabalho na fila ficou presa")
	}
	if id != "" {
		t.Errorf("inventou o trabalho %q para fechar depois", id)
	}
}

// A ROTA LITERAL NÃO PODE SER COMIDA PELO CURINGA
//
//	`/orcamentos/documentos/{id}` e `/orcamentos/documentos/porler` disputam o
//	mesmo lugar no caminho. O ServeMux do Go dá preferência ao literal, e é por
//	isso que as duas convivem — mas essa é uma regra que uma renomeação futura
//	pode quebrar em silêncio. O preço seria o botão "Ler notas" respondendo
//	"Endereço inválido" (a nota `porler` não é um UUID) sem ninguém entender por
//	quê.
func TestPorlerNaoCaiNoCuringaDoId(t *testing.T) {
	mux := http.NewServeMux()
	(&Modulo{}).Montar(mux)

	_, padrao := mux.Handler(httptest.NewRequest(http.MethodGet, "/orcamentos/documentos/porler", nil))
	if !strings.Contains(padrao, "porler") {
		t.Fatalf("quem atendeu /documentos/porler foi %q", padrao)
	}
}

// A FILA TEM QUE VIAJAR NO ENDEREÇO
//
// O DEFEITO QUE ESTE TESTE GUARDA, E QUE CHEGOU À TELA
//
//	Na primeira subida, 27/08/2026, o botão "Ler notas" da fila de rateio deu
//	"0 notas lidas · 3 não deu para ler — Esta nota não é desta fila." A lista
//	do que ler vinha certa (`porler?fila=rateio`), mas a leitura de cada nota
//	ia sem o parâmetro. Sem ele o motor assume `orcamento`, exige a permissão
//	de "Notas e DAVs" e, logo depois, recusa a nota por ser de outra fila.
//
//	O guarda funcionou — foi ele que recusou, em vez de deixar alguém ler uma
//	nota de rateio com a chave da porta ao lado. Faltou o chamador dizer de onde
//	veio, e é isso que este teste passa a cobrar.
//
// POR QUE É BUSCA POR TEXTO
//
//	Entender TypeScript daqui seria construir meio compilador para responder uma
//	pergunta de uma linha (é o mesmo motivo do `lerOFront`). O que importa é o
//	endereço montado, e ele está escrito ali, literal.
func TestOFrontDizDeQueFilaAsRotasDeLeituraSao(t *testing.T) {
	front := lerOFront(t)

	// Todo endereço de leitura montado no front, com o que vier depois do `/ler`
	// ou do `/porler` até fechar a crase.
	// Todo endereço de leitura montado no front, do começo até fechar a aspa (ou
	// a crase) que o delimita.
	enderecos := regexp.MustCompile(
		"/orcamentos/documentos/[^'\"`\n]*?(?:porler|/ler)[^'\"`\n]*").
		FindAllString(front, -1)
	if len(enderecos) < 2 {
		t.Fatalf("achei %d chamada(s) às rotas de leitura no front — as duas telas "+
			"deveriam chamar as duas rotas; o teste deixou de olhar o que devia: %v",
			len(enderecos), enderecos)
	}
	for _, e := range enderecos {
		if !strings.Contains(e, "fila=") {
			t.Errorf("a chamada %q vai sem a fila. O motor assume `orcamento`, "+
				"exige a permissão errada e depois recusa a nota — foi o "+
				"\"0 lidas · 3 não deu para ler\" da fila de rateio.", e)
		}
	}
}

// bancoQueAnota guarda cada escrita, para provar o que NÃO foi escrito.
func bancoQueAnota(t *testing.T, escritas *[]string) *Modulo {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		corpo := make([]byte, 512)
		n, _ := r.Body.Read(corpo)
		*escritas = append(*escritas, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(corpo[:n]))
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(s.Close)
	return &Modulo{bd: banco.Novo(&config.Config{
		Supabase: config.Supabase{URL: s.URL, ChaveServico: "chave-de-mentira"},
	})}
}

// A NOTA NÃO LEVA A CULPA DO SERVIDOR
//
// O QUE ACONTECEU NA TELA, EM 27/08/2026
//
//	O modelo de IA configurado no motor estava aposentado. As três notas da fila
//	de rateio voltaram marcadas `falhou`, em vermelho, com um recado sobre
//	variável de ambiente escrito por cima do nome do arquivo. Nenhuma tinha
//	problema: o errado era uma linha de configuração.
//
//	`FalhaTemporaria` quer dizer "a nota está boa, tente de novo". Quem condena a
//	nota depois de três tentativas é o robô, em lote, onde ninguém está olhando.
//	Aqui o freio é a pessoa.
func TestFalhaDoServidorNaoMarcaANota(t *testing.T) {
	var escritas []string
	m := bancoQueAnota(t, &escritas)

	m.leituraFalhou(context.Background(), "d1", "j1",
		leitura.FalhaTemporaria{Causa: errors.New("This model models/gemini-2.5-flash is no longer available")})

	for _, e := range escritas {
		if strings.Contains(e, "/documentos") {
			t.Fatalf("a nota foi marcada por uma falha do servidor: %s", e)
		}
	}
	if len(escritas) == 0 || !strings.Contains(escritas[0], "/jobs") {
		t.Fatalf("o trabalho não voltou para a fila: %v", escritas)
	}
	if !strings.Contains(escritas[0], "na_fila") {
		t.Errorf("o trabalho voltou, mas não para a fila: %s", escritas[0])
	}
}

// E O CONTRÁRIO: FALHA QUE É DA NOTA TEM QUE APARECER
//
//	Ficar de pé parecendo em ordem esconderia a nota — ninguém procura o que
//	parece em ordem. Se o teste acima passasse sozinho, a saída fácil seria nunca
//	marcar nada.
func TestFalhaDaPropriaNotaContinuaMarcando(t *testing.T) {
	var escritas []string
	m := bancoQueAnota(t, &escritas)

	m.leituraFalhou(context.Background(), "d1", "j1", errors.New("não achei o documento d1"))

	marcou := false
	for _, e := range escritas {
		if strings.Contains(e, "/documentos") && strings.Contains(e, "falhou") {
			marcou = true
		}
	}
	if !marcou {
		t.Fatalf("a nota tinha problema de verdade e ficou de pé, parecendo em ordem: %v", escritas)
	}
}
