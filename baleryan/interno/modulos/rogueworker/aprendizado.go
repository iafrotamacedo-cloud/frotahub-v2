package rogueworker

import (
	"net/http"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
)

type candidatoBanco struct {
	ID               string         `json:"id"`
	PerguntaOriginal string         `json:"pergunta_original"`
	RotinaResolvida  string         `json:"rotina_resolvida"`
	ComandoResolvido string         `json:"comando_resolvido"`
	Parametros       map[string]any `json:"parametros"`
	Status           string         `json:"status"`
}

func (m *Modulo) oferecerAprendizado(r *http.Request, p *seguranca.Principal, pergunta string, rec reconhecimento, canonico string, saida *Resposta) {
	if canonico == "" {
		canonico = normalizar(pergunta)
	}
	linha := map[string]any{
		"cliente_id":        p.ClienteID,
		"pergunta_original": pergunta,
		"rotina_resolvida":  rotinaDoComando(rec.comando),
		"comando_resolvido": rec.comando,
		"parametros": map[string]any{
			"formato_canonico": canonico,
			"ticket":           rec.ticket,
			"id":               rec.id,
			"tela":             rec.tela,
		},
	}
	var criados []candidatoBanco
	if err := m.bd.Inserir(r.Context(), "rogueworker_candidatos", []map[string]any{linha}, &criados); err != nil || len(criados) == 0 {
		return
	}
	saida.CandidatoID = criados[0].ID
	if rec.comando == cmdDesconhecido {
		saida.Pendente = &Pendente{Tipo: "aprendizado", Candidato: criados[0].ID}
		saida.Opcoes = []string{"Sim, pode lembrar", "Agora não"}
	} else if saida.Pendente == nil {
		saida.Texto += "\n\nIsso ajudou? posso lembrar da próxima vez?"
		saida.Pendente = &Pendente{Tipo: "aprendizado", Candidato: criados[0].ID}
		saida.Opcoes = []string{"Sim, pode lembrar", "Agora não"}
	}
}

func (m *Modulo) confirmarAprendizado(r *http.Request, p *seguranca.Principal, candidatoID string) Resposta {
	id, ok := umUUID(candidatoID)
	if !ok {
		return Resposta{Texto: "Não achei esse aprendizado para confirmar."}
	}
	_ = m.bd.InserirIgnorando(r.Context(), "rogueworker_confirmacoes", []map[string]any{{
		"candidato_id": id,
		"perfil_id":    p.UserID,
	}})

	var confs []struct {
		PerfilID string `json:"perfil_id"`
	}
	_ = m.bd.Buscar(r.Context(),
		"rogueworker_confirmacoes?candidato_id=eq."+banco.Escapar(id)+"&select=perfil_id", &confs)
	if len(confs) >= limiarDePromocao {
		_ = m.bd.Atualizar(r.Context(), "rogueworker_candidatos",
			"id=eq."+banco.Escapar(id)+"&status=eq.candidato",
			map[string]any{"status": "promovido"})
		return Resposta{Texto: "Anotado. Com confirmações suficientes, da próxima vez eu resolvo isso direto, sem perguntar de novo ao modelo."}
	}
	return Resposta{Texto: "Anotado. Quando mais gente confirmar o mesmo tipo de pedido, eu passo a reconhecer sozinha."}
}

func (m *Modulo) casarPromovido(r *http.Request, p *seguranca.Principal, pergunta string) *reconhecimento {
	var lista []candidatoBanco
	_ = m.bd.Buscar(r.Context(),
		"rogueworker_candidatos?cliente_id=eq."+banco.Escapar(p.ClienteID)+
			"&status=eq.promovido&select=id,pergunta_original,rotina_resolvida,comando_resolvido,parametros,status&limit=200",
		&lista)
	for _, c := range lista {
		canon := textoDe(c.Parametros["formato_canonico"])
		if canon == "" {
			canon = c.PerguntaOriginal
		}
		if !pareceCom(pergunta, canon) && !pareceCom(pergunta, c.PerguntaOriginal) {
			continue
		}
		if !comandoConhecido(c.ComandoResolvido) {
			continue
		}
		rec := reconhecimento{comando: c.ComandoResolvido}
		rec.ticket = inteiroDe(c.Parametros["ticket"])
		rec.id = textoDe(c.Parametros["id"])
		rec.tela = textoDe(c.Parametros["tela"])
		if rec.ticket == 0 {
			rec.ticket = extrairTicket(pergunta)
		}
		if rec.id == "" {
			rec.id = extrairUUID(pergunta)
		}
		return &rec
	}
	return nil
}

func rotinaDoComando(comando string) string {
	switch comando {
	case cmdPendencias, cmdExtrapoladas:
		return "CONTRATO_ORCAMENTOS_CORRECOES"
	case cmdStatusNota, cmdBloqueadas:
		return "CONTRATO_ORCAMENTOS_NOTAS"
	case cmdGerar, cmdGerarLote, cmdLancar, cmdLancarLote, cmdMaterial:
		return "CONTRATO_ORCAMENTOS_LANCAR"
	case cmdPagamos:
		return "CONTRATO_FINANCEIRO_PAGAR"
	case cmdFaturado, cmdFechamento:
		return "CONTRATO_ORCAMENTOS_FATURAR"
	default:
		return "rogueworker"
	}
}
