// rev 1 — PCO > Excluídas/Substituídas (12/09/2026)
//
// SÓ LEITURA, DE PROPÓSITO
//
//	Pedido do dono: "pode criar um card novo em pco só com essa lista, mas
//	sem opção de restauração, apenas dados e filtros." É o retrato de uma OC
//	que já tinha sido enviada e foi excluída ou substituída depois —
//	`ordens_compra_canceladas`, migração 063. Não tem "reparar", "ler",
//	"enviar" nem "excluir" — só "ver" (o PDF continua acessível) e os dois
//	filtros que existem no motor (tipo e período).
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { emReais, emDataHora, formatarCNPJ, type OrdemCancelada } from './tipos'

const ROTULO_TIPO: Record<OrdemCancelada['tipo'], string> = {
  excluida: 'Excluída',
  substituida: 'Substituída',
}

export function CanceladasPCO() {
  const [linhas, setLinhas] = useState<OrdemCancelada[] | null>(null)
  const [erro, setErro] = useState('')
  const [tipo, setTipo] = useState<'' | OrdemCancelada['tipo']>('')
  const [desde, setDesde] = useState('')
  const [ate, setAte] = useState('')
  const [vendo, setVendo] = useState<{ endereco: string; nome: string } | null>(null)

  const carregar = useCallback(async () => {
    try {
      const q = new URLSearchParams()
      if (tipo) q.set('tipo', tipo)
      if (desde) q.set('desde', desde)
      if (ate) q.set('ate', ate)
      const query = q.toString()
      const r = await motor<{ ordens: OrdemCancelada[] }>(
        '/administrativo/compras/pco/canceladas' + (query ? '?' + query : ''))
      setLinhas(r.ordens)
      setErro('')
    } catch (e) {
      setLinhas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
  }, [tipo, desde, ate])

  useEffect(() => { void carregar() }, [carregar])

  async function abrirArquivo(l: OrdemCancelada) {
    try {
      const r = await motor<{ url: string; nome: string }>(`/administrativo/compras/pco/canceladas/${l.id}/arquivo`)
      setVendo({ endereco: r.url, nome: r.nome || l.nome_arquivo || 'documento.pdf' })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  if (vendo) {
    return (
      <VisorDeDocumento
        endereco={vendo.endereco}
        nomeSugerido={vendo.nome}
        titulo={vendo.nome}
        voltar={() => setVendo(null)}
      />
    )
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Excluídas/Substituídas</h1>
          <p>OCs excluídas ou substituídas depois de já terem sido enviadas por PCO — só consulta.</p>
        </div>
      </header>

      <div className="adm-filtros-canceladas">
        <label>
          <span>Tipo</span>
          <select value={tipo} onChange={e => setTipo(e.target.value as typeof tipo)}>
            <option value="">todas</option>
            <option value="excluida">excluídas</option>
            <option value="substituida">substituídas</option>
          </select>
        </label>
        <label>
          <span>De</span>
          <input type="date" value={desde} onChange={e => setDesde(e.target.value)} />
        </label>
        <label>
          <span>Até</span>
          <input type="date" value={ate} onChange={e => setAte(e.target.value)} />
        </label>
      </div>

      {erro && <div className="erro-caixa">{erro}</div>}

      {linhas === null ? (
        <Carregando texto="Carregando..." />
      ) : linhas.length === 0 ? (
        <div className="vazio">Nenhuma OC excluída ou substituída depois do envio.</div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela adm-tabela-canceladas">
            <thead>
              <tr>
                <th>O.C.</th>
                <th>Obra/centro</th>
                <th>Faturamento</th>
                <th>Fornecedor</th>
                <th>Valor</th>
                <th>Tipo</th>
                <th>Enviada em</th>
                <th>Removida em</th>
                <th>Substituta</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {linhas.map(l => (
                <tr key={l.id}>
                  <td>{l.numero || '—'}</td>
                  <td>{l.obra_centro_custo || '—'}</td>
                  <td title={l.comprador_cnpj ? formatarCNPJ(l.comprador_cnpj) : undefined}>
                    {l.comprador_nome || '—'}
                  </td>
                  <td title={l.fornecedor_cnpj ? formatarCNPJ(l.fornecedor_cnpj) : undefined}>
                    {l.fornecedor_nome || '—'}
                  </td>
                  <td>{l.valor != null ? emReais(l.valor) : '—'}</td>
                  <td>
                    <span className={'pino ' + (l.tipo === 'excluida' ? 'pino-err' : 'pino-warn')}>
                      {ROTULO_TIPO[l.tipo]}
                    </span>
                  </td>
                  <td className="tri-fraco">{emDataHora(l.enviado_em)}</td>
                  <td className="tri-fraco">{emDataHora(l.removida_em)}</td>
                  <td>{l.substituta_numero || '—'}</td>
                  <td>
                    <button type="button" className="bt bt-mini" onClick={() => void abrirArquivo(l)}>ver</button>
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
