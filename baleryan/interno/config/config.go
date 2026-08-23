// rev 2 — configuração do baleryan
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

// Dropbox guarda o arquivo-mestre.
type Dropbox struct {
	AppKey       string
	AppSecret    string
	RefreshToken string
	Raiz         string
}

func (d Dropbox) Ligado() bool {
	return d.AppKey != "" && d.AppSecret != "" && d.RefreshToken != ""
}

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
	Supabase  Supabase
	R2        R2
	Dropbox   Dropbox
	Runtime   Runtime
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
		"dropbox":  c.Dropbox.Ligado(),
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
		ChavePublica: l.segredo("SUPABASE_ANON_KEY", true, 40,
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

	dbx := Dropbox{
		AppKey:       l.texto("DROPBOX_APP_KEY", "", false, ""),
		AppSecret:    l.segredo("DROPBOX_APP_SECRET", false, 0, ""),
		RefreshToken: l.segredo("DROPBOX_REFRESH_TOKEN", false, 0, ""),
		Raiz:         l.texto("DROPBOX_RAIZ", "/FROTAHUB", false, ""),
	}

	rt := Runtime{
		Porta:       l.inteiro("PORT", 8000, 1, 65535),
		FusoHoras:   l.inteiro("TZ_OFFSET_H", -3, -12, 14),
		Ambiente:    l.texto("AMBIENTE", "local", false, ""),
		OrigensCORS: l.lista("CORS_ORIGENS", []string{"*"}),
	}

	cfg := &Config{
		Supabase: sb, R2: r2, Dropbox: dbx, Runtime: rt,
		PinPepper: l.segredo("PIN_PEPPER", true, 16,
			"Tempero do hash do PIN. Se mudar, todos os PINs param de valer."),
		ChaveRobo: l.segredo("ROBOT_KEY", false, 24,
			"Segredo compartilhado com os robôs do GitHub Actions."),
		DominioLogin: l.texto("DOMINIO_LOGIN", "frotahub.local", false, ""),
	}

	// Coerências que só valem em produção.
	if cfg.Runtime.Producao() {
		if !cfg.R2.Ligado() {
			l.problema("R2_* — em produção o armazenamento dos arquivos vivos é obrigatório.")
		}
		if !cfg.Dropbox.Ligado() {
			l.problema("DROPBOX_* — em produção o arquivo-mestre é obrigatório.")
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
