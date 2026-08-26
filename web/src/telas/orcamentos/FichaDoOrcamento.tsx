// rev 1 — o orçamento aberto na tela, com o caminho de volta ao papel
//
// A TELA É O DOCUMENTO
//
//	O que a pessoa quer ver aqui é o orçamento como ele sai — o mesmo PDF que
//	vai para o cliente. Remontar os itens em HTML daria uma segunda versão do
//	documento, que um dia discordaria da primeira num arredondamento ou numa
//	linha cortada pelo teto. Uma versão só, e é a que vale.
//
// O BOTÃO QUE MOTIVOU A TELA
//
//	"ver doc original" abre a nota ou DAV que virou este orçamento. O sistema
//	sempre soube qual era — o vínculo está gravado desde que o orçamento nasceu
//	— e até hoje não oferecia o caminho. Quem queria conferir decorava o número
//	e ia procurar na outra tela.
import { useEffect, useState } from 'react'
import { motor } from '../../motor/cliente'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { emReais } from './tipos'

interface NotaDoOrcamento {
  id: string
  nome_arquivo: string
  numero: string | null
  dav_numero: string | null
}

interface Ficha {
  orcamento: { ticket: number; parte: number; valor: number; loja: string | null }
  notas: NotaDoOrcamento[]
}

export function FichaDoOrcamento({ id, voltar }: { id: string; voltar: () => void }) {
  const [ficha, setFicha] = useState<Ficha | null>(null)
  const [erro, setErro] = useState('')

  useEffect(() => {
    let vivo = true
    void (async () => {
      try {
        const f = await motor<Ficha>(`/orcamentos/ficha/${id}`)
        if (vivo) setFicha(f)
      } catch (e) {
        if (vivo) setErro(e instanceof Error ? e.message : 'Não consegui abrir o orçamento.')
      }
    })()
    return () => { vivo = false }
  }, [id])

  async function verDocOriginal(nota: NotaDoOrcamento) {
    try {
      const r = await motor<{ url: string }>(`/orcamentos/documentos/${nota.id}/arquivo`)
      window.open(r.url, '_blank', 'noopener')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui abrir o documento original.')
    }
  }

  const o = ficha?.orcamento
  const notas = ficha?.notas ?? []
  const nome = o ? `orcamento-${o.ticket}${o.parte > 1 ? `-${o.parte}` : ''}.pdf` : undefined

  return (
    <>
      {erro && <p className="erro">{erro}</p>}
      <VisorDeDocumento
        caminho={`/orcamentos/ficha/${id}/pdf`}
        nomeSugerido={nome}
        voltar={voltar}
        titulo={
          o
            ? `Ticket ${o.ticket}${o.parte > 1 ? `-${o.parte}` : ''}`
              + (o.loja ? ` · ${o.loja}` : '') + ` · ${emReais(o.valor)}`
            : 'Orçamento'
        }
        acoes={
          /* SEM NOTA VINCULADA O BOTÃO NÃO APARECE
             Um botão que abre nada é pior que a ausência dele: a pessoa clica,
             não acontece nada, e passa a desconfiar da tela inteira. Acontece
             com orçamento cujo vínculo foi solto — ver a 025. */
          notas.map(n => (
            <button key={n.id} type="button" className="orc-bt"
              onClick={() => void verDocOriginal(n)} title={n.nome_arquivo}>
              ver doc original
              {notas.length > 1 && (n.dav_numero ?? n.numero ? ` (${n.dav_numero ?? n.numero})` : '')}
            </button>
          ))
        }
      />
    </>
  )
}
