// rev 1 — a espera honesta
//
// O motor roda no plano gratuito do Render, que ADORMECE depois de um tempo sem
// uso. A primeira chamada do dia pode levar quase um minuto só para acordar o
// serviço. Uma tela em branco nesse intervalo parece defeito, e a pessoa recarrega
// a página — o que reinicia a espera.
//
// Por isso a mensagem muda depois de alguns segundos: em vez de esconder a
// demora, ela explica (P-18).
import { useEffect, useState } from 'react'

export function Carregando({ texto = 'Carregando...' }: { texto?: string }) {
  const [demorando, setDemorando] = useState(false)

  useEffect(() => {
    const t = setTimeout(() => setDemorando(true), 4000)
    return () => clearTimeout(t)
  }, [])

  return (
    <div className="carregando">
      <div className="giro" />
      <p>{demorando ? 'Acordando o servidor — a primeira vez do dia demora um pouco.' : texto}</p>
    </div>
  )
}
