// rev 1 — A pagar › pedido de faturamento
//
// O QUE ESTA TELA RESOLVE
//
//	A Rodrigues não emite nota a cada compra: ela emite um DAV e vai
//	acumulando. De tempos em tempos mandamos a relação das DAVs em aberto e ela
//	emite UMA nota cobrindo todas. Sem esse pedido o DAV fica solto, e material
//	comprado que ninguém cobra vira surpresa numa conferência de conta meses
//	depois.
//
// GERAR A RELAÇÃO NÃO MEXE EM NADA; FECHAR MEXE
//
//	Enquanto o pedido não é fechado, esta tela é uma consulta: dá para baixar o
//	Excel quantas vezes quiser, mudar o corte, conferir. Fechar é o passo que
//	tira as DAVs da fila — e é por isso que ele é um botão separado, com o
//	número do que vai levar escrito nele.
//
//	Confundir os dois seria o erro caro: alguém baixa a relação para conferir,
//	sem intenção de mandar, e as DAVs somem da fila do mês.
import { useCallback, useEffect, useState } from 'react'
import { motor, baixarDoMotor } from '../../motor/cliente'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { Carregando } from '../../componentes/Carregando'
import { emReais, emData, emDataHora } from '../orcamentos/tipos'

interface DAV {
  id: string
  numero: string | null
  dav_numero: string | null
  emissao: string | null
  valor_total: number | null
  nome_arquivo: string
  inserido_em: string
}

interface Pedido {
  id: string
  numero: number
  ate: string
  fechado_em: string | null
  enviado_em: string | null
  davs: number
  valor: number
}

interface Aberto {
  davs: DAV[]
  quantas: number
  valor: number
  sem_data: number
  pedidos: Pedido[]
}

export function APagar() {
  const [dados, setDados] = useState<Aberto | null>(null)
  const [ate, setAte] = useState(hoje())
  const [erro, setErro] = useState('')
  // O pedido é documento: abre na tela, e o salvar mora lá dentro.
  const [vendoPedido, setVendoPedido] = useState(false)
  const [recado, setRecado] = useState('')
  const [fechando, setFechando] = useState(false)

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<Aberto>('/orcamentos/pedido?ate=' + ate))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar as DAVs em aberto.')
    }
  }, [ate])

  useEffect(() => { void carregar() }, [carregar])

  async function fechar() {
    if (!dados?.quantas) return
    setFechando(true)
    try {
      const r = await motor<{ numero: number; davs: number }>('/orcamentos/pedido/fechar',
        { metodo: 'POST', corpo: { ate } })
      setRecado(`Pedido nº ${r.numero} fechado com ${r.davs} DAV${r.davs > 1 ? 's' : ''}. `
        + 'Elas saíram da fila — agora é mandar a relação para a Rodrigues.')
      setErro('')
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui fechar o pedido.')
    } finally { setFechando(false) }
  }

  async function marcar(p: Pedido, o: 'enviado' | 'reabrir') {
    try {
      await motor(`/orcamentos/pedido/${p.id}/${o}`, { metodo: 'POST' })
      setRecado(o === 'enviado'
        ? `Pedido nº ${p.numero} marcado como enviado.`
        : `Pedido nº ${p.numero} reaberto — as DAVs voltaram para a fila.`)
      setErro('')
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui.')
    }
  }

  if (vendoPedido) {
    return (
      <VisorDeDocumento
        caminho={'/orcamentos/pedido.pdf?ate=' + ate}
        nomeSugerido={`pedido-de-faturamento-${ate}.pdf`}
        titulo={`Pedido de faturamento · até ${ate}`}
        voltar={() => setVendoPedido(false)}
      />
    )
  }

  return (
    <div className="orc-tela">
      <p className="orc-aviso-fila">
        A Rodrigues emite <b>DAV</b> e acumula. Aqui você junta as que ainda não foram
        pedidas e manda a relação — ela emite <b>uma nota</b> cobrindo todas.
      </p>

      {erro && <p className="erro">{erro}</p>}
      {recado && <p className="orc-recado">{recado}</p>}

      {!dados ? <Carregando /> : (
        <>
          <div className="orc-barra-acoes">
            <label>
              emitidas até
              <input type="date" value={ate} onChange={e => setAte(e.target.value)} />
            </label>

            <span className="pg-resumo">
              <b>{dados.quantas}</b> DAV{dados.quantas === 1 ? '' : 's'} em aberto
              <u>{emReais(dados.valor)}</u>
            </span>

            <button type="button" className="orc-bt forte" disabled={!dados.quantas}
              onClick={() => void baixarDoMotor('/orcamentos/pedido.xlsx?ate=' + ate)}>
              baixar a relação (Excel)
            </button>
            {/* O EXCEL BAIXA; O PDF ABRE
                O Excel é matéria-prima: ninguém "vê" uma planilha, leva-se ela
                para outro programa. O PDF é o documento que vai ao fornecedor —
                e documento se confere na tela antes de sair. */}
            <button type="button" className="orc-bt forte" disabled={!dados.quantas}
              onClick={() => setVendoPedido(true)}>
              ver o pedido (PDF)
            </button>

            {/* FECHAR É O ÚNICO BOTÃO QUE MUDA ALGUMA COISA
                Por isso ele diz quantas DAVs vai levar: um botão que só diz
                "fechar" é um botão que se aperta sem saber o tamanho do que se
                está fechando. */}
            <button type="button" className="pg-fechar" disabled={!dados.quantas || fechando}
              onClick={() => void fechar()}
              title="tira estas DAVs da fila e cria o pedido">
              {fechando ? 'fechando…' : `fechar pedido com ${dados.quantas}`}
            </button>
          </div>

          {dados.sem_data > 0 && (
            <p className="orc-aviso-fila">
              {dados.sem_data} DAV{dados.sem_data > 1 ? 's' : ''} sem data de emissão lida —
              {dados.sem_data > 1 ? ' elas entram' : ' ela entra'} no pedido de qualquer corte,
              porque são devidas do mesmo jeito.
            </p>
          )}

          <div className="orc-lista">
            <div className="orc-rolagem">
              <table className="orc-tabela">
                <thead>
                  <tr>
                    <th>DAV</th><th>Emissão</th><th style={{ textAlign: 'right' }}>Valor</th>
                    <th>Arquivo</th><th>Inserida em</th>
                  </tr>
                </thead>
                <tbody>
                  {!dados.davs.length && (
                    <tr><td colSpan={5}><p className="orc-vazio">
                      Nenhuma DAV em aberto até esta data.
                    </p></td></tr>
                  )}
                  {dados.davs.map(d => (
                    <tr key={d.id}>
                      <td><span className="orc-nome">{d.dav_numero ?? d.numero ?? '—'}</span></td>
                      <td>{d.emissao ? emData(d.emissao) : <span className="mut">sem data</span>}</td>
                      <td style={{ textAlign: 'right' }}>{emReais(d.valor_total)}</td>
                      <td>{d.nome_arquivo}</td>
                      <td>{emDataHora(d.inserido_em)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {dados.pedidos.length > 0 && (
            <div className="orc-lista">
              <div className="orc-lista-cab">
                <h2>Pedidos anteriores</h2>
                <em>O que já foi mandado à Rodrigues — e o que ela ainda não faturou</em>
              </div>
              <div className="orc-rolagem">
                <table className="orc-tabela">
                  <thead>
                    <tr>
                      <th>Nº</th><th>Corte</th><th>DAVs</th>
                      <th style={{ textAlign: 'right' }}>Valor</th><th>Situação</th>
                      <th style={{ textAlign: 'right' }}>Ações</th>
                    </tr>
                  </thead>
                  <tbody>
                    {dados.pedidos.map(p => (
                      <tr key={p.id}>
                        <td><span className="orc-nome">{p.numero}</span></td>
                        <td>{emData(p.ate)}</td>
                        <td>{p.davs}</td>
                        <td style={{ textAlign: 'right' }}>{emReais(p.valor)}</td>
                        <td>
                          {p.enviado_em
                            ? <span className="orc-selo ok">enviado {emData(p.enviado_em)}</span>
                            : <span className="orc-selo espera">fechado, não enviado</span>}
                        </td>
                        <td className="orc-acoes">
                          {!p.enviado_em && (
                            <>
                              <button type="button" onClick={() => void marcar(p, 'enviado')}>
                                marcar como enviado
                              </button>
                              {/* REABRIR SÓ ENQUANTO NÃO FOI ENVIADO
                                  Depois disso a Rodrigues tem a relação em mãos,
                                  e mudar a nossa faria as duas discordarem. */}
                              <button type="button" className="mut" onClick={() => void marcar(p, 'reabrir')}>
                                reabrir
                              </button>
                            </>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function hoje(): string {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}
