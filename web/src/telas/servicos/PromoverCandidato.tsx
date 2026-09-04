// rev 1 — aprovar um candidato: escolher a conta e confirmar
//
// O motor exige "conta" no corpo porque um candidato ainda não tem chamado
// atribuído a Instalações ou Civil — é a única ação de Serviço que ainda
// pede essa escolha (a fila manual e o Kanban já sabem a conta pelo próprio
// chamado). Ver baleryan/.../servicos/kanban.go, PromoverCandidato.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import type { Candidato } from './tipos'

interface Props {
  candidato: Candidato
  aoFechar: () => void
  aoPromover: (aviso: string | null) => void
}

export function PromoverCandidato({ candidato, aoFechar, aoPromover }: Props) {
  const [conta, setConta] = useState<'instalacoes' | 'civil'>('instalacoes')
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    setErro(null)
    setSalvando(true)
    try {
      const resposta = await motor(`/servicos/candidatos/${candidato.id}/promover`, {
        metodo: 'POST',
        corpo: { conta },
      })
      aoPromover(avisoDe(resposta))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui marcar como Serviço.')
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo={`Marcar o ticket ${candidato.ticket} como Serviço`}
      descricao="O responsável do chamado troca no Trílogo e o card já entra no Quadro."
      aoFechar={aoFechar}
    >
      <form className="jn-corpo" onSubmit={enviar}>
        <label htmlFor="pc-conta">Qual conta atende este serviço</label>
        <select id="pc-conta" value={conta} onChange={e => setConta(e.target.value as 'instalacoes' | 'civil')}>
          <option value="instalacoes">Serviço Instalações</option>
          <option value="civil">Serviço Civil</option>
        </select>
        <p className="dica">Depende de quem vai executar — a mesma escolha que hoje se faz na tela do Trílogo.</p>

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando}>
            {salvando ? 'Marcando...' : 'Marcar como Serviço'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
