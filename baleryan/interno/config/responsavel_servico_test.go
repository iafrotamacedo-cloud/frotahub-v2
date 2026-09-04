package config

import "testing"

func triDeTeste() Trilogo {
	return Trilogo{
		FornecedorInstalacoes: 23927,
		FornecedorCivil:       0,

		ResponsavelServicoInstalacoes: 67277,
		ResponsavelServicoCivil:       67264,

		NomeResponsavelServicoInstalacoes: "Serviço Instalações",
		NomeResponsavelServicoCivil:       "Romario Santos",
	}
}

func TestFornecedorDaConta(t *testing.T) {
	tri := triDeTeste()
	if got := tri.FornecedorDaConta("instalacoes"); got != 23927 {
		t.Errorf("FornecedorDaConta(instalacoes) = %d, esperava 23927", got)
	}
	// Civil nasce zero até o dono levantar — zero é "não sei", não um id
	// válido.
	if got := tri.FornecedorDaConta("civil"); got != 0 {
		t.Errorf("FornecedorDaConta(civil) = %d, esperava 0", got)
	}
}

func TestResponsavelServicoDaConta(t *testing.T) {
	tri := triDeTeste()
	if got := tri.ResponsavelServicoDaConta("instalacoes"); got != 67277 {
		t.Errorf("ResponsavelServicoDaConta(instalacoes) = %d", got)
	}
	if got := tri.ResponsavelServicoDaConta("civil"); got != 67264 {
		t.Errorf("ResponsavelServicoDaConta(civil) = %d", got)
	}
}

// A METADE "DETECTAR" DO GATILHO AUTOMÁTICO
//
//	Romario Santos é uma PESSOA, cadastrada como responsável no Trílogo — o
//	nome cru que a leitura do robô grava em chamados.responsavel. Esta função
//	é quem reconhece que aquele nome quer dizer "isto é Serviço", sem chamar
//	o Trílogo de novo.
func TestContaDoNomeResponsavelServico(t *testing.T) {
	tri := triDeTeste()

	casos := map[string]string{
		"Serviço Instalações": "instalacoes",
		"Romario Santos":      "civil",
		"":                    "",
		"Jefferson Silva":     "", // um técnico normal, não é o gatilho
	}
	for nome, quer := range casos {
		if got := tri.ContaDoNomeResponsavelServico(nome); got != quer {
			t.Errorf("ContaDoNomeResponsavelServico(%q) = %q, esperava %q", nome, got, quer)
		}
	}
}

// SEM CONFIGURAÇÃO, NINGUÉM BATE COM NADA
//
//	Uma Trilogo{} zerada (os dois nomes em "") não pode fazer todo chamado
//	SEM responsável (nomeCru="") virar gatilho de Serviço — seria o oposto
//	exato do que a função existe para evitar.
func TestContaDoNomeResponsavelServicoSemConfiguracao(t *testing.T) {
	var tri Trilogo
	if got := tri.ContaDoNomeResponsavelServico(""); got != "" {
		t.Errorf("chamado sem responsável não pode virar gatilho, veio %q", got)
	}
	if got := tri.ContaDoNomeResponsavelServico("qualquer nome"); got != "" {
		t.Errorf("sem configuração nenhuma, nada deveria bater, veio %q", got)
	}
}
