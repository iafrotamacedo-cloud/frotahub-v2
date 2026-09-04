// rev 1 — responsável e cotação/orçamento de Serviço, por API
//
// O CONTRATO DESTE ARQUIVO NÃO FOI ADIVINHADO
//
//	Levantado em 04/09/2026 capturando a rede na tela real, ticket 135613
//	(LOJA 28 - ABOLIÇÃO, conta Frota Instalações):
//	  - o responsável foi trocado para "Serviço Instalações" e removido de
//	    volta, quatro vezes seguidas, sempre voltando ao estado original;
//	  - uma cotação foi criada, um orçamento com mão de obra + material +
//	    2 PDFs de teste (R$0,01 cada) foi inserido, conferido pela releitura
//	    (GetQuotationsTicketDetails) e excluído — o ticket voltou a R$0,00.
//	Está escrito em docs/api-trilogo-servicos.md.
//
// EM QUE ISTO DIFERE DO lancamento.go
//
//	lancamento.go grava direto em CreateTicketCost: é o ciclo de Materiais do
//	CONTRATO, sem aprovação visível ao cliente. Este arquivo é o ciclo de
//	SERVIÇO: Cotação (o pedido, sem preço) -> Orçamento (a resposta, com
//	preço e anexo) -> alguém marca um orçamento como vencedor NA TELA do
//	Trílogo. Essa aprovação é decisão do responsável MSLZ, de propósito, e
//	não foi automatizada aqui.
package trilogo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Responsável Executante
// ---------------------------------------------------------------------------

// ErrResponsavelNaoExiste é devolvido por MudarResponsavel quando o id
// pedido é zero — "não sei" (config sem o id configurado para aquela conta).
// Hoje Instalações e Civil já têm valor (ver config.Trilogo); o erro fica
// como rede de segurança para uma terceira conta que apareça sem
// configuração, em vez de mandar um id inválido pro Trílogo.
//
// POR QUE RECUSAR EM VEZ DE MANDAR ZERO PRO TRÍLOGO
//
//	Zero não é "sem responsável" lá — é só um id que não existe. O Trílogo
//	responderia com algum erro genérico, ou pior, aceitaria e associaria a
//	Deus-sabe-quem. Quem chama (a rota do botão "mandar para fila de
//	serviços") deve pegar este erro ANTES de tentar e mostrar "responsável
//	não existe".
var ErrResponsavelNaoExiste = errors.New("responsável não existe")

// MudarResponsavel troca o Responsável Executante do chamado. O id vem de
// config.Trilogo.ResponsavelServicoDaConta(conta).
func (s *Sessao) MudarResponsavel(ctx context.Context, ticketID, assigneeID int) error {
	if assigneeID == 0 {
		return ErrResponsavelNaoExiste
	}
	corpo, _ := json.Marshal(map[string]any{
		"serviceCompanyAssigneeId": assigneeID,
		"ticketId":                 ticketID,
	})
	return s.pedir(ctx, http.MethodPut, "/api/Ticket/ChangeServiceCompanyAssignee", corpo, nil)
}

// RemoverResponsavel tira o Responsável Executante do chamado — volta pro
// estado de antes de entrar na fila de Serviço (vazio).
func (s *Sessao) RemoverResponsavel(ctx context.Context, ticketID int) error {
	corpo, _ := json.Marshal(map[string]any{
		"serviceCompanyAssigneeId": nil,
		"ticketId":                 ticketID,
	})
	return s.pedir(ctx, http.MethodPut, "/api/Ticket/ChangeServiceCompanyAssignee", corpo, nil)
}

// RotuloResponsavelServico devolve o rótulo que o FRONT deve mostrar para um
// id de responsável — nunca o nome cru que vem do Trílogo.
//
// POR QUE ISTO PRECISA EXISTIR
//
//	Só o de Instalações se chama "Serviço Instalações" de verdade dentro do
//	Trílogo. O de Civil é uma PESSOA cadastrada como responsável — Romario
//	Santos, encarregado do setor — fazendo esse papel; não existe nenhum
//	"Serviço Civil" de verdade lá. Mostrar "Romario Santos" na nossa tela
//	confundiria quem opera, achando que é um técnico específico e não o
//	gatilho da fila de Serviço. Aqui o nome cru nunca escapa: se o id bate
//	com um dos dois configurados, devolve o rótulo nosso; senão, `ok=false` —
//	quem chama decide o que fazer (mostrar o nome cru é aceitável para
//	qualquer OUTRO responsável, que é gente de verdade mesmo).
func (s *Servico) RotuloResponsavelServico(id int) (rotulo string, ok bool) {
	tri := s.cfg.Trilogo
	switch id {
	case tri.ResponsavelServicoInstalacoes:
		return "Serviço Instalações", true
	case tri.ResponsavelServicoCivil:
		return "Serviço Civil", true
	default:
		return "", false
	}
}

// IDResponsavelServicoDaConta devolve o id configurado para a conta
// ("instalacoes" ou "civil") — o `cfg` de Servico não é exportado, então é
// por aqui que a rota do botão "mandar para fila de serviços" chega no id
// (e, com id zero, decide que o botão fica desabilitado com "responsável não
// existe", sem nem chamar MudarResponsavel).
func (s *Servico) IDResponsavelServicoDaConta(conta string) int {
	return s.cfg.Trilogo.ResponsavelServicoDaConta(conta)
}

// IDFornecedorDaConta devolve o supplierId configurado para a conta — a rota
// de criar orçamento usa para preencher NovoOrcamentoServico.SupplierID sem
// acessar o `cfg` interno.
func (s *Servico) IDFornecedorDaConta(conta string) int {
	return s.cfg.Trilogo.FornecedorDaConta(conta)
}

// SessaoDaConta entra no Trílogo com a conta pedida ("instalacoes" ou
// "civil") — mesma credencial que o robô usa em ler() e lerAlvos() (ver
// robo.go), só que para UMA conta, não as duas, porque a ação é sobre UM
// ticket que já se sabe de qual conta é.
func (s *Servico) SessaoDaConta(ctx context.Context, conta string) (*Sessao, error) {
	for _, c := range s.cfg.Trilogo.Contas() {
		if c.Nome == conta {
			return Entrar(ctx, c.Nome, c.Email, c.Senha)
		}
	}
	return nil, fmt.Errorf("conta %q não existe", conta)
}

// ---------------------------------------------------------------------------
// Cotação (o pedido) e Orçamento (a resposta)
// ---------------------------------------------------------------------------

// Cotacao é o container que recebe um ou mais Orçamentos como resposta.
//
// NÃO TEM EXCLUSÃO
//
//	Conferido na tela: o menu "..." de um Orçamento tem Excluir; a Cotação em
//	volta dele, não. Uma cotação criada por engano fica lá para sempre, vazia
//	— por isso CriarCotacao só deve ser chamada quando o orçamento vai ser
//	inserido de fato, não como rascunho.
type Cotacao struct {
	ID           int    `json:"id"`
	Description  string `json:"description"`
	CreationDate string `json:"creationDate"`

	// Status: 1 = aberta, sem orçamento nenhum ainda; 2 = em cotação, com
	// pelo menos um orçamento inserido. Outros códigos não foram vistos.
	Status          int      `json:"status"`
	WinningBudgetID *int     `json:"winningBudgetId"`
	HasBudget       bool     `json:"hasBudget"`
	ApprovedValue   *float64 `json:"approvedValue"`

	Budgets []Orcamento `json:"budgets"`
}

// Vencedor devolve o orçamento marcado como vencedor, se houver — é essa
// marcação (feita na tela pelo responsável MSLZ) que aprova o Serviço.
func (c Cotacao) Vencedor() *Orcamento {
	if c.WinningBudgetID == nil {
		return nil
	}
	for i := range c.Budgets {
		if c.Budgets[i].ID == *c.WinningBudgetID {
			return &c.Budgets[i]
		}
	}
	return nil
}

// Orcamento é a resposta à cotação — mão de obra, materiais e anexo, com
// preço. O Trílogo chama isto de "Budget" por dentro.
type Orcamento struct {
	ID            int      `json:"id"`
	CreationDate  string   `json:"creationDate"`
	DeadlineDate  *string  `json:"deadlineDate"`
	TotalValue    float64  `json:"totalValue"`
	ApprovedValue *float64 `json:"approvedValue"`
	IsWinning     bool     `json:"isWinning"`
	Supplier      struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"supplier"`
	Attachments []struct {
		FileName  string `json:"fileName"`
		Permalink string `json:"permalink"`
	} `json:"attachments"`
}

// Cotacoes lê todas as cotações do chamado, com os orçamentos de cada uma já
// dentro — uma chamada só, sem paginação (igual Detalhe).
func (s *Sessao) Cotacoes(ctx context.Context, ticketID int) ([]Cotacao, error) {
	var env struct {
		Quotations []Cotacao `json:"quotations"`
	}
	caminho := fmt.Sprintf("/api/Quotation/GetQuotationsTicketDetails?ticketId=%d", ticketID)
	if err := s.pedir(ctx, http.MethodGet, caminho, nil, &env); err != nil {
		return nil, err
	}
	return env.Quotations, nil
}

// CriarCotacao abre uma cotação vazia — sem itens de escopo, porque quem
// carrega mão de obra e materiais é o Orçamento (CriarOrcamentoServico), não
// a Cotação (ver MontarOrcamentoServico). Devolve o id, que o orçamento
// precisa em QuotationId.
func (s *Sessao) CriarCotacao(ctx context.Context, ticketID int, descricao string) (int, error) {
	corpo, err := json.Marshal(map[string]any{
		"ticketId":                   ticketID,
		"description":                descricao,
		"comment":                    "",
		"isAdditionalItemsAllowed":   true,
		"isBudgetAttachmentRequired": false,
		"showTicketAttachments":      true,
		"products":                   []any{},
		"services":                   []any{},
		"uploadedFiles":              []any{},
	})
	if err != nil {
		return 0, err
	}
	var fora struct {
		ID int `json:"id"`
	}
	if err := s.pedir(ctx, http.MethodPost, "/api/Quotation/CreateQuotation", corpo, &fora); err != nil {
		return 0, err
	}
	return fora.ID, nil
}

// ItemOrcamento é uma linha de mão de obra ou de material — os dois usam o
// mesmo formato de item; o que muda é a lista (Services / Products) dentro
// do corpo de CreateBudget.
type ItemOrcamento struct {
	Descricao string
	Valor     float64
	Qtd       float64
}

// itemBruto é o formato de item que o Trílogo espera dentro de products/
// services. MeasurementUnit (materiais) e TimeUnit (mão de obra) só foram
// testados com o valor-padrão da tela (1 = Unid. / Minutos) — o Trílogo tem
// outras opções na tela (Kg, Horas, ...) cujos códigos não foram levantados.
type itemBruto struct {
	IsAvailable     bool    `json:"isAvailable"`
	Description     string  `json:"description"`
	UnitPrice       float64 `json:"unitPrice"`
	Amount          float64 `json:"amount"`
	MeasurementUnit *int    `json:"measurementUnit,omitempty"`
	TimeUnit        *int    `json:"timeUnit,omitempty"`
}

var unidadePadrao = 1

// NovoOrcamentoServico é o corpo de CreateBudget.
type NovoOrcamentoServico struct {
	QuotationID  int     `json:"quotationId"`
	SupplierID   int     `json:"supplierId"`
	Contact      *string `json:"contact"`
	DeadlineDate *string `json:"deadlineDate"`
	Comment      *string `json:"comment"`
	Payment      struct {
		RateType       int      `json:"rateType"`
		Method         int      `json:"method"`
		Installments   int      `json:"installments"`
		RateValue      *float64 `json:"rateValue"`
		RatePercentage *float64 `json:"ratePercentage"`
	} `json:"payment"`
	Shipping struct {
		Deadline     int     `json:"deadline"`
		DeadlineUnit string  `json:"deadlineUnit"`
		Cost         float64 `json:"cost"`
	} `json:"shipping"`
	Products      []itemBruto `json:"products"`
	Services      []itemBruto `json:"services"`
	UploadedFiles []Subido    `json:"uploadedFiles"`
}

// MontarOrcamentoServico arma o corpo de CreateBudget com mão de obra E
// material JUNTOS no mesmo orçamento — é assim que a tela do Trílogo
// trabalha (um Orçamento, dois tipos de item dentro), e é o "orçamento único
// de MDO+material" que o dono pediu. Pagamento e frete saem no padrão da
// tela (boleto à vista, frete 1 dia) — não foram vistas outras opções ainda.
func MontarOrcamentoServico(quotationID, supplierID int, maoDeObra, materiais []ItemOrcamento, arquivos []Subido) NovoOrcamentoServico {
	var services, products []itemBruto
	for _, it := range maoDeObra {
		services = append(services, itemBruto{
			IsAvailable: true, Description: it.Descricao,
			UnitPrice: it.Valor, Amount: it.Qtd, TimeUnit: &unidadePadrao,
		})
	}
	for _, it := range materiais {
		products = append(products, itemBruto{
			IsAvailable: true, Description: it.Descricao,
			UnitPrice: it.Valor, Amount: it.Qtd, MeasurementUnit: &unidadePadrao,
		})
	}
	n := NovoOrcamentoServico{
		QuotationID: quotationID, SupplierID: supplierID,
		Products: products, Services: services, UploadedFiles: arquivos,
	}
	n.Payment.RateType = 1
	n.Payment.Method = 2
	n.Payment.Installments = 1
	n.Shipping.Deadline = 1
	n.Shipping.DeadlineUnit = "3"
	if n.Products == nil {
		n.Products = []itemBruto{}
	}
	if n.Services == nil {
		n.Services = []itemBruto{}
	}
	if n.UploadedFiles == nil {
		n.UploadedFiles = []Subido{}
	}
	return n
}

// CriarOrcamentoServico lança o orçamento e devolve o id.
//
// DIFERENTE DO CriarCusto: DEVOLVE O ID DIRETO
//
//	CreateTicketCost não devolve nada, e por isso CriarCusto precisa reler os
//	custos e comparar com o que existia antes. CreateBudget é mais gentil:
//	devolve `{"id": ..., "supplierId": ..., "contact": ...}` na hora — sem
//	releitura, sem diff.
func (s *Sessao) CriarOrcamentoServico(ctx context.Context, o NovoOrcamentoServico) (int, error) {
	corpo, err := json.Marshal(o)
	if err != nil {
		return 0, err
	}
	var fora struct {
		ID int `json:"id"`
	}
	if err := s.pedir(ctx, http.MethodPost, "/api/Budget/CreateBudget", corpo, &fora); err != nil {
		return 0, err
	}
	return fora.ID, nil
}

// ExcluirOrcamentoServico remove um orçamento. Só o orçamento — a cotação em
// volta dele não tem exclusão (ver comentário em Cotacao).
func (s *Sessao) ExcluirOrcamentoServico(ctx context.Context, orcamentoID int) error {
	caminho := fmt.Sprintf("/api/Budget/DeleteBudget/%d", orcamentoID)
	return s.pedir(ctx, http.MethodDelete, caminho, nil, nil)
}
