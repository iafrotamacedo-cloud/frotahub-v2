// rev 1 — o bloco de menu de Locações (Fase 2, 16/09/2026)
//
// MÓDULO DE 1º NÍVEL, NÃO SUBMENU DE ADMINISTRATIVO
//
//	Decisão do dono (30/08/2026, retomada na sessão de planejamento de
//	Locações): mesmo nível de Administrativo/Manutenção/Engenharia — a OC de
//	locação nasce em Compras, mas o que ela vira depois (vencimento,
//	renovação, devolução) é um trabalho próprio, do dia a dia de quem cuida
//	da obra, não do comprador. Por isso ganha `tela: 'locacoes'` sem `sub`:
//	os cartões (Descobertos/Vencendo/Ativos/Encerrados) são dados de um
//	painel, não uma árvore de navegação — mesmo desenho de `NotasFiscais.tsx`
//	cuidando do próprio `onde` por dentro.
import type { ItemMenu } from '../arvore'

export const locacoesMenu: ItemMenu = {
  t: 'Locações',
  rota: 'locacoes',
  icone: 'lista',
  desc: 'Equipamento locado — vencimento, renovação e devolução',
  tela: 'locacoes',
  // Item aparece pra quem tem qualquer uma das quatro rotinas do módulo —
  // mesmo desenho de "Notas fiscais" em administrativo.ts (rotina como
  // lista = "qualquer uma delas alcança").
  rotina: ['LOCACOES_RECEBER', 'LOCACOES_MONITORAR', 'LOCACOES_DECIDIR', 'LOCACOES_RENOVAR_OC'],
}
