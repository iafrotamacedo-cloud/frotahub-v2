// rev 8 — a árvore de menus
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
import { servicosMenu } from './modulos/servicos'

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

export type Icone =
  | 'chave-inglesa' | 'engrenagem' | 'loja' | 'servicos' | 'pessoas' | 'cadeado' | 'pessoa' | 'lista'
  | 'dinheiro' | 'saida' | 'entrada' | 'balanca' | 'grafico'

/** As rotinas já construídas. Cada nova entra aqui e ganha o seu arquivo em telas/. */
export type Tela =
  | 'usuarios' | 'categorias' | 'minha-conta' | 'trilogo-dados' | 'orcamentos' | 'faturar' | 'a-pagar'
  | 'consolidacao' | 'servicos-hub'
  // AS DOZE DE ESTATÍSTICAS, TODAS COM O PREFIXO `est-`
  //
  //	O prefixo é o que permite a App.tsx despachar a seção inteira num ramo só,
  //	em vez de doze linhas iguais. E os três primeiros são NÓS que também têm
  //	tela: as barras deles mostram número e prévia, e é isso que os distingue
  //	de um menu comum — por isso não podem cair no painel genérico da casca.
  | 'est-raiz' | 'est-operacionais' | 'est-financeiras'
  | 'est-expectativa' | 'est-chamados' | 'est-onde' | 'est-tempo' | 'est-fila'
  | 'est-custos' | 'est-orcamentos' | 'est-faturamento' | 'est-balanco'

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
            t: 'Orçamentos',
            rota: 'orcamentos',
            icone: 'lista',
            desc: 'Notas, rateio, lançamento no Trílogo e as planilhas de controle',
            tela: 'orcamentos',
            rotina: 'CONTRATO_ORCAMENTOS',
          },
          {
            // O MENU EXISTE ANTES DAS TELAS, E É DE PROPÓSITO
            //
            //   As três já aparecem marcadas "em breve": dá a medida do que vem
            //   sem prometer o que ainda não funciona (CORE-23). O que muda em
            //   relação a um menu escondido é que o desenho fica combinado
            //   agora, em vez de ser inventado no dia de construir cada uma.
            //
            //   Sem `rotina` ainda: a linha no catálogo de permissões nasce
            //   junto com a tela que ela protege. Rotina apontando para o vazio
            //   é uma caixa para alguém marcar sem saber o que está liberando.
            t: 'Financeiro',
            rota: 'financeiro',
            icone: 'dinheiro',
            desc: 'O que sai para o fornecedor, o que entra do cliente, e a diferença',
            sub: [
              {
                // A SEGUNDA A GANHAR TELA (26/08/2026)
                //
                //	Por enquanto ela tem UM botão: o pedido de faturamento. O
                //	resto do "a pagar" — conferir o CSV do Obra Prima, marcar o
                //	que foi pago — vem depois, e a tela já nasce sabendo que vai
                //	crescer.
                t: 'A pagar',
                rota: 'a-pagar',
                icone: 'saida',
                desc: 'DAVs e notas do fornecedor: o que foi faturado, o que foi pago e o que está em aberto',
                tela: 'a-pagar',
                rotina: 'CONTRATO_FINANCEIRO_PAGAR',
              },
              {
                // A PRIMEIRA DAS TRÊS A GANHAR TELA (26/08/2026)
                //
                //	"Faturar ao cliente" nasceu como barra dentro de Orçamentos,
                //	e estava no lugar errado: ali dentro tudo é sobre a nota que
                //	a gente COMPRA — ler, amarrar ticket, gerar, lançar. Faturar
                //	é o outro lado do balcão, e o dono mandou movê-la para cá.
                //
                //	Deixá-la onde estava faria alguém procurar cobrança no menu
                //	de compra pelos próximos anos.
                //
                //	E é por isso que a `rotina` aparece agora, e só agora: o
                //	comentário acima dizia que ela nasce junto com a tela que
                //	protege. A tela chegou.
                t: 'A receber',
                rota: 'a-receber',
                icone: 'entrada',
                desc: 'Espelhos de faturamento ao cliente e o que já voltou dele',
                tela: 'faturar',
                rotina: 'CONTRATO_ORCAMENTOS_FATURAR',
              },
              {
                // CONSOLIDAÇÃO, E NÃO MAIS "BALANÇO" (01/09/2026)
                //
                //	Decisão do dono, e ela resolve dois problemas de uma vez. O
                //	nome ficou honesto — esta tela CONFERE dois sistemas um
                //	contra o outro, não fecha um balanço — e a palavra "Balanço"
                //	deixa de aparecer duas vezes no mesmo contrato, que era o
                //	que estava anotado aqui embaixo esperando decisão.
                t: 'Consolidação',
                rota: 'consolidacao',
                icone: 'balanca',
                desc: 'O que se comprou contra o que se cobrou — nota e ticket, nos dois sentidos',
                tela: 'consolidacao',
                rotina: 'CONTRATO_FINANCEIRO_CONSOLIDACAO',
              },
            ],
          },
          {
            // ESTATÍSTICAS — TRÊS NÍVEIS, E OS NÓS TAMBÉM SÃO TELAS
            //
            //	O painel genérico da casca desenha qualquer nó com `sub`, mas
            //	desenha um MENU: título, descrição, seta. Aqui os nós precisam
            //	mostrar número e prévia — "5 lojas com chamado", "121 a lançar" —
            //	que é o que transforma a entrada da seção num painel em vez de
            //	uma lista de links.
            //
            //	Por isso eles têm `sub` E `tela`. A árvore continua sendo a
            //	verdade sobre a navegação (o endereço, o voltar do navegador, a
            //	migalha), e quem desenha as barras é a própria seção.
            //
            //	Uma `rotina` só, no nó de cima: quem alcança as estatísticas do
            //	contrato alcança as nove. Partir isso em nove linhas de permissão
            //	seria pedir que alguém marcasse nove caixas para liberar uma
            //	tela — e a matriz vira um lugar onde ninguém mais olha.
            t: 'Estatísticas',
            rota: 'estatisticas',
            icone: 'grafico',
            desc: 'Chamados, tempo de atendimento, custos e faturamento do contrato',
            tela: 'est-raiz',
            rotina: 'CONTRATO_ESTATISTICAS',
            sub: [
              {
                t: 'Operacionais',
                rota: 'operacionais',
                icone: 'chave-inglesa',
                desc: 'Chamados, onde, tempo de atendimento e a fila de hoje',
                tela: 'est-operacionais',
                sub: [
                  {
                    t: 'Expectativa × Realidade',
                    rota: 'expectativa',
                    icone: 'grafico',
                    desc: 'O plano de manutenção preventiva contra os chamados abertos, dia a dia',
                    tela: 'est-expectativa',
                  },
                  {
                    t: 'Chamados',
                    rota: 'chamados',
                    icone: 'lista',
                    desc: 'Quantos entraram, quantos foram resolvidos e como estão hoje',
                    tela: 'est-chamados',
                  },
                  {
                    t: 'Onde',
                    rota: 'onde',
                    icone: 'loja',
                    desc: 'Em que lojas e em que lugares os chamados acontecem',
                    tela: 'est-onde',
                  },
                  {
                    t: 'Tempo de atendimento',
                    rota: 'tempo',
                    icone: 'servicos',
                    desc: 'Quanto tempo leva para resolver um chamado',
                    tela: 'est-tempo',
                  },
                  {
                    t: 'Fila de hoje',
                    rota: 'fila',
                    icone: 'lista',
                    desc: 'O que está em aberto agora (não depende do período)',
                    tela: 'est-fila',
                  },
                ],
              },
              {
                t: 'Financeiras',
                rota: 'financeiras',
                icone: 'dinheiro',
                desc: 'Custos, orçamentos, faturamento e balanço',
                tela: 'est-financeiras',
                sub: [
                  {
                    t: 'Custos no Trílogo',
                    rota: 'custos',
                    icone: 'dinheiro',
                    desc: 'Quanto foi lançado nos chamados, e onde',
                    tela: 'est-custos',
                  },
                  {
                    t: 'Orçamentos',
                    rota: 'orcamentos',
                    icone: 'lista',
                    desc: 'O que espera lançamento, o que travou e o que já foi',
                    tela: 'est-orcamentos',
                  },
                  {
                    t: 'Faturamento ao cliente',
                    rota: 'faturamento',
                    icone: 'entrada',
                    desc: 'O que foi cobrado do cliente e o que já voltou',
                    tela: 'est-faturamento',
                  },
                  {
                    // A PALAVRA FICOU LIVRE (01/09/2026)
                    //
                    //	O "Balanço" de Financeiro virou Consolidação, então este
                    //	deixa de ser homônimo. Os dois convivem porque são
                    //	trabalhos diferentes: lá se CONFERE nota contra ticket;
                    //	aqui se MEDE — quanto se pagou, quanto se cobrou, qual
                    //	foi a margem.
                    t: 'Balanço',
                    rota: 'balanco',
                    icone: 'balanca',
                    desc: 'O que se paga ao fornecedor, o que se cobra do cliente, e a margem',
                    tela: 'est-balanco',
                  },
                ],
              },
            ],
          },
        ],
      },
      {
        // SUBIU UM NÍVEL (04/09/2026)
        //
        //	Morava dentro de "Contrato São Luiz", como o primeiro dos quatro
        //	cards. O dono pediu pra subir: hoje é clique direto na barra
        //	lateral, sem passar pelo painel do contrato antes.
        t: 'Dados do Trílogo',
        rota: 'dados-trilogo',
        icone: 'lista',
        desc: 'Os chamados do contrato, como o robô os trouxe',
        tela: 'trilogo-dados',
        rotina: 'CONTRATO_TRILOGO_DADOS',
      },
      servicosMenu,
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
