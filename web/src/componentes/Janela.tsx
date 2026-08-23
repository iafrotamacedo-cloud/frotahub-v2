// rev 1 — a janela sobreposta
//
// Uma peça só para todas as caixas de diálogo do sistema (CORE-06). Fecha no Esc e
// no clique fora, devolve o foco ao teclado, e prende a rolagem do fundo — três
// detalhes que, feitos em cada tela, seriam feitos diferente em cada tela.
import { useEffect, useRef, type ReactNode } from 'react'

interface Props {
  titulo: string
  descricao?: string
  aoFechar: () => void
  children: ReactNode
  largura?: number
}

export function Janela({ titulo, descricao, aoFechar, children, largura = 460 }: Props) {
  const caixa = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function tecla(e: KeyboardEvent) {
      if (e.key === 'Escape') aoFechar()
    }
    document.addEventListener('keydown', tecla)
    document.body.style.overflow = 'hidden'
    caixa.current?.querySelector<HTMLElement>('input, select, button')?.focus()
    return () => {
      document.removeEventListener('keydown', tecla)
      document.body.style.overflow = ''
    }
  }, [aoFechar])

  return (
    <div className="jn-fundo" onMouseDown={e => { if (e.target === e.currentTarget) aoFechar() }}>
      <div className="jn" style={{ maxWidth: largura }} ref={caixa} role="dialog" aria-modal="true" aria-label={titulo}>
        <header className="jn-topo">
          <div>
            <h3>{titulo}</h3>
            {descricao && <p>{descricao}</p>}
          </div>
          <button type="button" className="jn-x" onClick={aoFechar} aria-label="Fechar">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
              <path d="m6 6 12 12M18 6 6 18" />
            </svg>
          </button>
        </header>
        {children}
      </div>
    </div>
  )
}
