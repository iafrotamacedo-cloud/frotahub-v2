// rev 6 — a árvore de menus
//
// Um item com `breve: true` aparece desabilitado, para dar a medida do que falta.
// Um item com `tela` abre uma rotina construída. Um item com `soBuilder` só existe
// para o dono do sistema — o menu se ajusta ao login (P-17).
//
// SOBRE O CAMPO `rota`
//   É o pedaço que aparece no endereço do navegador. Ele é escrito à mão, e não
//   derivado do título, de propósito: título é texto de tela e muda quando alguém
//   acha uma palavra melhor. Se o endereço acompanhasse o título, todo favorito e
//   todo link colado numa conversa apontariam para o vazio no dia seguinte.
export interface ItemMenu {
  t: string
  /** O pedaço deste item no endereço. Curto, sem acento, e ESTÁVEL. */
  rota: string
  icone: Icone
  desc?: string
  breve?: boolean
  soBuilder?: boolean
  /**
   * O código no catálogo de permissões. Item com `rotina` só aparece para quem
   * a alcança — o menu se ajusta ao login (P-17).
   *
   * `soBuilder` continua existindo para o que NÃO passa pela matriz: mexer em
   * login é exclusividade do dono, e isso não é uma linha de permissão que
   * alguém possa marcar por engano.
   */
  rotina?: string
  tela?: Tela
  sub?: ItemMenu[]
}

export type Icone = 'chave-inglesa' | 'engrenagem' | 'loja' | 'servicos' | 'pessoas' | 'cadeado' | 'pessoa' | 'lista'

/** As rotinas já construídas. Cada nova entra aqui e ganha o seu arquivo em telas/. */
export type Tela = 'usuarios' | 'categorias' | 'minha-conta' | 'trilogo-dados'

const ARVORE_COMPLETA: ItemMenu[] = [
  {
    t: 'Manutenção',
    rota: 'manutencao',
    icone: 'chave-inglesa',
    desc: 'Contratos, chamados e serviços',
    sub: [
      {
        t: 'Contrato São Luiz',
        rota: 'contrato-sao-luiz',
        icone: 'loja',
        desc: 'Chamados, orçamentos e preventiva',
        sub: [
          {
            t: 'Dados do Trílogo',
            rota: 'dados-trilogo',
            icone: 'lista',
            desc: 'Os chamados do contrato, como o robô os trouxe',
            tela: 'trilogo-dados',
            rotina: 'CONTRATO_TRILOGO_DADOS',
          },
        ],
      },
      { t: 'Serviços', rota: 'servicos', icone: 'servicos', desc: 'Serviços avulsos e outros contratos', breve: true },
    ],
  },
  {
    t: 'Configurações',
    rota: 'configuracoes',
    icone: 'engrenagem',
    desc: 'Usuários, permissões e ajustes do sistema',
    sub: [
      {
        t: 'Usuários e Logins',
        rota: 'usuarios',
        icone: 'pessoas',
        desc: 'Quem entra no sistema e em que categoria',
        tela: 'usuarios',
        soBuilder: true,
      },
      {
        t: 'Categorias',
        rota: 'categorias',
        icone: 'cadeado',
        desc: 'Os grupos de acesso e o que cada um alcança',
        tela: 'categorias',
        soBuilder: true,
      },
      {
        // Sem `soBuilder`: é a porta de todo mundo. E é ela que faz Configurações
        // existir para quem não é builder — antes, tudo ali dentro era do dono do
        // sistema, então o bloco inteiro sumia, e quem quisesse trocar a própria
        // senha não tinha para onde ir.
        t: 'Minha conta',
        rota: 'minha-conta',
        icone: 'pessoa',
        desc: 'Os seus dados e a sua senha',
        tela: 'minha-conta',
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
export function arvoreVisivel(ehBuilder: boolean, rotinas: readonly string[] = []): ItemMenu[] {
  const alcanca = new Set(rotinas)

  function filtrar(itens: ItemMenu[]): ItemMenu[] {
    const fora: ItemMenu[] = []
    for (const item of itens) {
      if (item.soBuilder && !ehBuilder) continue
      // O builder passa sempre, aconteça o que acontecer com a matriz — é a
      // garantia de que uma configuração errada não tranca o dono para fora.
      if (item.rotina && !ehBuilder && !alcanca.has(item.rotina)) continue
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
