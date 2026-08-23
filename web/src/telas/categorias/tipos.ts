// rev 1 — o que a tela de categorias manipula
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
}

export interface Matriz {
  categoria: Categoria
  rotinas: Rotina[]
  permitidas: string[]
  /** Verdadeiro para a categoria do dono do sistema: ela alcança tudo por construção. */
  ignora_matriz: boolean
}

/** Os níveis que a tela oferece. `builder` fica de fora — ver o comentário no motor. */
export const NIVEIS: { valor: Nivel; rotulo: string }[] = [
  { valor: 'comum', rotulo: 'Comum' },
  { valor: 'gerente', rotulo: 'Gerente' },
  { valor: 'ceo', rotulo: 'CEO' },
]
