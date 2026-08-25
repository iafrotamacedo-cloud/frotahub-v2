// rev 1 — lançar o custo no Trílogo, por API
//
// O CONTRATO DESTE ARQUIVO NÃO FOI ADIVINHADO
//
//	Foi levantado em 25/08/2026 capturando a rede da tela real: dois custos
//	inseridos no ticket 130998 e os dois excluídos em seguida, com o ticket
//	voltando ao estado original. Está escrito em docs/api-trilogo.md.
//
//	Isto importa porque a alternativa — deduzir o formato pelo que a tela parece
//	fazer — teria errado em pelo menos três pontos: o campo do upload chama
//	`files` e não `file`, o valor vai em `ProductCost` ou `ServiceCost` conforme
//	o tipo (e não só em `TotalValue`), e a exclusão precisa do ticket junto do
//	custo.
//
// SÃO DUAS CHAMADAS, NUNCA UMA
//
//	Primeiro o arquivo sobe e o Trílogo devolve `{filename, permalink}`. Só
//	então o custo é criado, carregando esse par. Tentar mandar o arquivo dentro
//	do custo não funciona — e é o erro que a leitura apressada do endpoint faria.
package trilogo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"
)

// O endereço dos arquivos é OUTRO host. Variável pelo mesmo motivo de `base`:
// os testes apontam para um Trílogo de mentira.
var baseUpload = "https://upload.api.trilogo.app"

// Os códigos de tipo de custo, conferidos contra a resposta de GetTicketCosts no
// ticket 130998: o custo de Mão de obra veio com type 2 e valor em serviceCost;
// o de Materiais, type 1 e valor em productCost.
const (
	TipoMateriais = 1
	TipoMaoDeObra = 2
)

// Subido é o que o Trílogo devolve quando um arquivo sobe. O nome evita
// confusão com `Anexo`, que neste pacote é a FOTO de um chamado — coisa
// diferente, em outra coleção.
type Subido struct {
	Nome      string `json:"filename"`
	Permalink string `json:"permalink"`
}

// SubirArquivo manda um arquivo e devolve o par que o custo vai precisar.
func (s *Sessao) SubirArquivo(ctx context.Context, nome string, conteudo []byte) (Subido, error) {
	var corpo bytes.Buffer
	form := multipart.NewWriter(&corpo)

	// O campo chama `files`, no plural. O endpoint de leitura de nota usa `file`,
	// no singular — dois endereços do mesmo host com nomes diferentes. Trocar um
	// pelo outro devolve 200 com resposta vazia, que é o pior tipo de falha.
	parte, err := form.CreateFormFile("files", nome)
	if err != nil {
		return Subido{}, err
	}
	if _, err := parte.Write(conteudo); err != nil {
		return Subido{}, err
	}
	if err := form.Close(); err != nil {
		return Subido{}, err
	}

	s.mu.Lock()
	token := s.token
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseUpload+"/upload", &corpo)
	if err != nil {
		return Subido{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", form.FormDataContentType())

	resp, err := s.http.Do(req)
	if err != nil {
		return Subido{}, fmt.Errorf("subindo %s: %w", nome, err)
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	if resp.StatusCode != http.StatusOK {
		return Subido{}, fmt.Errorf("o Trílogo recusou o arquivo %s (%d): %s",
			nome, resp.StatusCode, primeiraLinha(bruto))
	}

	// A resposta é uma LISTA, mesmo com um arquivo só.
	var fora []Subido
	if err := json.Unmarshal(bruto, &fora); err != nil || len(fora) == 0 {
		return Subido{}, fmt.Errorf("subi %s mas não achei o permalink na resposta", nome)
	}
	if fora[0].Permalink == "" {
		return Subido{}, fmt.Errorf("subi %s e o permalink veio vazio", nome)
	}
	return fora[0], nil
}

// NovoCusto é o corpo de CreateTicketCost, com os nomes que o Trílogo espera.
//
// Os campos são maiúsculos porque é assim que eles viajam — mesma história do
// `UserEmail` do login. Nome bonito aqui viraria 400 lá.
type NovoCusto struct {
	TicketId   int     `json:"TicketId"`
	Type       int     `json:"Type"`
	TotalValue float64 `json:"TotalValue"`
	// Um dos dois, conforme o tipo. O outro vai nulo — mandar os dois
	// preenchidos faz o Trílogo somar em relatório.
	ProductCost  *float64 `json:"ProductCost"`
	ServiceCost  *float64 `json:"ServiceCost"`
	ShippingCost float64  `json:"ShippingCost"`

	DocumentNumber string  `json:"DocumentNumber"`
	IssueDate      string  `json:"IssueDate"` // "2026-08-04", sem hora e sem fuso
	DueDate        *string `json:"DueDate"`
	Comment        string  `json:"Comment"`
	CompanyId      int     `json:"CompanyId"`

	UploadedFiles        []Subido `json:"UploadedFiles"`
	UploadedInvoiceFiles []Subido `json:"UploadedInvoiceFiles"`
}

// MontarCusto arma o corpo pondo o valor no campo certo do tipo.
//
// A DATA VAI COMO VEIO
//
//	No teste real a leitura devolveu 2026-08-04 e a TELA mandou 2026-08-03 — um
//	dia a menos, por conversão para fuso local. É o mesmo defeito das datas do
//	Excel (P-34). Aqui a data entra como texto "AAAA-MM-DD" e não passa por
//	time.Time em fuso nenhum: o que não converte não erra.
func MontarCusto(ticket, tipo, empresa int, valor float64, documento, emissao, observacao string, nota Subido) NovoCusto {
	c := NovoCusto{
		TicketId:       ticket,
		Type:           tipo,
		TotalValue:     valor,
		ShippingCost:   0,
		DocumentNumber: documento,
		IssueDate:      emissao,
		Comment:        observacao,
		CompanyId:      empresa,
		UploadedFiles:  []Subido{},
	}
	if nota.Permalink != "" {
		c.UploadedInvoiceFiles = []Subido{nota}
	} else {
		c.UploadedInvoiceFiles = []Subido{}
	}
	switch tipo {
	case TipoMaoDeObra:
		c.ServiceCost = &valor
	default:
		c.ProductCost = &valor
	}
	return c
}

// CriarCusto lança. Devolve o id do custo criado.
//
// O TRÍLOGO NÃO CONFERE TETO NENHUM
//
//	Provado no levantamento: o ticket 130998 foi a R$ 2.442,26 sem um aviso. A
//	regra dos R$ 600 é inteiramente nossa, e por isso ela é conferida ANTES
//	desta chamada, em `regras.AplicarTeto`. Se falhar aqui, falhou de vez.
func (s *Sessao) CriarCusto(ctx context.Context, c NovoCusto) (int, error) {
	corpo, err := json.Marshal(c)
	if err != nil {
		return 0, err
	}
	antes, err := s.Custos(ctx, c.TicketId)
	if err != nil {
		return 0, fmt.Errorf("ticket %d: não consegui ler os custos antes de lançar: %w", c.TicketId, err)
	}
	jaExistia := map[int]bool{}
	for _, x := range antes {
		jaExistia[x.ID] = true
	}

	if err := s.pedir(ctx, http.MethodPost, "/api/Ticket/CreateTicketCost", corpo, nil); err != nil {
		return 0, fmt.Errorf("ticket %d: %w", c.TicketId, err)
	}

	// O POST não devolve o id do custo criado. Ele sai da releitura, comparando
	// com o que existia antes — que é a mesma releitura que confirma que gravou.
	// Lançar e não conferir é como o sistema antigo perdia orçamento em silêncio.
	depois, err := s.Custos(ctx, c.TicketId)
	if err != nil {
		return 0, fmt.Errorf("ticket %d: lancei, mas não consegui confirmar: %w", c.TicketId, err)
	}
	for _, x := range depois {
		if !jaExistia[x.ID] {
			return x.ID, nil
		}
	}
	return 0, fmt.Errorf("ticket %d: o Trílogo aceitou o lançamento mas o custo não apareceu na lista", c.TicketId)
}

// ExcluirCusto remove um custo. Precisa dos DOIS números — o custo sozinho não
// basta, e essa é uma das coisas que só a captura revelou.
func (s *Sessao) ExcluirCusto(ctx context.Context, ticket, custo int) error {
	caminho := "/api/Ticket/DeleteTicketCost?ticketId=" + strconv.Itoa(ticket) +
		"&ticketCostId=" + strconv.Itoa(custo)
	return s.pedir(ctx, http.MethodDelete, caminho, nil, nil)
}

// SomaDosCustos devolve quanto o ticket já tem lançado, de TODOS os tipos.
//
// De todos, e não só do nosso: um ticket com R$ 400 de Materiais antigo não
// pode aceitar mais R$ 600 de Mão de obra. Contar só o nosso tipo seria um teto
// que parece funcionar e vaza.
func SomaDosCustos(cs []Custo) float64 {
	var soma float64
	for _, c := range cs {
		switch {
		case c.TotalValue != nil:
			soma += *c.TotalValue
		case c.ServiceCost != nil:
			soma += *c.ServiceCost
		case c.ProductCost != nil:
			soma += *c.ProductCost
		}
	}
	return soma
}

func primeiraLinha(b []byte) string {
	s := string(bytes.TrimSpace(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// EmissaoDoTrilogo formata uma data no formato que o CreateTicketCost espera.
// Recebe já no fuso certo; não converte nada.
func EmissaoDoTrilogo(d time.Time) string { return d.Format("2006-01-02") }
