// rev 1 — o cartão-base do mobile (14/09/2026)
//
// UM CARTÃO SÓ PROS PADRÕES 1 E 2
//
//	Padrão 1 (painel de menu, ex: Administrativo, Manutenção): só o nome.
//	Padrão 2 (painel de dados, ex: Compras, PCO): nome + `subtitulo` (o
//	número — "3 ordens"). Mesma peça, um prop a mais — evita duas cópias do
//	mesmo botão por uma diferença de uma linha de texto (CORE-06).
//
// SEM ÍCONE, SEM DESCRIÇÃO
//
//	Pedido do dono: o cartão mostra só o nome, em letra grande, centralizado
//	— nada de ícone nem texto explicativo disputando espaço. A proporção
//	(altura = 0,4 × largura) é fixa, então o cartão sempre ocupa a mesma
//	fatia da tela, virando um alvo de toque grande e previsível.
import type { ReactNode } from 'react'

interface Props {
  titulo: string
  /** O número (Padrão 2) — "3 ordens", "12 chamados". Ausente = Padrão 1. */
  subtitulo?: ReactNode
  /** "Em breve" — cartão apagado, sem clique, mesmo espírito do resto do menu. */
  desabilitado?: boolean
  selo?: string
  onClick?: () => void
}

export function CartaoMobile({ titulo, subtitulo, desabilitado, selo, onClick }: Props) {
  return (
    <button
      type="button"
      className={'cm-cartao' + (desabilitado ? ' desabilitado' : '')}
      disabled={desabilitado}
      onClick={onClick}
    >
      <span className="cm-titulo">{titulo}</span>
      {subtitulo != null && !desabilitado && <span className="cm-subtitulo">{subtitulo}</span>}
      {desabilitado && <span className="cm-selo">{selo ?? 'Em breve'}</span>}
    </button>
  )
}
