// rev 1 — a conversa com o banco (Supabase / PostgREST)
//
// Um cliente só, com tempo-limite e erro que ESTOURA. Nada de gravar e seguir em
// frente sem saber se gravou: se o banco recusou, quem chamou fica sabendo.
package banco

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// Inserir grava linhas novas. Passe destino != nil para receber o que foi gravado.
func (c *Cliente) Inserir(ctx context.Context, tabela string, linhas any, destino any) error {
	cab := map[string]string{"Prefer": "return=minimal"}
	if destino != nil {
		cab["Prefer"] = "return=representation"
	}
	return c.executar(ctx, http.MethodPost, tabela, linhas, cab, destino)
}

// Atualizar altera as linhas que casarem com o filtro (sintaxe do PostgREST).
func (c *Cliente) Atualizar(ctx context.Context, tabela, filtro string, campos any) error {
	return c.executar(ctx, http.MethodPatch, tabela+"?"+filtro, campos, map[string]string{"Prefer": "return=minimal"}, nil)
}

// Escapar prepara um valor para entrar num filtro do PostgREST.
func Escapar(v string) string { return url.QueryEscape(v) }
