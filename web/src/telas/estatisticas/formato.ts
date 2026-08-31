// rev 1 — como cada número aparece
//
// Separado do cálculo de propósito: aqui não se decide nada, só se escreve. Um
// `if` que muda o que o número SIGNIFICA está no lugar errado — a régua é de
// `calculo.ts`.
//
// SEM JARGÃO (regra do dono, 30/08/2026)
//	"mediana" vira "metade dos chamados"; "vistoriado" ganha "aprovado pelo
//	cliente" ao lado. Traço para o que não existe, nunca zero: zero é uma
//	medida, ausência não.

export const DIA = 86400000

const SO_DATA = /^\d{4}-\d{2}-\d{2}$/
const FUSO_CHEIO = /(?:Z|[+-]\d{2}:\d{2})$/
const FUSO_CURTO = /[+-]\d{2}$/

const valida = (d: Date): Date | null => (Number.isNaN(d.getTime()) ? null : d)

/**
 * Converte o que o banco manda em Date. Devolve `null` para o que não dá para ler.
 *
 * TRÊS FORMAS, E ELAS NÃO SÃO INTERCAMBIÁVEIS
 *
 *	`2026-09-02`                   data pura (uma coluna `date`)
 *	`2026-08-28T19:57:42+00:00`    instante COM fuso — é o que o PostgREST manda
 *	`2026-08-28T19:57:42`          instante SEM fuso — é o que o retrato tinha
 *
 * O QUE ISSO CUSTOU (31/08/2026)
 *
 *	A primeira versão colava `Z` em tudo que não tivesse 10 caracteres. Sobre a
 *	forma COM fuso isso produz `…+00:00Z` — e aí entra a pegadinha: com `T` no
 *	meio, o parser de datas do JavaScript é ESTRITO e recusa; com ESPAÇO no
 *	meio ele cai no parser legado, permissivo, e aceita.
 *
 *	O retrato offline e o console do banco escrevem com espaço. O PostgREST
 *	escreve com `T`. Então a conta passou em toda a conferência — números
 *	idênticos aos da versão offline — e derrubou a tela no primeiro minuto de
 *	produção: `toISOString()` sobre Date inválida LANÇA, e o React levou a
 *	árvore inteira junto. Tela branca, sem nem a barra lateral.
 *
 * POR QUE `null`, E NÃO UMA DATE INVÁLIDA
 *
 *	Date inválida é `truthy`. Ela atravessa a conta inteira comparando falso
 *	com tudo, não aparece em lugar nenhum, e explode longe de onde nasceu.
 *	`null` para na primeira porta: a linha fica de fora do período, e a tela
 *	continua de pé. Formato que eu não souber ler vira ausência, não estrago.
 */
export function data(s: string | null | undefined): Date | null {
  if (!s) return null
  const t = s.trim()
  if (!t) return null
  // Data pura ganha meio-dia, para o fuso não puxá-la para o dia anterior.
  if (SO_DATA.test(t)) return valida(new Date(t + 'T12:00:00'))
  const comT = t.includes('T') ? t : t.replace(' ', 'T')
  if (FUSO_CHEIO.test(comT)) return valida(new Date(comT))
  // `+00` abreviado não é ISO 8601, e o parser estrito recusa. Vira `+00:00`.
  if (FUSO_CURTO.test(comT)) return valida(new Date(comT + ':00'))
  // Sem fuso nenhum: é UTC, que é como o banco grava.
  return valida(new Date(comT + 'Z'))
}

export const mesDe = (s: string | null | undefined): string | null => (s ? s.slice(0, 7) : null)

export const hoje = (): Date => {
  const h = new Date()
  h.setHours(23, 59, 59, 999)
  return h
}

export const iso = (d: Date): string => d.toISOString().slice(0, 10)

const NOME_MES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez']
export const mesCurto = (m: string | null): string =>
  m ? NOME_MES[+m.slice(5, 7) - 1] + '/' + m.slice(2, 4) : ''

export const CONTA: Record<string, string> = { instalacoes: 'Instalações', civil: 'Civil' }

export const reais = (v: number | null | undefined): string =>
  v == null ? '—' : v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })

/** O mesmo valor, curto, para caber no eixo de um gráfico. */
export const reaisCurto = (v: number | null | undefined): string => {
  if (v == null) return '—'
  const a = Math.abs(v)
  if (a >= 1e6) return (v / 1e6).toLocaleString('pt-BR', { maximumFractionDigits: 1 }) + ' mi'
  if (a >= 1e3) return (v / 1e3).toLocaleString('pt-BR', { maximumFractionDigits: 1 }) + ' mil'
  return v.toLocaleString('pt-BR', { maximumFractionDigits: 0 })
}

export const inteiro = (v: number | null | undefined): string =>
  v == null ? '—' : v.toLocaleString('pt-BR')

export const diasFmt = (v: number | null | undefined): string =>
  v == null ? '—' : v.toLocaleString('pt-BR', { minimumFractionDigits: 1, maximumFractionDigits: 1 })

export const dtBR = (s: string | null | undefined): string =>
  s ? s.slice(0, 10).split('-').reverse().join('/') : '—'

export const classeStatus = (s: string | null | undefined): string =>
  ({
    Aberto: 'st-aberto',
    'Em execução': 'st-execucao',
    Executado: 'st-executado',
    Vistoriado: 'st-vistoriado',
    Arquivado: 'st-arquivado',
  }[s ?? ''] || 'st-outro')
