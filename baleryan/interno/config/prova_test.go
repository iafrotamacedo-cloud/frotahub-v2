package config

import "testing"

// A lista de alvos chega colada à mão. Estes são os formatos que ela chega, e a
// garantia de que nenhum deles vira um número que ninguém pediu.
func TestListaDeNumerosAceitaComoAGenteCola(t *testing.T) {
	casos := []struct {
		nome     string
		bruto    string
		esperado []int
	}{
		{"vírgula", "121413,121725,67295", []int{121413, 121725, 67295}},
		{"vírgula e espaço", "121413, 121725, 67295", []int{121413, 121725, 67295}},
		{"quebra de linha", "121413\n121725\n67295", []int{121413, 121725, 67295}},
		{"colado da planilha", "121413;121725 67295\n", []int{121413, 121725, 67295}},
		{"repetido entra uma vez", "121413,121413,121725", []int{121413, 121725}},
		{"vazio é nada", "", nil},
		{"só separador é nada", " , ; \n", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Setenv("TRILOGO_ALVOS", c.bruto)
			l := &leitor{}
			veio := l.numeros("TRILOGO_ALVOS")
			if len(l.problemas) != 0 {
				t.Fatalf("não devia reclamar de %q: %v", c.bruto, l.problemas)
			}
			if len(veio) != len(c.esperado) {
				t.Fatalf("%q virou %v, esperava %v", c.bruto, veio, c.esperado)
			}
			for i := range veio {
				if veio[i] != c.esperado[i] {
					t.Fatalf("%q virou %v, esperava %v", c.bruto, veio, c.esperado)
				}
			}
		})
	}
}

// O ponto da coisa: número torto não é ignorado em silêncio.
//
// Ignorar seria o comportamento "amigável" — e leria uma lista menor do que a
// que o dono digitou, sem dizer. No modo `alvos` a lista é a única coisa que
// decide o que vai ser gravado; ler de menos é tão errado quanto ler de mais.
func TestNumeroTortoDerrubaAConfiguracao(t *testing.T) {
	for _, bruto := range []string{"121413,abc", "121413,-5", "121413,0", "12.34"} {
		t.Setenv("TRILOGO_ALVOS", bruto)
		l := &leitor{}
		l.numeros("TRILOGO_ALVOS")
		if len(l.problemas) == 0 {
			t.Errorf("%q passou calado e devia ter virado problema de configuração", bruto)
		}
	}
}
