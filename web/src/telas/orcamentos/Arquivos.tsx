// rev 1 — Notas e DAVs (2.1) e Notas para rateio (2.2)
//
// É UMA TELA SÓ, COM UMA CHAVE
//
//	As duas fazem exatamente a mesma coisa com o arquivo: inserir, listar, abrir,
//	tirar da fila e desfazer. A diferença é o que vem depois — na de rateio o
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
import { motor, enviarArquivos } from '../../motor/cliente'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { VisorDaNota } from './VisorDaNota'
import { ConferirValor } from './ConferirValor'
import { Carregando } from '../../componentes/Carregando'
import {
  emDataHora, emReais, confiancaEmPalavras, oQueConferir,
  type Documento, type Pagina, type ResultadoDaInsercao, type ResultadoDaGeracao,
  type TicketDoDocumento,
} from './tipos'

interface Props {
  fila: 'orcamento' | 'rateio'
  voltar: () => void
}

type Vista = 'fila' | 'processadas' | 'fora'

const tituloDaVista: Record<Vista, string> = {
  fila:        'Arquivos na fila',
  processadas: 'Já viraram orçamento',
  fora:        'Fora da fila',
}

/** O que o motor diz que falta ler nesta fila. */
interface PorLer {
  notas: { id: string; nome_arquivo: string; status: string }[]
  total: number
  teto: number
}

/** O andamento da leitura, para a barra não deixar a pessoa no escuro. */
interface LoteDeLeitura {
  total: number
  feito: number
  lidas: number
  falhas: number
  rodando: boolean
  agora?: string
  /** Os motivos, sem repetir — três notas com a mesma queixa são uma linha. */
  motivos: string[]
}

const vaziaDaVista: Record<Vista, string> = {
  fila:        'Nada na fila. Tudo o que entrou já foi resolvido.',
  processadas: 'Nenhuma nota virou orçamento ainda.',
  fora:        'Nada foi tirado da fila por aqui.',
}

export function Arquivos({ fila, voltar }: Props) {
  const [pagina, setPagina] = useState<Pagina<Documento> | null>(null)
  const [numero, setNumero] = useState(1)
  const [por, setPor] = useState(100)
  const [busca, setBusca] = useState('')
  // TRÊS VISTAS, NÃO UM INTERRUPTOR
  //
  //	Esta tela é uma FILA: o que aparece por padrão é o que ainda dá trabalho.
  //	A nota que já virou orçamento está resolvida e sai daqui — o caminho para
  //	ela passou a ser o orçamento, que guarda o número dela. Sem isso a lista
  //	cresceria para sempre, e em dois anos seriam milhares de linhas verdes
  //	escondendo as dez que precisam de gente.
  //
  //	Sair da vista não é sumir: as resolvidas e as tiradas da fila continuam a
  //	um clique.
  const [vista, setVista] = useState<Vista>('fila')
  const [erro, setErro] = useState('')
  const [ocupado, setOcupado] = useState(false)
  // O documento que está aberto na tela. Ver documento é na tela, sempre.
  const [vendo, setVendo] = useState<{ endereco: string; nome: string } | null>(null)
  // A nota que ficou na fila esperando conferência, aberta em tela cheia.
  const [conferindo, setConferindo] = useState<Documento | null>(null)

  // A nota aberta em tela cheia para receber tickets — o "+" da fila de rateio.
  const [abrindo, setAbrindo] = useState<Documento | null>(null)

  // O aviso de exclusão. Fica até o usuário responder: sumir sozinho
  // transformaria "desfazer" numa corrida contra o relógio.
  const [desfazer, setDesfazer] = useState<{ id: string; nome: string } | null>(null)

  // A GERAÇÃO ACONTECE AQUI, ONDE O TRABALHO TERMINA
  //
  //	Amarrar os tickets e gerar são o mesmo gesto para quem opera: a pessoa
  //	acabou de dizer quais tickets a nota atende e quer ver o orçamento. Mandá-la
  //	a outra tela para apertar um botão é fazer o programa aparecer no meio do
  //	trabalho. Lançar continua noutra tela — porque entre gerar e lançar existe
  //	uma conferência, e essa separação é de propósito.
  const [gerando, setGerando] = useState(false)
  const [resultados, setResultados] = useState<ResultadoDaGeracao[] | null>(null)

  // A LEITURA AGORA É UM BOTÃO, E NÃO UMA ESPERA
  //
  //	Até 27/08/2026 quem lia era só o robô do GitHub, de meia em meia hora, nas
  //	horas em que ele roda. Quem acabou de inserir a nota ficava olhando uma
  //	fila cheia de "na fila" sem nada para fazer — e a nota só vira orçamento
  //	depois de lida.
  //
  //	`porLer` é o que o motor diz que falta ler NA FILA INTEIRA, não na página
  //	à vista: quem tem noventa e uma notas e vê cem por página precisa saber
  //	que o botão vai além do que está na tela.
  const [porLer, setPorLer] = useState<PorLer | null>(null)
  const [lote, setLote] = useState<LoteDeLeitura | null>(null)
  const pararLeitura = useRef(false)


  const carregar = useCallback(async () => {
    setOcupado(true)
    try {
      const q = new URLSearchParams({
        fila, pagina: String(numero), por: String(por),
        ...(busca ? { busca } : {}),
        ...(vista !== 'fila' ? { vista } : {}),
      })
      setPagina(await motor<Pagina<Documento>>('/orcamentos/documentos?' + q))
      setErro('')
      // Quantas faltam ler é pergunta da FILA, não desta página — e por isso é
      // outra chamada. Falhar aqui não derruba a lista: some o botão, que é
      // ajuda, e a lista, que é o serviço, continua.
      try {
        setPorLer(await motor<PorLer>('/orcamentos/documentos/porler?fila=' + fila))
      } catch { setPorLer(null) }
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar a lista.')
    } finally {
      setOcupado(false)
    }
  }, [fila, numero, por, busca, vista])

  useEffect(() => { void carregar() }, [carregar])

  async function tirarDaFila(d: Documento) {
    try {
      await motor(`/orcamentos/documentos/${d.id}`, { metodo: 'DELETE' })
      setDesfazer({ id: d.id, nome: d.nome_arquivo })
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui tirar da fila.')
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

  // LER TODAS — UMA NOTA POR CHAMADA, DE PROPÓSITO
  //
  //	Uma chamada só que lesse noventa notas ficaria vinte minutos aberta e
  //	morreria no tempo limite do servidor, jogando fora o trabalho já feito.
  //	Uma por vez: cada nota que termina está gravada, e parar no meio deixa
  //	lidas as que já foram.
  //
  //	O "parar depois desta" existe pela mesma razão do lote de reconferência:
  //	quem começou tem que poder desistir sem fechar a aba no meio de uma
  //	gravação.
  async function lerTodas() {
    const notas = porLer?.notas ?? []
    if (notas.length === 0) return
    pararLeitura.current = false
    setErro('')
    setLote({ total: notas.length, feito: 0, lidas: 0, falhas: 0, rodando: true, motivos: [] })

    for (const n of notas) {
      if (pararLeitura.current) break
      setLote(l => (l ? { ...l, agora: n.nome_arquivo } : l))
      try {
        // A FILA VAI NO ENDEREÇO, E NÃO É DETALHE
        //
        //	É por ela que o motor escolhe a permissão a exigir — "Notas e DAVs"
        //	ou "Notas para rateio" — e depois confere se a nota é mesmo daquela
        //	fila, para ninguém entrar por uma porta com a chave da outra.
        //
        //	Sem este parâmetro o motor assume `orcamento`, e toda leitura na fila
        //	de rateio morria em "Esta nota não é desta fila." Foi o que aconteceu
        //	na primeira subida, 27/08/2026: 0 lidas, 3 recusadas.
        await motor(`/orcamentos/documentos/${n.id}/ler?fila=${fila}`, { metodo: 'POST' })
        setLote(l => l && ({ ...l, feito: l.feito + 1, lidas: l.lidas + 1 }))
      } catch (e) {
        // Uma nota que não leu não derruba as outras: ela fica marcada como
        // "falhou", com o motivo, e o próximo clique tenta de novo.
        const motivo = e instanceof Error ? e.message : 'não consegui ler'
        setLote(l => l && ({
          ...l,
          feito: l.feito + 1,
          falhas: l.falhas + 1,
          motivos: l.motivos.includes(motivo) ? l.motivos : [...l.motivos, motivo],
        }))
      }
    }

    setLote(l => (l ? { ...l, rodando: false, agora: undefined } : l))
    await carregar()
  }

  async function gerar() {
    setGerando(true)
    setResultados(null)
    try {
      const r = await motor<{ resultados: ResultadoDaGeracao[] }>('/orcamentos/gerar',
        { metodo: 'POST', corpo: { fila } })
      setResultados(r.resultados)
      await carregar()
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui gerar.')
    } finally {
      setGerando(false)
    }
  }

  // VER É NA TELA — REGRA DA CASA
  //   Abrir noutra aba tira a pessoa do sistema e deixa o documento à deriva num
  //   lugar sem voltar, sem contexto e sem as ações daquela nota. Ver documento
  //   é sempre aqui dentro. Ver `claude/padroes-de-tela.md`.
  async function abrirArquivo(d: Documento) {
    try {
      const r = await motor<{ url: string }>(`/orcamentos/documentos/${d.id}/arquivo`)
      setVendo({ endereco: r.url, nome: d.nome_arquivo })
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  // A NOTA DE CONFERÊNCIA SE RESOLVE AQUI MESMO
  //   Ela não vai para Correções: o problema dela não é ticket. Abre em tela
  //   cheia, com o motivo escrito acima do papel e todos os reparos à mão —
  //   inserir ticket, trocar, mandar para rateio, tirar da fila.
  if (conferindo) {
    const reparos = {
      excluir: async () => {
        await motor(`/orcamentos/documentos/${conferindo.id}`, { metodo: 'DELETE' })
        setConferindo(null); await carregar()
      },
      rateada: async () => {
        await motor(`/orcamentos/documentos/${conferindo.id}/fila`,
          { metodo: 'POST', corpo: { fila: 'rateio' } })
        setConferindo(null); await carregar()
      },
      concluir: async () => { setConferindo(null); await carregar() },
      fechar: () => setConferindo(null),
    }

    // A PERGUNTA DO VALOR TEM TELA PRÓPRIA
    //   Ela não é um aviso a ler, é uma resposta a dar — e a resposta está no
    //   papel, que fica logo abaixo.
    if (conferindo.motivo_conferencia === 'confirme o valor desta nota') {
      return (
        <ConferirValor
          documento={conferindo}
          acoes={reparos}
          aoResponder={async () => { setConferindo(null); await carregar() }}
        />
      )
    }

    return (
      <VisorDaNota
        documento={conferindo.id}
        nome={conferindo.nome_arquivo}
        valor={conferindo.valor_total}
        tickets={conferindo.ticket_numeros ?? []}
        modo="conferencia"
        motivo={conferindo.motivo_conferencia}
        acoes={reparos}
      />
    )
  }

  if (vendo) {
    return (
      <VisorDeDocumento
        endereco={vendo.endereco}
        nomeSugerido={vendo.nome}
        titulo={vendo.nome}
        voltar={() => setVendo(null)}
      />
    )
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
      <BarraDeVolta
        voltar={voltar}
        titulo={fila === 'rateio' ? 'Notas para rateio' : 'Notas e DAVs'}
        direita={
          <>
            <span className="orc-prontas">{contarProntas(pagina)}</span>
            {/* LER VEM ANTES DE GERAR, NA TELA COMO NA VIDA
                A nota só é liberada para gerar depois de lida. Pôr os dois
                botões nessa ordem é dizer isso sem precisar de aviso. */}
            {lote?.rodando ? (
              <button type="button" className="orc-bt"
                onClick={() => { pararLeitura.current = true }}>
                parar depois desta
              </button>
            ) : (
              porLer && porLer.notas.length > 0 && (
                <button
                  type="button"
                  className="orc-bt forte"
                  disabled={gerando}
                  title="Lê as notas que ainda estão na fila. Só depois de lida a nota pode virar orçamento."
                  onClick={() => void lerTodas()}
                >
                  Ler notas ({porLer.notas.length})
                </button>
              )
            )}
            <button
              type="button"
              className="orc-bt forte"
              disabled={gerando || lote?.rodando || !temProntas(pagina)}
              onClick={() => void gerar()}
            >
              {gerando ? 'Gerando…' : 'Gerar orçamentos'}
            </button>
          </>
        }
      />

      {lote && <PainelDaLeitura dados={lote} fechar={() => setLote(null)} />}

      {resultados && <ResumoDaGeracao resultados={resultados} fechar={() => setResultados(null)} />}

      <Insercao fila={fila} aoTerminar={carregar} aoFalhar={setErro} />

      <div className="orc-lista">
        <div className="orc-lista-cab">
          <h2>{tituloDaVista[vista]}</h2>
          <em>{pagina ? `${pagina.total.toLocaleString('pt-BR')} no total` : '—'}</em>

          <input
            className="orc-busca"
            placeholder="Procurar por nome, número ou fornecedor"
            value={busca}
            onChange={e => { setNumero(1); setBusca(e.target.value) }}
          />
          <select
            className={'orc-bt-fino' + (vista !== 'fila' ? ' ligado' : '')}
            value={vista}
            onChange={e => { setNumero(1); setVista(e.target.value as Vista) }}
          >
            <option value="fila">na fila</option>
            <option value="processadas">já viraram orçamento</option>
            <option value="fora">fora da fila</option>
          </select>
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
                    {vaziaDaVista[vista]}
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
                      {/* MOTIVO ESCONDIDO NO `title` É MOTIVO QUE NINGUÉM LÊ
                          Quem chega nesta tela para tratar precisa da frase à
                          vista — ela diz o que travou e o que fazer. Motivo sem
                          saída visível é o que faz o usuário desistir do sistema
                          e abrir uma planilha.

                          Vale para TODOS os impedimentos, e não só o bloqueio:
                          a leitura que falhou, a nota repetida, a aprovação
                          pedida. A frase vem da visão (`motivo_conferencia`),
                          então a lista e a tela cheia dizem a mesma coisa. */}
                      {d.motivo_conferencia && (
                        <span className="orc-detalhe ruim">{d.motivo_conferencia}</span>
                      )}
                      {d.duplicada_de && (
                        <span className="orc-detalhe ruim">
                          já entrou como “{d.duplicada_de_nome ?? 'outra nota'}”
                          {d.duplicada_de_em ? ` em ${emDataHora(d.duplicada_de_em)}` : ''}
                        </span>
                      )}
                    </td>
                    <td>{emDataHora(d.inserido_em)}</td>
                    {fila === 'rateio' && (
                      <td><Tickets numeros={d.ticket_numeros} soltos={d.ticket_soltos} /></td>
                    )}
                    <td><Leitura d={d} /></td>
                    <td className="orc-acoes">
                      {/* Ver é ver; conferir é resolver. Quando a nota tem um
                          motivo, o botão principal leva para o reparo. */}
                      {(d.motivo_conferencia || (fila === 'orcamento' && precisaDeGente(d)))
                        ? <button type="button" className="forte" onClick={() => setConferindo(d)}>conferir</button>
                        : <button type="button" onClick={() => void abrirArquivo(d)}>ver</button>}
                      {fila === 'rateio' && vista === 'fila' && (
                        <button type="button" className="orc-mais" onClick={() => setAbrindo(d)} title="amarrar tickets">+</button>
                      )}
                      {/* Tirar da fila NÃO apaga: preenche `oculto_em`, e o arquivo
                          continua inteiro no armazém. O nome antigo, "excluir",
                          prometia uma destruição que nunca aconteceu — e assustava
                          quem só queria limpar a lista. */}
                      {vista === 'fora'
                        ? <button type="button" className="mut" onClick={() => void restaurar(d.id)}>devolver à fila</button>
                        : <button type="button" className="mut" onClick={() => void tirarDaFila(d)}>tirar da fila</button>}
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
            <b>{desfazer.nome} saiu da fila</b>
            <small>Nada é apagado do armazém. Dá para devolver à fila quando quiser.</small>
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
export function Insercao({ fila, aoTerminar, aoFalhar }: {
  fila: string
  aoTerminar: () => Promise<void>
  aoFalhar: (s: string) => void
}) {
  const [escolhidos, setEscolhidos] = useState<File[]>([])
  const [enviando, setEnviando] = useState(false)
  const [sobre, setSobre] = useState(false)
  // Os arquivos que já estavam aqui. Fica até a pessoa responder: sumir sozinho
  // é como a recusa passou despercebida da primeira vez.
  const [repetidos, setRepetidos] = useState<{ nome: string; como: string }[]>([])
  const campo = useRef<HTMLInputElement>(null)

  async function inserir() {
    if (!escolhidos.length || enviando) return
    setEnviando(true)
    try {
      const r = await enviarArquivos<{ arquivos: ResultadoDaInsercao[] }>(
        '/orcamentos/documentos?fila=' + fila, escolhidos)
      const ruins = r.arquivos.filter(a => a.erro)
      const repetidos = r.arquivos.filter(a => a.ja_existia)
      if (ruins.length) {
        aoFalhar(ruins.map(a => `${a.nome}: ${a.erro}`).join(' · '))
      } else {
        aoFalhar('')
      }
      // ARQUIVO REPETIDO NÃO É ERRO, E DIZER QUANTOS NÃO É DIZER QUAIS
      //
      //	A tela escrevia "1 já estava na lista", em vermelho, e parava aí. Com
      //	dez arquivos de nome UUID isso não identifica nada: em 27/08/2026 o
      //	usuário mandou 10, contou 9 na fila e foi procurar defeito no sistema.
      //	O sistema estava certo — os dois arquivos eram byte a byte iguais — e
      //	calado.
      //
      //	Agora diz os DOIS nomes. A mesma recusa vira resposta.
      setRepetidos(repetidos.map(a => ({
        nome: a.nome,
        como: a.ja_existia_como ?? '',
      })))
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
      {repetidos.length > 0 && (
        <div className="orc-repetidos">
          <p>
            <b>{repetidos.length}</b>{' '}
            {repetidos.length === 1 ? 'arquivo não entrou' : 'arquivos não entraram'}
            {' '}porque {repetidos.length === 1 ? 'já estava' : 'já estavam'} aqui —
            {' '}mesmo arquivo, byte a byte. Nenhuma nota foi perdida.
          </p>
          <ul>
            {repetidos.map(x => (
              <li key={x.nome}>
                <b>{x.nome}</b>
                {x.como && x.como !== x.nome && <> é o mesmo que <b>{x.como}</b></>}
              </li>
            ))}
          </ul>
          <button type="button" className="orc-bt" onClick={() => setRepetidos([])}>Entendi</button>
        </div>
      )}

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

  // TIRAR UM TICKET JÁ GRAVADO
  //
  //	Até 28/08/2026 esta ficha só deixava tirar o que ainda NÃO tinha sido
  //	salvo. Ticket gravado errado só saía pelo visor de Correções — e a nota
  //	que já estava certa em tudo o mais não passa por lá. Quem errasse o
  //	número aqui não tinha caminho de volta nesta tela.
  //
  // PEDE CONFIRMAÇÃO PORQUE MEXE NO DESTINO DO DINHEIRO
  //
  //	O ticket é o chamado que vai receber o custo. Tirar o errado conserta;
  //	tirar o certo manda o orçamento para outro chamado, e ninguém percebe
  //	até o cliente perguntar.
  async function tirar(t: number) {
    if (salvando) return
    if (!window.confirm(`Tirar o ticket ${t} desta nota?`)) return
    setSalvando(true)
    try {
      await motor(`/orcamentos/documentos/${documento.id}/tickets/${t}/apagar`,
        { metodo: 'POST' })
      setTickets(tickets.filter(x => x.ticket !== t))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui tirar o ticket.')
    } finally {
      setSalvando(false)
    }
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
              <s role="button" tabIndex={0} title="tirar este ticket da nota"
                onClick={() => void tirar(t.ticket)}
                onKeyDown={e => { if (e.key === 'Enter') void tirar(t.ticket) }}>×</s>
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
 *  a quebra de linha é permitida, como o dono especificou.
 *
 *  O que não casou com a nossa base sai em âmbar. É o ticket que TRAVA a nota:
 *  enquanto ele existir, gerar produziria valor errado nas outras partes. Marcar
 *  na lista é o que permite consertar sem ter que abrir nota por nota. */
function Tickets({ numeros, soltos }: { numeros: number[] | null; soltos?: number[] | null }) {
  if (!numeros?.length) return <span className="orc-detalhe">sem ticket</span>
  const solto = new Set(soltos ?? [])
  const grupos: number[][] = []
  for (let i = 0; i < numeros.length; i += 4) grupos.push(numeros.slice(i, i + 4))
  return (
    <span className="orc-tks-celula">
      {grupos.map((g, i) => (
        <span key={i}>
          {g.map((n, j) => (
            <span key={n}>
              {j > 0 && ' · '}
              <span className={solto.has(n) ? 'orc-tk-solto' : undefined}
                title={solto.has(n) ? 'este número não existe na nossa base — a nota não gera enquanto ele estiver assim' : undefined}>
                {n}
              </span>
            </span>
          ))}
        </span>
      ))}
      {solto.size > 0 && <span className="orc-travada">travada</span>}
    </span>
  )
}

/** Quantas notas desta página estão prontas — o número que explica o botão. */
// O ANDAMENTO DA LEITURA, E O QUE SOBROU DELA
//
//	A leitura de uma nota leva segundos — noventa levam minutos. Sem barra, a
//	pessoa não sabe se o sistema está trabalhando ou travado, e a saída natural
//	dela é fechar a aba no meio de uma gravação.
//
//	No fim, os motivos das falhas vêm SEM REPETIR: trinta notas paradas pela
//	mesma cota estourada são um problema, não trinta.
function PainelDaLeitura({ dados, fechar }: { dados: LoteDeLeitura; fechar: () => void }) {
  const pct = dados.total === 0 ? 0 : Math.round((dados.feito / dados.total) * 100)
  return (
    <div className="orc-lote">
      <div className="orc-lote-topo">
        <span>
          {dados.rodando
            ? <>lendo{dados.agora ? ` — ${dados.agora}` : ''}…</>
            : <>leitura terminada</>}
        </span>
        <strong>{dados.feito} de {dados.total}</strong>
      </div>
      <div className="orc-lote-barra"><span style={{ width: `${pct}%` }} /></div>
      {!dados.rodando && (
        <div className="orc-lote-fim">
          <p>
            <strong>{dados.lidas}</strong>{' '}
            {dados.lidas === 1 ? 'nota lida' : 'notas lidas'}
            {dados.falhas > 0 && <> · <strong>{dados.falhas}</strong> não deu para ler</>}.
            {dados.lidas > 0 && ' Elas já podem virar orçamento.'}
          </p>
          {dados.motivos.length > 0 && (
            <ul className="orc-lote-motivos">
              {dados.motivos.map(m => <li key={m}>{m}</li>)}
            </ul>
          )}
          <button type="button" className="orc-bt" onClick={fechar}>Entendi</button>
        </div>
      )}
    </div>
  )
}

// PRECISA DE GENTE — a nota que foi lida e mesmo assim não gera
//
//	Sem ticket, ticket que a base não conhece, valor a confirmar, repetida,
//	bloqueada. Todas continuam na fila (é a regra: nada sai daqui antes de
//	gerar), e todas precisam de um botão que RESOLVA, não só de um que mostre.
//
//	Uma linha que fica na lista sem nada para fazer nela é pior que a linha
//	sumida: some da vista do mesmo jeito, e ainda ocupa espaço.
/** Há pelo menos uma nota que o botão de gerar consegue trabalhar. */
function temProntas(pagina: Pagina<Documento> | null): boolean {
  return !!pagina?.linhas.some(d => d.pronto_para_gerar)
}

function precisaDeGente(d: Documento): boolean {
  if (d.status === 'falhou') return true
  if (d.status !== 'lido') return false
  return !d.pronto_para_gerar
}

// O CABEÇALHO TEM QUE EXPLICAR A DIFERENÇA ENTRE A LISTA E O BOTÃO
//
//	A fila mostra tudo o que entrou e não virou orçamento — 12 linhas, digamos.
//	O botão gera 8. Sem uma palavra sobre as outras 4, a pessoa faz a conta na
//	cabeça e conclui que o sistema perdeu alguma coisa. Foi assim que "9 viraram
//	8" virou caça a um defeito, em 27/08/2026.
function contarProntas(pagina: Pagina<Documento> | null): string {
  if (!pagina) return ''
  const linhas = pagina.linhas
  const prontas = linhas.filter(d => d.pronto_para_gerar).length
  const porLer = linhas.filter(d => d.status === 'inserido' || d.status === 'lendo').length
  const precisam = linhas.filter(d => precisaDeGente(d)).length
  if (prontas === 0 && porLer === 0 && precisam === 0) return ''
  const partes: string[] = []
  if (prontas) partes.push(`${prontas} pronta${prontas > 1 ? 's' : ''}`)
  if (porLer) partes.push(`${porLer} por ler`)
  if (precisam) partes.push(`${precisam} precisa${precisam > 1 ? 'm' : ''} de você`)
  return partes.join(' · ')
}

function Leitura({ d }: { d: Documento }) {
  if (d.status === 'inserido') return <span className="orc-selo espera">na fila</span>
  if (d.status === 'falhou') return <span className="orc-selo ruim">falhou</span>
  if (d.status === 'usado') return <span className="orc-selo ok">virou orçamento</span>

  // O IMPEDIMENTO VEM ANTES DA QUALIDADE DA LEITURA
  //
  //	Uma nota repetida pode estar lida com 100% de confiança — e mostrar
  //	"leitura boa" numa nota que não vai gerar orçamento nenhum é responder a
  //	pergunta errada. Quem olha esta coluna quer saber se pode seguir, não se
  //	o OCR foi feliz.
  if (d.duplicada_de) {
    return (
      <span className="orc-selo ruim" title={d.duplicada_de_nome
        ? `é a mesma nota que "${d.duplicada_de_nome}"`
        : 'já existe uma nota igual a esta'}>repetida</span>
    )
  }
  if (d.bloqueio_motivo) {
    return <span className="orc-selo ruim" title={d.bloqueio_motivo}>bloqueada</span>
  }

  // E O TICKET É IMPEDIMENTO TAMBÉM
  //
  //	"Leitura boa" numa nota que está parada há uma semana esperando ticket
  //	responde a pergunta errada — a leitura foi ótima mesmo, e a nota continua
  //	sem poder virar orçamento. Quem olha esta coluna numa FILA quer saber por
  //	que aquela linha ainda está ali.
  if (d.ticket_soltos?.length) {
    return (
      <span className="orc-selo aviso"
        title={`o ticket ${d.ticket_soltos.join(', ')} não existe na nossa base de chamados`}>
        ticket não achado
      </span>
    )
  }
  if (!d.tickets) {
    return <span className="orc-selo aviso" title="nenhum ticket foi encontrado na nota">sem ticket</span>
  }
  if (d.aprovacao_pedida) {
    return <span className="orc-selo espera" title="esperando o cliente aceitar o valor">aprovação pedida</span>
  }

  const c = confiancaEmPalavras(d.leitura_camada, d.leitura_confianca)
  if (c.classe === 'ok') {
    return <span className={'orc-selo ' + c.classe}>{c.texto}</span>
  }
  // "CONFIRA" SOZINHO NÃO É INSTRUÇÃO
  //   Quem lê precisa saber o que abrir a nota para olhar. Sem isso, ou se relê
  //   a nota inteira, ou — mais provável — o aviso é ignorado por nunca ajudar.
  return (
    <span className={'orc-selo ' + c.classe} title={`confiança de ${Math.round((d.leitura_confianca ?? 0) * 100)}%`}>
      {c.texto}: {oQueConferir(d)}
    </span>
  )
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

export function ResumoDaGeracao({ resultados, fechar }: { resultados: ResultadoDaGeracao[]; fechar: () => void }) {
  const bons = resultados.filter(r => r.orcamento).length
  const ruins = resultados.filter(r => r.erro)
  return (
    <div className="orc-resumo">
      <div className="cab">
        <b>{bons} orçamento{bons === 1 ? '' : 's'} gerado{bons === 1 ? '' : 's'}</b>
        {ruins.length > 0 && <span> · {ruins.length} não deu</span>}
        <button type="button" onClick={fechar}>fechar</button>
      </div>
      {resultados.some(r => r.aviso) && (
        <ul className="avisos">
          {resultados.filter(r => r.aviso).map((r, i) => (
            <li key={i}><b>ticket {r.ticket}</b> — {r.aviso}</li>
          ))}
        </ul>
      )}
      {ruins.length > 0 && (
        <ul className="ruins">
          {ruins.map((r, i) => <li key={i}><b>{r.nome}</b> — {r.erro}</li>)}
        </ul>
      )}
    </div>
  )
}
