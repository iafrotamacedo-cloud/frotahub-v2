// rev 1 — a Ordem de Compra desenhada, clone visual do Obra Prima
//
// POR QUE GERAR DE NOVO, E NÃO TAMPONAR O PDF ANTIGO
//
//	Editar "em cima" do arquivo original deixa o texto velho no fluxo, marca
//	de revisão, e um leitor atento vê o remendo. O pedido foi substituir o
//	PDF antigo pelo editado sem vestígio na folha: então a folha é desenhada
//	do zero, com `relatorio.Folha` (o mesmo encanamento do orçamento), no
//	layout que a operação já conhece — letreiro, blocos, tabela, totais.
//
//	Não leva faixa azul FrotaHub nem rodapé "gerado em". É a OC, não um
//	relatório nosso.
package administrativo

import (
	"fmt"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
)

const (
	ocMargem = 40.0
	ocEsq    = 40.0
)

func ocDir() float64 { return relatorio.LarguraRetrato - ocMargem }

const (
	ocCor = relatorio.CorTexto
	ocTam = 8.0
)

// desenharOC monta o PDF da ordem no visual da OC 019731. Recalcula totais
// antes de desenhar, para a folha e a conta não divergirem.
func desenharOC(e Extraida) ([]byte, error) {
	e.RecalcularTotais()
	aplicarPadraoEmitente(&e)
	if strings.TrimSpace(e.Numero) == "" {
		return nil, fmt.Errorf("a ordem de compra precisa de número")
	}
	if len(e.Itens) == 0 {
		return nil, fmt.Errorf("a ordem de compra precisa de ao menos um item")
	}

	f := relatorio.NovaFolha()
	paginas := estimarPaginas(e)
	y := letreiroOC(f, e, 1, paginas)
	y = blocosDaOC(f, e, y)

	xs := colunasDaTabelaOC()
	y = cabecalhoTabelaOC(f, y, xs)
	y = itensDaOC(f, e, y, xs, paginas)
	totaisDaOC(f, e, y, xs, paginas)

	return f.PDF()
}

func aplicarPadraoEmitente(e *Extraida) {
	if strings.TrimSpace(e.EmitenteRazao) == "" {
		e.EmitenteRazao = "FROTA MACEDO ENGENHARIA LTDA"
	}
	if strings.TrimSpace(e.EmitenteEndereco) == "" {
		e.EmitenteEndereco = "Engenheiro Heitor de Oliveira Albuquerque, 295 - Cidade dos Funcionários - Fortaleza/CE"
	}
	if strings.TrimSpace(e.EmitenteContato) == "" {
		e.EmitenteContato = "85 2181 - 1386 - frotamacedoengenharia@gmail.com"
	}
	if strings.TrimSpace(e.EmitenteCNPJ) == "" {
		e.EmitenteCNPJ = "27.363.223/0001-70"
	}
	if strings.TrimSpace(e.DataImpressao) == "" {
		e.DataImpressao = time.Now().In(relatorio.FusoDaCasa()).Format("02/01/2006")
	}
	if strings.TrimSpace(e.TituloObra) == "" {
		e.TituloObra = e.ObraCentroCusto
	}
}

func letreiroOC(f *relatorio.Folha, e Extraida, pagina, paginas int) float64 {
	y := relatorio.AlturaRetrato - 36
	dir := ocDir()
	f.Texto(ocEsq, y, 10, true, ocCor, e.EmitenteRazao)
	f.Direita(dir, y, 9, false, ocCor, e.DataImpressao)
	y -= 13
	rotuloPag := fmt.Sprintf("Página %d/%d", pagina, paginas)
	if paginas < 1 {
		paginas = 1
		rotuloPag = fmt.Sprintf("Página %d/%d", pagina, paginas)
	}
	f.TextoCortado(ocEsq, y, 8, 380, false, ocCor, e.EmitenteEndereco)
	f.Direita(dir, y, 8, false, ocCor, rotuloPag)
	y -= 12
	contato := e.EmitenteContato
	if e.EmitenteCNPJ != "" {
		if contato != "" {
			contato += " - "
		}
		contato += "CNPJ: " + e.EmitenteCNPJ
	}
	f.TextoCortado(ocEsq, y, 8, dir-ocEsq, false, ocCor, contato)
	y -= 22
	f.Texto(ocEsq, y, 12, true, ocCor, "ORDEM DE COMPRA "+e.Numero)
	y -= 14
	if e.TituloObra != "" {
		f.TextoCortado(ocEsq, y, 9, dir-ocEsq, false, ocCor, e.TituloObra)
		y -= 16
	}
	return y
}

func blocosDaOC(f *relatorio.Folha, e Extraida, y float64) float64 {
	dir := ocDir()
	meio := ocEsq + 280

	f.Texto(ocEsq, y, 9, true, ocCor, "DADOS DA ORDEM DE COMPRA      "+e.Numero)
	y -= 16
	f.Texto(ocEsq, y, ocTam, false, ocCor, "Data: "+dataParaTela(e.Data))
	f.Texto(meio, y, ocTam, false, ocCor, "Previsão da entrega: "+dataParaTela(e.PrevisaoEntrega))
	y -= 13
	f.Texto(ocEsq, y, ocTam, false, ocCor, "Cond. pgto.: "+e.CondPgto)
	f.Texto(meio, y, ocTam, false, ocCor, "Forma pgto.: "+e.FormaPgto)
	y -= 13
	obs := "Observação:"
	if strings.TrimSpace(e.Observacao) != "" {
		obs += " " + e.Observacao
	}
	f.TextoCortado(ocEsq, y, ocTam, dir-ocEsq, false, ocCor, obs)
	y -= 18

	f.Texto(ocEsq, y, 9, true, ocCor, "RESPONSÁVEL PELA COMPRA")
	y -= 14
	f.Texto(ocEsq, y, ocTam, false, ocCor, "Nome: "+primeiroNaoVazio(e.ResponsavelNome, e.EmitenteRazao))
	f.Texto(meio, y, ocTam, false, ocCor, "Comprador: "+e.CompradorInterno)
	y -= 13
	email := e.ResponsavelEmail
	if email == "" {
		email = "compras@frotamacedo.com.br"
	}
	f.Texto(meio, y, ocTam, false, ocCor, "Email: "+email)
	y -= 18

	f.Texto(ocEsq, y, 9, true, ocCor, "DADOS DO FATURAMENTO")
	y -= 14
	y = parNomeEndereco(f, y, e.CompradorNome, e.FaturamentoEndereco)
	f.Texto(ocEsq, y, ocTam, false, ocCor, "CNPJ: "+e.CompradorCNPJ)
	y -= 13
	f.Texto(ocEsq, y, ocTam, false, ocCor, "I.E.: "+e.FaturamentoIE)
	y -= 18

	f.Texto(ocEsq, y, 9, true, ocCor, "DADOS DO FORNECEDOR")
	y -= 14
	y = parNomeEndereco(f, y, e.FornecedorNome, e.FornecedorEndereco)
	f.Texto(ocEsq, y, ocTam, false, ocCor, "CNPJ: "+e.FornecedorCNPJ)
	y -= 13
	f.Texto(ocEsq, y, ocTam, false, ocCor, "Telefone: "+e.FornecedorTelefone)
	y -= 13
	f.Texto(ocEsq, y, ocTam, false, ocCor, "Vendedor: "+e.FornecedorVendedor)
	y -= 13
	f.Texto(ocEsq, y, ocTam, false, ocCor, "E-mail: "+e.FornecedorEmail)
	y -= 18

	obra := "OBRA/CENTRO DE CUSTO: " + e.ObraCentroCusto
	f.TextoCortado(ocEsq, y, 9, 360, true, ocCor, obra)
	f.Texto(ocEsq+400, y, ocTam, true, ocCor, "CNO: "+e.CNO)
	y -= 16

	f.Texto(ocEsq, y, 9, true, ocCor, "ENDEREÇO ENTREGA:")
	f.Texto(meio+40, y, 9, true, ocCor, "ENDEREÇO COBRANÇA:")
	y -= 13
	linhasE := quebrarFolha(f, "Endereço: "+e.EnderecoEntrega, 270, ocTam)
	linhasC := quebrarFolha(f, "Endereço: "+e.EnderecoCobranca, 230, ocTam)
	n := len(linhasE)
	if len(linhasC) > n {
		n = len(linhasC)
	}
	for i := 0; i < n; i++ {
		if i < len(linhasE) {
			f.Texto(ocEsq, y, ocTam, false, ocCor, linhasE[i])
		}
		if i < len(linhasC) {
			f.Texto(meio+40, y, ocTam, false, ocCor, linhasC[i])
		}
		y -= 12
	}
	f.Texto(ocEsq, y, ocTam, false, ocCor, "Recebedor: "+e.Recebedor)
	return y - 16
}

func parNomeEndereco(f *relatorio.Folha, y float64, nome, endereco string) float64 {
	meio := ocEsq + 300
	f.TextoCortado(ocEsq, y, ocTam, 290, false, ocCor, "Nome: "+nome)
	linhas := quebrarFolha(f, "Endereço: "+endereco, 210, ocTam)
	for i, ln := range linhas {
		if i == 0 {
			f.Texto(meio, y, ocTam, false, ocCor, ln)
			continue
		}
		y -= 12
		f.Texto(meio, y, ocTam, false, ocCor, ln)
	}
	if len(linhas) == 0 {
		f.Texto(meio, y, ocTam, false, ocCor, "Endereço:")
	}
	return y - 13
}

// colunas: n.item | desc | qtd | unit | subtotal | desc$ | total  (x do FIM de cada numérica)
func colunasDaTabelaOC() []float64 {
	return []float64{ocEsq, ocEsq + 28, 318, 378, 438, 490, ocDir()}
}

func cabecalhoTabelaOC(f *relatorio.Folha, y float64, x []float64) float64 {
	f.Texto(x[0], y, 7.5, true, ocCor, "N. Item")
	f.Direita(x[2], y, 7.5, true, ocCor, "Qtd.")
	f.Direita(x[3], y, 7.5, true, ocCor, "Unit. (R$)")
	f.Direita(x[4], y, 7.5, true, ocCor, "Subtotal (R$)")
	f.Direita(x[5], y, 7.5, true, ocCor, "Desc. (R$)")
	f.Direita(x[6], y, 7.5, true, ocCor, "Total (R$)")
	y -= 4
	f.Linha(ocEsq, ocDir(), y, relatorio.CorLinha)
	return y - 12
}

func itensDaOC(f *relatorio.Folha, e Extraida, y float64, x []float64, paginas int) float64 {
	const pe = ocMargem + 80
	pagina := 1
	for i, it := range e.Itens {
		bruto := regras.DinheiroDe(it.Qtd * it.ValorUnit.Float())
		descLarg := x[2] - x[1] - 8
		partes := quebrarFolha(f, it.Descricao, descLarg, 7.5)
		if len(partes) == 0 {
			partes = []string{""}
		}
		alt := 12.0 + float64(len(partes))*11.0
		if y-alt < pe {
			f.Pagina()
			pagina++
			y = letreiroOC(f, e, pagina, paginas)
			y = cabecalhoTabelaOC(f, y, x)
		}
		n := i + 1
		base := y
		f.Texto(x[0], base, 7.5, false, ocCor, fmt.Sprintf("%d", n))
		f.TextoCortado(x[1], base, 7.5, descLarg, false, ocCor, partes[0])
		f.Direita(x[2], base, 7.5, false, ocCor, qtdBR(it.Qtd))
		f.Direita(x[3], base, 7.5, false, ocCor, it.ValorUnit.Reais())
		f.Direita(x[4], base, 7.5, false, ocCor, bruto.Reais())
		f.Direita(x[5], base, 7.5, false, ocCor, it.Desconto.Reais())
		f.Direita(x[6], base, 7.5, false, ocCor, it.Total.Reais())
		y -= 11
		for j := 1; j < len(partes); j++ {
			f.TextoCortado(x[1], y, 7.5, descLarg, false, ocCor, partes[j])
			y -= 11
		}
		un := it.Unidade
		if un == "" {
			un = "UN"
		}
		f.Texto(x[2]-28, y, 7.5, false, ocCor, un)
		y -= 13
	}
	return y
}

func totaisDaOC(f *relatorio.Folha, e Extraida, y float64, x []float64, paginas int) {
	if y < ocMargem+50 {
		f.Pagina()
		y = letreiroOC(f, e, paginas, paginas)
		y -= 20
	}
	y -= 6
	f.Texto(x[3]-40, y, 8, true, ocCor, "Subtotal")
	f.Direita(x[4], y, 8, true, ocCor, e.Subtotal.Reais())
	f.Direita(x[5], y, 8, true, ocCor, e.Desconto.Reais())
	f.Direita(x[6], y, 8, true, ocCor, (e.Subtotal - e.Desconto).Reais())
	y -= 13
	f.Texto(x[3]-40, y, 8, true, ocCor, "Frete")
	f.Direita(x[6], y, 8, true, ocCor, e.Frete.Reais())
	y -= 13
	f.Texto(x[3]-40, y, 8, true, ocCor, "Total")
	f.Direita(x[6], y, 8, true, ocCor, e.Total.Reais())
}

func estimarPaginas(e Extraida) int {
	// A 1ª folha leva o letreiro e os blocos (~8 itens, medido na 019731).
	n := len(e.Itens)
	if n <= 8 {
		return 1
	}
	return 1 + (n-8+11)/12
}

func qtdBR(v float64) string {
	return regras.DinheiroDe(v).Reais()
}

func dataParaTela(iso string) string {
	iso = strings.TrimSpace(iso)
	if iso == "" {
		return ""
	}
	if len(iso) == 10 && iso[4] == '-' {
		return iso[8:10] + "/" + iso[5:7] + "/" + iso[0:4]
	}
	return iso
}

func primeiroNaoVazio(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func quebrarFolha(f *relatorio.Folha, txt string, largura, tam float64) []string {
	txt = strings.TrimSpace(txt)
	if txt == "" {
		return nil
	}
	if largura <= 0 || f.Medir(txt, tam) <= largura {
		return []string{txt}
	}
	const maximo = 6
	var linhas []string
	atual := ""
	for _, palavra := range strings.Fields(txt) {
		tentativa := palavra
		if atual != "" {
			tentativa = atual + " " + palavra
		}
		if f.Medir(tentativa, tam) <= largura {
			atual = tentativa
			continue
		}
		if atual != "" {
			linhas = append(linhas, atual)
			if len(linhas) == maximo-1 {
				atual = palavra
				break
			}
		}
		atual = palavra
	}
	if atual != "" {
		linhas = append(linhas, atual)
	}
	if u := len(linhas) - 1; u >= 0 && f.Medir(linhas[u], tam) > largura {
		r := []rune(linhas[u])
		for len(r) > 1 {
			r = r[:len(r)-1]
			if f.Medir(string(r)+"…", tam) <= largura {
				linhas[u] = string(r) + "…"
				break
			}
		}
	}
	return linhas
}
