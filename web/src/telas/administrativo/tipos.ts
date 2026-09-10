// rev 1 — os tipos e a formatação da fila de Ordens de Compra
//
// Arquivo próprio, e não importado de Orçamentos, pelo mesmo motivo de
// `funcionarios/tipos.ts`: cada módulo cresce sem depender do vizinho — dois
// módulos puxando um do outro é o primeiro passo para um dia não poder mexer
// em nenhum dos dois sem quebrar o outro (P-13/CORE-16).
export interface OrdemDeCompra {
  id: string
  nome_arquivo: string
  status: 'inserido' | 'lendo' | 'lido' | 'falhou'
  erro_leitura: string | null
  numero: string | null
  comprador_nome: string | null
  total: number | null
  criado_em: string
}

export interface ResultadoDaInsercao {
  nome: string
  id?: string
  erro?: string
  ja_existia?: boolean
  ja_existia_como?: string
}

export function emReais(v: number | null | undefined): string {
  if (v === null || v === undefined) return '–'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

export function emDataHora(s: string | null | undefined): string {
  if (!s) return '–'
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return d.toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
}
