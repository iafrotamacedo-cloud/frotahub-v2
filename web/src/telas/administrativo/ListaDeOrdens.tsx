// rev 1 — a lista de uma vista de Ordens de Compra, compartilhada
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
import { useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { TabelaDeOrdens } from './TabelaDeOrdens'
import { type OrdemDeCompra, type VistaDasOrdens } from './tipos'

export function ListaDeOrdens({ vista, titulo, vazia, voltar }: {
  vista: VistaDasOrdens
  titulo: string
  /** A frase de "nada aqui" — cada vista tem a sua. */
  vazia: string
  voltar: () => void
}) {
  const [ordens, setOrdens] = useState<OrdemDeCompra[] | null>(null)
  const [erro, setErro] = useState('')
  const [vendo, setVendo] = useState<{ endereco: string; nome: string; titulo: string } | null>(null)

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

  async function abrirArquivo(o: OrdemDeCompra, reparar = false) {
    try {
      const r = await motor<{ url: string }>(`/administrativo/compras/ordens/${o.id}/arquivo`)
      const rotulo = o.numero || o.nome_arquivo.replace(/\.pdf$/i, '') || o.nome_arquivo
      setVendo({
        endereco: r.url,
        nome: o.nome_arquivo,
        titulo: reparar ? `Reparar · O.C. ${rotulo}` : o.nome_arquivo,
      })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  if (vendo) {
    return (
      <VisorDeDocumento
        endereco={vendo.endereco}
        nomeSugerido={vendo.nome}
        titulo={vendo.titulo}
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
