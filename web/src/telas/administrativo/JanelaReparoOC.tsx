// rev 1 — pop-up EDITAR na tela Reparar OC
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import type { EstadoReparoOC, ObraCentroSugerida, ResultadoReparoOC } from './tipos'

interface Props {
  ordemId: string
  fechar: () => void
  aoSalvar: (r: ResultadoReparoOC) => void
}

export function JanelaReparoOC({ ordemId, fechar, aoSalvar }: Props) {
  const [estado, setEstado] = useState<EstadoReparoOC | null>(null)
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)
  const [fornecedorCNPJ, setFornecedorCNPJ] = useState('')
  const [obraBusca, setObraBusca] = useState('')
  const [obraEscolhida, setObraEscolhida] = useState<ObraCentroSugerida | null>(null)
  const [sugestoes, setSugestoes] = useState<ObraCentroSugerida[]>([])
  const [buscandoObra, setBuscandoObra] = useState(false)

  useEffect(() => {
    void (async () => {
      try {
        const r = await motor<EstadoReparoOC>(`/administrativo/compras/ordens/${ordemId}/reparo`)
        setEstado(r)
        setFornecedorCNPJ(String(r.fornecedor_cnpj ?? '').replace(/\D/g, ''))
        if (r.obra_centro_custo) {
          setObraBusca(r.obra_centro_custo)
          setObraEscolhida({
            obra_centro_custo: r.obra_centro_custo,
            comprador_cnpj: r.comprador_cnpj,
            comprador_nome: r.comprador_nome,
          })
        }
      } catch (e) {
        setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o que precisa corrigir.')
      }
    })()
  }, [ordemId])

  const buscarObras = useCallback(async (q: string) => {
    if (q.trim().length < 2) {
      setSugestoes([])
      return
    }
    setBuscandoObra(true)
    try {
      const r = await motor<{ obras: ObraCentroSugerida[] }>(
        '/administrativo/compras/obras-centro?q=' + encodeURIComponent(q.trim()))
      setSugestoes(r.obras)
    } catch {
      setSugestoes([])
    } finally {
      setBuscandoObra(false)
    }
  }, [])

  useEffect(() => {
    if (!estado?.precisa_faturamento) return
    const t = window.setTimeout(() => { void buscarObras(obraBusca) }, 220)
    return () => window.clearTimeout(t)
  }, [obraBusca, buscarObras, estado?.precisa_faturamento])

  async function salvar() {
    if (!estado) return
    setSalvando(true)
    setErro('')
    try {
      const corpo: Record<string, string> = {}
      if (estado.precisa_fornecedor) corpo.fornecedor_cnpj = fornecedorCNPJ
      if (estado.precisa_faturamento) {
        corpo.obra_centro_custo = obraEscolhida?.obra_centro_custo ?? obraBusca.trim()
        if (obraEscolhida?.comprador_cnpj) corpo.comprador_cnpj = String(obraEscolhida.comprador_cnpj)
        if (obraEscolhida?.comprador_nome) corpo.comprador_nome = String(obraEscolhida.comprador_nome)
      }
      const r = await motor<ResultadoReparoOC>(`/administrativo/compras/ordens/${ordemId}/reparo`, {
        metodo: 'POST',
        corpo,
      })
      aoSalvar(r)
      if (r.status === 'lido') fechar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui gravar a correção.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <div className="orc-janela" role="dialog" aria-modal aria-labelledby="adm-reparo-titulo">
      <div className="orc-janela-caixa adm-reparo-janela">
        <h3 id="adm-reparo-titulo">Corrigir manualmente</h3>
        <p className="orc-dica">Preencha só o que deu erro na leitura. Quando passar nos filtros, a OC vai para Processadas e entra nas pendentes de envio do PCO.</p>

        {erro && <p className="erro">{erro}</p>}
        {!estado ? <Carregando /> : (
          <>
            {estado.precisa_fornecedor && (
              <label className="adm-reparo-campo">
                <span>CNPJ do fornecedor</span>
                <input
                  type="text"
                  inputMode="numeric"
                  value={fornecedorCNPJ}
                  onChange={e => setFornecedorCNPJ(e.target.value.replace(/\D/g, '').slice(0, 14))}
                  placeholder="14 dígitos"
                />
              </label>
            )}

            {estado.precisa_faturamento && (
              <label className="adm-reparo-campo">
                <span>Obra / centro de custo</span>
                <input
                  type="text"
                  value={obraBusca}
                  onChange={e => {
                    setObraBusca(e.target.value)
                    setObraEscolhida(null)
                  }}
                  placeholder="Digite ao menos 2 letras para buscar"
                  autoComplete="off"
                />
                {buscandoObra && <em className="adm-reparo-busca">buscando…</em>}
                {sugestoes.length > 0 && (
                  <ul className="adm-reparo-lista">
                    {sugestoes.map(o => (
                      <li key={o.obra_centro_custo}>
                        <button
                          type="button"
                          className={'adm-reparo-opcao' + (obraEscolhida?.obra_centro_custo === o.obra_centro_custo ? ' escolhida' : '')}
                          onClick={() => {
                            setObraEscolhida(o)
                            setObraBusca(o.obra_centro_custo)
                            setSugestoes([])
                          }}
                        >
                          {o.obra_centro_custo}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </label>
            )}

            <div className="adm-reparo-botoes">
              <button type="button" className="bt bt-neutro" onClick={fechar} disabled={salvando}>cancelar</button>
              <button type="button" className="bt bt-forte" onClick={() => void salvar()} disabled={salvando}>
                {salvando ? 'gravando…' : 'salvar correção'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
