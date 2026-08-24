// rev 4 — Dados do Trílogo: a lista, a ficha e a extração
//
// TUDO GIRA EM TORNO DO TICKET
//   É por isso que a busca por número tem borda escura e vem primeiro, antes dos
//   seletores. Quem chega nesta tela quase sempre chega com um número na mão —
//   veio de uma conversa, de uma nota ou de um papel. Os filtros servem para
//   quando NÃO se sabe o número; a busca serve para quando se sabe (P-29).
//
// A PÁGINA É GRANDE DE PROPÓSITO
//   100 linhas por padrão, com 250 e 500 à escolha. A leitura de 500 é tão barata
//   quanto a de 100 porque o banco lê em ordem e para na última linha da página —
//   está medido na migration 009: 3,7 ms para as 500 primeiras de 1.377.
//
// O QUE ESTA TELA NÃO FAZ
//   Não altera nada. É uma janela para o que o robô trouxe. Editar chamado é no
//   Trílogo, e continua sendo.
//
// A LISTA NÃO MORRE QUANDO A FICHA ABRE
//   Ela fica montada, escondida. Se fosse desmontada, voltar da ficha jogaria a
//   pessoa na página 1 sem filtro nenhum — depois de ela ter chegado à página 7
//   filtrando por loja e por data. Ninguém merece refazer isso a cada chamado
//   que abre (P-29).
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { motor, baixarDoMotor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { ajustarCelulas } from './encolher'
import { FichaChamado } from './FichaChamado'
import {
  SEM_FILTRO, classeDaPrioridade, classeDoStatus, contaPorExtenso, emReais, quando,
  type Escolhas, type Filtros, type Pagina, type Rodada,
} from './tipos'

/** Quantos lotes o botão aceita encadear antes de desistir. Trava de segurança:
 *  uma resposta que nunca diz "completo" não pode virar laço infinito. */
const LOTES_NO_MAXIMO = 60

interface Props {
  /** O ticket aberto, quando há um. Vem do endereço. */
  ticket?: string
  abrir: (numero: number) => void
  voltar: () => void
}

export function DadosTrilogo({ ticket, abrir, voltar }: Props) {
  const [filtros, setFiltros] = useState<Filtros | null>(null)
  const [escolhas, setEscolhas] = useState<Escolhas>(SEM_FILTRO)
  const [busca, setBusca] = useState('')
  const [pagina, setPagina] = useState(1)
  const [porPagina, setPorPagina] = useState(100)

  const [dados, setDados] = useState<Pagina | null>(null)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)

  const [ultima, setUltima] = useState<string | null>(null)
  const [andamento, setAndamento] = useState<string | null>(null)
  const [extraindo, setExtraindo] = useState<'pdf' | 'xlsx' | null>(null)

  const corpo = useRef<HTMLTableSectionElement>(null)
  const painel = useRef<HTMLDivElement>(null)

  // Digitar não dispara uma consulta por tecla: espera a pessoa parar (CORE-01).
  useEffect(() => {
    const t = setTimeout(() => {
      setEscolhas(e => (e.ticket === busca.trim() ? e : { ...e, ticket: busca.trim() }))
      setPagina(1)
    }, 350)
    return () => clearTimeout(t)
  }, [busca])

  // As opções dos seletores vêm do próprio dado, uma vez só.
  useEffect(() => {
    let vivo = true
    motor<Filtros>('/trilogo/filtros')
      .then(f => { if (vivo) setFiltros(f) })
      .catch(() => { /* a tela funciona sem os seletores; a busca por ticket não depende deles */ })
    return () => { vivo = false }
  }, [])

  const verUltimaLeitura = useCallback(() => {
    motor<{ rodadas: Rodada[] }>('/robos/trilogo/rodadas')
      .then(r => {
        // A cópia de arquivos NÃO é uma leitura do Trílogo: ela move bytes que já
        // tinham sido lidos. Contar a cópia aqui fazia o carimbo mostrar a hora
        // errada — foi o que apareceu na primeira vez que a tela subiu.
        const feita = r.rodadas.find(x =>
          x.situacao === 'concluida' && !!x.terminou_em && x.modo !== 'copia')
        setUltima(feita?.terminou_em ?? null)
      })
      .catch(() => { /* o carimbo é informação a mais; a falta dele não atrapalha */ })
  }, [])

  useEffect(() => { verUltimaLeitura() }, [verUltimaLeitura])

  /**
   * Os filtros viram endereço aqui, e só aqui.
   *
   * A lista e a extração usam ESTA função. Se cada uma montasse os seus, bastava
   * corrigir um lado e esquecer o outro para o arquivo sair com um conteúdo
   * diferente do que está na tela — e ninguém perceberia (CORE-06).
   */
  const paramsDoFiltro = useCallback(() => {
    const p = new URLSearchParams()
    for (const [chave, valor] of Object.entries(escolhas)) {
      if (valor) p.set(chave, valor)
    }
    return p
  }, [escolhas])

  const carregar = useCallback(async () => {
    setErro(null)
    const p = paramsDoFiltro()
    p.set('pagina', String(pagina))
    p.set('por_pagina', String(porPagina))
    try {
      setDados(await motor<Pagina>(`/trilogo/chamados?${p}`))
    } catch (e) {
      setDados(null)
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os chamados.')
    }
  }, [paramsDoFiltro, pagina, porPagina])

  useEffect(() => { void carregar() }, [carregar])

  // A página pedida pode não existir mais — o motor devolve a última que existe.
  // A tela acompanha, em vez de continuar mostrando um número que não é o que
  // está na frente da pessoa.
  useEffect(() => {
    if (dados && dados.pagina !== pagina) setPagina(dados.pagina)
  }, [dados, pagina])

  /**
   * A ÁREA DA TABELA TERMINA ONDE A JANELA TERMINA
   *
   * Antes, a barra de rolagem lateral ficava no fim da LISTA: com 100 linhas —
   * ou 500 — ela nascia a três telas de distância, e para ver a coluna da
   * direita era preciso descer até o fim, rolar de lado e subir de volta.
   *
   * Agora o painel tem a altura do que sobra da janela, e rola por dentro. A
   * barra lateral fica sempre na borda de baixo do campo de visão, e o cabeçalho
   * da tabela fica grudado no topo enquanto se rola.
   *
   * A altura é MEDIDA, não chutada: a faixa de filtros muda de altura conforme a
   * janela, e um número fixo estaria errado na maioria das telas.
   */
  useLayoutEffect(() => {
    function ajustar() {
      const el = painel.current
      if (!el) return
      // Tudo que fica ABAIXO do painel é medido, não estimado: a linha de
      // paginação e o respiro do rodapé da página. Com número fixo, a página
      // inteira ganhava uma rolagem de alguns pixels — e aí a barra lateral da
      // tabela deixava de estar exatamente na borda de baixo da janela, que é o
      // ponto todo desta conta.
      const topo = el.getBoundingClientRect().top
      const paginacao = el.parentElement?.querySelector('.tri-rodape') as HTMLElement | null
      const conteudo = el.closest('.content') as HTMLElement | null
      const abaixo = (paginacao ? paginacao.getBoundingClientRect().height + 12 : 52)
        + (conteudo ? parseFloat(getComputedStyle(conteudo).paddingBottom || '24') : 24)
      el.style.maxHeight = Math.max(240, window.innerHeight - topo - abaixo) + 'px'
    }
    ajustar()
    window.addEventListener('resize', ajustar)
    return () => window.removeEventListener('resize', ajustar)
  }, [dados, ticket, filtros])

  // O encolhimento roda depois do desenho, quando as colunas já têm largura, e de
  // novo quando a janela muda de tamanho.
  // `ticket` entra nas dependências de propósito: com a ficha aberta a lista fica
  // escondida, e escondida ela tem largura zero. Sem recalcular ao voltar, as
  // colunas voltariam com o tamanho de letra medido contra o nada.
  useLayoutEffect(() => { if (!ticket) ajustarCelulas(corpo.current) }, [dados, ticket])
  useEffect(() => {
    let t: number | undefined
    function aoRedimensionar() {
      window.clearTimeout(t)
      t = window.setTimeout(() => ajustarCelulas(corpo.current), 120)
    }
    window.addEventListener('resize', aoRedimensionar)
    return () => { window.clearTimeout(t); window.removeEventListener('resize', aoRedimensionar) }
  }, [])

  function mudar(campo: keyof Escolhas, valor: string) {
    setEscolhas(e => ({ ...e, [campo]: valor }))
    setPagina(1)
  }

  function limpar() {
    setEscolhas(SEM_FILTRO)
    setBusca('')
    setPagina(1)
  }

  const filtrando = Object.values(escolhas).some(Boolean)

  /**
   * O botão "Atualizar agora".
   *
   * Cada chamada faz UM lote e diz se sobrou trabalho. Não é capricho: o motor
   * dorme no plano gratuito, e uma tarefa de fundo morreria com ele sem ninguém
   * saber. Então quem encadeia os lotes é esta tela — e, enquanto encadeia, diz
   * em que ponto está, porque espera longa se explica (P-25).
   */
  async function atualizar() {
    setAndamento('Falando com o Trílogo...')
    setErro(null)
    setRecado(null)
    let lidos = 0
    try {
      for (let lote = 1; lote <= LOTES_NO_MAXIMO; lote++) {
        const r = await motor<Rodada>('/robos/trilogo/atualizacao', { metodo: 'POST' })
        lidos += r.chamados_lidos
        setAndamento(`Lote ${lote} · ${lidos} chamados lidos`)
        if (r.completo) break
      }
      setRecado(lidos === 0 ? 'Nada mudou no Trílogo desde a última leitura.' : `Leitura concluída: ${lidos} chamados conferidos.`)
      verUltimaLeitura()
      await carregar()
    } catch (e) {
      // 409 não é defeito: é "já tem gente fazendo isso". O motor já manda a
      // frase pronta, dizendo há quanto tempo.
      const msg = e instanceof ErroMotor ? e.message : 'Não consegui atualizar agora.'
      if (e instanceof ErroMotor && e.status === 409) setRecado(msg)
      else setErro(msg)
    } finally {
      setAndamento(null)
    }
  }

  // A posição do chamado aberto dentro da página, para o "‹ 18 de 100 ›".
  const naPagina = dados?.linhas.findIndex(l => String(l.numero) === ticket) ?? -1
  const vizinho = (passo: number) => {
    const alvo = dados?.linhas[naPagina + passo]
    return alvo ? () => abrir(alvo.numero) : undefined
  }

  /**
   * A extração obedece ao FILTRO, não à página.
   *
   * Quem filtrou 340 chamados e está vendo a página 1 quer os 340 no arquivo. Por
   * isso `pagina` e `por_pagina` não entram: só os filtros.
   */
  async function extrair(formato: 'pdf' | 'xlsx') {
    setExtraindo(formato)
    setErro(null)
    try {
      await baixarDoMotor(`/trilogo/chamados.${formato}?${paramsDoFiltro()}`)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui gerar o arquivo.')
    } finally {
      setExtraindo(null)
    }
  }

  return (
    <>
      {ticket && (
        <FichaChamado
          numero={ticket}
          voltar={voltar}
          anterior={naPagina > 0 ? vizinho(-1) : undefined}
          proximo={naPagina >= 0 && naPagina < (dados?.linhas.length ?? 0) - 1 ? vizinho(1) : undefined}
          posicao={naPagina >= 0
            ? `${(dados!.pagina - 1) * dados!.por_pagina + naPagina + 1} de ${dados!.total.toLocaleString('pt-BR')}`
            : undefined}
        />
      )}

      <div hidden={!!ticket}>
      {/* Sem faixa de título aqui: o cabeçalho da página já diz "Dados do
          Trílogo", e repetir logo abaixo só empurrava a tabela para baixo. */}
      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}

      <div className="tri-filtros">
        <label className="tri-campo tri-ticket">
          <span>Ticket</span>
          <input
            value={busca}
            onChange={e => setBusca(e.target.value)}
            placeholder="Buscar número do ticket..."
            inputMode="numeric"
          />
        </label>

        <label className="tri-campo">
          <span>Loja</span>
          <select value={escolhas.loja} onChange={e => mudar('loja', e.target.value)}>
            <option value="">Todas as lojas{filtros ? ` (${filtros.lojas.length})` : ''}</option>
            {filtros?.lojas.map(l => <option key={l.id} value={l.id}>{l.nome}</option>)}
          </select>
        </label>

        <label className="tri-campo">
          <span>Status</span>
          <select value={escolhas.status} onChange={e => mudar('status', e.target.value)}>
            <option value="">Todos</option>
            {filtros?.status.map(s => <option key={s} value={s}>{s}</option>)}
          </select>
        </label>

        <label className="tri-campo">
          <span>Conta</span>
          <select value={escolhas.conta} onChange={e => mudar('conta', e.target.value)}>
            <option value="">Todas</option>
            {filtros?.contas.map(c => <option key={c} value={c}>{contaPorExtenso(c)}</option>)}
          </select>
        </label>

        <label className="tri-campo">
          <span>Prioridade</span>
          <select value={escolhas.prioridade} onChange={e => mudar('prioridade', e.target.value)}>
            <option value="">Todas</option>
            {filtros?.prioridades.map(p => <option key={p} value={p}>{p}</option>)}
            {/* Chamado sem prioridade também é um caso de verdade, e precisa ser achável. */}
            <option value="sem">Sem prioridade</option>
          </select>
        </label>

        {/* As duas datas são UM filtro, e andam juntas. Soltas na faixa, a
            segunda caía sozinha para a linha de baixo e o "até" ficava órfão. */}
        <div className="tri-periodo">
          <label className="tri-campo">
            <span>Criado de</span>
            <input type="date" value={escolhas.de} onChange={e => mudar('de', e.target.value)} />
          </label>
          <span className="tri-ate">até</span>
          <label className="tri-campo">
            <span className="tri-oculto">Criado até</span>
            <input type="date" value={escolhas.ate} onChange={e => mudar('ate', e.target.value)} />
          </label>
        </div>

        {filtrando && (
          <button type="button" className="tri-limpar" onClick={limpar}>Limpar filtros</button>
        )}

        {/* Encostados à direita, na mesma linha das datas. */}
        <div className="tri-canto">
          {/* O menu abre no repouso do ponteiro — e também com o teclado, pelo
              foco, senão só existe para quem usa mouse. */}
          <div className="tri-extrair">
            <button className="bt bt-neutro" type="button" disabled={!!extraindo}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9"
                   strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M12 3v11" /><path d="m7.5 10 4.5 4.5 4.5-4.5" /><path d="M4 20h16" />
              </svg>
              {extraindo ? 'Gerando...' : 'Extrair'}
            </button>
            <div className="tri-opcoes" role="menu">
              <button type="button" role="menuitem" onClick={() => void extrair('pdf')} disabled={!!extraindo}>
                <b>PDF</b><span>para imprimir ou enviar</span>
              </button>
              <button type="button" role="menuitem" onClick={() => void extrair('xlsx')} disabled={!!extraindo}>
                <b>Excel</b><span>para somar e ordenar</span>
              </button>
            </div>
          </div>

          <span className="tri-carimbo">
            Última leitura: <b>{ultima ? quando(ultima) : '—'}</b>
          </span>
          <button className="bt bt-neutro" type="button" disabled={!!andamento} onClick={() => void atualizar()}>
            {andamento ?? 'Atualizar agora'}
          </button>
        </div>
      </div>

      {dados && !erro && (
        <div className="tri-resumo">
          <span><b>{dados.total.toLocaleString('pt-BR')}</b> {dados.total === 1 ? 'chamado' : 'chamados'}</span>
          {filtrando && <span className="tri-pt" />}
          {filtrando && <span>com os filtros de agora</span>}
        </div>
      )}

      {erro && <div className="erro-caixa">{erro}</div>}

      {dados === null && !erro ? (
        <Carregando texto="Carregando os chamados..." />
      ) : erro ? null : dados!.linhas.length === 0 ? (
        <div className="vazio">
          {filtrando
            ? 'Nenhum chamado corresponde a esses filtros.'
            : 'A base ainda está vazia. Rode a leitura do Trílogo.'}
        </div>
      ) : (
        <>
          <div className="tabela-rolo tri-painel" ref={painel}>
            <table className="tabela tri-tabela">
              <thead>
                <tr>
                  <th className="c-ticket">Ticket</th>
                  <th className="c-loja">Loja</th>
                  <th className="c-conta">Conta</th>
                  <th className="c-status">Status</th>
                  <th className="c-prio">Prio.</th>
                  <th>Descrição</th>
                  <th className="c-resp">Responsável</th>
                  <th className="c-data">Criado em</th>
                  <th className="c-prazo">Prazo</th>
                  <th className="c-num">Custo R$</th>
                  <th className="c-num c-anexos">Anexos</th>
                </tr>
              </thead>
              <tbody ref={corpo}>
                {dados!.linhas.map(c => (
                  <tr
                    key={c.id}
                    className="tri-linha"
                    onClick={() => abrir(c.numero)}
                    title={`Abrir o chamado ${c.numero}`}
                  >
                    <td className="c-ticket">
                      {/* Botão de verdade dentro da célula: a linha inteira é
                          clicável para o ponteiro, e o teclado tem por onde
                          entrar. Um `onClick` no `tr` sozinho não é alcançável
                          por quem navega com Tab. */}
                      <button
                        type="button"
                        className="tri-num"
                        onClick={e => { e.stopPropagation(); abrir(c.numero) }}
                      >
                        {c.numero}
                      </button>
                    </td>
                    <td className="c-loja" data-encolhe="loja" data-base="13" data-peso="400">
                      <span title={c.loja}>{c.loja}</span>
                    </td>
                    <td className="c-conta">
                      <span className={'tri-conta ' + (c.conta === 'civil' ? 'ct-civil' : 'ct-inst')}>
                        {contaPorExtenso(c.conta)}
                      </span>
                    </td>
                    <td className="c-status"><span className={'pino ' + classeDoStatus(c.status)}>{c.status || '—'}</span></td>
                    <td className="c-prio">
                      {c.prioridade
                        ? <span className={'pino ' + classeDaPrioridade(c.prioridade)}>{c.prioridade}</span>
                        : <span className="tri-vazio">—</span>}
                    </td>
                    {/* O texto inteiro fica no `title`: o que não coube na
                        linha não se perde, aparece ao parar o ponteiro. */}
                    <td data-encolhe="descricao" data-base="13" data-peso="400">
                      <span title={c.descricao || undefined}>
                        {(c.descricao || '—').replace(/\s+/g, ' ').trim()}
                      </span>
                    </td>
                    <td className="c-resp" data-encolhe="resp" data-base="12.6" data-peso="400">
                      <span>{c.responsavel || '—'}</span>
                    </td>
                    <td className="c-data tri-fraco">{quando(c.criado_em)}</td>
                    <td className="c-prazo tri-fraco">{quando(c.prazo, false)}</td>
                    <td className="c-num">{emReais(c.custo_total)}</td>
                    <td className="c-num c-anexos tri-fraco">{c.anexos || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="tri-rodape">
            <span className="tri-mostrando">
              Mostrando <b>{((dados!.pagina - 1) * dados!.por_pagina + 1).toLocaleString('pt-BR')}</b>
              –<b>{Math.min(dados!.pagina * dados!.por_pagina, dados!.total).toLocaleString('pt-BR')}</b>
              {' '}de <b>{dados!.total.toLocaleString('pt-BR')}</b>
            </span>

            <label className="tri-porpagina">
              Por página
              <select
                value={porPagina}
                onChange={e => { setPorPagina(Number(e.target.value)); setPagina(1) }}
              >
                {(filtros?.por_pagina ?? [100, 250, 500]).map(n => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            </label>

            <div className="tri-navega">
              <button type="button" disabled={dados!.pagina <= 1} onClick={() => setPagina(1)} title="Primeira página" aria-label="Primeira página">«</button>
              <button type="button" disabled={dados!.pagina <= 1} onClick={() => setPagina(p => p - 1)} title="Página anterior" aria-label="Página anterior">‹</button>
              <span className="tri-pagina">Página {dados!.pagina} de {dados!.paginas}</span>
              <button type="button" disabled={dados!.pagina >= dados!.paginas} onClick={() => setPagina(p => p + 1)} title="Próxima página" aria-label="Próxima página">›</button>
              <button type="button" disabled={dados!.pagina >= dados!.paginas} onClick={() => setPagina(dados!.paginas)} title="Última página" aria-label="Última página">»</button>
            </div>
          </div>
        </>
      )}
      </div>
    </>
  )
}
