// rev 1 — a câmera, para qualquer tela que precise de um rosto
//
// Não decide o que fazer com o descritor capturado — isso é assunto de quem
// chama (P-13). Esta peça só sabe abrir a câmera, mostrar o que ela vê, e
// devolver o descritor de um rosto por vez.
import { useEffect, useRef, useState } from 'react'
import { carregarModelos, descritorDoVideo } from './facial'

interface Props {
  aoCapturar: (descritor: Float32Array) => void
  /** Texto do botão de captura — muda entre "Cadastrar" e "Confirmar", por exemplo. */
  rotuloBotao: string
  desabilitado?: boolean
}

type Estado = 'preparando' | 'pronta' | 'sem-permissao' | 'sem-camera'

export function CapturaFacial({ aoCapturar, rotuloBotao, desabilitado }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const [estado, setEstado] = useState<Estado>('preparando')
  const [procurando, setProcurando] = useState(false)
  const [aviso, setAviso] = useState<string | null>(null)

  useEffect(() => {
    let vivo = true

    async function abrir() {
      // Os modelos e a câmera não dependem um do outro — pedir os dois ao
      // mesmo tempo poupa a pessoa de esperar duas filas.
      const modelos = carregarModelos()

      if (!navigator.mediaDevices?.getUserMedia) {
        if (vivo) setEstado('sem-camera')
        return
      }

      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: 'user', width: { ideal: 480 }, height: { ideal: 480 } },
        })
        if (!vivo) { stream.getTracks().forEach(t => t.stop()); return }
        streamRef.current = stream
        if (videoRef.current) videoRef.current.srcObject = stream
        await modelos
        if (vivo) setEstado('pronta')
      } catch {
        // Cai aqui tanto por recusa da pessoa quanto por não existir câmera
        // nenhuma — as duas merecem a mesma saída: sem câmera, sem captura.
        if (vivo) setEstado('sem-permissao')
      }
    }
    abrir()

    return () => {
      vivo = false
      streamRef.current?.getTracks().forEach(t => t.stop())
    }
  }, [])

  async function capturar() {
    if (!videoRef.current || procurando) return
    setProcurando(true)
    setAviso(null)
    try {
      const descritor = await descritorDoVideo(videoRef.current)
      if (!descritor) {
        setAviso('Não achei um rosto só — chegue mais perto, com boa luz, e tente de novo.')
        return
      }
      aoCapturar(descritor)
    } finally {
      setProcurando(false)
    }
  }

  if (estado === 'sem-camera') {
    return <div className="erro-caixa">Este aparelho não tem câmera acessível pelo navegador.</div>
  }
  if (estado === 'sem-permissao') {
    return (
      <div className="erro-caixa">
        Não consegui abrir a câmera. Confira se o navegador tem permissão para usá-la e tente de novo.
      </div>
    )
  }

  return (
    <div className="captura-facial">
      <div className="captura-facial-video">
        <video ref={videoRef} autoPlay playsInline muted />
        {estado === 'preparando' && <div className="captura-facial-carregando">Abrindo a câmera…</div>}
      </div>
      <button
        className="bt bt-forte"
        type="button"
        onClick={capturar}
        disabled={estado !== 'pronta' || procurando || desabilitado}
      >
        {procurando ? 'Procurando o rosto…' : rotuloBotao}
      </button>
      {aviso && <div className="erro-caixa">{aviso}</div>}
    </div>
  )
}
