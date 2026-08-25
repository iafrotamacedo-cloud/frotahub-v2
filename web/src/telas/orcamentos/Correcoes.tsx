// rev 1 — Correções (2.4), as quatro frentes
//
// A ORDEM DA TELA "SEM ASSOCIAÇÃO" É O CORAÇÃO DESTE ARQUIVO
//
//	A primeira versão abria pedindo o número certo do ticket. O dono corrigiu:
//	90% das vezes o ticket está CERTO na nossa base e ERRADO na nota — o
//	fornecedor escreveu 13O328 com a letra O, ou inverteu dois dígitos. O robô
//	roda a cada duas horas e não atendemos ticket anterior à data de corte, então
//	"não está na base" é o caso raro, não o comum.
//
//	Por isso a tela sugere candidatos ANTES de pedir qualquer coisa: o usuário
//	confere e confirma. Digitar fica para quando nenhuma sugestão serve.
import { useCallback, useEffect, useState } from 'react'
import { motor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta } from './Arquivos'
import {
  emReais, emDataHora, contaPorExtenso,
  type Correcoes as Dados, type Candidato,
} from './tipos'

const FRENTES = [
  { chave: 'sem-ticket', titulo: 'Sem ticket', desc: 'Notas lidas em que nenhum ticket foi encontrado' },
  { chave: 'sem-associacao', titulo: 'Sem associação', desc: 'Tickets escritos que não batem com a nossa base' },
  { chave: 'nao-lancados', titulo: 'Não lançados', desc: 'Orçamentos gerados que ainda não foram para o Trílogo' },
  { chave: 'apagados', titulo: 'Apagados', desc: 'O que foi excluído — e pode voltar' },
]

export function Correcoes({ frente, voltar }: { frente?: string; voltar: () => void }) {
  const [dados, setDados] = useState<Dados | null>(null)
  const [qual, setQual] = useState(frente ?? 'sem-ticket')
  const [erro, setErro] = useState('')


  const carregar = useCallback(async () => {
    try {
      setDados(await motor<Dados>('/orcamentos/correcoes'))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar as correções.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  const conta = (c: string) => {
    if (!dados) return 0
    switch (c) {
      case 'sem-ticket': return dados.sem_ticket.length
      case 'sem-associacao': return dados.sem_associacao.length
      case 'nao-lancados': return dados.nao_lancados.length
      default: return dados.apagados.length
    }
  }

  return (
    <div className="orc-tela">
      <BarraDeVolta voltar={voltar} titulo="Correções" />

      <div className="orc-abas">
        {FRENTES.map(f => (
          <button
            key={f.chave}
            type="button"
            className={'orc-aba' + (qual === f.chave ? ' ligada' : '')}
            onClick={() => setQual(f.chave)}
          >
            <b>{conta(f.chave)}</b>
            <span>{f.titulo}</span>
          </button>
        ))}
      </div>

      {erro && <p className="erro">{erro}</p>}
      {!dados ? <Carregando /> : (
        <div className="orc-lista">
          <div className="orc-lista-cab">
            <h2>{FRENTES.find(f => f.chave === qual)?.titulo}</h2>
            <em>{FRENTES.find(f => f.chave === qual)?.desc}</em>
          </div>

          {qual === 'sem-ticket' && <SemTicket dados={dados} recarregar={carregar} />}
          {qual === 'sem-associacao' && <SemAssociacao dados={dados} recarregar={carregar} />}
          {qual === 'nao-lancados' && <NaoLancados dados={dados} />}
          {qual === 'apagados' && <Apagados dados={dados} recarregar={carregar} />}
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// 2.4.1 — sem ticket
// ---------------------------------------------------------------------------

function SemTicket({ dados, recarregar }: { dados: Dados; recarregar: () => Promise<void> }) {
  const [erro, setErro] = useState('')

  async function amarrar(id: string) {
    const t = window.prompt('Qual o número do ticket desta nota?')
    if (!t) return
    const n = Number(t.replace(/\D/g, ''))
    if (!(n >= 10000 && n <= 999999)) { setErro('O ticket tem 5 ou 6 dígitos.'); return }
    try {
      await motor(`/orcamentos/documentos/${id}/tickets`, { metodo: 'POST', corpo: { tickets: [n] } })
      await recarregar()
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui amarrar.')
    }
  }

  if (!dados.sem_ticket.length) {
    return <p className="orc-vazio grande">Nenhuma nota órfã. Toda nota lida achou o seu ticket.</p>
  }
  return (
    <>
      {erro && <p className="erro" style={{ margin: '10px 16px' }}>{erro}</p>}
      <div className="orc-rolagem">
        <table className="orc-tabela">
          <thead>
            <tr><th>Arquivo</th><th>Nota</th><th style={{ textAlign: 'right' }}>Valor</th><th>Inserida em</th><th style={{ textAlign: 'right' }}>Ações</th></tr>
          </thead>
          <tbody>
            {dados.sem_ticket.map(d => (
              <tr key={d.id}>
                <td>
                  <span className="orc-nome">{d.nome_arquivo}</span>
                  {d.emitente_nome && <span className="orc-detalhe">{d.emitente_nome}</span>}
                </td>
                <td>{d.numero ?? d.dav_numero ?? '–'}</td>
                <td style={{ textAlign: 'right' }}>{emReais(d.valor_total)}</td>
                <td>{emDataHora(d.inserido_em)}</td>
                <td className="orc-acoes">
                  <button type="button" className="forte" onClick={() => void amarrar(d.id)}>informar o ticket</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}

// ---------------------------------------------------------------------------
// 2.4.2 — sem associação
// ---------------------------------------------------------------------------

function SemAssociacao({ dados, recarregar }: { dados: Dados; recarregar: () => Promise<void> }) {
  const [abrindo, setAbrindo] = useState<{ documento: string; ticket: number } | null>(null)

  if (!dados.sem_associacao.length) {
    return <p className="orc-vazio grande">Todos os tickets escritos casaram com a nossa base.</p>
  }
  return (
    <>
      <div className="orc-rolagem">
        <table className="orc-tabela">
          <thead>
            <tr><th>Ticket escrito</th><th>Arquivo</th><th>Nota</th><th style={{ textAlign: 'right' }}>Valor</th><th style={{ textAlign: 'right' }}>Ações</th></tr>
          </thead>
          <tbody>
            {dados.sem_associacao.map(l => (
              <tr key={l.id}>
                <td><span className="orc-nome">{l.ticket}</span><span className="orc-detalhe">não existe na nossa base</span></td>
                <td>{l.documentos.nome_arquivo}</td>
                <td>{l.documentos.numero ?? '–'}</td>
                <td style={{ textAlign: 'right' }}>{emReais(l.documentos.valor_total)}</td>
                <td className="orc-acoes">
                  <button type="button" className="forte"
                    onClick={() => setAbrindo({ documento: l.documento_id, ticket: l.ticket })}>
                    procurar o certo
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {abrindo && (
        <Candidatos
          documento={abrindo.documento}
          ticket={abrindo.ticket}
          fechar={() => setAbrindo(null)}
          concluir={async () => { setAbrindo(null); await recarregar() }}
        />
      )}
    </>
  )
}

/** A janela de sugestões — o coração da correção por semelhança. */
function Candidatos({ documento, ticket, fechar, concluir }: {
  documento: string
  ticket: number
  fechar: () => void
  concluir: () => Promise<void>
}) {
  const [lista, setLista] = useState<Candidato[] | null>(null)
  const [dica, setDica] = useState('')
  const [erro, setErro] = useState('')
  const [manual, setManual] = useState('')

  useEffect(() => {
    void (async () => {
      try {
        const r = await motor<{ candidatos: Candidato[]; dica: string }>(
          '/orcamentos/correcoes/candidatos?ticket=' + ticket)
        setLista(r.candidatos)
        setDica(r.dica)
      } catch (e) {
        setErro(e instanceof Error ? e.message : 'Não consegui buscar sugestões.')
      }
    })()
  }, [ticket])

  async function corrigir(para: number) {
    try {
      await motor(`/orcamentos/documentos/${documento}/tickets`, {
        metodo: 'PATCH', corpo: { de: ticket, para },
      })
      await concluir()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui corrigir.')
    }
  }

  return (
    <div className="orc-janela" role="dialog" aria-modal>
      <div className="orc-janela-caixa">
        <h3>O fornecedor escreveu <b>{ticket}</b></h3>
        <p className="orc-dica">{dica}</p>

        {erro && <p className="erro">{erro}</p>}
        {!lista ? <Carregando /> : (
          <div className="orc-candidatos">
            {lista.map(c => (
              <button key={c.numero} type="button" className="orc-candidato" onClick={() => void corrigir(c.numero)}>
                <b>{c.numero}</b>
                <span className="loja">{c.loja} · {contaPorExtenso(c.conta)}</span>
                <span className="desc">{c.descricao}</span>
                <span className="porque">{c.porque} · aberto em {emDataHora(c.criado_em)}</span>
              </button>
            ))}
            {lista.length === 0 && <p className="orc-vazio">Nenhum chamado parecido.</p>}
          </div>
        )}

        <div className="orc-manual">
          <span>Nenhum destes?</span>
          <input
            inputMode="numeric"
            placeholder="digitar o número certo"
            value={manual}
            maxLength={6}
            onChange={e => setManual(e.target.value.replace(/\D/g, ''))}
          />
          <button type="button" className="orc-bt"
            disabled={!(Number(manual) >= 10000)}
            onClick={() => void corrigir(Number(manual))}>corrigir</button>
          <button type="button" className="orc-bt" onClick={fechar}>fechar</button>
        </div>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// 2.4.3 e 2.4.4
// ---------------------------------------------------------------------------

function NaoLancados({ dados }: { dados: Dados }) {
  if (!dados.nao_lancados.length) {
    return <p className="orc-vazio grande">Nada parado: todo orçamento gerado já está no Trílogo.</p>
  }
  return (
    <div className="orc-rolagem">
      <table className="orc-tabela">
        <thead>
          <tr><th>Ticket</th><th>Loja</th><th>Nota</th><th style={{ textAlign: 'right' }}>Valor</th><th>Gerado em</th></tr>
        </thead>
        <tbody>
          {dados.nao_lancados.map(o => (
            <tr key={o.id}>
              <td><span className="orc-nome">{o.ticket}{o.parte > 1 ? `-${o.parte}` : ''}</span></td>
              <td>{o.loja ?? '–'}</td>
              <td>{o.notas ?? '–'}</td>
              <td style={{ textAlign: 'right' }}>{emReais(o.valor)}</td>
              <td>{emDataHora(o.criado_em)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function Apagados({ dados, recarregar }: { dados: Dados; recarregar: () => Promise<void> }) {
  const [erro, setErro] = useState('')

  async function restaurar(id: string) {
    try {
      await motor(`/orcamentos/ficha/${id}/restaurar`, { metodo: 'POST' })
      await recarregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui restaurar.')
    }
  }

  if (!dados.apagados.length) {
    return <p className="orc-vazio grande">Nenhum orçamento apagado.</p>
  }
  return (
    <>
      {erro && <p className="erro" style={{ margin: '10px 16px' }}>{erro}</p>}
      <div className="orc-rolagem">
        <table className="orc-tabela">
          <thead>
            <tr><th>Ticket</th><th>Loja</th><th style={{ textAlign: 'right' }}>Valor</th><th>Gerado em</th><th style={{ textAlign: 'right' }}>Ações</th></tr>
          </thead>
          <tbody>
            {dados.apagados.map(o => (
              <tr key={o.id}>
                <td><span className="orc-nome">{o.ticket}{o.parte > 1 ? `-${o.parte}` : ''}</span></td>
                <td>{o.loja ?? '–'}</td>
                <td style={{ textAlign: 'right' }}>{emReais(o.valor)}</td>
                <td>{emDataHora(o.criado_em)}</td>
                <td className="orc-acoes">
                  <button type="button" className="forte" onClick={() => void restaurar(o.id)}>restaurar</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}
