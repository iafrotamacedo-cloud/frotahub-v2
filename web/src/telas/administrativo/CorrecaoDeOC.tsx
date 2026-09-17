// rev 1 — Compras > Correção de OC (migração 076, 17/09/2026)
//
// A FILA DO RC
//
//	NF chegou com valor diferente do total da OC — dentro do desvio
//	automático de 3% (entra sozinha) ou mandada manualmente pela obra (RO,
//	na tela de Aguardando NF, quando a diferença é grande demais pro sistema
//	presumir). O RC resolve de duas formas:
//
//	"corrigir" — sobe a OC certa do Obra Prima. O motor migra a(s) nota(s)
//	fiscal(is) já recebida(s) pra ela (voltam pra `recebida`, nunca à
//	frente disso) e apaga a OC velha — a OC nova nasce sem PCO enviado, cai
//	sozinha em "Pendentes de envio" pro reenvio, mesmo e-mail de sempre.
//
//	"voltar pra aguardando" — desiste da correção (era parcial mesmo, ou o
//	fornecedor troca a nota) e a OC volta a esperar nota fiscal normalmente.
import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react'
import { motor, enviarFormulario, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { Confirmar } from '../../componentes/Confirmar'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { useEhMobile } from '../../componentes/useEhMobile'
import { emReais, emDataHora, type OrdemDeCompra } from './tipos'

export function CorrecaoDeOC() {
  const ehMobile = useEhMobile()
  const [ordens, setOrdens] = useState<OrdemDeCompra[] | null>(null)
  const [erro, setErro] = useState('')
  const [vendo, setVendo] = useState<{ ordem: OrdemDeCompra; endereco: string; nome: string } | null>(null)
  const [corrigindo, setCorrigindo] = useState<OrdemDeCompra | null>(null)
  const [voltando, setVoltando] = useState<OrdemDeCompra | null>(null)
  const [processando, setProcessando] = useState<string | null>(null)
  const entrada = useRef<HTMLInputElement>(null)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ ordens: OrdemDeCompra[] }>('/administrativo/compras/ordens?vista=correcao')
      setOrdens(r.ordens)
      setErro('')
    } catch (e) {
      setOrdens([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function abrirArquivo(o: OrdemDeCompra) {
    try {
      const r = await motor<{ url: string }>(`/administrativo/compras/ordens/${o.id}/arquivo`)
      setVendo({ ordem: o, endereco: r.url, nome: o.nome_arquivo })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  function pedirCorrecao(o: OrdemDeCompra) {
    setCorrigindo(o)
    window.setTimeout(() => entrada.current?.click(), 0)
  }

  async function arquivoEscolhido(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    e.target.value = ''
    const o = corrigindo
    setCorrigindo(null)
    if (!f || !o) return
    setProcessando(o.id)
    try {
      const forma = new FormData()
      forma.append('arquivo', f, f.name)
      await enviarFormulario(`/administrativo/compras/ordens/${o.id}/corrigir`, forma)
      setOrdens(atual => (atual ? atual.filter(x => x.id !== o.id) : atual))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui corrigir esta OC.')
    } finally {
      setProcessando(null)
    }
  }

  async function confirmarVoltar() {
    if (!voltando) return
    const o = voltando
    setProcessando(o.id)
    try {
      await motor(`/administrativo/compras/ordens/${o.id}/voltar-aguardando`, { metodo: 'POST' })
      setOrdens(atual => (atual ? atual.filter(x => x.id !== o.id) : atual))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui voltar esta OC para Aguardando NF.')
    } finally {
      setProcessando(null)
    }
  }

  if (vendo) {
    return (
      <VisorDeDocumento
        titulo={`O.C. ${vendo.ordem.numero ?? '—'}`}
        endereco={vendo.endereco}
        nomeSugerido={vendo.nome}
        voltar={() => setVendo(null)}
      />
    )
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Correção de OC</h1>
          <p>Notas fiscais com valor diferente do total da OC — a compra e a nota precisam bater, ou a OC precisa ser corrigida.</p>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {ordens === null ? (
        <Carregando texto="Carregando..." />
      ) : ordens.length === 0 ? (
        <div className="vazio">Nenhuma OC aguardando correção.</div>
      ) : ehMobile ? (
        <div className="cl-lista">
          {ordens.map(o => (
            <CartaoLinha
              key={o.id}
              titulo={o.numero || '—'}
              onClick={() => void abrirArquivo(o)}
              linhas={[
                { rotulo: 'Obra/centro', valor: o.obra_centro_custo || '—' },
                { rotulo: 'Total da OC', valor: emReais(o.total ?? 0) },
                { rotulo: 'Recebido', valor: emReais(o.recebido ?? 0) },
                { rotulo: 'Origem', valor: rotuloOrigem(o.correcao_origem) },
                { rotulo: 'Desde', valor: o.correcao_marcada_em ? emDataHora(o.correcao_marcada_em) : '—' },
              ]}
              acoes={
                <>
                  <button
                    type="button" className="bt bt-mini bt-forte" disabled={processando === o.id}
                    onClick={() => pedirCorrecao(o)}
                  >
                    {processando === o.id ? '...' : 'corrigir'}
                  </button>
                  <button
                    type="button" className="bt bt-mini bt-neutro" disabled={processando === o.id}
                    onClick={() => setVoltando(o)}
                  >
                    voltar p/ aguardando
                  </button>
                </>
              }
            />
          ))}
        </div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>O.C.</th><th>Obra/centro</th><th>Total da OC</th><th>Recebido</th>
                <th>Origem</th><th>Desde</th><th></th>
              </tr>
            </thead>
            <tbody>
              {ordens.map(o => (
                <tr key={o.id}>
                  <td>
                    <button type="button" className="bt-como-link" onClick={() => void abrirArquivo(o)}>
                      {o.numero || '—'}
                    </button>
                  </td>
                  <td>{o.obra_centro_custo || '—'}</td>
                  <td>{emReais(o.total ?? 0)}</td>
                  <td>{emReais(o.recebido ?? 0)}</td>
                  <td>{rotuloOrigem(o.correcao_origem)}</td>
                  <td className="tri-fraco">{o.correcao_marcada_em ? emDataHora(o.correcao_marcada_em) : '—'}</td>
                  <td style={{ display: 'flex', gap: 8 }}>
                    <button
                      type="button" className="bt bt-mini bt-forte" disabled={processando === o.id}
                      onClick={() => pedirCorrecao(o)}
                    >
                      {processando === o.id ? '...' : 'corrigir'}
                    </button>
                    <button
                      type="button" className="bt bt-mini bt-neutro" disabled={processando === o.id}
                      onClick={() => setVoltando(o)}
                    >
                      voltar p/ aguardando
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <input
        ref={entrada} type="file" accept="application/pdf" style={{ display: 'none' }}
        onChange={e => void arquivoEscolhido(e)}
      />

      {voltando && (
        <Confirmar
          titulo="Voltar para Aguardando NF?"
          mensagem={`A OC ${voltando.numero ?? '—'} sai da fila de correção e volta a esperar nota fiscal normalmente.`}
          aoConfirmar={() => void confirmarVoltar()}
          aoFechar={() => setVoltando(null)}
        />
      )}
    </>
  )
}

function rotuloOrigem(origem: OrdemDeCompra['correcao_origem']): string {
  return origem === 'manual' ? 'manual (obra)' : 'automática (3%)'
}
