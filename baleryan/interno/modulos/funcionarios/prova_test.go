// rev 1 — as contas puras do módulo
//
// Cobre o que não depende de banco nem de HTTP: a máscara de CPF/RG (é o que
// decide o que TST vê versus o que CEO/DP veem) e a validação do cadastro.
// Um harness de banco falso, como o de usuarios/prova_test.go, fica para
// quando este módulo ganhar sua segunda rodada — por ora o que mais importa
// provar é que a máscara nunca vaza o meio do documento.
package funcionarios

import "testing"

func TestMascararCPF(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"12345678900", "123***8900"},
		{"123.456.789-00", "123***9-00"},
		{"123", "***"},
		{"", "***"},
	}
	for _, c := range casos {
		if got := mascararCPF(c.entrada); got != c.esperado {
			t.Errorf("mascararCPF(%q) = %q, esperava %q", c.entrada, got, c.esperado)
		}
	}
}

func TestMascararRG(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"1234567", "***67"},
		{"12", "12"},
		{"", ""},
	}
	for _, c := range casos {
		if got := mascararRG(c.entrada); got != c.esperado {
			t.Errorf("mascararRG(%q) = %q, esperava %q", c.entrada, got, c.esperado)
		}
	}
}

func TestValidarCriar(t *testing.T) {
	casos := []struct {
		nome           string
		pedido         pedidoCriar
		esperaProblema bool
	}{
		{"vazio", pedidoCriar{}, true},
		{"sem cpf", pedidoCriar{NomeCompleto: "João Silva"}, true},
		{"cpf curto", pedidoCriar{NomeCompleto: "João Silva", CPF: "123"}, true},
		{"completo", pedidoCriar{NomeCompleto: "João Silva", CPF: "12345678900"}, false},
	}
	for _, c := range casos {
		problema := validarCriar(c.pedido) != ""
		if problema != c.esperaProblema {
			t.Errorf("%s: validarCriar(%+v) problema=%v, esperava %v", c.nome, c.pedido, problema, c.esperaProblema)
		}
	}
}

func TestTipoDoArquivo(t *testing.T) {
	casos := map[string]string{
		"aso.pdf":         "application/pdf",
		"FOTO.JPG":        "image/jpeg",
		"certificado.png": "image/png",
		"documento.xyz":   "application/octet-stream",
		"sem-extensao":    "application/octet-stream",
	}
	for nome, esperado := range casos {
		if got := tipoDoArquivo(nome); got != esperado {
			t.Errorf("tipoDoArquivo(%q) = %q, esperava %q", nome, got, esperado)
		}
	}
}

func TestNuloSeVazio(t *testing.T) {
	if v := nuloSeVazio("  "); v != nil {
		t.Errorf("esperava nil para string em branco, veio %v", v)
	}
	if v := nuloSeVazio(" 2026-09-02 "); v != "2026-09-02" {
		t.Errorf("esperava data sem espaço, veio %v", v)
	}
}
