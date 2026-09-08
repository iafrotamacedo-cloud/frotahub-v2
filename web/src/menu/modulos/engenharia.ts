// rev 1 — o bloco de menu de Engenharia > Planejamento
//
// Uma sub-rotina só por enquanto ("Obras"), do mesmo jeito que SESMT e DP
// nasceu com uma. RDO e Equipes (Fase 3) entram aqui quando ganharem tela —
// as rotinas já estão semeadas no banco, esperando (ver migração 056).
import type { ItemMenu } from '../arvore'

export const engenhariaMenu: ItemMenu = {
  t: 'Engenharia',
  rota: 'engenharia',
  icone: 'chave-inglesa',
  desc: 'Planejamento de obra — cronograma e EAP',
  sub: [
    {
      t: 'Obras',
      rota: 'obras',
      icone: 'lista',
      desc: 'Contratante, cronograma e estrutura analítica do projeto',
      tela: 'obras',
      rotina: 'PLANEJAMENTO_OBRAS_DADOS',
    },
  ],
}
