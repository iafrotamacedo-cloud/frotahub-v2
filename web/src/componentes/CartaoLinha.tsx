// rev 1 — o Padrão 3 do mobile: tabela vira cartão (14/09/2026)
//
// A PEÇA MAIS USADA DO CONJUNTO — E DE PROPÓSITO A MAIS SIMPLES
//
//	A pesquisa antes de desenhar isto confirmou que as telas de tabela de
//	hoje são heterogêneas demais (4 a 11 colunas, `TabelaDeOrdens.tsx` troca
//	o template inteiro da linha por uma flag, ações variam por status) pra
//	uma abstração grande em cima da tabela inteira. Esta peça não tenta
//	adivinhar colunas — cada tela continua buscando/filtrando os dados do
//	jeito que já faz, e só decide o que é `titulo` (o campo que mais
//	identifica a linha) e o que vira `linhas` (o resto, em pares
//	rótulo:valor).
//
// O CARTÃO INTEIRO CLICA QUANDO TEM FICHA ATRÁS
//
//	`onClick` no cartão inteiro (não só num botão "Ver") — pedido do dono.
//	`acoes` (Editar, Desativar...) fica num bloco à parte que para a
//	propagação do clique, senão tocar num botão de ação abriria a ficha
//	também.
import type { KeyboardEvent, ReactNode } from 'react'

export interface LinhaCartao {
  rotulo: string
  valor: ReactNode
}

interface Props {
  titulo: ReactNode
  linhas?: LinhaCartao[]
  acoes?: ReactNode
  onClick?: () => void
}

export function CartaoLinha({ titulo, linhas, acoes, onClick }: Props) {
  function aoTeclar(e: KeyboardEvent) {
    if (!onClick) return
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onClick()
    }
  }

  return (
    <div
      className={'cl-cartao' + (onClick ? ' clicavel' : '')}
      onClick={onClick}
      onKeyDown={aoTeclar}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      <div className="cl-titulo">{titulo}</div>

      {linhas && linhas.length > 0 && (
        <div className="cl-linhas">
          {linhas.map((l, i) => (
            <div className="cl-linha" key={i}>
              <span className="cl-rotulo">{l.rotulo}</span>
              <span className="cl-valor">{l.valor}</span>
            </div>
          ))}
        </div>
      )}

      {acoes && (
        <div className="cl-acoes" onClick={e => e.stopPropagation()}>
          {acoes}
        </div>
      )}
    </div>
  )
}
