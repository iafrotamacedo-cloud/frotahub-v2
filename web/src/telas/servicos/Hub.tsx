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
import { Icone } from '../../componentes/Icone'
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
      icone: <Icone nome="lista" />,
      numero: d.candidatos_pendentes,
      rotulo: 'pendentes',
      rodape: 'clique para decidir',
    },
    {
      chave: 'pendentes',
      titulo: 'Orçamentos pendentes',
      descricao: 'Na fila de Serviço, ainda sem orçamento.',
      icone: <Icone nome="lista" />,
      numero: d.orcamentos_pendentes,
      rotulo: 'sem orçamento',
      rodape: 'clique para inserir orçamento',
    },
    {
      chave: 'gerados',
      titulo: 'Orçamentos já gerados',
      descricao: 'O que já tem PDF anexado, feito ou lançado no Trílogo.',
      icone: <Icone nome="lista" />,
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
      icone: <Icone nome="chave-inglesa" />,
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
      icone: <Icone nome="dinheiro" />,
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
      icone: <Icone nome="grafico" />,
      // NÃO É FILA, É LIVRO-RAZÃO — mesma lógica de "Planilhas de controle"
      // em Orçamentos: mostrar "0" aqui leria como "vazio", que não é a
      // verdade quando é a soma de tudo que já existiu.
      numero: d.total_ativos,
      rotulo: 'no total',
      rodape: 'filtros e exportação',
    },
  ]
}
