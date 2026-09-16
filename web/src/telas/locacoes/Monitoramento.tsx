// rev 2 — Locações: a lista de equipamentos (Fase 2: monitoramento · Fase 3: devolver)
//
// POR QUE NÃO AGRUPA VISUALMENTE POR OC
//
//	O plano fechou "ordenada por vencimento mais próximo primeiro" como o
//	critério principal — agrupar por OC quebraria essa ordem toda vez que
//	dois equipamentos da mesma OC vencessem em datas diferentes (o caso
//	normal depois de uma renovação parcial, Fase 4). Em vez de agrupar, cada
//	linha mostra a OC de origem como uma etiqueta — dá pra ver a origem sem
//	perder a ordenação por urgência.
//
// SELECIONAR É DECIDIR — LOTE E PARCIAL NASCEM DO MESMO CHECKBOX
//
//	Marcar um equipamento e mandar devolver (ou pedir renovação) É o caso de
//	um só; marcar vários é o lote. Não existem dois fluxos (Fase 3/4, decisão
//	do dono) — só aparece pra quem tem LOCACOES_DECIDIR, e só nas vistas onde
//	faz sentido decidir (não em "Encerrados", que já não tem mais nada
//	ativo). Um equipamento com renovação pendente trava "Devolver" — é o
//	intertravamento (opção A) rodando também na tela, não só no motor.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { CartaoLinha } from '../../componentes/CartaoLinha'
import { useEhMobile } from '../../componentes/useEhMobile'
import type { Perfil } from '../../sessao/tipos'
import { DetalheEquipamento } from './Equipamento'
import { Devolver } from './Devolver'
import { PedirRenovacao } from './PedirRenovacao'
import { RotinaLocacoesDecidir, temRotina } from './rotinas'
import { emData, emReais, rotuloDaFaixa, rotuloDaPeriodicidade, type EquipamentoLocado } from './tipos'

interface Props {
  vista: 'ativos' | 'descobertos' | 'encerrados'
  titulo: string
  perfil?: Perfil | null
  /** Filtro aplicado no cliente — usado por "Vencendo" (um recorte de "ativos"). */
  filtro?: (e: EquipamentoLocado) => boolean
}

export function Monitoramento({ vista, titulo, perfil = null, filtro }: Props) {
  const ehMobile = useEhMobile()
  const [equipamentos, setEquipamentos] = useState<EquipamentoLocado[] | null>(null)
  const [erro, setErro] = useState('')
  const [aberto, setAberto] = useState<string | null>(null)
  const [selecionados, setSelecionados] = useState<Set<string>>(new Set())
  const [devolvendo, setDevolvendo] = useState(false)
  const [renovando, setRenovando] = useState(false)

  const podeDecidir = vista !== 'encerrados' && temRotina(perfil, RotinaLocacoesDecidir)

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ equipamentos: EquipamentoLocado[] }>(`/locacoes/equipamentos?vista=${vista}`)
      setEquipamentos(r.equipamentos)
      setErro('')
    } catch (e) {
      setEquipamentos([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os equipamentos.')
    }
  }, [vista])

  useEffect(() => { void carregar() }, [carregar])
  useEffect(() => { setSelecionados(new Set()) }, [vista])

  const lista = filtro ? equipamentos?.filter(filtro) ?? null : equipamentos

  function alternar(id: string) {
    setSelecionados(s => {
      const novo = new Set(s)
      if (novo.has(id)) novo.delete(id)
      else novo.add(id)
      return novo
    })
  }

  const selecionadosCompletos = (lista ?? []).filter(e => selecionados.has(e.id))
  const algumPendente = selecionadosCompletos.some(e => e.renovacao_pendente)

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>{titulo}</h1>
          <p>Equipamento locado, mais próximo do vencimento primeiro.</p>
        </div>
      </header>

      {erro && <div className="erro-caixa">{erro}</div>}

      {lista === null ? (
        <Carregando texto="Carregando..." />
      ) : lista.length === 0 ? (
        <div className="vazio">Nenhum equipamento aqui.</div>
      ) : ehMobile ? (
        <div className="cl-lista">
          {lista.map(e => (
            <CartaoLinha
              key={e.id}
              titulo={<>{e.descricao}{e.renovacao_pendente && <span className="loc-selo loc-selo-amarelo" style={{ marginLeft: 8 }}>renovação pedida</span>}</>}
              onClick={() => setAberto(e.id)}
              acoes={podeDecidir && (
                <label className="loc-check">
                  <input type="checkbox" checked={selecionados.has(e.id)} onChange={() => alternar(e.id)} />
                  selecionar
                </label>
              )}
              linhas={[
                { rotulo: 'Obra', valor: e.obra_centro_custo || '—' },
                { rotulo: 'Fornecedor', valor: e.fornecedor_nome || '—' },
                { rotulo: 'O.C.', valor: e.ordem_numero || '—' },
                { rotulo: 'Quantidade', valor: e.qtd_ativa },
                { rotulo: 'Prazo', valor: rotuloDaPeriodicidade(e.periodicidade) },
                {
                  rotulo: e.estado === 'encerrado' ? 'Encerrado em' : 'Vencimento',
                  valor: <FaixaSelo faixa={e.faixa} descoberto={e.descoberto} texto={`${emData(e.vencimento_atual)} · ${rotuloDaFaixa(e.dias_para_vencer, e.descoberto)}`} />,
                },
              ]}
            />
          ))}
        </div>
      ) : (
        <div className="tabela-rolo">
          <table className="tabela">
            <thead>
              <tr>
                {podeDecidir && <th></th>}
                <th>Equipamento</th><th>Obra</th><th>Fornecedor</th><th>O.C.</th>
                <th>Qtd.</th><th>Valor</th><th>Prazo</th><th>Vencimento</th>
              </tr>
            </thead>
            <tbody>
              {lista.map(e => (
                <tr key={e.id} onClick={() => setAberto(e.id)} style={{ cursor: 'pointer' }}>
                  {podeDecidir && (
                    <td onClick={ev => ev.stopPropagation()}>
                      <input type="checkbox" checked={selecionados.has(e.id)} onChange={() => alternar(e.id)} />
                    </td>
                  )}
                  <td>
                    {e.descricao}
                    {e.renovacao_pendente && <span className="loc-selo loc-selo-amarelo" style={{ marginLeft: 8 }}>renovação pedida</span>}
                  </td>
                  <td>{e.obra_centro_custo || '—'}</td>
                  <td>{e.fornecedor_nome || '—'}</td>
                  <td>{e.ordem_numero || '—'}</td>
                  <td>{e.qtd_ativa}</td>
                  <td>{emReais(e.valor_unit)}</td>
                  <td>{rotuloDaPeriodicidade(e.periodicidade)}</td>
                  <td>
                    <FaixaSelo faixa={e.faixa} descoberto={e.descoberto} texto={`${emData(e.vencimento_atual)} · ${rotuloDaFaixa(e.dias_para_vencer, e.descoberto)}`} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {podeDecidir && selecionadosCompletos.length > 0 && (
        <div className="loc-barra-selecao">
          <span>
            {selecionadosCompletos.length} selecionado{selecionadosCompletos.length === 1 ? '' : 's'}
            {algumPendente && ' — um deles já tem renovação pendente, cancele antes de devolver'}
          </span>
          <span style={{ display: 'flex', gap: 8 }}>
            <button type="button" className="bt bt-neutro" onClick={() => setRenovando(true)}>Pedir renovação</button>
            <button type="button" className="bt bt-forte" onClick={() => setDevolvendo(true)} disabled={algumPendente}>Devolver</button>
          </span>
        </div>
      )}

      {aberto && (
        <DetalheEquipamento
          id={aberto}
          perfil={perfil}
          aoFechar={() => setAberto(null)}
          aoMudar={() => void carregar()}
        />
      )}

      {devolvendo && (
        <Devolver
          equipamentos={selecionadosCompletos}
          aoFechar={() => setDevolvendo(false)}
          aoSalvar={() => { setDevolvendo(false); setSelecionados(new Set()); void carregar() }}
        />
      )}

      {renovando && (
        <PedirRenovacao
          equipamentos={selecionadosCompletos}
          aoFechar={() => setRenovando(false)}
          aoSalvar={() => { setRenovando(false); setSelecionados(new Set()); void carregar() }}
        />
      )}
    </>
  )
}

function FaixaSelo({ faixa, descoberto, texto }: { faixa: EquipamentoLocado['faixa']; descoberto: boolean; texto: string }) {
  return (
    <span className={`loc-selo loc-selo-${faixa}${descoberto ? ' loc-selo-descoberto' : ''}`}>
      {texto}
    </span>
  )
}
