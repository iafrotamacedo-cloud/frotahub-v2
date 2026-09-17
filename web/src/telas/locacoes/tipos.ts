// rev 1 — os tipos de Locações (Fase 1: só recebimento)
//
// Arquivo próprio, sem importar de `administrativo` — mesmo motivo de
// `administrativo/tipos.ts` (P-13): cada módulo cresce sem depender do
// vizinho.
export type Periodicidade = 'mensal' | 'quinzenal' | 'semanal'

/** O que `GET /locacoes/ordens/{id}/itens` devolve — pré-preenche a tela de
 *  recebimento com o que a leitura da OC já extraiu. */
export interface OrdemParaReceberLocacao {
  id: string
  numero: string | null
  obra_centro_custo: string | null
  fornecedor_nome: string | null
}

export interface ItemDaOrdemParaLocacao {
  id: string
  descricao: string
  unidade: string | null
  qtd: number
  valor_unit: number
}

/** O que `POST /locacoes/ordens/{id}/receber` devolve. */
export interface ResultadoDoRecebimento {
  id: string
  equipamentos: number
  vencimento: string
}

// ---------------------------------------------------------------------------
// Monitoramento (Fase 2)
// ---------------------------------------------------------------------------

export type Faixa = 'verde' | 'amarelo' | 'laranja' | 'vermelho'
export type EstadoEquipamento = 'ativo' | 'encerrado'

/** O que `GET /locacoes/painel` devolve — alimenta os cartões do hub. */
export interface PainelDeLocacoes {
  ativos: number
  descobertos: number
  vencendo: number
  encerrados: number
  renovacoes_pendentes: number
  total_mensal: number
  por_obra: { obra_centro_custo: string; total_mensal: number }[]
}

/** Uma linha de `GET /locacoes/equipamentos`. */
export interface EquipamentoLocado {
  id: string
  descricao: string
  unidade: string | null
  qtd_ativa: number
  valor_unit: number
  periodicidade: Periodicidade
  data_inicio: string
  vencimento_atual: string
  estado: EstadoEquipamento
  obra_centro_custo: string | null
  ordem_compra_id: string
  ordem_numero: string | null
  fornecedor_nome: string | null
  dias_para_vencer: number
  faixa: Faixa
  descoberto: boolean
  renovacao_pendente: boolean
}

export interface PeriodoDeLocacao {
  id: string
  numero: number
  tipo: 'original' | 'renovacao'
  inicio: string
  fim: string
  qtd: number
  valor_unit: number
  ordens_compra: { numero: string | null } | null
  valor_calculado: number
}

export interface FotoDeLocacao {
  id: string
  arquivo_sha256: string
}

export interface RecebimentoDeLocacao {
  data_recebimento: string
  romaneio_sha256: string
  nf_numero: string | null
  nf_sha256: string | null
}

export interface DevolucaoDeLocacao {
  id: string
  data_devolucao: string
  qtd: number
  romaneio_sha256: string
  frete_ordem_numero: string | null
}

export type RegraFaturamento = 'padrao' | 'sempre_proporcional' | 'sempre_mes_cheio'

export interface DetalheDoEquipamento {
  equipamento: EquipamentoLocado & { qtd_recebida: number; regra_faturamento: RegraFaturamento; total_calculado: number }
  periodos: PeriodoDeLocacao[]
  fotos: FotoDeLocacao[]
  recebimento: RecebimentoDeLocacao
  devolucoes: DevolucaoDeLocacao[]
}

/** O que `POST /locacoes/equipamentos/devolver` devolve. */
export interface ResultadoDaDevolucao {
  devolvidos: number
  encerrados: number
}

// ---------------------------------------------------------------------------
// Renovar, em duas etapas (Fase 4)
// ---------------------------------------------------------------------------

/** O que `POST /locacoes/equipamentos/renovar` devolve. */
export interface ResultadoDoPedidoDeRenovacao {
  pedidos: number
}

/** Uma linha de `GET /locacoes/renovacoes?estado=pendente` — a fila do RC. */
export interface PedidoDeRenovacao {
  id: string
  equipamento_id: string
  periodicidade: Periodicidade
  pedida_em: string
  descricao: string
  unidade: string | null
  qtd_ativa: number
  valor_unit: number
  vencimento_atual: string
  obra_centro_custo: string | null
  fornecedor_nome: string | null
}

/** O que `POST /administrativo/compras/ordens` devolve por arquivo enviado. */
export interface ResultadoDaInsercaoDeOC {
  nome: string
  id?: string
  erro?: string
  ja_existia?: boolean
  ja_existia_como?: string
}

/** O que `POST /administrativo/compras/ordens/{id}/ler` devolve. */
export interface ResultadoDaLeituraDeOC {
  status: 'lido' | 'falhou'
  motivo?: string
  numero?: string
  total?: number
  itens?: number
}

/** O que `POST /locacoes/renovacoes/concluir` devolve. */
export interface ResultadoDaConclusao {
  concluidas: number
}

// ---------------------------------------------------------------------------
// Faturamento (Fase 5) — só cálculo por período, sem ciclo/cliente ainda
// ---------------------------------------------------------------------------

/** O que `GET /locacoes/faturamento` devolve. */
export interface ResumoDeFaturamento {
  total: number
  por_obra: { obra_centro_custo: string; total_calculado: number }[]
}

/** O que `GET /locacoes/ordens/buscar?numero=X` devolve. */
export interface OrdemEncontrada {
  id: string
  numero: string | null
  total: number | null
  fornecedor_nome: string | null
}

export function emReais(v: number | null | undefined): string {
  if (v === null || v === undefined) return '–'
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

export function emData(s: string | null | undefined): string {
  if (!s) return '–'
  const [ano, mes, dia] = s.split('-')
  return ano && mes && dia ? `${dia}/${mes}/${ano}` : s
}

export function rotuloDaFaixa(dias: number, descoberto: boolean): string {
  if (descoberto) return `${Math.abs(dias)} dia${Math.abs(dias) === 1 ? '' : 's'} sem OC`
  if (dias === 0) return 'vence hoje'
  if (dias < 0) return `venceu há ${Math.abs(dias)} dia${Math.abs(dias) === 1 ? '' : 's'}`
  return `vence em ${dias} dia${dias === 1 ? '' : 's'}`
}

export function rotuloDaPeriodicidade(p: Periodicidade): string {
  return p === 'mensal' ? 'Mensal' : p === 'quinzenal' ? 'Quinzenal' : 'Semanal'
}
