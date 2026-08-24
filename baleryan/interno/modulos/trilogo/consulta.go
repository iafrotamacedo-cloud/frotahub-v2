// rev 1 — a tela: consultar o que o robô já trouxe
//
// O robô (robo.go) ESCREVE. Este arquivo só LÊ. São dois trabalhos com ritmos
// diferentes — um roda de madrugada e demora minutos, o outro responde a um
// clique e tem que ser instantâneo — e por isso moram em arquivos separados,
// ainda que no mesmo módulo.
//
// TUDO GIRA EM TORNO DO TICKET
//
//	Para quem opera, o chamado não tem "id": tem NÚMERO. É o número que a pessoa
//	tem no papel, no WhatsApp e na nota fiscal. Por isso a ficha abre por
//	/trilogo/chamados/130328 e não por um uuid que ninguém sabe de cor.
//
// DUAS ROTAS, NÃO SEIS
//
//	A lista devolve tudo que a tabela precisa, já com nome de loja e soma de
//	custos (a visão chamados_lista resolve isso no banco). A ficha devolve o
//	chamado inteiro — cabeçalho, linha do tempo, custos e anexos — numa resposta
//	só. Abrir um chamado é UMA viagem, não cinco: a tela abre de uma vez, e não
//	em pedaços que aparecem um a um (P-29).
package trilogo

import (
	"context"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/relatorio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// RotinaDados é o código no catálogo de permissões. A migration 009 o cadastra;
// antes dela, `posso()` responde não para todo mundo menos o builder.
const RotinaDados = "CONTRATO_TRILOGO_DADOS"

// PorPagina: as três opções da tela, e nada além delas.
//
// O tamanho da página vem do navegador, e o que vem do navegador não se obedece
// sem conferir. Sem esta lista, um `?por_pagina=999999` puxaria a tabela inteira
// para dentro do motor — não por malícia, basta alguém brincar com a URL.
var PorPagina = []int{100, 250, 500}

const porPaginaPadrao = 100

// ValidadeDoLink: quanto tempo o endereço de uma foto continua abrindo.
//
// Curto de propósito. O link existe para a tela mostrar a imagem agora; se
// vazar, vazou uma janela de cinco minutos, e não o arquivo para sempre.
const ValidadeDoLink = 5 * time.Minute

// TetoDaExtracao — o máximo de linhas que sai num arquivo.
//
// Não é medo do volume: são 1.377 chamados hoje. É que um PDF de 50 mil linhas
// são 1.500 folhas que ninguém vai ler, geradas com o motor parado esperando.
// Quando o teto morde, o documento DIZ que mordeu — corte silencioso é pior que
// corte, porque quem lê acha que está vendo tudo.
const TetoDaExtracao = 5000

type Consulta struct {
	bd   *banco.Cliente
	seg  *seguranca.Servico
	perm *permissao.Servico
	arm  *armazem.Cliente
}

func NovaConsulta(bd *banco.Cliente, seg *seguranca.Servico, perm *permissao.Servico, arm *armazem.Cliente) *Consulta {
	return &Consulta{bd: bd, seg: seg, perm: perm, arm: arm}
}

func (c *Consulta) Montar(mux *http.ServeMux) {
	mux.HandleFunc("GET /trilogo/filtros", c.filtros)
	mux.HandleFunc("GET /trilogo/chamados", c.lista)
	mux.HandleFunc("GET /trilogo/chamados/{numero}", c.ficha)
	mux.HandleFunc("GET /trilogo/chamados.xlsx", c.extrairPlanilha)
	mux.HandleFunc("GET /trilogo/chamados.pdf", c.extrairPDF)
}

// quem confere login e permissão, nessa ordem, e devolve nil se barrou (já tendo
// respondido). O robô não passa por aqui: ele não está na matriz.
func (c *Consulta) quem(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := c.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if err := c.perm.Exige(r.Context(), p, RotinaDados); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// ---------------------------------------------------------------------------
// GET /trilogo/filtros — o que existe para escolher
// ---------------------------------------------------------------------------
//
// As opções dos seletores saem do PRÓPRIO DADO, não de uma lista escrita no
// front. Quando o Trílogo inventar um status novo, ele aparece na tela sozinho
// (CORE-10: crescer é acrescentar linha, não reescrever código).
func (c *Consulta) filtros(w http.ResponseWriter, r *http.Request) {
	p := c.quem(w, r)
	if p == nil {
		return
	}
	cli := banco.Escapar(p.ClienteID)

	var lojas []map[string]any
	if err := c.bd.Buscar(r.Context(),
		"unidades?cliente_id=eq."+cli+"&no_escopo=is.true&select=id,nome,id_trilogo&order=nome",
		&lojas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar as lojas.")
		return
	}

	var chamados []struct {
		Status     string `json:"status"`
		Prioridade string `json:"prioridade"`
		Conta      string `json:"conta"`
	}
	if err := c.bd.Buscar(r.Context(),
		"chamados?cliente_id=eq."+cli+"&select=status,prioridade,conta", &chamados); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os filtros.")
		return
	}
	status, prioridades, contas := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, l := range chamados {
		if l.Status != "" {
			status[l.Status] = true
		}
		if l.Prioridade != "" {
			prioridades[l.Prioridade] = true
		}
		if l.Conta != "" {
			contas[l.Conta] = true
		}
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"lojas":       lojas,
		"status":      ordenado(status),
		"prioridades": ordenado(prioridades),
		"contas":      ordenado(contas),
		"por_pagina":  PorPagina,
	})
}

// ---------------------------------------------------------------------------
// GET /trilogo/chamados — a lista
// ---------------------------------------------------------------------------

type Pagina struct {
	Linhas    []map[string]any `json:"linhas"`
	Total     int              `json:"total"`
	Pagina    int              `json:"pagina"`
	Paginas   int              `json:"paginas"`
	PorPagina int              `json:"por_pagina"`
}

func (c *Consulta) lista(w http.ResponseWriter, r *http.Request) {
	p := c.quem(w, r)
	if p == nil {
		return
	}

	q := r.URL.Query()
	porPagina := umDosPermitidos(q.Get("por_pagina"))
	pagina := umNumero(q.Get("pagina"), 1)
	if pagina < 1 {
		pagina = 1
	}

	filtro, err := c.montarFiltro(p.ClienteID, q)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}

	saida, err := c.puxarPagina(r.Context(), filtro, pagina, porPagina)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os chamados.")
		return
	}

	// Pediram uma página que não existe mais — filtro apertou, alguém marcou o
	// endereço, a lista encolheu. Em vez de uma tabela vazia e sem explicação,
	// a última página existente. Custa uma segunda consulta num caso raro e
	// evita a tela em branco que parece defeito (P-29).
	if saida.Total > 0 && len(saida.Linhas) == 0 && pagina > saida.Paginas {
		if saida, err = c.puxarPagina(r.Context(), filtro, saida.Paginas, porPagina); err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os chamados.")
			return
		}
	}
	web.Responder(w, http.StatusOK, saida)
}

func (c *Consulta) puxarPagina(ctx context.Context, filtro string, pagina, porPagina int) (*Pagina, error) {
	linhas := []map[string]any{}
	caminho := fmt.Sprintf("chamados_lista?%s&select=*&order=criado_em.desc,numero.desc&limit=%d&offset=%d",
		filtro, porPagina, (pagina-1)*porPagina)

	total, err := c.bd.BuscarContando(ctx, caminho, &linhas)
	if err != nil {
		return nil, err
	}
	paginas := int(math.Ceil(float64(total) / float64(porPagina)))
	if paginas < 1 {
		paginas = 1
	}
	return &Pagina{Linhas: linhas, Total: total, Pagina: pagina, Paginas: paginas, PorPagina: porPagina}, nil
}

// montarFiltro traduz o que veio da tela para a sintaxe do PostgREST.
//
// O cliente entra SEMPRE, e primeiro. Não existe caminho neste arquivo que leia
// chamado sem dizer de quem ele é (CORE-11).
func (c *Consulta) montarFiltro(clienteID string, q map[string][]string) (string, error) {
	pega := func(k string) string {
		if v, ok := q[k]; ok && len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
		return ""
	}

	partes := []string{"cliente_id=eq." + banco.Escapar(clienteID)}

	if t := somenteDigitos(pega("ticket")); t != "" {
		partes = append(partes, "numero=eq."+t)
	}
	if v := pega("loja"); v != "" {
		partes = append(partes, "unidade_id=eq."+banco.Escapar(v))
	}
	if v := pega("status"); v != "" {
		partes = append(partes, "status=eq."+banco.Escapar(v))
	}
	if v := pega("conta"); v != "" {
		partes = append(partes, "conta=eq."+banco.Escapar(v))
	}
	if v := pega("prioridade"); v != "" {
		// "sem" é uma escolha de verdade: 39 chamados chegaram do Trílogo sem
		// prioridade nenhuma, e quem opera precisa conseguir achá-los.
		if strings.EqualFold(v, "sem") {
			partes = append(partes, "prioridade=eq.")
		} else {
			partes = append(partes, "prioridade=eq."+banco.Escapar(v))
		}
	}

	if v := pega("de"); v != "" {
		d, err := umaData(v, false)
		if err != nil {
			return "", fmt.Errorf("a data inicial não é uma data válida")
		}
		partes = append(partes, "criado_em=gte."+banco.Escapar(d))
	}
	if v := pega("ate"); v != "" {
		d, err := umaData(v, true)
		if err != nil {
			return "", fmt.Errorf("a data final não é uma data válida")
		}
		partes = append(partes, "criado_em=lte."+banco.Escapar(d))
	}
	return strings.Join(partes, "&"), nil
}

// ---------------------------------------------------------------------------
// GET /trilogo/chamados/{numero} — a ficha
// ---------------------------------------------------------------------------

func (c *Consulta) ficha(w http.ResponseWriter, r *http.Request) {
	p := c.quem(w, r)
	if p == nil {
		return
	}
	numero := somenteDigitos(r.PathValue("numero"))
	if numero == "" {
		web.Falhar(w, http.StatusBadRequest, "Informe o número do ticket.")
		return
	}
	cli := banco.Escapar(p.ClienteID)
	ctx := r.Context()

	cabecalhos := []map[string]any{}
	if err := c.bd.Buscar(ctx,
		"chamados_lista?cliente_id=eq."+cli+"&numero=eq."+numero+"&select=*&limit=1",
		&cabecalhos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar o chamado.")
		return
	}
	if len(cabecalhos) == 0 {
		web.Falhar(w, http.StatusNotFound, "Chamado "+numero+" não está na nossa base.")
		return
	}
	chamado := cabecalhos[0]
	id, _ := chamado["id"].(string)
	idEsc := banco.Escapar(id)

	eventos := []map[string]any{}
	if err := c.bd.Buscar(ctx,
		"chamado_eventos?chamado_id=eq."+idEsc+
			"&select=quando,tipo,tipo_codigo,status,autor,texto&order=quando.asc",
		&eventos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar a linha do tempo.")
		return
	}
	// O Trílogo devolve o texto do evento com HTML dentro ("<b>FROTA</b>",
	// "<br/>"). Limpo aqui, uma vez, e não em cada tela que for mostrar isso —
	// senão a primeira que esquecer mostra as tags para o usuário (CORE-06).
	for _, e := range eventos {
		if t, ok := e["texto"].(string); ok {
			e["texto"] = SemHTML(t)
		}
	}

	custos := []map[string]any{}
	if err := c.bd.Buscar(ctx,
		"chamado_custos?chamado_id=eq."+idEsc+"&select=*&order=criado_em.asc", &custos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os custos.")
		return
	}

	anexos := []map[string]any{}
	if err := c.bd.Buscar(ctx,
		"chamado_anexos?chamado_id=eq."+idEsc+
			"&select=id,colecao,nome,extensao,tamanho,tipo,autor,quando,copiar,custo_id,url_origem,arquivo_sha256,arquivo:arquivos(chave_r2)"+
			"&order=quando.asc", &anexos); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os anexos.")
		return
	}
	c.enderecar(anexos)

	web.Responder(w, http.StatusOK, map[string]any{
		"chamado": chamado,
		"eventos": eventos,
		"custos":  custos,
		"anexos":  anexos,
	})
}

// enderecar decide, anexo por anexo, de onde a tela vai buscar o arquivo.
//
// Copiado para o armazém  → endereço assinado, que vence em minutos.
// Não copiado (vídeo)     → o endereço no Trílogo, como está lá.
//
// A tela não precisa saber dessa diferença: ela recebe `link` e `onde`, e mostra.
// A regra de quem foi copiado é do robô, e não se repete aqui.
func (c *Consulta) enderecar(anexos []map[string]any) {
	for _, a := range anexos {
		a["onde"] = "trilogo"
		a["link"], _ = a["url_origem"].(string)

		chave := chaveDoAnexo(a)
		if chave == "" || !c.arm.Ligado() {
			continue
		}
		url, err := c.arm.LinkTemporario(chave, ValidadeDoLink)
		if err != nil {
			// Falhou assinar? A tela ainda mostra o arquivo, pelo Trílogo. Um
			// anexo sem link é uma foto que não abre; melhor o caminho antigo do
			// que um buraco na tela.
			continue
		}
		a["onde"] = "armazem"
		a["link"] = url
		a["vence_em"] = int(ValidadeDoLink.Seconds())
	}
	// url_origem só interessava para montar o link; não vai para o navegador. É o
	// endereço público do sistema antigo, e ele não precisa circular mais do que
	// já circula.
	for _, a := range anexos {
		if a["onde"] == "armazem" {
			delete(a, "url_origem")
		}
	}
}

func chaveDoAnexo(a map[string]any) string {
	arq, ok := a["arquivo"].(map[string]any)
	if !ok {
		return ""
	}
	chave, _ := arq["chave_r2"].(string)
	return chave
}

// ---------------------------------------------------------------------------
// Peças pequenas
// ---------------------------------------------------------------------------

var tags = regexp.MustCompile(`<[^>]*>`)

// SemHTML tira as marcações e devolve texto limpo, preservando as quebras que a
// marcação representava — um <br/> vira espaço, e não some colando duas palavras.
func SemHTML(s string) string {
	if s == "" || !strings.ContainsAny(s, "<&") {
		return s
	}
	s = strings.NewReplacer("<br>", " ", "<br/>", " ", "<br />", " ", "</p>", " ").Replace(s)
	s = tags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func somenteDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func umNumero(s string, padrao int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return padrao
}

func umDosPermitidos(s string) int {
	n := umNumero(s, porPaginaPadrao)
	for _, v := range PorPagina {
		if n == v {
			return n
		}
	}
	return porPaginaPadrao
}

var fusoDaCasa = sync.OnceValue(carregarFuso)

// umaData aceita 2026-07-01 e 01/07/2026, e devolve o instante no fuso da casa.
//
// O fuso não é firula: `criado_em` é guardado em UTC, e um chamado aberto às 22h
// de Fortaleza está gravado no DIA SEGUINTE em UTC. Filtrar sem fuso jogaria
// esse chamado para fora do dia em que ele aconteceu.
func umaData(s string, fimDoDia bool) (string, error) {
	s = strings.TrimSpace(s)
	var d time.Time
	var err error
	for _, formato := range []string{"2006-01-02", "02/01/2006"} {
		if d, err = time.ParseInLocation(formato, s, fusoDaCasa()); err == nil {
			break
		}
	}
	if err != nil {
		return "", err
	}
	if fimDoDia {
		d = d.Add(24*time.Hour - time.Nanosecond)
	}
	return d.Format(time.RFC3339), nil
}

func ordenado(m map[string]bool) []string {
	saida := make([]string, 0, len(m))
	for k := range m {
		saida = append(saida, k)
	}
	for i := 1; i < len(saida); i++ {
		for j := i; j > 0 && saida[j] < saida[j-1]; j-- {
			saida[j], saida[j-1] = saida[j-1], saida[j]
		}
	}
	return saida
}

// ---------------------------------------------------------------------------
// GET /trilogo/chamados.xlsx e .pdf — a extração
// ---------------------------------------------------------------------------

func (c *Consulta) extrairPlanilha(w http.ResponseWriter, r *http.Request) {
	tab, ok := c.montarRelatorio(w, r)
	if !ok {
		return
	}
	bytes, err := tab.Planilha()
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar a planilha.")
		return
	}
	entregar(w, bytes, "chamados-trilogo", "xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

func (c *Consulta) extrairPDF(w http.ResponseWriter, r *http.Request) {
	tab, ok := c.montarRelatorio(w, r)
	if !ok {
		return
	}
	// No papel, onze colunas não cabem numa folha sem virar letra de bula. Saem
	// as que se lê de relance; o resto está na planilha e na ficha.
	tab.Colunas = append(tab.Colunas[:0:0], colunasDoPDF...)
	enxutas := make([][]any, 0, len(tab.Linhas))
	for _, l := range tab.Linhas {
		enxutas = append(enxutas, []any{l[0], l[1], l[2], l[3], l[4], l[5], l[7], l[9]})
	}
	tab.Linhas = enxutas

	bytes, err := tab.PDF()
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui montar o PDF.")
		return
	}
	entregar(w, bytes, "chamados-trilogo", "pdf", "application/pdf")
}

// As colunas da planilha: tudo que a lista mostra.
var colunasDaPlanilha = []relatorio.Coluna{
	{Titulo: "Ticket", Peso: 7, Tipo: relatorio.Numero},
	{Titulo: "Loja", Peso: 16, Tipo: relatorio.Texto},
	{Titulo: "Conta", Peso: 8, Tipo: relatorio.Texto},
	{Titulo: "Status", Peso: 9, Tipo: relatorio.Texto},
	{Titulo: "Prioridade", Peso: 8, Tipo: relatorio.Texto},
	{Titulo: "Descrição", Peso: 40, Tipo: relatorio.Texto},
	{Titulo: "Ambiente", Peso: 26, Tipo: relatorio.Texto},
	{Titulo: "Criado em", Peso: 11, Tipo: relatorio.DataHora},
	{Titulo: "Prazo", Peso: 9, Tipo: relatorio.Data},
	{Titulo: "Responsável", Peso: 12, Tipo: relatorio.Texto},
	{Titulo: "Custo", Peso: 9, Tipo: relatorio.Dinheiro},
	{Titulo: "Anexos", Peso: 7, Tipo: relatorio.Numero},
}

// As do papel: as mesmas, menos ambiente, prazo, responsável e anexos.
var colunasDoPDF = []relatorio.Coluna{
	{Titulo: "Ticket", Peso: 6, Tipo: relatorio.Numero},
	{Titulo: "Loja", Peso: 15, Tipo: relatorio.Texto},
	{Titulo: "Conta", Peso: 8, Tipo: relatorio.Texto},
	{Titulo: "Status", Peso: 9, Tipo: relatorio.Texto},
	{Titulo: "Prioridade", Peso: 8, Tipo: relatorio.Texto},
	{Titulo: "Descrição", Peso: 35, Tipo: relatorio.Texto},
	{Titulo: "Criado em", Peso: 11, Tipo: relatorio.DataHora},
	{Titulo: "Custo", Peso: 8, Tipo: relatorio.Dinheiro},
}

type linhaExtracao struct {
	Numero      int     `json:"numero"`
	Loja        string  `json:"loja"`
	Conta       string  `json:"conta"`
	Status      string  `json:"status"`
	Prioridade  string  `json:"prioridade"`
	Descricao   *string `json:"descricao"`
	Ambiente    *string `json:"ambiente"`
	CriadoEm    string  `json:"criado_em"`
	Prazo       *string `json:"prazo"`
	Responsavel *string `json:"responsavel"`
	CustoTotal  string  `json:"custo_total"`
	Anexos      int     `json:"anexos"`
}

func (c *Consulta) montarRelatorio(w http.ResponseWriter, r *http.Request) (relatorio.Tabela, bool) {
	p := c.quem(w, r)
	if p == nil {
		return relatorio.Tabela{}, false
	}
	filtro, err := c.montarFiltro(p.ClienteID, r.URL.Query())
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return relatorio.Tabela{}, false
	}

	linhas := []linhaExtracao{}
	caminho := fmt.Sprintf("chamados_lista?%s&select=*&order=criado_em.desc,numero.desc&limit=%d",
		filtro, TetoDaExtracao)
	total, err := c.bd.BuscarContando(r.Context(), caminho, &linhas)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui carregar os chamados para a extração.")
		return relatorio.Tabela{}, false
	}

	tab := relatorio.Tabela{
		Titulo:    "Dados do Trílogo — chamados",
		Subtitulo: descreverFiltro(r.URL.Query(), linhas),
		Colunas:   colunasDaPlanilha,
		Gerado:    time.Now(),
	}
	for _, l := range linhas {
		tab.Linhas = append(tab.Linhas, []any{
			l.Numero, l.Loja, contaPorExtenso(l.Conta), l.Status, ouTraco(l.Prioridade),
			texto(l.Descricao), texto(l.Ambiente),
			instanteDoBanco(l.CriadoEm), instanteDoBanco(texto(l.Prazo)),
			texto(l.Responsavel), l.CustoTotal, l.Anexos,
		})
	}
	if total > len(linhas) {
		tab.Aviso = fmt.Sprintf("Mostrando as primeiras %s de %s — refine o filtro",
			comPonto(len(linhas)), comPonto(total))
	}
	return tab, true
}

// entregar manda o arquivo com o nome já pronto.
//
// O nome do arquivo vai no cabeçalho, e o cabeçalho precisa estar EXPOSTO no
// CORS para o navegador deixar o front lê-lo — senão o arquivo chega com o nome
// do endereço, que é feio e não diz nada.
func entregar(w http.ResponseWriter, corpo []byte, base, extensao, tipo string) {
	nome := fmt.Sprintf("%s-%s.%s", base, time.Now().In(relatorio.FusoDaCasa()).Format("2006-01-02"), extensao)
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+nome+"\"; filename*=UTF-8''"+url.PathEscape(nome))
	w.Header().Set("Content-Length", strconv.Itoa(len(corpo)))
	w.WriteHeader(http.StatusOK)
	w.Write(corpo)
}

// descreverFiltro escreve, em português, o que foi filtrado — para o documento
// dizer de si mesmo do que ele trata. Um relatório sem essa linha é um monte de
// linhas sem contexto três semanas depois.
func descreverFiltro(q url.Values, linhas []linhaExtracao) string {
	var partes []string
	if t := somenteDigitos(q.Get("ticket")); t != "" {
		partes = append(partes, "Ticket "+t)
	}
	if q.Get("loja") != "" && len(linhas) > 0 {
		partes = append(partes, "Loja: "+linhas[0].Loja)
	}
	if v := q.Get("status"); v != "" {
		partes = append(partes, "Status: "+v)
	}
	if v := q.Get("conta"); v != "" {
		partes = append(partes, "Conta: "+contaPorExtenso(v))
	}
	if v := q.Get("prioridade"); v != "" {
		if strings.EqualFold(v, "sem") {
			partes = append(partes, "Sem prioridade")
		} else {
			partes = append(partes, "Prioridade: "+v)
		}
	}
	de, ate := q.Get("de"), q.Get("ate")
	switch {
	case de != "" && ate != "":
		partes = append(partes, "Criados de "+diaBonito(de)+" a "+diaBonito(ate))
	case de != "":
		partes = append(partes, "Criados a partir de "+diaBonito(de))
	case ate != "":
		partes = append(partes, "Criados até "+diaBonito(ate))
	}
	if len(partes) == 0 {
		return "Todos os chamados da base"
	}
	return strings.Join(partes, " · ")
}

func contaPorExtenso(c string) string {
	switch c {
	case "instalacoes":
		return "Instalações"
	case "civil":
		return "Civil"
	}
	return c
}

func texto(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ouTraco(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// instanteDoBanco entende tanto o carimbo completo quanto uma data seca. O nome
// é comprido de propósito: `instante` já existe no rotas.go, deste mesmo módulo.
func instanteDoBanco(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, f := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return nil
}

func diaBonito(s string) string {
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(s)); err == nil {
		return t.Format("02/01/2006")
	}
	return s
}

// comPonto escreve 1377 como 1.377.
func comPonto(n int) string {
	s := strconv.Itoa(n)
	var partes []string
	for len(s) > 3 {
		partes = append([]string{s[len(s)-3:]}, partes...)
		s = s[:len(s)-3]
	}
	return strings.Join(append([]string{s}, partes...), ".")
}
