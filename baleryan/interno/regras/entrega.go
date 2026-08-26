// rev 1 — a linha de entrega, que vem de cabeça para baixo
//
// A CONVENÇÃO DO FORNECEDOR
//
//	O serviço de entrega não tem preço de tabela: depende do que o aplicativo
//	cobrar naquela hora. Para conseguir emitir a nota mesmo assim, a Rodrigues
//	padronizou o item no sistema dela com preço unitário de R$ 1,00 e usa a
//	QUANTIDADE como valor. Uma entrega de R$ 12 sai como 12 × R$ 1,00.
//
//	Do lado dela é honesto e resolve. Do nosso lado, sai no documento do cliente
//	como "12 unidades de serviço de entrega" — que não é uma frase que alguém
//	assine sem perguntar.
//
// O QUE ESTA REGRA FAZ
//
//	Devolve a linha ao que ela significa: UMA entrega, custando o total. O
//	dinheiro não muda — 12 × 1,00 e 1 × 12,00 somam o mesmo — e o documento passa
//	a dizer a verdade.
//
// POR QUE OS DOIS CRITÉRIOS, E NÃO SÓ O NOME
//
//	Pelo nome sozinho, uma entrega cobrada de verdade a R$ 8,50 a unidade (se um
//	dia houver) seria achatada em uma linha só. Pelo preço sozinho, qualquer
//	material de R$ 1,00 comprado às dúzias viraria "1 unidade" — e aí o
//	documento mentiria sobre o que foi comprado.
//
//	Juntos, descrevem exatamente a convenção: o item que se chama entrega E cujo
//	unitário é o R$ 1,00 padronizado.
package regras

import "strings"

// umReal é o preço padronizado que denuncia a convenção.
var umReal = PrecoDe(1)

// NormalizarEntrega inverte quantidade e preço nas linhas de entrega.
//
// Devolve uma fatia nova: a original é o que está gravado no documento, e o
// documento é a nota do fornecedor — ela continua valendo o que o papel diz.
// Quem inverte é o ORÇAMENTO, que é peça nossa.
func NormalizarEntrega(linhas []LinhaDaNota) []LinhaDaNota {
	saida := make([]LinhaDaNota, len(linhas))
	copy(saida, linhas)
	for i, l := range saida {
		if !ehEntregaPorQuantidade(l) {
			continue
		}
		total := Total(l.Quantidade, l.Unitario)
		unitario, deu := PrecoQueFecha(QuantidadeDe(1), total)
		if !deu {
			// Não deveria acontecer com quantidade 1, mas inverter só quando a
			// conta volta é o que garante que a soma do orçamento não muda.
			continue
		}
		saida[i].Quantidade = QuantidadeDe(1)
		saida[i].Unitario = unitario
	}
	return saida
}

func ehEntregaPorQuantidade(l LinhaDaNota) bool {
	return l.Unitario == umReal && l.Quantidade > QuantidadeDe(1) && ehEntrega(l.Descricao)
}

// ehEntrega olha o nome sem depender de acento nem de caixa: a mesma descrição
// aparece como "SERVICO DE ENTREGA" na leitura por IA e pode vir com cedilha na
// próxima. Duas grafias da mesma coisa não podem virar duas regras.
func ehEntrega(descricao string) bool {
	return strings.Contains(semAcento(strings.ToUpper(descricao)), "ENTREGA")
}

func semAcento(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case 'Á', 'À', 'Â', 'Ã', 'Ä':
			b.WriteRune('A')
		case 'É', 'Ê', 'Ë':
			b.WriteRune('E')
		case 'Í', 'Î', 'Ï':
			b.WriteRune('I')
		case 'Ó', 'Ô', 'Õ', 'Ö':
			b.WriteRune('O')
		case 'Ú', 'Û', 'Ü':
			b.WriteRune('U')
		case 'Ç':
			b.WriteRune('C')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
