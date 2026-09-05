// rev 2 — Lançar orçamentos (2.3)
//
// Esta tela LANÇA. Só isso.
//
// O BOTÃO DE GERAR SAIU DAQUI, E NÃO FOI SÓ ARRUMAÇÃO
//
//	Ele era o único botão vermelho da tela — o lugar onde, em todo o resto do
//	sistema, mora a ação principal. Numa tela chamada "Lançar orçamentos", com 72
//	esperando para subir, o botão mais destacado fazia outra coisa.
//
//	E fazia errado. O botão de gerar já existe na tela de Notas, e de lá ele
//	manda a `fila` junto: quem está no rateio gera o rateio e não encosta na
//	outra fila. Daqui ele mandava o corpo vazio — que no motor significa "as
//	duas filas". Ou seja: a porta duplicada não era nem uma cópia da outra, era
//	a versão que mistura o que o campo `fila` existe para separar (CORE-06).
//
// LANÇAR EM LOTE É O NAVEGADOR CHAMANDO A MESMA ROTA, UMA A UMA
//
//	Não existe rota de lote no motor, de propósito. Setenta e dois lançamentos
//	são setenta e duas gerações de PDF, subidas ao R2 e chamadas ao Trílogo: numa
//	requisição só, isso estoura qualquer tempo limite e não mostra nada enquanto
//	roda. Aqui o laço é da tela — ela vê o andamento, dá para parar no meio, e a
//	regra de lançar continua existindo UMA vez, na rota de sempre.
//
// QUEM ENTRA NO LOTE
//
//	Só `destino === 'pode_lancar'`, que a view calcula do status do ticket AGORA.
//	Não é a flag de bloqueio antiga: medimos 7 de 26 bloqueados destravando
//	sozinhos em seis dias. Tentar um chamado Arquivado é gastar uma subida de PDF
//	para receber a mesma recusa de ontem.
import { useCallback, useEffect, useRef, useState } from 'react'
import { motor } from '../../motor/cliente'
import { FichaDoOrcamento } from './FichaDoOrcamento'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta, Paginacao } from './Arquivos'
import {
  emReais, emDataHora, contaPorExtenso,
  type Orcamento, type Pagina,
} from './tipos'

export function Lancar({ voltar }: { voltar: () => void }) {
  const [pagina, setPagina] = useState<Pagina<Orcamento> | null>(null)
  const [numero, setNumero] = useState(1)
  const [por, setPor] = useState(100)
  const [status, setStatus] = useState('gerado')
  // A FILA É O QUE DÁ PARA FAZER AGORA
  //
  //   Medido em 27/08/2026: 93 orçamentos gerados, 5 podendo subir. A tela
  //   mostrava os 93 e o "lançar todos" percorria os 93, para 88 levarem a mesma
  //   recusa de sempre. O dono resumiu: "pode clicar até morrer que não adianta
  //   nada".
  //
  //   Agora ela abre com os que podem subir, e os outros têm o seu lugar —
  //   Pendências, que é a tela onde existe o que FAZER com eles. A caixinha
  //   continua permitindo ver a fila inteira, porque esconder o total seria
  //   trocar um problema por outro.
  const [soOsQuePodem, setSoOsQuePodem] = useState(true)
  const [erro, setErro] = useState('')
  const [aviso, setAviso] = useState('')
  const [lancando, setLancando] = useState<string | null>(null)
  // VER É ABRIR, NÃO BAIXAR
  //   O botão desta linha baixava o PDF. Conferir cinco orçamentos deixava cinco
  //   cópias na pasta de Downloads que ninguém queria guardar. Agora abre a
  //   ficha, e o salvar mora lá dentro — com escolha de pasta.
  const [abrindo, setAbrindo] = useState<string | null>(null)
  const [bloqueio, setBloqueio] = useState<BloqueioDoTeto | null>(null)
  // A DUPLICATA GUARDA O ORÇAMENTO, E NÃO SÓ A MENSAGEM
  //   Sem ele, o "lançar mesmo assim" não teria em quem mandar — e a única saída
  //   da tela seria fechar e clicar de novo na linha, torcendo para ser a certa.
  const [duplicata, setDuplicata] = useState<{ dados: BloqueioDeDuplicata; orc: Orcamento } | null>(null)
  const [lote, setLote] = useState<Lote | null>(null)
  // Parar é um `ref`, não um estado: o laço já está rodando quando a pessoa
  // clica, e um estado só chegaria nele no próximo desenho da tela.
  const parar = useRef(false)

  const carregar = useCallback(async () => {
    try {
      const q = new URLSearchParams({ pagina: String(numero), por: String(por), status })
      if (soOsQuePodem && status === 'gerado') {
        q.set('destino', 'pode_lancar')
        // Os travados por teto ou duplicidade moram em Pendências › Esperando
        // decisão. Aqui eles só apareceriam para não poder subir.
        q.set('decisao', 'nao')
      }
      setPagina(await motor<Pagina<Orcamento>>('/orcamentos?' + q))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar a lista.')
    }
  }, [numero, por, status, soOsQuePodem])

  useEffect(() => { void carregar() }, [carregar])

  // subirUm é a única chamada de lançamento da tela. O botão da linha e o lote
  // passam os dois por aqui — assim não existe uma segunda versão da regra que
  // um dia deixa de concordar com a primeira.
  //  A CONFIRMAÇÃO DE DUPLICATA É UM ARGUMENTO, E NUNCA UM PADRÃO
  //    `confirmandoDuplicata` só chega aqui como `true` vindo do botão que a
  //    pessoa aperta com o custo existente na frente. O lote chama sem ele.
  async function subirUm(o: Orcamento, confirmandoDuplicata = false): Promise<Resultado> {
    try {
      const r = await motor<{ trilogo_custo_id: number }>(
        `/orcamentos/ficha/${o.id}/lancar` + (confirmandoDuplicata ? '?duplicata=confirmo' : ''),
        { metodo: 'POST' })
      return { ok: true, custo: r.trilogo_custo_id }
    } catch (e) {
      const corpo = (e as { corpo?: QualquerBloqueio }).corpo
      // QUEM RECUSA DIZ O NOME — A TELA NÃO ADIVINHA MAIS
      //
      //   Antes, a escolha era pelo FORMATO: "o corpo tem `saidas`? então é
      //   teto". Três recusas diferentes têm `saidas`, e a do status do chamado
      //   caía na primeira. Medido em 26/08/2026, num lote de 56: dezessete
      //   chamados Arquivados apareceram como "passou do teto: – já no ticket
      //   + – deste, teto –". Os travessões eram `emReais(undefined)` — os
      //   números do teto não existem naquela resposta. A tela mandou a pessoa
      //   caçar um teto que nunca tinha estourado.
      //
      //   Agora o motor manda `bloqueio` com a mesma palavra que grava na coluna
      //   `lancamento_bloqueio`, e cada uma tem o seu desenho.
      switch (corpo?.bloqueio) {
        case 'possivel_duplicata':
          return { ok: false, duplicata: corpo, motivo: fraseDaDuplicata(corpo) }
        case 'teto':
          return { ok: false, teto: corpo, motivo: frasedoTeto(corpo) }
        case 'ticket_status':
          return { ok: false, motivo: fraseDoStatus(corpo) }
      }
      return { ok: false, motivo: e instanceof Error ? e.message : 'Não consegui lançar.' }
    }
  }

  async function lancar(o: Orcamento, confirmandoDuplicata = false) {
    setLancando(o.id)
    setAviso('')
    if (confirmandoDuplicata) setDuplicata(null)
    const r = await subirUm(o, confirmandoDuplicata)
    setLancando(null)
    if (r.ok) {
      setAviso(`Ticket ${o.ticket}: lançado no Trílogo (custo nº ${r.custo}).`)
      await carregar()
      return
    }
    // O CONFLITO DE TETO NÃO É UM ERRO QUALQUER
    //   Ele vem com números e com saídas. Mostrar só a frase jogaria fora
    //   justamente a parte que resolve — e é assim que o usuário acaba
    //   montando planilha paralela para entender o que travou.
    if (r.duplicata) setDuplicata({ dados: r.duplicata, orc: o })
    else if (r.teto) setBloqueio(r.teto)
    else setErro(r.motivo)
    await carregar()
  }

  async function lancarTodos() {
    const fila = (pagina?.linhas ?? []).filter(podeSubir)
    if (fila.length === 0) return
    parar.current = false
    setAviso('')
    setErro('')
    setLote({ total: fila.length, feito: 0, subiram: 0, travados: [], rodando: true })

    for (const o of fila) {
      if (parar.current) break
      setLote(l => (l ? { ...l, agora: o.ticket } : l))
      const r = await subirUm(o)
      setLote(l => {
        if (!l) return l
        return {
          ...l,
          feito: l.feito + 1,
          subiram: l.subiram + (r.ok ? 1 : 0),
          travados: r.ok ? l.travados : [...l.travados, { ticket: o.ticket, parte: o.parte, motivo: r.motivo }],
        }
      })
      // Uma pausa curta entre um e outro. O Trílogo é o sistema DO CLIENTE:
      // setenta e duas chamadas coladas parecem ataque, mesmo sendo trabalho.
      await new Promise(r => setTimeout(r, 400))
    }

    setLote(l => (l ? { ...l, rodando: false, agora: undefined } : l))
    await carregar()
  }

  // APAGAR É O CAMINHO DE VOLTA DE UM ORÇAMENTO ERRADO
  //
  //   Antes desta tela não havia nenhum: um orçamento gerado com valor errado
  //   ficava lá, e a nota atrás dele ficava presa em `usado`, fora da fila e
  //   sem orçamento válido. A saída era SQL na mão.
  //
  //   O motivo é obrigatório de propósito. Quem abrir isto daqui a seis meses
  //   vai encontrar um orçamento removido e uma nota reorçada com outro valor;
  //   sem a frase, não tem como saber se foi correção, arrependimento ou erro.
  async function apagar(o: Orcamento) {
    const motivo = window.prompt(
      `Apagar o orçamento do ticket ${o.ticket} (${emReais(o.valor)})?\n\n` +
      'A nota volta para a fila em Notas e DAVs e pode ser orçada de novo.\n' +
      'Diga por quê — fica registrado com o seu nome.')
    if (!motivo?.trim()) return
    try {
      await motor(`/orcamentos/ficha/${o.id}?motivo=${encodeURIComponent(motivo.trim())}`,
        { metodo: 'DELETE' })
      setErro('')
      setAviso(`Ticket ${o.ticket}: orçamento apagado. A nota voltou para a fila em Notas e DAVs.`)
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui apagar.')
    }
  }

  async function aprovar(o: Orcamento) {
    const nota = window.prompt(
      `Onde está a aprovação do cliente para o ticket ${o.ticket} (${emReais(o.valor)})?\n` +
      'Escreva a referência — e-mail, conversa, print. Fica registrado com o seu nome.')
    if (!nota) return
    try {
      await motor(`/orcamentos/ficha/${o.id}/aprovar`, { metodo: 'POST', corpo: { nota } })
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui aprovar.')
    }
  }


  // Quantos desta PÁGINA podem subir agora. Da página, e não do banco inteiro:
  // o botão só mexe no que está à vista, e é isso que ele promete no rótulo.
  const prontos = (pagina?.linhas ?? []).filter(podeSubir).length
  // Os que estão na lista e o lote não vai tentar. Contá-los é o que permite a
  // tela dizer por que o botão promete menos do que a lista mostra.
  const precisamDeDecisao = (pagina?.linhas ?? [])
    .filter(o => o.status === 'gerado' && o.destino === 'pode_lancar' && precisaDeDecisao(o)).length

  // Voltar da ficha recarrega: o orçamento pode ter sido apagado lá dentro, e
  // uma lista que mostra o que não existe mais é pior que uma lista lenta.
  if (abrindo) {
    return (
      <FichaDoOrcamento
        id={abrindo}
        voltar={() => { setAbrindo(null); void carregar() }}
      />
    )
  }

  return (
    <div className="orc-tela">
      <BarraDeVolta
        voltar={voltar}
        titulo="Lançar orçamentos"
        direita={
          lote?.rodando ? (
            <button type="button" className="orc-bt" onClick={() => { parar.current = true }}>
              Parar depois deste
            </button>
          ) : (
            <button
              type="button"
              // Sem nada para subir ele deixa de ser vermelho. Um botão da cor
              // da ação principal, só apagado, continua puxando o olho para um
              // clique que não existe.
              className={prontos === 0 ? 'orc-bt' : 'orc-bt forte'}
              disabled={prontos === 0}
              title={
                prontos === 0
                  ? 'Nenhum orçamento desta lista pode subir agora — o chamado precisa estar Executado ou Vistoriado.'
                  : 'Sobe um a um, na ordem da lista. Dá para parar no meio.'
              }
              onClick={() => void lancarTodos()}
            >
              {prontos === 0 ? 'Nada pode subir agora' : `Lançar todos que podem subir (${prontos})`}
            </button>
          )
        }
      />

      {lote && <Lote dados={lote} fechar={() => setLote(null)} />}
      {aviso && <p className="orc-ok">{aviso}</p>}
      {erro && <p className="erro">{erro}</p>}
      {bloqueio && <Bloqueio dados={bloqueio} fechar={() => setBloqueio(null)} />}
      {duplicata && (
        <Duplicata
          dados={duplicata.dados}
          lancando={lancando === duplicata.orc.id}
          fechar={() => setDuplicata(null)}
          lancarMesmoAssim={() => void lancar(duplicata.orc, true)}
        />
      )}

      <div className="orc-lista">
        <div className="orc-lista-cab">
          <h2>Orçamentos</h2>
          <em>{pagina ? `${pagina.total.toLocaleString('pt-BR')} no filtro` : '—'}</em>
          <select value={status} onChange={e => { setNumero(1); setStatus(e.target.value) }}>
            <option value="gerado">a lançar</option>
            <option value="aguardando_aprovacao">aguardando aprovação</option>
            <option value="lancado">lançados</option>
            <option value="removido">apagados</option>
          </select>
          {/* A CAIXINHA EXISTE PARA O TOTAL NÃO SUMIR
              Esconder os travados resolve a fila e cria outra pergunta sem
              resposta: "e os outros?". Aqui ela é respondida em uma linha, com
              o caminho de quem quer ir atrás. */}
          {status === 'gerado' && (
            <label className="orc-so-podem">
              <input
                type="checkbox"
                checked={soOsQuePodem}
                onChange={e => { setNumero(1); setSoOsQuePodem(e.target.checked) }}
              />
              só os que podem subir
            </label>
          )}
        </div>

        {status === 'gerado' && soOsQuePodem && precisamDeDecisao > 0 && (
          <p className="orc-aviso-fila">
            <strong>{precisamDeDecisao}</strong>{' '}
            {precisamDeDecisao === 1
              ? 'orçamento desta lista precisa de uma decisão sua'
              : 'orçamentos desta lista precisam de uma decisão sua'}{' '}
            — duplicidade ou teto. O botão de lote não mexe neles: use o botão da
            linha, com o motivo na frente.
          </p>
        )}

        {status === 'gerado' && soOsQuePodem && (
          <p className="orc-aviso-fila">
            Esta lista mostra só o que dá para lançar agora. O que está travado
            pelo status do chamado mora em <strong>Pendências</strong> — lá dá para
            extrair a lista para cobrar, e reconferir no Trílogo quem já liberou.
          </p>
        )}

        {!pagina ? <Carregando /> : (
          <div className="orc-rolagem">
            <table className="orc-tabela">
              <colgroup>
                <col style={{ width: '10%' }} />
                <col style={{ width: '24%' }} />
                <col style={{ width: '14%' }} />
                <col style={{ width: '12%' }} />
                <col style={{ width: '14%' }} />
                <col />
              </colgroup>
              <thead>
                <tr>
                  <th>Ticket</th><th>Loja</th><th>Nota</th>
                  <th style={{ textAlign: 'right' }}>Valor</th>
                  <th>Gerado em</th>
                  <th style={{ textAlign: 'right' }}>Ações</th>
                </tr>
              </thead>
              <tbody>
                {pagina.linhas.length === 0 && (
                  <tr><td colSpan={6} className="orc-vazio">Nada por aqui com este filtro.</td></tr>
                )}
                {pagina.linhas.map(o => (
                  <tr key={o.id}>
                    <td>
                      <span className="orc-nome">{o.ticket}{o.parte > 1 ? `-${o.parte}` : ''}</span>
                      <span className="orc-detalhe">{contaPorExtenso(o.conta)}{o.rateio ? ' · rateio' : ''}</span>
                      {/* POR QUE ESTE NÃO ENTRA NO LOTE
                          Sem isto, "Lançar todos (47)" numa lista de 72 vira
                          mistério — e mistério em botão faz a pessoa clicar
                          duas vezes para ver se muda. O status do chamado é a
                          resposta inteira. */}
                      {o.status === 'gerado' && !podeSubir(o) && (
                        <span className="orc-detalhe" title={o.lancamento_bloqueio_detalhe ?? undefined}>
                          {porqueNaoSobe(o)}
                        </span>
                      )}
                    </td>
                    <td title={o.chamado_descricao ?? undefined}>
                      <span className="orc-nome">{o.loja ?? '–'}</span>
                      {/* A DESCRIÇÃO DO CHAMADO SAIU DAQUI
                          Ela vem do Trílogo sem limite de tamanho — um chamado
                          de quatro parágrafos virava uma linha de meia tela, e
                          duas linhas assim já enchiam a página. Quem está
                          lançando precisa de ticket, loja e valor; o texto do
                          chamado é contexto, e contexto não pode empurrar a
                          informação para fora da vista. Continua no `title`,
                          para ler passando o mouse. */}
                    </td>
                    <td>{o.notas ?? '–'}</td>
                    <td style={{ textAlign: 'right' }}>
                      <span className="orc-nome">{emReais(o.valor)}</span>
                      {o.reduzido_pelo_teto && (
                        <span className="orc-detalhe" title="reduzido para caber no teto do ticket">
                          era {emReais(o.valor_antes_do_teto)}
                        </span>
                      )}
                      {/* O CARIMBO DO DESCONTO — NOSSO, NUNCA DO CLIENTE
                          Um orçamento de R$ 600 para uma nota de R$ 700 não se
                          explica sozinho. Sem esta linha, quem abrir isto daqui
                          a seis meses não tem como saber se foi regra, erro de
                          digitação ou coisa pior — e vai ter que abrir o PDF da
                          nota para descobrir.

                          Fica só aqui, na tela de quem trabalha. O documento
                          que sai para o cliente mostra o preço cobrado, e de
                          onde ele veio é assunto nosso. */}
                      {o.ajustado_pelo_teto && (
                        <span className="orc-detalhe aviso"
                          title={`a nota soma ${emReais(o.valor_nota_cheio)}, pagamos ${emReais(o.valor_nota)}, `
                            + 'e o orçamento foi fechado no teto do ticket'}>
                          ajustada pelo teto · nota {emReais(o.valor_nota_cheio)}
                        </span>
                      )}
                    </td>
                    <td>{emDataHora(o.criado_em)}</td>
                    <td className="orc-acoes">
                      <button type="button" onClick={() => setAbrindo(o.id)}>ver</button>
                      {o.status === 'gerado' && (
                        <button type="button" className="forte" disabled={lancando === o.id} onClick={() => void lancar(o)}>
                          {lancando === o.id ? 'lançando…' : 'lançar'}
                        </button>
                      )}
                      {o.status === 'aguardando_aprovacao' && (
                        <button type="button" className="forte" onClick={() => void aprovar(o)}>aprovar</button>
                      )}
                      {o.status === 'lancado' && (
                        <span className="orc-selo ok">custo {o.trilogo_custo_id}</span>
                      )}
                      {/* Um lançado não tem "apagar": o custo está vivo no
                          Trílogo, e apagar só aqui deixaria os dois sistemas
                          discordando. O motor recusa também — o botão some para
                          não prometer o que a rota nega. */}
                      {o.status !== 'lancado' && (
                        <button type="button" className="perigo" onClick={() => void apagar(o)}>apagar</button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

      </div>
      <Paginacao pagina={pagina} por={por} aoTrocarPagina={setNumero} aoTrocarPor={n => { setPor(n); setNumero(1) }} />
    </div>
  )
}

interface BloqueioDoTeto {
  bloqueio: 'teto'
  erro: string
  ticket: number
  ja_no_ticket: number
  deste: number
  teto: number
  saidas: string[]
}

/** O bloqueio do teto, com as contas e as saídas visíveis. Nunca um beco. */
function Bloqueio({ dados, fechar }: { dados: BloqueioDoTeto; fechar: () => void }) {
  return (
    <div className="orc-bloqueio">
      <h3>Ticket {dados.ticket} passou do teto</h3>
      <p>{dados.erro}</p>
      <table>
        <tbody>
          <tr><td>já lançado no ticket</td><td>{emReais(dados.ja_no_ticket)}</td></tr>
          <tr><td>este orçamento</td><td>{emReais(dados.deste)}</td></tr>
          <tr><td>teto contratado</td><td>{emReais(dados.teto)}</td></tr>
        </tbody>
      </table>
      <p className="orc-saidas">O que dá para fazer:</p>
      <ul>{dados.saidas.map(s => <li key={s}>{s}</li>)}</ul>
      <button type="button" className="orc-bt" onClick={fechar}>Entendi</button>
    </div>
  )
}

interface CustoLa {
  id: number
  valor: number
  emissao: string
  tipo: string
  documento: string
  permalink?: string
}

interface BloqueioDeDuplicata {
  bloqueio: 'possivel_duplicata'
  erro: string
  ticket: number
  deste: number
  iguais: CustoLa[]
  saidas: string[]
}

/** O ticket JÁ TEM um custo deste valor.
 *
 *  POR QUE ISTO NÃO É UM ERRO VERMELHO E PRONTO
 *    Duas notas iguais no mesmo serviço acontecem. O que não pode acontecer é
 *    lançar a segunda SEM SABER que a primeira está lá. Então a tela mostra o
 *    custo que existe — número, data, valor e o documento — e deixa a decisão
 *    com quem consegue abrir os dois e comparar.
 *
 *  O BOTÃO DE SEGUIR NÃO É O DESTAQUE
 *    "Não lançar" é o certo na maioria das vezes. Ele fica em primeiro e com o
 *    peso visual; seguir é a exceção, e tem que parecer uma. */
function Duplicata({ dados, lancando, fechar, lancarMesmoAssim }: {
  dados: BloqueioDeDuplicata
  lancando: boolean
  fechar: () => void
  lancarMesmoAssim: () => void
}) {
  return (
    <div className="orc-bloqueio orc-duplicata">
      <h3>Ticket {dados.ticket} já tem um custo deste valor</h3>
      <p>{dados.erro}</p>
      <table>
        <thead>
          <tr><th>custo</th><th>valor</th><th>emissão</th><th>tipo</th><th /></tr>
        </thead>
        <tbody>
          {dados.iguais.map(c => (
            <tr key={c.id}>
              <td>nº {c.id}</td>
              <td>{emReais(c.valor)}</td>
              <td>{c.emissao || '—'}</td>
              <td>{c.tipo || '—'}</td>
              <td>
                {c.permalink
                  ? <a href={c.permalink} target="_blank" rel="noreferrer">ver o documento</a>
                  : <span className="orc-detalhe">sem documento</span>}
              </td>
            </tr>
          ))}
          <tr className="orc-duplicata-este">
            <td>este orçamento</td>
            <td>{emReais(dados.deste)}</td>
            <td colSpan={3}>ainda não lançado</td>
          </tr>
        </tbody>
      </table>
      <p className="orc-saidas">O que dá para fazer:</p>
      <ul>{dados.saidas.map(s => <li key={s}>{s}</li>)}</ul>
      <div className="orc-duplicata-bts">
        <button type="button" className="orc-bt forte" onClick={fechar}>Não lançar</button>
        <button type="button" className="orc-bt" disabled={lancando} onClick={lancarMesmoAssim}>
          {lancando ? 'Lançando…' : 'São notas diferentes — lançar assim mesmo'}
        </button>
      </div>
    </div>
  )
}

/** O chamado não aceita custo AGORA — e o que trava não é dinheiro, é o estado
 *  do chamado no Trílogo. A frase diz o status e quem destrava, porque "não
 *  aceita" sem dono é o tipo de recado que ninguém age em cima. */
interface BloqueioDoTicket {
  bloqueio: 'ticket_status'
  erro: string
  ticket: number
  ticket_status: string
  quem_resolve?: string
}

/** As recusas que o motor sabe nomear. O `bloqueio` é a etiqueta comum: o mesmo
 *  texto que vai para a coluna `lancamento_bloqueio` no banco. */
type QualquerBloqueio = BloqueioDoTeto | BloqueioDeDuplicata | BloqueioDoTicket

function fraseDoStatus(b: BloqueioDoTicket): string {
  const dono = b.quem_resolve ? ` — cobrar de ${b.quem_resolve}` : ''
  return `chamado ${b.ticket_status}: o Trílogo não aceita custo nesse status${dono}`
}

function fraseDaDuplicata(b: BloqueioDeDuplicata): string {
  const ids = b.iguais.map(c => `nº ${c.id}`).join(', ')
  return `o ticket já tem ${emReais(b.deste)} lançado (${ids}) — pode ser duplicata`
}

// ---------------------------------------------------------------------------
// o lote
// ---------------------------------------------------------------------------

interface Travado {
  ticket: number
  parte: number
  motivo: string
}

interface Lote {
  total: number
  feito: number
  subiram: number
  travados: Travado[]
  rodando: boolean
  agora?: number
}

type Resultado =
  | { ok: true; custo: number }
  | { ok: false; motivo: string; teto?: BloqueioDoTeto; duplicata?: BloqueioDeDuplicata }

/** Entra no lote quem a view diz que pode subir AGORA. */
/** Entra no LOTE quem sobe sem perguntar nada a ninguém.
 *
 *  O BOTÃO NÃO PODE PROMETER O QUE NÃO VAI ENTREGAR
 *    Medido em 27/08/2026: a fila tinha 5 "podem subir", o botão dizia
 *    "Lançar todos que podem subir (5)", e o resultado foi **0 de 5 subiram** —
 *    três barrados por duplicidade e dois pelo teto. Os dois travas fizeram o
 *    que deviam; quem errou foi o botão, que contou como pronto o que precisa de
 *    uma pessoa decidindo.
 *
 *    É a mesma lição de um nível acima, quando a fila mostrava 93 e só 5 davam
 *    para trabalhar. Agora ela vale dentro da própria fila.
 *
 *  ELES NÃO SOMEM DA LISTA
 *    Continuam ali, com o motivo escrito e o botão da linha — que é onde a
 *    decisão cabe: ver o custo que já está no ticket e dizer se é duplicata,
 *    ou aprovar o que passou do teto. O que o lote não faz é decidir por
 *    ninguém. */
function podeSubir(o: Orcamento): boolean {
  return o.status === 'gerado' && o.destino === 'pode_lancar' && !precisaDeDecisao(o)
}

/** Os bloqueios que só uma pessoa desfaz, olhando o caso.
 *
 *  `ticket_status` e `trilogo_fora` NÃO entram aqui de propósito: aqueles
 *  destravam sozinhos quando o mundo anda, e tentar de novo é exatamente o
 *  certo. Estes dois não andam sozinhos — o ticket vai continuar com o mesmo
 *  custo lançado, e o teto vai continuar sendo R$ 600. */
function precisaDeDecisao(o: Orcamento): boolean {
  // A LISTA DE QUAIS BLOQUEIOS SÃO ESTES MORA NA VIEW
  //   Ela nasceu aqui, no navegador, em 27/08. No mesmo dia o motor e o balanço
  //   passaram a precisar da mesma resposta — e três cópias da mesma lista é
  //   como duas delas um dia discordam (CORE-06). Agora o banco responde, e
  //   esta função só lê. O `??` cobre a resposta de um motor antigo, em que a
  //   coluna ainda não existe.
  return o.precisa_decisao
    ?? (o.lancamento_bloqueio === 'possivel_duplicata' || o.lancamento_bloqueio === 'teto')
}

/** A frase curta de por que este ficou de fora. Sai do estado de agora, não da
 *  recusa de ontem — o chamado anda sem avisar. */
function porqueNaoSobe(o: Orcamento): string {
  if (o.destino === 'sem_chamado') return 'chamado não encontrado'
  if (o.reaberto) return `${o.ticket_status ?? 'reaberto'} · reaberto`
  if (o.ticket_status) return o.ticket_status
  return 'ainda não pode subir'
}

function frasedoTeto(b: BloqueioDoTeto): string {
  return `passou do teto: ${emReais(b.ja_no_ticket)} já no ticket + ${emReais(b.deste)} deste, teto ${emReais(b.teto)}`
}

/** O andamento e, no fim, o que travou.
 *
 *  O PAINEL NÃO SOME SOZINHO
 *    Setenta e dois lançamentos levam minutos. Quem sai da frente da tela e
 *    volta precisa encontrar o resultado ainda ali — um aviso que se apaga
 *    transforma "o que travou?" numa pergunta sem resposta. */
function Lote({ dados, fechar }: { dados: Lote; fechar: () => void }) {
  const pct = dados.total === 0 ? 0 : Math.round((dados.feito / dados.total) * 100)
  return (
    <div className="orc-lote">
      <div className="orc-lote-topo">
        <strong>
          {dados.rodando
            ? `Lançando ${dados.feito + 1} de ${dados.total}${dados.agora ? ` · ticket ${dados.agora}` : ''}`
            : `${dados.subiram} de ${dados.total} subiram`}
        </strong>
        {!dados.rodando && (
          <button type="button" className="orc-bt" onClick={fechar}>Fechar</button>
        )}
      </div>

      <div className="orc-lote-barra"><span style={{ width: `${pct}%` }} /></div>

      {!dados.rodando && dados.travados.length > 0 && (
        <>
          <p className="orc-lote-titulo">
            {dados.travados.length === 1 ? 'Um travou:' : `${dados.travados.length} travaram:`}
          </p>
          <ul className="orc-lote-travados">
            {dados.travados.map(t => (
              <li key={`${t.ticket}-${t.parte}`}>
                <strong>{t.ticket}{t.parte > 1 ? `-${t.parte}` : ''}</strong> {t.motivo}
              </li>
            ))}
          </ul>
          <p className="orc-lote-rodape">
            Cada um destes ficou com o motivo gravado, e continua nesta lista com o
            botão da linha — é ali que a decisão cabe.
          </p>
        </>
      )}
      {!dados.rodando && dados.travados.length === 0 && dados.feito > 0 && (
        <p className="orc-lote-rodape">Nenhum travou.</p>
      )}
    </div>
  )
}
