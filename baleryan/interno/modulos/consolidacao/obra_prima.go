// rev 1 — a nota do Obra Prima entra pelo CSV
//
// O QUE ESTE ARQUIVO FAZ
//
//	Lê o relatório "documentos a pagar" que o Obra Prima exporta e guarda cada
//	linha em `obra_prima_notas`. É o único lugar do sistema que grava nesta
//	tela — o resto de `consolidacao.go` continua de leitura só (ver o cabeçalho
//	daquele arquivo).
//
// O FORMATO DO ARQUIVO NÃO É GENTIL
//
//	Latin-1 (não UTF-8), separado por ";", com sete linhas de cabeçalho de
//	empresa antes da linha de verdade, número no formato brasileiro
//	("15.424,86") e data "dd/mm/aaaa". Nada disso é escolha nossa — é o que o
//	Obra Prima exporta — então o leitor é escrito para ESTE formato, e recusa
//	alto quando alguma coluna que ele precisa não está lá, em vez de adivinhar
//	uma posição.
//
// A GARANTIA QUE A VIEW (migração 046) CONFIA CEGAMENTE
//
//	`Bruto (R$)` é o valor do DOCUMENTO, repetido em cada parcela — não o valor
//	daquela parcela. Se duas linhas da mesma nota (`Núm.`) discordarem no
//	Bruto, a view não tem como saber qual delas está certa. Por isso
//	`parseObraPrima` recusa o ARQUIVO INTEIRO antes disso acontecer: ver
//	`conferirBrutoConsistente`.
package consolidacao

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TamanhoMaximoObraPrima é bem mais generoso que os ~30KB de um CSV real —
// existe só para não deixar alguém subir um arquivo do tamanho errado.
const TamanhoMaximoObraPrima = 5 << 20 // 5 MB

// Os nomes de coluna abaixo são os cabeçalhos que o Obra Prima usa, EXATAMENTE
// como vêm no arquivo (acento e ponto incluídos). Ler pelo NOME, e não pela
// posição, é o que sobrevive a uma coluna nova que o Obra Prima decida
// acrescentar um dia.
const (
	colDoc        = "Doc."
	colTipo       = "Tipo"
	colNum        = "Núm."
	colParc       = "Parc."
	colObra       = "Obra"
	colFornecedor = "Fornecedor"
	colVenc       = "Venc."
	colBruto      = "Bruto (R$)"
	colLiquido    = "Líquido (R$)"
	colDataPgto   = "Data Pgto."
	colValorPago  = "Vlr. pago (R$)"
	colSituacao   = "Situação"
	colDesc       = "Desc."
)

// colunasObrigatorias são as que o leitor não sobrevive sem. `Liquido`,
// `Venc.`, `Data Pgto.` e `Desc.` podem faltar ou vir vazias por linha; o
// resto não.
var colunasObrigatorias = []string{
	colDoc, colNum, colParc, colFornecedor, colBruto, colSituacao,
}

// linhaObraPrima é uma parcela já convertida — números em Dinheiro, datas em
// ISO, pronta para virar uma linha de `obra_prima_notas`.
type linhaObraPrima struct {
	Doc           string
	Tipo          string
	Num           string
	Parc          string
	Obra          string
	Fornecedor    string
	Vencimento    string // "aaaa-mm-dd", ou "" quando o CSV veio em branco
	Bruto         regras.Dinheiro
	Liquido       *regras.Dinheiro
	DataPagamento string
	ValorPago     *regras.Dinheiro
	Situacao      string
	Descricao     string
	linha         int // nº da linha no arquivo, só para as mensagens de erro
}

// parseObraPrima lê o CSV inteiro e devolve as linhas já convertidas, ou
// recusa com um erro que diz EXATAMENTE o que não deu para entender.
//
// TUDO OU NADA
//
//	Um arquivo de "documentos a pagar" é pequeno (a amostra real tem 157
//	linhas) e cada linha é uma nota que alguém vai cobrar ou não do cliente.
//	Importar 150 e descartar 7 em silêncio é o mesmo erro que o `carregar()`
//	de `consolidacao.go` já recusa fazer: a tela abriria bonita e mentindo por
//	omissão. Uma linha que não faz sentido derruba o arquivo inteiro, com o
//	número da linha e o motivo.
func parseObraPrima(bruto []byte) ([]linhaObraPrima, error) {
	texto := deLatin1(bruto)
	linhas := strings.Split(texto, "\n")

	idxCabecalho, cabecalho := acharCabecalho(linhas)
	if idxCabecalho < 0 {
		return nil, fmt.Errorf(
			"não achei a linha de cabeçalho (esperava uma linha com %q e %q)",
			colDoc, colNum)
	}
	posicao, err := mapearColunas(cabecalho)
	if err != nil {
		return nil, err
	}

	var saida []linhaObraPrima
	for i := idxCabecalho + 1; i < len(linhas); i++ {
		numeroDaLinha := i + 1 // 1-based, do jeito que um editor de planilha mostra
		crua := strings.TrimRight(linhas[i], "\r")
		campos := strings.Split(crua, ";")
		if todosVazios(campos) {
			continue // linha em branco no fim do arquivo — não é erro, é o CSV respirando
		}

		l, err := lerLinha(campos, posicao, numeroDaLinha)
		if err != nil {
			return nil, err
		}
		saida = append(saida, l)
	}

	if len(saida) == 0 {
		return nil, fmt.Errorf("o arquivo tem cabeçalho mas nenhuma linha de dados")
	}
	if err := conferirBrutoConsistente(saida); err != nil {
		return nil, err
	}
	return saida, nil
}

// deLatin1 converte byte a byte: Latin-1 (ISO-8859-1) é o único encoding em
// que isso é seguro, porque nele (e só nele) o valor do byte JÁ É o ponto de
// código Unicode. Foi conferido contra a amostra real (Documentos_a_pagar_7,
// 02/09/2026): sem isso "Núm." e "Situação" viram lixo, e a coluna nunca é
// encontrada pelo nome.
func deLatin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// acharCabecalho procura a linha que começa com "Empresa" e tem "Doc." e
// "Núm." em algum lugar — as sete linhas antes dela (nome da empresa,
// endereço, título do relatório) não têm formato fixo o bastante para contar
// posição, então a busca é por conteúdo, não por número de linha.
func acharCabecalho(linhas []string) (int, []string) {
	for i, l := range linhas {
		campos := strings.Split(strings.TrimRight(l, "\r"), ";")
		if len(campos) == 0 || strings.TrimSpace(campos[0]) != "Empresa" {
			continue
		}
		temDoc, temNum := false, false
		for _, c := range campos {
			switch strings.TrimSpace(c) {
			case colDoc:
				temDoc = true
			case colNum:
				temNum = true
			}
		}
		if temDoc && temNum {
			return i, campos
		}
	}
	return -1, nil
}

// mapearColunas transforma o cabeçalho em nome→posição, e recusa se faltar
// alguma coluna que o leitor não sobrevive sem.
func mapearColunas(cabecalho []string) (map[string]int, error) {
	posicao := make(map[string]int, len(cabecalho))
	for i, c := range cabecalho {
		nome := strings.TrimSpace(c)
		if nome != "" {
			posicao[nome] = i
		}
	}
	var faltando []string
	for _, col := range colunasObrigatorias {
		if _, ok := posicao[col]; !ok {
			faltando = append(faltando, col)
		}
	}
	if len(faltando) > 0 {
		return nil, fmt.Errorf(
			"o arquivo não tem a(s) coluna(s) %s — não é o relatório de \"documentos a pagar\" que eu conheço, ou o Obra Prima mudou o formato",
			strings.Join(faltando, ", "))
	}
	return posicao, nil
}

func todosVazios(campos []string) bool {
	for _, c := range campos {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// lerLinha converte uma linha de dados. Erros citam o número da linha (1-based,
// contando o cabeçalho de verdade e as linhas de empresa antes dele) porque é
// assim que a pessoa vai achar a linha, abrindo o CSV num editor de planilha.
func lerLinha(campos []string, pos map[string]int, numeroDaLinha int) (linhaObraPrima, error) {
	campo := func(nome string) string {
		i, ok := pos[nome]
		if !ok || i >= len(campos) {
			return ""
		}
		return strings.TrimSpace(campos[i])
	}

	l := linhaObraPrima{
		Doc:        campo(colDoc),
		Tipo:       campo(colTipo),
		Num:        limparNum(campo(colNum)),
		Parc:       campo(colParc),
		Obra:       campo(colObra),
		Fornecedor: campo(colFornecedor),
		Situacao:   campo(colSituacao),
		Descricao:  campo(colDesc),
		linha:      numeroDaLinha,
	}
	if l.Doc == "" {
		return l, fmt.Errorf("linha %d: sem %q — não dá para saber qual documento é este", numeroDaLinha, colDoc)
	}
	if l.Num == "" {
		return l, fmt.Errorf("linha %d (doc. %s): sem %q — sem o número da nota não dá para casar com ticket nenhum", numeroDaLinha, l.Doc, colNum)
	}
	if l.Parc == "" {
		l.Parc = "1/1"
	}
	if l.Fornecedor == "" {
		return l, fmt.Errorf("linha %d (nota %s): sem %q", numeroDaLinha, l.Num, colFornecedor)
	}

	bruto, err := dinheiroBR(campo(colBruto))
	if err != nil {
		return l, fmt.Errorf("linha %d (nota %s): %q não é um valor em reais válido (%q): %w",
			numeroDaLinha, l.Num, colBruto, campo(colBruto), err)
	}
	l.Bruto = bruto

	if txt := campo(colLiquido); txt != "" {
		v, err := dinheiroBR(txt)
		if err != nil {
			return l, fmt.Errorf("linha %d (nota %s): %q inválido (%q): %w", numeroDaLinha, l.Num, colLiquido, txt, err)
		}
		l.Liquido = &v
	}
	if txt := campo(colValorPago); txt != "" {
		v, err := dinheiroBR(txt)
		if err != nil {
			return l, fmt.Errorf("linha %d (nota %s): %q inválido (%q): %w", numeroDaLinha, l.Num, colValorPago, txt, err)
		}
		l.ValorPago = &v
	}
	if txt := campo(colVenc); txt != "" {
		iso, err := dataBR(txt)
		if err != nil {
			return l, fmt.Errorf("linha %d (nota %s): %q inválida (%q): %w", numeroDaLinha, l.Num, colVenc, txt, err)
		}
		l.Vencimento = iso
	}
	if txt := campo(colDataPgto); txt != "" {
		iso, err := dataBR(txt)
		if err != nil {
			return l, fmt.Errorf("linha %d (nota %s): %q inválida (%q): %w", numeroDaLinha, l.Num, colDataPgto, txt, err)
		}
		l.DataPagamento = iso
	}
	return l, nil
}

// limparNum tira um apóstrofo do FINAL do número da nota — o Obra Prima às
// vezes exporta "9214'" em vez de "9214" (confirmado pelo dono em 03/09/2026:
// é a mesma nota 9214 da associação manual, o apóstrofo é só defeito da
// exportação). Só o FINAL é limpo, e só apóstrofo/aspa simples: um apóstrofo
// no meio do número seria outra coisa, não um defeito de formatação, e essa
// função não tem como saber a diferença — por isso não mexe ali.
func limparNum(num string) string {
	return strings.TrimRight(num, "'’")
}

// dinheiroBR lê "15.424,86" (ponto de milhar, vírgula decimal — o formato
// brasileiro que o Obra Prima exporta) e devolve em centavos (P-12: dinheiro
// nunca em float solto). "0,00" e "" chegam aqui como zero — quem chama decide
// se isso vira nil ou fica.
func dinheiroBR(texto string) (regras.Dinheiro, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return 0, fmt.Errorf("valor vazio")
	}
	semMilhar := strings.ReplaceAll(texto, ".", "")
	comPonto := strings.ReplaceAll(semMilhar, ",", ".")
	v, err := strconv.ParseFloat(comPonto, 64)
	if err != nil {
		return 0, err
	}
	return regras.DinheiroDe(v), nil
}

// dataBR lê "28/08/2026" e devolve "2026-08-28" — o formato que o Postgres
// aceita sem ambiguidade nenhuma.
func dataBR(texto string) (string, error) {
	t, err := time.Parse("02/01/2006", strings.TrimSpace(texto))
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}

// conferirBrutoConsistente é a garantia que a view da migração 046 confia
// cegamente: todas as parcelas da MESMA nota (`Núm.`) têm que trazer o MESMO
// `Bruto`. Divergir aqui quer dizer que "somar/pegar um valor por nota" deixou
// de ser uma pergunta com resposta óbvia — e a resposta errada, escolhida em
// silêncio, é dinheiro errado numa tela financeira.
func conferirBrutoConsistente(linhas []linhaObraPrima) error {
	porNota := map[string][]linhaObraPrima{}
	for _, l := range linhas {
		porNota[l.Num] = append(porNota[l.Num], l)
	}
	// Ordena as chaves só para o erro sair sempre igual (linhas de mapa não
	// têm ordem, e um teste que compara mensagem de erro não pode depender de
	// sorte).
	nums := make([]string, 0, len(porNota))
	for num := range porNota {
		nums = append(nums, num)
	}
	sort.Strings(nums)

	for _, num := range nums {
		grupo := porNota[num]
		if len(grupo) < 2 {
			continue
		}
		primeiro := grupo[0]
		for _, outra := range grupo[1:] {
			if outra.Bruto != primeiro.Bruto {
				return fmt.Errorf(
					"nota %s: a linha %d diz %s de bruto e a linha %d diz %s — "+
						"parcelas da mesma nota precisam concordar no valor do documento "+
						"(ver o cabeçalho da migração 046)",
					num, primeiro.linha, primeiro.Bruto.Reais(), outra.linha, outra.Bruto.Reais())
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// POST /consolidacao/obra-prima — o único ponto de escrita desta tela
// ---------------------------------------------------------------------------

// resultadoDaImportacao é o que a tela usa para escrever o recado de sucesso.
type resultadoDaImportacao struct {
	Arquivo string  `json:"arquivo"`
	Linhas  int     `json:"linhas"`
	Notas   int     `json:"notas"`
	Valor   float64 `json:"valor"`
}

// linhaGravada é o formato que vai para `obra_prima_notas` — nomes de coluna
// e nada além do que a tabela espera (a mesma disciplina do `select=` de
// `consolidacao.go`, mas do lado de escrever).
//
// NENHUM CAMPO TEM `omitempty` — DE PROPÓSITO
//
//	O `Upsert` manda um ARRAY de objetos numa chamada só ao PostgREST, e o
//	PostgREST exige que todo objeto do array tenha o MESMO conjunto de
//	chaves — é assim que ele decide as colunas do INSERT antes de olhar linha
//	por linha. `omitempty` faz o `encoding/json` do Go omitir a chave quando o
//	valor é zero (""), e duas notas do mesmo arquivo raramente têm as MESMAS
//	colunas em branco: uma tem `Tipo` vazio e a outra não, e a chave some de
//	uma mas não da outra. O resultado é exatamente o erro que apareceu em
//	produção em 03/09/2026 (RECUSA-2): `PGRST102 "All object keys must
//	match"` — a importação inteira falha, mesmo tendo lido o CSV certinho.
//
//	A correção não é só tirar o `omitempty`: `Tipo`, `Obra`, `Situacao` e
//	`Descricao` viram ponteiro (como `Vencimento`/`Liquido` já eram), porque
//	"campo vazio no CSV" continua precisando virar NULL no banco, não uma
//	string vazia gravada como se fosse dado de verdade — ver `textoOuNil`.
//	Ponteiro nil serializa como `"campo":null`: a CHAVE continua presente
//	(o que o PostgREST exige), só o VALOR é nulo (o que o dado exige).
type linhaGravada struct {
	ClienteID     string   `json:"cliente_id"`
	Doc           string   `json:"doc"`
	Tipo          *string  `json:"tipo"`
	Num           string   `json:"num"`
	Parc          string   `json:"parc"`
	Obra          *string  `json:"obra"`
	Fornecedor    string   `json:"fornecedor"`
	Vencimento    *string  `json:"vencimento"`
	Bruto         float64  `json:"bruto"`
	Liquido       *float64 `json:"liquido"`
	DataPagamento *string  `json:"data_pagamento"`
	ValorPago     *float64 `json:"valor_pago"`
	Situacao      *string  `json:"situacao"`
	Descricao     *string  `json:"descricao"`
	Arquivo       string   `json:"arquivo"`
}

func (m *Modulo) importarObraPrima(w http.ResponseWriter, r *http.Request) {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return
	}
	if err := m.perm.Exige(r.Context(), p, RotinaModulo); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return
	}

	// CAMPO "arquivos", NO PLURAL — DE PROPÓSITO
	//   É o mesmo nome que `enviarArquivos()` do motor (frontend) já usa para
	//   subir PDF de nota. Reusar o campo é reusar a função inteira do lado de
	//   lá, em vez de escrever uma segunda rotina de upload só para diferir no
	//   nome do campo. Esta rota aceita só o PRIMEIRO arquivo — mais de um CSV
	//   por vez não faz sentido aqui.
	if err := r.ParseMultipartForm(TamanhoMaximoObraPrima); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo enviado.")
		return
	}
	recebidos := r.MultipartForm.File["arquivos"]
	if len(recebidos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escolha o arquivo CSV do Obra Prima.")
		return
	}
	cabecalho := recebidos[0]
	arquivo, err := cabecalho.Open()
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui abrir o arquivo enviado.")
		return
	}
	defer arquivo.Close()

	conteudo, err := io.ReadAll(io.LimitReader(arquivo, TamanhoMaximoObraPrima+1))
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o conteúdo do arquivo.")
		return
	}
	if len(conteudo) > TamanhoMaximoObraPrima {
		web.Falhar(w, http.StatusBadRequest,
			fmt.Sprintf("o arquivo passa de %d MB — não parece o relatório de documentos a pagar",
				TamanhoMaximoObraPrima>>20))
		return
	}

	linhas, err := parseObraPrima(conteudo)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o CSV: "+err.Error())
		return
	}

	gravar := paraGravar(p.ClienteID, cabecalho.Filename, linhas)
	if err := m.bd.Upsert(r.Context(), "obra_prima_notas?on_conflict=cliente_id,doc,parc", gravar, nil); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui gravar as notas: "+err.Error())
		return
	}

	web.Responder(w, http.StatusOK, resumoDaImportacao(cabecalho.Filename, linhas))
}

// paraGravar converte o que o CSV trouxe no formato que `obra_prima_notas`
// espera. Função pura, sem banco nem HTTP — é o que os testes exercitam para
// conferir cada coluna sem precisar montar uma requisição inteira.
func paraGravar(clienteID, arquivo string, linhas []linhaObraPrima) []linhaGravada {
	gravar := make([]linhaGravada, 0, len(linhas))
	for _, l := range linhas {
		gravar = append(gravar, linhaGravada{
			ClienteID:     clienteID,
			Doc:           l.Doc,
			Tipo:          textoOuNil(l.Tipo),
			Num:           l.Num,
			Parc:          l.Parc,
			Obra:          textoOuNil(l.Obra),
			Fornecedor:    l.Fornecedor,
			Vencimento:    dataOuNil(l.Vencimento),
			Bruto:         l.Bruto.Float(),
			Liquido:       dinheiroOuNil(l.Liquido),
			DataPagamento: dataOuNil(l.DataPagamento),
			ValorPago:     dinheiroOuNil(l.ValorPago),
			Situacao:      textoOuNil(l.Situacao),
			Descricao:     textoOuNil(l.Descricao),
			Arquivo:       arquivo,
		})
	}
	return gravar
}

// resumoDaImportacao soma por NOTA (não por linha — a mesma razão do
// `max(bruto)` da view: uma nota de 3 parcelas não vale 3x o próprio valor).
func resumoDaImportacao(arquivo string, linhas []linhaObraPrima) resultadoDaImportacao {
	valorPorNota := map[string]float64{}
	for _, l := range linhas {
		valorPorNota[l.Num] = l.Bruto.Float()
	}
	var soma float64
	for _, v := range valorPorNota {
		soma += v
	}
	return resultadoDaImportacao{
		Arquivo: arquivo,
		Linhas:  len(linhas),
		Notas:   len(valorPorNota),
		Valor:   soma,
	}
}

// ---------------------------------------------------------------------------
// POST /consolidacao/notas/intrusa — marca ou desmarca (migração 047)
// ---------------------------------------------------------------------------
//
// "Intrusa" é a nota que aparece no CSV do Obra Prima mas não é gasto de
// manutenção (o exemplo real: exames médicos) — não deveria contar na
// consolidação. Pedido do dono, 03/09/2026: um botão pequeno no fim da linha,
// "sempre que eu quiser" — ou seja, ALTERNA: chamar de novo desfaz. A marca
// mora em `obra_prima_nota_intrusa`, uma tabela à parte de `obra_prima_notas`
// (ver o cabeçalho da migração 047) — presença da linha é a marca inteira,
// não tem coluna de estado para ficar dessincronizada.

type pedidoIntrusa struct {
	NF string `json:"nf"`
}

type respostaIntrusa struct {
	NF      string `json:"nf"`
	Intrusa bool   `json:"intrusa"`
}

func (m *Modulo) alternarIntrusa(w http.ResponseWriter, r *http.Request) {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return
	}
	if err := m.perm.Exige(r.Context(), p, RotinaModulo); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return
	}

	var pedido pedidoIntrusa
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o pedido.")
		return
	}
	nf := strings.TrimSpace(pedido.NF)
	if nf == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe a NF da nota.")
		return
	}

	filtro := "cliente_id=eq." + banco.Escapar(p.ClienteID) + "&num=eq." + banco.Escapar(nf)

	// LÊ ANTES DE DECIDIR — NÃO ADIVINHA PELO QUE A TELA MANDOU
	//   A tela manda o estado que ELA acha que a nota tem, mas duas abas
	//   abertas ao mesmo tempo podem discordar. Conferir no banco, e não
	//   confiar num "estava marcada" que veio da requisição, é o que garante
	//   que dois cliques (de duas abas) alternam duas vezes e não uma.
	var existentes []struct {
		Num string `json:"num"`
	}
	if err := m.bd.Buscar(r.Context(), "obra_prima_nota_intrusa?"+filtro+"&select=num", &existentes); err != nil {
		log.Printf("consolidação: lendo obra_prima_nota_intrusa: %v", err)
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir a marca desta nota.")
		return
	}

	marcando := len(existentes) == 0
	if marcando {
		linha := map[string]any{
			"cliente_id":  p.ClienteID,
			"num":         nf,
			"marcado_por": p.UserID,
		}
		if err := m.bd.Inserir(r.Context(), "obra_prima_nota_intrusa", []map[string]any{linha}, nil); err != nil {
			log.Printf("consolidação: marcando %q como intrusa: %v", nf, err)
			web.Falhar(w, http.StatusInternalServerError, "Não consegui marcar a nota.")
			return
		}
	} else {
		if err := m.bd.Apagar(r.Context(), "obra_prima_nota_intrusa", filtro); err != nil {
			log.Printf("consolidação: desmarcando %q como intrusa: %v", nf, err)
			web.Falhar(w, http.StatusInternalServerError, "Não consegui desmarcar a nota.")
			return
		}
	}

	web.Responder(w, http.StatusOK, respostaIntrusa{NF: nf, Intrusa: marcando})
}

func dataOuNil(iso string) *string {
	if iso == "" {
		return nil
	}
	return &iso
}

// textoOuNil devolve nil quando o texto está vazio — mesma ideia de
// dataOuNil/dinheiroOuNil, mas para as colunas de texto opcionais (Tipo,
// Obra, Situação, Descrição): "não veio nesta linha do CSV" vira NULL na
// tabela, não uma string vazia gravada como se fosse dado de verdade. E, tão
// importante quanto o valor: NULL preserva a CHAVE no JSON (`"tipo":null`),
// que é o que o `Upsert` em lote precisa para todo objeto do array ter o
// mesmo conjunto de chaves — ver o comentário de `linhaGravada`.
func textoOuNil(texto string) *string {
	if texto == "" {
		return nil
	}
	return &texto
}

func dinheiroOuNil(d *regras.Dinheiro) *float64 {
	if d == nil {
		return nil
	}
	v := d.Float()
	return &v
}
