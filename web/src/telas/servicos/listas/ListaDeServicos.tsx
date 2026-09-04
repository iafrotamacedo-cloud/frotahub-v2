// rev 1 — a lista genérica das 9 filas de Serviço
//
// UMA TELA SÓ, PARAMETRIZADA — NÃO NOVE ARQUIVOS QUASE IDÊNTICOS
//
//	As 9 listas (Pendentes, Feitos, Lançados, os 3 de Execução, os 3 de
//	Faturamento) têm o MESMO esqueleto — tabela, paginação, "voltar pro
//	contrato" no fim da linha, clique abre a ficha — e diferem só no filtro
//	(status/pco) e na ação que a ficha oferece. Nove cópias do mesmo
//	componente seriam nove lugares para um mesmo defeito se esconder.
//
// MESMO PADRÃO DE telas/orcamentos/Arquivos.tsx: tabela com <colgroup>,
// paginação reaproveitada de lá (Paginacao), busca com debounce.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor, avisoDe } from '../../../motor/cliente'
import { Carregando } from '../../../componentes/Carregando'
import { Paginacao } from '../../orcamentos/Arquivos'
import type { Pagina } from '../../orcamentos/tipos'
import type { Perfil } from '../../../sessao/tipos'
import { CONTA_ROTULO, type ItemLista, type Status } from '../tipos'
import { FichaDoTicket, type Acao } from '../FichaDoTicket'

interface Props {
  titulo: string
  status: Status
  /** Só para o cartão de Faturamento, que divide UM status em dois sub-cards. */
  comPCO?: boolean
  semPCO?: boolean
  acao: Acao
  perfil: Perfil
  voltar: () => void
}

export function ListaDeServicos({ titulo, status, comPCO, semPCO, acao, perfil, voltar }: Props) {
  const [pagina, setPagina] = useState<Pagina<ItemLista> | null>(null)
  const [numeroDaPagina, setNumeroDaPagina] = useState(1)
  const [por, setPor] = useState(100)
  const [busca, setBusca] = useState('')
  const [buscaAplicada, setBuscaAplicada] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [aberto, setAberto] = useState<ItemLista | null>(null)

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
    if (!window.confirm(`Voltar o ticket ${item.ticket} para a fila do contrato?`)) return
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/reclassificar`, { metodo: 'POST', corpo: { motivo: '' } })
      setRecado(avisoDe(resposta) ?? 'Voltou para o contrato.')
      void carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui reclassificar.')
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
      <button type="button" className="bt bt-neutro sv-voltar" onClick={voltar}>‹ Voltar</button>

      <header className="hero">
        <h1>{titulo}</h1>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      <div className="barra-busca">
        <input
          value={busca} onChange={e => setBusca(e.target.value)}
          placeholder="Buscar por ticket ou loja..." aria-label="Buscar"
        />
      </div>

      {pagina === null && !erro ? (
        <Carregando texto="Carregando..." />
      ) : erro ? null : pagina!.linhas.length === 0 ? (
        <div className="vazio">
          {buscaAplicada ? 'Nenhum serviço corresponde a essa busca.' : 'Nenhum serviço nesta fila.'}
        </div>
      ) : (
        <>
          <div className="tabela-rolo">
            <table className="tabela">
              <thead>
                <tr>
                  <th>Ticket</th>
                  <th>Loja</th>
                  <th>Conta</th>
                  <th>Valor</th>
                  <th className="acoes-col">Ações</th>
                </tr>
              </thead>
              <tbody>
                {pagina!.linhas.map(it => (
                  <tr key={it.id}>
                    <td>
                      <button type="button" className="sv-ticket" onClick={() => setAberto(it)}>
                        <code>{it.ticket}</code>
                      </button>
                    </td>
                    <td>{it.loja ?? '—'}</td>
                    <td>{CONTA_ROTULO[it.conta] ?? it.conta}</td>
                    <td>
                      {it.orcamento_valor != null
                        ? it.orcamento_valor.toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
                        : '—'}
                    </td>
                    <td className="acoes">
                      <button type="button" className="bt bt-mini" onClick={() => setAberto(it)}>Abrir</button>
                      {status !== 'faturado' && (
                        <button type="button" className="bt bt-mini bt-neutro" onClick={() => void voltarProContrato(it)}>
                          Voltar pro contrato
                        </button>
                      )}
                    </td>
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
