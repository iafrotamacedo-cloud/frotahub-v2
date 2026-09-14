// rev 1 — o Padrão 2 do mobile: painel de dados (14/09/2026)
//
// AS MESMAS `Etapa[]` DE SEMPRE
//
//	Recebe exatamente o que `Painel.tsx` já recebe — nenhuma tela muda como
//	monta seus contadores. Só o desenho muda: cartão único (Padrão 1) com o
//	número como subtítulo, em vez de colunas com prévia.
//
// "FILHOS" SEM HOVER
//
//	No PC, um cartão com `filhos` (ex: Compras › OCs Inseridas) se abre em
//	4 sub-cards passando o mouse por cima — não existe hover no dedo. Aqui
//	vira um passo a mais de navegação: tocar no cartão troca a lista visível
//	pelos filhos dele, com um "← voltar" no topo. É navegação interna desta
//	tela só (não mexe no endereço/`extra[]` do app) — mesmo espírito de
//	`NotasFiscais.tsx` já cuidar do próprio `onde` por dentro.
//
//	Tocar num FILHO chama `aoEscolher(filho.chave)` normal — é a mesma
//	função que cada tela (Compras, Orçamentos, Hub) já usa pra decodificar
//	chaves compostas tipo `"ocs-inseridas:processadas"` (`chave.split(':')`,
//	ver os comentários em `App.tsx`/`Compras.tsx`). Zero mudança lá.
import { useState } from 'react'
import { CartaoMobile } from './CartaoMobile'
import type { Etapa } from './Painel'

interface Props {
  etapas: Etapa[]
  aoEscolher: (chave: string) => void
}

export function PainelDadosMobile({ etapas, aoEscolher }: Props) {
  const [aberta, setAberta] = useState<Etapa | null>(null)

  if (aberta) {
    return (
      <div className="cm-lista">
        <button type="button" className="cm-voltar" onClick={() => setAberta(null)}>
          ← {aberta.titulo}
        </button>
        {aberta.filhos!.map(f => (
          <CartaoMobile
            key={f.chave}
            titulo={f.titulo}
            subtitulo={formatarNumero(f.numero)}
            onClick={() => aoEscolher(f.chave)}
          />
        ))}
      </div>
    )
  }

  return (
    <div className="cm-lista">
      {etapas.map(e => (
        <CartaoMobile
          key={e.chave}
          titulo={e.titulo}
          subtitulo={formatarNumero(e.numero, e.rotulo)}
          desabilitado={e.desabilitada}
          selo={e.selo}
          onClick={() => (e.filhos?.length ? setAberta(e) : aoEscolher(e.chave))}
        />
      ))}
    </div>
  )
}

function formatarNumero(numero: number | null | undefined, rotulo?: string): string | undefined {
  if (numero === undefined) return undefined
  const n = numero === null ? '–' : numero.toLocaleString('pt-BR')
  return rotulo ? `${n} ${rotulo}` : n
}
