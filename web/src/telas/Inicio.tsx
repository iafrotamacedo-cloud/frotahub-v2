// rev 4 — a tela inicial
//
// Os blocos do sistema, no mesmo desenho de barras verticais do painel de
// Orçamentos. É a decisão do dono em 25/08/2026: um esquema de menu só para o
// programa inteiro — quem aprende a operar uma tela sabe operar todas.
//
// O que ainda não existe aparece apagado, para dar a medida do que falta
// (CORE-23).
//
// NO MOBILE, 3 CARTÕES A MAIS (14/09/2026)
//
//	Sem gaveta lateral, Rogue Worker e Minha conta/Sair não têm mais onde
//	morar — viravam ícones fixos, e o dono preferiu cartões na própria
//	Início, junto dos módulos, em vez de chrome permanente ocupando tela.
import { type ItemMenu } from '../menu/arvore'
import { etapasDoMenu } from '../menu/etapas'
import { Painel } from '../componentes/Painel'
import { CartaoMobile } from '../componentes/CartaoMobile'
import { Icone } from '../componentes/Icone'
import { useEhMobile } from '../componentes/useEhMobile'

interface Props {
  nome: string
  /** A árvore que ESTE login enxerga — a mesma da barra lateral, nunca outra. */
  arvore: ItemMenu[]
  abrir: (caminho: ItemMenu[]) => void
  abrirChat: () => void
  irParaMinhaConta: () => void
  sair: () => void
}

export function Inicio({ nome, arvore, abrir, abrirChat, irParaMinhaConta, sair }: Props) {
  const ehMobile = useEhMobile()
  const primeiroNome = nome.split(' ')[0]

  return (
    <>
      <header className="hero">
        <h1>Olá, {primeiroNome}.</h1>
        <p>Escolha por onde começar.</p>
      </header>

      {ehMobile ? (
        <div className="cm-lista">
          {arvore.map(item => (
            <CartaoMobile
              key={item.rota}
              titulo={item.t}
              icone={<Icone nome={item.icone} />}
              desabilitado={item.breve}
              onClick={() => abrir([item])}
            />
          ))}
          <CartaoMobile titulo="Rogue Worker" icone={<Icone nome="balao" />} onClick={abrirChat} />
          <CartaoMobile titulo="Minha conta" icone={<Icone nome="pessoa" />} onClick={irParaMinhaConta} />
          <CartaoMobile titulo="Sair" icone={<Icone nome="saida" />} onClick={sair} />
        </div>
      ) : (
        <Painel
          etapas={etapasDoMenu(arvore)}
          aoEscolher={rota => {
            const item = arvore.find(i => i.rota === rota)
            if (item) abrir([item])
          }}
        />
      )}
    </>
  )
}
