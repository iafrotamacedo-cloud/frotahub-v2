// rev 1 — a data que o modelo devolve
//
// O QUE ACONTECEU EM 26/08/2026
//
//	Na leitura das 81 notas, duas voltaram com a emissão no formato do papel —
//	"28/07/2026" e "27-07-2026". O Postgres recusou as duas e derrubou a
//	gravação INTEIRA: itens, valores, emitente. As duas se salvaram na segunda
//	tentativa apenas porque o modelo sorteou outro formato.
//
//	E o sorteio produziu coisa pior: "27-07-2026" voltou como "2027-07-26". Ano
//	de 2027, data válida, aceita sem reclamação. Como a `emissao` é o corte do
//	pedido de faturamento, aquela DAV nunca entraria em pedido nenhum.
package leitor

import (
	"testing"
	"time"
)

func TestDataLimpaAceitaOsFormatosQueOModeloDevolve(t *testing.T) {
	casos := map[string]string{
		"28/07/2026": "2026-07-28", // o caso da LEDS NF 19650
		"27-07-2026": "2026-07-27", // o caso da NF RATEIO SV 659629
		"2026-07-28": "2026-07-28", // o formato certo passa intacto
		"2026-7-8":   "2026-07-08", // ISO sem o zero à esquerda
		"27.07.2026": "2026-07-27",
		" 28/07/2026 ": "2026-07-28",
	}
	for bruto, esperado := range casos {
		if deu := DataLimpa(bruto); deu != esperado {
			t.Errorf("%q virou %q, esperava %q", bruto, deu, esperado)
		}
	}
}

// O PALPITE É PIOR QUE O VAZIO
//
//	Vazio a tela mostra como "sem data" e alguém digita. Palpite entra na base
//	parecendo certo — e ninguém confere o que parece certo.
func TestDataQueNaoSeEntendeViraVazio(t *testing.T) {
	for _, bruto := range []string{
		"", "sem data", "31/02/2026", "27/13/2026", "00/07/2026",
		"julho de 2026", "2026", "7/2026",
	} {
		if deu := DataLimpa(bruto); deu != "" {
			t.Errorf("%q virou %q, esperava vazio", bruto, deu)
		}
	}
}

// NOTA NÃO É EMITIDA AMANHÃ
//
//	Foi assim que "27-07-2026" virou 2027 sem ninguém ver. Data no futuro é
//	leitura errada, sempre.
func TestDataNoFuturoNaoPassa(t *testing.T) {
	amanha := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	if deu := DataLimpa(amanha); deu != "" {
		t.Errorf("aceitou a data futura %q como %q", amanha, deu)
	}
	// O caso real: a emissão que o modelo embaralhou.
	if deu := DataLimpa("2027-07-26"); deu != "" {
		t.Errorf("aceitou 2027-07-26 como %q — é a data que sumiria do pedido", deu)
	}
	// Ontem continua valendo: a folga do futuro não pode virar folga do passado.
	ontem := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if deu := DataLimpa(ontem); deu != ontem {
		t.Errorf("recusou a data de ontem %q (deu %q)", ontem, deu)
	}
}

// A data entra no `Arrumar()` junto com o resto — se ficasse de fora, a função
// que limpa a leitura limparia tudo menos o campo que derrubou a gravação.
func TestArrumarLimpaAData(t *testing.T) {
	l := &Leitura{Numero: "19650", Emissao: "28/07/2026"}
	l.Arrumar()
	if l.Emissao != "2026-07-28" {
		t.Errorf("Arrumar deixou a emissão em %q", l.Emissao)
	}
}
