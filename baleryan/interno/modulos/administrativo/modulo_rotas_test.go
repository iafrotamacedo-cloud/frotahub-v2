// rev 1 — trava contra rota que colide (12/09/2026)
//
// `http.ServeMux.HandleFunc` PANICA NO REGISTRO, NÃO NO PRIMEIRO PEDIDO
//
//	Duas rotas em que nenhuma é mais específica que a outra (ex.:
//	"nf/{id}/arquivo" vs "nf/ordens/{id}") só estouram quando o processo
//	sobe — go build e go vet não veem isso, e o primeiro lugar que vê é o
//	deploy. Este teste chama `Montar` de verdade, num `Modulo` vazio (só
//	registrar rota não toca em banco, armazém nem nada mais), então uma
//	rota nova que colide quebra aqui, não em produção.
package administrativo

import (
	"net/http"
	"testing"
)

func TestMontarNaoTemRotaQueColide(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Montar entrou em pânico ao registrar as rotas: %v", r)
		}
	}()
	(&Modulo{}).Montar(http.NewServeMux())
}
