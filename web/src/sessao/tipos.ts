// rev 2 — o que o sistema sabe sobre quem está logado
export type Nivel = 'builder' | 'ceo' | 'gerente' | 'comum'

export interface Perfil {
  id: string
  usuario: string
  nome: string
  ativo: boolean
  /** A empresa titular deste login. Não aparece em tela nenhuma. */
  clienteId: string
  categoriaId: string
  categoriaNome: string
  /** Vem da CATEGORIA, nunca da pessoa — assim os dois não podem discordar. */
  nivel: Nivel
}

/** O builder passa sempre, aconteça o que acontecer com a matriz de permissões. */
export function ehBuilder(p: Perfil | null): boolean {
  return p?.nivel === 'builder'
}
