package leitor

import "testing"

// A chave real da nota do teste no ticket 130998.
const chaveReal = "23260814248351000120550010000197021394632944"

func TestChaveValida(t *testing.T) {
	if !ChaveValida(chaveReal) {
		t.Fatal("a chave real da NF 19702 tinha que passar")
	}
	// Um dígito trocado — exatamente o que o OCR faz.
	torta := chaveReal[:20] + "9" + chaveReal[21:]
	if ChaveValida(torta) {
		t.Fatal("chave com dígito trocado passou; a trava de duplicidade seria furada")
	}
	if ChaveValida("123") || ChaveValida("") {
		t.Fatal("tamanho errado tinha que reprovar")
	}
	if ChaveValida(chaveReal[:43] + "x") {
		t.Fatal("caractere que não é dígito tinha que reprovar")
	}
}

// O caso que motivou o cuidado: a observação real da NF 19702 traz
// "NOSSO NÚMERO: 45945" e "DAV: 92080" — dois números com cara de ticket.
func TestTicketsIgnoraOsEnganosos(t *testing.T) {
	obs := "VALOR DO PIS: 12,53 VALOR COFINS: 57,75 ENTREGA/MANUTENCAO PREVENTIVA MSL " +
		"VIRGILIO TAVORA; Tributos Aprox. NOSSO NÚMERO: 45945 NUM: DAV: 92080; " +
		"ticket 130998 conforme combinado"

	tk := Tickets(obs)
	if len(tk) != 1 || tk[0] != 130998 {
		t.Fatalf("achei %v; queria só [130998] — 45945 é nosso número e 92080 é DAV", tk)
	}
}

func TestTicketsVariosSemRepetir(t *testing.T) {
	tk := Tickets("atende os tickets 130328, 130341 e 130328 novamente")
	if len(tk) != 2 || tk[0] != 130328 || tk[1] != 130341 {
		t.Fatalf("achei %v; queria [130328 130341]", tk)
	}
}

func TestTicketsNaoPicaNumeroGrande(t *testing.T) {
	// Uma chave de acesso solta no texto não pode virar seis tickets.
	if tk := Tickets(chaveReal); len(tk) != 0 {
		t.Fatalf("a chave de acesso virou ticket: %v", tk)
	}
	if tk := Tickets("CNPJ 14.248.351/0001-20"); len(tk) != 0 {
		t.Fatalf("pedaço de CNPJ virou ticket: %v", tk)
	}
}

func TestDecimal(t *testing.T) {
	casos := map[string]float64{
		"1.425,30": 1425.30,
		"1425.30":  1425.30,
		"950":      950,
		"0,05":     0.05,
		"66,96":    66.96,
	}
	for txt, esperado := range casos {
		v, ok := Decimal(txt)
		if !ok || v != esperado {
			t.Fatalf("Decimal(%q) = %v,%v; queria %v", txt, v, ok, esperado)
		}
	}
	if _, ok := Decimal("abc"); ok {
		t.Fatal("texto que não é número tinha que falhar")
	}
}

// ---------------------------------------------------------------------------
// camada 1
// ---------------------------------------------------------------------------

const xmlDeTeste = `<?xml version="1.0" encoding="UTF-8"?>
<nfeProc versao="4.00">
 <NFe>
  <infNFe Id="NFe23260814248351000120550010000197021394632944" versao="4.00">
   <ide><nNF>19702</nNF><serie>1</serie><dhEmi>2026-08-04T00:00:00-03:00</dhEmi></ide>
   <emit><CNPJ>14248351000120</CNPJ><xNome>CNIP COMERCIO NACIONAL DE ILUMINACAO PUBLICA LTDA</xNome></emit>
   <dest><CNPJ>27363223000170</CNPJ></dest>
   <det nItem="1"><prod>
     <cProd>6890000111868</cProd>
     <xProd>FITA LED DIRECT 24W 4000K IP65 220V FD424 LUMANTI</xProd>
     <uCom>UN</uCom><qCom>50.0000</qCom><vUnCom>19.0000</vUnCom><vProd>950.00</vProd>
   </prod></det>
   <total><ICMSTot><vNF>950.00</vNF><vFrete>0.00</vFrete></ICMSTot></total>
   <infAdic><infCpl>ENTREGA MSL VIRGILIO TAVORA ticket 130998 DAV: 92080</infCpl></infAdic>
  </infNFe>
 </NFe>
</nfeProc>`

func TestDoXML(t *testing.T) {
	l, err := DoXML([]byte(xmlDeTeste))
	if err != nil {
		t.Fatal(err)
	}
	if l.ChaveAcesso != chaveReal {
		t.Fatalf("chave = %q", l.ChaveAcesso)
	}
	if l.Numero != "19702" || l.Serie != "1" {
		t.Fatalf("número/série = %q/%q", l.Numero, l.Serie)
	}
	// A DATA É O PONTO: o XML diz 04, e 04 tem que chegar. Passar por time.Time
	// em fuso local devolveria 03 — foi o que a tela do Trílogo fez.
	if l.Emissao != "2026-08-04" {
		t.Fatalf("emissão = %q, queria 2026-08-04 — o dia se perdeu no fuso", l.Emissao)
	}
	if l.ValorTotal != 950 {
		t.Fatalf("valor = %v", l.ValorTotal)
	}
	if len(l.Itens) != 1 || l.Itens[0].Quantidade != 50 || l.Itens[0].Unitario != 19 {
		t.Fatalf("itens = %+v", l.Itens)
	}
	if l.Confianca != 1 {
		t.Fatalf("o XML é o documento original; confiança = %v", l.Confianca)
	}
	if tk := Tickets(l.Observacao); len(tk) != 1 || tk[0] != 130998 {
		t.Fatalf("tickets da observação = %v", tk)
	}
	if !l.Completa() {
		t.Fatal("uma nota com valor e item é completa")
	}
}

func TestDoXMLRecusaOQueNaoEh(t *testing.T) {
	if _, err := DoXML([]byte("<html><body>oi</body></html>")); err == nil {
		t.Fatal("HTML passou por NFe")
	}
	if _, err := DoXML([]byte("não é nem XML")); err == nil {
		t.Fatal("lixo passou por NFe")
	}
}

// ---------------------------------------------------------------------------
// camada 2
// ---------------------------------------------------------------------------

func TestDoTextoPegaAChaveEDeduzONumero(t *testing.T) {
	texto := `DANFE  DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRONICA
CHAVE DE ACESSO
2326 0814 2483 5100 0120 5500 1000 0197 0213 9463 2944
DATA DA EMISSAO 04/08/2026
VALOR TOTAL DA NOTA  950,00`

	l := DoTexto(texto)
	if l.ChaveAcesso != chaveReal {
		t.Fatalf("chave = %q", l.ChaveAcesso)
	}
	if l.Numero != "19702" {
		t.Fatalf("número deduzido da chave = %q", l.Numero)
	}
	if l.EmitenteCNPJ != "14248351000120" {
		t.Fatalf("cnpj deduzido da chave = %q", l.EmitenteCNPJ)
	}
	if l.Emissao != "2026-08-04" {
		t.Fatalf("emissão = %q", l.Emissao)
	}
	if l.ValorTotal != 950 {
		t.Fatalf("valor = %v", l.ValorTotal)
	}
	// A camada 2 não leu item nenhum, e por isso não pode se dizer confiante.
	if l.Completa() {
		t.Fatal("leitura sem itens não pode se declarar completa")
	}
	if l.Confianca > 0.6 {
		t.Fatalf("confiança %v alta demais para quem não leu os itens", l.Confianca)
	}
}

func TestDoTextoVazioNaoQuebra(t *testing.T) {
	if l := DoTexto(""); l == nil || l.Confianca != 0 {
		t.Fatal("texto vazio devia devolver leitura vazia, não pânico")
	}
}

// ---------------------------------------------------------------------------
// conferência e junção
// ---------------------------------------------------------------------------

func TestConferirApagaChaveInventada(t *testing.T) {
	l := &Leitura{
		ChaveAcesso: "12345678901234567890123456789012345678901234",
		ValorTotal:  950,
		Itens:       []Item{{Descricao: "x", Quantidade: 50, Unitario: 19, Total: 950}},
	}
	Conferir(l)
	if l.ChaveAcesso != "" {
		t.Fatal("chave que não fecha no verificador tinha que ser apagada, não guardada")
	}
}

func TestConferirPremiaSomaQueBate(t *testing.T) {
	bate := &Leitura{ValorTotal: 950, Itens: []Item{{Descricao: "x", Quantidade: 50, Unitario: 19, Total: 950}}}
	naoBate := &Leitura{ValorTotal: 950, Itens: []Item{{Descricao: "x", Quantidade: 50, Unitario: 19, Total: 400}}}

	if Conferir(bate) <= Conferir(naoBate) {
		t.Fatal("a leitura cuja soma fecha tem que valer mais que a que não fecha")
	}
}

// O regex acha a chave e não acha item; a IA acha item e erra a chave. Juntas,
// acertam as duas coisas — que é a razão de `Melhor` existir.
func TestMelhorJuntaOQueCadaCamadaAcertou(t *testing.T) {
	doRegex := &Leitura{ChaveAcesso: chaveReal, ValorTotal: 950, Camada: DoOCR}
	doRegex.Confianca = Conferir(doRegex)

	daIA := &Leitura{
		ChaveAcesso: "12345678901234567890123456789012345678901234", // errada
		ValorTotal:  950,
		Itens:       []Item{{Descricao: "FITA LED", Quantidade: 50, Unitario: 19, Total: 950}},
		Emissao:     "2026-08-04",
		Camada:      DaIA,
	}
	daIA.Confianca = Conferir(daIA)

	j := Melhor(doRegex, daIA)
	if j.ChaveAcesso != chaveReal {
		t.Fatalf("a chave boa se perdeu: %q", j.ChaveAcesso)
	}
	if len(j.Itens) != 1 {
		t.Fatal("os itens da IA se perderam")
	}
	if !j.Completa() {
		t.Fatal("juntas, as duas camadas dão uma leitura completa")
	}
}

func TestMelhorAceitaNulo(t *testing.T) {
	l := &Leitura{ValorTotal: 1}
	if Melhor(nil, l) != l || Melhor(l, nil) != l {
		t.Fatal("Melhor tem que aguentar nulo dos dois lados")
	}
}
