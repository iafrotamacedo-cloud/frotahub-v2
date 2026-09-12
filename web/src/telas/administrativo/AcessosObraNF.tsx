// rev 1 — Notas Fiscais > Acessos por obra (12/09/2026)
//
// "deve haver uma configuração onde auths de nível superior decidem quais
// obras ele pode ver/editar" — o dono, sobre o almoxarife. Escolhe um
// perfil, escolhe uma obra (do catálogo já aprendido em `centros_custo`),
// concede. A lista de baixo mostra quem já tem o quê, com revogar.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { emDataHora, type AcessoObraNF, type ObraNF, type PerfilParaAcessoNF } from './tipos'

export function AcessosObraNF() {
  const [obras, setObras] = useState<ObraNF[] | null>(null)
  const [perfis, setPerfis] = useState<PerfilParaAcessoNF[] | null>(null)
  const [acessos, setAcessos] = useState<AcessoObraNF[] | null>(null)
  const [erro, setErro] = useState('')
  const [perfilID, setPerfilID] = useState('')
  const [centroID, setCentroID] = useState('')
  const [concedendo, setConcedendo] = useState(false)

  const carregar = useCallback(async () => {
    try {
      const [o, p, a] = await Promise.all([
        motor<{ obras: ObraNF[] }>('/administrativo/nf/obras'),
        motor<{ perfis: PerfilParaAcessoNF[] }>('/administrativo/nf/perfis'),
        motor<{ acessos: AcessoObraNF[] }>('/administrativo/nf/acessos'),
      ])
      setObras(o.obras)
      setPerfis(p.perfis)
      setAcessos(a.acessos)
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar esta tela.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function conceder() {
    if (!perfilID || !centroID) return
    setConcedendo(true)
    try {
      await motor('/administrativo/nf/acessos', {
        metodo: 'POST',
        corpo: { perfil_id: perfilID, centro_custo_id: centroID },
      })
      setPerfilID('')
      setCentroID('')
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui conceder este acesso.')
    } finally {
      setConcedendo(false)
    }
  }

  async function revogar(id: string) {
    setAcessos(atual => (atual ? atual.filter(x => x.id !== id) : atual))
    try {
      await motor(`/administrativo/nf/acessos/${id}`, { metodo: 'DELETE' })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui revogar este acesso.')
      await carregar()
    }
  }

  if (obras === null || perfis === null || acessos === null) return <Carregando />

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Acessos por obra</h1>
          <p>Quem pode receber nota fiscal de qual obra — sem uma linha aqui, o almoxarife não vê a OC na fila.</p>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {obras.length === 0 && (
        <div className="vazio">
          Ainda não há nenhuma obra conhecida pelo sistema (`centros_custo` está em modo de
          teste). Nada para conceder até OCs de verdade começarem a passar pela leitura.
        </div>
      )}

      {obras.length > 0 && (
        <div className="adm-filtros-canceladas">
          <label>
            <span>Perfil</span>
            <select value={perfilID} onChange={e => setPerfilID(e.target.value)}>
              <option value="">Escolha...</option>
              {perfis.map(p => (
                <option key={p.id} value={p.id}>{p.nome} · {p.categorias?.nome ?? '—'}</option>
              ))}
            </select>
          </label>
          <label>
            <span>Obra</span>
            <select value={centroID} onChange={e => setCentroID(e.target.value)}>
              <option value="">Escolha...</option>
              {obras.map(o => (
                <option key={o.id} value={o.id}>{o.obra_centro_custo}</option>
              ))}
            </select>
          </label>
          <button type="button" className="bt bt-forte" disabled={!perfilID || !centroID || concedendo} onClick={() => void conceder()}>
            {concedendo ? 'Concedendo...' : 'Conceder acesso'}
          </button>
        </div>
      )}

      {acessos.length === 0 ? (
        <div className="vazio">Nenhum acesso concedido ainda.</div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr><th>Perfil</th><th>Obra</th><th>Concedido em</th><th></th></tr>
            </thead>
            <tbody>
              {acessos.map(a => (
                <tr key={a.id}>
                  <td>{a.perfis?.nome ?? '—'}</td>
                  <td>{a.centros_custo?.obra_centro_custo ?? '—'}</td>
                  <td className="tri-fraco">{emDataHora(a.criado_em)}</td>
                  <td>
                    <button type="button" className="bt bt-mini bt-perigo" onClick={() => void revogar(a.id)}>revogar</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}
