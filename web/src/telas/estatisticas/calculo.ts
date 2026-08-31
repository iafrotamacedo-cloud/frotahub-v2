// rev 1 — a conta das estatísticas
//
// FUNÇÕES PURAS: linhas + filtros entram, números e séries saem. Nada aqui sabe
// o que é um <div>, e nada aqui vai ao servidor.
//
// POR QUE A CONTA MORA AQUI E NÃO NO SQL
//
//	Porque ela é UMA (CORE-06). Escrita metade em view e metade aqui, as duas
//	metades discordariam no primeiro mês — e discordar em estatística não dá
//	erro, dá um número diferente que ninguém sabe qual é o certo.
//
//	Este arquivo é a tradução literal do que rodou offline sobre o retrato de
//	30/08/2026 e foi conferido contra ele. A tradução mudou UMA coisa, e está
//	comentada onde acontece: a loja passou a ser achada pelo número do Trílogo,
//	e não pela posição numa lista (ver `dados.ts`).
//
// AS DEFINIÇÕES, NAS PALAVRAS DO DONO (30/08/2026)
//
//	Chamado ABERTO é o aberto pela PRIMEIRA vez. Chamado ATENDIDO é o que entrou
//	em Executado pela PRIMEIRA vez. Retrabalho não conta duas vezes — vale sempre
//	o primeiro instante de cada marco, senão um chamado devolvido e refeito
//	aparece como dois na conta.
import type { Base, Chamado, Custo, Evento } from './dados'
import { CONTA, DIA, data, hoje, iso, mesCurto, mesDe } from './formato'

// ---------------------------------------------------------------------------
// ferramentas
// ---------------------------------------------------------------------------

export const pct = (a: number, b: number): number | null => (b ? Math.round((a / b) * 100) : null)
const emDias = (ms: number): number => ms / DIA

export const mediana = (xs: number[]): number | null => {
  if (!xs.length) return null
  const s = [...xs].sort((a, b) => a - b)
  const m = s.length >> 1
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2
}

export const media = (xs: number[]): number | null =>
  xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null

export function soma<T>(xs: readonly T[], f: (x: T) => number | null | undefined): number {
  return xs.reduce((a, x) => a + (f(x) || 0), 0)
}

function contar<T>(xs: readonly T[], f: (x: T) => string | null): Map<string, number> {
  const m = new Map<string, number>()
  for (const x of xs) {
    const k = f(x)
    if (k == null) continue
    m.set(k, (m.get(k) || 0) + 1)
  }
  return m
}

function somar<T>(xs: readonly T[], chave: (x: T) => string | null, valor: (x: T) => number | null | undefined): Map<string, number> {
  const m = new Map<string, number>()
  for (const x of xs) {
    const k = chave(x)
    if (k == null) continue
    m.set(k, (m.get(k) || 0) + (valor(x) || 0))
  }
  return m
}

/** Um par rótulo/valor, ordenado do maior para o menor. É o que os gráficos de barra comem. */
export type Par = [string, number]

const ordenado = (m: Map<string, number>, n = 10): Par[] =>
  [...m.entries()].sort((a, b) => b[1] - a[1]).slice(0, n)

/** Os meses entre duas datas, inclusive — para o gráfico não pular mês vazio. */
function mesesEntre(de: Date, ate: Date): string[] {
  const fora: string[] = []
  const d = new Date(de.getFullYear(), de.getMonth(), 1)
  while (d <= ate) {
    fora.push(d.toISOString().slice(0, 7))
    d.setMonth(d.getMonth() + 1)
  }
  return fora
}

/** O último pedaço do caminho: "LOJA 20 > Área interna > Térreo > Frente de loja" → "Frente de loja". */
const localDo = (amb: string | null): string | null => {
  if (!amb) return null
  const partes = amb.split('>').map(s => s.trim()).filter(Boolean)
  return partes.length > 1 ? partes[partes.length - 1] : null
}

/**
 * O filtro da tela. É UM, e todas as abas o aplicam.
 *
 * Cada aba filtra a própria tabela pela data que faz sentido nela — abertura do
 * chamado, data do custo, criação do orçamento — mas conta e loja valem igual
 * em todas, senão duas abas mostrariam recortes diferentes com o mesmo filtro
 * na tela.
 */
export interface Filtro {
  de: Date
  ate: Date
  conta: string
  loja: number | null
}

interface TemConta { conta: string | null }
interface TemUnidade { unidade: number | null }

function noPeriodo(f: Filtro, s: string | null | undefined): boolean {
  const d = data(s)
  if (!d) return false
  return d >= f.de && d <= f.ate
}
const daConta = (f: Filtro, r: TemConta): boolean => !f.conta || r.conta === f.conta
const daLoja = (f: Filtro, r: TemUnidade): boolean => f.loja == null || r.unidade === f.loja

/** A linha do tempo de cada chamado, resumida: o PRIMEIRO instante de cada status. */
export type Marcos = Record<number, Date>

function marcosPorChamado(eventos: readonly Evento[]): Map<number, Marcos> {
  const m = new Map<number, Marcos>()
  for (const e of eventos) {
    let x = m.get(e.chamado)
    if (!x) { x = {}; m.set(e.chamado, x) }
    const q = data(e.quando)
    if (!q) continue
    // Um chamado reaberto volta a "Em execução" — a PRIMEIRA execução é a que
    // mede o tempo de resposta.
    if (!x[e.sc] || q < x[e.sc]) x[e.sc] = q
  }
  return m
}

// ---------------------------------------------------------------------------
// expectativa × realidade — o plano de preventiva contra os chamados
// ---------------------------------------------------------------------------
//
// O PLANO (MSL_Plano_Preventiva_EXTERNO.pdf, cap. 6, Figura 1)
//	Índice em que 100 = patamar de demanda do regime atual. Julho/2026 é o
//	início do contrato; a 2ª quinzena de agosto é o pico do plano de
//	contingência (~120); de setembro em diante a preventiva de rotina derruba a
//	demanda em curva exponencial até −40% em mai/2028, na faixa de −27% a −53%.
//
// O PATAMAR (decisão do dono, 30/08/2026)
//	100% = 700 chamados abertos por mês. É o número do plano, e é contra o plano
//	que se mede. Fica em PATAMAR_MES, num lugar só.

export const PATAMAR_MES = 700
export const INICIO_CONTRATO = '2026-07-01'
export const FIM_DO_PLANO = '2028-05-31'

/** [data, índice esperado]; entre dois pontos a curva é interpolada por dia. */
const CURVA_DO_PLANO: Par[] = [
  ['2026-07-01', 100], ['2026-07-31', 106], ['2026-08-10', 116], ['2026-08-20', 120], ['2026-08-31', 118],
  ['2026-09-01', 100], ['2026-10-01', 92], ['2026-11-01', 86], ['2026-12-01', 81],
  ['2027-01-01', 77], ['2027-02-01', 74], ['2027-03-01', 71], ['2027-04-01', 69], ['2027-05-01', 68],
  ['2027-06-01', 66.5], ['2027-07-01', 65], ['2027-08-01', 64], ['2027-09-01', 63], ['2027-10-01', 62.5],
  ['2027-11-01', 62], ['2027-12-01', 61.5], ['2028-01-01', 61], ['2028-02-01', 60.7], ['2028-03-01', 60.4],
  ['2028-04-01', 60.2], ['2028-05-31', 60],
]

// A faixa: no fim do ciclo a queda esperada é 40%, entre 27% (pior) e 53%
// (melhor). Ela cresce junto com a queda: onde o plano já caiu X, a faixa vai
// de X·27/40 a X·53/40.
const FAIXA = { pior: 27 / 40, melhor: 53 / 40 }

/**
 * O dia de Fortaleza (UTC−3) de um instante.
 *
 * Quem sabe LER instante é `data()`, e só ela — antes esta função tinha a sua
 * própria regra (colar um `Z`), e foi por aí que a tela caiu em 31/08/2026.
 * Instante que não dá para ler devolve `null`, e quem chama decide o que fazer.
 */
export const diaLocal = (s: string | null | undefined): string | null => {
  const d = data(s)
  if (!d) return null
  d.setUTCHours(d.getUTCHours() - 3)
  return d.toISOString().slice(0, 10)
}

export const diaMais = (s: string, n: number): string => {
  const d = new Date(s + 'T12:00:00Z')
  d.setUTCDate(d.getUTCDate() + n)
  return d.toISOString().slice(0, 10)
}

export const diasEntre = (a: string, b: string): number =>
  Math.round((new Date(b + 'T12:00:00Z').getTime() - new Date(a + 'T12:00:00Z').getTime()) / DIA)

/** O índice esperado num dia qualquer, interpolando a curva. */
export function esperadoEm(dia: string): number {
  const c = CURVA_DO_PLANO
  if (dia <= c[0][0]) return c[0][1]
  for (let i = 1; i < c.length; i++) {
    if (dia <= c[i][0]) {
      const [d0, v0] = c[i - 1]
      const [d1, v1] = c[i]
      const t = diasEntre(d0, dia) / Math.max(1, diasEntre(d0, d1))
      return v0 + (v1 - v0) * t
    }
  }
  return c[c.length - 1][1]
}

export function faixaEm(dia: string): [number, number] {
  const e = esperadoEm(dia)
  const queda = Math.max(0, 100 - e)
  // Antes de setembro não há faixa: o plano de contingência é uma linha só.
  if (dia < '2026-09-01' || queda === 0) return [e, e]
  return [100 - queda * FAIXA.melhor, 100 - queda * FAIXA.pior]
}

export interface PontoDaSerie { dia: string; soma: number; indice: number }

export interface LojaDaExpectativa {
  unidade: number
  nome: string
  total: number
  patamar: number
  real: PontoDaSerie[]
  ultimo: PontoDaSerie | null
}

export interface MesDaExpectativa {
  mes: string
  abertos: number
  parcial: boolean
  indice: number
  projetado: number
  esperado: number
}

export interface Expectativa {
  hoje: string
  real: PontoDaSerie[]
  ultimo: PontoDaSerie | null
  esperadoHoje: number
  porMes: MesDaExpectativa[]
  patamar: number
  lojas: LojaDaExpectativa[]
  totalContrato: number
  diasDeContrato: number
}

function calcularExpectativa(base: Base): Expectativa {
  const { chamados, porUnidade } = base
  const ate = diaLocal(base.geradoEm) ?? iso(new Date())

  // Toda a rede, as duas contas, só as unidades do plano (`no_escopo`). O
  // chamado conta no dia em que foi ABERTO pela primeira vez — a demanda que o
  // plano quer reduzir.
  const porDia = new Map<string, number>()
  const porLojaDia = new Map<number, Map<string, number>>()
  const totalLoja = new Map<number, number>()
  for (const c of chamados) {
    // AQUI ESTÁ A ÚNICA MUDANÇA DA FUSÃO: a loja é achada pelo número do
    // Trílogo, não pela posição numa lista (ver `dados.ts`).
    if (c.unidade == null || !porUnidade.get(c.unidade)?.no_escopo || !c.criado_em) continue
    const d = diaLocal(c.criado_em)
    if (!d || d < INICIO_CONTRATO) continue
    porDia.set(d, (porDia.get(d) || 0) + 1)
    let m = porLojaDia.get(c.unidade)
    if (!m) { m = new Map<string, number>(); porLojaDia.set(c.unidade, m) }
    m.set(d, (m.get(d) || 0) + 1)
    totalLoja.set(c.unidade, (totalLoja.get(c.unidade) || 0) + 1)
  }

  const dias: string[] = []
  for (let d = INICIO_CONTRATO; d <= ate; d = diaMais(d, 1)) dias.push(d)
  const totalRede = [...porDia.values()].reduce((a, b) => a + b, 0)

  /** Série de 30 dias móveis ÷ patamar, a partir de um mapa dia→n. */
  const serie = (mapa: Map<string, number>, patamar: number): PontoDaSerie[] => {
    const fora: PontoDaSerie[] = []
    let janela = 0
    dias.forEach((d, i) => {
      janela += mapa.get(d) || 0
      if (i >= 30) janela -= mapa.get(dias[i - 30]) || 0
      if (i >= 29) fora.push({ dia: d, soma: janela, indice: (janela / patamar) * 100 })
    })
    return fora
  }

  const real = serie(porDia, PATAMAR_MES)

  // O PATAMAR DE CADA LOJA é a fatia dela no patamar da rede, pela participação
  // nas aberturas do contrato até hoje: patamar_loja = 700 × (aberturas da loja
  // ÷ aberturas da rede). Assim o índice da loja fica na MESMA régua do plano
  // (100% = o "normal" dela) e a soma das fatias fecha os 700. Loja pequena
  // (patamar de 5 a 15 por mês) tem linha mais nervosa — é o tamanho da amostra.
  const lojas: LojaDaExpectativa[] = [...totalLoja.entries()].map(([u, tot]) => {
    const patamar = PATAMAR_MES * (tot / Math.max(1, totalRede))
    const r = serie(porLojaDia.get(u) ?? new Map<string, number>(), patamar)
    return {
      unidade: u,
      nome: porUnidade.get(u)?.nome || '—',
      total: tot,
      patamar,
      real: r,
      ultimo: r[r.length - 1] || null,
    }
  }).sort((a, b) => b.total - a.total)

  // Mês a mês, para a tabela: total aberto no mês contra o que o plano esperava
  // no fim dele.
  const meses = new Map<string, number>()
  for (const [d, n] of porDia) meses.set(d.slice(0, 7), (meses.get(d.slice(0, 7)) || 0) + n)
  const porMes: MesDaExpectativa[] = [...meses.entries()].sort().map(([m, n]) => {
    const proximoMes = diaMais(m + '-01', 31).slice(0, 7)
    const fim = m === ate.slice(0, 7) ? ate
      : proximoMes === m ? m + '-31'
        : diaMais(proximoMes + '-01', -1)
    const parcial = m === ate.slice(0, 7)
    const diasNoMes = parcial ? diasEntre(m + '-01', ate) + 1 : diasEntre(m + '-01', fim) + 1
    return {
      mes: m,
      abertos: n,
      parcial,
      indice: (n / PATAMAR_MES) * 100,
      projetado: parcial ? (n / diasNoMes) * 30 : n,
      esperado: esperadoEm(fim),
    }
  })

  return {
    hoje: ate,
    real,
    ultimo: real[real.length - 1] || null,
    esperadoHoje: esperadoEm(ate),
    porMes,
    patamar: PATAMAR_MES,
    lojas,
    totalContrato: totalRede,
    diasDeContrato: dias.length,
  }
}

// ---------------------------------------------------------------------------
// a conta de cada aba
// ---------------------------------------------------------------------------

export interface SerieDeMes { rotulo: string; instalacoes: number; civil: number }
export interface SerieAbertoExecutado { rotulo: string; abertos: number; executados: number }
export interface SerieGeradoLancado { rotulo: string; gerados: number; lancados: number }
export interface SerieBalanco { rotulo: string; pagar: number; receber: number; margem: number }
export interface Faixa { rotulo: string; valor: number }

export interface LojaLenta { loja: string; n: number; media: number | null; mediana: number | null }
export interface TempoPorConta { rotulo: string; media: number | null; mediana: number | null; n: number }
export interface ChamadoAntigo extends Chamado { loja: string; idade: number }
export interface CicloResumo {
  id: string; competencia: string; ate: string | null; enviado_em: string | null; fechado_em: string | null
  faturado: number; recebido: number; emAberto: number; notas: number; comNF: number; comPCO: number
}
export interface FaturaDaLista {
  id: string; loja: string; ciclo: string | undefined; conta: string | null
  pco_numero: string | null; nf_numero: string | null; nf_em: string | null
  recebido_em: string | null; valor_recebido: number | null; valor_faturado: number | null
}
export interface BalancoDeLoja { loja: string; pagar: number; receber: number; margem: number; n: number }

/** Um custo do Trílogo com o chamado dele ao lado, e a data que vale. */
interface CustoComChamado extends Custo {
  ch: Chamado
  quando: string | null
  semData: boolean
}

export interface Resultado {
  nomeLoja: (id: number | null) => string
  exp: Expectativa
  chamados: {
    total: number; noPeriodo: number; abertosAgora: number; emExecucao: number
    executados: number; vistoriados: number; pctPrazo: number | null; comPrazo: number
    porMes: SerieDeMes[]; porMesExec: SerieAbertoExecutado[]
    porStatus: Par[]; porPrioridade: Par[]; porLoja: Par[]; porLocal: Par[]
    lojasComChamado: number
  }
  tempo: {
    n: number; pctAte7: number | null
    mediaExec: number | null; medianaExec: number | null
    mediaVist: number | null; medianaVist: number | null
    mediaExecucao: number | null; medianaExecucao: number | null
    mediaExecAteVist: number | null; medianaExecAteVist: number | null
    porConta: TempoPorConta[]
    histograma: Faixa[]; backlog: Faixa[]; lojasLentas: LojaLenta[]
    maisAntigos: ChamadoAntigo[]; atrasados: number
  }
  custos: {
    valorLancado: number; n: number; semData: number; ticketMedio: number | null
    custoPorMes: SerieDeMes[]; custoPorLoja: Par[]; custoPorTipo: Par[]
    fechados: number; fechadosComCusto: number; pctComCusto: number | null
    gerados: number; valorGerados: number; aguardando: number
    lancados: number; valorLancados: number
    bloqueados: number; porBloqueio: Par[]; noTeto: number; teto: number | null | undefined
    valorNota: number; valorCobrado: number; margem: number | null
    rateados: number; diretos: number
    orcPorStatus: Par[]; orcPorMes: SerieGeradoLancado[]
    semTicket: number; ticketSolto: number
    docsBloqueados: number; docsFalhou: number; docPorFila: Par[]; docs: number
  }
  fat: {
    ciclosResumo: CicloResumo[]; faturasLoja: FaturaDaLista[]
    totalFaturado: number; totalRecebido: number; emAberto: number
    aPagarAberto: number; pagoTotal: number; naoFaturado: number; valorNaoFaturado: number
    balancoPorMes: SerieBalanco[]; balancoLoja: BalancoDeLoja[]
    margemPeriodo: number; receberPeriodo: number; pagarPeriodo: number
  }
}

const NOME_BLOQUEIO: Record<string, string> = {
  ticket_status: 'Ticket sem status de executado',
  ticket_recusado: 'Ticket recusado',
  teto: 'Passou do teto',
  possivel_duplicata: 'Possível duplicata',
  sem_empresa: 'Sem empresa',
  trilogo_fora: 'Trílogo fora do ar',
  desconhecido: 'Desconhecido',
}

const NOME_STATUS_ORC: Record<string, string> = {
  gerado: 'Gerado (a lançar)',
  aguardando_aprovacao: 'Aguardando aprovação',
  lancado: 'Lançado',
}

const NOME_FILA: Record<string, string> = { orcamento: 'Orçamento', rateio: 'Rateio', direto: 'Direto' }

export function calcular(base: Base, f: Filtro): Resultado {
  const { chamados, eventos, custos, orcamentos, faturas, ciclos, documentos, porUnidade, parametros } = base

  // A loja pelo número do Trílogo — a mudança da fusão (ver `dados.ts`).
  const nomeLoja = (id: number | null): string =>
    id == null ? 'Sem loja' : porUnidade.get(id)?.nome || 'Sem loja'

  const marcos = lembrar(eventos, () => marcosPorChamado(eventos))
  const porNumero = new Map(chamados.map(c => [c.numero, c]))
  const meses = mesesEntre(f.de, f.ate)

  // ---------- 1. CHAMADOS ----------
  const ch = chamados.filter(c => daConta(f, c) && daLoja(f, c))
  const chPeriodo = ch.filter(c => noPeriodo(f, c.criado_em))
  const abertosAgora = ch.filter(c => c.status === 'Aberto' || c.status === 'Em execução')
  const noMarco = (c: Chamado, sc: number): Date | null => marcos.get(c.numero)?.[sc] ?? null
  const dentro = (d: Date | null): boolean => !!d && d >= f.de && d <= f.ate
  const executadosNoPeriodo = ch.filter(c => dentro(noMarco(c, 5)))
  const vistoriadosNoPeriodo = ch.filter(c => dentro(noMarco(c, 6)))
  const comPrazo = executadosNoPeriodo.filter(c => c.prazo)
  const noPrazo = comPrazo.filter(c => {
    const p = data(c.prazo)
    if (!p) return false
    p.setHours(23, 59, 59)
    const exec = noMarco(c, 5)
    return !!exec && exec <= p
  })

  const porMes: SerieDeMes[] = meses.map(m => ({
    rotulo: mesCurto(m),
    instalacoes: chPeriodo.filter(c => c.conta === 'instalacoes' && mesDe(c.criado_em) === m).length,
    civil: chPeriodo.filter(c => c.conta === 'civil' && mesDe(c.criado_em) === m).length,
  }))
  const porMesExec: SerieAbertoExecutado[] = meses.map(m => ({
    rotulo: mesCurto(m),
    abertos: chPeriodo.filter(c => mesDe(c.criado_em) === m).length,
    executados: executadosNoPeriodo.filter(c => {
      const e = noMarco(c, 5)
      return !!e && iso(e).slice(0, 7) === m
    }).length,
  }))
  const porStatus = ordenado(contar(chPeriodo, c => c.status || 'Sem status'), 8)
  const porPrioridade = ordenado(contar(chPeriodo, c => c.prioridade || 'Sem prioridade'), 6)
  const porLoja = ordenado(contar(chPeriodo, c => nomeLoja(c.unidade)), 12)
  const porLocal = ordenado(contar(chPeriodo, c => localDo(c.ambiente)), 10)

  // ---------- 2. TEMPO ----------
  const durAteExec: number[] = []
  const durAteVist: number[] = []
  const durExecucao: number[] = []
  const durExecAteVist: number[] = []
  const porContaTempo: Record<string, number[]> = { instalacoes: [], civil: [] }
  const porLojaTempo = new Map<string, number[]>()

  for (const c of executadosNoPeriodo) {
    const exec = noMarco(c, 5)
    if (!exec) continue
    const abertura = noMarco(c, 1) || data(c.criado_em)
    if (!abertura) continue
    const d = emDias(exec.getTime() - abertura.getTime())
    if (d < 0) continue
    durAteExec.push(d)
    const conta = c.conta ?? 'sem conta'
    ;(porContaTempo[conta] || (porContaTempo[conta] = [])).push(d)
    const L = nomeLoja(c.unidade)
    if (!porLojaTempo.has(L)) porLojaTempo.set(L, [])
    porLojaTempo.get(L)!.push(d)
    const emExec = noMarco(c, 7)
    if (emExec && exec >= emExec) durExecucao.push(emDias(exec.getTime() - emExec.getTime()))
    const vist = noMarco(c, 6)
    if (vist && vist >= exec) durExecAteVist.push(emDias(vist.getTime() - exec.getTime()))
  }
  for (const c of vistoriadosNoPeriodo) {
    const vist = noMarco(c, 6)
    const abertura = noMarco(c, 1) || data(c.criado_em)
    if (!vist || !abertura) continue
    const d = emDias(vist.getTime() - abertura.getTime())
    if (d >= 0) durAteVist.push(d)
  }

  const faixas: [string, number, number][] = [
    ['até 1 dia', 0, 1], ['1 a 3', 1, 3], ['3 a 7', 3, 7],
    ['7 a 15', 7, 15], ['15 a 30', 15, 30], ['mais de 30', 30, 1e9],
  ]
  const ate7 = durAteExec.filter(d => d < 7).length
  const histograma: Faixa[] = faixas.map(([r, a, b]) => ({
    rotulo: r, valor: durAteExec.filter(d => d >= a && d < b).length,
  }))
  const agora = hoje()
  const idade = (c: Chamado): number => {
    const d = data(c.criado_em)
    return d ? emDias(agora.getTime() - d.getTime()) : 0
  }
  const backlog: Faixa[] = faixas.map(([r, a, b]) => ({
    rotulo: r, valor: abertosAgora.filter(c => idade(c) >= a && idade(c) < b).length,
  }))
  const lojasLentas: LojaLenta[] = [...porLojaTempo.entries()]
    .filter(([, xs]) => xs.length >= 3)
    .map(([l, xs]) => ({ loja: l, n: xs.length, media: media(xs), mediana: mediana(xs) }))
    .sort((a, b) => (b.mediana ?? 0) - (a.mediana ?? 0))
    .slice(0, 12)
  const maisAntigos = [...abertosAgora]
    .sort((a, b) => (data(a.criado_em)?.getTime() ?? 0) - (data(b.criado_em)?.getTime() ?? 0))
    .slice(0, 10)

  // ---------- 3. CUSTOS E ORÇAMENTOS ----------
  //
  // O Trílogo não traz a data de boa parte dos custos (600 dos 718 na carga
  // inicial). Sem data, o custo vale na data em que o chamado foi EXECUTADO —
  // é quando o material foi usado — e, sem execução, na abertura do chamado.
  // A tela diz quantos foram datados assim, para ninguém tomar a série por
  // exata.
  const cu: CustoComChamado[] = []
  for (const k of custos) {
    const dono = porNumero.get(k.chamado)
    if (!dono) continue
    if (!daConta(f, dono) || !daLoja(f, dono)) continue
    const exec = marcos.get(k.chamado)?.[5]
    // ISO inteira, com o `Z`: cortar o fuso aqui seria fabricar outra vez a
    // forma ambígua que derrubou a tela.
    const quando = k.criado_em || (exec ? exec.toISOString() : dono.criado_em)
    cu.push({ ...k, ch: dono, quando, semData: !k.criado_em })
  }
  const cuPeriodo = cu.filter(k => noPeriodo(f, k.quando))
  const valorLancado = soma(cuPeriodo, k => k.valor)
  const custoPorMes: SerieDeMes[] = meses.map(m => ({
    rotulo: mesCurto(m),
    instalacoes: soma(cuPeriodo.filter(k => k.ch.conta === 'instalacoes' && mesDe(k.quando) === m), k => k.valor),
    civil: soma(cuPeriodo.filter(k => k.ch.conta === 'civil' && mesDe(k.quando) === m), k => k.valor),
  }))
  const custoPorLoja = ordenado(somar(cuPeriodo, k => nomeLoja(k.ch.unidade), k => k.valor), 12)
  const custoPorTipo = ordenado(somar(cuPeriodo, k => k.tipo, k => k.valor), 5)
  const chamadosComCusto = new Set(cu.map(k => k.chamado))
  const fechados = executadosNoPeriodo.length
  const fechadosComCusto = executadosNoPeriodo.filter(c => chamadosComCusto.has(c.numero)).length

  const vivos = orcamentos.filter(o => o.status !== 'removido' && daConta(f, o) && daLoja(f, o))
  const orPeriodo = vivos.filter(o => noPeriodo(f, o.criado_em))
  const gerados = vivos.filter(o => o.status === 'gerado')
  const aguardando = vivos.filter(o => o.status === 'aguardando_aprovacao')
  const lancados = orPeriodo.filter(o => o.status === 'lancado')
  const bloqueados = gerados.filter(o => o.lancamento_bloqueio)
  const porBloqueio = ordenado(
    contar(bloqueados, o => NOME_BLOQUEIO[o.lancamento_bloqueio ?? ''] || o.lancamento_bloqueio), 8)
  const noTeto = orPeriodo.filter(o => o.ajustado_pelo_teto)
  const valorNota = soma(orPeriodo, o => o.valor_nota)
  const valorCobrado = soma(orPeriodo, o => o.valor)
  const orcPorStatus = ordenado(contar(orPeriodo, o => NOME_STATUS_ORC[o.status] || o.status), 5)
  const orcPorMes: SerieGeradoLancado[] = meses.map(m => ({
    rotulo: mesCurto(m),
    gerados: orPeriodo.filter(o => mesDe(o.criado_em) === m).length,
    lancados: vivos.filter(o => o.status === 'lancado' && mesDe(o.lancado_em) === m).length,
  }))
  const teto = parametros.find(p => p.chave === 'teto_lancamento')?.valor

  const docs = documentos.filter(d => !d.oculto_em)
  const semTicket = docs.filter(d => d.tickets === 0 && d.status !== 'usado' && !d.duplicada_de)
  const ticketSolto = docs.filter(d => d.tickets_soltos > 0)
  const docsBloqueados = docs.filter(d => d.bloqueio_motivo)
  const docsFalhou = docs.filter(d => d.status === 'falhou')
  const docPorFila = ordenado(contar(docs, d => NOME_FILA[d.fila ?? ''] || d.fila), 3)

  // ---------- 4. FATURAMENTO ----------
  const fa = faturas.filter(x => daConta(f, x) && daLoja(f, x))
  const ciclosResumo: CicloResumo[] = ciclos.map(c => {
    const xs = fa.filter(x => x.ciclo_id === c.id)
    const faturado = soma(xs, x => x.valor_faturado)
    const recebido = soma(xs, x => x.valor_recebido)
    return {
      ...c, faturado, recebido, emAberto: faturado - recebido, notas: xs.length,
      comNF: xs.filter(x => x.nf_numero).length, comPCO: xs.filter(x => x.pco_numero).length,
    }
  }).sort((a, b) => a.competencia.localeCompare(b.competencia))
  const faturasLoja: FaturaDaLista[] = fa
    .map(x => ({ ...x, loja: nomeLoja(x.unidade), ciclo: ciclos.find(c => c.id === x.ciclo_id)?.competencia }))
    .sort((a, b) => (b.valor_faturado ?? 0) - (a.valor_faturado ?? 0))
  const totalFaturado = soma(ciclosResumo, c => c.faturado)
  const totalRecebido = soma(ciclosResumo, c => c.recebido)

  // O balanço olha o ORÇAMENTO: o que a nota custou (a pagar ao fornecedor) e o
  // que foi cobrado do cliente (a receber). A diferença é a margem bruta.
  const aPagarAberto = soma(vivos.filter(o => !o.pago), o => o.valor_nota)
  const pagoTotal = soma(vivos.filter(o => o.pago), o => o.valor_nota)
  const naoFaturado = vivos.filter(o => o.status === 'lancado' && !o.faturado)
  const balancoPorMes: SerieBalanco[] = meses.map(m => {
    const xs = orPeriodo.filter(o => mesDe(o.criado_em) === m)
    const pagar = soma(xs, o => o.valor_nota)
    const receber = soma(xs, o => o.valor)
    return { rotulo: mesCurto(m), pagar, receber, margem: receber - pagar }
  })
  const balancoLoja: BalancoDeLoja[] = [...somar(orPeriodo, o => nomeLoja(o.unidade), o => o.valor).entries()]
    .map(([loja, receber]) => {
      const daquela = orPeriodo.filter(o => nomeLoja(o.unidade) === loja)
      const pagar = soma(daquela, o => o.valor_nota)
      return { loja, pagar, receber, margem: receber - pagar, n: daquela.length }
    })
    .sort((a, b) => b.receber - a.receber)
    .slice(0, 12)

  return {
    nomeLoja,
    exp: lembrar(chamados, () => calcularExpectativa(base)),
    chamados: {
      total: ch.length, noPeriodo: chPeriodo.length, abertosAgora: abertosAgora.length,
      emExecucao: abertosAgora.filter(c => c.status === 'Em execução').length,
      executados: executadosNoPeriodo.length, vistoriados: vistoriadosNoPeriodo.length,
      pctPrazo: pct(noPrazo.length, comPrazo.length), comPrazo: comPrazo.length,
      porMes, porMesExec, porStatus, porPrioridade, porLoja, porLocal,
      lojasComChamado: new Set(chPeriodo.map(c => c.unidade)).size,
    },
    tempo: {
      n: durAteExec.length, pctAte7: pct(ate7, durAteExec.length),
      mediaExec: media(durAteExec), medianaExec: mediana(durAteExec),
      mediaVist: media(durAteVist), medianaVist: mediana(durAteVist),
      mediaExecucao: media(durExecucao), medianaExecucao: mediana(durExecucao),
      mediaExecAteVist: media(durExecAteVist), medianaExecAteVist: mediana(durExecAteVist),
      porConta: Object.entries(porContaTempo).map(([k, xs]) => ({
        rotulo: CONTA[k] || k, media: media(xs), mediana: mediana(xs), n: xs.length,
      })),
      histograma, backlog, lojasLentas,
      maisAntigos: maisAntigos.map(c => ({ ...c, loja: nomeLoja(c.unidade), idade: idade(c) })),
      atrasados: abertosAgora.filter(c => {
        const p = data(c.prazo)
        return !!p && p < agora
      }).length,
    },
    custos: {
      valorLancado, n: cuPeriodo.length, semData: cuPeriodo.filter(k => k.semData).length,
      ticketMedio: media(cuPeriodo.map(k => k.valor ?? 0)),
      custoPorMes, custoPorLoja, custoPorTipo, fechados, fechadosComCusto,
      pctComCusto: pct(fechadosComCusto, fechados),
      gerados: gerados.length, valorGerados: soma(gerados, o => o.valor), aguardando: aguardando.length,
      lancados: lancados.length, valorLancados: soma(lancados, o => o.valor),
      bloqueados: bloqueados.length, porBloqueio, noTeto: noTeto.length, teto,
      valorNota, valorCobrado,
      margem: valorNota ? Math.round(((valorCobrado - valorNota) / valorNota) * 100) : null,
      rateados: orPeriodo.filter(o => o.rateio).length,
      diretos: orPeriodo.filter(o => o.faturamento_direto).length,
      orcPorStatus, orcPorMes, semTicket: semTicket.length, ticketSolto: ticketSolto.length,
      docsBloqueados: docsBloqueados.length, docsFalhou: docsFalhou.length, docPorFila, docs: docs.length,
    },
    fat: {
      ciclosResumo, faturasLoja, totalFaturado, totalRecebido, emAberto: totalFaturado - totalRecebido,
      aPagarAberto, pagoTotal, naoFaturado: naoFaturado.length,
      valorNaoFaturado: soma(naoFaturado, o => o.valor),
      balancoPorMes, balancoLoja,
      margemPeriodo: valorCobrado - valorNota,
      receberPeriodo: valorCobrado, pagarPeriodo: valorNota,
    },
  }
}

// `calcular` não é um componente, então não pode usar `useMemo` — mas roda a
// cada mexida no filtro, e refazer a linha do tempo de 1.700 chamados a cada
// clique numa pílula de período trava a tela. O cache é preso ao ARRAY de
// origem: retrato novo, chave nova, conta refeita.
const guardado = new WeakMap<object, unknown>()
function lembrar<T>(chave: object, fazer: () => T): T {
  const achado = guardado.get(chave)
  if (achado !== undefined) return achado as T
  const novo = fazer()
  guardado.set(chave, novo)
  return novo
}
