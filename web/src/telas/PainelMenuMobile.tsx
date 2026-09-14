// rev 1 — o Padrão 1 do mobile: painel de menu (14/09/2026)
//
// SUBSTITUI `administrativo/AdmMobile.tsx`
//
//	Era a casca mobile só de Administrativo (`ehMobile && atual?.rota ===
//	'administrativo'`, em App.tsx). Generalizada: agora é a casca de
//	QUALQUER pasta do menu — Início, Manutenção, Configurações, Contrato
//	São Luiz — mesmo componente, para o sistema inteiro. `App.tsx` decide
//	quando ela entra (`ehMobile && atual?.sub?.length`), este arquivo só
//	desenha a lista.
//
// SÓ O NOME E O ÍCONE — NADA DE DESCRIÇÃO
//
//	Cartão grande, com o mesmo ícone do menu do PC, sem o texto explicativo
//	da barra. Ver `CartaoMobile.tsx` para o cartão em si.
import type { ItemMenu } from '../menu/arvore'
import { CartaoMobile } from '../componentes/CartaoMobile'
import { Icone } from '../componentes/Icone'

interface Props {
  itens: ItemMenu[]
  aoEscolher: (rota: string) => void
}

export function PainelMenuMobile({ itens, aoEscolher }: Props) {
  return (
    <div className="cm-lista">
      {itens.map(item => (
        <CartaoMobile
          key={item.rota}
          titulo={item.t}
          icone={<Icone nome={item.icone} />}
          desabilitado={item.breve}
          onClick={() => aoEscolher(item.rota)}
        />
      ))}
    </div>
  )
}
