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
  notas_travadas: number
  prontas_para_gerar: number

  // ---- a 015 ----
  // `a_lancar` conta TODO orçamento gerado — é o tamanho da fila.
  // `recusados` conta só os que já foram tentados e o Trílogo negou. A diferença
  // entre os dois é a diferença entre "trabalho a fazer" e "problema".
  recusados: number
  prontos_para_lancar: number
  esperando_cliente: number
  esperando_equipe: number

  // ---- a 017 ----
  // A fila do faturamento é uma coluna VAZIA (`fatura_id is null`), não um mês.
  // Em julho a planilha fechou no dia 29 e a leva do dia 31 rolou para agosto:
  // pelo mês, aqueles 39 orçamentos teriam sumido da cobrança para sempre.
  a_faturar: number
  valor_a_faturar: number
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
  /** Os tickets escritos que a nossa base não conhece. Enquanto houver um, a
   *  nota não gera — o valor de cada parte dependeria de quantos tickets são. */
  ticket_soltos: number[] | null
  pronto_para_gerar: boolean
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

  // ---- a 015 ----
  // Por que a ÚLTIMA tentativa de lançar foi recusada. Nulo = nunca tentado.
  // É esta coluna que separa "está na fila" de "deu problema": quem nunca foi
  // tentado não é uma correção, é trabalho a fazer.
  lancamento_bloqueio: Bloqueio | null
  // A frase do Trílogo, palavra por palavra. Nunca traduzir na tela: é ela que
  // explica o caso que a categoria não cobre.
  lancamento_bloqueio_detalhe: string | null
  lancamento_tentado_em: string | null
  lancamento_tentativas: number

  ticket_status: string | null
  ticket_status_codigo: number | null
  reaberto: boolean | null
  motivo_reabertura: string | null
  // Quem tem que agir. Calculado do status do ticket AGORA — muda sozinho
  // quando o chamado anda, sem ninguém regravar nada.
  destino: Destino | null
  avisado_em: string | null
}

export type Bloqueio =
  | 'ticket_status'
  | 'ticket_recusado'
  | 'teto'
  | 'sem_empresa'
  | 'trilogo_fora'
  | 'desconhecido'

export type Destino = 'pode_lancar' | 'cliente' | 'encarregados' | 'sem_chamado' | 'outro'

// ---------------------------------------------------------------------------
// faturamento ao cliente (017)
//
// NÃO CONFUNDIR COM `faturado`/`pago` DO ORÇAMENTO
//   Aquelas duas são do lado do FORNECEDOR — a nota que compramos, o dinheiro
//   que saiu. Isto aqui é o outro lado do balcão.
// ---------------------------------------------------------------------------

/** Um par loja×conta: exatamente uma nota do cliente. A palavra é dele — ele
 *  abre um PCO por conta/loja/período, e célula vazia não gera fatura. */
export interface Celula {
  unidade_id: string
  loja: string
  conta: string
  orcamentos: number
  valor: number
}

export interface Faturamento {
  competencia: string
  orcamentos: number
  valor: number
  celulas: Celula[]
  faturas: number
  /** Ainda não subiram para o Trílogo. Não impede faturar — em julho dois
   *  orçamentos sem custo lá foram cobrados e pagos — mas quem assina a
   *  planilha merece saber. */
  nao_lancados: number
  /** Sem loja ou sem conta não existe PCO possível. Estes não entram em célula
   *  nenhuma, e por isso aparecem sozinhos. */
  sem_destino: number
  desde: string
  ate: string
}

export interface Fatura {
  id: string
  competencia: string
  loja: string
  conta: string
  orcamentos: number
  valor: number
  pco_numero: string | null
  nf_numero: string | null
  nf_em: string | null
  recebido_em: string | null
  valor_recebido: number | null
  observacao: string | null
  recebida: boolean
  /** Faturado menos recebido. Diferente de zero é o que o controle existe para
   *  mostrar: pagamento parcial, glosa, desconto. */
  diferenca: number
}

export interface Faturas {
  faturas: Fatura[]
  faturado: number
  recebido: number
  a_receber: number
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
  recusados: Orcamento[]
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
