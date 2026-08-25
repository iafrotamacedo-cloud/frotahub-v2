// rev 3 — a tela inicial
//
// Os blocos do sistema, no mesmo desenho de barras verticais do painel de
// Orçamentos. É a decisão do dono em 25/08/2026: um esquema de menu só para o
// programa inteiro — quem aprende a operar uma tela sabe operar todas.
//
// O que ainda não existe aparece apagado, para dar a medida do que falta
// (CORE-23).
import { type ItemMenu } from '../menu/arvore'
import { etapasDoMenu } from '../menu/etapas'
import { Painel } from '../componentes/Painel'

interface Props {
  nome: string
  /** A árvore que ESTE login enxerga — a mesma da barra lateral, nunca outra. */
  arvore: ItemMenu[]
  abrir: (caminho: ItemMenu[]) => void
}

export function Inicio({ nome, arvore, abrir }: Props) {
  const primeiroNome = nome.split(' ')[0]

  return (
    <>
      <header className="hero">
        <h1>Olá, {primeiroNome}.</h1>
        <p>Escolha por onde começar.</p>
      </header>

      <Painel
        etapas={etapasDoMenu(arvore)}
        aoEscolher={rota => {
          const item = arvore.find(i => i.rota === rota)
          if (item) abrir([item])
        }}
      />
    </>
  )
}
