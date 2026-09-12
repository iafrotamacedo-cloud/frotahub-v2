// rev 1 — as três rotinas de Notas Fiscais (migração 064) e quem as alcança
import type { Perfil } from '../../sessao/tipos'

export const RotinaNFReceber = 'COMPRAS_NF_RECEBER'
export const RotinaNFEntregar = 'COMPRAS_NF_ENTREGAR'
export const RotinaNFConfigurarAcesso = 'COMPRAS_NF_CONFIGURAR_ACESSO'

/** Mesma regra usada em `Funcionarios.tsx`/`FichaChamado.tsx`: o builder
 *  passa sempre, o resto depende da rotina da categoria. Serve só para
 *  MOSTRAR ou ESCONDER botão — quem decide de verdade é o motor (P-29). */
export function temRotina(perfil: Perfil | null, rotina: string): boolean {
  return perfil?.nivel === 'builder' || !!perfil?.rotinas.includes(rotina)
}
