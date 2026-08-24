// rev 3 — o que o sistema sabe sobre quem está logado
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
  /**
   * As rotinas que esta categoria alcança.
   *
   * Serve para MONTAR O MENU, e só. Quem decide de verdade é o motor, a cada
   * chamada: esconder um item não protege nada, apenas evita oferecer uma porta
   * que não abre (P-29). Vazio para o builder — ele passa sempre, sem lista.
   */
  rotinas: string[]
}

/** O builder passa sempre, aconteça o que acontecer com a matriz de permissões. */
export function ehBuilder(p: Perfil | null): boolean {
  return p?.nivel === 'builder'
}
