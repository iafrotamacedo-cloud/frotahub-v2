package rogueworker

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

func (m *Modulo) conversar(w http.ResponseWriter, r *http.Request) {
	p := m.quem(w, r)
	if p == nil {
		return
	}
	var pedido Pedido
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&pedido)
	}
	pedido.Mensagem = strings.TrimSpace(pedido.Mensagem)
	if pedido.Mensagem == "" {
		web.Falhar(w, http.StatusBadRequest, "Escreva o que você precisa.")
		return
	}

	web.Responder(w, http.StatusOK, m.responderTurno(r, p, pedido))
}

func (m *Modulo) responderTurno(r *http.Request, p *seguranca.Principal, pedido Pedido) Resposta {
	if pedido.Pendente != nil {
		if ehNao(pedido.Mensagem) {
			return Resposta{Texto: "Certo, cancelei."}
		}
		switch pedido.Pendente.Tipo {
		case "acao":
			if ehSim(pedido.Mensagem) {
				return m.executarAcao(r, p, pedido.Pendente)
			}
			// a pessoa pode ter mudado de assunto em vez de confirmar
		case "escopo":
			if ehServicos(pedido.Mensagem) {
				return Resposta{
					Texto:    "Nesta versão eu só consulto o módulo de Orçamentos (Manutenção). Quer que eu calcule pela Manutenção?",
					Pendente: &Pendente{Tipo: "escopo", Comando: pedido.Pendente.Comando, Escopo: "manutencao"},
					Opcoes:   []string{"Manutenção", "Agora não"},
				}
			}
			if ehManutencao(pedido.Mensagem) || ehSim(pedido.Mensagem) {
				return m.executarComando(r, p, reconhecimento{comando: pedido.Pendente.Comando}, pedido.Mensagem)
			}
		case "excel":
			if ehSim(pedido.Mensagem) && pedido.Pendente.Excel != "" {
				return Resposta{
					Texto: "Pronto, estou baixando a planilha.",
					Excel: &OfertaExcel{Caminho: pedido.Pendente.Excel, Baixar: true},
				}
			}
		case "aprendizado":
			if ehSim(pedido.Mensagem) && pedido.Pendente.Candidato != "" {
				return m.confirmarAprendizado(r, p, pedido.Pendente.Candidato)
			}
			if ehNao(pedido.Mensagem) {
				return Resposta{Texto: "Tudo bem, não vou lembrar."}
			}
		}
	}

	rec := reconhecer(pedido.Mensagem)
	if rec.comando == cmdDesconhecido {
		if promovido := m.casarPromovido(r, p, pedido.Mensagem); promovido != nil {
			rec = *promovido
		}
	}
	if rec.comando == cmdDesconhecido {
		return m.cairNoGroq(r, p, pedido.Mensagem)
	}
	return m.executarComando(r, p, rec, pedido.Mensagem)
}

func (m *Modulo) executarComando(r *http.Request, p *seguranca.Principal, rec reconhecimento, frase string) Resposta {
	switch rec.comando {
	case cmdPendencias:
		return m.cmdPendencias(r, p)
	case cmdStatusNota:
		return m.cmdStatusNota(r, p, rec)
	case cmdExtrapoladas:
		return m.cmdExtrapoladas(r, p)
	case cmdBloqueadas:
		return m.cmdBloqueadas(r, p)
	case cmdGerar, cmdGerarLote, cmdLancar, cmdLancarLote:
		return m.proporAcao(r, p, rec)
	case cmdPagamos, cmdMaterial, cmdFaturado, cmdFechamento:
		return m.proporOuEntregarRelatorio(r, p, rec, frase)
	case cmdNavegar:
		return m.executarNavegar(r, p, rec)
	default:
		return Resposta{Texto: "Não entendi o pedido. Tente de outro jeito — por exemplo, \"quantas notas estão pendentes agora?\"."}
	}
}
