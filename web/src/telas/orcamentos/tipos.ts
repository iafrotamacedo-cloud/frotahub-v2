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
  /** Conta só o que o botão de lote CONSEGUE subir. */
  prontos_para_lancar: number
  esperando_cliente: number
  esperando_equipe: number
  /** A terceira lista de orçamentos parados: teto e duplicidade. */
  esperando_decisao?: number
  /** Notas que não cabem no teto do ticket. A frente existia desde a 020 e nunca
   *  teve número — entrou no painel com a migração 036. */
  extrapoladas?: number

  // ---- a 017 ----
  // A fila do faturamento é uma coluna VAZIA (`fatura_id is null`), não um mês.
  // Em julho a planilha fechou no dia 29 e a leva do dia 31 rolou para agosto:
  // pelo mês, aqueles 39 orçamentos teriam sumido da cobrança para sempre.
  a_faturar: number
  valor_a_faturar: number
  /** ---- a 023 ---- a fila do faturamento direto. */
  notas_direto?: number
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

  // ---- a 029 ----
  /** Onde esta nota mora: `fila`, `sem-ticket`, `sem-associacao` ou `usada`.
   *  A regra vive na visão, e não em condições repetidas por tela. */
  onde?: 'fila' | 'sem-ticket' | 'sem-associacao' | 'usada'
  /** Por que ela precisa de gente. Nulo quando não precisa. */
  motivo_conferencia?: string | null

  // ---- a 031 ----
  /** Quanto os itens lidos somam. É o número que se compara com o total. */
  soma_dos_itens?: number
  /** Itens sem descrição, sem quantidade ou sem preço. */
  itens_incompletos?: number
  /** A soma dos itens fecha com o total da nota, com 1% de tolerância. */
  conta_fecha?: boolean
  /** Quando alguém olhou o papel e respondeu a pergunta do valor. */
  valor_conferido_em?: string | null

  // ---- a 019 ----
  /** Quando preenchida, esta nota é a MESMA que a apontada — mesma chave de
   *  acesso, ou mesmo número e valor. Ela não gera orçamento. O nome da outra
   *  vem junto porque "repetida" sem dizer de quem obriga o usuário a caçar. */
  duplicada_de: string | null
  duplicada_de_nome: string | null
  duplicada_de_em: string | null

  // ---- a 020 ----
  /** A nota não pode virar orçamento, e a frase diz por quê. Some sozinha
   *  quando a condição muda e a nota passa numa tentativa seguinte. */
  bloqueio_motivo: string | null

  // ---- a 022 ----
  /** Desconto autorizado, em pontos-base do orçamento. 385 = 3,85%. Zero é o
   *  normal, e uma nota nunca ganha desconto duas vezes. */
  desconto_bp: number
  desconto_em: string | null
  /** A nota foi mandada para aprovação: o orçamento nasce com o valor CHEIO e
   *  parado, porque é ELE que vai ao cliente — não a nota. */
  aprovacao_pedida: boolean
  aprovacao_pedida_em: string | null
}

/** O que o motor responde sobre dar desconto numa nota que passa do teto.
 *
 *  O NÚMERO VEM DAQUI, NUNCA DO NAVEGADOR
 *    A tela mostra e confirma; quem calcula é o motor, e ele recalcula na hora
 *    de gravar. Aceitar um valor da tela seria aceitar 40% de quem soubesse
 *    escrever a chamada à mão. */
export interface Desconto {
  pode: boolean
  bp: number
  porcentagem: string
  orcamento_original: number
  orcamento_final: number
  motivo?: string
  ja_tem_desconto: boolean
}

export interface Orcamento {
  id: string
  ticket: number
  parte: number
  conta: string | null
  status: 'gerado' | 'aguardando_aprovacao' | 'lancado' | 'removido'
  valor: number | null
  /** O que se PAGOU pela nota. */
  valor_nota: number | null
  reduzido_pelo_teto: boolean
  valor_antes_do_teto: number | null

  // ---- a 020 ----
  /** A soma dos itens como estão na nota, ANTES do desconto do fornecedor.
   *  Igual a `valor_nota` quando não houve desconto. */
  valor_nota_cheio: number | null
  /** O bruto passava do maior custo que cabe no teto, e o orçamento foi fechado
   *  no teto exato.
   *
   *  CARIMBO INTERNO
   *    Ele existe para NÓS entendermos um corte que não tem outra explicação —
   *    seis meses depois, alguém abre um orçamento de R$ 600 para uma nota de
   *    R$ 700 e precisa saber se foi regra, erro de digitação ou coisa pior.
   *    Não entra no PDF que vai para o cliente, e nunca deve entrar. */
  ajustado_pelo_teto: boolean
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
  /** Travado por algo que só uma pessoa desfaz — teto ou duplicidade. A regra
   *  de QUAIS bloqueios são estes mora na view (migração 035), e não aqui: a
   *  mesma pergunta é feita pelo motor, por esta tela e pelo balanço. */
  precisa_decisao?: boolean
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
  // O TIPO É QUEM COBRA A TELA POR UM BLOQUEIO NOVO
  //   `SAIDAS` em Correções é um `Record<Bloqueio, …>`: acrescentar um nome aqui
  //   faz o TypeScript exigir a entrada lá. Foi o que faltou quando a trava de
  //   duplicidade entrou — o bloqueio existia no motor, não existia neste tipo,
  //   e a tela o desenhava como um "Recusado" genérico, sem cor e sem saídas.
  | 'possivel_duplicata'
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
  /** As que a geração recusou por limite de teto. */
  extrapoladas?: Documento[]
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

/**
 * O QUE está errado na conta desta nota — em uma frase que se resolve olhando o
 * papel.
 *
 * NÃO FALA DE CHAVE DE ACESSO, E ISSO É DE PROPÓSITO
 *   A chave tem 44 dígitos, ninguém a digita, não entra no orçamento e não
 *   impede nada. Ela derrubava o placar de confiança de DANFEs perfeitas e
 *   virava "confira" eterno. Aviso que nunca muda é aviso que ninguém lê.
 */
export function oQueConferir(d: Documento): string {
  if (!d.itens) return 'nenhum item foi lido nesta nota'
  if (d.itens_incompletos) {
    return d.itens_incompletos === 1
      ? 'um item ficou sem quantidade ou sem preço'
      : `${d.itens_incompletos} itens ficaram sem quantidade ou sem preço`
  }
  if (!d.valor_total) return 'o valor total não foi lido'
  return 'os itens não somam o total da nota'
}

export function contaPorExtenso(c: string | null): string {
  if (c === 'instalacoes') return 'Instalações'
  if (c === 'civil') return 'Civil'
  return c ?? '–'
}

// ---------------------------------------------------------------------------
// tratamento das filas (26/08/2026)
//
// A CONFERÊNCIA DA TELA É A MESMA DA GERAÇÃO
//   No sistema antigo eram duas, e discordavam: a tela dizia "pronta" e a
//   geração recusava, sem ninguém entender por quê. Estes campos vêm de
//   `avaliarPartes`, a mesma função que decide se o orçamento nasce.
// ---------------------------------------------------------------------------

export interface ParteConferida {
  ticket: number
  na_base: boolean
  loja?: string
  /** Quanto desta nota cabe a este ticket, já com o desconto do fornecedor. */
  custo: number
  /** Quanto o ticket já tem de custo lançado, de qualquer tipo. */
  ja_no_ticket: number
  /** O orçamento que sairia daqui. Zero quando este pedaço está travado. */
  orcamento: number
  decisao: 'livre' | 'reduzido' | 'aprovacao' | ''
  /** Aparada pela regra do desconto — o carimbo interno. */
  ajustada: boolean
  motivo?: string
}

export interface Conferencia {
  documento: string
  nome: string
  fila: 'orcamento' | 'rateio' | ''
  valor_total: number
  /** Verde ou vermelho. É esta a palavra que o botão reprocessar obedece. */
  pronta: boolean
  motivos?: string[]
  partes: ParteConferida[]
}


// ---------------------------------------------------------------------------
// As pendências — orçamentos parados esperando o chamado ANDAR
//
// A linha é o TICKET, com os orçamentos dele somados dentro. O motor agrupa; a
// tela só desenha. Os campos vêm em branco (nunca nulos) porque o motor já
// resolveu isso na borda — assim a tela não tem que decidir o que é "vazio".
// ---------------------------------------------------------------------------

export interface Pendencia {
  ticket: number
  /** Um orçamento daquele ticket — serve para pedir a reconferência do chamado.
   *  Qualquer um serve: o status é do chamado, não do orçamento. */
  orcamento: string
  loja: string
  conta: string
  ticket_status: string
  reaberto: boolean
  /** O que o CLIENTE escreveu ao reabrir. Vazio quando não houve reabertura. */
  motivo: string
  /** O que o chamado pediu — a coluna larga da lista dos encarregados. */
  descricao: string
  orcamentos: number
  partes: string[]
  valor: number
  /** O mais antigo dos orçamentos daquele ticket: há quanto tempo trava. */
  desde_em: string
  /** Quando esta lista foi cobrada pela última vez. Vazio = nunca. */
  avisado_em: string
  /** A última recusa deste ticket, quando alguém já tentou lançar. Vazio = nunca
   *  foi tentado — que não é problema, é trabalho a fazer. */
  recusa: string
  recusa_em: string
  tentativas: number
}

export interface ListaDePendencias {
  destino: 'cliente' | 'encarregados'
  titulo: string
  tickets: Pendencia[]
  /** A soma dos orçamentos de todos os tickets — não o número de linhas. */
  orcamentos: number
  valor: number
}


// ---------------------------------------------------------------------------
// O fechamento — as duas contas
//
// `chave` é o valor cru da view ('sem-ticket', 'espera-cliente'…) e `rotulo` é
// a frase. Os dois vêm do banco de propósito: a tela não traduz nada, senão
// existiriam duas listas de nomes para o mesmo estado, e um dia elas divergem.
// ---------------------------------------------------------------------------

export interface LinhaDoFechamento {
  /** A posição na leitura. 99 é o balde do desconhecido, e ele grita na tela. */
  ordem: number
  chave: string
  rotulo: string
  quantos: number
  valor: number
}

export interface ContaDoFechamento {
  linhas: LinhaDoFechamento[]
  /** A soma das linhas. */
  total: number
  valor: number
  /** Quantos existem na tabela, contados POR FORA da soma — é o que permite
   *  a diferença ser diferente de zero. */
  universo: number
  diferenca: number
}

export interface Fechamento {
  notas: ContaDoFechamento
  orcamentos: ContaDoFechamento
  ponte: {
    notas_usadas: number
    vinculos_vivos: number
    usadas_sem_vinculo: number
    vinculo_sem_nota_usada: number
    vinculo_de_orcamento_apagado: number
  }
}
