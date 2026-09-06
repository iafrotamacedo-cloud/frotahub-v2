// rev 1 — o chat da Rogue Worker, no lugar do menu
//
// A logo e o rodapé da barra continuam fixos. Este componente só ocupa o
// miolo (.sd-nav). Chamadas vão pelo `motor`, nunca por fetch solto (CORE-09).
import { useEffect, useRef, useState } from 'react'
import { Icone } from '../../componentes/Icone'
import { motor, baixarDoMotor, ErroMotor } from '../../motor/cliente'

interface Pendente {
  tipo: string
  comando?: string
  ids?: string[]
  rotulo?: string
  excel?: string
  candidato_id?: string
  escopo?: string
}

interface DestinoNav {
  rotas: string[]
  tela: string
  rotina: string
}

interface OfertaExcel {
  caminho: string
  baixar?: boolean
}

interface Resposta {
  texto: string
  pendente?: Pendente | null
  navegar?: DestinoNav | null
  excel?: OfertaExcel | null
  opcoes?: string[] | null
  candidato_id?: string
}

type Bolha = {
  quem: 'eu' | 'rw'
  texto: string
  opcoes?: string[]
  excel?: string
}

const boasVindas: Bolha = {
  quem: 'rw',
  texto: 'Oi. Posso responder sobre o que este login alcança no FrotaHub — chamados, orçamentos, serviços, consolidação, funcionários. Para gerar ou lançar, eu sempre confirmo antes. Também te levo até uma tela do menu.',
}

export function ChatRogue({
  aoVoltar,
  aoNavegar,
}: {
  aoVoltar: () => void
  aoNavegar: (rotas: string[]) => void
}) {
  const [bolhas, setBolhas] = useState<Bolha[]>([boasVindas])
  const [texto, setTexto] = useState('')
  const [pendente, setPendente] = useState<Pendente | null>(null)
  const [enviando, setEnviando] = useState(false)
  const [erro, setErro] = useState('')
  const fim = useRef<HTMLDivElement>(null)
  const ultima = bolhas[bolhas.length - 1]

  useEffect(() => {
    fim.current?.scrollIntoView({ block: 'end' })
  }, [bolhas, enviando])

  async function enviar(frase: string) {
    const msg = frase.trim()
    if (!msg || enviando) return
    setTexto('')
    setErro('')
    setBolhas(b => [...b, { quem: 'eu', texto: msg }])
    setEnviando(true)
    try {
      const r = await motor<Resposta>('/rogueworker/conversar', {
        metodo: 'POST',
        corpo: { mensagem: msg, pendente },
      })
      setPendente(r.pendente ?? null)
      const bolha: Bolha = { quem: 'rw', texto: r.texto }
      if (r.opcoes?.length) bolha.opcoes = r.opcoes
      if (r.excel?.caminho && !r.excel.baixar) bolha.excel = r.excel.caminho
      setBolhas(b => [...b, bolha])
      if (r.excel?.baixar && r.excel.caminho) {
        try { await baixarDoMotor(r.excel.caminho) } catch (e) {
          setErro(e instanceof Error ? e.message : 'Não consegui baixar a planilha.')
        }
      }
      if (r.navegar?.rotas?.length) aoNavegar(r.navegar.rotas)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : e instanceof Error ? e.message : 'Não consegui falar com a Rogue Worker.')
    } finally {
      setEnviando(false)
    }
  }

  async function baixar(caminho: string) {
    try {
      await baixarDoMotor(caminho)
      setPendente(null)
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui baixar a planilha.')
    }
  }

  return (
    <div className="sd-nav rw-chat">
      <div className="rw-head">
        <button className="rw-voltar" type="button" onClick={aoVoltar}>
          ‹ Menu
        </button>
        <span className="rw-titulo">
          <Icone nome="balao" />
          Rogue Worker
        </span>
      </div>

      <div className="rw-corpo">
        {bolhas.map((b, i) => (
          <div key={i} className={'rw-msg ' + b.quem}>{b.texto}</div>
        ))}
        {ultima?.quem === 'rw' && !enviando && ultima.opcoes && ultima.opcoes.length > 0 && (
          <div className="rw-chips">
            {ultima.opcoes.map(op => (
              <button key={op} className="rw-chip" type="button" onClick={() => void enviar(op)}>
                {op}
              </button>
            ))}
          </div>
        )}
        {ultima?.quem === 'rw' && !enviando && ultima.excel && (
          <button className="rw-xlsx" type="button" onClick={() => void baixar(ultima.excel!)}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7"
                 strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
              <path d="M14 3v6h6" />
            </svg>
            Baixar em Excel
          </button>
        )}
        {enviando && <div className="rw-msg rw">…</div>}
        {erro && <div className="rw-erro">{erro}</div>}
        <div ref={fim} />
      </div>

      <form
        className="rw-input"
        onSubmit={e => { e.preventDefault(); void enviar(texto) }}
      >
        <input
          value={texto}
          onChange={e => setTexto(e.target.value)}
          placeholder="Pergunte algo…"
          disabled={enviando}
          aria-label="Mensagem para a Rogue Worker"
        />
        <button className="rw-enviar" type="submit" disabled={enviando || !texto.trim()} aria-label="Enviar">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7"
               strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="m5 12 14-7-4 7 4 7-14-7z" />
          </svg>
        </button>
      </form>
    </div>
  )
}
