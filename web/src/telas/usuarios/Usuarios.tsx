// rev 1 — Usuários e Logins
//
// A primeira rotina de verdade do FrotaHub novo. Tudo aqui passa pelo motor, e não
// direto pelo banco, porque criar login e trocar senha exigem a chave de serviço,
// que nunca chega ao navegador (CORE-09).
//
// A LISTA É PAGINADA E FILTRADA NO BANCO (CORE-10). Hoje são dois ou três logins e
// isso parece exagero. É justamente por isso que se faz agora: paginar três itens
// custa zero, despaginar trezentos custa uma reescrita.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import type { Perfil } from '../../sessao/tipos'
import { Carregando } from './Carregando'
import { FormUsuario } from './FormUsuario'
import { TrocarSenha } from './TrocarSenha'
import { Historico } from './Historico'
import type { Categoria, LinhaUsuario } from './tipos'

type Janelinha =
  | { tipo: 'nenhuma' }
  | { tipo: 'novo' }
  | { tipo: 'editar'; alvo: LinhaUsuario }
  | { tipo: 'senha'; alvo: LinhaUsuario }
  | { tipo: 'historico'; alvo: LinhaUsuario }

export function Usuarios({ perfil }: { perfil: Perfil }) {
  const [linhas, setLinhas] = useState<LinhaUsuario[] | null>(null)
  const [categorias, setCategorias] = useState<Categoria[]>([])
  const [busca, setBusca] = useState('')
  const [buscaAplicada, setBuscaAplicada] = useState('')
  const [pagina, setPagina] = useState(1)
  const [temMais, setTemMais] = useState(false)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [janela, setJanela] = useState<Janelinha>({ tipo: 'nenhuma' })
  const [ocupado, setOcupado] = useState<string | null>(null)

  // Digitar não dispara uma consulta por tecla: espera a pessoa parar.
  // Cada consulta é uma ida ao banco, e o banco também é recurso (CORE-01).
  useEffect(() => {
    const t = setTimeout(() => { setBuscaAplicada(busca.trim()); setPagina(1) }, 350)
    return () => clearTimeout(t)
  }, [busca])

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const params = new URLSearchParams({ pagina: String(pagina) })
      if (buscaAplicada) params.set('busca', buscaAplicada)
      const r = await motor<{ usuarios: LinhaUsuario[]; tem_mais: boolean }>(`/usuarios?${params}`)
      setLinhas(r.usuarios)
      setTemMais(r.tem_mais)
    } catch (e) {
      setLinhas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os usuários.')
    }
  }, [pagina, buscaAplicada])

  useEffect(() => { void carregar() }, [carregar])

  // As categorias alimentam o formulário. Carregam uma vez, não a cada abertura.
  useEffect(() => {
    let vivo = true
    motor<{ categorias: Categoria[] }>('/categorias')
      .then(r => { if (vivo) setCategorias(r.categorias) })
      .catch(() => { /* o formulário já explica a lista vazia */ })
    return () => { vivo = false }
  }, [])

  async function alternarSituacao(u: LinhaUsuario) {
    setOcupado(u.id)
    setRecado(null)
    try {
      const r = await motor<{ aviso?: string }>(`/usuarios/${u.id}`, {
        metodo: 'PATCH',
        corpo: { ativo: !u.ativo },
      })
      setRecado(r.aviso ?? `${u.nome} ${u.ativo ? 'foi desativado' : 'voltou a ter acesso'}.`)
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui mudar a situação.')
    } finally {
      setOcupado(null)
    }
  }

  function fechar(aviso: string | null, feito: string) {
    setJanela({ tipo: 'nenhuma' })
    setRecado(aviso ?? feito)
    void carregar()
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Usuários e Logins</h1>
          <p>Quem entra no FrotaHub, com que nome e em que categoria.</p>
        </div>
        <button className="bt bt-forte" type="button" onClick={() => setJanela({ tipo: 'novo' })}>
          Novo login
        </button>
      </header>

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
          placeholder="Buscar por nome ou usuário..."
          aria-label="Buscar"
        />
      </div>

      {erro && <div className="erro-caixa">{erro}</div>}

      {linhas === null ? (
        <Carregando texto="Carregando os usuários..." />
      ) : erro ? null : linhas.length === 0 ? (
        <div className="vazio">
          {buscaAplicada ? 'Nenhum login corresponde a essa busca.' : 'Nenhum login cadastrado ainda.'}
        </div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>Usuário</th>
                <th>Nome</th>
                <th>Categoria</th>
                <th>Situação</th>
                <th className="acoes-col">Ações</th>
              </tr>
            </thead>
            <tbody>
              {linhas.map(u => (
                <tr key={u.id} className={u.ativo ? '' : 'inativa'}>
                  <td><code>{u.usuario}</code></td>
                  <td>
                    {u.nome}
                    {u.id === perfil.id && <span className="voce">você</span>}
                  </td>
                  <td>{u.categorias?.nome ?? '—'}</td>
                  <td>
                    <span className={'pino ' + (u.ativo ? 'pino-ok' : 'pino-off')}>
                      {u.ativo ? 'Ativo' : 'Inativo'}
                    </span>
                  </td>
                  <td className="acoes">
                    <button type="button" className="bt bt-mini" onClick={() => setJanela({ tipo: 'editar', alvo: u })}>Editar</button>
                    <button type="button" className="bt bt-mini" onClick={() => setJanela({ tipo: 'senha', alvo: u })}>Senha</button>
                    <button type="button" className="bt bt-mini" onClick={() => setJanela({ tipo: 'historico', alvo: u })}>Histórico</button>
                    <button
                      type="button"
                      className={'bt bt-mini' + (u.ativo ? ' bt-perigo' : '')}
                      disabled={ocupado === u.id || u.id === perfil.id}
                      // O próprio login não se desativa: sem esta trava, um clique
                      // distraído tranca o dono do sistema para fora.
                      title={u.id === perfil.id ? 'Você não pode desativar o seu próprio login.' : undefined}
                      onClick={() => void alternarSituacao(u)}
                    >
                      {u.ativo ? 'Desativar' : 'Reativar'}
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
        <FormUsuario
          usuario={null}
          categorias={categorias}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoSalvar={aviso => fechar(aviso, 'Login criado.')}
        />
      )}
      {janela.tipo === 'editar' && (
        <FormUsuario
          usuario={janela.alvo}
          categorias={categorias}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoSalvar={aviso => fechar(aviso, 'Alteração salva.')}
        />
      )}
      {janela.tipo === 'senha' && (
        <TrocarSenha
          usuario={janela.alvo}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoSalvar={aviso => fechar(aviso, 'Senha trocada.')}
        />
      )}
      {janela.tipo === 'historico' && (
        <Historico usuario={janela.alvo} aoFechar={() => setJanela({ tipo: 'nenhuma' })} />
      )}
    </>
  )
}
