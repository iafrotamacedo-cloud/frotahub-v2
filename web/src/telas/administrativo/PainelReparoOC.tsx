// rev 2 — pop-up ancorado ao campo bloqueado na folha (reparo híbrido)
//
// FATURAMENTO SAIU DAQUI EM 11/09/2026
//
//	Corrigir o CNPJ/nome de faturamento só no nosso lado deixava o registro
//	do Obra Prima errado para sempre — a correção virou substituir o arquivo
//	inteiro (ver o cabeçalho de `TabelaDeOrdens.tsx`), não editar um campo
//	aqui. Sobrou fornecedor (CNPJ, digitado) e endereço de cobrança (escolha
//	entre os dois candidatos que o próprio PDF já traz — nunca texto livre,
//	porque qualquer um dos dois já resolve o bloqueio).
import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import type { DocumentoOC } from './tipos'

export interface ValoresReparoPainel {
  fornecedor_cnpj?: string
  /** Só para `campo === 'endereco'`: o valor escolhido (obra ou faturamento). */
  endereco_cobranca?: string
}

interface Props {
  campo: 'fornecedor' | 'endereco'
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
  const [escolha, setEscolha] = useState<'obra' | 'faturamento' | null>(null)

  useEffect(() => {
    setFornecedorCNPJ(String(documento.fornecedor_cnpj ?? '').replace(/\D/g, ''))
    setEscolha(null)
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

  const enderecoObra = documento.endereco_entrega?.trim() || '(a obra não tem endereço de entrega lido)'
  const enderecoFaturamento = documento.faturamento_endereco?.trim() || '(o faturamento não tem endereço lido)'

  const confirmar = useCallback((e?: FormEvent) => {
    e?.preventDefault()
    if (processando) return
    if (campo === 'fornecedor') {
      aoConfirmar({ fornecedor_cnpj: fornecedorCNPJ })
      return
    }
    if (!escolha) return
    aoConfirmar({ endereco_cobranca: escolha === 'obra' ? documento.endereco_entrega : documento.faturamento_endereco })
  }, [processando, campo, fornecedorCNPJ, escolha, documento, aoConfirmar])

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
          {campo === 'fornecedor' ? 'CNPJ do fornecedor' : 'Endereço de cobrança'}
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

        {campo === 'endereco' && (
          <div className="adm-reparo-opcoes-endereco">
            <label className="adm-reparo-opcao-radio">
              <input
                type="radio"
                name="endereco-cobranca"
                checked={escolha === 'obra'}
                onChange={() => setEscolha('obra')}
              />
              <span>
                <b>Usar endereço da obra</b>
                <em>{enderecoObra}</em>
              </span>
            </label>
            <label className="adm-reparo-opcao-radio">
              <input
                type="radio"
                name="endereco-cobranca"
                checked={escolha === 'faturamento'}
                onChange={() => setEscolha('faturamento')}
              />
              <span>
                <b>Usar endereço do faturamento</b>
                <em>{enderecoFaturamento}</em>
              </span>
            </label>
          </div>
        )}

        <div className="adm-reparo-botoes">
          <button type="button" className="bt bt-neutro" onClick={fechar} disabled={processando}>cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={processando || (campo === 'endereco' && !escolha)}>
            {processando ? 'aplicando…' : 'OK'}
          </button>
        </div>
      </form>
    </div>
  )
}
