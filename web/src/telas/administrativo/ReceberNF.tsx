// rev 2 — a janela de "receber esta nota fiscal" (almoxarife, mobile-friendly)
//
// SÓ CÂMERA, NUNCA GALERIA — E DUAS FOTOS DIFERENTES
//
//	Pedido do dono (15/09/2026): separar "escanear a nota" de "fotografar o
//	material que chegou" — são duas evidências diferentes, e o material pode
//	precisar de várias fotos (a nota é sempre uma só). `capture="environment"`
//	abre a câmera direto no celular, sem passar pela galeria — e por enquanto
//	não existe outro jeito de anexar aqui: nada de escolher arquivo pronto.
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { enviarFormulario, ErroMotor } from '../../motor/cliente'
import type { OrdemAguardandoNF } from './tipos'

interface Props {
  ordem: OrdemAguardandoNF
  aoFechar: () => void
  aoSalvar: () => void
}

export function ReceberNF({ ordem, aoFechar, aoSalvar }: Props) {
  const [numero, setNumero] = useState('')
  const [valor, setValor] = useState(ordem.restante > 0 ? String(ordem.restante) : '')
  const [notaFoto, setNotaFoto] = useState<File | null>(null)
  const [materialFotos, setMaterialFotos] = useState<File[]>([])
  const [erro, setErro] = useState('')
  const [enviando, setEnviando] = useState(false)

  const campoNota = useRef<HTMLInputElement>(null)
  const campoMaterial = useRef<HTMLInputElement>(null)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    if (!numero.trim() || !valor.trim() || !notaFoto) {
      setErro('Preencha o número, o valor e escaneie a nota.')
      return
    }
    setErro('')
    setEnviando(true)
    try {
      const forma = new FormData()
      forma.append('numero', numero.trim())
      forma.append('valor', valor.trim())
      forma.append('arquivo', notaFoto, notaFoto.name)
      for (const foto of materialFotos) forma.append('fotos_material', foto, foto.name)
      await enviarFormulario(`/administrativo/nf/ordens/${ordem.ordem_compra_id}/receber`, forma)
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui registrar esta nota fiscal.')
      setEnviando(false)
    }
  }

  return (
    <Janela
      titulo={`Receber NF · O.C. ${ordem.numero ?? '—'}`}
      descricao={`${ordem.obra_centro_custo ?? 'obra não identificada'} — falta ${formatarReais(ordem.restante)}`}
      aoFechar={aoFechar}
    >
      <form className="jn-corpo" onSubmit={enviar}>
        <label htmlFor="nf-numero">Número da nota fiscal</label>
        <input id="nf-numero" value={numero} onChange={e => setNumero(e.target.value)} autoFocus />

        <label htmlFor="nf-valor">Valor</label>
        <input
          id="nf-valor" inputMode="decimal" value={valor}
          onChange={e => setValor(e.target.value.replace(/[^\d,.]/g, ''))}
          placeholder="0,00"
        />

        <label>Nota fiscal</label>
        <FotoUnica
          foto={notaFoto}
          rotulo="Escanear nota"
          onAbrir={() => campoNota.current?.click()}
        />
        <input
          ref={campoNota} id="nf-arquivo" type="file" accept="image/*" capture="environment"
          style={{ display: 'none' }}
          onChange={e => { setNotaFoto(e.target.files?.[0] ?? null); e.target.value = '' }}
        />

        <label style={{ marginTop: 14 }}>Fotos do material</label>
        <div className="nf-fotos-grade">
          {materialFotos.map((foto, i) => (
            <FotoMini key={i} foto={foto} onRemover={() => setMaterialFotos(fs => fs.filter((_, j) => j !== i))} />
          ))}
          <button type="button" className="nf-foto-add" onClick={() => campoMaterial.current?.click()}>
            <IconeCamera />
            <span>Tirar foto</span>
          </button>
        </div>
        <input
          ref={campoMaterial} type="file" accept="image/*" capture="environment"
          style={{ display: 'none' }}
          onChange={e => {
            const foto = e.target.files?.[0]
            if (foto) setMaterialFotos(fs => [...fs, foto])
            e.target.value = '' // permite tirar a próxima foto sem fechar e reabrir
          }}
        />

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={enviando}>
            {enviando ? 'Enviando...' : 'Receber'}
          </button>
        </div>
      </form>
    </Janela>
  )
}

/** A prévia da nota escaneada — ou o botão pra escanear, quando ainda não tem. */
function FotoUnica({ foto, rotulo, onAbrir }: { foto: File | null; rotulo: string; onAbrir: () => void }) {
  const url = useUrlDoArquivo(foto)
  if (!foto || !url) {
    return (
      <button type="button" className="nf-foto-add nf-foto-add-larga" onClick={onAbrir}>
        <IconeCamera />
        <span>{rotulo}</span>
      </button>
    )
  }
  return (
    <div className="nf-foto-preview">
      <img src={url} alt="Nota fiscal escaneada" />
      <button type="button" className="bt bt-mini bt-neutro" onClick={onAbrir}>Tirar outra</button>
    </div>
  )
}

/** Uma miniatura da grade de fotos do material, com o × pra remover. */
function FotoMini({ foto, onRemover }: { foto: File; onRemover: () => void }) {
  const url = useUrlDoArquivo(foto)
  return (
    <div className="nf-foto-mini">
      {url && <img src={url} alt="Foto do material" />}
      <button type="button" className="nf-foto-x" onClick={onRemover} aria-label="Remover esta foto">×</button>
    </div>
  )
}

/** O `URL.createObjectURL` de um arquivo, desfeito sozinho quando ele muda —
 *  sem isto cada foto tirada na obra fica presa na memória da aba até fechar. */
function useUrlDoArquivo(arquivo: File | null): string | null {
  const [url, setUrl] = useState<string | null>(null)
  useEffect(() => {
    if (!arquivo) { setUrl(null); return }
    const u = URL.createObjectURL(arquivo)
    setUrl(u)
    return () => URL.revokeObjectURL(u)
  }, [arquivo])
  return url
}

// O mesmo traço fino dos ícones do menu (`componentes/Icone.tsx`) — sem
// biblioteca externa, só que este é de uso só desta tela.
function IconeCamera() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 8h3l2-2.5h6L17 8h3a1 1 0 0 1 1 1v9a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1Z" />
      <circle cx="12" cy="13.5" r="3.4" />
    </svg>
  )
}

function formatarReais(v: number): string {
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}
