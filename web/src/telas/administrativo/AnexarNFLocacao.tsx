// rev 1 — anexar a NF de locação (migração 077, 17/09/2026)
//
// A NF NUNCA CHEGA NA OBRA — SÓ POR E-MAIL, PRO ADM
//
//	Diferente de `ReceberNF` (câmera, na obra) e de `TrocarNF` (corrige uma
//	nota que já existe), esta é a PRIMEIRA vez que a nota de locação ganha
//	número e valor — até aqui só existia o romaneio. Anexar já confirma a
//	entrega no escritório (não tem papel físico pra "chegar depois") — a
//	nota pula direto pra `entregue_escritorio`.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { enviarFormulario, ErroMotor } from '../../motor/cliente'
import type { NotaFiscal } from './tipos'

interface Props {
  nota: NotaFiscal
  aoFechar: () => void
  aoSalvar: () => void
}

export function AnexarNFLocacao({ nota, aoFechar, aoSalvar }: Props) {
  const [numero, setNumero] = useState('')
  const [valor, setValor] = useState('')
  const [arquivo, setArquivo] = useState<File | null>(null)
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)

  async function salvar(e: FormEvent) {
    e.preventDefault()
    if (!arquivo) {
      setErro('Anexe o PDF ou a foto da nota fiscal.')
      return
    }
    if (!numero.trim() || !valor.trim()) {
      setErro('Preencha o número e o valor da nota.')
      return
    }
    setErro('')
    setSalvando(true)
    try {
      const forma = new FormData()
      forma.append('numero', numero.trim())
      forma.append('valor', valor.trim())
      forma.append('arquivo', arquivo, arquivo.name)
      await enviarFormulario(`/administrativo/nf/notas/${nota.id}/anexar-locacao`, forma)
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui anexar esta nota fiscal.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo={`Anexar NF · O.C. ${nota.ordem_numero ?? '—'}`}
      descricao="A nota que a locadora mandou por e-mail — isto já confirma a entrega no escritório."
      aoFechar={aoFechar}
    >
      <form className="jn-corpo" onSubmit={e => void salvar(e)}>
        <label htmlFor="anl-arquivo">Arquivo da nota (foto ou PDF)</label>
        <input
          id="anl-arquivo" type="file" accept="image/*,application/pdf"
          onChange={e => setArquivo(e.target.files?.[0] ?? null)}
        />

        <label htmlFor="anl-numero" style={{ marginTop: 14 }}>Número da nota fiscal</label>
        <input id="anl-numero" value={numero} onChange={e => setNumero(e.target.value)} inputMode="numeric" />

        <label htmlFor="anl-valor">Valor</label>
        <input
          id="anl-valor" inputMode="decimal" value={valor}
          onChange={e => setValor(e.target.value.replace(/[^\d,.]/g, ''))}
          placeholder="0,00"
        />

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar} disabled={salvando}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando}>
            {salvando ? 'Anexando...' : 'Anexar nota'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
