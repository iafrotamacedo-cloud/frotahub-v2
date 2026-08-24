// rev 1 — o robô do Trílogo
//
// TRÊS MODOS, UM CÓDIGO SÓ (CORE-06)
//
//	levantamento — lê tudo desde a data de corte e grava chamado, timeline,
//	               custos e a FICHA de cada arquivo com o tamanho real, obtido
//	               do cabeçalho HTTP. NÃO baixa nada. No fim, sabe-se quantos GB
//	               a cópia vai custar antes de gastar o primeiro byte.
//
//	copia        — busca os bytes do que ainda não foi copiado e manda para o R2.
//
//	atualizacao  — a mesma leitura, com marca d'água: só os chamados cujo
//	               `dateOfLastChange` passou da última rodada concluída.
//
// A diferença entre eles é a janela e o que se faz com os arquivos. Nada mais.
//
// POR QUE EM LOTES
//
//	O motor roda num plano que adormece e reinicia. Trabalho longo em memória se
//	perde inteiro; em lotes, o que já foi gravado está gravado, e a próxima
//	chamada continua de onde parou (P-03).
package trilogo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "time/tzdata" // o fuso vai DENTRO do binário: a imagem do motor não tem tabela de fusos

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
)

const (
	ModoLevantamento = "levantamento"
	ModoCopia        = "copia"
	ModoAtualizacao  = "atualizacao"
)

// A data de onde a carga inicial começa. Decisão do dono.
var DataDeCorte = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

// O Trílogo devolve data e hora SEM fuso, no horário de Fortaleza — é o mesmo
// que aparece na tela dele. Guardar como se fosse UTC jogaria tudo três horas
// para frente e estragaria qualquer métrica de tempo.
var fusoDoCliente = carregarFuso()

func carregarFuso() *time.Location {
	l, err := time.LoadLocation("America/Fortaleza")
	if err != nil {
		return time.FixedZone("BRT", -3*3600)
	}
	return l
}

type Servico struct {
	cfg  *config.Config
	bd   *banco.Cliente
	arm  *armazem.Cliente
	http *http.Client // para os arquivos no S3 público; sem token
}

func Novo(cfg *config.Config, bd *banco.Cliente, arm *armazem.Cliente) *Servico {
	return &Servico{
		cfg: cfg, bd: bd, arm: arm,
		http: &http.Client{
			Timeout:   10 * time.Minute, // vídeo grande em rede ruim
			Transport: &http.Transport{MaxIdleConns: 64, MaxIdleConnsPerHost: 32},
		},
	}
}

// Resultado é o que uma rodada produziu.
type Resultado struct {
	ExecucaoID       string `json:"execucao_id"`
	Modo             string `json:"modo"`
	Situacao         string `json:"situacao"`
	ChamadosLidos    int    `json:"chamados_lidos"`
	ChamadosGravados int    `json:"chamados_gravados"`
	EventosGravados  int    `json:"eventos_gravados"`
	ArquivosVistos   int    `json:"arquivos_vistos"`
	BytesVistos      int64  `json:"bytes_vistos"`
	ArquivosCopiados int    `json:"arquivos_copiados"`
	BytesCopiados    int64  `json:"bytes_copiados"`
	Completo         bool   `json:"completo"` // falso = sobrou trabalho; chame de novo
	Erro             string `json:"erro,omitempty"`
	Duracao          string `json:"duracao"`
}

// ---------------------------------------------------------------------------
// A rodada
// ---------------------------------------------------------------------------

func (s *Servico) Rodar(ctx context.Context, modo, clienteID, disparadoPor string) (*Resultado, error) {
	if modo != ModoLevantamento && modo != ModoCopia && modo != ModoAtualizacao {
		return nil, fmt.Errorf("modo desconhecido: %s", modo)
	}
	if modo != ModoCopia && !s.cfg.Trilogo.Ligado() {
		return nil, fmt.Errorf("as credenciais do Trílogo não estão configuradas (TRILOGO_*)")
	}
	if modo == ModoCopia && !s.arm.Ligado() {
		return nil, fmt.Errorf("o armazém não está configurado (R2_*)")
	}

	comeco := time.Now()
	execID, err := s.abrirExecucao(ctx, modo, clienteID, disparadoPor)
	if err != nil {
		return nil, err
	}
	r := &Resultado{ExecucaoID: execID, Modo: modo}

	var erroDaRodada error
	switch modo {
	case ModoCopia:
		erroDaRodada = s.copiar(ctx, clienteID, r)
	default:
		erroDaRodada = s.ler(ctx, modo, clienteID, r)
	}

	r.Duracao = time.Since(comeco).Round(time.Second).String()
	r.Situacao = "concluida"
	if erroDaRodada != nil {
		r.Situacao = "falhou"
		r.Erro = erroDaRodada.Error()
		r.Completo = false
	}
	s.fecharExecucao(ctx, execID, r)
	return r, erroDaRodada
}

// ---------------------------------------------------------------------------
// Leitura (levantamento e atualização)
// ---------------------------------------------------------------------------

func (s *Servico) ler(ctx context.Context, modo, clienteID string, r *Resultado) error {
	foraDoEscopo, err := s.unidadesForaDoEscopo(ctx, clienteID)
	if err != nil {
		return err
	}

	// A marca d'água só existe na atualização, e vem da última rodada CONCLUÍDA.
	// Se viesse de uma rodada interrompida, os chamados que ela não processou
	// seriam pulados para sempre.
	var marca time.Time
	if modo == ModoAtualizacao {
		if marca, err = s.ultimaMarca(ctx, clienteID); err != nil {
			return err
		}
		// Uma hora de folga: relógios não são iguais, e é barato reprocessar
		// alguns chamados a mais. Perder um é que não pode.
		if !marca.IsZero() {
			marca = marca.Add(-time.Hour)
		}
	}

	limite := 0
	if modo == ModoAtualizacao {
		limite = s.cfg.Trilogo.Lote
	}
	var novaMarca time.Time
	sobrou := false

	for _, conta := range s.cfg.Trilogo.Contas() {
		sessao, err := Entrar(ctx, conta.Nome, conta.Email, conta.Senha)
		if err != nil {
			return err
		}

		// A lista vem do chamado mais novo para o mais velho. Assim que uma
		// página inteira fica antes da data de corte, não há mais nada a buscar.
		lista, err := sessao.Listar(ctx, func(p []Resumo) bool {
			for _, t := range p {
				if !t.Criacao().Before(DataDeCorte) {
					return false
				}
			}
			return true
		})
		if err != nil {
			return fmt.Errorf("conta %s: %w", conta.Nome, err)
		}

		var aFazer []Resumo
		for _, t := range lista {
			if t.Criacao().Before(DataDeCorte) {
				continue
			}
			if foraDoEscopo[t.Company.ID] {
				continue
			}
			if alt := hora(t.DateOfLastChange); !alt.IsZero() {
				if alt.After(novaMarca) {
					novaMarca = alt
				}
				if !marca.IsZero() && !alt.After(marca) {
					continue // nada mudou desde a última leitura
				}
			}
			aFazer = append(aFazer, t)
		}
		r.ChamadosLidos += len(lista)

		if limite > 0 && len(aFazer) > limite {
			aFazer = aFazer[:limite]
			sobrou = true
		}
		if limite > 0 {
			limite -= len(aFazer)
		}

		if err := s.processar(ctx, sessao, clienteID, aFazer, r); err != nil {
			return err
		}
		log.Printf("[trilogo] %s · conta %s · %d na janela · %d processados", modo, conta.Nome, len(lista), len(aFazer))
	}

	r.Completo = !sobrou
	// A marca d'água só avança se a rodada varreu tudo.
	if r.Completo && !novaMarca.IsZero() {
		s.marcarAgua(ctx, r.ExecucaoID, novaMarca)
	}
	return nil
}

// processar busca o detalhe de cada chamado em paralelo e grava em lotes.
func (s *Servico) processar(ctx context.Context, sessao *Sessao, clienteID string, alvos []Resumo, r *Resultado) error {
	const porLote = 50
	for i := 0; i < len(alvos); i += porLote {
		fim := min(i+porLote, len(alvos))
		if err := s.umLote(ctx, sessao, clienteID, alvos[i:fim], r); err != nil {
			return err
		}
	}
	return nil
}

type colhido struct {
	resumo  Resumo
	detalhe *Detalhe
	custos  []Custo
	// tamanho de cada arquivo, medido sem baixar: url -> (bytes, tipo)
	medidas map[string][2]string
}

func (s *Servico) umLote(ctx context.Context, sessao *Sessao, clienteID string, alvos []Resumo, r *Resultado) error {
	// --- 1) buscar tudo do Trílogo, em paralelo -----------------------------
	colhidos := make([]*colhido, len(alvos))
	vagas := make(chan struct{}, s.cfg.Trilogo.Paralelo)
	var espera sync.WaitGroup
	var mu sync.Mutex
	var primeiroErro error

	for i, alvo := range alvos {
		espera.Add(1)
		go func(i int, alvo Resumo) {
			defer espera.Done()
			vagas <- struct{}{}
			defer func() { <-vagas }()

			d, err := sessao.Detalhe(ctx, alvo.ID)
			if err != nil {
				mu.Lock()
				if primeiroErro == nil {
					primeiroErro = fmt.Errorf("chamado %d: %w", alvo.ID, err)
				}
				mu.Unlock()
				return
			}
			c, _ := sessao.Custos(ctx, alvo.ID) // sem custo é o caso comum, não é erro

			col := &colhido{resumo: alvo, detalhe: d, custos: c, medidas: map[string][2]string{}}
			for _, url := range urlsDoChamado(d, c) {
				if n, tipo, err := Medir(ctx, s.http, url); err == nil {
					col.medidas[url] = [2]string{strconv.FormatInt(n, 10), tipo}
				}
			}
			colhidos[i] = col
		}(i, alvo)
	}
	espera.Wait()
	if primeiroErro != nil {
		return primeiroErro
	}

	// --- 2) unidades --------------------------------------------------------
	unidades, err := s.garantirUnidades(ctx, clienteID, colhidos)
	if err != nil {
		return err
	}

	// --- 3) chamados --------------------------------------------------------
	ids, err := s.gravarChamados(ctx, clienteID, sessao.Conta, colhidos, unidades)
	if err != nil {
		return err
	}
	r.ChamadosGravados += len(ids)

	// --- 4) timeline, custos e fichas de arquivo ----------------------------
	return s.gravarFilhos(ctx, colhidos, ids, r)
}

func urlsDoChamado(d *Detalhe, custos []Custo) []string {
	var fora []string
	for _, a := range d.Attachments {
		if a.Image != "" {
			fora = append(fora, a.Image)
		}
	}
	for _, c := range custos {
		for _, f := range c.InvoiceFiles {
			if f.Permalink != "" {
				fora = append(fora, f.Permalink)
			}
		}
	}
	return fora
}

// ---------------------------------------------------------------------------
// Gravações
// ---------------------------------------------------------------------------

func (s *Servico) garantirUnidades(ctx context.Context, clienteID string, cols []*colhido) (map[int]string, error) {
	vistas := map[int]map[string]any{}
	for _, c := range cols {
		if c == nil || c.detalhe.Company.ID == 0 {
			continue
		}
		co := c.detalhe.Company
		vistas[co.ID] = map[string]any{
			"cliente_id": clienteID, "id_trilogo": co.ID,
			"nome": strings.TrimSpace(co.Name), "cidade": co.City, "uf": co.State, "endereco": co.Address,
		}
	}
	if len(vistas) == 0 {
		return map[int]string{}, nil
	}
	linhas := make([]map[string]any, 0, len(vistas))
	for _, v := range vistas {
		linhas = append(linhas, v)
	}
	// A unidade nova nasce no escopo. As quatro que ficam de fora já existem no
	// banco, semeadas pela migração — e como `no_escopo` não vai no envio, o
	// valor delas não é tocado.
	var fora []struct {
		ID        string `json:"id"`
		IDTrilogo int    `json:"id_trilogo"`
	}
	if err := s.bd.Upsert(ctx, "unidades?on_conflict=cliente_id,id_trilogo", linhas, &fora); err != nil {
		return nil, fmt.Errorf("unidades: %w", err)
	}
	mapa := map[int]string{}
	for _, u := range fora {
		mapa[u.IDTrilogo] = u.ID
	}
	return mapa, nil
}

func (s *Servico) gravarChamados(ctx context.Context, clienteID, conta string, cols []*colhido, unidades map[int]string) (map[int]string, error) {
	linhas := make([]map[string]any, 0, len(cols))
	for _, c := range cols {
		if c == nil {
			continue
		}
		d := c.detalhe
		l := map[string]any{
			"cliente_id": clienteID,
			"numero":     d.ID,
			"conta":      conta,
			"descricao":  strings.TrimSpace(d.Description),

			"status_codigo":     d.Status,
			"status":            RotuloStatus(d.Status),
			"prioridade_codigo": d.Priority,
			"prioridade":        RotuloPrioridade(d.Priority),
			"tipo_codigo":       d.Type,
			"tipo":              RotuloTipo(d.Type),

			"natureza":        d.Nature,
			"prestadora":      d.ServiceCompany.Name,
			"prestadora_cnpj": d.ServiceCompany.CNPJ,

			"criado_em":     nulo(d.CreationDateTime),
			"alterado_em":   nulo(d.DateOfLastChange),
			"executado_em":  nulo(d.DateOfLastExecution),
			"vistoriado_em": nulo(d.DateOfLastInspection),
			"concluido_em":  nulo(d.DateOfLastConclusion),
			"prazo":         dataNula(d.DeadlineDate),
			"lido_em":       time.Now().UTC().Format(time.RFC3339),
		}
		if id, ok := unidades[d.Company.ID]; ok {
			l["unidade_id"] = id
		}
		if d.Department != nil {
			l["ambiente"] = d.Department.FullAddress
			l["ambiente_id"] = d.Department.ID
		}
		if d.BuildingServiceType != nil {
			l["tipo_predial"] = d.BuildingServiceType.Name
		}
		if d.Creator != nil {
			l["criado_por"] = d.Creator.Name
			l["criado_por_id"] = d.Creator.ID
		}
		if d.ServiceCompanyAssignee != nil {
			l["responsavel"] = d.ServiceCompanyAssignee.Name
		}
		linhas = append(linhas, l)
	}
	if len(linhas) == 0 {
		return map[int]string{}, nil
	}

	var fora []struct {
		ID     string `json:"id"`
		Numero int    `json:"numero"`
	}
	if err := s.bd.Upsert(ctx, "chamados?on_conflict=cliente_id,numero", linhas, &fora); err != nil {
		return nil, fmt.Errorf("chamados: %w", err)
	}
	mapa := map[int]string{}
	for _, c := range fora {
		mapa[c.Numero] = c.ID
	}
	return mapa, nil
}

func (s *Servico) gravarFilhos(ctx context.Context, cols []*colhido, ids map[int]string, r *Resultado) error {
	var eventos, custos []map[string]any

	for _, c := range cols {
		if c == nil {
			continue
		}
		chamadoID, ok := ids[c.detalhe.ID]
		if !ok {
			continue
		}
		for _, e := range c.detalhe.Activity.Histories {
			eventos = append(eventos, map[string]any{
				"chamado_id":    chamadoID,
				"chave":         chaveDoEvento(e),
				"tipo_codigo":   e.RecordType,
				"tipo":          RotuloEvento(e.RecordType),
				"status_codigo": nuloSeZero(e.StatusAction),
				"status":        RotuloStatus(e.StatusAction),
				"quando":        nulo(e.CreationDate),
				"autor":         e.AuthorName,
				"autor_id":      nuloSeZero(e.AuthorID),
				"texto":         e.Texto(),
			})
		}
		for _, k := range c.custos {
			custos = append(custos, map[string]any{
				"chamado_id": chamadoID, "id_trilogo": k.ID,
				"tipo_codigo": k.Type, "tipo": RotuloCusto(k.Type),
				"valor": k.TotalValue, "valor_servico": k.ServiceCost, "valor_material": k.ProductCost,
				"numero_documento": k.DocumentNumber, "empresa": k.Company.Name,
				"criado_em": nulo(k.IssueDate),
			})
		}
	}

	// A timeline não se reescreve: evento gravado não muda. Mandar tudo de novo é
	// barato, e o banco descarta o que já tem.
	if len(eventos) > 0 {
		if err := s.bd.InserirIgnorando(ctx, "chamado_eventos?on_conflict=chamado_id,chave", eventos); err != nil {
			return fmt.Errorf("timeline: %w", err)
		}
		r.EventosGravados += len(eventos)
	}

	// Custo muda de valor, então este é upsert de verdade — e precisa voltar com
	// o id, porque o arquivo de orçamento aponta para ele.
	custoIDs := map[string]string{} // "chamadoID/idTrilogo" -> uuid
	if len(custos) > 0 {
		var fora []struct {
			ID        string `json:"id"`
			ChamadoID string `json:"chamado_id"`
			IDTrilogo int    `json:"id_trilogo"`
		}
		if err := s.bd.Upsert(ctx, "chamado_custos?on_conflict=chamado_id,id_trilogo", custos, &fora); err != nil {
			return fmt.Errorf("custos: %w", err)
		}
		for _, k := range fora {
			custoIDs[k.ChamadoID+"/"+strconv.Itoa(k.IDTrilogo)] = k.ID
		}
	}

	return s.gravarFichas(ctx, cols, ids, custoIDs, r)
}

// gravarFichas registra CADA APARIÇÃO de arquivo — com o tamanho, sem baixar.
func (s *Servico) gravarFichas(ctx context.Context, cols []*colhido, ids map[int]string, custoIDs map[string]string, r *Resultado) error {
	var fichas []map[string]any

	for _, c := range cols {
		if c == nil {
			continue
		}
		chamadoID, ok := ids[c.detalhe.ID]
		if !ok {
			continue
		}
		for _, a := range c.detalhe.Attachments {
			if a.Image == "" {
				continue
			}
			fichas = append(fichas, s.ficha(c, chamadoID, "anexo", a.ID, a.FileName, a.Image, "", map[string]any{
				"autor": a.Author, "autor_id": nuloSeZero(a.AuthorID), "quando": nulo(a.CreationDateTime),
			}))
		}
		for _, k := range c.custos {
			custoID := custoIDs[chamadoID+"/"+strconv.Itoa(k.ID)]
			for _, f := range k.InvoiceFiles {
				if f.Permalink == "" {
					continue
				}
				fichas = append(fichas, s.ficha(c, chamadoID, "orcamento", f.ID, f.FileName, f.Permalink, custoID, map[string]any{
					"origem": origemDoOrcamento(f.FileName),
				}))
			}
		}
	}
	if len(fichas) == 0 {
		return nil
	}

	// ATENÇÃO: `arquivo_sha256` NÃO vai no envio. Se fosse, o upsert apagaria com
	// nulo o vínculo de um arquivo já copiado, e a cópia seria refeita do zero a
	// cada leitura.
	if err := s.bd.Upsert(ctx, "chamado_anexos?on_conflict=chamado_id,colecao,id_trilogo", fichas, nil); err != nil {
		return fmt.Errorf("anexos: %w", err)
	}
	for _, f := range fichas {
		r.ArquivosVistos++
		if n, ok := f["tamanho"].(int64); ok {
			r.BytesVistos += n
		}
	}
	return nil
}

func (s *Servico) ficha(c *colhido, chamadoID, colecao string, idTrilogo int, nome, url, custoID string, extra map[string]any) map[string]any {
	f := map[string]any{
		"chamado_id": chamadoID, "colecao": colecao, "id_trilogo": idTrilogo,
		"nome": nome, "extensao": Extensao(nome), "url_origem": url,
	}
	if custoID != "" {
		f["custo_id"] = custoID
	}
	if m, ok := c.medidas[url]; ok {
		if n, err := strconv.ParseInt(m[0], 10, 64); err == nil && n > 0 {
			f["tamanho"] = n
		}
		if m[1] != "" {
			f["tipo"] = m[1]
		}
	}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

// origemDoOrcamento reconhece o que o robô antigo subiu.
//
// Ele gerava os PDFs com o nome temporário do Python ("tmp" + aleatório). Marcar
// isso agora deixa claro, para sempre, o que veio do sistema velho e o que o
// FrotaHub passou a produzir.
func origemDoOrcamento(nome string) string {
	if strings.HasPrefix(strings.ToLower(nome), "tmp") {
		return "sistema-antigo"
	}
	return "trilogo"
}

// chaveDoEvento devolve a identidade do evento na timeline.
//
// Quase todos têm id próprio. UM TIPO VEM COM id = 0 — e, sem uma chave para
// eles, entrariam de novo a cada leitura, para sempre. Para esses a identidade é
// uma impressão digital do próprio conteúdo.
func chaveDoEvento(e Evento) string {
	if e.ID != 0 {
		return strconv.Itoa(e.ID)
	}
	semente := strings.Join([]string{
		strconv.Itoa(e.RecordType), e.CreationDate, strconv.Itoa(e.AuthorID), e.AuthorName, e.Texto(),
	}, "|")
	soma := sha256.Sum256([]byte(semente))
	return "h:" + hex.EncodeToString(soma[:])[:24]
}

// ---------------------------------------------------------------------------
// Cópia dos arquivos
// ---------------------------------------------------------------------------

type pendente struct {
	ID       string `json:"id"`
	URL      string `json:"url_origem"`
	Nome     string `json:"nome"`
	Extensao string `json:"extensao"`
	Tipo     string `json:"tipo"`
}

func (s *Servico) copiar(ctx context.Context, clienteID string, r *Resultado) error {
	slug, err := s.slugDoCliente(ctx, clienteID)
	if err != nil {
		return err
	}

	lote := s.cfg.Trilogo.Lote
	var restam []pendente
	caminho := "chamado_anexos?arquivo_sha256=is.null&select=id,url_origem,nome,extensao,tipo&limit=" + strconv.Itoa(lote)
	if err := s.bd.Buscar(ctx, caminho, &restam); err != nil {
		return fmt.Errorf("não consegui listar o que falta copiar: %w", err)
	}
	if len(restam) == 0 {
		r.Completo = true
		return nil
	}

	vagas := make(chan struct{}, s.cfg.Trilogo.Paralelo)
	var espera sync.WaitGroup
	var mu sync.Mutex

	for _, p := range restam {
		espera.Add(1)
		go func(p pendente) {
			defer espera.Done()
			vagas <- struct{}{}
			defer func() { <-vagas }()

			sha, tamanho, err := s.umArquivo(ctx, clienteID, slug, p)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				// Um arquivo que falha não derruba a rodada: fica sem vínculo e a
				// próxima passada tenta de novo. Some do log, não do banco.
				log.Printf("[trilogo] arquivo %s falhou: %v", p.ID, err)
				return
			}
			if sha != "" {
				r.ArquivosCopiados++
				r.BytesCopiados += tamanho
			}
		}(p)
	}
	espera.Wait()

	// Menos que o lote significa que acabou.
	r.Completo = len(restam) < lote
	return nil
}

func (s *Servico) umArquivo(ctx context.Context, clienteID, slug string, p pendente) (string, int64, error) {
	corpo, tipo, err := Baixar(ctx, s.http, p.URL)
	if err != nil {
		return "", 0, err
	}
	defer corpo.Close()

	// Passa pelo disco de propósito. O caminho no armazém é o sha256 do conteúdo,
	// e o sha256 só se conhece depois de ler o arquivo inteiro — guardar vídeos de
	// dezenas de megabytes na memória, doze ao mesmo tempo, derrubaria o processo.
	tmp, err := os.CreateTemp("", "frotahub-*")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	soma := sha256.New()
	tamanho, err := io.Copy(io.MultiWriter(tmp, soma), corpo)
	if err != nil {
		return "", 0, err
	}
	sha := hex.EncodeToString(soma.Sum(nil))
	if tipo == "" {
		tipo = p.Tipo
	}

	// Já temos este CONTEÚDO? Então não se envia de novo — só se aponta para ele.
	// É isto que impede o orçamento de ocupar espaço duas vezes.
	var jaTem []struct {
		SHA string `json:"sha256"`
	}
	_ = s.bd.Buscar(ctx, "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=sha256&limit=1", &jaTem)

	if len(jaTem) == 0 {
		chave := armazem.Caminho(slug, sha, p.Extensao)
		if err := s.arm.Enviar(ctx, chave, tmp, tamanho, sha, tipo); err != nil {
			return "", 0, err
		}
		novo := []map[string]any{{
			"sha256": sha, "cliente_id": clienteID, "tamanho": tamanho, "tipo": tipo, "chave_r2": chave,
		}}
		if err := s.bd.InserirIgnorando(ctx, "arquivos?on_conflict=sha256", novo); err != nil {
			return "", 0, err
		}
	}

	if err := s.bd.Atualizar(ctx, "chamado_anexos", "id=eq."+banco.Escapar(p.ID),
		map[string]any{"arquivo_sha256": sha}); err != nil {
		return "", 0, err
	}
	if len(jaTem) > 0 {
		return "", 0, nil // já estava lá; não conta como cópia nova
	}
	return sha, tamanho, nil
}

// ---------------------------------------------------------------------------
// Apoio
// ---------------------------------------------------------------------------

func (s *Servico) unidadesForaDoEscopo(ctx context.Context, clienteID string) (map[int]bool, error) {
	var linhas []struct {
		IDTrilogo int `json:"id_trilogo"`
	}
	caminho := "unidades?cliente_id=eq." + banco.Escapar(clienteID) + "&no_escopo=is.false&select=id_trilogo"
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, fmt.Errorf("não consegui ler quais unidades ficam de fora: %w", err)
	}
	fora := map[int]bool{}
	for _, l := range linhas {
		fora[l.IDTrilogo] = true
	}
	return fora, nil
}

func (s *Servico) slugDoCliente(ctx context.Context, clienteID string) (string, error) {
	var linhas []struct {
		Slug string `json:"slug"`
	}
	if err := s.bd.Buscar(ctx, "clientes?id=eq."+banco.Escapar(clienteID)+"&select=slug&limit=1", &linhas); err != nil || len(linhas) == 0 {
		return "", fmt.Errorf("não achei o cliente")
	}
	return linhas[0].Slug, nil
}

func (s *Servico) ultimaMarca(ctx context.Context, clienteID string) (time.Time, error) {
	var linhas []struct {
		Marca string `json:"marca_dagua"`
	}
	caminho := "robo_execucoes?cliente_id=eq." + banco.Escapar(clienteID) +
		"&robo=eq.trilogo&situacao=eq.concluida&marca_dagua=not.is.null" +
		"&select=marca_dagua&order=marca_dagua.desc&limit=1"
	if err := s.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return time.Time{}, err
	}
	if len(linhas) == 0 {
		return time.Time{}, nil
	}
	t, _ := time.Parse(time.RFC3339, linhas[0].Marca)
	return t, nil
}

func (s *Servico) abrirExecucao(ctx context.Context, modo, clienteID, quem string) (string, error) {
	var fora []struct {
		ID string `json:"id"`
	}
	linha := []map[string]any{{
		"cliente_id": clienteID, "robo": "trilogo", "modo": modo,
		"disparado_por": quem, "janela_de": DataDeCorte.Format("2006-01-02"),
	}}
	if err := s.bd.Inserir(ctx, "robo_execucoes", linha, &fora); err != nil {
		return "", fmt.Errorf("não consegui abrir a rodada: %w", err)
	}
	if len(fora) == 0 {
		return "", fmt.Errorf("a rodada foi aberta mas o banco não devolveu qual")
	}
	return fora[0].ID, nil
}

func (s *Servico) fecharExecucao(ctx context.Context, id string, r *Resultado) {
	campos := map[string]any{
		"situacao": r.Situacao, "terminou_em": time.Now().UTC().Format(time.RFC3339),
		"chamados_lidos": r.ChamadosLidos, "chamados_gravados": r.ChamadosGravados,
		"eventos_gravados": r.EventosGravados,
		"arquivos_vistos":  r.ArquivosVistos, "bytes_vistos": r.BytesVistos,
		"arquivos_copiados": r.ArquivosCopiados, "bytes_copiados": r.BytesCopiados,
	}
	if r.Erro != "" {
		campos["erro"] = r.Erro
	}
	if err := s.bd.Atualizar(ctx, "robo_execucoes", "id=eq."+banco.Escapar(id), campos); err != nil {
		log.Printf("[trilogo] não consegui fechar a rodada %s: %v", id, err)
	}
}

func (s *Servico) marcarAgua(ctx context.Context, id string, t time.Time) {
	_ = s.bd.Atualizar(ctx, "robo_execucoes", "id=eq."+banco.Escapar(id),
		map[string]any{"marca_dagua": t.Format(time.RFC3339)})
}

// ---------------------------------------------------------------------------
// Datas
// ---------------------------------------------------------------------------

// hora entende o que o Trílogo manda e devolve o instante certo.
//
// Ele manda "2026-08-22T15:22:18.711", sem fuso — e é o horário de Fortaleza, o
// mesmo que aparece na tela dele. Tratar como UTC jogaria tudo três horas para
// frente e estragaria qualquer métrica de tempo entre estados.
func hora(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, f := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(f, s, fusoDoCliente); err == nil {
			return t
		}
	}
	// Alguns campos vêm no formato da tela.
	for _, f := range []string{"02/01/2006 15:04", "02/01/2006"} {
		if t, err := time.ParseInLocation(f, s, fusoDoCliente); err == nil {
			return t
		}
	}
	return time.Time{}
}

func nulo(s string) any {
	t := hora(s)
	if t.IsZero() {
		return nil
	}
	return t.Format(time.RFC3339)
}

func dataNula(s string) any {
	t := hora(s)
	if t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02")
}

func nuloSeZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

// Json é usado só pelos testes, para conferir o que seria enviado.
func Json(v any) string { b, _ := json.Marshal(v); return string(b) }
