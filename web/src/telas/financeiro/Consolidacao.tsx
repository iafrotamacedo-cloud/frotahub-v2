// rev 1 — Financeiro › Consolidação
//
// O QUE ESTA TELA RESPONDE
//
//	Duas perguntas que hoje só se responde abrindo três telas e uma planilha:
//
//	  "Comprei esta nota. Quanto dela já voltou do cliente?"   → aba das NOTAS
//	  "Cobrei este ticket. De que nota saiu, e já foi paga?"   → aba dos TICKETS
//
//	São o mesmo dinheiro visto dos dois lados. Ficam na mesma tela, e vêm na
//	mesma resposta do servidor, porque a pessoa clica de um lado e confere do
//	outro — e porque duas consultas em instantes diferentes podem discordar por
//	causa de um lançamento que entrou no meio.
//
// A TROCA DE ABA NÃO VOLTA AO SERVIDOR
//
//	As duas listas já estão na mão. Trocar de aba é filtro de tela, e é isso que
//	faz o clique responder na hora. O mesmo vale para os tickets de uma nota: a
//	lista deles sai da aba 2, que já foi carregada — nenhuma ida ao banco.
//
// O QUE ESTA TELA NÃO FAZ
//
//	Não grava nada. Não tem botão que muda estado. É conferência: quem lê aqui
//	vai agir em outra tela, com conhecimento de causa.
import { useEffect, useMemo, useState } from 'react'
import { motor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta } from '../orcamentos/Arquivos'
import { emReais, emData } from '../orcamentos/tipos'

interface Nota {
  documento_id: string
  nf: string | null
  tipo: 'NF' | 'DAV'
  fornecedor: string | null
  emissao: string | null
  valor: number | null
  fila: string | null
  status: string
  tickets: number
  ticket_numeros: number[]
  orcamentos: number
  orcado: number
  recebido: number
  pendente: number
  lancado: number
  inserido_em: string
}

interface Orcamento {
  orcamento_id: string
  ticket: number
  parte: number
  loja: string | null
  conta: string | null
  valor: number
  valor_nota: number | null
  nfs: string | null
  nota_excluida: boolean
  no_relatorio: boolean
  faturado: boolean
  pago: boolean
  status: string
  lancado_em: string | null
  rateio: boolean
  legado: boolean
  criado_em: string
}

interface Consolidado {
  gerado_em: string
  notas: Nota[]
  tickets: Orcamento[]
}

type Aba = 'notas' | 'tickets'

// TODOS / SIM / NÃO, E NÃO UMA CAIXA DE TEXTO
//   'nf' e 'pago ao fornecedor' são as duas colunas que o dono pediu para
//   filtrar por S/N. Não entram na busca por texto cru (linha acima) porque
//   "sim" e "não" não são o que está escrito no papel que a pessoa tem na
//   mão — são uma pergunta categórica ('tem nota ligada?', 'já foi pago?'),
//   que um <select> de três posições responde num clique e uma busca de
//   texto não responde nunca.
type FiltroSN = 'todos' | 'sim' | 'nao'

export function Consolidacao({ voltar }: { voltar: () => void }) {
  const [dados, setDados] = useState<Consolidado | null>(null)
  const [erro, setErro] = useState('')
  const [aba, setAba] = useState<Aba>('notas')
  const [busca, setBusca] = useState('')
  const [filtroNf, setFiltroNf] = useState<FiltroSN>('todos')
  const [filtroPago, setFiltroPago] = useState<FiltroSN>('todos')
  // A nota aberta mora aqui, não no endereço: é uma gaveta de conferência que
  // se fecha em dois segundos, não um lugar para onde alguém manda link.
  const [notaAberta, setNotaAberta] = useState<Nota | null>(null)

  useEffect(() => {
    void (async () => {
      try {
        setDados(await motor<Consolidado>('/consolidacao'))
        setErro('')
      } catch (e) {
        setErro(e instanceof Error ? e.message : 'Não consegui montar a consolidação.')
      }
    })()
  }, [])

  // O FILTRO É POR TEXTO CRU, DE PROPÓSITO
  //   Quem confere tem um número na mão — a NF no papel, o ticket no e-mail — e
  //   quer achá-lo. Filtro por coluna obrigaria a saber ANTES em qual coluna o
  //   número mora, que é justamente o que a pessoa está tentando descobrir.
  const q = busca.trim().toLowerCase()
  const notas = useMemo(
    () => !dados ? [] : !q ? dados.notas
      : dados.notas.filter(n =>
        (n.nf ?? '').toLowerCase().includes(q)
        || (n.fornecedor ?? '').toLowerCase().includes(q)
        || n.ticket_numeros.some(t => String(t).includes(q))),
    [dados, q])

  const tickets = useMemo(
    () => !dados ? [] : dados.tickets.filter(o =>
      (!q
        || String(o.ticket).includes(q)
        || (o.nfs ?? '').toLowerCase().includes(q)
        || (o.loja ?? '').toLowerCase().includes(q))
      && (filtroNf === 'todos' || (filtroNf === 'sim') === (o.nfs !== null))
      && (filtroPago === 'todos' || (filtroPago === 'sim') === o.pago)),
    [dados, q, filtroNf, filtroPago])

  const somaNotas = useMemo(() => ({
    valor: notas.reduce((s, n) => s + (n.valor ?? 0), 0),
    recebido: notas.reduce((s, n) => s + n.recebido, 0),
    pendente: notas.reduce((s, n) => s + n.pendente, 0),
    lancado: notas.reduce((s, n) => s + n.lancado, 0),
  }), [notas])

  const somaTickets = useMemo(() => ({
    valor: tickets.reduce((s, o) => s + o.valor, 0),
    noRelatorio: tickets.filter(o => o.no_relatorio).length,
    pagos: tickets.filter(o => o.pago).length,
    orfaos: tickets.filter(o => o.nota_excluida).length,
  }), [tickets])

  // Os tickets da nota aberta saem da aba 2, que já está carregada.
  const daNota = useMemo(() => {
    if (!dados || !notaAberta) return []
    const meus = new Set(notaAberta.ticket_numeros)
    return dados.tickets.filter(o => meus.has(o.ticket))
  }, [dados, notaAberta])

  return (
    <div className="tela">
      <BarraDeVolta voltar={voltar} titulo="Financeiro › Consolidação" />
      <div className="orc-lista-cab">
        <h2>Consolidação</h2>
        <em>O que se comprou contra o que se cobrou — nota e ticket, nos dois sentidos</em>
      </div>

      {erro && <p className="erro" style={{ margin: '0 16px 8px' }}>{erro}</p>}

      {!dados ? <Carregando /> : (
        <>
          <div className="orc-barra-acoes">
            {/* AS ABAS SÃO OS DOIS SENTIDOS DA MESMA CONFERÊNCIA */}
            <button type="button" className={aba === 'notas' ? 'forte' : ''}
              onClick={() => { setAba('notas'); setNotaAberta(null) }}>
              Obra Prima × FrotaHub
            </button>
            <button type="button" className={aba === 'tickets' ? 'forte' : ''}
              onClick={() => { setAba('tickets'); setNotaAberta(null) }}>
              FrotaHub × Obra Prima
            </button>
            <input type="search" value={busca} placeholder="nota, DAV, ticket ou loja"
              onChange={e => setBusca(e.target.value)} style={{ minWidth: 220 }} />
            {aba === 'tickets' && (
              <>
                <select aria-label="filtrar por NF" value={filtroNf}
                  onChange={e => setFiltroNf(e.target.value as FiltroSN)}>
                  <option value="todos">NF: todos</option>
                  <option value="sim">NF: sim</option>
                  <option value="nao">NF: não</option>
                </select>
                <select aria-label="filtrar por pago ao fornecedor" value={filtroPago}
                  onChange={e => setFiltroPago(e.target.value as FiltroSN)}>
                  <option value="todos">Pago: todos</option>
                  <option value="sim">Pago: sim</option>
                  <option value="nao">Pago: não</option>
                </select>
              </>
            )}
            <span className="pg-resumo">
              {aba === 'notas'
                ? <><b>{notas.length}</b> nota{notas.length === 1 ? '' : 's'}<u>{emReais(somaNotas.valor)}</u></>
                : <><b>{tickets.length}</b> orçamento{tickets.length === 1 ? '' : 's'}<u>{emReais(somaTickets.valor)}</u></>}
            </span>
          </div>

          {/* O ZERO DA COLUNA RECEBIDO TEM EXPLICAÇÃO, E ELA FICA NA TELA
              Número que nasce zerado sem motivo escrito é lido como defeito, e
              alguém abre chamado. Escrito, é informação: mostra exatamente onde
              o dinheiro parou. */}
          {aba === 'notas' && somaNotas.recebido === 0 && somaNotas.lancado > 0 && (
            <p className="orc-detalhe aviso" style={{ margin: '0 16px 10px', maxWidth: '94ch' }}>
              Nenhuma nota tem valor em <b>recebido</b> ainda: os orçamentos ligados a nota
              foram lançados no Trílogo ({emReais(somaNotas.lancado)}) mas nenhum entrou num
              fechamento mensal. A coluna passa a ter número no primeiro fechamento que
              incluir esses orçamentos.
            </p>
          )}

          {aba === 'tickets' && somaTickets.orfaos > 0 && (
            <p className="orc-detalhe aviso" style={{ margin: '0 16px 10px', maxWidth: '94ch' }}>
              {somaTickets.orfaos} orçamento{somaTickets.orfaos > 1 ? 's saíram' : ' saiu'} de
              nota que foi <b>excluída depois</b>. {somaTickets.orfaos > 1 ? 'Eles não aparecem' : 'Ele não aparece'} na
              aba das notas, mas a cobrança ao cliente continua de pé.
            </p>
          )}

          {aba === 'notas' ? (
            <div className="orc-rolagem">
              <table className="orc-tabela">
                <thead>
                  <tr>
                    <th>NF</th>
                    <th style={{ textAlign: 'right' }}>Valor</th>
                    <th>Tickets</th>
                    <th style={{ textAlign: 'right' }}>Recebido</th>
                    <th style={{ textAlign: 'right' }}>Pendente</th>
                  </tr>
                </thead>
                <tbody>
                  {!notas.length && (
                    <tr><td colSpan={5}><p className="orc-vazio">Nenhuma nota com esse texto.</p></td></tr>
                  )}
                  {notas.map(n => (
                    <tr key={n.documento_id}>
                      <td>
                        <span className="orc-nome">{n.nf ?? '—'}</span>
                        <span className="orc-detalhe">
                          {n.tipo}{n.fornecedor ? ' · ' + n.fornecedor : ''}
                          {n.emissao ? ' · ' + emData(n.emissao) : ''}
                        </span>
                      </td>
                      <td style={{ textAlign: 'right' }}>{emReais(n.valor)}</td>
                      <td>
                        {/* O LINK QUE ABRE OS TICKETS DA NOTA
                            Nota sem ticket não vira link: um link que abre uma
                            lista vazia é um clique que a pessoa dá duas vezes
                            achando que não funcionou. */}
                        {n.tickets === 0
                          ? <span className="mut">nenhum</span>
                          : <button type="button" className="mut"
                            onClick={() => setNotaAberta(n)}
                            title="ver os tickets que esta nota cobre">
                            {n.tickets} ticket{n.tickets > 1 ? 's' : ''}
                          </button>}
                        {n.orcamentos === 0 && (
                          <span className="orc-selo espera" style={{ marginLeft: 6 }}>
                            sem orçamento
                          </span>
                        )}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        {n.recebido > 0 ? emReais(n.recebido) : <span className="mut">—</span>}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        {n.pendente > 0 ? emReais(n.pendente) : <span className="mut">—</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
                {notas.length > 0 && (
                  <tfoot>
                    <tr>
                      <th>total</th>
                      <th style={{ textAlign: 'right' }}>{emReais(somaNotas.valor)}</th>
                      <th />
                      <th style={{ textAlign: 'right' }}>{emReais(somaNotas.recebido)}</th>
                      <th style={{ textAlign: 'right' }}>{emReais(somaNotas.pendente)}</th>
                    </tr>
                  </tfoot>
                )}
              </table>
            </div>
          ) : (
            <div className="orc-rolagem">
              <table className="orc-tabela">
                <thead>
                  <tr>
                    <th>Ticket</th>
                    <th style={{ textAlign: 'right' }}>Valor</th>
                    <th>NF</th>
                    <th>No relatório</th>
                    <th>Pago ao fornecedor</th>
                  </tr>
                </thead>
                <tbody>
                  {!tickets.length && (
                    <tr><td colSpan={5}><p className="orc-vazio">Nenhum orçamento com esse texto.</p></td></tr>
                  )}
                  {tickets.map(o => (
                    <tr key={o.orcamento_id}>
                      <td>
                        <span className="orc-nome">
                          {o.ticket}{o.parte > 1 ? '-' + o.parte : ''}
                        </span>
                        <span className="orc-detalhe">
                          {o.loja ?? 'loja não identificada'}
                          {o.rateio ? ' · rateio' : ''}
                        </span>
                      </td>
                      <td style={{ textAlign: 'right' }}>{emReais(o.valor)}</td>
                      <td>
                        {o.nfs
                          ? <>{o.nfs}{o.nota_excluida && (
                            <span className="orc-selo espera" style={{ marginLeft: 6 }}
                              title="a nota foi excluída depois que este orçamento nasceu">
                              nota excluída
                            </span>)}</>
                          // O LEGADO PRECISA DIZER O SEU NOME
                          //   Coluna vazia sem explicação parece defeito. Dita,
                          //   é história: veio da migração e nunca teve nota
                          //   ligada aqui.
                          : o.legado
                            ? <span className="mut" title="veio da migração do sistema antigo">do legado</span>
                            : <span className="mut">—</span>}
                      </td>
                      <td>{sim(o.no_relatorio)}</td>
                      <td>{sim(o.pago)}</td>
                    </tr>
                  ))}
                </tbody>
                {tickets.length > 0 && (
                  <tfoot>
                    <tr>
                      <th>total</th>
                      <th style={{ textAlign: 'right' }}>{emReais(somaTickets.valor)}</th>
                      <th />
                      <th>{somaTickets.noRelatorio} de {tickets.length}</th>
                      <th>{somaTickets.pagos} de {tickets.length}</th>
                    </tr>
                  </tfoot>
                )}
              </table>
            </div>
          )}

          {notaAberta && (
            <div className="orc-gaveta">
              <div className="orc-lista-cab">
                <h2>Nota {notaAberta.nf ?? '—'}</h2>
                <em>
                  {emReais(notaAberta.valor)} · {notaAberta.tickets} ticket
                  {notaAberta.tickets > 1 ? 's' : ''}
                  {notaAberta.fornecedor ? ' · ' + notaAberta.fornecedor : ''}
                </em>
                <button type="button" className="mut" onClick={() => setNotaAberta(null)}>fechar</button>
              </div>
              <div className="orc-rolagem">
                <table className="orc-tabela">
                  <thead>
                    <tr>
                      <th>Ticket</th><th>Loja</th>
                      <th style={{ textAlign: 'right' }}>Valor</th>
                      <th>No relatório</th><th>Pago</th>
                    </tr>
                  </thead>
                  <tbody>
                    {/* O TICKET SEM ORÇAMENTO TAMBÉM PRECISA APARECER
                        A nota aponta para ele, e ele não gerou nada. É o caso
                        que faz material comprado nunca ser cobrado — some da
                        lista, some da conferência. */}
                    {notaAberta.ticket_numeros.map(t => {
                      const meus = daNota.filter(o => o.ticket === t)
                      if (!meus.length) {
                        return (
                          <tr key={t}>
                            <td><span className="orc-nome">{t}</span></td>
                            <td colSpan={4}>
                              <span className="orc-selo espera">nenhum orçamento gerado</span>
                            </td>
                          </tr>
                        )
                      }
                      return meus.map(o => (
                        <tr key={o.orcamento_id}>
                          <td>
                            <span className="orc-nome">
                              {o.ticket}{o.parte > 1 ? '-' + o.parte : ''}
                            </span>
                          </td>
                          <td>{o.loja ?? <span className="mut">—</span>}</td>
                          <td style={{ textAlign: 'right' }}>{emReais(o.valor)}</td>
                          <td>{sim(o.no_relatorio)}</td>
                          <td>{sim(o.pago)}</td>
                        </tr>
                      ))
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// SIM E NÃO, E NÃO UM SÍMBOLO
//   O dono foi explícito sobre a coluna: S/N. Um ✓ verde e uma célula vazia
//   parecem a mesma coisa numa impressão em preto e branco, que é como esta
//   tela vai parar numa reunião.
function sim(v: boolean) {
  return v
    ? <span className="orc-selo ok">sim</span>
    : <span className="mut">não</span>
}
