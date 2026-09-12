// rev 1 — o aviso de "excluí, desfazer?" no pé da tela
//
// POR QUE NÃO APAGA NA HORA
//   Excluir de vez (ver `substituicao.go`) não tem volta. O pedido do dono
//   foi um intervalo pra se arrepender: a linha some da lista na hora
//   (efeito imediato), mas a chamada de exclusão de verdade só acontece se
//   os 10s passarem sem resposta, ou se a pessoa clicar "não" antes disso —
//   "sim" cancela tudo, a OC nunca chega a ser apagada.
import { useEffect, useState } from 'react'

interface Props {
  mensagem: string
  segundos?: number
  aoDesfazer: () => void
  aoConfirmar: () => void
}

export function AvisoDesfazer({ mensagem, segundos = 10, aoDesfazer, aoConfirmar }: Props) {
  const [restante, setRestante] = useState(segundos)

  useEffect(() => {
    if (restante <= 0) {
      aoConfirmar()
      return
    }
    const t = window.setTimeout(() => setRestante(r => r - 1), 1000)
    return () => window.clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [restante])

  return (
    <div className="adm-aviso-desfazer" role="alert">
      <span>{mensagem} Deseja desfazer?</span>
      <div className="adm-aviso-desfazer-botoes">
        <button type="button" className="bt bt-forte" onClick={aoDesfazer}>Sim</button>
        <button type="button" className="bt bt-perigo" onClick={aoConfirmar}>Não</button>
        <span className="adm-aviso-desfazer-contagem">{restante}s</span>
      </div>
    </div>
  )
}
