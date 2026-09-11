// rev 5 — o bloco de menu do Administrativo
//
// Fica no seu próprio arquivo, e não dentro de `arvore.ts`, porque este módulo
// cresce sem pedir edição da árvore além do import. Quem ainda não tem tela
// (PCO, Notas fiscais, e Equalizar Propostas) cai no `<EmBreve>` do
// `App.tsx`. Sem `breve`, o desenho é o mesmo da tela inicial — ícone da
// marca, seta, barra cheia — em vez da versão apagada.
//
// "OCs Inseridas" (10/09/2026) entra ENTRE "Inserir OC" e "Equalizar
// Propostas", pedido explícito do dono: é o hub das duas leituras que já
// aconteceram (processadas/rejeitadas) — ver `telas/administrativo/
// OcsInseridas.tsx`, mesmo desenho de `Orcamentos.tsx`.
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
          tela: 'inserir-oc',
          // Migração 059: só CEO ganha a rotina por padrão. Quem for inserir
          // no dia a dia (Nadyson) precisa dela na categoria pela tela de
          // Acesso — sem isso o item nem aparece no menu (P-17).
          rotina: 'COMPRAS_ORDENS_GERENCIAR',
        },
        {
          t: 'OCs Inseridas',
          rota: 'ocs-inseridas',
          icone: 'lista',
          desc: 'As ordens de compra já lidas — processadas e rejeitadas',
          tela: 'ocs-inseridas',
          rotina: 'COMPRAS_ORDENS_GERENCIAR',
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
