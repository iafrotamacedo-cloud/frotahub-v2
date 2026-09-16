// rev 1 — armazém: a mesma receita de administrativo/notas_fiscais.go
//
// Copiada, não importada (P-13, ver o cabeçalho de modulo.go): dedup por
// sha256, upsert em `arquivos`, endereço temporário para exibir depois.
package locacoes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ValidadeDoLink é quanto tempo vale o endereço temporário do arquivo — mesmo
// prazo do resto do sistema: para abrir agora, não para guardar.
const ValidadeDoLink = 5 * time.Minute

func lerCabecalhoMultipart(cab *multipart.FileHeader) ([]byte, error) {
	f, err := cab.Open()
	if err != nil {
		return nil, fmt.Errorf("não consegui abrir o arquivo enviado")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, TamanhoMaximo+1))
	if err != nil || len(b) == 0 {
		return nil, fmt.Errorf("não consegui ler o arquivo enviado")
	}
	if len(b) > TamanhoMaximo {
		return nil, fmt.Errorf("o arquivo passa de %d MB", TamanhoMaximo>>20)
	}
	return b, nil
}

func tipoDoNome(nome string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(nome), ".")) {
	case "pdf":
		return "application/pdf"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

func nomeExtensao(nome string) string {
	i := strings.LastIndex(nome, ".")
	if i < 0 {
		return ""
	}
	return nome[i:]
}

// guardarArquivo sobe um arquivo (romaneio, NF opcional, foto) e devolve o
// sha256 já gravado em `arquivos`.
func (m *Modulo) guardarArquivo(ctx context.Context, p *seguranca.Principal, conteudo []byte, nome string) (string, error) {
	soma := sha256.Sum256(conteudo)
	sha := hex.EncodeToString(soma[:])
	ext := strings.TrimPrefix(strings.ToLower(nomeExtensao(nome)), ".")
	tipoMIME := tipoDoNome(nome)
	chave := armazem.Caminho(p.ClienteID, sha, ext)
	if err := m.arm.Enviar(ctx, chave, bytes.NewReader(conteudo), int64(len(conteudo)), sha, tipoMIME); err != nil {
		return "", fmt.Errorf("não consegui guardar no armazém: %w", err)
	}
	if err := m.bd.Upsert(ctx, "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": p.ClienteID,
		"tamanho":    len(conteudo),
		"tipo":       tipoMIME,
		"chave_r2":   chave,
	}}, nil); err != nil {
		return "", fmt.Errorf("guardei o arquivo mas não consegui registrá-lo: %w", err)
	}
	return sha, nil
}

func (m *Modulo) lerEGuardarArquivo(ctx context.Context, p *seguranca.Principal, cab *multipart.FileHeader) (string, error) {
	conteudo, err := lerCabecalhoMultipart(cab)
	if err != nil {
		return "", err
	}
	return m.guardarArquivo(ctx, p, conteudo, cab.Filename)
}

// ---------------------------------------------------------------------------
// GET /locacoes/arquivos/{sha} — endereço temporário para ver um arquivo
// (romaneio, NF, foto) — qualquer uma das quatro rotinas do módulo alcança.
// ---------------------------------------------------------------------------

func (m *Modulo) arquivo(w http.ResponseWriter, r *http.Request) {
	p := m.quemComQualquerRotina(w, r, RotinaReceber, RotinaMonitorar, RotinaDecidir, RotinaRenovarOC)
	if p == nil {
		return
	}
	sha := strings.TrimSpace(r.PathValue("sha"))
	if sha == "" {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	arq, err := m.contarUm(r.Context(), "arquivos?sha256=eq."+banco.Escapar(sha)+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=chave_r2&limit=1")
	if err != nil {
		m.erro(w, "não achei este arquivo", err)
		return
	}
	chave := strCampo(arq["chave_r2"])
	link, err := m.arm.LinkTemporario(chave, ValidadeDoLink)
	if err != nil {
		m.erro(w, "não consegui montar o endereço do arquivo", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"url": link})
}
