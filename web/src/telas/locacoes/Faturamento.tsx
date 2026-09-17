// rev 1 — Locações: resumo do faturamento (Fase 5, 17/09/2026)
//
// SÓ O CÁLCULO POR OBRA, SEM CICLO NEM CLIENTE AINDA
//
//	Decisão do dono (17/09/2026): a Fase 5 de hoje fecha só o cálculo por
//	equipamento/período (mês cheio × proporcional, com override) — "cliente
//	de faturamento" e "dia de fechamento" ficam pra quando existir uma
//	segunda frente de verdade (hoje só São Luiz passa da leitura de OC).
//	Esta tela é o primeiro uso visível daquele cálculo: quanto está
//	calculado, somado por obra, sem fechar fatura nenhuma — é um raio-X, não
//	uma cobrança.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { emReais, type ResumoDeFaturamento } from './tipos'

export function Faturamento() {
  const [dados, setDados] = useState<ResumoDeFaturamento | null>(null)
  const [erro, setErro] = useState('')

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<ResumoDeFaturamento>('/locacoes/faturamento'))
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o faturamento.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Faturamento calculado</h1>
          <p>Mês cheio no 1º período, proporcional depois — soma de todos os períodos de todos os equipamentos, até hoje.</p>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {!dados ? (
        <Carregando texto="Carregando..." />
      ) : (
        <>
          <div className="loc-item" style={{ marginBottom: 16 }}>
            <div className="loc-item-cabecalho">
              <span className="loc-item-descricao">Total calculado</span>
              <span className="loc-item-valor" style={{ fontSize: 20, fontWeight: 700 }}>{emReais(dados.total)}</span>
            </div>
          </div>

          {dados.por_obra.length === 0 ? (
            <div className="vazio">Nenhum equipamento com período calculado ainda.</div>
          ) : (
            <div className="tabela-rolo">
              <table className="tabela">
                <thead>
                  <tr><th>Obra</th><th>Total calculado</th></tr>
                </thead>
                <tbody>
                  {[...dados.por_obra].sort((a, b) => b.total_calculado - a.total_calculado).map(o => (
                    <tr key={o.obra_centro_custo}>
                      <td>{o.obra_centro_custo}</td>
                      <td>{emReais(o.total_calculado)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </>
  )
}
