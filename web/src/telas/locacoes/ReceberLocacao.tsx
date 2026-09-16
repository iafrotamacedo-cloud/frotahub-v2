// rev 1 — Locações: receber equipamento (Fase 1, 16/09/2026)
//
// A BIFURCAÇÃO DE "AGUARDANDO NF"
//
//	Aberta de dentro de `administrativo/AguardandoNF.tsx`, quando o
//	almoxarife escolhe "Locação" em vez de escanear a nota. Não há nota
//	aqui — a prova de entrada é o romaneio (obrigatório) + as fotos de cada
//	item (obrigatórias, pelo menos uma cada). A data de recebimento é
//	sempre hoje, decidida no backend — sem campo nesta tela de propósito.
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { ScannerDeDocumento } from '../../componentes/scanner/ScannerDeDocumento'
import { motor, enviarFormulario, ErroMotor } from '../../motor/cliente'
import { emReais } from './tipos'
import type { ItemDaOrdemParaLocacao, OrdemParaReceberLocacao, Periodicidade, ResultadoDoRecebimento } from './tipos'

interface Props {
  ordemCompraId: string
  aoFechar: () => void
  aoSalvar: () => void
}

interface ItemEditavel extends ItemDaOrdemParaLocacao {
  qtdRecebida: string
  fotos: File[]
}

export function ReceberLocacao({ ordemCompraId, aoFechar, aoSalvar }: Props) {
  const [ordem, setOrdem] = useState<OrdemParaReceberLocacao | null>(null)
  const [itens, setItens] = useState<ItemEditavel[] | null>(null)
  const [periodicidade, setPeriodicidade] = useState<Periodicidade>('mensal')
  const [romaneio, setRomaneio] = useState<File[]>([])
  const [scannerAberto, setScannerAberto] = useState(false)
  const [nfNumero, setNfNumero] = useState('')
  const [nfArquivo, setNfArquivo] = useState<File | null>(null)
  const [erro, setErro] = useState('')
  const [carregando, setCarregando] = useState(true)
  const [salvando, setSalvando] = useState(false)

  const campoNF = useRef<HTMLInputElement>(null)

  useEffect(() => {
    let cancelado = false
    motor<{ ordem: OrdemParaReceberLocacao; itens: ItemDaOrdemParaLocacao[] }>(
      `/locacoes/ordens/${ordemCompraId}/itens`,
    )
      .then(r => {
        if (cancelado) return
        setOrdem(r.ordem)
        setItens(r.itens.map(it => ({ ...it, qtdRecebida: formatarQtd(it.qtd), fotos: [] })))
      })
      .catch(e => {
        if (cancelado) return
        setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os itens desta ordem de compra.')
      })
      .finally(() => { if (!cancelado) setCarregando(false) })
    return () => { cancelado = true }
  }, [ordemCompraId])

  function receberRomaneio(paginas: File[]) {
    setScannerAberto(false)
    if (paginas.length === 0) return
    setRomaneio(paginas)
    setErro('')
  }

  function tirarFoto(itemId: string, foto: File) {
    setItens(lista => lista?.map(it => (it.id === itemId ? { ...it, fotos: [...it.fotos, foto] } : it)) ?? null)
  }

  function removerFoto(itemId: string, i: number) {
    setItens(lista => lista?.map(it => (it.id === itemId ? { ...it, fotos: it.fotos.filter((_, j) => j !== i) } : it)) ?? null)
  }

  async function salvar(e?: FormEvent) {
    e?.preventDefault()
    if (!itens || itens.length === 0) {
      setErro('Esta ordem de compra não tem itens para receber.')
      return
    }
    if (romaneio.length === 0) {
      setErro('Escaneie o romaneio de entrega.')
      return
    }
    for (const it of itens) {
      const qtd = Number(it.qtdRecebida.replace(',', '.'))
      if (!qtd || qtd <= 0) {
        setErro(`Informe a quantidade recebida de ${it.descricao}.`)
        return
      }
      if (it.fotos.length === 0) {
        setErro(`Tire ao menos uma foto de ${it.descricao}.`)
        return
      }
    }
    setErro('')
    setSalvando(true)
    try {
      const forma = new FormData()
      forma.append('periodicidade', periodicidade)
      forma.append('itens', JSON.stringify(itens.map(it => ({
        item_id: it.id,
        descricao: it.descricao,
        unidade: it.unidade ?? '',
        qtd_recebida: Number(it.qtdRecebida.replace(',', '.')),
        valor_unit: it.valor_unit,
      }))))
      forma.append('romaneio', romaneio[0], romaneio[0].name)
      for (const pg of romaneio.slice(1)) forma.append('romaneio_paginas', pg, pg.name)
      if (nfNumero.trim()) forma.append('nf_numero', nfNumero.trim())
      if (nfArquivo) forma.append('nf', nfArquivo, nfArquivo.name)
      for (const it of itens) {
        for (const foto of it.fotos) forma.append(`fotos_${it.id}`, foto, foto.name)
      }
      await enviarFormulario<ResultadoDoRecebimento>(`/locacoes/ordens/${ordemCompraId}/receber`, forma)
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar o recebimento.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <>
      <Janela
        titulo={`Receber locação · O.C. ${ordem?.numero ?? '—'}`}
        descricao={ordem ? `${ordem.obra_centro_custo ?? 'obra não identificada'} — ${ordem.fornecedor_nome ?? 'fornecedor não identificado'}` : undefined}
        aoFechar={aoFechar}
        largura={560}
      >
        {carregando ? (
          <p className="dica" style={{ padding: '16px 20px' }}>Carregando os itens desta ordem de compra…</p>
        ) : (
          <form className="jn-corpo" onSubmit={salvar}>
            <label htmlFor="loc-prazo">Prazo da locação</label>
            <select id="loc-prazo" value={periodicidade} onChange={e => setPeriodicidade(e.target.value as Periodicidade)} disabled={salvando}>
              <option value="mensal">Mensal (1 mês)</option>
              <option value="quinzenal">Quinzenal (15 dias)</option>
              <option value="semanal">Semanal (7 dias)</option>
            </select>

            <label style={{ marginTop: 14 }}>Equipamentos recebidos</label>
            <div className="loc-itens">
              {itens?.map(it => (
                <div key={it.id} className="loc-item">
                  <div className="loc-item-cabecalho">
                    <span className="loc-item-descricao">{it.descricao}</span>
                    <span className="loc-item-valor">{emReais(it.valor_unit)}{it.unidade ? ` / ${it.unidade}` : ''}</span>
                  </div>
                  <label>Quantidade recebida (pedida: {formatarQtd(it.qtd)})</label>
                  <input
                    inputMode="decimal"
                    value={it.qtdRecebida}
                    disabled={salvando}
                    onChange={e => {
                      const v = e.target.value.replace(/[^\d,.]/g, '')
                      setItens(lista => lista?.map(x => (x.id === it.id ? { ...x, qtdRecebida: v } : x)) ?? null)
                    }}
                  />
                  <div className="nf-fotos-grade">
                    {it.fotos.map((foto, i) => (
                      <FotoMini key={i} foto={foto} onRemover={() => removerFoto(it.id, i)} />
                    ))}
                    <BotaoFoto disabled={salvando} aoEscolher={foto => tirarFoto(it.id, foto)} />
                  </div>
                </div>
              ))}
            </div>

            <label style={{ marginTop: 14 }}>Romaneio de entrega</label>
            <div className="nf-fotos-grade">
              {romaneio.map((pg, i) => (
                <PaginaMini key={`${pg.name}-${i}`} arquivo={pg} numero={i + 1} onRemover={() => setRomaneio(rs => rs.filter((_, j) => j !== i))} />
              ))}
              <button
                type="button"
                className={`nf-foto-add${romaneio.length === 0 ? ' nf-foto-add-larga' : ''}`}
                onClick={() => setScannerAberto(true)}
                disabled={salvando}
              >
                <IconeScanner />
                <span>{romaneio.length === 0 ? 'Escanear o romaneio' : 'Escanear de novo'}</span>
              </button>
            </div>

            <label style={{ marginTop: 14 }}>Nota fiscal (opcional — quando vem junto)</label>
            <input
              value={nfNumero}
              onChange={e => setNfNumero(e.target.value)}
              placeholder="Número da nota fiscal"
              disabled={salvando}
            />
            <div className="nf-fotos-grade">
              {nfArquivo && <FotoMini foto={nfArquivo} onRemover={() => setNfArquivo(null)} />}
              {!nfArquivo && (
                <button type="button" className="nf-foto-add" onClick={() => campoNF.current?.click()} disabled={salvando}>
                  <IconeCamera />
                  <span>Foto da NF</span>
                </button>
              )}
            </div>
            <input
              ref={campoNF} type="file" accept="image/*" capture="environment"
              style={{ display: 'none' }}
              onChange={e => {
                const f = e.target.files?.[0]
                if (f) setNfArquivo(f)
                e.target.value = ''
              }}
            />

            {erro && <div className="erro-caixa">{erro}</div>}

            <div className="jn-pe">
              <button type="button" className="bt bt-neutro" onClick={aoFechar} disabled={salvando}>Cancelar</button>
              <button type="submit" className="bt bt-forte" disabled={salvando || !itens || itens.length === 0}>
                {salvando ? 'Salvando...' : 'Salvar recebimento'}
              </button>
            </div>
          </form>
        )}
      </Janela>

      {scannerAberto && (
        <ScannerDeDocumento
          titulo={`Romaneio · O.C. ${ordem?.numero ?? '—'}`}
          paginaInicial={1}
          aoConcluir={receberRomaneio}
          aoCancelar={() => setScannerAberto(false)}
        />
      )}
    </>
  )
}

function BotaoFoto({ aoEscolher, disabled }: { aoEscolher: (f: File) => void; disabled?: boolean }) {
  const campo = useRef<HTMLInputElement>(null)
  return (
    <>
      <button type="button" className="nf-foto-add" onClick={() => campo.current?.click()} disabled={disabled}>
        <IconeCamera />
        <span>Foto</span>
      </button>
      <input
        ref={campo} type="file" accept="image/*" capture="environment"
        style={{ display: 'none' }}
        onChange={e => {
          const f = e.target.files?.[0]
          if (f) aoEscolher(f)
          e.target.value = ''
        }}
      />
    </>
  )
}

function PaginaMini({ arquivo, numero, onRemover }: { arquivo: File; numero: number; onRemover: () => void }) {
  const url = useUrlDoArquivo(arquivo)
  return (
    <div className="nf-foto-mini nf-pagina-mini">
      {url && <img src={url} alt={`Página ${numero}`} />}
      <span className="nf-pagina-num">{numero}</span>
      <button type="button" className="nf-foto-x" onClick={onRemover} aria-label={`Remover a página ${numero}`}>×</button>
    </div>
  )
}

function FotoMini({ foto, onRemover }: { foto: File; onRemover: () => void }) {
  const url = useUrlDoArquivo(foto)
  return (
    <div className="nf-foto-mini">
      {url && <img src={url} alt="Foto" />}
      <button type="button" className="nf-foto-x" onClick={onRemover} aria-label="Remover esta foto">×</button>
    </div>
  )
}

function useUrlDoArquivo(arquivo: File | null): string | null {
  const [url, setUrl] = useState<string | null>(null)
  useEffect(() => {
    if (!arquivo) { setUrl(null); return }
    const u = URL.createObjectURL(arquivo)
    setUrl(u)
    return () => URL.revokeObjectURL(u)
  }, [arquivo])
  return url
}

function formatarQtd(v: number): string {
  return v.toLocaleString('pt-BR', { minimumFractionDigits: 0, maximumFractionDigits: 3 })
}

function IconeCamera() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 8h3l2-2.5h6L17 8h3a1 1 0 0 1 1 1v9a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1Z" />
      <circle cx="12" cy="13.5" r="3.4" />
    </svg>
  )
}

function IconeScanner() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 8V5a1 1 0 0 1 1-1h3M16 4h3a1 1 0 0 1 1 1v3M20 16v3a1 1 0 0 1-1 1h-3M8 20H5a1 1 0 0 1-1-1v-3" />
      <path d="M7 12h10" />
    </svg>
  )
}
