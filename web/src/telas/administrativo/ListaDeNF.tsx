// rev 1 — Notas Fiscais > Recebidas / Entregues no escritório / Enviadas
//
// TRÊS TELAS, UMA SÓ — MESMO MOTIVO DE `ListaDeOrdens.tsx`
//
//	As três etapas depois do recebimento são a MESMA pergunta ("mostre as
//	notas desta etapa, com ver, avançar e cancelar") feita três vezes. Só o
//	botão de avançar muda de rótulo e de rota — "Enviadas" nem tem um
//	próximo passo, então fica só com ver/cancelar.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { CancelarNF } from './CancelarNF'
import { emReais, emDataHora, type NotaFiscal } from './tipos'

type VistaDeNF = 'recebidas' | 'entregues' | 'enviadas'

const ROTA_DA_VISTA: Record<VistaDeNF, string> = {
  recebidas: '/administrativo/nf/recebidas',
  entregues: '/administrativo/nf/entregues',
  enviadas: '/administrativo/nf/enviadas',
}

const AVANCO_DA_VISTA: Record<VistaDeNF, { rota: (id: string) => string; rotulo: string } | null> = {
  recebidas: { rota: id => `/administrativo/nf/${id}/entregar`, rotulo: 'confirmar entrega no escritório' },
  entregues: { rota: id => `/administrativo/nf/${id}/enviar-cliente`, rotulo: 'marcar enviada (malote)' },
  enviadas: null,
}

const VAZIA_DA_VISTA: Record<VistaDeNF, string> = {
  recebidas: 'Nenhuma nota fiscal recebida aguardando entrega no escritório.',
  entregues: 'Nenhuma nota fiscal entregue no escritório aguardando envio.',
  enviadas: 'Nenhuma nota fiscal enviada ao cliente ainda.',
}

export function ListaDeNF({ vista, titulo }: { vista: VistaDeNF; titulo: string }) {
  const [notas, setNotas] = useState<NotaFiscal[] | null>(null)
  const [erro, setErro] = useState('')
  const [avancando, setAvancando] = useState<string | null>(null)
  const [cancelando, setCancelando] = useState<string | null>(null)
  const [vendo, setVendo] = useState<{ endereco: string; nome: string } | null>(null)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ notas: NotaFiscal[] }>(ROTA_DA_VISTA[vista])
      setNotas(r.notas)
      setErro('')
    } catch (e) {
      setNotas([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
  }, [vista])

  useEffect(() => { void carregar() }, [carregar])

  async function avancar(nf: NotaFiscal) {
    const avanco = AVANCO_DA_VISTA[vista]
    if (!avanco) return
    setAvancando(nf.id)
    try {
      await motor(avanco.rota(nf.id), { metodo: 'POST' })
      setNotas(atual => (atual ? atual.filter(x => x.id !== nf.id) : atual))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui avançar esta nota fiscal.')
    } finally {
      setAvancando(null)
    }
  }

  async function abrirArquivo(nf: NotaFiscal) {
    try {
      const r = await motor<{ url: string; nome: string }>(`/administrativo/nf/${nf.id}/arquivo`)
      setVendo({ endereco: r.url, nome: r.nome || nf.numero })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  if (vendo) {
    return <VisorDeDocumento endereco={vendo.endereco} nomeSugerido={vendo.nome} titulo={vendo.nome} voltar={() => setVendo(null)} />
  }

  const avanco = AVANCO_DA_VISTA[vista]

  return (
    <>
      <header className="hero hero-linha">
        <div><h1>{titulo}</h1></div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {notas === null ? (
        <Carregando texto="Carregando..." />
      ) : notas.length === 0 ? (
        <div className="vazio">{VAZIA_DA_VISTA[vista]}</div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>NF</th><th>O.C.</th><th>Obra/centro</th><th>Valor</th><th>Recebida em</th><th></th>
              </tr>
            </thead>
            <tbody>
              {notas.map(nf => (
                <tr key={nf.id}>
                  <td>{nf.numero}</td>
                  <td>{nf.ordem_numero || '—'}</td>
                  <td>{nf.obra_centro_custo || '—'}</td>
                  <td>{emReais(nf.valor)}</td>
                  <td className="tri-fraco">{emDataHora(nf.recebida_em)}</td>
                  <td>
                    <button type="button" className="bt bt-mini" onClick={() => void abrirArquivo(nf)}>ver</button>
                    {avanco && (
                      <button
                        type="button" className="bt bt-mini bt-forte" disabled={avancando === nf.id}
                        onClick={() => void avancar(nf)}
                      >
                        {avancando === nf.id ? '...' : avanco.rotulo}
                      </button>
                    )}
                    <button type="button" className="bt bt-mini bt-perigo" onClick={() => setCancelando(nf.id)}>
                      cancelar
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {cancelando && (
        <CancelarNF
          notaID={cancelando}
          aoFechar={() => setCancelando(null)}
          aoSalvar={() => {
            setNotas(atual => (atual ? atual.filter(x => x.id !== cancelando) : atual))
            setCancelando(null)
          }}
        />
      )}
    </>
  )
}
