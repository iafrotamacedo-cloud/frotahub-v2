// rev 1 — Locações: pedir renovação (Fase 4, 16/09/2026)
//
// SÓ PEDE — QUEM COBRE O PERÍODO É O RC, DEPOIS
//
//	Este formulário não mexe em vencimento nenhum. Ele só cria o pedido
//	pendente que sinaliza o RC (card "Renovações pendentes"); enquanto isso,
//	Devolver fica bloqueado pra estes equipamentos (intertravamento).
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor } from '../../motor/cliente'
import { rotuloDaPeriodicidade, type EquipamentoLocado, type Periodicidade, type ResultadoDoPedidoDeRenovacao } from './tipos'

interface Props {
  equipamentos: EquipamentoLocado[]
  aoFechar: () => void
  aoSalvar: () => void
}

export function PedirRenovacao({ equipamentos, aoFechar, aoSalvar }: Props) {
  const [periodicidade, setPeriodicidade] = useState<Periodicidade | ''>('')
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)

  async function salvar(e?: FormEvent) {
    e?.preventDefault()
    setErro('')
    setSalvando(true)
    try {
      await motor<ResultadoDoPedidoDeRenovacao>('/locacoes/equipamentos/renovar', {
        metodo: 'POST',
        corpo: { equipamentos: equipamentos.map(e => e.id), periodicidade: periodicidade || undefined },
      })
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui registrar o pedido de renovação.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo={equipamentos.length === 1 ? `Pedir renovação · ${equipamentos[0].descricao}` : `Pedir renovação de ${equipamentos.length} equipamentos`}
      descricao="O RC vai ver este pedido e concluir com a OC de renovação."
      aoFechar={aoFechar}
    >
      <form className="jn-corpo" onSubmit={salvar}>
        <ul style={{ margin: '0 0 14px', paddingLeft: 18 }}>
          {equipamentos.map(e => (
            <li key={e.id}>{e.descricao} — {e.obra_centro_custo || 'obra não identificada'}</li>
          ))}
        </ul>

        <label htmlFor="ren-prazo">Prazo da renovação</label>
        <select id="ren-prazo" value={periodicidade} onChange={e => setPeriodicidade(e.target.value as Periodicidade | '')} disabled={salvando}>
          <option value="">Manter o prazo atual de cada equipamento</option>
          <option value="mensal">{rotuloDaPeriodicidade('mensal')}</option>
          <option value="quinzenal">{rotuloDaPeriodicidade('quinzenal')}</option>
          <option value="semanal">{rotuloDaPeriodicidade('semanal')}</option>
        </select>

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar} disabled={salvando}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando}>
            {salvando ? 'Enviando...' : 'Pedir renovação'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
