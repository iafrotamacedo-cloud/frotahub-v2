// rev 2 — lançar o orçamento no Trílogo, por API
//
// O contrato das chamadas está em docs/api-trilogo.md, levantado por captura de
// rede na tela real em 25/08/2026 — não deduzido.
//
// A ORDEM IMPORTA, E ELA É ESTA
//
//  1. confere o teto DE NOVO, contra os custos que estão no Trílogo AGORA
//
//  2. gera o PDF do orçamento
//
//  3. guarda o PDF no nosso armazém
//
//  4. sobe o PDF no Trílogo e recebe {filename, permalink}
//
//  5. cria o custo carregando esse par
//
//  6. relê os custos para confirmar, e guarda o id que o Trílogo deu
//
//     O passo 6 não é zelo excessivo. O `CreateTicketCost` responde 200 sem dizer o
//     id do que criou; sem a releitura, "lancei" seria uma suposição. No sistema
//     antigo um erro de gravação virava um `print()` e a rotina seguia como se
//     tivesse salvo — era assim que orçamento sumia sem ninguém notar.
package orcamentos

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TipoDoCusto é o que a Frota Macedo lança. Decisão do dono em 25/08/2026:
// sempre Mão de obra. A base histórica é Materiais, e por isso a conferência do
// teto soma TODOS os tipos — senão o teto vazaria pela porta do tipo antigo.
const TipoDoCusto = trilogo.TipoMaoDeObra

func (m *Modulo) lancar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaLancar)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if !m.cfg.Trilogo.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"As contas do Trílogo não estão configuradas no servidor.")
		return
	}

	orc, err := m.contarUm(r.Context(), "orcamentos?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=*&limit=1")
	if err != nil {
		m.erro(w, "não achei este orçamento", err)
		return
	}

	switch fmt.Sprint(orc["status"]) {
	case "lancado":
		web.Falhar(w, http.StatusConflict, "Este orçamento já está lançado no Trílogo.")
		return
	case "aguardando_aprovacao":
		web.Falhar(w, http.StatusConflict,
			"Este orçamento passou do teto e ainda não foi aprovado. Aprove antes de lançar.")
		return
	case "removido":
		web.Falhar(w, http.StatusConflict, "Este orçamento está apagado. Restaure antes de lançar.")
		return
	}

	conta := fmt.Sprint(orc["conta"])
	empresa := m.cfg.Trilogo.EmpresaDaConta(conta)
	if empresa == 0 {
		// Recusar é melhor que chutar: um CompanyId errado lança o custo na
		// empresa errada dentro do sistema do cliente.
		det := fmt.Sprintf("TRILOGO_EMPRESA_%s não está configurada no servidor",
			map[string]string{"civil": "CIVIL", "instalacoes": "INSTALACOES"}[conta])
		m.bloquear(r.Context(), p, id, orc, "sem_empresa", det)
		web.Falhar(w, http.StatusFailedDependency, fmt.Sprintf(
			"Não sei o id da empresa prestadora da conta %q no Trílogo. "+
				"Configure TRILOGO_EMPRESA_%s no servidor antes de lançar por aqui.",
			conta, map[string]string{"civil": "CIVIL", "instalacoes": "INSTALACOES"}[conta]))
		return
	}

	ticket := int(numeroDe(orc["ticket"]))
	valor := regras.DinheiroDe(numeroDe(orc["valor"]))

	sessao, err := m.entrarNoTrilogo(r.Context(), conta)
	if err != nil {
		m.erro(w, "não consegui entrar no Trílogo", err)
		return
	}

	// PASSO 1 — o teto, de novo, contra a verdade de agora.
	custos, err := sessao.Custos(r.Context(), ticket)
	if err != nil {
		m.erro(w, "não consegui ler os custos do ticket no Trílogo", err)
		return
	}
	par, _, _ := m.param.Do(r.Context(), p.ClienteID)
	jaLa := regras.DinheiroDe(trilogo.SomaDosCustos(custos))

	if v := regras.AplicarTeto(valor, jaLa, par); v.Decisao == regras.Aprovacao {
		// NUNCA UM BECO SEM SAÍDA
		//   A resposta diz quanto o ticket já tem, quanto é este orçamento e
		//   qual é o teto. Bloqueio sem saída visível é o que faz o usuário
		//   criar planilha paralela.
		m.bloquear(r.Context(), p, id, orc, "teto", fmt.Sprintf(
			"o ticket já tem %s lançado; com %s passa do teto de %s",
			jaLa.Reais(), valor.Reais(), par.Teto.Reais()))
		web.Responder(w, http.StatusConflict, map[string]any{
			"erro": fmt.Sprintf(
				"Entre a geração e agora, o ticket %d mudou: já tem %s lançado. "+
					"Com este orçamento de %s, passa do teto de %s.",
				ticket, jaLa.Reais(), valor.Reais(), par.Teto.Reais()),
			"ticket":       ticket,
			"ja_no_ticket": jaLa.Float(),
			"deste":        valor.Float(),
			"teto":         par.Teto.Float(),
			"saidas": []string{
				"aprovar este orçamento com a autorização do cliente",
				"reduzir o valor gerando de novo",
				"apagar este orçamento",
			},
		})
		return
	}

	// PASSO 1b — O STATUS DO TICKET, ANTES DE GASTAR QUALQUER COISA
	//
	// POR QUE AQUI, E NÃO LÁ NA FRENTE
	//
	//	A ordem antiga era: gerar PDF, guardar no R2, SUBIR NO TRÍLOGO, criar o
	//	custo. Quando o custo era recusado, o arquivo já tinha subido — e ficava
	//	órfão no S3 do cliente, sem dono e sem endpoint conhecido para apagar.
	//	Medido em 25/08/2026: o `TESTE_FROTAHUB_126211.pdf` que o teste deixou lá.
	//
	//	Com 39 orçamentos travados e uma tentativa por dia, isso seriam ~270
	//	arquivos órfãos por semana no armazém deles.
	//
	// SÓ RECUSO O QUE FOI PROVADO QUE ELES RECUSAM
	//
	//	A lista abaixo tem três status medidos contra o Trílogo de verdade. Status
	//	que eu não conheço PASSA e vai tentar: se um dia eles liberarem outro, o
	//	lançamento funciona sozinho, sem alguém precisar lembrar de mexer aqui.
	//	Uma conferência nossa nunca deve ser mais restritiva que a deles.
	if d, err := sessao.Detalhe(r.Context(), ticket); err != nil {
		m.bloquear(r.Context(), p, id, orc, "trilogo_fora", err.Error())
		m.erro(w, "não consegui ler o chamado no Trílogo", err)
		return
	} else if d == nil || d.ID != ticket {
		// O 200 COM CORPO VAZIO
		//	O Trílogo responde assim quando o chamado não é da conta que perguntou.
		//	Sem esta conferência, viraria um Detalhe zerado e o status 0 passaria.
		det := fmt.Sprintf("a conta %s não devolveu o chamado %d", conta, ticket)
		m.bloquear(r.Context(), p, id, orc, "ticket_recusado", det)
		web.Falhar(w, http.StatusConflict, det+". Confira o número e a conta.")
		return
	} else if nome, travado := statusQueRecusa[d.Status]; travado {
		det := fmt.Sprintf("o Trílogo não aceita custo em chamado %s (status %d)", nome, d.Status)
		m.bloquear(r.Context(), p, id, orc, "ticket_status", det)
		web.Responder(w, http.StatusConflict, map[string]any{
			"erro":          det,
			"ticket":        ticket,
			"ticket_status": nome,
			"saidas": []string{
				"esperar o chamado andar e tentar de novo",
				"cobrar de quem resolve: " + quemResolve(d.Status),
			},
		})
		return
	}

	// PASSO 2 e 3 — o documento, e a nossa cópia dele.
	//
	// NO FATURAMENTO DIRETO, O DOCUMENTO É A PRÓPRIA NOTA
	//
	//	Não existe orçamento para montar: a nota é faturada pelo fornecedor
	//	direto ao cliente, e o que o Trílogo precisa é do comprovante do custo.
	//	Montar um PDF nosso aqui seria fabricar um documento que não representa
	//	transação nenhuma — e ele acabaria anexado a um chamado como se fosse
	//	uma cobrança nossa.
	pdf, nome, err := m.arquivoParaLancar(r.Context(), p.ClienteID, id, orc, ticket)
	if err != nil {
		m.erro(w, "não consegui preparar o documento do lançamento", err)
		return
	}
	sha, err := m.guardarPDF(r.Context(), p.ClienteID, pdf)
	if err != nil {
		m.erro(w, "não consegui guardar o arquivo", err)
		return
	}

	// PASSO 4 — o arquivo sobe primeiro, sozinho.
	subido, err := sessao.SubirArquivo(r.Context(), nome, pdf)
	if err != nil {
		m.erro(w, "o Trílogo não aceitou o arquivo", err)
		return
	}

	// PASSO 5 — o custo, carregando o permalink.
	emissao := time.Now().In(fusoDaCasa()).Format("2006-01-02")
	custo := trilogo.MontarCusto(ticket, TipoDoCusto, empresa, valor.Float(),
		fmt.Sprint(ticket), emissao,
		fmt.Sprintf("Orçamento FrotaHub %d-%v", ticket, orc["parte"]), subido)

	// PASSO 6 — cria e CONFIRMA.
	custoID, err := sessao.CriarCusto(r.Context(), custo)
	if err != nil {
		// Passou pela conferência do passo 1b e mesmo assim foi recusado. Pode ser
		// o status tendo mudado entre a leitura e o envio, ou uma regra que não
		// conhecemos. Guarda a frase deles e chama de desconhecido — que é a
		// verdade, e é melhor do que adivinhar lendo o texto.
		flag, det := "desconhecido", err.Error()
		if e, ok := trilogo.Recusa(err); ok {
			det = e.Corpo
		} else {
			flag = "trilogo_fora"
		}
		m.bloquear(r.Context(), p, id, orc, flag, det)
		web.Responder(w, http.StatusConflict, map[string]any{
			"erro":    "O Trílogo recusou o lançamento.",
			"trilogo": det,
			"ticket":  ticket,
		})
		return
	}

	campos := map[string]any{
		"status":             "lancado",
		"lancado_em":         time.Now().UTC().Format(time.RFC3339),
		"lancado_por":        p.UserID,
		"trilogo_custo_id":   custoID,
		"trilogo_permalink":  subido.Permalink,
		"arquivo_pdf_sha256": sha,
		// Deu certo: a marca do bloqueio anterior some. Ela descreve a ÚLTIMA
		// tentativa, e a última tentativa agora é esta.
		"lancamento_bloqueio":         nil,
		"lancamento_bloqueio_detalhe": nil,
	}
	if err := m.bd.Atualizar(r.Context(), "orcamentos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID), campos); err != nil {
		// O CASO FEIO, DITO EM VOZ ALTA
		//   O custo ESTÁ no Trílogo e a nossa base não sabe. Não dá para
		//   desfazer com segurança daqui (excluir poderia apagar outro custo se
		//   o id vier errado), então a resposta diz exatamente o que aconteceu e
		//   dá o número para o acerto manual. Silêncio aqui viraria orçamento
		//   lançado duas vezes na próxima tentativa.
		log.Printf("orcamentos: ATENÇÃO — custo %d criado no ticket %d mas não gravei aqui: %v",
			custoID, ticket, err)
		web.Responder(w, http.StatusInternalServerError, map[string]any{
			"erro": fmt.Sprintf(
				"O custo FOI criado no Trílogo (nº %d, ticket %d), mas não consegui registrar isso aqui. "+
					"NÃO lance de novo — isso duplicaria. Avise o suporte com estes números.",
				custoID, ticket),
			"trilogo_custo_id": custoID,
			"ticket":           ticket,
		})
		return
	}

	_ = m.hist.Registrar(r.Context(), p, "orcamentos", id, "lancar", nil)
	web.Responder(w, http.StatusOK, map[string]any{
		"ok":               true,
		"trilogo_custo_id": custoID,
		"permalink":        subido.Permalink,
		"valor":            valor.Float(),
	})
}

// Os status em que o Trílogo PROVADAMENTE recusa custo, medidos em 25/08/2026
// contra o sistema de verdade.
//
// A MENSAGEM DELES É INCOMPLETA — NÃO USE A FRASE PARA DECIDIR
//
//	O 400 diz "não é permitida para tickets com o status 'Aberto' ou 'Em
//	execução'". O ticket 126211 estava ARQUIVADO e foi recusado do mesmo jeito.
//	Quem ramificasse pelo texto concluiria que arquivado passa. Aqui a decisão é
//	pelo CÓDIGO, que não mente.
//
// O mapa é deliberadamente curto: o que não está aqui vai tentar. Conferência
// nossa nunca pode ser mais restritiva que a deles.
var statusQueRecusa = map[int]string{
	1: "Aberto",
	3: "Arquivado",
	7: "Em execução",
}

// quemResolve traduz o status em quem tem que agir — a mesma regra que a view
// `orcamentos_lista` usa na coluna `destino`, e pelo mesmo motivo: aberto e em
// execução são serviço nosso; arquivado só o cliente destrava.
//
// A view é a fonte para as listas; isto aqui é só a frase da resposta. Se um dia
// as duas discordarem, a view é que vale.
func quemResolve(status int) string {
	switch status {
	case 1, 7:
		return "os encarregados, para concluir o serviço"
	case 3:
		return "o cliente, que é quem reabre chamado arquivado"
	}
	return "quem cuida do chamado"
}

// bloquear grava POR QUE não deu, e não interrompe nada quando falha.
//
// POR QUE ELE ENGOLE O PRÓPRIO ERRO
//
//	Ele é chamado no caminho de uma recusa que já vai ser respondida ao usuário.
//	Se a gravação da marca falhasse e virasse um segundo erro, a pessoa receberia
//	uma mensagem sobre o banco no lugar da mensagem sobre o Trílogo — trocando o
//	diagnóstico verdadeiro por um ruído. Falhou: vai para o log e a vida segue.
//
// `lancamento_tentativas` VEM DA LINHA QUE JÁ ESTÁ NA MÃO
//
//	O PostgREST não faz `tentativas = tentativas + 1`. Como o orçamento inteiro já
//	foi lido no começo do lançamento, o valor de agora sai dali — sem uma segunda
//	ida ao banco só para somar um.
func (m *Modulo) bloquear(ctx context.Context, p *seguranca.Principal, id string,
	orc map[string]any, flag, detalhe string) {

	campos := map[string]any{
		"lancamento_bloqueio":         flag,
		"lancamento_bloqueio_detalhe": detalhe,
		"lancamento_tentado_em":       time.Now().UTC().Format(time.RFC3339),
		"lancamento_tentativas":       int(numeroDe(orc["lancamento_tentativas"])) + 1,
	}
	if err := m.bd.Atualizar(ctx, "orcamentos",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID), campos); err != nil {
		log.Printf("orcamentos: não consegui marcar o bloqueio %q em %s: %v", flag, id, err)
	}
}

// entrarNoTrilogo abre uma sessão na conta certa.
//
// UMA SESSÃO POR LANÇAMENTO, DE PROPÓSITO
//
//	Guardar a sessão entre requisições economizaria um login — e traria de volta
//	o estado em memória que o motor não tem (P-03). O plano gratuito adormece; a
//	sessão guardada acordaria expirada, e o erro apareceria como "o Trílogo
//	recusou", que manda a pessoa conferir a senha certa.
func (m *Modulo) entrarNoTrilogo(ctx context.Context, conta string) (*trilogo.Sessao, error) {
	for _, c := range m.cfg.Trilogo.Contas() {
		if c.Nome == conta {
			return trilogo.Entrar(ctx, c.Nome, c.Email, c.Senha)
		}
	}
	return nil, fmt.Errorf("não conheço a conta %q", conta)
}

func (m *Modulo) guardarPDF(ctx context.Context, clienteID string, pdf []byte) (string, error) {
	sha := somaDe(pdf)
	chave := armazem.Caminho(clienteID, sha, "pdf")
	if err := m.arm.Enviar(ctx, chave, leitorDeBytes(pdf), int64(len(pdf)), sha, "application/pdf"); err != nil {
		return "", err
	}
	return sha, m.bd.Upsert(ctx, "arquivos?on_conflict=sha256", []map[string]any{{
		"sha256":     sha,
		"cliente_id": clienteID,
		"tamanho":    len(pdf),
		"tipo":       "application/pdf",
		"chave_r2":   chave,
	}}, nil)
}

func numeroDe(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, _ := regrasDecimal(n)
		return f
	}
	return 0
}

// arquivoParaLancar devolve o que sobe para o Trílogo, e com que nome.
//
// SÃO DOIS DOCUMENTOS DIFERENTES, E ISSO NÃO É DETALHE
//
//	No fluxo normal sobe o ORÇAMENTO: um documento nosso, com a nossa margem
//	embutida, que representa o que vamos cobrar. No faturamento direto sobe a
//	NOTA original: o comprovante do que o fornecedor cobrou do cliente.
//
//	Trocar um pelo outro por engano seria anexar ao chamado um documento que
//	afirma uma cobrança que não existe — e ninguém olhando o Trílogo teria como
//	saber, porque os dois têm a cara de um PDF de custo.
func (m *Modulo) arquivoParaLancar(ctx context.Context, clienteID, orcamentoID string,
	orc map[string]any, ticket int) ([]byte, string, error) {

	direto, _ := orc["faturamento_direto"].(bool)
	if !direto {
		pdf, err := m.montarPDF(ctx, clienteID, orcamentoID)
		return pdf, fmt.Sprintf("ORCAMENTO-%d-%v.pdf", ticket, orc["parte"]), err
	}

	if !m.arm.Ligado() {
		return nil, "", fmt.Errorf("o armazenamento não está configurado, e a nota original vive nele")
	}

	// O caminho é: orçamento → nota → arquivo no armazém.
	var vinculos []struct {
		DocumentoID string `json:"documento_id"`
	}
	if err := m.bd.Buscar(ctx, "orcamento_documentos?orcamento_id=eq."+orcamentoID+
		"&select=documento_id&limit=1", &vinculos); err != nil {
		return nil, "", err
	}
	if len(vinculos) == 0 {
		return nil, "", fmt.Errorf("este lançamento não está ligado a nenhuma nota")
	}
	doc, err := m.contarUm(ctx, "documentos?id=eq."+vinculos[0].DocumentoID+
		"&cliente_id=eq."+banco.Escapar(clienteID)+"&select=nome_arquivo,arquivo_sha256&limit=1")
	if err != nil {
		return nil, "", err
	}
	sha, _ := doc["arquivo_sha256"].(string)
	if sha == "" {
		return nil, "", fmt.Errorf("a nota desta linha não tem arquivo guardado")
	}
	arq, err := m.contarUm(ctx, "arquivos?sha256=eq."+sha+"&select=chave_r2&limit=1")
	if err != nil {
		return nil, "", err
	}
	chave, _ := arq["chave_r2"].(string)
	if chave == "" {
		return nil, "", fmt.Errorf("não achei o arquivo da nota no armazém")
	}
	bruto, err := m.arm.Baixar(ctx, chave)
	if err != nil {
		return nil, "", err
	}
	nome, _ := doc["nome_arquivo"].(string)
	if nome == "" {
		nome = fmt.Sprintf("NOTA-%d.pdf", ticket)
	}
	return bruto, nome, nil
}
