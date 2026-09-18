// rev 1 — Notas Fiscais > Aguardando NF de locação (migração 077, 17/09/2026)
//
// SÓ O ADM — NUNCA A OBRA
//
//	Estas notas nasceram no recebimento de um equipamento locado
//	(`locacoes/ReceberLocacao`), sem número nem valor ainda — a NF de
//	verdade chega por e-mail, direto pro escritório. Por isso a lista
//	mostra a OC (o que a obra já sabe) até o ADM anexar a nota de verdade.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { useEhMobile } from '../../componentes/useEhMobile'
import { AnexarNFLocacao } from './AnexarNFLocacao'
import { VisualizarComposicaoLocacao } from './VisualizarComposicaoLocacao'
import { emDataHora, type NotaFiscal } from './tipos'

export function AguardandoNFLocacao() {
  const ehMobile = useEhMobile()
  const [notas, setNotas] = useState<NotaFiscal[] | null>(null)
  const [erro, setErro] = useState('')
  const [anexando, setAnexando] = useState<NotaFiscal | null>(null)
  // O link novo abre a composição (OC + romaneio + fotos) — "anexar NF"
  // continua um botão à parte, ação diferente (18/09/2026).
  const [vendo, setVendo] = useState<NotaFiscal | null>(null)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ notas: NotaFiscal[] }>('/administrativo/nf/aguardando-locacao')
      setNotas(r.notas)
      setErro('')
    } catch (e) {
      setNotas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  if (vendo) {
    return <VisualizarComposicaoLocacao nota={vendo} aoFechar={() => setVendo(null)} />
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Aguardando NF de locação</h1>
          <p>Equipamento já recebido na obra — falta a NF que a locadora manda por e-mail.</p>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {notas === null ? (
        <Carregando texto="Carregando..." />
      ) : notas.length === 0 ? (
        <div className="vazio">Nenhuma locação aguardando NF.</div>
      ) : ehMobile ? (
        <div className="cl-lista">
          {notas.map(nf => (
            <CartaoLinha
              key={nf.id}
              titulo={`O.C. ${nf.ordem_numero ?? '—'}`}
              onClick={() => setVendo(nf)}
              linhas={[
                { rotulo: 'Obra/centro', valor: nf.obra_centro_custo || '—' },
                { rotulo: 'Recebido em', valor: emDataHora(nf.recebida_em) },
              ]}
              acoes={
                <button type="button" className="bt bt-mini bt-forte" onClick={() => setAnexando(nf)}>
                  anexar NF
                </button>
              }
            />
          ))}
        </div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr><th>O.C.</th><th>Obra/centro</th><th>Recebido em</th><th></th></tr>
            </thead>
            <tbody>
              {notas.map(nf => (
                <tr key={nf.id}>
                  <td>
                    <button type="button" className="bt-como-link" onClick={() => setVendo(nf)}>
                      {nf.ordem_numero || '—'}
                    </button>
                  </td>
                  <td>{nf.obra_centro_custo || '—'}</td>
                  <td className="tri-fraco">{emDataHora(nf.recebida_em)}</td>
                  <td>
                    <button type="button" className="bt bt-mini bt-forte" onClick={() => setAnexando(nf)}>
                      anexar NF
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {anexando && (
        <AnexarNFLocacao
          nota={anexando}
          aoFechar={() => setAnexando(null)}
          aoSalvar={() => { setAnexando(null); void carregar() }}
        />
      )}
    </>
  )
}
