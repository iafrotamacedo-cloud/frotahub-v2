// rev 1 — a nota à vista enquanto se digita o ticket
//
// POR QUE NÃO É UMA JANELA POR CIMA
//
//	Decisão do dono em 26/08/2026: "abre a nota na tela (sem pop up)". E a razão
//	é a tarefa, não o gosto: quem está aqui precisa LER o número escrito na nota
//	enquanto digita. Uma janela por cima esconde justamente a lista de onde a
//	pessoa veio, e uma que abre em outra aba tira a nota da vista no instante em
//	que ela volta para digitar.
//
//	Aqui a nota ocupa a tela, a barra de ação fica presa em cima dela, e os dois
//	ficam visíveis ao mesmo tempo — que é a única maneira de conferir sem
//	decorar.
//
// A BARRA TEM TRÊS ZONAS, E A ORDEM DELAS NÃO É ACASO
//
//	Esquerda: o que se faz o tempo todo (digitar o ticket).
//	Meio:     o que se acumulou (os tickets já postos), para dar o que conferir.
//	Direita:  o que se faz uma vez e muda o destino da nota — e por último o OK,
//	          maior, porque é o único que fecha.
//
//	Pôr "excluir" ao lado de "adicionar" seria convidar o dedo errado.
import { useEffect, useState } from 'react'
import { motor } from '../../motor/cliente'
import { usePedirFoco } from '../../componentes/Foco'
import { Candidatos } from './Candidatos'
import { emReais } from './tipos'
import type { Conferencia } from './tipos'

export interface Acoes {
  /** Some da fila, mas dá para restaurar. */
  excluir: () => Promise<void>
  /** Manda a nota para a fila de rateio na hora. */
  rateada: () => Promise<void>
  /** Fecha e volta para a lista. */
  concluir: () => Promise<void>
  fechar: () => void
}

export function VisorDaNota({
  documento, nome, valor, tickets, modo, acoes,
}: {
  documento: string
  nome: string
  valor: number | null
  /** Os tickets que a nota já tem. Vazio na fila "sem ticket". */
  tickets: number[]
  modo: 'sem-ticket' | 'sem-associacao'
  acoes: Acoes
}) {
  // A NOTA PEDE A TELA INTEIRA
  //   Quem está aqui precisa LER o papel. A migalha, o título e as abas de
  //   contagem custavam um terço da altura e o pé da nota ficava fora do
  //   alcance. Elas voltam sozinhas quando este visor sai.
  usePedirFoco()

  const [url, setUrl] = useState('')
  const [erro, setErro] = useState('')
  const [ocupado, setOcupado] = useState(false)
  const [digitado, setDigitado] = useState('')
  const [postos, setPostos] = useState<number[]>(tickets)
  const [conf, setConf] = useState<Conferencia | null>(null)
  const [trocando, setTrocando] = useState<number | null>(null)

  // O ENDEREÇO DO ARQUIVO É TEMPORÁRIO
  //   Ele vem assinado e expira. Buscar na abertura, e não guardar na lista, é
  //   o que impede a tela de mostrar um visor quebrado numa aba deixada aberta
  //   desde a manhã.
  useEffect(() => {
    let vivo = true
    void (async () => {
      try {
        const r = await motor<{ url: string }>(`/orcamentos/documentos/${documento}/arquivo`)
        if (vivo) setUrl(r.url)
      } catch (e) {
        if (vivo) setErro(e instanceof Error ? e.message : 'Não consegui abrir a nota.')
      }
    })()
    return () => { vivo = false }
  }, [documento])

  // A CONFERÊNCIA É A FONTE DA VERDADE SOBRE OS TICKETS
  //
  //	`postos` também muda na hora do clique, para a tela responder sem esperar
  //	a viagem. Mas quem manda é o que voltou do motor — senão, uma amarração
  //	que falhou em silêncio continuaria desenhada na tela como se tivesse dado
  //	certo, e o usuário sairia daqui achando que resolveu.
  const conferir = async () => {
    try {
      const c = await motor<Conferencia>(`/orcamentos/documentos/${documento}/conferir`)
      setConf(c)
      if (c.partes?.length) setPostos(c.partes.map(x => x.ticket))
    } catch { /* conferência é ajuda, não obrigação: sem ela a tela ainda serve */ }
  }
  useEffect(() => { void conferir() }, [documento])

  async function adicionar() {
    const n = Number(digitado.replace(/\D/g, ''))
    if (!(n >= 10000 && n <= 999999)) {
      setErro('O ticket tem 5 ou 6 dígitos.')
      return
    }
    if (postos.includes(n)) { setErro(`O ticket ${n} já está nesta nota.`); return }
    setOcupado(true)
    try {
      await motor(`/orcamentos/documentos/${documento}/tickets`,
        { metodo: 'POST', corpo: { tickets: [n] } })
      setPostos([...postos, n])
      setDigitado('')
      setErro('')
      await conferir()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui amarrar o ticket.')
    } finally { setOcupado(false) }
  }

  async function tirar(t: number) {
    setOcupado(true)
    try {
      const r = await motor<{ conferencia?: Conferencia }>(
        `/orcamentos/documentos/${documento}/tickets/${t}/apagar`, { metodo: 'POST' })
      setPostos(postos.filter(x => x !== t))
      if (r.conferencia) setConf(r.conferencia)
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui tirar o ticket.')
    } finally { setOcupado(false) }
  }

  // TROCAR NÃO PEDE O NÚMERO: SUGERE
  //
  //	90% das vezes o ticket está certo na nossa base e errado na nota — o
  //	fornecedor escreveu 13O328 com a letra O, ou inverteu dois dígitos. Pedir
  //	o número de volta é devolver ao usuário o trabalho que a máquina faz
  //	melhor: comparar. Digitar continua existindo, para quando nenhuma
  //	sugestão serve.
  async function depoisDeTrocar() {
    setTrocando(null)
    await conferir()
  }

  async function correr(o: () => Promise<void>) {
    setOcupado(true)
    try { await o(); setErro('') } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui.')
    } finally { setOcupado(false) }
  }

  const ehPDF = nome.toLowerCase().endsWith('.pdf')

  return (
    <div className="visor">
      <div className="visor-barra">
        <div className="visor-esq">
          {modo === 'sem-ticket' && (
            <>
              <input
                type="text" inputMode="numeric" placeholder="número do ticket"
                value={digitado} disabled={ocupado}
                onChange={e => setDigitado(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') void adicionar() }}
                aria-label="número do ticket"
              />
              <button type="button" className="forte" disabled={ocupado || !digitado}
                onClick={() => void adicionar()}>adicionar</button>
            </>
          )}

          <span className="visor-postos">
            {postos.map(t => (
              <span key={t} className={'visor-chip' + (paradoNoTicket(conf, t) ? ' ruim' : '')}>
                <b>{t}</b>
                <i>{lojaDoTicket(conf, t)}</i>
                {modo === 'sem-associacao' && (
                  <u onClick={() => setTrocando(t)} role="button" tabIndex={0}
                    onKeyDown={e => { if (e.key === 'Enter') setTrocando(t) }}>trocar</u>
                )}
                <s onClick={() => void tirar(t)} role="button" tabIndex={0} title="tirar este ticket da nota"
                  onKeyDown={e => { if (e.key === 'Enter') void tirar(t) }}>×</s>
              </span>
            ))}
            {!postos.length && <em className="mut">nenhum ticket nesta nota ainda</em>}
          </span>
        </div>

        <div className="visor-dir">
          <button type="button" className="mut" disabled={ocupado}
            onClick={() => void correr(acoes.rateada)}
            title="manda esta nota para a fila de rateio agora">nota rateada</button>
          <button type="button" className="mut perigo" disabled={ocupado}
            onClick={() => void correr(acoes.excluir)}
            title="tira a nota de todas as filas — dá para restaurar depois">excluir</button>
          <button type="button" className="visor-ok" disabled={ocupado}
            onClick={() => void correr(acoes.concluir)}>OK</button>
        </div>
      </div>

      <div className="visor-aviso">
        <span><b>{nome}</b> · {emReais(valor)}</span>
        <Veredicto conf={conf} />
        <button type="button" className="mut" onClick={acoes.fechar}>voltar sem gravar mais nada</button>
      </div>

      {erro && <p className="erro" style={{ margin: '0 16px 8px' }}>{erro}</p>}

      {trocando !== null && (
        <Candidatos
          documento={documento}
          ticket={trocando}
          fechar={() => setTrocando(null)}
          concluir={depoisDeTrocar}
        />
      )}

      <div className="visor-papel">
        {!url ? <p className="orc-vazio">abrindo a nota…</p>
          : ehPDF
            // SEM A BARRA DE MINIATURAS DO NAVEGADOR
            //   `navpanes=0` tira a coluna de páginas à esquerda e `toolbar=0`
            //   a régua de cima. Numa nota de uma página elas só comem largura
            //   — e a largura aqui é onde os valores estão.
            ? <iframe src={url + '#toolbar=0&navpanes=0&view=FitH'} title={nome} />
            : <img src={url} alt={nome} />}
      </div>
    </div>
  )
}

/**
 * O QUE A CONFERÊNCIA DIZ, EM UMA LINHA
 *
 *   É a MESMA avaliação que a geração faz — não uma segunda opinião escrita
 *   para a tela. Ter duas foi o defeito do sistema antigo: a tela dizia
 *   "pronta" e a geração recusava, sem ninguém entender por quê.
 */
function Veredicto({ conf }: { conf: Conferencia | null }) {
  if (!conf) return <span className="mut">conferindo…</span>
  if (conf.pronta) {
    return <span className="visor-ok-txt">pronta para gerar orçamento</span>
  }
  return <span className="visor-ruim-txt">{conf.motivos?.[0] ?? 'ainda não dá para gerar'}</span>
}

function paradoNoTicket(conf: Conferencia | null, t: number): boolean {
  return !!conf?.partes?.find(p => p.ticket === t && p.motivo)
}

function lojaDoTicket(conf: Conferencia | null, t: number): string {
  const p = conf?.partes?.find(x => x.ticket === t)
  if (!p) return ''
  if (!p.na_base) return 'não existe na base'
  return p.loja || ''
}
