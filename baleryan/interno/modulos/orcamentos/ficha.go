// rev 1 — a ficha de UM orçamento
//
// POR QUE ESTA TELA EXISTE
//
//	Até 26/08/2026 um orçamento gerado não abria em lugar nenhum: a única coisa
//	que se fazia com ele era baixar o PDF. Quem batia o olho numa linha da
//	planilha e queria conferir contra a nota tinha que decorar o número, voltar
//	para Notas e DAVs, procurar o arquivo e abrir de lá.
//
//	O caminho de volta ao papel é justamente o que o sistema tem de melhor: ele
//	SABE de qual nota cada orçamento nasceu, porque o vínculo está gravado. Não
//	oferecer esse caminho era guardar a informação e não usá-la.
package orcamentos

import (
	"context"
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// notaDoOrcamento é o caminho de volta ao papel.
type notaDoOrcamento struct {
	ID     string  `json:"id"`
	Nome   string  `json:"nome_arquivo"`
	Numero *string `json:"numero"`
	DAV    *string `json:"dav_numero"`
}

func (m *Modulo) fichaDoOrcamento(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r, RotinaPlanilhas)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	var linhas []map[string]any
	if err := m.bd.Buscar(r.Context(), "orcamentos_lista?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(p.ClienteID)+"&select=*&limit=1", &linhas); err != nil {
		m.erro(w, "não consegui abrir o orçamento", err)
		return
	}
	if len(linhas) == 0 {
		web.Falhar(w, http.StatusNotFound, "Não achei este orçamento.")
		return
	}

	// AS NOTAS VÊM SEPARADAS, E SÓ AS VIVAS
	//
	//	Um vínculo marcado (`removido_em`) pertence a um orçamento que foi
	//	apagado — apontar para ele daria à pessoa um "ver doc original" que abre
	//	a nota errada, ou nenhuma. Ver a 025.
	notas, err := m.notasVivasDoOrcamento(r.Context(), id)
	if err != nil {
		m.erro(w, "não consegui achar a nota deste orçamento", err)
		return
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"orcamento": linhas[0],
		"notas":     notas,
	})
}

func (m *Modulo) notasVivasDoOrcamento(ctx context.Context, orcamentoID string) ([]notaDoOrcamento, error) {
	var vinculos []struct {
		Documento string `json:"documento_id"`
	}
	if err := m.bd.Buscar(ctx, "orcamento_documentos?orcamento_id=eq."+orcamentoID+
		"&removido_em=is.null&select=documento_id", &vinculos); err != nil {
		return nil, err
	}
	saida := make([]notaDoOrcamento, 0, len(vinculos))
	for _, v := range vinculos {
		var docs []notaDoOrcamento
		if err := m.bd.Buscar(ctx, "documentos?id=eq."+v.Documento+
			"&select=id,nome_arquivo,numero,dav_numero&limit=1", &docs); err != nil {
			return nil, err
		}
		// A NOTA OCULTA CONTINUA ABRINDO
		//   Tirar da fila é organização da lista de trabalho, não descarte. Se
		//   o orçamento existe, o papel que o originou tem que ser alcançável —
		//   é a razão de "tirar da fila" nunca ter sido "apagar".
		saida = append(saida, docs...)
	}
	return saida, nil
}
