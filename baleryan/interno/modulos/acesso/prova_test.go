package acesso

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

const (
	uidBuilder = "11111111-1111-1111-1111-111111111111"
	idCliente  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	idComum    = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	idProt     = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	idCeoCat   = "99999999-1111-1111-1111-111111111111"

	// perfis de teste pra hierarquia — categoria diferente de perfil, de
	// propósito, pra não confundir "id da categoria ceo" com "id do login ceo".
	idPerfilCeo          = "a0000000-1111-1111-1111-111111111111"
	idPerfilGerencial    = "a0000000-2222-2222-2222-222222222222"
	idPerfilSupervisorio = "a0000000-3333-3333-3333-333333333333"
	idPerfilOperacional  = "a0000000-4444-4444-4444-444444444444"
	idCatGerencial       = "a0000000-5555-5555-5555-555555555555"
	idCatSupervisorio    = "a0000000-6666-6666-6666-666666666666"
)

type falso struct {
	srv          *httptest.Server
	gravados     []map[string]any
	upserts      []map[string]any
	patches      []string
	catalogo     []map[string]any
	marcadas     []map[string]any
	ativosNaC    int
	nivelUsuario string // o nível de quem está chamando

	// fixtures pros testes de hierarquia
	perfis   map[string]map[string]any // id -> linha (com categorias embutida)
	vinculos map[string]string         // perfil_id -> superior_id
}

func novoFalso() *falso {
	f := &falso{
		catalogo: []map[string]any{}, marcadas: []map[string]any{}, nivelUsuario: "builder",
		vinculos: map[string]string{},
		perfis: map[string]map[string]any{
			idPerfilCeo: {
				"id": idPerfilCeo, "nome": "CEO Teste", "usuario": "ceo1", "ativo": true,
				"categoria_id": idCeoCat, "categorias": map[string]any{"nome": "CEO", "nivel": "ceo"},
			},
			idPerfilGerencial: {
				"id": idPerfilGerencial, "nome": "Gerencial Teste", "usuario": "gerencial1", "ativo": true,
				"categoria_id": idCatGerencial, "categorias": map[string]any{"nome": "Gerencial", "nivel": "gerencial"},
			},
			idPerfilSupervisorio: {
				"id": idPerfilSupervisorio, "nome": "Supervisório Teste", "usuario": "supervisorio1", "ativo": true,
				"categoria_id": idCatSupervisorio, "categorias": map[string]any{"nome": "Supervisório", "nivel": "supervisorio"},
			},
			idPerfilOperacional: {
				"id": idPerfilOperacional, "nome": "Operacional Teste", "usuario": "operacional1", "ativo": true,
				"categoria_id": idComum, "categorias": map[string]any{"nome": "Administrativo", "nivel": "operacional"},
			},
		},
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/v1/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bom" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": uidBuilder})
	})

	mux.HandleFunc("GET /rest/v1/perfis", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		switch {
		// loginsAtivos (acesso.go): só `select=id`, sem mais nada — a contagem
		// de logins ativos numa categoria, pro "não desativa com gente dentro".
		case strings.HasSuffix(q, "select=id"):
			fora := []map[string]any{}
			for i := 0; i < f.ativosNaC; i++ {
				fora = append(fora, map[string]any{"id": uidBuilder})
			}
			json.NewEncoder(w).Encode(fora)
		// o próprio login (seguranca.PerfilDe) — nível dinâmico via f.nivelUsuario.
		case strings.Contains(q, "id=eq."+uidBuilder):
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": uidBuilder, "usuario": "builder", "nome": "Igor Tostes", "ativo": true,
				"cliente_id": idCliente, "categoria_id": idProt,
				"clientes":   map[string]any{"nome": "Frota Macedo Engenharia"},
				"categorias": map[string]any{"nome": "Builder", "nivel": f.nivelUsuario},
			}})
		// perfilLeve (hierarquia.go): um perfil específico por id.
		case temFiltro(q, "id=eq."):
			id := valorDoFiltro(q, "id=eq.")
			if pf, ok := f.perfis[id]; ok {
				json.NewEncoder(w).Encode([]map[string]any{pf})
			} else {
				json.NewEncoder(w).Encode([]map[string]any{})
			}
		// ceoDoCliente (hierarquia.go): o perfil daquela categoria.
		case strings.Contains(q, "categoria_id=eq."):
			catID := valorDoFiltro(q, "categoria_id=eq.")
			for _, pf := range f.perfis {
				if pf["categoria_id"] == catID {
					json.NewEncoder(w).Encode([]map[string]any{pf})
					return
				}
			}
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	})

	mux.HandleFunc("GET /rest/v1/categorias", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		prot := map[string]any{"id": idProt, "codigo": "builder", "nome": "Builder",
			"nivel": "builder", "protegida": true, "ativo": true, "criado_em": "2026-08-23"}
		comum := map[string]any{"id": idComum, "codigo": "administrativo", "nome": "Administrativo",
			"nivel": "operacional", "protegida": false, "ativo": true, "criado_em": "2026-08-23"}
		ceoCat := map[string]any{"id": idCeoCat, "codigo": "ceo", "nome": "CEO",
			"nivel": "ceo", "protegida": false, "ativo": true, "criado_em": "2026-08-23"}
		switch {
		case strings.Contains(q, "id=eq."+idProt):
			json.NewEncoder(w).Encode([]map[string]any{prot})
		case strings.Contains(q, "id=eq."+idComum):
			json.NewEncoder(w).Encode([]map[string]any{comum})
		case strings.Contains(q, "id=eq."+idCeoCat):
			json.NewEncoder(w).Encode([]map[string]any{ceoCat})
		// ceoDoCliente (hierarquia.go): acha a categoria ceo do cliente — TEM
		// que vir antes do `temFiltro(q, "id=eq.")` de baixo, senão
		// "cliente_id=eq." (que contém "id=eq." como substring!) cai lá.
		case strings.Contains(q, "nivel=eq.ceo"):
			json.NewEncoder(w).Encode([]map[string]any{ceoCat})
		case temFiltro(q, "id=eq."):
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			json.NewEncoder(w).Encode([]map[string]any{prot, comum, ceoCat})
		}
	})

	mux.HandleFunc("GET /rest/v1/vinculos_hierarquicos", func(w http.ResponseWriter, r *http.Request) {
		pid := valorDoFiltro(r.URL.RawQuery, "perfil_id=eq.")
		if sup, ok := f.vinculos[pid]; ok {
			json.NewEncoder(w).Encode([]map[string]any{{"superior_id": sup}})
			return
		}
		json.NewEncoder(w).Encode([]map[string]any{})
	})
	mux.HandleFunc("POST /rest/v1/vinculos_hierarquicos", func(w http.ResponseWriter, r *http.Request) {
		bruto, _ := io.ReadAll(r.Body)
		var linhas []map[string]any
		json.Unmarshal(bruto, &linhas)
		for _, l := range linhas {
			pid, _ := l["perfil_id"].(string)
			sup, _ := l["superior_id"].(string)
			f.vinculos[pid] = sup
		}
		w.WriteHeader(201)
	})
	mux.HandleFunc("DELETE /rest/v1/vinculos_hierarquicos", func(w http.ResponseWriter, r *http.Request) {
		pid := valorDoFiltro(r.URL.RawQuery, "perfil_id=eq.")
		delete(f.vinculos, pid)
		w.WriteHeader(200)
	})
	mux.HandleFunc("POST /rest/v1/categorias", func(w http.ResponseWriter, r *http.Request) {
		bruto, _ := io.ReadAll(r.Body)
		if strings.Contains(string(bruto), "\"repetida\"") {
			w.WriteHeader(409)
			w.Write([]byte(`{"code":"23505","message":"duplicate key"}`))
			return
		}
		var linhas []map[string]any
		json.Unmarshal(bruto, &linhas)
		nova := linhas[0]
		nova["id"] = "99999999-9999-9999-9999-999999999999"
		nova["protegida"] = false
		nova["ativo"] = true
		w.WriteHeader(201)
		json.NewEncoder(w).Encode([]map[string]any{nova})
	})
	mux.HandleFunc("PATCH /rest/v1/categorias", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	})

	mux.HandleFunc("GET /rest/v1/rotinas", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(f.catalogo)
	})
	mux.HandleFunc("GET /rest/v1/categoria_permissoes", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(f.marcadas)
	})
	mux.HandleFunc("POST /rest/v1/categoria_permissoes", func(w http.ResponseWriter, r *http.Request) {
		bruto, _ := io.ReadAll(r.Body)
		var linhas []map[string]any
		json.Unmarshal(bruto, &linhas)
		f.upserts = append(f.upserts, linhas...)
		w.WriteHeader(201)
	})
	mux.HandleFunc("PATCH /rest/v1/categoria_permissoes", func(w http.ResponseWriter, r *http.Request) {
		f.patches = append(f.patches, r.URL.RawQuery)
		w.WriteHeader(204)
	})

	mux.HandleFunc("POST /rest/v1/historico", func(w http.ResponseWriter, r *http.Request) {
		bruto, _ := io.ReadAll(r.Body)
		var linhas []map[string]any
		json.Unmarshal(bruto, &linhas)
		f.gravados = append(f.gravados, linhas...)
		w.WriteHeader(201)
	})
	mux.HandleFunc("GET /rest/v1/historico", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "acao": "criou", "autor_usuario": "builder", "quando": "2026-08-23T20:00:00Z"},
		})
	})

	f.srv = httptest.NewServer(mux)
	return f
}

// temFiltro diz se a query tem ESTE filtro, e não só um parecido — sem isto,
// `strings.Contains(q, "id=eq.")` bate em "cliente_id=eq." também (é
// substring!), e o mock respondia com o fixture errado.
func temFiltro(q, prefixo string) bool {
	return strings.HasPrefix(q, prefixo) || strings.Contains(q, "&"+prefixo)
}

// valorDoFiltro extrai o valor de um filtro `chave=eq.valor` da query string
// do PostgREST — só o suficiente pro mock encontrar o id, nunca um parser de
// verdade.
func valorDoFiltro(q, prefixo string) string {
	i := strings.Index(q, prefixo)
	if i < 0 {
		return ""
	}
	resto := q[i+len(prefixo):]
	if j := strings.Index(resto, "&"); j >= 0 {
		return resto[:j]
	}
	return resto
}

func (f *falso) chamar(t *testing.T, metodo, caminho, corpo, token string) (int, map[string]any) {
	t.Helper()
	cfg := &config.Config{
		Supabase: config.Supabase{URL: f.srv.URL, ChaveServico: "servico", ChavePublica: "publica"},
		Runtime:  config.Runtime{Ambiente: "local"},
	}
	bd := banco.Novo(cfg)
	mux := http.NewServeMux()
	Novo(bd, seguranca.Novo(cfg, bd), historico.Novo(bd)).Montar(mux)

	var leitor io.Reader
	if corpo != "" {
		leitor = strings.NewReader(corpo)
	}
	req := httptest.NewRequest(metodo, caminho, leitor)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// ---------------------------------------------------------------------------

func TestCriarCategoriaGravaHistorico(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "POST", "/categorias",
		`{"codigo":"administrativo","nome":"Administrativo","nivel":"operacional"}`, "bom")
	if cod != 201 {
		t.Fatalf("esperava 201, veio %d: %v", cod, resp)
	}
	if len(f.gravados) != 1 {
		t.Fatalf("esperava 1 linha, vieram %d", len(f.gravados))
	}
	h := f.gravados[0]
	if h["modulo"] != "acesso" || h["acao"] != "criou" {
		t.Fatalf("linha errada: %v", h)
	}
}

func TestNaoSeCriaCategoriaBuilder(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	for _, corpo := range []string{
		`{"codigo":"builder","nome":"Outro dono","nivel":"operacional"}`,
		`{"codigo":"chefia","nome":"Chefia","nivel":"builder"}`,
	} {
		cod, resp := f.chamar(t, "POST", "/categorias", corpo, "bom")
		if cod != 400 {
			t.Fatalf("%s devia dar 400, deu %d: %v", corpo, cod, resp)
		}
	}
	if len(f.gravados) != 0 {
		t.Fatalf("não podia gravar nada: %v", f.gravados)
	}
}

func TestCodigoRepetidoDaFraseClara(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "POST", "/categorias",
		`{"codigo":"repetida","nome":"Repetida","nivel":"operacional"}`, "bom")
	if cod != 409 {
		t.Fatalf("esperava 409, veio %d", cod)
	}
	if !strings.Contains(resp["erro"].(string), "Já existe") {
		t.Fatalf("mensagem pouco clara: %v", resp)
	}
}

func TestCategoriaProtegidaNaoSeEdita(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, _ := f.chamar(t, "PATCH", "/categorias/"+idProt, `{"nome":"Outro nome"}`, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400, veio %d", cod)
	}
	cod, _ = f.chamar(t, "PUT", "/categorias/"+idProt+"/permissoes", `{"rotinas":[]}`, "bom")
	if cod != 400 {
		t.Fatalf("matriz da protegida devia dar 400, deu %d", cod)
	}
}

func TestNaoDesativaCategoriaComGenteDentro(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.ativosNaC = 3

	cod, resp := f.chamar(t, "PATCH", "/categorias/"+idComum, `{"ativo":false}`, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400, veio %d: %v", cod, resp)
	}
	if !strings.Contains(resp["erro"].(string), "3") {
		t.Fatalf("a mensagem devia dizer quantos são: %v", resp)
	}
}

func TestMatrizVaziaAbreSemQuebrar(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "GET", "/categorias/"+idComum+"/permissoes", "", "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	if len(resp["rotinas"].([]any)) != 0 || len(resp["permitidas"].([]any)) != 0 {
		t.Fatalf("com catálogo vazio, os dois têm que vir vazios: %v", resp)
	}
}

func TestMatrizGravaSoADiferenca(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.catalogo = []map[string]any{
		{"codigo": "A", "nome": "Rotina A", "modulo": "m", "ordem": 1},
		{"codigo": "B", "nome": "Rotina B", "modulo": "m", "ordem": 2},
		{"codigo": "C", "nome": "Rotina C", "modulo": "m", "ordem": 3},
	}
	// hoje: A e B marcadas. Pedido: B e C. Logo: marca C, desmarca A, mantém B.
	f.marcadas = []map[string]any{{"rotina": "A", "pode": true}, {"rotina": "B", "pode": true}}

	cod, resp := f.chamar(t, "PUT", "/categorias/"+idComum+"/permissoes", `{"rotinas":["B","C"]}`, "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	if len(f.upserts) != 1 || f.upserts[0]["rotina"] != "C" {
		t.Fatalf("devia ter liberado só a C: %v", f.upserts)
	}
	if len(f.patches) != 1 || !strings.Contains(f.patches[0], "A") {
		t.Fatalf("devia ter retirado só a A: %v", f.patches)
	}
	if len(f.gravados) != 1 {
		t.Fatalf("esperava 1 linha de histórico para o salvamento inteiro, vieram %d", len(f.gravados))
	}
	mud := f.gravados[0]["mudancas"].(map[string]any)
	if len(mud) != 2 {
		t.Fatalf("o histórico devia listar só as 2 que mudaram: %v", mud)
	}
	if mud["C"].(map[string]any)["para"] != true || mud["A"].(map[string]any)["para"] != false {
		t.Fatalf("de/para errado: %v", mud)
	}
	if _, tem := mud["B"]; tem {
		t.Fatalf("B não mudou e não podia aparecer: %v", mud)
	}
}

func TestMatrizSemMudancaNaoGrava(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.catalogo = []map[string]any{{"codigo": "A", "nome": "A", "modulo": "m", "ordem": 1}}
	f.marcadas = []map[string]any{{"rotina": "A", "pode": true}}

	cod, resp := f.chamar(t, "PUT", "/categorias/"+idComum+"/permissoes", `{"rotinas":["A"]}`, "bom")
	if cod != 200 || resp["sem_mudanca"] != true {
		t.Fatalf("esperava sem_mudanca, veio %d %v", cod, resp)
	}
	if len(f.gravados) != 0 {
		t.Fatalf("não podia gravar histórico: %v", f.gravados)
	}
}

func TestRotinaInventadaERecusada(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.catalogo = []map[string]any{{"codigo": "A", "nome": "A", "modulo": "m", "ordem": 1}}

	cod, resp := f.chamar(t, "PUT", "/categorias/"+idComum+"/permissoes", `{"rotinas":["INVENTADA"]}`, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400, veio %d: %v", cod, resp)
	}
	if len(f.upserts) != 0 || len(f.gravados) != 0 {
		t.Fatalf("nada podia ter sido gravado")
	}
}

func TestSoBuilderMexeEmAcesso(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "gerencial"

	for _, caso := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/categorias", ""},
		{"POST", "/categorias", `{"codigo":"x","nome":"X","nivel":"operacional"}`},
		{"PATCH", "/categorias/" + idComum, `{"nome":"X"}`},
		{"GET", "/categorias/" + idComum + "/permissoes", ""},
		{"PUT", "/categorias/" + idComum + "/permissoes", `{"rotinas":[]}`},
		{"GET", "/categorias/" + idComum + "/historico", ""},
	} {
		cod, _ := f.chamar(t, caso.metodo, caso.caminho, caso.corpo, "bom")
		if cod != 403 {
			t.Fatalf("%s %s com gerencial devia dar 403, deu %d", caso.metodo, caso.caminho, cod)
		}
	}
}

func TestCEOGerenciaCategoriaAbaixoDele(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"

	for _, caso := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/categorias", ""},
		{"POST", "/categorias", `{"codigo":"yyy","nome":"Y","nivel":"gerencial"}`},
		{"PATCH", "/categorias/" + idComum, `{"nome":"Novo nome"}`},
		{"GET", "/categorias/" + idComum + "/permissoes", ""},
		{"PUT", "/categorias/" + idComum + "/permissoes", `{"rotinas":[]}`},
	} {
		cod, resp := f.chamar(t, caso.metodo, caso.caminho, caso.corpo, "bom")
		if cod < 200 || cod >= 300 {
			t.Fatalf("%s %s com ceo numa categoria abaixo dele devia passar, deu %d: %v", caso.metodo, caso.caminho, cod, resp)
		}
	}
}

func TestCEONaoMexeEmCategoriaCeoOuBuilder(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"

	for _, caso := range []struct{ metodo, caminho, corpo string }{
		{"PATCH", "/categorias/" + idProt, `{"nome":"X"}`},
		{"GET", "/categorias/" + idProt + "/permissoes", ""},
		{"PUT", "/categorias/" + idProt + "/permissoes", `{"rotinas":[]}`},
		{"PATCH", "/categorias/" + idCeoCat, `{"nome":"X"}`},
		{"GET", "/categorias/" + idCeoCat + "/permissoes", ""},
		{"PUT", "/categorias/" + idCeoCat + "/permissoes", `{"rotinas":[]}`},
		{"POST", "/categorias", `{"codigo":"zzz","nome":"Z","nivel":"ceo"}`},
	} {
		cod, _ := f.chamar(t, caso.metodo, caso.caminho, caso.corpo, "bom")
		if cod != 403 {
			t.Fatalf("%s %s com ceo fora do próprio alcance devia dar 403, deu %d", caso.metodo, caso.caminho, cod)
		}
	}
}

func TestSemTokenNaoEntra(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	cod, _ := f.chamar(t, "GET", "/categorias", "", "")
	if cod != 401 {
		t.Fatalf("esperava 401, veio %d", cod)
	}
}

func TestCategoriaInexistenteDa404(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	cod, _ := f.chamar(t, "PATCH", "/categorias/00000000-0000-0000-0000-000000000000", `{"nome":"X"}`, "bom")
	if cod != 404 {
		t.Fatalf("esperava 404, veio %d", cod)
	}
}

// ---------------------------------------------------------------------------
// vínculo hierárquico
// ---------------------------------------------------------------------------

func TestNivelEncaixaAbaixoDe(t *testing.T) {
	casos := []struct {
		nivel, acima string
		espera       bool
	}{
		{"gerencial", "ceo", true},
		{"gerencial", "gerencial", false},
		{"gerencial", "supervisorio", false},
		{"supervisorio", "gerencial", true},
		{"supervisorio", "ceo", false},
		{"operacional", "ceo", true},
		{"operacional", "gerencial", true},
		{"operacional", "supervisorio", true},
		{"operacional", "operacional", false},
	}
	for _, c := range casos {
		if got := nivelEncaixaAbaixoDe(c.nivel, c.acima); got != c.espera {
			t.Fatalf("nivelEncaixaAbaixoDe(%q, %q) = %v, esperava %v", c.nivel, c.acima, got, c.espera)
		}
	}
}

// Sem NENHUM vínculo gravado, a cadeia é sempre "CEO implícito > o próprio
// login" — o default nunca vira linha na tabela (ver o cabeçalho do
// hierarquia.go).
func TestCadeiaDefaultEhCeoImplicito(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "GET", "/perfis/"+idPerfilOperacional+"/hierarquia", "", "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	cadeia, _ := resp["cadeia"].([]any)
	if len(cadeia) != 2 {
		t.Fatalf("esperava cadeia de 2 nós (CEO implícito + o próprio), veio %d: %v", len(cadeia), cadeia)
	}
	topo := cadeia[0].(map[string]any)
	if topo["nivel"] != "ceo" || topo["implicito"] != true {
		t.Fatalf("o topo devia ser o CEO implícito, veio %v", topo)
	}
	alvo := cadeia[1].(map[string]any)
	if alvo["id"] != idPerfilOperacional {
		t.Fatalf("o segundo nó devia ser o próprio alvo, veio %v", alvo)
	}
}

// Com um vínculo explícito no meio, a cadeia caminha por ele em vez de
// pular direto pro CEO.
func TestCadeiaCaminhaPelosVinculosExplicitos(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.vinculos[idPerfilGerencial] = idPerfilCeo
	f.vinculos[idPerfilSupervisorio] = idPerfilGerencial
	f.vinculos[idPerfilOperacional] = idPerfilSupervisorio

	cod, resp := f.chamar(t, "GET", "/perfis/"+idPerfilOperacional+"/hierarquia", "", "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	cadeia, _ := resp["cadeia"].([]any)
	if len(cadeia) != 4 {
		t.Fatalf("esperava 4 nós (ceo, gerencial, supervisorio, operacional), veio %d: %v", len(cadeia), cadeia)
	}
	ordem := []string{idPerfilCeo, idPerfilGerencial, idPerfilSupervisorio, idPerfilOperacional}
	for i, esperado := range ordem {
		if cadeia[i].(map[string]any)["id"] != esperado {
			t.Fatalf("nó %d devia ser %s, veio %v", i, esperado, cadeia[i])
		}
	}
}

func TestInserirNaHierarquiaFeliz(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"
	// pré-condição: quem vai entrar (Gerencial) já responde ao CEO.
	f.vinculos[idPerfilGerencial] = idPerfilCeo

	corpo := `{"acima_id":"` + idPerfilCeo + `","novo_superior_id":"` + idPerfilGerencial + `"}`
	cod, resp := f.chamar(t, "PUT", "/perfis/"+idPerfilOperacional+"/hierarquia", corpo, "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	if f.vinculos[idPerfilOperacional] != idPerfilGerencial {
		t.Fatalf("esperava %s passando a responder a %s, ficou %q", idPerfilOperacional, idPerfilGerencial, f.vinculos[idPerfilOperacional])
	}
}

// SEM vínculo nenhum gravado, X já responde ao CEO IMPLICITAMENTE — a
// pré-condição usa esse default, não só linha gravada (é o bug que o dono
// achou testando: sem isto, a PRIMEIRA inserção de qualquer cadeia nunca
// conseguia acontecer, porque ninguém tem linha gravada antes da primeira
// inserção de todas).
func TestInserirAceitaPreCondicaoImplicita(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"
	// idPerfilGerencial não tem vínculo gravado nenhum — o default dele já
	// é o CEO, sem precisar de linha nenhuma.

	corpo := `{"acima_id":"` + idPerfilCeo + `","novo_superior_id":"` + idPerfilGerencial + `"}`
	cod, resp := f.chamar(t, "PUT", "/perfis/"+idPerfilOperacional+"/hierarquia", corpo, "bom")
	if cod != 200 {
		t.Fatalf("esperava 200 (pré-condição implícita satisfeita), veio %d: %v", cod, resp)
	}
	if f.vinculos[idPerfilOperacional] != idPerfilGerencial {
		t.Fatalf("esperava %s passando a responder a %s, ficou %q", idPerfilOperacional, idPerfilGerencial, f.vinculos[idPerfilOperacional])
	}
}

// X (quem está entrando) precisa já se reportar a P especificamente — nem
// "ter algum superior em algum lugar", nem o default implícito quando P NÃO
// é o CEO (ver o cabeçalho do arquivo).
func TestInserirRecusaSemPreCondicao(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"
	// idPerfilOperacional já responde de verdade ao Gerencial — é o "P" desta
	// inserção. idPerfilSupervisorio não tem vínculo nenhum, então o default
	// dele é o CEO, não o Gerencial: não satisfaz a pré-condição.
	f.vinculos[idPerfilOperacional] = idPerfilGerencial

	corpo := `{"acima_id":"` + idPerfilGerencial + `","novo_superior_id":"` + idPerfilSupervisorio + `"}`
	cod, _ := f.chamar(t, "PUT", "/perfis/"+idPerfilOperacional+"/hierarquia", corpo, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400 (pré-condição não satisfeita), veio %d", cod)
	}
	if f.vinculos[idPerfilOperacional] != idPerfilGerencial {
		t.Fatalf("não podia ter mudado nada")
	}
}

// Supervisório só entra logo abaixo de Gerencial — direto sob o CEO é nível
// errado pro degrau.
func TestInserirRecusaNivelErrado(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"
	f.vinculos[idPerfilSupervisorio] = idPerfilCeo // tem vínculo, mas o NÍVEL não bate

	corpo := `{"acima_id":"` + idPerfilCeo + `","novo_superior_id":"` + idPerfilSupervisorio + `"}`
	cod, _ := f.chamar(t, "PUT", "/perfis/"+idPerfilOperacional+"/hierarquia", corpo, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400 (nível não encaixa), veio %d", cod)
	}
}

// A linha da tela precisa bater com a cadeia de verdade — "acima_id" errado
// (cadeia mudou nesse meio tempo) é recusado, não aceito silenciosamente.
func TestInserirRecusaAcimaDesatualizado(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"
	f.vinculos[idPerfilGerencial] = idPerfilCeo

	corpo := `{"acima_id":"` + idPerfilGerencial + `","novo_superior_id":"` + idPerfilGerencial + `"}`
	cod, _ := f.chamar(t, "PUT", "/perfis/"+idPerfilOperacional+"/hierarquia", corpo, "bom")
	if cod != 409 {
		t.Fatalf("esperava 409 (acima_id desatualizado — o de verdade é o CEO implícito), veio %d", cod)
	}
}

func TestRemoverDaHierarquiaVoltaAoDefault(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.nivelUsuario = "ceo"
	f.vinculos[idPerfilOperacional] = idPerfilGerencial

	cod, _ := f.chamar(t, "DELETE", "/perfis/"+idPerfilOperacional+"/hierarquia", "", "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d", cod)
	}
	if _, existe := f.vinculos[idPerfilOperacional]; existe {
		t.Fatalf("o vínculo devia ter sumido")
	}
}
