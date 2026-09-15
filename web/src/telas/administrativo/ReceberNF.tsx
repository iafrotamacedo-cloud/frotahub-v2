// rev 3 — receber NF com ERA READ, pagina a pagina
//
// Cada pagina e escaneada e SALVA de uma vez. A primeira cria a nota (numero,
// valor, pagina 1); as seguintes vao em POST /notas/{id}/paginas. O ERA READ
// le cada pagina no servidor (Melhorador + OCR) quando configurado.
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { enviarFormulario, ErroMotor } from '../../motor/cliente'
import type { OrdemAguardandoNF } from './tipos'

interface Props {
  ordem: OrdemAguardandoNF
  aoFechar: () => void
  aoSalvar: () => void
}

interface RespostaPagina {
  id: string
  pagina: number
  numero?: string
  valor?: number
  era_read?: boolean
}

interface RespostaEscanear {
  numero?: string
  valor?: number
  era_read?: boolean
}

export function ReceberNF({ ordem, aoFechar, aoSalvar }: Props) {
  const [numero, setNumero] = useState('')
  const [valor, setValor] = useState(ordem.restante > 0 ? String(ordem.restante) : '')
  const [preview, setPreview] = useState<File | null>(null)
  const [materialFotos, setMaterialFotos] = useState<File[]>([])
  const [nfId, setNfId] = useState<string | null>(null)
  const [paginasSalvas, setPaginasSalvas] = useState<number[]>([])
  const [erro, setErro] = useState('')
  const [lendo, setLendo] = useState(false)
  const [salvando, setSalvando] = useState(false)

  const campoNota = useRef<HTMLInputElement>(null)
  const campoMaterial = useRef<HTMLInputElement>(null)

  const primeiraPagina = nfId === null
  const proximaPagina = paginasSalvas.length + 1

  async function aoCapturarNota(foto: File) {
    setPreview(foto)
    setErro('')
    if (!primeiraPagina) return
    setLendo(true)
    try {
      const forma = new FormData()
      forma.append('arquivo', foto, foto.name)
      const r = await enviarFormulario<RespostaEscanear>(
        `/administrativo/nf/ordens/${ordem.ordem_compra_id}/escanear`, forma)
      if (r.numero && !numero.trim()) setNumero(r.numero)
      if (r.valor && r.valor > 0 && !valor.trim()) setValor(formatarValor(r.valor))
    } catch {
      // ERA READ opcional — sem ele o almoxarife preenche manualmente
    } finally {
      setLendo(false)
    }
  }

  async function salvarPagina(e?: FormEvent) {
    e?.preventDefault()
    if (!preview) {
      setErro('Escaneie a página antes de salvar.')
      return
    }
    if (primeiraPagina && (!numero.trim() || !valor.trim())) {
      setErro('Preencha o número e o valor da nota na primeira página.')
      return
    }
    setErro('')
    setSalvando(true)
    try {
      const forma = new FormData()
      forma.append('arquivo', preview, preview.name)
      let r: RespostaPagina
      if (primeiraPagina) {
        forma.append('numero', numero.trim())
        forma.append('valor', valor.trim())
        for (const foto of materialFotos) forma.append('fotos_material', foto, foto.name)
        r = await enviarFormulario<RespostaPagina>(
          `/administrativo/nf/ordens/${ordem.ordem_compra_id}/receber`, forma)
        setNfId(r.id)
        if (r.numero) setNumero(r.numero)
        if (r.valor && r.valor > 0) setValor(formatarValor(r.valor))
      } else {
        r = await enviarFormulario<RespostaPagina>(
          `/administrativo/nf/notas/${nfId}/paginas`, forma)
      }
      setPaginasSalvas(ps => [...ps, r.pagina])
      setPreview(null)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar esta página.')
    } finally {
      setSalvando(false)
    }
  }

  function concluir() {
    if (paginasSalvas.length > 0) aoSalvar()
    else aoFechar()
  }

  return (
    <Janela
      titulo={`Receber NF · O.C. ${ordem.numero ?? '—'}`}
      descricao={`${ordem.obra_centro_custo ?? 'obra não identificada'} — falta ${formatarReais(ordem.restante)}`}
      aoFechar={concluir}
    >
      <form className="jn-corpo" onSubmit={salvarPagina}>
        {paginasSalvas.length > 0 && (
          <p className="nf-paginas-salvas">
            {paginasSalvas.length} página{paginasSalvas.length > 1 ? 's' : ''} salva{paginasSalvas.length > 1 ? 's' : ''}
            {' '}(páginas {paginasSalvas.join(', ')})
          </p>
        )}

        <label htmlFor="nf-numero">Número da nota fiscal</label>
        <input
          id="nf-numero" value={numero} onChange={e => setNumero(e.target.value)}
          readOnly={!primeiraPagina} autoFocus={primeiraPagina}
        />

        <label htmlFor="nf-valor">Valor</label>
        <input
          id="nf-valor" inputMode="decimal" value={valor}
          onChange={e => setValor(e.target.value.replace(/[^\d,.]/g, ''))}
          placeholder="0,00" readOnly={!primeiraPagina}
        />

        <label>Página {proximaPagina} da nota</label>
        <FotoUnica
          foto={preview}
          rotulo={lendo ? 'Lendo nota...' : `Escanear página ${proximaPagina}`}
          desabilitado={lendo || salvando}
          onAbrir={() => campoNota.current?.click()}
        />
        <input
          ref={campoNota} id="nf-arquivo" type="file" accept="image/*" capture="environment"
          style={{ display: 'none' }}
          onChange={e => {
            const foto = e.target.files?.[0]
            if (foto) void aoCapturarNota(foto)
            e.target.value = ''
          }}
        />

        {primeiraPagina && (
          <>
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
                e.target.value = ''
              }}
            />
          </>
        )}

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={concluir}>
            {paginasSalvas.length > 0 ? 'Concluir' : 'Cancelar'}
          </button>
          <button type="submit" className="bt bt-forte" disabled={salvando || lendo || !preview}>
            {salvando ? 'Salvando...' : `Salvar página ${proximaPagina}`}
          </button>
        </div>
      </form>
    </Janela>
  )
}

function FotoUnica({
  foto, rotulo, onAbrir, desabilitado,
}: { foto: File | null; rotulo: string; onAbrir: () => void; desabilitado?: boolean }) {
  const url = useUrlDoArquivo(foto)
  if (!foto || !url) {
    return (
      <button type="button" className="nf-foto-add nf-foto-add-larga" onClick={onAbrir} disabled={desabilitado}>
        <IconeCamera />
        <span>{rotulo}</span>
      </button>
    )
  }
  return (
    <div className="nf-foto-preview">
      <img src={url} alt="Página escaneada" />
      <button type="button" className="bt bt-mini bt-neutro" onClick={onAbrir} disabled={desabilitado}>
        Escanear de novo
      </button>
    </div>
  )
}

function FotoMini({ foto, onRemover }: { foto: File; onRemover: () => void }) {
  const url = useUrlDoArquivo(foto)
  return (
    <div className="nf-foto-mini">
      {url && <img src={url} alt="Foto do material" />}
      <button type="button" className="nf-foto-x" onClick={onRemover} aria-label="Remover esta foto">×</button>
    </div>
  )
}

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

function formatarValor(v: number): string {
  return v.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
