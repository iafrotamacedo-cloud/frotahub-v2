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
  obra_centro_custo: string | null
  comprador_nome: string | null
  /** Presente quando a leitura já achou um fornecedor com nome e CNPJ. */
  fornecedor_id: string | null
  total: number | null
  criado_em: string
}

/** A vista da fila — mesmo desenho de três vistas de Orçamentos. As duas de
 *  PCO não são uma fila nova de OCs: são a mesma "processadas", filtrada por
 *  `pco_enviado_em` (ver o cabeçalho de `filtroDasOrdens` no motor). */
export type VistaDasOrdens = 'fila' | 'processadas' | 'rejeitadas' | 'pco-pendentes' | 'pco-enviados'

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

/** O que `GET /administrativo/compras/pco/painel` devolve — alimenta os dois
 *  cartões do hub de PCO: Pendentes de envio e Enviados. */
export interface PainelDoPCO {
  pendentes: number
  enviados: number
  previa?: {
    pendentes?: LinhaDaPreviaDeOrdem[]
    enviados?: LinhaDaPreviaDeOrdem[]
  }
}

export interface LinhaDaPreviaDeOrdem {
  nome_arquivo: string
  numero: string | null
  erro_leitura: string | null
  criado_em: string
}

/** Um destinatário do e-mail de PCO (migração 061). */
export interface Destinatario {
  id: string
  email: string
  ativo: boolean
  criado_em: string
}

/** O que `POST /administrativo/compras/pco/enviar` (geral ou uma OC)
 *  devolve. "nunca enviar vazio" é `enviado: false` sem `erro` nenhum — não
 *  é falha, é "não tinha nada para mandar". */
export interface ResultadoDoEnvio {
  enviado: boolean
  motivo?: string
  quantidade?: number
  valor_total?: number
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

export function motivoRejeicaoSimplificado(motivo: string | null): string {
  if (!motivo?.trim()) return '—'
  return motivo
    .split(';')
    .map(part => simplificarMotivoRejeicao(part.trim()))
    .filter(Boolean)
    .join(' · ')
}

function simplificarMotivoRejeicao(s: string): string {
  const lower = s.toLowerCase()
  if (lower.includes('fornecedor') && (lower.includes('cnpj') || lower.includes('nome'))) {
    return 'Fornecedor sem CNPJ'
  }
  if (lower.includes('faturamento') && lower.includes('não achei')) {
    return 'Faturamento sem CNPJ'
  }
  if (lower.includes('faturamento') && (lower.includes('não começa') || lower.includes('frota macedo'))) {
    return 'CNPJ de faturamento errado'
  }
  return s.replace(/\bfalhou\b/gi, '').replace(/\s+/g, ' ').trim()
}

export interface ObraCentroSugerida {
  obra_centro_custo: string
  comprador_cnpj?: string | null
  comprador_nome?: string | null
}

export interface EstadoReparoOC {
  precisa_fornecedor: boolean
  precisa_faturamento: boolean
  motivos: string[]
  fornecedor_cnpj?: string | null
  obra_centro_custo?: string | null
  comprador_cnpj?: string | null
  comprador_nome?: string | null
}

export interface ResultadoReparoOC {
  status: 'lido' | 'falhou'
  motivo?: string
  motivos: string[]
  precisa_fornecedor: boolean
  precisa_faturamento: boolean
}
