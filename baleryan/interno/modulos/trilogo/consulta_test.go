package trilogo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// ---------------------------------------------------------------------------
// Um PostgREST de mentira, EXIGENTE
// ---------------------------------------------------------------------------
//
// A lição da carga inicial: um dublê frouxo dá teste verde e produção vermelha.
// Duas vezes o motor passou nos testes e quebrou no Actions porque o servidor de
// verdade cobrava algo que o de mentira deixava passar. Este aqui cobra:
//
//   - sem `Prefer: count=exact`, não devolve contagem — devolve erro, como o de
//     verdade faria ao ser perguntado sem pedir a conta;
//   - `limit` e `offset` são obrigatórios na lista;
//   - o filtro do cliente TEM que estar presente. Uma consulta sem cliente é o
//     tipo de descuido que, num sistema com mais de um cliente, mostra o dado de
//     um para outro (CORE-11).
type bancoFalso struct {
	servidor *httptest.Server
	total    int
	linhas   []map[string]any
	pedido   string // o último caminho consultado
}

func novoBancoFalso(t *testing.T, total int, linhas []map[string]any) *bancoFalso {
	t.Helper()
	b := &bancoFalso{total: total, linhas: linhas}
	b.servidor = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.pedido = r.URL.RequestURI()

		if !strings.Contains(r.URL.RawQuery, "cliente_id=eq.") {
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"consulta sem cliente"}`))
			return
		}
		if !strings.Contains(r.Header.Get("Prefer"), "count=exact") {
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"pediu paginacao sem pedir a contagem"}`))
			return
		}

		limite, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		salto, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limite <= 0 {
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"sem limit"}`))
			return
		}

		fatia := []map[string]any{}
		for i := salto; i < salto+limite && i < len(b.linhas); i++ {
			fatia = append(fatia, b.linhas[i])
		}
		fim := salto + len(fatia) - 1
		if len(fatia) == 0 {
			w.Header().Set("Content-Range", "*/"+strconv.Itoa(b.total))
		} else {
			w.Header().Set("Content-Range",
				strconv.Itoa(salto)+"-"+strconv.Itoa(fim)+"/"+strconv.Itoa(b.total))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fatia)
	}))
	t.Cleanup(b.servidor.Close)
	return b
}

func consultaDeTeste(t *testing.T, b *bancoFalso) *Consulta {
	t.Helper()
	cfg := &config.Config{
		Supabase: config.Supabase{URL: b.servidor.URL, ChaveServico: "servico"},
		R2:       config.R2{ContaID: "conta", Bucket: "frotahub", ChaveID: "id", ChaveSecreta: "segredo"},
	}
	return NovaConsulta(banco.Novo(cfg), nil, nil, armazem.Novo(cfg))
}

func fabricar(n int) []map[string]any {
	linhas := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		linhas = append(linhas, map[string]any{"numero": 130000 + i, "loja": "LOJA 29 - MONDUBIM"})
	}
	return linhas
}

// ---------------------------------------------------------------------------
// Paginação
// ---------------------------------------------------------------------------

func TestPaginacaoContaAsPaginasCertas(t *testing.T) {
	// 1.377 é o número real da carga inicial. Em páginas de 100 dá 14 páginas,
	// com 77 linhas na última — o caso que quase sempre sai errado.
	b := novoBancoFalso(t, 1377, fabricar(1377))
	c := consultaDeTeste(t, b)

	for _, caso := range []struct {
		porPagina, pagina, querLinhas, querPaginas int
	}{
		{100, 1, 100, 14},
		{100, 14, 77, 14},
		{250, 6, 127, 6},
		{500, 3, 377, 3},
	} {
		p, err := c.puxarPagina(context.Background(), "cliente_id=eq.abc", caso.pagina, caso.porPagina)
		if err != nil {
			t.Fatalf("%d por página: %v", caso.porPagina, err)
		}
		if len(p.Linhas) != caso.querLinhas {
			t.Errorf("página %d de %d em %d: vieram %d linhas, esperava %d",
				caso.pagina, p.Paginas, caso.porPagina, len(p.Linhas), caso.querLinhas)
		}
		if p.Paginas != caso.querPaginas {
			t.Errorf("%d por página: %d páginas, esperava %d", caso.porPagina, p.Paginas, caso.querPaginas)
		}
		if p.Total != 1377 {
			t.Errorf("total veio %d", p.Total)
		}
	}
}

func TestListaVaziaNaoViraPaginaZero(t *testing.T) {
	b := novoBancoFalso(t, 0, nil)
	c := consultaDeTeste(t, b)
	p, err := c.puxarPagina(context.Background(), "cliente_id=eq.abc", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 0 || p.Paginas != 1 {
		t.Fatalf("lista vazia devia ser 0 registros em 1 página; veio %d em %d", p.Total, p.Paginas)
	}
}

// O tamanho da página vem do navegador. O que vem do navegador não se obedece.
func TestTamanhoDePaginaSoAceitaOsTres(t *testing.T) {
	for entrada, quer := range map[string]int{
		"100": 100, "250": 250, "500": 500,
		"": 100, "7": 100, "999999": 100, "-1": 100, "abc": 100, "100.5": 100,
	} {
		if veio := umDosPermitidos(entrada); veio != quer {
			t.Errorf("por_pagina=%q virou %d, esperava %d", entrada, veio, quer)
		}
	}
}

// ---------------------------------------------------------------------------
// Filtros
// ---------------------------------------------------------------------------

func TestFiltroSempreCarregaOCliente(t *testing.T) {
	c := &Consulta{}
	f, err := c.montarFiltro("cli-1", map[string][]string{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(f, "cliente_id=eq.cli-1") {
		t.Fatalf("o cliente tem que vir primeiro e sempre: %s", f)
	}
}

func TestFiltrosCombinam(t *testing.T) {
	c := &Consulta{}
	f, err := c.montarFiltro("cli-1", map[string][]string{
		"ticket":     {"130328"},
		"loja":       {"uuid-da-loja"},
		"status":     {"Vistoriado"},
		"conta":      {"instalacoes"},
		"prioridade": {"Média"},
		"de":         {"2026-07-01"},
		"ate":        {"24/08/2026"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, pedaco := range []string{
		"cliente_id=eq.cli-1", "numero=eq.130328", "unidade_id=eq.uuid-da-loja",
		"status=eq.Vistoriado", "conta=eq.instalacoes", "prioridade=eq.M",
		"criado_em=gte.", "criado_em=lte.",
	} {
		if !strings.Contains(f, pedaco) {
			t.Errorf("faltou %q em %s", pedaco, f)
		}
	}
}

// O ticket é digitado por gente com pressa: "#130328", "130328 ", "130.328".
// Tudo isso é o mesmo ticket.
func TestBuscaDeTicketIgnoraOQueNaoEDigito(t *testing.T) {
	c := &Consulta{}
	for _, entrada := range []string{"130328", "#130328", " 130328 ", "130.328", "ticket 130328"} {
		f, _ := c.montarFiltro("cli-1", map[string][]string{"ticket": {entrada}})
		if !strings.Contains(f, "numero=eq.130328") {
			t.Errorf("%q não virou o ticket 130328: %s", entrada, f)
		}
	}
	// E o que não tem dígito nenhum não vira filtro de número nenhum.
	f, _ := c.montarFiltro("cli-1", map[string][]string{"ticket": {"abc"}})
	if strings.Contains(f, "numero=eq.") {
		t.Errorf("texto sem dígito virou filtro de número: %s", f)
	}
}

func TestPrioridadeSemEUmaEscolha(t *testing.T) {
	c := &Consulta{}
	f, _ := c.montarFiltro("cli-1", map[string][]string{"prioridade": {"sem"}})
	if !strings.Contains(f, "prioridade=eq.&") && !strings.HasSuffix(f, "prioridade=eq.") {
		t.Fatalf("«sem prioridade» devia virar prioridade vazia: %s", f)
	}
}

func TestDataInvalidaExplicaEmVezDeIgnorar(t *testing.T) {
	c := &Consulta{}
	if _, err := c.montarFiltro("cli-1", map[string][]string{"de": {"31/02/2026"}}); err == nil {
		t.Error("31 de fevereiro passou como data válida")
	}
	if _, err := c.montarFiltro("cli-1", map[string][]string{"ate": {"ontem"}}); err == nil {
		t.Error("«ontem» passou como data válida")
	}
}

// O dia começa e termina no fuso da casa, não em UTC. Um chamado aberto às 22h
// de Fortaleza está gravado no dia seguinte em UTC; filtrar sem fuso o jogaria
// para fora do dia em que aconteceu.
func TestOIntervaloDeDataUsaOFusoDeFortaleza(t *testing.T) {
	inicio, err := umaData("2026-07-01", false)
	if err != nil {
		t.Fatal(err)
	}
	fim, err := umaData("2026-07-01", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(inicio, "2026-07-01T00:00:00-03:00") {
		t.Errorf("início do dia veio %s", inicio)
	}
	if !strings.HasPrefix(fim, "2026-07-01T23:59:59") || !strings.HasSuffix(fim, "-03:00") {
		t.Errorf("fim do dia veio %s", fim)
	}
	i, _ := time.Parse(time.RFC3339, inicio)
	f, _ := time.Parse(time.RFC3339, fim)
	if !f.After(i) || f.Sub(i) > 24*time.Hour {
		t.Errorf("o intervalo do dia ficou estranho: %s → %s", inicio, fim)
	}
}

// ---------------------------------------------------------------------------
// Anexos
// ---------------------------------------------------------------------------

func TestAnexoCopiadoGanhaLinkAssinadoEEsconceOEnderecoPublico(t *testing.T) {
	c := consultaDeTeste(t, novoBancoFalso(t, 0, nil))

	anexos := []map[string]any{
		{ // foto copiada para o armazém
			"nome": "IMG_1.jpg", "url_origem": "https://s3.amazonaws.com/files.trilogo.app/publico/IMG_1.jpg",
			"arquivo_sha256": "abc", "copiar": true,
			"arquivo": map[string]any{"chave_r2": "clientes/frota-macedo/arquivos/ab/cd/abcd.jpg"},
		},
		{ // vídeo, que por decisão nossa não foi copiado
			"nome": "VID_1.mp4", "url_origem": "https://s3.amazonaws.com/files.trilogo.app/publico/VID_1.mp4",
			"arquivo_sha256": nil, "copiar": false,
		},
	}
	c.enderecar(anexos)

	foto, video := anexos[0], anexos[1]

	if foto["onde"] != "armazem" {
		t.Errorf("a foto copiada devia vir do armazém, veio de %v", foto["onde"])
	}
	link, _ := foto["link"].(string)
	if !strings.Contains(link, "X-Amz-Signature=") || !strings.Contains(link, "X-Amz-Expires=300") {
		t.Errorf("a foto não veio com endereço assinado e com prazo: %s", link)
	}
	if _, ainda := foto["url_origem"]; ainda {
		t.Error("o endereço público do sistema antigo continuou saindo na resposta")
	}

	if video["onde"] != "trilogo" {
		t.Errorf("o vídeo não foi copiado; devia apontar para o Trílogo, apontou para %v", video["onde"])
	}
	if video["link"] != "https://s3.amazonaws.com/files.trilogo.app/publico/VID_1.mp4" {
		t.Errorf("o link do vídeo mudou: %v", video["link"])
	}
}

// ---------------------------------------------------------------------------
// Texto da linha do tempo
// ---------------------------------------------------------------------------

func TestTextoDoEventoChegaLimpoNaTela(t *testing.T) {
	for entrada, quer := range map[string]string{
		// caso real, tal como está gravado no banco
		"Nova empresa prestadora: <b>FROTA - INSTALAÇÕES</b> <br/>Anterior: <br>ARTEC CLIMATIZAÇÃO</br>": "Nova empresa prestadora: FROTA - INSTALAÇÕES Anterior: ARTEC CLIMATIZAÇÃO",
		"Prioridade: Média":           "Prioridade: Média",
		"":                            "",
		"caf&eacute; &amp; leite":     "café & leite",
		"<script>alerta()</script>ok": "alerta()ok",
	} {
		if veio := SemHTML(entrada); veio != quer {
			t.Errorf("SemHTML(%q)\n veio: %q\n quer: %q", entrada, veio, quer)
		}
	}
}
