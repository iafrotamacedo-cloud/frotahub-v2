// rev 1 — perguntar ao navegador se a tela é de celular
//
// MESMA MEDIDA DO RESTO DO SISTEMA
//
//	900px é o ponto onde `base.css` já vira a barra lateral em gaveta — o
//	único "isto é celular" que o FrotaHub tinha até hoje. Em vez de inventar
//	um segundo número, este hook só dá nome ao mesmo limite, para telas como
//	a de Administrativo (`AdmMobile.tsx`) decidirem o que desenhar.
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
