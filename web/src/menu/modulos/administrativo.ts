// rev 8 — o bloco de menu do Administrativo
//
// Fica no seu próprio arquivo, e não dentro de `arvore.ts`, porque este módulo
// cresce sem pedir edição da árvore além do import. Quem ainda não tem tela
// nem `breve: true` (hoje só "Equalizar Propostas") cai no `<EmBreve>` do
// `App.tsx`, com o desenho normal (ícone da marca, seta, barra cheia). Quem
// tem `breve: true` (hoje só "Protocolos") aparece já marcado como "Em breve"
// — apagado, sem clique — porque o card existe de propósito, para mostrar o
// próximo passo, mesmo sem tela ainda.
//
// "PCO" GANHOU TELA (10/09/2026) — SEGUNDA DIMENSÃO DE STATUS DA MESMA OC
//
//	Pedido do dono: toda OC processada em Compras nasce "pendente de envio"
//	em PCO, e sai de lá (sem sair de "Processadas") quando o pedido de PCO
//	for enviado ao cliente — fase futura. Ver o cabeçalho de
//	`telas/administrativo/Pco.tsx` para a explicação completa de por que
//	isso NÃO é uma segunda cópia de dados.
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
      tela: 'pco',
      rotina: 'COMPRAS_ORDENS_GERENCIAR',
    },
    {
      t: 'Notas fiscais',
      rota: 'notas-fiscais',
      icone: 'lista',
      desc: 'O recebimento da nota, da obra ao envio ao cliente',
      tela: 'nf',
      // Sem `rotina` única: almoxarife (COMPRAS_NF_RECEBER) e escritório
      // (COMPRAS_NF_ENTREGAR) são pessoas diferentes, cada uma só com a sua —
      // o item precisa aparecer para os dois, ver o comentário em `arvore.ts`.
      rotina: ['COMPRAS_NF_RECEBER', 'COMPRAS_NF_ENTREGAR', 'COMPRAS_NF_CONFIGURAR_ACESSO'],
    },
    {
      // O "Bloco B" (12/09/2026, pedido do dono): o documento de Protocolo —
      // agrupa as notas fiscais entregues por dia e por centro de custo, com
      // numeração sequencial por obra, e o extrato geral do mês. Fica como
      // "Em breve" — `breve: true` — até esse bloco ser construído; o card
      // existe desde já para o dono ver o próximo passo do ciclo OC → NF →
      // Protocolo, do mesmo jeito que "Equalizar Propostas" já marca o que
      // falta em Compras.
      t: 'Protocolos',
      rota: 'protocolos',
      icone: 'lista',
      desc: 'O documento de entrega — notas agrupadas por dia, por obra e por mês',
      breve: true,
    },
  ],
}
