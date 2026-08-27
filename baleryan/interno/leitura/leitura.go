// Package leitura lê uma nota: baixa o arquivo, interpreta, grava e confere
// duplicidade. rev 1
//
// POR QUE ISTO SAIU DE DENTRO DO `cmd/leitor`
//
//	Toda esta cascata morava em `cmd/leitor/main.go`, num pacote `main`. Pacote
//	`main` não se importa: o motor da web não conseguia chamar nada dali, e a
//	única maneira de uma nota ser lida era esperar a corrida do GitHub — que
//	roda de trinta em trinta minutos, nas horas em que roda.
//
//	Quem insere a nota está OLHANDO para ela, e quer gerar o orçamento agora.
//	Esperar meia hora por um robô que já está pago para dormir é a diferença
//	entre um sistema que responde e um que se explica.
//
//	Decisão do dono em 27/08/2026: "o motor le na hora, opcao A, e tira a
//	leitura de 30 em 30 minutos do git".
//
// O QUE FICOU DE FORA, E POR QUÊ
//
//	A FILA ficou no `cmd/leitor`: tomar trabalho sem duas máquinas pegarem o
//	mesmo, recolher órfão, contar tentativa, desistir depois de três. Isso é
//	assunto de quem roda em lote — o motor lê UMA nota, a que a pessoa pediu, e
//	responde na hora se deu certo.
//
//	Assim as duas pontas leem pelo MESMO código. Duas leituras diferentes para a
//	mesma nota é como nasce a tela que diz uma coisa e o robô que diz outra.
package leitura

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/leitor"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
)

// Servico é a leitura com tudo de que ela precisa: o banco, o armazém e a IA.
type Servico struct {
	bd  *banco.Cliente
	arm *armazem.Cliente
	ia  *leitor.IA
}

func Novo(bd *banco.Cliente, arm *armazem.Cliente, ia *leitor.IA) *Servico {
	return &Servico{bd: bd, arm: arm, ia: ia}
}

// NovoDaConfig monta o serviço com a IA já ajustada pelo ambiente.
//
//	O ajuste — modelo e intervalo entre chamadas — estava escrito dentro do
//	`main` do robô. Com o motor lendo também, duas cópias do mesmo ajuste seriam
//	duas maneiras de a leitura da tela sair diferente da leitura do lote por
//	causa de uma variável que alguém lembrou de ler num lugar e esqueceu no
//	outro.
func NovoDaConfig(cfg *config.Config, bd *banco.Cliente, arm *armazem.Cliente) *Servico {
	ia := leitor.NovaIA(cfg.IA.Chave)
	if cfg.IA.Modelo != "" {
		ia.Modelo = cfg.IA.Modelo
	}
	if cfg.IA.IntervaloSegundos > 0 {
		ia.Intervalo = time.Duration(cfg.IA.IntervaloSegundos) * time.Second
	}
	return &Servico{bd: bd, arm: arm, ia: ia}
}

// FaltaOLeitor diz que este arquivo não tem como ser lido NESTA máquina, e por
// quê. Devolve string vazia quando dá para ler.
//
// POR QUE ISTO EXISTE, E POR QUE A RESPOSTA MUDA DE MÁQUINA PARA MÁQUINA
//
//	As duas pontas rodam a MESMA cascata em lugares diferentes. O robô do GitHub
//	instala o `pdftotext` a cada corrida; o motor roda num contêiner enxuto que
//	não o tem. E a chave da IA pode estar configurada num e não no outro.
//
//	Sem esta pergunta, uma nota inserida num servidor sem chave seria mandada
//	para a cascata, falharia lá dentro e voltaria marcada `falhou` — noventa
//	notas acusadas de um problema que é do servidor, não delas. Perguntar antes
//	custa nada e não deixa marca em nota nenhuma.
//
//	XML nunca precisa de ninguém: é o documento fiscal, não uma interpretação
//	dele. PDF com `pdftotext` à mão PODE ter camada de texto — aí a resposta é
//	"tente", e se não tiver, a cascata diz.
func (s *Servico) FaltaOLeitor(nome string) string {
	ext := strings.ToLower(filepath.Ext(nome))
	if ext == ".xml" {
		return ""
	}
	if s.ia != nil && s.ia.Ligada() {
		return ""
	}
	if ext == ".pdf" {
		if _, err := exec.LookPath("pdftotext"); err == nil {
			return ""
		}
		return "este PDF só pode ser lido pela IA, e esta máquina não tem a chave dela configurada"
	}
	return "este arquivo só pode ser lido pela IA, e esta máquina não tem a chave dela configurada"
}

// IA é para quem precisa DIZER o que está configurado — o robô anuncia o modelo
// na largada. Não é porta para mexer no serviço por fora.
func (s *Servico) IA() *leitor.IA { return s.ia }

// FalhaTemporaria marca o erro que vale tentar de novo.
//
// A DIFERENÇA IMPORTA PARA QUEM CHAMA, NÃO PARA QUEM LÊ
//
//	Baixar, interpretar e gravar podem falhar por rede, por cota da IA, por um
//	minuto ruim do armazém — e a nota continua boa. "Não achei o documento",
//	não: a nota não existe, e tentar de novo é gastar corrida para receber o
//	mesmo não.
//
//	A fila usa isso para escolher entre devolver o trabalho e desistir de vez.
//	O motor usa para escolher entre "tente de novo" e "esta nota tem problema".
//	Sem a marca, as duas pontas teriam de adivinhar pelo texto do erro.
type FalhaTemporaria struct{ Causa error }

func (f FalhaTemporaria) Error() string { return f.Causa.Error() }
func (f FalhaTemporaria) Unwrap() error { return f.Causa }

func temporaria(err error) error { return FalhaTemporaria{Causa: err} }

// Resultado é o que a leitura tem a dizer depois de gravar.
type Resultado struct {
	Nome      string  `json:"nome"`
	Numero    string  `json:"numero"`
	Camada    string  `json:"camada"`
	Confianca float64 `json:"confianca"`
	Itens     int     `json:"itens"`
	Repetida  bool    `json:"repetida"`
}

// Ler faz a cascata inteira para um documento e devolve o que gravou.
//
// A ORDEM NÃO É NEGOCIÁVEL
//
//	documento → baixar → interpretar → gravar → conferir duplicidade.
//
//	A duplicidade vem por último porque antes da leitura a nota não tem chave
//	nem número, e é por eles que se compara. E falha nela NÃO derruba a leitura:
//	a nota está lida e correta; o que falta é um aviso, e aviso que falta é
//	melhor que leitura perdida.
func (s *Servico) Ler(ctx context.Context, documentoID string) (*Resultado, error) {
	doc, err := s.documento(ctx, documentoID)
	if err != nil {
		return nil, err
	}

	bruto, err := s.baixar(ctx, doc.SHA)
	if err != nil {
		return nil, temporaria(err)
	}

	lida, err := s.interpretar(ctx, doc, bruto)
	if err != nil {
		return nil, temporaria(err)
	}

	if err := s.gravar(ctx, doc, lida); err != nil {
		return nil, temporaria(err)
	}

	repetida := false
	if err := s.conferirRepetida(ctx, doc, lida); err != nil {
		log.Printf("documento %s: não consegui conferir duplicidade (%v)", doc.Nome, err)
	} else {
		repetida = s.ficouRepetida(ctx, doc.ID)
	}

	log.Printf("documento %s · %s · camada %s · confiança %.0f%% · %d itens",
		doc.Nome, ouTraco(lida.Numero), lida.Camada, lida.Confianca*100, len(lida.Itens))

	return &Resultado{
		Nome:      doc.Nome,
		Numero:    lida.Numero,
		Camada:    string(lida.Camada),
		Confianca: lida.Confianca,
		Itens:     len(lida.Itens),
		Repetida:  repetida,
	}, nil
}

// ficouRepetida pergunta ao banco se a marca caiu NESTA nota.
//
//	`conferirRepetida` marca quem chegou depois — que pode ser a outra, não
//	esta. Quem está olhando a tela precisa saber se a nota DELE virou cópia, e
//	só o banco sabe disso depois da comparação. Erro aqui vira "não repetida":
//	é um aviso a mais na tela, e a lista mostra a verdade de qualquer jeito.
func (s *Servico) ficouRepetida(ctx context.Context, id string) bool {
	var d []struct {
		Duplicada *string `json:"duplicada_de"`
	}
	if err := s.bd.Buscar(ctx, "documentos?id=eq."+id+"&select=duplicada_de&limit=1", &d); err != nil {
		return false
	}
	return len(d) > 0 && d[0].Duplicada != nil
}

// ---------------------------------------------------------------------------
// a cascata
// ---------------------------------------------------------------------------

// interpretar escolhe quem lê o documento, e devolve a leitura.
func (s *Servico) interpretar(ctx context.Context, doc *documento, bruto []byte) (*leitor.Leitura, error) {
	// CAMADA 1 — o XML é o documento fiscal, não uma interpretação dele.
	if strings.EqualFold(filepath.Ext(doc.Nome), ".xml") {
		return leitor.DoXML(bruto)
	}

	// CAMADA 2 — o PDF que já traz texto de verdade.
	//
	// Um em cada treze PDFs do contrato é digital. Ali o texto veio do arquivo,
	// não de um palpite sobre a imagem dele: se a soma dos itens fechar com o
	// total, a leitura se provou sozinha e não há o que a IA acrescente.
	var doTexto *leitor.Leitura
	if strings.EqualFold(filepath.Ext(doc.Nome), ".pdf") {
		if texto := textoDePDF(ctx, bruto); len(strings.TrimSpace(texto)) >= minimoDeTextoDePDF {
			if doTexto = leitor.DoDAV(texto); doTexto == nil {
				doTexto = leitor.DoTexto(texto)
			}
			doTexto.Camada = leitor.DoTextoCru
			if leitor.ContaFecha(doTexto) {
				log.Printf("documento %s · texto do próprio PDF · a conta fecha (%d itens somam %.2f) — sem IA",
					doc.Nome, len(doTexto.Itens), doTexto.ValorTotal)
				return doTexto, nil
			}
		}
	}

	// CAMADA 3 — o arquivo inteiro vai para o modelo.
	if !s.ia.Ligada() {
		if doTexto != nil {
			log.Printf("documento %s · sem GEMINI_API_KEY — fica só o que o regex achou no texto do PDF", doc.Nome)
			return doTexto, nil
		}
		// Nem XML, nem texto no PDF, nem IA. Não existe leitor para este
		// arquivo, e dizer isso é melhor que gravar uma nota vazia.
		return nil, fmt.Errorf("este arquivo só pode ser lido pela IA, e não há GEMINI_API_KEY configurada")
	}

	daIA, err := s.ia.LerArquivo(ctx, leitor.MimeDoNome(doc.Nome), bruto)
	if err != nil {
		if doTexto != nil {
			log.Printf("documento %s: a IA falhou (%v) — fica só o que o regex achou", doc.Nome, err)
			return doTexto, nil
		}
		return nil, err
	}

	// A TRAVA ARITMÉTICA MUDOU DE POSTO
	//   Antes ela decidia SE a IA seria chamada. Agora confere o que a IA
	//   devolveu — a mesma prova, no outro lado da porta. Item inventado
	//   continua não fechando com o total.
	melhor := leitor.Melhor(doTexto, daIA)
	if leitor.ContaFecha(melhor) {
		log.Printf("documento %s · a conta fecha (%d itens somam %.2f)",
			doc.Nome, len(melhor.Itens), melhor.ValorTotal)
	} else {
		log.Printf("documento %s · A CONTA NÃO FECHA: %d itens somam %.2f e a nota diz %.2f — confira",
			doc.Nome, len(melhor.Itens), leitor.SomaDosItens(melhor), melhor.ValorTotal)
	}
	return melhor, nil
}

// ---------------------------------------------------------------------------
// a nota que já estava aqui
// ---------------------------------------------------------------------------

// candidata é uma nota já lida, para comparar com a que acabou de ser lida.
type candidata struct {
	ID       string  `json:"id"`
	Nome     string  `json:"nome_arquivo"`
	Numero   *string `json:"numero"`
	Chave    *string `json:"chave_acesso"`
	Valor    float64 `json:"valor_total"`
	Inserido string  `json:"inserido_em"`
}

// conferirRepetida marca a nota repetida — ou marca a outra, se a repetida for
// ela.
//
// POR QUE ESTA TRAVA NÃO É O sha256 DO ARQUIVO
//
//	O sha já protege contra subir o MESMO arquivo duas vezes, e isso cobre o
//	dedo escorregando no Explorer. Não cobre o caso real: a mesma nota chegando
//	como arquivo diferente — a foto e o PDF dela, dois escaneamentos, o mesmo
//	PDF renomeado. Bytes diferentes, sha diferente, dois documentos, dois
//	orçamentos, e a loja pagando o mesmo material duas vezes.
//
//	A identidade da NOTA é outra coisa: onde existe chave de acesso são 44
//	dígitos únicos no Brasil inteiro, e onde não existe (DAV) é o par número e
//	valor. É o que `regras.MesmaNota` decide — uma função que existia, tinha
//	teste, e não era chamada de lugar nenhum.
//
// QUEM CHEGOU DEPOIS É A CÓPIA, E NÃO "QUEM EU ESTOU LENDO AGORA"
//
//	A ordem de leitura não é a ordem de chegada. Se as duas cópias entram na
//	mesma rodada e a segunda é lida primeiro, ela não acha a primeira — que
//	ainda não tem chave, porque ainda não foi lida. Depois a primeira é lida,
//	acha a segunda, e se a regra fosse "marco a que estou lendo" ela marcaria a
//	ORIGINAL como cópia da cópia.
//
//	Por isso a comparação é pela data de chegada, e a marca vai em quem chegou
//	depois — seja ela a que está sendo lida ou a outra.
func (s *Servico) conferirRepetida(ctx context.Context, doc *documento, lida *leitor.Leitura) error {
	chave := lida.ChaveAcesso
	numero := lida.Numero
	if chave == "" && numero == "" {
		// Sem chave e sem número não há identidade para comparar. Deixar passar
		// é o certo: inventar duplicidade a partir de valor igual acusaria de
		// cópia duas notas de R$ 14,90 que não têm nada a ver uma com a outra.
		return nil
	}

	filtro := "documentos?cliente_id=eq." + banco.Escapar(doc.Cliente) +
		"&id=neq." + doc.ID + "&oculto_em=is.null&duplicada_de=is.null" +
		"&select=id,nome_arquivo,numero,chave_acesso,valor_total,inserido_em" +
		"&" + ouEntao(chave, numero)

	var achadas []candidata
	if err := s.bd.Buscar(ctx, filtro, &achadas); err != nil {
		return err
	}

	valor := regras.DinheiroDe(lida.ValorTotal)
	for _, c := range achadas {
		if !regras.MesmaNota(chave, numero, valor,
			texto(c.Chave), texto(c.Numero), regras.DinheiroDe(c.Valor)) {
			continue
		}
		copia, original := doc.ID, c.ID
		nomeCopia, nomeOriginal := doc.Nome, c.Nome
		if c.Inserido > doc.Inserido {
			// A outra chegou depois: a cópia é ela.
			copia, original = c.ID, doc.ID
			nomeCopia, nomeOriginal = c.Nome, doc.Nome
		}
		if err := s.bd.Atualizar(ctx, "documentos", "id=eq."+copia,
			map[string]any{"duplicada_de": original}); err != nil {
			return err
		}
		log.Printf("documento %s · é a MESMA nota que %s — marcada como repetida, não vai gerar orçamento",
			nomeCopia, nomeOriginal)
		return nil
	}
	return nil
}

// ouEntao monta a busca das candidatas: por chave OU por número.
//
// O DEFEITO QUE ISTO CONSERTA — R$ 854,40 COBRADOS DUAS VEZES
//
//	Era assim, e a assimetria custou dinheiro:
//
//	  if chave != "" { filtro += "&chave_acesso=eq." + chave }
//	  else           { filtro += "&numero=eq." + numero }
//
//	`MesmaNota` é simétrica e generosa: faltando a chave de um dos lados, o
//	número basta. Mas a BUSCA acima era exclusiva — a nota que tinha chave
//	procurava SÓ por chave, e a cópia sem chave nunca entrava na lista de
//	candidatas. `MesmaNota` nunca chegava a ser chamada para aquele par.
//
//	Medido em 27/08/2026, e é o caso exato: a NF 17936 entrou duas vezes, com 3
//	segundos de diferença. `CCF17082026_0006.pdf` sem chave (a IA não leu os 44
//	dígitos naquele scan) e `nf frota macedo - 10.08_p3.pdf` com chave.
//
//	  · a sem chave foi lida primeiro, procurou por `numero=17936`, e a outra
//	    ainda não tinha número gravado — não achou;
//	  · a com chave foi lida depois, procurou por `chave_acesso=...`, e a
//	    primeira não tem chave — não achou.
//
//	Nenhuma das duas encontrou a outra. As duas viraram orçamento, R$ 600,00
//	cada, no MESMO ticket 126342. E a NF 17937 repetiu a história no 112449.
//
// A REGRA MORA EM `MesmaNota`, E A BUSCA SÓ PRECISA ENTREGAR OS CANDIDATOS
//
//	É a divisão de trabalho certa: a consulta pesca largo — quem tem esta chave
//	OU este número — e o julgamento fica num só lugar, em Go, com teste. Uma
//	busca que já decide é uma segunda regra escondida, e foi ela que discordou
//	da primeira.
func ouEntao(chave, numero string) string {
	var partes []string
	if chave != "" {
		partes = append(partes, "chave_acesso.eq."+banco.Escapar(chave))
	}
	if numero != "" {
		partes = append(partes, "numero.eq."+banco.Escapar(numero))
	}
	return "or=(" + strings.Join(partes, ",") + ")"
}

func texto(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ouTraco(s string) string {
	if s == "" {
		return "sem número"
	}
	return "nº " + s
}

type documento struct {
	ID   string `json:"id"`
	Nome string `json:"nome_arquivo"`
	SHA  string `json:"arquivo_sha256"`
	// Qual das duas entradas: `orcamento` ou `rateio`. Muda quem manda no
	// ticket — veja `amarraSozinho`.
	Fila string `json:"fila"`
	// Quando a nota chegou. É o desempate da duplicidade: quem chegou depois é
	// a cópia.
	Inserido string `json:"inserido_em"`
	Cliente  string `json:"cliente_id"`
}

func (s *Servico) documento(ctx context.Context, id string) (*documento, error) {
	var d []documento
	if err := s.bd.Buscar(ctx, "documentos?id=eq."+id+
		"&select=id,nome_arquivo,arquivo_sha256,fila,inserido_em,cliente_id&limit=1", &d); err != nil {
		return nil, err
	}
	if len(d) == 0 {
		return nil, fmt.Errorf("não achei o documento %s", id)
	}
	if d[0].SHA == "" {
		return nil, fmt.Errorf("o documento %s não tem arquivo guardado", id)
	}
	return &d[0], nil
}

// baixar traz o arquivo do armazém pelo endereço assinado.
func (s *Servico) baixar(ctx context.Context, sha string) ([]byte, error) {
	var arq []struct {
		Chave string `json:"chave_r2"`
	}
	if err := s.bd.Buscar(ctx, "arquivos?sha256=eq."+sha+"&select=chave_r2&limit=1", &arq); err != nil {
		return nil, err
	}
	if len(arq) == 0 {
		return nil, fmt.Errorf("o arquivo %s não está registrado", sha[:8])
	}
	url, err := s.arm.LinkTemporario(arq[0].Chave, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("o armazém respondeu %d ao baixar o arquivo", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func (s *Servico) gravar(ctx context.Context, doc *documento, lida *leitor.Leitura) error {
	bruto, _ := json.Marshal(lida)
	campos := map[string]any{
		"status":            "lido",
		"tipo":              vazioVira(lida.Tipo, "nf"),
		"numero":            nuloSeVazio(lida.Numero),
		"serie":             nuloSeVazio(lida.Serie),
		"chave_acesso":      nuloSeVazio(lida.ChaveAcesso),
		"dav_numero":        nuloSeVazio(lida.DAV),
		"emitente_cnpj":     nuloSeVazio(lida.EmitenteCNPJ),
		"emitente_nome":     nuloSeVazio(lida.EmitenteNome),
		"destinatario_cnpj": nuloSeVazio(lida.DestinatarioCNPJ),
		"emissao":           nuloSeVazio(lida.Emissao),
		"valor_total":       lida.ValorTotal,
		"valor_frete":       lida.ValorFrete,
		"observacao":        nuloSeVazio(lida.Observacao),
		"leitura_camada":    string(lida.Camada),
		"leitura_confianca": lida.Confianca,
		"leitura_bruta":     json.RawMessage(bruto),
		"leitura_erro":      nil,
	}
	if err := s.bd.Atualizar(ctx, "documentos", "id=eq."+doc.ID, campos); err != nil {
		return err
	}

	if len(lida.Itens) > 0 {
		linhas := make([]map[string]any, 0, len(lida.Itens))
		for i, it := range lida.Itens {
			linhas = append(linhas, map[string]any{
				"documento_id":   doc.ID,
				"ordem":          i + 1,
				"codigo":         nuloSeVazio(it.Codigo),
				"descricao":      it.Descricao,
				"unidade":        nuloSeVazio(it.Unidade),
				"quantidade":     it.Quantidade,
				"valor_unitario": it.Unitario,
				"valor_total":    it.Total,
			})
		}
		if err := s.bd.Upsert(ctx, "documento_itens?on_conflict=documento_id,ordem", linhas, nil); err != nil {
			return err
		}
	}

	return s.amarrarOTicket(ctx, doc, lida)
}

// amarrarOTicket liga a nota ao chamado — ou deixa de propósito sem ticket.
//
// TRÊS CONDIÇÕES, E TODAS PRECISAM VALER
//
//  1. o número saiu do CAMPO de observação, não de um lugar qualquer da página
//  2. o campo tem UM número de ticket, não dois nem nenhum
//  3. esse número existe na nossa base de chamados
//
// Falhando qualquer uma, a nota entra SEM ticket e vai para a fila de quem
// digita o número na mão. É trabalho — e é muito mais barato que o contrário:
// um ticket errado não trava nada, ele gera, lança e cobra a loja errada, e
// ninguém descobre.
//
// Medido em 35 documentos reais: o campo de observação foi lido em 29 deles, e
// os 29 tickets estavam CERTOS. Nenhum ticket errado. Os 6 que ficaram de fora
// caem na fila manual, que é exatamente onde deveriam cair.
func (s *Servico) amarrarOTicket(ctx context.Context, doc *documento, lida *leitor.Leitura) error {
	ticket, pode, porque := amarraSozinho(doc.Fila, lida)
	if !pode {
		log.Printf("documento %s · %s (campo=%v, observação=%q) — vai para SEM TICKET",
			doc.Nome, porque, lida.ObservacaoDoCampo, encurtar(lida.Observacao, 60))
		return nil
	}
	return s.amarrarTickets(ctx, doc.ID, []int{ticket})
}

// FilaRateio é a entrada em que o ticket NÃO sai da nota.
const FilaRateio = "rateio"

// amarraSozinho responde se o robô pode amarrar o ticket que leu.
//
// A FILA DE RATEIO É O CONTRÁRIO DA OUTRA
//
//	Na fila `orcamento`, a nota traz o próprio ticket na observação e ler esse
//	número é o serviço. Na fila `rateio`, a nota foi separada JUSTAMENTE porque
//	o material dela se divide entre vários chamados — quem dita os tickets é o
//	usuário, na tela.
//
//	O leitor não sabia disso: rodava a mesma regra nas duas. Uma nota de rateio
//	com um ticket escrito na observação seria amarrada sozinha, e aí o custo
//	inteiro cairia numa loja só — o rateio cancelado antes de alguém abrir a
//	tela para fazê-lo. E sem travar nada: gera, lança e cobra a loja errada, em
//	silêncio. É o mesmo modo de falha que a regra dos três critérios existe para
//	impedir, entrando por uma porta que ninguém tinha olhado.
//
//	Em 26/08/2026 as duas notas de rateio da fila escaparam por sorte: o campo
//	de observação não foi lido, porque a IA estava fora do ar. Sorte não é
//	regra.
//
// É função à parte, e não um `if` dentro do método, porque a decisão é o que
// importa aqui — e decisão que não dá para testar sem subir um banco não é
// testada (P-30).
func amarraSozinho(fila string, lida *leitor.Leitura) (int, bool, string) {
	if fila == FilaRateio {
		return 0, false, "é da fila de rateio, onde quem dita o ticket é o usuário"
	}
	ticket, confiavel := leitor.TicketConfiavel(lida)
	if !confiavel {
		return 0, false, "ticket não confiável"
	}
	return ticket, true, ""
}

func (s *Servico) amarrarTickets(ctx context.Context, documentoID string, tickets []int) error {
	numeros := make([]string, 0, len(tickets))
	for _, t := range tickets {
		numeros = append(numeros, fmt.Sprint(t))
	}
	var achados []struct {
		ID     string `json:"id"`
		Numero int    `json:"numero"`
	}
	if err := s.bd.Buscar(ctx, "chamados?numero=in.("+strings.Join(numeros, ",")+
		")&select=id,numero", &achados); err != nil {
		return err
	}
	naBase := map[int]string{}
	for _, a := range achados {
		naBase[a.Numero] = a.ID
	}

	linhas := make([]map[string]any, 0, len(tickets))
	for _, t := range tickets {
		linha := map[string]any{"documento_id": documentoID, "ticket": t, "chamado_id": nil}
		if id, tem := naBase[t]; tem {
			linha["chamado_id"] = id
		}
		linhas = append(linhas, linha)
	}
	return s.bd.Upsert(ctx, "documento_tickets?on_conflict=documento_id,ticket", linhas, nil)
}

func nuloSeVazio(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func vazioVira(s, padrao string) string {
	if s == "" {
		return padrao
	}
	return s
}

// ---------------------------------------------------------------------------
// OCR
// ---------------------------------------------------------------------------

// extrair transforma o arquivo em texto, pelo caminho mais barato que servir.
//
// A ORDEM É A DO CUSTO, E TAMBÉM A DA QUALIDADE
//
//	texto do PDF     exato e instantâneo. Quando existe, não há o que melhorar.
//	imagem + OCR     tudo o mais. E aqui a imagem passa antes pelo otimizador.
//
// minimoDeTextoDePDF separa o PDF digital do escaneado.
//
// Não é zero de propósito: PDF de imagem às vezes traz um punhado de caracteres
// de metadado ou uma marca d'água em texto, e tratá-lo como digital devolveria
// três palavras no lugar da nota inteira.
const minimoDeTextoDePDF = 400

// textoDePDF devolve a camada de texto do arquivo, ou string vazia.
//
// Vazio aqui é resposta, não erro: PDF escaneado não TEM camada de texto, e é
// exatamente o caso normal. Quem chama decide o que fazer com o silêncio.
func textoDePDF(ctx context.Context, bruto []byte) string {
	pasta, err := os.MkdirTemp("", "leitor")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(pasta)

	caminho := filepath.Join(pasta, "nota.pdf")
	if err := os.WriteFile(caminho, bruto, 0o600); err != nil {
		return ""
	}
	fora, err := exec.CommandContext(ctx, "pdftotext", caminho, "-").Output()
	if err != nil {
		return ""
	}
	return string(fora)
}

func encurtar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
