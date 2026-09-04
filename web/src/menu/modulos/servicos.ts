// rev 2 — o bloco de menu de Serviço
//
// Serviço é o que fica FORA do contrato de manutenção: instalação, obra,
// ampliação — não conserto do que já existia. Ver baleryan/interno/modulos/
// servicos para o desenho completo (Candidatos, o funil, a fila).
//
// UMA FOLHA SÓ, NÃO DUAS
//
//	O hub (Hub.tsx) já é o painel de 6 cards — mesmo padrão de Orçamentos
//	(um nó com `sub`, mas cujo conteúdo é desenhado pela própria tela, não
//	pelo painel genérico da casca). Candidatos, os 9 sub-status e a Planilha
//	de controle vivem TODOS dentro dele, roteados por `onde` — não viram nós
//	novos na árvore de menu.
import type { ItemMenu } from '../arvore'

export const servicosMenu: ItemMenu = {
  t: 'Serviços',
  rota: 'servicos',
  icone: 'servicos',
  desc: 'Instalação, obra e ampliação — fora do contrato de manutenção',
  tela: 'servicos-hub',
  rotina: 'CONTRATO_SERVICO_GERENCIAR',
}
