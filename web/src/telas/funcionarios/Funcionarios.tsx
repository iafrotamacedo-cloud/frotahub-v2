// rev 1 — SESMT e DP: Funcionários
//
// Cadastro de quem trabalha na obra e a pilha de documento de cada um — o que
// a fiscalização pede: RG, CTPS, ASO, e os certificados de NR que a função
// exigir. Mesmo desenho de Usuários e Logins (busca com espera, paginação no
// banco), com um retrato de conformidade no topo em vez de um botão só.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import type { Perfil } from '../../sessao/tipos'
import { Carregando } from '../../componentes/Carregando'
import { FormFuncionario } from './FormFuncionario'
import { DocumentosFuncionario } from './DocumentosFuncionario'
import type { Funcionario, Funcao, Conformidade } from './tipos'

type Janelinha =
  | { tipo: 'nenhuma' }
  | { tipo: 'novo' }
  | { tipo: 'documentos'; alvo: Funcionario }

function alcanca(perfil: Perfil, rotina: string): boolean {
  return perfil.nivel === 'builder' || perfil.rotinas.includes(rotina)
}

export function Funcionarios({ perfil }: { perfil: Perfil }) {
  const [linhas, setLinhas] = useState<Funcionario[] | null>(null)
  const [funcoes, setFuncoes] = useState<Funcao[]>([])
  const [conformidade, setConformidade] = useState<Conformidade | null>(null)
  const [busca, setBusca] = useState('')
  const [buscaAplicada, setBuscaAplicada] = useState('')
  const [pagina, setPagina] = useState(1)
  const [temMais, setTemMais] = useState(false)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [janela, setJanela] = useState<Janelinha>({ tipo: 'nenhuma' })

  const podeCadastrar = alcanca(perfil, 'CONTRATO_FUNCIONARIOS_DADO_COMPLETO')

  useEffect(() => {
    const t = setTimeout(() => { setBuscaAplicada(busca.trim()); setPagina(1) }, 350)
    return () => clearTimeout(t)
  }, [busca])

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const params = new URLSearchParams({ pagina: String(pagina) })
      if (buscaAplicada) params.set('busca', buscaAplicada)
      const r = await motor<{ funcionarios: Funcionario[]; tem_mais: boolean }>(`/funcionarios?${params}`)
      setLinhas(r.funcionarios)
      setTemMais(r.tem_mais)
    } catch (e) {
      setLinhas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os funcionários.')
    }
  }, [pagina, buscaAplicada])

  const carregarConformidade = useCallback(async () => {
    try {
      const r = await motor<Conformidade>('/funcionarios/conformidade')
      setConformidade(r)
    } catch {
      // O retrato é um extra. Se ele falhar, a lista continua útil sozinha.
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])
  useEffect(() => { void carregarConformidade() }, [carregarConformidade])

  useEffect(() => {
    let vivo = true
    motor<{ funcoes: Funcao[] }>('/funcoes')
      .then(r => { if (vivo) setFuncoes(r.funcoes) })
      .catch(() => { /* o formulário já explica a lista vazia */ })
    return () => { vivo = false }
  }, [])

  function fechar(aviso: string | null, feito: string) {
    setJanela({ tipo: 'nenhuma' })
    setRecado(aviso ?? feito)
    void carregar()
    void carregarConformidade()
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Funcionários</h1>
          <p>Cadastro e documentação de quem trabalha na obra.</p>
        </div>
        {podeCadastrar && (
          <button className="bt bt-forte" type="button" onClick={() => setJanela({ tipo: 'novo' })}>
            Novo funcionário
          </button>
        )}
      </header>

      {conformidade && (
        <div className="cartoes">
          <section className="cartao cartao-numero">
            <b>{conformidade.total_funcionarios}</b>
            <span>funcionários ativos</span>
          </section>
          <section className="cartao cartao-numero cartao-ok">
            <b>{conformidade.conformes}</b>
            <span>em dia com a documentação</span>
          </section>
          <section className="cartao cartao-numero cartao-alerta">
            <b>{conformidade.com_pendencia}</b>
            <span>com pendência</span>
          </section>
        </div>
      )}

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}

      <div className="barra-busca">
        <input
          value={busca}
          onChange={e => setBusca(e.target.value)}
          placeholder="Buscar por nome ou CPF..."
          aria-label="Buscar"
        />
      </div>

      {erro && <div className="erro-caixa">{erro}</div>}

      {linhas === null ? (
        <Carregando texto="Carregando os funcionários..." />
      ) : erro ? null : linhas.length === 0 ? (
        <div className="vazio">
          {buscaAplicada ? 'Nenhum funcionário corresponde a essa busca.' : 'Nenhum funcionário cadastrado ainda.'}
        </div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>Nome</th>
                <th>Função</th>
                <th>CPF</th>
                <th>Situação</th>
                <th className="acoes-col">Ações</th>
              </tr>
            </thead>
            <tbody>
              {linhas.map(f => (
                <tr key={f.id} className={f.status === 'ativo' ? '' : 'inativa'}>
                  <td>{f.nome_completo}</td>
                  <td>{f.funcoes?.nome ?? '—'}</td>
                  <td><code>{f.cpf}</code></td>
                  <td>
                    <span className={'pino ' + (f.status === 'ativo' ? 'pino-ok' : 'pino-off')}>
                      {f.status === 'ativo' ? 'Ativo' : f.status === 'inativo' ? 'Inativo' : 'Desligado'}
                    </span>
                  </td>
                  <td className="acoes">
                    <button type="button" className="bt bt-mini" onClick={() => setJanela({ tipo: 'documentos', alvo: f })}>
                      Documentos
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!erro && (pagina > 1 || temMais) && (
        <div className="paginas">
          <button type="button" className="bt bt-neutro" disabled={pagina === 1} onClick={() => setPagina(p => p - 1)}>
            Anterior
          </button>
          <span>Página {pagina}</span>
          <button type="button" className="bt bt-neutro" disabled={!temMais} onClick={() => setPagina(p => p + 1)}>
            Próxima
          </button>
        </div>
      )}

      {janela.tipo === 'novo' && (
        <FormFuncionario
          funcoes={funcoes}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoSalvar={aviso => fechar(aviso, 'Funcionário cadastrado.')}
        />
      )}
      {janela.tipo === 'documentos' && (
        <DocumentosFuncionario
          funcionario={janela.alvo}
          perfil={perfil}
          aoFechar={() => { setJanela({ tipo: 'nenhuma' }); void carregarConformidade() }}
          aoMudar={() => void carregarConformidade()}
        />
      )}
    </>
  )
}
