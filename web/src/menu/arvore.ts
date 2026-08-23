// rev 2 — a árvore de menus
//
// Um item com `breve: true` aparece desabilitado, para dar a medida do que falta.
// Um item com `tela` abre uma rotina construída. Um item com `soBuilder` só existe
// para o dono do sistema — o menu se ajusta ao login (P-17).
export interface ItemMenu {
  t: string
  icone: Icone
  desc?: string
  breve?: boolean
  soBuilder?: boolean
  tela?: Tela
  sub?: ItemMenu[]
}

export type Icone = 'chave-inglesa' | 'engrenagem' | 'loja' | 'servicos' | 'pessoas' | 'cadeado'

/** As rotinas já construídas. Cada nova entra aqui e ganha o seu arquivo em telas/. */
export type Tela = 'usuarios'

const ARVORE_COMPLETA: ItemMenu[] = [
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
    sub: [
      {
        t: 'Usuários e Logins',
        icone: 'pessoas',
        desc: 'Quem entra no sistema e em que categoria',
        tela: 'usuarios',
        soBuilder: true,
      },
      {
        t: 'Categorias',
        icone: 'cadeado',
        desc: 'Os grupos de acesso e o que cada um alcança',
        breve: true,
        soBuilder: true,
      },
    ],
  },
]

/**
 * A árvore que ESTE login enxerga.
 *
 * Um bloco cujos filhos todos sumiram some junto: menu com pasta vazia é pior que
 * menu sem a pasta — parece defeito.
 */
export function arvoreVisivel(ehBuilder: boolean): ItemMenu[] {
  function filtrar(itens: ItemMenu[]): ItemMenu[] {
    const fora: ItemMenu[] = []
    for (const item of itens) {
      if (item.soBuilder && !ehBuilder) continue
      if (item.sub) {
        const sub = filtrar(item.sub)
        if (sub.length === 0) continue
        fora.push({ ...item, sub })
      } else {
        fora.push(item)
      }
    }
    return fora
  }
  return filtrar(ARVORE_COMPLETA)
}
