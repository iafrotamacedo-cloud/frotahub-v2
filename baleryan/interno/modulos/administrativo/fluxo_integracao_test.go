//go:build integracao

// Teste de integração: as 30 OCs de OCs_Teste passam pelo fluxo real
// (inserir → ler → vistas → envio PCO), usando Supabase + R2 de verdade.
//
// Rodar (PowerShell, a partir de baleryan/):
//
//	$env:Path = "<poppler>\Library\bin;" + $env:Path
//	./dev.ps1  # numa janela, ou carregue o .env na mesma
//	go test -tags=integracao ./interno/modulos/administrativo/ -run TestFluxoCompleto30OCs -count=1 -timeout 15m -v
package administrativo

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

type casoGabarito struct {
	Arquivo         string  `json:"arquivo"`
	Numero          string  `json:"numero"`
	Esperado        string  `json:"esperado"`
	Motivo          string  `json:"motivo"`
	FornecedorNome  string  `json:"fornecedor_nome"`
	FornecedorCNPJ  string  `json:"fornecedor_cnpj"`
	CompradorCNPJ   string  `json:"comprador_cnpj"`
	NItens          int     `json:"n_itens"`
	Subtotal        float64 `json:"subtotal"`
	Total           float64 `json:"total"`
}

func TestFluxoCompleto30OCs(t *testing.T) {
	if _, err := execLookPath("pdftotext"); err != nil {
		t.Skip("pdftotext não está no PATH — instale poppler-utils ou aponte o PATH")
	}

	cfg, err := config.Carregar()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !cfg.R2.Ligado() {
		t.Skip("R2 não configurado — inserir OC precisa do armazém")
	}

	pasta := filepath.Join(repoRaiz(t), "OCs_Teste")
	gabaritoPath := filepath.Join(pasta, "_gabarito.json")
	bruto, err := os.ReadFile(gabaritoPath)
	if err != nil {
		t.Fatalf("gabarito: %v", err)
	}
	var casos []casoGabarito
	if err := json.Unmarshal(bruto, &casos); err != nil {
		t.Fatalf("gabarito json: %v", err)
	}

	ctx := context.Background()
	bd := banco.Novo(cfg)
	arm := armazem.Novo(cfg)
	hist := historico.Novo(bd)
	mod := Novo(cfg, bd, seguranca.Novo(cfg, bd), permissao.Novo(bd), arm, hist)

	p, err := principalCEO(ctx, bd)
	if err != nil {
		t.Fatalf("principal: %v", err)
	}
	if n, err := limparDadosTestePCO(ctx, bd, arm, p.ClienteID); err != nil {
		t.Fatalf("limpar antes do teste: %v", err)
	} else if n > 0 {
		t.Logf("limpei %d OC(s) de teste anteriores", n)
	}

	ids := map[string]string{}
	for _, c := range casos {
		pdf, err := os.ReadFile(filepath.Join(pasta, c.Arquivo))
		if err != nil {
			t.Fatalf("%s: %v", c.Arquivo, err)
		}
		hdr := cabecalhoMultipart(t, c.Arquivo, pdf)
		id, _, err := mod.guardarUma(ctx, p, hdr)
		if err != nil {
			t.Fatalf("inserir %s: %v", c.Arquivo, err)
		}
		ids[c.Arquivo] = id
	}

	fila, err := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "fila")+"&select=id&limit=1", nil)
	if err != nil {
		t.Fatalf("contar fila: %v", err)
	}
	if fila != len(casos) {
		t.Fatalf("fila após inserir: esperava %d, veio %d", len(casos), fila)
	}
	t.Logf("inserção: %d OCs na fila", fila)

	for _, c := range casos {
		id := ids[c.Arquivo]
		status, motivo, ex, err := mod.lerUmaIntegracao(ctx, p, id)
		if err != nil {
			t.Fatalf("ler %s: %v", c.Arquivo, err)
		}
		wantStatus := "lido"
		if c.Esperado == "REJEITADA" {
			wantStatus = "falhou"
		}
		if status != wantStatus {
			t.Fatalf("%s: status %q, esperava %q (motivo: %q)", c.Arquivo, status, wantStatus, motivo)
		}
		if ex.Numero != c.Numero {
			t.Fatalf("%s: numero %q, esperava %q", c.Arquivo, ex.Numero, c.Numero)
		}
		if ex.FornecedorNome != c.FornecedorNome && c.FornecedorCNPJ != "" {
			t.Fatalf("%s: fornecedor %q, esperava %q", c.Arquivo, ex.FornecedorNome, c.FornecedorNome)
		}
		if ex.CompradorCNPJ != c.CompradorCNPJ && c.CompradorCNPJ != "" {
			t.Fatalf("%s: comprador_cnpj %q, esperava %q", c.Arquivo, ex.CompradorCNPJ, c.CompradorCNPJ)
		}
		if len(ex.Itens) != c.NItens {
			t.Fatalf("%s: n_itens %d, esperava %d", c.Arquivo, len(ex.Itens), c.NItens)
		}
		if diff := ex.Total.Float() - c.Total; diff < -0.02 || diff > 0.02 {
			t.Fatalf("%s: total %v, esperava %v", c.Arquivo, ex.Total.Float(), c.Total)
		}
		if c.Esperado == "REJEITADA" && len(ex.MotivosDeRejeicao()) == 0 {
			t.Fatalf("%s: deveria ter motivo de rejeição", c.Arquivo)
		}
	}

	processadas, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "processadas")+"&select=id&limit=1", nil)
	rejeitadas, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "rejeitadas")+"&select=id&limit=1", nil)
	pendentes, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "pco-pendentes")+"&select=id&limit=1", nil)
	enviados, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "pco-enviados")+"&select=id&limit=1", nil)

	t.Logf("vistas: processadas=%d rejeitadas=%d pco-pendentes=%d pco-enviados=%d",
		processadas, rejeitadas, pendentes, enviados)

	if processadas != 15 {
		t.Fatalf("processadas: esperava 15, veio %d", processadas)
	}
	if rejeitadas != 15 {
		t.Fatalf("rejeitadas: esperava 15, veio %d", rejeitadas)
	}
	if pendentes != 15 {
		t.Fatalf("pco-pendentes: esperava 15, veio %d", pendentes)
	}
	if enviados != 0 {
		t.Fatalf("pco-enviados antes do envio: esperava 0, veio %d", enviados)
	}

	// --- envio PCO (só as 15 processadas) ---
	if !mod.correio.Ligado() {
		t.Log("SMTP_SENHA ausente — pulando envio real; vistas já conferidas")
		return
	}

	pendentesLista, err := mod.buscarPendentes(ctx, p.ClienteID, "")
	if err != nil {
		t.Fatalf("buscar pendentes: %v", err)
	}
	if len(pendentesLista) != 15 {
		t.Fatalf("pendentes para envio: esperava 15, veio %d", len(pendentesLista))
	}

	idsPendentes := idsDe(pendentesLista)
	agora := time.Now().UTC().Format(time.RFC3339)
	var confirmadas []struct {
		ID string `json:"id"`
	}
	filtroCAS := "id=in.(" + strings.Join(idsPendentes, ",") + ")&pco_enviado_em=is.null"
	if err := mod.bd.AtualizarDevolvendo(ctx, "ordens_compra", filtroCAS,
		map[string]any{"pco_enviado_em": agora}, &confirmadas); err != nil {
		t.Fatalf("marcar enviadas (CAS): %v", err)
	}
	if len(confirmadas) != 15 {
		t.Fatalf("CAS envio: esperava 15, veio %d", len(confirmadas))
	}

	if err := mod.enviarPorEmail(ctx, p.ClienteID, pendentesLista); err != nil {
		_ = mod.bd.Atualizar(ctx, "ordens_compra",
			"id=in.("+strings.Join(idsPendentes, ",")+")",
			map[string]any{"pco_enviado_em": nil})
		t.Fatalf("enviar e-mail: %v", err)
	}

	pendentesDepois, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "pco-pendentes")+"&select=id&limit=1", nil)
	enviadosDepois, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "pco-enviados")+"&select=id&limit=1", nil)
	processadasDepois, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "processadas")+"&select=id&limit=1", nil)

	if pendentesDepois != 0 {
		t.Fatalf("pco-pendentes após envio: esperava 0, veio %d", pendentesDepois)
	}
	if enviadosDepois != 15 {
		t.Fatalf("pco-enviados após envio: esperava 15, veio %d", enviadosDepois)
	}
	if processadasDepois != 15 {
		t.Fatalf("processadas após envio PCO: esperava 15 (permanecem), veio %d", processadasDepois)
	}
	t.Log("envio PCO: 15 OCs marcadas como enviadas; e-mail disparado pelo Brevo")
}

// TestEnviarPCOPendentes manda o pacote das OCs ainda pendentes (após leitura).
// Rodar: go test -tags=integracao -run TestEnviarPCOPendentes -v
func TestEnviarPCOPendentes(t *testing.T) {
	cfg, err := config.Carregar()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !cfg.Brevo.Ligado() {
		t.Fatal("BREVO_API_KEY não configurada — preencha o .env antes de enviar")
	}

	ctx := context.Background()
	bd := banco.Novo(cfg)
	arm := armazem.Novo(cfg)
	mod := Novo(cfg, bd, seguranca.Novo(cfg, bd), permissao.Novo(bd), arm, historico.Novo(bd))

	p, err := principalCEO(ctx, bd)
	if err != nil {
		t.Fatalf("principal: %v", err)
	}

	pendentesLista, err := mod.buscarPendentes(ctx, p.ClienteID, "")
	if err != nil {
		t.Fatalf("buscar pendentes: %v", err)
	}
	if len(pendentesLista) == 0 {
		t.Fatal("nenhuma OC pendente de envio — rode o fluxo completo antes ou insira OCs processadas")
	}
	t.Logf("pendentes de envio: %d OC(s)", len(pendentesLista))

	idsPendentes := idsDe(pendentesLista)
	agora := time.Now().UTC().Format(time.RFC3339)
	var confirmadas []struct {
		ID string `json:"id"`
	}
	filtroCAS := "id=in.(" + strings.Join(idsPendentes, ",") + ")&pco_enviado_em=is.null"
	if err := mod.bd.AtualizarDevolvendo(ctx, "ordens_compra", filtroCAS,
		map[string]any{"pco_enviado_em": agora}, &confirmadas); err != nil {
		t.Fatalf("marcar enviadas (CAS): %v", err)
	}
	if len(confirmadas) == 0 {
		t.Fatal("CAS não marcou nenhuma OC — outra chamada pode ter enviado antes")
	}
	enviar := pendentesLista[:0:0]
	confirmadasID := make(map[string]bool, len(confirmadas))
	for _, c := range confirmadas {
		confirmadasID[c.ID] = true
	}
	for _, o := range pendentesLista {
		if confirmadasID[o.ID] {
			enviar = append(enviar, o)
		}
	}

	if err := mod.enviarPorEmail(ctx, p.ClienteID, enviar); err != nil {
		idsRollback := idsDe(enviar)
		_ = mod.bd.Atualizar(ctx, "ordens_compra",
			"id=in.("+strings.Join(idsRollback, ",")+")",
			map[string]any{"pco_enviado_em": nil})
		t.Fatalf("enviar e-mail: %v", err)
	}

	n := len(enviar)
	pendentesDepois, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "pco-pendentes")+"&select=id&limit=1", nil)
	enviadosDepois, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "pco-enviados")+"&select=id&limit=1", nil)
	processadas, _ := mod.bd.BuscarContando(ctx, filtroDasOrdens(p.ClienteID, "processadas")+"&select=id&limit=1", nil)

	t.Logf("após envio: pendentes=%d enviados=%d processadas=%d", pendentesDepois, enviadosDepois, processadas)
	if pendentesDepois != 0 {
		t.Fatalf("pco-pendentes após envio: esperava 0, veio %d", pendentesDepois)
	}
	if enviadosDepois < n {
		t.Fatalf("pco-enviados: esperava pelo menos %d, veio %d", n, enviadosDepois)
	}
	t.Logf("e-mail PCO enviado pelo Brevo com %d OC(s) no zip", n)
}

// TestRecuperarPresasEmLendo relê OCs presas em "lendo" (bug do comprador_cnpj
// antes de 11/09/2026). Rodar: go test -tags=integracao -run TestRecuperarPresasEmLendo -v
func TestRecuperarPresasEmLendo(t *testing.T) {
	if _, err := execLookPath("pdftotext"); err != nil {
		t.Skip("pdftotext não está no PATH")
	}
	cfg, err := config.Carregar()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	bd := banco.Novo(cfg)
	arm := armazem.Novo(cfg)
	mod := Novo(cfg, bd, seguranca.Novo(cfg, bd), permissao.Novo(bd), arm, historico.Novo(bd))

	p, err := principalCEO(ctx, bd)
	if err != nil {
		t.Fatalf("principal: %v", err)
	}

	var presas []map[string]any
	caminho := "ordens_compra?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&status=eq.lendo&select=id,nome_arquivo&order=criado_em"
	if err := bd.Buscar(ctx, caminho, &presas); err != nil {
		t.Fatalf("buscar presas: %v", err)
	}
	if len(presas) == 0 {
		t.Log("nenhuma OC presa em lendo")
		return
	}
	t.Logf("OCs presas em lendo: %d", len(presas))

	var lidas, falhas int
	for _, row := range presas {
		id, _ := row["id"].(string)
		nome, _ := row["nome_arquivo"].(string)
		status, motivo, _, err := mod.lerUmaIntegracao(ctx, p, id)
		if err != nil {
			t.Fatalf("%s (%s): %v", nome, id, err)
		}
		if status == "lido" {
			lidas++
		} else {
			falhas++
		}
		t.Logf("%s → %s (%s)", nome, status, motivo)
	}

	var aindaPresas int
	aindaPresas, err = bd.BuscarContando(ctx, caminho+"&limit=1", nil)
	if err != nil {
		t.Fatalf("conferir presas: %v", err)
	}
	if aindaPresas != 0 {
		t.Fatalf("ainda restam %d OC(s) em lendo", aindaPresas)
	}
	t.Logf("recuperação OK: %d lida(s), %d rejeitada(s)", lidas, falhas)
}

func (m *Modulo) lerUmaIntegracao(ctx context.Context, p *seguranca.Principal, id string) (status, motivo string, ex Extraida, err error) {
	var tomadas []map[string]any
	if err = m.bd.AtualizarDevolvendo(ctx, "ordens_compra",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&status=in.("+statusPorLer+")",
		map[string]any{"status": "lendo", "erro_leitura": nil}, &tomadas); err != nil {
		return "", "", Extraida{}, err
	}
	if len(tomadas) == 0 {
		return "", "", Extraida{}, errNaoAchei
	}
	sha, _ := tomadas[0]["arquivo_sha256"].(string)
	arq, err := m.contarUm(ctx, "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		return "", "", Extraida{}, err
	}
	chave, _ := arq["chave_r2"].(string)
	pdf, err := m.arm.Baixar(ctx, chave)
	if err != nil {
		return "", "", Extraida{}, err
	}
	ex, err = Ler(ctx, pdf)
	if err != nil {
		return "", "", Extraida{}, err
	}
	fornecedorID, _ := m.resolverFornecedor(ctx, p.ClienteID, ex)
	status, motivo = "lido", ""
	if motivos := ex.MotivosDeRejeicao(); len(motivos) > 0 {
		status, motivo = "falhou", strings.Join(motivos, "; ")
	}
	if err := m.terminarLeitura(ctx, p, id, status, motivo, camposLidos(ex, fornecedorID), &ex); err != nil {
		return "", "", Extraida{}, err
	}
	return status, motivo, ex, nil
}

func principalCEO(ctx context.Context, bd *banco.Cliente) (*seguranca.Principal, error) {
	var clientes []map[string]any
	if err := bd.Buscar(ctx, "clientes?select=id,nome&limit=1", &clientes); err != nil {
		return nil, err
	}
	if len(clientes) == 0 {
		return nil, errNaoAchei
	}
	clienteID, _ := clientes[0]["id"].(string)
	var perfis []map[string]any
	caminho := "perfis?cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=id,usuario,nome,cliente_id,categoria_id,categorias(codigo,nome,nivel)&ativo=eq.true"
	if err := bd.Buscar(ctx, caminho, &perfis); err != nil {
		return nil, err
	}
	var escolhido map[string]any
	for _, p := range perfis {
		cat, _ := p["categorias"].(map[string]any)
		if cat == nil {
			continue
		}
		cod, _ := cat["codigo"].(string)
		niv, _ := cat["nivel"].(string)
		if cod == "ceo" || niv == "builder" {
			escolhido = p
			break
		}
	}
	if escolhido == nil && len(perfis) > 0 {
		escolhido = perfis[0]
	}
	if escolhido == nil {
		return nil, errNaoAchei
	}
	principal := &seguranca.Principal{
		Tipo:      seguranca.TipoUsuario,
		UserID:    escolhido["id"].(string),
		ClienteID: clienteID,
		Ativo:     true,
	}
	if u, ok := escolhido["usuario"].(string); ok {
		principal.Usuario = u
	}
	if n, ok := escolhido["nome"].(string); ok {
		principal.Nome = n
	}
	if cat, ok := escolhido["categorias"].(map[string]any); ok {
		if n, ok := cat["nome"].(string); ok {
			principal.CategoriaNome = n
		}
		if n, ok := cat["nivel"].(string); ok {
			principal.Nivel = n
		}
	}
	return principal, nil
}

func cabecalhoMultipart(t *testing.T, nome string, pdf []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("arquivos", nome)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(pdf); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(TamanhoMaximo); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["arquivos"][0]
}

func repoRaiz(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "OCs_Teste", "_gabarito.json")); err == nil {
			return dir
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			t.Fatal("não achei OCs_Teste/_gabarito.json subindo a partir de " + dir)
		}
		dir = pai
	}
}

func execLookPath(name string) (string, error) {
	return exec.LookPath(name)
}
