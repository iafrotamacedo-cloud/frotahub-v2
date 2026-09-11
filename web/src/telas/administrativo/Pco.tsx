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
import { ListaDeOrdens } from './ListaDeOrdens'
import { type PainelDoPCO, type LinhaDaPreviaDeOrdem } from './tipos'

interface Props {
  /** A sub-tela aberta, vinda do endereço. Vazio = o painel. */
  onde?: string
  abrir: (onde: string) => void
  voltar: () => void
}

export function Pco({ onde, abrir, voltar }: Props) {
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
    return (
      <ListaDeOrdens vista="pco-pendentes" titulo="Pendentes de envio"
        vazia="Nenhuma OC pendente de envio." voltar={voltar} />
    )
  }
  if (onde === 'enviados') {
    return (
      <ListaDeOrdens vista="pco-enviados" titulo="Enviados"
        vazia="Nenhuma OC enviada ainda." voltar={voltar} />
    )
  }

  if (erro) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  return (
    <div className="orc-painel">
      <Painel etapas={montarEtapas(dados)} aoEscolher={abrir} />
    </div>
  )
}

function montarEtapas(d: PainelDoPCO): Etapa[] {
  const linhas = (l?: LinhaDaPreviaDeOrdem[]) =>
    (l ?? []).map(x => ({
      texto: x.numero ? `${x.nome_arquivo} · nº ${x.numero}` : x.nome_arquivo,
      fim: emDia(x.criado_em),
    }))

  return [
    {
      chave: 'pendentes',
      titulo: 'Pendentes de envio',
      descricao: 'OCs processadas que ainda não tiveram o pedido de PCO enviado ao cliente.',
      icone: <IconeRelogio />,
      numero: d.pendentes,
      rotulo: 'ordens',
      previaTitulo: 'últimas processadas',
      previa: linhas(d.previa?.pendentes),
      previaVazia: 'nada pendente de envio',
      rodape: 'aguardando o envio',
    },
    {
      chave: 'enviados',
      titulo: 'Enviados',
      descricao: 'OCs cujo pedido de PCO já foi mandado ao cliente.',
      icone: <IconeOk />,
      numero: d.enviados,
      rotulo: 'ordens',
      previaTitulo: 'últimas enviadas',
      previa: linhas(d.previa?.enviados),
      previaVazia: 'nenhuma OC enviada ainda',
    },
  ]
}

function emDia(s: string): string {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })
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
