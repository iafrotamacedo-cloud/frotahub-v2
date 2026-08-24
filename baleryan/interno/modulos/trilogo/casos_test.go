package trilogo

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLeituraGravaOChamadoInteiro(t *testing.T) {
	m := novoMundo(t)
	m.chamado(131712, 82, "", []map[string]any{
		{"id": 453201, "fileName": "foto.jpg", "image": m.arquivo.URL + "/foto.jpg",
			"author": "Operação - Loja 13", "authorId": 32571, "creationDateTime": "2026-08-22T12:45:55.542"},
	}, []map[string]any{
		{"id": 588923, "recordType": 0, "statusAction": 5, "isStatus": true,
			"creationDate": "2026-08-22T15:22:18.711", "authorName": "Frota Instalações", "authorId": 31274},
	})
	m.conteudo[m.arquivo.URL+"/foto.jpg"] = []byte(strings.Repeat("x", 2048))

	r, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	// as duas contas leem a MESMA lista de mentira, então tudo aparece 2x
	if len(m.logins) != 2 {
		t.Fatalf("tinha que entrar nas duas contas, entrou em %d", len(m.logins))
	}
	c := m.linhas("chamados")
	if len(c) == 0 {
		t.Fatal("nenhum chamado gravado")
	}
	l := c[0]
	confere(t, l, "numero", float64(131712))
	confere(t, l, "status_codigo", float64(5))
	confere(t, l, "status", "Executado")
	confere(t, l, "prioridade_codigo", float64(2))
	confere(t, l, "prioridade", "Baixa") // o robô antigo grava "Média" aqui
	confere(t, l, "tipo_predial", "Eletrica")
	confere(t, l, "ambiente", "LOJA X > Térreo > Açougue")
	confere(t, l, "natureza", "Manual")
	confere(t, l, "prestadora_cnpj", "27363223000170")

	if r.ArquivosVistos == 0 || r.BytesVistos != 2048*2 {
		t.Fatalf("o tamanho tinha que vir do cabeçalho: vistos=%d bytes=%d", r.ArquivosVistos, r.BytesVistos)
	}
}

func TestOHorarioNaoEscorregaTresHoras(t *testing.T) {
	m := novoMundo(t)
	m.chamado(1, 82, "", nil, nil)
	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	criado, _ := m.linhas("chamados")[0]["criado_em"].(string)
	got, err := time.Parse(time.RFC3339, criado)
	if err != nil {
		t.Fatalf("data ilegível: %q", criado)
	}
	// 12:45 na tela do Trílogo é 12:45 em Fortaleza, ou seja 15:45 UTC.
	if got.UTC().Hour() != 15 || got.UTC().Minute() != 45 {
		t.Fatalf("o horário escorregou: %s (esperava 15:45 UTC)", got.UTC().Format(time.RFC3339))
	}
}

func TestLojaForaDoEscopoNaoEntra(t *testing.T) {
	m := novoMundo(t)
	m.foraDoEscopo = []int{94}
	m.chamado(111, 94, "", nil, nil) // Crato
	m.chamado(222, 82, "", nil, nil) // Santos Dumont
	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	for _, l := range m.linhas("chamados") {
		if l["numero"] == float64(111) {
			t.Fatal("o chamado da loja fora do escopo entrou")
		}
	}
	if len(m.linhas("chamados")) == 0 {
		t.Fatal("o chamado da loja no escopo tinha que entrar")
	}
}

func TestChamadoAnteriorAoCorteNaoEntra(t *testing.T) {
	m := novoMundo(t)
	m.chamado(999, 82, "antigo", nil, nil) // 15/06/2026, antes de 01/07
	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	if len(m.linhas("chamados")) != 0 {
		t.Fatalf("chamado anterior a 01/07/2026 entrou: %v", m.linhas("chamados"))
	}
}

func TestEventoSemIdGanhaChaveEstavel(t *testing.T) {
	m := novoMundo(t)
	semID := map[string]any{"id": 0, "recordType": 54, "creationDate": "2026-08-22T13:02:13",
		"authorName": "Junior", "authorId": 900, "description": "Frota Instalações"}
	comID := map[string]any{"id": 588923, "recordType": 0, "statusAction": 5, "isStatus": true,
		"creationDate": "2026-08-22T15:22:18.711", "authorName": "Frota Instalações", "authorId": 31274}
	m.chamado(1, 82, "", nil, []map[string]any{comID, semID})

	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	ev := m.linhas("chamado_eventos")
	if len(ev) < 2 {
		t.Fatalf("esperava os dois eventos, vieram %d", len(ev))
	}
	var chaves []string
	for _, e := range ev {
		chaves = append(chaves, e["chave"].(string))
	}
	if !contem(chaves, "588923") {
		t.Fatalf("o evento com id tinha que usar o id: %v", chaves)
	}
	var hash string
	for _, k := range chaves {
		if strings.HasPrefix(k, "h:") {
			hash = k
		}
	}
	if hash == "" {
		t.Fatalf("o evento SEM id tinha que ganhar impressão digital: %v", chaves)
	}

	// a mesma leitura de novo tem que produzir A MESMA chave — senão duplica
	m2 := novoMundo(t)
	m2.chamado(1, 82, "", nil, []map[string]any{semID})
	if _, err := m2.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	if k := m2.linhas("chamado_eventos")[0]["chave"].(string); k != hash {
		t.Fatalf("a chave mudou entre leituras: %s != %s", k, hash)
	}
}

func TestFichaDeArquivoNaoApagaOVinculo(t *testing.T) {
	m := novoMundo(t)
	m.chamado(1, 82, "", []map[string]any{
		{"id": 1, "fileName": "a.jpg", "image": m.arquivo.URL + "/a.jpg"},
	}, nil)
	m.conteudo[m.arquivo.URL+"/a.jpg"] = []byte("abc")
	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	f := m.linhas("chamado_anexos")[0]
	if _, tem := f["arquivo_sha256"]; tem {
		t.Fatal("a ficha NÃO pode mandar arquivo_sha256: apagaria o vínculo de um arquivo já copiado")
	}
	confere(t, f, "colecao", "anexo")
	confere(t, f, "tamanho", float64(3))
}

func TestOrcamentoDoSistemaAntigoEReconhecido(t *testing.T) {
	m := novoMundo(t)
	m.chamado(1, 82, "", nil, nil)
	m.custos[1] = []map[string]any{{
		"id": 38238, "type": 2, "totalValue": 18.0, "documentNumber": "1",
		"invoiceFiles": []map[string]any{{"id": 453046, "fileName": "tmphymsviw2.pdf", "permalink": m.arquivo.URL + "/o.pdf"}},
	}}
	m.conteudo[m.arquivo.URL+"/o.pdf"] = []byte("pdf")
	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	var orc map[string]any
	for _, f := range m.linhas("chamado_anexos") {
		if f["colecao"] == "orcamento" {
			orc = f
		}
	}
	if orc == nil {
		t.Fatal("o orçamento não virou ficha")
	}
	confere(t, orc, "origem", "sistema-antigo")
	if orc["custo_id"] == nil {
		t.Fatal("a ficha do orçamento tinha que apontar para o custo")
	}
	if len(m.linhas("chamado_custos")) == 0 {
		t.Fatal("o custo não foi gravado")
	}
	confere(t, m.linhas("chamado_custos")[0], "tipo", "Materiais")
}

func TestCopiaNaoDuplicaConteudoRepetido(t *testing.T) {
	m := novoMundo(t)
	m.conteudo[m.arquivo.URL+"/um.pdf"] = []byte("mesmo conteudo")
	m.conteudo[m.arquivo.URL+"/dois.pdf"] = []byte("mesmo conteudo")
	m.pendentes = []map[string]any{
		{"id": "a1", "url_origem": m.arquivo.URL + "/um.pdf", "nome": "ORC.pdf", "extensao": "pdf"},
	}
	svc := m.servico()

	r, err := svc.Rodar(context.Background(), ModoCopia, cliente, "teste")
	if err != nil {
		t.Fatal(err)
	}
	if r.ArquivosCopiados != 1 {
		t.Fatalf("esperava 1 cópia, vieram %d", r.ArquivosCopiados)
	}
	if len(m.enviado) != 1 {
		t.Fatalf("esperava 1 objeto no armazém, vieram %d", len(m.enviado))
	}
	var chave string
	for k := range m.enviado {
		chave = k
	}

	// agora o MESMO conteúdo aparece por outro endereço — é o ciclo do orçamento
	m.arquivosJaTem = []string{shaDe(m.linhas("arquivos"))}
	m.pendentes = []map[string]any{
		{"id": "a2", "url_origem": m.arquivo.URL + "/dois.pdf", "nome": "ORC.pdf", "extensao": "pdf"},
	}
	r2, err := svc.Rodar(context.Background(), ModoCopia, cliente, "teste")
	if err != nil {
		t.Fatal(err)
	}
	if r2.ArquivosCopiados != 0 {
		t.Fatalf("não podia copiar de novo: %d", r2.ArquivosCopiados)
	}
	if len(m.enviado) != 1 {
		t.Fatalf("o armazém tinha que continuar com 1 objeto, tem %d", len(m.enviado))
	}
	if len(m.linhas("arquivos")) != 1 {
		t.Fatalf("tinha que haver 1 linha de arquivo, há %d", len(m.linhas("arquivos")))
	}

	// e as DUAS aparições apontam para o mesmo conteúdo
	vinculos := m.linhas("PATCH:chamado_anexos")
	if len(vinculos) != 2 {
		t.Fatalf("esperava 2 vínculos, vieram %d", len(vinculos))
	}
	if vinculos[0]["arquivo_sha256"] != vinculos[1]["arquivo_sha256"] {
		t.Fatal("as duas aparições tinham que apontar para o mesmo sha256")
	}
	if !strings.Contains(chave, vinculos[0]["arquivo_sha256"].(string)) {
		t.Fatalf("o caminho no armazém tinha que ser o sha256: %s", chave)
	}
}

func TestCredencialErradaFalaClaro(t *testing.T) {
	m := novoMundo(t)
	svc := m.servico()
	svc.cfg.Trilogo.SenhaInstalacoes = "errada"
	_, err := svc.Rodar(context.Background(), ModoLevantamento, cliente, "teste")
	if err == nil || !strings.Contains(err.Error(), "recusou o login") {
		t.Fatalf("esperava erro de login legível, veio: %v", err)
	}
	// A FRASE do Trílogo tem que chegar ao log. Ele responde 400 tanto para
	// pedido malformado quanto para senha errada; sem a frase, as duas falhas
	// ficam iguais e a pessoa troca a senha certa achando que o problema é ela.
	if !strings.Contains(err.Error(), "Email ou senha incorretos") {
		t.Fatalf("faltou a explicação do Trílogo no erro: %v", err)
	}
}

// O nome dos campos do login não é adivinhável, e errá-lo custou uma rodada
// inteira do Actions. Este teste existe para não custar duas.
func TestLoginMandaOsNomesCertos(t *testing.T) {
	m := novoMundo(t)
	m.chamado(1, 82, "", nil, nil)
	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatalf("o login com UserEmail/UserPassword tinha que passar: %v", err)
	}
	if len(m.logins) != 2 {
		t.Fatalf("esperava entrar nas duas contas, entrou em %d", len(m.logins))
	}
}

func TestSemCredencialNaoTenta(t *testing.T) {
	m := novoMundo(t)
	svc := m.servico()
	svc.cfg.Trilogo.SenhaCivil = ""
	if _, err := svc.Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err == nil {
		t.Fatal("tinha que recusar antes de tentar")
	}
	if len(m.logins) != 0 {
		t.Fatal("não podia ter tentado entrar")
	}
}

// ---------------------------------------------------------------------------

func confere(t *testing.T, l map[string]any, campo string, esperado any) {
	t.Helper()
	if l[campo] != esperado {
		t.Errorf("%s: esperava %v (%T), veio %v (%T)", campo, esperado, esperado, l[campo], l[campo])
	}
}

func contem(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func shaDe(linhas []map[string]any) string {
	if len(linhas) == 0 {
		return ""
	}
	s, _ := linhas[0]["sha256"].(string)
	return s
}

// A primeira carga de verdade morreu aqui, depois de ler 1.050 chamados: um
// chamado sem ambiente gerava uma linha com uma chave a menos, e o PostgREST
// recusa o LOTE INTEIRO nesse caso. Este teste mistura chamados completos e
// incompletos de propósito.
func TestChamadosDesiguaisNaoQuebramOLote(t *testing.T) {
	m := novoMundo(t)

	// completo
	m.chamado(1, 82, "", []map[string]any{
		{"id": 1, "fileName": "a.jpg", "image": m.arquivo.URL + "/a.jpg", "author": "Fulano", "authorId": 7},
	}, nil)
	m.conteudo[m.arquivo.URL+"/a.jpg"] = []byte("aa")

	// pelado: sem ambiente, sem tipo predial, sem quem criou, sem anexo
	m.chamado(2, 82, "", nil, nil)
	delete(m.detalhes[2], "department")
	delete(m.detalhes[2], "buildingServiceType")
	delete(m.detalhes[2], "creator")

	// com orçamento, que na tabela de anexos tem custo e não tem autor
	m.chamado(3, 82, "", []map[string]any{
		{"id": 9, "fileName": "b.jpg", "image": m.arquivo.URL + "/b.jpg", "author": "Beltrano", "authorId": 8},
	}, nil)
	m.conteudo[m.arquivo.URL+"/b.jpg"] = []byte("bb")
	m.custos[3] = []map[string]any{{
		"id": 55, "type": 2, "totalValue": 10.0,
		"invoiceFiles": []map[string]any{{"id": 77, "fileName": "tmpx.pdf", "permalink": m.arquivo.URL + "/o.pdf"}},
	}}
	m.conteudo[m.arquivo.URL+"/o.pdf"] = []byte("pp")

	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatalf("o lote misturado tinha que passar: %v", err)
	}
	if len(m.linhas("chamados")) < 3 {
		t.Fatalf("esperava os três chamados, vieram %d", len(m.linhas("chamados")))
	}
	// e o chamado pelado não pode ter inventado valor nenhum
	for _, l := range m.linhas("chamados") {
		if l["numero"] == float64(2) {
			for _, campo := range []string{"ambiente", "tipo_predial", "criado_por"} {
				if l[campo] != nil {
					t.Errorf("o chamado sem %s tinha que gravar nulo, gravou %v", campo, l[campo])
				}
			}
		}
	}
	// foto e orçamento convivem no mesmo lote de anexos
	var temFoto, temOrc bool
	for _, f := range m.linhas("chamado_anexos") {
		if f["colecao"] == "anexo" {
			temFoto = true
		}
		if f["colecao"] == "orcamento" {
			temOrc = true
		}
	}
	if !temFoto || !temOrc {
		t.Fatalf("esperava foto e orçamento no mesmo lote: foto=%v orcamento=%v", temFoto, temOrc)
	}
}

// Trinta e oito chamados da Instalações foram parar na Civil na primeira carga,
// só porque a Civil é lida depois. Quem manda é a prestadora, não a ordem.
func TestAContaVemDaPrestadoraNaoDeQuemLeu(t *testing.T) {
	casos := []struct{ prestadora, leu, quer string }{
		{"FROTA - INSTALAÇÕES", "civil", "instalacoes"},
		{"FROTA - INSTALAÇÕES", "instalacoes", "instalacoes"},
		{"FROTA - CIVIL", "instalacoes", "civil"},
		{"FROTA - CIVIL", "civil", "civil"},
		// empresa de fora: fica com quem enxerga; a verdade está em `prestadora`
		{"DESENTUPIDORA QUALIDADE", "instalacoes", "instalacoes"},
		{"", "civil", "civil"},
	}
	for _, c := range casos {
		if got := ContaDe(c.prestadora, c.leu); got != c.quer {
			t.Errorf("prestadora %q lida por %q: esperava %q, veio %q", c.prestadora, c.leu, c.quer, got)
		}
	}
}

// O chamado lido pela conta errada tem que ser gravado na conta certa.
func TestChamadoDaInstalacoesLidoPelaCivilFicaNaInstalacoes(t *testing.T) {
	m := novoMundo(t)
	m.chamado(1, 82, "", nil, nil)
	// o mundo de mentira faz as DUAS contas lerem a mesma lista; a Civil lê por
	// último, então antes da correção este chamado terminava como 'civil'
	m.detalhes[1]["serviceCompany"] = map[string]any{"name": "FROTA - INSTALAÇÕES", "cnpj": "27363223000170"}

	if _, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	linhas := m.linhas("chamados")
	if len(linhas) == 0 {
		t.Fatal("nada gravado")
	}
	for _, l := range linhas {
		if l["conta"] != "instalacoes" {
			t.Fatalf("esperava conta instalacoes, veio %v", l["conta"])
		}
	}
}

// Os tipos de evento batizados a partir dos 17 mil da carga inicial.
func TestTiposDeEventoBatizados(t *testing.T) {
	for codigo, quer := range map[int]string{
		23: "interrupcao", 57: "fornecedor", 58: "relacionamento",
		75: "duplicacao", 79: "prestadora", 103: "responsavel", 4: "status",
	} {
		if got := RotuloEvento(codigo); got != quer {
			t.Errorf("recordType %d: esperava %q, veio %q", codigo, quer, got)
		}
	}
	// e o que ninguém mapeou continua VISÍVEL, não vira vazio
	if got := RotuloEvento(999); got != "codigo 999" {
		t.Errorf("código desconhecido tinha que aparecer, veio %q", got)
	}
}

// A decisão do dono: vídeo não vem para o armazém, fica só o endereço no Trílogo.
// 678 vídeos carregavam 86% do peso do acervo.
func TestVideoFicaSoComOLink(t *testing.T) {
	m := novoMundo(t)
	m.chamado(1, 82, "", []map[string]any{
		{"id": 1, "fileName": "obra.mp4", "image": m.arquivo.URL + "/v.mp4"},
		{"id": 2, "fileName": "quadro.jpg", "image": m.arquivo.URL + "/f.jpg"},
		{"id": 3, "fileName": "antes.MOV", "image": m.arquivo.URL + "/w.mov"},
	}, nil)
	m.conteudo[m.arquivo.URL+"/v.mp4"] = []byte(strings.Repeat("v", 5000))
	m.conteudo[m.arquivo.URL+"/f.jpg"] = []byte(strings.Repeat("f", 100))
	m.conteudo[m.arquivo.URL+"/w.mov"] = []byte(strings.Repeat("w", 3000))

	r, err := m.servico().Rodar(context.Background(), ModoLevantamento, cliente, "teste")
	if err != nil {
		t.Fatal(err)
	}
	porNome := map[string]map[string]any{}
	for _, f := range m.linhas("chamado_anexos") {
		porNome[f["nome"].(string)] = f
	}
	if porNome["quadro.jpg"]["copiar"] != true {
		t.Error("a foto tinha que ser copiada")
	}
	if porNome["obra.mp4"]["copiar"] != false {
		t.Error("o vídeo NÃO podia ser marcado para cópia")
	}
	// maiúscula não pode enganar a regra
	if porNome["antes.MOV"]["copiar"] != false {
		t.Error(".MOV em maiúscula também é vídeo")
	}
	// o endereço do Trílogo continua guardado — é ele que a tela vai abrir
	if porNome["obra.mp4"]["url_origem"] == nil || porNome["obra.mp4"]["url_origem"] == "" {
		t.Error("o vídeo tem que manter o endereço no Trílogo")
	}
	// e o log tem que separar os dois números
	if r.ArquivosSoLink != 4 { // 2 vídeos × 2 contas no mundo de mentira
		t.Errorf("esperava 4 só-link, vieram %d", r.ArquivosSoLink)
	}
	if r.BytesSoLink != (5000+3000)*2 {
		t.Errorf("o peso do que fica no Trílogo saiu errado: %d", r.BytesSoLink)
	}
}

// A fila de cópia não pode carregar para sempre o que nunca vai ser copiado.
func TestFilaDeCopiaIgnoraOQueFicaNoTrilogo(t *testing.T) {
	m := novoMundo(t)
	m.conteudo[m.arquivo.URL+"/f.jpg"] = []byte("foto")
	m.pendentes = []map[string]any{
		{"id": "a1", "url_origem": m.arquivo.URL + "/f.jpg", "nome": "f.jpg", "extensao": "jpg"},
	}
	if _, err := m.servico().Rodar(context.Background(), ModoCopia, cliente, "teste"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.ultimaConsulta, "copiar=is.true") {
		t.Fatalf("a fila tinha que filtrar por copiar=is.true; consultou: %s", m.ultimaConsulta)
	}
}
