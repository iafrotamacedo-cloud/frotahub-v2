// rev 1 — o que o motor devolve para as telas de Orçamentos
//
// Os nomes são os do banco, sem tradução. Renomear no caminho cria um segundo
// vocabulário que só existe no front — e é assim que, meses depois, ninguém sabe
// mais se `valor` é o da nota ou o cobrado.

export interface Painel {
  notas_arquivos: number
  rateio_sem_ticket: number
  a_lancar: number
  sem_ticket: number
  sem_associacao: number
  aguardando_aprovacao: number
  apagados: number
  no_total: number
  valor_total: number
  ultima_insercao?: string | null
  /** As últimas linhas de cada etapa — é o que faz a barra virar painel. */
  previa?: {
    notas?: Array<{ nome_arquivo: string; inserido_em: string; valor_total: number | null }>
    rateio?: Array<{ nome_arquivo: string; inserido_em: string; valor_total: number | null }>
    lancar?: Array<{ ticket: number; loja: string | null; valor: number | null }>
  }
}

export interface Pagina<T> {
  linhas: T[]
  total: number
  pagina: number
  por_pagina: number
  paginas: number
}

export interface Documento {
  id: string
  fila: 'orcamento' | 'rateio'
  tipo: string | null
  numero: string | null
  dav_numero: string | null
  chave_acesso: string | null
  emitente_nome: string | null
  emissao: string | null
  valor_total: number | null
  status: 'inserido' | 'lendo' | 'lido' | 'falhou' | 'usado'
  leitura_camada: string | null
  leitura_confianca: number | null
  nome_arquivo: string
  inserido_em: string
  oculto_em: string | null
  tickets: number
  ticket_numeros: number[] | null
  itens: number
}

export interface Orcamento {
  id: string
  ticket: number
  parte: number
  conta: string | null
  status: 'gerado' | 'aguardando_aprovacao' | 'lancado' | 'removido'
  valor: number | null
  valor_nota: number | null
  reduzido_pelo_teto: boolean
  valor_antes_do_teto: number | null
  rateio: boolean
  criado_em: string
  lancado_em: string | null
  faturado: boolean
  pago: boolean
  trilogo_custo_id: number | null
  loja: string | null
  chamado_descricao: string | null
  notas: string | null
  davs: string | null
}

export interface TicketDoDocumento {
  id: string
  ticket: number
  chamado_id: string | null
}

export interface Correcoes {
  sem_ticket: Documento[]
  sem_associacao: Array<{
    id: string
    ticket: number
    documento_id: string
    documentos: { id: string; nome_arquivo: string; numero: string | null; valor_total: number | null }
  }>
  nao_lancados: Orcamento[]
  apagados: Orcamento[]
  aguardando: Orcamento[]
}

export interface Candidato {
  numero: number
  loja: string
  descricao: string
  criado_em: string
  conta: string
  porque: string
}

export interface ResultadoDaInsercao {
  nome: string
  id?: string
  erro?: string
  ja_existia?: boolean
}

export interface ResultadoDaGeracao {
  documento: string
  nome: string
  ticket?: number
  orcamento?: string
  aviso?: string
  erro?: string
}

// ---------------------------------------------------------------------------
// formatação — escrita uma vez, usada por todas as telas do módulo
// ---------------------------------------------------------------------------

export function emReais(v: number | null | undefined): string {
  if (v === null || v === undefined) return '–'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

export function emData(s: string | null | undefined): string {
  if (!s) return '–'
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return d.toLocaleDateString('pt-BR')
}

export function emDataHora(s: string | null | undefined): string {
  if (!s) return '–'
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return d.toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
}

/**
 * O quanto se pode confiar no que foi lido, dito em palavras.
 *
 * POR QUE NÃO MOSTRAR SÓ A PORCENTAGEM
 *   "62%" não diz a ninguém o que fazer. "Confira antes de gerar" diz. A
 *   diferença entre um valor lido por IA e um valor digitado por gente só é
 *   útil se virar instrução.
 */
export function confiancaEmPalavras(camada: string | null, c: number | null): { texto: string; classe: string } {
  if (camada === 'xml') return { texto: 'XML da nota', classe: 'ok' }
  if (camada === 'manual') return { texto: 'digitado', classe: 'ok' }
  const v = c ?? 0
  if (v >= 0.85) return { texto: 'leitura boa', classe: 'ok' }
  if (v >= 0.6) return { texto: 'confira', classe: 'aviso' }
  return { texto: 'confira com atenção', classe: 'ruim' }
}

export function contaPorExtenso(c: string | null): string {
  if (c === 'instalacoes') return 'Instalações'
  if (c === 'civil') return 'Civil'
  return c ?? '–'
}
