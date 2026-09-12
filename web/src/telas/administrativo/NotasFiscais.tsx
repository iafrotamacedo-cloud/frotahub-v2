// rev 1 — Administrativo > Notas Fiscais (Bloco A, 12/09/2026)
//
// QUATRO CARTÕES, UM KANBAN SÓ
//
//	Recebida (o almoxarife escaneou na obra) → entregue no escritório
//	(autenticado por quem recebeu) → enviada ao cliente, no malote. Fim do
//	ciclo OC-NF. "Aguardando NF" não é uma etapa da nota — é a fila de OCs
//	do lado de fora esperando a primeira nota nascer (ver o cabeçalho de
//	`notas_fiscais.go`, `nf_progresso_ordens`).
//
// "OS KANBANS DEVEM ESTAR SEMPRE RELACIONADOS UM COM O OUTRO" (o dono)
//
//	Por isso "Aguardando" fica bem ao lado das outras três, no mesmo painel
//	— a OC nunca larga a nota de vista, e a nota nunca esquece de qual OC
//	veio.
//
// "ACESSOS POR OBRA" NÃO MORA AQUI
//
//	Mesmo lugar de "PCO — Destinatários": Configurações › [item próprio],
//	não um botão dentro do painel — são as duas telas de quem CONFIGURA o
//	módulo, e não de quem TRABALHA nele todo dia (ver `arvore.ts`).
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Painel, type Etapa } from '../../componentes/Painel'
import { Carregando } from '../../componentes/Carregando'
import type { Perfil } from '../../sessao/tipos'
import { AguardandoNF } from './AguardandoNF'
import { ListaDeNF } from './ListaDeNF'
import { RotinaNFReceber, RotinaNFEntregar, temRotina } from './rotinasNF'
import type { PainelDeNF } from './tipos'

interface Props {
  onde?: string
  perfil: Perfil | null
  abrir: (onde: string) => void
}

export function NotasFiscais({ onde, perfil, abrir }: Props) {
  const [dados, setDados] = useState<PainelDeNF | null>(null)
  const [erro, setErro] = useState('')

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<PainelDeNF>('/administrativo/nf/painel'))
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o painel.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar, onde])

  if (onde === 'aguardando') return <AguardandoNF />
  if (onde === 'recebidas') return <ListaDeNF vista="recebidas" titulo="Recebidas" />
  if (onde === 'entregues') return <ListaDeNF vista="entregues" titulo="Entregues no escritório" />
  if (onde === 'enviadas') return <ListaDeNF vista="enviadas" titulo="Enviadas ao cliente" />

  if (erro) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  return (
    // SEM O SUFIXO "--N": aquele teto (`escuro.css`) foi desenhado para o
    // hover-expandir de "OCs Inseridas" em Compras — reserva espaço para um
    // quarto slot que só existe ali. Nenhum cartão daqui tem `filhos`, e a
    // quantidade varia com a rotina de quem está logado (1, 3 ou 4) — o
    // teto fixo deixaria um vão à direita sem sentido nenhum.
    <div className="orc-painel orc-painel--estreito">
      <Painel etapas={montarEtapas(dados, perfil)} aoEscolher={abrir} />
    </div>
  )
}

function montarEtapas(d: PainelDeNF, perfil: Perfil | null): Etapa[] {
  const podeReceber = temRotina(perfil, RotinaNFReceber)
  const podeEntregar = temRotina(perfil, RotinaNFEntregar)
  const etapas: Etapa[] = []
  if (podeReceber) {
    etapas.push({
      chave: 'aguardando',
      titulo: 'Aguardando NF',
      descricao: 'OCs já enviadas ao cliente, esperando a nota fiscal chegar.',
      icone: <IconeRelogio />,
      numero: d.aguardando,
      rotulo: 'ordens',
      rodape: 'aguardando o recebimento',
    })
  }
  if (podeEntregar) {
    etapas.push(
      {
        chave: 'recebidas',
        titulo: 'Recebidas',
        descricao: 'Notas escaneadas pelo almoxarife, aguardando chegar fisicamente no escritório.',
        icone: <IconeCaixa />,
        numero: d.recebidas,
        rotulo: 'notas',
      },
      {
        chave: 'entregues',
        titulo: 'Entregues no escritório',
        descricao: 'Notas já em mãos, aguardando o envio ao cliente.',
        icone: <IconeOk />,
        numero: d.entregues,
        rotulo: 'notas',
      },
      {
        chave: 'enviadas',
        titulo: 'Enviadas ao cliente',
        descricao: 'Notas já despachadas em malote — fim do ciclo.',
        icone: <IconeMalote />,
        numero: d.enviadas,
        rotulo: 'notas',
      },
    )
  }
  return etapas
}

function IconeRelogio() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="8.4" />
      <path d="M12 7.4V12l3.2 2" />
    </svg>
  )
}

function IconeCaixa() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 8 12 4l8 4-8 4-8-4Z" />
      <path d="M4 8v9l8 4 8-4V8" />
      <path d="M12 12v9" />
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

function IconeMalote() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 10h16v9a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-9Z" />
      <path d="M8 10V7a4 4 0 0 1 8 0v3" />
      <path d="M4 14h16" />
    </svg>
  )
}
