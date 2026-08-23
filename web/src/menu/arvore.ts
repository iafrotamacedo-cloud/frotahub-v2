// rev 1 — a árvore de menus
//
// Um item com `breve: true` aparece desabilitado, para dar a medida do que falta
// (CORE-23). A árvore completa do sistema está no diário; aqui entra só o que já
// foi construído ou já foi decidido que existe.
export interface ItemMenu {
  t: string
  icone: Icone
  desc?: string
  breve?: boolean
  sub?: ItemMenu[]
}

export type Icone = 'chave-inglesa' | 'engrenagem' | 'loja' | 'servicos'

export const ARVORE: ItemMenu[] = [
  {
    t: 'Manutenção',
    icone: 'chave-inglesa',
    desc: 'Contratos, chamados e serviços',
    sub: [
      { t: 'Contrato São Luiz', icone: 'loja', desc: 'Chamados, orçamentos e preventiva', breve: true },
      { t: 'Serviços', icone: 'servicos', desc: 'Serviços avulsos e outros contratos', breve: true },
    ],
  },
  {
    t: 'Configurações',
    icone: 'engrenagem',
    desc: 'Usuários, permissões e ajustes do sistema',
    breve: true,
  },
]
