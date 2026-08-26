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
  const [erro, setErro] = useState('')
  const [aviso, setAviso] = useState('')
  const [lancando, setLancando] = useState<string | null>(null)
  // VER É ABRIR, NÃO BAIXAR
  //   O botão desta linha baixava o PDF. Conferir cinco orçamentos deixava cinco
  //   cópias na pasta de Downloads que ninguém queria guardar. Agora abre a
  //   ficha, e o salvar mora lá dentro — com escolha de pasta.
  const [abrindo, setAbrindo] = useState<string | null>(null)
  const [bloqueio, setBloqueio] = useState<BloqueioDoTeto | null>(null)
  const [lote, setLote] = useState<Lote | null>(null)
  // Parar é um `ref`, não um estado: o laço já está rodando quando a pessoa
  // clica, e um estado só chegaria nele no próximo desenho da tela.
  const parar = useRef(false)

  const carregar = useCallback(async () => {
    try {
      const q = new URLSearchParams({ pagina: String(numero), por: String(por), status })
      setPagina(await motor<Pagina<Orcamento>>('/orcamentos?' + q))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar a lista.')
    }
  }, [numero, por, status])

  useEffect(() => { void carregar() }, [carregar])

  // subirUm é a única chamada de lançamento da tela. O botão da linha e o lote
  // passam os dois por aqui — assim não existe uma segunda versão da regra que
  // um dia deixa de concordar com a primeira.
  async function subirUm(o: Orcamento): Promise<Resultado> {
    try {
      const r = await motor<{ trilogo_custo_id: number }>(
        `/orcamentos/ficha/${o.id}/lancar`, { metodo: 'POST' })
      return { ok: true, custo: r.trilogo_custo_id }
    } catch (e) {
      const corpo = (e as { corpo?: BloqueioDoTeto }).corpo
      if (corpo?.saidas) return { ok: false, teto: corpo, motivo: frasedoTeto(corpo) }
      return { ok: false, motivo: e instanceof Error ? e.message : 'Não consegui lançar.' }
    }
  }

  async function lancar(o: Orcamento) {
    setLancando(o.id)
    setAviso('')
    const r = await subirUm(o)
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
    if (r.teto) setBloqueio(r.teto)
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
        </div>

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

        <Paginacao pagina={pagina} por={por} aoTrocarPagina={setNumero} aoTrocarPor={n => { setPor(n); setNumero(1) }} />
      </div>
    </div>
  )
}

interface BloqueioDoTeto {
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
  | { ok: false; motivo: string; teto?: BloqueioDoTeto }

/** Entra no lote quem a view diz que pode subir AGORA. */
function podeSubir(o: Orcamento): boolean {
  return o.status === 'gerado' && o.destino === 'pode_lancar'
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
            Cada um destes ficou com o motivo gravado e aparece em Correções ▸ Recusados.
          </p>
        </>
      )}
      {!dados.rodando && dados.travados.length === 0 && dados.feito > 0 && (
        <p className="orc-lote-rodape">Nenhum travou.</p>
      )}
    </div>
  )
}
