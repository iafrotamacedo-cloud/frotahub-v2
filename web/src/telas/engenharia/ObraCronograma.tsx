// rev 1 — a EAP de uma obra
//
// SEM CPM AINDA — datas e duração são digitadas à mão; folga e caminho
// crítico ficam para quando a Fase 3 chegar no motor.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import type { Cronograma, Dependencia, EAPNo, Obra } from './tipos'
import { STATUS_EAP, labelNivel } from './tipos'

export function ObraCronograma({ obraId, voltar }: { obraId: string; voltar: () => void }) {
  const [obra, setObra] = useState<Obra | null>(null)
  const [cronograma, setCronograma] = useState<Cronograma | null>(null)
  const [arvore, setArvore] = useState<EAPNo[]>([])
  const [deps, setDeps] = useState<Dependencia[]>([])
  const [erro, setErro] = useState<string | null>(null)
  const [carregando, setCarregando] = useState(true)
  const [selecionado, setSelecionado] = useState<EAPNo | null>(null)
  const [editando, setEditando] = useState(false)

  const recarregar = useCallback(async () => {
    setErro(null)
    try {
      const o = await motor<Obra>(`/obras/${obraId}`)
      setObra(o)
      let c: Cronograma
      try {
        c = await motor<Cronograma>(`/obras/${obraId}/cronograma-atual`)
      } catch {
        c = await motor<Cronograma>(`/obras/${obraId}/cronogramas`, { metodo: 'POST', corpo: { tipo: 'atual' } })
      }
      setCronograma(c)
      const nos = await motor<EAPNo[]>(`/cronogramas/${c.id}/eap?formato=arvore`)
      setArvore(nos)
      const d = await motor<{ dependencias: Dependencia[] }>(`/cronogramas/${c.id}/dependencias`)
      setDeps(d.dependencias)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o cronograma.')
    } finally {
      setCarregando(false)
    }
  }, [obraId])

  useEffect(() => { void recarregar() }, [recarregar])

  async function adicionarNo(paiId?: string) {
    if (!cronograma) return
    const nome = window.prompt('Nome do item da EAP:')
    if (!nome?.trim()) return
    try {
      await motor(`/cronogramas/${cronograma.id}/eap`, { metodo: 'POST', corpo: { nome: nome.trim(), pai_id: paiId ?? null } })
      await recarregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui criar o item.')
    }
  }

  async function salvarNo(no: EAPNo, dados: Partial<EAPNo>) {
    if (!cronograma) return
    try {
      const resposta = await motor(`/eap/${no.id}`, {
        metodo: 'PATCH',
        corpo: {
          nome: dados.nome ?? no.nome,
          descricao: dados.descricao ?? no.descricao ?? null,
          duracao_dias: dados.duracao_dias ?? no.duracao_dias ?? null,
          data_inicio_prevista: dados.data_inicio_prevista ?? no.data_inicio_prevista ?? null,
          data_fim_prevista: dados.data_fim_prevista ?? no.data_fim_prevista ?? null,
          is_marco: dados.is_marco ?? no.is_marco,
          status: dados.status ?? no.status,
        },
      })
      setEditando(false)
      const aviso = avisoDe(resposta)
      if (aviso) setErro(aviso)
      await recarregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar o item.')
    }
  }

  async function adicionarDependencia() {
    if (!cronograma || !selecionado) return
    const pred = window.prompt('ID do predecessor (cole da tabela de dependências abaixo):')
    if (!pred?.trim()) return
    const tipo = window.prompt('Tipo (FS, SS, FF ou SF):', 'FS') ?? 'FS'
    try {
      await motor(`/cronogramas/${cronograma.id}/dependencias`, {
        metodo: 'POST',
        corpo: { predecessor_id: pred.trim(), sucessor_id: selecionado.id, tipo },
      })
      await recarregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui criar a dependência.')
    }
  }

  async function removerDep(id: string) {
    try {
      await motor(`/dependencias/${id}`, { metodo: 'DELETE' })
      await recarregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui remover a dependência.')
    }
  }

  if (carregando) return <Carregando texto="Carregando o cronograma..." />

  return (
    <>
      <p className="muted" style={{ margin: '0 0 8px' }}>
        <button type="button" className="linkbtn" onClick={voltar}>← Obras</button>
      </p>
      <h1 className="h1">{obra?.nome ?? 'Cronograma'}</h1>
      <p className="sub">
        {obra?.codigo && <span className="pill" style={{ marginRight: 8 }}>{obra.codigo}</span>}
        Cronograma v{cronograma?.versao ?? '—'} ({cronograma?.tipo ?? '…'}) · datas manuais — CPM ainda não calculado
      </p>

      {erro && <div className="erro-caixa">{erro}</div>}

      <div style={{ display: 'flex', gap: 10, marginBottom: 16, flexWrap: 'wrap' }}>
        <button className="bt bt-neutro" type="button" onClick={() => adicionarNo()}>+ Grupo (raiz)</button>
        {selecionado && (
          <>
            <button className="bt bt-neutro" type="button" onClick={() => adicionarNo(selecionado.id)}>
              + Filho de {selecionado.codigo_eap}
            </button>
            <button className="bt bt-neutro" type="button" onClick={() => setEditando(true)}>Editar item</button>
            <button className="bt bt-neutro" type="button" onClick={adicionarDependencia}>+ Dependência → este</button>
          </>
        )}
      </div>

      <div className="eap-layout">
        <div className="cartao eap-arvore">
          <h3 style={{ marginTop: 0 }}>EAP</h3>
          {arvore.length === 0 ? (
            <p className="muted">Árvore vazia — adicione um grupo para começar.</p>
          ) : (
            <ul className="eap-lista">
              {arvore.map(n => (
                <NoArvore key={n.id} no={n} selecionadoId={selecionado?.id} onSelect={setSelecionado} profundidade={0} />
              ))}
            </ul>
          )}
        </div>

        <div className="cartao eap-detalhe">
          {selecionado ? (
            editando ? (
              <FormNo no={selecionado} onSalvar={d => salvarNo(selecionado, d)} onCancelar={() => setEditando(false)} />
            ) : (
              <DetalheNo no={selecionado} />
            )
          ) : (
            <p className="muted">Selecione um item da EAP para ver os detalhes.</p>
          )}

          {deps.length > 0 && (
            <>
              <h3>Dependências</h3>
              <table className="tabela">
                <thead><tr><th>De → Para</th><th>Tipo</th><th>Lag</th><th></th></tr></thead>
                <tbody>
                  {deps.map(d => (
                    <tr key={d.id}>
                      <td style={{ fontSize: 12 }}>{d.predecessor_id.slice(0, 8)}… → {d.sucessor_id.slice(0, 8)}…</td>
                      <td>{d.tipo}</td>
                      <td>{d.lag_dias}d</td>
                      <td><button className="linkbtn" type="button" onClick={() => removerDep(d.id)}>remover</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}
        </div>
      </div>
    </>
  )
}

function NoArvore({
  no, selecionadoId, onSelect, profundidade,
}: { no: EAPNo; selecionadoId?: string; onSelect: (n: EAPNo) => void; profundidade: number }) {
  return (
    <li>
      <button
        type="button"
        className={'eap-no' + (selecionadoId === no.id ? ' on' : '')}
        style={{ paddingLeft: 12 + profundidade * 16 }}
        onClick={() => onSelect(no)}
      >
        <span className="eap-codigo">{no.codigo_eap}</span>
        <span className="eap-nome">{no.nome}</span>
        <span className="eap-meta">{labelNivel(no.nivel)}</span>
        {no.is_marco && <span className="pill" style={{ fontSize: 9, marginLeft: 6 }}>marco</span>}
        {no.duracao_dias != null && <span className="eap-dur">{no.duracao_dias}d</span>}
      </button>
      {no.filhos && no.filhos.length > 0 && (
        <ul className="eap-lista">
          {no.filhos.map(f => (
            <NoArvore key={f.id} no={f} selecionadoId={selecionadoId} onSelect={onSelect} profundidade={profundidade + 1} />
          ))}
        </ul>
      )}
    </li>
  )
}

function DetalheNo({ no }: { no: EAPNo }) {
  return (
    <>
      <h3 style={{ marginTop: 0 }}>{no.codigo_eap} · {no.nome}</h3>
      <p className="muted">{labelNivel(no.nivel)} · {STATUS_EAP[no.status] ?? no.status}</p>
      {no.descricao && <p>{no.descricao}</p>}
      <dl className="eap-dl">
        <dt>Início previsto</dt><dd>{no.data_inicio_prevista ?? '—'}</dd>
        <dt>Fim previsto</dt><dd>{no.data_fim_prevista ?? '—'}</dd>
        <dt>Duração</dt><dd>{no.duracao_dias != null ? `${no.duracao_dias} dias` : '—'}</dd>
        <dt>Marco</dt><dd>{no.is_marco ? 'Sim' : 'Não'}</dd>
      </dl>
      <p className="muted" style={{ fontSize: 11 }}>ID: {no.id}</p>
    </>
  )
}

function FormNo({
  no, onSalvar, onCancelar,
}: { no: EAPNo; onSalvar: (d: Partial<EAPNo>) => void; onCancelar: () => void }) {
  const [nome, setNome] = useState(no.nome)
  const [duracao, setDuracao] = useState(no.duracao_dias?.toString() ?? '')
  const [inicio, setInicio] = useState(no.data_inicio_prevista ?? '')
  const [fim, setFim] = useState(no.data_fim_prevista ?? '')
  const [marco, setMarco] = useState(no.is_marco)
  const [status, setStatus] = useState(no.status)

  return (
    <form onSubmit={e => {
      e.preventDefault()
      onSalvar({
        nome,
        duracao_dias: duracao ? Number(duracao) : null,
        data_inicio_prevista: inicio || null,
        data_fim_prevista: fim || null,
        is_marco: marco,
        status,
      })
    }}>
      <h3 style={{ marginTop: 0 }}>Editar {no.codigo_eap}</h3>
      <label htmlFor="eap-nome">Nome</label>
      <input id="eap-nome" value={nome} onChange={e => setNome(e.target.value)} required />
      <label htmlFor="eap-duracao">Duração (dias)</label>
      <input id="eap-duracao" type="number" min={0} step={0.5} value={duracao} onChange={e => setDuracao(e.target.value)} />
      <label htmlFor="eap-inicio">Início previsto</label>
      <input id="eap-inicio" type="date" value={inicio} onChange={e => setInicio(e.target.value)} />
      <label htmlFor="eap-fim">Fim previsto</label>
      <input id="eap-fim" type="date" value={fim} onChange={e => setFim(e.target.value)} />
      <label htmlFor="eap-status">Status</label>
      <select id="eap-status" value={status} onChange={e => setStatus(e.target.value)}>
        {Object.entries(STATUS_EAP).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
      </select>
      <label style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 12 }}>
        <input type="checkbox" checked={marco} onChange={e => setMarco(e.target.checked)} />
        É marco (milestone)
      </label>
      <div style={{ display: 'flex', gap: 8, marginTop: 14 }}>
        <button className="bt bt-forte" type="submit">Salvar</button>
        <button className="bt bt-neutro" type="button" onClick={onCancelar}>Cancelar</button>
      </div>
    </form>
  )
}
