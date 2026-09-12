// rev 3 — a tabela de Ordens de Compra, compartilhada
//
// EXTRAÍDA DE `InserirOC.tsx` (rev 2) QUANDO "OCs Inseridas" GANHOU TELA
// PRÓPRIA
//
// FORMATO TRÍLOGO (11/09/2026)
//   Colunas separadas (número, obra, valor, data) no mesmo desenho compacto
//   de `servicos.css` / DadosTrilogo — não filename + dica numa célula só.
//
// REJEITADAS (11/09/2026)
//   Seis colunas fixas: O.C., obra/centro, faturamento, valor, inserida em,
//   motivo simplificado em vermelho, e a ação (reparar OU ver+substituir).
//
// FATURAMENTO GANHA COLUNA PRÓPRIA (11/09/2026, mais tarde)
//   O CNPJ de faturamento sozinho parou de identificar "de qual obra é essa
//   OC" — passou a existir o caso de uma obra nova faturar temporariamente
//   pelo CNPJ da matriz, então duas obras diferentes podem aparecer com o
//   MESMO CNPJ. Centro de custo continua sendo a coluna que diferencia; esta
//   é só para conferir o CNPJ sem abrir a OC.
//
// VER + SUBSTITUIR, NÃO REPARAR (11/09/2026)
//   Faturamento errado deixou de ser corrigível por dentro do sistema — o
//   CNPJ pertence ao Obra Prima, e um remendo só do nosso lado deixa o
//   registro de lá errado pra sempre. Uma OC rejeitada por faturamento (
//   mesmo que também tenha outro motivo junto) mostra "ver" + "substituir"
//   em vez de "REPARAR": a pessoa corrige no Obra Prima, baixa o PDF novo e
//   substitui — a OC velha some, a nova entra do zero na fila de leitura.
import { useEffect, useLayoutEffect, useRef } from 'react'
import { ajustarCelulas } from '../trilogo/encolher'
import { quando } from '../trilogo/tipos'
import {
  emReais, formatarCNPJ, motivoRejeicaoSimplificado, precisaSubstituir,
  type OrdemDeCompra, type VistaDasOrdens,
} from './tipos'

const NOME_STATUS: Record<OrdemDeCompra['status'], string> = {
  inserido: 'na fila',
  lendo: 'lendo…',
  lido: 'lida',
  falhou: 'falhou',
}

const CLASSE_STATUS: Record<OrdemDeCompra['status'], string> = {
  inserido: 'pino-off',
  lendo: 'pino-warn',
  lido: 'pino-ok',
  falhou: 'pino-err',
}

export function TabelaDeOrdens({
  ordens, vista, onVer, onReparar, onSubstituir, onExcluir, onLer, lendoId, loteRodando, onEnviar, enviandoId,
}: {
  ordens: OrdemDeCompra[]
  vista?: VistaDasOrdens
  onVer: (o: OrdemDeCompra) => void
  /** Só na vista rejeitadas — abre a OC em tela inteira para correção. */
  onReparar?: (o: OrdemDeCompra) => void
  /** Só na vista rejeitadas, só quando o motivo é de faturamento. */
  onSubstituir?: (o: OrdemDeCompra) => void
  /** Ausente = sem botão de excluir (Enviados não tem — já foi pro cliente). */
  onExcluir?: (o: OrdemDeCompra) => void
  /** Ausente = tabela só de consulta, sem botão de ler. */
  onLer?: (id: string) => void
  lendoId?: string | null
  loteRodando?: boolean
  /** Ausente = sem botão de enviar (Enviados, Processadas, Rejeitadas). */
  onEnviar?: (id: string) => void
  enviandoId?: string | null
}) {
  const rejeitadas = vista === 'rejeitadas'
  const corpo = useRef<HTMLTableSectionElement>(null)
  useLayoutEffect(() => {
    if (!rejeitadas) ajustarCelulas(corpo.current)
  }, [ordens, rejeitadas])
  useEffect(() => {
    if (rejeitadas) return
    let t: number | undefined
    function aoRedimensionar() {
      window.clearTimeout(t)
      t = window.setTimeout(() => ajustarCelulas(corpo.current), 120)
    }
    window.addEventListener('resize', aoRedimensionar)
    return () => { window.clearTimeout(t); window.removeEventListener('resize', aoRedimensionar) }
  }, [rejeitadas])

  return (
    <div className="tabela-rolo tri-painel">
      <table className={'tabela adm-tabela' + (rejeitadas ? ' adm-tabela--rejeitadas' : '')}>
        <thead>
          {rejeitadas ? (
            <tr>
              <th className="c-oc">O.C.</th>
              <th className="c-obra">Obra/centro</th>
              <th className="c-fat">Faturamento</th>
              <th className="c-valor">Valor</th>
              <th className="c-data">Inserida em</th>
              <th className="c-motivo">Motivo da rejeição</th>
              <th className="c-reparar"></th>
            </tr>
          ) : (
            <tr>
              <th className="c-oc">OC</th>
              <th className="c-obra">Obra / centro</th>
              <th className="c-fat">Faturamento</th>
              <th className="c-valor">Valor (R$)</th>
              <th className="c-data">Inserida em</th>
              <th className="c-leitura">Leitura</th>
              <th className="acoes-col"></th>
            </tr>
          )}
        </thead>
        <tbody ref={corpo}>
          {ordens.map(o => rejeitadas ? (
            <tr key={o.id}>
              <td className="c-oc">
                <span className="tri-num" title={o.nome_arquivo}>{rotuloOC(o)}</span>
              </td>
              <td className="c-obra">
                <span title={tituloObra(o)}>{nomeObraSemCidade(o)}</span>
              </td>
              <td className="c-fat">
                <span title={faturamentoDaLinha(o)}>{faturamentoDaLinha(o)}</span>
              </td>
              <td className="c-valor">{o.total != null ? emReais(o.total) : '—'}</td>
              <td className="c-data tri-fraco">{quando(o.criado_em)}</td>
              <td className="c-motivo">
                <span className="adm-motivo-rejeicao" title={o.erro_leitura ?? undefined}>
                  {motivoRejeicaoSimplificado(o.erro_leitura)}
                </span>
              </td>
              <td className="c-reparar">
                {precisaSubstituir(o.erro_leitura) ? (
                  <>
                    <button type="button" className="bt bt-mini" onClick={() => onVer(o)}>ver</button>
                    <button
                      type="button"
                      className="bt bt-mini"
                      title="Substituir pelo PDF corrigido no Obra Prima"
                      onClick={() => onSubstituir?.(o)}
                    >
                      substituir
                    </button>
                  </>
                ) : (
                  <button
                    type="button"
                    className="bt bt-mini adm-reparar"
                    title="Reparar"
                    aria-label="Reparar"
                    onClick={() => (onReparar ?? onVer)(o)}
                  >
                    REPARAR
                  </button>
                )}
                {onExcluir && (
                  <button type="button" className="bt bt-mini bt-perigo" onClick={() => onExcluir(o)}>excluir</button>
                )}
              </td>
            </tr>
          ) : (
            <tr key={o.id}>
              <td className="c-oc">
                <span className="tri-num" title={o.nome_arquivo}>{rotuloOC(o)}</span>
              </td>
              <td className="c-obra" data-encolhe="obra" data-base="13" data-peso="400">
                <span title={tituloObra(o)}>{obraDaLinha(o)}</span>
              </td>
              <td className="c-fat">
                <span title={faturamentoDaLinha(o)}>{faturamentoDaLinha(o)}</span>
              </td>
              <td className="c-valor">{o.total != null ? emReais(o.total) : '—'}</td>
              <td className="c-data tri-fraco">{quando(o.criado_em)}</td>
              <td className="c-leitura">
                <span className={'pino ' + CLASSE_STATUS[o.status]}>{NOME_STATUS[o.status]}</span>
                {o.erro_leitura && (
                  <span className="adm-dica adm-dica-alerta adm-dica-bloco" title={o.erro_leitura}>
                    {o.erro_leitura}
                  </span>
                )}
              </td>
              <td className="acoes">
                {onLer && (o.status === 'inserido' || o.status === 'falhou' || o.status === 'lendo') && (
                  <button
                    type="button"
                    className="bt bt-mini"
                    disabled={lendoId === o.id || loteRodando}
                    onClick={() => onLer(o.id)}
                  >
                    {lendoId === o.id
                      ? 'lendo…'
                      : o.status === 'falhou'
                        ? 'ler de novo'
                        : o.status === 'lendo'
                          ? 'retomar'
                          : 'ler'}
                  </button>
                )}
                {onEnviar && (
                  <button
                    type="button"
                    className="bt bt-mini"
                    disabled={enviandoId === o.id || enviandoId === '*'}
                    onClick={() => onEnviar(o.id)}
                  >
                    {enviandoId === o.id ? 'enviando…' : 'enviar'}
                  </button>
                )}
                <button type="button" className="bt bt-mini" onClick={() => onVer(o)}>ver</button>
                {onSubstituir && (
                  <button
                    type="button"
                    className="bt bt-mini"
                    title="Substituir por outra OC (número pode ser diferente)"
                    onClick={() => onSubstituir(o)}
                  >
                    substituir
                  </button>
                )}
                {onExcluir && (
                  <button type="button" className="bt bt-mini bt-perigo" onClick={() => onExcluir(o)}>excluir</button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function rotuloOC(o: OrdemDeCompra): string {
  if (o.numero) return o.numero
  return o.nome_arquivo.replace(/\.pdf$/i, '') || '—'
}

function obraDaLinha(o: OrdemDeCompra): string {
  const t = (o.obra_centro_custo || o.comprador_nome || '').replace(/\s+/g, ' ').trim()
  return t || '—'
}

/** Obra/centro sem o sufixo da cidade — padrão "NOME - CIDADE". */
function nomeObraSemCidade(o: OrdemDeCompra): string {
  const t = obraDaLinha(o)
  if (t === '—') return t
  const corte = t.lastIndexOf(' - ')
  return corte > 0 ? t.slice(0, corte).trim() : t
}

function faturamentoDaLinha(o: OrdemDeCompra): string {
  const nome = (o.comprador_nome || '').trim()
  const cnpj = o.comprador_cnpj ? formatarCNPJ(o.comprador_cnpj) : ''
  if (!nome && !cnpj) return '—'
  return [nome, cnpj].filter(Boolean).join(' · ')
}

function tituloObra(o: OrdemDeCompra): string | undefined {
  const obra = obraDaLinha(o)
  if (obra === '—') return o.nome_arquivo
  return `${obra} · ${o.nome_arquivo}`
}
