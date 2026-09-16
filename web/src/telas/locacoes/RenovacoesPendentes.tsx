// rev 1 — Locações: a fila do RC (Fase 4, 16/09/2026)
//
// AGRUPAR POR FORNECEDOR É SÓ VISUAL
//
//	Uma OC de renovação costuma cobrir vários equipamentos do mesmo
//	fornecedor de uma vez — agrupar ajuda o RC a ver de cara "isto tudo vai
//	numa OC só", mas a seleção é livre: ele pode marcar equipamentos de
//	fornecedores diferentes e concluir em rodadas separadas.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { Confirmar } from '../../componentes/Confirmar'
import { useEhMobile } from '../../componentes/useEhMobile'
import { ConcluirRenovacao } from './ConcluirRenovacao'
import { emData, emReais, rotuloDaPeriodicidade, type PedidoDeRenovacao } from './tipos'

export function RenovacoesPendentes() {
  const ehMobile = useEhMobile()
  const [pedidos, setPedidos] = useState<PedidoDeRenovacao[] | null>(null)
  const [erro, setErro] = useState('')
  const [selecionados, setSelecionados] = useState<Set<string>>(new Set())
  const [concluindo, setConcluindo] = useState(false)
  const [cancelando, setCancelando] = useState<PedidoDeRenovacao | null>(null)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ renovacoes: PedidoDeRenovacao[] }>('/locacoes/renovacoes?estado=pendente')
      setPedidos(r.renovacoes)
      setErro('')
    } catch (e) {
      setPedidos([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os pedidos de renovação.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  function alternar(id: string) {
    setSelecionados(s => {
      const novo = new Set(s)
      if (novo.has(id)) novo.delete(id)
      else novo.add(id)
      return novo
    })
  }

  async function cancelar() {
    if (!cancelando) return
    try {
      await motor(`/locacoes/renovacoes/${cancelando.id}/cancelar`, { metodo: 'POST' })
      setCancelando(null)
      void carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui cancelar este pedido.')
      setCancelando(null)
    }
  }

  const selecionadosCompletos = (pedidos ?? []).filter(p => selecionados.has(p.id))

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Renovações pendentes</h1>
          <p>Pedidos da obra esperando a OC de renovação.</p>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {pedidos === null ? (
        <Carregando texto="Carregando..." />
      ) : pedidos.length === 0 ? (
        <div className="vazio">Nenhum pedido de renovação pendente.</div>
      ) : ehMobile ? (
        <div className="cl-lista">
          {pedidos.map(p => (
            <CartaoLinha
              key={p.id}
              titulo={p.descricao}
              acoes={
                <>
                  <label className="loc-check">
                    <input type="checkbox" checked={selecionados.has(p.id)} onChange={() => alternar(p.id)} />
                    selecionar
                  </label>
                  <button type="button" className="bt bt-mini bt-neutro" onClick={() => setCancelando(p)}>cancelar</button>
                </>
              }
              linhas={[
                { rotulo: 'Obra', valor: p.obra_centro_custo || '—' },
                { rotulo: 'Fornecedor', valor: p.fornecedor_nome || '—' },
                { rotulo: 'Vencia em', valor: emData(p.vencimento_atual) },
                { rotulo: 'Prazo pedido', valor: rotuloDaPeriodicidade(p.periodicidade) },
                { rotulo: 'Pedido em', valor: emData(p.pedida_em) },
              ]}
            />
          ))}
        </div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th></th><th>Equipamento</th><th>Obra</th><th>Fornecedor</th>
                <th>Vencia em</th><th>Prazo pedido</th><th>Qtd.</th><th>Valor</th><th></th>
              </tr>
            </thead>
            <tbody>
              {pedidos.map(p => (
                <tr key={p.id}>
                  <td><input type="checkbox" checked={selecionados.has(p.id)} onChange={() => alternar(p.id)} /></td>
                  <td>{p.descricao}</td>
                  <td>{p.obra_centro_custo || '—'}</td>
                  <td>{p.fornecedor_nome || '—'}</td>
                  <td>{emData(p.vencimento_atual)}</td>
                  <td>{rotuloDaPeriodicidade(p.periodicidade)}</td>
                  <td>{p.qtd_ativa}</td>
                  <td>{emReais(p.valor_unit)}</td>
                  <td><button type="button" className="bt bt-mini bt-neutro" onClick={() => setCancelando(p)}>cancelar</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selecionadosCompletos.length > 0 && (
        <div className="loc-barra-selecao">
          <span>{selecionadosCompletos.length} selecionado{selecionadosCompletos.length === 1 ? '' : 's'}</span>
          <button type="button" className="bt bt-forte" onClick={() => setConcluindo(true)}>Inserir OC e concluir</button>
        </div>
      )}

      {concluindo && (
        <ConcluirRenovacao
          pedidos={selecionadosCompletos}
          aoFechar={() => setConcluindo(false)}
          aoSalvar={() => { setConcluindo(false); setSelecionados(new Set()); void carregar() }}
        />
      )}

      {cancelando && (
        <Confirmar
          titulo="Cancelar pedido de renovação"
          mensagem={`Isto cancela o pedido de "${cancelando.descricao}" e libera o equipamento pra ser devolvido de novo.`}
          rotuloConfirmar="Cancelar pedido"
          perigo
          aoConfirmar={() => void cancelar()}
          aoFechar={() => setCancelando(null)}
        />
      )}
    </>
  )
}
