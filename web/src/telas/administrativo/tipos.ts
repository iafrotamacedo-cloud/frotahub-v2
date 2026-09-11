// rev 2 — os tipos e a formatação da fila de Ordens de Compra
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
  /** Presente quando a leitura já achou um fornecedor com nome e CNPJ. */
  fornecedor_id: string | null
  total: number | null
  criado_em: string
}

/** A vista da fila — mesmo desenho de três vistas de Orçamentos. */
export type VistaDasOrdens = 'fila' | 'processadas' | 'rejeitadas'

/** O que `GET /administrativo/compras/ordens/porler` devolve. */
export interface OrdensPorLer {
  ordens: { id: string; nome_arquivo: string; status: string }[]
  total: number
  teto: number
}

/** O andamento do botão "Ler ordens (N)" — mesmo desenho de `LoteDeLeitura`
 *  em Orçamentos. */
export interface LoteDeLeitura {
  total: number
  feito: number
  lidas: number
  falhas: number
  rodando: boolean
  agora?: string
  /** Os motivos, sem repetir — três OCs com a mesma queixa são uma linha. */
  motivos: string[]
}

/** O que `GET /administrativo/compras/ordens/painel` devolve — alimenta os
 *  cartões do hub de Compras: "Inserir OC" (fila) e "OCs Inseridas", que se
 *  abre em Processadas/Rejeitadas. */
export interface PainelDeOrdens {
  fila: number
  processadas: number
  rejeitadas: number
  previa?: {
    processadas?: LinhaDaPreviaDeOrdem[]
    rejeitadas?: LinhaDaPreviaDeOrdem[]
  }
}

export interface LinhaDaPreviaDeOrdem {
  nome_arquivo: string
  numero: string | null
  erro_leitura: string | null
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
