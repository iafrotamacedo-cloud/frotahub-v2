// rev 2 — Financeiro › Consolidação
//
// O QUE ESTA TELA RESPONDE
//
//	Duas perguntas que hoje só se responde abrindo três telas e uma planilha:
//
//	  "Comprei esta nota. Quanto ela já virou de orçamento?"   → aba das NOTAS
//	  "Cobrei este ticket. De que nota saiu, e já foi paga?"   → aba dos TICKETS
//
//	São o mesmo dinheiro visto dos dois lados. Ficam na mesma tela, e vêm na
//	mesma resposta do servidor, porque a pessoa clica de um lado e confere do
//	outro — e porque duas consultas em instantes diferentes podem discordar por
//	causa de um lançamento que entrou no meio.
//
// A ABA DAS NOTAS MUDOU NA MIGRAÇÃO 046
//
//	Até então ela comparava o FrotaHub contra si mesmo — a NF do Obra Prima
//	ainda não estava aqui. Agora `nf`/`valor` vêm do CSV que o Obra Prima
//	exporta (botão "importar CSV" abaixo), e a nota virou de verdade uma nota
//	do FORNECEDOR — inclusive as nota-pacote da Rodrigues (9192, 9220...) que
//	nunca tiveram `documento` 1:1 aqui. `tickets` é a associação manual desta
//	primeira carga (`obra_prima_ticket`) — sem tela própria ainda.
//
// A TROCA DE ABA NÃO VOLTA AO SERVIDOR
//
//	As duas listas já estão na mão. Trocar de aba é filtro de tela, e é isso que
//	faz o clique responder na hora. O mesmo vale para os tickets de uma nota: a
//	lista deles sai da aba 2, que já foi carregada — nenhuma ida ao banco.
//
// O QUE ESTA TELA NÃO FAZ
//
//	Não grava nada, com UMA exceção nomeada: importar o CSV do Obra Prima (aba
//	1). Fora isso é conferência — quem lê aqui vai agir em outra tela, com
//	conhecimento de causa.
import { useEffect, useMemo, useState } from 'react'
import { motor, enviarArquivos, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { emReais } from '../orcamentos/tipos'

interface Nota {
  nf: string
  fornecedor: string | null
  valor: number
  situacao: string | null
  parcelas: number
  tickets: number
  ticket_numeros: number[]
  orcado: number
  margem: number | null
  // MIGRAÇÃO 047 — marcada à mão como "não é gasto de manutenção" (o
  // exemplo real: exames médicos). Não conta nas somas do rodapé, mas
  // continua na lista — ver `somaNotas` e o botão no fim da linha.
  intrusa: boolean
  importado_em: string
}

interface RespostaIntrusa {
  nf: string
  intrusa: boolean
}

interface ResultadoImportacao {
  arquivo: string
  linhas: number
  notas: number
  valor: number
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

export function Consolidacao() {
  const [dados, setDados] = useState<Consolidado | null>(null)
  const [erro, setErro] = useState('')
  const [aba, setAba] = useState<Aba>('notas')
  const [busca, setBusca] = useState('')
  const [filtroNf, setFiltroNf] = useState<FiltroSN>('todos')
  const [filtroPago, setFiltroPago] = useState<FiltroSN>('todos')
  // A nota aberta mora aqui, não no endereço: é uma gaveta de conferência que
  // se fecha em dois segundos, não um lugar para onde alguém manda link.
  const [notaAberta, setNotaAberta] = useState<Nota | null>(null)
  const [importando, setImportando] = useState(false)
  const [recadoImportacao, setRecadoImportacao] = useState('')
  // NFs com o pedido de marcar/desmarcar em voo — trava o botão daquela
  // linha para um clique duplo não mandar dois pedidos que se cruzam.
  const [alternando, setAlternando] = useState<Set<string>>(new Set())

  const carregar = async () => {
    try {
      setDados(await motor<Consolidado>('/consolidacao'))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui montar a consolidação.')
    }
  }

  useEffect(() => { void carregar() }, [])

  // IMPORTAR SUBSTITUI, NÃO ACRESCENTA
  //   O motor faz upsert por (cliente, doc, parcela): reimportar o mesmo CSV —
  //   ou uma versão mais nova dele — atualiza as notas que já existiam em vez
  //   de duplicar. É seguro clicar de novo achando que não subiu da primeira
  //   vez.
  async function importarCSV(arquivo: File) {
    setImportando(true)
    setErro('')
    setRecadoImportacao('')
    try {
      const r = await enviarArquivos<ResultadoImportacao>('/consolidacao/obra-prima', [arquivo])
      setRecadoImportacao(
        `${r.arquivo}: ${r.notas} nota${r.notas === 1 ? '' : 's'} (${r.linhas} linha${r.linhas === 1 ? '' : 's'} do CSV), `
        + `${emReais(r.valor)} de bruto.`)
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui importar o CSV.')
    } finally {
      setImportando(false)
    }
  }

  // MARCAR/DESMARCAR NÃO RECARREGA A LISTA INTEIRA
  //   Só a nota mexida muda de estado — igual à troca de aba, ir ao servidor
  //   de novo seria uma espera sem motivo para uma resposta que já sabemos
  //   qual é (o motor devolve o novo estado, e é nele que confiamos, não no
  //   que a tela achava que era antes de perguntar).
  async function alternarIntrusa(nf: string) {
    if (alternando.has(nf)) return
    setAlternando(prev => new Set(prev).add(nf))
    try {
      const r = await motor<RespostaIntrusa>('/consolidacao/notas/intrusa', {
        metodo: 'POST',
        corpo: { nf },
      })
      setDados(d => d && {
        ...d,
        notas: d.notas.map(n => n.nf === r.nf ? { ...n, intrusa: r.intrusa } : n),
      })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui marcar/desmarcar esta nota.')
    } finally {
      setAlternando(prev => { const s = new Set(prev); s.delete(nf); return s })
    }
  }

  // O FILTRO É POR TEXTO CRU, DE PROPÓSITO
  //   Quem confere tem um número na mão — a NF no papel, o ticket no e-mail — e
  //   quer achá-lo. Filtro por coluna obrigaria a saber ANTES em qual coluna o
  //   número mora, que é justamente o que a pessoa está tentando descobrir.
  const q = busca.trim().toLowerCase()
  const notas = useMemo(
    () => !dados ? [] : !q ? dados.notas
      : dados.notas.filter(n =>
        n.nf.toLowerCase().includes(q)
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

  // A INTRUSA CONTINUA NA LISTA, MAS SAI DA SOMA
  //   Pedido do dono (03/09/2026): nota marcada como "não é gasto de
  //   manutenção" não pode contar na consolidação — mas precisa continuar
  //   visível para poder ser desmarcada "sempre que eu quiser".
  const somaNotas = useMemo(() => {
    const contam = notas.filter(n => !n.intrusa)
    return {
      valor: contam.reduce((s, n) => s + n.valor, 0),
      orcado: contam.reduce((s, n) => s + n.orcado, 0),
      semTicket: contam.filter(n => n.tickets === 0).length,
      intrusas: notas.length - contam.length,
    }
  }, [notas])

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
    <div className="orc-tela">
      <header className="hero">
        <h1>Consolidação</h1>
        <p>O que se comprou contra o que se cobrou — nota e ticket, nos dois sentidos</p>
      </header>

      {erro && <p className="erro" style={{ margin: '0 16px 8px' }}>{erro}</p>}

      {!dados ? <Carregando /> : (
        <>
          <div className="orc-barra-acoes">
            {/* AS ABAS SÃO OS DOIS SENTIDOS DA MESMA CONFERÊNCIA */}
            <button type="button" className={'orc-bt' + (aba === 'notas' ? ' forte' : '')}
              onClick={() => { setAba('notas'); setNotaAberta(null) }}>
              Obra Prima × FrotaHub
            </button>
            <button type="button" className={'orc-bt' + (aba === 'tickets' ? ' forte' : '')}
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
            {aba === 'notas' && (
              <label className="mut" style={{ display: 'inline-flex', alignItems: 'center', gap: 6, cursor: importando ? 'default' : 'pointer' }}>
                <input type="file" accept=".csv" disabled={importando} style={{ display: 'none' }}
                  onChange={e => {
                    const arquivo = e.target.files?.[0]
                    e.target.value = '' // permite escolher o MESMO arquivo de novo (reimportar)
                    if (arquivo) void importarCSV(arquivo)
                  }} />
                <span className="orc-bt">
                  {importando ? 'importando…' : 'importar CSV do Obra Prima'}
                </span>
              </label>
            )}
            <span className="pg-resumo">
              {aba === 'notas'
                ? <><b>{notas.length}</b> nota{notas.length === 1 ? '' : 's'}<u>{emReais(somaNotas.valor)}</u></>
                : <><b>{tickets.length}</b> orçamento{tickets.length === 1 ? '' : 's'}<u>{emReais(somaTickets.valor)}</u></>}
            </span>
          </div>

          {recadoImportacao && (
            <p className="orc-recado" style={{ margin: '0 16px 8px' }}>{recadoImportacao}</p>
          )}

          {/* A NOTA SEM TICKET TEM EXPLICAÇÃO, E ELA FICA NA TELA
              A associação nota×ticket desta primeira carga é manual (não tem
              tela própria ainda) — uma nota recém-importada legitimamente não
              tem ticket nenhum até alguém dizer quais são. */}
          {aba === 'notas' && somaNotas.semTicket > 0 && (
            <p className="orc-aviso-fila">
              {somaNotas.semTicket} nota{somaNotas.semTicket > 1 ? 's ainda não têm' : ' ainda não tem'} ticket
              associado — <b>orçado</b> e <b>margem</b> ficam em branco até alguém dizer a quais
              tickets ela{somaNotas.semTicket > 1 ? 's' : ''} pertence{somaNotas.semTicket > 1 ? 'm' : ''}.
            </p>
          )}

          {aba === 'notas' && somaNotas.intrusas > 0 && (
            <p className="orc-aviso-fila">
              {somaNotas.intrusas} nota{somaNotas.intrusas > 1 ? 's marcadas' : ' marcada'} como <b>intrusa</b>
              {somaNotas.intrusas > 1 ? ' não entram' : ' não entra'} no total — continua{somaNotas.intrusas > 1 ? 'm' : ''} na
              lista para poder{somaNotas.intrusas > 1 ? 'em' : ''} ser desmarcada{somaNotas.intrusas > 1 ? 's' : ''} a qualquer hora.
            </p>
          )}

          {aba === 'tickets' && somaTickets.orfaos > 0 && (
            <p className="orc-aviso-fila">
              {somaTickets.orfaos} orçamento{somaTickets.orfaos > 1 ? 's saíram' : ' saiu'} de
              nota que foi <b>excluída depois</b>. {somaTickets.orfaos > 1 ? 'Eles não aparecem' : 'Ele não aparece'} na
              aba das notas, mas a cobrança ao cliente continua de pé.
            </p>
          )}

          <div className="orc-lista">
          {aba === 'notas' ? (
            <div className="orc-rolagem">
              <table className="orc-tabela">
                <thead>
                  <tr>
                    <th>NF</th>
                    <th style={{ textAlign: 'right' }}>Valor</th>
                    <th>Tickets</th>
                    <th style={{ textAlign: 'right' }}>Orçado</th>
                    <th style={{ textAlign: 'right' }}>Margem</th>
                    <th style={{ textAlign: 'center' }}><span className="mut">intrusa</span></th>
                  </tr>
                </thead>
                <tbody>
                  {!notas.length && (
                    <tr><td colSpan={6}><p className="orc-vazio">
                      Nenhuma nota — importe o CSV do Obra Prima para começar.
                    </p></td></tr>
                  )}
                  {notas.map(n => (
                    <tr key={n.nf} style={n.intrusa ? { opacity: 0.55 } : undefined}>
                      <td>
                        <span className="orc-nome">{n.nf}</span>
                        <span className="orc-detalhe">
                          {n.fornecedor ?? 'fornecedor não identificado'}
                          {n.parcelas > 1 ? ` · ${n.parcelas} parcelas` : ''}
                          {n.situacao ? ' · ' + n.situacao : ''}
                          {n.intrusa ? ' · marcada como intrusa' : ''}
                        </span>
                      </td>
                      <td style={{ textAlign: 'right' }}>{emReais(n.valor)}</td>
                      <td>
                        {/* O LINK QUE ABRE OS TICKETS DA NOTA
                            Nota sem ticket não vira link: um link que abre uma
                            lista vazia é um clique que a pessoa dá duas vezes
                            achando que não funcionou. */}
                        {n.tickets === 0
                          ? <span className="mut">nenhum ainda</span>
                          : <button type="button" className="mut"
                            onClick={() => setNotaAberta(n)}
                            title="ver os tickets associados a esta nota">
                            {n.tickets} ticket{n.tickets > 1 ? 's' : ''}
                          </button>}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        {n.tickets > 0 ? emReais(n.orcado) : <span className="mut">—</span>}
                      </td>
                      <td style={{ textAlign: 'right' }}>
                        {n.margem === null ? <span className="mut">—</span> : emPercentual(n.margem)}
                      </td>
                      <td style={{ textAlign: 'center' }}>
                        <BotaoIntrusa marcada={n.intrusa} carregando={alternando.has(n.nf)}
                          onClick={() => void alternarIntrusa(n.nf)} />
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
                      <th style={{ textAlign: 'right' }}>{emReais(somaNotas.orcado)}</th>
                      <th style={{ textAlign: 'right' }}>
                        {somaNotas.orcado > 0 ? emPercentual((somaNotas.orcado - somaNotas.valor) / somaNotas.orcado) : '—'}
                      </th>
                      <th />
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
                <h2>Nota {notaAberta.nf}</h2>
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
          </div>
        </>
      )}
    </div>
  )
}

// O BOTÃO DA INTRUSA — PEQUENO, NO FIM DA LINHA, ÍCONE DE "LADRÃO"
//   Pedido do dono, verbatim: "aquela imagem do ladrão, típica de
//   ilustrações, um botão pequeno". Desenhado à mão no mesmo traço fino dos
//   ícones do menu (`componentes/Icone.tsx`) para não destoar — sem
//   biblioteca externa, sem preencher a tela de dependência por um botão
//   deste tamanho. Cor muda com o estado: apagado quando a nota conta
//   normalmente, forte (vermelho) quando está marcada — a mesma lógica de
//   "estado ligado tem cor, estado desligado é `mut`" do resto da tela.
function BotaoIntrusa({ marcada, carregando, onClick }: {
  marcada: boolean
  carregando: boolean
  onClick: () => void
}) {
  return (
    <button type="button" onClick={onClick} disabled={carregando}
      title={marcada ? 'desmarcar: voltar a contar esta nota na consolidação' : 'marcar como intrusa: não é gasto de manutenção, tirar do total'}
      style={{
        background: 'none', border: 'none', padding: 2, lineHeight: 0,
        cursor: carregando ? 'default' : 'pointer',
        color: marcada ? '#b3261e' : '#9a9a9a',
        opacity: carregando ? 0.5 : 1,
      }}>
      <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor"
        strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        {/* boné puxado, máscara e o saco de roubo nas costas — o "ladrão" de ilustração */}
        <path d="M6.4 10.2c0-3.4 2.5-6 5.6-6s5.6 2.6 5.6 6" />
        <path d="M4.6 10.4c1.6-1 4-1.6 7.4-1.6s5.8.6 7.4 1.6c.5.3.3 1-.3 1-1 0-1.6.5-1.6 1.4v1.6c0 2.7-2.5 4.9-5.5 4.9s-5.5-2.2-5.5-4.9v-1.6c0-.9-.6-1.4-1.6-1.4-.6 0-.8-.7-.3-1Z" />
        <circle cx="9.8" cy="13.2" r="1" fill="currentColor" stroke="none" />
        <circle cx="14.2" cy="13.2" r="1" fill="currentColor" stroke="none" />
        <path d="M17.5 13.5c1.6.3 2.8 1.3 3.1 2.8.3 1.6-.9 2.9-2.5 2.6" />
      </svg>
    </button>
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

// A VIEW MANDA A RAZÃO CRUA; A TELA MULTIPLICA E FORMATA
//   Mesma regra da migração 043 ("este módulo não calcula nada"): o motor
//   entrega (orçado−valor)/orçado sem tocar em ×100 nem em casa decimal.
//   Negativo é legítimo aqui — quer dizer que o orçado ficou ABAIXO do valor
//   da nota, ou seja, prejuízo — e por isso o sinal não é escondido.
function emPercentual(fracao: number): string {
  return (fracao * 100).toLocaleString('pt-BR', { minimumFractionDigits: 1, maximumFractionDigits: 1 }) + '%'
}
