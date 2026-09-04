// rev 1 — a entrada manual: marcar um chamado como Serviço pelo número
//
// A OUTRA METADE DA REGRA DE DUPLICAÇÃO
//
//	Não pede "conta" — o motor já lê de chamados.conta (ver baleryan/.../
//	servicos/kanban.go, MarcarComoServico). Se o chamado já estiver na fila
//	(marcado por aqui, pelo gatilho automático, ou por um Candidato
//	promovido), o motor não repete a troca no Trílogo — só avisa que já
//	estava lá, e esta janela mostra isso como sucesso, não como erro.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor } from '../../motor/cliente'

interface Resposta {
  ok: boolean
  responsavel: string
  ja_estava_na_fila?: boolean
}

interface Props {
  aoFechar: () => void
  aoMarcar: (recado: string) => void
}

export function MarcarComoServico({ aoFechar, aoMarcar }: Props) {
  const [numero, setNumero] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    const n = numero.trim()
    if (!n) return
    setErro(null)
    setSalvando(true)
    try {
      const r = await motor<Resposta>(`/servicos/chamados/${encodeURIComponent(n)}/fila`, { metodo: 'POST' })
      aoMarcar(
        r.ja_estava_na_fila
          ? `O ticket ${n} já estava na fila de Serviço (${r.responsavel}).`
          : `Ticket ${n} marcado como Serviço — responsável agora é ${r.responsavel}.`,
      )
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui marcar este chamado.')
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo="Marcar chamado como Serviço"
      descricao="Pelo número do ticket no Trílogo — a conta é lida do próprio chamado."
      aoFechar={aoFechar}
    >
      <form className="jn-corpo" onSubmit={enviar}>
        <label htmlFor="mc-numero">Número do ticket</label>
        <input
          id="mc-numero" inputMode="numeric" value={numero}
          onChange={e => setNumero(e.target.value)} placeholder="135613" autoFocus
        />

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando || !numero.trim()}>
            {salvando ? 'Marcando...' : 'Marcar como Serviço'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
