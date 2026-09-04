// rev 1 — as células que as três listas de Serviço repetem
//
// Ticket / loja / conta / descrição / data / valor no mesmo desenho de
// trilogo/DadosTrilogo.tsx. As três tabelas (Candidatos, as 9 filas, a
// Planilha) só diferem no que VEM DEPOIS dessas colunas — motivo e ações,
// valor e ações, status/PCO/NF. Sem isto, o mesmo ponto azul da conta
// existiria em três lugares e o primeiro a mudar deixaria os outros para trás.
import { useEffect, useLayoutEffect, useRef } from 'react'
import { ajustarCelulas } from '../trilogo/encolher'
import { contaPorExtenso, emReais, quando } from '../trilogo/tipos'

export function useEncolher(chave: unknown) {
  const corpo = useRef<HTMLTableSectionElement>(null)
  useLayoutEffect(() => { ajustarCelulas(corpo.current) }, [chave])
  useEffect(() => {
    let t: number | undefined
    function aoRedimensionar() {
      window.clearTimeout(t)
      t = window.setTimeout(() => ajustarCelulas(corpo.current), 120)
    }
    window.addEventListener('resize', aoRedimensionar)
    return () => { window.clearTimeout(t); window.removeEventListener('resize', aoRedimensionar) }
  }, [])
  return corpo
}

export function CelulaTicket({ ticket, aoAbrir }: { ticket: number; aoAbrir?: () => void }) {
  return (
    <td className="c-ticket">
      {aoAbrir ? (
        <button
          type="button"
          className="tri-num"
          onClick={e => { e.stopPropagation(); aoAbrir() }}
        >
          {ticket}
        </button>
      ) : (
        <span className="tri-num">{ticket}</span>
      )}
    </td>
  )
}

export function CelulaLoja({ loja }: { loja: string | null | undefined }) {
  return (
    <td className="c-loja" data-encolhe="loja" data-base="13" data-peso="400">
      <span title={loja || undefined}>{loja || '—'}</span>
    </td>
  )
}

export function CelulaConta({ conta }: { conta: string }) {
  return (
    <td className="c-conta">
      <span className={'tri-conta ' + (conta === 'civil' ? 'ct-civil' : 'ct-inst')}>
        {contaPorExtenso(conta)}
      </span>
    </td>
  )
}

export function CelulaDescricao({ texto }: { texto: string | null | undefined }) {
  const limpo = (texto || '—').replace(/\s+/g, ' ').trim()
  return (
    <td data-encolhe="descricao" data-base="13" data-peso="400">
      <span title={texto || undefined}>{limpo}</span>
    </td>
  )
}

export function CelulaData({ iso }: { iso: string | null | undefined }) {
  return <td className="c-data tri-fraco">{iso ? quando(iso) : '—'}</td>
}

export function CelulaValor({ valor }: { valor: number | null | undefined }) {
  return <td className="c-valor">{valor != null ? emReais(valor) : '—'}</td>
}
