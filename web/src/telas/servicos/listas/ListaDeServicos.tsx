// rev 4 — a lista genérica das 9 filas de Serviço
//
// UMA TELA SÓ, PARAMETRIZADA — NÃO NOVE ARQUIVOS QUASE IDÊNTICOS
//
//	As 9 listas (Pendentes, Feitos, Lançados, os 3 de Execução, os 3 de
//	Faturamento) têm o MESMO esqueleto — tabela, paginação, "voltar pro
//	contrato" no fim da linha, clique abre a ficha — e diferem só no filtro
//	(status/pco) e na ação que a ficha oferece. Nove cópias do mesmo
//	componente seriam nove lugares para um mesmo defeito se esconder.
//
// A TABELA SEGUE trilogo/DadosTrilogo.tsx (ticket, loja, conta, descrição),
// mais valor e a ação desta fila. Paginação reaproveitada de
// telas/orcamentos/Arquivos.tsx.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor, avisoDe } from '../../../motor/cliente'
import { Carregando } from '../../../componentes/Carregando'
import { Paginacao } from '../../orcamentos/Arquivos'
import type { Pagina } from '../../orcamentos/tipos'
import type { Perfil } from '../../../sessao/tipos'
import type { ItemLista, Status } from '../tipos'
import { FichaDoTicket, type Acao } from '../FichaDoTicket'
import { CelulaConta, CelulaDescricao, CelulaLoja, CelulaTicket, CelulaValor, useEncolher } from '../celulas'

interface Props {
  titulo: string
  status: Status
  /** Só para o cartão de Faturamento, que divide UM status em dois sub-cards. */
  comPCO?: boolean
  semPCO?: boolean
  acao: Acao
  perfil: Perfil
}

export function ListaDeServicos({ titulo, status, comPCO, semPCO, acao, perfil }: Props) {
  const [pagina, setPagina] = useState<Pagina<ItemLista> | null>(null)
  const [numeroDaPagina, setNumeroDaPagina] = useState(1)
  const [por, setPor] = useState(100)
  const [busca, setBusca] = useState('')
  const [buscaAplicada, setBuscaAplicada] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [aberto, setAberto] = useState<ItemLista | null>(null)
  const corpo = useEncolher(pagina)

  useEffect(() => {
    const t = setTimeout(() => { setBuscaAplicada(busca.trim()); setNumeroDaPagina(1) }, 350)
    return () => clearTimeout(t)
  }, [busca])

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const params = new URLSearchParams({ status, pagina: String(numeroDaPagina), por_pagina: String(por) })
      if (comPCO) params.set('pco', 'sim')
      if (semPCO) params.set('pco', 'nao')
      if (buscaAplicada) params.set('busca', buscaAplicada)
      setPagina(await motor<Pagina<ItemLista>>(`/servicos/lista?${params}`))
    } catch (e) {
      setPagina(null)
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
  }, [status, comPCO, semPCO, numeroDaPagina, por, buscaAplicada])

  useEffect(() => { void carregar() }, [carregar])

  async function voltarProContrato(item: ItemLista) {
    if (!window.confirm(
      `Voltar o ticket ${item.ticket} para a fila do contrato?\n\nO PDF do orçamento sai do arquivo e o orçamento desvincula deste ticket.`,
    )) return
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/reclassificar`, { metodo: 'POST', corpo: { motivo: '' } })
      setRecado(avisoDe(resposta) ?? 'Voltou para o contrato. Orçamento removido.')
      void carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui reclassificar.')
    }
  }

  async function rejeitarOrcamento(item: ItemLista) {
    if (!window.confirm(
      `Rejeitar o orçamento do ticket ${item.ticket}?\n\nO ticket volta para o contrato. O orçamento some no Trílogo e o PDF fica guardado — se o chamado voltar para Serviço, cai em Feitos para lançar de novo.`,
    )) return
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/rejeitar`, { metodo: 'POST' })
      setRecado(avisoDe(resposta) ?? 'Orçamento rejeitado — voltou para o contrato.')
      void carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui rejeitar o orçamento.')
    }
  }

  if (aberto) {
    return (
      <FichaDoTicket
        item={aberto}
        perfil={perfil}
        acao={acao}
        voltar={() => setAberto(null)}
        aoMudar={recadoNovo => { setRecado(recadoNovo); void carregar() }}
      />
    )
  }

  return (
    <>
      <header className="hero hero-linha sv-hero-busca">
        <h1>{titulo}</h1>
        <div className="barra-busca">
          <input
            value={busca} onChange={e => setBusca(e.target.value)}
            placeholder="Buscar por ticket ou loja..." aria-label="Buscar"
          />
        </div>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      {pagina === null && !erro ? (
        <Carregando texto="Carregando..." />
      ) : erro ? null : pagina!.linhas.length === 0 ? (
        <div className="vazio">
          {buscaAplicada ? 'Nenhum serviço corresponde a essa busca.' : 'Nenhum serviço nesta fila.'}
        </div>
      ) : (
        <>
          <div className="tabela-rolo">
            <table className="tabela sv-tabela">
              <thead>
                <tr>
                  <th className="c-ticket">Ticket</th>
                  <th className="c-loja">Loja</th>
                  <th className="c-conta">Conta</th>
                  <th>Descrição</th>
                  <th className="c-valor">Valor</th>
                  {status !== 'faturado' && <th className="acoes-col">Ações</th>}
                </tr>
              </thead>
              <tbody ref={corpo}>
                {pagina!.linhas.map(it => (
                  <tr
                    key={it.id}
                    className="tri-linha"
                    onClick={() => setAberto(it)}
                    title={`Abrir o chamado ${it.ticket}`}
                  >
                    <CelulaTicket ticket={it.ticket} aoAbrir={() => setAberto(it)} />
                    <CelulaLoja loja={it.loja} />
                    <CelulaConta conta={it.conta} />
                    <CelulaDescricao texto={it.chamado_descricao} />
                    <CelulaValor valor={it.orcamento_valor} />
                    {status !== 'faturado' && (
                      <td className="acoes" onClick={e => e.stopPropagation()}>
                        {status === 'orcamento_lancado' && (
                          <button type="button" className="bt bt-mini bt-perigo" onClick={() => void rejeitarOrcamento(it)}>
                            Rejeitar
                          </button>
                        )}
                        <button type="button" className="bt bt-mini bt-neutro" onClick={() => void voltarProContrato(it)}>
                          Voltar pro contrato
                        </button>
                      </td>
                    )}
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
