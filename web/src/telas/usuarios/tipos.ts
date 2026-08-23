// rev 1 — o que a tela de usuários manipula
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

export interface Par {
  de: unknown
  para: unknown
}

export interface Evento {
  id: number
  acao: string
  autor_usuario: string
  quando: string
  mudancas: Record<string, Par> | null
}

export interface Pagina<T> {
  pagina: number
  por_pagina: number
  tem_mais: boolean
  usuarios?: T[]
  historico?: T[]
}
