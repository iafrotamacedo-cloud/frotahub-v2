// rev 1 — faturamento direto
//
// A FILA QUE NÃO TEM ETAPAS
//
//	As outras filas são um caminho: a nota entra, é lida, ganha ticket, vira
//	orçamento, é lançada, é faturada. Esta tem dois passos e acabou — entra,
//	ganha o ticket, vai para o lançamento. Nada de margem, teto, rateio ou
//	espelho de cobrança.
//
//	Por isso a tela é uma lista com filtros, e não um painel de etapas: o que
//	sobra depois de lançar é histórico, e histórico sem filtro é uma tabela que
//	ninguém abre pela segunda vez.
//
// O TICKET É DIGITADO, E CONFIAMOS
//
//	Decisão do dono: aqui não se confere o Trílogo nem se lê o ticket da nota.
//	As travas de ticket existem para proteger o que a gente COBRA — ticket
//	errado num orçamento nosso vira dinheiro cobrado da loja errada. Aqui não há
//	cobrança nossa; o pior caso é uma estatística no chamado errado, que se
//	conserta olhando.
import { useCallback, useEffect, useState } from 'react'
import { motor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta, Insercao, Paginacao } from './Arquivos'
import { emReais, emDataHora, type Documento, type Pagina } from './tipos'

export function Direto({ voltar }: { voltar: () => void }) {
  const [pagina, setPagina] = useState<Pagina<Documento> | null>(null)
  const [erro, setErro] = useState('')
  const [recado, setRecado] = useState('')
  const [pag, setPag] = useState(1)
  const [por, setPor] = useState(50)
  const [ticket, setTicket] = useState('')
  const [loja, setLoja] = useState('')
  const [de, setDe] = useState('')
  const [ate, setAte] = useState('')
  const [digitando, setDigitando] = useState<string | null>(null)
  const [numero, setNumero] = useState('')

  const carregar = useCallback(async () => {
    const q = new URLSearchParams({ pagina: String(pag), por: String(por) })
    if (ticket) q.set('ticket', ticket)
    if (loja) q.set('loja', loja)
    if (de) q.set('de', de)
    if (ate) q.set('ate', ate)
    try {
      setPagina(await motor<Pagina<Documento>>('/orcamentos/direto?' + q))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar a lista.')
    }
  }, [pag, por, ticket, loja, de, ate])

  useEffect(() => { void carregar() }, [carregar])

  async function amarrar(id: string) {
    const n = Number(numero.replace(/\D/g, ''))
    if (!(n >= 10000 && n <= 999999)) { setErro('O ticket tem 5 ou 6 dígitos.'); return }
    try {
      await motor(`/orcamentos/documentos/${id}/tickets`, { metodo: 'POST', corpo: { tickets: [n] } })
      setDigitando(null); setNumero(''); setErro('')
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui amarrar o ticket.')
    }
  }

  async function mandar(d: Documento) {
    try {
      const r = await motor<{ aviso?: string; ticket: number }>(
        `/orcamentos/direto/${d.id}/lancar`, { metodo: 'POST' })
      setRecado(r.aviso || `Custo do ticket ${r.ticket} pronto para lançar.`)
      setErro('')
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui mandar para o lançamento.')
    }
  }

  return (
    <div className="orc-tela">
      <BarraDeVolta voltar={voltar} titulo="Faturamento direto" />

      <p className="orc-aviso-fila">
        Notas que o fornecedor cobra <b>direto do cliente</b>. Entram só para o custo ser
        lançado no Trílogo — <b>nunca</b> entram em faturamento nosso.
      </p>

      <Insercao fila="direto" aoTerminar={carregar} aoFalhar={setErro} />

      <div className="orc-filtros">
        <input placeholder="ticket" inputMode="numeric" value={ticket}
          onChange={e => { setPag(1); setTicket(e.target.value.replace(/\D/g, '')) }} />
        <input placeholder="loja" value={loja}
          onChange={e => { setPag(1); setLoja(e.target.value) }} />
        <label>de <input type="date" value={de} onChange={e => { setPag(1); setDe(e.target.value) }} /></label>
        <label>até <input type="date" value={ate} onChange={e => { setPag(1); setAte(e.target.value) }} /></label>
        {(ticket || loja || de || ate) && (
          <button type="button" className="mut"
            onClick={() => { setTicket(''); setLoja(''); setDe(''); setAte(''); setPag(1) }}>
            limpar
          </button>
        )}
      </div>

      {erro && <p className="erro" style={{ margin: '0 16px 8px' }}>{erro}</p>}
      {recado && <p className="orc-recado" style={{ margin: '0 16px 8px' }}>{recado}</p>}

      {!pagina ? <Carregando /> : (
        <>
          <div className="orc-rolagem">
            <table className="orc-tabela">
              <thead>
                <tr>
                  <th>Arquivo</th><th>Nota</th><th>Loja</th><th>Ticket</th>
                  <th style={{ textAlign: 'right' }}>Valor</th><th>Inserida em</th>
                  <th style={{ textAlign: 'right' }}>Ações</th>
                </tr>
              </thead>
              <tbody>
                {!pagina.linhas.length && (
                  <tr><td colSpan={7}><p className="orc-vazio">Nenhuma nota nesta fila.</p></td></tr>
                )}
                {pagina.linhas.map(d => {
                  const temTicket = (d.ticket_numeros?.length ?? 0) > 0
                  return (
                    <tr key={d.id}>
                      <td>
                        <span className="orc-nome">{d.nome_arquivo}</span>
                        {d.emitente_nome && <span className="orc-detalhe">{d.emitente_nome}</span>}
                      </td>
                      <td>{d.numero ?? d.dav_numero ?? '–'}</td>
                      <td>{(d as Documento & { loja?: string }).loja ?? '–'}</td>
                      <td>
                        {temTicket
                          ? d.ticket_numeros!.map(t => <span key={t} className="orc-tk">{t}</span>)
                          : digitando === d.id
                            ? (
                              <span className="orc-digita">
                                <input autoFocus inputMode="numeric" placeholder="ticket" value={numero}
                                  onChange={e => setNumero(e.target.value.replace(/\D/g, ''))}
                                  onKeyDown={e => {
                                    if (e.key === 'Enter') void amarrar(d.id)
                                    if (e.key === 'Escape') { setDigitando(null); setNumero('') }
                                  }} />
                                <button type="button" className="forte" onClick={() => void amarrar(d.id)}>ok</button>
                              </span>
                            )
                            : <button type="button" className="orc-mais"
                              onClick={() => { setDigitando(d.id); setNumero('') }}>informar</button>}
                      </td>
                      <td style={{ textAlign: 'right' }}>{emReais(d.valor_total)}</td>
                      <td>{emDataHora(d.inserido_em)}</td>
                      <td className="orc-acoes">
                        {d.status === 'usado'
                          ? <span className="orc-selo ok">no lançamento</span>
                          : (
                            <button type="button" className="forte"
                              disabled={!temTicket || !d.valor_total}
                              title={!temTicket ? 'informe o ticket primeiro'
                                : !d.valor_total ? 'esta nota está sem valor lido'
                                  : 'cria o custo na fila de lançamento, com o arquivo original'}
                              onClick={() => void mandar(d)}>
                              mandar para lançar
                            </button>
                          )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <Paginacao pagina={pagina} por={por}
            aoTrocarPagina={setPag} aoTrocarPor={n => { setPor(n); setPag(1) }} />
        </>
      )}
    </div>
  )
}
