package administrativo

import "testing"

// O valor da nota chega do celular nas duas grafias: "1.234,56" (como a
// tela formata a sugestão) e "1234.56" (como o teclado decimal digita).
func TestParseValorNF(t *testing.T) {
	casos := []struct {
		entrada string
		quer    float64
		erro    bool
	}{
		{"1234.56", 1234.56, false},
		{"1.234,56", 1234.56, false},
		{"1234,56", 1234.56, false},
		{"  350 ", 350, false},
		{"0", 0, true},
		{"", 0, true},
		{"abc", 0, true},
		{"-10", 0, true},
	}
	for _, c := range casos {
		v, err := parseValorNF(c.entrada)
		if c.erro {
			if err == nil {
				t.Errorf("%q: esperava erro, veio %v", c.entrada, v)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: erro inesperado: %v", c.entrada, err)
			continue
		}
		if v != c.quer {
			t.Errorf("%q: esperava %v, veio %v", c.entrada, c.quer, v)
		}
	}
}

func TestTipoDaImagem(t *testing.T) {
	if got := tipoDaImagem([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}); got != "image/png" {
		t.Fatalf("PNG: %s", got)
	}
	if got := tipoDaImagem([]byte("%PDF-1.4 ...")); got != "application/pdf" {
		t.Fatalf("PDF: %s", got)
	}
	if got := tipoDaImagem([]byte{0xff, 0xd8, 0xff, 0xe0}); got != "image/jpeg" {
		t.Fatalf("JPEG: %s", got)
	}
}

// Sem ERA ligado, agendar nunca falha e nunca bloqueia — a nota já está salva.
func TestAgendarLeituraERASemMotor(t *testing.T) {
	m := &Modulo{}
	if m.agendarLeituraERA(trabalhoERA{nfID: "x", pagina: 1}) {
		t.Fatal("sem motor devia devolver false")
	}
}
