// rev 1 — extrair a lista para papel e para planilha
//
// POR QUE ESCRITO À MÃO
//
//	O motor não tem dependência externa nenhuma, e não vai ter por causa de um
//	relatório. As duas bibliotecas que resolveriam isto sozinhas trazem, juntas,
//	mais código que o FrotaHub inteiro — e uma delas passaria a decidir como o
//	nosso documento se parece.
//
//	Um `.xlsx` é um zip de XML: `archive/zip` e `encoding/xml` já vêm com o Go.
//	Um PDF é um arquivo de texto com uma tabela de posições no fim, e as suas
//	catorze fontes padrão dispensam embutir arquivo de fonte. Nenhum dos dois é
//	difícil; os dois são só detalhistas.
//
// UMA TABELA, DOIS FORMATOS
//
//	Quem chama monta a Tabela uma vez e pede o formato que quiser. A regra do que
//	entra no relatório mora em um lugar só (CORE-06) — se amanhã entrar uma
//	coluna, ela nasce na Tabela e aparece nos dois.
package relatorio

import "time"

type Tipo int

const (
	Texto Tipo = iota
	Numero
	Dinheiro
	Data
	DataHora
)

type Coluna struct {
	Titulo string
	// Peso relativo da coluna. No PDF vira largura proporcional à folha; na
	// planilha, largura em caracteres. Um número só para os dois: larguras
	// separadas viravam duas verdades sobre a mesma coluna.
	Peso float64
	Tipo Tipo
}

// Tabela é o relatório antes de virar arquivo.
type Tabela struct {
	Titulo string
	// Aba é o nome da guia dentro do .xlsx. Vazio = o título, cortado ao que o
	// Excel aceita.
	//
	// POR QUE ISTO EXISTE
	//	A guia vinha escrita "Chamados" em toda planilha do sistema, porque a
	//	primeira a existir era a do Trílogo. Numa planilha de orçamentos enviada
	//	AO CLIENTE, a guia dizendo "Chamados" é a primeira coisa que ele lê — e
	//	está errada.
	Aba       string
	Subtitulo string // o filtro aplicado, escrito para ser lido
	Colunas   []Coluna
	Linhas    [][]any // string, float64, int, time.Time ou nil
	// Aviso aparece no rodapé quando o resultado foi cortado. Corte silencioso é
	// pior que corte: quem lê acha que está vendo tudo.
	Aviso string
	// Gerado é carimbado no documento. Vem de fora para o teste poder fixá-lo.
	Gerado time.Time
}

func (t Tabela) valor(l, c int) any {
	if l < 0 || l >= len(t.Linhas) {
		return nil
	}
	linha := t.Linhas[l]
	if c < 0 || c >= len(linha) {
		return nil
	}
	return linha[c]
}
