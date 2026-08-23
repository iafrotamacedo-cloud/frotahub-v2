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
}

func novoFalso() *falso {
	f := &falso{catalogo: []map[string]any{}, marcadas: []map[string]any{}, nivelUsuario: "builder"}
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
		// contagem de logins ativos numa categoria
		if strings.Contains(q, "categoria_id=eq.") {
			fora := []map[string]any{}
			for i := 0; i < f.ativosNaC; i++ {
				fora = append(fora, map[string]any{"id": uidBuilder})
			}
			json.NewEncoder(w).Encode(fora)
			return
		}
		json.NewEncoder(w).Encode([]map[string]any{{
			"id": uidBuilder, "usuario": "builder", "nome": "Igor Tostes", "ativo": true,
			"cliente_id": idCliente, "categoria_id": idProt,
			"clientes":   map[string]any{"nome": "Frota Macedo Engenharia"},
			"categorias": map[string]any{"nome": "Builder", "nivel": f.nivelUsuario},
		}})
	})

	mux.HandleFunc("GET /rest/v1/categorias", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		prot := map[string]any{"id": idProt, "codigo": "builder", "nome": "Builder",
			"nivel": "builder", "protegida": true, "ativo": true, "criado_em": "2026-08-23"}
		comum := map[string]any{"id": idComum, "codigo": "administrativo", "nome": "Administrativo",
			"nivel": "comum", "protegida": false, "ativo": true, "criado_em": "2026-08-23"}
		switch {
		case strings.Contains(q, "id=eq."+idProt):
			json.NewEncoder(w).Encode([]map[string]any{prot})
		case strings.Contains(q, "id=eq."+idComum):
			json.NewEncoder(w).Encode([]map[string]any{comum})
		case strings.Contains(q, "id=eq."):
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			json.NewEncoder(w).Encode([]map[string]any{prot, comum})
		}
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
		`{"codigo":"administrativo","nome":"Administrativo","nivel":"comum"}`, "bom")
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
		`{"codigo":"builder","nome":"Outro dono","nivel":"comum"}`,
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
		`{"codigo":"repetida","nome":"Repetida","nivel":"comum"}`, "bom")
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
	f.nivelUsuario = "gerente"

	for _, caso := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/categorias", ""},
		{"POST", "/categorias", `{"codigo":"x","nome":"X","nivel":"comum"}`},
		{"PATCH", "/categorias/" + idComum, `{"nome":"X"}`},
		{"GET", "/categorias/" + idComum + "/permissoes", ""},
		{"PUT", "/categorias/" + idComum + "/permissoes", `{"rotinas":[]}`},
		{"GET", "/categorias/" + idComum + "/historico", ""},
	} {
		cod, _ := f.chamar(t, caso.metodo, caso.caminho, caso.corpo, "bom")
		if cod != 403 {
			t.Fatalf("%s %s com gerente devia dar 403, deu %d", caso.metodo, caso.caminho, cod)
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
