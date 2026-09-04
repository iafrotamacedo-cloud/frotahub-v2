// rev 2 — a Planilha de controle: todos os serviços, com filtro e exportação
//
// MESMO PADRÃO DE telas/trilogo/DadosTrilogo.tsx: filtros na faixa de cima,
// tabela no visual do Trílogo (ticket, loja, conta, descrição), "Extrair"
// com PDF/Excel — a mesma fonte (servicos_lista) que a tela lê.
import { useCallback, useEffect, useState } from 'react'
import { motor, baixarDoMotor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { Paginacao } from '../orcamentos/Arquivos'
import type { Pagina } from '../orcamentos/tipos'
import type { Perfil } from '../../sessao/tipos'
import { FichaChamado } from '../trilogo/FichaChamado'
import { CONTA_ROTULO, STATUS_ORDEM, STATUS_ROTULO, type ItemLista } from './tipos'
import { CelulaConta, CelulaData, CelulaDescricao, CelulaLoja, CelulaTicket, CelulaValor, useEncolher } from './celulas'

export function Planilha({ perfil }: { perfil: Perfil }) {
  const [status, setStatus] = useState('')
  const [conta, setConta] = useState('')
  const [busca, setBusca] = useState('')
  const [buscaAplicada, setBuscaAplicada] = useState('')
  const [pagina, setPagina] = useState<Pagina<ItemLista> | null>(null)
  const [numeroDaPagina, setNumeroDaPagina] = useState(1)
  const [por, setPor] = useState(100)
  const [erro, setErro] = useState<string | null>(null)
  const [extraindo, setExtraindo] = useState<'pdf' | 'xlsx' | null>(null)
  const [aberto, setAberto] = useState<number | null>(null)
  const corpo = useEncolher(pagina)

  useEffect(() => {
    const t = setTimeout(() => { setBuscaAplicada(busca.trim()); setNumeroDaPagina(1) }, 350)
    return () => clearTimeout(t)
  }, [busca])

  const paramsDoFiltro = useCallback(() => {
    const p = new URLSearchParams({ todos: '1' })
    if (status) p.set('status', status)
    if (conta) p.set('conta', conta)
    if (buscaAplicada) p.set('busca', buscaAplicada)
    return p
  }, [status, conta, buscaAplicada])

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const p = paramsDoFiltro()
      p.set('pagina', String(numeroDaPagina))
      p.set('por_pagina', String(por))
      setPagina(await motor<Pagina<ItemLista>>(`/servicos/lista?${p}`))
    } catch (e) {
      setPagina(null)
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a planilha.')
    }
  }, [paramsDoFiltro, numeroDaPagina, por])

  useEffect(() => { void carregar() }, [carregar])

  async function extrair(formato: 'pdf' | 'xlsx') {
    setExtraindo(formato)
    setErro(null)
    try {
      await baixarDoMotor(`/servicos/lista.${formato}?${paramsDoFiltro()}`)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui gerar o arquivo.')
    } finally {
      setExtraindo(null)
    }
  }

  if (aberto !== null) {
    return (
      <FichaChamado
        numero={String(aberto)}
        perfil={perfil}
        voltar={() => setAberto(null)}
        permitirMarcarServico={false}
      />
    )
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Planilha de controle</h1>
          <p>Todos os serviços, em qualquer fila — o livro-razão.</p>
        </div>
        <div className="sv-extrair">
          <button type="button" className="bt bt-neutro" disabled={!!extraindo} onClick={() => void extrair('xlsx')}>
            {extraindo === 'xlsx' ? 'Gerando...' : 'Excel'}
          </button>
          <button type="button" className="bt bt-neutro" disabled={!!extraindo} onClick={() => void extrair('pdf')}>
            {extraindo === 'pdf' ? 'Gerando...' : 'PDF'}
          </button>
        </div>
      </header>

      <div className="tri-filtros">
        <label className="tri-campo tri-ticket">
          <span>Buscar</span>
          <input value={busca} onChange={e => setBusca(e.target.value)} placeholder="Ticket ou loja..." />
        </label>
        <label className="tri-campo">
          <span>Status</span>
          <select value={status} onChange={e => { setStatus(e.target.value); setNumeroDaPagina(1) }}>
            <option value="">Todos</option>
            {STATUS_ORDEM.map(s => <option key={s} value={s}>{STATUS_ROTULO[s]}</option>)}
          </select>
        </label>
        <label className="tri-campo">
          <span>Conta</span>
          <select value={conta} onChange={e => { setConta(e.target.value); setNumeroDaPagina(1) }}>
            <option value="">Todas</option>
            <option value="instalacoes">{CONTA_ROTULO.instalacoes}</option>
            <option value="civil">{CONTA_ROTULO.civil}</option>
          </select>
        </label>
      </div>

      {erro && <div className="erro-caixa">{erro}</div>}

      {pagina === null && !erro ? (
        <Carregando texto="Carregando a planilha..." />
      ) : erro ? null : pagina!.linhas.length === 0 ? (
        <div className="vazio">Nenhum serviço com esses filtros.</div>
      ) : (
        <>
          <div className="tabela-rolo">
            <table className="tabela sv-tabela sv-tabela-livro">
              <thead>
                <tr>
                  <th className="c-ticket">Ticket</th>
                  <th className="c-loja">Loja</th>
                  <th className="c-conta">Conta</th>
                  <th>Descrição</th>
                  <th className="c-status">Status</th>
                  <th className="c-valor">Valor</th>
                  <th className="c-pco">PCO</th>
                  <th className="c-nf">Nota fiscal</th>
                  <th className="c-data">Entrou em</th>
                </tr>
              </thead>
              <tbody ref={corpo}>
                {pagina!.linhas.map(it => (
                  <tr
                    key={it.id}
                    className={'tri-linha' + (it.removido_em ? ' inativa' : '')}
                    onClick={() => setAberto(it.ticket)}
                    title={`Abrir o chamado ${it.ticket}`}
                  >
                    <CelulaTicket ticket={it.ticket} aoAbrir={() => setAberto(it.ticket)} />
                    <CelulaLoja loja={it.loja} />
                    <CelulaConta conta={it.conta} />
                    <CelulaDescricao texto={it.chamado_descricao} />
                    <td className="c-status">{STATUS_ROTULO[it.status] ?? it.status}</td>
                    <CelulaValor valor={it.orcamento_valor} />
                    <td className="c-pco">{it.pco_numero ?? '—'}</td>
                    <td className="c-nf">{it.nf_numero ?? (it.com_nf ? 'sim' : '—')}</td>
                    <CelulaData iso={it.entrou_em} />
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Paginacao
            pagina={pagina} por={por}
            aoTrocarPagina={setNumeroDaPagina}
            aoTrocarPor={p => { setPor(p); setNumeroDaPagina(1) }}
          />
        </>
      )}
    </>
  )
}
