// rev 1 — a regra do cliente, lida do banco
//
// O teto de R$ 600 e a margem de 20% são decisões da empresa, não do programa.
// Este pacote é a única porta para lê-las, e existe por um motivo prático: no
// dia em que o teto virar R$ 800, ninguém precisa publicar código.
//
// GUARDA POR ALGUNS MINUTOS, DE PROPÓSITO
//
//	Gerar cinquenta orçamentos não pode virar cinquenta consultas ao mesmo par de
//	números. Mas guardar para sempre faria a mudança do teto só valer no próximo
//	deploy — que é exatamente o problema que a tabela veio resolver. Cinco
//	minutos é o meio-termo: rápido para o lote, curto para quem mudou.
package parametros

import (
	"context"
	"sync"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

// ValidadeDoCache é quanto tempo os números ficam guardados na memória.
const ValidadeDoCache = 5 * time.Minute

type Servico struct {
	bd *banco.Cliente

	mu       sync.Mutex
	guardado map[string]lembranca
	agora    func() time.Time // injetável para o teste não depender do relógio
}

type lembranca struct {
	valores regras.Parametros
	ate     time.Time
}

func Novo(bd *banco.Cliente) *Servico {
	return &Servico{bd: bd, guardado: map[string]lembranca{}, agora: time.Now}
}

type linha struct {
	Chave string  `json:"chave"`
	Valor float64 `json:"valor"`
}

// Do devolve os parâmetros vigentes deste cliente.
//
// QUANDO NÃO ACHA
//
//	Devolve o padrão e NÃO estoura. Um orçamento que não gera porque a tabela de
//	parâmetros está vazia é pior do que um orçamento gerado com a regra de
//	sempre. Mas quem chama recebe `false` no segundo retorno e avisa em log —
//	silêncio aqui viraria uma regra fantasma, que é o defeito oposto e igualmente
//	ruim.
func (s *Servico) Do(ctx context.Context, clienteID string) (regras.Parametros, bool, error) {
	s.mu.Lock()
	if l, tem := s.guardado[clienteID]; tem && s.agora().Before(l.ate) {
		s.mu.Unlock()
		return l.valores, true, nil
	}
	s.mu.Unlock()

	caminho := "parametros?cliente_id=eq." + banco.Escapar(clienteID) +
		"&vigencia_fim=is.null&select=chave,valor"

	var linhas []linha
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return regras.Padrao, false, err
	}
	if len(linhas) == 0 {
		return regras.Padrao, false, nil
	}

	p := regras.Padrao
	achou := map[string]bool{}
	for _, l := range linhas {
		switch l.Chave {
		case "teto_lancamento":
			p.Teto = regras.DinheiroDe(l.Valor)
			achou["teto"] = true
		case "margem":
			// Guardado como fração (0,2000) e usado como pontos-base (2000).
			p.MargemBP = int64(l.Valor * 10000)
			achou["margem"] = true
		case "teto_folga_pct":
			p.FolgaBP = int64(l.Valor * 10000)
			achou["folga"] = true
		}
	}
	completo := achou["teto"] && achou["margem"] && achou["folga"]

	if completo {
		s.mu.Lock()
		s.guardado[clienteID] = lembranca{valores: p, ate: s.agora().Add(ValidadeDoCache)}
		s.mu.Unlock()
	}
	return p, completo, nil
}

// Esquecer descarta o que está guardado. É o que a tela de parâmetros chama
// depois de gravar — assim a mudança vale na hora, não daqui a cinco minutos.
func (s *Servico) Esquecer(clienteID string) {
	s.mu.Lock()
	delete(s.guardado, clienteID)
	s.mu.Unlock()
}
