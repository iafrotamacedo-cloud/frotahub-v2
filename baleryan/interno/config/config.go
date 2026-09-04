// rev 4 — configuração do baleryan
//
// Toda variável de ambiente que o motor usa é declarada AQUI, com tipo, valor padrão
// (quando faz sentido ter um) e uma linha dizendo para que serve. Não existe leitura
// de variável de ambiente espalhada pelo resto do código.
//
// Duas escolhas deste arquivo:
//
//  1. Se faltar variável obrigatória, o motor NÃO SOBE — e reclama de TODAS de uma vez,
//     em português, não de uma por vez. Falha na largada é barata; falha no meio do
//     expediente, não.
//
//  2. Nenhum parâmetro de NEGÓCIO mora aqui. A margem, o teto de lançamento e o limite
//     de extrapolado ficam na tabela `parametros`, com histórico de quem mudou e quando.
//     Variável de ambiente é para infraestrutura — endereço, chave, credencial.
//     Regra da empresa precisa deixar rastro.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// ErroDeConfiguracao é devolvido por Carregar quando a configuração está incompleta.
type ErroDeConfiguracao struct {
	Problemas []string
}

func (e *ErroDeConfiguracao) Error() string {
	return "O motor não pode subir. Corrija a configuração:\n  - " +
		strings.Join(e.Problemas, "\n  - ")
}

// ---------------------------------------------------------------------------
// leitor: acumula problemas em vez de estourar no primeiro
// ---------------------------------------------------------------------------

type leitor struct{ problemas []string }

func (l *leitor) bruto(nome string) string {
	return strings.TrimSpace(os.Getenv(nome))
}

func (l *leitor) problema(f string, a ...any) {
	l.problemas = append(l.problemas, fmt.Sprintf(f, a...))
}

func (l *leitor) texto(nome, padrao string, obrigatorio bool, paraQue string) string {
	v := l.bruto(nome)
	if v == "" {
		if obrigatorio {
			l.problema("%s — faltando. %s", nome, paraQue)
		}
		return padrao
	}
	return v
}

func (l *leitor) url(nome, padrao string, obrigatorio bool, paraQue string) string {
	v := l.texto(nome, padrao, obrigatorio, paraQue)
	if v != "" && !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
		l.problema("%s — precisa começar com http:// ou https:// (veio %q).", nome, v)
		return ""
	}
	return strings.TrimRight(v, "/")
}

func (l *leitor) segredo(nome string, obrigatorio bool, minimo int, paraQue string) string {
	v := l.texto(nome, "", obrigatorio, paraQue)
	if v != "" && minimo > 0 && len(v) < minimo {
		l.problema("%s — curta demais: precisa de pelo menos %d caracteres.", nome, minimo)
	}
	return v
}

func (l *leitor) inteiro(nome string, padrao, minimo, maximo int) int {
	v := l.bruto(nome)
	if v == "" {
		return padrao
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.problema("%s — precisa ser um número inteiro (veio %q).", nome, v)
		return padrao
	}
	if n < minimo || n > maximo {
		l.problema("%s — precisa ficar entre %d e %d (veio %d).", nome, minimo, maximo, n)
		return padrao
	}
	return n
}

func (l *leitor) lista(nome string, padrao []string) []string {
	v := l.bruto(nome)
	if v == "" {
		return padrao
	}
	var saida []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			saida = append(saida, p)
		}
	}
	return saida
}

// numeros lê uma lista de números inteiros positivos.
//
// SEPARADOR GENEROSO, VALOR RIGOROSO
//
//	Esta lista chega COLADA à mão, de uma planilha ou de uma consulta ao banco —
//	ou seja, com vírgula, com espaço, com quebra de linha, e às vezes com os
//	três. Aceitar qualquer um deles como separador não custa nada e evita a
//	rodada que falha porque sobrou um "\n" no fim.
//
//	O que NÃO é generoso é o conteúdo: pedaço que não seja um inteiro positivo
//	vira problema de configuração e o programa não sobe. É de propósito. Um
//	número de chamado digitado errado leria o chamado errado — e no modo `alvos`
//	essa lista é a única coisa que decide o que vai ser gravado. Melhor recusar
//	a lista inteira do que ler um chamado que ninguém pediu.
//
//	Repetido entra uma vez só: pedir duas vezes o mesmo chamado seria trabalho
//	dobrado sem efeito nenhum.
func (l *leitor) numeros(nome string) []int {
	v := l.bruto(nome)
	if v == "" {
		return nil
	}
	partes := strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})
	vistos := map[int]bool{}
	saida := make([]int, 0, len(partes))
	for _, p := range partes {
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			l.problema("%s — %q não é um número de chamado.", nome, p)
			continue
		}
		if vistos[n] {
			continue
		}
		vistos[n] = true
		saida = append(saida, n)
	}
	return saida
}

// ---------------------------------------------------------------------------
// grupos
// ---------------------------------------------------------------------------

// Supabase é o banco e o login.
type Supabase struct {
	URL          string
	ChaveServico string // service_role — só o motor conhece, nunca chega ao navegador
	ChavePublica string // usada apenas para validar o token do usuário
}

func (s Supabase) REST() string { return s.URL + "/rest/v1" }
func (s Supabase) Auth() string { return s.URL + "/auth/v1" }

// R2 guarda os arquivos do fluxo vivo.
type R2 struct {
	ContaID        string
	Bucket         string
	ChaveID        string
	ChaveSecreta   string
	DiasLixeira    int
	ValidadeURLseg int
}

func (r R2) Ligado() bool {
	return r.ContaID != "" && r.Bucket != "" && r.ChaveID != "" && r.ChaveSecreta != ""
}

func (r R2) Endpoint() string {
	return "https://" + r.ContaID + ".r2.cloudflarestorage.com"
}

// Trilogo é o sistema do cliente de onde os chamados são lidos.
//
// São DUAS contas, e o robô lê as duas. Não existe conta única com visão das
// duas: o Trílogo separa por empresa prestadora.
type Trilogo struct {
	EmailInstalacoes string
	SenhaInstalacoes string
	EmailCivil       string
	SenhaCivil       string
	// Quantos chamados o robô processa ao mesmo tempo. Mais que isso não acelera
	// (o gargalo passa a ser o Trílogo) e começa a parecer abuso.
	Paralelo int
	// As extensões que NÃO vão para o armazém — fica só o endereço no Trílogo.
	//
	// É lista, e não regra fixa no código, porque isto é decisão de negócio e
	// muda com o tamanho da conta (P-08). Hoje são os vídeos: 86% do peso do
	// acervo em 13% dos arquivos.
	SoLink []string
	// Quantos chamados por lote na atualização. O motor do plano gratuito pode
	// adormecer no meio; lote pequeno significa perder pouco e retomar rápido.
	Lote int

	// Os chamados que o modo `alvos` vai buscar, pelo número.
	//
	// POR QUE É CONFIGURAÇÃO, E NÃO UMA CONSULTA AO BANCO
	//
	//	Seria tentador o robô descobrir sozinho quais chamados faltam. Mas então
	//	a lista do que ele grava passaria a depender de uma consulta que pode
	//	mudar entre o momento em que se decide rodar e o momento em que se roda.
	//	Aqui a lista é DITADA, escrita à mão no disparo, e fica registrada no log
	//	do Actions: dá para conferir depois exatamente o que foi pedido.
	//
	//	Vazia em toda rodada que não seja `alvos` — e o modo `alvos` recusa
	//	rodar sem ela.
	Alvos []int

	// O id da empresa prestadora DENTRO do Trílogo, que o lançamento de custo
	// precisa mandar em `CompanyId`.
	//
	// POR QUE NÃO É CONSTANTE NO CÓDIGO
	//
	//	O 35 de Instalações foi levantado capturando a rede em 25/08/2026. O da
	//	Civil ainda não foi — e um número errado aqui lança o custo na empresa
	//	errada, dentro do sistema do cliente. Preferimos RECUSAR o lançamento a
	//	chutar: sem o id, a rota responde com a frase que diz o que falta.
	EmpresaInstalacoes int
	EmpresaCivil       int

	// O id do FORNECEDOR (supplier) dentro do Trílogo, que a criação de
	// orçamento de Serviço precisa mandar em `supplierId`.
	//
	// OUTRO ESPAÇO DE NÚMEROS — não confundir com Empresa*: aquele é o
	// `CompanyId` do CreateTicketCost (ciclo de Materiais do contrato); este é
	// o `supplierId` do CreateBudget (ciclo de Serviço). Descobertos em datas
	// e telas diferentes, e nada garante que coincidam.
	//
	// O 23927 de Instalações foi levantado em 04/09/2026, criando e apagando
	// um orçamento de teste (R$0,01) no ticket 135613. O da Civil nasce zero,
	// mesma regra do EmpresaCivil: zero é "não sei", e "não sei" recusa.
	FornecedorInstalacoes int
	FornecedorCivil       int

	// Os dois valores especiais de "Responsável Executante" que tiram um
	// chamado da fila de manutenção do contrato e colocam na fila de Serviço.
	//
	// NENHUM DOS DOIS SE CHAMA "Serviço X" DENTRO DO TRÍLOGO
	//
	//	O de Instalações (67277) é o único com o nome literal "Serviço
	//	Instalações". O de Civil (67264) é uma PESSOA real cadastrada como
	//	responsável — Romario Santos, encarregado do setor — não existe
	//	nenhum "Serviço Civil" de verdade lá dentro. A correção veio do dono
	//	em 04/09/2026, depois do primeiro levantamento ter concluído (errado)
	//	que Civil não existia.
	//
	//	O NOSSO sistema trata os dois IDs do mesmo jeito (gatilho de entrada
	//	na fila de Serviço) e o FRONT SEMPRE mostra "Serviço Civil" pro id
	//	67264 — nunca o nome "Romario Santos" cru, que confundiria quem
	//	opera. Ver trilogo.RotuloResponsavelServico.
	ResponsavelServicoInstalacoes int
	ResponsavelServicoCivil       int

	// O NOME CRU, exatamente como o Trílogo devolve e como a leitura do
	// robô grava em `chamados.responsavel` (robo.go: `d.ServiceCompanyAssignee.Name`
	// — a API de leitura do chamado só traz o NOME, nunca o id). É por aqui,
	// não pelos ids acima, que o job de Candidatos (cmd/servicos) reconhece
	// que um chamado já lido entrou na fila de Serviço: comparando texto com
	// `chamados.responsavel`, não chamando o Trílogo de novo.
	NomeResponsavelServicoInstalacoes string
	NomeResponsavelServicoCivil       string
}

// EmpresaDaConta devolve o id da empresa prestadora, ou zero se não sabemos.
func (t Trilogo) EmpresaDaConta(conta string) int {
	if conta == "civil" {
		return t.EmpresaCivil
	}
	return t.EmpresaInstalacoes
}

// FornecedorDaConta devolve o supplierId para orçamento de Serviço, ou zero
// se não sabemos (hoje, sempre zero para "civil").
func (t Trilogo) FornecedorDaConta(conta string) int {
	if conta == "civil" {
		return t.FornecedorCivil
	}
	return t.FornecedorInstalacoes
}

// ResponsavelServicoDaConta devolve o id do responsável especial de Serviço,
// ou zero se não sabemos (hoje, sempre zero para "civil").
func (t Trilogo) ResponsavelServicoDaConta(conta string) int {
	if conta == "civil" {
		return t.ResponsavelServicoCivil
	}
	return t.ResponsavelServicoInstalacoes
}

// ContaDoNomeResponsavelServico devolve qual conta aquele NOME CRU de
// responsável representa, ou "" se o texto não bate com nenhum dos dois
// configurados — é a metade "detectar" do gatilho automático (a metade
// "agir" é MudarResponsavel, em trilogo/servico.go).
func (t Trilogo) ContaDoNomeResponsavelServico(nomeCru string) string {
	if nomeCru == "" {
		return ""
	}
	switch nomeCru {
	case t.NomeResponsavelServicoInstalacoes:
		return "instalacoes"
	case t.NomeResponsavelServicoCivil:
		return "civil"
	default:
		return ""
	}
}

// SoLinkPadrao é a lista que vale quando ninguém disse outra coisa.
//
// Mora aqui, e não escondida dentro da leitura do ambiente, para que os testes
// usem EXATAMENTE a mesma lista da produção. Duas listas iguais escritas em dois
// lugares viram duas listas diferentes na primeira vez que alguém mexe numa.
var SoLinkPadrao = []string{"mp4", "mov", "avi", "3gp", "m4v", "mkv", "wmv", "webm"}

// FicaSoOLink diz se aquele tipo de arquivo não vem para o nosso armazém.
func (t Trilogo) FicaSoOLink(extensao string) bool {
	e := strings.ToLower(strings.TrimPrefix(extensao, "."))
	for _, x := range t.SoLink {
		if e == strings.ToLower(x) {
			return true
		}
	}
	return false
}

func (t Trilogo) Ligado() bool {
	return t.EmailInstalacoes != "" && t.SenhaInstalacoes != "" &&
		t.EmailCivil != "" && t.SenhaCivil != ""
}

// Contas devolve as duas na ordem em que são lidas.
func (t Trilogo) Contas() []Conta {
	return []Conta{
		{Nome: "instalacoes", Email: t.EmailInstalacoes, Senha: t.SenhaInstalacoes},
		{Nome: "civil", Email: t.EmailCivil, Senha: t.SenhaCivil},
	}
}

type Conta struct {
	Nome  string // 'instalacoes' | 'civil'
	Email string
	Senha string
}

// O DROPBOX SAIU DO STACK EM 25/08/2026.
//
// Ele era o arquivo-mestre — as pastas 10 e 11, a rede de segurança que uma vez
// recuperou 15 orçamentos perdidos. Quem assume esse papel agora é o R2, por
// duas escolhas que juntas fazem o mesmo trabalho e um pouco mais:
//
//	o endereço do arquivo é o sha256 do conteúdo, então duplicar é impossível;
//	nada é apagado do R2 — "excluir" preenche `oculto_em` no banco e pronto.
//
// A consequência prática é que o "desfazer" da tela é um update, e não um
// resgate de lixeira de terceiro. Nenhuma variável DROPBOX_* é lida.

// IA é a terceira camada da leitura de nota fiscal.
//
// A primeira é o XML (exato, de graça); a segunda é o OCR com expressão regular
// (pega a chave de acesso e o valor); a terceira estrutura o resto. As duas
// primeiras não precisam de configuração nenhuma.
type IA struct {
	Chave  string
	Modelo string
	// Segundos entre duas chamadas. Zero = usar o padrão do pacote `leitor`.
	IntervaloSegundos int
}

func (i IA) Ligada() bool { return i.Chave != "" }

// Groq classifica candidato a Serviço (Manutenção › Serviços › Candidatos).
//
// POR QUE NÃO É O MESMO `IA` DO GEMINI
//
//	São dois provedores DIFERENTES, com chave e cota diferentes — e é
//	proposital: a leitura de nota já usa toda a cota do Gemini, e classificar
//	até 200 chamados/dia por cima dela estourava o limite. O Groq tem um
//	tier gratuito de verdade (14.400 requisições/dia, reseta todo dia, sem
//	cartão) — levantado em 03/09/2026.
type Groq struct {
	Chave  string
	Modelo string
	// Quantos chamados o job de Candidatos classifica numa rodada. Existe
	// como teto de segurança, não porque a cota aperte (200/dia cabe folgado
	// em 14.400) — é para um pico de chamados num único dia não virar uma
	// rodada de horas.
	LimitePorRodada int
}

func (g Groq) Ligado() bool { return g.Chave != "" }

// ModeloGroqPadrao é o modelo leve — rápido e mais que suficiente para uma
// classificação binária de uma frase.
const ModeloGroqPadrao = "llama-3.1-8b-instant"

// Runtime é como o processo roda.
type Runtime struct {
	Porta       int
	FusoHoras   int // Fortaleza = -3; o servidor roda em UTC
	Ambiente    string
	OrigensCORS []string
}

func (r Runtime) Producao() bool { return r.Ambiente == "producao" }

// Config é tudo junto.
type Config struct {
	Supabase Supabase
	R2       R2
	Trilogo  Trilogo
	IA       IA
	Groq     Groq
	Runtime  Runtime
	// 'motor' ou 'robo'. Muda o que é obrigatório, e nada mais.
	Papel     string
	PinPepper string
	ChaveRobo string

	// Domínio usado para transformar um usuário curto ("builder") num e-mail, que
	// é o que o Supabase espera. Ninguém digita e-mail para entrar no FrotaHub.
	DominioLogin string
}

// Resumo é o que o /saude mostra: diz o que está ligado SEM revelar nenhuma chave.
func (c Config) Resumo() map[string]any {
	return map[string]any{
		"ambiente": c.Runtime.Ambiente,
		"supabase": c.Supabase.URL != "",
		"r2":       c.R2.Ligado(),
		"ia":       c.IA.Ligada(),
		"groq":     c.Groq.Ligado(),
		"trilogo":  c.Trilogo.Ligado(),
		"robos":    c.ChaveRobo != "",
	}
}

// ---------------------------------------------------------------------------

// Carregar lê o ambiente e devolve a configuração, ou um erro listando tudo que falta.
func Carregar() (*Config, error) {
	l := &leitor{}

	sb := Supabase{
		URL: l.url("SUPABASE_URL", "", true,
			"Endereço do projeto Supabase — sem ele não há banco nem login."),
		ChaveServico: l.segredo("SUPABASE_SERVICE_KEY", true, 40,
			"Chave service_role. Supabase > Project Settings > API."),
		ChavePublica: l.segredo("SUPABASE_ANON_KEY", os.Getenv("PAPEL") != "robo", 40,
			"Chave pública (anon/publishable), usada só para validar o token do usuário."),
	}

	r2 := R2{
		ContaID:        l.texto("R2_ACCOUNT_ID", "", false, ""),
		Bucket:         l.texto("R2_BUCKET", "frotahub", false, ""),
		ChaveID:        l.texto("R2_ACCESS_KEY_ID", "", false, ""),
		ChaveSecreta:   l.segredo("R2_SECRET_ACCESS_KEY", false, 0, ""),
		DiasLixeira:    l.inteiro("R2_DIAS_LIXEIRA", 30, 1, 365),
		ValidadeURLseg: l.inteiro("R2_VALIDADE_URL_SEG", 3600, 60, 86400),
	}

	tri := Trilogo{
		EmailInstalacoes: l.texto("TRILOGO_EMAIL_INSTALACOES", "", false, ""),
		SenhaInstalacoes: l.segredo("TRILOGO_SENHA_INSTALACOES", false, 0, ""),
		EmailCivil:       l.texto("TRILOGO_EMAIL_CIVIL", "", false, ""),
		SenhaCivil:       l.segredo("TRILOGO_SENHA_CIVIL", false, 0, ""),
		SoLink:           l.lista("TRILOGO_SO_LINK", SoLinkPadrao),
		Paralelo:         l.inteiro("TRILOGO_PARALELO", 12, 1, 32),
		Lote:             l.inteiro("TRILOGO_LOTE", 150, 10, 1000),
		Alvos:            l.numeros("TRILOGO_ALVOS"),
		// 35 é o valor levantado da conta Instalações. O da Civil nasce zero de
		// propósito: zero é "não sei", e "não sei" recusa o lançamento.
		EmpresaInstalacoes: l.inteiro("TRILOGO_EMPRESA_INSTALACOES", 35, 0, 1<<30),
		EmpresaCivil:       l.inteiro("TRILOGO_EMPRESA_CIVIL", 0, 0, 1<<30),

		FornecedorInstalacoes: l.inteiro("TRILOGO_FORNECEDOR_INSTALACOES", 23927, 0, 1<<30),
		FornecedorCivil:       l.inteiro("TRILOGO_FORNECEDOR_CIVIL", 0, 0, 1<<30),

		// 67277 = "Serviço Instalações" (nome literal). 67264 = Romario
		// Santos, encarregado do setor Civil — é ELE que faz o papel de
		// "Serviço Civil" do lado de lá; o rótulo bonito é só nosso.
		ResponsavelServicoInstalacoes: l.inteiro("TRILOGO_RESPONSAVEL_SERVICO_INSTALACOES", 67277, 0, 1<<30),
		ResponsavelServicoCivil:       l.inteiro("TRILOGO_RESPONSAVEL_SERVICO_CIVIL", 67264, 0, 1<<30),

		// O NOME, não o id — ver o comentário do campo. Mesma dupla de contas,
		// mesmos valores levantados em 04/09/2026.
		NomeResponsavelServicoInstalacoes: l.texto("TRILOGO_NOME_RESPONSAVEL_SERVICO_INSTALACOES", "Serviço Instalações", false, ""),
		NomeResponsavelServicoCivil:       l.texto("TRILOGO_NOME_RESPONSAVEL_SERVICO_CIVIL", "Romario Santos", false, ""),
	}

	// A leitura da nota, camada 3. Sem chave, o motor continua subindo: as
	// camadas 1 e 2 funcionam sozinhas, só com menos alcance.
	ia := IA{
		Chave: l.segredo("GEMINI_API_KEY", false, 0, ""),
		// VAZIO DE PROPÓSITO
		//   O nome do modelo é do pacote `leitor` (ModeloPadrao). Repetir o padrão
		//   aqui criava DOIS lugares para trocar — e este vencia sempre, porque
		//   nunca ficava vazio. Trocar só o outro não mudaria nada, e o erro
		//   voltaria idêntico na rodada seguinte.
		Modelo: l.texto("GEMINI_MODELO", "", false, ""),
		// Segundos entre duas chamadas. Zero mantém o padrão do pacote; o
		// número real da conta está em aistudio.google.com/rate-limit.
		IntervaloSegundos: l.inteiro("GEMINI_INTERVALO", 0, 0, 600),
	}

	// A classificação de Candidatos a Serviço. Sem chave, o job de Candidatos
	// simplesmente não roda — não é obrigatória em lugar nenhum, nem em
	// produção: o gatilho oficial (mudar o responsável) funciona sem ela.
	groq := Groq{
		Chave:           l.segredo("GROQ_API_KEY", false, 0, ""),
		Modelo:          l.texto("GROQ_MODELO", ModeloGroqPadrao, false, ""),
		LimitePorRodada: l.inteiro("GROQ_LIMITE_POR_RODADA", 200, 1, 5000),
	}

	// QUEM ESTÁ LIGANDO
	//   O motor e o robô rodam o mesmo pacote de configuração, mas precisam de
	//   coisas diferentes. Exigir do robô o tempero do PIN — que ele nunca usa —
	//   só criaria um segredo de mentira no GitHub, que é pior que não ter.
	papel := l.texto("PAPEL", "motor", false, "")
	ehRobo := papel == "robo"

	rt := Runtime{
		Porta:       l.inteiro("PORT", 8000, 1, 65535),
		FusoHoras:   l.inteiro("TZ_OFFSET_H", -3, -12, 14),
		Ambiente:    l.texto("AMBIENTE", "local", false, ""),
		OrigensCORS: l.lista("CORS_ORIGENS", []string{"*"}),
	}

	cfg := &Config{
		Supabase: sb, R2: r2, Trilogo: tri, IA: ia, Groq: groq, Runtime: rt,
		Papel: papel,
		PinPepper: l.segredo("PIN_PEPPER", !ehRobo, 16,
			"Tempero do hash do PIN. Se mudar, todos os PINs param de valer."),
		ChaveRobo: l.segredo("ROBOT_KEY", false, 24,
			"Segredo compartilhado com os robôs do GitHub Actions."),
		DominioLogin: l.texto("DOMINIO_LOGIN", "frotahub.local", false, ""),
	}

	// O robô sem credencial do Trílogo não tem o que fazer: melhor recusar a subir
	// do que rodar e não ler nada.
	if ehRobo && !cfg.Trilogo.Ligado() {
		l.problema("TRILOGO_* — o robô precisa das duas contas (instalações e civil).")
	}

	// Coerências que só valem em produção.
	if cfg.Runtime.Producao() {
		if !cfg.R2.Ligado() {
			l.problema("R2_* — em produção o armazenamento dos arquivos vivos é obrigatório.")
		}
		for _, o := range cfg.Runtime.OrigensCORS {
			if o == "*" {
				l.problema("CORS_ORIGENS — em produção liste os endereços; \"*\" libera o mundo inteiro.")
			}
		}
		if cfg.ChaveRobo == "" {
			l.problema("ROBOT_KEY — em produção é obrigatória, senão os robôs não autenticam.")
		}
	}

	if len(l.problemas) > 0 {
		return nil, &ErroDeConfiguracao{Problemas: l.problemas}
	}
	return cfg, nil
}
