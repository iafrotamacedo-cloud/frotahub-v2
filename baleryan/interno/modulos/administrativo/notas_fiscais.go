// rev 1 — Notas Fiscais: o Bloco A (12/09/2026)
//
// O CICLO, DO JEITO QUE O DONO DESCREVEU
//
//	Toda OC que anda o ciclo inteiro (chega a "Enviados" em PCO e não é
//	excluída) espera uma ou mais notas fiscais. O almoxarife na obra escaneia
//	o material recebido — "recebida". Alguém leva a nota física até o
//	escritório, e quem recebe lá confirma — "entregue_escritorio" (essa
//	confirmação, com login de quem recebeu, é o protocolo de amanhã, Bloco
//	B). Por fim, o malote sai para o cliente — "enviada_cliente", fim do
//	ciclo OC-NF. Uma nota nunca cobre mais de uma OC, mas uma OC pode
//	receber várias notas em momentos diferentes (recebimento parcial) — ver
//	o cabeçalho da migração 064.
//
// A OC "COMPLETA" É CALCULADA, NUNCA GRAVADA
//
//	`nf_progresso_ordens` (migração 065) soma as notas não-canceladas de
//	cada OC e compara com o total — a mesma disciplina de nunca duplicar um
//	dado que já existe em outro lugar (CORE-06).
//
// SÓ QUEM TEM A OBRA LIBERADA RECEBE NOTA DELA
//
//	"o almoxarife só pode receber uma nota derivada de uma oc que foi
//	gerada" — e só das obras que alguém de nível superior concedeu a ele
//	(`centro_custo_acessos`, migração 064; concessão em `acessos_obra.go`).
//	O builder e quem tem `COMPRAS_NF_CONFIGURAR_ACESSO` (hoje só CEO) passam
//	direto, sem precisar de concessão — mesma exceção de sempre.
package administrativo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// porteiros — um por rotina, mesmo desenho de `quemPodeDestinatarios`
// ---------------------------------------------------------------------------

func (m *Modulo) quemPodeReceberNF(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	return m.quemComRotina(w, r, RotinaNFReceber)
}

func (m *Modulo) quemPodeEntregarNF(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	return m.quemComRotina(w, r, RotinaNFEntregar)
}

func (m *Modulo) quemPodeEnviarNFAoCliente(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	return m.quemComRotina(w, r, RotinaNFEnviarCliente)
}

func (m *Modulo) quemComRotina(w http.ResponseWriter, r *http.Request, rotina string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, rotina); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// quemComQualquerRotina autentica uma vez e aceita a primeira rotina, das
// listadas, que o principal alcançar — usado onde mais de uma rotina de NF dá
// acesso à mesma consulta (o painel dos 4 cartões interessa tanto a quem
// recebe quanto a quem entrega).
//
// NUNCA chame `quemComRotina`/`quemPodeReceberNF`/`quemPodeEntregarNF` em
// sequência para este caso ("tenta A, se não tiver tenta B"): cada uma delas
// já escreve a resposta de erro sozinha (`web.Falhar`) assim que a primeira
// rotina falha — e como um `http.ResponseWriter` só aceita o primeiro
// `WriteHeader`, o cliente recebia o 403 da tentativa QUE FALHOU, mesmo
// quando a segunda tentativa passava. Foi exatamente isto que deixou o CEO
// (só com `COMPRAS_NF_ENTREGAR`, sem `COMPRAS_NF_RECEBER`) sem ver o painel
// de Notas Fiscais: a primeira checagem (Receber) escrevia o 403 antes da
// segunda (Entregar) sequer rodar.
func (m *Modulo) quemComQualquerRotina(w http.ResponseWriter, r *http.Request, rotinas ...string) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	for _, rotina := range rotinas {
		pode, err := m.perm.Pode(r.Context(), p, rotina)
		if err != nil {
			m.erro(w, "não consegui conferir sua permissão", err)
			return nil
		}
		if !pode {
			continue
		}
		if p.ClienteID == "" {
			web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
			return nil
		}
		return p
	}
	web.Falhar(w, http.StatusForbidden, "Você não tem acesso a esta rotina.")
	return nil
}

// temAcessoAObra confere `centro_custo_acessos` — a concessão por obra que só
// quem tem `COMPRAS_NF_CONFIGURAR_ACESSO` (ou o builder) pode dar (ver
// `acessos_obra.go`). Quem já tem essa rotina de configurar passa direto: não
// faria sentido um CEO precisar conceder acesso a si mesmo.
func (m *Modulo) temAcessoAObra(ctx context.Context, p *seguranca.Principal, obraCentroCusto string) (bool, error) {
	if p.Builder() {
		return true, nil
	}
	if pode, err := m.perm.Pode(ctx, p, RotinaNFConfigurarAcesso); err == nil && pode {
		return true, nil
	}
	obra := strings.TrimSpace(obraCentroCusto)
	if obra == "" {
		return false, nil
	}
	centro, err := m.contarUm(ctx, "centros_custo?cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&obra_centro_custo=eq."+banco.Escapar(obra)+"&select=id&limit=1")
	if err != nil {
		if err == errNaoAchei {
			return false, nil
		}
		return false, err
	}
	centroID := strCampo(centro["id"])
	if centroID == "" {
		return false, nil
	}
	var linhas []map[string]any
	if err := m.bd.Buscar(ctx, "centro_custo_acessos?perfil_id=eq."+banco.Escapar(p.UserID)+
		"&centro_custo_id=eq."+banco.Escapar(centroID)+"&select=id&limit=1", &linhas); err != nil {
		return false, err
	}
	return len(linhas) > 0, nil
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/painel — os quatro cartões do hub
// ---------------------------------------------------------------------------

func (m *Modulo) painelDeNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemComQualquerRotina(w, r, RotinaNFReceber, RotinaNFEntregar, RotinaNFEnviarCliente)
	if p == nil {
		return
	}
	aguardando, err := m.bd.BuscarContando(r.Context(),
		"nf_progresso_ordens?cliente_id=eq."+banco.Escapar(p.ClienteID)+"&completa=eq.false&select=ordem_compra_id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as OCs aguardando nota fiscal", err)
		return
	}
	recebidas, err := m.bd.BuscarContando(r.Context(), filtroDasNF(p.ClienteID, "recebidas")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as notas recebidas", err)
		return
	}
	entregues, err := m.bd.BuscarContando(r.Context(), filtroDasNF(p.ClienteID, "entregues")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as notas entregues no escritório", err)
		return
	}
	enviadas, err := m.bd.BuscarContando(r.Context(), filtroDasNF(p.ClienteID, "enviadas")+"&select=id&limit=1", nil)
	if err != nil {
		m.erro(w, "não consegui contar as notas enviadas ao cliente", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"aguardando": aguardando,
		"recebidas":  recebidas,
		"entregues":  entregues,
		"enviadas":   enviadas,
	})
}

func filtroDasNF(clienteID, vista string) string {
	base := "notas_fiscais?cliente_id=eq." + banco.Escapar(clienteID) + "&cancelada=eq.false"
	switch vista {
	case "entregues":
		return base + "&status=eq.entregue_escritorio&order=entregue_escritorio_em.desc"
	case "enviadas":
		return base + "&status=eq.enviada_cliente&order=enviada_cliente_em.desc"
	default:
		return base + "&status=eq.recebida&order=recebida_em.desc"
	}
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/aguardando — OCs enviadas, ainda sem nota completa
// ---------------------------------------------------------------------------

func (m *Modulo) ordensAguardandoNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := "nf_progresso_ordens?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&completa=eq.false&order=numero" +
		"&select=ordem_compra_id,numero,obra_centro_custo,fornecedor_id,total,recebido,restante" +
		"&limit=" + fmt.Sprint(TetoDaLista)
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar as OCs aguardando nota fiscal", err)
		return
	}
	linhas = m.comAcessoNaObra(r.Context(), p, linhas)
	m.comNomeDoFornecedor(r.Context(), linhas)
	web.Responder(w, http.StatusOK, map[string]any{"ordens": ouVazio(linhas)})
}

// comAcessoNaObra filtra a lista para as obras que ESTE almoxarife pode
// receber — quem tem `COMPRAS_NF_CONFIGURAR_ACESSO` (ou o builder) já vê
// tudo, sem essa peneira (ver `temAcessoAObra`).
func (m *Modulo) comAcessoNaObra(ctx context.Context, p *seguranca.Principal, linhas []map[string]any) []map[string]any {
	if p.Builder() {
		return linhas
	}
	if pode, err := m.perm.Pode(ctx, p, RotinaNFConfigurarAcesso); err == nil && pode {
		return linhas
	}
	saida := make([]map[string]any, 0, len(linhas))
	for _, l := range linhas {
		ok, err := m.temAcessoAObra(ctx, p, strCampo(l["obra_centro_custo"]))
		if err == nil && ok {
			saida = append(saida, l)
		}
	}
	return saida
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/ordens/{id}/arquivo — abrir a OC (17/09/2026)
// ---------------------------------------------------------------------------
//
// O card de "Aguardando NF" ganhou um link pra ver a OC original — pedido do
// dono, obra piloto MSL Fátima. Existe `arquivoDaOrdem` em ordens.go fazendo
// a mesma coisa, mas atrás de `COMPRAS_ORDENS_GERENCIAR` — rotina de
// Compras, que o almoxarife que recebe nota não tem. Reusar aquela rota
// devolveria 403 bem na cara de quem o link foi feito pra atender. Esta é a
// mesma busca, atrás da rotina certa (`RotinaNFReceber`) e peneirada pela
// MESMA regra de obra que já protege a lista de Aguardando
// (`temAcessoAObra`): só abre o arquivo de uma OC cuja obra este perfil tem
// liberada — a permissão de configurar acesso por obra não fica maior só
// porque ganhou um atalho novo.
func (m *Modulo) arquivoDaOrdemNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=nome_arquivo,arquivo_sha256,obra_centro_custo&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	liberado, err := m.temAcessoAObra(r.Context(), p, strCampo(ordem["obra_centro_custo"]))
	if err != nil {
		m.erro(w, "não consegui conferir o acesso a esta obra", err)
		return
	}
	if !liberado {
		web.Falhar(w, http.StatusForbidden, "Você não tem esta obra liberada.")
		return
	}
	sha, _ := ordem["arquivo_sha256"].(string)
	if sha == "" {
		web.Falhar(w, http.StatusNotFound, "Esta ordem de compra não tem arquivo guardado.")
		return
	}
	arq, err := m.contarUm(r.Context(), "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		m.erro(w, "não achei o arquivo", err)
		return
	}
	chave, _ := arq["chave_r2"].(string)
	link, err := m.arm.LinkTemporario(chave, ValidadeDoLink)
	if err != nil {
		m.erro(w, "não consegui montar o endereço do arquivo", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"url":  link,
		"nome": ordem["nome_arquivo"],
	})
}

func (m *Modulo) comNomeDoFornecedor(ctx context.Context, linhas []map[string]any) {
	for _, l := range linhas {
		fid := strCampo(l["fornecedor_id"])
		if fid == "" {
			continue
		}
		if f, err := m.contarUm(ctx, "fornecedores?id=eq."+banco.Escapar(fid)+"&select=razao_social&limit=1"); err == nil {
			l["fornecedor_nome"] = f["razao_social"]
		}
	}
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/ordens/{id} — as notas já recebidas de uma OC
// ---------------------------------------------------------------------------

func (m *Modulo) notasDaOrdem(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=id,numero,obra_centro_custo,total&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	var notas []map[string]any
	if err := m.bd.Buscar(r.Context(), "notas_fiscais?ordem_compra_id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&order=recebida_em.desc"+
		"&select=id,numero,valor,status,cancelada,motivo_cancelamento,recebida_em,entregue_escritorio_em,enviada_cliente_em",
		&notas); err != nil {
		m.erro(w, "não consegui listar as notas fiscais desta ordem de compra", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"ordem": ordem, "notas": ouVazio(notas)})
}

// ---------------------------------------------------------------------------
// POST /administrativo/nf/ordens/{id}/receber — o almoxarife escaneia a NF
// ---------------------------------------------------------------------------

// UMA NOTA, TODAS AS PÁGINAS, UM RECEBIMENTO SÓ (15/09/2026)
//
//	A rev 1 salvava página a página: a primeira criava a nota, as
//	seguintes iam para /notas/{id}/paginas. Na prática o almoxarife
//	fechava a janela depois da página 1 e abria de novo para a página 2 —
//	e a OC ganhava DOIS recebimentos da mesma nota. Agora o celular
//	escaneia todas as páginas primeiro (scanner com "próxima página") e
//	manda tudo de uma vez: `arquivo` é a página 1, `paginas` são as
//	seguintes, em ordem. Um POST, uma nota.
//
//	Os arquivos sobem TODOS antes de a nota existir no banco: se uma
//	página falhar no armazém, a resposta é erro e nada foi criado — sem
//	nota pela metade.
//
//	O ERA READ não participa desta rota (ver o cabeçalho de nf_era.go):
//	depois de gravar, cada página entra na fila de leitura.
func (m *Modulo) receberNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeReceberNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado. Sem ele, receber a nota seria perdê-la.")
		return
	}
	if _, err := m.ordemProntaParaReceber(r, p, id); err != nil {
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	}

	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o que foi enviado.")
		return
	}
	numero := strings.TrimSpace(r.FormValue("numero"))
	if numero == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o número da nota fiscal.")
		return
	}
	valor, err := parseValorNF(r.FormValue("valor"))
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}

	// As páginas, na ordem: a 1 em `arquivo`, as seguintes em `paginas`.
	primeira := r.MultipartForm.File["arquivo"]
	if len(primeira) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escaneie a nota fiscal.")
		return
	}
	cabecalhos := append([]*multipart.FileHeader{primeira[0]}, r.MultipartForm.File["paginas"]...)
	if len(cabecalhos) > PaginasPorNota {
		web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Uma nota pode ter no máximo %d páginas.", PaginasPorNota))
		return
	}
	// RECEBER POR PDF É UMA CAPACIDADE À PARTE (migração 075, 17/09/2026)
	//
	//	`guardarArquivoNF` já sabe guardar qualquer tipo de arquivo — PDF
	//	incluso, é só olhar `tipoDoNome` — então sem esta trava, quem só tem
	//	COMPRAS_NF_RECEBER (a câmera, na obra) ganharia PDF de brinde só
	//	porque o formulário aceita qualquer coisa. A rotina nova é quem
	//	decide isso, concedida por categoria (o dono escolhe quem — Compras
	//	e Administrativo, por exemplo).
	for _, cab := range cabecalhos {
		if !strings.HasSuffix(strings.ToLower(cab.Filename), ".pdf") {
			continue
		}
		pode, err := m.perm.Pode(r.Context(), p, RotinaNFReceberPDF)
		if err != nil {
			m.erro(w, "não consegui conferir sua permissão para receber nota em PDF", err)
			return
		}
		if !pode && !p.Builder() {
			web.Falhar(w, http.StatusForbidden, "Você não tem permissão para receber nota fiscal em PDF — escaneie pela câmera.")
			return
		}
		break
	}
	type paginaLida struct {
		raw []byte
		sha string
	}
	paginas := make([]paginaLida, 0, len(cabecalhos))
	for i, cab := range cabecalhos {
		raw, err := lerCabecalhoMultipart(cab)
		if err != nil {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Página %d: %s.", i+1, err.Error()))
			return
		}
		sha, err := m.guardarArquivoNF(r.Context(), p, raw, fmt.Sprintf("nf-p%d.jpg", i+1))
		if err != nil {
			m.erro(w, fmt.Sprintf("não consegui guardar a página %d da nota", i+1), err)
			return
		}
		paginas = append(paginas, paginaLida{raw: raw, sha: sha})
	}

	// FOTOS DO MATERIAL — PELO MENOS UMA, E SOBEM ANTES DA NOTA EXISTIR
	//
	//	Pedido do dono (15/09/2026): a nota só é aceita com a foto do que
	//	chegou. Por isso as fotos saem da posição de "extra que não trava"
	//	e passam a subir junto com as páginas, antes do INSERT — falhou uma,
	//	não existe nota pela metade.
	fotos := r.MultipartForm.File["fotos_material"]
	if len(fotos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Tire pelo menos uma foto do material recebido.")
		return
	}
	shasFotos := make([]string, 0, len(fotos))
	for i, cab := range fotos {
		sha, err := m.lerEGuardarArquivoNF(r.Context(), p, cab)
		if err != nil {
			web.Falhar(w, http.StatusBadRequest, fmt.Sprintf("Foto do material %d: %s.", i+1, err.Error()))
			return
		}
		shasFotos = append(shasFotos, sha)
	}

	linha := map[string]any{
		"cliente_id":      p.ClienteID,
		"ordem_compra_id": id,
		"numero":          numero,
		"valor":           valor,
		"arquivo_sha256":  paginas[0].sha,
		"status":          "recebida",
		"recebida_por":    p.UserID,
	}
	var criadas []map[string]any
	if err := m.bd.Inserir(r.Context(), "notas_fiscais", []map[string]any{linha}, &criadas); err != nil {
		m.erro(w, "não consegui gravar a nota fiscal", err)
		return
	}
	if len(criadas) == 0 {
		m.erro(w, "gravei a nota fiscal mas o banco não devolveu o id", fmt.Errorf("insert sem retorno"))
		return
	}
	nfID := strCampo(criadas[0]["id"])
	_ = m.hist.Registrar(r.Context(), p, "administrativo", nfID, "receber_nota_fiscal", map[string]historico.Mudanca{
		"ordem_compra_id": {De: nil, Para: id},
		"numero":          {De: nil, Para: numero},
		"paginas":         {De: nil, Para: len(paginas)},
	})

	if len(paginas) > 1 {
		linhas := make([]map[string]any, 0, len(paginas)-1)
		for i := 1; i < len(paginas); i++ {
			linhas = append(linhas, map[string]any{
				"cliente_id":     p.ClienteID,
				"nota_fiscal_id": nfID,
				"pagina":         i + 1,
				"arquivo_sha256": paginas[i].sha,
			})
		}
		if err := m.bd.Inserir(r.Context(), "notas_fiscais_paginas", linhas, nil); err != nil {
			// A nota já existe com a página 1; as outras ficaram no armazém
			// mas sem linha. Melhor avisar do que fingir que deu certo.
			m.erro(w, "gravei a nota mas não consegui registrar as páginas seguintes", err)
			return
		}
	}

	enfileiradas := 0
	for i, pg := range paginas {
		if m.agendarLeituraERA(trabalhoERA{
			clienteID: p.ClienteID, userID: p.UserID, nfID: nfID, pagina: i + 1, sha: pg.sha, raw: pg.raw,
		}) {
			enfileiradas++
		}
	}

	linhasFotos := make([]map[string]any, 0, len(shasFotos))
	for _, sha := range shasFotos {
		linhasFotos = append(linhasFotos, map[string]any{
			"cliente_id":     p.ClienteID,
			"nota_fiscal_id": nfID,
			"arquivo_sha256": sha,
		})
	}
	if err := m.bd.Inserir(r.Context(), "notas_fiscais_fotos_material", linhasFotos, nil); err != nil {
		m.erro(w, "guardei a nota mas não consegui gravar as fotos do material", err)
		return
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"id":             nfID,
		"paginas":        len(paginas),
		"fotos_material": len(shasFotos),
		"era_read":       enfileiradas > 0,
		"numero":         numero,
		"valor":          valor,
	})
}

// lerEGuardarArquivoNF lê um cabeçalho de multipart (foto de material —
// mesmo teto de tamanho, mesmo armazém das páginas) e devolve o sha256 já
// gravado.
func (m *Modulo) lerEGuardarArquivoNF(ctx context.Context, p *seguranca.Principal, cab *multipart.FileHeader) (string, error) {
	conteudo, err := lerCabecalhoMultipart(cab)
	if err != nil {
		return "", err
	}
	return m.guardarArquivoNF(ctx, p, conteudo, cab.Filename)
}

// guardarArquivoNF sobe a foto/PDF da nota — mesma receita de `guardarUma`
// (dedup por sha256, upsert em `arquivos`), sem criar linha em
// `ordens_compra`: aqui quem referencia o arquivo é `notas_fiscais`.
func (m *Modulo) guardarArquivoNF(ctx context.Context, p *seguranca.Principal, conteudo []byte, nome string) (string, error) {
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

func nomeExtensao(nome string) string {
	i := strings.LastIndex(nome, ".")
	if i < 0 {
		return ""
	}
	return nome[i:]
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/recebidas · /entregues · /enviadas
// ---------------------------------------------------------------------------

// RECEBIDAS E ENTREGUES SE LEEM COM QUALQUER UMA DAS DUAS ROTINAS
//
//	Pedido do dono (15/09/2026): o almoxarife (COMPRAS_NF_RECEBER) acompanha
//	o caminho da nota que ele mesmo escaneou até sair do escritório — mas
//	não confirma a entrega física nem o envio (isso é `avancarNF`/
//	`cancelarNF`, que continuam travados em `quemPodeEntregarNF`, sem
//	mudança nenhuma aqui). "Enviadas" fica de fora: é o fim do ciclo, e só
//	interessa a quem entrega.
func (m *Modulo) listarNF(w http.ResponseWriter, r *http.Request, vista string) {
	var p *seguranca.Principal
	if vista == "enviadas" {
		// Antes só quem entrega. Com COMPRAS_NF_ENVIAR_CLIENTE separada
		// (migração 075), quem só tem essa rotina também precisa ver o que
		// já mandou — senão a própria etapa que ele conclui vira invisível
		// pra ele.
		p = m.quemComQualquerRotina(w, r, RotinaNFEntregar, RotinaNFEnviarCliente)
	} else {
		p = m.quemComQualquerRotina(w, r, RotinaNFReceber, RotinaNFEntregar, RotinaNFEnviarCliente)
	}
	if p == nil {
		return
	}
	var linhas []map[string]any
	caminho := filtroDasNF(p.ClienteID, vista) +
		"&select=id,numero,valor,ordem_compra_id,recebida_em,entregue_escritorio_em,enviada_cliente_em" +
		"&limit=" + fmt.Sprint(TetoDaLista)
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		m.erro(w, "não consegui listar as notas fiscais", err)
		return
	}
	m.comDadosDaOrdem(r.Context(), linhas)
	web.Responder(w, http.StatusOK, map[string]any{"notas": ouVazio(linhas)})
}

func (m *Modulo) listarNFRecebidas(w http.ResponseWriter, r *http.Request) { m.listarNF(w, r, "recebidas") }
func (m *Modulo) listarNFEntregues(w http.ResponseWriter, r *http.Request) { m.listarNF(w, r, "entregues") }
func (m *Modulo) listarNFEnviadas(w http.ResponseWriter, r *http.Request)  { m.listarNF(w, r, "enviadas") }

func (m *Modulo) comDadosDaOrdem(ctx context.Context, linhas []map[string]any) {
	for _, l := range linhas {
		oid := strCampo(l["ordem_compra_id"])
		if oid == "" {
			continue
		}
		if o, err := m.contarUm(ctx, "ordens_compra?id=eq."+banco.Escapar(oid)+
			"&select=numero,obra_centro_custo&limit=1"); err == nil {
			l["ordem_numero"] = o["numero"]
			l["obra_centro_custo"] = o["obra_centro_custo"]
		}
	}
}

// ---------------------------------------------------------------------------
// POST /administrativo/nf/{id}/entregar — chegou no escritório
// POST /administrativo/nf/{id}/enviar-cliente — saiu no malote
// POST /administrativo/nf/{id}/cancelar — o fornecedor cancelou a nota
// ---------------------------------------------------------------------------

func (m *Modulo) entregarNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeEntregarNF(w, r)
	if p == nil {
		return
	}
	m.avancarNF(w, r, p, "recebida", "entregue_escritorio", map[string]any{
		"status":                 "entregue_escritorio",
		"entregue_escritorio_em": time.Now().UTC().Format(time.RFC3339),
	}, "entregar_nota_fiscal_escritorio")
}

// enviarNFAoCliente — atrás de COMPRAS_NF_ENVIAR_CLIENTE, não mais de
// COMPRAS_NF_ENTREGAR (migração 075, 17/09/2026). Eram a mesma rotina desde
// a 064; o dono pediu pra "deixar bem definida" a permissão de recebimento
// no escritório — o que só faz sentido separando-a do passo seguinte
// (mandar pro cliente), senão as duas continuam empacotadas juntas.
func (m *Modulo) enviarNFAoCliente(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeEnviarNFAoCliente(w, r)
	if p == nil {
		return
	}
	m.avancarNF(w, r, p, "entregue_escritorio", "enviada_cliente", map[string]any{
		"status":             "enviada_cliente",
		"enviada_cliente_em": time.Now().UTC().Format(time.RFC3339),
	}, "enviar_nota_fiscal_cliente")
}

// avancarNF é o passo comum das duas etapas do escritório — bloqueio
// otimista no FILTRO (`status=eq.<deEsperado>`), mesma disciplina de
// `lerOrdem`: duas confirmações da mesma nota não colidem. `p` já vem
// resolvido: cada chamador tem seu próprio porteiro (rotinas diferentes,
// migração 075), então autenticar aqui de novo escolheria a rotina errada
// pra uma das duas.
func (m *Modulo) avancarNF(w http.ResponseWriter, r *http.Request, p *seguranca.Principal, deEsperado, paraStatus string, campos map[string]any, acao string) {
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	quemConfirma := "entregue_confirmado_por"
	if paraStatus == "enviada_cliente" {
		quemConfirma = "enviada_confirmado_por"
	}
	campos[quemConfirma] = p.UserID

	var tomadas []map[string]any
	if err := m.bd.AtualizarDevolvendo(r.Context(), "notas_fiscais",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&cancelada=eq.false&status=eq."+deEsperado,
		campos, &tomadas); err != nil {
		m.erro(w, "não consegui atualizar esta nota fiscal", err)
		return
	}
	if len(tomadas) == 0 {
		web.Falhar(w, http.StatusConflict, "Esta nota fiscal não está na etapa esperada — a lista pode ter mudado, atualize a página.")
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, acao, map[string]historico.Mudanca{
		"status": {De: deEsperado, Para: paraStatus},
	})
	web.Responder(w, http.StatusOK, map[string]any{"status": paraStatus})
}

func (m *Modulo) cancelarNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeEntregarNF(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	var corpo struct {
		Motivo string `json:"motivo"`
	}
	_ = json.NewDecoder(r.Body).Decode(&corpo)
	motivo := strings.TrimSpace(corpo.Motivo)
	if motivo == "" {
		web.Falhar(w, http.StatusBadRequest, "Explique por que esta nota fiscal está sendo cancelada.")
		return
	}
	if err := m.bd.Atualizar(r.Context(), "notas_fiscais",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID),
		map[string]any{"cancelada": true, "motivo_cancelamento": motivo}); err != nil {
		m.erro(w, "não consegui cancelar esta nota fiscal", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "cancelar_nota_fiscal", map[string]historico.Mudanca{
		"cancelada": {De: false, Para: true},
	})
	web.Responder(w, http.StatusOK, map[string]any{"cancelada": true})
}

// ---------------------------------------------------------------------------
// GET /administrativo/nf/{id}/arquivo
// ---------------------------------------------------------------------------

func (m *Modulo) arquivoDaNF(w http.ResponseWriter, r *http.Request) {
	p := m.quemComQualquerRotina(w, r, RotinaNFReceber, RotinaNFEntregar, RotinaNFEnviarCliente)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	nf, err := m.contarUm(r.Context(), "notas_fiscais?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=numero,arquivo_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei esta nota fiscal", err)
		return
	}
	sha := strCampo(nf["arquivo_sha256"])
	if sha == "" {
		web.Falhar(w, http.StatusNotFound, "Esta nota fiscal não tem arquivo guardado.")
		return
	}
	arq, err := m.contarUm(r.Context(), "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		m.erro(w, "não achei o arquivo", err)
		return
	}
	chave, _ := arq["chave_r2"].(string)
	link, err := m.arm.LinkTemporario(chave, ValidadeDoLink)
	if err != nil {
		m.erro(w, "não consegui montar o endereço do arquivo", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"url": link, "nome": nf["numero"]})
}
