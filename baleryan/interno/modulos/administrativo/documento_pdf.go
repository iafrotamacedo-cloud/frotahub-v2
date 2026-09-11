// rev 2 — a Ordem de Compra desenhada, clone da OC 019731 (Obra Prima)
//
// POR QUE GERAR DE NOVO, E NÃO TAMPONAR O PDF ANTIGO
//
//	Editar "em cima" do arquivo original deixa o texto velho no fluxo. O pedido
//	foi substituir a folha sem vestígio: então ela é desenhada do zero, no
//	layout medido na OC 019731 de 10/09/2026 — marca, faixas cinzas, colunas,
//	tabela com grade.
//
//	Não leva faixa azul FrotaHub nem rodapé "gerado em". É a OC, não um
//	relatório nosso.
//
//	As coordenadas (x do texto, topo das faixas, colunas da tabela) saíram do
//	PDF original, não de um chute. Helvetica no lugar de Arial — é a fonte que
//	todo leitor já tem, e visualmente é a irmã.
package administrativo

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
)

//go:embed marca_oc.jpg
var marcaOC []byte

const (
	ocX0      = 19.44
	ocX1      = 576.0
	ocTextoX  = 109.4
	ocMeio    = 297.0
	ocValEsq  = 90.0
	ocValDir  = 411.9
	ocRotFim  = 88.0
	ocFaixaH  = 18.03
	ocFaixa   = "0.77 0.77 0.77"
	ocPreto   = "0 0 0"
	ocTam     = 9.0
	ocLinha   = 14.4
	ocAltItem = 28.8
)

func ocLarg() float64 { return ocX1 - ocX0 }

func yt(topo, tam float64) float64 {
	return relatorio.AlturaRetrato - topo - tam*0.78
}

func yCaixa(topo, alt float64) float64 {
	return relatorio.AlturaRetrato - topo - alt
}

// DesenharOC é o gerador público da folha — o editor e as ferramentas de
// teste (OCs_Teste) usam o mesmo desenho, para o PDF de teste ser o mesmo
// documento que a tela gera.
func DesenharOC(e Extraida) ([]byte, error) {
	return desenharOC(e)
}

func desenharOC(e Extraida) ([]byte, error) {
	e.RecalcularTotais()
	aplicarPadraoEmitente(&e)
	if strings.TrimSpace(e.Numero) == "" {
		return nil, fmt.Errorf("a ordem de compra precisa de número")
	}
	if len(e.Itens) == 0 {
		return nil, fmt.Errorf("a ordem de compra precisa de ao menos um item")
	}
	for i := range e.Itens {
		if strings.TrimSpace(e.Itens[i].Unidade) == "" {
			e.Itens[i].Unidade = "UN"
		}
	}

	f := relatorio.NovaFolha()
	paginas := estimarPaginas(e)
	topo := letreiroOC(f, e, 1, paginas)
	topo = blocosDaOC(f, e, topo)
	xs := colunasDaTabelaOC()
	topo = cabecalhoTabelaOC(f, topo, xs)
	topo = itensDaOC(f, e, topo, xs, paginas)
	totaisDaOC(f, e, topo, xs, paginas)
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
	_ = f.Imagem(25.92, yCaixa(24.3, 32.4), 64.8, 32.4, marcaOC)

	f.Texto(ocTextoX, yt(21.5, 12), 12, true, ocPreto, e.EmitenteRazao)
	f.Direita(ocX1, yt(20.7, 8.2), 8.2, false, ocPreto, e.DataImpressao)

	f.TextoCortado(ocTextoX, yt(38.2, 8), 8, 400, false, ocPreto, e.EmitenteEndereco)
	f.Texto(519.0, yt(36.1, 8.2), 8.2, false, ocPreto, "Página")
	if paginas < 1 {
		paginas = 1
	}
	f.Texto(554.6, yt(36.1, 8.2), 8.2, true, ocPreto, fmt.Sprintf("%d/%d", pagina, paginas))

	contato := e.EmitenteContato
	if e.EmitenteCNPJ != "" {
		if contato != "" {
			contato += " - "
		}
		contato += "CNPJ: " + e.EmitenteCNPJ
	}
	f.TextoCortado(ocTextoX, yt(49.2, 8), 8, ocX1-ocTextoX, false, ocPreto, contato)

	f.Texto(ocTextoX, yt(73.3, 12), 12, true, ocPreto, "ORDEM DE COMPRA "+e.Numero)
	if e.TituloObra != "" {
		f.TextoCortado(ocTextoX, yt(87.1, 12), 12, ocX1-ocTextoX, true, ocPreto, e.TituloObra)
	}
	return 114.44
}

func faixa(f *relatorio.Folha, topo float64) {
	f.Caixa(ocX0, yCaixa(topo, ocFaixaH), ocLarg(), ocFaixaH, ocFaixa)
}

func faixaTitulo(f *relatorio.Folha, topo, x float64, txt string) {
	faixa(f, topo)
	f.Texto(x, yt(topo+4.36, ocTam), ocTam, true, ocPreto, txt)
}

func blocosDaOC(f *relatorio.Folha, e Extraida, topo float64) float64 {
	faixa(f, topo)
	f.Texto(22.3, yt(topo+4.36, ocTam), ocTam, true, ocPreto, "DADOS DA ORDEM DE COMPRA")
	f.Texto(182.0, yt(topo+4.36, ocTam), ocTam, true, ocPreto, e.Numero)
	y := topo + ocFaixaH + 5.6

	f.Texto(60.3, yt(y, ocTam), ocTam, true, ocPreto, "Data:")
	f.Texto(ocValEsq, yt(y, ocTam), ocTam, false, ocPreto, dataParaTela(e.Data))
	f.Texto(316.2, yt(y, ocTam), ocTam, true, ocPreto, "Previsão da entrega:")
	f.Texto(ocValDir, yt(y, ocTam), ocTam, false, ocPreto, dataParaTela(e.PrevisaoEntrega))
	y += ocLinha

	f.Texto(29.8, yt(y, ocTam), ocTam, true, ocPreto, "Cond. pgto.: "+e.CondPgto)
	f.Texto(349.7, yt(y, ocTam), ocTam, true, ocPreto, "Forma pgto.: "+e.FormaPgto)
	y += ocLinha + 2.2

	obs := "Observação:"
	if strings.TrimSpace(e.Observacao) != "" {
		obs += " " + e.Observacao
	}
	f.TextoCortado(26.8, yt(y, ocTam), ocTam, ocX1-26.8, true, ocPreto, obs)
	y += 19.4

	faixaTitulo(f, y, 22.3, "RESPONSÁVEL PELA COMPRA")
	y += ocFaixaH + 5.9
	f.Direita(ocRotFim, yt(y, ocTam), ocTam, true, ocPreto, "Nome:")
	f.Texto(91.4, yt(y, ocTam), ocTam, false, ocPreto, primeiroNaoVazio(e.ResponsavelNome, e.EmitenteRazao))
	f.Direita(408, yt(y, ocTam), ocTam, true, ocPreto, "Comprador:")
	f.Texto(ocValDir, yt(y, ocTam), ocTam, false, ocPreto, e.CompradorInterno)
	y += ocLinha
	email := e.ResponsavelEmail
	if email == "" {
		email = "compras@frotamacedo.com.br"
	}
	f.Texto(377.7, yt(y, ocTam), ocTam, true, ocPreto, "Email: "+email)
	y += 31.9

	faixaTitulo(f, y, 22.3, "DADOS DO FATURAMENTO")
	y += ocFaixaH + 7.6
	y = parNomeEndereco(f, y, e.CompradorNome, e.FaturamentoEndereco)
	f.Direita(ocRotFim, yt(y, ocTam), ocTam, true, ocPreto, "CNPJ:")
	f.Texto(91.4, yt(y, ocTam), ocTam, false, ocPreto, e.CompradorCNPJ)
	y += ocLinha
	f.Direita(ocRotFim, yt(y, ocTam), ocTam, true, ocPreto, "I.E.:")
	f.Texto(91.4, yt(y, ocTam), ocTam, false, ocPreto, e.FaturamentoIE)
	y += 25.5

	faixaTitulo(f, y, 22.3, "DADOS DO FORNECEDOR")
	y += ocFaixaH + 4.6
	yNome := y
	linhasEnd := quebrarFolha(f, e.FornecedorEndereco, 164, ocTam)
	f.Direita(ocRotFim, yt(y, ocTam), ocTam, true, ocPreto, "Nome:")
	f.TextoCortado(91.4, yt(y, ocTam), ocTam, 250, false, ocPreto, e.FornecedorNome)
	f.Texto(360.7, yt(y, ocTam), ocTam, true, ocPreto, "Endereço:")
	for i, ln := range linhasEnd {
		f.Texto(ocValDir, yt(yNome+float64(i)*10.4, ocTam), ocTam, false, ocPreto, ln)
	}
	y += ocLinha
	f.Direita(ocRotFim, yt(y, ocTam), ocTam, true, ocPreto, "CNPJ:")
	f.Texto(91.4, yt(y, ocTam), ocTam, false, ocPreto, e.FornecedorCNPJ)
	y += ocLinha
	f.Texto(44.2, yt(y, ocTam), ocTam, true, ocPreto, "Telefone: "+e.FornecedorTelefone)
	y += ocLinha
	f.Texto(39.7, yt(y, ocTam), ocTam, true, ocPreto, "Vendedor: "+e.FornecedorVendedor)
	y += ocLinha
	f.Texto(54.2, yt(y, ocTam), ocTam, true, ocPreto, "E-mail: "+e.FornecedorEmail)
	y += 32.0

	faixa(f, y)
	f.Texto(22.3, yt(y+4.36, ocTam), ocTam, true, ocPreto, "OBRA/CENTRO DE CUSTO:")
	f.TextoCortado(155.6, yt(y+4.36, ocTam), ocTam, 300, true, ocPreto, e.ObraCentroCusto)
	f.Texto(472.7, yt(y+4.36, ocTam), ocTam, true, ocPreto, "CNO: "+e.CNO)
	y += ocFaixaH + 2.1

	f.Texto(25.2, yt(y, ocTam), ocTam, true, ocPreto, "ENDEREÇO ENTREGA:")
	f.Texto(306.7, yt(y, ocTam), ocTam, true, ocPreto, "ENDEREÇO COBRANÇA:")
	y += ocLinha
	linhasE := quebrarFolha(f, e.EnderecoEntrega, 200, ocTam)
	linhasC := quebrarFolha(f, e.EnderecoCobranca, 164, ocTam)
	n := len(linhasE)
	if len(linhasC) > n {
		n = len(linhasC)
	}
	f.Texto(38.8, yt(y, ocTam), ocTam, true, ocPreto, "Endereço:")
	f.Texto(360.7, yt(y, ocTam), ocTam, true, ocPreto, "Endereço:")
	for i := 0; i < n; i++ {
		yy := y + float64(i)*10.4
		if i < len(linhasE) {
			f.Texto(ocValEsq, yt(yy, ocTam), ocTam, false, ocPreto, linhasE[i])
		}
		if i < len(linhasC) {
			f.Texto(ocValDir, yt(yy, ocTam), ocTam, false, ocPreto, linhasC[i])
		}
	}
	if n < 1 {
		n = 1
	}
	// Divisor vertical entre entrega e cobrança, como na 019731.
	altDiv := 14.4 + float64(n)*10.4 + 8
	f.Caixa(ocMeio, yCaixa(y-14.4, altDiv), 0.72, altDiv, ocFaixa)
	y += float64(n)*10.4 + 14.4
	f.Texto(25.2, yt(y, ocTam), ocTam, true, ocPreto, "   Recebedor: "+e.Recebedor)
	return y + 22.0
}

func parNomeEndereco(f *relatorio.Folha, y float64, nome, endereco string) float64 {
	f.Direita(ocRotFim, yt(y, ocTam), ocTam, true, ocPreto, "Nome:")
	f.TextoCortado(91.4, yt(y, ocTam), ocTam, 250, false, ocPreto, nome)
	f.Texto(360.7, yt(y, ocTam), ocTam, true, ocPreto, "Endereço:")
	linhas := quebrarFolha(f, endereco, 164, ocTam)
	for i, ln := range linhas {
		f.Texto(ocValDir, yt(y+float64(i)*10.4, ocTam), ocTam, false, ocPreto, ln)
	}
	return y + ocLinha
}

// colunas: N | desc | qtd | unit | subtotal | desc$ | total | (fim)
func colunasDaTabelaOC() []float64 {
	return []float64{19.44, 40.62, 223.20, 295.20, 360.0, 432.0, 496.80, 576.0}
}

func cabecalhoTabelaOC(f *relatorio.Folha, topo float64, x []float64) float64 {
	f.Caixa(ocX0, yCaixa(topo, ocFaixaH), ocLarg(), ocFaixaH, ocFaixa)
	ty := yt(topo+4.36, ocTam)
	// "N. Item" é um marcador da leitura — tem que sair com um espaço só,
	// senão o ExtrairDoTexto não acha o começo da tabela.
	f.Texto(25.5, ty, ocTam, true, ocPreto, "N. Item")
	f.Texto(250.2, ty, ocTam, true, ocPreto, "Qtd.")
	f.Texto(307.6, ty, ocTam, true, ocPreto, "Unit. (R$)")
	f.Texto(368.0, ty, ocTam, true, ocPreto, "Subtotal (R$)")
	f.Texto(442.4, ty, ocTam, true, ocPreto, "Desc. (R$)")
	f.Texto(531.6, ty, ocTam, true, ocPreto, "Total (R$)")
	return topo + ocFaixaH
}

func molduraLinha(f *relatorio.Folha, topo, alt float64, x []float64) {
	yb := yCaixa(topo, alt)
	for i := 0; i < len(x)-1; i++ {
		f.Moldura(x[i], yb, x[i+1]-x[i], alt, ocFaixa)
	}
}

func itensDaOC(f *relatorio.Folha, e Extraida, topo float64, x []float64, paginas int) float64 {
	const pe = 800.0
	pagina := 1
	for i, it := range e.Itens {
		bruto := regras.DinheiroDe(it.Qtd * it.ValorUnit.Float())
		descLarg := x[2] - x[1] - 6
		partes := quebrarFolha(f, it.Descricao, descLarg, ocTam)
		if len(partes) == 0 {
			partes = []string{""}
		}
		linhas := len(partes)
		if linhas < 2 {
			linhas = 2 // a UN ocupa a segunda linha, como na 019731
		}
		alt := 18.0 + float64(linhas-1)*10.4
		if alt < ocAltItem {
			alt = ocAltItem
		}
		if topo+alt > pe {
			f.Pagina()
			pagina++
			topo = letreiroOC(f, e, pagina, paginas)
			topo = cabecalhoTabelaOC(f, topo, x)
		}
		molduraLinha(f, topo, alt, x)
		n := i + 1
		ty := topo + 3.6
		f.Texto(27.5, yt(ty, ocTam), ocTam, false, ocPreto, fmt.Sprintf("%d", n))
		f.TextoCortado(43.5, yt(ty, ocTam), ocTam, descLarg, false, ocPreto, partes[0])
		f.Direita(275, yt(ty, ocTam), ocTam, false, ocPreto, qtdBR(it.Qtd))
		f.Direita(x[4]-6, yt(ty, ocTam), ocTam, false, ocPreto, it.ValorUnit.Reais())
		f.Direita(x[5]-6, yt(ty, ocTam), ocTam, false, ocPreto, bruto.Reais())
		f.Direita(x[6]-6, yt(ty, ocTam), ocTam, false, ocPreto, it.Desconto.Reais())
		f.Direita(x[7]-6, yt(ty, ocTam), ocTam, false, ocPreto, it.Total.Reais())
		for j := 1; j < len(partes); j++ {
			f.TextoCortado(43.5, yt(ty+float64(j)*10.4, ocTam), ocTam, descLarg, false, ocPreto, partes[j])
		}
		un := it.Unidade
		if un == "" {
			un = "UN"
		}
		f.Texto(252.0, yt(ty+11.5, ocTam), ocTam, false, ocPreto, un)
		topo += alt
	}
	return topo
}

func totaisDaOC(f *relatorio.Folha, e Extraida, topo float64, x []float64, paginas int) {
	if topo > 760 {
		f.Pagina()
		topo = letreiroOC(f, e, paginas, paginas)
		topo += 20
	}
	topo += 8
	// Label e números na mesma fonte — Helvetica vs Helvetica-Bold desloca
	// o topo do glifo em 0,1pt e o simulador (agrupa por y arredondado a
	// 0,1) separava "Subtotal" dos valores.
	ty := yt(topo, ocTam)
	f.Texto(298.1, ty, ocTam, true, ocPreto, "Subtotal")
	f.Direita(x[5]-6, ty, ocTam, true, ocPreto, e.Subtotal.Reais())
	f.Direita(x[6]-6, ty, ocTam, true, ocPreto, e.Desconto.Reais())
	f.Direita(x[7]-6, ty, ocTam, true, ocPreto, (e.Subtotal - e.Desconto).Reais())
	topo += 18.8
	ty = yt(topo, ocTam)
	f.Texto(298.1, ty, ocTam, true, ocPreto, "Frete")
	f.Direita(x[7]-6, ty, ocTam, true, ocPreto, e.Frete.Reais())
	topo += 18.8
	ty = yt(topo, ocTam)
	f.Texto(298.1, ty, ocTam, true, ocPreto, "Total")
	f.Direita(x[7]-6, ty, ocTam, true, ocPreto, e.Total.Reais())
}

func estimarPaginas(e Extraida) int {
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
