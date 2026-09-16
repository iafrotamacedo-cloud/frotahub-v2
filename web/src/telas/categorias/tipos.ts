// rev 2 — o que a tela de categorias manipula
import type { Nivel } from '../../sessao/tipos'

export interface Categoria {
  id: string
  codigo: string
  nome: string
  nivel: Nivel
  protegida: boolean
  ativo: boolean
  criado_em: string
}

export interface Rotina {
  codigo: string
  nome: string
  modulo: string
  ordem: number
  /** Só vem preenchido pra quem pode gerenciar acesso — em qual das 5-6
   *  seções do menu esta rotina mora, pro bypass de módulo do CEO. */
  modulo_menu?: string
  /** Falso até o builder decidir o contrário — "nasce oculto" (ver
   *  066_niveis_gerencial_supervisorio.sql / 067_categoria_modulos_e_bypass.sql). */
  liberada_para_bypass?: boolean
}

export interface Matriz {
  categoria: Categoria
  rotinas: Rotina[]
  permitidas: string[]
  /** Verdadeiro para a categoria do dono do sistema: ela alcança tudo por construção. */
  ignora_matriz: boolean
  /** Só vem preenchido quando `categoria.nivel === 'ceo'` — os módulos que o
   *  builder já liberou por inteiro pra esta categoria. */
  modulos_liberados?: string[]
}

/**
 * Os níveis que a tela oferece. `builder` fica de fora — ver o comentário no
 * motor. `fornecedor` (074_portal_fornecedor.sql) vem por último: é o único
 * que não entra na cadeia de hierarquia dos outros quatro.
 */
export const NIVEIS: { valor: Nivel; rotulo: string }[] = [
  { valor: 'operacional', rotulo: 'Operacional' },
  { valor: 'supervisorio', rotulo: 'Supervisório' },
  { valor: 'gerencial', rotulo: 'Gerencial' },
  { valor: 'ceo', rotulo: 'CEO' },
  { valor: 'fornecedor', rotulo: 'Fornecedor' },
]

/**
 * Os níveis que a tela do CEO oferece — um degrau abaixo do que o builder
 * alcança (nem `ceo`, nem `builder`, os dois protegidos deste form). Ver
 * `Categorias.tsx`, prop `escopo="gerencial"`.
 */
export const NIVEIS_CEO: { valor: Nivel; rotulo: string }[] = NIVEIS.filter(n => n.valor !== 'ceo')

/**
 * As 5-6 grandes seções do menu — vocabulário fechado, o mesmo que
 * `rotinas.modulo_menu` e `categoria_modulos_liberados.modulo` no banco
 * (067_categoria_modulos_e_bypass.sql). Nada a ver com `Rotina.modulo`, que é
 * só o agrupamento mais fino da própria tela de Permissões.
 */
export const MODULOS: { valor: string; rotulo: string }[] = [
  { valor: 'administrativo', rotulo: 'Administrativo' },
  { valor: 'manutencao', rotulo: 'Manutenção' },
  { valor: 'engenharia', rotulo: 'Engenharia' },
  { valor: 'sesmt-dp', rotulo: 'SESMT/DP' },
  { valor: 'configuracoes', rotulo: 'Configurações' },
]
