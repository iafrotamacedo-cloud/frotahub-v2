// rev 1 — a fonte das estatísticas
//
// O retrato chega do motor em UMA resposta, no formato `colunas` + `linhas`:
// os nomes das colunas uma vez, e os valores em array. Escrito assim, e não
// como lista de objetos, porque o nome de cada coluna repetido em cada linha
// era metade do peso.
//
// A CONTA NÃO MORA AQUI
//
//	Este arquivo só desempacota. Toda a estatística — mediana, tempo entre
//	marcos, índice contra o plano — está em `calculo.ts`, e num lugar só
//	(CORE-06). Foi assim que ela pôde ser conferida contra o retrato offline
//	linha a linha, e é assim que ela continua valendo agora que o dado vem do
//	banco.
//
// A LOJA É O NÚMERO DO TRÍLOGO, NÃO A POSIÇÃO NA LISTA
//
//	Enquanto a tela rodava offline, a loja era identificada pela POSIÇÃO dela no
//	array — a loja era "26" porque era a 26ª linha do arquivo. Isso funciona num
//	arquivo parado e quebra no primeiro dia em que uma unidade nova entra no meio
//	da lista: todo chamado passa a apontar para a loja errada, em silêncio, e o
//	número continua fazendo sentido na tela.
//
//	A view `estatisticas_unidades` (migração 042) entrega `id` = número do
//	Trílogo, e é por ele que chamado, orçamento e fatura dizem de que loja são.
//	Quem precisa da loja usa `porUnidade`, nunca `unidades[n]`.
import { motor } from '../../motor/cliente'

export interface Unidade {
  id: number
  nome: string
  cidade: string | null
  no_escopo: boolean
}

export interface Chamado {
  numero: number
  unidade: number | null
  conta: string | null
  status: string | null
  prioridade: string | null
  ambiente: string | null
  responsavel: string | null
  criado_em: string | null
  prazo: string | null
}

/** Uma mudança de status na linha do tempo. `sc` é o código do status. */
export interface Evento {
  chamado: number
  sc: number
  quando: string
}

export interface Custo {
  chamado: number
  tipo: string | null
  valor: number | null
  criado_em: string | null
}

export interface Orcamento {
  ticket: number
  unidade: number | null
  conta: string | null
  valor_nota: number | null
  valor: number | null
  ajustado_pelo_teto: boolean
  rateio: boolean
  status: string
  lancado_em: string | null
  faturado: boolean
  pago: boolean
  pago_em: string | null
  criado_em: string | null
  lancamento_bloqueio: string | null
  faturamento_direto: boolean
  fatura_id: string | null
}

export interface Ciclo {
  id: string
  competencia: string
  ate: string | null
  enviado_em: string | null
  fechado_em: string | null
}

export interface Fatura {
  id: string
  ciclo_id: string | null
  unidade: number | null
  conta: string | null
  pco_numero: string | null
  nf_numero: string | null
  nf_em: string | null
  recebido_em: string | null
  valor_recebido: number | null
  valor_faturado: number | null
}

export interface Documento {
  fila: string | null
  tipo: string | null
  emissao: string | null
  valor_total: number | null
  status: string | null
  inserido_em: string | null
  oculto_em: string | null
  bloqueio_motivo: string | null
  duplicada_de: boolean
  tickets: number
  tickets_soltos: number
  orcamentos: number
}

export interface Parametro {
  chave: string
  valor: number | null
  vigencia_inicio: string | null
  vigencia_fim: string | null
}

export interface Base {
  geradoEm: string
  unidades: Unidade[]
  /** A loja pelo número do Trílogo. É por aqui que se acha o nome. */
  porUnidade: Map<number, Unidade>
  chamados: Chamado[]
  eventos: Evento[]
  custos: Custo[]
  orcamentos: Orcamento[]
  ciclos: Ciclo[]
  faturas: Fatura[]
  documentos: Documento[]
  parametros: Parametro[]
}

/** Uma tabela como o motor a entrega. */
interface Peca {
  colunas: string[]
  linhas: unknown[][]
}

interface Retrato {
  gerado_em: string
  unidades: Peca
  chamados: Peca
  eventos: Peca
  custos: Peca
  orcamentos: Peca
  ciclos: Peca
  faturas: Peca
  documentos: Peca
  parametros: Peca
}

/**
 * Abre uma tabela de `colunas` + `linhas` em objetos.
 *
 * A posição manda: `linhas[i][3]` é sempre `colunas[3]`. O motor garante isso —
 * a lista que nomeia as colunas é a MESMA que pede os dados ao banco, e há
 * teste de sabotagem provando que uma coluna ausente vira nulo no lugar dela em
 * vez de deslocar a linha.
 */
function abrir<T>(p: Peca): T[] {
  return p.linhas.map(linha => {
    const o: Record<string, unknown> = {}
    p.colunas.forEach((c, i) => { o[c] = linha[i] })
    return o as T
  })
}

export const Fonte = {
  async carregar(): Promise<Base> {
    const d = await motor<Retrato>('/estatisticas')
    const unidades = abrir<Unidade>(d.unidades)
    return {
      geradoEm: d.gerado_em,
      unidades,
      porUnidade: new Map(unidades.map(u => [u.id, u])),
      chamados: abrir<Chamado>(d.chamados),
      eventos: abrir<Evento>(d.eventos),
      custos: abrir<Custo>(d.custos),
      orcamentos: abrir<Orcamento>(d.orcamentos),
      ciclos: abrir<Ciclo>(d.ciclos),
      faturas: abrir<Fatura>(d.faturas),
      documentos: abrir<Documento>(d.documentos),
      parametros: abrir<Parametro>(d.parametros),
    }
  },
}
