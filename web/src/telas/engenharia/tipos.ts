// rev 1 — o que a tela de Engenharia > Obras manipula
export interface Contratante {
  id: string
  nome: string
  cpf_cnpj?: string | null
}

export interface Obra {
  id: string
  codigo?: string | null
  nome: string
  status: string
  tipo_obra?: string | null
  data_inicio_prevista?: string | null
  data_fim_prevista?: string | null
  cliente_contratante?: Contratante | null
  unidades?: { id: string; nome: string; cidade?: string; uf?: string } | null
}

export interface Cronograma {
  id: string
  obra_id: string
  versao: number
  tipo: string
  ativo: boolean
  motivo_revisao?: string | null
}

export interface EAPNo {
  id: string
  pai_id?: string | null
  nivel: number
  codigo_eap: string
  ordem: number
  nome: string
  descricao?: string | null
  duracao_dias?: number | null
  data_inicio_prevista?: string | null
  data_fim_prevista?: string | null
  is_marco: boolean
  status: string
  filhos?: EAPNo[]
}

export interface Dependencia {
  id: string
  predecessor_id: string
  sucessor_id: string
  tipo: string
  lag_dias: number
}

export const STATUS_OBRA: Record<string, string> = {
  planejamento: 'Planejamento',
  em_execucao: 'Em execução',
  paralisada: 'Paralisada',
  concluida: 'Concluída',
  cancelada: 'Cancelada',
}

export const STATUS_EAP: Record<string, string> = {
  nao_iniciado: 'Não iniciado',
  em_andamento: 'Em andamento',
  concluido: 'Concluído',
  atrasado: 'Atrasado',
  paralisado: 'Paralisado',
}

const NIVEL_EAP: Record<number, string> = { 1: 'Grupo', 2: 'Fase', 3: 'Item', 4: 'Subitem' }

export function labelNivel(n: number): string {
  return NIVEL_EAP[n] ?? `Nível ${n}`
}
