// rev 3 — lançar no Trílogo: descrição + mão de obra + materiais
//
// AS DUAS LISTAS ENTRAM NO MESMO ORÇAMENTO
//
//	Decisão do dono em 04/09/2026 era só mão de obra; revista em 08/09/2026
//	para incluir material também — o motor (servicos/lancar.go,
//	LancarNoTrilogo) já manda as duas pro mesmo trilogo.MontarOrcamentoServico,
//	este formulário só precisava da segunda lista.
//
// VALOR É TEXTO ENQUANTO SE DIGITA, NÃO NÚMERO
//
//	Mesmo padrão de orcamentos/Faturamento.tsx e ConferirValor.tsx: vírgula é
//	como se digita dinheiro aqui, e um `Number(e.target.value)` a cada tecla
//	transforma "150,50" em NaN → 0, silenciosamente. O valor só vira número
//	no envio, com o mesmo `.replace(/\./g, '').replace(',', '.')`. O total
//	formatado (emReais) embaixo de cada lista é o retorno visual de que o
//	que foi digitado bateu — o mesmo motivo de ConferirValor mostrar a soma
//	dos itens ao lado do total.
import { useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { emReais } from '../orcamentos/tipos'
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

/** Um item enquanto é digitado — qtd/valor em texto, não em número. */
interface ItemRascunho {
  descricao: string
  qtd: string
  valor: string
}

const ITEM_VAZIO: ItemRascunho = { descricao: '', qtd: '1', valor: '' }

/** "1.234,56" (como se digita) → 1234.56. Mesma conta de Faturamento.tsx. */
function paraNumero(digitado: string): number {
  const n = Number(digitado.replace(/\./g, '').replace(',', '.'))
  return Number.isFinite(n) ? n : 0
}

function paraItens(rascunhos: ItemRascunho[]): ItemDeOrcamento[] {
  return rascunhos
    .filter(it => it.descricao.trim())
    .map(it => ({ descricao: it.descricao.trim(), qtd: paraNumero(it.qtd) || 1, valor: paraNumero(it.valor) }))
}

function total(rascunhos: ItemRascunho[]): number {
  return paraItens(rascunhos).reduce((soma, it) => soma + it.qtd * it.valor, 0)
}

export function FormularioDeLancamento({ itemID, aoFeito, aoCancelar }: Props) {
  const [descricao, setDescricao] = useState('')
  const [maoDeObra, setMaoDeObra] = useState<ItemRascunho[]>([{ ...ITEM_VAZIO }])
  const [materiais, setMateriais] = useState<ItemRascunho[]>([])
  const [erro, setErro] = useState<string | null>(null)
  const [enviando, setEnviando] = useState(false)

  // Ao menos um item PREENCHIDO (descrição não vazia), de qualquer lista —
  // uma linha em branco sozinha (o estado inicial) não conta.
  const temItem = (lista: ItemRascunho[]) => lista.some(it => it.descricao.trim())
  const podeEnviar = descricao.trim() && (temItem(maoDeObra) || temItem(materiais))
  const totalGeral = total(maoDeObra) + total(materiais)

  async function enviar() {
    if (!podeEnviar) return
    setEnviando(true)
    setErro(null)
    try {
      const r = await motor<RespostaLancamento>(`/servicos/kanban/${itemID}/lancar`, {
        metodo: 'POST',
        corpo: {
          descricao: descricao.trim(),
          mao_de_obra: paraItens(maoDeObra),
          materiais: paraItens(materiais),
        },
      })
      aoFeito(`Lançado no Trílogo — orçamento ${r.orcamento_id}, ${emReais(r.valor)}.`)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui lançar no Trílogo.')
      setEnviando(false)
    }
  }

  return (
    <div className="sv-form">
      <label htmlFor="fl-descricao">Descrição da cotação</label>
      <input
        id="fl-descricao" value={descricao} onChange={e => setDescricao(e.target.value)}
        placeholder="O que está sendo orçado" disabled={enviando}
      />

      <ListaDeItens
        titulo="Mão de obra" itens={maoDeObra} aoMudar={setMaoDeObra} enviando={enviando}
      />
      <ListaDeItens
        titulo="Materiais" itens={materiais} aoMudar={setMateriais} enviando={enviando}
      />

      <p className="sv-total">Total do orçamento: <b>{emReais(totalGeral)}</b></p>

      {erro && <div className="erro-caixa">{erro}</div>}

      <div className="jn-pe">
        <button type="button" className="bt bt-neutro" disabled={enviando} onClick={aoCancelar}>Cancelar</button>
        <button
          type="button" className="bt bt-forte" disabled={enviando || !podeEnviar}
          onClick={() => void enviar()}
        >
          {enviando ? 'Lançando...' : 'Lançar no Trílogo'}
        </button>
      </div>
    </div>
  )
}

function ListaDeItens({ titulo, itens, aoMudar, enviando }: {
  titulo: string
  itens: ItemRascunho[]
  aoMudar: (itens: ItemRascunho[]) => void
  enviando: boolean
}) {
  function atualizar(i: number, campo: keyof ItemRascunho, valor: string) {
    aoMudar(itens.map((it, idx) => idx !== i ? it : { ...it, [campo]: valor }))
  }

  const subtotal = total(itens)

  return (
    <>
      <span className="sv-itens-titulo">
        {titulo}
        {subtotal > 0 && <span className="sv-subtotal">{emReais(subtotal)}</span>}
      </span>
      {itens.map((it, i) => (
        <div key={i} className="sv-item-linha">
          <input
            value={it.descricao} placeholder="Descrição" disabled={enviando}
            onChange={e => atualizar(i, 'descricao', e.target.value)}
          />
          <input
            value={it.qtd} placeholder="Qtd" inputMode="decimal" disabled={enviando}
            onChange={e => atualizar(i, 'qtd', e.target.value)}
          />
          <input
            value={it.valor} placeholder="Valor unit. (ex: 150,00)" inputMode="decimal" disabled={enviando}
            onChange={e => atualizar(i, 'valor', e.target.value)}
          />
          <button
            type="button" className="bt bt-mini bt-neutro" disabled={enviando}
            onClick={() => aoMudar(itens.filter((_, idx) => idx !== i))}
            aria-label={`Remover item ${i + 1} de ${titulo}`}
          >
            ×
          </button>
        </div>
      ))}
      <button
        type="button" className="bt bt-mini bt-neutro" disabled={enviando}
        onClick={() => aoMudar([...itens, { ...ITEM_VAZIO }])}
      >
        + item
      </button>
    </>
  )
}
