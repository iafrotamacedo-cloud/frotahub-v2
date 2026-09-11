// rev 1 — Configurações > PCO — Destinatários
//
// PERMISSÃO PRÓPRIA, SEPARADA DE "ENVIAR" (ver `Pco.tsx`/`pco_enviar.go`)
//
//	Pedido do dono (11/09/2026): editar QUEM recebe o e-mail é uma rotina
//	diferente de apertar o botão de enviar. Quem só enxerga esta tela não
//	necessariamente enxerga o botão de "Enviar tudo" em PCO, e vice-versa.
//
// NUNCA APAGA — SÓ ATIVA/DESATIVA (CORE-05, ver migração 061)
//
//	Mesmo padrão de Categorias: "Arquivar"/"Reativar", nunca um X que some a
//	linha. Um destinatário desligado continua no histórico de quem recebeu
//	o quê.
import { useCallback, useEffect, useRef, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { Historico } from '../../componentes/Historico'
import { emDataHora, type Destinatario } from './tipos'

export function DestinatariosPCO() {
  const [linhas, setLinhas] = useState<Destinatario[] | null>(null)
  const [erro, setErro] = useState('')
  const [recado, setRecado] = useState('')
  const [ocupado, setOcupado] = useState<string | null>(null)
  const [novoEmail, setNovoEmail] = useState('')
  const [salvandoNovo, setSalvandoNovo] = useState(false)
  const [verHistorico, setVerHistorico] = useState<Destinatario | null>(null)
  const campo = useRef<HTMLInputElement>(null)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ destinatarios: Destinatario[] }>('/administrativo/compras/pco/destinatarios')
      setLinhas(r.destinatarios)
      setErro('')
    } catch (e) {
      setLinhas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os destinatários.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function adicionar() {
    const email = novoEmail.trim()
    if (!email || salvandoNovo) return
    setSalvandoNovo(true)
    setErro('')
    try {
      await motor('/administrativo/compras/pco/destinatarios', { metodo: 'POST', corpo: { email } })
      setNovoEmail('')
      setRecado(`${email} cadastrado.`)
      await carregar()
      campo.current?.focus()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui cadastrar este e-mail.')
    } finally {
      setSalvandoNovo(false)
    }
  }

  async function alternar(d: Destinatario) {
    setOcupado(d.id)
    setErro('')
    setRecado('')
    try {
      const r = await motor<{ aviso?: string }>(`/administrativo/compras/pco/destinatarios/${d.id}`, {
        metodo: 'PATCH', corpo: { ativo: !d.ativo },
      })
      setRecado(r.aviso ?? `${d.email} ${d.ativo ? 'saiu de circulação' : 'voltou a receber o PCO'}.`)
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui mudar a situação.')
    } finally {
      setOcupado(null)
    }
  }

  return (
    <>
      <header className="hero">
        <h1>PCO — Destinatários</h1>
        <p>Quem recebe o e-mail do pacote de PCO. Todo ativo entra no "Para" do envio.</p>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado('')} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      <div className="adm-insercao">
        <p>adicionar um destinatário</p>
        <div className="linha">
          <input
            ref={campo}
            style={{ flex: 1, minWidth: 220, font: 'inherit', border: '1px solid var(--line)', borderRadius: 'var(--r-s)', padding: '9px 12px' }}
            type="email"
            placeholder="nome@empresa.com.br"
            value={novoEmail}
            onChange={e => setNovoEmail(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); void adicionar() } }}
          />
          <button type="button" className="bt bt-forte" disabled={!novoEmail.trim() || salvandoNovo} onClick={() => void adicionar()}>
            {salvandoNovo ? 'Adicionando…' : 'Adicionar'}
          </button>
        </div>
      </div>

      {linhas === null ? (
        <Carregando texto="Carregando..." />
      ) : linhas.length === 0 ? (
        <div className="vazio">Nenhum destinatário cadastrado ainda.</div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>E-mail</th>
                <th>Cadastrado em</th>
                <th>Situação</th>
                <th className="acoes-col">Ações</th>
              </tr>
            </thead>
            <tbody>
              {linhas.map(d => (
                <tr key={d.id} className={d.ativo ? '' : 'inativa'}>
                  <td>{d.email}</td>
                  <td>{emDataHora(d.criado_em)}</td>
                  <td>
                    <span className={'pino ' + (d.ativo ? 'pino-ok' : 'pino-off')}>
                      {d.ativo ? 'Recebendo' : 'Desativado'}
                    </span>
                  </td>
                  <td className="acoes">
                    <button type="button" className="bt bt-mini" onClick={() => setVerHistorico(d)}>histórico</button>
                    <button
                      type="button"
                      className={'bt bt-mini' + (d.ativo ? ' bt-perigo' : '')}
                      disabled={ocupado === d.id}
                      onClick={() => void alternar(d)}
                    >
                      {d.ativo ? 'desativar' : 'reativar'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {verHistorico && (
        <Historico
          caminho={`/administrativo/compras/pco/destinatarios/${verHistorico.id}/historico`}
          titulo="Histórico"
          descricao={verHistorico.email}
          aoFechar={() => setVerHistorico(null)}
        />
      )}
    </>
  )
}
