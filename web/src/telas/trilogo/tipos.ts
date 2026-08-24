// rev 2 — o que as telas de Dados do Trílogo recebem do motor
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

// ---------------------------------------------------------------------------
// A ficha de um chamado
// ---------------------------------------------------------------------------

export interface Evento {
  quando: string
  tipo: string
  tipo_codigo: number | null
  status: string
  autor: string
  texto: string
}

export interface Custo {
  id: string
  tipo: string
  valor: string | number
  numero_documento: string | null
  empresa: string | null
  criado_em: string | null
}

export interface Anexo {
  id: string
  colecao: string
  nome: string
  extensao: string
  tamanho: number | null
  tipo: string | null
  autor: string | null
  quando: string | null
  copiar: boolean
  custo_id: string | null
  /** Onde a tela vai buscar o arquivo: no nosso armazém ou no Trílogo. */
  onde: 'armazem' | 'trilogo'
  link: string
  vence_em?: number
}

export interface Ficha {
  chamado: LinhaChamado & {
    tipo?: string | null
    natureza?: string | null
    prestadora?: string | null
    lido_em?: string | null
  }
  eventos: Evento[]
  custos: Custo[]
  anexos: Anexo[]
}

/** Um evento da linha do tempo, já pronto para desenhar. */
export interface Passo {
  quando: string
  titulo: string
  autor: string
  texto: string
  /** "13 anexos", "1 vídeo" — o que foi agrupado nesta linha. */
  nota: string
  /** Mudança de status ganha marca cheia; o resto, círculo vazio. */
  marco: boolean
}

/**
 * A linha do tempo como ela se lê.
 *
 * O Trílogo grava um evento por ANEXO. O chamado 130328 tem 33 eventos, e 13
 * deles são o mesmo "anexo" no mesmo segundo da abertura: metade da linha do
 * tempo é a mesma frase repetida. Aqui os anexos seguidos do mesmo autor, no
 * mesmo minuto, viram uma linha só — "13 anexos enviados".
 *
 * O dado no banco continua inteiro; o agrupamento é de LEITURA. Quem for tirar
 * métrica depois lê os 33.
 */
export function linhaDoTempo(eventos: Evento[]): Passo[] {
  const passos: Passo[] = []
  const contagem = new Map<Passo, number>()
  const jaAberto = new Map<string, Passo>()

  for (const e of eventos) {
    if (e.tipo === 'anexo') {
      // A chave é AUTOR + MINUTO, e a busca é pela linha já aberta com essa
      // chave — não pela linha anterior. Os 13 anexos da abertura do 130328 têm
      // o evento "Aberto" no meio deles; olhando só para a linha de trás, eles
      // viravam três entradas ("1 arquivo", "Aberto", "11 arquivos") em vez de
      // uma.
      const chave = (e.autor || '') + '|' + (e.quando || '').slice(0, 16)
      const linha = jaAberto.get(chave)
      if (linha) {
        const quantos = (contagem.get(linha) ?? 1) + 1
        contagem.set(linha, quantos)
        linha.nota = `${quantos} arquivos enviados`
        continue
      }
      const nova: Passo = {
        quando: e.quando, titulo: 'Anexos', autor: e.autor || '—',
        texto: '', nota: '1 arquivo enviado', marco: false,
      }
      contagem.set(nova, 1)
      jaAberto.set(chave, nova)
      passos.push(nova)
      continue
    }

    passos.push({
      quando: e.quando,
      titulo: tituloDoEvento(e),
      autor: e.autor || '—',
      texto: e.texto,
      nota: '',
      marco: e.tipo === 'status',
    })
  }
  return passos
}

function tituloDoEvento(e: Evento): string {
  // Mudança de status é o próprio status: "Executado" diz mais que "Status".
  if (e.tipo === 'status') return e.status || 'Status'
  switch (e.tipo) {
    case 'prestadora': return 'Prestadora'
    case 'responsavel': return 'Responsável'
    case 'prioridade': return 'Prioridade'
    case 'prazo': return 'Prazo'
    case 'comentario': return 'Comentário'
    default: return e.tipo || 'Evento'
  }
}

/** Tamanho de arquivo do jeito que se fala. */
export function tamanhoLegivel(bytes: number | null): string {
  if (!bytes) return ''
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return Math.round(bytes / 1024) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1).replace('.', ',') + ' MB'
}

export function ehImagem(a: Anexo): boolean {
  return (a.tipo ?? '').startsWith('image/') ||
    ['jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp'].includes((a.extensao || '').toLowerCase())
}

export function ehVideo(a: Anexo): boolean {
  return (a.tipo ?? '').startsWith('video/') ||
    ['mp4', 'mov', 'avi', '3gp', 'm4v', 'mkv', 'wmv', 'webm'].includes((a.extensao || '').toLowerCase())
}
