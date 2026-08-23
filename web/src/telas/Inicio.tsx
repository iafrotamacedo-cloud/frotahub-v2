// rev 1 — a tela inicial
//
// Mostra os blocos do sistema. O que ainda não existe aparece marcado, para dar a
// medida do que falta (CORE-23).
import { ARVORE, type ItemMenu } from '../menu/arvore'
import { Icone } from '../componentes/Icone'

interface Props {
  nome: string
  abrir: (caminho: ItemMenu[]) => void
}

export function Inicio({ nome, abrir }: Props) {
  const primeiroNome = nome.split(' ')[0]

  return (
    <>
      <header className="hero">
        <h1>Olá, {primeiroNome}.</h1>
        <p>Escolha por onde começar.</p>
      </header>

      <div className="mods">
        {ARVORE.map(item => (
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
