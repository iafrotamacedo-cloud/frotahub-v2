// rev 6 — receber NF: escaneia TODAS as páginas, salva uma vez (15/09/2026)
//
// O FLUXO NO CELULAR
//
//	1. A janela abre no formulário (não mais direto na câmera — 17/09/2026,
//	   a pedido do dono: abrir já na câmera pegava o almoxarife de surpresa
//	   antes de ele ver de qual O.C. é o recebimento). O botão "Escanear a
//	   nota" é que chama o scanner (ScannerDeDocumento) em tela cheia:
//	   requadro automático, uma página atrás da outra, "Concluir".
//	2. As páginas aparecem como miniaturas. Número e valor são digitados —
//	   a sugestão pela IA (/escanear) existe, mas está DESLIGADA por decisão
//	   do dono (15/09/2026); ver SUGERIR_PELA_IA.
//	3. Fotos do material, pela câmera do sistema — pelo menos UMA, senão
//	   não salva (o motor recusa também).
//	4. "Salvar nota" manda tudo num POST só: página 1 em `arquivo`, as
//	   seguintes em `paginas`, fotos em `fotos_material`.
//
// POR QUE NÃO É MAIS "SALVAR PÁGINA 1", "SALVAR PÁGINA 2"
//
//	A rev 3 salvava página a página, e a nota nascia na primeira. Fechar a
//	janela e abrir de novo pra página 2 criava uma SEGUNDA nota na mesma
//	OC. Agora a nota só existe quando todas as páginas estão na mão.
//
// "ENVIAR PDF" — SEGUNDO CAMINHO PRA MESMA CAIXA (17/09/2026)
//
//	Nem toda nota chega na obra pra ser fotografada — às vezes já veio em
//	PDF do fornecedor, por e-mail. O botão só aparece pra quem tem
//	COMPRAS_NF_RECEBER_PDF (migração 075 — rotina À PARTE de RECEBER, o
//	dono concede por categoria); o motor confere de novo no servidor
//	(P-29), então esconder o botão aqui é só não oferecer o que a pessoa
//	não pode usar, nunca a única trava. O PDF entra na mesma lista de
//	`paginas` que as fotos escaneadas — pro motor é só mais um arquivo
//	(`guardarArquivoNF` já sabe guardar qualquer tipo).
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { ScannerDeDocumento } from '../../componentes/scanner/ScannerDeDocumento'
import { enviarFormulario, ErroMotor } from '../../motor/cliente'
import type { Perfil } from '../../sessao/tipos'
import { RotinaNFReceberPDF, temRotina } from './rotinasNF'
import type { OrdemAguardandoNF } from './tipos'

/**
 * Liga a sugestão de número/valor pela IA (rota /escanear do motor).
 * Desligada em 15/09/2026 a pedido do dono — "por enquanto, manual". Para
 * religar basta trocar para true: a rota continua no motor.
 */
const SUGERIR_PELA_IA = false

interface Props {
  ordem: OrdemAguardandoNF
  perfil?: Perfil | null
  aoFechar: () => void
  aoSalvar: () => void
}

interface RespostaReceber {
  id: string
  paginas: number
  fotos_material: number
  era_read?: boolean
}

interface RespostaEscanear {
  fonte: 'ia' | 'nenhuma'
  numero?: string
  valor?: number
  tipo?: string
  emitente?: string
  aviso?: string
}

export function ReceberNF({ ordem, perfil = null, aoFechar, aoSalvar }: Props) {
  const [numero, setNumero] = useState('')
  const [valor, setValor] = useState('')
  const [paginas, setPaginas] = useState<File[]>([])
  const [materialFotos, setMaterialFotos] = useState<File[]>([])
  const [scannerAberto, setScannerAberto] = useState(false)
  const [erro, setErro] = useState('')
  const [aviso, setAviso] = useState('')
  const [lendo, setLendo] = useState(false)
  const [salvando, setSalvando] = useState(false)
  const podeReceberPDF = temRotina(perfil, RotinaNFReceberPDF)

  const campoMaterial = useRef<HTMLInputElement>(null)
  const campoPDF = useRef<HTMLInputElement>(null)
  const jaSugeriu = useRef(false)

  // A sugestão de número/valor sai da primeira página, uma vez só.
  useEffect(() => {
    const primeira = paginas[0]
    if (!primeira || jaSugeriu.current) return
    jaSugeriu.current = true
    if (!SUGERIR_PELA_IA) {
      // Manual, por decisão do dono (15/09/2026): o valor que falta na OC
      // entra como palpite, o número o almoxarife digita.
      setValor(v => v.trim() || ordem.restante <= 0 ? v : formatarValor(ordem.restante))
      return
    }
    let cancelado = false
    setLendo(true)
    const forma = new FormData()
    forma.append('arquivo', primeira, primeira.name)
    enviarFormulario<RespostaEscanear>(`/administrativo/nf/ordens/${ordem.ordem_compra_id}/escanear`, forma)
      .then(r => {
        if (cancelado) return
        if (r.numero) setNumero(n => n.trim() ? n : r.numero!)
        if (r.valor && r.valor > 0) setValor(v => v.trim() ? v : formatarValor(r.valor!))
        else setValor(v => v.trim() || ordem.restante <= 0 ? v : formatarValor(ordem.restante))
        if (r.fonte === 'nenhuma') {
          setAviso(r.aviso ?? 'Leitura automática indisponível — confira número e valor.')
        }
      })
      .catch(() => {
        if (cancelado) return
        // Sem leitura, o valor que falta na OC é o palpite mais útil.
        setValor(v => v.trim() || ordem.restante <= 0 ? v : formatarValor(ordem.restante))
        setAviso('Não consegui ler a nota automaticamente — confira número e valor.')
      })
      .finally(() => { if (!cancelado) setLendo(false) })
    return () => { cancelado = true }
  }, [paginas, ordem.ordem_compra_id, ordem.restante])

  function receberPaginas(novas: File[]) {
    setScannerAberto(false)
    if (novas.length === 0) return
    setPaginas(ps => [...ps, ...novas])
    setErro('')
  }

  function receberPDF(f: File | undefined) {
    if (!f) return
    const arquivo = new File([f], `nf-p${paginas.length + 1}.pdf`, { type: 'application/pdf' })
    setPaginas(ps => [...ps, arquivo])
    setErro('')
  }

  function removerPagina(i: number) {
    // A extensão de cada página SEGUE COM ELA — trocar tudo por `.jpg` na
    // renumeração derrubaria o PDF de "receber por PDF" pra imagem, e o
    // motor decide o tipo do arquivo pela extensão (`tipoDoNome`).
    setPaginas(ps => ps.filter((_, j) => j !== i).map((f, j) => {
      const ponto = f.name.lastIndexOf('.')
      const ext = ponto >= 0 ? f.name.slice(ponto) : '.jpg'
      return new File([f], `nf-p${j + 1}${ext}`, { type: f.type })
    }))
  }

  async function salvar(e?: FormEvent) {
    e?.preventDefault()
    if (paginas.length === 0) {
      setErro('Escaneie a nota antes de salvar.')
      return
    }
    if (!numero.trim() || !valor.trim()) {
      setErro('Preencha o número e o valor da nota.')
      return
    }
    if (materialFotos.length === 0) {
      setErro('Tire pelo menos uma foto do material recebido.')
      return
    }
    setErro('')
    setSalvando(true)
    try {
      const forma = new FormData()
      forma.append('numero', numero.trim())
      forma.append('valor', valor.trim())
      forma.append('arquivo', paginas[0], paginas[0].name)
      for (const pg of paginas.slice(1)) forma.append('paginas', pg, pg.name)
      for (const foto of materialFotos) forma.append('fotos_material', foto, foto.name)
      await enviarFormulario<RespostaReceber>(`/administrativo/nf/ordens/${ordem.ordem_compra_id}/receber`, forma)
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar a nota.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <>
      <Janela
        titulo={`Receber NF · O.C. ${ordem.numero ?? '—'}`}
        descricao={`${ordem.obra_centro_custo ?? 'obra não identificada'} — falta ${formatarReais(ordem.restante)}`}
        aoFechar={aoFechar}
      >
        <form className="jn-corpo" onSubmit={salvar}>
          <label>Páginas da nota</label>
          <div className="nf-fotos-grade">
            {paginas.map((pg, i) => (
              <PaginaMini key={`${pg.name}-${pg.size}-${i}`} arquivo={pg} numero={i + 1} onRemover={() => removerPagina(i)} />
            ))}
            <button
              type="button"
              className={`nf-foto-add${paginas.length === 0 ? ' nf-foto-add-larga' : ''}`}
              onClick={() => setScannerAberto(true)}
              disabled={salvando}
            >
              <IconeScanner />
              <span>{paginas.length === 0 ? 'Escanear a nota' : 'Mais uma página'}</span>
            </button>
            {podeReceberPDF && (
              <button
                type="button"
                className="nf-foto-add"
                onClick={() => campoPDF.current?.click()}
                disabled={salvando}
              >
                <IconePDF />
                <span>Enviar PDF</span>
              </button>
            )}
          </div>
          {podeReceberPDF && (
            <input
              ref={campoPDF} type="file" accept="application/pdf"
              style={{ display: 'none' }}
              onChange={e => { receberPDF(e.target.files?.[0]); e.target.value = '' }}
            />
          )}

          <label htmlFor="nf-numero">Número da nota fiscal</label>
          <input
            id="nf-numero" value={numero} onChange={e => setNumero(e.target.value)}
            placeholder={lendo ? 'Lendo a nota…' : ''} inputMode="numeric"
          />

          <label htmlFor="nf-valor">Valor</label>
          <input
            id="nf-valor" inputMode="decimal" value={valor}
            onChange={e => setValor(e.target.value.replace(/[^\d,.]/g, ''))}
            placeholder={lendo ? 'Lendo a nota…' : '0,00'}
          />
          {lendo && <p className="dica">Lendo o número e o valor da nota…</p>}
          {!lendo && aviso && <p className="dica">{aviso}</p>}

          <label style={{ marginTop: 14 }}>Fotos do material (pelo menos uma)</label>
          <div className="nf-fotos-grade">
            {materialFotos.map((foto, i) => (
              <FotoMini key={i} foto={foto} onRemover={() => setMaterialFotos(fs => fs.filter((_, j) => j !== i))} />
            ))}
            <button type="button" className="nf-foto-add" onClick={() => campoMaterial.current?.click()} disabled={salvando}>
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

          {erro && <div className="erro-caixa">{erro}</div>}

          <div className="jn-pe">
            <button type="button" className="bt bt-neutro" onClick={aoFechar} disabled={salvando}>Cancelar</button>
            <button type="submit" className="bt bt-forte" disabled={salvando || lendo || paginas.length === 0}>
              {salvando ? 'Salvando...' : paginas.length > 1 ? `Salvar nota (${paginas.length} páginas)` : 'Salvar nota'}
            </button>
          </div>
        </form>
      </Janela>

      {scannerAberto && (
        <ScannerDeDocumento
          titulo={`NF · O.C. ${ordem.numero ?? '—'}`}
          paginaInicial={paginas.length + 1}
          aoConcluir={receberPaginas}
          aoCancelar={() => setScannerAberto(false)}
        />
      )}
    </>
  )
}

function PaginaMini({ arquivo, numero, onRemover }: { arquivo: File; numero: number; onRemover: () => void }) {
  const url = useUrlDoArquivo(arquivo)
  // Um PDF não renderiza dentro de <img> — a miniatura vira só o rótulo do
  // arquivo em vez de um quadro quebrado (17/09/2026, "receber por PDF").
  const ehPDF = arquivo.type === 'application/pdf' || arquivo.name.toLowerCase().endsWith('.pdf')
  return (
    <div className="nf-foto-mini nf-pagina-mini">
      {ehPDF ? <div className="nf-pagina-pdf"><IconePDF /></div> : url && <img src={url} alt={`Página ${numero}`} />}
      <span className="nf-pagina-num">{numero}</span>
      <button type="button" className="nf-foto-x" onClick={onRemover} aria-label={`Remover a página ${numero}`}>×</button>
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

function IconeScanner() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 8V5a1 1 0 0 1 1-1h3M16 4h3a1 1 0 0 1 1 1v3M20 16v3a1 1 0 0 1-1 1h-3M8 20H5a1 1 0 0 1-1-1v-3" />
      <path d="M7 12h10" />
    </svg>
  )
}

function IconePDF() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M7 3h7l4 4v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z" />
      <path d="M14 3v4a1 1 0 0 0 1 1h4" />
    </svg>
  )
}

function formatarReais(v: number): string {
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

function formatarValor(v: number): string {
  return v.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
