// rev 1 — Planilhas de controle (2.5)
//
// SÃO DUAS VISÕES DA MESMA TABELA
//
//	5.1  todos os gerados, com a coluna "lançado" (traço ou marca)
//	5.2  só os lançados, com a data do lançamento e SEM a coluna "lançado" —
//	     que ali teria o mesmo valor em todas as linhas
//
// SOBRE `faturado` E `pago`
//
//	Hoje são sempre "não", porque quem as escreve é o módulo financeiro, que
//	ainda não existe. A tela DIZ isso, em vez de mostrar uma coluna de traços que
//	parece informação. No sistema antigo essas duas colunas existiam, ninguém as
//	escrevia, e a planilha reportava falso para sempre como se fosse verdade.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useAlturaDeTela } from '../../componentes/alturaDeTela'
import { motor, baixarDoMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta, Paginacao } from './Arquivos'
import { emReais, emDataHora, contaPorExtenso, type Orcamento, type Pagina } from './tipos'

export function Planilhas({ voltar }: { voltar: () => void }) {
  const [tipo, setTipo] = useState<'gerados' | 'lancados'>('gerados')
  const [pagina, setPagina] = useState<Pagina<Orcamento> | null>(null)
  const [numero, setNumero] = useState(1)
  const [por, setPor] = useState(100)
  const [conta, setConta] = useState('')
  const [de, setDe] = useState('')
  const [ate, setAte] = useState('')
  const [ticket, setTicket] = useState('')
  const [erro, setErro] = useState('')
  const [abrindoExtracao, setAbrindoExtracao] = useState(false)

  const filtro = useCallback(() => {
    const q = new URLSearchParams({ tipo, pagina: String(numero), por: String(por) })
    if (conta) q.set('conta', conta)
    if (de) q.set('de', de)
    if (ate) q.set('ate', ate)
    if (ticket) q.set('ticket', ticket)
    return q
  }, [tipo, numero, por, conta, de, ate, ticket])

  const carregar = useCallback(async () => {
    try {
      setPagina(await motor<Pagina<Orcamento>>('/orcamentos/planilhas?' + filtro()))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar a planilha.')
    }
  }, [filtro])

  useEffect(() => { void carregar() }, [carregar])

  // A EXTRAÇÃO OBEDECE AOS FILTROS
  //   O mesmo conjunto de parâmetros que monta a tela monta o arquivo. Se a
  //   extração tivesse os próprios filtros, o Excel e a tela passariam a
  //   discordar — e os dois números pareceriam certos separados.
  async function extrair(formato: 'xlsx' | 'pdf') {
    setAbrindoExtracao(false)
    try {
      await baixarDoMotor(`/orcamentos/planilhas.${formato}?` + filtro())
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui gerar o arquivo.')
    }
  }

  const soLancados = tipo === 'lancados'

  const painel = useRef<HTMLDivElement>(null)
  useAlturaDeTela(painel, [pagina, tipo])

  return (
    <div className="orc-tela">
      <BarraDeVolta
        voltar={voltar}
        titulo="Planilhas de controle"
        direita={
          <span
            className="orc-extrair"
            onMouseEnter={() => setAbrindoExtracao(true)}
            onMouseLeave={() => setAbrindoExtracao(false)}
          >
            <button type="button" className="orc-bt" onClick={() => setAbrindoExtracao(!abrindoExtracao)}>
              Extrair ▾
            </button>
            {abrindoExtracao && (
              <span className="orc-extrair-menu">
                <button type="button" onClick={() => void extrair('pdf')}>PDF</button>
                <button type="button" onClick={() => void extrair('xlsx')}>Excel</button>
              </span>
            )}
          </span>
        }
      />

      <div className="orc-filtros">
        <select value={tipo} onChange={e => { setNumero(1); setTipo(e.target.value as 'gerados' | 'lancados') }}>
          <option value="gerados">orçamentos gerados</option>
          <option value="lancados">só os lançados</option>
        </select>
        <select value={conta} onChange={e => { setNumero(1); setConta(e.target.value) }}>
          <option value="">todas as contas</option>
          <option value="instalacoes">Instalações</option>
          <option value="civil">Civil</option>
        </select>
        <input
          className="orc-busca-ticket"
          inputMode="numeric"
          placeholder="ticket"
          value={ticket}
          maxLength={6}
          onChange={e => { setNumero(1); setTicket(e.target.value.replace(/\D/g, '')) }}
        />
        <span className="orc-periodo">
          <input type="date" value={de} onChange={e => { setNumero(1); setDe(e.target.value) }} />
          <span>até</span>
          <input type="date" value={ate} onChange={e => { setNumero(1); setAte(e.target.value) }} />
        </span>
      </div>

      {erro && <p className="erro">{erro}</p>}

      <div className="orc-lista" ref={painel}>
        <div className="orc-lista-cab">
          <h2>{soLancados ? 'Orçamentos lançados' : 'Orçamentos gerados'}</h2>
          <em>{pagina ? `${pagina.total.toLocaleString('pt-BR')} no filtro` : '—'}</em>
        </div>

        {!pagina ? <Carregando /> : (
          <div className="orc-rolagem">
            <table className="orc-tabela">
              <thead>
                <tr>
                  <th>Ticket</th><th>Nota</th><th>DAV</th>
                  <th style={{ textAlign: 'right' }}>Valor</th>
                  <th>{soLancados ? 'Lançado em' : 'Data'}</th>
                  <th>Conta</th>
                  {!soLancados && <th style={{ textAlign: 'center' }}>Lançado</th>}
                  <th style={{ textAlign: 'center' }}>Faturado</th>
                  <th style={{ textAlign: 'center' }}>Pago</th>
                </tr>
              </thead>
              <tbody>
                {pagina.linhas.length === 0 && (
                  <tr><td colSpan={soLancados ? 8 : 9} className="orc-vazio">Nada por aqui com este filtro.</td></tr>
                )}
                {pagina.linhas.map(o => (
                  <tr key={o.id}>
                    <td><span className="orc-nome">{o.ticket}{o.parte > 1 ? `-${o.parte}` : ''}</span>
                      {o.loja && <span className="orc-detalhe">{o.loja}</span>}</td>
                    <td>{o.notas ?? '–'}</td>
                    <td>{o.davs ?? '–'}</td>
                    <td style={{ textAlign: 'right' }}>{emReais(o.valor)}</td>
                    <td>{emDataHora(soLancados ? o.lancado_em : o.criado_em)}</td>
                    <td>{contaPorExtenso(o.conta)}</td>
                    {!soLancados && <td style={{ textAlign: 'center' }}><Marca v={o.status === 'lancado'} /></td>}
                    <td style={{ textAlign: 'center' }}><Marca v={o.faturado} /></td>
                    <td style={{ textAlign: 'center' }}><Marca v={o.pago} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <Paginacao pagina={pagina} por={por} aoTrocarPagina={setNumero} aoTrocarPor={n => { setPor(n); setNumero(1) }} />
      </div>

      <p className="orc-nota-de-rodape">
        <b>Faturado</b> e <b>pago</b> ainda não são preenchidos por ninguém: quem vai escrevê-los é o
        módulo financeiro, que ainda não existe. Enquanto isso, as duas colunas mostram traço — e o
        traço aqui quer dizer “ninguém informou”, não “não foi pago”.
      </p>
    </div>
  )
}

/** Traço para não, marca para sim — como o dono pediu. */
function Marca({ v }: { v: boolean }) {
  return v
    ? <span className="orc-marca" title="sim">✓</span>
    : <span className="orc-traco" title="não">–</span>
}
