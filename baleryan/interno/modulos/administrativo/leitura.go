// rev 1 — a leitura da Ordem de Compra: determinística, sem IA
//
// O PEDIDO DO DONO (10/09/2026)
//
//	Leitura determinística de PDF, sem IA; um botão "Ler"; dois filtros de
//	negócio (fornecedor precisa ter CNPJ; o CNPJ de faturamento — o
//	"comprador", na migração 059 — precisa começar com 03720882); quem passa
//	vai para "Processadas", quem falha vai para "Rejeitadas"; nenhuma OC pode
//	ficar presa na fila inicial depois de uma tentativa de leitura.
//
// POR QUE `-layout`, E NÃO O `pdftotext` CRU QUE `leitura.textoDePDF` JÁ USA
//
//	Aquele outro `pdftotext` (Notas/DAVs, camada 2 do OCR) lê texto solto —
//	não depende de coluna nenhuma. A OC do Obra Prima é uma TABELA: item,
//	quantidade, unitário, subtotal, desconto, total, cada um na própria
//	coluna. Sem `-layout` esse alinhamento se perde.
//
//	Testado com o poppler de verdade (o mesmo `apk add poppler-utils` do
//	Dockerfile) contra uma OC real de 2 páginas e 10 itens
//	(OrdemDeCompra_019731, 10/09/2026): `-layout` devolve cada linha de item
//	com os 5 números na ordem certa. `-table`, que existe no xpdf (outro
//	programa, não o que o Dockerfile instala), NÃO existe no poppler — e o
//	`pdftotext` do Windows, sem o poppler instalado à parte, É o xpdf. As duas
//	ferramentas têm nomes e flags iguais mas são programas diferentes; testar
//	com a errada quase produziu um leitor inteiro em cima de um sintoma que
//	não existe em produção.
//
// OS ITENS FICAM NUMA SEGUNDA PASSADA, DEPOIS DOS CAMPOS DO CABEÇALHO
//
//	`extrairCabecalho` sozinho já resolve os dois filtros (CNPJ do comprador,
//	nome+CNPJ do fornecedor) — eles não dependem de item nenhum. `extrairItens`
//	é generosa: um item que não bate o padrão esperado vira parte da descrição
//	do item anterior, em vez de derrubar a OC inteira. Dinheiro errado num
//	item é ruim; a OC inteira presa em "rejeitada" por causa de um item mal
//	formatado é pior, porque esconde os outros nove que estavam certos.
package administrativo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

// CNPJRaizPermitida é a raiz fixa do CNPJ de quem fatura direto — a mesma
// regra que a migração 059 já grava como `check` em `comprador_cnpj`. Escrita
// nos dois lados por necessidade: o banco valida na gravação, aqui se decide
// ANTES de gravar, para poder explicar o motivo da rejeição.
const CNPJRaizPermitida = "03720882"

// ErroDoServidor é o que aconteceu por CULPA DO SERVIDOR, não do PDF — hoje só
// "pdftotext não está instalado". `ler.go` usa isto para não marcar a OC como
// `falhou`: o documento pode estar perfeito, e tentar de novo depois de
// consertar o servidor é o único caminho que faz sentido (mesmo espírito de
// `leitura.FalhaTemporaria` em Orçamentos).
type ErroDoServidor struct{ Motivo string }

func (e ErroDoServidor) Error() string { return e.Motivo }

// ItemExtraido é uma linha da tabela da OC.
type ItemExtraido struct {
	Descricao string
	Qtd       float64
	Unidade   string
	ValorUnit regras.Dinheiro
	Desconto  regras.Dinheiro
	Total     regras.Dinheiro
}

// Extraida é tudo que a leitura consegue tirar do PDF — preenchido o quanto
// der, nunca fingindo que um campo ausente é zero.
type Extraida struct {
	Numero           string
	Data             string // ISO, "" quando não achou
	ObraCentroCusto  string
	CondPgto         string
	FormaPgto        string
	PrevisaoEntrega  string // ISO
	CompradorInterno string
	CompradorNome    string
	CompradorCNPJ    string
	FornecedorNome   string
	FornecedorCNPJ   string
	Subtotal         regras.Dinheiro
	Desconto         regras.Dinheiro
	Frete            regras.Dinheiro
	Total            regras.Dinheiro
	Itens            []ItemExtraido
}

// MotivosDeRejeicao aplica os dois filtros do pedido do dono. Lista vazia
// quer dizer "passa" — nunca `nil` com significado escondido, sempre a lista
// mesma, para a tela mostrar exatamente o que faltou.
func (e Extraida) MotivosDeRejeicao() []string {
	var motivos []string
	if strings.TrimSpace(e.FornecedorNome) == "" || strings.TrimSpace(e.FornecedorCNPJ) == "" {
		motivos = append(motivos, "não achei o nome e o CNPJ do fornecedor nesta OC")
	}
	if !strings.HasPrefix(e.CompradorCNPJ, CNPJRaizPermitida) {
		if e.CompradorCNPJ == "" {
			motivos = append(motivos, "não achei o CNPJ de faturamento (\"DADOS DO FATURAMENTO\") nesta OC")
		} else {
			motivos = append(motivos, fmt.Sprintf(
				"o CNPJ de faturamento (%s) não começa com %s — esta OC não é para a Frota Macedo faturar",
				e.CompradorCNPJ, CNPJRaizPermitida))
		}
	}
	return motivos
}

// Ler roda o `pdftotext -layout` sobre os bytes do PDF e extrai a OC. Função
// pura quanto a banco e rede (CORE-06): recebe bytes, devolve dados — quem
// chama decide o que gravar.
func Ler(ctx context.Context, pdf []byte) (Extraida, error) {
	texto, err := textoDoPDF(ctx, pdf)
	if err != nil {
		return Extraida{}, err
	}
	return ExtrairDoTexto(texto)
}

// textoDoPDF grava o PDF numa pasta descartável e chama o `pdftotext` de
// verdade — mesmo desenho de `leitura.textoDePDF`, mais o `-layout` que a
// tabela da OC precisa.
func textoDoPDF(ctx context.Context, pdf []byte) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", ErroDoServidor{"o pdftotext não está instalado neste servidor (Dockerfile precisa de poppler-utils)"}
	}
	pasta, err := os.MkdirTemp("", "oc")
	if err != nil {
		return "", ErroDoServidor{"não consegui abrir uma pasta temporária: " + err.Error()}
	}
	defer os.RemoveAll(pasta)

	caminho := filepath.Join(pasta, "oc.pdf")
	if err := os.WriteFile(caminho, pdf, 0o600); err != nil {
		return "", ErroDoServidor{"não consegui gravar o PDF temporário: " + err.Error()}
	}

	fora, err := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", "-eol", "unix", caminho, "-").Output()
	if err != nil {
		return "", fmt.Errorf("não consegui ler este PDF (pode estar corrompido ou protegido): %w", err)
	}
	texto := string(fora)
	if len(strings.TrimSpace(texto)) < 100 {
		return "", errors.New("este PDF não tem texto suficiente para ler — parece ser uma imagem escaneada, e a OC precisa ser um PDF digital")
	}
	return texto, nil
}

// ---------------------------------------------------------------------------
// o cabeçalho — numero, datas, comprador, fornecedor, obra
// ---------------------------------------------------------------------------

var (
	reNumero          = regexp.MustCompile(`(?m)^DADOS DA ORDEM DE COMPRA\s+(\d+)`)
	reData            = regexp.MustCompile(`Data:\s*(\d{2}/\d{2}/\d{4})`)
	rePrevisao        = regexp.MustCompile(`Previsão da entrega:\s*(\d{2}/\d{2}/\d{4})`)
	reCondPgto        = regexp.MustCompile(`Cond\.\s*pgto\.:\s*(\S(?:.*?\S)?)\s{2,}`)
	reFormaPgto       = regexp.MustCompile(`(?m)Forma pgto\.:\s*(\S(?:.*\S)?)\s*$`)
	reComprador       = regexp.MustCompile(`(?m)Comprador:\s*(\S(?:.*\S)?)\s*$`)
	reObraCentroCusto = regexp.MustCompile(`(?m)^OBRA/CENTRO DE CUSTO:\s*(\S(?:.*?\S)?)\s{2,}`)
	reNomeDoBloco     = regexp.MustCompile(`(?m)^\s*Nome:\s*(\S(?:.*?\S)?)\s{2,}`)
	reCNPJDoBloco     = regexp.MustCompile(`CNPJ:\s*([\d./\-]+)`)
)

// ExtrairDoTexto é a parte 100% pura — sem processo externo, sem I/O — que os
// testes exercitam direto, sem precisar do `pdftotext` instalado na máquina
// que roda `go test`.
func ExtrairDoTexto(texto string) (Extraida, error) {
	if strings.TrimSpace(texto) == "" {
		return Extraida{}, errors.New("o texto do PDF veio vazio")
	}

	var e Extraida
	if m := reNumero.FindStringSubmatch(texto); m != nil {
		e.Numero = m[1]
	}
	if e.Numero == "" {
		return Extraida{}, errors.New("não achei o número da OC (\"DADOS DA ORDEM DE COMPRA\") — este PDF parece não ser uma Ordem de Compra do Obra Prima")
	}
	if m := reData.FindStringSubmatch(texto); m != nil {
		e.Data = dataBR(m[1])
	}
	if m := rePrevisao.FindStringSubmatch(texto); m != nil {
		e.PrevisaoEntrega = dataBR(m[1])
	}
	if m := reCondPgto.FindStringSubmatch(texto); m != nil {
		e.CondPgto = strings.TrimSpace(m[1])
	}
	if m := reFormaPgto.FindStringSubmatch(texto); m != nil {
		e.FormaPgto = strings.TrimSpace(m[1])
	}
	if m := reComprador.FindStringSubmatch(texto); m != nil {
		e.CompradorInterno = strings.TrimSpace(m[1])
	}
	if m := reObraCentroCusto.FindStringSubmatch(texto); m != nil {
		e.ObraCentroCusto = strings.TrimSpace(m[1])
	}

	// FATURAMENTO E FORNECEDOR SÃO LIDOS DO BLOCO, NÃO DO TEXTO INTEIRO
	//
	//	"Nome:" e "CNPJ:" aparecem TRÊS vezes no documento (RESPONSÁVEL PELA
	//	COMPRA, DADOS DO FATURAMENTO, DADOS DO FORNECEDOR). Um regex solto no
	//	texto inteiro pegaria sempre a primeira ocorrência — errado nas duas
	//	vezes que importam. Recortar o bloco antes é o que garante que "Nome:"
	//	respondido é o do bloco certo.
	blocoFaturamento := blocoEntre(texto, "DADOS DO FATURAMENTO", "DADOS DO FORNECEDOR")
	if m := reNomeDoBloco.FindStringSubmatch(blocoFaturamento); m != nil {
		e.CompradorNome = strings.TrimSpace(m[1])
	}
	if m := reCNPJDoBloco.FindStringSubmatch(blocoFaturamento); m != nil {
		e.CompradorCNPJ = soDigitos(m[1])
	}

	blocoFornecedor := blocoEntre(texto, "DADOS DO FORNECEDOR", "OBRA/CENTRO DE CUSTO")
	if m := reNomeDoBloco.FindStringSubmatch(blocoFornecedor); m != nil {
		e.FornecedorNome = strings.TrimSpace(m[1])
	}
	if m := reCNPJDoBloco.FindStringSubmatch(blocoFornecedor); m != nil {
		e.FornecedorCNPJ = soDigitos(m[1])
	}

	e.Itens = extrairItens(texto)
	extrairTotais(texto, &e)

	return e, nil
}

// blocoEntre devolve o texto entre dois marcadores, sem incluir nenhum dos
// dois. `fim == ""` ou não encontrado devolve até o final do texto.
func blocoEntre(texto, inicio, fim string) string {
	i := strings.Index(texto, inicio)
	if i < 0 {
		return ""
	}
	resto := texto[i+len(inicio):]
	if fim == "" {
		return resto
	}
	j := strings.Index(resto, fim)
	if j < 0 {
		return resto
	}
	return resto[:j]
}

func soDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// os itens — N. Item / Qtd. / Unit. (R$) / Subtotal (R$) / Desc. (R$) / Total (R$)
// ---------------------------------------------------------------------------

// reInicioItem casa a primeira linha de um item: o número, a descrição, e os
// 5 números da linha (qtd, unitário, subtotal, desconto, total) — nesta
// ordem, que é a ordem das colunas do cabeçalho da tabela.
var reInicioItem = regexp.MustCompile(
	`^\s*(\d{1,4})\s+(\S.*?)\s+([\d.]+,\d{2,3})\s+([\d.]+,\d{2})\s+([\d.]+,\d{2})\s+([\d.]+,\d{2})\s+([\d.]+,\d{2})\s*$`)

// A unidade de medida ("UN", "Unidades", "KG"...) aparece de DOIS jeitos na
// continuação da descrição — medido nesta mesma OC de 10 itens:
//
//	sozinha na linha        item 2: "                              UN"
//	grudada com o resto     item 1: "      BRANCO                 UN"
//
// `reUnidadeSozinha` casa a primeira forma; `reUnidadeNoFim`, a segunda.
// LISTA FECHADA, DE PROPÓSITO: uma lista aberta (qualquer palavra curta no
// fim da linha) arriscaria cortar a última palavra de uma descrição de
// verdade. Uma unidade fora da lista fica sem `unidade` preenchida — campo
// secundário, sem efeito nos dois filtros de negócio nem no total.
const padraoDeUnidades = `UN|Unidades?|Unid\.?|PC|Peças?|Pça|PCT|CX|KG|G|M2?|M3?|L|RL|Rolo|PAR|CJ|Conjunto`

var (
	reUnidadeSozinha = regexp.MustCompile(`(?i)^\s*(` + padraoDeUnidades + `)\s*$`)
	reUnidadeNoFim   = regexp.MustCompile(`(?i)^(.*\S)\s{2,}(` + padraoDeUnidades + `)\s*$`)
)

// reFimDaTabela marca onde a tabela de itens acaba — a linha do rodapé
// "Subtotal", não a palavra em qualquer lugar do texto (ela também aparece
// solta se algum item tiver "Subtotal" na descrição, embora isso não tenha
// acontecido em nenhuma amostra vista até aqui).
var reFimDaTabela = regexp.MustCompile(`(?m)^\s*Subtotal\s`)

// O Obra Prima repete, no topo de CADA página nova, um bloco fixo: a data no
// canto (sozinha na própria linha), o nome da empresa, o endereço com
// "Página X/Y" embutido no meio da linha, o CNPJ, "ORDEM DE COMPRA
// <número>", o cliente, e por fim o cabeçalho de coluna da tabela de novo
// ("N. Item ... Total (R$)").
//
// MEDIDO NA OC REAL DE 2 PÁGINAS (019731, 10/09/2026)
//
//	Sem remover esse bloco inteiro, ele vira parte da descrição do último
//	item da página anterior — foi exatamente o item 8 (ELETRODUTO) que
//	"engoliu" a data, a empresa, o número da OC e o cabeçalho da tabela de
//	novo, nas duas primeiras versões deste arquivo. A remoção é em duas
//	passadas: a data sozinha na linha (regex genérica, sem depender do nome
//	da empresa) e depois "FROTA MACEDO ENGENHARIA LTDA" até o próximo
//	"N. Item" — a primeira ocorrência de cada uma, no topo do documento,
//	fica ANTES do "N. Item" que já corta o início de `corpo`.
var (
	reDataSozinhaNaLinha = regexp.MustCompile(`(?m)^\s*\d{2}/\d{2}/\d{4}\s*$`)
	// Consome até o FIM da linha do "N. Item" — o resto do cabeçalho de
	// coluna ("Qtd. Unit. (R$)...") está nessa mesma linha física, depois de
	// "N. Item", não numa linha à parte.
	rePaginaNova = regexp.MustCompile(`(?s)FROTA MACEDO ENGENHARIA LTDA.*?N\. Item[^\n]*\n?`)
)

// extrairItens lê a tabela linha a linha, entre o cabeçalho "N. Item" e o
// rodapé "Subtotal". GENEROSA DE PROPÓSITO: uma linha que não bate o formato
// esperado vira parte da descrição do item corrente, nunca derruba a leitura
// inteira — ver o cabeçalho do arquivo.
func extrairItens(texto string) []ItemExtraido {
	inicio := strings.Index(texto, "N. Item")
	if inicio < 0 {
		return nil
	}
	corpo := texto[inicio:]
	corpo = reDataSozinhaNaLinha.ReplaceAllString(corpo, "")
	corpo = rePaginaNova.ReplaceAllString(corpo, "")
	if loc := reFimDaTabela.FindStringIndex(corpo); loc != nil {
		corpo = corpo[:loc[0]]
	}

	var itens []ItemExtraido
	proximo := 1
	for _, linha := range strings.Split(corpo, "\n") {
		if m := reInicioItem.FindStringSubmatch(linha); m != nil {
			n, err := strconv.Atoi(m[1])
			if err == nil && n == proximo {
				itens = append(itens, ItemExtraido{
					Descricao: strings.TrimSpace(m[2]),
					Qtd:       numeroBR(m[3]),
					ValorUnit: dinheiroBR(m[4]),
					Total:     dinheiroBR(m[7]),
					Desconto:  dinheiroBR(m[6]),
				})
				proximo++
				continue
			}
		}
		// Não é início de item novo — é continuação (descrição que quebrou de
		// linha, com ou sem a unidade grudada no fim) do item anterior.
		if len(itens) == 0 {
			continue // ainda não achamos o primeiro item — cabeçalho da tabela
		}
		linha := strings.TrimRight(linha, "\r")
		if strings.TrimSpace(linha) == "" {
			continue
		}
		atual := &itens[len(itens)-1]
		if atual.Unidade == "" {
			if m := reUnidadeSozinha.FindStringSubmatch(linha); m != nil {
				atual.Unidade = m[1]
				continue
			}
			if m := reUnidadeNoFim.FindStringSubmatch(linha); m != nil {
				atual.Unidade = m[2]
				atual.Descricao = juntar(atual.Descricao, strings.TrimSpace(m[1]))
				continue
			}
		}
		atual.Descricao = juntar(atual.Descricao, strings.TrimSpace(linha))
	}
	return itens
}

func juntar(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

// reFooterTresNumeros casa a linha "Subtotal <valor> <desconto> <total>" do
// rodapé — os três números da soma de cada coluna.
var (
	reFooterSubtotal = regexp.MustCompile(`(?m)^\s*Subtotal\s+([\d.]+,\d{2})\s+([\d.]+,\d{2})\s+([\d.]+,\d{2})\s*$`)
	reFooterFrete    = regexp.MustCompile(`(?m)^\s*Frete\s+([\d.]+,\d{2})\s*$`)
	reFooterTotal    = regexp.MustCompile(`(?m)^\s*Total\s+([\d.]+,\d{2})\s*$`)
)

// extrairTotais lê o rodapé (Subtotal / Frete / Total). Os três são
// independentes: um PDF que só perdeu o "Frete" (0,00 é comum, mas a linha
// pode faltar em formatos diferentes) ainda entrega Subtotal e Total.
func extrairTotais(texto string, e *Extraida) {
	if m := reFooterSubtotal.FindStringSubmatch(texto); m != nil {
		e.Subtotal = dinheiroBR(m[1])
		e.Desconto = dinheiroBR(m[2])
	}
	if m := reFooterFrete.FindStringSubmatch(texto); m != nil {
		e.Frete = dinheiroBR(m[1])
	}
	if m := reFooterTotal.FindStringSubmatch(texto); m != nil {
		e.Total = dinheiroBR(m[1])
	}
}

// ---------------------------------------------------------------------------
// conversões do formato brasileiro — arquivo próprio, mesma regra de
// `consolidacao.dinheiroBR`/`dataBR` (P-13/CORE-16: cada módulo com a sua)
// ---------------------------------------------------------------------------

// dinheiroBR lê "1.234,56" (ponto de milhar, vírgula decimal) e devolve em
// centavos (P-12: dinheiro nunca em float solto). Texto que não converte
// devolve zero — quem chama já confere se o campo existe antes de usar isto.
func dinheiroBR(texto string) regras.Dinheiro {
	v := numeroBR(texto)
	return regras.DinheiroDe(v)
}

// numeroBR é a mesma conversão, mas devolvendo o float cru — para colunas
// como quantidade, que não são dinheiro.
func numeroBR(texto string) float64 {
	semMilhar := strings.ReplaceAll(strings.TrimSpace(texto), ".", "")
	comPonto := strings.ReplaceAll(semMilhar, ",", ".")
	v, err := strconv.ParseFloat(comPonto, 64)
	if err != nil {
		return 0
	}
	return v
}

// dataBR lê "28/08/2026" e devolve "2026-08-28". Data que não converte
// devolve "" — nulo no banco, não uma data inventada.
func dataBR(texto string) string {
	partes := strings.Split(strings.TrimSpace(texto), "/")
	if len(partes) != 3 {
		return ""
	}
	dia, mes, ano := partes[0], partes[1], partes[2]
	if len(dia) != 2 || len(mes) != 2 || len(ano) != 4 {
		return ""
	}
	return ano + "-" + mes + "-" + dia
}
