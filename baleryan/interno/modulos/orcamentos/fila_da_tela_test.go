// rev 1 — a tela de Notas e DAVs é uma fila, não um arquivo morto
//
//	A nota que já virou orçamento sai da vista padrão. Sem isso a lista cresce
//	para sempre: em dois anos seriam milhares de linhas resolvidas escondendo as
//	dez que precisam de gente.
//
//	O teste olha o FILTRO, e não o resultado, porque é no filtro que a proteção
//	mora — e o sintoma de ela sumir é mudo: a fila volta a mostrar tudo e
//	ninguém nota até estar com mil linhas.
package orcamentos

import (
	"net/url"
	"strings"
	"testing"
)

func filtroDaVista(vista string) string {
	q := url.Values{}
	if vista != "" {
		q.Set("vista", vista)
	}
	return filtroDosDocumentos("11111111-1111-1111-1111-111111111111", "orcamento", q)
}

func TestAFilaNaoMostraOQueJaVirouOrcamento(t *testing.T) {
	f := filtroDaVista("")
	if !strings.Contains(f, "status=neq.usado") {
		t.Errorf("a vista padrão mostra as notas já resolvidas: %q", f)
	}
	if !strings.Contains(f, "oculto_em=is.null") {
		t.Errorf("a vista padrão perdeu o filtro das tiradas da fila: %q", f)
	}
}

func TestAsProcessadasContinuamAlcancaveis(t *testing.T) {
	// Sair da fila não é sumir. Se não houvesse esta vista, "tirar da lista"
	// viraria "esconder para sempre" — e aí a queixa seria a oposta.
	f := filtroDaVista("processadas")
	if !strings.Contains(f, "status=eq.usado") {
		t.Errorf("não dá para ver o que já virou orçamento: %q", f)
	}
}

func TestForaDaFilaMostraAsTiradas(t *testing.T) {
	f := filtroDaVista("fora")
	if !strings.Contains(f, "oculto_em=not.is.null") {
		t.Errorf("a vista de fora da fila não filtra pelas tiradas: %q", f)
	}
	// Aqui o status NÃO entra: tirada da fila é tirada, tenha virado orçamento
	// ou não. Filtrar por status faria a nota resolvida E tirada não aparecer em
	// vista nenhuma — sumida de verdade, que é o que este desenho evita.
	if strings.Contains(f, "status=") {
		t.Errorf("a vista de fora da fila filtra por status e esconde casos: %q", f)
	}
}

// A BUSCA VALE EM QUALQUER VISTA
//
//	Procurar pelo nome do arquivo é justamente o que a pessoa faz quando a nota
//	não está onde ela esperava.
func TestABuscaValeEmQualquerVista(t *testing.T) {
	for _, vista := range []string{"", "processadas", "fora"} {
		q := url.Values{"busca": {"19329"}}
		if vista != "" {
			q.Set("vista", vista)
		}
		f := filtroDosDocumentos("11111111-1111-1111-1111-111111111111", "orcamento", q)
		if !strings.Contains(f, "19329") {
			t.Errorf("vista %q perdeu a busca: %q", vista, f)
		}
	}
}
