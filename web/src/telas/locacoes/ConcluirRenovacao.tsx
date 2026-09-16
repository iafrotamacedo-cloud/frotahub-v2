// rev 1 — Locações: concluir a renovação (Fase 4, 16/09/2026)
//
// TRÊS PASSOS, UMA TELA SÓ
//
//	1. Sobe o PDF da OC de renovação por `POST /administrativo/compras/
//	   ordens` — a MESMA rota de Compras, só que com `destino_recebimento=
//	   locacao` (por isso essa OC nunca aparece em "Aguardando NF"). 2. Lê
//	   com `POST /administrativo/compras/ordens/{id}/ler` — síncrono, volta
//	   com número/total pra conferência. 3. Amarra com
//	   `POST /locacoes/renovacoes/concluir`, que é quem de fato cria o
//	   próximo período e empurra o vencimento.
//
//	Cruzar módulos pelo FRONT (chamar rotas de administrativo daqui) não
//	fere P-13 — a disciplina é sobre pacotes Go não se importarem; telas já
//	conversam com qualquer rota do motor que precisem.
import { useRef, useState } from 'react'
import { Janela } from '../../componentes/Janela'
import { enviarFormulario, motor, ErroMotor } from '../../motor/cliente'
import { emReais } from './tipos'
import type { PedidoDeRenovacao, ResultadoDaConclusao, ResultadoDaInsercaoDeOC, ResultadoDaLeituraDeOC } from './tipos'

interface Props {
  pedidos: PedidoDeRenovacao[]
  aoFechar: () => void
  aoSalvar: () => void
}

type Etapa = 'escolher' | 'lendo' | 'lida' | 'concluindo'

export function ConcluirRenovacao({ pedidos, aoFechar, aoSalvar }: Props) {
  const [etapa, setEtapa] = useState<Etapa>('escolher')
  const [arquivo, setArquivo] = useState<File | null>(null)
  const [ordemCompraId, setOrdemCompraId] = useState<string | null>(null)
  const [leitura, setLeitura] = useState<ResultadoDaLeituraDeOC | null>(null)
  const [erro, setErro] = useState('')
  const campo = useRef<HTMLInputElement>(null)

  async function subirEler(f: File) {
    setArquivo(f)
    setErro('')
    setEtapa('lendo')
    try {
      const forma = new FormData()
      forma.append('arquivos', f, f.name)
      forma.append('destino_recebimento', 'locacao')
      const r = await enviarFormulario<{ arquivos: ResultadoDaInsercaoDeOC[] }>('/administrativo/compras/ordens', forma)
      const um = r.arquivos[0]
      if (!um || um.erro) {
        setErro(um?.erro || 'Não consegui inserir esta OC.')
        setEtapa('escolher')
        return
      }
      setOrdemCompraId(um.id ?? null)
      if (!um.id) {
        setErro('Inseri a OC mas não recebi o identificador dela.')
        setEtapa('escolher')
        return
      }
      const lida = await motor<ResultadoDaLeituraDeOC>(`/administrativo/compras/ordens/${um.id}/ler`, { metodo: 'POST' })
      setLeitura(lida)
      setEtapa('lida')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui inserir ou ler esta OC.')
      setEtapa('escolher')
    }
  }

  async function concluir() {
    if (!ordemCompraId) return
    setErro('')
    setEtapa('concluindo')
    try {
      await motor<ResultadoDaConclusao>('/locacoes/renovacoes/concluir', {
        metodo: 'POST',
        corpo: { ordem_compra_id: ordemCompraId, renovacoes: pedidos.map(p => p.id) },
      })
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui concluir a renovação.')
      setEtapa('lida')
    }
  }

  function tentarOutraVez() {
    setArquivo(null)
    setOrdemCompraId(null)
    setLeitura(null)
    setErro('')
    setEtapa('escolher')
  }

  return (
    <Janela
      titulo={pedidos.length === 1 ? `Concluir renovação · ${pedidos[0].descricao}` : `Concluir renovação de ${pedidos.length} equipamentos`}
      descricao="Insira o PDF da OC de renovação — a mesma esteira de Compras."
      aoFechar={aoFechar}
      largura={520}
    >
      <div className="jn-corpo">
        <ul style={{ margin: '0 0 14px', paddingLeft: 18 }}>
          {pedidos.map(p => (
            <li key={p.id}>{p.descricao} — {p.fornecedor_nome || 'fornecedor não identificado'}</li>
          ))}
        </ul>

        {etapa === 'escolher' && (
          <>
            <button type="button" className="bt bt-forte" onClick={() => campo.current?.click()}>
              Escolher o PDF da OC
            </button>
            <input
              ref={campo} type="file" accept="application/pdf"
              style={{ display: 'none' }}
              onChange={e => {
                const f = e.target.files?.[0]
                if (f) void subirEler(f)
                e.target.value = ''
              }}
            />
          </>
        )}

        {(etapa === 'lendo') && <p className="dica">Lendo {arquivo?.name}…</p>}

        {(etapa === 'lida' || etapa === 'concluindo') && leitura && (
          <div className="loc-item">
            {leitura.status === 'falhou' ? (
              <>
                <p style={{ margin: '0 0 8px' }}>Não consegui ler esta OC: {leitura.motivo}</p>
                <button type="button" className="bt bt-mini bt-neutro" onClick={tentarOutraVez}>Tentar outro arquivo</button>
              </>
            ) : (
              <>
                <div className="loc-item-cabecalho">
                  <span className="loc-item-descricao">O.C. {leitura.numero ?? '—'}</span>
                  <span className="loc-item-valor">{emReais(leitura.total)}</span>
                </div>
                <p style={{ margin: '4px 0' }}>{leitura.itens} item{leitura.itens === 1 ? '' : 'ns'} lidos.</p>
              </>
            )}
          </div>
        )}

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar} disabled={etapa === 'lendo' || etapa === 'concluindo'}>
            Cancelar
          </button>
          {etapa === 'lida' && leitura?.status === 'lido' && (
            <button type="button" className="bt bt-forte" onClick={() => void concluir()}>
              Concluir renovação
            </button>
          )}
          {etapa === 'concluindo' && (
            <button type="button" className="bt bt-forte" disabled>Concluindo...</button>
          )}
        </div>
      </div>
    </Janela>
  )
}
