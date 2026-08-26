// rev 5 — os ícones do sistema, desenhados à mão
//
// Sem biblioteca de ícones: são poucos, e assim eles herdam a cor e a espessura do
// resto da interface em vez de trazerem um estilo próprio (CORE-25).
import type { Icone as NomeIcone } from '../menu/arvore'

const TRACOS = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.7,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

const CAMINHOS: Record<NomeIcone, JSX.Element> = {
  'chave-inglesa': (
    <path d="M14.7 6.3a3.9 3.9 0 0 0 5.1 5.1l-8.3 8.3a2.4 2.4 0 0 1-3.4-3.4l8.3-8.3ZM14.7 6.3 17.9 3a5.4 5.4 0 0 1 3.1 3.1l-3.2 3.2" />
  ),
  'engrenagem': (
    <>
      <circle cx="12" cy="12" r="3.1" />
      <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5v.2a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1h.2a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z" />
    </>
  ),
  'loja': (
    <>
      <path d="M3.5 9.5 5 4.5h14l1.5 5" />
      <path d="M3.5 9.5a2.6 2.6 0 0 0 4.6 1.7 2.6 2.6 0 0 0 4.4 0 2.6 2.6 0 0 0 4.4 0 2.6 2.6 0 0 0 4.6-1.7" />
      <path d="M5 12v7.5h14V12" />
      <path d="M10 19.5v-4.2h4v4.2" />
    </>
  ),
  'servicos': (
    <>
      <path d="M4 20.5 9 15.5" />
      <path d="M8.4 12.2 4.6 8.4a3.6 3.6 0 0 1 4.6-4.6l3.8 3.8" />
      <path d="m13 7.6 3.4-3.4 3.9 3.9-3.4 3.4" />
      <path d="m11.5 13.2 3.8 3.8a3.6 3.6 0 0 0 4.6-4.6l-3.8-3.8" />
    </>
  ),
  'pessoas': (
    <>
      <path d="M15.5 20v-1.7a3.4 3.4 0 0 0-3.4-3.4H6.4A3.4 3.4 0 0 0 3 18.3V20" />
      <circle cx="9.2" cy="7.6" r="3.4" />
      <path d="M21 20v-1.7a3.4 3.4 0 0 0-2.6-3.3" />
      <path d="M16 4.3a3.4 3.4 0 0 1 0 6.6" />
    </>
  ),
  'cadeado': (
    <>
      <rect x="4.2" y="10.3" width="15.6" height="10" rx="2.2" />
      <path d="M8 10.3V7.4a4 4 0 0 1 8 0v2.9" />
      <path d="M12 14.4v2.1" />
    </>
  ),
  'pessoa': (
    <>
      <path d="M19.5 20v-1.9a4 4 0 0 0-4-4h-7a4 4 0 0 0-4 4V20" />
      <circle cx="12" cy="7.4" r="3.7" />
    </>
  ),
  'lista': (
    <>
      <rect x="3.4" y="4.6" width="17.2" height="14.8" rx="2" />
      <path d="M3.4 9.2h17.2" />
      <path d="M8.6 9.2v10.2" />
    </>
  ),

  // ---- Financeiro ----
  //
  // A SETA APONTA PARA FORA OU PARA DENTRO DA CAIXA, E É SÓ ISSO
  //   Dois ícones quase iguais, invertidos, são o desenho certo aqui: a
  //   diferença entre pagar e receber é a DIREÇÃO do dinheiro, e um par
  //   espelhado diz isso sem precisar de legenda.
  'dinheiro': (
    <>
      <rect x="2.6" y="6.2" width="18.8" height="11.6" rx="2" />
      <circle cx="12" cy="12" r="2.6" />
      <path d="M5.9 12h.2M17.9 12h.2" />
    </>
  ),
  'saida': (
    <>
      <path d="M10.4 4.6H5.6a2 2 0 0 0-2 2v10.8a2 2 0 0 0 2 2h4.8" />
      <path d="M20.4 12h-10" />
      <path d="m17 8.6 3.4 3.4-3.4 3.4" />
    </>
  ),
  'entrada': (
    <>
      <path d="M13.6 4.6h4.8a2 2 0 0 1 2 2v10.8a2 2 0 0 1-2 2h-4.8" />
      <path d="M3.6 12h10" />
      <path d="m10.2 8.6 3.4 3.4-3.4 3.4" />
    </>
  ),
  'balanca': (
    <>
      <path d="M12 4.8v14.7" />
      <path d="M8.4 19.5h7.2" />
      <path d="M4.6 7.6h14.8" />
      <path d="M4.6 7.6 2 13.3a2.6 2.6 0 0 0 5.2 0Z" />
      <path d="M19.4 7.6 16.8 13.3a2.6 2.6 0 0 0 5.2 0Z" />
    </>
  ),
}

export function Icone({ nome }: { nome: NomeIcone }) {
  return <svg viewBox="0 0 24 24" {...TRACOS} aria-hidden="true">{CAMINHOS[nome]}</svg>
}

export function Seta() {
  return (
    <svg className="ch" viewBox="0 0 24 24" {...TRACOS} aria-hidden="true">
      <path d="m9 5 7 7-7 7" />
    </svg>
  )
}

export function Menu() {
  return (
    <svg viewBox="0 0 24 24" {...TRACOS} aria-hidden="true">
      <path d="M4 7h16M4 12h16M4 17h16" />
    </svg>
  )
}

export function Relogio() {
  return (
    <svg viewBox="0 0 24 24" {...TRACOS} aria-hidden="true">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.2V12l3 1.8" />
    </svg>
  )
}
