// rev 1 — lançar o orçamento no Trílogo, por API
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

	// PASSO 2 e 3 — o documento, e a nossa cópia dele.
	pdf, err := m.montarPDF(r.Context(), p.ClienteID, id)
	if err != nil {
		m.erro(w, "não consegui montar o PDF do orçamento", err)
		return
	}
	sha, err := m.guardarPDF(r.Context(), p.ClienteID, pdf)
	if err != nil {
		m.erro(w, "não consegui guardar o PDF", err)
		return
	}

	// PASSO 4 — o arquivo sobe primeiro, sozinho.
	nome := fmt.Sprintf("ORCAMENTO-%d-%v.pdf", ticket, orc["parte"])
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
		m.erro(w, "o Trílogo recusou o lançamento", err)
		return
	}

	campos := map[string]any{
		"status":             "lancado",
		"lancado_em":         time.Now().UTC().Format(time.RFC3339),
		"lancado_por":        p.UserID,
		"trilogo_custo_id":   custoID,
		"trilogo_permalink":  subido.Permalink,
		"arquivo_pdf_sha256": sha,
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
