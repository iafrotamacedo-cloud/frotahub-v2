// rev 1 — Engenharia > Obras
//
// SEM REACT-ROUTER, DE PROPÓSITO
//
//	O resto do FrotaHub navega pelo endereço via `menu/navegacao.ts` (caminho +
//	`extra[]`), não por uma biblioteca de rotas — é o mesmo desenho de
//	`DadosTrilogo` (ticket aberto = `extra[0]`). Aqui, obra aberta = `obraId`
//	vindo de `extra[0]`: sem obra, mostra a lista; com obra, mostra o
//	cronograma dela. Uma dependência de rota a menos no front inteiro.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import { Janela } from '../../componentes/Janela'
import { Carregando } from '../../componentes/Carregando'
import { ObraCronograma } from './ObraCronograma'
import type { Obra, Contratante } from './tipos'
import { STATUS_OBRA } from './tipos'

interface Props {
  obraId?: string
  abrir: (obraId: string) => void
  voltar: () => void
}

export function Obras({ obraId, abrir, voltar }: Props) {
  if (obraId) {
    return <ObraCronograma obraId={obraId} voltar={voltar} />
  }
  return <ListaDeObras abrir={abrir} />
}

function ListaDeObras({ abrir }: { abrir: (obraId: string) => void }) {
  const [obras, setObras] = useState<Obra[] | null>(null)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [formAberto, setFormAberto] = useState(false)

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const r = await motor<{ obras: Obra[] }>('/obras')
      setObras(r.obras)
    } catch (e) {
      setObras([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar as obras.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Obras</h1>
          <p>Cronograma e EAP — estrutura analítica do projeto.</p>
        </div>
        <button className="bt bt-forte" type="button" onClick={() => setFormAberto(true)}>
          Nova obra
        </button>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      {obras === null ? (
        <Carregando texto="Carregando as obras..." />
      ) : erro ? null : obras.length === 0 ? (
        <div className="vazio">Nenhuma obra cadastrada ainda.</div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr><th>Código</th><th>Obra</th><th>Contratante</th><th>Situação</th><th></th></tr>
            </thead>
            <tbody>
              {obras.map(o => (
                <tr key={o.id}>
                  <td><code>{o.codigo ?? '—'}</code></td>
                  <td>{o.nome}</td>
                  <td>{o.cliente_contratante?.nome ?? '—'}</td>
                  <td><span className={'pino ' + (o.status === 'em_execucao' ? 'pino-ok' : 'pino-off')}>{STATUS_OBRA[o.status] ?? o.status}</span></td>
                  <td className="acoes">
                    <button type="button" className="bt bt-mini" onClick={() => abrir(o.id)}>Cronograma</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {formAberto && (
        <FormObra
          aoFechar={() => setFormAberto(false)}
          aoSalvar={aviso => {
            setFormAberto(false)
            setRecado(aviso ?? 'Obra cadastrada.')
            void carregar()
          }}
        />
      )}
    </>
  )
}

function FormObra({ aoFechar, aoSalvar }: { aoFechar: () => void; aoSalvar: (aviso: string | null) => void }) {
  const [nome, setNome] = useState('')
  const [codigo, setCodigo] = useState('')
  const [contratanteId, setContratanteId] = useState('')
  const [contratantes, setContratantes] = useState<Contratante[]>([])
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  useEffect(() => {
    let vivo = true
    motor<{ contratantes: Contratante[] }>('/contratantes')
      .then(r => { if (vivo) setContratantes(r.contratantes) })
      .catch(() => { /* formulário funciona sem contratante escolhido */ })
    return () => { vivo = false }
  }, [])

  async function enviar(e: React.FormEvent) {
    e.preventDefault()
    setErro(null)
    setSalvando(true)
    try {
      const resposta = await motor('/obras', {
        metodo: 'POST',
        corpo: {
          nome: nome.trim(),
          codigo: codigo.trim() || null,
          cliente_contratante_id: contratanteId || null,
        },
      })
      aoSalvar(avisoDe(resposta))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui cadastrar a obra.')
      setSalvando(false)
    }
  }

  return (
    <Janela titulo="Nova obra" descricao="Os dados de cronograma entram depois, na EAP." aoFechar={aoFechar}>
      <form className="jn-corpo" onSubmit={enviar}>
        <label htmlFor="ob-codigo">Código (opcional)</label>
        <input id="ob-codigo" value={codigo} onChange={e => setCodigo(e.target.value)} placeholder="OB-2026-014" />

        <label htmlFor="ob-nome">Nome da obra</label>
        <input id="ob-nome" value={nome} onChange={e => setNome(e.target.value)} placeholder="Reforma loja Centro" />

        <label htmlFor="ob-contratante">Contratante (opcional)</label>
        <select id="ob-contratante" value={contratanteId} onChange={e => setContratanteId(e.target.value)}>
          <option value="">— sem contratante definido —</option>
          {contratantes.map(c => (
            <option key={c.id} value={c.id}>{c.nome}</option>
          ))}
        </select>

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando || !nome.trim()}>
            {salvando ? 'Salvando...' : 'Criar obra'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
