// rev 1 — reparo híbrido: PDF na tela, caixas nos campos bloqueados, pop-up ao lado
import { useCallback, useEffect, useRef, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { PainelReparoOC, type ValoresReparoPainel } from './PainelReparoOC'
import { REGIAO_FATURAMENTO, REGIAO_FORNECEDOR, REGIAO_OBRA } from './regioesReparoOC'
import type { DocumentoOC, EstadoDocumentoOC } from './tipos'

interface Props {
  ordemId: string
  enderecoInicial: string
  nome: string
  titulo: string
  motivos: string[]
  motivoCompleto?: string | null
  voltar: () => void
  aoSalvar: (r: EstadoDocumentoOC) => void
}

/** Posições calibradas no layout 019731 — ver regioesReparoOC.ts */
export function RepararOrdemOC({
  ordemId, enderecoInicial, nome, titulo, motivos, motivoCompleto, voltar, aoSalvar,
}: Props) {
  const [carregando, setCarregando] = useState(true)
  const [erro, setErro] = useState('')
  const [documento, setDocumento] = useState<DocumentoOC | null>(null)
  const [validacao, setValidacao] = useState<Pick<EstadoDocumentoOC, 'precisa_fornecedor' | 'precisa_faturamento' | 'motivos' | 'status'> | null>(null)
  const [camposIniciais, setCamposIniciais] = useState<{ fornecedor: boolean; faturamento: boolean } | null>(null)
  const [pdfUrl, setPdfUrl] = useState(enderecoInicial)
  const [pdfBlob, setPdfBlob] = useState<Blob | null>(null)
  const [pdfChave, setPdfChave] = useState(0)
  const urlLocalRef = useRef<string | null>(null)
  const [popup, setPopup] = useState<{ campo: 'fornecedor' | 'faturamento'; ancora: { x: number; y: number } } | null>(null)
  const [anteverendo, setAnteverendo] = useState(false)
  const [salvando, setSalvando] = useState(false)

  const revogarLocal = useCallback(() => {
    if (urlLocalRef.current) {
      URL.revokeObjectURL(urlLocalRef.current)
      urlLocalRef.current = null
    }
  }, [])

  useEffect(() => () => { revogarLocal() }, [revogarLocal])

  useEffect(() => {
    let vivo = true
    void (async () => {
      try {
        const r = await motor<EstadoDocumentoOC>(`/administrativo/compras/ordens/${ordemId}/documento`)
        if (!vivo) return
        const doc = fecharConta(r.documento)
        setDocumento(doc)
        setValidacao({
          precisa_fornecedor: r.precisa_fornecedor,
          precisa_faturamento: r.precisa_faturamento,
          motivos: r.motivos,
          status: r.status,
        })
        setCamposIniciais({ fornecedor: r.precisa_fornecedor, faturamento: r.precisa_faturamento })

        // A PRÉVIA ABRE JÁ NO NOSSO LAYOUT, NUNCA NO PDF ORIGINAL
        //
        //	O arquivo original (armazém) tem o layout que o Obra Prima mandou —
        //	endereço mais comprido, I.E. vazio, o que for — e as caixas de
        //	regioesReparoOC.ts são calibradas em cima do NOSSO desenho
        //	(documento_pdf.go), não do original. Mostrar o original aqui fazia
        //	as caixas caírem em qualquer lugar que não os campos de verdade.
        //	Redesenhar já na entrada (mesmo `?antever=1` que a edição usa)
        //	garante que a prévia é exatamente o PDF que "Salvar" vai gravar, e
        //	é a MESMA régua das caixas — sem isso, calibrar é mirar num alvo
        //	que nem está na tela.
        await antever(doc)
      } catch (e) {
        if (vivo) setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o documento.')
      } finally {
        if (vivo) setCarregando(false)
      }
    })()
    return () => { vivo = false }
  }, [ordemId])

  const aplicarPdfPreview = useCallback((base64: string) => {
    revogarLocal()
    const bin = atob(base64)
    const bytes = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
    const blob = new Blob([bytes], { type: 'application/pdf' })
    const url = URL.createObjectURL(blob)
    urlLocalRef.current = url
    setPdfBlob(blob)
    setPdfUrl(url)
    setPdfChave(k => k + 1)
  }, [revogarLocal])

  async function antever(doc: DocumentoOC) {
    setAnteverendo(true)
    setErro('')
    try {
      const r = await motor<EstadoDocumentoOC>(
        `/administrativo/compras/ordens/${ordemId}/documento?antever=1`,
        { metodo: 'POST', corpo: { documento: fecharConta(doc) } },
      )
      setDocumento(fecharConta(r.documento))
      setValidacao({
        precisa_fornecedor: r.precisa_fornecedor,
        precisa_faturamento: r.precisa_faturamento,
        motivos: r.motivos,
        status: r.status,
      })
      if (r.pdf_base64) aplicarPdfPreview(r.pdf_base64)
      setPopup(null)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui atualizar a prévia do PDF.')
    } finally {
      setAnteverendo(false)
    }
  }

  function mesclarReparo(doc: DocumentoOC, campo: 'fornecedor' | 'faturamento', v: ValoresReparoPainel): DocumentoOC {
    const d = { ...doc }
    if (campo === 'fornecedor' && v.fornecedor_cnpj !== undefined) {
      d.fornecedor_cnpj = v.fornecedor_cnpj.replace(/\D/g, '').slice(0, 14)
      if (!d.fornecedor_nome.trim()) d.fornecedor_nome = 'Fornecedor CNPJ ' + d.fornecedor_cnpj
    }
    if (campo === 'faturamento') {
      if (v.obra_centro_custo) d.obra_centro_custo = v.obra_centro_custo.trim()
      if (v.comprador_cnpj) d.comprador_cnpj = v.comprador_cnpj.replace(/\D/g, '').slice(0, 14)
      if (v.comprador_nome) d.comprador_nome = v.comprador_nome.trim()
    }
    return fecharConta(d)
  }

  function abrirPopup(campo: 'fornecedor' | 'faturamento', el: HTMLElement) {
    const r = el.getBoundingClientRect()
    setPopup({ campo, ancora: { x: r.right + 10, y: r.top } })
  }

  async function confirmarPopup(valores: ValoresReparoPainel) {
    if (!documento || !popup) return
    const novo = mesclarReparo(documento, popup.campo, valores)
    await antever(novo)
  }

  async function salvarFinal() {
    if (!documento || !validacao) return
    if (validacao.precisa_fornecedor || validacao.precisa_faturamento) return
    setSalvando(true)
    setErro('')
    try {
      const r = await motor<EstadoDocumentoOC>(`/administrativo/compras/ordens/${ordemId}/documento`, {
        metodo: 'POST',
        corpo: { documento: fecharConta(documento) },
      })
      aoSalvar(r)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui gravar a ordem de compra.')
    } finally {
      setSalvando(false)
    }
  }

  if (carregando) return <Carregando texto="Abrindo reparo…" />
  if (!documento || !validacao || !camposIniciais) {
    return (
      <div className="orc-tela">
        <div className="orc-barra">
          <button type="button" className="orc-voltar" onClick={voltar}>← voltar</button>
          <b>{titulo}</b>
        </div>
        <p className="erro">{erro || 'Não consegui montar a tela de reparo.'}</p>
      </div>
    )
  }

  const pronto = !validacao.precisa_fornecedor && !validacao.precisa_faturamento
  const barraAlta = (validacao.motivos.length || motivos.length) > 1
  const motivosBarra = validacao.motivos.length > 0 ? validacao.motivos : motivos

  const overlays = (
    <div className="adm-campos-bloqueio" aria-hidden={anteverendo}>
      {camposIniciais.fornecedor ? (
        <button
          type="button"
          className={'adm-campo-bloqueio' + (validacao.precisa_fornecedor ? ' erro' : ' ok')}
          style={REGIAO_FORNECEDOR}
          disabled={anteverendo}
          onClick={e => abrirPopup('fornecedor', e.currentTarget)}
          title={validacao.precisa_fornecedor ? 'Clique para corrigir o CNPJ do fornecedor' : 'Clique para ajustar o fornecedor'}
        >
          <span className="adm-campo-bloqueio-rotulo">Fornecedor · CNPJ</span>
          <span className="adm-campo-bloqueio-valor">
            {documento.fornecedor_cnpj || '(vazio)'}
          </span>
        </button>
      ) : null}

      {camposIniciais.faturamento ? (
        <>
          <button
            type="button"
            className={'adm-campo-bloqueio' + (validacao.precisa_faturamento ? ' erro' : ' ok')}
            style={REGIAO_FATURAMENTO}
            disabled={anteverendo}
            onClick={e => abrirPopup('faturamento', e.currentTarget)}
            title={validacao.precisa_faturamento ? 'Clique para corrigir o faturamento' : 'Clique para ajustar o faturamento'}
          >
            <span className="adm-campo-bloqueio-rotulo">Faturamento · CNPJ</span>
            <span className="adm-campo-bloqueio-valor">
              {documento.comprador_cnpj || '(vazio)'}
              {documento.comprador_nome ? ` · ${documento.comprador_nome}` : ''}
            </span>
          </button>
          <button
            type="button"
            className={'adm-campo-bloqueio' + (validacao.precisa_faturamento ? ' erro' : ' ok')}
            style={REGIAO_OBRA}
            disabled={anteverendo}
            onClick={e => abrirPopup('faturamento', e.currentTarget)}
            title={validacao.precisa_faturamento ? 'Clique para corrigir obra / centro de custo' : 'Clique para ajustar a obra'}
          >
            <span className="adm-campo-bloqueio-rotulo">Obra / centro de custo</span>
            <span className="adm-campo-bloqueio-valor">
              {documento.obra_centro_custo || '(vazio)'}
            </span>
          </button>
        </>
      ) : null}
    </div>
  )

  return (
    <>
      <VisorDeDocumento
        endereco={pdfUrl}
        nomeSugerido={nome}
        titulo={titulo}
        barraAlta={barraAlta}
        chavePDF={pdfChave}
        blobArquivo={pdfBlob}
        folhaProporcional
        sobreDocumento={overlays}
        destaque={motivosBarra.length > 0 ? (
          motivosBarra.map((m, i) => (
            <span key={i} className="adm-motivo-barra" title={motivoCompleto ?? undefined}>{m}</span>
          ))
        ) : undefined}
        acoes={(
          <button
            type="button"
            className="bt bt-forte adm-salvar-reparo"
            disabled={!pronto || salvando || anteverendo}
            onClick={() => void salvarFinal()}
          >
            {salvando ? 'gravando…' : 'Salvar'}
          </button>
        )}
        voltar={voltar}
      />

      {erro && <p className="erro adm-erro-flutuante">{erro}</p>}

      {popup && (
        <PainelReparoOC
          campo={popup.campo}
          documento={documento}
          ancora={popup.ancora}
          fechar={() => setPopup(null)}
          aoConfirmar={v => void confirmarPopup(v)}
          processando={anteverendo}
        />
      )}
    </>
  )
}

function round2(n: number): number {
  return Math.round(n * 100) / 100
}

function fecharConta(doc: DocumentoOC): DocumentoOC {
  const itens = (doc.itens ?? []).map(it => {
    const bruto = round2((Number(it.qtd) || 0) * (Number(it.valor_unit) || 0))
    const desconto = Number(it.desconto) || 0
    return { ...it, total: Math.max(0, round2(bruto - desconto)) }
  })
  const subtotal = round2(itens.reduce((s, it) => s + round2((Number(it.qtd) || 0) * (Number(it.valor_unit) || 0)), 0))
  const desconto = round2(itens.reduce((s, it) => s + (Number(it.desconto) || 0), 0))
  const frete = Number(doc.frete) || 0
  return { ...doc, itens, subtotal, desconto, frete, total: round2(subtotal - desconto + frete) }
}
