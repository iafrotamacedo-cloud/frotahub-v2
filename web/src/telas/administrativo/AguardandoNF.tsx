// rev 2 — Notas Fiscais > Aguardando (o almoxarife recebe aqui)
//
// SÓ AS OBRAS QUE ESTE LOGIN TEM LIBERADAS
//
//	O motor já filtra por `centro_custo_acessos` (ver `temAcessoAObra` em
//	notas_fiscais.go) — esta tela nunca precisa saber a regra, só mostrar o
//	que veio.
//
// A BIFURCAÇÃO PARA LOCAÇÃO (16/09/2026, Fase 1 do módulo Locações)
//
//	Nem toda OC que chega aqui é uma compra — algumas são locação, e locação
//	não tem nota fiscal (é aluguel, a prova de entrada é o romaneio). Em vez
//	de escanear a NF, o almoxarife escolhe "Locação" e abre
//	`ReceberLocacao`, de `telas/locacoes`. É a ÚNICA importação cruzando
//	módulos deste jeito no sistema (P-13 normalmente proíbe) — documentada
//	aqui de propósito, porque é exatamente este ponto que o plano do módulo
//	desenhou como a bifurcação: a OC entra e anda 100% igual até aqui, e só
//	aqui os dois caminhos se separam. Quem não tem `LOCACOES_RECEBER` nem
//	vê o botão — o motor recusaria de qualquer forma (P-29).
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { useEhMobile } from '../../componentes/useEhMobile'
import type { Perfil } from '../../sessao/tipos'
import { ReceberNF } from './ReceberNF'
import { emReais, type OrdemAguardandoNF } from './tipos'
import { ReceberLocacao } from '../locacoes/ReceberLocacao'
import { RotinaLocacoesReceber, temRotina as temRotinaLocacoes } from '../locacoes/rotinas'

interface Props {
  perfil?: Perfil | null
}

export function AguardandoNF({ perfil = null }: Props) {
  const ehMobile = useEhMobile()
  const [ordens, setOrdens] = useState<OrdemAguardandoNF[] | null>(null)
  const [erro, setErro] = useState('')
  const [recebendo, setRecebendo] = useState<OrdemAguardandoNF | null>(null)
  const [recebendoLocacao, setRecebendoLocacao] = useState<OrdemAguardandoNF | null>(null)
  const podeReceberLocacao = temRotinaLocacoes(perfil, RotinaLocacoesReceber)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ ordens: OrdemAguardandoNF[] }>('/administrativo/nf/aguardando')
      setOrdens(r.ordens)
      setErro('')
    } catch (e) {
      setOrdens([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Aguardando NF</h1>
          <p>OCs já enviadas ao cliente, esperando a nota fiscal chegar — recebimento parcial é permitido.</p>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {ordens === null ? (
        <Carregando texto="Carregando..." />
      ) : ordens.length === 0 ? (
        <div className="vazio">
          Nenhuma OC aguardando nota fiscal — ou você ainda não tem nenhuma obra liberada
          para receber. Fale com quem configura os acessos por obra.
        </div>
      ) : ehMobile ? (
        <div className="cl-lista">
          {ordens.map(o => (
            <CartaoLinha
              key={o.ordem_compra_id}
              titulo={o.numero || '—'}
              linhas={[
                { rotulo: 'Obra/centro', valor: o.obra_centro_custo || '—' },
                { rotulo: 'Fornecedor', valor: o.fornecedor_nome || '—' },
                { rotulo: 'Total', valor: emReais(o.total) },
                { rotulo: 'Recebido', valor: emReais(o.recebido) },
                { rotulo: 'Falta', valor: emReais(o.restante) },
              ]}
              acoes={
                <>
                  <button type="button" className="bt bt-mini" onClick={() => setRecebendo(o)}>receber NF</button>
                  {podeReceberLocacao && (
                    <button type="button" className="bt bt-mini bt-neutro" onClick={() => setRecebendoLocacao(o)}>locação</button>
                  )}
                </>
              }
            />
          ))}
        </div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                <th>O.C.</th><th>Obra/centro</th><th>Fornecedor</th>
                <th>Total</th><th>Recebido</th><th>Falta</th><th></th>
              </tr>
            </thead>
            <tbody>
              {ordens.map(o => (
                <tr key={o.ordem_compra_id}>
                  <td>{o.numero || '—'}</td>
                  <td>{o.obra_centro_custo || '—'}</td>
                  <td>{o.fornecedor_nome || '—'}</td>
                  <td>{emReais(o.total)}</td>
                  <td>{emReais(o.recebido)}</td>
                  <td>{emReais(o.restante)}</td>
                  <td style={{ display: 'flex', gap: 8 }}>
                    <button type="button" className="bt bt-mini" onClick={() => setRecebendo(o)}>
                      receber NF
                    </button>
                    {podeReceberLocacao && (
                      <button type="button" className="bt bt-mini bt-neutro" onClick={() => setRecebendoLocacao(o)}>
                        locação
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {recebendo && (
        <ReceberNF
          ordem={recebendo}
          aoFechar={() => setRecebendo(null)}
          aoSalvar={() => { setRecebendo(null); void carregar() }}
        />
      )}

      {recebendoLocacao && (
        <ReceberLocacao
          ordemCompraId={recebendoLocacao.ordem_compra_id}
          aoFechar={() => setRecebendoLocacao(null)}
          aoSalvar={() => { setRecebendoLocacao(null); void carregar() }}
        />
      )}
    </>
  )
}
