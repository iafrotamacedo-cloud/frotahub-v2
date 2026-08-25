// rev 2 — a conversa com o Trílogo
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
	"errors"
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
	// OS NOMES DOS CAMPOS SÃO ESTES, E NÃO "email"/"password".
	//
	// Descoberto na marra: a primeira tentativa mandou os nomes óbvios e o Trílogo
	// respondeu 400 "Informe o email" — ou seja, leu o JSON e não achou o campo.
	// O nome verdadeiro estava no código do site: `UserEmail` / `UserPassword`.
	// O primeiro nome é o que a tela usa por dentro; estes são os que viajam.
	corpo, _ := json.Marshal(map[string]string{"UserEmail": email, "UserPassword": senha})
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
		// O Trílogo responde 400 tanto para pedido malformado quanto para senha
		// errada — o que muda é a FRASE. Sem ela no log, as duas falhas ficam
		// idênticas, e a pessoa fica trocando a senha certa achando que é ela.
		var m struct {
			Message string `json:"message"`
		}
		json.Unmarshal(bruto, &m)
		if m.Message != "" {
			return nil, fmt.Errorf("conta %s: o Trílogo recusou o login (%d): %s", conta, resp.StatusCode, m.Message)
		}
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

// ErroDoTrilogo é uma recusa DELES, com a frase deles dentro.
//
// POR QUE ISTO PRECISOU EXISTIR
//
//	Antes, uma recusa virava a string "…/CreateTicketCost devolveu 400" e o corpo
//	da resposta era descartado. Em 25/08/2026, testando contra o Trílogo de
//	verdade, os dois 400 que recebemos diziam:
//
//	  "Necessário anexar a nota fiscal referente ao número informado."
//	  "Está ação não é permitida para tickets com o status 'Aberto' ou 'Em execução'"
//
//	São duas causas completamente diferentes, com conserto diferente, e as duas
//	chegavam à tela como o mesmo "devolveu 400". A frase é o diagnóstico.
//
// NÃO DECIDA LENDO A FRASE
//
//	A segunda mensagem MENTE por omissão: cita só 'Aberto' e 'Em execução', mas o
//	ticket 126211 estava 'Arquivado' e foi recusado do mesmo jeito. A frase serve
//	para a pessoa entender, não para o programa ramificar.
type ErroDoTrilogo struct {
	Caminho string
	Codigo  int
	Corpo   string
}

func (e *ErroDoTrilogo) Error() string {
	if e.Corpo == "" {
		return fmt.Sprintf("%s devolveu %d", e.Caminho, e.Codigo)
	}
	return fmt.Sprintf("%s devolveu %d: %s", e.Caminho, e.Codigo, e.Corpo)
}

// Recusa desembrulha o erro até achar uma recusa do Trílogo, se houver.
func Recusa(err error) (*ErroDoTrilogo, bool) {
	var e *ErroDoTrilogo
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// frasedeles tira a frase de dentro do JSON de erro. O Trílogo responde
// {"message":"..."}; quando não responder, fica o texto cru, cortado — porque um
// HTML de 40 kB de página de erro não ajuda ninguém dentro de um log.
func frasedeles(bruto []byte) string {
	bruto = bytes.TrimSpace(bruto)
	if len(bruto) == 0 {
		return ""
	}
	var env struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Title   string `json:"title"`
	}
	if json.Unmarshal(bruto, &env) == nil {
		for _, s := range []string{env.Message, env.Error, env.Title} {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	}
	s := strings.TrimSpace(string(bruto))
	if len(s) > 400 {
		s = s[:400] + "…"
	}
	return s
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

	// A ORDEM DESTAS DUAS CONFERÊNCIAS IMPORTA, E ELA ESTAVA TROCADA
	//
	//	Antes, o corpo vazio era conferido PRIMEIRO e devolvia nil — então um 400
	//	sem corpo passava como sucesso. É o irmão gêmeo da armadilha que já nos
	//	mordeu no robô, onde um 200 vazio virava chamado de número zero.
	//
	//	Agora o código vem antes. Vazio só é sucesso quando o Trílogo disse que
	//	deu certo.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return &ErroDoTrilogo{
			Caminho: caminho,
			Codigo:  resp.StatusCode,
			Corpo:   frasedeles(bruto),
		}
	}
	// 204 é resposta legítima e vazia em vários endereços do Trílogo.
	if resp.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(bruto)) == 0 {
		return nil
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
