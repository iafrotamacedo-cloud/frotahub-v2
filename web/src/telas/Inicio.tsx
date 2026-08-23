// rev 2 — a tela inicial
//
// Mostra os blocos do sistema. O que ainda não existe aparece marcado, para dar a
// medida do que falta (CORE-23).
import { type ItemMenu } from '../menu/arvore'
import { Icone } from '../componentes/Icone'

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

      <div className="mods">
        {arvore.map(item => (
          <button
            key={item.t}
            className={'mod' + (item.breve ? ' breve' : '')}
            onClick={() => abrir([item])}
            type="button"
          >
            <div className="ic"><Icone nome={item.icone} /></div>
            <h3>{item.t}</h3>
            <p>{item.desc}</p>
            {item.breve && <p style={{ marginTop: 10 }}><span className="selo">Em breve</span></p>}
          </button>
        ))}
      </div>
    </>
  )
}
