// rev 1 — a tela que pede o espaço todo
//
// O PROBLEMA QUE ISTO RESOLVE
//
//	Quando a nota aparece para ser corrigida, a pessoa precisa LER o papel — o
//	número do ticket, o item, o valor. Só que acima dela havia quatro faixas de
//	casca: a migalha com o título "Orçamentos", a barra de voltar, as cinco
//	abas de contagem e o cabeçalho da lista. Sobrava um terço da altura para o
//	documento, e o pé da nota ficava inalcançável.
//
// POR QUE UM CONTEXTO, E NÃO UM `prop`
//
//	Quem sabe que a nota está aberta é o visor — três telas diferentes o abrem,
//	cada uma guardando esse estado por conta. Quem precisa reagir são a casca do
//	programa (esconder a migalha) e a tela de Correções (esconder as abas), que
//	estão ACIMA de todas elas. Passar isso por `prop` obrigaria cada tela do
//	caminho a carregar um estado que não é dela e a repassá-lo sem usar.
//
//	Com o contexto, o visor DECLARA que precisa do espaço, e quem estiver acima
//	responde. Um mecanismo, três efeitos, e nenhuma tela intermediária envolvida.
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

const Contexto = createContext<{ focado: boolean; pedir: (v: boolean) => void }>({
  focado: false,
  pedir: () => {},
})

export function ProvedorDeFoco({ children }: { children: ReactNode }) {
  const [focado, pedir] = useState(false)
  return <Contexto.Provider value={{ focado, pedir }}>{children}</Contexto.Provider>
}

/** Lê se alguma tela pediu o espaço todo. */
export function useFocado(): boolean {
  return useContext(Contexto).focado
}

/**
 * Declara que ESTA tela precisa do espaço todo enquanto estiver montada.
 *
 * A limpeza no retorno não é detalhe: sem ela, sair da nota por qualquer
 * caminho — o OK, o voltar, um erro que desmonta — deixaria o programa sem
 * cabeçalho para sempre, e a pessoa sem como navegar.
 */
export function usePedirFoco(): void {
  const { pedir } = useContext(Contexto)
  useEffect(() => {
    pedir(true)
    return () => pedir(false)
  }, [pedir])
}
