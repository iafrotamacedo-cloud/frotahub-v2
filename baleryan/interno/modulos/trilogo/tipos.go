// rev 2 — o que o Trílogo devolve, e o que a gente entende disso
//
// SÓ ESTÃO AQUI OS CAMPOS QUE USAMOS. A lista tem 99 campos e o detalhe tem 81;
// declarar todos seria carregar peso que ninguém lê. O que faltar depois se
// acrescenta — reler os 1.430 chamados leva dois minutos.
//
// SOBRE OS CÓDIGOS
//
//	Todo código do Trílogo é guardado CRU no banco, ao lado do rótulo traduzido.
//	O motivo é concreto: o robô antigo grava a prioridade TROCADA (diz que 1 é
//	Baixa quando 1 é Média), e dois arquivos dele discordam entre si sobre o
//	significado do status 5. Com o número guardado, um erro de tradução se
//	corrige com um UPDATE em vez de uma releitura inteira.
//
//	As tabelas abaixo foram conferidas código contra tela, no Trílogo, em
//	23/08/2026.
package trilogo

import (
	"fmt"
	"strings"
	"time"
)

// Resumo é a linha da lista.
type Resumo struct {
	ID               int    `json:"id"`
	Description      string `json:"description"`
	Type             int    `json:"type"`
	Status           int    `json:"status"`
	Priority         int    `json:"priority"`
	CompanyName      string `json:"companyName"`
	DateOfLastChange string `json:"dateOfLastChange"`
	Date             int    `json:"date"`
	Month            int    `json:"month"`
	Year             int    `json:"year"`
	Company          struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"company"`
}

// Criacao devolve a data em que o chamado nasceu.
func (r Resumo) Criacao() time.Time {
	if r.Year == 0 {
		return time.Time{}
	}
	return time.Date(r.Year, time.Month(max(1, r.Month)), max(1, r.Date), 0, 0, 0, 0, time.UTC)
}

// Detalhe é o chamado inteiro.
type Detalhe struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Type        int    `json:"type"`
	Status      int    `json:"status"`
	Priority    int    `json:"priority"`
	Nature      string `json:"nature"`
	Tags        []any  `json:"tags"`

	Company struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		City    string `json:"city"`
		State   string `json:"state"`
		Address string `json:"address"`
	} `json:"company"`

	ServiceCompany struct {
		Name string `json:"name"`
		CNPJ string `json:"cnpj"`
	} `json:"serviceCompany"`

	// O AMBIENTE do chamado. O Trílogo chama de "department", e o `fullAddress`
	// já vem com o caminho montado: "LOJA 13 > Área interna > Térreo > ...".
	Department *struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		FullAddress string `json:"fullAddress"`
	} `json:"department"`

	BuildingServiceType *struct {
		Name string `json:"name"`
	} `json:"buildingServiceType"`

	Creator *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"creator"`

	ServiceCompanyAssignee *struct {
		Name string `json:"name"`
	} `json:"serviceCompanyAssignee"`

	CreationDateTime     string `json:"creationDateTime"`
	DeadlineDate         string `json:"deadlineDate"`
	DateOfLastChange     string `json:"dateOfLastChange"`
	DateOfLastExecution  string `json:"dateOfLastExecution"`
	DateOfLastInspection string `json:"dateOfLastInspection"`
	DateOfLastConclusion string `json:"dateOfLastConclusion"`

	Attachments []Anexo `json:"attachments"`

	Activity struct {
		Histories []Evento `json:"histories"`
	} `json:"activity"`
}

// Anexo é uma foto, um vídeo, um áudio — qualquer arquivo solto no chamado.
type Anexo struct {
	ID               int    `json:"id"`
	FileName         string `json:"fileName"`
	Image            string `json:"image"`
	Author           string `json:"author"`
	AuthorID         int    `json:"authorId"`
	CreationDateTime string `json:"creationDateTime"`
}

// Evento é uma linha da timeline.
//
// ATENÇÃO AO ID: um dos tipos vem com id = 0. Ver `Chave()`.
type Evento struct {
	ID           int    `json:"id"`
	RecordType   int    `json:"recordType"`
	StatusAction int    `json:"statusAction"`
	IsStatus     bool   `json:"isStatus"`
	CreationDate string `json:"creationDate"`
	AuthorName   string `json:"authorName"`
	AuthorID     int    `json:"authorId"`
	Description  string `json:"description"`
	Comment      string `json:"comment"`
	Observation  string `json:"observation"`
}

// Custo é um lançamento financeiro no chamado.
type Custo struct {
	ID             int      `json:"id"`
	Type           int      `json:"type"`
	TotalValue     *float64 `json:"totalValue"`
	ServiceCost    *float64 `json:"serviceCost"`
	ProductCost    *float64 `json:"productCost"`
	DocumentNumber string   `json:"documentNumber"`
	IssueDate      string   `json:"issueDate"`
	Company        struct {
		// O id é o que o lançamento precisa mandar de volta em CompanyId. Levantado
		// em 25/08/2026: 35 = FROTA - INSTALAÇÕES.
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"company"`
	// É AQUI que moram os nossos orçamentos — não junto das fotos.
	InvoiceFiles []struct {
		ID        int    `json:"id"`
		FileName  string `json:"fileName"`
		Permalink string `json:"permalink"`
	} `json:"invoiceFiles"`
}

// ---------------------------------------------------------------------------
// Traduções — conferidas código contra tela em 23/08/2026
// ---------------------------------------------------------------------------

var rotuloStatus = map[int]string{
	1: "Aberto",
	3: "Arquivado",
	5: "Executado",
	6: "Vistoriado",
	7: "Em execução",
}

// CUIDADO: o robô antigo tem estes dois TROCADOS. Conferido em seis chamados:
// o chamado que a tela mostra como "Baixa" vem com priority = 2.
var rotuloPrioridade = map[int]string{
	1: "Média",
	2: "Baixa",
	3: "Alta",
	4: "Urgente",
}

var rotuloTipo = map[int]string{
	2: "Predial",
}

// Os tipos de evento da timeline, como aparecem na tela do Trílogo.
// Os tipos abaixo foram batizados olhando o TEXTO de cada um deles nos 17 mil
// eventos da carga inicial. Os que continuam sem nome aqui aparecem como
// "codigo N" — visíveis, para alguém mapear depois. Sumir em silêncio é como o
// erro de prioridade sobreviveu tanto tempo no sistema antigo.
var rotuloEvento = map[int]string{
	0:   "status",
	2:   "status",
	3:   "anexo",
	4:   "status", // "Chamado finalizado"
	11:  "responsavel",
	12:  "prioridade",
	17:  "prazo",
	23:  "interrupcao", // "Interrupção de Execução"
	26:  "situacao",
	28:  "descricao",
	32:  "tag",
	35:  "status",       // "De Vistoriado Para Aberto"
	41:  "tipo",         // "De Tipo predial Para Procedimento"
	49:  "tipo_predial", // "De Mobilia para Eletrica"
	54:  "comentario",
	55:  "comentario",
	57:  "fornecedor",     // "Fornecedor(es) Adicionado(s): ..."
	58:  "relacionamento", // "Ticket X Relacionado Com Ticket Y"
	59:  "relacionamento",
	75:  "duplicacao", // "Ticket X duplicado a partir deste"
	78:  "prestadora",
	79:  "prestadora", // "Prestador removido: ..."
	102: "responsavel",
	103: "responsavel", // "Responsável removido: ..."
}

// Rotular traduz o código, e NUNCA inventa: código desconhecido volta como
// "codigo N", visível, para alguém perceber e mapear — em vez de virar um vazio
// silencioso no relatório.
func Rotular(mapa map[int]string, codigo int) string {
	if r, ok := mapa[codigo]; ok {
		return r
	}
	if codigo == 0 {
		return ""
	}
	return fmt.Sprintf("codigo %d", codigo)
}

func RotuloStatus(c int) string     { return Rotular(rotuloStatus, c) }
func RotuloPrioridade(c int) string { return Rotular(rotuloPrioridade, c) }
func RotuloTipo(c int) string       { return Rotular(rotuloTipo, c) }
func RotuloEvento(c int) string     { return Rotular(rotuloEvento, c) }

var rotuloCusto = map[int]string{
	1: "Mão de obra",
	2: "Materiais",
	3: "Mão de obra e materiais",
}

func RotuloCusto(c int) string { return Rotular(rotuloCusto, c) }

// ContaDe decide a QUAL das duas contas o chamado pertence.
//
// POR QUE NÃO É SIMPLESMENTE "QUEM LEU"
//
//	Trinta e oito chamados aparecem na lista das DUAS contas. Gravando quem leu,
//	quem lesse por último ganhava — e a carga inicial pôs 38 chamados da
//	Instalações dentro da Civil, só porque a Civil é lida depois.
//
//	Aqui quem manda é a empresa prestadora, que é o fato. A ordem da leitura
//	deixou de importar.
//
// O CASO QUE SOBRA: chamado atendido por uma empresa de fora (aparecem umas
// dezenas — desentupidora, refrigeração, estruturas metálicas). Esses ficam com a
// conta que os enxerga. Quem serve de verdade está sempre em `prestadora`.
func ContaDe(prestadora, contaQueLeu string) string {
	p := strings.ToUpper(prestadora)
	switch {
	case strings.Contains(p, "INSTALA"):
		return "instalacoes"
	case strings.Contains(p, "CIVIL"):
		return "civil"
	default:
		return contaQueLeu
	}
}

// Texto escolhe o que o evento tem de legível — os campos se alternam conforme
// o tipo, e nunca vêm os três juntos.
func (e Evento) Texto() string {
	for _, s := range []string{e.Description, e.Comment, e.Observation} {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return ""
}
