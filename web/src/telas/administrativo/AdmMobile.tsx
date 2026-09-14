// rev 1 — Administrativo, a casca mobile (12/09/2026)
//
// PARA QUEM TRABALHA EM PÉ, NA OBRA
//
//	O painel de colunas (`Painel.tsx`) foi desenhado para mouse e para uma
//	altura de tela fixa, sem rolagem — bom num monitor, ruim com um polegar
//	e o celular na mão do encarregado ou do almoxarife em campo. Esta tela
//	toma a MESMA decisão de navegação — um cartão por item que a rotina de
//	quem está logado alcança, a mesma árvore de `arvoreVisivel` — só que em
//	lista vertical, com rolagem normal e alvo de toque grande.
//
// SÓ A CASCA, DE PROPÓSITO (pedido do dono, 14/09/2026)
//
//	Nenhum conteúdo novo mora aqui. Cada cartão abre a MESMA tela que o
//	resto do sistema já abre — `App.tsx` continua decidindo por `atual.tela`,
//	como sempre. O que muda é só o desenho da entrada, nunca o que existe
//	atrás dela.
import type { ItemMenu } from '../../menu/arvore'
import { Icone } from '../../componentes/Icone'

interface Props {
  itens: ItemMenu[]
  aoEscolher: (rota: string) => void
}

export function AdmMobile({ itens, aoEscolher }: Props) {
  return (
    <div className="adm-mob">
      {itens.map(item => (
        <button
          key={item.rota}
          type="button"
          className={'adm-mob-it' + (item.breve ? ' breve' : '')}
          disabled={item.breve}
          aria-label={item.desc ? `${item.t} — ${item.desc}` : item.t}
          onClick={() => aoEscolher(item.rota)}
        >
          <span className="adm-mob-ic"><Icone nome={item.icone} /></span>
          <span className="adm-mob-tx">
            <b>{item.t}</b>
            {item.desc && <span>{item.desc}</span>}
          </span>
          {item.breve
            ? <span className="pn-selo-breve">Em breve</span>
            : <span className="adm-mob-seta" aria-hidden>→</span>}
        </button>
      ))}
    </div>
  )
}
