// rev 1 — Categorias
//
// O grupo a que cada login pertence, e — pela matriz — o que esse grupo alcança.
//
// Tela separada da de usuários de propósito. São duas frequências diferentes:
// mexer em login é rotina (alguém entra, alguém sai, alguém esquece a senha),
// mexer em categoria é raro e estrutural. Na mesma tela, a ação perigosa ficaria a
// um clique da corriqueira.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { Historico } from '../../componentes/Historico'
import { FormCategoria } from './FormCategoria'
import { Permissoes } from './Permissoes'
import type { Categoria } from './tipos'

type Janelinha =
  | { tipo: 'nenhuma' }
  | { tipo: 'nova' }
  | { tipo: 'editar'; alvo: Categoria }
  | { tipo: 'permissoes'; alvo: Categoria }
  | { tipo: 'historico'; alvo: Categoria }

export function Categorias() {
  const [linhas, setLinhas] = useState<Categoria[] | null>(null)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [janela, setJanela] = useState<Janelinha>({ tipo: 'nenhuma' })
  const [ocupado, setOcupado] = useState<string | null>(null)

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      // As arquivadas entram na lista: quem administra precisa enxergá-las para
      // poder devolvê-las à circulação. Quem só vai escolher uma categoria ao
      // criar um login recebe apenas as ativas — esse filtro é do motor.
      const r = await motor<{ categorias: Categoria[] }>('/categorias?incluir_inativas=1')
      setLinhas(r.categorias)
    } catch (e) {
      setLinhas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar as categorias.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function alternarSituacao(c: Categoria) {
    setOcupado(c.id)
    setRecado(null)
    setErro(null)
    try {
      const r = await motor<{ aviso?: string }>(`/categorias/${c.id}`, {
        metodo: 'PATCH',
        corpo: { ativo: !c.ativo },
      })
      setRecado(r.aviso ?? `${c.nome} ${c.ativo ? 'saiu de circulação' : 'voltou à circulação'}.`)
      await carregar()
    } catch (e) {
      // O motor recusa tirar de circulação uma categoria com gente dentro, e diz
      // quantos são. Esse texto vale mais que qualquer coisa que a tela invente.
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
          <h1>Categorias</h1>
          <p>Os grupos de acesso. O nível é da categoria, nunca da pessoa.</p>
        </div>
        <button className="bt bt-forte" type="button" onClick={() => setJanela({ tipo: 'nova' })}>
          Nova categoria
        </button>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}

      {erro && <div className="erro-caixa">{erro}</div>}

      {linhas === null ? (
        <Carregando texto="Carregando as categorias..." />
      ) : linhas.length === 0 ? (
        <div className="vazio">Nenhuma categoria cadastrada ainda.</div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>Código</th>
                <th>Nome</th>
                <th>Nível</th>
                <th>Situação</th>
                <th className="acoes-col">Ações</th>
              </tr>
            </thead>
            <tbody>
              {linhas.map(c => (
                <tr key={c.id} className={c.ativo ? '' : 'inativa'}>
                  <td><code>{c.codigo}</code></td>
                  <td>
                    {c.nome}
                    {c.protegida && <span className="voce">protegida</span>}
                  </td>
                  <td style={{ textTransform: 'capitalize' }}>{c.nivel}</td>
                  <td>
                    <span className={'pino ' + (c.ativo ? 'pino-ok' : 'pino-off')}>
                      {c.ativo ? 'Em uso' : 'Arquivada'}
                    </span>
                  </td>
                  <td className="acoes">
                    <button
                      type="button" className="bt bt-mini"
                      disabled={c.protegida}
                      title={c.protegida ? 'A categoria do dono do sistema não se edita.' : undefined}
                      onClick={() => setJanela({ tipo: 'editar', alvo: c })}
                    >
                      Editar
                    </button>
                    <button type="button" className="bt bt-mini" onClick={() => setJanela({ tipo: 'permissoes', alvo: c })}>
                      Permissões
                    </button>
                    <button type="button" className="bt bt-mini" onClick={() => setJanela({ tipo: 'historico', alvo: c })}>
                      Histórico
                    </button>
                    <button
                      type="button"
                      className={'bt bt-mini' + (c.ativo ? ' bt-perigo' : '')}
                      disabled={ocupado === c.id || c.protegida}
                      title={c.protegida ? 'A categoria do dono do sistema não sai de circulação.' : undefined}
                      onClick={() => void alternarSituacao(c)}
                    >
                      {c.ativo ? 'Arquivar' : 'Reativar'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {janela.tipo === 'nova' && (
        <FormCategoria
          categoria={null}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoSalvar={aviso => fechar(aviso, 'Categoria criada.')}
        />
      )}
      {janela.tipo === 'editar' && (
        <FormCategoria
          categoria={janela.alvo}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoSalvar={aviso => fechar(aviso, 'Alteração salva.')}
        />
      )}
      {janela.tipo === 'permissoes' && (
        <Permissoes
          categoria={janela.alvo}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoSalvar={(aviso, feito) => fechar(aviso, feito)}
        />
      )}
      {janela.tipo === 'historico' && (
        <Historico
          caminho={`/categorias/${janela.alvo.id}/historico`}
          titulo="Histórico"
          descricao={`${janela.alvo.nome} · ${janela.alvo.codigo}`}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
        />
      )}
    </>
  )
}
