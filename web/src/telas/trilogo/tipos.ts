// rev 1 — o que a tela de Dados do Trílogo recebe do motor
//
// Os nomes são os do banco, sem tradução no meio do caminho: o que a consulta
// devolve é o que a tela lê. Uma camada de renomeação aqui só criaria dois
// vocabulários para a mesma coisa.

export interface LinhaChamado {
  id: string
  numero: number
  loja: string
  unidade_id: string
  conta: string
  status: string
  prioridade: string
  descricao: string | null
  ambiente: string | null
  responsavel: string | null
  criado_em: string
  prazo: string | null
  custo_total: string | number
  anexos: number
}

export interface Pagina {
  linhas: LinhaChamado[]
  total: number
  pagina: number
  paginas: number
  por_pagina: number
}

export interface Loja {
  id: string
  nome: string
  id_trilogo: number
}

export interface Filtros {
  lojas: Loja[]
  status: string[]
  prioridades: string[]
  contas: string[]
  por_pagina: number[]
}

/** O que uma rodada do robô devolve. Só o que a tela usa. */
export interface Rodada {
  modo: string
  situacao: string
  chamados_lidos: number
  chamados_gravados: number
  completo: boolean
  comecou_em?: string
  terminou_em?: string
}

/** Os filtros como a tela os guarda. Tudo texto: é o que sai de um formulário. */
export interface Escolhas {
  ticket: string
  loja: string
  status: string
  conta: string
  prioridade: string
  de: string
  ate: string
}

export const SEM_FILTRO: Escolhas = {
  ticket: '', loja: '', status: '', conta: '', prioridade: '', de: '', ate: '',
}

export function contaPorExtenso(c: string): string {
  if (c === 'instalacoes') return 'Instalações'
  if (c === 'civil') return 'Civil'
  return c || '—'
}

/** A classe do pino de status. Um status novo cai no cinza, e não quebra a tela. */
export function classeDoStatus(s: string): string {
  switch (s) {
    case 'Aberto': return 'st-aberto'
    case 'Em execução': return 'st-execucao'
    case 'Executado': return 'st-executado'
    case 'Vistoriado': return 'st-vistoriado'
    case 'Arquivado': return 'st-arquivado'
    default: return 'st-outro'
  }
}

export function classeDaPrioridade(p: string): string {
  switch (p) {
    case 'Alta': return 'pr-alta'
    case 'Média': return 'pr-media'
    case 'Baixa': return 'pr-baixa'
    default: return 'pr-outra'
  }
}

/** Dinheiro em português, sem o "R$" (que fica no cabeçalho da coluna). */
export function emReais(v: string | number | null): string {
  const n = typeof v === 'string' ? Number(v) : v
  if (!n) return '—'
  return n.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

/**
 * Data e hora no fuso de quem está olhando.
 *
 * O banco guarda em UTC; o navegador da Frota Macedo está em Fortaleza. Deixar o
 * próprio navegador converter é o certo — e é o que faz o horário bater com o que
 * a pessoa vê no Trílogo.
 */
export function quando(iso: string | null, comHora = true): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const data = d.toLocaleDateString('pt-BR')
  if (!comHora) return data
  return data + ' ' + d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
}
