// rev 1 — Administrativo > Compras (o painel)
//
// O PEDIDO (10/09/2026): "OCs Inseridas" tem que se abrir DENTRO do próprio
// cartão, em dois cartões menores — Processadas e Rejeitadas — "tal qual
// fazemos em Manutenção › Contrato São Luiz › Orçamentos". O card de
// referência é "Correções": ele não navega para outra tela ao clicar — ao
// passar o mouse, se abre em quatro sub-cartões dentro do próprio espaço
// (`componentes/Painel.tsx`, campo `filhos`). Só existe nesse desenho
// porque "Correções" é um cartão de um PAINEL DE DADOS (Orçamentos), não de
// um menu de navegação comum — por isso "Compras" precisou virar painel
// também, em vez de continuar caindo no menu genérico da casca.
//
// MESMO MECANISMO DE `Orcamentos.tsx`, MENOR
//   Busca os números uma vez, monta as etapas, entrega para o <Painel>
//   compartilhado. A diferença: aqui as etapas simples ("Inserir OC",
//   "Equalizar Propostas") reaproveitam os MESMOS ícones já declarados no
//   menu (`administrativo.ts`) em vez de ganhar SVG próprio — são
//   navegação pura, sem prévia, e o conjunto de ícones do menu já existe
//   para isso.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Painel, type Etapa } from '../../componentes/Painel'
import { Carregando } from '../../componentes/Carregando'
import { Icone } from '../../componentes/Icone'
import type { PainelDeOrdens } from './tipos'

interface Props {
  aoEscolher: (chave: string) => void
}

export function Compras({ aoEscolher }: Props) {
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

  useEffect(() => { void carregar() }, [carregar])

  if (erro) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  return (
    <div className="orc-painel orc-painel--estreito">
      <Painel etapas={montarEtapas(dados)} aoEscolher={aoEscolher} />
    </div>
  )
}

function montarEtapas(d: PainelDeOrdens): Etapa[] {
  return [
    {
      chave: 'inserir-oc',
      titulo: 'Inserir OC',
      descricao: 'Cadastre uma ordem de compra vinda do Obra Prima.',
      icone: <Icone nome="lista" />,
      numero: d.fila,
      rotulo: 'na fila',
      rodape: 'clique para inserir',
    },
    {
      // O CARTÃO NÃO NAVEGA — ELE SE ABRE
      //   `filhos` tira o clique deste cartão: em vez de `aoEscolher`, ele só
      //   alterna o próprio estado `aberta` (dentro de `<Painel>`). Quem
      //   navega são os dois filhos, cada um com a própria chave
      //   ("ocs-inseridas:processadas"/"...:rejeitadas") — ver o tratamento
      //   dela em `App.tsx`.
      chave: 'ocs-inseridas',
      titulo: 'OCs Inseridas',
      descricao: 'As ordens de compra que já passaram pela leitura.',
      icone: <Icone nome="lista" />,
      numero: d.processadas + d.rejeitadas,
      rotulo: 'lidas',
      previaTitulo: 'o que a leitura decidiu',
      previa: [
        { texto: 'Processadas', fim: String(d.processadas) },
        { texto: 'Rejeitadas', fim: String(d.rejeitadas) },
      ],
      filhos: [
        {
          chave: 'ocs-inseridas:processadas',
          titulo: 'Processadas',
          descricao: 'Passou nos dois filtros — fornecedor com CNPJ, faturamento para a Frota Macedo.',
          numero: d.processadas,
          previaVazia: 'nenhuma OC processada ainda',
        },
        {
          chave: 'ocs-inseridas:rejeitadas',
          titulo: 'Rejeitadas',
          descricao: 'Caiu no bloqueio de um dos dois filtros.',
          numero: d.rejeitadas,
          previaVazia: 'nenhuma OC rejeitada',
        },
      ],
    },
    {
      chave: 'equalizar-propostas',
      titulo: 'Equalizar Propostas',
      descricao: 'Comparar as propostas dos fornecedores.',
      icone: <Icone nome="balanca" />,
      desabilitada: true,
      selo: 'Em breve',
    },
  ]
}
