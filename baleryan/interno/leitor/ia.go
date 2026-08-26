// rev 1 — camada 3: a IA estrutura o que sobrou
//
// SEM SDK
//
//	É uma chamada HTTPS com um JSON dentro. `net/http` e `encoding/json` já vêm
//	com o Go. O SDK oficial traria dezenas de pacotes para economizar as quarenta
//	linhas abaixo — e passaria a decidir, por nós, coisas como tempo-limite e
//	política de repetição. A regra de zero dependência continua de pé.
//
// A CHAVE VAI NO CABEÇALHO, NÃO NA URL
//
//	A API aceita `?key=`. Não usamos: endereço aparece em log de proxy, em
//	mensagem de erro e no histórico de qualquer ferramenta pelo caminho. No
//	cabeçalho, não aparece em nenhum dos três.
//
// O QUE A IA DIZ NÃO É VERDADE ATÉ SER CONFERIDO
//
//	A resposta passa por `Conferir` antes de virar leitura: a chave de acesso tem
//	que fechar no dígito verificador, e a soma dos itens tem que bater com o
//	total declarado. Modelo que inventa item é um problema conhecido; aceitar a
//	resposta porque veio bem formatada é como se produz orçamento errado com
//	aparência de certo.
package leitor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ErrSemIA = errors.New("a leitura por IA não está configurada")

// IA é a camada 3.
type IA struct {
	Chave  string
	Modelo string
	Base   string // variável para o teste apontar para um servidor de mentira
	// Intervalo mínimo entre duas chamadas. Zero desliga a espera — é o que os
	// testes usam, e o que quem tem plano pago pode querer.
	Intervalo time.Duration
	http      *http.Client

	mu     sync.Mutex
	ultima time.Time
}

// ModeloPadrao é o que o dono escolheu: rápido e barato o bastante para rodar em
// toda nota sem pensar duas vezes.
//
// MODELO APOSENTADO NÃO AVISA ANTES, AVISA DEPOIS
//
//	Em 26/08/2026 as cinco chamadas de uma rodada voltaram com "This model
//	models/gemini-2.5-flash is no longer available to new users". Não foi
//	chave, nem cota, nem rede: o modelo foi desligado do lado de lá, e a única
//	notícia disso foi a nota que entrou sem item nenhum.
//
//	Por isso o nome mora AQUI, uma vez só (CORE-25), e `GEMINI_MODELO` existe
//	para trocar de modelo sem subir código no dia em que isso se repetir — que
//	vai se repetir.
const ModeloPadrao = "gemini-3.6-flash"

// IntervaloPadrao é a distância mínima entre duas chamadas.
//
// POR QUE ESPERAR DE PROPÓSITO
//
//	O limite do plano gratuito é por MINUTO, e estourá-lo não devolve "espere":
//	devolve erro. Uma fila de oitenta notas disparada sem pausa gasta a cota em
//	segundos e as setenta restantes falham em sequência — cada uma com três
//	tentativas, cada tentativa queimando mais cota. O robô se afogaria sozinho.
//
//	Sete segundos cabem em qualquer limite de dez por minuto com folga para o
//	relógio do Actions não bater com o do Google. Oitenta notas viram dez
//	minutos de corrida, o que é barato; oitenta notas falhando não têm preço
//	porque não há leitura nenhuma no fim.
//
//	Quando o limite da conta for outro, `GEMINI_INTERVALO` muda isto sem subir
//	código. O número real de cada conta está em aistudio.google.com/rate-limit.
const IntervaloPadrao = 7 * time.Second

func NovaIA(chave string) *IA {
	return &IA{
		Chave:     chave,
		Modelo:    ModeloPadrao,
		Base:      "https://generativelanguage.googleapis.com",
		Intervalo: IntervaloPadrao,
		http:      &http.Client{Timeout: 120 * time.Second},
	}
}

func (ia *IA) Ligada() bool { return ia != nil && ia.Chave != "" }

// esperarAVez segura a chamada até o intervalo mínimo ter passado.
//
// A espera é contada do FIM da chamada anterior, e não do começo: se a leitura
// demorou nove segundos, a próxima sai na hora — o limite é de chamadas por
// minuto, e nove segundos de espera já foram gastos esperando resposta.
func (ia *IA) esperarAVez(ctx context.Context) error {
	ia.mu.Lock()
	espera := time.Duration(0)
	if ia.Intervalo > 0 && !ia.ultima.IsZero() {
		if passou := time.Since(ia.ultima); passou < ia.Intervalo {
			espera = ia.Intervalo - passou
		}
	}
	ia.mu.Unlock()

	if espera <= 0 {
		return nil
	}
	relogio := time.NewTimer(espera)
	defer relogio.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-relogio.C:
		return nil
	}
}

const instrucao = `Você recebe o texto bruto, extraído por OCR, de uma nota fiscal
brasileira (DANFE) ou de um DAV. O texto está torto: colunas desalinhadas,
descrições quebradas em várias linhas, dígitos trocados.

Devolva SOMENTE um JSON com esta forma exata:

{"tipo":"nf|dav","numero":"","serie":"","chave_acesso":"","dav_numero":"",
 "emitente_cnpj":"","emitente_nome":"","destinatario_cnpj":"",
 "emissao":"AAAA-MM-DD","valor_total":0,"valor_frete":0,"observacao":"",
 "itens":[{"codigo":"","descricao":"","unidade":"","quantidade":0,
           "valor_unitario":0,"valor_total":0}]}

Regras:
- Campo que você não encontrar no texto: string vazia ou 0. NUNCA invente.
- chave_acesso são exatamente 44 dígitos, sem espaços. Se não tiver certeza dos
  44, devolva string vazia.
- observacao: copie as informações complementares/observações da nota como estão,
  inteiras. É de lá que sai o número do ticket, então não resuma e não corte.
- valor_total é o valor total DA NOTA.
- Um item por linha de produto. quantidade x valor_unitario deve dar valor_total
  do item.
- Datas sempre AAAA-MM-DD.`

type pedidoIA struct {
	Contents []conteudo `json:"contents"`
	System   *conteudo  `json:"systemInstruction,omitempty"`
	Config   configIA   `json:"generationConfig"`
}

type conteudo struct {
	Parts []parte `json:"parts"`
}

type parte struct {
	Text string `json:"text,omitempty"`
	// O arquivo em si. `omitempty` não é enfeite: mandar `inlineData: null`
	// junto com texto faz a API recusar o pedido inteiro.
	Inline *dadosEmLinha `json:"inlineData,omitempty"`
}

// dadosEmLinha é o arquivo viajando dentro do JSON, em base64.
type dadosEmLinha struct {
	Tipo  string `json:"mimeType"`
	Dados string `json:"data"`
}

type configIA struct {
	// Zero: queremos a leitura mais provável, não a mais criativa. Criatividade
	// aqui é sinônimo de item inventado.
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType"`
}

type respostaIA struct {
	Candidates []struct {
		Content conteudo `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// TamanhoMaximoEmLinha é o teto do arquivo que viaja dentro do JSON.
//
// O base64 engorda os bytes em um terço, e a API recusa pedido acima de ~20 MB.
// Quinze é o maior número redondo que ainda cabe depois de engordado. Acima
// disso existe a API de arquivos, que é outro fluxo — e nenhuma nota deste
// contrato chega perto, então a recusa aqui é aviso, não limitação.
const TamanhoMaximoEmLinha = 15 << 20

// Ler manda o TEXTO e devolve a leitura estruturada.
//
// Continua existindo para o PDF que já traz camada de texto: ali o texto é
// exato, veio do arquivo, e transformá-lo em imagem para o modelo olhar seria
// piorar de propósito o que já estava pronto.
func (ia *IA) Ler(ctx context.Context, texto string) (*Leitura, error) {
	if strings.TrimSpace(texto) == "" {
		return nil, errors.New("não tenho texto para ler")
	}
	return ia.pedir(ctx, []parte{{Text: texto}})
}

// LerArquivo manda o DOCUMENTO e devolve a leitura estruturada.
//
// POR QUE O ARQUIVO, E NÃO O TEXTO DO OCR
//
//	Porque o OCR achata a página. Ele varre linha por linha e cospe uma fita de
//	texto onde a coluna "quantidade" e a coluna "valor unitário" viraram
//	vizinhas de fita — e o modelo, que só recebe a fita, tem que adivinhar onde
//	uma acabou e a outra começou.
//
//	A prova disso está em `LEDS NF 19650`, de 26/08/2026: PDF escaneado, o
//	Tesseract devolveu `LED SPOT SW EM QUAD SLEGI5A JO00R`, e a IA — lendo
//	aquilo — gravou um item com quantidade zero e preço zero numa nota de
//	R$ 112,60. Não foi erro do modelo: ele leu certo um texto que já estava
//	errado.
//
//	O modelo enxerga a página. As colunas continuam colunas, e a decisão de qual
//	número é quantidade e qual é preço deixa de ser adivinhação.
func (ia *IA) LerArquivo(ctx context.Context, tipo string, bruto []byte) (*Leitura, error) {
	if len(bruto) == 0 {
		return nil, errors.New("não tenho arquivo para ler")
	}
	if len(bruto) > TamanhoMaximoEmLinha {
		return nil, fmt.Errorf("o arquivo tem %d MB e o limite para mandar de uma vez é %d MB",
			len(bruto)>>20, TamanhoMaximoEmLinha>>20)
	}
	return ia.pedir(ctx, []parte{
		{Inline: &dadosEmLinha{Tipo: tipo, Dados: base64.StdEncoding.EncodeToString(bruto)}},
		{Text: "Leia este documento e devolva o JSON no formato pedido."},
	})
}

// pedir é a chamada em si — a única do arquivo que fala com a rede.
func (ia *IA) pedir(ctx context.Context, partes []parte) (*Leitura, error) {
	if !ia.Ligada() {
		return nil, ErrSemIA
	}
	if err := ia.esperarAVez(ctx); err != nil {
		return nil, err
	}

	corpo, err := json.Marshal(pedidoIA{
		System:   &conteudo{Parts: []parte{{Text: instrucao}}},
		Contents: []conteudo{{Parts: partes}},
		Config:   configIA{Temperature: 0, ResponseMimeType: "application/json"},
	})
	if err != nil {
		return nil, err
	}

	endereco := ia.Base + "/v1beta/models/" + ia.Modelo + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endereco, bytes.NewReader(corpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", ia.Chave)

	resp, err := ia.http.Do(req)

	// O RELÓGIO COMEÇA AQUI, DÊ NO QUE DER
	//   A chamada que falhou também gastou cota — erro de limite gasta mais,
	//   não menos. Marcar só o sucesso faria o robô responder a uma recusa
	//   disparando de novo na hora, que é exatamente como se transforma um
	//   estouro momentâneo num bloqueio do dia.
	ia.mu.Lock()
	ia.ultima = time.Now()
	ia.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("não consegui falar com a IA: %w", err)
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	var r respostaIA
	if err := json.Unmarshal(bruto, &r); err != nil {
		return nil, fmt.Errorf("a IA respondeu algo que não entendi (%d)", resp.StatusCode)
	}
	if r.Error != nil {
		return nil, fmt.Errorf("a IA recusou: %s", r.Error.Message)
	}
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("a IA respondeu sem conteúdo")
	}

	var l Leitura
	if err := json.Unmarshal([]byte(r.Candidates[0].Content.Parts[0].Text), &l); err != nil {
		return nil, fmt.Errorf("a IA devolveu um JSON que não bate com o esperado: %w", err)
	}
	l.Camada = DaIA
	l.Arrumar()
	if l.Tipo == "" {
		l.Tipo = "nf"
	}
	l.Confianca = Conferir(&l)
	return &l, nil
}

// Conferir avalia a leitura por conta própria e devolve a confiança.
//
// É aqui que a leitura da IA deixa de ser palpite. Três provas, e cada uma vale
// um pedaço:
//
//	a chave fecha no dígito verificador
//	a soma dos itens bate com o total declarado
//	existe pelo menos um item, com quantidade e preço
//
// Se a chave não fecha, ela é APAGADA — melhor não ter chave do que ter uma
// chave errada, que quebraria a trava de duplicidade em silêncio.
func Conferir(l *Leitura) float64 {
	if l == nil {
		return 0
	}
	c := 0.0

	if l.ChaveAcesso != "" {
		if len(l.ChaveAcesso) == 44 && ChaveValida(l.ChaveAcesso) {
			c += 0.35
		} else {
			l.ChaveAcesso = ""
		}
	}

	// O DAV NÃO TEM CHAVE DE ACESSO, E ISSO NÃO É CULPA DELE
	//
	//	A conta acima nasceu pensando em DANFE, onde 0,35 da confiança vem do
	//	dígito verificador da chave. Um DAV do SysPDV não tem chave nenhuma —
	//	então, por perfeito que fosse, ele empacava em 0,65 e a tela dizia
	//	"confira" para sempre, inclusive numa leitura que fecha a própria conta.
	//	Aviso que nunca muda é aviso que ninguém lê.
	//
	//	A identidade dele é outra: o número do documento e o CNPJ de quem
	//	emitiu. Valem menos que um dígito verificador, e por isso valem menos
	//	pontos — mas valem.
	if l.ChaveAcesso == "" && l.Tipo == "dav" {
		if l.DAV != "" {
			c += 0.12
		}
		if len(l.EmitenteCNPJ) == 14 {
			c += 0.08
		}
	}

	if len(l.Itens) > 0 {
		c += 0.20
		var soma float64
		completos := 0
		for _, it := range l.Itens {
			soma += it.Total
			if it.Descricao != "" && it.Quantidade > 0 && it.Unitario > 0 {
				completos++
			}
		}
		if completos == len(l.Itens) {
			c += 0.15
		}
		// A soma dos itens contra o total da nota. Um por cento de tolerância
		// cobre desconto e arredondamento sem deixar passar item inventado.
		if l.ValorTotal > 0 && soma > 0 {
			erro := math.Abs(soma-l.ValorTotal) / l.ValorTotal
			switch {
			case erro <= 0.01:
				c += 0.25
			case erro <= 0.05:
				c += 0.10
			}
		}
	}

	if l.Emissao != "" {
		c += 0.05
	}
	if c > 1 {
		c = 1
	}
	return arredondarConfianca(c)
}

// arredondarConfianca corta o rabo do ponto flutuante.
//
// A soma 0,35 + 0,15 + 0,05 + 0,05 dá 0,6000000000000001 em float64 — o mesmo
// fenômeno que expulsou o ponto flutuante do dinheiro. Aqui ele não faz estrago
// (é um placar, não um valor), mas gravar 0,6000000000000001 numa coluna
// numeric(4,3) é sujeira à toa.
func arredondarConfianca(c float64) float64 {
	return math.Round(c*1000) / 1000
}

// ---------------------------------------------------------------------------
// juntar as camadas
// ---------------------------------------------------------------------------

// Melhor escolhe entre duas leituras da mesma nota, campo a campo.
//
// POR QUE NÃO "FICA A DE MAIOR CONFIANÇA"
//
//	Porque as camadas erram em lugares diferentes. O regex acha a chave de acesso
//	com certeza matemática e não acha item nenhum; a IA acha os itens e às vezes
//	troca um dígito da chave. Escolher uma inteira jogaria fora a parte boa da
//	outra.
func Melhor(a, b *Leitura) *Leitura {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	// A base é a mais confiante; a outra preenche os buracos.
	base, extra := a, b
	if b.Confianca > a.Confianca {
		base, extra = b, a
	}
	saida := *base

	if saida.ChaveAcesso == "" && extra.ChaveAcesso != "" && ChaveValida(extra.ChaveAcesso) {
		saida.ChaveAcesso = extra.ChaveAcesso
	}
	if saida.Numero == "" {
		saida.Numero = extra.Numero
	}
	if saida.Serie == "" {
		saida.Serie = extra.Serie
	}
	if saida.DAV == "" {
		saida.DAV = extra.DAV
	}
	if saida.EmitenteCNPJ == "" {
		saida.EmitenteCNPJ = extra.EmitenteCNPJ
	}
	if saida.EmitenteNome == "" {
		saida.EmitenteNome = extra.EmitenteNome
	}
	if saida.Emissao == "" {
		saida.Emissao = extra.Emissao
	}
	if saida.ValorTotal == 0 {
		saida.ValorTotal = extra.ValorTotal
	}
	if len(saida.Itens) == 0 {
		saida.Itens = extra.Itens
	}
	// A observação mais longa ganha: é de lá que sai o ticket, e cortar
	// observação é perder ticket.
	if len(extra.Observacao) > len(saida.Observacao) {
		saida.Observacao = extra.Observacao
	}
	saida.Confianca = Conferir(&saida)
	return &saida
}
