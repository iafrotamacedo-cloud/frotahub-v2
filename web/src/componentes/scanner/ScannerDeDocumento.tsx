// rev 1 — o scanner de documento pela câmera (15/09/2026)
//
// O QUE O ALMOXARIFE VÊ
//
//	A câmera em tela cheia, como no app de Notas da Apple: quando a folha
//	entra no quadro, um requadro amarelo se desenha por cima dela e
//	acompanha a mão. Parou de mexer por um segundo, a página é capturada
//	sozinha — recortada pelos cantos e endireitada. Aí ele confere,
//	"Próxima página" volta pra câmera, "Concluir" entrega todas as páginas
//	de uma vez. Também existe o botão redondo de captura manual, pra quando
//	a detecção não pega (folha da mesma cor da mesa, por exemplo): captura
//	o quadro inteiro.
//
// POR QUE AS PÁGINAS SAEM DAQUI JUNTAS
//
//	Era página a página que a OC ganhava dois recebimentos da mesma nota
//	(ver o cabeçalho de receberNF em notas_fiscais.go). Este componente
//	acumula as páginas e só as entrega em `aoConcluir` — quem chama manda
//	tudo num POST só.
//
// QUANDO NÃO HÁ CÂMERA
//
//	Sem `getUserMedia` (endereço sem HTTPS, permissão negada, navegador
//	velho), cai no `<input capture="environment">` de sempre: a câmera do
//	sistema tira a foto, sem requadro, e a página entra do mesmo jeito.
import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  detectarDocumento, distanciaQuad, escalarQuad, paraCinza, recortarPerspectiva,
  suavizarQuad, type Quad,
} from './deteccao'

interface Props {
  titulo: string
  /** Número da primeira página que este scanner vai produzir (1 numa nota nova). */
  paginaInicial: number
  aoConcluir: (paginas: File[]) => void
  aoCancelar: () => void
}

const LARGURA_ANALISE = 240
const INTERVALO_ANALISE_MS = 110
const ESTAVEL_MS = 1000
const LADO_MAXIMO_SAIDA = 2200

type Modo = 'iniciando' | 'camera' | 'revisao' | 'semCamera'

interface Captura { arquivo: File; url: string }

export function ScannerDeDocumento({ titulo, paginaInicial, aoConcluir, aoCancelar }: Props) {
  const [modo, setModo] = useState<Modo>('iniciando')
  const [motivoSemCamera, setMotivoSemCamera] = useState('')
  const [paginas, setPaginas] = useState<Captura[]>([])
  const [ultima, setUltima] = useState<Captura | null>(null)
  const [temLanterna, setTemLanterna] = useState(false)
  const [lanterna, setLanterna] = useState(false)
  const [dica, setDica] = useState('Enquadre a nota')
  const [progresso, setProgresso] = useState(0) // 0..1 até a captura automática
  const [capturando, setCapturando] = useState(false)

  const video = useRef<HTMLVideoElement>(null)
  const sobreposicao = useRef<HTMLCanvasElement>(null)
  const analise = useRef<HTMLCanvasElement | null>(null)
  const stream = useRef<MediaStream | null>(null)
  const quadro = useRef<number>(0)
  const ultimaAnalise = useRef(0)
  const quadAtual = useRef<Quad | null>(null)
  const perdidos = useRef(0)
  const estavelDesde = useRef(0)
  const ultimaCaptura = useRef(0)
  const capturandoRef = useRef(false)
  const modoRef = useRef<Modo>('iniciando')
  const entrada = useRef<HTMLInputElement>(null)

  useEffect(() => { modoRef.current = modo }, [modo])

  const pararCamera = useCallback(() => {
    cancelAnimationFrame(quadro.current)
    stream.current?.getTracks().forEach(t => t.stop())
    stream.current = null
  }, [])

  // ---- ligar a câmera -----------------------------------------------------
  const ligarCamera = useCallback(async () => {
    if (!navigator.mediaDevices?.getUserMedia) {
      setMotivoSemCamera('Este navegador não abre a câmera pela página.')
      setModo('semCamera')
      return
    }
    try {
      const s = await navigator.mediaDevices.getUserMedia({
        audio: false,
        video: {
          facingMode: { ideal: 'environment' },
          width: { ideal: 2560 },
          height: { ideal: 1920 },
        },
      })
      stream.current = s
      const v = video.current
      if (!v) { s.getTracks().forEach(t => t.stop()); return }
      v.srcObject = s
      await v.play()
      const faixa = s.getVideoTracks()[0]
      const cap = (faixa.getCapabilities?.() ?? {}) as MediaTrackCapabilities & { torch?: boolean }
      setTemLanterna(Boolean(cap.torch))
      setModo('camera')
    } catch (e) {
      const nome = (e as { name?: string })?.name
      setMotivoSemCamera(nome === 'NotAllowedError'
        ? 'A permissão da câmera foi negada. Libere a câmera para este site ou use a câmera do sistema.'
        : 'Não consegui abrir a câmera. Use a câmera do sistema.')
      setModo('semCamera')
    }
  }, [])

  useEffect(() => {
    void ligarCamera()
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      pararCamera()
      document.body.style.overflow = overflow
    }
  }, [ligarCamera, pararCamera])

  // ---- capturar ----------------------------------------------------------
  const capturar = useCallback(async (usarQuad: boolean) => {
    const v = video.current
    if (!v || !v.videoWidth || capturandoRef.current) return
    capturandoRef.current = true
    setCapturando(true)
    try {
      const cheio = document.createElement('canvas')
      cheio.width = v.videoWidth
      cheio.height = v.videoHeight
      const ctx = cheio.getContext('2d', { willReadFrequently: true })!
      ctx.drawImage(v, 0, 0)
      let resultado: HTMLCanvasElement = cheio
      const q = usarQuad ? quadAtual.current : null
      if (q && analise.current) {
        const dados = ctx.getImageData(0, 0, cheio.width, cheio.height)
        const qCheio = escalarQuad(q, analise.current.width, analise.current.height, cheio.width, cheio.height)
        resultado = recortarPerspectiva(dados, qCheio, LADO_MAXIMO_SAIDA)
      } else if (Math.max(cheio.width, cheio.height) > LADO_MAXIMO_SAIDA) {
        const f = LADO_MAXIMO_SAIDA / Math.max(cheio.width, cheio.height)
        const menor = document.createElement('canvas')
        menor.width = Math.round(cheio.width * f)
        menor.height = Math.round(cheio.height * f)
        menor.getContext('2d')!.drawImage(cheio, 0, 0, menor.width, menor.height)
        resultado = menor
      }
      const blob = await new Promise<Blob | null>(r => resultado.toBlob(r, 'image/jpeg', 0.9))
      if (!blob) throw new Error('sem imagem')
      const numero = paginaInicial + paginas.length
      const arquivo = new File([blob], `nf-p${numero}.jpg`, { type: 'image/jpeg' })
      setUltima({ arquivo, url: URL.createObjectURL(blob) })
      setModo('revisao')
      ultimaCaptura.current = performance.now()
      quadAtual.current = null
      estavelDesde.current = 0
      setProgresso(0)
    } finally {
      capturandoRef.current = false
      setCapturando(false)
    }
  }, [paginaInicial, paginas.length])

  // ---- o laço de análise --------------------------------------------------
  useEffect(() => {
    if (modo !== 'camera') return
    const v = video.current
    const sobre = sobreposicao.current
    if (!v || !sobre) return
    // O <video> é desmontado na revisão e volta novo na câmera: sem isto, a
    // segunda página abria uma tela preta (elemento sem stream).
    if (stream.current && v.srcObject !== stream.current) {
      v.srcObject = stream.current
      void v.play().catch(() => { /* o navegador pode pedir gesto; o próximo toque resolve */ })
    }
    quadAtual.current = null
    perdidos.current = 0
    estavelDesde.current = 0
    setProgresso(0)
    setDica('Enquadre a nota')

    if (!analise.current) analise.current = document.createElement('canvas')
    const pequeno = analise.current
    const ctxP = pequeno.getContext('2d', { willReadFrequently: true })!
    const ctxS = sobre.getContext('2d')!

    const passo = (agora: number) => {
      quadro.current = requestAnimationFrame(passo)
      if (modoRef.current !== 'camera' || !v.videoWidth) return
      if (agora - ultimaAnalise.current < INTERVALO_ANALISE_MS) return
      ultimaAnalise.current = agora

      const w = LARGURA_ANALISE
      const h = Math.round(v.videoHeight * (w / v.videoWidth))
      if (pequeno.width !== w || pequeno.height !== h) { pequeno.width = w; pequeno.height = h }
      ctxP.drawImage(v, 0, 0, w, h)
      const cinza = paraCinza(ctxP.getImageData(0, 0, w, h).data, w, h)
      const achado = detectarDocumento(cinza, w, h)

      if (achado) {
        perdidos.current = 0
        const anterior = quadAtual.current
        const novo = suavizarQuad(anterior, achado, 0.45)
        const mexeu = anterior ? distanciaQuad(anterior, novo) : Infinity
        quadAtual.current = novo
        const limiteMovimento = 0.012 * Math.min(w, h)
        if (mexeu > limiteMovimento) estavelDesde.current = agora
        else if (!estavelDesde.current) estavelDesde.current = agora
        const parado = agora - estavelDesde.current
        const p = Math.min(1, parado / ESTAVEL_MS)
        setProgresso(p)
        setDica(p < 1 ? 'Segure firme…' : 'Capturando')
        if (p >= 1 && agora - ultimaCaptura.current > 1500 && !capturandoRef.current) {
          void capturar(true)
        }
      } else {
        perdidos.current++
        if (perdidos.current >= 3) {
          quadAtual.current = null
          estavelDesde.current = 0
          setProgresso(0)
          setDica('Enquadre a nota')
        }
      }

      // Desenha o requadro na mesma geometria do vídeo (os dois com object-fit: cover).
      if (sobre.width !== v.videoWidth || sobre.height !== v.videoHeight) {
        sobre.width = v.videoWidth; sobre.height = v.videoHeight
      }
      ctxS.clearRect(0, 0, sobre.width, sobre.height)
      const q = quadAtual.current
      if (q) {
        const qs = escalarQuad(q, w, h, sobre.width, sobre.height)
        ctxS.beginPath()
        ctxS.moveTo(qs[0].x, qs[0].y)
        for (let i = 1; i < 4; i++) ctxS.lineTo(qs[i].x, qs[i].y)
        ctxS.closePath()
        ctxS.fillStyle = 'rgba(255, 204, 0, 0.22)'
        ctxS.fill()
        ctxS.lineWidth = Math.max(3, sobre.width / 300)
        ctxS.strokeStyle = '#ffcc00'
        ctxS.lineJoin = 'round'
        ctxS.stroke()
        ctxS.fillStyle = '#ffcc00'
        for (const c of qs) {
          ctxS.beginPath()
          ctxS.arc(c.x, c.y, Math.max(6, sobre.width / 140), 0, Math.PI * 2)
          ctxS.fill()
        }
      }
    }
    quadro.current = requestAnimationFrame(passo)
    return () => cancelAnimationFrame(quadro.current)
  }, [modo, capturar])

  // ---- lanterna -----------------------------------------------------------
  async function alternarLanterna() {
    const faixa = stream.current?.getVideoTracks()[0]
    if (!faixa) return
    try {
      await faixa.applyConstraints({ advanced: [{ torch: !lanterna } as MediaTrackConstraintSet] })
      setLanterna(l => !l)
    } catch { /* o aparelho disse que tinha, mas não deixou */ }
  }

  // ---- revisão -----------------------------------------------------------
  function refazer() {
    if (ultima) URL.revokeObjectURL(ultima.url)
    setUltima(null)
    setModo(stream.current ? 'camera' : 'semCamera')
  }

  function aceitarEContinuar() {
    if (!ultima) return
    setPaginas(ps => [...ps, ultima])
    setUltima(null)
    setModo(stream.current ? 'camera' : 'semCamera')
  }

  function concluir() {
    const todas = ultima ? [...paginas, ultima] : paginas
    pararCamera()
    if (todas.length === 0) { aoCancelar(); return }
    aoConcluir(todas.map(p => p.arquivo))
    for (const p of todas) URL.revokeObjectURL(p.url)
  }

  function fechar() {
    pararCamera()
    if (paginas.length > 0) {
      // Fechar no meio não joga fora o que já foi conferido.
      aoConcluir(paginas.map(p => p.arquivo))
    } else {
      aoCancelar()
    }
  }

  // ---- sem câmera: a câmera do sistema ------------------------------------
  function receberDoSistema(f: File | undefined) {
    if (!f) return
    const numero = paginaInicial + paginas.length
    const arquivo = new File([f], `nf-p${numero}.jpg`, { type: f.type || 'image/jpeg' })
    setUltima({ arquivo, url: URL.createObjectURL(f) })
    setModo('revisao')
  }

  const numeroAtual = paginaInicial + paginas.length

  return createPortal(
    <div className="sc" role="dialog" aria-modal="true" aria-label={titulo}>
      <header className="sc-topo">
        <button type="button" className="sc-icone" onClick={fechar} aria-label="Fechar o scanner">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="m6 6 12 12M18 6 6 18" /></svg>
        </button>
        <div className="sc-titulo">
          <strong>{titulo}</strong>
          <span>{modo === 'revisao' ? `Página ${numeroAtual} — confira` : `Página ${numeroAtual}`}</span>
        </div>
        {modo === 'camera' && temLanterna && (
          <button type="button" className={`sc-icone${lanterna ? ' sc-icone-ativo' : ''}`} onClick={() => void alternarLanterna()} aria-label="Lanterna">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
              <path d="M8 3h8v3l-2 3v10a2 2 0 0 1-4 0V9L8 6V3Z" /><path d="M12 12v3" />
            </svg>
          </button>
        )}
      </header>

      {(modo === 'iniciando' || modo === 'camera') && (
        <div className="sc-palco">
          <video ref={video} className="sc-video" playsInline muted autoPlay />
          <canvas ref={sobreposicao} className="sc-sobre" />
          {modo === 'iniciando' && <div className="sc-aviso">Abrindo a câmera…</div>}
          {modo === 'camera' && (
            <div className={`sc-dica${progresso > 0 ? ' sc-dica-ativa' : ''}`}>{dica}</div>
          )}
        </div>
      )}

      {modo === 'semCamera' && (
        <div className="sc-palco sc-palco-vazio">
          <p>{motivoSemCamera}</p>
          <button type="button" className="bt bt-forte" onClick={() => entrada.current?.click()}>
            Tirar foto da página {numeroAtual}
          </button>
          <input
            ref={entrada} type="file" accept="image/*" capture="environment" style={{ display: 'none' }}
            onChange={e => { receberDoSistema(e.target.files?.[0]); e.target.value = '' }}
          />
        </div>
      )}

      {modo === 'revisao' && ultima && (
        <div className="sc-palco sc-palco-revisao">
          <img src={ultima.url} alt={`Página ${numeroAtual} escaneada`} />
        </div>
      )}

      <footer className="sc-pe">
        {modo === 'camera' && (
          <>
            <div className="sc-contador" aria-live="polite">
              {paginas.length > 0 ? `${paginas.length} pág.` : ''}
            </div>
            <button
              type="button" className="sc-disparo" onClick={() => void capturar(true)}
              disabled={capturando} aria-label="Capturar página"
              style={{ ['--p' as string]: progresso }}
            >
              <span />
            </button>
            <button type="button" className="bt bt-neutro sc-concluir" onClick={concluir} disabled={paginas.length === 0}>
              Concluir{paginas.length > 0 ? ` (${paginas.length})` : ''}
            </button>
          </>
        )}
        {modo === 'revisao' && (
          <>
            <button type="button" className="bt bt-neutro" onClick={refazer}>Refazer</button>
            <button type="button" className="bt bt-neutro" onClick={aceitarEContinuar}>Próxima página</button>
            <button type="button" className="bt bt-forte" onClick={concluir}>
              Concluir ({paginas.length + 1})
            </button>
          </>
        )}
        {modo === 'semCamera' && paginas.length > 0 && (
          <button type="button" className="bt bt-forte" onClick={concluir}>Concluir ({paginas.length})</button>
        )}
      </footer>
    </div>,
    document.body,
  )
}
