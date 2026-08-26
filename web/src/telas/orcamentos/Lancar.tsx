// rev 1 — Lançar orçamentos (2.3)
//
// Duas ações e uma lista:
//
//	GERAR   transforma as notas lidas em orçamentos, aplicando margem e teto
//	LANÇAR  sobe o PDF e cria o custo no Trílogo, por API
//
// POR QUE GERAR E LANÇAR SÃO SEPARADOS
//
//	Porque entre um e outro existe uma decisão humana: conferir o documento. Um
//	botão só — "gerar e lançar" — parece mais prático e é justamente o que faz um
//	erro de leitura chegar ao cliente sem passar por ninguém.
import { useCallback, useEffect, useState } from 'react'
import { motor, baixarDoMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta, Paginacao, ResumoDaGeracao } from './Arquivos'
import {
  emReais, emDataHora, contaPorExtenso,
  type Orcamento, type Pagina, type ResultadoDaGeracao,
} from './tipos'

export function Lancar({ voltar }: { voltar: () => void }) {
  const [pagina, setPagina] = useState<Pagina<Orcamento> | null>(null)
  const [numero, setNumero] = useState(1)
  const [por, setPor] = useState(100)
  const [status, setStatus] = useState('gerado')
  const [erro, setErro] = useState('')
  const [aviso, setAviso] = useState('')
  const [gerando, setGerando] = useState(false)
  const [lancando, setLancando] = useState<string | null>(null)
  const [resultados, setResultados] = useState<ResultadoDaGeracao[] | null>(null)
  const [bloqueio, setBloqueio] = useState<BloqueioDoTeto | null>(null)

  const carregar = useCallback(async () => {
    try {
      const q = new URLSearchParams({ pagina: String(numero), por: String(por), status })
      setPagina(await motor<Pagina<Orcamento>>('/orcamentos?' + q))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar a lista.')
    }
  }, [numero, por, status])

  useEffect(() => { void carregar() }, [carregar])

  async function gerar() {
    setGerando(true)
    setResultados(null)
    try {
      const r = await motor<{ resultados: ResultadoDaGeracao[] }>('/orcamentos/gerar', { metodo: 'POST', corpo: {} })
      setResultados(r.resultados)
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui gerar.')
    } finally {
      setGerando(false)
    }
  }

  async function lancar(o: Orcamento) {
    setLancando(o.id)
    setAviso('')
    try {
      const r = await motor<{ trilogo_custo_id: number }>(`/orcamentos/ficha/${o.id}/lancar`, { metodo: 'POST' })
      setAviso(`Ticket ${o.ticket}: lançado no Trílogo (custo nº ${r.trilogo_custo_id}).`)
      await carregar()
    } catch (e) {
      // O CONFLITO DE TETO NÃO É UM ERRO QUALQUER
      //   Ele vem com números e com saídas. Mostrar só a frase jogaria fora
      //   justamente a parte que resolve — e é assim que o usuário acaba
      //   montando planilha paralela para entender o que travou.
      const corpo = (e as { corpo?: BloqueioDoTeto }).corpo
      if (corpo?.saidas) setBloqueio(corpo)
      else setErro(e instanceof Error ? e.message : 'Não consegui lançar.')
    } finally {
      setLancando(null)
    }
  }

  async function aprovar(o: Orcamento) {
    const nota = window.prompt(
      `Onde está a aprovação do cliente para o ticket ${o.ticket} (${emReais(o.valor)})?\n` +
      'Escreva a referência — e-mail, conversa, print. Fica registrado com o seu nome.')
    if (!nota) return
    try {
      await motor(`/orcamentos/ficha/${o.id}/aprovar`, { metodo: 'POST', corpo: { nota } })
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui aprovar.')
    }
  }


  return (
    <div className="orc-tela">
      <BarraDeVolta
        voltar={voltar}
        titulo="Lançar orçamentos"
        direita={
          <button type="button" className="orc-bt forte" disabled={gerando} onClick={() => void gerar()}>
            {gerando ? 'Gerando…' : 'Gerar orçamentos das notas lidas'}
          </button>
        }
      />

      {resultados && <ResumoDaGeracao resultados={resultados} fechar={() => setResultados(null)} />}
      {aviso && <p className="orc-ok">{aviso}</p>}
      {erro && <p className="erro">{erro}</p>}
      {bloqueio && <Bloqueio dados={bloqueio} fechar={() => setBloqueio(null)} />}

      <div className="orc-lista">
        <div className="orc-lista-cab">
          <h2>Orçamentos</h2>
          <em>{pagina ? `${pagina.total.toLocaleString('pt-BR')} no filtro` : '—'}</em>
          <select value={status} onChange={e => { setNumero(1); setStatus(e.target.value) }}>
            <option value="gerado">a lançar</option>
            <option value="aguardando_aprovacao">aguardando aprovação</option>
            <option value="lancado">lançados</option>
            <option value="removido">apagados</option>
          </select>
        </div>

        {!pagina ? <Carregando /> : (
          <div className="orc-rolagem">
            <table className="orc-tabela">
              <colgroup>
                <col style={{ width: '10%' }} />
                <col style={{ width: '24%' }} />
                <col style={{ width: '14%' }} />
                <col style={{ width: '12%' }} />
                <col style={{ width: '14%' }} />
                <col />
              </colgroup>
              <thead>
                <tr>
                  <th>Ticket</th><th>Loja</th><th>Nota</th>
                  <th style={{ textAlign: 'right' }}>Valor</th>
                  <th>Gerado em</th>
                  <th style={{ textAlign: 'right' }}>Ações</th>
                </tr>
              </thead>
              <tbody>
                {pagina.linhas.length === 0 && (
                  <tr><td colSpan={6} className="orc-vazio">Nada por aqui com este filtro.</td></tr>
                )}
                {pagina.linhas.map(o => (
                  <tr key={o.id}>
                    <td>
                      <span className="orc-nome">{o.ticket}{o.parte > 1 ? `-${o.parte}` : ''}</span>
                      <span className="orc-detalhe">{contaPorExtenso(o.conta)}{o.rateio ? ' · rateio' : ''}</span>
                    </td>
                    <td title={o.chamado_descricao ?? undefined}>
                      <span className="orc-nome">{o.loja ?? '–'}</span>
                      {/* A DESCRIÇÃO DO CHAMADO SAIU DAQUI
                          Ela vem do Trílogo sem limite de tamanho — um chamado
                          de quatro parágrafos virava uma linha de meia tela, e
                          duas linhas assim já enchiam a página. Quem está
                          lançando precisa de ticket, loja e valor; o texto do
                          chamado é contexto, e contexto não pode empurrar a
                          informação para fora da vista. Continua no `title`,
                          para ler passando o mouse. */}
                    </td>
                    <td>{o.notas ?? '–'}</td>
                    <td style={{ textAlign: 'right' }}>
                      <span className="orc-nome">{emReais(o.valor)}</span>
                      {o.reduzido_pelo_teto && (
                        <span className="orc-detalhe" title="reduzido para caber no teto do ticket">
                          era {emReais(o.valor_antes_do_teto)}
                        </span>
                      )}
                    </td>
                    <td>{emDataHora(o.criado_em)}</td>
                    <td className="orc-acoes">
                      <button type="button" onClick={() => void baixarDoMotor(`/orcamentos/ficha/${o.id}/pdf`)}>pdf</button>
                      {o.status === 'gerado' && (
                        <button type="button" className="forte" disabled={lancando === o.id} onClick={() => void lancar(o)}>
                          {lancando === o.id ? 'lançando…' : 'lançar'}
                        </button>
                      )}
                      {o.status === 'aguardando_aprovacao' && (
                        <button type="button" className="forte" onClick={() => void aprovar(o)}>aprovar</button>
                      )}
                      {o.status === 'lancado' && (
                        <span className="orc-selo ok">custo {o.trilogo_custo_id}</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <Paginacao pagina={pagina} por={por} aoTrocarPagina={setNumero} aoTrocarPor={n => { setPor(n); setNumero(1) }} />
      </div>
    </div>
  )
}

interface BloqueioDoTeto {
  erro: string
  ticket: number
  ja_no_ticket: number
  deste: number
  teto: number
  saidas: string[]
}

/** O bloqueio do teto, com as contas e as saídas visíveis. Nunca um beco. */
function Bloqueio({ dados, fechar }: { dados: BloqueioDoTeto; fechar: () => void }) {
  return (
    <div className="orc-bloqueio">
      <h3>Ticket {dados.ticket} passou do teto</h3>
      <p>{dados.erro}</p>
      <table>
        <tbody>
          <tr><td>já lançado no ticket</td><td>{emReais(dados.ja_no_ticket)}</td></tr>
          <tr><td>este orçamento</td><td>{emReais(dados.deste)}</td></tr>
          <tr><td>teto contratado</td><td>{emReais(dados.teto)}</td></tr>
        </tbody>
      </table>
      <p className="orc-saidas">O que dá para fazer:</p>
      <ul>{dados.saidas.map(s => <li key={s}>{s}</li>)}</ul>
      <button type="button" className="orc-bt" onClick={fechar}>Entendi</button>
    </div>
  )
}
