// rev 2 — Administrativo > Compras > Inserir OC
//
// PASSO 1 (10/09/2026): "um botão para abrir o doc do PC e colocar na tela,
// outro botão para inserir, uma lista embaixo de todos os documentos que
// estão na fila." Mesmo padrão de inserção que Orçamentos já usa
// (`Arquivos.tsx`/`Insercao`).
//
// PASSO 2 (mesma rodada): A LEITURA
//
//	Determinística, sem IA — dois filtros decidem sozinhos: fornecedor precisa
//	ter CNPJ; o CNPJ de faturamento precisa começar com 03720882. Quem passa
//	vai para "Processadas", quem falha vai para "Rejeitadas", com o motivo.
//	Três vistas, botão de leitura em lote com painel de progresso — o mesmo
//	desenho já provado em Orçamentos (`Arquivos.tsx`: `porLer`/`lerTodas`/
//	`PainelDaLeitura`), reaproveitado aqui em vez de reinventado.
import { useCallback, useEffect, useRef, useState } from 'react'
import { motor, enviarArquivos, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { TabelaDeOrdens } from './TabelaDeOrdens'
import {
  type OrdemDeCompra, type ResultadoDaInsercao,
  type VistaDasOrdens, type OrdensPorLer, type LoteDeLeitura,
} from './tipos'

const TITULO_DA_VISTA: Record<VistaDasOrdens, string> = {
  fila: 'Na fila',
  processadas: 'Processadas',
  rejeitadas: 'Rejeitadas',
}

const VAZIA_DA_VISTA: Record<VistaDasOrdens, string> = {
  fila: 'Nada na fila. Insira o PDF de uma OC para começar.',
  processadas: 'Nenhuma OC passou pela leitura ainda.',
  rejeitadas: 'Nenhuma OC foi rejeitada.',
}

export function InserirOC() {
  const [ordens, setOrdens] = useState<OrdemDeCompra[] | null>(null)
  const [vista, setVista] = useState<VistaDasOrdens>('fila')
  const [erro, setErro] = useState('')
  const [recado, setRecado] = useState('')
  const [vendo, setVendo] = useState<{ endereco: string; nome: string } | null>(null)

  // "LER TODAS" PRECISA SABER QUANTAS FALTAM NA FILA INTEIRA, NÃO NA TELA
  //   Mesma razão de `orcamentos.Arquivos`: quem vê 100 por vez não pode achar
  //   que o botão "leu tudo" quando só leu o que estava à vista.
  const [porLer, setPorLer] = useState<OrdensPorLer | null>(null)
  const [lote, setLote] = useState<LoteDeLeitura | null>(null)
  const pararLeitura = useRef(false)
  const [lendoUma, setLendoUma] = useState<string | null>(null)

  const carregar = useCallback(async () => {
    try {
      const q = vista === 'fila' ? '' : '?vista=' + vista
      const r = await motor<{ ordens: OrdemDeCompra[] }>('/administrativo/compras/ordens' + q)
      setOrdens(r.ordens)
    } catch (e) {
      setOrdens([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
    try {
      setPorLer(await motor<OrdensPorLer>('/administrativo/compras/ordens/porler'))
    } catch { setPorLer(null) }
  }, [vista])

  useEffect(() => { void carregar() }, [carregar])

  async function lerUma(id: string) {
    if (lendoUma || lote?.rodando) return
    setLendoUma(id)
    setErro('')
    try {
      await motor(`/administrativo/compras/ordens/${id}/ler`, { metodo: 'POST' })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui ler esta ordem de compra.')
    } finally {
      setLendoUma(null)
      await carregar()
    }
  }

  // LER TODAS — UMA OC POR CHAMADA, DE PROPÓSITO
  //
  //	Uma chamada só que lesse cinquenta OCs ficaria minutos aberta e morreria
  //	no tempo limite do servidor, jogando fora o trabalho já feito. Uma por
  //	vez: cada OC que termina está gravada, e parar no meio deixa lidas as que
  //	já foram. "Parar depois desta" existe pela mesma razão: quem começou tem
  //	que poder desistir sem fechar a aba no meio de uma gravação.
  async function lerTodas() {
    const alvo = porLer?.ordens ?? []
    if (alvo.length === 0) return
    pararLeitura.current = false
    setErro('')
    setLote({ total: alvo.length, feito: 0, lidas: 0, falhas: 0, rodando: true, motivos: [] })

    for (const o of alvo) {
      if (pararLeitura.current) break
      setLote(l => (l ? { ...l, agora: o.nome_arquivo } : l))
      try {
        await motor(`/administrativo/compras/ordens/${o.id}/ler`, { metodo: 'POST' })
        setLote(l => l && ({ ...l, feito: l.feito + 1, lidas: l.lidas + 1 }))
      } catch (e) {
        // Uma OC que não leu não derruba as outras: ela fica "falhou", com o
        // motivo, e o próximo clique tenta de novo.
        const motivo = e instanceof ErroMotor ? e.message : 'não consegui ler'
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

  // VER É NA TELA — REGRA DA CASA (`claude/padroes-de-tela.md`)
  //   Abrir noutra aba tira a pessoa do sistema e deixa o documento à deriva
  //   num lugar sem voltar. Ver documento é sempre aqui dentro.
  async function abrirArquivo(o: OrdemDeCompra) {
    try {
      const r = await motor<{ url: string }>(`/administrativo/compras/ordens/${o.id}/arquivo`)
      setVendo({ endereco: r.url, nome: o.nome_arquivo })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
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

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Inserir OC</h1>
          <p>A ordem de compra aprovada no Obra Prima, sempre em PDF digital. A leitura confere fornecedor e faturamento sozinha.</p>
        </div>
        <div className="adm-hero-acoes">
          {lote?.rodando ? (
            <button type="button" className="bt bt-neutro" onClick={() => { pararLeitura.current = true }}>
              parar depois desta
            </button>
          ) : (
            porLer && porLer.ordens.length > 0 && (
              <button
                type="button"
                className="bt bt-forte"
                disabled={!!lendoUma}
                title="Lê as ordens de compra que ainda estão na fila."
                onClick={() => void lerTodas()}
              >
                Ler ordens ({porLer.ordens.length})
              </button>
            )
          )}
        </div>
      </header>

      {lote && <PainelDaLeitura dados={lote} fechar={() => setLote(null)} />}

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado('')} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      <Insercao
        aoTerminar={carregar}
        aoRecado={setRecado}
        aoFalhar={setErro}
      />

      <div className="adm-lista-cab">
        <h2>{TITULO_DA_VISTA[vista]}</h2>
        <select
          className={'adm-vista' + (vista !== 'fila' ? ' ligado' : '')}
          value={vista}
          onChange={e => setVista(e.target.value as VistaDasOrdens)}
        >
          <option value="fila">na fila</option>
          <option value="processadas">processadas</option>
          <option value="rejeitadas">rejeitadas</option>
        </select>
      </div>

      {ordens === null ? (
        <Carregando texto="Carregando..." />
      ) : ordens.length === 0 ? (
        <div className="vazio">{VAZIA_DA_VISTA[vista]}</div>
      ) : (
        <TabelaDeOrdens
          ordens={ordens}
          onVer={o => void abrirArquivo(o)}
          onLer={id => void lerUma(id)}
          lendoId={lendoUma}
          loteRodando={lote?.rodando}
        />
      )}
    </>
  )
}

// ---------------------------------------------------------------------------
// o painel de progresso — mesmo desenho de `orcamentos.PainelDaLeitura`
// ---------------------------------------------------------------------------

function PainelDaLeitura({ dados, fechar }: { dados: LoteDeLeitura; fechar: () => void }) {
  const pct = dados.total === 0 ? 0 : Math.round((dados.feito / dados.total) * 100)
  return (
    <div className="adm-lote">
      <div className="adm-lote-topo">
        <span>
          {dados.rodando
            ? <>lendo{dados.agora ? ` — ${dados.agora}` : ''}…</>
            : <>leitura terminada</>}
        </span>
        <strong>{dados.feito} de {dados.total}</strong>
      </div>
      <div className="adm-lote-barra"><span style={{ width: `${pct}%` }} /></div>
      {!dados.rodando && (
        <div className="adm-lote-fim">
          <p>
            <strong>{dados.lidas}</strong>{' '}
            {dados.lidas === 1 ? 'ordem processada' : 'ordens processadas'}
            {dados.falhas > 0 && <> · <strong>{dados.falhas}</strong> rejeitada{dados.falhas === 1 ? '' : 's'}</>}.
          </p>
          {dados.motivos.length > 0 && (
            <ul className="adm-lote-motivos">
              {dados.motivos.map(m => <li key={m}>{m}</li>)}
            </ul>
          )}
          <button type="button" className="bt bt-neutro" onClick={fechar}>Entendi</button>
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// a barra de inserção — mesmo desenho de Orçamentos (Arquivos.tsx/Insercao):
// arrastar ou escolher, um botão Inserir, e o aviso de arquivo repetido com
// os dois nomes (o de agora e o de quem já estava), nunca só a contagem.
// ---------------------------------------------------------------------------

function Insercao({ aoTerminar, aoRecado, aoFalhar }: {
  aoTerminar: () => Promise<void>
  aoRecado: (s: string) => void
  aoFalhar: (s: string) => void
}) {
  const [escolhidos, setEscolhidos] = useState<File[]>([])
  const [enviando, setEnviando] = useState(false)
  const [sobre, setSobre] = useState(false)
  const [repetidos, setRepetidos] = useState<{ nome: string; como: string }[]>([])
  const campo = useRef<HTMLInputElement>(null)

  async function inserir() {
    if (!escolhidos.length || enviando) return
    setEnviando(true)
    aoFalhar('')
    try {
      const r = await enviarArquivos<{ arquivos: ResultadoDaInsercao[] }>(
        '/administrativo/compras/ordens', escolhidos)
      const ruins = r.arquivos.filter(a => a.erro)
      const repetidas = r.arquivos.filter(a => a.ja_existia)
      const novas = r.arquivos.filter(a => a.id && !a.ja_existia)

      if (ruins.length) {
        aoFalhar(ruins.map(a => `${a.nome}: ${a.erro}`).join(' · '))
      } else if (novas.length) {
        aoRecado(novas.length === 1 ? '1 ordem de compra inserida.' : `${novas.length} ordens de compra inseridas.`)
      }
      setRepetidos(repetidas.map(a => ({ nome: a.nome, como: a.ja_existia_como ?? '' })))
      setEscolhidos([])
      if (campo.current) campo.current.value = ''
      await aoTerminar()
    } catch (e) {
      aoFalhar(e instanceof ErroMotor ? e.message : 'Não consegui inserir os arquivos.')
    } finally {
      setEnviando(false)
    }
  }

  return (
    <div
      className={'adm-insercao' + (sobre ? ' sobre' : '')}
      onDragOver={e => { e.preventDefault(); setSobre(true) }}
      onDragLeave={() => setSobre(false)}
      onDrop={e => {
        e.preventDefault()
        setSobre(false)
        setEscolhidos(Array.from(e.dataTransfer.files))
      }}
    >
      {repetidos.length > 0 && (
        <div className="aviso-caixa">
          <b>{repetidos.length}</b>{' '}
          {repetidos.length === 1 ? 'arquivo não entrou' : 'arquivos não entraram'}
          {' '}porque {repetidos.length === 1 ? 'já estava' : 'já estavam'} na fila —
          {' '}mesmo arquivo, byte a byte. Nenhuma OC foi perdida.
          <ul style={{ margin: '6px 0 0', paddingLeft: 18 }}>
            {repetidos.map(x => (
              <li key={x.nome}>
                <b>{x.nome}</b>
                {x.como && x.como !== x.nome && <> é a mesma que <b>{x.como}</b></>}
              </li>
            ))}
          </ul>
        </div>
      )}

      <p>insira o PDF da OC direto pelo site — sempre digital, nunca foto</p>
      <div className="linha">
        <button type="button" className="bt bt-neutro" onClick={() => campo.current?.click()}>
          Escolher arquivos
        </button>
        <input
          ref={campo}
          type="file"
          multiple
          accept=".pdf"
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
          className="bt bt-forte"
          disabled={!escolhidos.length || enviando}
          onClick={() => void inserir()}
        >
          {enviando ? 'Inserindo…' : 'Inserir'}
        </button>
      </div>
    </div>
  )
}
