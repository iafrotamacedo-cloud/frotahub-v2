// rev 1 — lançar no Trílogo: descrição + mão de obra
//
// SÓ MÃO DE OBRA (decisão do dono, 04/09/2026)
//
//	Serviço não usa a categoria "material" do Trílogo — o motor
//	(servicos/lancar.go, LancarNoTrilogo) já manda a lista de materiais
//	sempre vazia; este formulário nem pergunta.
import { useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import type { ItemDeOrcamento } from './tipos'

interface Props {
  itemID: string
  aoFeito: (recado: string) => void
  aoCancelar: () => void
}

interface RespostaLancamento {
  ok: boolean
  cotacao_id: number
  orcamento_id: number
  valor: number
}

export function FormularioDeLancamento({ itemID, aoFeito, aoCancelar }: Props) {
  const [descricao, setDescricao] = useState('')
  const [itens, setItens] = useState<ItemDeOrcamento[]>([{ descricao: '', valor: 0, qtd: 1 }])
  const [erro, setErro] = useState<string | null>(null)
  const [enviando, setEnviando] = useState(false)

  async function enviar() {
    if (!descricao.trim() || itens.length === 0) return
    setEnviando(true)
    setErro(null)
    try {
      const r = await motor<RespostaLancamento>(`/servicos/kanban/${itemID}/lancar`, {
        metodo: 'POST',
        corpo: { descricao: descricao.trim(), itens },
      })
      aoFeito(`Lançado no Trílogo — orçamento ${r.orcamento_id}, ${r.valor.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })}.`)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui lançar no Trílogo.')
      setEnviando(false)
    }
  }

  function atualizar(i: number, campo: keyof ItemDeOrcamento, valor: string) {
    setItens(atual => atual.map((it, idx) => idx !== i
      ? it
      : { ...it, [campo]: campo === 'descricao' ? valor : Number(valor) || 0 }))
  }

  return (
    <div className="sv-form">
      <label htmlFor="fl-descricao">Descrição da cotação</label>
      <input
        id="fl-descricao" value={descricao} onChange={e => setDescricao(e.target.value)}
        placeholder="O que está sendo orçado" disabled={enviando}
      />

      <span className="sv-itens-titulo">Mão de obra</span>
      {itens.map((it, i) => (
        <div key={i} className="sv-item-linha">
          <input
            value={it.descricao} placeholder="Descrição" disabled={enviando}
            onChange={e => atualizar(i, 'descricao', e.target.value)}
          />
          <input
            value={it.qtd || ''} placeholder="Qtd" inputMode="decimal" disabled={enviando}
            onChange={e => atualizar(i, 'qtd', e.target.value)}
          />
          <input
            value={it.valor || ''} placeholder="Valor unit." inputMode="decimal" disabled={enviando}
            onChange={e => atualizar(i, 'valor', e.target.value)}
          />
          <button
            type="button" className="bt bt-mini bt-neutro" disabled={enviando || itens.length === 1}
            onClick={() => setItens(atual => atual.filter((_, idx) => idx !== i))}
            aria-label={`Remover item ${i + 1}`}
          >
            ×
          </button>
        </div>
      ))}
      <button
        type="button" className="bt bt-mini bt-neutro" disabled={enviando}
        onClick={() => setItens(atual => [...atual, { descricao: '', valor: 0, qtd: 1 }])}
      >
        + item
      </button>

      {erro && <div className="erro-caixa">{erro}</div>}

      <div className="jn-pe">
        <button type="button" className="bt bt-neutro" disabled={enviando} onClick={aoCancelar}>Cancelar</button>
        <button
          type="button" className="bt bt-forte" disabled={enviando || !descricao.trim()}
          onClick={() => void enviar()}
        >
          {enviando ? 'Lançando...' : 'Lançar no Trílogo'}
        </button>
      </div>
    </div>
  )
}
