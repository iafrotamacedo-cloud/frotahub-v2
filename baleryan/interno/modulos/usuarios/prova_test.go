package usuarios

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
	uidAlvo    = "22222222-2222-2222-2222-222222222222"
	idCliente  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	idCatA     = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	idCatB     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

type falso struct {
	srv          *httptest.Server
	gravados     []map[string]any // tudo que caiu na tabela historico
	historicoErr bool
	alvoExiste   bool
	alvoNome     string
	alvoAtivo    bool
	alvoCat      string
}

func novoFalso() *falso {
	f := &falso{alvoExiste: true, alvoNome: "Maria Souza", alvoAtivo: true, alvoCat: idCatA}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/v1/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer bom" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": uidBuilder})
	})
	mux.HandleFunc("POST /auth/v1/admin/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": uidAlvo})
	})
	mux.HandleFunc("PUT /auth/v1/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	// grant_type=password: aceita só a senha atual "certa123"
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		bruto, _ := io.ReadAll(r.Body)
		var c struct {
			Password string `json:"password"`
		}
		json.Unmarshal(bruto, &c)
		if c.Password != "certa123" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "x"})
	})

	mux.HandleFunc("GET /rest/v1/perfis", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		switch {
		case strings.Contains(q, "clientes%28nome%29") || strings.Contains(q, "clientes(nome)"):
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": uidBuilder, "usuario": "builder", "nome": "Igor Tostes", "ativo": true,
				"cliente_id": idCliente, "categoria_id": idCatA,
				"clientes":   map[string]any{"nome": "Frota Macedo Engenharia"},
				"categorias": map[string]any{"nome": "Builder", "nivel": "builder"},
			}})
		default:
			if !f.alvoExiste {
				json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{{
				"id": uidAlvo, "usuario": "maria", "nome": f.alvoNome,
				"categoria_id": f.alvoCat, "ativo": f.alvoAtivo,
				"categorias": map[string]any{"nome": "Administrativo"},
			}})
		}
	})
	mux.HandleFunc("GET /rest/v1/categorias", func(w http.ResponseWriter, r *http.Request) {
		nome := "Administrativo"
		if strings.Contains(r.URL.RawQuery, idCatB) {
			nome = "Operacional"
		}
		json.NewEncoder(w).Encode([]map[string]any{{"id": idCatA, "nome": nome}})
	})
	mux.HandleFunc("POST /rest/v1/perfis", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) })
	mux.HandleFunc("PATCH /rest/v1/perfis", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })

	mux.HandleFunc("POST /rest/v1/historico", func(w http.ResponseWriter, r *http.Request) {
		bruto, _ := io.ReadAll(r.Body)
		if f.historicoErr {
			w.WriteHeader(500)
			w.Write([]byte(`{"message":"boom"}`))
			return
		}
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

func (f *falso) modulo() *Modulo {
	cfg := &config.Config{
		Supabase:     config.Supabase{URL: f.srv.URL, ChaveServico: "servico", ChavePublica: "publica"},
		Runtime:      config.Runtime{Ambiente: "local"},
		DominioLogin: "frotahub.local",
	}
	bd := banco.Novo(cfg)
	return Novo(cfg, bd, seguranca.Novo(cfg, bd), historico.Novo(bd))
}

func (f *falso) chamar(t *testing.T, metodo, caminho, corpo, token string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	f.modulo().Montar(mux)

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

func (f *falso) ultimo() map[string]any {
	if len(f.gravados) == 0 {
		return nil
	}
	return f.gravados[len(f.gravados)-1]
}

// ---------------------------------------------------------------------------

func TestCriarGravaHistorico(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "POST", "/usuarios",
		`{"usuario":"maria","nome":"Maria Souza","senha":"segredo123","categoria_id":"`+idCatA+`"}`, "bom")
	if cod != 201 {
		t.Fatalf("esperava 201, veio %d: %v", cod, resp)
	}
	if _, tem := resp["aviso"]; tem {
		t.Fatalf("não devia ter aviso: %v", resp)
	}
	if len(f.gravados) != 1 {
		t.Fatalf("esperava 1 linha de histórico, vieram %d", len(f.gravados))
	}
	h := f.ultimo()
	if h["acao"] != "criou" || h["modulo"] != "usuarios" || h["autor_usuario"] != "builder" ||
		h["autor_id"] != uidBuilder || h["registro_id"] != uidAlvo || h["cliente_id"] != idCliente {
		t.Fatalf("cabeçalho do histórico errado: %v", h)
	}
	mud, _ := h["mudancas"].(map[string]any)
	for _, campo := range []string{"usuario", "nome", "categoria"} {
		par, ok := mud[campo].(map[string]any)
		if !ok {
			t.Fatalf("faltou o campo %q em mudancas: %v", campo, mud)
		}
		if par["de"] != nil {
			t.Fatalf("na criação, o %q tinha que ter 'de' nulo: %v", campo, par)
		}
	}
	if mud["categoria"].(map[string]any)["para"] != "Administrativo" {
		t.Fatalf("a categoria tinha que ir pelo NOME: %v", mud["categoria"])
	}
}

func TestSenhaNuncaEntraNoHistorico(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	f.chamar(t, "POST", "/usuarios",
		`{"usuario":"maria","nome":"Maria Souza","senha":"segredo123","categoria_id":"`+idCatA+`"}`, "bom")
	f.chamar(t, "POST", "/usuarios/"+uidAlvo+"/senha", `{"senha":"outrasenha99"}`, "bom")

	tudo, _ := json.Marshal(f.gravados)
	for _, proibido := range []string{"segredo123", "outrasenha99", "senha\":\""} {
		if strings.Contains(string(tudo), proibido) {
			t.Fatalf("vazou %q no histórico: %s", proibido, tudo)
		}
	}
	h := f.ultimo()
	if h["acao"] != "trocou_senha" {
		t.Fatalf("esperava trocou_senha, veio %v", h["acao"])
	}
	if _, tem := h["mudancas"]; tem {
		t.Fatalf("troca de senha não pode ter conteúdo: %v", h)
	}
}

func TestEditarSeparaDadosDeSituacao(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "PATCH", "/usuarios/"+uidAlvo,
		`{"nome":"Maria de Souza","ativo":false}`, "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	if len(f.gravados) != 2 {
		t.Fatalf("esperava 2 linhas (alterou + desativou), vieram %d: %v", len(f.gravados), f.gravados)
	}
	a, b := f.gravados[0], f.gravados[1]
	if a["acao"] != "alterou" {
		t.Fatalf("primeira linha devia ser 'alterou': %v", a)
	}
	par := a["mudancas"].(map[string]any)["nome"].(map[string]any)
	if par["de"] != "Maria Souza" || par["para"] != "Maria de Souza" {
		t.Fatalf("de/para do nome errado: %v", par)
	}
	if b["acao"] != "desativou" {
		t.Fatalf("segunda linha devia ser 'desativou': %v", b)
	}
}

func TestEditarSemMudancaRealNaoGrava(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	// Reenvia exatamente o que já está lá.
	cod, resp := f.chamar(t, "PATCH", "/usuarios/"+uidAlvo, `{"nome":"Maria Souza","ativo":true}`, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400, veio %d: %v", cod, resp)
	}
	if len(f.gravados) != 0 {
		t.Fatalf("não podia ter gravado nada: %v", f.gravados)
	}
}

func TestEditarLoginInexistenteDa404(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.alvoExiste = false

	cod, resp := f.chamar(t, "PATCH", "/usuarios/"+uidAlvo, `{"nome":"Fantasma"}`, "bom")
	if cod != 404 {
		t.Fatalf("esperava 404, veio %d: %v", cod, resp)
	}
}

func TestHistoricoQueFalhaViraAviso(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.historicoErr = true

	cod, resp := f.chamar(t, "PATCH", "/usuarios/"+uidAlvo, `{"nome":"Maria de Souza"}`, "bom")
	if cod != 200 {
		t.Fatalf("a alteração ACONTECEU, então tem que responder 200; veio %d", cod)
	}
	if resp["aviso"] == nil || !strings.Contains(resp["aviso"].(string), "histórico") {
		t.Fatalf("faltou o aviso de histórico não gravado: %v", resp)
	}
}

func TestNaoSeDesativaSozinho(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "PATCH", "/usuarios/"+uidBuilder, `{"ativo":false}`, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400, veio %d: %v", cod, resp)
	}
	if len(f.gravados) != 0 {
		t.Fatalf("não podia ter gravado nada: %v", f.gravados)
	}
}

func TestSemTokenNaoEntra(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	for _, caso := range []struct{ metodo, caminho, corpo string }{
		{"GET", "/usuarios", ""},
		{"POST", "/usuarios", `{}`},
		{"PATCH", "/usuarios/" + uidAlvo, `{}`},
		{"POST", "/usuarios/" + uidAlvo + "/senha", `{}`},
		{"GET", "/usuarios/" + uidAlvo + "/historico", ""},
	} {
		cod, _ := f.chamar(t, caso.metodo, caso.caminho, caso.corpo, "")
		if cod != 401 {
			t.Fatalf("%s %s sem token devia dar 401, deu %d", caso.metodo, caso.caminho, cod)
		}
	}
	if len(f.gravados) != 0 {
		t.Fatalf("nada podia ter sido gravado: %v", f.gravados)
	}
}

func TestRotaDeHistoricoResponde(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "GET", "/usuarios/"+uidAlvo+"/historico", "", "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	linhas, ok := resp["historico"].([]any)
	if !ok || len(linhas) != 1 {
		t.Fatalf("esperava 1 linha de histórico: %v", resp)
	}
	if resp["por_pagina"].(float64) != 25 {
		t.Fatalf("paginação errada: %v", resp)
	}
}

func TestHistoricoDeLoginInexistenteDa404(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()
	f.alvoExiste = false

	cod, _ := f.chamar(t, "GET", "/usuarios/"+uidAlvo+"/historico", "", "bom")
	if cod != 404 {
		t.Fatalf("esperava 404, veio %d", cod)
	}
}

// ---------------------------------------------------------------------------
// minha conta
// ---------------------------------------------------------------------------

func TestTrocarPropriaSenhaExigeASenhaAtual(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "POST", "/minha-conta/senha",
		`{"senha_atual":"chutei","senha_nova":"novasenha1"}`, "bom")
	if cod != 401 {
		t.Fatalf("senha atual errada tinha que dar 401, deu %d: %v", cod, resp)
	}
	if len(f.gravados) != 0 {
		t.Fatalf("não podia ter gravado histórico: %v", f.gravados)
	}
}

func TestTrocarPropriaSenhaFunciona(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, resp := f.chamar(t, "POST", "/minha-conta/senha",
		`{"senha_atual":"certa123","senha_nova":"novasenha1"}`, "bom")
	if cod != 200 {
		t.Fatalf("esperava 200, veio %d: %v", cod, resp)
	}
	if len(f.gravados) != 1 {
		t.Fatalf("esperava 1 linha de histórico, vieram %d", len(f.gravados))
	}
	h := f.ultimo()
	if h["acao"] != "trocou_senha" || h["registro_id"] != uidBuilder || h["autor_id"] != uidBuilder {
		t.Fatalf("a linha devia ser trocou_senha do próprio: %v", h)
	}
	if _, tem := h["mudancas"]; tem {
		t.Fatalf("troca de senha não pode ter conteúdo: %v", h)
	}
	tudo, _ := json.Marshal(f.gravados)
	for _, proibido := range []string{"certa123", "novasenha1"} {
		if strings.Contains(string(tudo), proibido) {
			t.Fatalf("vazou %q no histórico: %s", proibido, tudo)
		}
	}
}

func TestSenhaNovaIgualAAtualERecusada(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, _ := f.chamar(t, "POST", "/minha-conta/senha",
		`{"senha_atual":"certa123","senha_nova":"certa123"}`, "bom")
	if cod != 400 {
		t.Fatalf("esperava 400, veio %d", cod)
	}
}

func TestMinhaContaSemTokenNaoEntra(t *testing.T) {
	f := novoFalso()
	defer f.srv.Close()

	cod, _ := f.chamar(t, "POST", "/minha-conta/senha", `{"senha_atual":"a","senha_nova":"b"}`, "")
	if cod != 401 {
		t.Fatalf("esperava 401, veio %d", cod)
	}
}
