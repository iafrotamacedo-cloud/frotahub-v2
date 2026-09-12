// rev 1 — janela de "cancelar esta nota fiscal", com motivo obrigatório
//
// O caso real: o fornecedor cancela a NF depois de já recebida (visto no
// protocolo de exemplo, 12/09/2026). O motivo fica gravado — `cancelada` não
// apaga a nota, só tira ela da esteira (ver `cancelarNF` em notas_fiscais.go).
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor } from '../../motor/cliente'

interface Props {
  notaID: string
  aoFechar: () => void
  aoSalvar: () => void
}

export function CancelarNF({ notaID, aoFechar, aoSalvar }: Props) {
  const [motivo, setMotivo] = useState('')
  const [erro, setErro] = useState('')
  const [enviando, setEnviando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    if (!motivo.trim()) {
      setErro('Explique por que esta nota fiscal está sendo cancelada.')
      return
    }
    setErro('')
    setEnviando(true)
    try {
      await motor(`/administrativo/nf/notas/${notaID}/cancelar`, { metodo: 'POST', corpo: { motivo: motivo.trim() } })
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui cancelar esta nota fiscal.')
      setEnviando(false)
    }
  }

  return (
    <Janela titulo="Cancelar nota fiscal" aoFechar={aoFechar}>
      <form className="jn-corpo" onSubmit={enviar}>
        <label htmlFor="nf-motivo">Motivo</label>
        <textarea id="nf-motivo" rows={3} value={motivo} onChange={e => setMotivo(e.target.value)} autoFocus />
        {erro && <div className="erro-caixa">{erro}</div>}
        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Voltar</button>
          <button type="submit" className="bt bt-perigo" disabled={enviando}>
            {enviando ? 'Cancelando...' : 'Cancelar nota'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
