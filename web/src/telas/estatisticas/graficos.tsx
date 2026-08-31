// rev 1 — os gráficos, em SVG próprio
//
// SEM BIBLIOTECA DE GRÁFICO, E É DE PROPÓSITO
//
//	São quatro formas — barra, ranking, rosca e linha — e todas cabem em SVG
//	escrito à mão. Uma biblioteca traria trezentos kilobytes, um tema próprio
//	para brigar com o do FrotaHub, e a próxima versão dela.
//
// O QUE MUDOU DA VERSÃO OFFLINE
//
//	A dica que segue o mouse era montada com `dangerouslySetInnerHTML`, porque
//	o texto dela leva negrito. Só que o texto leva NOME DE LOJA e caminho de
//	ambiente — dado que vem do Trílogo, digitado por gente de fora. Dentro do
//	FrotaHub isso é injeção de HTML na tela de quem estiver olhando o gráfico.
//
//	Agora a dica recebe JSX montado, não texto. O negrito continua, e o que vem
//	do banco entra como texto — que é o que ele é.
import { useState, type ReactNode } from 'react'
import type { Par, PontoDaSerie, LojaDaExpectativa, Expectativa } from './calculo'
import {
  FIM_DO_PLANO, INICIO_CONTRATO, diaMais, diasEntre, esperadoEm, faixaEm, pct, soma,
} from './calculo'
import { CONTA, inteiro, mesCurto, reais, reaisCurto } from './formato'

export const CORES = [
  'var(--c-inst)', 'var(--c-civil)', 'var(--c-3)', 'var(--c-4)',
  'var(--c-5)', 'var(--c-6)', '#8A9099', '#5E5455',
]

// ---------------------------------------------------------------------------
// a dica que segue o mouse
// ---------------------------------------------------------------------------

interface Dica { x: number; y: number; conteudo: ReactNode }

function useDica() {
  const [dica, setDica] = useState<Dica | null>(null)
  const mostrar = (e: { clientX: number; clientY: number }, conteudo: ReactNode) =>
    setDica({ x: e.clientX + 12, y: e.clientY + 12, conteudo })
  const sumir = () => setDica(null)
  const el = dica
    ? <div className="dica" style={{ left: dica.x, top: dica.y }}>{dica.conteudo}</div>
    : null
  return { mostrar, sumir, el }
}

function Legenda({ itens }: { itens: string[] }) {
  return (
    <div className="legenda">
      {itens.map((t, i) => <span key={t}><i style={{ background: CORES[i] }} />{t}</span>)}
    </div>
  )
}

const Vazio = () => <div className="vazio">Nada no período.</div>

/** Lê uma série de uma linha do gráfico. O valor pode faltar; zero é a resposta. */
function valor<T>(d: T, s: keyof T): number {
  const v = d[s]
  return typeof v === 'number' ? v : 0
}

// ---------------------------------------------------------------------------
// barras verticais
// ---------------------------------------------------------------------------

interface BarrasProps<T extends { rotulo: string }> {
  dados: T[]
  series: (keyof T & string)[]
  rotulos: string[]
  empilhar?: boolean
  dinheiro?: boolean
  altura?: number
}

/** Barras verticais, empilhadas ou lado a lado, uma série por chave. */
export function Barras<T extends { rotulo: string }>(
  { dados, series, rotulos, empilhar = true, dinheiro = false, altura = 220 }: BarrasProps<T>,
) {
  const { mostrar, sumir, el } = useDica()
  const L = 1000, A = altura, mE = dinheiro ? 64 : 44, mD = 8, mT = 22, mB = 26
  const totais = dados.map(d =>
    empilhar ? soma(series, s => valor(d, s)) : Math.max(...series.map(s => valor(d, s))))
  const max = Math.max(1, ...totais)
  const y = (v: number) => mT + (A - mT - mB) * (1 - v / max)
  const n = dados.length, larg = (L - mE - mD) / Math.max(n, 1)
  const fmt = dinheiro ? reaisCurto : inteiro
  const ticks = [0, 0.5, 1].map(p => p * max)
  if (!dados.length || (max <= 1 && totais.every(t => !t))) return <Vazio />
  return <>
    <svg viewBox={`0 0 ${L} ${A}`} width="100%" style={{ display: 'block' }}>
      {ticks.map(t => (
        <g key={t}>
          <line className="grade-linha" x1={mE} x2={L - mD} y1={y(t)} y2={y(t)} />
          <text className="eixo" x={mE - 6} y={y(t) + 4} textAnchor="end">{fmt(Math.round(t))}</text>
        </g>
      ))}
      {dados.map((d, i) => {
        let acum = 0
        const x0 = mE + i * larg
        const bw = empilhar ? larg * 0.62 : (larg * 0.7) / series.length
        return (
          <g key={i}>
            {series.map((s, j) => {
              const v = valor(d, s)
              const yTopo = empilhar ? y(acum + v) : y(v)
              const h = empilhar ? y(acum) - yTopo : y(0) - yTopo
              const x = empilhar ? x0 + (larg - bw) / 2 : x0 + larg * 0.15 + j * bw
              acum += v
              return (
                <rect key={String(s)} className="barra" x={x} y={yTopo}
                  width={bw - (empilhar ? 0 : 3)} height={Math.max(0, h)} rx="5" fill={CORES[j]}
                  onMouseMove={e => mostrar(e, <><b>{d.rotulo}</b> · {rotulos[j]}: <b>{dinheiro ? reais(v) : inteiro(v)}</b></>)}
                  onMouseLeave={sumir} />
              )
            })}
            {empilhar && totais[i] > 0 && (
              <text className="rotulo" x={x0 + larg / 2} y={y(totais[i]) - 4} textAnchor="middle">
                {fmt(Math.round(totais[i]))}
              </text>
            )}
            <text className="eixo" x={x0 + larg / 2} y={A - 8} textAnchor="middle">{d.rotulo}</text>
          </g>
        )
      })}
      <line className="eixo-linha" x1={mE} x2={L - mD} y1={y(0)} y2={y(0)} />
    </svg>
    <Legenda itens={rotulos} />{el}
  </>
}

// ---------------------------------------------------------------------------
// ranking horizontal
// ---------------------------------------------------------------------------

/**
 * Ranking horizontal: [rótulo, valor].
 *
 * É HTML, não SVG, de propósito: nome de loja é texto longo, e texto dentro de
 * SVG não quebra linha nem fica nítido em qualquer largura.
 */
export function BarrasH(
  { itens, dinheiro = false, cor = CORES[0], total }:
  { itens: Par[]; dinheiro?: boolean; cor?: string; total?: number },
) {
  if (!itens.length) return <Vazio />
  const max = Math.max(1, ...itens.map(i => i[1]))
  const fmt = dinheiro ? reaisCurto : inteiro
  return (
    <div className="rank">
      {itens.map(([r, v]) => (
        <div className="rk" key={r} title={`${r}: ${dinheiro ? reais(v) : inteiro(v)}`}>
          <span className="rk-r">{r}</span>
          <span className="rk-b"><i style={{ width: Math.max(1, (v / max) * 100) + '%', background: cor }} /></span>
          <span className="rk-v">
            {fmt(Math.round(v))}{total ? <small> · {pct(v, total)}%</small> : null}
          </span>
        </div>
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// rosca
// ---------------------------------------------------------------------------

export function Rosca({ itens, dinheiro = false }: { itens: Par[]; dinheiro?: boolean }) {
  const { mostrar, sumir, el } = useDica()
  const total = soma(itens, i => i[1])
  if (!total) return <Vazio />
  const R = 70, r = 44, cx = 90, cy = 90
  let ang = -Math.PI / 2
  const arcos = itens.map(([rot, v], i) => {
    const a = (v / total) * Math.PI * 2
    const a0 = ang, a1 = ang + a
    ang = a1
    const p = (rr: number, t: number): [number, number] => [cx + rr * Math.cos(t), cy + rr * Math.sin(t)]
    const [x0, y0] = p(R, a0), [x1, y1] = p(R, a1), [x2, y2] = p(r, a1), [x3, y3] = p(r, a0)
    const grande = a > Math.PI ? 1 : 0
    return {
      d: `M${x0},${y0} A${R},${R} 0 ${grande} 1 ${x1},${y1} L${x2},${y2} A${r},${r} 0 ${grande} 0 ${x3},${y3} Z`,
      rot, v, i,
    }
  })
  return (
    <div style={{ display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
      <svg viewBox="0 0 180 180" width="170" height="170">
        {arcos.map(a => (
          <path key={a.rot} className="barra" d={a.d} fill={CORES[a.i]}
            onMouseMove={e => mostrar(e, <><b>{a.rot}</b>: {dinheiro ? reais(a.v) : inteiro(a.v)} ({pct(a.v, total)}%)</>)}
            onMouseLeave={sumir} />
        ))}
        <text x={cx} y={cy - 2} textAnchor="middle"
          style={{ fill: 'var(--ink)', fontSize: 20, fontWeight: 750 }}>
          {dinheiro ? reaisCurto(total) : inteiro(total)}
        </text>
        <text x={cx} y={cy + 14} textAnchor="middle" className="eixo">total</text>
      </svg>
      <div className="legenda" style={{ flexDirection: 'column', gap: 6 }}>
        {itens.map(([rot, v], i) => (
          <span key={rot}>
            <i style={{ background: CORES[i] }} />{rot}
            <b style={{ color: 'var(--ink)', marginLeft: 4 }}>{dinheiro ? reaisCurto(v) : inteiro(v)}</b>
            <span style={{ color: 'var(--mut)' }}>· {pct(v, total)}%</span>
          </span>
        ))}
      </div>{el}
    </div>
  )
}

// ---------------------------------------------------------------------------
// linhas
// ---------------------------------------------------------------------------

interface LinhasProps<T extends { rotulo: string }> {
  dados: T[]
  series: (keyof T & string)[]
  rotulos: string[]
  dinheiro?: boolean
  altura?: number
  largura?: number
}

export function Linhas<T extends { rotulo: string }>(
  { dados, series, rotulos, dinheiro = false, altura = 220, largura = 1000 }: LinhasProps<T>,
) {
  const { mostrar, sumir, el } = useDica()
  const L = largura, A = altura, mE = dinheiro ? 64 : 44, mD = 34, mT = 16, mB = 26
  const max = Math.max(1, ...dados.flatMap(d => series.map(s => valor(d, s))))
  const min = Math.min(0, ...dados.flatMap(d => series.map(s => valor(d, s))))
  const y = (v: number) => mT + (A - mT - mB) * (1 - (v - min) / (max - min || 1))
  const n = dados.length
  const x = (i: number) => mE + (n > 1 ? (i * (L - mE - mD)) / (n - 1) : (L - mE - mD) / 2)
  const fmt = dinheiro ? reaisCurto : inteiro
  if (!dados.length) return <Vazio />
  const ticks = [0, 0.25, 0.5, 0.75, 1].map(p => min + p * (max - min))
  return <>
    <svg viewBox={`0 0 ${L} ${A}`} width="100%" style={{ display: 'block' }}>
      {ticks.map(t => (
        <g key={t}>
          <line className="grade-linha" x1={mE} x2={L - mD} y1={y(t)} y2={y(t)} />
          <text className="eixo" x={mE - 6} y={y(t) + 4} textAnchor="end">{fmt(Math.round(t))}</text>
        </g>
      ))}
      {min < 0 && <line className="eixo-linha" x1={mE} x2={L - mD} y1={y(0)} y2={y(0)} />}
      {series.map((s, j) => (
        <polyline key={String(s)} fill="none" stroke={CORES[j]} strokeWidth="2.5" strokeLinejoin="round"
          points={dados.map((d, i) => `${x(i)},${y(valor(d, s))}`).join(' ')} />
      ))}
      {series.map((s, j) => dados.map((d, i) => (
        <circle key={String(s) + i} cx={x(i)} cy={y(valor(d, s))} r="4" fill={CORES[j]}
          stroke="var(--card)" strokeWidth="1.5"
          onMouseMove={e => mostrar(e, <><b>{d.rotulo}</b> · {rotulos[j]}: <b>{dinheiro ? reais(valor(d, s)) : inteiro(valor(d, s))}</b></>)}
          onMouseLeave={sumir} />
      )))}
      {dados.map((d, i) => (
        <text key={i} className="eixo" x={x(i)} y={A - 8} textAnchor="middle">{d.rotulo}</text>
      ))}
    </svg>
    <Legenda itens={rotulos} />{el}
  </>
}

// ---------------------------------------------------------------------------
// as peças de texto de cada tela
// ---------------------------------------------------------------------------

export const Bloco = ({ t, sub, c = 'c6', children }:
{ t: string; sub?: ReactNode; c?: string; children: ReactNode }) => (
  <section className={'bloco ' + c}><h3>{t}</h3>{sub && <div className="sub">{sub}</div>}{children}</section>
)

/** A frase do topo: a resposta em português. Os números vêm em negrito. */
export const Frase = ({ children }: { children: ReactNode }) => <p className="frase">{children}</p>
export const B = ({ children }: { children: ReactNode }) => <b>{children}</b>

/** Três números, e só. [rótulo, valor, sub?, tom?] */
export type ItemDeNumero = [string, string, string?, string?]

export const Numeros = ({ itens }: { itens: ItemDeNumero[] }) => (
  <div className="nums">
    {itens.map(([r, v, s, tom], i) => (
      <div key={i} className={'num' + (tom ? ' ' + tom : '')}>
        <span className="num-v">{v}</span>
        <span className="num-r">{r}</span>
        {s && <span className="num-s">{s}</span>}
      </div>
    ))}
  </div>
)

/** O que fica dobrado. Abre com um clique, e lembra a escolha só nesta visita. */
export function Detalhes({ children }: { children: ReactNode }) {
  const [aberto, setAberto] = useState(false)
  return (
    <section className="detalhes">
      <button type="button" className="det-btn" onClick={() => setAberto(a => !a)}>
        {aberto ? 'Esconder os detalhes' : 'Mais detalhes'}
        <span className={'det-seta' + (aberto ? ' aberta' : '')}>›</span>
      </button>
      {aberto && <div className="grade">{children}</div>}
    </section>
  )
}

export const Principal = ({ t, sub, children }:
{ t: string; sub?: ReactNode; children: ReactNode }) => (
  <section className="bloco principal"><h3>{t}</h3>{sub && <div className="sub">{sub}</div>}{children}</section>
)

export const Lista = ({ t, sub, children }:
{ t: string; sub?: ReactNode; children: ReactNode }) => (
  <section className="bloco lista"><h3>{t}</h3>{sub && <div className="sub">{sub}</div>}{children}</section>
)

export const Duas = ({ children }: { children: ReactNode }) => <div className="duas">{children}</div>

export const ContaPill = ({ conta }: { conta: string | null }) => (
  <span className={'ct ct-' + (conta === 'civil' ? 'civil' : 'inst')}>{CONTA[conta ?? ''] || conta}</span>
)

// ---------------------------------------------------------------------------
// expectativa × realidade
// ---------------------------------------------------------------------------

const dtCurta = (s: string) => s.split('-').reverse().join('/')

/** O gráfico: a curva do plano ao fundo (com a faixa), a realidade por cima, dia a dia. */
export function GraficoExpectativa(
  { exp, horizonte, serie, patamar }:
  { exp: Expectativa; horizonte: string; serie: PontoDaSerie[] | null; patamar: number | null },
) {
  const { mostrar, sumir, el } = useDica()
  const L = 1000, A = 340, mE = 46, mD = 16, mT = 18, mB = 30
  const de = INICIO_CONTRATO
  const ate = horizonte === 'ciclo' ? FIM_DO_PLANO : diaMais(exp.hoje, 14)
  const total = Math.max(1, diasEntre(de, ate))
  const x = (d: string) => mE + ((L - mE - mD) * diasEntre(de, d)) / total
  const REAL = serie || exp.real
  const maxY = Math.min(300, Math.max(130, ...REAL.map(r => r.indice)) + 5)
  const y = (v: number) => mT + (A - mT - mB) * (1 - v / maxY)

  // Os pontos do plano, um a cada três dias — leve o bastante, e sem quina.
  const diasPlano: string[] = []
  for (let d = de; d <= ate; d = diaMais(d, 3)) diasPlano.push(d)
  const plano = diasPlano.map(d => `${x(d)},${y(esperadoEm(d))}`).join(' ')
  const faixa = diasPlano.map(d => `${x(d)},${y(faixaEm(d)[0])}`).join(' ')
    + ' ' + [...diasPlano].reverse().map(d => `${x(d)},${y(faixaEm(d)[1])}`).join(' ')
  const real = REAL.map(r => `${x(r.dia)},${y(r.indice)}`).join(' ')

  const meses: string[] = []
  for (let m = de.slice(0, 7); m <= ate.slice(0, 7); m = diaMais(m + '-01', 32).slice(0, 7)) meses.push(m)
  const ticks = [0, 25, 50, 75, 100, 125]
  const ult = REAL[REAL.length - 1] || null
  const setembro = '2026-09-01'

  return <>
    <svg viewBox={`0 0 ${L} ${A}`} width="100%" style={{ display: 'block' }}
      onMouseLeave={sumir}
      onMouseMove={e => {
        const r = e.currentTarget.getBoundingClientRect()
        const px = ((e.clientX - r.left) / r.width) * L
        const d = diaMais(de, Math.round(((px - mE) / (L - mE - mD)) * total))
        if (d < de || d > ate) { sumir(); return }
        const pt = REAL.find(p => p.dia === d)
        mostrar(e, <>
          <b>{dtCurta(d)}</b><br />
          plano: <b>{Math.round(esperadoEm(d))}%</b>
          {pt && <><br />realidade: <b>{Math.round(pt.indice)}%</b> ({inteiro(pt.soma)} chamados em 30 dias)</>}
        </>)
      }}>
      {ticks.filter(t => t < maxY).map(t => (
        <g key={t}>
          <line className="grade-linha" x1={mE} x2={L - mD} y1={y(t)} y2={y(t)} />
          <text className="eixo" x={mE - 8} y={y(t) + 4} textAnchor="end">{t}%</text>
        </g>
      ))}
      <line x1={mE} x2={L - mD} y1={y(100)} y2={y(100)} stroke="var(--mut)" strokeWidth="1" strokeDasharray="4 4" />
      <text className="eixo" x={L - mD} y={y(100) - 5} textAnchor="end">
        patamar = {(patamar ?? exp.patamar).toLocaleString('pt-BR', { maximumFractionDigits: 1 })}/mês
      </text>
      {/* contingência: julho e agosto */}
      <rect x={x(de)} y={mT} width={x(setembro) - x(de)} height={A - mT - mB} fill="rgba(209,73,76,.06)" />
      <text className="eixo" x={(x(de) + x(setembro)) / 2} y={mT + 12} textAnchor="middle"
        style={{ fill: 'var(--mut)' }}>contingência</text>
      <polygon points={faixa} fill="rgba(79,134,216,.13)" />
      <polyline points={plano} fill="none" stroke="var(--c-inst)" strokeWidth="2.2" strokeLinejoin="round" opacity=".9" />
      <polyline points={real} fill="none" stroke="var(--red)" strokeWidth="2.6" strokeLinejoin="round" strokeLinecap="round" />
      {ult && <>
        <circle cx={x(ult.dia)} cy={y(ult.indice)} r="5" fill="var(--red)" stroke="var(--card)" strokeWidth="2" />
        <text className="rotulo" x={x(ult.dia) + 9} y={y(ult.indice) + 4}
          style={{ fill: 'var(--red)', fontSize: 12.5, fontWeight: 700 }}>{Math.round(ult.indice)}%</text>
      </>}
      {horizonte === 'ciclo' && (
        <text className="rotulo" x={L - mD} y={y(60) + 16} textAnchor="end"
          style={{ fill: 'var(--c-inst)' }}>−40% em mai/28</text>
      )}
      {meses.map(m => (
        <g key={m}>
          <line className="eixo-linha" x1={x(m + '-01')} x2={x(m + '-01')} y1={A - mB} y2={A - mB + 4} />
          {(horizonte !== 'ciclo' || +m.slice(5) % 2 === 1) && (
            <text className="eixo" x={x(m + '-01') + 3} y={A - 10}>{mesCurto(m)}</text>
          )}
        </g>
      ))}
      <line className="eixo-linha" x1={mE} x2={L - mD} y1={A - mB} y2={A - mB} />
    </svg>
    <div className="legenda">
      <span><i style={{ background: 'var(--red)' }} />Realidade — abertos nos últimos 30 dias, dia a dia</span>
      <span><i style={{ background: 'var(--c-inst)' }} />Plano — Figura 1 do plano de preventiva</span>
      <span><i style={{ background: 'rgba(79,134,216,.3)' }} />Faixa do plano (−27% a −53%)</span>
    </div>{el}
  </>
}

/** Miniatura de uma loja: o plano ao fundo, a linha da loja por cima, o valor de hoje. */
export function MiniExpectativa(
  { loja, exp, ativa, aoClicar }:
  { loja: LojaDaExpectativa; exp: Expectativa; ativa: boolean; aoClicar: () => void },
) {
  // A MARGEM DA DIREITA CABE O RÓTULO, NÃO A LINHA
  //	O valor de hoje é escrito DEPOIS do último ponto. Com 34 de folga, a loja
  //	que passa de 100% perdia o "%" no corte do viewBox — e o número aparecia
  //	como "157'" na tela.
  const L = 300, A = 92, mE = 4, mD = 46, mT = 8, mB = 6
  const de = INICIO_CONTRATO, ate = exp.hoje
  const total = Math.max(1, diasEntre(de, ate))
  const x = (d: string) => mE + ((L - mE - mD) * diasEntre(de, d)) / total
  const maxY = Math.min(300, Math.max(140, ...loja.real.map(r => r.indice)) + 10)
  const y = (v: number) => mT + (A - mT - mB) * (1 - v / maxY)
  const diasPlano: string[] = []
  for (let d = de; d <= ate; d = diaMais(d, 4)) diasPlano.push(d)
  const plano = diasPlano.map(d => `${x(d)},${y(esperadoEm(d))}`).join(' ')
  const real = loja.real.map(r => `${x(r.dia)},${y(r.indice)}`).join(' ')
  const u = loja.ultimo
  const acima = !!u && u.indice > esperadoEm(exp.hoje)
  return (
    <button type="button" className={'mini-exp' + (ativa ? ' ativa' : '')} onClick={aoClicar}
      title={`${loja.nome} — patamar ${loja.patamar.toFixed(1)}/mês`}>
      <span className="me-nome">{loja.nome.replace(/^LOJA /i, '')}</span>
      <svg viewBox={`0 0 ${L} ${A}`} width="100%" style={{ display: 'block' }}>
        <line x1={mE} x2={L - mD} y1={y(100)} y2={y(100)} stroke="var(--line)" strokeWidth="1" strokeDasharray="3 3" />
        <polyline points={plano} fill="none" stroke="var(--c-inst)" strokeWidth="1.6" opacity=".75" />
        <polyline points={real} fill="none" stroke="var(--red)" strokeWidth="2" strokeLinejoin="round" />
        {u && <>
          <circle cx={x(u.dia)} cy={y(u.indice)} r="3" fill="var(--red)" />
          <text x={x(u.dia) + 5} y={y(u.indice) + 4}
            style={{ fill: acima ? 'var(--red)' : 'var(--ok)', fontSize: 13, fontWeight: 700, fontFamily: 'var(--f)' }}>
            {Math.round(u.indice)}%
          </text>
        </>}
      </svg>
      <span className="me-sub">{inteiro(loja.total)} chamados · patamar {loja.patamar.toFixed(1)}/mês</span>
    </button>
  )
}
