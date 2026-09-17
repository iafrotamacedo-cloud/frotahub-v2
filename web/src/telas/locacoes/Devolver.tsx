// rev 2 — Locações: devolver (Fase 3, 16/09/2026 · frete Fase 5, 17/09/2026)
//
// LOTE E PARCIAL NASCEM DO MESMO FORMULÁRIO
//
//	"Total" é aceitar a quantidade sugerida (= tudo o que está ativo) em
//	todos os itens selecionados; "parcial" é editar a quantidade de um item
//	pra menos. Não são dois fluxos — é o mesmo, com o número editável.
//
// A OC DE FRETE É OPCIONAL E BUSCADA PELO NÚMERO
//
//	Sem vínculo automático (o sistema não tem como adivinhar qual PDF solto
//	do Obra Prima é o frete desta locação) — quem devolve busca a OC já
//	inserida pelo número e confirma. Uma OC só vale pro lote inteiro, do
//	mesmo jeito que o romaneio.
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { ScannerDeDocumento } from '../../componentes/scanner/ScannerDeDocumento'
import { enviarFormulario, motor, ErroMotor } from '../../motor/cliente'
import type { EquipamentoLocado, OrdemEncontrada, ResultadoDaDevolucao } from './tipos'

interface Props {
  equipamentos: EquipamentoLocado[]
  aoFechar: () => void
  aoSalvar: () => void
}

interface ItemEditavel {
  equipamento: EquipamentoLocado
  qtd: string
  fotos: File[]
}

export function Devolver({ equipamentos, aoFechar, aoSalvar }: Props) {
  const [itens, setItens] = useState<ItemEditavel[]>(
    equipamentos.map(e => ({ equipamento: e, qtd: formatarQtd(e.qtd_ativa), fotos: [] })),
  )
  const [romaneio, setRomaneio] = useState<File[]>([])
  const [scannerAberto, setScannerAberto] = useState(false)
  const [freteNumero, setFreteNumero] = useState('')
  const [frete, setFrete] = useState<OrdemEncontrada | null>(null)
  const [buscandoFrete, setBuscandoFrete] = useState(false)
  const [erro, setErro] = useState('')
  const [salvando, setSalvando] = useState(false)

  async function buscarFrete() {
    if (!freteNumero.trim()) return
    setBuscandoFrete(true)
    setErro('')
    try {
      const r = await motor<OrdemEncontrada>(`/locacoes/ordens/buscar?numero=${encodeURIComponent(freteNumero.trim())}`)
      setFrete(r)
    } catch (e) {
      setFrete(null)
      setErro(e instanceof ErroMotor ? e.message : 'Não achei nenhuma OC com este número.')
    } finally {
      setBuscandoFrete(false)
    }
  }

  function receberRomaneio(paginas: File[]) {
    setScannerAberto(false)
    if (paginas.length === 0) return
    setRomaneio(paginas)
    setErro('')
  }

  function tirarFoto(id: string, foto: File) {
    setItens(lista => lista.map(it => (it.equipamento.id === id ? { ...it, fotos: [...it.fotos, foto] } : it)))
  }

  function removerFoto(id: string, i: number) {
    setItens(lista => lista.map(it => (it.equipamento.id === id ? { ...it, fotos: it.fotos.filter((_, j) => j !== i) } : it)))
  }

  async function salvar(e?: FormEvent) {
    e?.preventDefault()
    if (romaneio.length === 0) {
      setErro('Escaneie o romaneio de devolução.')
      return
    }
    for (const it of itens) {
      const qtd = Number(it.qtd.replace(',', '.'))
      if (!qtd || qtd <= 0) {
        setErro(`Informe a quantidade devolvida de ${it.equipamento.descricao}.`)
        return
      }
      if (qtd > it.equipamento.qtd_ativa) {
        setErro(`A quantidade devolvida de ${it.equipamento.descricao} não pode passar de ${formatarQtd(it.equipamento.qtd_ativa)}.`)
        return
      }
      if (it.fotos.length === 0) {
        setErro(`Tire ao menos uma foto do estado de ${it.equipamento.descricao}.`)
        return
      }
    }
    setErro('')
    setSalvando(true)
    try {
      const forma = new FormData()
      forma.append('itens', JSON.stringify(itens.map(it => ({
        equipamento_id: it.equipamento.id,
        qtd: Number(it.qtd.replace(',', '.')),
      }))))
      forma.append('romaneio', romaneio[0], romaneio[0].name)
      for (const pg of romaneio.slice(1)) forma.append('romaneio_paginas', pg, pg.name)
      if (frete) forma.append('ordem_compra_frete_id', frete.id)
      for (const it of itens) {
        for (const foto of it.fotos) forma.append(`fotos_${it.equipamento.id}`, foto, foto.name)
      }
      await enviarFormulario<ResultadoDaDevolucao>('/locacoes/equipamentos/devolver', forma)
      aoSalvar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar a devolução.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <>
      <Janela
        titulo={itens.length === 1 ? `Devolver · ${itens[0].equipamento.descricao}` : `Devolver ${itens.length} equipamentos`}
        aoFechar={aoFechar}
        largura={560}
      >
        <form className="jn-corpo" onSubmit={salvar}>
          <label>Quantidade devolvida</label>
          <div className="loc-itens">
            {itens.map(it => (
              <div key={it.equipamento.id} className="loc-item">
                <div className="loc-item-cabecalho">
                  <span className="loc-item-descricao">{it.equipamento.descricao}</span>
                  <span className="loc-item-valor">ativo: {formatarQtd(it.equipamento.qtd_ativa)}{it.equipamento.unidade ? ` ${it.equipamento.unidade}` : ''}</span>
                </div>
                <input
                  inputMode="decimal"
                  value={it.qtd}
                  disabled={salvando}
                  onChange={e => {
                    const v = e.target.value.replace(/[^\d,.]/g, '')
                    setItens(lista => lista.map(x => (x.equipamento.id === it.equipamento.id ? { ...x, qtd: v } : x)))
                  }}
                />
                <div className="nf-fotos-grade">
                  {it.fotos.map((foto, i) => (
                    <FotoMini key={i} foto={foto} onRemover={() => removerFoto(it.equipamento.id, i)} />
                  ))}
                  <BotaoFoto disabled={salvando} aoEscolher={foto => tirarFoto(it.equipamento.id, foto)} />
                </div>
              </div>
            ))}
          </div>

          <label style={{ marginTop: 14 }}>Romaneio de devolução</label>
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

          <label style={{ marginTop: 14 }}>OC de frete (desmobilização) — opcional</label>
          {frete ? (
            <div className="loc-item">
              <div className="loc-item-cabecalho">
                <span className="loc-item-descricao">O.C. {frete.numero}</span>
                <span className="loc-item-valor">{frete.fornecedor_nome || 'fornecedor não identificado'}</span>
              </div>
              <button type="button" className="bt bt-mini bt-neutro" onClick={() => { setFrete(null); setFreteNumero('') }} disabled={salvando}>
                remover vínculo
              </button>
            </div>
          ) : (
            <div style={{ display: 'flex', gap: 8 }}>
              <input
                value={freteNumero}
                onChange={e => setFreteNumero(e.target.value)}
                placeholder="Número da OC de frete, se já inserida"
                disabled={salvando || buscandoFrete}
              />
              <button type="button" className="bt bt-mini" onClick={() => void buscarFrete()} disabled={salvando || buscandoFrete || !freteNumero.trim()}>
                {buscandoFrete ? 'Buscando...' : 'Buscar'}
              </button>
            </div>
          )}

          {erro && <div className="erro-caixa">{erro}</div>}

          <div className="jn-pe">
            <button type="button" className="bt bt-neutro" onClick={aoFechar} disabled={salvando}>Cancelar</button>
            <button type="submit" className="bt bt-forte" disabled={salvando}>
              {salvando ? 'Salvando...' : 'Salvar devolução'}
            </button>
          </div>
        </form>
      </Janela>

      {scannerAberto && (
        <ScannerDeDocumento
          titulo="Romaneio de devolução"
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
