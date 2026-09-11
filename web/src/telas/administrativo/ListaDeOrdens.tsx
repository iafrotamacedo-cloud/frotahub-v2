// rev 2 — a lista de uma vista de Ordens de Compra, compartilhada
//
// EXTRAÍDA DE `OcsInseridas.tsx` QUANDO PCO PRECISOU DA MESMA TELA
//
//	Processadas, Rejeitadas, Pendentes de envio e Enviados são a MESMA
//	pergunta — "mostre as OCs desta vista, com ver e voltar" — feita quatro
//	vezes. Escrever de novo a cada hub novo é como duas cópias da tabela
//	iam divergir (ver `TabelaDeOrdens.tsx`, extraída pelo mesmo motivo).
//
// SÓ CONSULTA, SEM AÇÃO
//   A leitura e a releitura acontecem em "Inserir OC", que é onde a fila
//   vive. Estas listas são as vistas de controle — Compras (Processadas/
//   Rejeitadas) e PCO (Pendentes/Enviados) — não o lugar de trabalhar a OC.
//
// EDITAR DOCUMENTO (11/09/2026)
//   Só rejeitadas: a processada já passou nos filtros, não se mexe. EDITAR
//   abre a folha da OC; salvar gera um PDF novo e substitui o arquivo. Se
//   passar nos filtros, "Salvar e voltar" devolve à lista.
import { useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { TabelaDeOrdens } from './TabelaDeOrdens'
import { EditorDeOC } from './EditorDeOC'
import {
  type OrdemDeCompra, type EstadoDocumentoOC, type VistaDasOrdens,
  motivoRejeicaoSimplificado,
} from './tipos'

export function ListaDeOrdens({ vista, titulo, vazia, voltar }: {
  vista: VistaDasOrdens
  titulo: string
  /** A frase de "nada aqui" — cada vista tem a sua. */
  vazia: string
  voltar: () => void
}) {
  const [ordens, setOrdens] = useState<OrdemDeCompra[] | null>(null)
  const [erro, setErro] = useState('')
  const [vendo, setVendo] = useState<{
    ordemId: string
    endereco: string
    nome: string
    titulo: string
    motivos: string[]
    motivoCompleto?: string | null
    reparar: boolean
    corrigida: boolean
  } | null>(null)
  const [editando, setEditando] = useState(false)

  useEffect(() => {
    void (async () => {
      try {
        const r = await motor<{ ordens: OrdemDeCompra[] }>('/administrativo/compras/ordens?vista=' + vista)
        setOrdens(r.ordens)
      } catch (e) {
        setOrdens([])
        setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
      }
    })()
  }, [vista])

  function motivosDaOrdem(o: OrdemDeCompra): string[] {
    if (!o.erro_leitura?.trim()) return []
    return o.erro_leitura
      .split(';')
      .map(p => motivoRejeicaoSimplificado(p.trim()))
      .filter(m => m && m !== '—')
  }

  async function abrirArquivo(o: OrdemDeCompra, reparar = false) {
    try {
      const r = await motor<{ url: string }>(`/administrativo/compras/ordens/${o.id}/arquivo`)
      const rotulo = o.numero || o.nome_arquivo.replace(/\.pdf$/i, '') || o.nome_arquivo
      let motivos = motivosDaOrdem(o)
      if (reparar) {
        try {
          const est = await motor<{ motivos: string[] }>(`/administrativo/compras/ordens/${o.id}/reparo`)
          if (est.motivos?.length) motivos = est.motivos
        } catch { /* usa o simplificado local */ }
      }
      setVendo({
        ordemId: o.id,
        endereco: r.url,
        nome: o.nome_arquivo,
        titulo: reparar ? `Reparar · O.C. ${rotulo}` : o.nome_arquivo,
        motivos,
        motivoCompleto: reparar ? o.erro_leitura : undefined,
        reparar,
        corrigida: false,
      })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  function aoDocumentoSalvo(r: EstadoDocumentoOC) {
    if (!vendo) return
    const ordemId = vendo.ordemId
    const lido = r.status === 'lido'
    const motivos = lido ? [] : (r.motivos?.length ? r.motivos : [])
    setEditando(false)
    void (async () => {
      try {
        const arq = await motor<{ url: string }>(`/administrativo/compras/ordens/${ordemId}/arquivo`)
        setVendo(v => v && v.ordemId === ordemId ? {
          ...v,
          endereco: arq.url,
          motivos,
          motivoCompleto: r.motivo ?? v.motivoCompleto,
          corrigida: lido,
        } : v)
        const lista = await motor<{ ordens: OrdemDeCompra[] }>('/administrativo/compras/ordens?vista=' + vista)
        setOrdens(lista.ordens)
      } catch {
        setVendo(v => v && v.ordemId === ordemId ? { ...v, motivos, corrigida: lido } : v)
      }
    })()
  }

  const podeEditar = vista === 'rejeitadas'

  if (editando && vendo) {
    return (
      <EditorDeOC
        ordemId={vendo.ordemId}
        voltar={() => setEditando(false)}
        aoSalvar={aoDocumentoSalvo}
      />
    )
  }

  if (vendo) {
    const barraAlta = vendo.motivos.length > 1
    return (
      <VisorDeDocumento
        endereco={vendo.endereco}
        nomeSugerido={vendo.nome}
        titulo={vendo.titulo}
        barraAlta={barraAlta}
        barAcoes={podeEditar && !vendo.corrigida ? (
          <button type="button" className="orc-bt adm-editar" onClick={() => setEditando(true)}>
            EDITAR
          </button>
        ) : undefined}
        destaque={vendo.motivos.length > 0 ? (
          vendo.motivos.map((m, i) => (
            <span key={i} className="adm-motivo-barra" title={vendo.motivoCompleto ?? undefined}>{m}</span>
          ))
        ) : undefined}
        acoes={vendo.reparar && vendo.corrigida ? (
          <button type="button" className="bt bt-forte adm-salvar-voltar" onClick={voltar}>
            Salvar e voltar
          </button>
        ) : undefined}
        voltar={() => setVendo(null)}
      />
    )
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <button type="button" className="bt bt-neutro" onClick={voltar}>← voltar</button>
          <h1>{titulo}</h1>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {ordens === null ? (
        <Carregando texto="Carregando..." />
      ) : ordens.length === 0 ? (
        <div className="vazio">{vazia}</div>
      ) : (
        <TabelaDeOrdens
          ordens={ordens}
          vista={vista}
          onVer={o => void abrirArquivo(o)}
          onReparar={vista === 'rejeitadas' ? o => void abrirArquivo(o, true) : undefined}
        />
      )}
    </>
  )
}
