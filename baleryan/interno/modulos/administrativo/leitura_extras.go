// rev 1 — campos extras da OC, lidos pelas mesmas palavras-chave
//
// A leitura de negócio (filtros, banco) não precisa de endereço nem de
// e-mail. O editor de documento precisa: é o que aparece na folha, e o que
// a pessoa pode corrigir. Tudo aqui é por marcador, nunca por coordenada —
// outros modelos de OC, com as mesmas palavras, ainda preenchem o que tiverem.
package administrativo

import (
	"regexp"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

var (
	reOrdemTitulo  = regexp.MustCompile(`(?m)^\s*ORDEM DE COMPRA\s+\d+\s*$`)
	reLinhaData    = regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)
	reCNPJEmitente = regexp.MustCompile(`CNPJ:\s*([\d./\-]+)`)
	rePaginaNoMeio = regexp.MustCompile(`(?i)\s*P[aá]gina\s+\d+\s*/\s*\d+\s*`)
	reCNO          = regexp.MustCompile(`(?i)CNO:\s*(\S(?:.*\S)?)`)
	reIE           = regexp.MustCompile(`(?i)I\.?E\.?:\s*(\S(?:.*\S)?)`)
)

// RecalcularTotais fecha a conta pelos itens: subtotal = soma (qtd × unitário),
// total do item = subtotal − desconto, total da OC = subtotal − descontos +
// frete. O PDF e o banco usam este resultado, não o que a tela mandou.
func (e *Extraida) RecalcularTotais() {
	var sub, desc regras.Dinheiro
	for i := range e.Itens {
		it := &e.Itens[i]
		bruto := regras.DinheiroDe(it.Qtd * it.ValorUnit.Float())
		total := bruto - it.Desconto
		if total < 0 {
			total = 0
		}
		it.Total = total
		sub += bruto
		desc += it.Desconto
	}
	e.Subtotal = sub
	e.Desconto = desc
	e.Total = sub - desc + e.Frete
}

func extrairCamposExtras(texto string, e *Extraida) {
	extrairEmitente(texto, e)
	e.Observacao = valorAposRotulo(blocoEntreVar(texto,
		[]string{"Observação:", "Observacao:"},
		[]string{"RESPONSÁVEL PELA COMPRA", "RESPONSAVEL PELA COMPRA", "DADOS DO FATURAMENTO"}),
		[]string{})

	resp := blocoEntreVar(texto,
		[]string{"RESPONSÁVEL PELA COMPRA", "RESPONSAVEL PELA COMPRA"},
		[]string{"DADOS DO FATURAMENTO"})
	e.ResponsavelNome = nomeDoBloco(resp)
	e.ResponsavelEmail = primeiroRotulo(resp, "Email:", "E-mail:", "E-Mail:")

	fat := blocoEntreVar(texto, []string{"DADOS DO FATURAMENTO"}, []string{"DADOS DO FORNECEDOR"})
	e.FaturamentoIE = primeiroRotulo(fat, "I.E.:", "IE:", "I.E:")
	if e.FaturamentoIE == "" {
		if m := reIE.FindStringSubmatch(fat); m != nil {
			e.FaturamentoIE = strings.TrimSpace(m[1])
		}
	}
	e.FaturamentoEndereco = enderecoDoBloco(fat)

	forn := blocoEntreVar(texto, []string{"DADOS DO FORNECEDOR"},
		[]string{"OBRA/CENTRO DE CUSTO", "OBRA / CENTRO DE CUSTO"})
	e.FornecedorTelefone = primeiroRotulo(forn, "Telefone:")
	e.FornecedorVendedor = primeiroRotulo(forn, "Vendedor:")
	e.FornecedorEmail = primeiroRotulo(forn, "E-mail:", "Email:", "E-Mail:")
	e.FornecedorEndereco = enderecoDoBloco(forn)

	obraLinha := ""
	if m := regexp.MustCompile(`(?mi)^\s*OBRA\s*/\s*CENTRO DE CUSTO:[^\n]*`).FindString(texto); m != "" {
		obraLinha = m
	}
	if m := reCNO.FindStringSubmatch(obraLinha); m != nil {
		e.CNO = strings.TrimSpace(m[1])
	}

	entregaBloco := blocoEntreVar(texto,
		[]string{"ENDEREÇO ENTREGA", "ENDERECO ENTREGA"},
		[]string{"N. Item"})
	esq, dir := duasColunas(entregaBloco)
	if !strings.Contains(strings.ToUpper(entregaBloco), "COBRAN") {
		esq = entregaBloco
		dir = ""
	}
	e.EnderecoEntrega = enderecoDoBloco(esq)
	e.Recebedor = primeiroRotulo(esq, "Recebedor:")
	e.EnderecoCobranca = enderecoDoBloco(dir)
	if e.EnderecoCobranca == "" {
		cob := blocoEntreVar(texto,
			[]string{"ENDEREÇO COBRANÇA", "ENDERECO COBRANCA", "ENDEREÇO COBRANCA", "ENDERECO COBRANÇA"},
			[]string{"N. Item", "Recebedor:"})
		if e.EnderecoCobranca == "" {
			e.EnderecoCobranca = enderecoDoBloco(cob)
		}
	}
}

func extrairEmitente(texto string, e *Extraida) {
	i, _ := indiceMarcador(texto, "DADOS DA ORDEM DE COMPRA")
	cab := texto
	if i >= 0 {
		cab = texto[:i]
	}
	if m := regexp.MustCompile(`\d{2}/\d{2}/\d{4}`).FindString(cab); m != "" {
		e.DataImpressao = m
	}
	if m := reCNPJEmitente.FindStringSubmatch(cab); m != nil {
		e.EmitenteCNPJ = strings.TrimSpace(m[1])
	}

	var uteis []string
	for _, ln := range strings.Split(cab, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || reLinhaData.MatchString(t) {
			continue
		}
		uteis = append(uteis, t)
	}
	for _, t := range uteis {
		if strings.HasPrefix(strings.ToUpper(t), "ORDEM DE COMPRA") {
			continue
		}
		if rePaginaNoMeio.MatchString(t) {
			e.EmitenteEndereco = strings.TrimSpace(rePaginaNoMeio.ReplaceAllString(t, " "))
			continue
		}
		if strings.Contains(t, "CNPJ:") {
			contato := t
			if idx := strings.Index(contato, " - CNPJ:"); idx >= 0 {
				contato = contato[:idx]
			} else if idx := strings.Index(contato, "CNPJ:"); idx >= 0 {
				contato = strings.TrimSpace(contato[:idx])
				contato = strings.TrimRight(contato, "- ")
			}
			e.EmitenteContato = strings.TrimSpace(contato)
			continue
		}
		if e.EmitenteRazao == "" {
			e.EmitenteRazao = t
			continue
		}
		if e.TituloObra == "" && !strings.Contains(strings.ToUpper(t), "CNPJ") {
			e.TituloObra = t
		}
	}
	// O título da obra é a linha DEPOIS de "ORDEM DE COMPRA N", não a razão
	// social. Se a razão ficou no título, troca.
	if loc := reOrdemTitulo.FindStringIndex(cab); loc != nil {
		resto := cab[loc[1]:]
		for _, ln := range strings.Split(resto, "\n") {
			t := strings.TrimSpace(ln)
			if t == "" {
				continue
			}
			e.TituloObra = t
			break
		}
	}
}

func primeiroRotulo(bloco string, rotulos ...string) string {
	for _, ln := range strings.Split(bloco, "\n") {
		for _, col := range partirColunas(ln) {
			for _, r := range rotulos {
				if i := indexFold(col, r); i >= 0 {
					v := strings.TrimSpace(col[i+len(r):])
					if v != "" {
						return v
					}
				}
			}
		}
	}
	return ""
}

func valorAposRotulo(bloco string, _ []string) string {
	return strings.TrimSpace(bloco)
}

func enderecoDoBloco(bloco string) string {
	var partes []string
	pegando := false
	for _, ln := range strings.Split(bloco, "\n") {
		cols := partirColunas(ln)
		for ci, col := range cols {
			if i := indexFold(col, "Endereço:"); i >= 0 || indexFold(col, "Endereco:") >= 0 {
				rot := "Endereço:"
				if j := indexFold(col, "Endereco:"); j >= 0 && (i < 0 || j < i) {
					i, rot = j, "Endereco:"
				}
				v := strings.TrimSpace(col[i+len(rot):])
				if v != "" {
					partes = append(partes, v)
				}
				pegando = true
				// o resto das colunas desta linha (mais à direita) também é
				// continuação do endereço no layout de duas colunas.
				for _, extra := range cols[ci+1:] {
					if temRotuloConhecido(extra) {
						continue
					}
					if t := strings.TrimSpace(extra); t != "" {
						partes = append(partes, t)
					}
				}
				continue
			}
			if !pegando {
				continue
			}
			if temRotuloConhecido(col) {
				// CNPJ/Telefone à esquerda, continuação do endereço à direita.
				continue
			}
			if t := strings.TrimSpace(col); t != "" {
				partes = append(partes, t)
			}
		}
	}
	return strings.TrimSpace(strings.Join(partes, " "))
}

func temRotuloConhecido(s string) bool {
	s = strings.TrimSpace(s)
	rotulos := []string{
		"Nome:", "CNPJ:", "I.E.:", "IE:", "I.E:", "Telefone:", "Vendedor:",
		"E-mail:", "Email:", "E-Mail:", "Comprador:", "Recebedor:", "CNO:",
		"Data:", "Observação:", "Observacao:", "Cond. pgto.:", "Forma pgto.:",
	}
	for _, r := range rotulos {
		if strings.HasPrefix(s, r) || indexFold(s, r) == 0 {
			return true
		}
	}
	return false
}

func partirColunas(linha string) []string {
	linha = strings.TrimRight(linha, " \t")
	if strings.TrimSpace(linha) == "" {
		return nil
	}
	partes := regexp.MustCompile(`\s{5,}`).Split(linha, -1)
	var saida []string
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if p != "" {
			saida = append(saida, p)
		}
	}
	if len(saida) == 0 {
		return []string{strings.TrimSpace(linha)}
	}
	return saida
}

func duasColunas(bloco string) (esq, dir string) {
	var bEsq, bDir strings.Builder
	for _, ln := range strings.Split(bloco, "\n") {
		cols := partirColunas(ln)
		switch len(cols) {
		case 0:
			continue
		case 1:
			// Linha só à direita (muita folga à esquerda) cai na cobrança.
			if len(ln)-len(strings.TrimLeft(ln, " ")) > 40 && bDir.Len() > 0 {
				bDir.WriteString(cols[0])
				bDir.WriteByte('\n')
				continue
			}
			bEsq.WriteString(cols[0])
			bEsq.WriteByte('\n')
		default:
			bEsq.WriteString(cols[0])
			bEsq.WriteByte('\n')
			bDir.WriteString(strings.Join(cols[1:], " "))
			bDir.WriteByte('\n')
		}
	}
	return bEsq.String(), bDir.String()
}

func indexFold(s, sub string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(sub))
}
