// rev 4 — a conversa com o banco (Supabase / PostgREST)
//
// Um cliente só, com tempo-limite e erro que ESTOURA. Nada de gravar e seguir em
// frente sem saber se gravou: se o banco recusou, quem chamou fica sabendo.
package banco

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// Erro descreve uma resposta que o banco recusou.
type Erro struct {
	Status  int
	Caminho string
	Corpo   string
}

func (e *Erro) Error() string {
	return fmt.Sprintf("banco respondeu %d em %s: %s", e.Status, e.Caminho, e.Corpo)
}

// Cliente fala com o PostgREST do Supabase usando a chave de serviço.
type Cliente struct {
	base  string
	chave string
	http  *http.Client
}

func Novo(cfg *config.Config) *Cliente {
	return &Cliente{
		base:  cfg.Supabase.REST(),
		chave: cfg.Supabase.ChaveServico,
		http:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Cliente) requisicao(ctx context.Context, metodo, caminho string, corpo any, cabecalhos map[string]string) (*http.Response, error) {
	var leitor io.Reader
	if corpo != nil {
		b, err := json.Marshal(corpo)
		if err != nil {
			return nil, fmt.Errorf("não consegui montar o JSON de %s: %w", caminho, err)
		}
		leitor = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, metodo, c.base+"/"+strings.TrimLeft(caminho, "/"), leitor)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.chave)
	req.Header.Set("Authorization", "Bearer "+c.chave)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range cabecalhos {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}

func (c *Cliente) executar(ctx context.Context, metodo, caminho string, corpo any, cabecalhos map[string]string, destino any) error {
	resp, err := c.requisicao(ctx, metodo, caminho, corpo, cabecalhos)
	if err != nil {
		return fmt.Errorf("não consegui falar com o banco em %s: %w", caminho, err)
	}
	defer resp.Body.Close()

	bruto, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &Erro{Status: resp.StatusCode, Caminho: caminho, Corpo: strings.TrimSpace(string(bruto))}
	}
	if destino == nil || len(bytes.TrimSpace(bruto)) == 0 {
		return nil
	}
	if err := json.Unmarshal(bruto, destino); err != nil {
		return fmt.Errorf("resposta do banco em %s não é o que eu esperava: %w", caminho, err)
	}
	return nil
}

// Buscar faz um GET e joga o resultado em destino (que deve ser ponteiro para slice).
func (c *Cliente) Buscar(ctx context.Context, caminho string, destino any) error {
	return c.executar(ctx, http.MethodGet, caminho, nil, nil, destino)
}

// BuscarContando faz um GET e devolve TAMBÉM quantas linhas existem no total,
// ignorando o limite da página.
//
// POR QUE NÃO UM SEGUNDO "select count(*)"
//
//	Uma tela paginada precisa de duas respostas: as linhas desta página e o total
//	para escrever "1–100 de 1.377". A saída óbvia é fazer duas consultas — e são
//	duas viagens ao banco, com dois conjuntos de filtros que precisam ficar
//	iguais para sempre. Basta alguém corrigir o filtro de um lado e esquecer o
//	outro para a tela mentir a contagem.
//
//	O PostgREST resolve numa viagem só: pedindo `count=exact`, ele devolve o
//	total no cabeçalho Content-Range, no formato "0-99/1377". Um filtro, uma
//	consulta, duas respostas.
//
// Quando não vem nada, o cabeçalho é "*/0" e o total é zero.
func (c *Cliente) BuscarContando(ctx context.Context, caminho string, destino any) (int, error) {
	resp, err := c.requisicao(ctx, http.MethodGet, caminho, nil,
		map[string]string{"Prefer": "count=exact"})
	if err != nil {
		return 0, fmt.Errorf("não consegui falar com o banco em %s: %w", caminho, err)
	}
	defer resp.Body.Close()

	bruto, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return 0, &Erro{Status: resp.StatusCode, Caminho: caminho, Corpo: strings.TrimSpace(string(bruto))}
	}
	if destino != nil && len(bytes.TrimSpace(bruto)) > 0 {
		if err := json.Unmarshal(bruto, destino); err != nil {
			return 0, fmt.Errorf("resposta do banco em %s não é o que eu esperava: %w", caminho, err)
		}
	}
	return TotalDoIntervalo(resp.Header.Get("Content-Range")), nil
}

// TotalDoIntervalo lê o total de um cabeçalho Content-Range ("0-99/1377").
// Cabeçalho ausente, estranho ou com "*" depois da barra devolve zero — a tela
// mostra "0 registros", que é chato, mas nunca um número inventado.
func TotalDoIntervalo(cab string) int {
	barra := strings.LastIndex(cab, "/")
	if barra < 0 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(cab[barra+1:]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// Inserir grava linhas novas. Passe destino != nil para receber o que foi gravado.
func (c *Cliente) Inserir(ctx context.Context, tabela string, linhas any, destino any) error {
	cab := map[string]string{"Prefer": "return=minimal"}
	if destino != nil {
		cab["Prefer"] = "return=representation"
	}
	return c.executar(ctx, http.MethodPost, tabela, linhas, cab, destino)
}

// Gravar insere, e atualiza o que já existir com a mesma chave primária.
//
// Serve para conjuntos que se reescrevem inteiros — a matriz de permissões é o
// caso: marcar uma rotina já marcada não pode virar erro de chave duplicada.
func (c *Cliente) Gravar(ctx context.Context, tabela string, linhas any) error {
	return c.executar(ctx, http.MethodPost, tabela, linhas,
		map[string]string{"Prefer": "resolution=merge-duplicates,return=minimal"}, nil)
}

// Upsert grava e RESOLVE conflito: linha que já existe é atualizada.
//
// A chave do conflito vai no próprio caminho, do jeito do PostgREST:
//
//	Upsert(ctx, "chamados?on_conflict=cliente_id,numero", linhas, &fora)
//
// Passe destino != nil para receber de volta o que ficou gravado — é assim que o
// robô descobre o id interno de um chamado que ele acabou de gravar.
func (c *Cliente) Upsert(ctx context.Context, tabela string, linhas any, destino any) error {
	cab := map[string]string{"Prefer": "resolution=merge-duplicates,return=minimal"}
	if destino != nil {
		cab["Prefer"] = "resolution=merge-duplicates,return=representation"
	}
	return c.executar(ctx, http.MethodPost, tabela, linhas, cab, destino)
}

// InserirIgnorando insere e IGNORA em silêncio o que já existe.
//
// É o oposto do Upsert, e serve para o que não se reescreve: um evento de
// histórico já gravado não muda. Aqui, mandar de novo a timeline inteira é
// barato e seguro — o banco fica com a primeira versão e descarta o resto.
func (c *Cliente) InserirIgnorando(ctx context.Context, tabela string, linhas any) error {
	return c.executar(ctx, http.MethodPost, tabela, linhas,
		map[string]string{"Prefer": "resolution=ignore-duplicates,return=minimal"}, nil)
}

// Duplicado diz se o erro é de chave repetida — o que quase sempre significa que
// a pessoa tentou criar algo com um código que já existe, e merece uma frase
// específica em vez de "erro no banco".
func Duplicado(err error) bool {
	var e *Erro
	if !errors.As(err, &e) {
		return false
	}
	return e.Status == http.StatusConflict || strings.Contains(e.Corpo, "23505")
}

// Atualizar altera as linhas que casarem com o filtro (sintaxe do PostgREST).
func (c *Cliente) Atualizar(ctx context.Context, tabela, filtro string, campos any) error {
	return c.executar(ctx, http.MethodPatch, tabela+"?"+filtro, campos, map[string]string{"Prefer": "return=minimal"}, nil)
}

// Escapar prepara um valor para entrar num filtro do PostgREST.
func Escapar(v string) string { return url.QueryEscape(v) }
