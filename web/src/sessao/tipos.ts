// rev 5 — o que o sistema sabe sobre quem está logado
//
// CINCO NÍVEIS, NÃO MAIS QUATRO (14/09/2026)
//
//	`gerente` virou `gerencial` e `comum` virou `operacional` — mesmos dois
//	nomes de sempre, só honestos com o que representam agora que a hierarquia
//	ganhou regra própria (quem controla quem, ver `db/migrations/
//	066_niveis_gerencial_supervisorio.sql`). `supervisorio` é novo, entre os
//	dois: controlado pelo Gerencial, controla o Operacional — sempre por
//	VÍNCULO HIERÁRQUICO (tabela `vinculos_hierarquicos`), nunca pela
//	categoria inteira de uma vez.
//
// `FORNECEDOR` É UM SEXTO, MAS FORA DA HIERARQUIA (074_portal_fornecedor.sql)
//
//	Os cinco de cima são todos gente da casa, numa cadeia de controle. O
//	fornecedor não responde a ninguém dali nem ninguém responde por ele — por
//	isso fica fora da cadeia, não vira um sexto degrau dela. `App.tsx` desvia
//	este nível pro `PortalFornecedor` ANTES de montar a casca normal: sem
//	menu, sem barra lateral, só a tela de enviar nota/DAV e a própria senha.
export type Nivel = 'builder' | 'ceo' | 'gerencial' | 'supervisorio' | 'operacional' | 'fornecedor'

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
