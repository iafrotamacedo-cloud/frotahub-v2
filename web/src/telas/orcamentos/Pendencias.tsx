// rev 1 — Pendências: os orçamentos que esperam alguém MOVER o chamado
//
// A TELA QUE FALTAVA, E O BURACO QUE ELA TAPA
//
//	Medido em 27/08/2026: 93 orçamentos gerados, 25 podendo subir, 68 travados
//	pelo status do chamado — e apenas 23 visíveis em algum lugar do sistema. Os
//	23 eram os RECUSADOS, que é outra coisa: a marca `lancamento_bloqueio` só
//	nasce quando alguém aperta o botão e o Trílogo nega. Os 68 nunca foram
//	tentados, então não têm marca, então não apareciam em tela nenhuma.
//
//	O motor já montava esta lista desde a 015 — agrupada por ticket, com PDF e
//	planilha prontos — e nenhuma tela chamava. Regra escrita, testada e invisível
//	é regra que não existe para quem trabalha.
//
// SÃO DUAS LISTAS PORQUE VÃO PARA DUAS PESSOAS
//
//	encarregados  o chamado está Aberto ou Em execução: é serviço NOSSO a
//	              terminar. Enquanto não for executado, o Trílogo não aceita o
//	              custo.
//	cliente       o chamado está Arquivado ou foi Reaberto. Nenhuma das duas nós
//	              destravamos.
//
//	Por isso as colunas mudam de uma para a outra: ao cliente interessa a frase
//	que ELE escreveu ao reabrir; ao encarregado, o que foi pedido no chamado.
//	É a mesma escolha que o PDF do motor faz, e de propósito — duas telas para o
//	mesmo dado que discordam na hora de imprimir são duas verdades (CORE-06).
//
// A LINHA É O TICKET, NÃO O ORÇAMENTO
//
//	Um ticket com três partes paradas vai como UMA linha, com a soma. Mandar três
//	faria o cliente perguntar por que três — e "são partes" é detalhe nosso.
//
// VER A LISTA É NA TELA
//
//	O PDF abre no visor, como todo documento desta casa. O "salvar como" mora lá
//	dentro e sempre pergunta a pasta. A planilha, essa não tem como abrir na
//	tela: ela é para mandar, e vai direto para o seletor de pasta.
//
// MARCAR COMO COBRADO NÃO MANDA NADA
//
//	Quem manda é a pessoa, com a lista na mão. Isto aqui só registra que saiu, e
//	em que dia — é o que separa "já cobrei e não veio" de "nunca cobrei". Um
//	registro de envio que mente é pior do que nenhum, então o botão só marca o
//	que a pessoa escolheu, uma escolha por vez.
import { useCallback, useEffect, useState } from 'react'
import { motor, arquivoDoMotor, salvarArquivo } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { useFocado } from '../../componentes/Foco'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { BarraDeVolta } from './Arquivos'
import { emReais, emDataHora, type ListaDePendencias, type Pendencia } from './tipos'

const LISTAS = [
  {
    chave: 'encarregados',
    titulo: 'Nossa equipe',
    desc: 'Chamados Abertos ou Em execução — o Trílogo não aceita custo enquanto o serviço não for concluído',
  },
  {
    chave: 'cliente',
    titulo: 'Cliente',
    desc: 'Chamados Arquivados ou Reabertos — só o cliente destrava',
  },
] as const

type Qual = (typeof LISTAS)[number]['chave']

export function Pendencias({ lista, voltar }: { lista?: string; voltar: () => void }) {
  const [qual, setQual] = useState<Qual>(lista === 'cliente' ? 'cliente' : 'encarregados')
  const [dados, setDados] = useState<ListaDePendencias | null>(null)
  const [erro, setErro] = useState('')
  const [aviso, setAviso] = useState('')
  const [marcando, setMarcando] = useState(false)
  const [vendo, setVendo] = useState(false)
  // A ESCOLHA MORRE AO TROCAR DE LISTA
  //   Os números marcados são de tickets daquela lista. Carregá-los para a outra
  //   faria o botão dizer "marcar 6" com nenhum dos 6 na tela.
  const [escolhidos, setEscolhidos] = useState<Set<number>>(new Set())
  const focado = useFocado()

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<ListaDePendencias>('/orcamentos/pendencias?destino=' + qual))
      setErro('')
    } catch (e) {
      setDados(null)
      setErro(e instanceof Error ? e.message : 'Não consegui carregar as pendências.')
    }
  }, [qual])

  useEffect(() => { void carregar() }, [carregar])

  function trocar(c: Qual) {
    setQual(c)
    setEscolhidos(new Set())
    setAviso('')
  }

  function alternar(ticket: number) {
    setEscolhidos(s => {
      const novo = new Set(s)
      if (novo.has(ticket)) novo.delete(ticket)
      else novo.add(ticket)
      return novo
    })
  }

  const tickets = dados?.tickets ?? []
  const todosMarcados = tickets.length > 0 && escolhidos.size === tickets.length

  async function baixarPlanilha() {
    try {
      const a = await arquivoDoMotor(`/orcamentos/pendencias.xlsx?destino=${qual}`)
      await salvarArquivo(a.blob!, a.nome)
      URL.revokeObjectURL(a.url)
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui montar a planilha.')
    }
  }

  async function marcarCobrados() {
    if (escolhidos.size === 0) return
    setMarcando(true)
    try {
      const r = await motor<{ marcados: number; ignorados: number }>(
        '/orcamentos/pendencias/avisar',
        { metodo: 'POST', corpo: { destino: qual, tickets: [...escolhidos] } })
      // O IGNORADO É NOTÍCIA, NÃO RUÍDO
      //   Ele significa que o chamado ANDOU entre a tela abrir e o clique — e
      //   isso é justamente o que a pessoa queria que acontecesse. Esconder o
      //   número faria a conta não fechar sem explicação.
      setAviso(r.ignorados > 0
        ? `${r.marcados} marcados como cobrados. ${r.ignorados} saíram da lista antes do clique — o chamado andou.`
        : `${r.marcados} marcados como cobrados.`)
      setEscolhidos(new Set())
      await carregar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui registrar o aviso.')
    } finally {
      setMarcando(false)
    }
  }

  if (vendo) {
    return (
      <VisorDeDocumento
        titulo={<>Pendências · {LISTAS.find(l => l.chave === qual)?.titulo}</>}
        caminho={`/orcamentos/pendencias.pdf?destino=${qual}`}
        nomeSugerido={`pendencias-${qual}.pdf`}
        voltar={() => setVendo(false)}
      />
    )
  }

  return (
    <div className="orc-tela">
      <BarraDeVolta voltar={voltar} titulo="Pendências" />

      {!focado && <div className="orc-abas">
        {LISTAS.map(l => (
          <button
            key={l.chave}
            type="button"
            className={'orc-aba' + (qual === l.chave ? ' ligada' : '')}
            onClick={() => trocar(l.chave)}
          >
            <b>{qual === l.chave ? tickets.length : '·'}</b>
            <span>{l.titulo}</span>
          </button>
        ))}
      </div>}

      {erro && <p className="erro">{erro}</p>}
      {aviso && <p className="orc-aviso-fila">{aviso}</p>}

      {!dados ? <Carregando /> : (
        <div className="orc-lista">
          <div className="orc-lista-cab">
            <h2>{LISTAS.find(l => l.chave === qual)?.titulo}</h2>
            <em>{LISTAS.find(l => l.chave === qual)?.desc}</em>
          </div>

          {/* O QUE ESTÁ PARADO, EM DINHEIRO, ANTES DA TABELA
              Trinta e quatro chamados não movem ninguém; oito mil e oitocentos
              reais parados movem. O número é o argumento da cobrança. */}
          <div className="orc-pend-resumo">
            <span><b>{tickets.length}</b> chamados</span>
            <span><b>{dados.orcamentos}</b> orçamentos</span>
            <span><b>{emReais(dados.valor)}</b> parados</span>
            <span className="orc-pend-bts">
              <button type="button" className="orc-bt" onClick={() => setVendo(true)}
                disabled={tickets.length === 0}>ver a lista</button>
              <button type="button" className="orc-bt" onClick={() => void baixarPlanilha()}
                disabled={tickets.length === 0}>baixar planilha</button>
              <button type="button" className="orc-bt forte"
                disabled={escolhidos.size === 0 || marcando}
                onClick={() => void marcarCobrados()}>
                {marcando ? 'marcando…' : `marcar ${escolhidos.size || ''} como cobrados`}
              </button>
            </span>
          </div>

          {tickets.length === 0 ? (
            <p className="orc-vazio grande">
              Nada esperando aqui. Todo orçamento desta fila já pode subir, ou
              já subiu.
            </p>
          ) : (
            <div className="orc-rolagem">
              <table className="orc-tabela">
                <thead>
                  <tr>
                    <th className="orc-pend-marca">
                      <input
                        type="checkbox"
                        aria-label="marcar todos"
                        checked={todosMarcados}
                        onChange={() => setEscolhidos(todosMarcados
                          ? new Set()
                          : new Set(tickets.map(t => t.ticket)))}
                      />
                    </th>
                    <th>Chamado</th>
                    <th>Loja</th>
                    <th>Conta</th>
                    <th>Situação</th>
                    <th>{qual === 'cliente' ? 'O que o cliente disse' : 'O que foi pedido'}</th>
                    <th style={{ textAlign: 'right' }}>Parado</th>
                    <th>Desde</th>
                    <th>Cobrado</th>
                  </tr>
                </thead>
                <tbody>
                  {tickets.map(t => (
                    <Linha
                      key={t.ticket}
                      t={t}
                      qual={qual}
                      marcado={escolhidos.has(t.ticket)}
                      alternar={() => alternar(t.ticket)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function Linha({ t, qual, marcado, alternar }: {
  t: Pendencia
  qual: Qual
  marcado: boolean
  alternar: () => void
}) {
  return (
    <tr className={marcado ? 'orc-pend-escolhida' : undefined}>
      <td className="orc-pend-marca">
        <input type="checkbox" checked={marcado} onChange={alternar}
          aria-label={`marcar o chamado ${t.ticket}`} />
      </td>
      <td>
        <span className="orc-nome">{t.ticket}</span>
        {/* Quantas partes daquele ticket estão paradas. Some da linha quando é
            uma só: "1 orçamento" é ruído em trinta e quatro linhas. */}
        {t.orcamentos > 1 && (
          <div className="orc-sub">{t.orcamentos} partes · {t.partes.join(', ')}</div>
        )}
      </td>
      <td>{t.loja || '–'}</td>
      <td>{t.conta || '–'}</td>
      <td>
        {t.ticket_status || '–'}
        {t.reaberto && <div className="orc-sub aviso">reaberto</div>}
      </td>
      <td className="orc-pend-texto">
        {/* Ao cliente, a frase DELE ao reabrir: muda a conversa de "reabriram"
            para "reabriram dizendo que o serviço não foi executado". Ao
            encarregado, o que o chamado pediu — é onde ele vai e o que termina. */}
        {(qual === 'cliente' ? t.motivo : t.descricao) || <span className="orc-detalhe">–</span>}
      </td>
      <td style={{ textAlign: 'right' }}>{emReais(t.valor)}</td>
      <td>{t.desde_em ? emDataHora(t.desde_em) : '–'}</td>
      <td>
        {t.avisado_em
          ? emDataHora(t.avisado_em)
          : <span className="orc-detalhe">nunca</span>}
      </td>
    </tr>
  )
}
