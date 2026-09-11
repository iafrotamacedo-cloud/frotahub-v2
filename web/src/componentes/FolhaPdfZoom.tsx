// rev 1 — o PDF desenhado por nós (pdf.js), não pelo leitor nativo do navegador
//
// POR QUE NÃO <iframe src="doc.pdf">
//
//	O leitor de PDF embutido do Chrome não avisa a página quando alguém dá
//	zoom ou rola DENTRO dele — as caixas de clique do reparo híbrido
//	(regioesReparoOC.ts) ficavam com a posição calibrada, mas presas a um
//	momento só; qualquer zoom do leitor as deixava para trás. Testado ao
//	vivo em 11/09/2026: a folha nem preenchia a tela direito.
//
//	Aqui o PDF é desenhado num <canvas>, do tamanho exato de pixel que a
//	escala pede — zoom é redesenhar o canvas maior/menor, não um recurso do
//	leitor. A rolagem é a rolagem normal do navegador: canvas e caixas são
//	irmãos dentro do mesmo contêiner com scroll, então rolam juntos sem
//	nenhum código extra pra sincronizar.
import { useEffect, useRef, useState, type ReactNode, type WheelEvent } from 'react'
import * as pdfjsLib from 'pdfjs-dist'
import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorkerUrl

const ESCALA_MIN = 0.4
const ESCALA_MAX = 4
const PASSO_ZOOM = 0.2

interface Props {
  url: string
  /** Muda quando o PDF em si troca (nova prévia local) — força recarregar. */
  chave?: number | string
  /** As caixas de clique, desenhadas na MESMA escala que o canvas usou. */
  overlay?: (escala: number) => ReactNode
}

export function FolhaPdfZoom({ url, chave, overlay }: Props) {
  const rolagemRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const paginaRef = useRef<pdfjsLib.PDFPageProxy | null>(null)
  const tarefaRef = useRef<ReturnType<pdfjsLib.PDFPageProxy['render']> | null>(null)
  const [escala, setEscala] = useState<number | null>(null)
  const [tamanho, setTamanho] = useState({ largura: 0, altura: 0 })
  const [erro, setErro] = useState('')

  // Abre o documento e escolhe a escala inicial: cabe a altura disponível
  // do contêiner — é o "preencher a tela" que o leitor nativo fazia sozinho
  // (view=Fit), só que calculado por nós porque agora o zoom também é nosso.
  useEffect(() => {
    let vivo = true
    setEscala(null)
    setTamanho({ largura: 0, altura: 0 })
    setErro('')
    void (async () => {
      try {
        const doc = await pdfjsLib.getDocument(url).promise
        const pagina = await doc.getPage(1)
        if (!vivo) return
        paginaRef.current = pagina
        const base = pagina.getViewport({ scale: 1 })
        const alturaDisponivel = rolagemRef.current?.clientHeight ?? 800
        const larguraDisponivel = rolagemRef.current?.clientWidth ?? 600
        const folga = 32
        const escalaAltura = (alturaDisponivel - folga) / base.height
        const escalaLargura = (larguraDisponivel - folga) / base.width
        const inicial = Math.min(ESCALA_MAX, Math.max(ESCALA_MIN, Math.min(escalaAltura, escalaLargura)))
        setEscala(inicial)
      } catch {
        if (vivo) setErro('Não consegui abrir o PDF.')
      }
    })()
    return () => { vivo = false }
  }, [url, chave])

  // Redesenha o canvas sempre que a escala muda.
  //
  // CANCELA O DESENHO ANTERIOR ANTES DE COMEÇAR OUTRO
  //
  //	Dois cliques rápidos no zoom disparam dois `render()` em cima do MESMO
  //	canvas — sem cancelar o primeiro, ele continua escrevendo pixels
  //	enquanto o segundo já mudou o `canvas.width/height` para outro
  //	tamanho, e o resultado sai corrompido (visto ao vivo: a folha inteira
  //	saía de cabeça para baixo, 11/09/2026). `renderTask.cancel()` é o jeito
  //	documentado do pdf.js de desistir de um render em andamento.
  useEffect(() => {
    if (escala == null || !paginaRef.current) return
    let vivo = true
    void (async () => {
      const pagina = paginaRef.current!
      const viewport = pagina.getViewport({ scale: escala })
      const canvas = canvasRef.current
      if (!canvas) return
      tarefaRef.current?.cancel()
      canvas.width = viewport.width
      canvas.height = viewport.height
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      const tarefa = pagina.render({ canvasContext: ctx, viewport })
      tarefaRef.current = tarefa
      try {
        await tarefa.promise
        if (vivo) setTamanho({ largura: viewport.width, altura: viewport.height })
      } catch {
        // Cancelamento esperado (RenderingCancelledException) quando um
        // zoom mais novo chega antes deste terminar — não é erro de verdade.
      } finally {
        if (tarefaRef.current === tarefa) tarefaRef.current = null
      }
    })()
    return () => { vivo = false }
  }, [escala])

  function mudarEscala(delta: number) {
    setEscala(e => (e == null ? e : Math.min(ESCALA_MAX, Math.max(ESCALA_MIN, Number((e + delta).toFixed(2))))))
  }

  // Ctrl+roda dá zoom, como em qualquer leitor de PDF de verdade — a roda
  // sozinha continua rolando a página normalmente (não intercepto).
  function aoRolar(e: WheelEvent) {
    if (!e.ctrlKey) return
    e.preventDefault()
    mudarEscala(e.deltaY < 0 ? PASSO_ZOOM : -PASSO_ZOOM)
  }

  return (
    <div className="adm-pdf-zoom-area">
      <div className="adm-pdf-zoom-barra">
        <button type="button" onClick={() => mudarEscala(-PASSO_ZOOM)} disabled={escala == null} aria-label="Diminuir zoom">−</button>
        <span>{escala == null ? '…' : `${Math.round(escala * 100)}%`}</span>
        <button type="button" onClick={() => mudarEscala(PASSO_ZOOM)} disabled={escala == null} aria-label="Aumentar zoom">+</button>
      </div>
      <div className="adm-pdf-rolagem" ref={rolagemRef} onWheel={aoRolar}>
        {erro && <p className="erro">{erro}</p>}
        {escala != null && (
          <div className="adm-pdf-folha" style={{ width: tamanho.largura || undefined, height: tamanho.altura || undefined }}>
            <canvas ref={canvasRef} />
            {overlay?.(escala)}
          </div>
        )}
      </div>
    </div>
  )
}
