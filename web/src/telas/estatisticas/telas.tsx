// rev 1 — as nove telas
//
// A REGRA DE CADA TELA (pedido do dono em 30/08/2026: "muita informação, pouco fluido")
//
//	1. Uma FRASE em português, no topo, respondendo a pergunta da tela.
//	2. Três números, no máximo.
//	3. Um gráfico principal, grande.
//	4. Uma lista curta.
//	5. Tudo o que sobrar vai para "Mais detalhes", dobrado — quem quiser abre.
//
//	Sem jargão: "mediana" vira "metade dos chamados"; "vistoriado" ganha
//	"aprovado pelo cliente" ao lado.
//
// O QUE MUDOU DA VERSÃO OFFLINE
//
//	A "Fila de hoje" mostrava a DESCRIÇÃO de cada chamado antigo. Para desenhar
//	dez linhas, o retrato carregava a descrição dos 1.719 chamados — 14% do peso
//	da resposta. Decisão do dono em 30/08/2026: não carregar. As dez linhas
//	viraram link para Dados do Trílogo, que já mostra o chamado inteiro, e o
//	texto fica num lugar só.
import { useState } from 'react'
import type { Etapa } from '../../componentes/Painel'
import type { Resultado } from './calculo'
import { INICIO_CONTRATO, pct } from './calculo'
import { diasFmt, classeStatus, dtBR, inteiro, mesCurto, reais, reaisCurto } from './formato'
import {
  B, Barras, BarrasH, Bloco, CORES, ContaPill, Detalhes, Duas, Frase, GraficoExpectativa,
  Lista, MiniExpectativa, Numeros, Principal, Rosca, Linhas, type ItemDeNumero,
} from './graficos'

const n_ = inteiro
const semLoja = (s: string) => s.replace(/^LOJA /i, '')

/**
 * O endereço de um chamado em Dados do Trílogo.
 *
 * As rotas de `menu/arvore.ts` são escritas à mão e ESTÁVEIS justamente para um
 * link como este sobreviver à troca do título de um item — está no comentário
 * do topo daquele arquivo.
 */
const noTrilogo = (ticket: number) => `#/manutencao/contrato-sao-luiz/dados-trilogo/${ticket}`

// ---------------------------------------------------------------------------
// os ícones de cada barra
// ---------------------------------------------------------------------------

const traco = {
  fill: 'none', stroke: 'currentColor', strokeWidth: 1.9,
  strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const,
}

export const ICONE = {
  chamados: <svg viewBox="0 0 24 24" {...traco}><path d="M4 5h16v11H8l-4 4z" /><path d="M8 9h8M8 12h5" /></svg>,
  onde: <svg viewBox="0 0 24 24" {...traco}><path d="M3 10l9-6 9 6v10H3z" /><path d="M9 20v-6h6v6" /></svg>,
  tempo: <svg viewBox="0 0 24 24" {...traco}><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>,
  fila: <svg viewBox="0 0 24 24" {...traco}><path d="M4 6h16M4 12h10M4 18h6" /></svg>,
  custos: <svg viewBox="0 0 24 24" {...traco}><path d="M12 3v18M17 7.5c0-1.9-2.2-3-5-3s-5 1.1-5 3 2.2 2.5 5 2.5 5 .8 5 3-2.2 3.5-5 3.5-5-1.4-5-3.3" /></svg>,
  orcamentos: <svg viewBox="0 0 24 24" {...traco}><path d="M6 3h9l4 4v14H6z" /><path d="M14 3v5h5M9 13h6M9 17h6" /></svg>,
  faturamento: <svg viewBox="0 0 24 24" {...traco}><path d="M4 8l8-5 8 5v8l-8 5-8-5z" /><path d="M4 8l8 5 8-5M12 13v8" /></svg>,
  expectativa: <svg viewBox="0 0 24 24" {...traco}><path d="M3 20h18M4 16c3-2 5-9 8-9s5 5 8 3" /><path d="M4 12c4 0 6-6 9-6s4 8 7 8" strokeDasharray="2 3" /></svg>,
  balanco: <svg viewBox="0 0 24 24" {...traco}><path d="M12 3v18M4 7h16M6 7l-3 7a3 3 0 0 0 6 0zM18 7l-3 7a3 3 0 0 0 6 0z" /></svg>,
}

// ---------------------------------------------------------------------------
// as barras de cada nó
// ---------------------------------------------------------------------------
//
// O DONO PEDIU BARRAS COM NÚMERO, NÃO UM MENU (30/08/2026)
//	"Tem muita informação em uma tela só." O nó de entrada usa o MESMO
//	componente `Painel` do resto do FrotaHub, e a `chave` de cada barra é a
//	`rota` do item em `menu/arvore.ts` — é isso que faz o clique andar pela
//	navegação da casca, e não por um endereço próprio.

/** A descrição de cada barra: número, prévia e a tela que abre. */
export function cartoes(r: Resultado): { operacionais: Etapa[]; financeiras: Etapa[] } {
  const c = r.chamados, t = r.tempo, k = r.custos, f = r.fat
  const linha = (texto: string, fim: string) => ({ texto, fim })
  // Dinheiro na barra estreita: o número vai em MILHARES e o rótulo diz "mil
  // reais". Um valor cheio de seis dígitos não cabe numa coluna de 1/5 da tela,
  // e encolher a fonte até caber faz o número principal virar letra miúda.

  return {
    operacionais: [
      {
        chave: 'expectativa', titulo: 'Expectativa × Realidade',
        descricao: 'A curva do plano de preventiva contra os chamados abertos, dia a dia',
        icone: ICONE.expectativa,
        numero: r.exp.ultimo ? Math.round(r.exp.ultimo.indice) : null,
        rotulo: 'do patamar, hoje', previaTitulo: 'Últimos 30 dias',
        previa: [
          linha('Chamados abertos', r.exp.ultimo ? n_(r.exp.ultimo.soma) : '—'),
          linha('O plano esperava', Math.round(r.exp.esperadoHoje) + '%'),
          linha('Patamar (100%)', n_(r.exp.patamar) + '/mês'),
          linha('Queda até mai/28', '−40%'),
        ],
        rodape: r.exp.ultimo
          ? (r.exp.ultimo.indice <= r.exp.esperadoHoje
            ? 'dentro do esperado'
            : `${Math.round(r.exp.ultimo.indice - r.exp.esperadoHoje)} pontos acima do plano`)
          : 'ainda sem 30 dias',
        faixa: r.exp.ultimo && r.exp.ultimo.indice > r.exp.esperadoHoje + 10 ? 'var(--red-lift)' : undefined,
      },
      {
        chave: 'chamados', titulo: 'Chamados',
        descricao: 'Abertos, executados e vistoriados no período', icone: ICONE.chamados,
        numero: c.noPeriodo, rotulo: 'abertos no período', previaTitulo: 'Situação atual',
        previa: c.porStatus.slice(0, 5).map(([s, n]) => linha(s, n_(n))),
        rodape: `${n_(c.executados)} executados`,
      },
      {
        chave: 'onde', titulo: 'Onde', descricao: 'Lojas e locais', icone: ICONE.onde,
        numero: c.lojasComChamado, rotulo: 'lojas com chamado', previaTitulo: 'Mais chamados',
        previa: c.porLoja.slice(0, 5).map(([l, n]) => linha(semLoja(l), n_(n))),
        rodape: `${c.porLocal[0]?.[0] ?? '—'} é o local mais comum`,
      },
      {
        chave: 'tempo', titulo: 'Tempo de atendimento',
        descricao: 'Da abertura à execução e à vistoria', icone: ICONE.tempo,
        numero: t.medianaExec == null ? null : Math.round(t.medianaExec * 10) / 10,
        rotulo: 'dias até executar (metade dos chamados)', previaTitulo: 'Por conta',
        previa: t.porConta.map(x => linha(x.rotulo, diasFmt(x.mediana) + ' d')),
        rodape: `vistoria em ${diasFmt(t.medianaVist)} d`,
      },
      {
        chave: 'fila', titulo: 'Fila de hoje', descricao: 'O que está em aberto agora',
        icone: ICONE.fila,
        numero: c.abertosAgora, rotulo: 'em aberto agora', previaTitulo: 'Os mais antigos',
        previa: t.maisAntigos.slice(0, 5).map(x =>
          linha(`${x.numero} · ${semLoja(x.loja)}`, Math.floor(x.idade) + ' d')),
        rodape: `${n_(t.atrasados)} com prazo vencido`,
        faixa: t.atrasados ? 'var(--red-lift)' : undefined,
      },
    ],
    financeiras: [
      {
        chave: 'custos', titulo: 'Custos no Trílogo', descricao: 'O que foi lançado nos chamados',
        icone: ICONE.custos,
        numero: Math.round(k.valorLancado / 1000), rotulo: 'mil reais lançados',
        previaTitulo: 'Lojas que mais custaram',
        previa: k.custoPorLoja.slice(0, 5).map(([l, v]) => linha(semLoja(l), reaisCurto(v))),
        rodape: `${n_(k.n)} custos · ${k.pctComCusto ?? '—'}% dos executados`,
      },
      {
        chave: 'orcamentos', titulo: 'Orçamentos',
        descricao: 'Gerados, lançados, bloqueados e as notas pendentes', icone: ICONE.orcamentos,
        numero: k.gerados, rotulo: 'a lançar', previaTitulo: 'Pendências',
        previa: [
          linha('Com bloqueio', n_(k.bloqueados)),
          linha('Notas sem ticket', n_(k.semTicket)),
          linha('Ticket não associado', n_(k.ticketSolto)),
          linha('Lançados no período', n_(k.lancados)),
          linha('Ajustados pelo teto', n_(k.noTeto)),
        ],
        rodape: `margem realizada ${k.margem ?? '—'}%`,
      },
      {
        chave: 'faturamento', titulo: 'Faturamento ao cliente',
        descricao: 'Ciclos, faturas e o que voltou', icone: ICONE.faturamento,
        numero: Math.round(f.totalFaturado / 1000),
        rotulo: 'mil reais faturados', previaTitulo: 'Ciclos',
        previa: f.ciclosResumo.slice(-5).map(x =>
          linha(mesCurto(x.competencia) + ' · ' + x.notas + ' notas', reaisCurto(x.faturado))),
        rodape: `${n_(f.naoFaturado)} lançados sem faturar`,
      },
      {
        chave: 'balanco', titulo: 'Balanço', descricao: 'A pagar contra a receber, e a margem',
        icone: ICONE.balanco,
        numero: Math.round(f.margemPeriodo / 1000),
        rotulo: 'mil reais de margem bruta', previaTitulo: 'Por mês',
        previa: f.balancoPorMes.slice(-5).map(x => linha(x.rotulo, reaisCurto(x.margem))),
        rodape: `${f.pagarPeriodo ? Math.round((f.margemPeriodo / f.pagarPeriodo) * 100) : '—'}% sobre a nota`,
        faixa: f.margemPeriodo < 0 ? 'var(--red-lift)' : undefined,
      },
    ],
  }
}

// ---------------------------------------------------------------------------
// operação
// ---------------------------------------------------------------------------

export function TelaChamados({ r, semConta }: { r: Resultado; semConta: boolean }) {
  const c = r.chamados
  const inst = c.porMes.reduce((a, m) => a + m.instalacoes, 0)
  const civ = c.porMes.reduce((a, m) => a + m.civil, 0)
  const legivel = c.porStatus.map(([s, n]) =>
    [s === 'Vistoriado' ? 'Vistoriado (aprovado pelo cliente)' : s, n] as [string, number])
  const numeros: ItemDeNumero[] = [
    ['abertos no período', n_(c.noPeriodo)],
    ['executados', n_(c.executados), 'pela linha do tempo do Trílogo'],
    ['dentro do prazo', c.pctPrazo == null ? '—' : c.pctPrazo + '%', `de ${n_(c.comPrazo)} com prazo`,
      c.pctPrazo == null ? '' : c.pctPrazo >= 80 ? 'ok' : c.pctPrazo >= 60 ? 'warn' : 'err'],
  ]
  return <>
    <Frase>
      No período foram abertos <B>{n_(c.noPeriodo)} chamados</B>
      {semConta && <> — {n_(inst)} de Instalações e {n_(civ)} de Civil</>}. <B>{n_(c.executados)}</B> foram
      executados{c.pctPrazo != null && <>, e <B>{c.pctPrazo}%</B> deles dentro do prazo</>}.
    </Frase>
    <Numeros itens={numeros} />
    <Duas>
      <Principal t="Chamados abertos por mês" sub="quantos entraram em cada mês, por conta">
        <Barras dados={c.porMes} series={['instalacoes', 'civil']} rotulos={['Instalações', 'Civil']} altura={290} />
      </Principal>
      <Lista t="Como eles estão hoje" sub="a situação atual de cada um">
        <Rosca itens={legivel} />
      </Lista>
    </Duas>
    <Detalhes>
      <Bloco t="Abertos × executados" sub="por mês — quando a linha de executados fica abaixo, a fila cresce" c="c8">
        <Linhas dados={c.porMesExec} series={['abertos', 'executados']} rotulos={['Abertos', 'Executados']} />
      </Bloco>
      <Bloco t="Por prioridade" c="c4"><Rosca itens={c.porPrioridade} /></Bloco>
      <Bloco t="Vistoriados no período" sub="aprovados pelo cliente" c="c4">
        <div className="num solto"><span className="num-v">{n_(c.vistoriados)}</span></div>
      </Bloco>
    </Detalhes>
  </>
}

export function TelaOnde({ r }: { r: Resultado }) {
  const c = r.chamados
  const [l1, n1] = c.porLoja[0] || ['—', 0]
  const [p1] = c.porLocal[0] || ['—']
  return <>
    <Frase>
      A loja com mais chamados foi <B>{l1}</B> ({n_(n1)}, {pct(n1, c.noPeriodo)}% do total).
      O lugar mais comum dentro das lojas foi <B>{p1}</B>.
    </Frase>
    <Duas>
      <Principal t="Lojas com mais chamados" sub="as 10 primeiras no período">
        <BarrasH itens={c.porLoja.slice(0, 10)} total={c.noPeriodo} />
      </Principal>
      <Lista t="Onde, dentro da loja" sub="os 8 lugares mais frequentes">
        <BarrasH itens={c.porLocal.slice(0, 8)} cor={CORES[2]} total={c.noPeriodo} />
      </Lista>
    </Duas>
  </>
}

export function TelaTempo({ r }: { r: Resultado }) {
  const t = r.tempo
  const numeros: ItemDeNumero[] = [
    ['dias até resolver', diasFmt(t.medianaExec), 'metade dos chamados, em até'],
    ['resolvidos em menos de 7 dias', t.pctAte7 == null ? '—' : t.pctAte7 + '%',
      `de ${n_(t.n)} executados`, (t.pctAte7 ?? 0) >= 70 ? 'ok' : 'warn'],
    ['dias até o cliente aprovar', diasFmt(t.medianaExecAteVist), 'do executado ao vistoriado'],
  ]
  return <>
    <Frase>
      Metade dos chamados é resolvida em até <B>{diasFmt(t.medianaExec)} dias</B>
      {t.pctAte7 != null && <>, e <B>{t.pctAte7}%</B> em menos de uma semana</>}. Depois de executado,
      o cliente leva em torno de <B>{diasFmt(t.medianaExecAteVist)} dia(s)</B> para aprovar (vistoriar).
    </Frase>
    <Numeros itens={numeros} />
    <Duas>
      <Principal t="Em quantos dias os chamados foram resolvidos"
        sub="cada barra é uma faixa de dias; quanto mais à esquerda, melhor">
        <Barras dados={t.histograma} series={['valor']} rotulos={['Chamados']} altura={290} />
      </Principal>
      <Lista t="Lojas que demoram mais" sub="dias até resolver, para a metade dos chamados">
        <div className="painel"><table>
          <thead><tr><th>Loja</th><th className="n">Chamados</th><th className="n">Dias</th></tr></thead>
          <tbody>{t.lojasLentas.slice(0, 8).map(x => (
            <tr key={x.loja}>
              <td className="forte">{x.loja}</td><td className="n">{n_(x.n)}</td>
              <td className="n forte">{diasFmt(x.mediana)}</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Lista>
    </Duas>
    <Detalhes>
      <Bloco t="Por conta" c="c6">
        <div className="painel"><table>
          <thead><tr><th>Conta</th><th className="n">Chamados</th><th className="n">Metade em até</th><th className="n">Média</th></tr></thead>
          <tbody>{t.porConta.map(x => (
            <tr key={x.rotulo}>
              <td className="forte">{x.rotulo}</td><td className="n">{n_(x.n)}</td>
              <td className="n forte">{diasFmt(x.mediana)} d</td><td className="n">{diasFmt(x.media)} d</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Bloco>
      <Bloco t="Outros tempos" sub="metade dos chamados, em até" c="c6">
        <div className="painel"><table><tbody>
          <tr><td>Da abertura à aprovação do cliente</td><td className="n forte">{diasFmt(t.medianaVist)} d</td><td className="n">média {diasFmt(t.mediaVist)}</td></tr>
          <tr><td>Do "em execução" ao "executado"</td><td className="n forte">{diasFmt(t.medianaExecucao)} d</td><td className="n">média {diasFmt(t.mediaExecucao)}</td></tr>
          <tr><td>Da abertura à execução (média)</td><td className="n forte">{diasFmt(t.mediaExec)} d</td><td className="n">{n_(t.n)} chamados</td></tr>
        </tbody></table></div>
      </Bloco>
    </Detalhes>
  </>
}

export function TelaFila({ r }: { r: Resultado }) {
  const t = r.tempo, c = r.chamados
  const mais30 = t.backlog[5]?.valor ?? 0
  const numeros: ItemDeNumero[] = [
    ['em aberto agora', n_(c.abertosAgora), `${n_(c.emExecucao)} em execução`],
    ['com prazo vencido', n_(t.atrasados), 'hoje', t.atrasados ? 'err' : 'ok'],
    ['esperando há mais de 30 dias', n_(mais30), '', mais30 ? 'warn' : 'ok'],
  ]
  return <>
    <Frase>
      Hoje há <B>{n_(c.abertosAgora)} chamados em aberto</B> ({n_(c.emExecucao)} já em execução).
      <B> {n_(t.atrasados)}</B> estão com o prazo vencido e <B>{n_(mais30)}</B> esperam há mais de 30 dias.
    </Frase>
    <Numeros itens={numeros} />
    <Duas>
      <Principal t="Há quanto tempo a fila espera" sub="quantos chamados em aberto há em cada faixa de dias">
        <Barras dados={t.backlog} series={['valor']} rotulos={['Em aberto']} altura={290} />
      </Principal>
      <Lista t="Os mais antigos" sub="os 8 que esperam há mais tempo">
        <div className="painel"><table>
          <thead><tr><th>Ticket</th><th>Loja</th><th className="n">Dias</th></tr></thead>
          <tbody>{t.maisAntigos.slice(0, 8).map(x => (
            <tr key={x.numero}>
              <td className="forte"><a href={noTrilogo(x.numero)}>{x.numero}</a></td>
              <td>{x.loja}</td><td className="n forte">{Math.floor(x.idade)}</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Lista>
    </Duas>
    <Detalhes>
      {/* O TEXTO DO CHAMADO NÃO MORA AQUI
          Ele mora em Dados do Trílogo, e o número leva até lá. Carregar a
          descrição dos 1.719 chamados para escrever dez linhas era 14% do peso
          da resposta (decisão do dono, 30/08/2026). */}
      <Bloco t="Os dez mais antigos" sub="clique no ticket para abrir o chamado em Dados do Trílogo" c="c12">
        <div className="painel"><table>
          <thead><tr><th>Ticket</th><th>Loja</th><th>Conta</th><th>Status</th><th>Aberto em</th><th className="n">Dias</th></tr></thead>
          <tbody>{t.maisAntigos.map(x => (
            <tr key={x.numero}>
              <td className="forte"><a href={noTrilogo(x.numero)}>{x.numero}</a></td>
              <td>{x.loja}</td>
              <td><ContaPill conta={x.conta} /></td>
              <td><span className={'pino ' + classeStatus(x.status)}>{x.status}</span></td>
              <td>{dtBR(x.criado_em)}</td>
              <td className="n forte">{Math.floor(x.idade)}</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Bloco>
    </Detalhes>
  </>
}

// ---------------------------------------------------------------------------
// dinheiro
// ---------------------------------------------------------------------------

export function TelaCustos({ r }: { r: Resultado }) {
  const k = r.custos
  const [l1, v1] = k.custoPorLoja[0] || ['—', 0]
  const numeros: ItemDeNumero[] = [
    ['lançado no Trílogo', reaisCurto(k.valorLancado),
      k.semData ? `${n_(k.semData)} custos datados pela execução` : ''],
    ['executados com custo', k.pctComCusto == null ? '—' : k.pctComCusto + '%',
      `${n_(k.fechadosComCusto)} de ${n_(k.fechados)}`, (k.pctComCusto ?? 0) >= 70 ? 'ok' : 'warn'],
    ['custo médio por lançamento', reais(k.ticketMedio)],
  ]
  return <>
    <Frase>
      No período foram lançados <B>{reais(k.valorLancado)}</B> em custos no Trílogo, em <B>{n_(k.n)}</B> lançamentos
      (em média {reais(k.ticketMedio)} cada). A loja que mais custou foi <B>{l1}</B> ({reaisCurto(v1)}).
    </Frase>
    <Numeros itens={numeros} />
    <Duas>
      <Principal t="Custo lançado por mês" sub="em reais, por conta">
        <Barras dados={k.custoPorMes} series={['instalacoes', 'civil']}
          rotulos={['Instalações', 'Civil']} dinheiro altura={290} />
      </Principal>
      <Lista t="Lojas que mais custaram" sub="as 8 primeiras">
        <BarrasH itens={k.custoPorLoja.slice(0, 8)} dinheiro total={k.valorLancado} />
      </Lista>
    </Duas>
    <Detalhes>
      <Bloco t="Por tipo de custo" c="c6"><Rosca itens={k.custoPorTipo} dinheiro /></Bloco>
      <Bloco t="Todas as lojas" c="c6"><BarrasH itens={k.custoPorLoja} dinheiro total={k.valorLancado} /></Bloco>
    </Detalhes>
  </>
}

export function TelaOrcamentos({ r }: { r: Resultado }) {
  const k = r.custos
  const pend = k.semTicket + k.ticketSolto
  const numeros: ItemDeNumero[] = [
    ['a lançar', n_(k.gerados), reais(k.valorGerados)],
    ['travados', n_(k.bloqueados), 'não puderam ser lançados', k.bloqueados ? 'warn' : 'ok'],
    ['lançados no período', n_(k.lancados), reais(k.valorLancados)],
  ]
  return <>
    <Frase>
      <B>{n_(k.gerados)} orçamentos</B> estão esperando lançamento no Trílogo ({reais(k.valorGerados)})
      {k.bloqueados ? <>, e <B>{n_(k.bloqueados)}</B> deles estão travados</> : null}. No período foram
      lançados <B>{n_(k.lancados)}</B> ({reais(k.valorLancados)}).
      {pend ? <> Há <B>{n_(pend)}</B> nota(s) esperando alguém dizer o ticket.</> : null}
    </Frase>
    <Numeros itens={numeros} />
    <Duas>
      <Principal t="Orçamentos gerados e lançados" sub="por mês">
        <Barras dados={k.orcPorMes} series={['gerados', 'lancados']}
          rotulos={['Gerados', 'Lançados']} empilhar={false} altura={290} />
      </Principal>
      <Lista t="Por que travou" sub="o motivo de cada orçamento que não foi lançado">
        {k.porBloqueio.length
          ? <BarrasH itens={k.porBloqueio} cor={CORES[5]} />
          : <div className="vazio">Nenhum travado.</div>}
        <div className="mini-nums">
          <span><b>{n_(k.semTicket)}</b> notas sem ticket</span>
          <span><b>{n_(k.ticketSolto)}</b> com ticket não associado</span>
          <span><b>{n_(k.docsBloqueados)}</b> bloqueadas</span>
        </div>
      </Lista>
    </Duas>
    <Detalhes>
      <Bloco t="Margem realizada" sub="o que a nota custou e o que foi cobrado" c="c4">
        <div className="num solto">
          <span className="num-v">{k.margem == null ? '—' : k.margem + '%'}</span>
          <span className="num-s">nota {reaisCurto(k.valorNota)} → cobrado {reaisCurto(k.valorCobrado)}</span>
        </div>
      </Bloco>
      <Bloco t="Ajustados pelo teto" sub={k.teto ? `teto de ${reais(k.teto)} por ticket` : ''} c="c4">
        <div className="num solto"><span className="num-v">{n_(k.noTeto)}</span></div>
      </Bloco>
      <Bloco t="Por situação" c="c4"><Rosca itens={k.orcPorStatus} /></Bloco>
      <Bloco t="Notas no sistema" c="c4">
        <div className="num solto">
          <span className="num-v">{n_(k.docs)}</span>
          <span className="num-s">{k.docPorFila.map(([a, b]) => `${a}: ${b}`).join(' · ')}</span>
        </div>
      </Bloco>
    </Detalhes>
  </>
}

export function TelaFaturamento({ r }: { r: Resultado }) {
  const f = r.fat
  const numeros: ItemDeNumero[] = [
    ['faturado ao cliente', reaisCurto(f.totalFaturado), `${n_(f.ciclosResumo.length)} ciclo(s)`],
    ['recebido', reaisCurto(f.totalRecebido), `em aberto ${reaisCurto(f.emAberto)}`,
      f.emAberto > 0 ? 'warn' : 'ok'],
    ['esperando o próximo ciclo', n_(f.naoFaturado), reais(f.valorNaoFaturado),
      f.naoFaturado ? 'warn' : 'ok'],
  ]
  return <>
    <Frase>
      Já foram faturados <B>{reais(f.totalFaturado)}</B> ao cliente em {n_(f.ciclosResumo.length)} ciclo(s);
      <B> {reais(f.emAberto)}</B> ainda não voltaram. <B>{n_(f.naoFaturado)}</B> orçamentos
      lançados ({reais(f.valorNaoFaturado)}) esperam o próximo ciclo.
    </Frase>
    <Numeros itens={numeros} />
    <Principal t="Ciclos de faturamento" sub="o que foi cobrado do cliente em cada mês e o que já voltou">
      <div className="painel"><table>
        <thead><tr><th>Mês</th><th>Fechado em</th><th className="n">Notas</th><th className="n">Faturado</th><th className="n">Recebido</th><th className="n">Em aberto</th></tr></thead>
        <tbody>{f.ciclosResumo.length ? f.ciclosResumo.map(c => (
          <tr key={c.id}>
            <td className="forte">{mesCurto(c.competencia)}</td><td>{dtBR(c.fechado_em)}</td>
            <td className="n">{c.notas}</td><td className="n forte">{reais(c.faturado)}</td>
            <td className="n">{reais(c.recebido)}</td><td className="n">{reais(c.emAberto)}</td>
          </tr>
        )) : <tr><td colSpan={6} className="vazio">Nenhum ciclo fechado.</td></tr>}</tbody>
      </table></div>
    </Principal>
    <Detalhes>
      <Bloco t="Faturas por loja" sub="uma nota por loja e conta em cada ciclo — o PCO do cliente" c="c12">
        <div className="painel" style={{ maxHeight: 480 }}><table>
          <thead><tr><th>Mês</th><th>Loja</th><th>Conta</th><th>PCO</th><th>NF</th><th className="n">Faturado</th><th className="n">Recebido</th></tr></thead>
          <tbody>{f.faturasLoja.map(x => (
            <tr key={x.id}>
              <td>{mesCurto(x.ciclo ?? null)}</td><td className="forte">{x.loja}</td>
              <td><ContaPill conta={x.conta} /></td>
              <td>{x.pco_numero || '—'}</td><td>{x.nf_numero || '—'}</td>
              <td className="n forte">{reais(x.valor_faturado)}</td>
              <td className="n">{x.valor_recebido == null ? '—' : reais(x.valor_recebido)}</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Bloco>
    </Detalhes>
  </>
}

export function TelaBalanco({ r }: { r: Resultado }) {
  const f = r.fat
  const pctM = f.pagarPeriodo ? Math.round((f.margemPeriodo / f.pagarPeriodo) * 100) : null
  const numeros: ItemDeNumero[] = [
    ['a pagar ao fornecedor', reaisCurto(f.pagarPeriodo), 'valor das notas'],
    ['a receber do cliente', reaisCurto(f.receberPeriodo), 'valor cobrado'],
    ['margem bruta', reaisCurto(f.margemPeriodo), pctM != null ? pctM + '% sobre a nota' : '',
      f.margemPeriodo >= 0 ? 'ok' : 'err'],
  ]
  return <>
    <Frase>
      No período, as notas do fornecedor somaram <B>{reais(f.pagarPeriodo)}</B> e o cobrado do
      cliente, <B>{reais(f.receberPeriodo)}</B>. A diferença — a margem bruta — foi
      de <B>{reais(f.margemPeriodo)}</B>{pctM != null && <> ({pctM}% sobre a nota)</>}.
    </Frase>
    <Numeros itens={numeros} />
    <Duas>
      <Principal t="A pagar e a receber, por mês" sub="lado a lado; a diferença é a margem">
        <Barras dados={f.balancoPorMes} series={['pagar', 'receber']}
          rotulos={['A pagar', 'A receber']} empilhar={false} dinheiro altura={290} />
      </Principal>
      <Lista t="Margem por loja" sub="as 8 lojas com mais margem no período">
        <div className="painel"><table>
          <thead><tr><th>Loja</th><th className="n">A receber</th><th className="n">Margem</th></tr></thead>
          <tbody>{[...f.balancoLoja].sort((a, b) => b.margem - a.margem).slice(0, 8).map(x => (
            <tr key={x.loja}>
              <td className="forte">{x.loja}</td><td className="n">{reais(x.receber)}</td>
              <td className="n forte">{reais(x.margem)}</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Lista>
    </Duas>
    <Detalhes>
      <Bloco t="Margem por mês" c="c6">
        <Linhas dados={f.balancoPorMes} series={['margem']} rotulos={['Margem bruta']} dinheiro largura={700} />
      </Bloco>
      <Bloco t="Fornecedor" sub="o que ainda está em aberto e o que já foi pago" c="c6">
        <div className="num solto">
          <span className="num-v">{reaisCurto(f.aPagarAberto)}</span>
          <span className="num-r">em aberto</span>
          <span className="num-s">já pago {reais(f.pagoTotal)}</span>
        </div>
      </Bloco>
      <Bloco t="Balanço completo por loja" c="c12">
        <div className="painel"><table>
          <thead><tr><th>Loja</th><th className="n">Orç.</th><th className="n">A pagar</th><th className="n">A receber</th><th className="n">Margem</th></tr></thead>
          <tbody>{f.balancoLoja.map(x => (
            <tr key={x.loja}>
              <td className="forte">{x.loja}</td><td className="n">{x.n}</td>
              <td className="n">{reais(x.pagar)}</td><td className="n forte">{reais(x.receber)}</td>
              <td className="n">{reais(x.margem)}</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Bloco>
    </Detalhes>
  </>
}

// ---------------------------------------------------------------------------
// expectativa × realidade
// ---------------------------------------------------------------------------

export function TelaExpectativa({ r }: { r: Resultado }) {
  const e = r.exp
  const [horizonte, setHorizonte] = useState('ciclo')
  const [lojaSel, setLojaSel] = useState('') // '' = geral (a rede inteira)
  const loja = lojaSel === '' ? null : e.lojas.find(l => String(l.unidade) === lojaSel) ?? null
  const u = loja ? loja.ultimo : e.ultimo
  const dif = u ? Math.round(u.indice - e.esperadoHoje) : null
  const dt = (s: string) => s.split('-').reverse().join('/')
  const numeros: ItemDeNumero[] = [
    ['do patamar, hoje', u ? Math.round(u.indice) + '%' : '—', u ? `${n_(u.soma)} abertos em 30 dias` : ''],
    ['o plano esperava', Math.round(e.esperadoHoje) + '%', 'para ' + dt(e.hoje)],
    ['diferença', dif == null ? '—' : (dif > 0 ? '+' : '') + dif + ' pontos',
      dif == null ? '' : dif <= 0 ? 'dentro do esperado' : 'acima do plano',
      dif == null ? '' : dif <= 0 ? 'ok' : dif <= 10 ? 'warn' : 'err'],
  ]
  return <>
    <Frase>{u ? <>
      {loja ? <><B>{loja.nome}</B>: nos</> : 'Nos'} últimos 30 dias foram
      abertos <B>{n_(u.soma)} chamados</B> — <B>{Math.round(u.indice)}%</B> do patamar
      de {loja ? loja.patamar.toFixed(1) : n_(e.patamar)} por mês. Para hoje, o plano
      esperava <B>{Math.round(e.esperadoHoje)}%</B>: a realidade
      está <B>{Math.abs(dif ?? 0)} ponto(s) {(dif ?? 0) > 0 ? 'acima' : 'abaixo'}</B> do plano.
      A queda de verdade só começa em setembro, quando entra a preventiva de rotina.
    </> : <>Ainda não há 30 dias de contrato para medir a realidade contra o plano.</>}</Frase>
    <Numeros itens={numeros} />
    <section className="bloco principal">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap' }}>
        <div>
          <h3>Expectativa × Realidade</h3>
          <div className="sub">
            o plano ao fundo, a realidade por cima; 100% = {loja
              ? `${loja.patamar.toFixed(1)} chamados por mês (a fatia da loja nos ${n_(e.patamar)})`
              : `${n_(e.patamar)} chamados por mês`}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <select className="sel" value={lojaSel} onChange={ev => setLojaSel(ev.target.value)}>
            <option value="">Geral — toda a rede</option>
            {e.lojas.map(l => <option key={l.unidade} value={l.unidade}>{l.nome}</option>)}
          </select>
          <div className="pilulas">
            {([['ciclo', 'Ciclo completo (até mai/28)'], ['hoje', 'Até hoje']] as const).map(([k, t]) => (
              <button key={k} type="button" className={horizonte === k ? 'ativo' : ''}
                onClick={() => setHorizonte(k)}>{t}</button>
            ))}
          </div>
        </div>
      </div>
      <GraficoExpectativa exp={e} horizonte={horizonte}
        serie={loja ? loja.real : null} patamar={loja ? loja.patamar : null} />
    </section>
    <section className="bloco" style={{ marginBottom: 12 }}>
      <h3>As {e.lojas.length} lojas</h3>
      <div className="sub">
        cada miniatura é o mesmo gráfico da loja: plano ao fundo, realidade por cima, valor de hoje.
        O patamar de cada loja é a fatia dela nos {n_(e.patamar)}/mês, pela participação nas aberturas
        do contrato. Clique para abrir no gráfico grande.
      </div>
      <div className="mini-grade">
        {e.lojas.map(l => (
          <MiniExpectativa key={l.unidade} loja={l} exp={e} ativa={String(l.unidade) === lojaSel}
            aoClicar={() => { setLojaSel(String(l.unidade)); window.scrollTo({ top: 0, behavior: 'smooth' }) }} />
        ))}
      </div>
    </section>
    <Detalhes>
      <Bloco t="Mês a mês (geral)" sub="chamados abertos no mês contra o que o plano esperava no fim dele" c="c8">
        <div className="painel"><table>
          <thead><tr><th>Mês</th><th className="n">Abertos</th><th className="n">% do patamar</th><th className="n">Plano</th><th></th></tr></thead>
          <tbody>{e.porMes.map(m => (
            <tr key={m.mes}>
              <td className="forte">{mesCurto(m.mes)}</td>
              <td className="n">{n_(m.abertos)}{m.parcial && <span className="tri-fraco"> (até {dt(e.hoje)})</span>}</td>
              <td className="n forte">
                {Math.round(m.parcial ? (m.projetado / e.patamar) * 100 : m.indice)}%
                {m.parcial && <span className="tri-fraco"> proj.</span>}
              </td>
              <td className="n">{Math.round(m.esperado)}%</td>
              <td>{m.parcial ? <span className="pino st-outro">mês em andamento</span> : null}</td>
            </tr>
          ))}</tbody>
        </table></div>
      </Bloco>
      <Bloco t="Como é feita a conta" c="c4">
        <div className="explica">
          <p><b>Patamar.</b> 100% = {n_(e.patamar)} chamados por mês, o número do plano.</p>
          <p><b>Realidade.</b> Chamado <b>aberto</b> = aberto pela primeira vez (retrabalho não conta duas
            vezes). Cada ponto do dia soma os abertos nos 30 dias anteriores e divide pelo patamar — é a
            demanda que o treinamento e a preventiva querem reduzir. Toda a rede, as duas contas, só as
            unidades do plano.</p>
          <p><b>Plano.</b> A curva da Figura 1: 100 em julho, pico de 120 na 2ª quinzena de agosto
            (contingência), e de setembro em diante a queda até −40% em mai/2028, na faixa de −27% a −53%.</p>
          <p className="tri-fraco">{n_(e.totalContrato)} abertos desde {dt(INICIO_CONTRATO)} ·
            {' '}{e.diasDeContrato} dias de contrato · retrato de {dt(e.hoje)}</p>
        </div>
      </Bloco>
    </Detalhes>
  </>
}
