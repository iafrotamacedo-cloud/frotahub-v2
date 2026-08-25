// rev 1 — Notas e DAVs (2.1) e Notas para rateio (2.2)
//
// É UMA TELA SÓ, COM UMA CHAVE
//
//	As duas fazem exatamente a mesma coisa com o arquivo: inserir, listar, abrir,
//	excluir e desfazer. A diferença é o que vem depois — na fila de rateio o
//	usuário dita os tickets. Duas telas duplicariam a barra de inserção, a
//	tabela, o aviso de exclusão e o desfazer, que são justamente as quatro
//	partes onde um detalhe divergindo passa despercebido.
//
// A EXCLUSÃO NÃO APAGA NADA
//
//	"Excluir" preenche `oculto_em` no banco. O arquivo continua inteiro no
//	armazém, e o desfazer é um update — por isso o aviso pode oferecer "desfazer"
//	sem prazo e sem medo. Com o Dropbox fora do stack, esta coluna é a rede de
//	segurança que a lixeira dele era.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useAlturaDeTela } from '../../componentes/alturaDeTela'
import { motor, enviarArquivos } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import {
  emDataHora, emReais, confiancaEmPalavras,
  type Documento, type Pagina, type ResultadoDaInsercao, type TicketDoDocumento,
} from './tipos'

interface Props {
  fila: 'orcamento' | 'rateio'
  voltar: () => void
}

export function Arquivos({ fila, voltar }: Props) {
  const [pagina, setPagina] = useState<Pagina<Documento> | null>(null)
  const [numero, setNumero] = useState(1)
  const [por, setPor] = useState(100)
  const [busca, setBusca] = useState('')
  const [ocultos, setOcultos] = useState(false)
  const [erro, setErro] = useState('')
  const [ocupado, setOcupado] = useState(false)

  // A nota aberta em tela cheia para receber tickets — o "+" da fila de rateio.
  const [abrindo, setAbrindo] = useState<Documento | null>(null)

  // O aviso de exclusão. Fica até o usuário responder: sumir sozinho
  // transformaria "desfazer" numa corrida contra o relógio.
  const [desfazer, setDesfazer] = useState<{ id: string; nome: string } | null>(null)

  const painel = useRef<HTMLDivElement>(null)
  useAlturaDeTela(painel, [pagina, ocultos, fila])

  const carregar = useCallback(async () => {
    setOcupado(true)
    try {
      const q = new URLSearchParams({
        fila, pagina: String(numero), por: String(por),
        ...(busca ? { busca } : {}),
        ...(ocultos ? { ocultos: '1' } : {}),
      })
      setPagina(await motor<Pagina<Documento>>('/orcamentos/documentos?' + q))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar a lista.')
    } finally {
      setOcupado(false)
    }
  }, [fila, numero, por, busca, ocultos])

  useEffect(() => { void carregar() }, [carregar])

  async function excluir(d: Documento) {
    try {
      await motor(`/orcamentos/documentos/${d.id}`, { metodo: 'DELETE' })
      setDesfazer({ id: d.id, nome: d.nome_arquivo })
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui excluir.')
    }
  }

  async function restaurar(id: string) {
    try {
      await motor(`/orcamentos/documentos/${id}/restaurar`, { metodo: 'POST' })
      setDesfazer(null)
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui restaurar.')
    }
  }

  async function abrirArquivo(d: Documento) {
    try {
      const r = await motor<{ url: string }>(`/orcamentos/documentos/${d.id}/arquivo`)
      window.open(r.url, '_blank', 'noopener')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  if (abrindo) {
    return (
      <FichaDaNota
        documento={abrindo}
        fechar={() => { setAbrindo(null); void carregar() }}
      />
    )
  }

  return (
    <div className="orc-tela">
      <BarraDeVolta voltar={voltar} titulo={fila === 'rateio' ? 'Notas para rateio' : 'Notas e DAVs'} />

      <Insercao fila={fila} aoTerminar={carregar} aoFalhar={setErro} />

      <div className="orc-lista" ref={painel}>
        <div className="orc-lista-cab">
          <h2>{ocultos ? 'Arquivos excluídos' : 'Arquivos inseridos'}</h2>
          <em>{pagina ? `${pagina.total.toLocaleString('pt-BR')} no total` : '—'}</em>

          <input
            className="orc-busca"
            placeholder="Procurar por nome, número ou fornecedor"
            value={busca}
            onChange={e => { setNumero(1); setBusca(e.target.value) }}
          />
          <button
            type="button"
            className={'orc-bt-fino' + (ocultos ? ' ligado' : '')}
            onClick={() => { setNumero(1); setOcultos(!ocultos) }}
          >
            {ocultos ? 'ver os ativos' : 'ver os excluídos'}
          </button>
        </div>

        {erro && <p className="erro" style={{ margin: '10px 16px' }}>{erro}</p>}

        {!pagina && ocupado ? <Carregando /> : (
          <div className="orc-rolagem">
            <table className="orc-tabela">
              <colgroup>
                <col style={{ width: fila === 'rateio' ? '34%' : '46%' }} />
                <col style={{ width: '16%' }} />
                {fila === 'rateio' && <col style={{ width: '22%' }} />}
                <col style={{ width: '14%' }} />
                <col />
              </colgroup>
              <thead>
                <tr>
                  <th>Nome</th>
                  <th>Data de inserção</th>
                  {fila === 'rateio' && <th>Tickets</th>}
                  <th>Leitura</th>
                  <th style={{ textAlign: 'right' }}>Ações</th>
                </tr>
              </thead>
              <tbody>
                {pagina?.linhas.length === 0 && (
                  <tr><td colSpan={fila === 'rateio' ? 5 : 4} className="orc-vazio">
                    {ocultos ? 'Nada foi excluído por aqui.' : 'Nenhum arquivo inserido ainda.'}
                  </td></tr>
                )}
                {pagina?.linhas.map(d => (
                  <tr key={d.id}>
                    <td>
                      <span className="orc-nome">{d.nome_arquivo}</span>
                      {(d.numero || d.emitente_nome) && (
                        <span className="orc-detalhe">
                          {d.numero ? `nº ${d.numero}` : ''}
                          {d.numero && d.emitente_nome ? ' · ' : ''}
                          {d.emitente_nome ?? ''}
                          {d.valor_total ? ` · ${emReais(d.valor_total)}` : ''}
                        </span>
                      )}
                    </td>
                    <td>{emDataHora(d.inserido_em)}</td>
                    {fila === 'rateio' && <td><Tickets numeros={d.ticket_numeros} /></td>}
                    <td><Leitura d={d} /></td>
                    <td className="orc-acoes">
                      <button type="button" onClick={() => void abrirArquivo(d)}>ver</button>
                      {fila === 'rateio' && !ocultos && (
                        <button type="button" className="orc-mais" onClick={() => setAbrindo(d)} title="amarrar tickets">+</button>
                      )}
                      {ocultos
                        ? <button type="button" className="mut" onClick={() => void restaurar(d.id)}>restaurar</button>
                        : <button type="button" className="mut" onClick={() => void excluir(d)}>excluir</button>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <Paginacao
          pagina={pagina}
          por={por}
          aoTrocarPagina={setNumero}
          aoTrocarPor={n => { setPor(n); setNumero(1) }}
        />
      </div>

      {desfazer && (
        <div className="orc-desfaz" role="status">
          <div className="tx">
            <b>Você excluiu o arquivo {desfazer.nome}</b>
            <small>Nada é apagado do armazém. Dá para restaurar quando quiser.</small>
          </div>
          <div className="bts">
            <button type="button" className="orc-bt" onClick={() => void restaurar(desfazer.id)}>Desfazer</button>
            <button type="button" className="orc-bt forte" onClick={() => setDesfazer(null)}>Ok</button>
          </div>
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// a barra de inserção
// ---------------------------------------------------------------------------

/**
 * A barra de 4 cm, com a única frase que o dono pediu.
 *
 * NÃO TEM BOTÃO DE ATUALIZAR, DE PROPÓSITO
 *   A lista se recarrega sozinha depois de inserir. Um botão "atualizar" numa
 *   tela que já sabe quando mudou é um botão que só existe para o usuário
 *   desconfiar do que está vendo.
 */
function Insercao({ fila, aoTerminar, aoFalhar }: {
  fila: string
  aoTerminar: () => Promise<void>
  aoFalhar: (s: string) => void
}) {
  const [escolhidos, setEscolhidos] = useState<File[]>([])
  const [enviando, setEnviando] = useState(false)
  const [sobre, setSobre] = useState(false)
  const campo = useRef<HTMLInputElement>(null)

  async function inserir() {
    if (!escolhidos.length || enviando) return
    setEnviando(true)
    try {
      const r = await enviarArquivos<{ arquivos: ResultadoDaInsercao[] }>(
        '/orcamentos/documentos?fila=' + fila, escolhidos)
      const ruins = r.arquivos.filter(a => a.erro)
      const repetidos = r.arquivos.filter(a => a.ja_existia)
      const partes: string[] = []
      if (ruins.length) partes.push(ruins.map(a => `${a.nome}: ${a.erro}`).join(' · '))
      if (repetidos.length) partes.push(`${repetidos.length} já estava${repetidos.length > 1 ? 'm' : ''} na lista`)
      aoFalhar(partes.join(' — '))
      setEscolhidos([])
      if (campo.current) campo.current.value = ''
      await aoTerminar()
    } catch (e) {
      aoFalhar(e instanceof Error ? e.message : 'Não consegui inserir os arquivos.')
    } finally {
      setEnviando(false)
    }
  }

  return (
    <div
      className={'orc-insercao' + (sobre ? ' sobre' : '')}
      onDragOver={e => { e.preventDefault(); setSobre(true) }}
      onDragLeave={() => setSobre(false)}
      onDrop={e => {
        e.preventDefault()
        setSobre(false)
        setEscolhidos(Array.from(e.dataTransfer.files))
      }}
    >
      <p>insira e gerencie os arquivos direto pelo site</p>
      <div className="linha">
        <button type="button" className="orc-bt" onClick={() => campo.current?.click()}>
          Escolher arquivos
        </button>
        <input
          ref={campo}
          type="file"
          multiple
          accept=".pdf,.xml,.jpg,.jpeg,.png"
          style={{ display: 'none' }}
          onChange={e => setEscolhidos(Array.from(e.target.files ?? []))}
        />
        <div className="caixa">
          {escolhidos.length === 0
            ? 'nenhum arquivo escolhido — ou arraste até aqui'
            : escolhidos.length === 1
              ? escolhidos[0].name
              : `${escolhidos[0].name} e mais ${escolhidos.length - 1}`}
        </div>
        <button
          type="button"
          className="orc-bt forte"
          disabled={!escolhidos.length || enviando}
          onClick={() => void inserir()}
        >
          {enviando ? 'Inserindo…' : 'Inserir'}
        </button>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// a ficha da nota, para amarrar tickets
// ---------------------------------------------------------------------------

/**
 * A nota em tela cheia com a barra de tickets em cima.
 *
 * A BARRA CRESCE, A NOTA NÃO ENCOLHE
 *   O dono desenhou uma barra de 1 cm que só cresce quando passa de dez tickets.
 *   É o comportamento certo: quem está lendo a nota para achar o número não pode
 *   ver a nota diminuir a cada ticket que insere.
 */
function FichaDaNota({ documento, fechar }: { documento: Documento; fechar: () => void }) {
  const [tickets, setTickets] = useState<TicketDoDocumento[]>([])
  const [novos, setNovos] = useState<number[]>([])
  const [digitando, setDigitando] = useState('')
  const [url, setUrl] = useState('')
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)

  useEffect(() => {
    void (async () => {
      try {
        const [ficha, arq] = await Promise.all([
          motor<{ tickets: TicketDoDocumento[] }>(`/orcamentos/documentos/${documento.id}`),
          motor<{ url: string }>(`/orcamentos/documentos/${documento.id}/arquivo`),
        ])
        setTickets(ficha.tickets)
        setUrl(arq.url)
      } catch (e) {
        setErro(e instanceof Error ? e.message : 'Não consegui abrir a nota.')
      }
    })()
  }, [documento.id])

  function juntar() {
    const n = Number(digitando.replace(/\D/g, ''))
    // Cinco ou seis dígitos: é o tamanho de ticket do São Luiz. Fora disso é
    // número de nota, DAV ou engano — e aceitar viraria orçamento no chamado
    // errado.
    if (!(n >= 10000 && n <= 999999)) {
      setErro('O ticket tem 5 ou 6 dígitos.')
      return
    }
    if (novos.includes(n) || tickets.some(t => t.ticket === n)) {
      setErro('Este ticket já está na lista.')
      setDigitando('')
      return
    }
    setNovos([...novos, n])
    setDigitando('')
    setErro('')
  }

  async function inserir() {
    if (!novos.length || salvando) return
    setSalvando(true)
    try {
      const r = await motor<{ tickets: TicketDoDocumento[] }>(
        `/orcamentos/documentos/${documento.id}/tickets`,
        { metodo: 'POST', corpo: { tickets: novos } })
      setTickets(r.tickets)
      setNovos([])
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui amarrar os tickets.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <div className="orc-ficha">
      <div className="orc-ficha-barra">
        <button type="button" className="orc-voltar" onClick={fechar}>← voltar</button>

        <div className="orc-tks">
          {tickets.map(t => (
            <span key={t.ticket} className={'orc-tk' + (t.chamado_id ? '' : ' solto')}
              title={t.chamado_id ? 'casou com um chamado da nossa base' : 'este número não existe na nossa base'}>
              {t.ticket}
            </span>
          ))}
          {novos.map(n => (
            <span key={n} className="orc-tk novo" onClick={() => setNovos(novos.filter(x => x !== n))}
              title="clique para tirar">
              {n} ×
            </span>
          ))}
        </div>

        <input
          className="orc-tk-campo"
          inputMode="numeric"
          placeholder="ticket"
          value={digitando}
          maxLength={6}
          onChange={e => setDigitando(e.target.value.replace(/\D/g, ''))}
          onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); juntar() } }}
        />
        <button type="button" className="orc-bt" onClick={juntar}>+</button>
        <button type="button" className="orc-bt forte" disabled={!novos.length || salvando} onClick={() => void inserir()}>
          {salvando ? 'Inserindo…' : 'Inserir'}
        </button>
      </div>

      {erro && <p className="erro" style={{ margin: '8px 0' }}>{erro}</p>}

      <div className="orc-ficha-nota">
        {url
          ? <iframe title={documento.nome_arquivo} src={url} />
          : <Carregando />}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// peças pequenas
// ---------------------------------------------------------------------------

export function BarraDeVolta({ voltar, titulo, direita }: {
  voltar: () => void
  titulo: string
  direita?: React.ReactNode
}) {
  return (
    <div className="orc-barra">
      <button type="button" className="orc-voltar" onClick={voltar}>← voltar</button>
      <span className="orc-barra-titulo">{titulo}</span>
      <span className="orc-barra-dir">{direita}</span>
    </div>
  )
}

/** Os tickets da linha, quebrando a cada quatro — o único lugar da tabela onde
 *  a quebra de linha é permitida, como o dono especificou. */
function Tickets({ numeros }: { numeros: number[] | null }) {
  if (!numeros?.length) return <span className="orc-detalhe">sem ticket</span>
  const grupos: number[][] = []
  for (let i = 0; i < numeros.length; i += 4) grupos.push(numeros.slice(i, i + 4))
  return (
    <span className="orc-tks-celula">
      {grupos.map((g, i) => <span key={i}>{g.join(' · ')}</span>)}
    </span>
  )
}

function Leitura({ d }: { d: Documento }) {
  if (d.status === 'inserido') return <span className="orc-selo espera">na fila</span>
  if (d.status === 'falhou') return <span className="orc-selo ruim">falhou</span>
  if (d.status === 'usado') return <span className="orc-selo ok">virou orçamento</span>
  const c = confiancaEmPalavras(d.leitura_camada, d.leitura_confianca)
  return <span className={'orc-selo ' + c.classe}>{c.texto}</span>
}

export function Paginacao({ pagina, por, aoTrocarPagina, aoTrocarPor }: {
  pagina: Pagina<unknown> | null
  por: number
  aoTrocarPagina: (n: number) => void
  aoTrocarPor: (n: number) => void
}) {
  if (!pagina) return null
  const p = pagina.pagina
  const inicio = pagina.total === 0 ? 0 : (p - 1) * pagina.por_pagina + 1
  const fim = Math.min(p * pagina.por_pagina, pagina.total)
  return (
    <div className="orc-rodape">
      <span>{inicio.toLocaleString('pt-BR')}–{fim.toLocaleString('pt-BR')} de {pagina.total.toLocaleString('pt-BR')}</span>
      <span className="orc-paginas">
        <button type="button" disabled={p <= 1} onClick={() => aoTrocarPagina(1)}>primeira</button>
        <button type="button" disabled={p <= 1} onClick={() => aoTrocarPagina(p - 1)}>anterior</button>
        <b>{p}/{pagina.paginas}</b>
        <button type="button" disabled={p >= pagina.paginas} onClick={() => aoTrocarPagina(p + 1)}>próxima</button>
        <button type="button" disabled={p >= pagina.paginas} onClick={() => aoTrocarPagina(pagina.paginas)}>última</button>
        <select value={por} onChange={e => aoTrocarPor(Number(e.target.value))}>
          {[100, 250, 500].map(n => <option key={n} value={n}>{n} por página</option>)}
        </select>
      </span>
    </div>
  )
}
