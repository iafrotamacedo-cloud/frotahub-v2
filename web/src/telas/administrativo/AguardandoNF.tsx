// rev 1 — Notas Fiscais > Aguardando (o almoxarife recebe aqui)
//
// SÓ AS OBRAS QUE ESTE LOGIN TEM LIBERADAS
//
//	O motor já filtra por `centro_custo_acessos` (ver `temAcessoAObra` em
//	notas_fiscais.go) — esta tela nunca precisa saber a regra, só mostrar o
//	que veio.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { ReceberNF } from './ReceberNF'
import { emReais, type OrdemAguardandoNF } from './tipos'

export function AguardandoNF() {
  const [ordens, setOrdens] = useState<OrdemAguardandoNF[] | null>(null)
  const [erro, setErro] = useState('')
  const [recebendo, setRecebendo] = useState<OrdemAguardandoNF | null>(null)

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
                  <td>
                    <button type="button" className="bt bt-mini" onClick={() => setRecebendo(o)}>
                      receber NF
                    </button>
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
    </>
  )
}
