// rev 2 — o que a tela de usuários manipula
import type { Nivel } from '../../sessao/tipos'

export interface LinhaUsuario {
  id: string
  usuario: string
  nome: string
  ativo: boolean
  criado_em: string
  categorias: { codigo: string; nome: string; nivel: Nivel } | null
}

export interface Categoria {
  id: string
  codigo: string
  nome: string
  nivel: Nivel
  protegida: boolean
  ativo: boolean
}

export interface Pagina<T> {
  pagina: number
  por_pagina: number
  tem_mais: boolean
  usuarios?: T[]
  historico?: T[]
}

/** Um nó do organograma — `VinculoHierarquico.tsx`. */
export interface NoCadeia {
  id: string
  nome: string
  usuario: string
  nivel: Nivel
  categoria_nome: string
  /** Só verdadeiro no nó de topo quando ele vem do default (CEO único do
   *  cliente) — ainda não existe vínculo gravado até ali. */
  implicito?: boolean
}

/** Um candidato no popup de "quem entra aqui" — `GET /perfis`. */
export interface PerfilLeve {
  id: string
  nome: string
  usuario: string
  nivel: Nivel
  categoria_nome: string
  ativo: boolean
}
