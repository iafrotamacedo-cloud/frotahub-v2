// rev 2 — o que sobrou do reparo manual, depois da substituição por arquivo
//
// ATÉ 11/09/2026, ESTE ARQUIVO TINHA AS ROTAS DE /reparo E /obras-centro
//
//	Existia uma tela própria (JanelaReparoOC) para digitar CNPJ do fornecedor
//	e escolher obra/faturamento por busca. O reparo híbrido (RepararOrdemOC +
//	documento.go) tomou o lugar dela em cima do MESMO editor de documento que
//	já existia — e ninguém mais chamava `POST/GET .../reparo` nem
//	`GET .../obras-centro`. Removidas (CORE-16): duas rotas fazendo a mesma
//	coisa por dois caminhos é o tipo de coisa que diverge sem ninguém notar.
//
//	Pedido do dono no mesmo dia: faturamento errado deixa de ser corrigível
//	por aqui — o CNPJ de faturamento pertence ao Obra Prima, e remendar só o
//	nosso lado deixa o registro de lá errado para sempre. A correção virou
//	SUBSTITUIR o arquivo (ver `substituicao.go`), não editar um campo.
//	`errosDaRejeicao` continua existindo porque `documento.go` ainda usa para
//	decidir que caixas mostrar no reparo híbrido (fornecedor, endereço).
package administrativo

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

// extraidaAtual monta o que temos hoje: o PDF relido + o que já está no banco.
func (m *Modulo) extraidaAtual(ctx context.Context, ordem map[string]any) (Extraida, error) {
	ex := extraidaDoBanco(ordem)
	sha, _ := ordem["arquivo_sha256"].(string)
	if sha == "" {
		return ex, nil
	}
	arq, err := m.contarUm(ctx, "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		return ex, nil
	}
	chave, _ := arq["chave_r2"].(string)
	pdf, err := m.arm.Baixar(ctx, chave)
	if err != nil {
		return ex, nil
	}
	lida, err := Ler(ctx, pdf)
	if err != nil {
		return ex, nil
	}
	ex = mesclarExtraida(lida, ex)
	return ex, nil
}

func extraidaDoBanco(ordem map[string]any) Extraida {
	var ex Extraida
	ex.Numero = fmt.Sprint(ordem["numero"])
	if ex.Numero == "<nil>" {
		ex.Numero = ""
	}
	ex.ObraCentroCusto = strCampo(ordem["obra_centro_custo"])
	ex.CompradorNome = strCampo(ordem["comprador_nome"])
	ex.CompradorCNPJ = strCampo(ordem["comprador_cnpj"])
	ex.CompradorInterno = strCampo(ordem["comprador_interno"])
	ex.CondPgto = strCampo(ordem["cond_pgto"])
	ex.FormaPgto = strCampo(ordem["forma_pgto"])
	ex.Data = interpretarData(strCampo(ordem["data"]))
	ex.PrevisaoEntrega = interpretarData(strCampo(ordem["previsao_entrega"]))
	ex.Subtotal = regras.DinheiroDe(numeroDeJSON(ordem["subtotal"]))
	ex.Desconto = regras.DinheiroDe(numeroDeJSON(ordem["desconto"]))
	ex.Frete = regras.DinheiroDe(numeroDeJSON(ordem["frete"]))
	ex.Total = regras.DinheiroDe(numeroDeJSON(ordem["total"]))
	return ex
}

func mesclarExtraida(lida, banco Extraida) Extraida {
	if banco.Numero != "" {
		lida.Numero = banco.Numero
	}
	if banco.ObraCentroCusto != "" {
		lida.ObraCentroCusto = banco.ObraCentroCusto
	}
	if banco.CompradorNome != "" {
		lida.CompradorNome = banco.CompradorNome
	}
	if banco.CompradorCNPJ != "" {
		lida.CompradorCNPJ = banco.CompradorCNPJ
	}
	return lida
}

func strCampo(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func fmtStatus(v any) string {
	return strings.TrimSpace(fmt.Sprint(v))
}

// errosDaRejeicao lê o texto gravado em `erro_leitura` e diz o que falta
// corrigir. `faturamento` não é mais editável no reparo híbrido (vira
// substituição de arquivo, ver `substituicao.go`) — o booleano continua
// existindo porque `documento.go` usa para decidir se mostra a caixa de
// fornecedor/endereço, ou se a OC nem chega a abrir o reparo (a lista já
// desvia para "Ver + Substituir" antes disso).
func errosDaRejeicao(motivo string) (fornecedor, faturamento, endereco bool) {
	m := strings.ToLower(motivo)
	if strings.Contains(m, "fornecedor") {
		fornecedor = true
	}
	if strings.Contains(m, "faturamento") || strings.Contains(m, "03720882") {
		faturamento = true
	}
	if strings.Contains(m, "endereço de cobrança") || strings.Contains(m, "endereco de cobranca") {
		endereco = true
	}
	return
}

// linhasMotivo quebra o motivo em linhas curtas para a barra do visor.
func linhasMotivo(motivo string) []string {
	if strings.TrimSpace(motivo) == "" {
		return []string{}
	}
	partes := strings.Split(motivo, ";")
	saida := make([]string, 0, len(partes))
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if p != "" {
			saida = append(saida, simplificarMotivoAPI(p))
		}
	}
	return saida
}

func simplificarMotivoAPI(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "fornecedor"):
		return "Fornecedor sem CNPJ"
	case strings.Contains(lower, "faturamento") && strings.Contains(lower, "não achei"):
		return "Faturamento sem CNPJ"
	case strings.Contains(lower, "faturamento"):
		return "CNPJ de faturamento errado"
	case strings.Contains(lower, "endereço de cobrança"):
		return "Endereço de cobrança errado"
	default:
		return strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return ' '
			}
			return r
		}, s))
	}
}
