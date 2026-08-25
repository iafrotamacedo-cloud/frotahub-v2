package trilogo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

const cliente = "cccccccc-cccc-cccc-cccc-cccccccccccc"

// ---------------------------------------------------------------------------
// Um Trílogo, um banco e um armazém de mentira
// ---------------------------------------------------------------------------

type mundo struct {
	mu sync.Mutex

	trilogo *httptest.Server
	postgre *httptest.Server
	arquivo *httptest.Server // o S3 público onde ficam fotos e PDFs
	r2      *httptest.Server

	// o que o robô mandou gravar, tabela por tabela
	gravado map[string][]map[string]any
	// o que subiu para o armazém: caminho -> bytes
	enviado map[string][]byte

	conteudo       map[string][]byte // url -> bytes do arquivo
	lista          []map[string]any
	detalhes       map[int]map[string]any
	custos         map[int][]map[string]any
	foraDoEscopo   []int
	arquivosJaTem  []string
	pendentes      []map[string]any
	ultimaConsulta string
	logins         []string
	// A marca d'água como o banco a guarda: escrita pelo PATCH, lida pelo GET.
	// Sem isto o dublê respondia "não tem marca" para sempre, e o laço infinito
	// do cursor passava despercebido.
	marcaDagua string
}

func (m *mundo) registrar(tabela string, linhas []map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gravado[tabela] = append(m.gravado[tabela], linhas...)
}

func (m *mundo) linhas(tabela string) []map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gravado[tabela]
}

func novoMundo(t *testing.T) *mundo {
	t.Helper()
	m := &mundo{
		gravado: map[string][]map[string]any{}, enviado: map[string][]byte{},
		conteudo: map[string][]byte{}, detalhes: map[int]map[string]any{}, custos: map[int][]map[string]any{},
	}

	// --- Trílogo -----------------------------------------------------------
	tri := http.NewServeMux()
	tri.HandleFunc("POST /api/Login/SignIn", func(w http.ResponseWriter, r *http.Request) {
		// Este Trílogo de mentira é EXIGENTE de propósito: ele só entende
		// UserEmail/UserPassword, como o de verdade. Se alguém trocar os nomes no
		// robô achando que "email/password" é mais bonito, os testes quebram na
		// hora — em vez de a descoberta acontecer de novo num log do Actions.
		var c struct {
			UserEmail    string `json:"UserEmail"`
			UserPassword string `json:"UserPassword"`
		}
		json.NewDecoder(r.Body).Decode(&c)
		if c.UserEmail == "" {
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"Informe o email"}`))
			return
		}
		if c.UserPassword != "certa" {
			// O Trílogo devolve 400, e não 401, também para senha errada.
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"Email ou senha incorretos"}`))
			return
		}
		m.mu.Lock()
		m.logins = append(m.logins, c.UserEmail)
		m.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"accessToken": "tok-" + c.UserEmail, "authenticated": true})
	})
	tri.HandleFunc("POST /api/Ticket/ListTicketsByUser", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok-") {
			w.WriteHeader(401)
			return
		}
		var c struct{ Offset, Limit int }
		json.NewDecoder(r.Body).Decode(&c)
		if c.Offset > 0 {
			json.NewEncoder(w).Encode(map[string]any{"tickets": []any{}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"tickets": m.lista})
	})
	tri.HandleFunc("GET /api/Ticket/GetTicketDetail", func(w http.ResponseWriter, r *http.Request) {
		id := inteiroDe(r.URL.Query().Get("id"))
		json.NewEncoder(w).Encode(m.detalhes[id])
	})
	tri.HandleFunc("GET /api/Ticket/GetTicketCosts/", func(w http.ResponseWriter, r *http.Request) {
		id := inteiroDe(r.URL.Query().Get("ticketId"))
		json.NewEncoder(w).Encode(m.custos[id])
	})
	m.trilogo = httptest.NewServer(tri)
	base = m.trilogo.URL
	t.Cleanup(func() { base = "https://web.api.trilogo.app" })

	// --- arquivos (S3 público) ---------------------------------------------
	m.arquivo = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := m.conteudo[m.arquivo.URL+r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", itoa(len(b)))
		if r.Method == http.MethodHead {
			return
		}
		w.Write(b)
	}))

	// --- PostgREST ---------------------------------------------------------
	m.postgre = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tabela := strings.TrimPrefix(r.URL.Path, "/rest/v1/")
		q := r.URL.RawQuery

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case tabela == "clientes":
				json.NewEncoder(w).Encode([]map[string]any{{"id": cliente, "slug": "frota-macedo"}})
			case tabela == "unidades" && strings.Contains(q, "no_escopo=is.false"):
				fora := []map[string]any{}
				for _, id := range m.foraDoEscopo {
					fora = append(fora, map[string]any{"id_trilogo": id})
				}
				json.NewEncoder(w).Encode(fora)
			case tabela == "robo_execucoes":
				m.mu.Lock()
				marca := m.marcaDagua
				m.mu.Unlock()
				if marca == "" || !strings.Contains(q, "marca_dagua") {
					json.NewEncoder(w).Encode([]map[string]any{})
					return
				}
				json.NewEncoder(w).Encode([]map[string]any{{"marca_dagua": marca}})
			case tabela == "arquivos":
				fora := []map[string]any{}
				for _, s := range m.arquivosJaTem {
					if strings.Contains(q, s) {
						fora = append(fora, map[string]any{"sha256": s})
					}
				}
				json.NewEncoder(w).Encode(fora)
			case tabela == "chamado_anexos":
				m.mu.Lock()
				m.ultimaConsulta = q
				m.mu.Unlock()
				json.NewEncoder(w).Encode(m.pendentes)
			default:
				json.NewEncoder(w).Encode([]map[string]any{})
			}
			return
		}

		if r.Method == http.MethodPatch {
			var campos map[string]any
			json.NewDecoder(r.Body).Decode(&campos)
			if marca, ok := campos["marca_dagua"].(string); ok && tabela == "robo_execucoes" {
				m.mu.Lock()
				m.marcaDagua = marca
				m.mu.Unlock()
			}
			campos["__filtro"] = q
			m.registrar("PATCH:"+tabela, []map[string]any{campos})
			w.WriteHeader(204)
			return
		}

		// POST = inserção / upsert
		var linhas []map[string]any
		bruto, _ := io.ReadAll(r.Body)
		json.Unmarshal(bruto, &linhas)

		// O PostgREST de verdade RECUSA o lote inteiro quando as linhas não têm
		// exatamente as mesmas chaves. Este de mentira faz igual — foi assim que
		// a primeira carga morreu, depois de ler 1.050 chamados: um chamado sem
		// ambiente tinha uma chave a menos que os outros.
		if chaves := chavesDiferentes(linhas); chaves != "" {
			w.WriteHeader(400)
			w.Write([]byte(`{"code":"PGRST102","message":"All object keys must match","details":"` + chaves + `"}`))
			return
		}
		m.registrar(tabela, linhas)

		fora := make([]map[string]any, 0, len(linhas))
		for i, l := range linhas {
			c := map[string]any{"id": tabela + "-" + itoa(i)}
			for _, k := range []string{"numero", "id_trilogo", "chamado_id"} {
				if v, ok := l[k]; ok {
					c[k] = v
				}
			}
			fora = append(fora, c)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(fora)
	}))

	// --- R2 ----------------------------------------------------------------
	m.r2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.enviado[r.URL.Path] = b
		m.mu.Unlock()
		if r.Header.Get("Authorization") == "" || r.Header.Get("x-amz-content-sha256") == "" {
			w.WriteHeader(403)
			return
		}
		w.WriteHeader(200)
	}))

	t.Cleanup(func() {
		m.trilogo.Close()
		m.postgre.Close()
		m.arquivo.Close()
		m.r2.Close()
	})
	return m
}

func (m *mundo) servico() *Servico {
	cfg := &config.Config{
		Supabase: config.Supabase{URL: m.postgre.URL, ChaveServico: "servico", ChavePublica: "publica"},
		R2:       config.R2{ContaID: "conta", Bucket: "frotahub", ChaveID: "id", ChaveSecreta: "segredo"},
		Trilogo: config.Trilogo{
			EmailInstalacoes: "jonas@x", SenhaInstalacoes: "certa",
			EmailCivil: "humberto@x", SenhaCivil: "certa",
			SoLink:   config.SoLinkPadrao, // a MESMA lista da produção
			Paralelo: 4, Lote: 150,
		},
		Runtime: config.Runtime{Ambiente: "local"},
	}
	bd := banco.Novo(cfg)
	arm := armazem.NovoEm(cfg, m.r2.URL)
	return Novo(cfg, bd, arm)
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
func inteiroDe(s string) int {
	var n int
	json.Unmarshal([]byte(s), &n)
	return n
}

// chamado monta um detalhe de mentira já no formato do Trílogo.
func (m *mundo) chamado(numero, unidade int, criado string, anexos []map[string]any, eventos []map[string]any) {
	m.lista = append(m.lista, map[string]any{
		"id": numero, "dateOfLastChange": "2026-08-22T15:22:18.711",
		"date": 22, "month": 8, "year": 2026,
		"company": map[string]any{"id": unidade, "name": "LOJA X"},
	})
	if criado != "" {
		n := len(m.lista) - 1
		m.lista[n]["date"], m.lista[n]["month"], m.lista[n]["year"] = 15, 6, 2026
	}
	m.detalhes[numero] = map[string]any{
		"id": numero, "description": "Tomada com curto",
		"status": 5, "priority": 2, "type": 2, "nature": "Manual",
		"company":             map[string]any{"id": unidade, "name": "LOJA X", "city": "FORTALEZA", "state": "CE"},
		"serviceCompany":      map[string]any{"name": "FROTA - INSTALAÇÕES", "cnpj": "27363223000170"},
		"department":          map[string]any{"id": 639, "fullAddress": "LOJA X > Térreo > Açougue"},
		"buildingServiceType": map[string]any{"name": "Eletrica"},
		"creator":             map[string]any{"id": 32571, "name": "Operação - Loja 13"},
		"creationDateTime":    "2026-08-22T12:45:55.542",
		"dateOfLastChange":    "2026-08-22T15:22:18.711",
		"attachments":         anexos,
		"activity":            map[string]any{"histories": eventos},
	}
}

// chavesDiferentes devolve a descrição do problema, ou vazio se está tudo certo.
func chavesDiferentes(linhas []map[string]any) string {
	if len(linhas) < 2 {
		return ""
	}
	molde := map[string]bool{}
	for k := range linhas[0] {
		molde[k] = true
	}
	for i, l := range linhas[1:] {
		if len(l) != len(molde) {
			return "linha " + itoa(i+1) + " tem " + itoa(len(l)) + " chaves, a primeira tem " + itoa(len(molde))
		}
		for k := range l {
			if !molde[k] {
				return "linha " + itoa(i+1) + " tem a chave " + k + ", que a primeira não tem"
			}
		}
	}
	return ""
}
