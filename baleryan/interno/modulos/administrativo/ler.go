// rev 1 — ler a Ordem de Compra: determinística, sem robô, na hora do clique
//
// O PEDIDO DO DONO (10/09/2026)
//
//	Um botão "Ler". Dois filtros de negócio decidem sozinhos: fornecedor
//	precisa ter CNPJ; o CNPJ de faturamento precisa começar com 03720882
//	(migração 059). Quem passa vai para "Processadas" (`status = lido`), quem
//	falha vai para "Rejeitadas" (`status = falhou`, com o motivo). Nenhuma OC
//	pode ficar presa em "inserido" depois de uma tentativa — ela sempre sai
//	com um status novo, mesmo quando falha.
//
// BLOQUEIO OTIMISTA, MESMO DESENHO DE ORÇAMENTOS
//
//	A condição vai no FILTRO do `AtualizarDevolvendo`
//	(`status=in.(inserido,falhou,lendo)`), não numa leitura seguida de escrita.
//	Duas leituras da mesma OC não colidem: só quem recebe a linha de volta
//	ganhou o direito de ler.
//
// A OC VOLTA PARA "inserido" QUANDO A CULPA É DO SERVIDOR
//
//	`ErroDoServidor` (hoje só "sem pdftotext instalado") não marca a OC como
//	`falhou` — o documento pode estar perfeito. Mesmo espírito de
//	`leitura.FalhaTemporaria` em Orçamentos: falha do servidor não é falha do
//	documento, e a pessoa tenta de novo depois de o servidor ser consertado.
//
// RETOMAR "lendo" PRESO
//
//	Orçamentos disputa a leitura com um robô (GitHub Actions) e precisa de
//	`jobs` com timeout. Aqui só existe o clique — sem concorrente — mas um
//	UPDATE que falha no meio (ex.: CHECK do `comprador_cnpj` antes da
//	correção de 11/09/2026) também deixava a OC presa em "lendo". Por isso o
//	CAS aceita `lendo` de novo: "ler"/"ler todas" retomam a leitura em vez de
//	exigir arrumar à mão no banco.
package administrativo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// TetoDaLeitura limita quantas OCs o botão de lote recebe de uma vez — mesma
// razão de `orcamentos.TetoDaLeitura`: a pessoa fica olhando a barra andar, e
// quando o corte acontece a tela DIZ que aconteceu.
const TetoDaLeitura = 500

// statusPorLer são os status que o botão "ler" pode tomar — inclui `lendo`
// para retomar OCs presas (ver cabeçalho).
const statusPorLer = "inserido,falhou,lendo"

// ---------------------------------------------------------------------------
// GET /administrativo/compras/ordens/porler
// ---------------------------------------------------------------------------

func (m *Modulo) ordensPorLer(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	var linhas []map[string]any
	total, err := m.bd.BuscarContando(r.Context(), "ordens_compra?cliente_id=eq."+
		banco.Escapar(p.ClienteID)+"&status=in.("+statusPorLer+")&order=criado_em"+
		"&select=id,nome_arquivo,status&limit="+strconv.Itoa(TetoDaLeitura), &linhas)
	if err != nil {
		m.erro(w, "não consegui listar as ordens por ler", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{
		"ordens": ouVazio(linhas),
		"total":  total,
		"teto":   TetoDaLeitura,
	})
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/ordens/{id}/ler
// ---------------------------------------------------------------------------

func (m *Modulo) lerOrdem(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	var tomadas []map[string]any
	if err := m.bd.AtualizarDevolvendo(r.Context(), "ordens_compra",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&status=in.("+statusPorLer+")",
		map[string]any{"status": "lendo", "erro_leitura": nil}, &tomadas); err != nil {
		m.erro(w, "não consegui começar a leitura desta ordem de compra", err)
		return
	}
	if len(tomadas) == 0 {
		atual, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
			"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=status&limit=1")
		if err != nil {
			m.erro(w, "não achei esta ordem de compra", err)
			return
		}
		web.Falhar(w, http.StatusConflict, motivoDeNaoLer(fmt.Sprint(atual["status"])))
		return
	}
	sha, _ := tomadas[0]["arquivo_sha256"].(string)

	if sha == "" {
		if errGravar := m.terminarLeitura(r.Context(), p, id, "falhou", "esta ordem de compra não tem arquivo guardado", nil, nil); errGravar != nil {
			m.voltarParaInserido(r.Context(), id)
			m.erro(w, "não consegui gravar a rejeição desta ordem de compra", errGravar)
			return
		}
		web.Responder(w, http.StatusOK, map[string]any{"status": "falhou",
			"motivo": "esta ordem de compra não tem arquivo guardado"})
		return
	}
	arq, err := m.contarUm(r.Context(), "arquivos?sha256=eq."+banco.Escapar(sha)+"&select=chave_r2&limit=1")
	if err != nil {
		m.voltarParaInserido(r.Context(), id)
		m.erro(w, "não achei o arquivo desta ordem de compra no armazém", err)
		return
	}
	chave, _ := arq["chave_r2"].(string)
	pdf, err := m.arm.Baixar(r.Context(), chave)
	if err != nil {
		m.voltarParaInserido(r.Context(), id)
		web.Falhar(w, http.StatusServiceUnavailable,
			"Não consegui baixar o PDF desta ordem de compra. Tente de novo em instantes.")
		return
	}

	ex, err := Ler(r.Context(), pdf)
	if err != nil {
		var doServidor ErroDoServidor
		if errors.As(err, &doServidor) {
			// A culpa é do servidor, não da OC — ela volta para "inserido",
			// não para "falhou" (ver o cabeçalho do arquivo).
			m.voltarParaInserido(r.Context(), id)
			web.Falhar(w, http.StatusServiceUnavailable,
				"Não consegui ler agora: "+doServidor.Motivo+". Tente de novo em instantes.")
			return
		}
		if errGravar := m.terminarLeitura(r.Context(), p, id, "falhou", err.Error(), nil, nil); errGravar != nil {
			m.voltarParaInserido(r.Context(), id)
			m.erro(w, "rejeitei a ordem de compra mas não consegui gravar o resultado", errGravar)
			return
		}
		web.Responder(w, http.StatusOK, map[string]any{"status": "falhou", "motivo": err.Error()})
		return
	}

	fornecedorID, ferr := m.resolverFornecedor(r.Context(), p.ClienteID, ex)
	if ferr != nil {
		log.Printf("administrativo: gravando fornecedor da OC %s: %v", id, ferr)
	}
	if cerr := m.resolverCentroCusto(r.Context(), p.ClienteID, ex); cerr != nil {
		log.Printf("administrativo: gravando centro de custo da OC %s: %v", id, cerr)
	}

	status, motivo := "lido", ""
	if motivos := ex.MotivosDeRejeicao(); len(motivos) > 0 {
		status, motivo = "falhou", strings.Join(motivos, "; ")
	}

	// OS CAMPOS LIDOS SÃO GRAVADOS SEMPRE, MESMO QUANDO A OC VAI SER
	// REJEITADA: os dois filtros são sobre comprador e fornecedor, não sobre
	// os itens — uma OC rejeitada por CNPJ errado ainda tem itens válidos, e
	// quem vai corrigir e reenviar se beneficia de ver o que já foi lido.
	if err := m.terminarLeitura(r.Context(), p, id, status, motivo, camposLidos(ex, fornecedorID), &ex); err != nil {
		m.voltarParaInserido(r.Context(), id)
		m.erro(w, "li a ordem de compra mas não consegui gravar o resultado", err)
		return
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"status": status,
		"motivo": motivo,
		"numero": ex.Numero,
		"total":  ex.Total.Float(),
		"itens":  len(ex.Itens),
	})
}

// motivoDeNaoLer explica por que o bloqueio otimista não pegou a linha — a
// mesma ideia de `orcamentos.podeLerAgora`, adaptada às 4 situações da OC.
func motivoDeNaoLer(status string) string {
	switch status {
	case "lendo":
		return "Esta ordem de compra não está em condição de ser lida agora."
	case "lido":
		return "Esta ordem de compra já foi lida."
	default:
		return "Não achei esta ordem de compra, ou ela não está em condição de ser lida agora."
	}
}

// voltarParaInserido desfaz a marca "lendo" quando o problema foi baixar o
// arquivo ou falar com o armazém — falha do servidor, não da OC.
func (m *Modulo) voltarParaInserido(ctx context.Context, id string) {
	_ = m.bd.Atualizar(ctx, "ordens_compra", "id=eq."+id, map[string]any{"status": "inserido"})
}

// resolverFornecedor acha ou cria o fornecedor pelo par (cliente, CNPJ) —
// mesma regra da migração 059: "a tela de inserção cria o fornecedor na
// hora, se for novo". Devolve "" sem erro quando a OC não trouxe fornecedor
// (nome ou CNPJ vazio) — nesse caso o filtro de negócio já vai rejeitar a OC,
// e `ordens_compra.fornecedor_id` fica nulo.
func (m *Modulo) resolverFornecedor(ctx context.Context, clienteID string, ex Extraida) (string, error) {
	if ex.FornecedorNome == "" || ex.FornecedorCNPJ == "" {
		return "", nil
	}
	var gravados []map[string]any
	if err := m.bd.Upsert(ctx, "fornecedores?on_conflict=cliente_id,cnpj", []map[string]any{{
		"cliente_id":   clienteID,
		"razao_social": ex.FornecedorNome,
		"cnpj":         ex.FornecedorCNPJ,
	}}, &gravados); err != nil {
		return "", err
	}
	if len(gravados) == 0 {
		return "", fmt.Errorf("upsert do fornecedor não devolveu id")
	}
	return fmt.Sprint(gravados[0]["id"]), nil
}

// TRAVA TEMPORÁRIA (12/09/2026) — DESLIGADA DE PROPÓSITO
//
//	O dono está testando o resto do módulo com OCs falsas (OCs_Teste, as
//	OC_BLOC_* que ele mandou analisar) e não quer essas obras/CNPJs de
//	mentira "aprendidas" em `centros_custo`. Vira `true` (e esta trava some)
//	quando as OCs de verdade começarem a passar pelo sistema de novo.
const aprenderCentrosCusto = false

// resolverCentroCusto alimenta `centros_custo` sozinho — mesma receita de
// `resolverFornecedor`, migração 062. Só registra quando o CNPJ de
// faturamento já passou no filtro de raiz (`compradorCNPJParaBanco`
// devolvendo não-nil): um centro de custo com CNPJ errado não é "aprendido"
// como se fosse bom. Pedido do dono: só guardar, sem validar nada com isso —
// por isso o erro aqui nunca impede a leitura, só vira log (mesmo trato de
// `resolverFornecedor` acima).
func (m *Modulo) resolverCentroCusto(ctx context.Context, clienteID string, ex Extraida) error {
	if !aprenderCentrosCusto {
		return nil
	}
	obra := strings.TrimSpace(ex.ObraCentroCusto)
	if obra == "" || compradorCNPJParaBanco(ex.CompradorCNPJ) == nil {
		return nil
	}
	return m.bd.Upsert(ctx, "centros_custo?on_conflict=cliente_id,obra_centro_custo", []map[string]any{{
		"cliente_id":        clienteID,
		"obra_centro_custo": obra,
		"comprador_nome":    textoOuNil(ex.CompradorNome),
		"comprador_cnpj":    ex.CompradorCNPJ,
	}}, nil)
}

// camposLidos é o que a leitura tirou do PDF, no formato da coluna do banco.
func camposLidos(ex Extraida, fornecedorID string) map[string]any {
	campos := map[string]any{
		"numero":            textoOuNil(ex.Numero),
		"data":              textoOuNil(ex.Data),
		"obra_centro_custo": textoOuNil(ex.ObraCentroCusto),
		"cond_pgto":         textoOuNil(ex.CondPgto),
		"forma_pgto":        textoOuNil(ex.FormaPgto),
		"previsao_entrega":  textoOuNil(ex.PrevisaoEntrega),
		"comprador_interno": textoOuNil(ex.CompradorInterno),
		"comprador_nome":    textoOuNil(ex.CompradorNome),
		"comprador_cnpj":    compradorCNPJParaBanco(ex.CompradorCNPJ),
		"subtotal":          ex.Subtotal.Float(),
		"desconto":          ex.Desconto.Float(),
		"frete":             ex.Frete.Float(),
		"total":             ex.Total.Float(),
	}
	if fornecedorID != "" {
		campos["fornecedor_id"] = fornecedorID
	}
	return campos
}

// compradorCNPJParaBanco grava o CNPJ de faturamento só quando passa no CHECK
// da migração 059 (`^03720882`). CNPJ inválido continua visível no
// `erro_leitura` (MotivosDeRejeicao), mas não pode ir para a coluna — senão o
// UPDATE de uma OC rejeitada falha e ela fica presa em "lendo".
func compradorCNPJParaBanco(cnpj string) any {
	cnpj = strings.TrimSpace(cnpj)
	if cnpj == "" || !strings.HasPrefix(cnpj, CNPJRaizPermitida) {
		return nil
	}
	return cnpj
}

// terminarLeitura fecha a OC com o desfecho final — status, motivo, os
// campos lidos (quando houver) e os itens (quando `ex` não é nil) — mais o
// rastro, na mesma disciplina de rastro da MOD-USUARIOS-01, com o Principal
// de verdade (não nil, porque `historico.Registrar` lê `p.ClienteID`
// internamente).
//
// UMA CHAMADA SÓ AO BANCO PARA OS CAMPOS DA OC
//
//	Juntar `camposLidos` + status + erro_leitura numa única `Atualizar`
//	encolhe ao mínimo a janela em que a OC fica em "lendo" — ver o
//	cabeçalho do arquivo sobre por que não existe recuperação de órfão aqui.
func (m *Modulo) terminarLeitura(ctx context.Context, p *seguranca.Principal, id, status, motivo string,
	camposLidos map[string]any, ex *Extraida) error {
	campos := map[string]any{"status": status, "erro_leitura": textoOuNil(motivo)}
	for k, v := range camposLidos {
		campos[k] = v
	}
	if err := m.bd.Atualizar(ctx, "ordens_compra", "id=eq."+id, campos); err != nil {
		log.Printf("administrativo: fechando a leitura da OC %s como %q: %v", id, status, err)
		return err
	}

	if ex != nil {
		m.gravarItens(ctx, id, ex.Itens)
	}

	acao := "ler_ordem_compra"
	if status == "falhou" {
		acao = "rejeitar_ordem_compra"
	}
	_ = m.hist.Registrar(ctx, p, "administrativo", id, acao, map[string]historico.Mudanca{
		"status": {De: nil, Para: status},
	})
	return nil
}

// gravarItens apaga e reinsere tudo a cada leitura — nunca soma em cima.
// Mesma regra de `documento_itens` em Orçamentos (P-06): uma releitura
// substitui, não acumula.
func (m *Modulo) gravarItens(ctx context.Context, ordemID string, itens []ItemExtraido) {
	if err := m.bd.Apagar(ctx, "ordens_compra_itens", "ordem_compra_id=eq."+ordemID); err != nil {
		log.Printf("administrativo: limpando itens antigos da OC %s: %v", ordemID, err)
		return
	}
	if len(itens) == 0 {
		return
	}
	linhas := make([]map[string]any, 0, len(itens))
	for _, it := range itens {
		linhas = append(linhas, map[string]any{
			"ordem_compra_id": ordemID,
			"descricao":       it.Descricao,
			"qtd":             it.Qtd,
			"unidade":         textoOuNil(it.Unidade),
			"valor_unit":      it.ValorUnit.Float(),
			"desconto":        it.Desconto.Float(),
			"total":           it.Total.Float(),
		})
	}
	if err := m.bd.Inserir(ctx, "ordens_compra_itens", linhas, nil); err != nil {
		log.Printf("administrativo: gravando os %d itens da OC %s: %v", len(linhas), ordemID, err)
	}
}

func textoOuNil(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
