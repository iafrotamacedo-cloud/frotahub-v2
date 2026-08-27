// rev 1 — o motivo da falha, escrito para gente
//
// POR QUE ISTO EXISTE
//
//	Quando a leitura falha, o motivo vai para `leitura_erro` e de lá para a tela,
//	onde alguém que precisa DECIDIR o que fazer o lê. Até 26/08/2026 o que
//	chegava lá era a frase crua do Google:
//
//	  "This model models/gemini-2.5-flash-lite is no longer available to new
//	   users. Please update your code to use models/gemini-3.5-flash-lite for the
//	   latest features and improvements."
//
//	Está em inglês, fala com o programador, e não diz o que a pessoa na frente da
//	tela deve fazer. Para ela, é ruído com cara de erro grave.
//
// O DETALHE TÉCNICO NÃO SE PERDE
//
//	Ele vai entre parênteses, no fim. Quem opera lê a primeira parte e sabe o que
//	fazer; quem for investigar tem a frase original inteira. Traduzir jogando
//	fora o original seria trocar um problema por outro.
package leitor

import "strings"

// pistas liga um pedaço reconhecível da mensagem técnica à frase que resolve.
//
// A ordem importa: a primeira que casar vence. As mais específicas vêm antes.
var pistas = []struct {
	contem   string
	explicar string
}{
	// ONDE FICA A CONFIGURAÇÃO DEPENDE DE QUEM ESTÁ LENDO
	//
	//	Estas frases nasceram quando só o robô do GitHub lia, e mandavam a pessoa
	//	para "o workflow". Desde 27/08/2026 o motor lê também, e ali a variável
	//	mora no painel do serviço. Mandar quem está na tela procurar no lugar
	//	errado é pior do que não dizer nada — ele troca a linha, clica de novo e
	//	recebe o mesmo erro, achando que não adiantou.
	{"no longer available",
		"o modelo de IA configurado foi aposentado pelo Google. Troque o GEMINI_MODELO por um modelo vivo (no painel do motor e no workflow do robô) e mande ler de novo"},
	{"is not found",
		"o modelo de IA configurado não existe. Confira o GEMINI_MODELO no painel do motor e no workflow do robô"},
	{"api key not valid",
		"a chave da IA foi recusada. Confira o GEMINI_API_KEY no painel do motor e nos segredos do repositório"},
	{"api_key_invalid",
		"a chave da IA foi recusada. Confira o GEMINI_API_KEY no painel do motor e nos segredos do repositório"},
	{"permission",
		"a chave da IA não tem permissão para este modelo"},
	{"quota",
		"a cota diária da IA acabou. Ela volta à meia-noite no horário do Pacífico — mande ler de novo depois disso"},
	{"resource_exhausted",
		"a cota da IA acabou por agora. Espere alguns minutos e mande ler de novo"},
	{"rate limit",
		"a IA recebeu pedidos demais em pouco tempo. Espere alguns minutos e mande ler de novo"},
	{"sobrecarregada",
		"a IA está sobrecarregada. É passageiro: mande ler de novo daqui a pouco"},
	{"overloaded",
		"a IA está sobrecarregada. É passageiro: mande ler de novo daqui a pouco"},
	{"unavailable",
		"a IA está fora do ar no momento. É passageiro: mande ler de novo daqui a pouco"},
	{"deadline", "a IA demorou demais para responder"},
	{"timeout", "a IA demorou demais para responder"},
	{"context deadline exceeded", "a leitura demorou demais e foi interrompida"},
	{"too large",
		"o arquivo é grande demais para a leitura. Reduza a imagem ou mande as páginas separadas"},
	{"não está registrado",
		"o arquivo desta nota não está no armazém. Insira a nota de novo"},
	{"a conta não fecha",
		"os itens lidos não somam o total da nota. Confira o papel e corrija antes de gerar"},
	{"sem chave", "a leitura por IA está desligada — falta a chave"},
}

// EmPortugues devolve o motivo da falha como alguém que opera precisa lê-lo.
//
// Mensagem que já está em português passa intacta: as recusas do próprio sistema
// (a conta que não fecha, o item que não fecha) já foram escritas para gente.
func EmPortugues(bruto string) string {
	limpo := strings.TrimSpace(bruto)
	if limpo == "" {
		return "a leitura desta nota falhou, e o motivo não foi registrado"
	}
	baixo := strings.ToLower(limpo)
	for _, p := range pistas {
		if !strings.Contains(baixo, p.contem) {
			continue
		}
		// A frase original vira nota de rodapé, não some.
		if strings.EqualFold(p.explicar, limpo) {
			return p.explicar
		}
		return p.explicar + " (" + resumir(limpo) + ")"
	}
	// Sem pista conhecida, ao menos a moldura fica em português e a frase técnica
	// aparece como o que é: detalhe, e não instrução.
	return "a leitura desta nota falhou (" + resumir(limpo) + ")"
}

// resumir corta a frase técnica: elas vêm com duas ou três orações e a primeira
// já identifica o caso. O resto empurraria a explicação para fora da tela.
//
// O CORTE É NO PONTO FINAL, E NÃO EM QUALQUER PONTO
//
//	"gemini-2.5-flash-lite" tem dois pontos no meio do nome. Cortar no primeiro
//	ponto que aparece decepava justamente a informação que identifica o caso — o
//	nome do modelo — e deixava "This model models/gemini-2". Fim de frase é
//	ponto seguido de espaço, ou fim de linha.
func resumir(s string) string {
	if i := strings.Index(s, ". "); i > 20 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '\n'); i > 20 {
		s = s[:i]
	}
	const teto = 120
	r := []rune(s)
	if len(r) > teto {
		return strings.TrimSpace(string(r[:teto-1])) + "…"
	}
	return strings.TrimSpace(s)
}
