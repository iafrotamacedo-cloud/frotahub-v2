// rev 3 — Locações: o hub (Fase 2: monitoramento · Fase 4: renovações · Fase 5: faturamento)
//
// MESMO DESENHO DE `administrativo/NotasFiscais.tsx`
//
//	Um painel de cartões que decide o próprio `onde` por dentro — o item de
//	menu não tem `sub`, só `tela: 'locacoes'`. "Renovações pendentes" só
//	aparece pra quem tem LOCACOES_RENOVAR_OC (o RC) — o almoxarife ou o
//	engenheiro que só monitora não precisa dessa fila.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Painel, type Etapa } from '../../componentes/Painel'
import { PainelDadosMobile } from '../../componentes/PainelDadosMobile'
import { useEhMobile } from '../../componentes/useEhMobile'
import { Carregando } from '../../componentes/Carregando'
import type { Perfil } from '../../sessao/tipos'
import { Faturamento } from './Faturamento'
import { Monitoramento } from './Monitoramento'
import { RenovacoesPendentes } from './RenovacoesPendentes'
import { RotinaLocacoesMonitorar, RotinaLocacoesRenovarOC, temRotina } from './rotinas'
import { emReais, type PainelDeLocacoes } from './tipos'

interface Props {
  onde?: string
  perfil: Perfil | null
  abrir: (onde: string) => void
}

export function Locacoes({ onde, perfil, abrir }: Props) {
  const ehMobile = useEhMobile()
  const [dados, setDados] = useState<PainelDeLocacoes | null>(null)
  const [erro, setErro] = useState('')

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<PainelDeLocacoes>('/locacoes/painel'))
      setErro('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o painel.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar, onde])

  if (onde === 'descobertos') return <Monitoramento vista="descobertos" titulo="Descobertos" perfil={perfil} />
  if (onde === 'vencendo') return <Monitoramento vista="ativos" titulo="Vencendo" perfil={perfil} filtro={e => e.dias_para_vencer <= 7 && !e.descoberto} />
  if (onde === 'ativos') return <Monitoramento vista="ativos" titulo="Ativos" perfil={perfil} />
  if (onde === 'encerrados') return <Monitoramento vista="encerrados" titulo="Encerrados" perfil={perfil} />
  if (onde === 'renovacoes') return <RenovacoesPendentes />
  if (onde === 'faturamento') return <Faturamento />

  if (!temRotina(perfil, RotinaLocacoesMonitorar)) {
    return <p className="dica" style={{ padding: 20 }}>Você não tem acesso ao monitoramento de Locações.</p>
  }
  if (erro) return <p className="erro">{erro}</p>
  if (!dados) return <Carregando />

  const etapas = montarEtapas(dados, perfil)

  return (
    <>
      {dados.total_mensal > 0 && (
        <p className="dica" style={{ padding: '0 4px 14px' }}>
          Total ativo, hoje: <b>{emReais(dados.total_mensal)}</b>
        </p>
      )}
      {ehMobile ? (
        <PainelDadosMobile etapas={etapas} aoEscolher={abrir} />
      ) : (
        <div className="orc-painel orc-painel--estreito">
          <Painel etapas={etapas} aoEscolher={abrir} />
        </div>
      )}
    </>
  )
}

function montarEtapas(d: PainelDeLocacoes, perfil: Perfil | null): Etapa[] {
  const etapas: Etapa[] = [
    {
      chave: 'descobertos',
      titulo: 'Descobertos',
      descricao: 'Venceram e ninguém decidiu — a locadora continua cobrando.',
      icone: <IconeAlerta />,
      numero: d.descobertos,
      rotulo: 'equipamentos',
      rodape: d.descobertos > 0 ? 'sem OC cobrindo' : undefined,
    },
    {
      chave: 'vencendo',
      titulo: 'Vencendo',
      descricao: 'Vencem nos próximos 7 dias.',
      icone: <IconeRelogio />,
      numero: d.vencendo,
      rotulo: 'equipamentos',
    },
    {
      chave: 'ativos',
      titulo: 'Ativos',
      descricao: 'Todo equipamento locado em uso hoje.',
      icone: <IconeCaixa />,
      numero: d.ativos,
      rotulo: 'equipamentos',
    },
    {
      chave: 'faturamento',
      titulo: 'Faturamento calculado',
      descricao: 'Mês cheio × proporcional, somado por obra — um raio-X, não uma cobrança fechada.',
      icone: <IconeDinheiro />,
    },
  ]
  // "Renovações pendentes" só pro RC (LOCACOES_RENOVAR_OC) — pra quem só
  // monitora ou só recebe, essa fila não é dele.
  if (temRotina(perfil, RotinaLocacoesRenovarOC)) {
    etapas.push({
      chave: 'renovacoes',
      titulo: 'Renovações pendentes',
      descricao: 'Pedidos da obra esperando a OC de renovação.',
      icone: <IconeRenovar />,
      numero: d.renovacoes_pendentes,
      rotulo: 'pedidos',
    })
  }
  etapas.push({
    chave: 'encerrados',
    titulo: 'Encerrados',
    descricao: 'Já devolvidos por inteiro.',
    icone: <IconeOk />,
    numero: d.encerrados,
    rotulo: 'equipamentos',
  })
  return etapas
}

function IconeAlerta() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 4 2 20h20L12 4Z" />
      <path d="M12 10v4" />
      <circle cx="12" cy="17" r="0.6" fill="currentColor" stroke="none" />
    </svg>
  )
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

function IconeDinheiro() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="8.4" />
      <path d="M12 7.5v9M14.6 9.4c0-1.1-1.16-2-2.6-2s-2.6.9-2.6 2c0 1.1 1.16 1.6 2.6 2s2.6.9 2.6 2c0 1.1-1.16 2-2.6 2s-2.6-.9-2.6-2" />
    </svg>
  )
}

function IconeRenovar() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M20 11a8 8 0 1 0-2.34 5.66" />
      <path d="M20 5v6h-6" />
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
