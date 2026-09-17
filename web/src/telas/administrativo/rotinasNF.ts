// rev 2 — as rotinas de Notas Fiscais e quem as alcança
//
// MIGRAÇÃO 064 (12/09/2026): RECEBER, ENTREGAR, CONFIGURAR_ACESSO.
// MIGRAÇÃO 075 (17/09/2026, obra piloto MSL Fátima): mais duas —
//   RECEBER_PDF (subir a nota em PDF em vez de escanear — capacidade à
//   parte de RECEBER, concedida por categoria à escolha do dono) e
//   ENVIAR_CLIENTE (ENTREGAR virava sem querer duas responsabilidades —
//   confirmar chegada no escritório E marcar saída no malote; agora são
//   duas rotinas, ver o cabeçalho de `notas_fiscais.go`).
import type { Perfil } from '../../sessao/tipos'

export const RotinaNFReceber = 'COMPRAS_NF_RECEBER'
export const RotinaNFReceberPDF = 'COMPRAS_NF_RECEBER_PDF'
export const RotinaNFEntregar = 'COMPRAS_NF_ENTREGAR'
export const RotinaNFEnviarCliente = 'COMPRAS_NF_ENVIAR_CLIENTE'
export const RotinaNFConfigurarAcesso = 'COMPRAS_NF_CONFIGURAR_ACESSO'

/** Mesma regra usada em `Funcionarios.tsx`/`FichaChamado.tsx`: o builder
 *  passa sempre, o resto depende da rotina da categoria. Serve só para
 *  MOSTRAR ou ESCONDER botão — quem decide de verdade é o motor (P-29). */
export function temRotina(perfil: Perfil | null, rotina: string): boolean {
  return perfil?.nivel === 'builder' || !!perfil?.rotinas.includes(rotina)
}
