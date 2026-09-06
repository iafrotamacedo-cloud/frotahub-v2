package rogueworker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// chamarInterno bate no MESMO mux, com o MESMO Bearer. É o equivalente a
// clicar no botão: o handler original autentica, checa a rotina, aplica a
// regra e grava o histórico dele. Daqui não se escreve tabela de orçamento.
func (m *Modulo) chamarInterno(origem *http.Request, metodo, caminho string, corpo any) (int, []byte, error) {
	if m.mux == nil {
		return 0, nil, fmt.Errorf("mux interno não foi ligado")
	}
	var leitor io.Reader
	if corpo != nil {
		b, err := json.Marshal(corpo)
		if err != nil {
			return 0, nil, err
		}
		leitor = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(origem.Context(), metodo, "http://rw"+caminho, leitor)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", origem.Header.Get("Authorization"))
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	m.mux.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes(), nil
}

func (m *Modulo) jsonInterno(origem *http.Request, metodo, caminho string, corpo, dest any) (int, []byte, error) {
	status, bruto, err := m.chamarInterno(origem, metodo, caminho, corpo)
	if err != nil {
		return status, bruto, err
	}
	if dest != nil && len(bruto) > 0 {
		if err := json.Unmarshal(bruto, dest); err != nil {
			return status, bruto, fmt.Errorf("resposta interna não é o JSON esperado: %w", err)
		}
	}
	return status, bruto, nil
}

func (m *Modulo) erroInterno(status int, bruto []byte) string {
	var e struct {
		Erro string `json:"erro"`
	}
	if json.Unmarshal(bruto, &e) == nil && e.Erro != "" {
		return e.Erro
	}
	s := strings.TrimSpace(string(bruto))
	if s == "" {
		return fmt.Sprintf("o sistema respondeu %d", status)
	}
	if len(s) > 240 {
		s = s[:240] + "…"
	}
	return s
}

type paginaJSON struct {
	Linhas  []map[string]any `json:"linhas"`
	Total   int              `json:"total"`
	Paginas int              `json:"paginas"`
}

// paginar percorre a lista do handler original, página a página. O teto
// evita um laço infinito se a paginação vier quebrada.
func (m *Modulo) paginar(origem *http.Request, caminho string) ([]map[string]any, int, error) {
	var todas []map[string]any
	total := 0
	for pagina := 1; pagina <= 40; pagina++ {
		sep := "?"
		if strings.Contains(caminho, "?") {
			sep = "&"
		}
		var p paginaJSON
		status, bruto, err := m.jsonInterno(origem, http.MethodGet,
			caminho+sep+"pagina="+fmt.Sprint(pagina)+"&por=500", nil, &p)
		if err != nil {
			return todas, total, err
		}
		if status == http.StatusForbidden {
			return []map[string]any{}, 0, nil
		}
		if status != http.StatusOK {
			return todas, total, fmt.Errorf("%s", m.erroInterno(status, bruto))
		}
		if pagina == 1 {
			total = p.Total
		}
		todas = append(todas, p.Linhas...)
		if p.Paginas > 0 && pagina >= p.Paginas {
			break
		}
		if len(p.Linhas) == 0 {
			break
		}
	}
	if todas == nil {
		todas = []map[string]any{}
	}
	return todas, total, nil
}

func query(caminho string, pares ...string) string {
	u, err := url.Parse(caminho)
	if err != nil {
		return caminho
	}
	q := u.Query()
	for i := 0; i+1 < len(pares); i += 2 {
		q.Set(pares[i], pares[i+1])
	}
	u.RawQuery = q.Encode()
	return u.String()
}
