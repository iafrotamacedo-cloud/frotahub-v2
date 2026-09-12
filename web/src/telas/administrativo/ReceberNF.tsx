// rev 1 — a janela de "receber esta nota fiscal" (almoxarife, mobile-friendly)
//
// SÓ TRÊS CAMPOS, DE PROPÓSITO
//
//	Número, valor, foto/PDF — o mínimo que o almoxarife tem em mãos na hora
//	de conferir um material na obra. `capture="environment"` abre a câmera
//	direto no celular, sem passar pela galeria primeiro.
import { useState, type FormEvent } from 'react'
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
  const [arquivo, setArquivo] = useState<File | null>(null)
  const [erro, setErro] = useState('')
  const [enviando, setEnviando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    if (!numero.trim() || !valor.trim() || !arquivo) {
      setErro('Preencha o número, o valor e escolha a foto ou o PDF da nota.')
      return
    }
    setErro('')
    setEnviando(true)
    try {
      const forma = new FormData()
      forma.append('numero', numero.trim())
      forma.append('valor', valor.trim())
      forma.append('arquivo', arquivo, arquivo.name)
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

        <label htmlFor="nf-arquivo">Foto ou PDF da nota</label>
        <input
          id="nf-arquivo" type="file" accept="application/pdf,image/*" capture="environment"
          onChange={e => setArquivo(e.target.files?.[0] ?? null)}
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

function formatarReais(v: number): string {
  return v.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}
