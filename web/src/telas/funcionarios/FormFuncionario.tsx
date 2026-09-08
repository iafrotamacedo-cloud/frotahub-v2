// rev 1 — cadastrar funcionário
//
// Exige CONTRATO_FUNCIONARIOS_DADO_COMPLETO, não CONTRATO_FUNCIONARIOS_DADOS: quem
// cadastra digita o CPF inteiro, então precisa do mesmo acesso de quem vê sem
// máscara. O motor confere de novo — este formulário só evita levar alguém a
// preencher uma tela inteira para ouvir "sem permissão" no final.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import type { Funcao } from './tipos'

interface Props {
  funcoes: Funcao[]
  aoFechar: () => void
  aoSalvar: (aviso: string | null) => void
}

export function FormFuncionario({ funcoes, aoFechar, aoSalvar }: Props) {
  const [nome, setNome] = useState('')
  const [cpf, setCpf] = useState('')
  const [funcaoId, setFuncaoId] = useState('')
  const [dataAdmissao, setDataAdmissao] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    setErro(null)
    setSalvando(true)
    try {
      const resposta = await motor('/funcionarios', {
        metodo: 'POST',
        corpo: {
          nome_completo: nome.trim(),
          cpf: cpf.trim(),
          funcao_id: funcaoId || null,
          data_admissao: dataAdmissao || null,
        },
      })
      aoSalvar(avisoDe(resposta))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui cadastrar.')
      setSalvando(false)
    }
  }

  return (
    <Janela titulo="Novo funcionário" descricao="Dados de admissão — os documentos entram depois, na ficha dele." aoFechar={aoFechar}>
      <form className="jn-corpo" onSubmit={enviar}>
        <label htmlFor="f-nome">Nome completo</label>
        <input id="f-nome" value={nome} onChange={e => setNome(e.target.value)} placeholder="José da Silva" />

        <label htmlFor="f-cpf">CPF</label>
        <input id="f-cpf" value={cpf} onChange={e => setCpf(e.target.value)} placeholder="000.000.000-00" />

        <label htmlFor="f-funcao">Função</label>
        {funcoes.length === 0 ? (
          <div className="vazio-inline">
            Nenhuma função cadastrada ainda. Dá para cadastrar o funcionário sem
            função e associar depois.
          </div>
        ) : (
          <select id="f-funcao" value={funcaoId} onChange={e => setFuncaoId(e.target.value)}>
            <option value="">— sem função definida —</option>
            {funcoes.map(f => (
              <option key={f.id} value={f.id}>{f.nome}</option>
            ))}
          </select>
        )}

        <label htmlFor="f-admissao">Data de admissão</label>
        <input id="f-admissao" type="date" value={dataAdmissao} onChange={e => setDataAdmissao(e.target.value)} />

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando || !nome.trim() || !cpf.trim()}>
            {salvando ? 'Salvando...' : 'Cadastrar'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
