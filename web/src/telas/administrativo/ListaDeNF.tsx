// rev 2 — Notas Fiscais > Recebidas / Entregues no escritório / Enviadas
//
// TRÊS TELAS, UMA SÓ — MESMO MOTIVO DE `ListaDeOrdens.tsx`
//
//	As três etapas depois do recebimento são a MESMA pergunta ("mostre as
//	notas desta etapa, com ver, avançar e cancelar") feita três vezes. Só o
//	botão de avançar muda de rótulo e de rota — "Enviadas" nem tem um
//	próximo passo, então fica só com ver/cancelar.
//
// `somenteLeitura` — O ALMOXARIFE VÊ, NÃO PROTOCOLA (15/09/2026)
//
//	Pedido do dono: o almoxarife acompanha Recebidas/Entregues (as notas que
//	ele mesmo escaneou, seguindo o caminho até o cliente), mas quem confirma
//	a entrega física no escritório e quem cancela é outra rotina
//	(COMPRAS_NF_ENTREGAR — ver `notas_fiscais.go`, `listarNF`, que já aceita
//	as duas rotinas pra leitura e continua travando `avancarNF`/`cancelarNF`
//	só pra quem entrega). Aqui, `somenteLeitura` tira os dois botões de
//	escrita e deixa só "ver" — nunca mostra um botão que o motor recusaria.
//
// "TROCAR" NÃO SEGUE `somenteLeitura` (migração 076, 17/09/2026)
//
//	RC ou RO, a qualquer momento — pedido do dono. No motor,
//	`trocarNF` aceita as mesmas três rotinas que já dão acesso de LEITURA a
//	estas listas (RECEBER/ENTREGAR/ENVIAR_CLIENTE) — quem enxerga a lista já
//	pode trocar, mesmo sem poder avançar/cancelar. Por isso o botão aparece
//	sempre, inclusive pro almoxarife (RO) em modo `somenteLeitura`.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { useEhMobile } from '../../componentes/useEhMobile'
import { CancelarNF } from './CancelarNF'
import { TrocarNF } from './TrocarNF'
import { emReais, emDataHora, type NotaFiscal } from './tipos'

type VistaDeNF = 'recebidas' | 'entregues' | 'enviadas'

const ROTA_DA_VISTA: Record<VistaDeNF, string> = {
  recebidas: '/administrativo/nf/recebidas',
  entregues: '/administrativo/nf/entregues',
  enviadas: '/administrativo/nf/enviadas',
}

const AVANCO_DA_VISTA: Record<VistaDeNF, { rota: (id: string) => string; rotulo: string } | null> = {
  recebidas: { rota: id => `/administrativo/nf/notas/${id}/entregar`, rotulo: 'confirmar entrega no escritório' },
  entregues: { rota: id => `/administrativo/nf/notas/${id}/enviar-cliente`, rotulo: 'marcar enviada (malote)' },
  enviadas: null,
}

const VAZIA_DA_VISTA: Record<VistaDeNF, string> = {
  recebidas: 'Nenhuma nota fiscal recebida aguardando entrega no escritório.',
  entregues: 'Nenhuma nota fiscal entregue no escritório aguardando envio.',
  enviadas: 'Nenhuma nota fiscal enviada ao cliente ainda.',
}

export function ListaDeNF({ vista, titulo, somenteLeitura }: { vista: VistaDeNF; titulo: string; somenteLeitura?: boolean }) {
  const ehMobile = useEhMobile()
  const [notas, setNotas] = useState<NotaFiscal[] | null>(null)
  const [erro, setErro] = useState('')
  const [avancando, setAvancando] = useState<string | null>(null)
  const [cancelando, setCancelando] = useState<string | null>(null)
  const [trocando, setTrocando] = useState<NotaFiscal | null>(null)
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
      const r = await motor<{ url: string; nome: string }>(`/administrativo/nf/notas/${nf.id}/arquivo`)
      setVendo({ endereco: r.url, nome: r.nome || nf.numero })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  if (vendo) {
    return <VisorDeDocumento endereco={vendo.endereco} nomeSugerido={vendo.nome} titulo={vendo.nome} voltar={() => setVendo(null)} />
  }

  const avanco = somenteLeitura ? null : AVANCO_DA_VISTA[vista]

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
      ) : ehMobile ? (
        <div className="cl-lista">
          {notas.map(nf => (
            <CartaoLinha
              key={nf.id}
              titulo={nf.numero}
              onClick={() => void abrirArquivo(nf)}
              linhas={[
                { rotulo: 'O.C.', valor: nf.ordem_numero || '—' },
                { rotulo: 'Obra/centro', valor: nf.obra_centro_custo || '—' },
                { rotulo: 'Valor', valor: emReais(nf.valor) },
                { rotulo: 'Recebida em', valor: emDataHora(nf.recebida_em) },
              ]}
              acoes={
                <>
                  {!somenteLeitura && avanco && (
                    <button
                      type="button" className="bt bt-mini bt-forte" disabled={avancando === nf.id}
                      onClick={() => void avancar(nf)}
                    >
                      {avancando === nf.id ? '...' : avanco.rotulo}
                    </button>
                  )}
                  <button type="button" className="bt bt-mini bt-neutro" onClick={() => setTrocando(nf)}>
                    trocar
                  </button>
                  {!somenteLeitura && (
                    <button type="button" className="bt bt-mini bt-perigo" onClick={() => setCancelando(nf.id)}>
                      cancelar
                    </button>
                  )}
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
                    {!somenteLeitura && avanco && (
                      <button
                        type="button" className="bt bt-mini bt-forte" disabled={avancando === nf.id}
                        onClick={() => void avancar(nf)}
                      >
                        {avancando === nf.id ? '...' : avanco.rotulo}
                      </button>
                    )}
                    <button type="button" className="bt bt-mini bt-neutro" onClick={() => setTrocando(nf)}>
                      trocar
                    </button>
                    {!somenteLeitura && (
                      <button type="button" className="bt bt-mini bt-perigo" onClick={() => setCancelando(nf.id)}>
                        cancelar
                      </button>
                    )}
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

      {trocando && (
        <TrocarNF
          nota={trocando}
          aoFechar={() => setTrocando(null)}
          aoSalvar={() => {
            // Recarrega em vez de só tirar da lista: em "recebidas" a nota
            // trocada CONTINUA aparecendo (só mudou o arquivo/valor); em
            // "entregues"/"enviadas" ela some, porque voltou pra `recebida`.
            setTrocando(null)
            void carregar()
          }}
        />
      )}
    </>
  )
}
