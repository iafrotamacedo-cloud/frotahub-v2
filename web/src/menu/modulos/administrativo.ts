// rev 6 — o bloco de menu do Administrativo
//
// Fica no seu próprio arquivo, e não dentro de `arvore.ts`, porque este módulo
// cresce sem pedir edição da árvore além do import. Quem ainda não tem tela
// (PCO, Notas fiscais, e Equalizar Propostas) cai no `<EmBreve>` do
// `App.tsx`. Sem `breve`, o desenho é o mesmo da tela inicial — ícone da
// marca, seta, barra cheia — em vez da versão apagada.
//
// "COMPRAS" GANHOU `tela` PRÓPRIA (10/09/2026), E CONTINUA COM `sub`
//
//	Pedido do dono: "OCs Inseridas" tem que se abrir DENTRO do próprio card,
//	em dois sub-cards (Processadas/Rejeitadas), "tal qual fazemos em
//	Manutenção › Contrato São Luiz › Orçamentos" — o cartão "Correções", que
//	se abre em quatro ao passar o mouse (`componentes/Painel.tsx`, `filhos`).
//	Isso só existe num painel de DADOS (contador, prévia), não no menu de
//	navegação genérico — por isso "Compras" ganhou `tela: 'compras'`, mesmo
//	tendo `sub`: é o mesmo desenho de `est-raiz` (`arvore.ts`), um nó que é
//	pasta E painel ao mesmo tempo. `sub` continua aqui porque é dele que vem
//	a permissão (`rotina`) e o roteamento de cada filho — `telas/
//	administrativo/Compras.tsx` só decide O DESENHO da barra, nunca quem pode
//	o quê.
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
      tela: 'compras',
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
