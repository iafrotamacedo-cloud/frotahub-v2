// rev 1 — as quatro rotinas de Locações (migração 072)
import type { Perfil } from '../../sessao/tipos'

export const RotinaLocacoesReceber = 'LOCACOES_RECEBER'
export const RotinaLocacoesMonitorar = 'LOCACOES_MONITORAR'
export const RotinaLocacoesDecidir = 'LOCACOES_DECIDIR'
export const RotinaLocacoesRenovarOC = 'LOCACOES_RENOVAR_OC'

/** Mesma regra usada em `administrativo/rotinasNF.ts`: o builder passa
 *  sempre, o resto depende da rotina da categoria. Só decide o que MOSTRAR —
 *  quem decide de verdade é o motor (P-29). */
export function temRotina(perfil: Perfil | null, rotina: string): boolean {
  return perfil?.nivel === 'builder' || !!perfil?.rotinas.includes(rotina)
}
