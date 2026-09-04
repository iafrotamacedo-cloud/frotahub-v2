// rev 2 — as rotas de Candidatos e do Kanban de Serviço
//
// Toda ação daqui mexe no ESTADO LOCAL (servicos_candidatos,
// servicos_orcamentos), então a lógica mora em candidatos.go/kanban.go e
// este arquivo só decide quem pode, decodifica o pedido e grava o rastro.
//
// O BOTÃO "MANDAR PARA FILA" MORA AQUI, NÃO EM trilogo/
//
//	Nasceu em trilogo/rotas_servico.go (rev 1) só mudando o responsável no
//	Trílogo — sem tocar no Kanban, a linha só aparecia na próxima varredura
//	agendada. Virou dois jeitos de fazer a MESMA coisa com bookkeeping
//	diferente, exatamente o risco de duplicação/desencontro que o dono pediu
//	pra revisar (04/09/2026). Movida pra cá: kanban.go/MarcarComoServico faz
//	as duas coisas (Trílogo + Kanban) numa chamada só, do mesmo jeito que
//	PromoverCandidato já fazia pro caminho de Candidatos.
package servicos

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// RotinaGerenciar é o código no catálogo de permissões pra toda ação de
// Serviço — fila, cotação, orçamento, Candidatos, Kanban. A migration 049 a
// cadastra (era CONTRATO_SERVICO_GERENCIAR de trilogo/rotas_servico.go antes
// dessa rota mudar de pacote — o texto é o mesmo, só a constante Go mudou de
// endereço).
const RotinaGerenciar = "CONTRATO_SERVICO_GERENCIAR"

type Modulo struct {
	svc  *Servico
	seg  *seguranca.Servico
	perm *permissao.Servico
	bd   *banco.Cliente
	hist *historico.Servico
}

func NovoModulo(svc *Servico, seg *seguranca.Servico, perm *permissao.Servico, bd *banco.Cliente, hist *historico.Servico) *Modulo {
	return &Modulo{svc: svc, seg: seg, perm: perm, bd: bd, hist: hist}
}

func (m *Modulo) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /servicos/candidatos", m.listarCandidatos)
	mux.HandleFunc("POST /servicos/candidatos/{id}/descartar", m.descartarCandidato)
	mux.HandleFunc("POST /servicos/candidatos/{id}/promover", m.promoverCandidato)

	mux.HandleFunc("POST /servicos/chamados/{numero}/fila", m.marcarComoServico)

	mux.HandleFunc("GET /servicos/kanban", m.listarKanban)
	mux.HandleFunc("POST /servicos/kanban/{id}/status", m.mudarStatus)
	mux.HandleFunc("POST /servicos/kanban/{id}/reclassificar", m.reclassificar)

	mux.HandleFunc("GET /servicos/kanban/{id}/cotacoes", m.listarCotacoes)
	mux.HandleFunc("POST /servicos/kanban/{id}/cotacoes", m.criarCotacao)
	mux.HandleFunc("POST /servicos/kanban/{id}/orcamentos", m.criarOrcamento)
	mux.HandleFunc("DELETE /servicos/kanban/{id}/orcamentos", m.excluirOrcamento)

	// O hub de cards e a Planilha de controle (migração 054).
	mux.HandleFunc("GET /servicos/painel", m.lerPainel)
	mux.HandleFunc("GET /servicos/lista", m.listarServicos)
	mux.HandleFunc("GET /servicos/lista.xlsx", m.extrairListaXLSX)
	mux.HandleFunc("GET /servicos/lista.pdf", m.extrairListaPDF)

	// O orçamento em duas etapas: anexar (rascunho local) e lançar (Trílogo).
	mux.HandleFunc("POST /servicos/kanban/{id}/arquivo-orcamento", m.inserirArquivoDeOrcamento)
	mux.HandleFunc("GET /servicos/kanban/{id}/arquivo", m.linkDoArquivo)
	mux.HandleFunc("POST /servicos/kanban/{id}/lancar", m.lancarNoTrilogo)

	// Faturamento por ticket (migração 053).
	mux.HandleFunc("POST /servicos/kanban/{id}/pco", m.preencherPCO)
	mux.HandleFunc("POST /servicos/kanban/{id}/nota-fiscal", m.inserirNotaFiscal)
}

// ---------------------------------------------------------------------------
// peças comuns
// ---------------------------------------------------------------------------

func (m *Modulo) quem(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := m.perm.Exige(r.Context(), p, RotinaGerenciar); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// erro escolhe a frase certa: se a causa foi o Trílogo recusando (mudar
// responsável, criar orçamento), a frase É a dele; senão, uma transição de
// Kanban inválida fala por si (ErrTransicaoInvalida/ErrJaFaturado já vêm com
// a explicação); o resto cai na frase genérica.
func (m *Modulo) erro(w http.ResponseWriter, frase string, err error) {
	if erroTri, ok := trilogo.Recusa(err); ok {
		web.Falhar(w, http.StatusBadGateway, "O Trílogo recusou: "+erroTri.Corpo)
		return
	}
	switch {
	case err == ErrCandidatoNaoEstaPendente, err == ErrJaFaturado,
		err == ErrSemCotacao, err == ErrSemOrcamento,
		err == ErrSemPCO, err == ErrSemArquivoDeOrcamento:
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	case err == ErrArquivoVazio:
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, transicao := err.(*ErrTransicaoInvalida); transicao {
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	}
	if _, duplicado := err.(*ErrOrcamentoJaExiste); duplicado {
		web.Falhar(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, trilogo.ErrResponsavelNaoExiste) {
		web.Falhar(w, http.StatusUnprocessableEntity, "responsável não existe")
		return
	}
	if errors.Is(err, ErrFornecedorNaoExiste) {
		web.Falhar(w, http.StatusUnprocessableEntity, "fornecedor não existe")
		return
	}
	log.Printf("servicos: %s: %v", frase, err)
	web.Falhar(w, http.StatusInternalServerError,
		"Não consegui completar: "+frase+". Tente de novo em instantes.")
}

func contaValida(w http.ResponseWriter, bruta string) (string, bool) {
	conta := strings.ToLower(strings.TrimSpace(bruta))
	if conta != "instalacoes" && conta != "civil" {
		web.Falhar(w, http.StatusBadRequest, `A conta tem que ser "instalacoes" ou "civil".`)
		return "", false
	}
	return conta, true
}

func numeroDoCaminho(w http.ResponseWriter, r *http.Request, nome string) (int, bool) {
	n, err := strconv.Atoi(r.PathValue(nome))
	if err != nil || n <= 0 {
		web.Falhar(w, http.StatusBadRequest, "Número inválido na URL.")
		return 0, false
	}
	return n, true
}

func decodificar(w http.ResponseWriter, r *http.Request, destino any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(destino); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi o pedido.")
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Candidatos
// ---------------------------------------------------------------------------

// GET /servicos/candidatos
func (m *Modulo) listarCandidatos(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	candidatos, err := m.svc.ListarCandidatos(r.Context(), p.ClienteID)
	if err != nil {
		m.erro(w, "listar os candidatos", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"candidatos": candidatos})
}

// POST /servicos/candidatos/{id}/descartar
//
// "Não é Serviço" — definitivo. Não pede motivo: o card já mostra o motivo
// do GROQ (por que ELE achou que era); o motivo de descartar é do usuário, e
// fica implícito no clique — pedir para digitar de novo só atrasaria a
// decisão mais comum (a maioria dos candidatos vira "não, é conserto mesmo").
func (m *Modulo) descartarCandidato(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	if err := m.svc.DescartarCandidato(r.Context(), p.ClienteID, id, p.UserID); err != nil {
		m.erro(w, "descartar o candidato", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "descartar_candidato", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

type pedidoDePromocao struct {
	Conta string `json:"conta"`
}

// POST /servicos/candidatos/{id}/promover   {"conta":"instalacoes"}
//
// O atalho: aprova o palpite do Groq de uma vez — muda o responsável no
// Trílogo e já cria o cartão no Kanban, sem esperar a próxima rodada do
// gatilho automático.
func (m *Modulo) promoverCandidato(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var pedido pedidoDePromocao
	if !decodificar(w, r, &pedido) {
		return
	}
	conta, ok := contaValida(w, pedido.Conta)
	if !ok {
		return
	}
	rotulo, err := m.svc.PromoverCandidato(r.Context(), p.ClienteID, id, conta, p.UserID)
	if err != nil {
		if rotulo != "" {
			// O Trílogo já mudou; só a nossa tabela falhou (ver kanban.go,
			// PromoverCandidato). Isto ACONTECEU — não é 500 de "tente de
			// novo", é aviso, do mesmo jeito que historico.Aviso.
			web.Responder(w, http.StatusOK, map[string]any{
				"ok": true, "responsavel": rotulo,
				"aviso": "Mudei o responsável no Trílogo, mas não consegui gravar no Kanban local. A próxima varredura automática corrige sozinha.",
			})
			return
		}
		m.erro(w, "promover o candidato", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "promover_candidato",
		map[string]historico.Mudanca{"responsavel": {De: nil, Para: rotulo}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "responsavel": rotulo})
}

// ---------------------------------------------------------------------------
// Entrada manual — o botão "mandar para fila de serviços"
// ---------------------------------------------------------------------------

// POST /servicos/chamados/{numero}/fila
//
// Contrato -> Serviço, pelo clique. A troca reversa (Serviço -> Contrato) é
// reclassificar, mais abaixo. Não pede "conta" no corpo — kanban.go/
// MarcarComoServico já lê de chamados.conta, então não existe como escolher
// a conta errada.
func (m *Modulo) marcarComoServico(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	numero, ok := numeroDoCaminho(w, r, "numero")
	if !ok {
		return
	}
	rotulo, jaEstava, err := m.svc.MarcarComoServico(r.Context(), p.ClienteID, numero)
	if err != nil {
		m.erro(w, "mandar para a fila de serviços", err)
		return
	}
	if jaEstava {
		web.Responder(w, http.StatusOK, map[string]any{
			"ok": true, "responsavel": rotulo, "ja_estava_na_fila": true,
		})
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", strconv.Itoa(numero), "mandar_para_fila_servico",
		map[string]historico.Mudanca{"responsavel": {De: nil, Para: rotulo}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "responsavel": rotulo})
}

// ---------------------------------------------------------------------------
// Kanban
// ---------------------------------------------------------------------------

// GET /servicos/kanban
func (m *Modulo) listarKanban(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	itens, err := m.svc.ListarKanban(r.Context(), p.ClienteID)
	if err != nil {
		m.erro(w, "listar o Kanban", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"itens": itens})
}

type pedidoDeStatus struct {
	Status string `json:"status"`
}

// POST /servicos/kanban/{id}/status   {"status":"orcamento_feito"}
func (m *Modulo) mudarStatus(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var pedido pedidoDeStatus
	if !decodificar(w, r, &pedido) {
		return
	}
	status := strings.TrimSpace(pedido.Status)
	if status == "" {
		web.Falhar(w, http.StatusBadRequest, "Diga para qual status mover.")
		return
	}
	if err := m.svc.MudarStatus(r.Context(), p.ClienteID, id, status); err != nil {
		m.erro(w, "mudar o status", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "mudar_status_kanban",
		map[string]historico.Mudanca{"status": {De: nil, Para: status}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

type pedidoDeReclassificacao struct {
	Motivo string `json:"motivo"`
}

// POST /servicos/kanban/{id}/reclassificar   {"motivo":"..."}
//
// A troca de fila que tem que existir sempre, nos dois sentidos: esta é
// Serviço -> Contrato. A outra (Contrato -> Serviço) é marcarComoServico,
// mais acima.
func (m *Modulo) reclassificar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var pedido pedidoDeReclassificacao
	if !decodificar(w, r, &pedido) {
		return
	}
	if err := m.svc.Reclassificar(r.Context(), p.ClienteID, id, p.UserID, pedido.Motivo); err != nil {
		m.erro(w, "reclassificar para o contrato", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "reclassificar_para_contrato",
		map[string]historico.Mudanca{"motivo": {De: nil, Para: pedido.Motivo}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// Cotação e Orçamento — dentro do card
// ---------------------------------------------------------------------------
//
// Nenhuma destas rotas pede "conta" — kanban.go/sessaoDoItem já lê de
// item.Conta, então não existe como escolher a conta errada nem precisar
// saber o número do ticket: tudo é endereçado pelo id do card.

// GET /servicos/kanban/{id}/cotacoes
func (m *Modulo) listarCotacoes(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	cotacoes, err := m.svc.ListarCotacoes(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "ler as cotações", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"cotacoes": cotacoes})
}

type pedidoDeCotacao struct {
	Descricao string `json:"descricao"`
}

// POST /servicos/kanban/{id}/cotacoes   {"descricao":"..."}
func (m *Modulo) criarCotacao(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var pedido pedidoDeCotacao
	if !decodificar(w, r, &pedido) {
		return
	}
	descricao := strings.TrimSpace(pedido.Descricao)
	if descricao == "" {
		web.Falhar(w, http.StatusBadRequest, "Descreva o que está sendo orçado.")
		return
	}
	cotacaoID, err := m.svc.CriarCotacao(r.Context(), p.ClienteID, id, descricao)
	if err != nil {
		m.erro(w, "criar a cotação", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "criar_cotacao",
		map[string]historico.Mudanca{"cotacao_id": {De: nil, Para: cotacaoID}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "cotacao_id": cotacaoID})
}

// itemDoPedido é como mão de obra / material chegam no corpo do pedido —
// mesmos três campos de trilogo.ItemOrcamento, com nome de JSON.
type itemDoPedido struct {
	Descricao string  `json:"descricao"`
	Valor     float64 `json:"valor"`
	Qtd       float64 `json:"qtd"`
}

func paraItens(brutos []itemDoPedido) []trilogo.ItemOrcamento {
	itens := make([]trilogo.ItemOrcamento, 0, len(brutos))
	for _, b := range brutos {
		itens = append(itens, trilogo.ItemOrcamento{Descricao: b.Descricao, Valor: b.Valor, Qtd: b.Qtd})
	}
	return itens
}

// TamanhoMaximoDoOrcamento — mesmo teto do upload de documento de
// funcionário (20 MB): PDF de orçamento com fotos anexadas não é pequeno,
// mas também não precisa de mais que isso.
const TamanhoMaximoDoOrcamento = 20 << 20

// POST /servicos/kanban/{id}/orcamentos — multipart/form-data
//
//	mao_de_obra   JSON: [{"descricao":"...","valor":0.01,"qtd":1}, ...]
//	materiais     JSON, mesmo formato
//	anexos        um ou mais arquivos (o PDF do orçamento)
//
// Mão de obra e material entram no MESMO orçamento — nunca dois orçamentos
// separados (ver trilogo/servico.go, MontarOrcamentoServico).
func (m *Modulo) criarOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	if err := r.ParseMultipartForm(TamanhoMaximoDoOrcamento); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o que foi enviado.")
		return
	}
	var maoDeObraBruta, materiaisBruta []itemDoPedido
	if s := r.FormValue("mao_de_obra"); s != "" {
		if err := json.Unmarshal([]byte(s), &maoDeObraBruta); err != nil {
			web.Falhar(w, http.StatusBadRequest, "mao_de_obra não é uma lista válida.")
			return
		}
	}
	if s := r.FormValue("materiais"); s != "" {
		if err := json.Unmarshal([]byte(s), &materiaisBruta); err != nil {
			web.Falhar(w, http.StatusBadRequest, "materiais não é uma lista válida.")
			return
		}
	}
	if len(maoDeObraBruta) == 0 && len(materiaisBruta) == 0 {
		web.Falhar(w, http.StatusBadRequest, "O orçamento precisa de ao menos um item, de mão de obra ou material.")
		return
	}

	ctx := r.Context()
	// A sessão daqui é só pra subir os anexos; CriarOrcamento abre a dela
	// própria pra gravar — duas entradas no Trílogo por uma ação só, mesmo
	// custo que as rotas antigas já pagavam por chamada.
	sessao, _, err := m.svc.sessaoDoItem(ctx, p.ClienteID, id)
	if err != nil {
		m.erro(w, "entrar no Trílogo", err)
		return
	}

	var subidos []trilogo.Subido
	for _, cab := range r.MultipartForm.File["anexos"] {
		arq, err := cab.Open()
		if err != nil {
			m.erro(w, "ler o anexo "+cab.Filename, err)
			return
		}
		conteudo, err := io.ReadAll(arq)
		arq.Close()
		if err != nil {
			m.erro(w, "ler o anexo "+cab.Filename, err)
			return
		}
		subido, err := sessao.SubirArquivo(ctx, cab.Filename, conteudo)
		if err != nil {
			m.erro(w, "subir o anexo "+cab.Filename, err)
			return
		}
		subidos = append(subidos, subido)
	}

	orcamentoID, valor, err := m.svc.CriarOrcamento(ctx, p.ClienteID, id, paraItens(maoDeObraBruta), paraItens(materiaisBruta), subidos)
	if err != nil {
		m.erro(w, "criar o orçamento", err)
		return
	}

	_ = m.hist.Registrar(ctx, p, "servicos", id, "criar_orcamento",
		map[string]historico.Mudanca{"orcamento_id": {De: nil, Para: orcamentoID}})

	web.Responder(w, http.StatusOK, map[string]any{"ok": true, "orcamento_id": orcamentoID, "valor": valor})
}

// DELETE /servicos/kanban/{id}/orcamentos
func (m *Modulo) excluirOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	if err := m.svc.ExcluirOrcamento(r.Context(), p.ClienteID, id); err != nil {
		m.erro(w, "excluir o orçamento", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "excluir_orcamento", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// O hub de 6 cards e a Planilha de controle
// ---------------------------------------------------------------------------

// GET /servicos/painel
func (m *Modulo) lerPainel(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	painel, err := m.svc.LerPainel(r.Context(), p.ClienteID)
	if err != nil {
		m.erro(w, "ler o painel", err)
		return
	}
	web.Responder(w, http.StatusOK, painel)
}

// filtroDaQuery lê os parâmetros comuns às 9 telas de lista e à exportação —
// um lugar só, pra tela e PDF/Excel nunca discordarem do que foi pedido.
func filtroDaQuery(q url.Values) FiltroLista {
	f := FiltroLista{
		Status:   strings.TrimSpace(q.Get("status")),
		Conta:    strings.TrimSpace(q.Get("conta")),
		Busca:    strings.TrimSpace(q.Get("busca")),
		PCO:      strings.TrimSpace(q.Get("pco")),
		SoAtivos: q.Get("todos") != "1",
	}
	if n, err := strconv.Atoi(q.Get("pagina")); err == nil {
		f.Pagina = n
	}
	if n, err := strconv.Atoi(q.Get("por_pagina")); err == nil {
		f.PorPagina = n
	}
	return f
}

// GET /servicos/lista?status=...&conta=...&busca=...&pagina=...&por_pagina=...
//
// A resposta segue a MESMA forma de qualquer tela paginada do sistema
// (linhas/total/pagina/por_pagina/paginas — ver orcamentos/tipos.ts, Pagina<T>
// no front) de propósito: o componente de paginação já pronto (Paginacao,
// telas/orcamentos/Arquivos.tsx) é reaproveitado sem adaptador nenhum.
func (m *Modulo) listarServicos(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	f := filtroDaQuery(r.URL.Query())
	itens, total, err := m.svc.Lista(r.Context(), p.ClienteID, f)
	if err != nil {
		m.erro(w, "listar os serviços", err)
		return
	}
	pagina := f.Pagina
	if pagina < 1 {
		pagina = 1
	}
	porPagina := f.PorPagina
	if porPagina < 1 || porPagina > porPaginaMaximaDaLista {
		porPagina = porPaginaPadraoDaLista
	}
	paginas := (total + porPagina - 1) / porPagina
	if paginas < 1 {
		paginas = 1
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"linhas": itens, "total": total, "pagina": pagina,
		"por_pagina": porPagina, "paginas": paginas,
	})
}

// ---------------------------------------------------------------------------
// O orçamento em duas etapas — anexar (rascunho) e lançar (Trílogo)
// ---------------------------------------------------------------------------

// POST /servicos/kanban/{id}/arquivo-orcamento — multipart/form-data: arquivo=<PDF>
//
// Pendentes -> Feitos. Só guarda no R2, não fala com o Trílogo ainda — ver
// documentos.go, InserirArquivoDeOrcamento.
func (m *Modulo) inserirArquivoDeOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	if err := r.ParseMultipartForm(TamanhoMaximoDeArquivo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o que foi enviado.")
		return
	}
	arquivo, cabecalho, err := r.FormFile("arquivo")
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Envie o arquivo do orçamento no campo \"arquivo\".")
		return
	}
	defer arquivo.Close()
	conteudo, err := io.ReadAll(io.LimitReader(arquivo, TamanhoMaximoDeArquivo+1))
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo.")
		return
	}

	if err := m.svc.InserirArquivoDeOrcamento(r.Context(), p.ClienteID, id, cabecalho.Filename, conteudo); err != nil {
		m.erro(w, "anexar o orçamento", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "anexar_orcamento",
		map[string]historico.Mudanca{"arquivo": {De: nil, Para: cabecalho.Filename}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /servicos/kanban/{id}/arquivo?tipo=orcamento|nf
//
// Devolve um link temporário assinado (armazem.LinkTemporario) — o arquivo
// nunca passa pelo motor, mesmo padrão de orcamentos/documentos.go.
func (m *Modulo) linkDoArquivo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	tipo := r.URL.Query().Get("tipo")
	if tipo != "orcamento" && tipo != "nf" {
		web.Falhar(w, http.StatusBadRequest, `O tipo tem que ser "orcamento" ou "nf".`)
		return
	}
	chaveR2, err := m.svc.ArquivoDoItem(r.Context(), p.ClienteID, id, tipo)
	if err != nil {
		m.erro(w, "achar o arquivo", err)
		return
	}
	endereco, err := m.svc.arm.LinkTemporario(chaveR2, ValidadeDoLinkDeArquivo)
	if err != nil {
		m.erro(w, "gerar o link do arquivo", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"url": endereco})
}

type pedidoDeLancamento struct {
	Descricao string         `json:"descricao"`
	Itens     []itemDoPedido `json:"itens"`
}

// POST /servicos/kanban/{id}/lancar   {"descricao":"...","itens":[{descricao,valor,qtd}]}
//
// Feitos -> Lançados. Cria a cotação (se faltar) e o orçamento no Trílogo de
// verdade, subindo o PDF já anexado — ver lancar.go, LancarNoTrilogo. SEMPRE
// mão de obra (decisão do dono, 04/09/2026) — não existe seção de materiais.
func (m *Modulo) lancarNoTrilogo(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var pedido pedidoDeLancamento
	if !decodificar(w, r, &pedido) {
		return
	}
	descricao := strings.TrimSpace(pedido.Descricao)
	if descricao == "" {
		web.Falhar(w, http.StatusBadRequest, "Descreva o que está sendo orçado.")
		return
	}
	if len(pedido.Itens) == 0 {
		web.Falhar(w, http.StatusBadRequest, "O lançamento precisa de ao menos um item de mão de obra.")
		return
	}

	cotacaoID, orcamentoID, valor, err := m.svc.LancarNoTrilogo(r.Context(), p.ClienteID, id, descricao, paraItens(pedido.Itens))
	if err != nil {
		m.erro(w, "lançar no Trílogo", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "lancar_no_trilogo",
		map[string]historico.Mudanca{"orcamento_id": {De: nil, Para: orcamentoID}})
	web.Responder(w, http.StatusOK, map[string]any{
		"ok": true, "cotacao_id": cotacaoID, "orcamento_id": orcamentoID, "valor": valor,
	})
}

// ---------------------------------------------------------------------------
// Faturamento por ticket — PCO e nota fiscal
// ---------------------------------------------------------------------------

type pedidoDePCO struct {
	PCO string `json:"pco"`
}

// POST /servicos/kanban/{id}/pco   {"pco":"..."}
//
// Não muda status — só separa "Aguardando PCO" de "A faturar" na tela (ver
// kanban.go, PreencherPCO). Pode ser reescrito.
func (m *Modulo) preencherPCO(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	var pedido pedidoDePCO
	if !decodificar(w, r, &pedido) {
		return
	}
	pco := strings.TrimSpace(pedido.PCO)
	if pco == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o número do PCO.")
		return
	}
	if err := m.svc.PreencherPCO(r.Context(), p.ClienteID, id, p.UserID, pco); err != nil {
		m.erro(w, "preencher o PCO", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "preencher_pco",
		map[string]historico.Mudanca{"pco_numero": {De: nil, Para: pco}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /servicos/kanban/{id}/nota-fiscal — multipart/form-data: arquivo=<PDF>, numero=<opcional>
//
// A -> faturar -> Faturado. A ÚNICA transição de status real do cartão de
// Faturamento — ver documentos.go, InserirArquivoDeNF.
func (m *Modulo) inserirNotaFiscal(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id := r.PathValue("id")
	if err := r.ParseMultipartForm(TamanhoMaximoDeArquivo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o que foi enviado.")
		return
	}
	arquivo, cabecalho, err := r.FormFile("arquivo")
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Envie o arquivo da nota fiscal no campo \"arquivo\".")
		return
	}
	defer arquivo.Close()
	conteudo, err := io.ReadAll(io.LimitReader(arquivo, TamanhoMaximoDeArquivo+1))
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo.")
		return
	}
	numero := strings.TrimSpace(r.FormValue("numero"))

	if err := m.svc.InserirArquivoDeNF(r.Context(), p.ClienteID, id, numero, cabecalho.Filename, conteudo); err != nil {
		m.erro(w, "anexar a nota fiscal", err)
		return
	}
	_ = m.hist.Registrar(r.Context(), p, "servicos", id, "anexar_nota_fiscal",
		map[string]historico.Mudanca{"arquivo": {De: nil, Para: cabecalho.Filename}})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}
