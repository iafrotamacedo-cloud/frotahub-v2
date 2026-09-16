// rev 3 — a Planilha de controle: o funil vivo, com filtro e exportação
//
// MESMO PADRÃO DE telas/trilogo/DadosTrilogo.tsx: filtros na faixa de cima,
// tabela no visual do Trílogo (ticket, loja, conta, descrição), "Extrair"
// com PDF/Excel — a mesma fonte (servicos_lista) que a tela lê.
//
// SÓ O QUE AINDA É SERVIÇO DE VERDADE
//
//	Candidato não entra: ainda está na fila de decisão, sem linha no Kanban.
//	Voltar pro contrato e Rejeitar fecham a linha (`removido_em`). Mandar
//	`todos=1` trazia esse histórico com o último status — um ticket que já
//	voltou ao contrato (ou que só foi candidato) continuava na planilha como
//	"Orçamento feito". A planilha acompanha as idas e voltas: sai quando sai
//	do Kanban, e se o mesmo ticket reentrar nasce uma linha nova.
import { useCallback, useEffect, useState } from 'react'
import { motor, baixarDoMotor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { Paginacao } from '../orcamentos/Arquivos'
import { emData, type Pagina, type RelatorioMensalDados } from '../orcamentos/tipos'
import type { Perfil } from '../../sessao/tipos'
import { FichaChamado } from '../trilogo/FichaChamado'
import { CONTA_ROTULO, STATUS_ORDEM, STATUS_ROTULO, type ItemLista } from './tipos'
import { CelulaConta, CelulaData, CelulaDescricao, CelulaLoja, CelulaTicket, CelulaValor, useEncolher } from './celulas'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { CarregarMais } from '../../componentes/CarregarMais'
import { useEhMobile } from '../../componentes/useEhMobile'
import { emReais, quando } from '../trilogo/tipos'

// A PLANILHA E O RELATÓRIO MENSAL SÃO DUAS TELAS, MESMO PADRÃO DE
// orcamentos/Faturamento.tsx
//
//	Planilha   o funil vivo, com filtro — "quem está em Serviço, e em que pé".
//
//	Relatório  a relação que VAI AO CLIENTE, no modelo dele (o mesmo de
//	mensal     orcamentos/relatorio_mensal.go — só muda a fila: aqui é
//	           "vistoriado e sem PCO", lá é "lançado e não cobrado). Só
//	           extração, e só em Excel — mesma razão da tela de materiais: o
//	           arquivo é o produto, não uma vista dele.
export function Planilha({ perfil }: { perfil: Perfil }) {
  const [aba, setAba] = useState<'planilha' | 'relatorio'>('planilha')
  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Planilha de controle</h1>
          <p>
            {aba === 'planilha'
              ? 'Quem já é Serviço e está no funil — candidato e quem voltou pro contrato não entram.'
              : 'A relação que vai ao cliente, no modelo dele — vistoriados, aguardando o PCO.'}
          </p>
        </div>
        <span className="fat-abas">
          <button type="button" className={aba === 'planilha' ? 'ativa' : ''} onClick={() => setAba('planilha')}>
            Planilha
          </button>
          <button type="button" className={aba === 'relatorio' ? 'ativa' : ''} onClick={() => setAba('relatorio')}>
            Relatório mensal
          </button>
        </span>
      </header>
      {aba === 'planilha' ? <ListaDaPlanilha perfil={perfil} /> : <RelatorioMensal />}
    </>
  )
}

// ---------------------------------------------------------------------------
// A planilha — o funil vivo, com filtro e exportação
// ---------------------------------------------------------------------------

function ListaDaPlanilha({ perfil }: { perfil: Perfil }) {
  const ehMobile = useEhMobile()
  const [status, setStatus] = useState('')
  const [conta, setConta] = useState('')
  const [busca, setBusca] = useState('')
  const [buscaAplicada, setBuscaAplicada] = useState('')
  const [pagina, setPagina] = useState<Pagina<ItemLista> | null>(null)
  const [acumulado, setAcumulado] = useState<ItemLista[]>([])
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
    const p = new URLSearchParams()
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
      const r = await motor<Pagina<ItemLista>>(`/servicos/lista?${p}`)
      setPagina(r)
      setAcumulado(atual => (numeroDaPagina === 1 ? r.linhas : [...atual, ...r.linhas]))
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
      <div className="sv-extrair" style={{ justifyContent: 'flex-end' }}>
        <button type="button" className="bt bt-neutro" disabled={!!extraindo} onClick={() => void extrair('xlsx')}>
          {extraindo === 'xlsx' ? 'Gerando...' : 'Excel'}
        </button>
        <button type="button" className="bt bt-neutro" disabled={!!extraindo} onClick={() => void extrair('pdf')}>
          {extraindo === 'pdf' ? 'Gerando...' : 'PDF'}
        </button>
      </div>

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
      ) : ehMobile ? (
        <>
          <div className="cl-lista">
            {acumulado.map(it => (
              <CartaoLinha
                key={it.id}
                titulo={it.ticket}
                onClick={() => setAberto(it.ticket)}
                linhas={[
                  { rotulo: 'Loja', valor: it.loja || '—' },
                  { rotulo: 'Conta', valor: CONTA_ROTULO[it.conta] ?? it.conta },
                  { rotulo: 'Descrição', valor: (it.chamado_descricao || '—').replace(/\s+/g, ' ').trim() },
                  { rotulo: 'Status', valor: STATUS_ROTULO[it.status] ?? it.status },
                  { rotulo: 'Valor', valor: it.orcamento_valor != null ? emReais(it.orcamento_valor) : '—' },
                  { rotulo: 'PCO', valor: it.pco_numero ?? '—' },
                  { rotulo: 'Nota fiscal', valor: it.nf_numero ?? (it.com_nf ? 'sim' : '—') },
                  { rotulo: 'Entrou em', valor: it.entrou_em ? quando(it.entrou_em) : '—' },
                ]}
              />
            ))}
          </div>
          <CarregarMais
            temMais={numeroDaPagina < pagina!.paginas}
            onClick={() => setNumeroDaPagina(n => n + 1)}
          />
        </>
      ) : (
        <>
          <div className="tabela-rolo tri-painel">
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
          {!ehMobile && <Paginacao
            pagina={pagina} por={por}
            aoTrocarPagina={setNumeroDaPagina}
            aoTrocarPor={p => { setPor(p); setNumeroDaPagina(1) }}
          />}
        </>
      )}
    </>
  )
}

// ---------------------------------------------------------------------------
// Relatório mensal — a relação que vai ao cliente, no modelo dele
// ---------------------------------------------------------------------------
//
// MESMO COMPONENTE DE orcamentos/Faturamento.tsx (RelatorioMensal), SÓ A
// FILA MUDA
//
//	Lá a fila é "lançado e não cobrado" (fatura_id is null); aqui é
//	"vistoriado e sem PCO" (servicos_a_cobrar, migração 073) — o mesmo
//	sub-card "Aguardando PCO" que já existe no hub. A tela mostra a planilha
//	inteira, não um resumo dela, pela mesma razão: quem vai mandar o arquivo
//	ao cliente quer VER a planilha antes. Só Excel, de propósito: o arquivo é
//	o produto — sai daqui, o cliente preenche o PCO e devolve.
function RelatorioMensal() {
  const ehMobile = useEhMobile()
  const [dados, setDados] = useState<RelatorioMensalDados | null>(null)
  const [erro, setErro] = useState('')
  const [baixando, setBaixando] = useState(false)

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<RelatorioMensalDados>('/servicos/relatorio-mensal'))
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o relatório.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function extrair() {
    setBaixando(true)
    try {
      await baixarDoMotor('/servicos/relatorio-mensal.xlsx')
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui gerar a planilha.')
    } finally {
      setBaixando(false)
    }
  }

  if (erro && !dados) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  const vazio = dados.quantos === 0

  return (
    <>
      <div className="fat-resumo">
        <Cartao titulo="Aguardando PCO" valor={String(dados.quantos)} rodape="serviços vistoriados e sem PCO" />
        <Cartao titulo="Valor" valor={emReais(dados.valor)} rodape="soma do que vai na planilha" />
        <Cartao
          titulo="Período"
          valor={dados.de ? `${emData(dados.de)} – ${emData(dados.ate)}` : '–'}
          rodape="desde que entrou na fila de faturamento"
        />
      </div>

      {/* A LOJA SEM O NOME DO CLIENTE APARECE ANTES DE O ARQUIVO SAIR —
          mesmo aviso da tela de materiais (Faturamento.tsx). */}
      {dados.lojas_sem_nome.length > 0 && (
        <p className="fat-aviso">
          {dados.lojas_sem_nome.length === 1 ? 'Uma loja está' : `${dados.lojas_sem_nome.length} lojas estão`} sem
          o nome que o cliente usa: <b>{dados.lojas_sem_nome.join(', ')}</b>. As linhas dela sairiam com a coluna
          LOJA em branco, e o cliente não conseguiria lançar no centro de custo. Cadastre o nome antes de enviar.
        </p>
      )}

      {dados.quantos >= dados.teto && (
        <p className="fat-aviso">
          Há mais serviços a cobrar do que cabe num arquivo ({dados.teto}). A planilha sai cortada e diz
          isso no rodapé — feche esta e gere de novo.
        </p>
      )}

      <div className="orc-lista">
        <div className="orc-lista-cab">
          <h2>Serviços aguardando PCO</h2>
          <em>o que vai na planilha, no modelo do cliente</em>
          <span style={{ flex: 1 }} />
          <button type="button" className="orc-bt forte" disabled={vazio || baixando} onClick={() => void extrair()}>
            {baixando ? 'gerando…' : 'Extrair Excel'}
          </button>
        </div>

        {vazio ? (
          <p className="orc-vazio grande">
            Nada a cobrar. Todo serviço vistoriado já tem PCO ou ainda não foi vistoriado.
          </p>
        ) : ehMobile ? (
          <div className="cl-lista">
            {dados.linhas.map((l, i) => (
              <CartaoLinha
                key={i}
                titulo={l[2] ? String(l[2]) : <em className="ruim">sem o nome do cliente</em>}
                linhas={[
                  { rotulo: 'Nº', valor: String(l[0]) },
                  { rotulo: 'Ticket', valor: String(l[1]) },
                  { rotulo: 'Valor', valor: emReais(Number(l[3])) },
                  { rotulo: 'Data', valor: emData(String(l[4])) },
                  { rotulo: 'Orçamento', valor: emReais(Number(l[5])) },
                  { rotulo: 'Conta', valor: String(l[6]) },
                ]}
              />
            ))}
          </div>
        ) : (
          <div className="orc-rolagem">
            <table className="orc-tabela rel-mensal">
              <thead>
                <tr>
                  <th style={{ textAlign: 'right' }}>Nº</th>
                  <th style={{ textAlign: 'right' }}>TICKET</th>
                  <th>LOJA</th>
                  <th style={{ textAlign: 'right' }}>VALOR</th>
                  <th>DATA</th>
                  <th style={{ textAlign: 'right' }}>ORÇAMENTO</th>
                  <th>CONTA</th>
                  <th>PCO</th>
                </tr>
              </thead>
              <tbody>
                {dados.linhas.map((l, i) => (
                  <tr key={i}>
                    <td className="num">{String(l[0])}</td>
                    <td className="num">{String(l[1])}</td>
                    {/* A LOJA VAZIA APARECE COMO FALTA, NÃO COMO ESPAÇO EM BRANCO — mesma razão da tela de materiais. */}
                    <td>{l[2] ? String(l[2]) : <em className="ruim">sem o nome do cliente</em>}</td>
                    <td className="num">{emReais(Number(l[3]))}</td>
                    <td>{emData(String(l[4]))}</td>
                    <td className="num">{emReais(Number(l[5]))}</td>
                    <td className="mut">{String(l[6])}</td>
                    <td className="mut">—</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {erro && <p className="erro">{erro}</p>}
    </>
  )
}

function Cartao({ titulo, valor, rodape }: { titulo: string; valor: string; rodape: string }) {
  return (
    <div className="fat-cartao">
      <span className="fat-cartao-titulo">{titulo}</span>
      <strong>{valor}</strong>
      <span className="fat-cartao-rodape">{rodape}</span>
    </div>
  )
}
