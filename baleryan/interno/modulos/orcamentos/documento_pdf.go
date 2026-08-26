// rev 2 — o orçamento como documento, e não como listagem
//
// O QUE MUDOU, E POR QUÊ
//
//	A rev 1 montava um `relatorio.Tabela` — a mesma listagem das extrações de
//	planilha. Saía em A4 deitada, com cabeçalho de relatório e rodapé "FrotaHub
//	· gerado em". Funcionava como despejo de dados e falhava como documento: sem
//	marca, sem as partes, sem valor por extenso, sem aceite. E é este arquivo que
//	vai ao cliente.
//
//	Agora ele é desenhado: capa com marca e dados do emitente, faixa com o
//	número, obra, os dois quadros de prestador e tomador, a discriminação, o
//	total por extenso, as observações e a linha de assinatura. O espelho do que o
//	sistema antigo entregava, que é o que a operação conhece.
//
// DE ONDE VÊM OS DADOS QUE NÃO SÃO DO ORÇAMENTO
//
//	`emitente` (migração 026) guarda razão social, CNPJ, endereço, contato,
//	forma de pagamento, validade, observações e a marca. Por cliente, e não no
//	binário: telefone de escritório não deve exigir deploy.
//
//	`unidades` guarda o tomador. O CNPJ da loja é opcional no desenho — a linha
//	simplesmente não sai enquanto o cadastro não tiver o dado, em vez de imprimir
//	um rótulo vazio.
package orcamentos

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// Medidas da folha, em pontos. A margem é a mesma dos quatro lados.
const (
	margemDoc  = 42.0
	larguraDoc = relatorio.LarguraRetrato - 2*margemDoc
)

type dadosDoEmitente struct {
	Razao      string   `json:"razao_social"`
	CNPJ       string   `json:"cnpj"`
	Endereco   string   `json:"endereco"`
	Contato    string   `json:"contato"`
	Pagamento  string   `json:"forma_pagamento"`
	Validade   int      `json:"validade_dias"`
	Observacao []string `json:"observacoes"`
	MarcaB64   *string  `json:"marca_jpeg_base64"`
}

type dadosDoTomador struct {
	Nome     string  `json:"nome"`
	CNPJ     *string `json:"cnpj"`
	Endereco *string `json:"endereco"`
	Cidade   *string `json:"cidade"`
	UF       *string `json:"uf"`
}

type itemDoOrcamento struct {
	Ordem      int     `json:"ordem"`
	Descricao  string  `json:"descricao"`
	Unidade    *string `json:"unidade"`
	Quantidade float64 `json:"quantidade"`
	Cobrado    float64 `json:"valor_unitario_cobrado"`
	Total      float64 `json:"valor_total"`
}

func (m *Modulo) montarPDF(ctx context.Context, clienteID, orcamentoID string) ([]byte, error) {
	cabeca, err := m.contarUm(ctx, "orcamentos_lista?id=eq."+orcamentoID+
		"&cliente_id=eq."+banco.Escapar(clienteID)+"&select=*&limit=1")
	if err != nil {
		return nil, err
	}

	var itens []itemDoOrcamento
	if err := m.bd.Buscar(ctx, "orcamento_itens?orcamento_id=eq."+orcamentoID+
		"&order=ordem&select=ordem,descricao,unidade,quantidade,valor_unitario_cobrado,valor_total",
		&itens); err != nil {
		return nil, err
	}
	if len(itens) == 0 {
		return nil, fmt.Errorf("este orçamento não tem itens")
	}

	emi, err := m.emitenteDe(ctx, clienteID)
	if err != nil {
		return nil, err
	}
	tomador := m.tomadorDe(ctx, cabeca)

	return desenharOrcamento(cabeca, itens, emi, tomador)
}

// emitenteDe lê quem assina. A falta dele é erro, e não um documento sem capa:
// orçamento sem CNPJ do emitente não serve para cobrar ninguém.
func (m *Modulo) emitenteDe(ctx context.Context, clienteID string) (*dadosDoEmitente, error) {
	var linhas []dadosDoEmitente
	if err := m.bd.Buscar(ctx, "emitente?cliente_id=eq."+banco.Escapar(clienteID)+
		"&select=*&limit=1", &linhas); err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		return nil, fmt.Errorf("os dados do emitente não estão cadastrados — sem eles o orçamento não pode ser emitido")
	}
	return &linhas[0], nil
}

// tomadorDe lê a loja. Falta de unidade NÃO derruba o documento: o orçamento
// continua válido com o nome que veio do chamado, e um quadro incompleto é
// melhor que nenhum documento.
func (m *Modulo) tomadorDe(ctx context.Context, cabeca map[string]any) dadosDoTomador {
	nome, _ := cabeca["loja"].(string)
	saida := dadosDoTomador{Nome: nome}

	id, ok := cabeca["unidade_id"].(string)
	if !ok || id == "" {
		return saida
	}
	var us []dadosDoTomador
	if err := m.bd.Buscar(ctx, "unidades?id=eq."+id+
		"&select=nome,cnpj,endereco,cidade,uf&limit=1", &us); err != nil || len(us) == 0 {
		return saida
	}
	if us[0].Nome == "" {
		us[0].Nome = nome
	}
	return us[0]
}

// ---------------------------------------------------------------------------
// o desenho
// ---------------------------------------------------------------------------

func desenharOrcamento(cabeca map[string]any, itens []itemDoOrcamento,
	emi *dadosDoEmitente, tom dadosDoTomador) ([]byte, error) {

	f := relatorio.NovaFolha()
	y := relatorio.AlturaRetrato - margemDoc

	ticket := fmt.Sprint(cabeca["ticket"])
	parte := fmt.Sprint(cabeca["parte"])
	quando := time.Now().In(fusoDaCasa())
	if c, ok := cabeca["criado_em"].(string); ok && c != "" {
		if t, err := time.Parse(time.RFC3339, c); err == nil {
			quando = t.In(fusoDaCasa())
		}
	}
	data := quando.Format("02/01/2006")

	y = capa(f, y, emi, ticket, data)
	y = faixaDoNumero(f, y, ticket, parte)
	loja, _ := cabeca["loja"].(string)
	y = linhaDaObra(f, y, loja, ticket)
	y = quadrosDasPartes(f, y, emi, tom, data)
	y, soma := discriminacao(f, y, itens)
	y = porExtenso(f, y, soma)
	y = observacoes(f, y, emi, cabeca)
	assinaturas(f, y, emi)
	rodape(f, emi)

	return f.PDF()
}

// capa: marca à esquerda, identificação no meio, data e número à direita.
func capa(f *relatorio.Folha, y float64, emi *dadosDoEmitente, ticket, data string) float64 {
	topo := y
	xTexto := margemDoc

	if emi.MarcaB64 != nil && *emi.MarcaB64 != "" {
		if bruto, err := base64.StdEncoding.DecodeString(*emi.MarcaB64); err == nil {
			// 200x100 na origem; 56x28 no papel mantém a proporção exata.
			if err := f.Imagem(margemDoc, topo-30, 56, 28, bruto); err == nil {
				xTexto = margemDoc + 68
			}
		}
		// Marca ilegível não derruba o documento — ele sai sem ela. O orçamento
		// vale pelo conteúdo, e uma exceção aqui deixaria o cliente sem nada.
	}

	f.Texto(xTexto, topo-10, 11, true, relatorio.CorTexto, emi.Razao)
	f.Texto(xTexto, topo-22, 6.8, false, relatorio.CorMuda, emi.Endereco)
	f.Texto(xTexto, topo-32, 6.8, false, relatorio.CorMuda,
		emi.Contato+" • CNPJ "+emi.CNPJ)

	f.Direita(relatorio.LarguraRetrato-margemDoc, topo-10, 7.5, false, relatorio.CorMuda, data)
	f.Direita(relatorio.LarguraRetrato-margemDoc, topo-22, 7.5, false, relatorio.CorMuda,
		"Orçamento nº "+ticket)

	return topo - 52
}

func faixaDoNumero(f *relatorio.Folha, y float64, ticket, parte string) float64 {
	const alt = 62.0
	f.Caixa(margemDoc, y-alt, larguraDoc, alt, relatorio.CorFaixa)
	f.Texto(margemDoc+22, y-34, 21, true, relatorio.CorBranco, "ORÇAMENTO N° "+ticket)

	// A REVISÃO É A PARTE, E SÓ APARECE QUANDO EXISTE
	//   Um ticket com duas notas gera duas partes. Chamá-las de revisão é o
	//   vocabulário que a operação já usa; escrever "REVISÃO 1" em todo
	//   orçamento tiraria o sentido da palavra.
	if parte != "" && parte != "1" {
		f.Texto(margemDoc+22, y-50, 8, false, relatorio.CorClara, "REVISÃO "+parte)
	}
	return y - alt - 14
}

func linhaDaObra(f *relatorio.Folha, y float64, loja, ticket string) float64 {
	const alt = 22.0
	f.Caixa(margemDoc, y-alt, larguraDoc, alt, relatorio.CorFundo)
	obra := "MANUTENÇÃO"
	if loja != "" {
		obra += " " + strings.ToUpper(loja)
	}
	f.Texto(margemDoc+10, y-15, 8, true, relatorio.CorTexto, "OBRA: "+obra+" #"+ticket)
	return y - alt - 16
}

// quadrosDasPartes desenha prestador e tomador lado a lado.
func quadrosDasPartes(f *relatorio.Folha, y float64, emi *dadosDoEmitente,
	tom dadosDoTomador, data string) float64 {

	const vao = 12.0
	larg := (larguraDoc - vao) / 2

	esq := []linhaDoQuadro{
		{"Nome:", emi.Razao},
		{"CNPJ:", emi.CNPJ},
		{"Forma de pagamento:", emi.Pagamento},
		{"Data de faturamento:", data},
	}
	dir := []linhaDoQuadro{{"Nome:", tom.Nome}}
	if v := valorDe(tom.CNPJ); v != "" {
		dir = append(dir, linhaDoQuadro{"CNPJ:", v})
	}
	if v := valorDe(tom.Endereco); v != "" {
		dir = append(dir, linhaDoQuadro{"Endereço:", v})
	}
	if cidade := valorDe(tom.Cidade); cidade != "" {
		if uf := valorDe(tom.UF); uf != "" {
			cidade += " - " + uf
		}
		dir = append(dir, linhaDoQuadro{"Cidade/Estado:", cidade})
	}

	// Os dois quadros têm a MESMA altura, mesmo com contagens diferentes de
	// linha. Alturas desiguais lado a lado leem-se como erro de montagem.
	n := len(esq)
	if len(dir) > n {
		n = len(dir)
	}
	alt := 22.0 + float64(n)*15.0 + 8.0

	quadro(f, margemDoc, y, larg, alt, "DADOS DO PRESTADOR", esq)
	quadro(f, margemDoc+larg+vao, y, larg, alt, "DADOS DO TOMADOR", dir)
	return y - alt - 18
}

type linhaDoQuadro struct{ rotulo, valor string }

func quadro(f *relatorio.Folha, x, y, larg, alt float64, titulo string, linhas []linhaDoQuadro) {
	f.Moldura(x, y-alt, larg, alt, relatorio.CorLinha)
	f.Caixa(x, y-22, larg, 22, relatorio.CorFaixa)
	f.Texto(x+10, y-15, 7.5, true, relatorio.CorBranco, titulo)

	// A COLUNA DE VALOR COMEÇA NO MESMO x PARA TODAS AS LINHAS
	//   Medindo rótulo a rótulo, cada valor começava num lugar — e o rótulo mais
	//   longo colava no próprio valor, porque a medida é feita com a tabela da
	//   fonte normal e o rótulo é negrito, que é mais largo. Uma coluna só
	//   resolve o alinhamento e a colagem de uma vez.
	var maior float64
	for _, l := range linhas {
		if w := f.Medir(l.rotulo, 7.5) * 1.08; w > maior {
			maior = w
		}
	}
	xv := x + 10 + maior + 6

	ly := y - 38
	for _, l := range linhas {
		f.Texto(x+10, ly, 7.5, true, relatorio.CorTexto, l.rotulo)
		f.TextoCortado(xv, ly, 7.5, x+larg-10-xv, false, relatorio.CorMuda, l.valor)
		ly -= 15
	}
}

// cabecalhoDaTabela desenha a faixa dos títulos. Existe à parte porque a
// segunda página precisa dela de novo: uma tabela que continua sem repetir o
// cabeçalho obriga quem lê a voltar a folha para saber que coluna é qual.
func cabecalhoDaTabela(f *relatorio.Folha, y float64, x []float64) float64 {
	const altCab = 20.0
	f.Caixa(margemDoc, y-altCab, larguraDoc, altCab, relatorio.CorFaixa)
	titulos := []string{"ITEM", "DESCRIÇÃO", "QUANT.", "UNID.", "VALOR UNIT.", "TOTAL"}
	for i, t := range titulos {
		if i >= 2 {
			f.Direita(x[i+1]-8, y-13, 7, true, relatorio.CorBranco, t)
			continue
		}
		f.Texto(x[i]+8, y-13, 7, true, relatorio.CorBranco, t)
	}
	return y - altCab
}

// as colunas da discriminação, em proporção da largura útil
var pesosDaTabela = []float64{0.07, 0.47, 0.11, 0.10, 0.12, 0.13}

func discriminacao(f *relatorio.Folha, y float64, itens []itemDoOrcamento) (float64, regras.Dinheiro) {
	f.Texto(margemDoc, y, 8, true, relatorio.CorTexto, "DISCRIMINAÇÃO DOS ITENS")
	y -= 12

	x := make([]float64, len(pesosDaTabela)+1)
	x[0] = margemDoc
	for i, p := range pesosDaTabela {
		x[i+1] = x[i] + p*larguraDoc
	}

	y = cabecalhoDaTabela(f, y, x)

	var soma regras.Dinheiro
	const altLinha = 18.0
	// DUAS RÉGUAS DIFERENTES, E ISSO É DE PROPÓSITO
	//
	//	A linha de item quebra a página quando não cabe MAIS UMA LINHA. O rabo do
	//	documento — total, extenso, observações, assinatura — quebra quando não
	//	cabe ELE INTEIRO, e isso é conferido depois, uma vez só.
	//
	//	Reservar o rabo a cada linha era o caminho fácil e deixava um palmo de
	//	branco no pé de toda primeira página. Reservar nada partiria as
	//	observações ao meio.
	const pePagina = margemDoc + 24.0

	for i, it := range itens {
		if y-altLinha < pePagina {
			f.Pagina()
			y = relatorio.AlturaRetrato - margemDoc
			y = cabecalhoDaTabela(f, y, x)
		}
		if i%2 == 1 {
			f.Caixa(margemDoc, y-altLinha, larguraDoc, altLinha, relatorio.CorFundo)
		}
		soma += regras.DinheiroDe(it.Total)

		f.Texto(x[0]+8, y-12, 7.5, false, relatorio.CorMuda, fmt.Sprint(it.Ordem))
		f.TextoCortado(x[1]+8, y-12, 7.5, x[2]-x[1]-16, false, relatorio.CorTexto, it.Descricao)
		f.Direita(x[3]-8, y-12, 7.5, false, relatorio.CorTexto, emQuantidade(it.Quantidade))
		f.Direita(x[4]-8, y-12, 7.5, false, relatorio.CorTexto, valorDe(it.Unidade))
		f.Direita(x[5]-8, y-12, 7.5, false, relatorio.CorTexto, "R$ "+regras.DinheiroDe(it.Cobrado).Reais())
		f.Direita(x[6]-8, y-12, 7.5, false, relatorio.CorTexto, "R$ "+regras.DinheiroDe(it.Total).Reais())
		y -= altLinha
	}

	// O RABO DO DOCUMENTO NÃO SE PARTE
	//   Total, extenso, observações e aceite andam juntos. Um total numa folha e
	//   a assinatura noutra é documento que ninguém assina com confiança.
	const alturaDoRabo = 230.0
	if y-alturaDoRabo < margemDoc {
		f.Pagina()
		y = relatorio.AlturaRetrato - margemDoc
	}

	// O TOTAL VEM DA SOMA DOS ITENS, E NÃO DO CAMPO `valor`
	//   Se os dois divergirem, o documento mostra a divergência em vez de
	//   escondê-la — e alguém conserta. Um total que não fecha com as linhas
	//   acima é o primeiro que a auditoria pega.
	const altTotal = 22.0
	f.Caixa(margemDoc, y-altTotal, larguraDoc, altTotal, relatorio.CorFundo)
	f.Direita(x[5]-8, y-15, 8, true, relatorio.CorTexto, "TOTAL GERAL")
	f.Direita(x[6]-8, y-15, 8.5, true, relatorio.CorTexto, "R$ "+soma.Reais())
	f.Moldura(margemDoc, y-altTotal, larguraDoc, altTotal, relatorio.CorLinha)

	return y - altTotal - 20, soma
}

func porExtenso(f *relatorio.Folha, y float64, soma regras.Dinheiro) float64 {
	rotulo := "Valor total por extenso: "
	f.Texto(margemDoc, y, 7.5, true, relatorio.CorTexto, rotulo)
	f.TextoCortado(margemDoc+f.Medir(rotulo, 7.5), y, 7.5,
		larguraDoc-f.Medir(rotulo, 7.5), false, relatorio.CorMuda, regras.PorExtenso(soma)+".")
	return y - 22
}

func observacoes(f *relatorio.Folha, y float64, emi *dadosDoEmitente, cabeca map[string]any) float64 {
	linhas := append([]string{}, emi.Observacao...)

	// O CORTE PELO TETO APARECE NO DOCUMENTO
	//   Corte que não aparece é corte que ninguém explica depois. O que NÃO
	//   aparece aqui é o desconto do fornecedor: aquilo é conta nossa.
	if reduzido, _ := cabeca["reduzido_pelo_teto"].(bool); reduzido {
		antes := regras.DinheiroDe(numeroDe(cabeca["valor_antes_do_teto"]))
		linhas = append(linhas,
			"Valor ajustado ao limite contratado (de R$ "+antes.Reais()+").")
	}
	if len(linhas) == 0 {
		return y
	}

	alt := 22.0 + float64(len(linhas))*13.0 + 8.0
	f.Moldura(margemDoc, y-alt, larguraDoc, alt, relatorio.CorLinha)
	f.Caixa(margemDoc, y-22, larguraDoc, 22, relatorio.CorFaixa)
	f.Texto(margemDoc+10, y-15, 7.5, true, relatorio.CorBranco, "OBSERVAÇÕES")

	ly := y - 37
	for _, l := range linhas {
		f.Texto(margemDoc+10, ly, 7.2, false, relatorio.CorTexto, "• "+l)
		ly -= 13
	}
	return y - alt - 30
}

func assinaturas(f *relatorio.Folha, y float64, emi *dadosDoEmitente) {
	if y < margemDoc+70 {
		y = margemDoc + 70
	}
	meio := margemDoc + larguraDoc/2
	f.Linha(margemDoc+20, meio-20, y, relatorio.CorLinha)
	f.Linha(meio+20, margemDoc+larguraDoc-20, y, relatorio.CorLinha)

	centrar(f, margemDoc+20, meio-20, y-12, emi.Razao)
	centrar(f, meio+20, margemDoc+larguraDoc-20, y-12, "Aceite do Cliente — Nome / Data")
}

func centrar(f *relatorio.Folha, x1, x2, y float64, txt string) {
	f.Texto(x1+((x2-x1)-f.Medir(txt, 7.5))/2, y, 7.5, false, relatorio.CorMuda, txt)
}

func rodape(f *relatorio.Folha, emi *dadosDoEmitente) {
	txt := emi.Razao + "  •  CNPJ " + emi.CNPJ
	f.Texto(margemDoc+(larguraDoc-f.Medir(txt, 6.8))/2, margemDoc-6, 6.8, false, relatorio.CorClara, txt)
}

// emQuantidade escreve 2,00 e 10,00 — duas casas sempre, como no documento que
// a operação conhece.
func emQuantidade(q float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", q), ".", ",", 1)
}

func valorDe(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func (m *Modulo) pdfDoOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	pdf, err := m.montarPDF(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não consegui montar o PDF", err)
		return
	}
	entregar(w, pdf, "orcamento", "pdf", "application/pdf")
}

// encurtar corta um texto longo no limite, com reticências. Vive aqui porque
// nasceu com o PDF do orçamento; hoje as pendências e as correções também usam.
func encurtar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// entregar manda o arquivo com o nome certo.
//
// O nome carrega a data e a hora: dois downloads do mesmo relatório no mesmo
// dia não podem virar "(1)" na pasta de quem baixou.
func entregar(w http.ResponseWriter, corpo []byte, base, extensao, tipo string) {
	nome := fmt.Sprintf("%s%s.%s", base, time.Now().In(fusoDaCasa()).Format("200601021504"), extensao)
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`"`)
	w.Header().Set("Content-Length", fmt.Sprint(len(corpo)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(corpo)
}
