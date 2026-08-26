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
import { arquivoDoMotor, baixarDoMotor, motor } from '../../motor/cliente'
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
  const [pdf, setPdf] = useState('')
  const [erro, setErro] = useState('')

  useEffect(() => {
    let vivo = true
    let endereco = ''
    void (async () => {
      try {
        const [f, url] = await Promise.all([
          motor<Ficha>(`/orcamentos/ficha/${id}`),
          arquivoDoMotor(`/orcamentos/ficha/${id}/pdf`),
        ])
        endereco = url
        if (!vivo) return
        setFicha(f)
        setPdf(url)
      } catch (e) {
        if (vivo) setErro(e instanceof Error ? e.message : 'Não consegui abrir o orçamento.')
      }
    })()
    // SEM ISTO CADA VISITA DEIXA UM PDF PRESO NA MEMÓRIA DA ABA
    //   Quem abre vinte orçamentos numa tarde carrega vinte arquivos até
    //   fechar o navegador.
    return () => {
      vivo = false
      if (endereco) URL.revokeObjectURL(endereco)
    }
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

  return (
    <div className="orc-tela">
      <div className="orc-barra">
        <button type="button" className="orc-voltar" onClick={voltar}>← voltar</button>
        <b>
          {o ? `Ticket ${o.ticket}${o.parte > 1 ? `-${o.parte}` : ''}` : 'Orçamento'}
          {o?.loja ? ` · ${o.loja}` : ''}
          {o ? ` · ${emReais(o.valor)}` : ''}
        </b>

        <span className="orc-ficha-direita">
          {/* SEM NOTA VINCULADA O BOTÃO NÃO APARECE
              Um botão que abre nada é pior que a ausência dele: a pessoa
              clica, não acontece nada, e passa a desconfiar da tela inteira.
              Acontece com orçamento cujo vínculo foi solto — ver a 025. */}
          {notas.map(n => (
            <button key={n.id} type="button" className="orc-bt"
              onClick={() => void verDocOriginal(n)} title={n.nome_arquivo}>
              ver doc original
              {notas.length > 1 && (n.dav_numero ?? n.numero ? ` (${n.dav_numero ?? n.numero})` : '')}
            </button>
          ))}
          <button type="button" className="orc-bt forte"
            onClick={() => void baixarDoMotor(`/orcamentos/ficha/${id}/pdf`)}>baixar pdf</button>
        </span>
      </div>

      {erro && <p className="erro">{erro}</p>}

      <div className="orc-visor-doc">
        {!pdf
          ? <p className="orc-vazio">abrindo o orçamento…</p>
          : <iframe src={pdf} title="orçamento" />}
      </div>
    </div>
  )
}
