// rev 1 — Administrativo > Compras > OCs Inseridas
//
// O PEDIDO (10/09/2026): um card entre "Inserir OC" e "Equalizar Propostas",
// com dois cards menores dentro — Processadas e Rejeitadas — "tal qual
// fazemos em Manutenção › Contrato São Luiz › Orçamentos".
//
// MESMO DESENHO DO HUB DE ORÇAMENTOS, NÃO UM NOVO
//
//	`Orcamentos.tsx` é o hub de referência do sistema: um painel de cartões
//	(`componentes/Painel.tsx`) que a tela alimenta com contador + prévia, e um
//	roteador interno por `onde` (o pedaço do endereço depois da tela) que troca
//	o painel por uma sub-tela. Esta tela segue a MESMA receita — dois cartões
//	em vez de sete, mas o mecanismo é idêntico (CORE-06).
//
// A SUB-TELA VEM DO ENDEREÇO
//   `#/administrativo/compras/ocs-inseridas/processadas` abre a lista, e o
//   voltar do navegador devolve ao painel em vez de sair do sistema — mesma
//   decisão de `Orcamentos`/`ServicosHub`/`DadosTrilogo` (P-33).
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Painel, type Etapa } from '../../componentes/Painel'
import { Carregando } from '../../componentes/Carregando'
import { ListaDeOrdens } from './ListaDeOrdens'
import { type PainelDeOrdens } from './tipos'

interface Props {
  /** A sub-tela aberta, vinda do endereço. Vazio = o painel. */
  onde?: string
  abrir: (onde: string) => void
  voltar: () => void
}

export function OcsInseridas({ onde, abrir, voltar }: Props) {
  const [dados, setDados] = useState<PainelDeOrdens | null>(null)
  const [erro, setErro] = useState('')

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<PainelDeOrdens>('/administrativo/compras/ordens/painel'))
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o painel.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar, onde])

  if (onde === 'processadas') {
    return <ListaDeOrdens vista="processadas" titulo="Processadas" vazia="Nenhuma OC processada ainda." voltar={voltar} />
  }
  if (onde === 'rejeitadas') {
    return <ListaDeOrdens vista="rejeitadas" titulo="Rejeitadas" vazia="Nenhuma OC rejeitada." voltar={voltar} />
  }

  if (erro) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  return (
    <div className="orc-painel">
      <Painel etapas={montarEtapas(dados)} aoEscolher={abrir} />
    </div>
  )
}

function montarEtapas(d: PainelDeOrdens): Etapa[] {
  const linhas = (l?: { nome_arquivo: string; numero: string | null; erro_leitura: string | null; criado_em: string }[]) =>
    (l ?? []).map(x => ({
      texto: x.numero ? `${x.nome_arquivo} · nº ${x.numero}` : x.nome_arquivo,
      fim: emDia(x.criado_em),
    }))

  return [
    {
      chave: 'processadas',
      titulo: 'Processadas',
      descricao: 'OCs cuja leitura passou nos dois filtros — fornecedor com CNPJ, faturamento para a Frota Macedo.',
      icone: <IconeOk />,
      numero: d.processadas,
      rotulo: 'ordens',
      previaTitulo: 'últimas processadas',
      previa: linhas(d.previa?.processadas),
      previaVazia: 'nenhuma OC processada ainda',
    },
    {
      chave: 'rejeitadas',
      titulo: 'Rejeitadas',
      descricao: 'OCs que caíram no bloqueio dos filtros — fornecedor sem CNPJ, ou faturamento para outro grupo.',
      icone: <IconeAviso />,
      numero: d.rejeitadas,
      rotulo: 'ordens',
      faixa: '#B8801F',
      previaTitulo: 'últimas rejeitadas',
      previa: linhas(d.previa?.rejeitadas),
      previaVazia: 'nenhuma OC rejeitada',
    },
  ]
}

function emDia(s: string): string {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })
}

/* Os ícones vivem aqui, e não no `Icone` compartilhado, porque são deste
   módulo — mesma razão de `Orcamentos.tsx`. */

function IconeOk() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  )
}

function IconeAviso() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 9v4M12 17h.01" />
      <path d="M10.3 3.9 2.6 17a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" />
    </svg>
  )
}
