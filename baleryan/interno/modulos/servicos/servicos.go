// rev 1 — Candidatos e o Kanban de Serviço
//
// Este módulo fica ACIMA do trilogo: pergunta pro trilogo.Servico quando
// precisa falar com o Trílogo de verdade (mudar responsável, criar
// orçamento), e guarda o ESTADO LOCAL do funil — em que coluna cada chamado
// está — nas tabelas servicos_candidatos e servicos_orcamentos (migração
// 050_candidatos_e_kanban_de_servico.sql).
//
// POR QUE NÃO É TUDO DENTRO DE trilogo
//
//	trilogo é o CLIENTE da API de terceiro — não sabe o que é "candidato" nem
//	"coluna do Kanban", só sabe conversar com o Trílogo. Misturar os dois
//	faria o pacote que fala com o cliente carregar regra de negócio nossa: o
//	dia que o Trílogo mudar de API, a régua do Kanban mudaria junto sem
//	precisar (e vice-versa).
//
// AS DUAS PERGUNTAS QUE ESTE MÓDULO RESPONDE, E SÃO DE DONO DIFERENTE
//
//	Candidatos pergunta "será que isto é Serviço?" — resposta de máquina
//	(Groq), provisória, descartável. Kanban pergunta "em que pé está ESTE
//	serviço?" — fato, uma vez que o responsável foi trocado de verdade no
//	Trílogo. candidatos.go cuida da primeira; kanban.go, da segunda.
package servicos

import (
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/armazem"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/config"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/trilogo"
)

type Servico struct {
	cfg  *config.Config
	bd   *banco.Cliente
	tri  *trilogo.Servico
	arm  *armazem.Cliente
	groq *clienteGroq
}

// arm é o MESMO armazém que trilogo.Servico já usa (ver cmd/baleryan/baleryan.go
// e cmd/servicos/main.go) — um R2 só, um dedupe só por sha256 (migração 007).
func Novo(cfg *config.Config, bd *banco.Cliente, tri *trilogo.Servico, arm *armazem.Cliente) *Servico {
	return &Servico{cfg: cfg, bd: bd, tri: tri, arm: arm, groq: novoClienteGroq(cfg.Groq)}
}

// CorteServico — chamado de antes desta data nunca vira Candidato (a
// classificação do Groq nem olha pra ele — ver a view `servicos_a_classificar`,
// que filtra por `chamados.criado_em`). É OUTRA data da `trilogo.DataDeCorte`
// (que é sobre quando a LEITURA do Trílogo começa) — esta é sobre quando o
// funil de Serviço passa a valer, decisão do dono em 04/09/2026.
//
// UM CHAMADO ANTIGO AINDA PODE ENTRAR NO KANBAN, SÓ NÃO SOZINHO
//
//	A regra (correção do dono, 04/09/2026) é só sobre CANDIDATOS. O Kanban de
//	verdade continua aberto pra qualquer chamado, de qualquer data, desde que
//	o responsável tenha sido trocado pra Serviço DEPOIS do corte — manualmente
//	(o botão da tela, ver kanban.go, MarcarComoServico) ou direto no Trílogo. A
//	varredura do gatilho (kanban.go, RodarDeteccaoDeGatilho) usa
//	`chamados.alterado_em` — não `criado_em` — exatamente pra permitir isso:
//	um chamado de 2025 que só teve o responsável trocado ontem tem
//	`alterado_em` de ontem, mesmo com `criado_em` de 2025.
const CorteServico = "2026-09-03"
