// Package eraleitura liga o FrotaHub ao Melhorador + ERA READ (era-regen/integrar).
package eraleitura

import (
	"fmt"
	"sync"

	"github.com/iafrotamacedo-cloud/era-read/read"
	"github.com/iafrotamacedo-cloud/era-regen/integrar"
	"github.com/iafrotamacedo-cloud/era-regen/melhorador"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// Motor encapsula o ERA READ configurado no ambiente.
type Motor struct {
	cfg config.ERARead
	eng *read.Engine
	mu  sync.Mutex
	err error
}

// Novo cria o motor; se os caminhos ONNX nao estiverem configurados, Ligado()
// devolve false e Ler falha com mensagem clara.
func Novo(cfg config.ERARead) *Motor {
	return &Motor{cfg: cfg}
}

// Ligado diz se o ERA READ esta configurado para escanear notas.
func (m *Motor) Ligado() bool {
	return m.cfg.Ligado()
}

func (m *Motor) engine() (*read.Engine, error) {
	if !m.Ligado() {
		return nil, fmt.Errorf("eraleitura: ERA READ nao configurado (ERA_DET_ONNX, ERA_REC_ONNX, ERA_DICT)")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.eng != nil {
		return m.eng, m.err
	}
	if m.err != nil {
		return nil, m.err
	}
	cfg := read.EngineConfig{
		DetONNX: m.cfg.DetONNX,
		RecONNX: m.cfg.RecONNX,
		Dict:    m.cfg.Dict,
	}
	if m.cfg.FiltroJSON != "" {
		f, err := read.ParseFiltroRead([]byte(m.cfg.FiltroJSON))
		if err != nil {
			m.err = err
			return nil, m.err
		}
		cfg.Filtro = &f
	}
	eng, err := read.AbrirEngine(cfg)
	if err != nil {
		m.err = fmt.Errorf("eraleitura: abrir motor: %w", err)
		return nil, m.err
	}
	m.eng = eng
	return m.eng, nil
}

// Ler prepara a imagem e le a nota fiscal.
func (m *Motor) Ler(raw []byte, meta integrar.Lancamento) (integrar.Saida, error) {
	eng, err := m.engine()
	if err != nil {
		return integrar.Saida{}, err
	}
	return integrar.Ler(raw, melhorador.Options{}, meta, eng)
}
