// rev 1 — trocar a nota fiscal (migração 076, 17/09/2026)
//
// DIFERENTE DE "CORRIGIR A OC" (`CorrecaoDeOC.tsx`)
//
//	Aqui não muda a OC — só a nota em si, porque o fornecedor mandou uma
//	segunda via (corrigida ou não). RC ou RO, a qualquer momento, mesmo
//	depois de já entregue no escritório ou enviada ao cliente. A troca
//	sempre manda a nota de volta pra `recebida` — nunca à frente disso — a
//	nota nova sempre precisa passar pelas duas etapas do escritório de novo.
//
// SEM SCANNER PRÓPRIO, DE PROPÓSITO
//
//	`ReceberNF` usa `ScannerDeDocumento` porque é o fluxo do dia a dia, na
//	obra. Trocar é raro e pode vir do escritório (RC recebendo um PDF por
//	e-mail) ou da obra (RO com a câmera) — um seletor de arquivo comum
//	cobre os dois sem duplicar o scanner de câmera aqui.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { enviarFormulario, ErroMotor } from '../../motor/cliente'
import type { NotaFiscal } from './tipos'

interface Props {
  nota: NotaFiscal
  aoFechar: () => void
  aoSalvar: () => void
}

export function TrocarNF({ nota, aoFechar, aoSalvar }: Props) {
  const [numero, setNumero] = useState(nota.numero ?? '')
  const [valor, setValor] = useState(nota.valor != null ? String(nota.valor).replace('.', ',') : '')
  const [arquivo, setArquivo] = useState<File | null>(null)
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)

  async function salvar(e: FormEvent) {
    e.preventDefault()
    if (!arquivo) {
      setErro('Escolha o arquivo da nota que vai entrar no lugar.')
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
      await enviarFormulario(`/administrativo/nf/notas/${nota.id}/trocar`, forma)
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui trocar esta nota fiscal.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <Janela titulo={`Trocar a nota ${nota.numero ?? '—'}`} descricao="A nota nova volta pra Recebidas — precisa ser entregue no escritório de novo." aoFechar={aoFechar}>
      <form className="jn-corpo" onSubmit={e => void salvar(e)}>
        <label htmlFor="tn-arquivo">Arquivo da nota (foto ou PDF)</label>
        <input
          id="tn-arquivo" type="file" accept="image/*,application/pdf"
          onChange={e => setArquivo(e.target.files?.[0] ?? null)}
        />

        <label htmlFor="tn-numero" style={{ marginTop: 14 }}>Número da nota fiscal</label>
        <input id="tn-numero" value={numero} onChange={e => setNumero(e.target.value)} inputMode="numeric" />

        <label htmlFor="tn-valor">Valor</label>
        <input
          id="tn-valor" inputMode="decimal" value={valor}
          onChange={e => setValor(e.target.value.replace(/[^\d,.]/g, ''))}
        />

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar} disabled={salvando}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando}>
            {salvando ? 'Trocando...' : 'Trocar nota'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
