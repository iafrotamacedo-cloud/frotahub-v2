// rev 1 — o hub de Serviço: 6 cards, mesmo padrão de Orçamentos
//
// A SUB-TELA VEM DO ENDEREÇO
//
//	Mesma decisão de telas/orcamentos/Orcamentos.tsx: `onde` é a chave da
//	etapa aberta, e cards com `filhos` usam uma chave composta
//	("execucao:aberto") que este arquivo decodifica — não vira nó novo na
//	árvore de menu (ver menu/arvore.ts).
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Painel as PainelDeCards, type Etapa } from '../../componentes/Painel'
import { Carregando } from '../../componentes/Carregando'
import type { Perfil } from '../../sessao/tipos'
import type { Painel } from './tipos'
import { Candidatos } from './listas/Candidatos'
import { ListaDeServicos } from './listas/ListaDeServicos'
import { Planilha } from './Planilha'
import { MarcarComoServico } from './MarcarComoServico'

// abrevia as nove chamadas de ListaDeServicos abaixo — cada uma só muda
// título/status/pco/ação, o resto é sempre o mesmo.
function lista(props: Omit<Parameters<typeof ListaDeServicos>[0], 'perfil' | 'voltar'>, perfil: Perfil, voltar: () => void) {
  return <ListaDeServicos {...props} perfil={perfil} voltar={voltar} />
}

interface Props {
  onde?: string
  perfil: Perfil
  abrir: (onde: string) => void
  voltar: () => void
}

export function Hub({ onde, perfil, abrir, voltar }: Props) {
  const [dados, setDados] = useState<Painel | null>(null)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [marcando, setMarcando] = useState(false)

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      setDados(await motor<Painel>('/servicos/painel'))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o painel.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar, onde])

  // ---- roteamento das 9 listas + a planilha ----
  if (onde === 'candidatos') return <Candidatos voltar={voltar} />
  if (onde === 'pendentes') {
    return lista({ titulo: 'Orçamentos pendentes', status: 'aguardando_orcamento', acao: 'inserir-orcamento' }, perfil, voltar)
  }
  if (onde === 'gerados:feitos') {
    return lista({ titulo: 'Orçamentos feitos', status: 'orcamento_feito', acao: 'lancar' }, perfil, voltar)
  }
  if (onde === 'gerados:lancados') {
    return lista({ titulo: 'Orçamentos lançados', status: 'orcamento_lancado', acao: 'aprovar-rejeitar' }, perfil, voltar)
  }
  if (onde === 'execucao:aberto') {
    return lista({ titulo: 'Execução — Aberto', status: 'aprovado_execucao', acao: 'nenhuma' }, perfil, voltar)
  }
  if (onde === 'execucao:em-curso') {
    return lista({ titulo: 'Execução — Em execução', status: 'em_execucao', acao: 'nenhuma' }, perfil, voltar)
  }
  if (onde === 'execucao:executado') {
    return lista({ titulo: 'Execução — Executado', status: 'finalizado', acao: 'nenhuma' }, perfil, voltar)
  }
  if (onde === 'faturamento:aguardando-pco') {
    return lista({ titulo: 'Aguardando PCO', status: 'aguardando_faturamento', semPCO: true, acao: 'preencher-pco' }, perfil, voltar)
  }
  if (onde === 'faturamento:a-faturar') {
    return lista({ titulo: 'A faturar', status: 'aguardando_faturamento', comPCO: true, acao: 'anexar-nf' }, perfil, voltar)
  }
  if (onde === 'faturamento:faturado') {
    return lista({ titulo: 'Faturado', status: 'faturado', acao: 'nenhuma' }, perfil, voltar)
  }
  if (onde === 'planilha') return <Planilha voltar={voltar} />

  if (erro) return <div className="erro-caixa">{erro}</div>
  if (!dados) return <Carregando texto="Carregando o painel de Serviço..." />

  return (
    <>
      <div className="hero-linha" style={{ margin: '2px 0 14px' }}>
        <span />
        <button type="button" className="bt bt-neutro" onClick={() => setMarcando(true)}>
          Marcar chamado como Serviço
        </button>
      </div>
      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}
      <PainelDeCards etapas={montarEtapas(dados)} aoEscolher={abrir} />
      {marcando && (
        <MarcarComoServico
          aoFechar={() => setMarcando(false)}
          aoMarcar={recadoNovo => { setMarcando(false); setRecado(recadoNovo); void carregar() }}
        />
      )}
    </>
  )
}

function montarEtapas(d: Painel): Etapa[] {
  return [
    {
      chave: 'candidatos',
      titulo: 'Candidatos',
      descricao: 'Chamados que o sistema suspeita serem Serviço, aguardando decisão.',
      icone: <IconeCandidato />,
      numero: d.candidatos_pendentes,
      rotulo: 'pendentes',
      rodape: 'clique para decidir',
    },
    {
      chave: 'pendentes',
      titulo: 'Orçamentos pendentes',
      descricao: 'Na fila de Serviço, ainda sem orçamento.',
      icone: <IconeAnexo />,
      numero: d.orcamentos_pendentes,
      rotulo: 'sem orçamento',
      rodape: 'clique para inserir orçamento',
    },
    {
      chave: 'gerados',
      titulo: 'Orçamentos já gerados',
      descricao: 'O que já tem PDF anexado, feito ou lançado no Trílogo.',
      icone: <IconeGerado />,
      numero: d.orcamento_feito + d.orcamento_lancado,
      rotulo: 'gerados',
      previaTitulo: 'situação',
      previa: [
        { texto: 'Feitos', fim: String(d.orcamento_feito) },
        { texto: 'Lançados', fim: String(d.orcamento_lancado) },
      ],
      filhos: [
        {
          chave: 'gerados:feitos', titulo: 'Feitos', descricao: 'PDF anexado, pronto pra lançar no Trílogo',
          numero: d.orcamento_feito, previaVazia: 'nenhum orçamento feito',
        },
        {
          chave: 'gerados:lancados', titulo: 'Lançados', descricao: 'Já lançado no Trílogo, aguardando decisão do cliente',
          numero: d.orcamento_lancado, previaVazia: 'nenhum orçamento lançado',
        },
      ],
    },
    {
      chave: 'execucao',
      titulo: 'Execução',
      descricao: 'Aprovado pelo cliente — avança sozinho conforme o Trílogo confirma.',
      icone: <IconeExecucao />,
      numero: d.execucao_aberto + d.execucao_em_curso + d.execucao_executado,
      rotulo: 'em execução',
      previaTitulo: 'situação',
      previa: [
        { texto: 'Aberto', fim: String(d.execucao_aberto) },
        { texto: 'Em execução', fim: String(d.execucao_em_curso) },
        { texto: 'Executado', fim: String(d.execucao_executado) },
      ],
      filhos: [
        { chave: 'execucao:aberto', titulo: 'Aberto', descricao: 'Aprovado, aguardando começar',
          numero: d.execucao_aberto, previaVazia: 'nada aberto' },
        { chave: 'execucao:em-curso', titulo: 'Em execução', descricao: 'O Trílogo confirma que já começou',
          numero: d.execucao_em_curso, previaVazia: 'nada em execução' },
        { chave: 'execucao:executado', titulo: 'Executado', descricao: 'Aguardando vistoria',
          numero: d.execucao_executado, previaVazia: 'nada executado' },
      ],
    },
    {
      chave: 'faturamento',
      titulo: 'Faturamento',
      descricao: 'Vistoriado — aguardando PCO, faturar ou já faturado.',
      icone: <IconeFatura />,
      numero: d.faturamento_aguardando_pco + d.faturamento_a_faturar,
      rotulo: 'a faturar',
      previaTitulo: 'situação',
      previa: [
        { texto: 'Aguardando PCO', fim: String(d.faturamento_aguardando_pco) },
        { texto: 'A faturar', fim: String(d.faturamento_a_faturar) },
        { texto: 'Faturado', fim: String(d.faturamento_faturado) },
      ],
      filhos: [
        { chave: 'faturamento:aguardando-pco', titulo: 'Aguardando PCO', descricao: 'Vistoriado, esperando o PCO do cliente',
          numero: d.faturamento_aguardando_pco, previaVazia: 'nenhum aguardando PCO' },
        { chave: 'faturamento:a-faturar', titulo: 'A faturar', descricao: 'PCO em mãos, falta a nota fiscal',
          numero: d.faturamento_a_faturar, previaVazia: 'nada a faturar' },
        { chave: 'faturamento:faturado', titulo: 'Faturado', descricao: 'Nota fiscal anexada — fim do funil',
          numero: d.faturamento_faturado, previaVazia: 'nada faturado ainda' },
      ],
    },
    {
      chave: 'planilha',
      titulo: 'Planilha de controle',
      descricao: 'O livro-razão: todos os serviços, em qualquer fila.',
      icone: <IconeTabela />,
      // NÃO É FILA, É LIVRO-RAZÃO — mesma lógica de "Planilhas de controle"
      // em Orçamentos: mostrar "0" aqui leria como "vazio", que não é a
      // verdade quando é a soma de tudo que já existiu.
      numero: d.total_ativos,
      rotulo: 'no total',
      rodape: 'filtros e exportação',
    },
  ]
}

/* Os ícones vivem aqui, e não no `Icone` compartilhado do menu lateral, pela
   mesma razão de orcamentos/Orcamentos.tsx: são deste módulo, e um conjunto
   compartilhado que cresce a cada tela vira um arquivo que ninguém consegue
   mais ler.

   A última — IconeTabela — é IGUAL, de propósito, à de Orçamentos: os dois
   cards são o mesmo conceito (o livro-razão da fila), então usam o mesmo
   desenho — é isso que padroniza a identidade visual entre os dois módulos,
   não um ícone genérico do menu emprestado sem sentido próprio (o que havia
   antes: o mesmo ícone de lista repetido em três cards diferentes). */

function IconeCandidato() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="m5 19 12-12" />
      <path d="m17 3 1 2 2 1-2 1-1 2-1-2-2-1 2-1z" />
      <path d="m5 15 .6 1.4L7 17l-1.4.6L5 19l-.6-1.4L3 17l1.4-.6z" />
    </svg>
  )
}

function IconeAnexo() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
      <path d="M14 3v5h5" /><path d="M12 12v6M9 15h6" />
    </svg>
  )
}

function IconeGerado() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 8h10a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1z" />
      <path d="M5 5h10a1 1 0 0 1 1 1v2" /><path d="M10 13h5M10 17h5" />
    </svg>
  )
}

function IconeExecucao() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="m15 3 6 6-3 3-6-6z" /><path d="M13.5 8.5 4 18l2 2 9.5-9.5" />
    </svg>
  )
}

function IconeFatura() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M6 3h12v17l-2-1.3-2 1.3-2-1.3-2 1.3-2-1.3-2 1.3z" />
      <path d="M9 8h6M9 12h6" />
      <path d="M9.5 16.2c.4.5 1 .8 1.7.8 1 0 1.8-.6 1.8-1.4s-.8-1.1-1.8-1.3c-1-.2-1.7-.6-1.7-1.3 0-.8.8-1.4 1.7-1.4.7 0 1.3.3 1.7.8" />
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
