// rev 2 — o bloco de menu do Administrativo
//
// Fica no seu próprio arquivo, e não dentro de `arvore.ts`, porque este módulo
// cresce sem pedir edição da árvore além do import. Os três cards de baixo
// ainda não têm tela: sem `tela` e sem `rotina`, cada um cai no `<EmBreve>`
// do `App.tsx`. Sem `breve`, o desenho é o mesmo da tela inicial — ícone da
// marca, seta, barra cheia — em vez da versão apagada.
import type { ItemMenu } from '../arvore'

export const administrativoMenu: ItemMenu = {
  t: 'Administrativo',
  rota: 'administrativo',
  icone: 'prancheta',
  desc: 'Compras, PCO e notas fiscais',
  sub: [
    {
      t: 'Compras',
      rota: 'compras',
      icone: 'dinheiro',
      desc: 'O que se compra e o que se cobra do cliente',
    },
    {
      t: 'PCO',
      rota: 'pco',
      icone: 'lista',
      desc: 'Envio das Ordens de Compra ao cliente',
    },
    {
      t: 'Notas fiscais',
      rota: 'notas-fiscais',
      icone: 'lista',
      desc: 'O ciclo da nota, da entrada ao pagamento',
    },
  ],
}
