// rev 2 — Correções (2.4), as quatro frentes
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
import { VisorDaNota, type Acoes } from './VisorDaNota'
import { ConfirmarComSenha } from './ConfirmarComSenha'
import {
  emReais, emDataHora,
  type Correcoes as Dados, type Bloqueio,
  type Documento, type Conferencia, type Desconto,
} from './tipos'

const FRENTES = [
  { chave: 'sem-ticket', titulo: 'Sem ticket', desc: 'Notas lidas em que nenhum ticket foi encontrado' },
  { chave: 'sem-associacao', titulo: 'Sem associação', desc: 'Tickets escritos que não batem com a nossa base' },
  { chave: 'recusados', titulo: 'Recusados', desc: 'O Trílogo não aceitou o lançamento — e por quê' },
  { chave: 'extrapoladas', titulo: 'Passam do teto', desc: 'Notas que não cabem no limite do ticket — e o que dá para fazer' },
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
      case 'extrapoladas': return dados.extrapoladas?.length ?? 0
      case 'recusados': return dados.recusados.length
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
          {qual === 'extrapoladas' && <Extrapoladas dados={dados} recarregar={carregar} />}
          {qual === 'recusados' && <Recusados dados={dados} />}
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
  const [abrindo, setAbrindo] = useState<Documento | null>(null)
  const [erro, setErro] = useState('')

  const acoes = (d: Documento): Acoes => ({
    // "Tira a nota de todas as filas" — e dá para restaurar. Decisão do dono:
    // conclusão sobre nota não pode ser irreversível por um clique.
    excluir: async () => {
      await motor(`/orcamentos/documentos/${d.id}`, { metodo: 'DELETE' })
      setAbrindo(null); await recarregar()
    },
    rateada: async () => {
      await motor(`/orcamentos/documentos/${d.id}/fila`,
        { metodo: 'POST', corpo: { fila: 'rateio' } })
      setAbrindo(null); await recarregar()
    },
    concluir: async () => { setAbrindo(null); await recarregar() },
    fechar: () => setAbrindo(null),
  })

  if (abrindo) {
    return (
      <VisorDaNota
        documento={abrindo.id}
        nome={abrindo.nome_arquivo}
        valor={abrindo.valor_total}
        tickets={abrindo.ticket_numeros ?? []}
        modo="sem-ticket"
        acoes={acoes(abrindo)}
      />
    )
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
                  <button type="button" className="forte" onClick={() => { setErro(''); setAbrindo(d) }}>
                    inserir ticket
                  </button>
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

// O REPROCESSAR SÓ EXISTE DEPOIS DE UM ATUALIZAR, E MORRE DEPOIS DE CADA USO
//
//	Pedido do dono, e a razão é boa: sem essa trava, alguém corrige uma nota,
//	clica reprocessar, e o sistema tenta gerar as outras trinta e nove que
//	continuam travadas — gastando tempo e enchendo a tela de erro que já se
//	sabia de antemão.
//
//	"Verde" é uma foto de um instante. Reprocessar duas vezes seguidas com a
//	mesma foto é agir sobre um estado que já mudou. Por isso o botão trava de
//	novo assim que roda: a única forma de saber o estado é olhar de novo.
function SemAssociacao({ dados, recarregar }: { dados: Dados; recarregar: () => Promise<void> }) {
  const [abrindo, setAbrindo] = useState<{ documento: string; nome: string; valor: number | null; tickets: number[] } | null>(null)
  const [conf, setConf] = useState<Record<string, Conferencia>>({})
  const [atualizando, setAtualizando] = useState(false)
  const [processando, setProcessando] = useState(false)
  const [podeReprocessar, setPodeReprocessar] = useState(false)
  const [recado, setRecado] = useState('')
  const [erro, setErro] = useState('')

  // Uma linha por NOTA, e não por ticket: a correção é da nota, e uma nota com
  // três tickets soltos apareceria três vezes pedindo o mesmo trabalho.
  const notas = agruparPorNota(dados.sem_associacao)

  async function atualizar() {
    setAtualizando(true); setErro(''); setRecado('')
    // TENTA O TRÍLOGO; SE ESTIVER OCUPADO, CONFERE COM O QUE JÁ TEMOS
    //   Escolha do dono. Uma leitura em andamento não pode impedir a pessoa de
    //   ver o estado — e dizer de quando são os dados é mais honesto que fingir
    //   que acabaram de chegar.
    try {
      await motor('/robos/trilogo/atualizacao', { metodo: 'POST' })
      setRecado('Trílogo relido agora.')
    } catch (e) {
      const msg = e instanceof Error ? e.message : ''
      setRecado(/andamento|conflito|409/i.test(msg)
        ? 'Já havia uma leitura do Trílogo em andamento — conferi com os chamados que já estavam aqui.'
        : 'Não consegui reler o Trílogo agora — conferi com os chamados que já estavam aqui.')
    }
    try {
      const r = await motor<{ notas: Conferencia[] }>('/orcamentos/correcoes/conferir',
        { metodo: 'POST', corpo: { documentos: notas.map(n => n.documento) } })
      const mapa: Record<string, Conferencia> = {}
      for (const c of r.notas) mapa[c.documento] = c
      setConf(mapa)
      setPodeReprocessar(r.notas.some(c => c.pronta))
      await recarregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui conferir as notas.')
      setPodeReprocessar(false)
    } finally { setAtualizando(false) }
  }

  async function reprocessar() {
    const verdes = Object.values(conf).filter(c => c.pronta).map(c => c.documento)
    if (!verdes.length) return
    setProcessando(true); setErro('')
    try {
      await motor('/orcamentos/gerar', { metodo: 'POST', corpo: { documentos: verdes } })
      setRecado(`${verdes.length} nota${verdes.length > 1 ? 's' : ''} processada${verdes.length > 1 ? 's' : ''}.`)
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui reprocessar.')
    } finally {
      // Trava de novo, sempre — inclusive quando deu errado. O estado mudou de
      // qualquer jeito, e a foto anterior deixou de valer.
      setPodeReprocessar(false)
      setConf({})
      setProcessando(false)
      await recarregar()
    }
  }

  const acoes = (documento: string): Acoes => ({
    excluir: async () => {
      await motor(`/orcamentos/documentos/${documento}`, { metodo: 'DELETE' })
      setAbrindo(null); setPodeReprocessar(false); await recarregar()
    },
    rateada: async () => {
      await motor(`/orcamentos/documentos/${documento}/fila`,
        { metodo: 'POST', corpo: { fila: 'rateio' } })
      setAbrindo(null); setPodeReprocessar(false); await recarregar()
    },
    // Mexer numa nota invalida a foto: quem corrigiu precisa atualizar de novo
    // para o reprocessar voltar a valer.
    concluir: async () => { setAbrindo(null); setPodeReprocessar(false); await recarregar() },
    fechar: () => setAbrindo(null),
  })

  if (abrindo) {
    return (
      <VisorDaNota
        documento={abrindo.documento} nome={abrindo.nome} valor={abrindo.valor}
        tickets={abrindo.tickets} modo="sem-associacao" acoes={acoes(abrindo.documento)}
      />
    )
  }

  if (!notas.length) {
    return <p className="orc-vazio grande">Todos os tickets escritos casaram com a nossa base.</p>
  }
  return (
    <>
      <div className="orc-barra-acoes">
        <button type="button" className="forte" disabled={atualizando || processando}
          onClick={() => void atualizar()}>
          {atualizando ? 'atualizando…' : 'atualizar'}
        </button>
        <button type="button" className="forte" disabled={!podeReprocessar || processando || atualizando}
          onClick={() => void reprocessar()}
          title={podeReprocessar
            ? 'gera os orçamentos das notas verdes'
            : 'atualize primeiro — reprocessar sem olhar o estado atual tentaria gerar o que continua travado'}>
          {processando ? 'processando…' : 'reprocessar'}
        </button>
        {recado && <span className="orc-recado">{recado}</span>}
      </div>

      {erro && <p className="erro" style={{ margin: '0 16px 8px' }}>{erro}</p>}

      <div className="orc-rolagem">
        <table className="orc-tabela">
          <thead>
            <tr><th>Arquivo</th><th>Tickets</th><th style={{ textAlign: 'right' }}>Valor</th><th>Situação</th><th style={{ textAlign: 'right' }}>Ações</th></tr>
          </thead>
          <tbody>
            {notas.map(n => {
              const c = conf[n.documento]
              const cor = c ? (c.pronta ? ' linha-verde' : ' linha-vermelha') : ''
              return (
                <tr key={n.documento} className={cor.trim()}>
                  <td>
                    <span className="orc-nome">{n.nome}</span>
                    {n.numero && <span className="orc-detalhe">nº {n.numero}</span>}
                  </td>
                  <td>
                    {n.tickets.map(t => (
                      <span key={t} className="orc-tk">{t}</span>
                    ))}
                  </td>
                  <td style={{ textAlign: 'right' }}>{emReais(n.valor)}</td>
                  <td>
                    {!c ? <span className="mut">atualize para ver</span>
                      : c.pronta ? <span className="orc-selo ok">pronta para processar</span>
                        : <span className="orc-detalhe ruim">{c.motivos?.[0] ?? 'ticket continua não associado'}</span>}
                  </td>
                  <td className="orc-acoes">
                    <button type="button" className="forte"
                      onClick={() => setAbrindo({ documento: n.documento, nome: n.nome, valor: n.valor, tickets: n.tickets })}>
                      corrigir
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </>
  )
}

/** Uma linha por NOTA. O motor devolve um registro por TICKET solto, e uma nota
 *  com três soltos apareceria três vezes pedindo o mesmo trabalho. */
function agruparPorNota(linhas: Dados['sem_associacao']) {
  const mapa = new Map<string, { documento: string; nome: string; numero: string | null; valor: number | null; tickets: number[] }>()
  for (const l of linhas) {
    const atual = mapa.get(l.documento_id)
    if (atual) { atual.tickets.push(l.ticket); continue }
    mapa.set(l.documento_id, {
      documento: l.documento_id,
      nome: l.documentos.nome_arquivo,
      numero: l.documentos.numero,
      valor: l.documentos.valor_total,
      tickets: [l.ticket],
    })
  }
  return [...mapa.values()]
}

// ---------------------------------------------------------------------------
// as notas que passam do teto
// ---------------------------------------------------------------------------

// QUATRO SAÍDAS, E CADA UMA CUSTA UMA COISA DIFERENTE
//
//	desconto     custa DINHEIRO NOSSO — abre mão de parte da margem para a nota
//	             caber no teto. Por isso é o único que pede senha.
//	aprovação    custa uma CONVERSA — o orçamento vai ao cliente com o valor
//	             cheio e ele decide.
//	rateio       não custa nada: a nota estava na gaveta errada.
//	excluir      tira da fila, e dá para restaurar.
//
//	Elas parecem intercambiáveis na tela e não são. Por isso o desconto mostra o
//	número antes de perguntar, e as outras não precisam.
function Extrapoladas({ dados, recarregar }: { dados: Dados; recarregar: () => Promise<void> }) {
  const [abrindo, setAbrindo] = useState<Documento | null>(null)
  const [desconto, setDesconto] = useState<Desconto | null>(null)
  const [confirmando, setConfirmando] = useState(false)
  const [erro, setErro] = useState('')

  const notas = dados.extrapoladas ?? []

  async function abrir(d: Documento) {
    setErro('')
    setAbrindo(d)
    setDesconto(null)
    try {
      setDesconto(await motor<Desconto>(`/orcamentos/documentos/${d.id}/desconto`))
    } catch {
      // Sem a conta do desconto o botão fica apagado, que é o mesmo destino de
      // quando ele não cabe. Melhor apagado do que oferecendo o que não dá.
    }
  }

  async function autorizar(d: Documento) {
    await motor(`/orcamentos/documentos/${d.id}/desconto`, { metodo: 'POST' })
    setAbrindo(null)
    await recarregar()
  }

  const acoes = (d: Documento): Acoes => ({
    excluir: async () => {
      await motor(`/orcamentos/documentos/${d.id}`, { metodo: 'DELETE' })
      setAbrindo(null); await recarregar()
    },
    rateada: async () => {
      await motor(`/orcamentos/documentos/${d.id}/fila`,
        { metodo: 'POST', corpo: { fila: 'rateio' } })
      setAbrindo(null); await recarregar()
    },
    concluir: async () => { setAbrindo(null); await recarregar() },
    fechar: () => setAbrindo(null),
  })

  if (abrindo) {
    return (
      <>
        <div className="orc-barra-acoes">
          <button type="button" className="forte"
            disabled={!desconto?.pode}
            title={desconto?.pode
              ? `desconta ${desconto.porcentagem} para o orçamento fechar no teto`
              : (desconto?.motivo ?? 'conferindo se cabe algum desconto…')}
            onClick={() => setConfirmando(true)}>
            {desconto?.pode ? `dar desconto de ${desconto.porcentagem}` : 'dar desconto'}
          </button>
          <button type="button" className="forte"
            onClick={() => void (async () => {
              try {
                await motor(`/orcamentos/documentos/${abrindo.id}/aprovacao`, { metodo: 'POST' })
                setAbrindo(null); await recarregar()
              } catch (e) {
                setErro(e instanceof Error ? e.message : 'Não consegui mandar para aprovação.')
              }
            })()}
            title="o orçamento nasce com o valor cheio e parado — é ele que vai ao cliente">
            mandar para aprovação
          </button>
          {desconto && !desconto.pode && (
            <span className="orc-recado">{desconto.motivo}</span>
          )}
        </div>

        {erro && <p className="erro" style={{ margin: '0 16px 8px' }}>{erro}</p>}

        <VisorDaNota
          documento={abrindo.id} nome={abrindo.nome_arquivo} valor={abrindo.valor_total}
          tickets={abrindo.ticket_numeros ?? []} modo="sem-associacao" acoes={acoes(abrindo)}
        />

        {confirmando && desconto?.pode && (
          <ConfirmarComSenha
            pergunta={`Dar desconto de ${desconto.porcentagem}? O orçamento cai de `
              + `${emReais(desconto.orcamento_original)} para ${emReais(desconto.orcamento_final)}.`}
            fechar={() => setConfirmando(false)}
            aoConfirmar={() => autorizar(abrindo)}
          />
        )}
      </>
    )
  }

  if (!notas.length) {
    return <p className="orc-vazio grande">Nenhuma nota passando do teto.</p>
  }
  return (
    <>
      {erro && <p className="erro" style={{ margin: '10px 16px' }}>{erro}</p>}
      <div className="orc-rolagem">
        <table className="orc-tabela">
          <thead>
            <tr><th>Arquivo</th><th>Tickets</th><th style={{ textAlign: 'right' }}>Valor</th><th>Por quê</th><th style={{ textAlign: 'right' }}>Ações</th></tr>
          </thead>
          <tbody>
            {notas.map(d => (
              <tr key={d.id}>
                <td>
                  <span className="orc-nome">{d.nome_arquivo}</span>
                  {d.numero && <span className="orc-detalhe">nº {d.numero}</span>}
                </td>
                <td>{(d.ticket_numeros ?? []).map(t => <span key={t} className="orc-tk">{t}</span>)}</td>
                <td style={{ textAlign: 'right' }}>{emReais(d.valor_total)}</td>
                <td>
                  <span className="orc-detalhe ruim">{d.bloqueio_motivo}</span>
                  {d.desconto_bp > 0 && (
                    <span className="orc-selo aviso">desconto de {(d.desconto_bp / 100).toLocaleString('pt-BR')}% autorizado</span>
                  )}
                </td>
                <td className="orc-acoes">
                  <button type="button" className="forte" onClick={() => void abrir(d)}>tratar</button>
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
// 2.4.3 e 2.4.4
// ---------------------------------------------------------------------------

// O QUE CADA MOTIVO OFERECE DE SAÍDA
//
//   A tela não mostra as mesmas opções para todo mundo: oferecer "corrigir o
//   ticket" a quem esbarrou no teto é pedir para a pessoa tentar o caminho que
//   não resolve. Cada motivo abre só o que faz sentido para ele.
//
//   Repare no `ticket_status`: ele NÃO oferece nada de editar. O problema não é
//   nosso — é o chamado que ainda não andou. Fingir que existe conserto aqui
//   dentro seria empurrar a pessoa para uma manobra.
const SAIDAS: Record<Bloqueio, { rotulo: string; cor: string; opcoes: string[] }> = {
  ticket_status: {
    rotulo: 'O chamado não aceita custo ainda',
    cor: 'espera',
    opcoes: ['Reconferir status agora', 'Incluir na lista de cobrança'],
  },
  ticket_recusado: {
    rotulo: 'Ticket não encontrado nesta conta',
    cor: 'erro',
    opcoes: ['Corrigir o ticket', 'Apagar o orçamento'],
  },
  teto: {
    rotulo: 'Passa do teto com o que já está lançado',
    cor: 'erro',
    opcoes: ['Aprovar com autorização', 'Gerar de novo com valor menor', 'Apagar o orçamento'],
  },
  sem_empresa: {
    rotulo: 'Falta configurar a empresa no servidor',
    cor: 'erro',
    opcoes: ['Avisar quem administra'],
  },
  trilogo_fora: {
    rotulo: 'O Trílogo não respondeu',
    cor: 'espera',
    opcoes: ['Tentar de novo'],
  },
  desconhecido: {
    rotulo: 'O Trílogo recusou',
    cor: 'erro',
    opcoes: ['Tentar de novo'],
  },
}

function Recusados({ dados }: { dados: Dados }) {
  if (!dados.recusados.length) {
    return (
      <p className="orc-vazio grande">
        Nenhuma recusa. O que está na fila de lançar ainda não foi tentado —
        e isso não é problema, é trabalho a fazer.
      </p>
    )
  }
  return (
    <div className="orc-rolagem">
      <table className="orc-tabela">
        <thead>
          <tr>
            <th>Ticket</th><th>Loja</th><th style={{ textAlign: 'right' }}>Valor</th>
            <th>Por que não subiu</th><th>Quem resolve</th><th>Tentado</th>
          </tr>
        </thead>
        <tbody>
          {dados.recusados.map(o => {
            const s = o.lancamento_bloqueio ? SAIDAS[o.lancamento_bloqueio] : null
            return (
              <tr key={o.id}>
                <td><span className="orc-nome">{o.ticket}{o.parte > 1 ? `-${o.parte}` : ''}</span></td>
                <td>{o.loja ?? '–'}</td>
                <td style={{ textAlign: 'right' }}>{emReais(o.valor)}</td>
                <td>
                  <span className={`orc-marca ${s?.cor ?? ''}`}>{s?.rotulo ?? 'Recusado'}</span>
                  {/* A frase do Trílogo, como ela veio. É o que explica o caso
                      que a categoria não cobre — e o que a pessoa lê quando a
                      categoria não basta. */}
                  {o.lancamento_bloqueio_detalhe && (
                    <div className="orc-sub">{o.lancamento_bloqueio_detalhe}</div>
                  )}
                  {s && <div className="orc-sub opcoes">{s.opcoes.join(' · ')}</div>}
                </td>
                <td>
                  {o.destino === 'cliente' ? 'Cliente'
                    : o.destino === 'encarregados' ? 'Encarregados'
                    : o.destino === 'pode_lancar' ? 'Já liberou — pode tentar'
                    : '–'}
                  {o.reaberto && o.motivo_reabertura && (
                    <div className="orc-sub">reaberto: {o.motivo_reabertura}</div>
                  )}
                </td>
                <td>
                  {o.lancamento_tentado_em ? emDataHora(o.lancamento_tentado_em) : '–'}
                  {o.lancamento_tentativas > 1 && (
                    <div className="orc-sub">{o.lancamento_tentativas} tentativas</div>
                  )}
                </td>
              </tr>
            )
          })}
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
