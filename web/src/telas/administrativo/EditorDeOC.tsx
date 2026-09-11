// rev 1 — editor da OC: a folha editável, um campo por rótulo da amostra
import { useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import type { DocumentoOC, EstadoDocumentoOC, ItemDocumentoOC } from './tipos'

interface Props {
  ordemId: string
  voltar: () => void
  aoSalvar: (r: EstadoDocumentoOC) => void
}

export function EditorDeOC({ ordemId, voltar, aoSalvar }: Props) {
  const [estado, setEstado] = useState<EstadoDocumentoOC | null>(null)
  const [doc, setDoc] = useState<DocumentoOC | null>(null)
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)

  useEffect(() => {
    void (async () => {
      try {
        const r = await motor<EstadoDocumentoOC>(`/administrativo/compras/ordens/${ordemId}/documento`)
        setEstado(r)
        setDoc(fecharConta(r.documento))
      } catch (e) {
        setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o documento.')
      }
    })()
  }, [ordemId])

  function mudar<K extends keyof DocumentoOC>(campo: K, valor: DocumentoOC[K]) {
    setDoc(d => d ? fecharConta({ ...d, [campo]: valor }) : d)
  }

  function mudarItem(i: number, campo: keyof ItemDocumentoOC, valor: string | number) {
    setDoc(d => {
      if (!d) return d
      const itens = d.itens.map((it, n) => n === i ? { ...it, [campo]: valor } : it)
      return fecharConta({ ...d, itens })
    })
  }

  function addItem() {
    setDoc(d => {
      if (!d) return d
      const itens = [...d.itens, { descricao: '', qtd: 1, unidade: 'UN', valor_unit: 0, desconto: 0, total: 0 }]
      return fecharConta({ ...d, itens })
    })
  }

  function apagarItem(i: number) {
    setDoc(d => {
      if (!d || d.itens.length <= 1) return d
      return fecharConta({ ...d, itens: d.itens.filter((_, n) => n !== i) })
    })
  }

  async function salvar() {
    if (!doc) return
    setSalvando(true)
    setErro('')
    try {
      const r = await motor<EstadoDocumentoOC>(`/administrativo/compras/ordens/${ordemId}/documento`, {
        metodo: 'POST',
        corpo: { documento: fecharConta(doc) },
      })
      setEstado(r)
      setDoc(fecharConta(r.documento))
      aoSalvar(r)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui gravar o documento.')
    } finally {
      setSalvando(false)
    }
  }

  if (!doc && !erro) return <Carregando texto="Abrindo o documento…" />
  if (!doc) {
    return (
      <div className="orc-tela">
        <div className="orc-barra">
          <button type="button" className="orc-voltar" onClick={voltar}>← voltar</button>
          <b>Editar ordem de compra</b>
        </div>
        <p className="erro">{erro}</p>
      </div>
    )
  }

  const alertaForn = !!estado?.precisa_fornecedor
  const alertaFat = !!estado?.precisa_faturamento

  return (
    <div className="orc-tela">
      <div className={'orc-barra' + ((estado?.motivos.length ?? 0) > 1 ? ' orc-barra--alta' : '')}>
        <button type="button" className="orc-voltar" onClick={voltar}>← voltar</button>
        <b>Editar · O.C. {doc.numero || 'sem número'}</b>
        {estado?.motivos && estado.motivos.length > 0 && (
          <div className="adm-motivos-barra">
            {estado.motivos.map((m, i) => (
              <span key={i} className="adm-motivo-barra">{m}</span>
            ))}
          </div>
        )}
        <span className="orc-ficha-direita">
          <button type="button" className="bt bt-forte" onClick={() => void salvar()} disabled={salvando}>
            {salvando ? 'gravando…' : 'salvar PDF'}
          </button>
        </span>
      </div>

      {erro && <p className="erro">{erro}</p>}
      {estado?.pco_enviado && (
        <p className="adm-aviso-pco">O e-mail do PCO já saiu com o PDF antigo. Salvar substitui só o arquivo guardado.</p>
      )}

      <div className="adm-editor-rolo">
        <div className="adm-folha">
          <header className="adm-folha-topo">
            <label className="adm-campo adm-campo--largo">
              <input value={doc.emitente_razao} onChange={e => mudar('emitente_razao', e.target.value)} />
            </label>
            <label className="adm-campo adm-campo--curto">
              <span>impressão</span>
              <input value={doc.data_impressao} onChange={e => mudar('data_impressao', e.target.value)} />
            </label>
            <label className="adm-campo adm-campo--largo">
              <input value={doc.emitente_endereco} onChange={e => mudar('emitente_endereco', e.target.value)} />
            </label>
            <label className="adm-campo adm-campo--largo">
              <span>contato / CNPJ</span>
              <div className="adm-folha-linha">
                <input value={doc.emitente_contato} onChange={e => mudar('emitente_contato', e.target.value)} />
                <input value={doc.emitente_cnpj} onChange={e => mudar('emitente_cnpj', e.target.value)} />
              </div>
            </label>
          </header>

          <div className="adm-folha-titulo">
            <span>ORDEM DE COMPRA</span>
            <input className="adm-oc-num" value={doc.numero} onChange={e => mudar('numero', e.target.value)} />
          </div>
          <label className="adm-campo">
            <input value={doc.titulo} onChange={e => mudar('titulo', e.target.value)} placeholder="obra / cliente" />
          </label>

          <h4>DADOS DA ORDEM DE COMPRA</h4>
          <div className="adm-folha-grade">
            <Campo rotulo="Data" valor={doc.data} onChange={v => mudar('data', v)} />
            <Campo rotulo="Previsão da entrega" valor={doc.previsao_entrega} onChange={v => mudar('previsao_entrega', v)} />
            <Campo rotulo="Cond. pgto." valor={doc.cond_pgto} onChange={v => mudar('cond_pgto', v)} />
            <Campo rotulo="Forma pgto." valor={doc.forma_pgto} onChange={v => mudar('forma_pgto', v)} />
            <Campo rotulo="Observação" valor={doc.observacao} onChange={v => mudar('observacao', v)} largo />
          </div>

          <h4>RESPONSÁVEL PELA COMPRA</h4>
          <div className="adm-folha-grade">
            <Campo rotulo="Nome" valor={doc.responsavel_nome} onChange={v => mudar('responsavel_nome', v)} />
            <Campo rotulo="Comprador" valor={doc.comprador_interno} onChange={v => mudar('comprador_interno', v)} />
            <Campo rotulo="Email" valor={doc.responsavel_email} onChange={v => mudar('responsavel_email', v)} />
          </div>

          <h4>DADOS DO FATURAMENTO</h4>
          <div className="adm-folha-grade">
            <Campo rotulo="Nome" valor={doc.comprador_nome} onChange={v => mudar('comprador_nome', v)} alerta={alertaFat} />
            <Campo rotulo="CNPJ" valor={doc.comprador_cnpj} onChange={v => mudar('comprador_cnpj', v.replace(/\D/g, '').slice(0, 14))} alerta={alertaFat} />
            <Campo rotulo="I.E." valor={doc.faturamento_ie} onChange={v => mudar('faturamento_ie', v)} />
            <Campo rotulo="Endereço" valor={doc.faturamento_endereco} onChange={v => mudar('faturamento_endereco', v)} largo />
          </div>

          <h4>DADOS DO FORNECEDOR</h4>
          <div className="adm-folha-grade">
            <Campo rotulo="Nome" valor={doc.fornecedor_nome} onChange={v => mudar('fornecedor_nome', v)} alerta={alertaForn} />
            <Campo rotulo="CNPJ" valor={doc.fornecedor_cnpj} onChange={v => mudar('fornecedor_cnpj', v.replace(/\D/g, '').slice(0, 14))} alerta={alertaForn} />
            <Campo rotulo="Telefone" valor={doc.fornecedor_telefone} onChange={v => mudar('fornecedor_telefone', v)} />
            <Campo rotulo="Vendedor" valor={doc.fornecedor_vendedor} onChange={v => mudar('fornecedor_vendedor', v)} />
            <Campo rotulo="E-mail" valor={doc.fornecedor_email} onChange={v => mudar('fornecedor_email', v)} />
            <Campo rotulo="Endereço" valor={doc.fornecedor_endereco} onChange={v => mudar('fornecedor_endereco', v)} largo />
          </div>

          <div className="adm-folha-grade">
            <Campo rotulo="Obra / centro de custo" valor={doc.obra_centro_custo} onChange={v => mudar('obra_centro_custo', v)} alerta={alertaFat} />
            <Campo rotulo="CNO" valor={doc.cno} onChange={v => mudar('cno', v)} />
            <Campo rotulo="Endereço entrega" valor={doc.endereco_entrega} onChange={v => mudar('endereco_entrega', v)} />
            <Campo rotulo="Recebedor" valor={doc.recebedor} onChange={v => mudar('recebedor', v)} />
            <Campo rotulo="Endereço cobrança" valor={doc.endereco_cobranca} onChange={v => mudar('endereco_cobranca', v)} largo />
          </div>

          <h4>ITENS</h4>
          <table className="adm-folha-itens">
            <thead>
              <tr>
                <th>#</th>
                <th>Descrição</th>
                <th>Qtd.</th>
                <th>Un.</th>
                <th>Unit. (R$)</th>
                <th>Desc. (R$)</th>
                <th>Total (R$)</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {doc.itens.map((it, i) => (
                <tr key={i}>
                  <td>{i + 1}</td>
                  <td>
                    <input value={it.descricao} onChange={e => mudarItem(i, 'descricao', e.target.value)} />
                  </td>
                  <td>
                    <input inputMode="decimal" value={it.qtd} onChange={e => mudarItem(i, 'qtd', num(e.target.value))} />
                  </td>
                  <td>
                    <input value={it.unidade} onChange={e => mudarItem(i, 'unidade', e.target.value)} />
                  </td>
                  <td>
                    <input inputMode="decimal" value={it.valor_unit} onChange={e => mudarItem(i, 'valor_unit', num(e.target.value))} />
                  </td>
                  <td>
                    <input inputMode="decimal" value={it.desconto} onChange={e => mudarItem(i, 'desconto', num(e.target.value))} />
                  </td>
                  <td className="adm-num">{it.total.toLocaleString('pt-BR', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</td>
                  <td>
                    <button type="button" className="bt bt-mini" onClick={() => apagarItem(i)} disabled={doc.itens.length <= 1}>×</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <button type="button" className="bt bt-neutro adm-add-item" onClick={addItem}>+ item</button>

          <div className="adm-folha-totais">
            <label className="adm-campo">
              <span>Frete (R$)</span>
              <input inputMode="decimal" value={doc.frete} onChange={e => mudar('frete', num(e.target.value))} />
            </label>
            <p>Subtotal <b>{emBR(doc.subtotal)}</b></p>
            <p>Desconto <b>{emBR(doc.desconto)}</b></p>
            <p>Total <b>{emBR(doc.total)}</b></p>
          </div>
        </div>
      </div>
    </div>
  )
}

function Campo({ rotulo, valor, onChange, largo, alerta }: {
  rotulo: string
  valor: string
  onChange: (v: string) => void
  largo?: boolean
  alerta?: boolean
}) {
  return (
    <label className={'adm-campo' + (largo ? ' adm-campo--largo' : '') + (alerta ? ' adm-campo--alerta' : '')}>
      <span>{rotulo}</span>
      <input value={valor} onChange={e => onChange(e.target.value)} />
    </label>
  )
}

function num(s: string): number {
  const v = Number(String(s).replace(',', '.'))
  return Number.isFinite(v) ? v : 0
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

function emBR(n: number): string {
  return n.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}
