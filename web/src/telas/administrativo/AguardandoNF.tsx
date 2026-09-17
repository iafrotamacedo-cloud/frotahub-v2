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
//
// "VER OC" ABRE O PDF ORIGINAL (17/09/2026, obra piloto MSL Fátima)
//
//	O almoxarife recebendo a nota às vezes precisa conferir o que a OC pediu
//	de verdade. `GET /administrativo/nf/ordens/{id}/arquivo` existe só pra
//	isto — não é a mesma rota que Compras usa (`arquivoDaOrdem`, atrás de
//	COMPRAS_ORDENS_GERENCIAR): esta fica atrás de `COMPRAS_NF_RECEBER` e
//	peneirada pela obra, senão o link devolveria 403 bem na cara de quem
//	ele foi feito pra atender.
//
// "ENVIAR P/ CORREÇÃO" — SÓ QUANDO O DESVIO É GRANDE DEMAIS PRO AUTOMÁTICO
// (migração 076)
//
//	Uma OC com divergência dentro de 3% já sai sozinha desta lista assim que
//	a nota é recebida (`avaliarDivergenciaOC`, no motor) — nunca aparece
//	aqui pra alguém mandar. Este botão é só pro caso de fora da faixa: a
//	obra (RO) decide, na hora, que aquele desvio grande também é a OC que
//	está errada, não uma nota parcial de verdade. Só aparece quando já tem
//	algo recebido — mandar uma OC com zero recebido pra correção não faz
//	sentido nenhum.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { Confirmar } from '../../componentes/Confirmar'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
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
  const [vendoOC, setVendoOC] = useState<{ ordem: OrdemAguardandoNF; endereco: string; nome: string } | null>(null)
  const [mandandoCorrecao, setMandandoCorrecao] = useState<OrdemAguardandoNF | null>(null)
  const [processando, setProcessando] = useState<string | null>(null)
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

  async function abrirOC(o: OrdemAguardandoNF) {
    try {
      const r = await motor<{ url: string; nome: string }>(`/administrativo/nf/ordens/${o.ordem_compra_id}/arquivo`)
      setVendoOC({ ordem: o, endereco: r.url, nome: r.nome })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir esta O.C.')
    }
  }

  async function confirmarEnviarCorrecao() {
    if (!mandandoCorrecao) return
    const o = mandandoCorrecao
    setProcessando(o.ordem_compra_id)
    try {
      await motor(`/administrativo/nf/ordens/${o.ordem_compra_id}/marcar-correcao`, { metodo: 'POST' })
      setOrdens(atual => (atual ? atual.filter(x => x.ordem_compra_id !== o.ordem_compra_id) : atual))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui mandar esta OC para a correção.')
    } finally {
      setProcessando(null)
    }
  }

  if (vendoOC) {
    return (
      <VisorDeDocumento
        titulo={`O.C. ${vendoOC.ordem.numero ?? '—'}`}
        endereco={vendoOC.endereco}
        nomeSugerido={vendoOC.nome}
        voltar={() => setVendoOC(null)}
      />
    )
  }

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
              onClick={() => void abrirOC(o)}
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
                  {o.recebido > 0 && (
                    <button
                      type="button" className="bt bt-mini bt-neutro" disabled={processando === o.ordem_compra_id}
                      onClick={() => setMandandoCorrecao(o)}
                    >
                      enviar p/ correção
                    </button>
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
                  <td>
                    <button type="button" className="bt-como-link" onClick={() => void abrirOC(o)}>
                      {o.numero || '—'}
                    </button>
                  </td>
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
                    {o.recebido > 0 && (
                      <button
                        type="button" className="bt bt-mini bt-neutro" disabled={processando === o.ordem_compra_id}
                        onClick={() => setMandandoCorrecao(o)}
                      >
                        enviar p/ correção
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
          perfil={perfil}
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

      {mandandoCorrecao && (
        <Confirmar
          titulo="Enviar para correção de OC?"
          mensagem={`A OC ${mandandoCorrecao.numero ?? '—'} sai de Aguardando NF e vai pra fila de correção, em Compras — quem corrige é o RC.`}
          aoConfirmar={() => void confirmarEnviarCorrecao()}
          aoFechar={() => setMandandoCorrecao(null)}
        />
      )}
    </>
  )
}
