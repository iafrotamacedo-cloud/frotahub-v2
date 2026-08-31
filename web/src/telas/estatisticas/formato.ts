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

/** Converte o que o banco manda em Date. Data pura ganha meio-dia, para o fuso não puxá-la para o dia anterior. */
export const data = (s: string | null | undefined): Date | null =>
  s ? new Date(s.length === 10 ? s + 'T12:00:00' : s + 'Z') : null

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
