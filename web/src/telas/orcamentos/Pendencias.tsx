// rev 1 — Pendências: os orçamentos que esperam alguém MOVER o chamado
//
// A TELA QUE FALTAVA, E O BURACO QUE ELA TAPA
//
//	Medido em 27/08/2026: 93 orçamentos gerados, 25 podendo subir, 68 travados
//	pelo status do chamado — e apenas 23 visíveis em algum lugar do sistema. Os
//	23 eram os RECUSADOS, que é outra coisa: a marca `lancamento_bloqueio` só
//	nasce quando alguém aperta o botão e o Trílogo nega. Os 68 nunca foram
//	tentados, então não têm marca, então não apareciam em tela nenhuma.
//
//	O motor já montava esta lista desde a 015 — agrupada por ticket, com PDF e
//	planilha prontos — e nenhuma tela chamava. Regra escrita, testada e invisível
//	é regra que não existe para quem trabalha.
//
// SÃO DUAS LISTAS PORQUE VÃO PARA DUAS PESSOAS
//
//	encarregados  o chamado está Aberto ou Em execução: é serviço NOSSO a
//	              terminar. Enquanto não for executado, o Trílogo não aceita o
//	              custo.
//	cliente       o chamado está Arquivado ou foi Reaberto. Nenhuma das duas nós
//	              destravamos.
//
//	Por isso as colunas mudam de uma para a outra: ao cliente interessa a frase
//	que ELE escreveu ao reabrir; ao encarregado, o que foi pedido no chamado.
//	É a mesma escolha que o PDF do motor faz, e de propósito — duas telas para o
//	mesmo dado que discordam na hora de imprimir são duas verdades (CORE-06).
//
// A LINHA É O TICKET, NÃO O ORÇAMENTO
//
//	Um ticket com três partes paradas vai como UMA linha, com a soma. Mandar três
//	faria o cliente perguntar por que três — e "são partes" é detalhe nosso.
//
// VER A LISTA É NA TELA
//
//	O PDF abre no visor, como todo documento desta casa. O "salvar como" mora lá
//	dentro e sempre pergunta a pasta. A planilha, essa não tem como abrir na
//	tela: ela é para mandar, e vai direto para o seletor de pasta.
//
// MARCAR COMO COBRADO NÃO MANDA NADA
//
//	Quem manda é a pessoa, com a lista na mão. Isto aqui só registra que saiu, e
//	em que dia — é o que separa "já cobrei e não veio" de "nunca cobrei". Um
//	registro de envio que mente é pior do que nenhum, então o botão só marca o
//	que a pessoa escolheu, uma escolha por vez.
import { useCallback, useEffect, useRef, useState } from 'react'
import { motor, arquivoDoMotor, salvarArquivo } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { useFocado } from '../../componentes/Foco'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { BarraDeVolta } from './Arquivos'
import { emReais, emDataHora, type ListaDePendencias, type Pendencia, type Orcamento, type Pagina } from './tipos'

const LISTAS = [
  {
    chave: 'encarregados',
    titulo: 'Nossa equipe',
    desc: 'Chamados Abertos ou Em execução — o Trílogo não aceita custo enquanto o serviço não for concluído',
  },
  {
    chave: 'cliente',
    titulo: 'Cliente',
    desc: 'Chamados Arquivados ou Reabertos — só o cliente destrava',
  },
  // A TERCEIRA LISTA — 28/08/2026
  //
  //	As duas de cima esperam o MUNDO andar: o chamado ser executado, o cliente
  //	reabrir. Esta espera VOCÊ. O ticket vai continuar com o mesmo custo
  //	lançado e o teto vai continuar sendo R$ 600 — tentar de novo dá o mesmo
  //	resultado, para sempre.
  //
  //	Ela estava na fila de Lançar, e foi de lá que veio o "0 de 5 subiram":
  //	cinco orçamentos que a tela chamava de prontos e que nenhuma quantidade de
  //	cliques ia subir. Aqui eles têm o que faltava — o que fazer com eles.
  {
    chave: 'decisao',
    titulo: 'Esperando você',
    desc: 'Travados por teto ou por duplicidade — nada disso destrava sozinho',
  },
] as const

type Qual = (typeof LISTAS)[number]['chave']

export function Pendencias({ lista, voltar, embutido }: {
  lista?: string
  /** Só quando ela é a tela inteira. Embutida em Correções, quem tem o voltar
   *  é a casca de fora. */
  voltar?: () => void
  /** EMBUTIDA EM CORREÇÕES — 28/08/2026
   *
   *  Os orçamentos parados passaram a morar dentro de Correções, junto das
   *  notas que travaram antes de virar orçamento: é a mesma pergunta, e
   *  respondê-la em dois lugares obriga quem trabalha a lembrar dos dois.
   *
   *  Embutida, ela abre mão da própria barra de voltar e das próprias abas —
   *  quem manda nas duas é a casca de fora. O miolo é o mesmo, e é por isso que
   *  esta tela é um componente e não uma página: a lista, o lote de
   *  reconferência e as três saídas continuam existindo UMA vez (CORE-06). */
  embutido?: boolean
}) {
  // Correções chama a lista da equipe de 'equipe'; aqui ela sempre se chamou
  // 'encarregados', que é o nome que o motor usa no `destino`. A tradução é uma
  // linha, e trocar o nome no motor seria mexer numa palavra que a view, o PDF e
  // o registro de cobrança já usam.
  const [qual, setQual] = useState<Qual>(
    lista === 'cliente' || lista === 'decisao' ? lista : 'encarregados')
  useEffect(() => {
    setQual(lista === 'cliente' || lista === 'decisao' ? lista : 'encarregados')
  }, [lista])
  const [dados, setDados] = useState<ListaDePendencias | null>(null)
  const [decisoes, setDecisoes] = useState<Pagina<Orcamento> | null>(null)
  const [tratando, setTratando] = useState<string | null>(null)
  const [erro, setErro] = useState('')
  const [aviso, setAviso] = useState('')
  const [marcando, setMarcando] = useState(false)
  const [vendo, setVendo] = useState(false)
  // A ESCOLHA MORRE AO TROCAR DE LISTA
  //   Os números marcados são de tickets daquela lista. Carregá-los para a outra
  //   faria o botão dizer "marcar 6" com nenhum dos 6 na tela.
  const [escolhidos, setEscolhidos] = useState<Set<number>>(new Set())
  // A RECONFERÊNCIA EM LOTE
  //   Sem ela, uma pendência é um beco: a nota fica ali até alguém lembrar de
  //   abrir cada orçamento e apertar "reconferir". Medimos que 7 de 26 chamados
  //   destravam sozinhos em seis dias — o trabalho existe, e ninguém o faz um
  //   por um.
  const [lote, setLote] = useState<Lote | null>(null)
  const parar = useRef(false)
  const focado = useFocado()

  const carregar = useCallback(async () => {
    try {
      if (qual === 'decisao') {
        // Esta lista é de ORÇAMENTO, não de ticket: a decisão sobre uma
        // duplicata é sobre aquela linha, não sobre o chamado inteiro.
        setDecisoes(await motor<Pagina<Orcamento>>(
          '/orcamentos?status=gerado&decisao=so&por=100'))
        setDados(null)
      } else {
        setDados(await motor<ListaDePendencias>('/orcamentos/pendencias?destino=' + qual))
        setDecisoes(null)
      }
      setErro('')
    } catch (e) {
      setDados(null); setDecisoes(null)
      setErro(e instanceof Error ? e.message : 'Não consegui carregar as pendências.')
    }
  }, [qual])

  useEffect(() => { void carregar() }, [carregar])

  function trocar(c: Qual) {
    setQual(c)
    setEscolhidos(new Set())
    setAviso('')
  }

  function alternar(ticket: number) {
    setEscolhidos(s => {
      const novo = new Set(s)
      if (novo.has(ticket)) novo.delete(ticket)
      else novo.add(ticket)
      return novo
    })
  }

  const tickets = dados?.tickets ?? []
  const todosMarcados = tickets.length > 0 && escolhidos.size === tickets.length

  async function baixarPlanilha() {
    try {
      const a = await arquivoDoMotor(`/orcamentos/pendencias.xlsx?destino=${qual}`)
      await salvarArquivo(a.blob!, a.nome)
      URL.revokeObjectURL(a.url)
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui montar a planilha.')
    }
  }

  // RECONFERIR É LER, NÃO LANÇAR
  //
  //   Ela pergunta ao Trílogo o status de agora e grava o que viu. Se o chamado
  //   andou, o orçamento sai desta lista e volta para a fila de lançar — quem
  //   manda subir continua sendo a pessoa. Um botão que diz "conferir" e lança
  //   seria um botão que mente sobre o que faz.
  //
  //   É UM POR TICKET, e não um por orçamento: o status é do chamado, e um
  //   ticket de três partes destrava as três de uma vez. Reconferir uma a uma
  //   seriam três idas ao Trílogo para a mesma resposta.
  //
  //   O laço é da tela, como o de lançar, e pelo mesmo motivo: sessenta idas ao
  //   Trílogo numa requisição só estouram qualquer tempo limite e não mostram
  //   nada enquanto rodam.
  async function reconferirTodos() {
    const fila = tickets.filter(t => t.orcamento)
    if (fila.length === 0) return
    parar.current = false
    setAviso('')
    setErro('')
    setLote({ total: fila.length, feito: 0, liberaram: 0, rodando: true, liberados: [] })

    for (const t of fila) {
      if (parar.current) break
      setLote(l => (l ? { ...l, agora: t.ticket } : l))
      try {
        const r = await motor<{ liberou: boolean; ticket: number; ticket_status: string }>(
          `/orcamentos/ficha/${t.orcamento}/reconferir`, { metodo: 'POST' })
        setLote(l => l && ({
          ...l,
          feito: l.feito + 1,
          liberaram: l.liberaram + (r.liberou ? 1 : 0),
          liberados: r.liberou ? [...l.liberados, r.ticket] : l.liberados,
        }))
      } catch {
        // Um chamado que não respondeu não derruba a rodada: ele continua na
        // lista e a próxima passada tenta de novo.
        setLote(l => l && ({ ...l, feito: l.feito + 1 }))
      }
      // O Trílogo é o sistema DO CLIENTE: sessenta chamadas coladas parecem
      // ataque, mesmo sendo trabalho.
      await new Promise(r => setTimeout(r, 400))
    }

    setLote(l => (l ? { ...l, rodando: false, agora: undefined } : l))
    await carregar()
  }

  // AS TRÊS SAÍDAS, E CADA UMA DESFAZ UM DOS DOIS BLOQUEIOS
  //
  //	apagar          serve aos dois: a duplicata que é duplicata mesmo, e o
  //	                orçamento que não vai ser cobrado.
  //	lançar assim    só para duplicidade: duas notas iguais de verdade
  //	                acontecem, e quem viu o custo que já está lá decide.
  //	pedir aprovação só para teto: o cliente autoriza passar dos R$ 600.
  //
  //	Nenhuma delas roda sem confirmação: são as três coisas desta tela que
  //	mexem em dinheiro.
  async function tratar(o: Orcamento, acao: 'apagar' | 'lancar' | 'aprovar') {
    const frases = {
      apagar: `Apagar o orçamento do ticket ${o.ticket}, de ${emReais(o.valor)}? A nota volta para a fila.`,
      lancar: `Lançar ${emReais(o.valor)} no ticket ${o.ticket} mesmo com um custo do mesmo valor já lá?`,
      aprovar: `Registrar que o cliente autorizou ${emReais(o.valor)} acima do teto no ticket ${o.ticket}?`,
    }
    if (!window.confirm(frases[acao])) return
    setTratando(o.id)
    setErro('')
    try {
      if (acao === 'apagar') {
        await motor(`/orcamentos/ficha/${o.id}`, { metodo: 'DELETE' })
        setAviso(`Orçamento do ticket ${o.ticket} apagado. A nota voltou para a fila.`)
      } else if (acao === 'lancar') {
        const r = await motor<{ trilogo_custo_id: number }>(
          `/orcamentos/ficha/${o.id}/lancar?duplicata=confirmo`, { metodo: 'POST' })
        setAviso(`Ticket ${o.ticket}: lançado no Trílogo (custo nº ${r.trilogo_custo_id}).`)
      } else {
        await motor(`/orcamentos/ficha/${o.id}/aprovar`, { metodo: 'POST', corpo: {} })
        setAviso(`Ticket ${o.ticket}: aprovação registrada. Ele volta para a fila de lançar.`)
      }
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui concluir.')
    } finally {
      setTratando(null)
    }
  }

  async function marcarCobrados() {
    if (escolhidos.size === 0) return
    setMarcando(true)
    try {
      const r = await motor<{ marcados: number; ignorados: number }>(
        '/orcamentos/pendencias/avisar',
        { metodo: 'POST', corpo: { destino: qual, tickets: [...escolhidos] } })
      // O IGNORADO É NOTÍCIA, NÃO RUÍDO
      //   Ele significa que o chamado ANDOU entre a tela abrir e o clique — e
      //   isso é justamente o que a pessoa queria que acontecesse. Esconder o
      //   número faria a conta não fechar sem explicação.
      setAviso(r.ignorados > 0
        ? `${r.marcados} marcados como cobrados. ${r.ignorados} saíram da lista antes do clique — o chamado andou.`
        : `${r.marcados} marcados como cobrados.`)
      setEscolhidos(new Set())
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui registrar o aviso.')
    } finally {
      setMarcando(false)
    }
  }

  if (vendo) {
    return (
      <VisorDeDocumento
        titulo={<>Pendências · {LISTAS.find(l => l.chave === qual)?.titulo}</>}
        caminho={`/orcamentos/pendencias.pdf?destino=${qual}`}
        nomeSugerido={`pendencias-${qual}.pdf`}
        voltar={() => setVendo(false)}
      />
    )
  }

  return (
    <div className="orc-tela">
      {!embutido && voltar && <BarraDeVolta voltar={voltar} titulo="Pendências" />}

      {!embutido && !focado && <div className="orc-abas">
        {LISTAS.map(l => (
          <button
            key={l.chave}
            type="button"
            className={'orc-aba' + (qual === l.chave ? ' ligada' : '')}
            onClick={() => trocar(l.chave)}
          >
            <b>{qual !== l.chave ? '·' : l.chave === 'decisao' ? (decisoes?.total ?? 0) : tickets.length}</b>
            <span>{l.titulo}</span>
          </button>
        ))}
      </div>}

      {erro && <p className="erro">{erro}</p>}
      {aviso && <p className="orc-aviso-fila">{aviso}</p>}
      {lote && <PainelDoLote dados={lote} fechar={() => setLote(null)} />}

      {qual === 'decisao' ? (
        !decisoes ? <Carregando /> : (
          <div className="orc-lista">
            {!embutido && (
              <div className="orc-lista-cab">
                <h2>Esperando você</h2>
                <em>{LISTAS.find(l => l.chave === 'decisao')?.desc}</em>
              </div>
            )}
            {decisoes.linhas.length === 0 ? (
              <p className="orc-vazio grande">
                Nada esperando decisão. Nenhum orçamento está travado por teto
                ou por duplicidade.
              </p>
            ) : (
              <div className="orc-rolagem">
                <table className="orc-tabela">
                  <thead>
                    <tr>
                      <th>Chamado</th><th>Loja</th>
                      <th style={{ textAlign: 'right' }}>Valor</th>
                      <th>Por que travou</th><th>O que dá para fazer</th>
                    </tr>
                  </thead>
                  <tbody>
                    {decisoes.linhas.map(o => (
                      <Decisao key={o.id} o={o}
                        ocupado={tratando === o.id}
                        tratar={a => void tratar(o, a)} />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )
      ) : !dados ? <Carregando /> : (
        <div className="orc-lista">
          {!embutido && (
            <div className="orc-lista-cab">
              <h2>{LISTAS.find(l => l.chave === qual)?.titulo}</h2>
              <em>{LISTAS.find(l => l.chave === qual)?.desc}</em>
            </div>
          )}

          {/* O QUE ESTÁ PARADO, EM DINHEIRO, ANTES DA TABELA
              Trinta e quatro chamados não movem ninguém; oito mil e oitocentos
              reais parados movem. O número é o argumento da cobrança. */}
          <div className="orc-pend-resumo">
            <span><b>{tickets.length}</b> chamados</span>
            <span><b>{dados.orcamentos}</b> orçamentos</span>
            <span><b>{emReais(dados.valor)}</b> parados</span>
            <span className="orc-pend-bts">
              <button type="button" className="orc-bt" onClick={() => setVendo(true)}
                disabled={tickets.length === 0}>ver a lista</button>
              <button type="button" className="orc-bt" onClick={() => void baixarPlanilha()}
                disabled={tickets.length === 0}>baixar planilha</button>
              {/* Reconferir pergunta o status do CHAMADO, e por isso ele não
                  aparece na lista "Esperando você" — lá o chamado está
                  liberado, e o que trava é outra coisa. O TypeScript já sabe
                  disso aqui: este ramo só existe para as outras duas listas. */}
              {lote?.rodando ? (
                <button type="button" className="orc-bt" onClick={() => { parar.current = true }}>
                  parar depois deste
                </button>
              ) : (
                <button type="button" className="orc-bt forte"
                  disabled={tickets.length === 0}
                  title="Pergunta ao Trílogo o status de cada chamado desta lista. Quem já liberou volta para a fila de lançar."
                  onClick={() => void reconferirTodos()}>
                  reconferir no Trílogo ({tickets.length})
                </button>
              )}
              <button type="button" className="orc-bt forte"
                disabled={escolhidos.size === 0 || marcando}
                onClick={() => void marcarCobrados()}>
                {marcando ? 'marcando…' : `marcar ${escolhidos.size || ''} como cobrados`}
              </button>
            </span>
          </div>

          {tickets.length === 0 ? (
            <p className="orc-vazio grande">
              Nada esperando aqui. Todo orçamento desta fila já pode subir, ou
              já subiu.
            </p>
          ) : (
            <div className="orc-rolagem">
              <table className="orc-tabela">
                <thead>
                  <tr>
                    <th className="orc-pend-marca">
                      <input
                        type="checkbox"
                        aria-label="marcar todos"
                        checked={todosMarcados}
                        onChange={() => setEscolhidos(todosMarcados
                          ? new Set()
                          : new Set(tickets.map(t => t.ticket)))}
                      />
                    </th>
                    <th>Chamado</th>
                    <th>Loja</th>
                    <th>Conta</th>
                    <th>Situação</th>
                    <th>{qual === 'cliente' ? 'O que o cliente disse' : 'O que foi pedido'}</th>
                    <th style={{ textAlign: 'right' }}>Parado</th>
                    <th>Desde</th>
                    <th>Cobrado</th>
                  </tr>
                </thead>
                <tbody>
                  {tickets.map(t => (
                    <Linha
                      key={t.ticket}
                      t={t}
                      qual={qual}
                      marcado={escolhidos.has(t.ticket)}
                      alternar={() => alternar(t.ticket)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

/** Uma linha da lista "Esperando você".
 *
 *  AS SAÍDAS MUDAM COM O MOTIVO, E ISSO NÃO É DETALHE
 *    Oferecer "lançar assim mesmo" a quem passou do teto seria oferecer o
 *    caminho que não resolve — o Trílogo aceita, e o cliente recebe uma
 *    cobrança acima do que autorizou. E oferecer "pedir aprovação" a uma
 *    duplicata seria pedir ao cliente que autorize pagar duas vezes. */
function Decisao({ o, ocupado, tratar }: {
  o: Orcamento
  ocupado: boolean
  tratar: (acao: 'apagar' | 'lancar' | 'aprovar') => void
}) {
  const duplicata = o.lancamento_bloqueio === 'possivel_duplicata'
  return (
    <tr>
      <td>
        <span className="orc-nome">{o.ticket}{o.parte > 1 ? `-${o.parte}` : ''}</span>
      </td>
      <td>{o.loja ?? '–'}</td>
      <td style={{ textAlign: 'right' }}>{emReais(o.valor)}</td>
      <td className="orc-pend-texto">
        <span className={`orc-marca ${duplicata ? 'erro' : 'espera'}`}>
          {duplicata ? 'já tem custo deste valor' : 'passa do teto'}
        </span>
        {o.lancamento_bloqueio_detalhe && (
          <div className="orc-sub">{o.lancamento_bloqueio_detalhe}</div>
        )}
      </td>
      <td>
        <div className="orc-pend-bts">
          {duplicata ? (
            <button type="button" className="orc-bt" disabled={ocupado}
              title="Se forem duas notas iguais de verdade, sobe assim mesmo."
              onClick={() => tratar('lancar')}>lançar assim mesmo</button>
          ) : (
            <button type="button" className="orc-bt" disabled={ocupado}
              title="Registra que o cliente autorizou passar do teto."
              onClick={() => tratar('aprovar')}>pedir aprovação</button>
          )}
          <button type="button" className="orc-bt forte" disabled={ocupado}
            title="O orçamento sai, e a nota volta para a fila."
            onClick={() => tratar('apagar')}>apagar</button>
        </div>
      </td>
    </tr>
  )
}

interface Lote {
  total: number
  feito: number
  liberaram: number
  liberados: number[]
  rodando: boolean
  agora?: number
}

/** O andamento da reconferência, e o resultado dela.
 *
 *  O PAINEL NÃO SOME SOZINHO
 *    Sessenta chamados levam minutos. Quem sai da frente da tela e volta
 *    precisa encontrar o resultado ainda ali — um aviso que se apaga transforma
 *    "quantos liberaram?" numa pergunta sem resposta. */
function PainelDoLote({ dados, fechar }: { dados: Lote; fechar: () => void }) {
  const pct = dados.total === 0 ? 0 : Math.round((dados.feito / dados.total) * 100)
  return (
    <div className="orc-lote">
      <div className="orc-lote-topo">
        <span>
          {dados.rodando
            ? <>conferindo no Trílogo{dados.agora ? ` — chamado ${dados.agora}` : ''}…</>
            : <>conferência terminada</>}
        </span>
        <strong>{dados.feito} de {dados.total}</strong>
      </div>
      <div className="orc-lote-barra"><span style={{ width: `${pct}%` }} /></div>
      {!dados.rodando && (
        <div className="orc-lote-fim">
          {dados.liberaram === 0 ? (
            <p>
              Nenhum chamado liberou. Eles continuam parados pelo mesmo motivo —
              e isso não é erro: é a resposta de hoje.
            </p>
          ) : (
            <p>
              <strong>{dados.liberaram}</strong>{' '}
              {dados.liberaram === 1 ? 'chamado liberou e voltou' : 'chamados liberaram e voltaram'}{' '}
              para a fila de lançar: {dados.liberados.join(', ')}.
            </p>
          )}
          <button type="button" className="orc-bt" onClick={fechar}>Entendi</button>
        </div>
      )}
    </div>
  )
}

function Linha({ t, qual, marcado, alternar }: {
  t: Pendencia
  qual: Qual
  marcado: boolean
  alternar: () => void
}) {
  return (
    <tr className={marcado ? 'orc-pend-escolhida' : undefined}>
      <td className="orc-pend-marca">
        <input type="checkbox" checked={marcado} onChange={alternar}
          aria-label={`marcar o chamado ${t.ticket}`} />
      </td>
      <td>
        <span className="orc-nome">{t.ticket}</span>
        {/* Quantas partes daquele ticket estão paradas. Some da linha quando é
            uma só: "1 orçamento" é ruído em trinta e quatro linhas. */}
        {t.orcamentos > 1 && (
          <div className="orc-sub">{t.orcamentos} partes · {t.partes.join(', ')}</div>
        )}
      </td>
      <td>{t.loja || '–'}</td>
      <td>{t.conta || '–'}</td>
      <td>
        {t.ticket_status || '–'}
        {t.reaberto && <div className="orc-sub aviso">reaberto</div>}
        {/* A ÚLTIMA RECUSA, ONDE O TRABALHO É FEITO
            Antes ela morava numa lista separada, que dizia o motivo e não
            oferecia conserto. Aqui ela fica a um botão de distância do
            "reconferir no Trílogo", que é o que a desfaz. */}
        {t.recusa && (
          <div className="orc-sub">
            já tentado{t.tentativas > 1 ? ` ${t.tentativas}×` : ''}
            {t.recusa_em ? ` em ${emDataHora(t.recusa_em)}` : ''}
          </div>
        )}
      </td>
      <td className="orc-pend-texto">
        {/* Ao cliente, a frase DELE ao reabrir: muda a conversa de "reabriram"
            para "reabriram dizendo que o serviço não foi executado". Ao
            encarregado, o que o chamado pediu — é onde ele vai e o que termina. */}
        {(qual === 'cliente' ? t.motivo : t.descricao) || <span className="orc-detalhe">–</span>}
      </td>
      <td style={{ textAlign: 'right' }}>{emReais(t.valor)}</td>
      <td>{t.desde_em ? emDataHora(t.desde_em) : '–'}</td>
      <td>
        {t.avisado_em
          ? emDataHora(t.avisado_em)
          : <span className="orc-detalhe">nunca</span>}
      </td>
    </tr>
  )
}
