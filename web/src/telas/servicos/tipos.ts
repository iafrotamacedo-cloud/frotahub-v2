// rev 2 — o que as telas de Serviço manipulam
//
// Espelha exatamente o que o motor devolve — ver baleryan/interno/modulos/
// servicos (painel.go, kanban.go, candidatos.go) e trilogo/servico.go para os
// dois tipos que vêm direto do Trílogo (Cotacao/Orcamento), que por isso NÃO
// seguem o snake_case do resto: são o JSON de terceiro, sem tradução.
export interface Candidato {
  id: string
  chamado_id: string
  ticket: number
  motivo: string
  status: string
  criado_em: string
}

export const STATUS_ORDEM = [
  'aguardando_orcamento', 'orcamento_feito', 'orcamento_lancado',
  'orcamento_aprovado', 'orcamento_rejeitado',
  'aprovado_execucao', 'em_execucao', 'finalizado',
  'aguardando_faturamento', 'faturado',
] as const

export type Status = typeof STATUS_ORDEM[number]

export const STATUS_ROTULO: Record<Status, string> = {
  aguardando_orcamento: 'Aguardando orçamento',
  orcamento_feito: 'Orçamento feito',
  orcamento_lancado: 'Orçamento lançado',
  orcamento_aprovado: 'Orçamento aprovado',
  orcamento_rejeitado: 'Orçamento rejeitado',
  aprovado_execucao: 'Aprovado p/ execução',
  em_execucao: 'Em execução',
  finalizado: 'Finalizado',
  aguardando_faturamento: 'Aguardando faturamento',
  faturado: 'Faturado',
}

// O MESMO MAPA DE baleryan/interno/modulos/servicos/kanban.go (proximosStatus)
//
//	Só usado hoje pela fila "Orçamentos lançados", pra aprovar/rejeitar
//	manualmente (a detecção automática de aprovação depende de um teste ao
//	vivo no Trílogo ainda não feito — ver o plano). O resto do funil avança
//	sozinho e não chama isto.
export const PROXIMOS_STATUS: Record<Status, Status[]> = {
  aguardando_orcamento: ['orcamento_feito'],
  orcamento_feito: ['orcamento_lancado'],
  orcamento_lancado: ['orcamento_aprovado', 'orcamento_rejeitado'],
  orcamento_aprovado: ['aprovado_execucao'],
  orcamento_rejeitado: ['orcamento_feito'],
  aprovado_execucao: ['em_execucao'],
  em_execucao: ['finalizado'],
  finalizado: ['aguardando_faturamento'],
  aguardando_faturamento: ['faturado'],
  faturado: [],
}

export const CONTA_ROTULO: Record<string, string> = {
  instalacoes: 'Serviço Instalações',
  civil: 'Serviço Civil',
}

// ---------------------------------------------------------------------------
// O hub — GET /servicos/painel
// ---------------------------------------------------------------------------

export interface Painel {
  candidatos_pendentes: number
  orcamentos_pendentes: number
  orcamento_feito: number
  orcamento_lancado: number
  execucao_aberto: number
  execucao_em_curso: number
  execucao_executado: number
  faturamento_aguardando_pco: number
  faturamento_a_faturar: number
  faturamento_faturado: number
  total_ativos: number
}

// ---------------------------------------------------------------------------
// As listas — GET /servicos/lista
// ---------------------------------------------------------------------------

export interface ItemLista {
  id: string
  chamado_id: string
  ticket: number
  conta: 'instalacoes' | 'civil'
  status: Status

  chamado_descricao: string | null
  chamado_status_codigo: number | null
  chamado_status: string | null
  executado_em: string | null
  vistoriado_em: string | null
  loja: string | null

  cotacao_trilogo_id: number | null
  orcamento_trilogo_id: number | null
  orcamento_valor: number | null
  orcamento_arquivo_sha256: string | null
  orcamento_arquivo_nome: string | null
  orcamento_arquivo_em: string | null
  orcamento_aprovado_em: string | null
  orcamento_rejeitado_em: string | null

  pco_numero: string | null
  pco_preenchido_em: string | null
  nf_numero: string | null
  nf_arquivo_sha256: string | null
  nf_arquivo_nome: string | null
  nf_arquivo_em: string | null

  origem: 'gatilho' | 'candidato' | 'manual'
  entrou_em: string
  atualizado_em: string
  removido_em: string | null
  removido_motivo: string | null

  com_orcamento: boolean
  com_pco: boolean
  com_nf: boolean
  esta_aprovado: boolean
  esta_rejeitado: boolean
}

// ---------------------------------------------------------------------------
// Cotação e Orçamento — o JSON do Trílogo, sem tradução (ver trilogo/servico.go)
// ---------------------------------------------------------------------------

export interface Orcamento {
  id: number
  creationDate: string
  deadlineDate: string | null
  totalValue: number
  approvedValue: number | null
  isWinning: boolean
  supplier: { id: number; name: string }
  attachments: { fileName: string; permalink: string }[]
}

export interface Cotacao {
  id: number
  description: string
  creationDate: string
  status: number
  winningBudgetId: number | null
  hasBudget: boolean
  approvedValue: number | null
  budgets: Orcamento[]
}

export interface ItemDeOrcamento {
  descricao: string
  valor: number
  qtd: number
}
