// rev 1 — o mesmo número escrito de três jeitos
//
// O DEFEITO QUE ESTE ARQUIVO GUARDA
//
//	Em 26/08/2026 a mesma nota entrou duas vezes com dois números: `9160`, na
//	leitura por regex, e `000.009.160`, na leitura pela IA — que copiou como
//	está impresso na DANFE. A chave de acesso das duas era a mesma.
//
//	A trava de duplicidade compara número com número. Duas grafias são duas
//	notas, dois orçamentos, e a loja pagando o mesmo material duas vezes sem
//	nada apitar.
package leitor

import "testing"

func TestNumeroLimpoTiraEnfeiteDeImpressao(t *testing.T) {
	casos := map[string]string{
		"000.009.160": "9160",  // o caso real de 26/08/2026
		"9160":        "9160",  // já limpo, não mexe
		"000009160":   "9160",  // zeros de largura de campo
		" 19.702 ":    "19702", // espaço da coluna
		"1":           "1",
		"":            "",
	}
	for entrada, esperado := range casos {
		if veio := NumeroLimpo(entrada); veio != esperado {
			t.Errorf("NumeroLimpo(%q) = %q, e a nota é a %q", entrada, veio, esperado)
		}
	}
}

// UM NÚMERO QUE É SÓ ZEROS CONTINUA EXISTINDO
//
//	Cortar zeros à esquerda de "000" daria string vazia — e vazio, no banco,
//	quer dizer "não achei o número". São coisas diferentes.
func TestNumeroLimpoNaoApagaOZero(t *testing.T) {
	if veio := NumeroLimpo("000"); veio != "0" {
		t.Errorf("NumeroLimpo(\"000\") = %q, esperava \"0\"", veio)
	}
}

// O QUE NÃO É SÓ DÍGITO E PONTO PASSA INTEIRO
//
//	Barra e letra podem ser parte da identidade da nota. Um número feio que
//	confere é melhor que um número bonito que alguém inventou.
func TestNumeroLimpoNaoInventa(t *testing.T) {
	for _, cru := range []string{"123/2026", "NF-A1", "12-345", "SÉRIE2"} {
		if veio := NumeroLimpo(cru); veio != cru {
			t.Errorf("NumeroLimpo(%q) mexeu e virou %q", cru, veio)
		}
	}
}

// Arrumar tem que valer para TODAS as camadas, não só para a que deu problema.
func TestArrumarNormalizaTodosOsCampos(t *testing.T) {
	l := &Leitura{
		Numero:           "000.009.160",
		Serie:            "001",
		DAV:              "0092.080",
		ChaveAcesso:      "2326 0714 7886 3300 0110 5500 1000 0091 6010 0005 5288",
		EmitenteCNPJ:     "14.788.633/0001-10",
		DestinatarioCNPJ: "12.345.678/0001-99",
	}
	l.Arrumar()

	if l.Numero != "9160" || l.Serie != "1" || l.DAV != "92080" {
		t.Errorf("número/série/DAV saíram %q/%q/%q", l.Numero, l.Serie, l.DAV)
	}
	if len(l.ChaveAcesso) != 44 {
		t.Errorf("a chave ficou com %d dígitos: %q", len(l.ChaveAcesso), l.ChaveAcesso)
	}
	if l.EmitenteCNPJ != "14788633000110" {
		t.Errorf("o CNPJ do emitente saiu %q", l.EmitenteCNPJ)
	}
	if l.DestinatarioCNPJ != "12345678000199" {
		t.Errorf("o CNPJ do destinatário saiu %q", l.DestinatarioCNPJ)
	}
}

// A CHAVE DE ACESSO CARREGA O NÚMERO DENTRO DELA
//
//	Posições 26 a 34 da chave são o número da nota, com nove dígitos. É a prova
//	independente de que "000.009.160" e "9160" são a mesma coisa — e é de graça.
func TestONumeroBateComAChaveDeAcesso(t *testing.T) {
	const chave = "23260714788633000110550010000091601000055288"
	if veio := NumeroLimpo(chave[25:34]); veio != "9160" {
		t.Errorf("a chave diz que a nota é a %q, e NumeroLimpo devolveu %q", "9160", veio)
	}
}

// Item com total zero não é item barato: é item que não foi lido.
func TestContaNaoFechaComItemZerado(t *testing.T) {
	// O caso real de LEDS NF 19650: nota de 112,60 com um item vazio.
	l := &Leitura{ValorTotal: 112.60, Itens: []Item{{Descricao: "LED SPOT", Total: 0}}}
	if ContaFecha(l) {
		t.Error("a conta 'fechou' com um item de valor zero numa nota de 112,60")
	}
}

func TestSomaDosItensMostraADivergencia(t *testing.T) {
	// O caso real de NF 9160: itens somam 514,60, a nota diz 463,14 (10% de
	// desconto). A conta NÃO fecha, e o log precisa dizer os dois números.
	l := &Leitura{ValorTotal: 463.14, Itens: []Item{
		{Total: 335.60}, {Total: 119.60}, {Total: 17.90}, {Total: 27.60}, {Total: 13.90},
	}}
	if ContaFecha(l) {
		t.Error("10% de desconto passou pela tolerância de 1% — a trava não estaria travando nada")
	}
	if soma := SomaDosItens(l); soma < 514.59 || soma > 514.61 {
		t.Errorf("SomaDosItens = %.2f, esperava 514,60", soma)
	}
}
