// rev 2 — a pré-triagem: "será que isto é Serviço?"
//
// Só mostra PENDENTES — descartado é definitivo e resolvido já virou card no
// funil (ver baleryan/.../servicos/candidatos.go, ListarCandidatos). Sem
// paginação: é fila de decisão, não histórico.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor, avisoDe } from '../../../motor/cliente'
import { Carregando } from '../../../componentes/Carregando'
import { FichaChamado } from '../../trilogo/FichaChamado'
import type { Perfil } from '../../../sessao/tipos'
import type { Candidato } from '../tipos'
import { PromoverCandidato } from '../PromoverCandidato'

type Janelinha = { tipo: 'nenhuma' } | { tipo: 'promover'; alvo: Candidato }

export function Candidatos({ perfil, voltar }: { perfil: Perfil; voltar: () => void }) {
  const [linhas, setLinhas] = useState<Candidato[] | null>(null)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [agindo, setAgindo] = useState<string | null>(null)
  const [janela, setJanela] = useState<Janelinha>({ tipo: 'nenhuma' })
  // O ticket aberto na ficha — igual às outras listas de Serviço (ver
  // ListaDeServicos.tsx): a lista some, a ficha do Trílogo entra no lugar.
  const [aberto, setAberto] = useState<number | null>(null)

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const r = await motor<{ candidatos: Candidato[] }>('/servicos/candidatos')
      setLinhas(r.candidatos)
    } catch (e) {
      setLinhas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os candidatos.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function descartar(c: Candidato) {
    if (!window.confirm(`Marcar o chamado ${c.ticket} como "não é Serviço"? Isto é definitivo — ele não volta para esta fila.`)) return
    setAgindo(c.id)
    setErro(null)
    try {
      const resposta = await motor(`/servicos/candidatos/${c.id}/descartar`, { metodo: 'POST' })
      setRecado(avisoDe(resposta) ?? 'Descartado — não é Serviço.')
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui descartar.')
    } finally {
      setAgindo(null)
    }
  }

  if (aberto !== null) {
    return (
      <FichaChamado
        numero={String(aberto)}
        perfil={perfil}
        voltar={() => setAberto(null)}
      />
    )
  }

  return (
    <>
      <button type="button" className="bt bt-neutro sv-voltar" onClick={voltar}>‹ Voltar</button>

      <header className="hero">
        <h1>Candidatos</h1>
        <p>Chamados novos que o sistema suspeita não serem conserto do que já existia — aguardando alguém decidir.</p>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      {linhas === null ? (
        <Carregando texto="Carregando os candidatos..." />
      ) : erro ? null : linhas.length === 0 ? (
        <div className="vazio">Nenhum candidato pendente no momento.</div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>Ticket</th>
                <th>Por que o sistema suspeita</th>
                <th>Desde</th>
                <th className="acoes-col">Ações</th>
              </tr>
            </thead>
            <tbody>
              {linhas.map(c => (
                <tr key={c.id}>
                  <td>
                    <button type="button" className="sv-ticket" onClick={() => setAberto(c.ticket)}>
                      <code>{c.ticket}</code>
                    </button>
                  </td>
                  <td>{c.motivo || '—'}</td>
                  <td>{new Date(c.criado_em).toLocaleDateString('pt-BR')}</td>
                  <td className="acoes">
                    <button
                      type="button" className="bt bt-mini" disabled={agindo === c.id}
                      onClick={() => setJanela({ tipo: 'promover', alvo: c })}
                    >
                      É Serviço
                    </button>
                    <button
                      type="button" className="bt bt-mini bt-neutro" disabled={agindo === c.id}
                      onClick={() => void descartar(c)}
                    >
                      Não é Serviço
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {janela.tipo === 'promover' && (
        <PromoverCandidato
          candidato={janela.alvo}
          aoFechar={() => setJanela({ tipo: 'nenhuma' })}
          aoPromover={aviso => {
            setJanela({ tipo: 'nenhuma' })
            setRecado(aviso ?? 'Chamado marcado como Serviço.')
            void carregar()
          }}
        />
      )}
    </>
  )
}
