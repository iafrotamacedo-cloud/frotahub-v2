// rev 1 — Administrativo > PCO
//
// O PEDIDO (10/09/2026): "uma cópia de toda a OC processada vai para uma
// fila em PCO" — dois cartões, inicialmente: Pendentes de envio (onde toda
// OC processada nasce) e Enviados (quando o pedido de PCO for mandado ao
// cliente — o envio em si fica para uma rodada futura).
//
// NÃO É UMA CÓPIA DE VERDADE — É A MESMA LINHA, OUTRO FILTRO
//
//	"Processadas" em Compras é a planilha de controle do comprador: cresce
//	para sempre, uma OC nunca sai de lá (mesmo cancelada, um dia, ela fica —
//	só ganha uma marca). PCO é uma SEGUNDA pergunta sobre a mesma OC — "já
//	foi enviada?" — respondida pela coluna `pco_enviado_em` (migração 059,
//	já reservada para isto). Por isso a OC SAI de "Pendentes de envio" assim
//	que for enviada (o filtro deixa de bater), mas nunca sai de "Processadas"
//	— são perguntas diferentes sobre o mesmo registro, não duas tabelas.
//
// MESMO DESENHO DE `OcsInseridas.tsx`/`Orcamentos.tsx`
//   Painel de cartões com contador real, sub-tela decidida pelo endereço.
//   Aqui os dois cartões são de primeiro nível (sem `filhos`): PCO não tem
//   um terceiro irmão disputando o espaço, como "OCs Inseridas" tem em
//   Compras — não há por que escondê-los atrás de um hover.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Painel, type Etapa } from '../../componentes/Painel'
import { Carregando } from '../../componentes/Carregando'
import { Confirmar } from '../../componentes/Confirmar'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { AvisoDesfazer } from './AvisoDesfazer'
import { CanceladasPCO } from './CanceladasPCO'
import { ListaDeOrdens } from './ListaDeOrdens'
import { TabelaDeOrdens } from './TabelaDeOrdens'
import { caminhoSubstituirPCO, useAcoesDaOrdem } from './useAcoesDaOrdem'
import {
  emReais,
  type PainelDoPCO, type OrdemDeCompra, type ResultadoDoEnvio,
} from './tipos'

interface Props {
  /** A sub-tela aberta, vinda do endereço. Vazio = o painel. */
  onde?: string
  abrir: (onde: string) => void
}

export function Pco({ onde, abrir }: Props) {
  const [dados, setDados] = useState<PainelDoPCO | null>(null)
  const [erro, setErro] = useState('')

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<PainelDoPCO>('/administrativo/compras/pco/painel'))
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o painel.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar, onde])

  if (onde === 'pendentes') {
    return <PendentesDeEnvio />
  }
  if (onde === 'enviados') {
    return <ListaDeOrdens vista="pco-enviados" titulo="Enviados" vazia="Nenhuma OC enviada ainda." />
  }
  if (onde === 'canceladas') {
    return <CanceladasPCO />
  }

  if (erro) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  return (
    <div className="orc-painel orc-painel--estreito orc-painel--3">
      <Painel etapas={montarEtapas(dados)} aoEscolher={abrir} />
    </div>
  )
}

// MESMO DESENHO DE `Compras.tsx` (card simplificado)
//   Só contador + título — a lista de arquivos fica na sub-tela ao clicar.
function montarEtapas(d: PainelDoPCO): Etapa[] {
  return [
    {
      chave: 'pendentes',
      titulo: 'Pendentes de envio',
      descricao: 'OCs processadas que ainda não tiveram o pedido de PCO enviado ao cliente.',
      icone: <IconeRelogio />,
      numero: d.pendentes,
      rotulo: 'ordens',
      rodape: 'aguardando o envio',
    },
    {
      chave: 'enviados',
      titulo: 'Enviados',
      descricao: 'OCs cujo pedido de PCO já foi mandado ao cliente.',
      icone: <IconeOk />,
      numero: d.enviados,
      rotulo: 'ordens',
    },
    {
      chave: 'canceladas',
      titulo: 'Excluídas/Substituídas',
      descricao: 'OCs excluídas ou substituídas depois de já terem sido enviadas — só consulta, sem opção de desfazer.',
      icone: <IconeArquivo />,
      numero: d.canceladas,
      rotulo: 'ordens',
      faixa: '#8a8a8a',
    },
  ]
}

// ---------------------------------------------------------------------------
// Pendentes de envio — a ÚNICA das quatro listas de OC com ação de escrita
// (as outras três — fila de Inserir OC à parte — são só consulta). Por isso
// não usa `ListaDeOrdens` (que é read-only de propósito): tem botão por
// linha ("uma por OC") e um geral ("enviar tudo"), os dois pedidos
// explicitamente pelo dono, com a MESMA rota do motor por trás — ver o
// cabeçalho de `pco_enviar.go`.
// ---------------------------------------------------------------------------

function PendentesDeEnvio() {
  const [ordens, setOrdens] = useState<OrdemDeCompra[] | null>(null)
  const [erro, setErro] = useState('')
  const [recado, setRecado] = useState('')
  // '*' = o lote geral está enviando (desliga todos os botões de linha
  // também, para não brigar pela mesma OC no meio do lote).
  const [enviandoId, setEnviandoId] = useState<string | null>(null)
  const [vendo, setVendo] = useState<{ endereco: string; nome: string } | null>(null)
  const acoes = useAcoesDaOrdem(setOrdens, setErro, caminhoSubstituirPCO)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ ordens: OrdemDeCompra[] }>('/administrativo/compras/ordens?vista=pco-pendentes')
      setOrdens(r.ordens)
    } catch (e) {
      setOrdens([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a lista.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  function recadoDoEnvio(r: ResultadoDoEnvio, singular: string, plural: string): string {
    if (!r.enviado) return r.motivo ?? 'Nada para enviar.'
    const n = r.quantidade ?? 0
    return `${n} ${n === 1 ? singular : plural}${r.valor_total ? ` · ${emReais(r.valor_total)}` : ''}.`
  }

  async function enviarUma(id: string) {
    setEnviandoId(id)
    setErro('')
    setRecado('')
    try {
      const r = await motor<ResultadoDoEnvio>(`/administrativo/compras/pco/ordens/${id}/enviar`, { metodo: 'POST' })
      setRecado(recadoDoEnvio(r, 'OC enviada', 'OCs enviadas'))
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui enviar esta OC.')
    } finally {
      setEnviandoId(null)
    }
  }

  async function enviarTodas() {
    if (!ordens?.length) return
    setEnviandoId('*')
    setErro('')
    setRecado('')
    try {
      const r = await motor<ResultadoDoEnvio>('/administrativo/compras/pco/enviar', { metodo: 'POST' })
      setRecado(recadoDoEnvio(r, 'OC enviada', 'OCs enviadas'))
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui enviar o lote.')
    } finally {
      setEnviandoId(null)
    }
  }

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
          <h1>Pendentes de envio</h1>
        </div>
        {ordens && ordens.length > 0 && (
          <button
            type="button"
            className="bt bt-forte"
            disabled={enviandoId !== null}
            title="Manda um e-mail só, com todas as OCs pendentes anexadas num zip."
            onClick={() => void enviarTodas()}
          >
            {enviandoId === '*' ? 'Enviando…' : `Enviar tudo (${ordens.length})`}
          </button>
        )}
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado('')} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      {ordens === null ? (
        <Carregando texto="Carregando..." />
      ) : ordens.length === 0 ? (
        <div className="vazio">Nenhuma OC pendente de envio.</div>
      ) : (
        <TabelaDeOrdens
          ordens={ordens}
          onVer={o => void abrirArquivo(o)}
          onEnviar={id => void enviarUma(id)}
          enviandoId={enviandoId}
          onSubstituir={acoes.pedirSubstituicao}
          onExcluir={acoes.pedirExclusao}
        />
      )}

      <input
        ref={acoes.inputRef}
        type="file"
        accept="application/pdf"
        style={{ display: 'none' }}
        onChange={acoes.arquivoSelecionado}
      />
      {acoes.substituindo && acoes.arquivoEscolhido && (
        <Confirmar
          titulo="Substituir ordem de compra"
          mensagem={`Substituir "${acoes.substituindo.nome_arquivo}" pelo arquivo "${acoes.arquivoEscolhido.name}"?\nA OC antiga sai desta fila, e a nova entra do zero em Inserir OC.`}
          perigo
          rotuloConfirmar="Substituir"
          aoConfirmar={() => void acoes.confirmarSubstituicao()}
          aoFechar={acoes.cancelarSubstituicao}
        />
      )}
      {acoes.exclusao && (
        <AvisoDesfazer
          mensagem={`Você excluiu a OC ${acoes.exclusao.numero || acoes.exclusao.nome_arquivo}.`}
          aoDesfazer={acoes.desfazerExclusao}
          aoConfirmar={() => void acoes.confirmarExclusao()}
        />
      )}
    </>
  )
}

/* Os ícones vivem aqui, e não no `Icone` compartilhado, porque são deste
   módulo — mesma razão de `Orcamentos.tsx`/`OcsInseridas.tsx`. */

function IconeRelogio() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="8.4" />
      <path d="M12 7.4V12l3.2 2" />
    </svg>
  )
}

function IconeOk() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  )
}

function IconeArquivo() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 4h10l6 6v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1Z" />
      <path d="M14 4v6h6" />
      <path d="M8 14h8M8 17.5h5" />
    </svg>
  )
}
