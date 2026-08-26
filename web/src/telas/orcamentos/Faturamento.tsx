// rev 1 — Faturar ao cliente (2.6)
//
// DUAS TELAS, PORQUE SÃO DUAS PERGUNTAS
//
//	A faturar   o que ainda não foi cobrado, agrupado em células loja×conta —
//	            uma célula é um PCO dele, que vira uma nota. É daqui que sai a
//	            planilha do mês.
//
//	Faturas     o que já saiu, e quanto voltou. É onde os números de PCO e de
//	            nota são anotados, e onde a diferença entre faturado e recebido
//	            aparece.
//
// POR QUE A FILA NÃO TEM FILTRO DE MÊS
//
//	Porque o mês não é o critério — o critério é "ainda não foi cobrado". Em
//	julho a planilha fechou no dia 29 e a leva do dia 31 rolou para agosto. Um
//	filtro por mês teria feito aqueles 39 orçamentos sumirem da cobrança.
import { useCallback, useEffect, useState } from 'react'
import { motor, baixarDoMotor } from '../../motor/cliente'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta } from './Arquivos'
import {
  emReais, emData, contaPorExtenso,
  type Faturamento as Dados, type Faturas as DadosDeFaturas, type Fatura,
} from './tipos'

export function Faturamento({ voltar }: { voltar: () => void }) {
  const [aba, setAba] = useState<'fila' | 'emitidas'>('fila')
  return (
    <div className="orc-tela">
      <BarraDeVolta
        voltar={voltar}
        titulo="Faturar ao cliente"
        direita={
          <span className="fat-abas">
            <button type="button" className={aba === 'fila' ? 'ativa' : ''} onClick={() => setAba('fila')}>
              A faturar
            </button>
            <button type="button" className={aba === 'emitidas' ? 'ativa' : ''} onClick={() => setAba('emitidas')}>
              Faturas
            </button>
          </span>
        }
      />
      {aba === 'fila' ? <Fila /> : <Emitidas />}
    </div>
  )
}

// ---------------------------------------------------------------------------
// A fila — o que vai na planilha deste mês
// ---------------------------------------------------------------------------

function Fila() {
  const [dados, setDados] = useState<Dados | null>(null)
  const [erro, setErro] = useState('')
  const [recado, setRecado] = useState('')
  const [fechando, setFechando] = useState(false)
  const [confirmando, setConfirmando] = useState(false)
  // PDF É DOCUMENTO: ABRE NA TELA (ver `claude/padroes-de-tela.md`).
  const [vendoPDF, setVendoPDF] = useState(false)

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<Dados>('/orcamentos/faturamento'))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar o faturamento.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function extrair(formato: 'xlsx' | 'pdf') {
    // O Excel é matéria-prima: ninguém "vê" uma planilha, leva-se ela para
    // outro programa. O PDF é peça de leitura, e peça de leitura abre aqui.
    if (formato === 'pdf') { setVendoPDF(true); return }
    try {
      await baixarDoMotor(`/orcamentos/faturamento.${formato}`)
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui gerar o arquivo.')
    }
  }

  // FECHAR É IRREVERSÍVEL PELA TELA, E POR ISSO PERGUNTA ANTES
  //   Depois de fechar, estes orçamentos somem da fila — é exatamente o que
  //   impede cobrar duas vezes. Desfazer existe, mas é no banco.
  async function fechar() {
    if (!dados) return
    setFechando(true)
    setRecado('')
    try {
      const r = await motor<{ faturas: number; orcamentos: number; competencia: string }>(
        '/orcamentos/faturamento/fechar',
        { metodo: 'POST', corpo: { competencia: dados.competencia } },
      )
      setRecado(`Competência ${r.competencia} fechada: ${r.orcamentos} orçamentos em ${r.faturas} faturas.`)
      setConfirmando(false)
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui fechar a competência.')
    } finally {
      setFechando(false)
    }
  }

  if (vendoPDF) {
    return (
      <VisorDeDocumento
        caminho="/orcamentos/faturamento.pdf"
        nomeSugerido="faturamento.pdf"
        titulo="Faturamento ao cliente"
        voltar={() => setVendoPDF(false)}
      />
    )
  }

  if (erro && !dados) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  const vazio = dados.orcamentos === 0

  return (
    <>
      <div className="fat-resumo">
        <Cartao titulo="A faturar" valor={String(dados.orcamentos)} rodape="orçamentos" />
        <Cartao titulo="Valor" valor={emReais(dados.valor)} rodape={`${dados.faturas} faturas`} />
        <Cartao
          titulo="Período"
          valor={dados.desde ? `${emData(dados.desde)} – ${emData(dados.ate)}` : '–'}
          rodape={`competência ${dados.competencia}`}
        />
      </div>

      {/* Os dois avisos que mudam o que a pessoa faz, e nada além disso. */}
      {dados.nao_lancados > 0 && (
        <p className="fat-aviso">
          {dados.nao_lancados} {dados.nao_lancados === 1 ? 'orçamento ainda não subiu' : 'orçamentos ainda não subiram'} para
          o Trílogo. Isso não impede faturar — em julho dois foram cobrados e pagos sem custo lá — mas vale conferir antes de enviar.
        </p>
      )}
      {dados.sem_destino > 0 && (
        <p className="fat-aviso ruim">
          {dados.sem_destino} sem loja ou sem conta. Estes <strong>não entram em fatura nenhuma</strong> —
          não existe PCO para "ninguém". Resolva antes de fechar, ou eles ficam na fila para o mês que vem.
        </p>
      )}
      {erro && <p className="erro">{erro}</p>}
      {recado && <p className="fat-recado">{recado}</p>}

      <div className="orc-lista">
        <div className="orc-lista-cab">
          <h2>Uma fatura por loja e conta</h2>
          <em>{dados.faturas} {dados.faturas === 1 ? 'fatura' : 'faturas'}</em>
        </div>

        <div className="orc-rolagem">
          <table className="orc-tabela">
            <thead>
              <tr>
                <th>Loja</th><th>Conta</th>
                <th style={{ textAlign: 'right' }}>Orçamentos</th>
                <th style={{ textAlign: 'right' }}>Valor</th>
              </tr>
            </thead>
            <tbody>
              {vazio && (
                <tr><td colSpan={4} className="orc-vazio">Nada a faturar — tudo que existe já está numa fatura.</td></tr>
              )}
              {dados.celulas.map(c => (
                <tr key={c.unidade_id + c.conta}>
                  <td><span className="orc-nome">{c.loja}</span></td>
                  <td>{contaPorExtenso(c.conta)}</td>
                  <td style={{ textAlign: 'right' }}>{c.orcamentos}</td>
                  <td style={{ textAlign: 'right' }}>{emReais(c.valor)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="fat-acoes">
        <button type="button" className="orc-bt" disabled={vazio} onClick={() => void extrair('xlsx')}>
          Planilha do cliente (Excel)
        </button>
        <button type="button" className="orc-bt" disabled={vazio} onClick={() => void extrair('pdf')}>
          PDF
        </button>
        {!confirmando ? (
          <button type="button" className="orc-bt forte" disabled={vazio} onClick={() => setConfirmando(true)}>
            Fechar {dados.competencia}
          </button>
        ) : (
          <span className="fat-confirma">
            <span>
              Fechar {dados.competencia} tira estes {dados.orcamentos} orçamentos da fila. É o que impede cobrá-los de novo.
            </span>
            <button type="button" className="orc-bt forte" disabled={fechando} onClick={() => void fechar()}>
              {fechando ? 'fechando…' : 'confirmar'}
            </button>
            <button type="button" className="orc-bt" onClick={() => setConfirmando(false)}>cancelar</button>
          </span>
        )}
      </div>
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

// ---------------------------------------------------------------------------
// As faturas emitidas — e o que voltou de cada uma
// ---------------------------------------------------------------------------

type Rascunho = Partial<Pick<Fatura, 'pco_numero' | 'nf_numero' | 'nf_em' | 'recebido_em'>> & {
  valor_recebido?: string
}

function Emitidas() {
  const [dados, setDados] = useState<DadosDeFaturas | null>(null)
  const [erro, setErro] = useState('')
  const [competencia, setCompetencia] = useState('')
  const [rascunhos, setRascunhos] = useState<Record<string, Rascunho>>({})
  const [salvando, setSalvando] = useState('')

  const carregar = useCallback(async () => {
    try {
      const q = competencia ? '?competencia=' + encodeURIComponent(competencia) : ''
      setDados(await motor<DadosDeFaturas>('/orcamentos/faturas' + q))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar as faturas.')
    }
  }, [competencia])

  useEffect(() => { void carregar() }, [carregar])

  function anotar(id: string, campo: keyof Rascunho, valor: string) {
    setRascunhos(r => ({ ...r, [id]: { ...r[id], [campo]: valor } }))
  }

  // Só o que a pessoa mexeu é enviado. Campo ausente não é campo vazio — é o
  // que deixa marcar o recebimento sem apagar o número da nota que já estava lá.
  async function salvar(f: Fatura) {
    const r = rascunhos[f.id]
    if (!r) return
    setSalvando(f.id)
    try {
      const corpo: Record<string, unknown> = {}
      if (r.pco_numero !== undefined) corpo.pco_numero = r.pco_numero
      if (r.nf_numero !== undefined) corpo.nf_numero = r.nf_numero
      if (r.nf_em !== undefined) corpo.nf_em = r.nf_em
      if (r.recebido_em !== undefined) corpo.recebido_em = r.recebido_em
      if (r.valor_recebido !== undefined) {
        const n = Number(r.valor_recebido.replace(/\./g, '').replace(',', '.'))
        corpo.valor_recebido = r.valor_recebido.trim() === '' || Number.isNaN(n) ? null : n
      }
      await motor(`/orcamentos/faturas/${f.id}`, { metodo: 'POST', corpo })
      setRascunhos(x => { const y = { ...x }; delete y[f.id]; return y })
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui anotar a fatura.')
    } finally {
      setSalvando('')
    }
  }

  if (erro && !dados) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  return (
    <>
      <div className="fat-resumo">
        <Cartao titulo="Faturado" valor={emReais(dados.faturado)} rodape={`${dados.faturas.length} faturas`} />
        <Cartao titulo="Recebido" valor={emReais(dados.recebido)} rodape="informado pela tela" />
        <Cartao titulo="A receber" valor={emReais(dados.a_receber)} rodape="faturado menos recebido" />
      </div>

      <div className="orc-filtros">
        <input
          type="month"
          value={competencia}
          onChange={e => setCompetencia(e.target.value)}
          aria-label="competência"
        />
        {competencia && (
          <button type="button" className="orc-bt" onClick={() => setCompetencia('')}>todas</button>
        )}
      </div>

      {erro && <p className="erro">{erro}</p>}

      <div className="orc-lista">
        <div className="orc-lista-cab">
          <h2>Faturas emitidas</h2>
          <em>{competencia || 'todas as competências'}</em>
        </div>

        <div className="orc-rolagem">
          <table className="orc-tabela">
            <thead>
              <tr>
                <th>Competência</th><th>Loja</th><th>Conta</th>
                <th style={{ textAlign: 'right' }}>Valor</th>
                <th>PCO</th><th>Nota</th><th>Recebido em</th>
                <th style={{ textAlign: 'right' }}>Valor recebido</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {dados.faturas.length === 0 && (
                <tr><td colSpan={9} className="orc-vazio">Nenhuma fatura nesta competência.</td></tr>
              )}
              {dados.faturas.map(f => {
                const r = rascunhos[f.id] ?? {}
                const mexeu = Object.keys(r).length > 0
                return (
                  <tr key={f.id} className={f.recebida ? 'fat-paga' : ''}>
                    <td>{f.competencia}</td>
                    <td><span className="orc-nome">{f.loja}</span>
                      <span className="orc-detalhe">{f.orcamentos} orçamentos</span></td>
                    <td>{contaPorExtenso(f.conta)}</td>
                    <td style={{ textAlign: 'right' }}>{emReais(f.valor)}</td>
                    <td>
                      <input
                        className="fat-campo" maxLength={40} placeholder="—"
                        value={r.pco_numero ?? f.pco_numero ?? ''}
                        onChange={e => anotar(f.id, 'pco_numero', e.target.value)}
                      />
                    </td>
                    <td>
                      <input
                        className="fat-campo" maxLength={40} placeholder="—"
                        value={r.nf_numero ?? f.nf_numero ?? ''}
                        onChange={e => anotar(f.id, 'nf_numero', e.target.value)}
                      />
                    </td>
                    <td>
                      <input
                        type="date" className="fat-campo"
                        value={r.recebido_em ?? f.recebido_em ?? ''}
                        onChange={e => anotar(f.id, 'recebido_em', e.target.value)}
                      />
                    </td>
                    <td style={{ textAlign: 'right' }}>
                      <input
                        className="fat-campo fat-campo-valor" inputMode="decimal" placeholder="—"
                        value={r.valor_recebido ?? (f.valor_recebido === null ? '' : String(f.valor_recebido).replace('.', ','))}
                        onChange={e => anotar(f.id, 'valor_recebido', e.target.value)}
                      />
                      {/* A diferença só aparece quando existe. Zero em toda linha
                          vira ruído e some do olho justamente quando importa. */}
                      {f.recebida && Math.abs(f.diferenca) >= 0.01 && (
                        <span className="fat-diferenca" title="faturado menos recebido">
                          {f.diferenca > 0 ? 'faltam ' : 'sobram '}{emReais(Math.abs(f.diferenca))}
                        </span>
                      )}
                    </td>
                    <td>
                      {mexeu && (
                        <button
                          type="button" className="orc-bt"
                          disabled={salvando === f.id}
                          onClick={() => void salvar(f)}
                        >
                          {salvando === f.id ? '…' : 'salvar'}
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </div>
    </>
  )
}
