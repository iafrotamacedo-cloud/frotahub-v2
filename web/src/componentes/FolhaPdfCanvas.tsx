// rev 2 — o PDF desenhado por nós (pdf.js), não pelo leitor nativo do navegador
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
//	escala pede. A rolagem é a rolagem normal do navegador: canvas e caixas
//	são irmãos dentro do mesmo contêiner com scroll, então rolam juntos sem
//	nenhum código extra pra sincronizar.
//
// SEM ZOOM INTERATIVO — SÓ A LARGURA DISPONÍVEL, E ROLA
//
//	Pedido do dono depois do primeiro teste ao vivo (11/09/2026): nada de
//	botões +/− ou Ctrl+roda — a folha cabe na LARGURA da tela sozinha, e o
//	resto é rolagem vertical normal, como abrir o PDF teria que ser desde
//	sempre. A escala é medida uma vez (e de novo se a janela mudar de
//	tamanho — ResizeObserver), nunca por interação da pessoa.
import { useEffect, useRef, useState, type ReactNode } from 'react'
import * as pdfjsLib from 'pdfjs-dist'
import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorkerUrl

interface Props {
  url: string
  /** Muda quando o PDF em si troca (nova prévia local) — força recarregar. */
  chave?: number | string
  /** As caixas de clique, desenhadas na MESMA escala que o canvas usou. */
  overlay?: (escala: number) => ReactNode
}

export function FolhaPdfCanvas({ url, chave, overlay }: Props) {
  const rolagemRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const paginaRef = useRef<pdfjsLib.PDFPageProxy | null>(null)
  const tarefaRef = useRef<ReturnType<pdfjsLib.PDFPageProxy['render']> | null>(null)
  const [escala, setEscala] = useState<number | null>(null)
  const [tamanho, setTamanho] = useState({ largura: 0, altura: 0 })
  const [erro, setErro] = useState('')

  // Abre o documento e mede a largura disponível do contêiner — a folha
  // cabe nela, sozinha, sem zoom nenhum de quem está vendo.
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
        const larguraDisponivel = rolagemRef.current?.clientWidth ?? 600
        const folga = 32
        setEscala(Math.max(0.1, (larguraDisponivel - folga) / base.width))
      } catch {
        if (vivo) setErro('Não consegui abrir o PDF.')
      }
    })()
    return () => { vivo = false }
  }, [url, chave])

  // A largura disponível muda (janela redimensionada, barra lateral
  // recolhida) — a folha reajusta sozinha, sem esperar clique em nada.
  // `paginaRef` é lido de dentro do callback (não como dependência): o
  // observador é montado uma vez só e continua valendo depois que o
  // documento carrega.
  useEffect(() => {
    const alvo = rolagemRef.current
    if (!alvo) return
    const observador = new ResizeObserver(() => {
      const pagina = paginaRef.current
      if (!pagina) return
      const base = pagina.getViewport({ scale: 1 })
      const folga = 32
      setEscala(Math.max(0.1, (alvo.clientWidth - folga) / base.width))
    })
    observador.observe(alvo)
    return () => observador.disconnect()
  }, [])

  // Redesenha o canvas sempre que a escala muda.
  //
  // CANCELA O DESENHO ANTERIOR ANTES DE COMEÇAR OUTRO
  //
  //	Dois redesenhos em sequência (ex.: dois ajustes de largura rápidos)
  //	disparam dois `render()` em cima do MESMO canvas — sem cancelar o
  //	primeiro, ele continua escrevendo pixels enquanto o segundo já mudou o
  //	`canvas.width/height` para outro tamanho, e o resultado sai corrompido
  //	(visto ao vivo: a folha inteira saía de cabeça para baixo, 11/09/2026).
  //	`renderTask.cancel()` é o jeito documentado do pdf.js de desistir de
  //	um render em andamento.
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
        // redesenho mais novo chega antes deste terminar — não é erro de verdade.
      } finally {
        if (tarefaRef.current === tarefa) tarefaRef.current = null
      }
    })()
    return () => { vivo = false }
  }, [escala])

  return (
    <div className="adm-pdf-rolagem" ref={rolagemRef}>
      {erro && <p className="erro">{erro}</p>}
      {escala != null && (
        <div className="adm-pdf-folha" style={{ width: tamanho.largura || undefined, height: tamanho.altura || undefined }}>
          <canvas ref={canvasRef} />
          {overlay?.(escala)}
        </div>
      )}
    </div>
  )
}
