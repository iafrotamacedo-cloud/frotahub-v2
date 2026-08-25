// rev 2 — o robô do Trílogo
//
// QUATRO MODOS, UM CÓDIGO SÓ (CORE-06)
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
//	alvos        — lê CHAMADOS DITADOS, pelo número, e mais nenhum. Não tem
//	               janela, não tem marca d'água, não varre lista.
//
// Entre os três primeiros a diferença é a janela e o que se faz com os arquivos.
// O quarto é de outra natureza: não pergunta "o que mudou?", pergunta "traga
// estes". É o modo de buscar um punhado de chamados antigos sem arrastar junto
// tudo o que foi criado entre eles e hoje.
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
	"sort"
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
	ModoAlvos        = "alvos"
)

// Modos é a lista completa, para quem precisa validar sem repetir os nomes.
// `robo_execucoes.modo` tem um CHECK com exatamente estes valores (migração 012):
// modo novo aqui sem migração lá é uma rodada que morre na primeira linha.
var Modos = []string{ModoLevantamento, ModoCopia, ModoAtualizacao, ModoAlvos}

func modoConhecido(m string) bool {
	for _, x := range Modos {
		if x == m {
			return true
		}
	}
	return false
}

// A data de onde a carga inicial começa. Decisão do dono.
var DataDeCorte = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

// FolgaDoRelogio — quanto se volta no tempo antes da marca d'água, ao filtrar.
//
// Existe porque o relógio do Trílogo e o nosso não são o mesmo, e porque um
// chamado pode mudar no exato instante da leitura anterior. Reprocessar alguns é
// barato; perder um, não.
//
// ERA UMA HORA, E ISSO CUSTAVA CARO. A folga entra em TODA volta, então vira
// retrabalho a cada lote: com uma hora de folga e chamados mudando de minuto em
// minuto, um lote de 75 gastava 60 refazendo e só 15 andavam de fato — 300
// chamados levavam 16 voltas em vez de 5. A diferença entre os dois relógios é
// de segundos, não de horas.
const FolgaDoRelogio = 5 * time.Minute

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
	// O que NÃO vem para o armazém: fica só o endereço no Trílogo. Aparece
	// separado no log porque a diferença entre os dois é a decisão que o dono
	// tomou — e ela precisa estar visível toda vez, não escondida numa soma.
	ArquivosSoLink   int    `json:"arquivos_so_link"`
	BytesSoLink      int64  `json:"bytes_so_link"`
	ArquivosCopiados int    `json:"arquivos_copiados"`
	BytesCopiados    int64  `json:"bytes_copiados"`
	Completo         bool   `json:"completo"` // falso = sobrou trabalho; chame de novo
	Erro             string `json:"erro,omitempty"`
	Duracao          string `json:"duracao"`
	// Só o modo `alvos` preenche: os números pedidos que não existem em conta
	// nenhuma. Não é erro da rodada — é resposta, e precisa aparecer. Uma lista
	// de 71 que grava 66 sem dizer quais cinco faltaram é pior que um erro.
	NaoEncontrados []int `json:"nao_encontrados,omitempty"`
}

// ---------------------------------------------------------------------------
// A rodada
// ---------------------------------------------------------------------------

func (s *Servico) Rodar(ctx context.Context, modo, clienteID, disparadoPor string) (*Resultado, error) {
	if !modoConhecido(modo) {
		return nil, fmt.Errorf("modo desconhecido: %s (conheço %s)", modo, strings.Join(Modos, ", "))
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
	case ModoAlvos:
		erroDaRodada = s.lerAlvos(ctx, clienteID, r)
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

// ler traz o que mudou no Trílogo.
//
// A MARCA D'ÁGUA É UM CURSOR, E CURSOR SÓ SERVE SE ANDAR
//
//	A primeira versão só gravava a marca quando a rodada varria TUDO. Como cada
//	rodada processa um lote e devolve "faltou", a marca nunca era gravada — e a
//	rodada seguinte recomeçava da data de corte, pegava os mesmos 150 e devolvia
//	"faltou" outra vez. Laço infinito: nove rodadas seguidas lendo 1.600 e
//	gravando os mesmos 150, até o corte de tempo do Actions matar o processo.
//
//	O conserto tem três partes:
//
//	1. Processar do MAIS ANTIGO para o mais novo. Pegando sempre os 150 mais
//	   recentes, o lote seguinte é o mesmo lote. Pegando os mais antigos, cada
//	   volta empurra a fronteira para a frente.
//
//	2. Gravar a marca do que FOI PROCESSADO, e não do que foi visto. A marca
//	   passa a significar "tudo que mudou até aqui já entrou" — que é o único
//	   significado que permite continuar de onde parou.
//
//	3. Orçamento POR CONTA. O limite era um só, descontado na primeira conta e
//	   chegando zerado na segunda: a Civil rodava sem limite nenhum, e nas
//	   rodadas em que a Instalações consumia tudo, a Civil não andava nunca.
//
//	A marca gravada é a MENOR fronteira entre as contas — o ponto até onde todas
//	terminaram. Guardar a maior faria a conta atrasada pular o que não leu.
func (s *Servico) ler(ctx context.Context, modo, clienteID string, r *Resultado) error {
	foraDoEscopo, err := s.unidadesForaDoEscopo(ctx, clienteID)
	if err != nil {
		return err
	}

	// `marcaGravada` é o que está no banco; `desde` é ela com a folga do relógio,
	// para o filtro. O avanço é sempre comparado com a GRAVADA, senão a folga
	// faria a marca andar para trás.
	var marcaGravada time.Time
	if modo == ModoAtualizacao {
		if marcaGravada, err = s.ultimaMarca(ctx, clienteID); err != nil {
			return err
		}
	}
	desde := marcaGravada
	if !desde.IsZero() {
		desde = desde.Add(-FolgaDoRelogio)
	}

	contas := s.cfg.Trilogo.Contas()
	porConta := 0
	if modo == ModoAtualizacao && len(contas) > 0 {
		porConta = s.cfg.Trilogo.Lote / len(contas)
		if porConta < 25 {
			porConta = 25
		}
	}

	sobrou := false
	var fronteiras []time.Time

	for _, conta := range contas {
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
		r.ChamadosLidos += len(lista)

		var pendentes []Resumo
		var maisNovoDaConta time.Time
		for _, t := range lista {
			if t.Criacao().Before(DataDeCorte) || foraDoEscopo[t.Company.ID] {
				continue
			}
			alt := hora(t.DateOfLastChange)
			if alt.After(maisNovoDaConta) {
				maisNovoDaConta = alt
			}
			// Chamado sem data de alteração entra sempre: não dá para saber se
			// mudou, e deixar de fora seria apostar que não.
			if !desde.IsZero() && !alt.IsZero() && !alt.After(desde) {
				continue
			}
			pendentes = append(pendentes, t)
		}

		// Do mais antigo para o mais novo: é isto que faz a fronteira andar.
		sort.SliceStable(pendentes, func(i, j int) bool {
			return hora(pendentes[i].DateOfLastChange).Before(hora(pendentes[j].DateOfLastChange))
		})

		aFazer := pendentes
		fronteira := maisNovoDaConta // conta em dia: chegou até o fim da lista
		if porConta > 0 && len(aFazer) > porConta {
			aFazer = aFazer[:porConta]
			sobrou = true
			// A fronteira é a alteração do ÚLTIMO processado. O que vier depois
			// dele fica para a próxima volta.
			if ultima := hora(aFazer[len(aFazer)-1].DateOfLastChange); !ultima.IsZero() {
				fronteira = ultima
			} else {
				fronteira = marcaGravada // sem data confiável, não move a marca
			}
		}
		if !fronteira.IsZero() {
			fronteiras = append(fronteiras, fronteira)
		}

		if err := s.processar(ctx, sessao, clienteID, numerosDe(aFazer), r); err != nil {
			return err
		}
		log.Printf("[trilogo] %s · conta %s · %d na lista · %d pendentes · %d processados",
			modo, conta.Nome, len(lista), len(pendentes), len(aFazer))
	}

	r.Completo = !sobrou

	if modo == ModoAtualizacao {
		if nova := menorInstante(fronteiras); !nova.IsZero() && nova.After(marcaGravada) {
			s.marcarAgua(ctx, r.ExecucaoID, nova)
			log.Printf("[trilogo] marca d'água avançou para %s", nova.Format(time.RFC3339))
		} else if !r.Completo {
			// Não andou e ainda falta: a próxima volta faria exatamente o mesmo
			// trabalho. Melhor parar e dizer.
			log.Printf("[trilogo] AVISO: o lote não fez a marca d'água avançar (marca %s). "+
				"Pode haver mais de %d chamados com o mesmo horário de alteração.",
				marcaGravada.Format(time.RFC3339), porConta)
		}
	}
	return nil
}

// menorInstante devolve a menor fronteira — o ponto até onde TODAS as contas
// chegaram. Instantes zerados (conta vazia) não entram na conta: uma conta sem
// chamado nenhum não pode segurar o progresso das outras.
func menorInstante(ts []time.Time) time.Time {
	var menor time.Time
	for _, t := range ts {
		if t.IsZero() {
			continue
		}
		if menor.IsZero() || t.Before(menor) {
			menor = t
		}
	}
	return menor
}

// ---------------------------------------------------------------------------
// Leitura por alvo — chamados ditados, pelo número
// ---------------------------------------------------------------------------

// lerAlvos lê exatamente os chamados da lista, e mais nenhum.
//
// POR QUE ISTO NÃO É UMA JANELA MAIOR
//
//	A alternativa óbvia seria mandar o `levantamento` voltar mais no tempo. Mas
//	os chamados que faltam podem estar a um ano de distância: a janela leria
//	dezenas de milhares para aproveitar algumas dezenas, e — o que importa mais —
//	dependeria de um FILTRO para não gravar todo o resto. Filtro é código, e
//	código erra. Aqui a garantia é de outra ordem: o que não está na lista não
//	chega a ser consultado. Não existe caminho pelo qual uma linha de outro
//	chamado seja escrita.
//
// AS DUAS CONTAS
//
//	Cada chamado pertence a uma delas, e nem sempre se sabe qual. Então pergunta
//	à primeira; o que ela não conhece vai para a segunda. "Não é desta conta" é a
//	resposta esperada em boa parte das perguntas, não um erro — por isso o lote
//	roda em modo tolerante (ver `colher`).
//
// UMA RODADA SÓ
//
//	Não há cursor nem marca d'água para avançar: a lista é finita e conhecida
//	desde o começo. A rodada devolve `Completo` e o laço de fora não repete.
func (s *Servico) lerAlvos(ctx context.Context, clienteID string, r *Resultado) error {
	pedidos := s.cfg.Trilogo.Alvos
	if len(pedidos) == 0 {
		return fmt.Errorf("o modo %s precisa da lista de chamados em TRILOGO_ALVOS "+
			"(números separados por vírgula, espaço ou quebra de linha)", ModoAlvos)
	}
	r.ChamadosLidos = len(pedidos)
	log.Printf("[trilogo] alvos · %d chamado(s) pedidos: %s", len(pedidos), listaDeNumeros(pedidos))

	faltam := pedidos
	for _, conta := range s.cfg.Trilogo.Contas() {
		if len(faltam) == 0 {
			break
		}
		sessao, err := Entrar(ctx, conta.Nome, conta.Email, conta.Senha)
		if err != nil {
			return fmt.Errorf("conta %s: %w", conta.Nome, err)
		}

		const porLote = 50
		var ausentes []int
		for i := 0; i < len(faltam); i += porLote {
			fim := min(i+porLote, len(faltam))
			fora, err := s.umLote(ctx, sessao, clienteID, faltam[i:fim], r, true)
			if err != nil {
				return err
			}
			ausentes = append(ausentes, fora...)
		}
		log.Printf("[trilogo] alvos · conta %s · %d perguntados · %d gravados · %d são de outra conta",
			conta.Nome, len(faltam), len(faltam)-len(ausentes), len(ausentes))
		faltam = ausentes
	}

	r.Completo = true
	r.NaoEncontrados = faltam
	if len(faltam) > 0 {
		// Não derruba a rodada: os outros foram gravados e isso é trabalho bom.
		// Mas tem que ficar GRITADO no log, senão some no meio dos números.
		log.Printf("[trilogo] AVISO: %d chamado(s) não existem em NENHUMA das contas: %s",
			len(faltam), listaDeNumeros(faltam))
	}
	return nil
}

func listaDeNumeros(ns []int) string {
	p := make([]string, len(ns))
	for i, n := range ns {
		p[i] = strconv.Itoa(n)
	}
	return strings.Join(p, ", ")
}

// numerosDe extrai só o que o resto do caminho usa: o número do chamado.
//
// A lista serve para DESCOBRIR quais chamados ler; da descoberta em diante o
// `Resumo` não é mais consultado — tudo vem do `Detalhe`. Reduzir a passagem ao
// número é o que permite o modo `alvos` reaproveitar exatamente o mesmo caminho
// de gravação sem ter uma lista de onde tirar `Resumo` nenhum.
func numerosDe(rs []Resumo) []int {
	ns := make([]int, len(rs))
	for i, t := range rs {
		ns[i] = t.ID
	}
	return ns
}

// processar busca o detalhe de cada chamado em paralelo e grava em lotes.
func (s *Servico) processar(ctx context.Context, sessao *Sessao, clienteID string, numeros []int, r *Resultado) error {
	const porLote = 50
	for i := 0; i < len(numeros); i += porLote {
		fim := min(i+porLote, len(numeros))
		// Aqui os números vieram da lista da própria conta: se um não responde,
		// é falha de verdade e a rodada para.
		if _, err := s.umLote(ctx, sessao, clienteID, numeros[i:fim], r, false); err != nil {
			return err
		}
	}
	return nil
}

type colhido struct {
	detalhe *Detalhe
	custos  []Custo
	// tamanho de cada arquivo, medido sem baixar: url -> (bytes, tipo)
	medidas map[string][2]string
}

// colher busca detalhe, custos e a ficha dos arquivos de cada chamado, em
// paralelo. Devolve também os números que a conta NÃO conhece.
//
// POR QUE CONFERIR `d.ID` CONTRA O NÚMERO PEDIDO
//
//	O nosso `pedir` trata corpo vazio como sucesso — e com razão: em vários
//	endereços do Trílogo, vazio é resposta legítima. Só que a resposta a um
//	chamado que não é desta conta também é vazia. Sem esta conferência, o que
//	volta é um `Detalhe` ZERADO sem erro nenhum, e o caminho de gravação
//	escreveria um chamado de NÚMERO 0 para cada ticket da outra conta. O modo
//	`alvos` consulta as duas contas de propósito, então isso deixaria de ser
//	hipótese e passaria a acontecer em metade das chamadas.
//
// `tolerante` diz o que fazer com quem não respondeu: no caminho normal os
// números vieram da lista da própria conta e faltar é falha; no modo `alvos` é
// a resposta esperada, e o número vai para a conta seguinte.
func (s *Servico) colher(ctx context.Context, sessao *Sessao, numeros []int, tolerante bool) ([]*colhido, []int, error) {
	colhidos := make([]*colhido, len(numeros))
	ausente := make([]bool, len(numeros))
	vagas := make(chan struct{}, s.cfg.Trilogo.Paralelo)
	var espera sync.WaitGroup
	var mu sync.Mutex
	var primeiroErro error

	for i, numero := range numeros {
		espera.Add(1)
		go func(i, numero int) {
			defer espera.Done()
			vagas <- struct{}{}
			defer func() { <-vagas }()

			d, err := sessao.Detalhe(ctx, numero)
			if err == nil && (d == nil || d.ID != numero) {
				err = fmt.Errorf("a conta %s não devolveu este chamado", sessao.Conta)
			}
			if err != nil {
				if tolerante {
					ausente[i] = true
					return
				}
				mu.Lock()
				if primeiroErro == nil {
					primeiroErro = fmt.Errorf("chamado %d: %w", numero, err)
				}
				mu.Unlock()
				return
			}
			c, _ := sessao.Custos(ctx, numero) // sem custo é o caso comum, não é erro

			col := &colhido{detalhe: d, custos: c, medidas: map[string][2]string{}}
			for _, url := range urlsDoChamado(d, c) {
				if n, tipo, err := Medir(ctx, s.http, url); err == nil {
					col.medidas[url] = [2]string{strconv.FormatInt(n, 10), tipo}
				}
			}
			colhidos[i] = col
		}(i, numero)
	}
	espera.Wait()
	if primeiroErro != nil {
		return nil, nil, primeiroErro
	}

	var faltaram []int
	for i, n := range numeros {
		if ausente[i] {
			faltaram = append(faltaram, n)
		}
	}
	return colhidos, faltaram, nil
}

// umLote colhe e grava. Devolve os números que esta conta não conhece.
func (s *Servico) umLote(ctx context.Context, sessao *Sessao, clienteID string, numeros []int, r *Resultado, tolerante bool) ([]int, error) {
	// --- 1) buscar tudo do Trílogo, em paralelo -----------------------------
	colhidos, ausentes, err := s.colher(ctx, sessao, numeros, tolerante)
	if err != nil {
		return nil, err
	}

	// --- 2) unidades --------------------------------------------------------
	unidades, err := s.garantirUnidades(ctx, clienteID, colhidos)
	if err != nil {
		return nil, err
	}

	// --- 3) chamados --------------------------------------------------------
	ids, err := s.gravarChamados(ctx, clienteID, sessao.Conta, colhidos, unidades)
	if err != nil {
		return nil, err
	}
	r.ChamadosGravados += len(ids)

	// --- 4) timeline, custos e fichas de arquivo ----------------------------
	if err := s.gravarFilhos(ctx, colhidos, ids, r); err != nil {
		return nil, err
	}
	return ausentes, nil
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
	// TODAS as linhas do envio precisam ter EXATAMENTE as mesmas chaves.
	//
	// Não é preciosismo meu: o PostgREST recusa o lote inteiro com
	// "All object keys must match" se uma linha tiver um campo que outra não tem.
	// Montar o mapa condicionalmente — "se tem ambiente, acrescenta ambiente" —
	// parece natural e quebra na primeira linha sem ambiente. Já quebrou.
	//
	// Por isso todo campo entra SEMPRE, valendo nulo quando não existe.
	linhas := make([]map[string]any, 0, len(cols))
	for _, c := range cols {
		if c == nil {
			continue
		}
		d := c.detalhe

		var unidadeID, ambiente, ambienteID, tipoPredial any
		var criadoPor, criadoPorID, responsavel any
		if id, ok := unidades[d.Company.ID]; ok {
			unidadeID = id
		}
		if d.Department != nil {
			ambiente = d.Department.FullAddress
			ambienteID = nuloSeZero(d.Department.ID)
		}
		if d.BuildingServiceType != nil {
			tipoPredial = d.BuildingServiceType.Name
		}
		if d.Creator != nil {
			criadoPor = d.Creator.Name
			criadoPorID = nuloSeZero(d.Creator.ID)
		}
		if d.ServiceCompanyAssignee != nil {
			responsavel = d.ServiceCompanyAssignee.Name
		}

		linhas = append(linhas, map[string]any{
			"cliente_id": clienteID,
			"numero":     d.ID,
			"conta":      ContaDe(d.ServiceCompany.Name, conta),
			"unidade_id": unidadeID,
			"descricao":  strings.TrimSpace(d.Description),

			"status_codigo":     d.Status,
			"status":            RotuloStatus(d.Status),
			"prioridade_codigo": d.Priority,
			"prioridade":        RotuloPrioridade(d.Priority),
			"tipo_codigo":       d.Type,
			"tipo":              RotuloTipo(d.Type),

			"natureza":        d.Nature,
			"tipo_predial":    tipoPredial,
			"ambiente":        ambiente,
			"ambiente_id":     ambienteID,
			"criado_por":      criadoPor,
			"criado_por_id":   criadoPorID,
			"responsavel":     responsavel,
			"prestadora":      d.ServiceCompany.Name,
			"prestadora_cnpj": d.ServiceCompany.CNPJ,

			"criado_em":     nulo(d.CreationDateTime),
			"alterado_em":   nulo(d.DateOfLastChange),
			"executado_em":  nulo(d.DateOfLastExecution),
			"vistoriado_em": nulo(d.DateOfLastInspection),
			"concluido_em":  nulo(d.DateOfLastConclusion),
			"prazo":         dataNula(d.DeadlineDate),
			"lido_em":       time.Now().UTC().Format(time.RFC3339),
		})
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
		n, _ := f["tamanho"].(int64)
		r.BytesVistos += n
		if copiar, _ := f["copiar"].(bool); !copiar {
			r.ArquivosSoLink++
			r.BytesSoLink += n
		}
	}
	return nil
}

// ficha monta UMA aparição de arquivo.
//
// Mesma regra dos chamados: todas as chaves em todas as linhas, sempre. Uma foto
// tem autor e não tem custo; um orçamento tem custo e não tem autor — e o
// PostgREST recusa o lote se os dois não tiverem o mesmo formato.
func (s *Servico) ficha(c *colhido, chamadoID, colecao string, idTrilogo int, nome, url, custoID string, extra map[string]any) map[string]any {
	var tamanho, tipo any
	if m, ok := c.medidas[url]; ok {
		if n, err := strconv.ParseInt(m[0], 10, 64); err == nil && n > 0 {
			tamanho = n
		}
		if m[1] != "" {
			tipo = m[1]
		}
	}
	extensao := Extensao(nome)
	f := map[string]any{
		"chamado_id": chamadoID,
		"colecao":    colecao,
		"id_trilogo": idTrilogo,
		"custo_id":   nil,
		"nome":       nome,
		"extensao":   extensao,
		"url_origem": url,
		"tamanho":    tamanho,
		"tipo":       tipo,
		"autor":      nil,
		"autor_id":   nil,
		"quando":     nil,
		"origem":     "trilogo",
		// Falso = fica só o endereço no Trílogo. Sem esta marca, "ainda não
		// copiei" e "nunca vou copiar" seriam a mesma coisa no banco, e a fila de
		// cópia tentaria os vídeos para sempre.
		"copiar": !s.cfg.Trilogo.FicaSoOLink(extensao),
	}
	if custoID != "" {
		f["custo_id"] = custoID
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
	// `copiar=is.true` é o que impede a fila de carregar para sempre os arquivos
	// que o dono decidiu deixar no Trílogo.
	caminho := "chamado_anexos?arquivo_sha256=is.null&copiar=is.true" +
		"&select=id,url_origem,nome,extensao,tipo&limit=" + strconv.Itoa(lote)
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

// fecharAbandonadas marca como interrompidas as rodadas que ficaram penduradas
// em "rodando".
//
// Elas acontecem: o Actions corta por tempo, o Render dorme, a rede cai. A linha
// fica lá para sempre — e, pior, a trava do botão "Atualizar agora" enxerga essa
// linha e recusa leituras novas por trinta minutos. Uma rodada morta não pode
// bloquear as vivas.
func (s *Servico) fecharAbandonadas(ctx context.Context, clienteID string) {
	limite := time.Now().UTC().Add(-JanelaDaRodada).Format(time.RFC3339)
	_ = s.bd.Atualizar(ctx, "robo_execucoes",
		"cliente_id=eq."+banco.Escapar(clienteID)+"&robo=eq.trilogo"+
			"&situacao=eq.rodando&comecou_em=lt."+banco.Escapar(limite),
		map[string]any{
			"situacao":    "interrompida",
			"erro":        "a rodada não foi encerrada; provavelmente o processo morreu antes de terminar",
			"terminou_em": time.Now().UTC().Format(time.RFC3339),
		})
}

func (s *Servico) abrirExecucao(ctx context.Context, modo, clienteID, quem string) (string, error) {
	// Antes de abrir a próxima, enterra as que ficaram penduradas.
	s.fecharAbandonadas(ctx, clienteID)

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
