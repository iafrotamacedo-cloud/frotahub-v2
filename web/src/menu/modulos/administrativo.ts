// rev 4 — o bloco de menu do Administrativo
//
// Fica no seu próprio arquivo, e não dentro de `arvore.ts`, porque este módulo
// cresce sem pedir edição da árvore além do import. Quem ainda não tem tela
// (PCO, Notas fiscais, Equalizar, e as três filas de OC) cai no `<EmBreve>`
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
      sub: [
        {
          t: 'Inserir OC',
          rota: 'inserir-oc',
          icone: 'lista',
          desc: 'Cadastrar uma ordem de compra',
          sub: [
            {
              t: 'OCs Inseridas',
              rota: 'inseridas',
              icone: 'lista',
              desc: 'Ordens de compra recém-cadastradas',
            },
            {
              t: 'OCs Processadas',
              rota: 'processadas',
              icone: 'lista',
              desc: 'Ordens de compra já processadas',
            },
            {
              t: 'OCs Rejeitadas',
              rota: 'rejeitadas',
              icone: 'lista',
              desc: 'Ordens de compra que não passaram',
            },
          ],
        },
        {
          t: 'Equalizar Propostas',
          rota: 'equalizar-propostas',
          icone: 'balanca',
          desc: 'Comparar as propostas dos fornecedores',
        },
      ],
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
