// rev 1 — camada 1: o XML da NFe
//
// Quando o fornecedor manda o XML, acabou a conversa: é o documento fiscal
// original, com a chave de acesso, os itens e os valores exatos. Nenhuma outra
// camada chega perto disso, e ela custa zero.
//
// `encoding/xml` já vem com o Go. Nenhuma dependência entra por causa disto.
package leitor

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

var ErrNaoEhXML = errors.New("isto não é um XML de NFe")

// A estrutura abaixo é a NFe 4.00 podada: só o que o orçamento precisa. Campos
// que não usamos não são declarados — o parser do Go ignora o resto sozinho.
type nfeProc struct {
	XMLName xml.Name `xml:"nfeProc"`
	NFe     nfe      `xml:"NFe"`
}

type nfe struct {
	XMLName xml.Name `xml:"NFe"`
	InfNFe  infNFe   `xml:"infNFe"`
}

type infNFe struct {
	// O atributo Id vem como "NFe" + os 44 dígitos da chave.
	ID  string `xml:"Id,attr"`
	Ide struct {
		NNF   string `xml:"nNF"`
		Serie string `xml:"serie"`
		DhEmi string `xml:"dhEmi"`
		DEmi  string `xml:"dEmi"` // versões antigas
	} `xml:"ide"`
	Emit struct {
		CNPJ  string `xml:"CNPJ"`
		XNome string `xml:"xNome"`
	} `xml:"emit"`
	Dest struct {
		CNPJ string `xml:"CNPJ"`
		CPF  string `xml:"CPF"`
	} `xml:"dest"`
	Det []struct {
		Prod struct {
			CProd  string `xml:"cProd"`
			XProd  string `xml:"xProd"`
			UCom   string `xml:"uCom"`
			QCom   string `xml:"qCom"`
			VUnCom string `xml:"vUnCom"`
			VProd  string `xml:"vProd"`
		} `xml:"prod"`
	} `xml:"det"`
	Total struct {
		ICMSTot struct {
			VNF    string `xml:"vNF"`
			VFrete string `xml:"vFrete"`
		} `xml:"ICMSTot"`
	} `xml:"total"`
	InfAdic struct {
		InfCpl string `xml:"infCpl"`
	} `xml:"infAdic"`
}

// DoXML lê o XML de uma NFe.
//
// Aceita tanto o arquivo processado (`nfeProc`, com o protocolo de autorização)
// quanto a `NFe` solta — os fornecedores mandam os dois, e recusar um deles por
// formalidade seria mandar o usuário converter arquivo à mão.
func DoXML(bruto []byte) (*Leitura, error) {
	var raiz nfeProc
	inf := infNFe{}

	if err := xml.Unmarshal(bruto, &raiz); err == nil && raiz.NFe.InfNFe.ID != "" {
		inf = raiz.NFe.InfNFe
	} else {
		var solta nfe
		if err := xml.Unmarshal(bruto, &solta); err != nil || solta.InfNFe.ID == "" {
			return nil, ErrNaoEhXML
		}
		inf = solta.InfNFe
	}

	l := &Leitura{
		Tipo:             "nf",
		Numero:           strings.TrimSpace(inf.Ide.NNF),
		Serie:            strings.TrimSpace(inf.Ide.Serie),
		ChaveAcesso:      SoDigitos(inf.ID),
		EmitenteCNPJ:     SoDigitos(inf.Emit.CNPJ),
		EmitenteNome:     Enxugar(inf.Emit.XNome),
		DestinatarioCNPJ: SoDigitos(inf.Dest.CNPJ + inf.Dest.CPF),
		Observacao:       Enxugar(inf.InfAdic.InfCpl),
		Camada:           DoXMLdaNota,
		// Um é o único lugar do sistema onde a confiança pode ser 1: o documento
		// fiscal original não é interpretação de ninguém.
		Confianca: 1,
	}

	if len(l.ChaveAcesso) != 44 {
		return nil, fmt.Errorf("%w: a chave de acesso veio com %d dígitos", ErrNaoEhXML, len(l.ChaveAcesso))
	}

	// A data vem "2026-08-04T00:00:00-03:00" ou, nas versões antigas, "2026-08-04".
	// Cortamos no T e não convertemos nada — é o que evita o dia a menos.
	quando := inf.Ide.DhEmi
	if quando == "" {
		quando = inf.Ide.DEmi
	}
	if i := strings.IndexByte(quando, 'T'); i > 0 {
		quando = quando[:i]
	}
	if len(quando) >= 10 {
		l.Emissao = quando[:10]
	}

	l.ValorTotal, _ = Decimal(inf.Total.ICMSTot.VNF)
	l.ValorFrete, _ = Decimal(inf.Total.ICMSTot.VFrete)

	for _, d := range inf.Det {
		q, _ := Decimal(d.Prod.QCom)
		u, _ := Decimal(d.Prod.VUnCom)
		tot, _ := Decimal(d.Prod.VProd)
		l.Itens = append(l.Itens, Item{
			Codigo:     strings.TrimSpace(d.Prod.CProd),
			Descricao:  Enxugar(d.Prod.XProd),
			Unidade:    strings.TrimSpace(d.Prod.UCom),
			Quantidade: q,
			Unitario:   u,
			Total:      tot,
		})
	}
	// A confiança do XML NÃO é recalculada: ela é 1 porque o documento é o
	// original, não porque alguém somou os itens.
	l.Arrumar()
	return l, nil
}
