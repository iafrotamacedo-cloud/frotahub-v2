// rev 1 — a tela principal de Orçamentos
//
// É o painel dos cinco botões e o roteador das cinco sub-telas. Nada mais: a
// regra de cada etapa mora no arquivo dela.
//
// A SUB-TELA VEM DO ENDEREÇO
//   `#/manutencao/contrato-sao-luiz/orcamentos/notas` abre a lista de notas, e
//   o voltar do navegador devolve ao painel em vez de sair do sistema. É a
//   mesma decisão da ficha do chamado (P-33): voltar não custa o caminho andado.
import { useCallback, useEffect, useState } from 'react'
import { motor } from '../../motor/cliente'
import { Painel, type Etapa } from '../../componentes/Painel'
import { Carregando } from '../../componentes/Carregando'
import { Arquivos } from './Arquivos'
import { Lancar } from './Lancar'
import { Correcoes } from './Correcoes'
import { Planilhas } from './Planilhas'
import { emReais, emDataHora, type Painel as DadosDoPainel } from './tipos'

interface Props {
  /** A sub-tela aberta, vinda do endereço. Vazio = o painel. */
  onde?: string
  abrir: (onde: string) => void
  voltar: () => void
}

export function Orcamentos({ onde, abrir, voltar }: Props) {
  const [dados, setDados] = useState<DadosDoPainel | null>(null)
  const [erro, setErro] = useState('')

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<DadosDoPainel>('/orcamentos/painel'))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar o painel.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar, onde])

  if (onde === 'notas') return <Arquivos fila="orcamento" voltar={voltar} />
  if (onde === 'rateio') return <Arquivos fila="rateio" voltar={voltar} />
  if (onde === 'lancar') return <Lancar voltar={voltar} />
  if (onde && onde.startsWith('correcoes')) {
    return <Correcoes frente={onde.split(':')[1]} voltar={voltar} />
  }
  if (onde === 'planilhas') return <Planilhas voltar={voltar} />

  if (erro) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  return (
    <div className="orc-painel">
      <div className="orc-topo">
        <span className="orc-pulso" aria-hidden />
        <span>
          {dados.ultima_insercao
            ? `última inserção · ${emDataHora(dados.ultima_insercao)}`
            : 'nenhum arquivo inserido ainda'}
        </span>
      </div>
      <Painel etapas={montarEtapas(dados)} aoEscolher={abrir} />
    </div>
  )
}

function montarEtapas(d: DadosDoPainel): Etapa[] {
  const arquivo = (l?: Array<{ nome_arquivo: string; inserido_em: string }>) =>
    (l ?? []).map(x => ({ texto: x.nome_arquivo, fim: emDia(x.inserido_em) }))
  const fila = (l?: Array<{ ticket: number; loja: string | null; valor: number | null }>) =>
    (l ?? []).map(x => ({ texto: `${x.ticket} · ${primeiraPalavra(x.loja)}`, fim: emReais(x.valor) }))

  return [
    {
      chave: 'notas',
      titulo: 'Notas e DAVs',
      descricao: 'Insira e gerencie os arquivos direto pelo site.',
      icone: <IconeNota />,
      numero: d.notas_arquivos,
      rotulo: 'arquivos',
      previaTitulo: 'últimos inseridos',
      previa: arquivo(d.previa?.notas),
      previaVazia: 'nenhum arquivo inserido',
      rodape: 'clique para inserir',
    },
    {
      chave: 'rateio',
      titulo: 'Notas para rateio',
      descricao: 'Amarre cada nota aos tickets que ela atende.',
      icone: <IconeRateio />,
      numero: d.rateio_sem_ticket,
      rotulo: 'sem ticket',
      previaTitulo: 'esperando ticket',
      previa: arquivo(d.previa?.rateio),
      previaVazia: 'todas as notas amarradas',
    },
    {
      chave: 'lancar',
      titulo: 'Lançar orçamentos',
      descricao: 'Gera, confere o teto e lança no Trílogo.',
      icone: <IconeRobo />,
      numero: d.a_lancar,
      rotulo: 'na fila',
      previaTitulo: 'próximos da fila',
      previa: fila(d.previa?.lancar),
      previaVazia: 'nada esperando lançamento',
    },
    {
      chave: 'correcoes',
      titulo: 'Correções',
      descricao: 'O que travou antes de virar orçamento.',
      icone: <IconeAviso />,
      // O número da coluna é a SOMA das quatro frentes: é o que o usuário
      // precisa saber antes de passar o mouse.
      numero: d.sem_ticket + d.sem_associacao + d.apagados,
      rotulo: 'pendências',
      faixa: '#B8801F',
      previaTitulo: 'o que travou',
      previa: [
        { texto: 'Sem ticket', fim: String(d.sem_ticket) },
        { texto: 'Sem associação', fim: String(d.sem_associacao) },
        { texto: 'Não lançados', fim: String(d.a_lancar) },
        { texto: 'Apagados', fim: String(d.apagados) },
      ],
      filhos: [
        {
          chave: 'correcoes:sem-ticket',
          titulo: 'Sem ticket',
          descricao: 'Nota lida, nenhum ticket encontrado',
          numero: d.sem_ticket,
          previaVazia: 'nenhuma nota órfã',
        },
        {
          chave: 'correcoes:sem-associacao',
          titulo: 'Sem associação',
          descricao: 'Ticket escrito não bate com a base',
          numero: d.sem_associacao,
          previaVazia: 'todos os tickets casaram',
        },
        {
          chave: 'correcoes:nao-lancados',
          titulo: 'Não lançados',
          descricao: 'Gerados, ainda fora do Trílogo',
          numero: d.a_lancar,
          previaVazia: 'nada parado aqui',
        },
        {
          chave: 'correcoes:apagados',
          titulo: 'Apagados',
          descricao: 'Restaurar o que foi excluído',
          numero: d.apagados,
          previaVazia: 'nenhum na lixeira',
        },
      ],
    },
    {
      chave: 'planilhas',
      titulo: 'Planilhas de controle',
      descricao: 'O livro-razão: gerados, lançados, faturados, pagos.',
      icone: <IconeTabela />,
      // NÃO É FILA, É LIVRO-RAZÃO
      //   Mostrar "0 na fila" aqui leria como "não tem nada", que é o contrário
      //   da verdade: são todos os orçamentos que já existiram.
      numero: d.no_total,
      rotulo: 'no total',
      rodape: emReais(d.valor_total) + ' acumulados',
      previaTitulo: 'situação',
      previa: [
        { texto: 'Aguardando aprovação', fim: String(d.aguardando_aprovacao) },
        { texto: 'A lançar', fim: String(d.a_lancar) },
        { texto: 'Apagados', fim: String(d.apagados) },
      ],
    },
  ]
}

/** O dia, curto: a coluna é estreita e a data por extenso empurraria o nome. */
function emDia(s: string): string {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })
}

/** Só o essencial do nome da loja: "LOJA 20 - RUI BARBOSA" vira "LOJA 20". */
function primeiraPalavra(loja: string | null): string {
  if (!loja) return '–'
  const corte = loja.split(' - ')[0]
  return corte.length > 14 ? corte.slice(0, 13) + '…' : corte
}

/* Os ícones vivem aqui, e não no `Icone` compartilhado, porque são deste módulo.
   Um conjunto compartilhado que cresce a cada tela vira um arquivo que ninguém
   consegue mais ler. */

function IconeNota() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5M9 13h6M9 17h4" />
    </svg>
  )
}

function IconeRateio() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 7h16M4 12h16M4 17h9" /><circle cx="18.5" cy="17" r="2.5" />
    </svg>
  )
}

function IconeRobo() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3v4M5.6 5.6l2.8 2.8M3 12h4M18.4 5.6l-2.8 2.8M21 12h-4" />
      <rect x="7" y="12" width="10" height="9" rx="2" /><path d="M10 16h4" />
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

function IconeTabela() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="4" width="18" height="16" rx="2" /><path d="M3 9h18M9 9v11M15 9v11" />
    </svg>
  )
}
