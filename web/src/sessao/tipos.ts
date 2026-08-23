// rev 1 — o que o sistema sabe sobre quem está logado
export type Nivel = 'builder' | 'ceo' | 'gerente' | 'comum'

export interface Perfil {
  id: string
  usuario: string
  nome: string
  nivel: Nivel
  ativo: boolean
}
