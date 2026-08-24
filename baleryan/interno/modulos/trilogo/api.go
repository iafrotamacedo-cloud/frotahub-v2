// rev 1 — a conversa com o Trílogo
//
// COMO ISTO FOI DESCOBERTO
//
//	O Trílogo não publica documentação. O robô antigo, em Python, abria um Chrome
//	inteiro, fazia login como gente e ROUBAVA o token que o site usa nas próprias
//	chamadas — e mesmo assim só conhecia dois endereços.
//
//	Em 23/08/2026 os endereços foram mapeados no navegador, chamada por chamada.
//	O achado que mais pesa está logo abaixo: `Login/SignIn` devolve o token
//	direto. Sem navegador, sem Playwright, em milissegundos.
//
// O QUE SE APRENDEU E ESTÁ CODIFICADO AQUI
//   - Um chamado inteiro cabe numa chamada só (`GetTicketDetail` já traz os
//     anexos e a timeline dentro).
//   - A lista vem ordenada pelo número, do maior para o menor — ou seja, do mais
//     novo para o mais velho. É isso que permite parar de paginar ao passar da
//     data de corte.
//   - A API IGNORA filtros de data e de ordenação. Foram testados: StartDate,
//     InitialDate, DateFrom, SortField. Nenhum muda o resultado, e `OrderBy`
//     devolve erro 500. Não adianta tentar de novo.
//   - Os arquivos ficam num S3 público: baixar é um GET simples, sem token.
package trilogo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// O endereço da API. É variável, e não constante, por um motivo só: os testes
// apontam para um Trílogo de mentira. Nada no sistema muda isto em produção.
var base = "https://web.api.trilogo.app"

// Sessao é uma conta logada.
type Sessao struct {
	Conta string
	token string
	http  *http.Client
	mu    sync.Mutex
}

// Entrar troca e-mail e senha por um token.
func Entrar(ctx context.Context, conta, email, senha string) (*Sessao, error) {
	s := &Sessao{
		Conta: conta,
		// Um cliente por conta, com conexões reaproveitadas. Sem isto, cada uma
		// das milhares de chamadas abriria uma conexão TLS nova — que é o que
		// mais custa tempo numa varredura desse tamanho.
		http: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        64,
				MaxIdleConnsPerHost: 32,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	corpo, _ := json.Marshal(map[string]string{"email": email, "password": senha})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/Login/SignIn", bytes.NewReader(corpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("conta %s: não consegui falar com o Trílogo: %w", conta, err)
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("conta %s: o Trílogo recusou o login (%d)", conta, resp.StatusCode)
	}
	var fora struct {
		AccessToken   string `json:"accessToken"`
		Authenticated bool   `json:"authenticated"`
	}
	if err := json.Unmarshal(bruto, &fora); err != nil || fora.AccessToken == "" {
		return nil, fmt.Errorf("conta %s: entrei, mas não achei o token na resposta", conta)
	}
	s.token = fora.AccessToken
	return s, nil
}

func (s *Sessao) pedir(ctx context.Context, metodo, caminho string, corpo []byte, destino any) error {
	s.mu.Lock()
	token := s.token
	s.mu.Unlock()

	var leitor io.Reader
	if corpo != nil {
		leitor = bytes.NewReader(corpo)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, base+caminho, leitor)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", caminho, err)
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))

	// 204 é resposta legítima e vazia em vários endereços do Trílogo.
	if resp.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(bruto)) == 0 {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s devolveu %d", caminho, resp.StatusCode)
	}
	if destino == nil {
		return nil
	}
	if err := json.Unmarshal(bruto, destino); err != nil {
		return fmt.Errorf("%s: resposta não é o que eu esperava: %w", caminho, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Os três endereços que importam
// ---------------------------------------------------------------------------

// Todos os códigos de status. O Trílogo exige a lista; não existe "traga tudo".
const todosOsStatus = "1,2,3,4,5,6,7,8,9,10"

const porPagina = 50

// Listar percorre a lista até `parar` dizer que chega.
//
// `parar` recebe a última página lida e decide. É assim que a carga se limita a
// 01/07/2026 sem varrer o histórico inteiro: como a lista vem do mais novo para o
// mais velho, basta parar quando a página inteira ficou antes da data.
func (s *Sessao) Listar(ctx context.Context, parar func(pagina []Resumo) bool) ([]Resumo, error) {
	var tudo []Resumo
	for offset := 0; ; offset += porPagina {
		corpo, _ := json.Marshal(map[string]any{
			"StatusActions": todosOsStatus,
			"OnlyUnread":    false,
			"Offset":        offset,
			"Limit":         porPagina,
		})
		var env struct {
			Tickets []Resumo `json:"tickets"`
		}
		if err := s.pedir(ctx, http.MethodPost, "/api/Ticket/ListTicketsByUser", corpo, &env); err != nil {
			return tudo, err
		}
		if len(env.Tickets) == 0 {
			return tudo, nil
		}
		tudo = append(tudo, env.Tickets...)
		if len(env.Tickets) < porPagina {
			return tudo, nil
		}
		if parar != nil && parar(env.Tickets) {
			return tudo, nil
		}
		// Trava de segurança: se um dia a API passar a ignorar o Offset, isto
		// impede o robô de girar para sempre lendo a mesma página.
		if offset > 200000 {
			return tudo, fmt.Errorf("parei em %d chamados: a lista não estava avançando", len(tudo))
		}
	}
}

func (s *Sessao) Detalhe(ctx context.Context, numero int) (*Detalhe, error) {
	var d Detalhe
	if err := s.pedir(ctx, http.MethodGet, "/api/Ticket/GetTicketDetail?id="+strconv.Itoa(numero), nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Sessao) Custos(ctx context.Context, numero int) ([]Custo, error) {
	var c []Custo
	if err := s.pedir(ctx, http.MethodGet, "/api/Ticket/GetTicketCosts/?ticketId="+strconv.Itoa(numero), nil, &c); err != nil {
		return nil, err
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// Arquivos
// ---------------------------------------------------------------------------

// Medir descobre o tamanho de um arquivo SEM baixá-lo.
//
// É o que permite responder "quantos GB isto vai ocupar?" antes de gastar o
// primeiro byte de armazenamento. Os arquivos do Trílogo ficam num S3 público, e
// por isso não vai token nenhum aqui.
func Medir(ctx context.Context, cli *http.Client, url string) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("o arquivo respondeu %d", resp.StatusCode)
	}
	return resp.ContentLength, resp.Header.Get("Content-Type"), nil
}

// Baixar entrega o corpo do arquivo. Quem chama fecha.
func Baixar(ctx context.Context, cli *http.Client, url string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("o arquivo respondeu %d", resp.StatusCode)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// Extensao tira a extensão do nome do arquivo, em minúsculas e sem o ponto.
func Extensao(nome string) string {
	i := strings.LastIndex(nome, ".")
	if i < 0 || i == len(nome)-1 {
		return ""
	}
	e := strings.ToLower(nome[i+1:])
	if len(e) > 8 {
		return ""
	}
	return e
}
