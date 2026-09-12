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
  comprador_cnpj: string | null
  /** Presente quando a leitura já achou um fornecedor com nome e CNPJ. */
  fornecedor_id: string | null
  total: number | null
  criado_em: string
}

/** O erro de faturamento não se corrige editando — a OC precisa ser
 *  substituída por um PDF novo, corrigido no Obra Prima (ver o cabeçalho de
 *  `TabelaDeOrdens.tsx`). */
export function precisaSubstituir(erroLeitura: string | null): boolean {
  const m = (erroLeitura ?? '').toLowerCase()
  return m.includes('faturamento') || m.includes('03720882')
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

/** O que `GET /administrativo/compras/pco/painel` devolve — alimenta os três
 *  cartões do hub de PCO: Pendentes de envio, Enviados e
 *  Excluídas/Substituídas. */
export interface PainelDoPCO {
  pendentes: number
  enviados: number
  canceladas: number
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

/** Uma linha de `GET /administrativo/compras/pco/canceladas` — o retrato de
 *  uma OC excluída ou substituída depois de já ter sido enviada (migração
 *  063). Só leitura: não tem `status` nem campos de ação, porque não é fila
 *  de trabalho. */
export interface OrdemCancelada {
  id: string
  tipo: 'excluida' | 'substituida'
  numero: string | null
  obra_centro_custo: string | null
  comprador_nome: string | null
  comprador_cnpj: string | null
  fornecedor_nome: string | null
  fornecedor_cnpj: string | null
  valor: number | null
  nome_arquivo: string | null
  enviado_em: string | null
  removida_em: string
  substituta_numero: string | null
  substituta_id: string | null
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

/** XX.XXX.XXX/YYYY-ZZ — mesma máscara que o motor usa no e-mail do PCO. */
export function formatarCNPJ(digitos: string | null | undefined): string {
  const d = (digitos ?? '').replace(/\D/g, '')
  if (d.length !== 14) return digitos ?? ''
  return `${d.slice(0, 2)}.${d.slice(2, 5)}.${d.slice(5, 8)}/${d.slice(8, 12)}-${d.slice(12)}`
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
  if (lower.includes('endereço de cobrança')) {
    return 'Endereço de cobrança errado'
  }
  return s.replace(/\bfalhou\b/gi, '').replace(/\s+/g, ' ').trim()
}

export interface ItemDocumentoOC {
  descricao: string
  qtd: number
  unidade: string
  valor_unit: number
  desconto: number
  total: number
}

export interface DocumentoOC {
  numero: string
  data: string
  previsao_entrega: string
  cond_pgto: string
  forma_pgto: string
  observacao: string
  titulo: string
  data_impressao: string
  emitente_razao: string
  emitente_endereco: string
  emitente_contato: string
  emitente_cnpj: string
  responsavel_nome: string
  responsavel_email: string
  comprador_interno: string
  comprador_nome: string
  comprador_cnpj: string
  faturamento_ie: string
  faturamento_endereco: string
  fornecedor_nome: string
  fornecedor_cnpj: string
  fornecedor_telefone: string
  fornecedor_vendedor: string
  fornecedor_email: string
  fornecedor_endereco: string
  obra_centro_custo: string
  cno: string
  endereco_entrega: string
  recebedor: string
  endereco_cobranca: string
  subtotal: number
  desconto: number
  frete: number
  total: number
  itens: ItemDocumentoOC[]
}

export interface EstadoDocumentoOC {
  documento: DocumentoOC
  status: string
  pco_enviado: boolean
  motivos: string[]
  precisa_fornecedor: boolean
  /** Faturamento não se edita mais aqui — só sinaliza pra tela nem abrir o
   *  reparo híbrido (a lista já desvia para "Ver + Substituir"). */
  precisa_faturamento: boolean
  precisa_endereco: boolean
  motivo?: string
  /** Só na prévia (`POST .../documento?antever=1`). */
  pdf_base64?: string
}
