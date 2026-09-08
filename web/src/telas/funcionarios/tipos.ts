// rev 1 — o que a tela de Funcionários manipula
export interface Funcionario {
  id: string
  nome_completo: string
  cpf: string
  rg: string | null
  status: 'ativo' | 'inativo' | 'desligado'
  funcao_id: string | null
  unidade_id: string | null
  funcoes: { nome: string } | null
  unidades: { nome: string } | null
}

export interface Funcao {
  id: string
  nome: string
  criado_em: string
  funcao_documento_requisitos?: {
    tipo_documento_id: string
    obrigatorio: boolean
    tipos_documento: { nome: string } | null
  }[]
}

export interface TipoDocumento {
  id: string
  nome: string
  categoria: 'pessoal' | 'admissional' | 'saude' | 'nr' | 'contratual'
  tem_validade: boolean
  validade_dias_padrao: number | null
  dias_alerta: number
}

export type StatusDocumento = 'pendente' | 'enviado' | 'aprovado' | 'reprovado' | 'vencido'

export interface FuncionarioDocumento {
  id: string
  tipo_documento_id: string
  status: StatusDocumento
  data_emissao: string | null
  data_validade: string | null
  motivo_reprovacao: string | null
  aprovado_em: string | null
  tipos_documento: { nome: string; categoria: string; tem_validade: boolean; dias_alerta: number } | null
}

export interface Pendencia {
  funcionario_id: string
  nome_completo: string
  faltando: string[] | null
  vencidos: string[] | null
}

export interface Conformidade {
  total_funcionarios: number
  conformes: number
  com_pendencia: number
  funcionarios_com_pendencia: Pendencia[] | null
}
