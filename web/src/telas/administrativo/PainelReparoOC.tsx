// rev 1 — pop-up ancorado ao campo bloqueado na folha (reparo híbrido)
import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { motor } from '../../motor/cliente'
import type { DocumentoOC, ObraCentroSugerida } from './tipos'

export interface ValoresReparoPainel {
  fornecedor_cnpj?: string
  obra_centro_custo?: string
  comprador_cnpj?: string
  comprador_nome?: string
}

interface Props {
  campo: 'fornecedor' | 'faturamento'
  documento: DocumentoOC
  ancora: { x: number; y: number }
  fechar: () => void
  aoConfirmar: (valores: ValoresReparoPainel) => void
  processando?: boolean
}

export function PainelReparoOC({ campo, documento, ancora, fechar, aoConfirmar, processando }: Props) {
  const caixaRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState(ancora)
  const [fornecedorCNPJ, setFornecedorCNPJ] = useState('')
  const [obraBusca, setObraBusca] = useState('')
  const [obraEscolhida, setObraEscolhida] = useState<ObraCentroSugerida | null>(null)
  const [sugestoes, setSugestoes] = useState<ObraCentroSugerida[]>([])
  const [buscandoObra, setBuscandoObra] = useState(false)

  useEffect(() => {
    setFornecedorCNPJ(String(documento.fornecedor_cnpj ?? '').replace(/\D/g, ''))
    setObraBusca(documento.obra_centro_custo ?? '')
    if (documento.obra_centro_custo) {
      setObraEscolhida({
        obra_centro_custo: documento.obra_centro_custo,
        comprador_cnpj: documento.comprador_cnpj,
        comprador_nome: documento.comprador_nome,
      })
    } else {
      setObraEscolhida(null)
    }
  }, [documento, campo])

  useEffect(() => {
    const el = caixaRef.current
    if (!el) return
    const margem = 12
    const r = el.getBoundingClientRect()
    let x = ancora.x
    let y = ancora.y
    if (x + r.width > window.innerWidth - margem) {
      x = Math.max(margem, window.innerWidth - r.width - margem)
    }
    if (y + r.height > window.innerHeight - margem) {
      y = Math.max(margem, window.innerHeight - r.height - margem)
    }
    setPos({ x, y })
  }, [ancora])

  const buscarObras = useCallback(async (q: string) => {
    if (q.trim().length < 2) {
      setSugestoes([])
      return
    }
    setBuscandoObra(true)
    try {
      const r = await motor<{ obras: ObraCentroSugerida[] }>(
        '/administrativo/compras/obras-centro?q=' + encodeURIComponent(q.trim()))
      setSugestoes(r.obras)
    } catch {
      setSugestoes([])
    } finally {
      setBuscandoObra(false)
    }
  }, [])

  useEffect(() => {
    if (campo !== 'faturamento') return
    const t = window.setTimeout(() => { void buscarObras(obraBusca) }, 220)
    return () => window.clearTimeout(t)
  }, [obraBusca, buscarObras, campo])

  useEffect(() => {
    function fora(e: MouseEvent) {
      if (caixaRef.current && !caixaRef.current.contains(e.target as Node)) fechar()
    }
    function esc(e: KeyboardEvent) {
      if (e.key === 'Escape') fechar()
    }
    document.addEventListener('mousedown', fora)
    document.addEventListener('keydown', esc)
    return () => {
      document.removeEventListener('mousedown', fora)
      document.removeEventListener('keydown', esc)
    }
  }, [fechar])

  function confirmar(e?: FormEvent) {
    e?.preventDefault()
    if (processando) return
    if (campo === 'fornecedor') {
      aoConfirmar({ fornecedor_cnpj: fornecedorCNPJ })
      return
    }
    aoConfirmar({
      obra_centro_custo: obraEscolhida?.obra_centro_custo ?? obraBusca.trim(),
      comprador_cnpj: obraEscolhida?.comprador_cnpj ? String(obraEscolhida.comprador_cnpj) : undefined,
      comprador_nome: obraEscolhida?.comprador_nome ? String(obraEscolhida.comprador_nome) : undefined,
    })
  }

  return (
    <div
      ref={caixaRef}
      className="adm-reparo-painel"
      style={{ top: pos.y, left: pos.x }}
      role="dialog"
      aria-modal="true"
      aria-labelledby="adm-reparo-painel-titulo"
    >
      <form onSubmit={confirmar}>
        <h3 id="adm-reparo-painel-titulo">
          {campo === 'fornecedor' ? 'CNPJ do fornecedor' : 'Obra / faturamento'}
        </h3>

        {campo === 'fornecedor' && (
          <label className="adm-reparo-campo">
            <span>CNPJ do fornecedor</span>
            <input
              type="text"
              inputMode="numeric"
              autoFocus
              value={fornecedorCNPJ}
              onChange={ev => setFornecedorCNPJ(ev.target.value.replace(/\D/g, '').slice(0, 14))}
              placeholder="14 dígitos"
            />
          </label>
        )}

        {campo === 'faturamento' && (
          <>
            <label className="adm-reparo-campo">
              <span>Obra / centro de custo</span>
              <input
                type="text"
                autoFocus
                value={obraBusca}
                onChange={ev => {
                  setObraBusca(ev.target.value)
                  setObraEscolhida(null)
                }}
                placeholder="Digite ao menos 2 letras para buscar"
                autoComplete="off"
              />
              {buscandoObra && <em className="adm-reparo-busca">buscando…</em>}
              {sugestoes.length > 0 && (
                <ul className="adm-reparo-lista">
                  {sugestoes.map(o => (
                    <li key={o.obra_centro_custo}>
                      <button
                        type="button"
                        className={'adm-reparo-opcao' + (obraEscolhida?.obra_centro_custo === o.obra_centro_custo ? ' escolhida' : '')}
                        onClick={() => {
                          setObraEscolhida(o)
                          setObraBusca(o.obra_centro_custo)
                          setSugestoes([])
                        }}
                      >
                        {o.obra_centro_custo}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </label>
            {(documento.comprador_cnpj || documento.comprador_nome) && (
              <p className="adm-reparo-dica">
                Faturamento: {documento.comprador_nome || '—'}
                {documento.comprador_cnpj ? ` · CNPJ ${documento.comprador_cnpj}` : ''}
              </p>
            )}
          </>
        )}

        <div className="adm-reparo-botoes">
          <button type="button" className="bt bt-neutro" onClick={fechar} disabled={processando}>cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={processando}>
            {processando ? 'aplicando…' : 'OK'}
          </button>
        </div>
      </form>
    </div>
  )
}
