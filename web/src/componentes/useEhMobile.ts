// rev 2 — perguntar ao navegador se a tela é de celular
//
// O NÚMERO QUE DECIDE A CASCA INTEIRA (14/09/2026)
//
//	900px é o "isto é celular" único do FrotaHub — `App.tsx` usa este hook
//	pra decidir a casca inteira (sem gaveta lateral, sem hambúrguer) e cada
//	tela-hub (`Compras.tsx`, `Pco.tsx`, ...) usa o mesmo hook pra trocar
//	`<Painel>` por `<PainelDadosMobile>`. Um número só, um lugar só.
import { useEffect, useState } from 'react'

const LARGURA_CELULAR = '(max-width: 900px)'

export function useEhMobile(): boolean {
  const [ehMobile, setEhMobile] = useState(() => window.matchMedia(LARGURA_CELULAR).matches)

  useEffect(() => {
    const consulta = window.matchMedia(LARGURA_CELULAR)
    const ouvir = (e: MediaQueryListEvent) => setEhMobile(e.matches)
    consulta.addEventListener('change', ouvir)
    return () => consulta.removeEventListener('change', ouvir)
  }, [])

  return ehMobile
}
