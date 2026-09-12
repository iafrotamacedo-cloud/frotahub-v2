// rev 1 — excluir e substituir OCs (11/09/2026)
//
// O PEDIDO DO DONO
//
//	Faturamento errado não se edita mais (ver o cabeçalho de `reparo.go`) —
//	o CNPJ de faturamento pertence ao Obra Prima, e um remendo só do nosso
//	lado deixa o registro de lá errado pra sempre. A correção virou: a
//	pessoa acerta a OC no Obra Prima, baixa o PDF novo, e SUBSTITUI aqui —
//	a OC velha é apagada de vez, a nova entra do zero na fila de leitura.
//
//	Separado disso, mas na mesma frente: um botão de EXCLUIR de verdade,
//	disponível em qualquer fila antes do e-mail de PCO sair. Os dois casos
//	apagam a mesma coisa (linha, itens, arquivo no R2, registro de dedup em
//	`arquivos`) — por isso dividem `apagarOrdemDeVez`.
//
// POR QUE APAGAR DE VERDADE, E NÃO SÓ MARCAR INATIVA (FURO NO CORE-05)
//
//	Exceção deliberada, pedida pelo dono: manter o PDF errado no R2 pra
//	sempre só encheria o armazém sem necessidade nenhuma — nada consulta uma
//	OC substituída ou excluída depois do fato. O histórico (`historico`)
//	continua imutável — a linha em si some, o rastro de que ela existiu e
//	foi apagada não.
package administrativo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// ---------------------------------------------------------------------------
// DELETE /administrativo/compras/ordens/{id}
// ---------------------------------------------------------------------------

func (m *Modulo) excluirOrdem(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,nome_arquivo,arquivo_sha256,pco_enviado_em&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if temPCOEnviado(ordem["pco_enviado_em"]) {
		web.Falhar(w, http.StatusConflict, "Esta ordem de compra já foi enviada por e-mail e não pode mais ser excluída.")
		return
	}

	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "excluir_ordem_compra", map[string]historico.Mudanca{
		"nome_arquivo": {De: strCampo(ordem["nome_arquivo"]), Para: ""},
	})
	if err := m.apagarOrdemDeVez(r.Context(), p.ClienteID, id, strCampo(ordem["arquivo_sha256"])); err != nil {
		m.erro(w, "não consegui excluir esta ordem de compra", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"excluida": true})
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/ordens/{id}/substituir
// ---------------------------------------------------------------------------

func (m *Modulo) substituirOrdem(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	if !m.arm.Ligado() {
		web.Falhar(w, http.StatusServiceUnavailable,
			"O armazenamento de arquivos não está configurado. Sem ele, substituir perderia o arquivo.")
		return
	}
	ordem, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+
		"&select=id,status,erro_leitura,arquivo_sha256&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if fmtStatus(ordem["status"]) != "falhou" {
		web.Falhar(w, http.StatusConflict, "Só dá para substituir uma ordem de compra rejeitada.")
		return
	}
	if _, faturamento, _ := errosDaRejeicao(strCampo(ordem["erro_leitura"])); !faturamento {
		web.Falhar(w, http.StatusConflict,
			"Esta ordem de compra não precisa de substituição — o problema não é no faturamento.")
		return
	}

	if err := r.ParseMultipartForm(TamanhoMaximo); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo enviado.")
		return
	}
	cabecalhos := r.MultipartForm.File["arquivo"]
	if len(cabecalhos) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Escolha o PDF corrigido.")
		return
	}
	cabecalho := cabecalhos[0]

	// CONFERE ANTES DE TROCAR NADA: não pode ser o mesmo arquivo de antes.
	//
	//	`guardarUma` faz dedup por sha256 — se a pessoa escolher sem querer o
	//	MESMO PDF errado, ele "acharia" a própria OC que estamos prestes a
	//	apagar, e a substituição sumiria com a OC sem deixar nada no lugar.
	//	Melhor recusar cedo, com uma mensagem que explica o motivo.
	f, ferr := cabecalho.Open()
	if ferr != nil {
		web.Falhar(w, http.StatusBadRequest, "Não consegui abrir o arquivo enviado.")
		return
	}
	conteudo, rerr := io.ReadAll(io.LimitReader(f, TamanhoMaximo+1))
	f.Close()
	if rerr != nil || len(conteudo) == 0 {
		web.Falhar(w, http.StatusBadRequest, "Não consegui ler o arquivo enviado.")
		return
	}
	soma := sha256.Sum256(conteudo)
	shaNovo := hex.EncodeToString(soma[:])
	shaAntigo := strCampo(ordem["arquivo_sha256"])
	if shaNovo == shaAntigo {
		web.Falhar(w, http.StatusBadRequest,
			"Este é o mesmo arquivo de antes — escolha o PDF já corrigido no Obra Prima.")
		return
	}

	// PRIMEIRO GUARDA O NOVO, DEPOIS APAGA O VELHO
	//
	//	Se o envio falhar no meio, a OC antiga continua lá — errada, mas
	//	visível — em vez de sumir sem nada no lugar.
	novoID, _, err := m.guardarUma(r.Context(), p, cabecalho)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = m.hist.Registrar(r.Context(), p, "administrativo", id, "substituir_ordem_compra", map[string]historico.Mudanca{
		"arquivo_sha256": {De: shaAntigo, Para: shaNovo},
	})
	if err := m.apagarOrdemDeVez(r.Context(), p.ClienteID, id, shaAntigo); err != nil {
		m.erro(w, "guardei o arquivo novo, mas não consegui remover a ordem de compra antiga", err)
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"nova_ordem_id": novoID})
}

// ---------------------------------------------------------------------------
// apagar de vez — compartilhado por excluir e substituir
// ---------------------------------------------------------------------------

func (m *Modulo) apagarOrdemDeVez(ctx context.Context, clienteID, id, sha string) error {
	if err := m.bd.Apagar(ctx, "ordens_compra_itens", "ordem_compra_id=eq."+id); err != nil {
		return err
	}
	if err := m.bd.Apagar(ctx, "ordens_compra",
		"id=eq."+id+"&cliente_id=eq."+banco.Escapar(clienteID)); err != nil {
		return err
	}
	if sha != "" {
		_ = apagarArquivoSeOrfao(ctx, m.bd, m.arm, sha)
	}
	return nil
}
