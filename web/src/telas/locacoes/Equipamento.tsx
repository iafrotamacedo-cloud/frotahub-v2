// rev 1 — Locações: o detalhe de um equipamento (Fase 2, 16/09/2026)
//
// ROMANEIO E FOTOS ABREM NO VISOR PADRÃO
//
//	Mesma regra da casa (26/08/2026): documento nunca é baixado só pra
//	olhar. `GET /locacoes/arquivos/{sha}` devolve um endereço assinado do
//	armazém (mesmo desenho de `arquivoDaNF`), e o front busca o endereço uma
//	vez e entrega ao `VisorDeDocumento` — nunca ao `motor()` puro.
import { useCallback, useEffect, useState } from 'react'
import { Janela } from '../../componentes/Janela'
import { Carregando } from '../../componentes/Carregando'
import { Confirmar } from '../../componentes/Confirmar'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { motor, ErroMotor } from '../../motor/cliente'
import type { Perfil } from '../../sessao/tipos'
import { emData, emReais, rotuloDaFaixa, rotuloDaPeriodicidade, type DetalheDoEquipamento } from './tipos'

interface Props {
  id: string
  perfil?: Perfil | null
  aoFechar: () => void
  /** Chamado quando reabrir muda o estado do equipamento — quem chamou recarrega a lista. */
  aoMudar?: () => void
}

export function DetalheEquipamento({ id, perfil = null, aoFechar, aoMudar }: Props) {
  const [dados, setDados] = useState<DetalheDoEquipamento | null>(null)
  const [erro, setErro] = useState('')
  const [vendo, setVendo] = useState<{ endereco: string; nome: string } | null>(null)
  const [confirmandoReabrir, setConfirmandoReabrir] = useState(false)
  const [reabrindo, setReabrindo] = useState(false)

  const carregar = useCallback(() => {
    motor<DetalheDoEquipamento>(`/locacoes/equipamentos/${id}`)
      .then(r => setDados(r))
      .catch(e => setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar este equipamento.'))
  }, [id])

  useEffect(() => { carregar() }, [carregar])

  async function abrir(sha: string, nome: string) {
    try {
      const r = await motor<{ url: string }>(`/locacoes/arquivos/${sha}`)
      setVendo({ endereco: r.url, nome })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir este arquivo.')
    }
  }

  async function reabrir() {
    setConfirmandoReabrir(false)
    setReabrindo(true)
    try {
      await motor(`/locacoes/equipamentos/${id}/reabrir`, { metodo: 'POST' })
      carregar()
      aoMudar?.()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui reabrir a última devolução.')
    } finally {
      setReabrindo(false)
    }
  }

  if (vendo) {
    return <VisorDeDocumento endereco={vendo.endereco} nomeSugerido={vendo.nome} titulo={vendo.nome} voltar={() => setVendo(null)} />
  }

  return (
    <>
      <Janela titulo="Equipamento locado" aoFechar={aoFechar} largura={560}>
      <div className="jn-corpo">
        {erro && <div className="erro-caixa">{erro}</div>}
        {!dados ? (
          <Carregando texto="Carregando..." />
        ) : (
          <>
            <h3 style={{ margin: '0 0 4px' }}>{dados.equipamento.descricao}</h3>
            <p className="dica" style={{ margin: '0 0 14px' }}>
              {dados.equipamento.obra_centro_custo || 'obra não identificada'} — {dados.equipamento.fornecedor_nome || 'fornecedor não identificado'}
            </p>

            <div className="loc-itens">
              <div className="loc-item">
                <div className="loc-item-cabecalho">
                  <span className="loc-item-descricao">O.C. {dados.equipamento.ordem_numero ?? '—'}</span>
                  <span className="loc-item-valor">{emReais(dados.equipamento.valor_unit)}{dados.equipamento.unidade ? ` / ${dados.equipamento.unidade}` : ''}</span>
                </div>
                <p style={{ margin: '4px 0' }}>
                  Recebido: {dados.equipamento.qtd_recebida} — ativo: {dados.equipamento.qtd_ativa} — {rotuloDaPeriodicidade(dados.equipamento.periodicidade)}
                </p>
                <p style={{ margin: '4px 0' }}>
                  Vencimento atual: {emData(dados.equipamento.vencimento_atual)} ({rotuloDaFaixa(dados.equipamento.dias_para_vencer, dados.equipamento.descoberto)})
                </p>
                <p style={{ margin: '4px 0' }}>Estado: {dados.equipamento.estado === 'ativo' ? 'ativo' : 'encerrado'}</p>
              </div>
            </div>

            <label style={{ marginTop: 14 }}>Recebimento</label>
            <p style={{ margin: '4px 0' }}>{emData(dados.recebimento.data_recebimento)}{dados.recebimento.nf_numero ? ` — NF ${dados.recebimento.nf_numero}` : ''}</p>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 10 }}>
              <button type="button" className="bt bt-mini" onClick={() => void abrir(dados.recebimento.romaneio_sha256, 'Romaneio de entrega')}>
                ver romaneio
              </button>
              {dados.recebimento.nf_sha256 && (
                <button type="button" className="bt bt-mini bt-neutro" onClick={() => void abrir(dados.recebimento.nf_sha256!, 'Nota fiscal')}>
                  ver NF
                </button>
              )}
              {dados.fotos.map((f, i) => (
                <button key={f.id} type="button" className="bt bt-mini bt-neutro" onClick={() => void abrir(f.arquivo_sha256, `Foto ${i + 1}`)}>
                  foto {i + 1}
                </button>
              ))}
            </div>

            <label style={{ marginTop: 14 }}>Períodos de cobertura</label>
            <div className="tabela-rolo">
              <table className="tabela">
                <thead>
                  <tr><th>#</th><th>Tipo</th><th>Início</th><th>Fim</th><th>Qtd.</th><th>O.C.</th></tr>
                </thead>
                <tbody>
                  {dados.periodos.map(p => (
                    <tr key={p.id}>
                      <td>{p.numero}</td>
                      <td>{p.tipo === 'original' ? 'original' : 'renovação'}</td>
                      <td>{emData(p.inicio)}</td>
                      <td>{emData(p.fim)}</td>
                      <td>{p.qtd}</td>
                      <td>{p.ordens_compra?.numero ?? '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {dados.devolucoes.length > 0 && (
              <>
                <label style={{ marginTop: 14 }}>Devoluções</label>
                <div className="tabela-rolo">
                  <table className="tabela">
                    <thead>
                      <tr><th>Data</th><th>Qtd.</th><th></th></tr>
                    </thead>
                    <tbody>
                      {dados.devolucoes.map(d => (
                        <tr key={d.id}>
                          <td>{emData(d.data_devolucao)}</td>
                          <td>{d.qtd}</td>
                          <td>
                            <button type="button" className="bt bt-mini bt-neutro" onClick={() => void abrir(d.romaneio_sha256, 'Romaneio de devolução')}>
                              ver romaneio
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                {perfil?.nivel === 'builder' && (
                  <p style={{ marginTop: 10 }}>
                    <button type="button" className="bt bt-mini bt-perigo" onClick={() => setConfirmandoReabrir(true)} disabled={reabrindo}>
                      {reabrindo ? 'Reabrindo...' : 'Reabrir a última devolução'}
                    </button>
                  </p>
                )}
              </>
            )}

            <div className="jn-pe">
              <button type="button" className="bt bt-neutro" onClick={aoFechar}>Fechar</button>
            </div>
          </>
        )}
      </div>
      </Janela>

      {confirmandoReabrir && (
        <Confirmar
          titulo="Reabrir a última devolução"
          mensagem="Isto apaga a devolução mais recente deste equipamento (romaneio, fotos e quantidade), e ele volta a ficar ativo. Não dá pra desfazer esta ação."
          perigo
          rotuloConfirmar="Reabrir"
          aoConfirmar={() => void reabrir()}
          aoFechar={() => setConfirmandoReabrir(false)}
        />
      )}
    </>
  )
}
