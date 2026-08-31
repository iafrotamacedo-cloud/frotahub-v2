// rev 1 — a tela de Estatísticas dentro do FrotaHub
//
// O QUE ESTE ARQUIVO É, E O QUE ELE DEIXOU DE SER
//
//	Enquanto rodava offline, o bloco de Estatísticas trazia a própria casca:
//	barra lateral, cabeçalho, migalha de pão e um endereço só dele
//	(`#/operacionais/chamados`). Nada disso sobreviveu à fusão, e não deveria:
//	o FrotaHub já tem uma casca, e duas cascas na mesma página são duas
//	navegações discordando sobre onde a pessoa está.
//
//	O que ficou é o miolo: carregar o retrato, filtrar, e desenhar. Quem diz
//	QUAL tela mostrar é a árvore de menus (`menu/arvore.ts`), e quem anda entre
//	elas é a navegação da casca — por isso o `abrir` chega de fora.
//
// O RETRATO É CARREGADO UMA VEZ, E FILTRADO NO NAVEGADOR
//
//	As nove telas cruzam as mesmas tabelas o tempo todo, e o filtro de período é
//	uma pílula que tem que responder na hora. Voltar ao servidor a cada clique
//	deixaria a tela lenta para não guardar nada — o retrato inteiro cabe numa
//	resposta só (ver `interno/modulos/estatisticas` no motor).
import { useEffect, useMemo, useState } from 'react'
import { Painel, type Etapa } from '../../componentes/Painel'
import { Fonte, type Base } from './dados'
import { calcular, type Filtro } from './calculo'
import { hoje, iso } from './formato'
import { ICONE, cartoes } from './telas'
import {
  TelaBalanco, TelaChamados, TelaCustos, TelaExpectativa, TelaFaturamento,
  TelaFila, TelaOnde, TelaOrcamentos, TelaTempo,
} from './telas'

/**
 * As telas desta seção, pelo nome que elas têm em `menu/arvore.ts`.
 *
 * O prefixo `est-` não é enfeite: é por ele que a casca sabe, numa linha só,
 * que a tela é desta seção — sem precisar listar as doze uma a uma em App.tsx.
 */
export type TelaEstatistica =
  | 'est-raiz' | 'est-operacionais' | 'est-financeiras'
  | 'est-expectativa' | 'est-chamados' | 'est-onde' | 'est-tempo' | 'est-fila'
  | 'est-custos' | 'est-orcamentos' | 'est-faturamento' | 'est-balanco'

const GRUPOS: Etapa[] = [
  {
    chave: 'operacionais', titulo: 'Operacionais',
    descricao: 'Chamados, onde, tempo de atendimento e a fila de hoje',
    icone: ICONE.chamados,
  },
  {
    chave: 'financeiras', titulo: 'Financeiras',
    descricao: 'Custos, orçamentos, faturamento e balanço',
    icone: ICONE.custos,
  },
]

const ATALHOS: [string, () => [Date, Date]][] = [
  ['Este mês', () => { const a = hoje(); return [new Date(a.getFullYear(), a.getMonth(), 1), a] }],
  ['Mês passado', () => {
    const a = hoje()
    const d = new Date(a.getFullYear(), a.getMonth() - 1, 1)
    const u = new Date(a.getFullYear(), a.getMonth(), 0, 23, 59, 59)
    return [d, u]
  }],
  ['3 meses', () => { const a = hoje(); return [new Date(a.getFullYear(), a.getMonth() - 2, 1), a] }],
  ['Desde julho', () => [new Date(2026, 6, 1), hoje()]],
  ['Tudo', () => [new Date(2025, 0, 1), hoje()]],
]

const PADRAO = 'Desde julho'
const atalhoDe = (nome: string) => ATALHOS.find(a => a[0] === nome)![1]()

/** As telas que NÃO têm filtro: a Expectativa mede o contrato inteiro, sempre. */
const SEM_FILTRO = new Set<TelaEstatistica>(['est-expectativa'])

interface Props {
  tela: TelaEstatistica
  titulo: string
  descricao: string
  /** O que a barra do nó abre. Vem da árvore de menus, pela casca. */
  abrir: (rota: string) => void
}

export function Estatisticas({ tela, titulo, descricao, abrir }: Props) {
  const [base, setBase] = useState<Base | null>(null)
  const [erro, setErro] = useState<string | null>(null)

  const [atalho, setAtalho] = useState<string | null>(PADRAO)
  const [[de, ate], setPeriodo] = useState<[Date, Date]>(() => atalhoDe(PADRAO))
  const [conta, setConta] = useState('')
  const [loja, setLoja] = useState('')
  const [personalizar, setPersonalizar] = useState(false)

  useEffect(() => {
    let vivo = true
    Fonte.carregar()
      .then(b => { if (vivo) setBase(b) })
      .catch((e: Error) => { if (vivo) setErro(e.message) })
    return () => { vivo = false }
  }, [])

  const f: Filtro = useMemo(
    () => ({ de, ate, conta, loja: loja === '' ? null : +loja }),
    [de, ate, conta, loja],
  )
  const r = useMemo(() => (base ? calcular(base, f) : null), [base, f])

  if (erro) {
    return <div className="aviso-caixa">Não consegui carregar as estatísticas. {erro}</div>
  }
  if (!base || !r) {
    return <div className="carregando"><span className="giro" /><p>Carregando as estatísticas…</p></div>
  }

  const escolherAtalho = (nome: string) => { setAtalho(nome); setPeriodo(atalhoDe(nome)) }
  const mudarData = (qual: 'de' | 'ate', v: string) => {
    if (!v) return
    setAtalho(null)
    const d = new Date(v + (qual === 'de' ? 'T00:00:00' : 'T23:59:59'))
    setPeriodo(([a, b]) => (qual === 'de' ? [d, b] : [a, d]))
  }

  const lojas = base.unidades.filter(u => u.no_escopo).sort((a, b) => a.nome.localeCompare(b.nome))
  const carimbo = base.geradoEm
    ? new Date(base.geradoEm).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
    : '—'
  const barras = cartoes(r)

  // ---- os três nós: as barras verticais, sem filtro ----
  if (tela === 'est-raiz' || tela === 'est-operacionais' || tela === 'est-financeiras') {
    const etapas = tela === 'est-raiz' ? GRUPOS
      : tela === 'est-operacionais' ? barras.operacionais
        : barras.financeiras
    return (
      <div className="est">
        <header className="hero">
          <div><h1>{titulo}</h1><p>{descricao}</p></div>
          <div className="carimbo">
            Lido do banco às <b>{carimbo}</b>
            {tela !== 'est-raiz' && <> · período: <b>{atalho ?? 'personalizado'}</b></>}
          </div>
        </header>
        <Painel etapas={etapas} aoEscolher={abrir} />
      </div>
    )
  }

  // ---- as nove telas ----
  const semFiltro = SEM_FILTRO.has(tela)

  // O INVÓLUCRO `.est` NÃO É DECORAÇÃO
  //
  //	O CSS desta seção redefine nomes que a casca também usa — `.num`, `.pino`,
  //	`.vazio`, `.forte`, `.barra`. Solto na folha global, ele mudaria a cara de
  //	telas que ninguém pediu para mexer. Preso aqui dentro, não sai.
  return (
    <div className="est">
    <p className="sub-tela">{descricao}</p>

    {/* A BARRA DE FILTRO É UMA LINHA SÓ
        Período em pílulas, conta em pílulas, loja num seletor. As datas livres
        ficam atrás de "Personalizar": quem precisa acha, quem não precisa não vê. */}
    {!semFiltro && <>
      <div className="barra">
        <div className="pilulas">
          {ATALHOS.map(([n]) => (
            <button key={n} type="button" className={atalho === n ? 'ativo' : ''}
              onClick={() => { escolherAtalho(n); setPersonalizar(false) }}>{n}</button>
          ))}
          <button type="button" className={personalizar || !atalho ? 'ativo' : ''}
            onClick={() => setPersonalizar(p => !p)}>Personalizar</button>
        </div>
        <div className="pilulas">
          {([['', 'Todas'], ['instalacoes', 'Instalações'], ['civil', 'Civil']] as const).map(([v, n]) => (
            <button key={v} type="button" className={conta === v ? 'ativo' : ''}
              onClick={() => setConta(v)}>{n}</button>
          ))}
        </div>
        <select className="sel" value={loja} onChange={e => setLoja(e.target.value)}>
          <option value="">Todas as lojas</option>
          {lojas.map(u => <option key={u.id} value={u.id}>{u.nome}</option>)}
        </select>
      </div>
      {(personalizar || !atalho) && (
        <div className="barra datas">
          <label>De <input type="date" value={iso(de)} onChange={e => mudarData('de', e.target.value)} /></label>
          <label>Até <input type="date" value={iso(ate)} onChange={e => mudarData('ate', e.target.value)} /></label>
          <span className="periodo-txt">
            {de.toLocaleDateString('pt-BR')} a {ate.toLocaleDateString('pt-BR')}
          </span>
        </div>
      )}
    </>}

    {tela === 'est-expectativa' ? <TelaExpectativa r={r} />
      : tela === 'est-chamados' ? <TelaChamados r={r} semConta={!conta} />
        : tela === 'est-onde' ? <TelaOnde r={r} />
          : tela === 'est-tempo' ? <TelaTempo r={r} />
            : tela === 'est-fila' ? <TelaFila r={r} />
              : tela === 'est-custos' ? <TelaCustos r={r} />
                : tela === 'est-orcamentos' ? <TelaOrcamentos r={r} />
                  : tela === 'est-faturamento' ? <TelaFaturamento r={r} />
                    : <TelaBalanco r={r} />}
    </div>
  )
}
