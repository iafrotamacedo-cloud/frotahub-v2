// Package eraleitura liga o FrotaHub ao Melhorador + ERA READ (era-regen/integrar).
//
// O QUE ESTE PACOTE PROMETE — E O QUE ELE NÃO PROMETE (15/09/2026)
//
//	Promete: dado o JPEG de uma página de nota fiscal, devolver a leitura
//	congelada do ERA READ (texto com posição + campos), sem derrubar o
//	motor por causa de nada que aconteça lá dentro. A leitura é PESADA —
//	medida em 15/09/2026, no PC do dono (16 threads): 30 s a 6 min por
//	página, 1,5 GB de memória numa foto de 4400 px. Por isso quem chama
//	este pacote NUNCA o faz dentro de uma requisição HTTP: a página é
//	salva antes, e a leitura entra numa fila (ver nf_era.go).
//
//	Não promete: que a leitura esteja certa. O ERA READ ainda está em
//	calibração (era-regen/auditor), e o que ele devolve é gravado como
//	`leitura_era` para o calibrador comparar — não decide número nem valor
//	da nota sozinho.
//
// TRÊS CUIDADOS QUE NÃO ESTAVAM NA PRIMEIRA VERSÃO
//
//  1. `melhorador.Options{}` zerado fazia `enhance.NormalizeContrast`
//     entrar em pânico ("percentis invalidos") em TODA imagem — o
//     era-regen não aplica os próprios defaults quando o chamador passa
//     zero. Aqui os defaults de `enhance` entram explicitamente.
//
//  2. Com os limiares padrão de deformação, toda região de uma página
//     plana saía classificada como N3 (curvatura forte) e era descartada
//     antes do reconhecimento: 359 regiões detectadas, 0 linhas lidas.
//     A página que chega aqui já vem plana — recortada em perspectiva
//     pelo scanner do celular e endireitada pelo Melhorador — então os
//     níveis N2/N3 (retificação de curva, rede de dewarp) são desligados
//     de propósito: tudo é lido como N0/N1, recorte reto.
//
//  3. A imagem é reduzida a `LadoMaximo` antes de qualquer conta. O
//     detector já trabalha em 960 px; o que passa disso só custa memória
//     no Melhorador e nos recortes do reconhecedor.
//
// PLACEHOLDER NÃO LIGA NADA
//
//	No Render as três variáveis existiam com caminhos de mentira
//	("/caminho/det.onnx"), e `Ligado()` — que só olhava se o texto era
//	vazio — dizia que o ERA estava ligado. Resultado: a rota de escanear
//	tentava abrir o motor, falhava, e o recebimento da nota era recusado
//	junto. Agora `Novo` confere se os três arquivos existem; se não, o ERA
//	fica desligado com aviso no log, e a nota é recebida do mesmo jeito.
package eraleitura

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // o scanner manda JPEG, mas uma foto de galeria pode vir PNG
	"log"
	"os"
	"sync"

	"github.com/iafrotamacedo-cloud/era-read/read"
	"github.com/iafrotamacedo-cloud/era-regen/integrar"
	"github.com/iafrotamacedo-cloud/era-regen/melhorador"
	"github.com/iafrotamacedo-cloud/era-regen/melhorador/enhance"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// Motor encapsula o ERA READ configurado no ambiente.
type Motor struct {
	cfg    config.ERARead
	ligado bool
	motivo string

	// UMA LEITURA POR VEZ
	//
	//	O `read.Engine` é seguro para uso paralelo, mas duas páginas ao
	//	mesmo tempo dobram o pico de memória — e o plano gratuito do
	//	Render tem 512 MB para o motor inteiro. A fila em nf_era.go já é
	//	sequencial; este mutex garante a regra mesmo se alguém chamar
	//	`Ler` de outro lugar.
	leitura sync.Mutex

	abrir sync.Once
	eng   *read.Engine
	err   error
}

// Novo cria o motor. Se algum dos três arquivos não existir, o ERA READ
// fica desligado e `Motivo()` diz por quê — o motor do FrotaHub sobe do
// mesmo jeito.
func Novo(cfg config.ERARead) *Motor {
	m := &Motor{cfg: cfg}
	if cfg.DetONNX == "" && cfg.RecONNX == "" && cfg.Dict == "" {
		m.motivo = "ERA_DET_ONNX, ERA_REC_ONNX e ERA_DICT não configurados"
		return m
	}
	for nome, caminho := range map[string]string{
		"ERA_DET_ONNX": cfg.DetONNX, "ERA_REC_ONNX": cfg.RecONNX, "ERA_DICT": cfg.Dict,
	} {
		if caminho == "" {
			m.motivo = nome + " vazio"
			break
		}
		if info, err := os.Stat(caminho); err != nil || info.IsDir() || info.Size() == 0 {
			m.motivo = fmt.Sprintf("%s aponta para %q, que não existe ou está vazio", nome, caminho)
			break
		}
	}
	if m.motivo != "" {
		log.Printf("[baleryan] aviso: ERA READ desligado — %s. O recebimento de NF continua manual.", m.motivo)
		return m
	}
	m.ligado = true
	return m
}

// Ligado diz se o ERA READ está configurado (e os arquivos existem).
func (m *Motor) Ligado() bool { return m != nil && m.ligado }

// Motivo explica por que está desligado ("" quando ligado).
func (m *Motor) Motivo() string {
	if m == nil {
		return "motor ausente"
	}
	return m.motivo
}

// LadoMaximo é o maior lado, em pixels, que uma página tem ao entrar no
// ERA READ.
func (m *Motor) LadoMaximo() int {
	if m == nil || m.cfg.LadoMaximo <= 0 {
		return config.ERALadoMaximoPadrao
	}
	return m.cfg.LadoMaximo
}

func (m *Motor) engine() (*read.Engine, error) {
	if !m.Ligado() {
		return nil, fmt.Errorf("eraleitura: ERA READ desligado (%s)", m.motivo)
	}
	m.abrir.Do(func() {
		opts := read.DefaultOptions()
		// Página plana: sem N2/N3 (ver o cabeçalho, cuidado 2).
		opts.Thresholds.RetoPx = 1e9
		opts.Thresholds.CurvoPx = 1e9
		opts.FatorDeformacaoRelativo = 0

		cfg := read.EngineConfig{
			DetONNX: m.cfg.DetONNX,
			RecONNX: m.cfg.RecONNX,
			Dict:    m.cfg.Dict,
			Opts:    opts,
		}
		if m.cfg.FiltroJSON != "" {
			f, err := read.ParseFiltroRead([]byte(m.cfg.FiltroJSON))
			if err != nil {
				m.err = fmt.Errorf("eraleitura: filtro_read: %w", err)
				return
			}
			cfg.Filtro = &f
		}
		eng, err := read.AbrirEngine(cfg)
		if err != nil {
			m.err = fmt.Errorf("eraleitura: abrir motor: %w", err)
			return
		}
		m.eng = eng
	})
	return m.eng, m.err
}

// Ler prepara a imagem (Melhorador) e lê a página com o ERA READ.
//
// Nunca entra em pânico: qualquer pânico lá dentro vira erro, porque quem
// chama é uma goroutine de fila, sem o `web.Recuperar` das rotas HTTP.
func (m *Motor) Ler(raw []byte, meta integrar.Lancamento) (saida integrar.Saida, err error) {
	eng, err := m.engine()
	if err != nil {
		return integrar.Saida{}, err
	}
	reduzido, err := Reduzir(raw, m.LadoMaximo())
	if err != nil {
		return integrar.Saida{}, err
	}

	m.leitura.Lock()
	defer m.leitura.Unlock()
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("eraleitura: pânico na leitura: %v", p)
		}
	}()

	opts := melhorador.Options{Enhance: enhance.DefaultOptions()}
	return integrar.Ler(reduzido, opts, meta, eng)
}

// Reduzir devolve a imagem com o maior lado em no máximo `lado` pixels,
// como JPEG. Uma imagem que já cabe volta como veio, byte a byte.
//
// A redução é por média de área (cada pixel de saída é a média do bloco
// de entrada que ele cobre) — o suficiente para OCR e escrita aqui em
// vez de trazer golang.org/x/image: o motor é, de propósito, sem
// dependência fora dos dois repositórios do ERA.
func Reduzir(raw []byte, lado int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("eraleitura: decodificar imagem: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("eraleitura: imagem vazia")
	}
	maior := max(w, h)
	if lado <= 0 || maior <= lado {
		return raw, nil
	}
	f := float64(lado) / float64(maior)
	dw := max(1, int(float64(w)*f+0.5))
	dh := max(1, int(float64(h)*f+0.5))
	dst := reduzirPorArea(img, dw, dh)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 92}); err != nil {
		return nil, fmt.Errorf("eraleitura: reencodar imagem: %w", err)
	}
	return buf.Bytes(), nil
}

func reduzirPorArea(src image.Image, dw, dh int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	// Uma cópia RGBA da origem para ler pixel a pixel sem a interface.
	orig := image.NewRGBA(image.Rect(0, 0, sw, sh))
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			orig.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0 := y * sh / dh
		y1 := max(y0+1, (y+1)*sh/dh)
		for x := 0; x < dw; x++ {
			x0 := x * sw / dw
			x1 := max(x0+1, (x+1)*sw/dw)
			var r, g, bl, n uint32
			for yy := y0; yy < y1; yy++ {
				i := yy*orig.Stride + x0*4
				for xx := x0; xx < x1; xx++ {
					r += uint32(orig.Pix[i])
					g += uint32(orig.Pix[i+1])
					bl += uint32(orig.Pix[i+2])
					n++
					i += 4
				}
			}
			o := y*dst.Stride + x*4
			dst.Pix[o] = uint8(r / n)
			dst.Pix[o+1] = uint8(g / n)
			dst.Pix[o+2] = uint8(bl / n)
			dst.Pix[o+3] = 255
		}
	}
	return dst
}
