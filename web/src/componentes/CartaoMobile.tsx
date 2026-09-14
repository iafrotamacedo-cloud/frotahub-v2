// rev 2 — o cartão-base do mobile (14/09/2026, redesenhado 15/09/2026)
//
// UM CARTÃO SÓ PROS PADRÕES 1 E 2
//
//	Padrão 1 (painel de menu, ex: Administrativo, Manutenção): só o nome.
//	Padrão 2 (painel de dados, ex: Compras, PCO): nome + `subtitulo` (o
//	número — "3 ordens"). Mesma peça, um prop a mais — evita duas cópias do
//	mesmo botão por uma diferença de uma linha de texto (CORE-06).
//
// A IDENTIDADE DO PC VOLTOU, SÓ QUE EM COLUNA ÚNICA
//
//	A primeira versão tirou ícone e descrição do cartão — pedido de então.
//	Vendo ao vivo, o resultado lia como tela genérica, sem nada da marca: o
//	painel de barras do PC (`Painel.tsx`) é ali que a identidade mora — o
//	selo do ícone tingido de vermelho, o título à esquerda, a seta no
//	rodapé. Este cartão traz os TRÊS elementos de volta, na mesma
//	linguagem visual, sem copiar o grid do PC (que não cabe em coluna
//	única) nem a prévia (que não cabe sem espaço pra tabela).
import type { ReactNode } from 'react'

interface Props {
  titulo: string
  /** O número (Padrão 2) — "3 ordens", "12 chamados". Ausente = Padrão 1. */
  subtitulo?: ReactNode
  /** O ícone do módulo — o mesmo do menu do PC (`componentes/Icone.tsx`). */
  icone?: ReactNode
  /**
   * Tingido de vermelho (como o PC faz pra etapa "viva", com fila > 0) ou
   * neutro. Painel de menu puro (sem contador) é sempre vivo — é o mesmo
   * `.pn.simples` do PC: lá dentro, tudo é pra ser usado.
   */
  viva?: boolean
  /** "Em breve" — cartão apagado, sem clique, mesmo espírito do resto do menu. */
  desabilitado?: boolean
  selo?: string
  onClick?: () => void
}

export function CartaoMobile({ titulo, subtitulo, icone, viva = true, desabilitado, selo, onClick }: Props) {
  return (
    <button
      type="button"
      className={'cm-cartao' + (desabilitado ? ' desabilitado' : '') + (viva && !desabilitado ? ' viva' : '')}
      disabled={desabilitado}
      onClick={onClick}
    >
      {icone && <span className="cm-icone">{icone}</span>}
      <span className="cm-corpo">
        <span className="cm-titulo">{titulo}</span>
        {subtitulo != null && !desabilitado && <span className="cm-subtitulo">{subtitulo}</span>}
      </span>
      <span className="cm-rodape">
        <span>{desabilitado ? <b className="cm-selo">{selo ?? 'Em breve'}</b> : ''}</span>
        {!desabilitado && <span className="cm-seta" aria-hidden>→</span>}
      </span>
    </button>
  )
}
