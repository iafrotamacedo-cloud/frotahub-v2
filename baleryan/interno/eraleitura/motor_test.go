package eraleitura

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/iafrotamacedo-cloud/era-regen/integrar"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

// Placeholder no ambiente ("/caminho/det.onnx") NÃO liga o ERA — era isso
// que fazia o recebimento de NF falhar no Render (15/09/2026).
func TestNovoComPlaceholderFicaDesligado(t *testing.T) {
	m := Novo(config.ERARead{DetONNX: "/caminho/det.onnx", RecONNX: "/caminho/rec.onnx", Dict: "/caminho/dict.txt"})
	if m.Ligado() {
		t.Fatal("caminho inexistente ligou o ERA")
	}
	if m.Motivo() == "" {
		t.Fatal("desligado sem dizer por quê")
	}
	if _, err := m.Ler([]byte("x"), lancamentoVazio()); err == nil {
		t.Fatal("Ler com o motor desligado devia falhar com erro, não em silêncio")
	}
}

func TestNovoSemNadaFicaDesligadoSemAviso(t *testing.T) {
	m := Novo(config.ERARead{})
	if m.Ligado() {
		t.Fatal("sem configuração nenhuma o ERA devia ficar desligado")
	}
}

// Os três arquivos existindo (mesmo que não sejam modelos de verdade) é o
// que `Novo` confere; abrir o grafo é trabalho preguiçoso, só na leitura.
func TestNovoComArquivosLiga(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"det.onnx", "rec.onnx", "dict.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("não é um modelo"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := Novo(config.ERARead{
		DetONNX: filepath.Join(dir, "det.onnx"),
		RecONNX: filepath.Join(dir, "rec.onnx"),
		Dict:    filepath.Join(dir, "dict.txt"),
	})
	if !m.Ligado() {
		t.Fatalf("com os três arquivos presentes devia ligar: %s", m.Motivo())
	}
	// Um modelo falso falha ao abrir — como ERRO, nunca como pânico.
	if _, err := m.Ler(jpegDeTeste(t, 40, 30), lancamentoVazio()); err == nil {
		t.Fatal("modelo falso devia falhar ao abrir")
	}
}

func TestReduzirMantemQuemJaCabe(t *testing.T) {
	raw := jpegDeTeste(t, 800, 600)
	saida, err := Reduzir(raw, 1600)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saida, raw) {
		t.Fatal("imagem que já cabe devia voltar byte a byte")
	}
}

func TestReduzirEncolheProporcional(t *testing.T) {
	raw := jpegDeTeste(t, 3000, 2000)
	saida, err := Reduzir(raw, 1500)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(saida))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 1500 || b.Dy() != 1000 {
		t.Fatalf("esperava 1500×1000, veio %d×%d", b.Dx(), b.Dy())
	}
	// A cor média tem que sobreviver à redução (a imagem de teste é cinza 128).
	c := color.GrayModel.Convert(img.At(700, 500)).(color.Gray)
	if c.Y < 118 || c.Y > 138 {
		t.Fatalf("a redução mudou a cor: %d", c.Y)
	}
}

func TestReduzirRecusaLixo(t *testing.T) {
	if _, err := Reduzir([]byte("isto não é imagem"), 1600); err == nil {
		t.Fatal("bytes que não são imagem deviam dar erro")
	}
}

func lancamentoVazio() integrar.Lancamento { return integrar.Lancamento{LeituraID: "t", NotaID: "t"} }

func jpegDeTeste(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
