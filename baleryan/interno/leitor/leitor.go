// rev 1 — ler a nota fiscal
//
// TRÊS CAMADAS, NESTA ORDEM
//
//  1. XML da NFe        exato, de graça, sem chute. Onde existe, acaba a conversa.
//  2. texto + regex     o que dá para arrancar do texto sem inteligência nenhuma.
//  3. IA                estrutura o que sobrou.
//
// POR QUE NÃO EXISTE "LER O TEXTO DO PDF"
//
//	Existia no plano, e foi cortada depois de medir: as notas do São Luiz são
//	PDF ESCANEADO — imagem JPEG a 300 dpi, sem uma linha de texto. `pdftotext`
//	não extrai um caractere. Escrever um extrator de texto de PDF em Go puro
//	seria semanas de trabalho para uma camada que, nestes arquivos, devolveria
//	string vazia.
//
//	Então o texto da camada 2 vem do OCR, que roda no GitHub Actions — onde há
//	tempo, disco e liberdade para instalar o Tesseract. O motor continua sem
//	dependência externa nenhuma; ele só lê o resultado.
//
// A CONFIANÇA VIAJA JUNTO
//
//	Toda leitura diz de onde veio e quanto vale. Valor lido por IA que ninguém
//	conferiu não pode parecer valor digitado por gente — é a diferença entre um
//	erro que aparece na tela e um erro que vira orçamento errado no cliente.
package leitor

import (
	"regexp"
	"strconv"
	"strings"
)

// Camada diz quem leu.
type Camada string

const (
	DoXMLdaNota Camada = "xml"
	DoTextoCru  Camada = "texto"
	DoOCR       Camada = "ocr"
	DaIA        Camada = "ia"
	DaPessoa    Camada = "manual"
)

// Item é uma linha da nota.
type Item struct {
	Codigo     string  `json:"codigo,omitempty"`
	Descricao  string  `json:"descricao"`
	Unidade    string  `json:"unidade,omitempty"`
	Quantidade float64 `json:"quantidade"`
	Unitario   float64 `json:"valor_unitario"`
	Total      float64 `json:"valor_total"`
}

// Leitura é a nota depois de lida, seja lá por qual camada.
type Leitura struct {
	Tipo        string `json:"tipo"` // "nf" | "dav"
	Numero      string `json:"numero,omitempty"`
	Serie       string `json:"serie,omitempty"`
	ChaveAcesso string `json:"chave_acesso,omitempty"`
	DAV         string `json:"dav_numero,omitempty"`

	EmitenteCNPJ     string `json:"emitente_cnpj,omitempty"`
	EmitenteNome     string `json:"emitente_nome,omitempty"`
	DestinatarioCNPJ string `json:"destinatario_cnpj,omitempty"`

	// Texto "AAAA-MM-DD". Não é time.Time de propósito: no teste real a data
	// virou um dia a menos ao passar por fuso local (P-34). O que não converte
	// não erra.
	Emissao string `json:"emissao,omitempty"`

	ValorTotal float64 `json:"valor_total"`
	ValorFrete float64 `json:"valor_frete"`
	Observacao string  `json:"observacao,omitempty"`

	Itens []Item `json:"itens"`

	// ObservacaoDoCampo diz se a observação acima é o CAMPO do documento ou a
	// página inteira usada como último recurso.
	//
	// É ESTA MARCA QUE DECIDE SE O TICKET VALE
	//
	//	O fornecedor digita o ticket no campo "Observação" do sistema dele. Um
	//	número lido DALI é texto digitado, e o OCR acerta. Um número achado
	//	solto no meio da página pode ser qualquer coisa — inclusive rabisco à
	//	mão na margem, que é onde o OCR erra e erra em silêncio.
	//
	//	Amarrar um orçamento ao chamado errado não falha alto: gera, lança e
	//	cobra a loja errada. Por isso, sem o campo, a nota vai para "sem ticket"
	//	e uma pessoa digita o número.
	ObservacaoDoCampo bool `json:"observacao_do_campo,omitempty"`

	Camada    Camada  `json:"camada"`
	Confianca float64 `json:"confianca"`
}

// Completa diz se a leitura tem o mínimo para virar orçamento: um valor, e pelo
// menos um item. Sem itens não existe documento — existe um número solto.
func (l *Leitura) Completa() bool {
	return l != nil && l.ValorTotal > 0 && len(l.Itens) > 0
}

// ---------------------------------------------------------------------------
// os tickets escondidos nas observações
// ---------------------------------------------------------------------------

// Os tickets do São Luiz têm 5 ou 6 dígitos. Menos que isso é número de nota,
// código de produto ou CEP picado; mais, é chave de acesso partida.
var doTicket = regexp.MustCompile(`\b(\d{5,6})\b`)

// Estas palavras aparecem coladas em números que NÃO são ticket. Um número que
// vem logo depois de "DAV:" é um DAV, por mais que tenha seis dígitos.
var enganosas = regexp.MustCompile(`(?i)(dav|nosso\s*n[úu]mero|nf|nota|cep|pedido|ordem|boleto|c[óo]d)\s*[:.\-]?\s*$`)

// Tickets acha os números de ticket num texto livre — quase sempre as
// observações da nota, que é onde o fornecedor escreve.
//
// POR QUE NÃO É SÓ "ACHE OS NÚMEROS DE 6 DÍGITOS"
//
//	Porque a mesma observação traz "NOSSO NÚMERO: 45945" e "DAV: 92080", os dois
//	com o mesmo tamanho de um ticket. Sem olhar o que vem ANTES do número, a
//	leitura inventaria dois tickets que não existem — e um ticket inventado vira
//	orçamento lançado no chamado errado, que é pior do que não achar nenhum.
func Tickets(texto string) []int {
	var saida []int
	visto := map[int]bool{}

	for _, m := range doTicket.FindAllStringSubmatchIndex(texto, -1) {
		inicio, fim := m[2], m[3]
		antes := texto[:inicio]
		if enganosas.MatchString(antes) {
			continue
		}
		// Número grudado em outros dígitos por pontuação (uma chave de acesso
		// quebrada, um CNPJ) não é ticket.
		if inicio > 0 && ehDigito(texto[inicio-1]) {
			continue
		}
		if fim < len(texto) && ehDigito(texto[fim]) {
			continue
		}
		n, err := strconv.Atoi(texto[inicio:fim])
		if err != nil || n < 10000 || visto[n] {
			continue
		}
		visto[n] = true
		saida = append(saida, n)
	}
	return saida
}

func ehDigito(b byte) bool { return b >= '0' && b <= '9' }

// ---------------------------------------------------------------------------
// higiene de texto
// ---------------------------------------------------------------------------

var espacos = regexp.MustCompile(`\s+`)

// Enxugar deixa o texto numa linha só, sem espaço dobrado. Serve para gravar a
// observação sem os saltos de linha do OCR.
func Enxugar(s string) string {
	return strings.TrimSpace(espacos.ReplaceAllString(s, " "))
}

// SoDigitos devolve apenas os algarismos — para CNPJ e chave de acesso, que o
// OCR entrega pontuados de jeitos diferentes a cada leitura.
func SoDigitos(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if ehDigito(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// Decimal lê um número no formato brasileiro ("1.425,30") ou no americano
// ("1425.30"). O OCR devolve os dois, dependendo da nota.
func Decimal(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	temVirgula := strings.Contains(s, ",")
	temPonto := strings.Contains(s, ".")
	switch {
	case temVirgula && temPonto:
		// "1.425,30" — o ponto é milhar.
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	case temVirgula:
		s = strings.Replace(s, ",", ".", 1)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
