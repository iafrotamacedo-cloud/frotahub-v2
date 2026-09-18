// rev 1 — a composição da NF de locação: OC + romaneio + fotos, tela cheia (18/09/2026)
//
// O QUE O DONO PEDIU
//
//	Um link em cada linha de "Aguardando NF de locação" que abre, em tela
//	cheia (PC e celular), os três documentos que provam o recebimento —
//	como se fossem folhas A4 empilhadas: a OC, o romaneio escaneado, e as
//	fotos do equipamento (numa grade que aproveita a folha, no PC; uma
//	embaixo da outra, sem girar, no celular — ver o cabeçalho de
//	`composicaoLocacao.ts`, que decide o layout).
//
// A FOLHA SÓ EXISTE NA TELA — VIRA PDF SÓ QUANDO PEDIDO
//
//	A prévia usa os links do armazém direto (`<img src>`), o jeito mais
//	barato de mostrar — o navegador busca e mostra sozinho. Só ao clicar em
//	"Extrair PDF"/"Enviar" é que cada imagem é baixada de verdade e
//	convertida pra JPEG (`carregarComoJPEG`), porque o jsPDF precisa dos
//	bytes na mão, não de um endereço.
//
// EXTRAIR (PC) VS ENVIAR (CELULAR) — MESMO PDF, DOIS DESTINOS
//
//	No PC, "Extrair PDF" baixa o arquivo — é assim que documento se leva
//	pra outro lugar num computador. No celular, baixar um PDF e depois abrir
//	o app certo pra mandar é um caminho longo; `navigator.share` entrega o
//	arquivo direto pro atalho nativo (WhatsApp, e-mail, Drive) — o que a
//	pessoa já usa pra mandar qualquer coisa do celular.
import { useEffect, useMemo, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { usePedirFoco } from '../../componentes/Foco'
import { useVoltarLocal } from '../../componentes/VoltarLocal'
import { useEhMobile } from '../../componentes/useEhMobile'
import {
  A4_ALTURA_MM, A4_LARGURA_MM, carregarComoJPEG, montarFolhasDeFotos, montarFolhasDeFotosMobile,
  ocComoJPEG, type Folha, type FotoOrientada,
} from './composicaoLocacao'
import type { NotaFiscal } from './tipos'

interface Composicao {
  oc: { url: string; nome: string }
  romaneio: { paginas: string[] }
  fotos: { id: string; url: string }[]
}

interface Props {
  nota: NotaFiscal
  aoFechar: () => void
}

function folhaDeImagemUnica(url: string): Folha {
  return [{ url, xMM: 0, yMM: 0, larguraMM: A4_LARGURA_MM, alturaMM: A4_ALTURA_MM }]
}

/** Só a dimensão — leve, sem baixar nem converter nada (a prévia usa o link direto). */
function detectarOrientacao(url: string): Promise<boolean> {
  return new Promise(resolve => {
    const img = new Image()
    img.onload = () => resolve(img.naturalHeight > img.naturalWidth)
    img.onerror = () => resolve(false)
    img.src = url
  })
}

export function VisualizarComposicaoLocacao({ nota, aoFechar }: Props) {
  const ehMobile = useEhMobile()
  usePedirFoco()
  useVoltarLocal(aoFechar)

  const [dados, setDados] = useState<Composicao | null>(null)
  const [erro, setErro] = useState('')
  const [fotosOrientadas, setFotosOrientadas] = useState<FotoOrientada[] | null>(null)
  const [ocDataURL, setOcDataURL] = useState<string | null>(null)
  const [erroOC, setErroOC] = useState(false)
  const [exportando, setExportando] = useState(false)

  useEffect(() => {
    let vivo = true
    void (async () => {
      try {
        const r = await motor<Composicao>(`/administrativo/nf/notas/${nota.id}/composicao-locacao`)
        if (vivo) setDados(r)
      } catch (e) {
        if (vivo) setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a composição.')
      }
    })()
    return () => { vivo = false }
  }, [nota.id])

  // Orientação de cada foto — só pra montar a grade da prévia.
  useEffect(() => {
    if (!dados) return
    let vivo = true
    void Promise.all(dados.fotos.map(async f => ({ url: f.url, retrato: await detectarOrientacao(f.url) })))
      .then(classificadas => { if (vivo) setFotosOrientadas(classificadas) })
    return () => { vivo = false }
  }, [dados])

  // A OC rasterizada uma vez — a prévia mostra, e a exportação reaproveita.
  useEffect(() => {
    if (!dados?.oc.url) return
    let vivo = true
    setErroOC(false)
    void ocComoJPEG(dados.oc.url)
      .then(d => { if (vivo) setOcDataURL(d) })
      .catch(() => { if (vivo) setErroOC(true) })
    return () => { vivo = false }
  }, [dados])

  const folhas = useMemo<Folha[] | null>(() => {
    if (!dados || !fotosOrientadas) return null
    const ocFolha = ocDataURL ? [folhaDeImagemUnica(ocDataURL)] : []
    const romaneioFolhas = dados.romaneio.paginas.map(folhaDeImagemUnica)
    const fotosFolhas = ehMobile ? montarFolhasDeFotosMobile(fotosOrientadas) : montarFolhasDeFotos(fotosOrientadas)
    return [...ocFolha, ...romaneioFolhas, ...fotosFolhas]
  }, [dados, ocDataURL, fotosOrientadas, ehMobile])

  async function extrairOuEnviar() {
    if (!dados) return
    setExportando(true)
    setErro('')
    try {
      const [{ jsPDF }, ocJPEG] = await Promise.all([
        import('jspdf'),
        ocDataURL ? Promise.resolve(ocDataURL) : ocComoJPEG(dados.oc.url),
      ])
      const pdf = new jsPDF({ unit: 'mm', format: 'a4' })
      let primeira = true
      const novaPagina = () => { if (!primeira) pdf.addPage(); primeira = false }
      const desenharFolha = (folha: Folha) => {
        novaPagina()
        for (const item of folha) pdf.addImage(item.url, 'JPEG', item.xMM, item.yMM, item.larguraMM, item.alturaMM)
      }

      desenharFolha(folhaDeImagemUnica(ocJPEG))

      for (const url of dados.romaneio.paginas) {
        const { dataURL } = await carregarComoJPEG(url)
        desenharFolha(folhaDeImagemUnica(dataURL))
      }

      // As fotos precisam dos bytes de verdade — a prévia usa o link direto,
      // que o jsPDF não aceita.
      const fotosComBytes: FotoOrientada[] = []
      for (const foto of dados.fotos) {
        const { dataURL, retrato } = await carregarComoJPEG(foto.url)
        fotosComBytes.push({ url: dataURL, retrato })
      }
      const folhasDeFotos = ehMobile ? montarFolhasDeFotosMobile(fotosComBytes) : montarFolhasDeFotos(fotosComBytes)
      for (const folha of folhasDeFotos) desenharFolha(folha)

      const nomeArquivo = `locacao-oc-${nota.ordem_numero ?? nota.id}.pdf`
      const compartilhavel = ehMobile && typeof navigator.share === 'function' && typeof navigator.canShare === 'function'
      if (compartilhavel) {
        const blob = pdf.output('blob') as Blob
        const arquivo = new File([blob], nomeArquivo, { type: 'application/pdf' })
        if (navigator.canShare({ files: [arquivo] })) {
          await navigator.share({ files: [arquivo], title: nomeArquivo })
        } else {
          pdf.save(nomeArquivo)
        }
      } else {
        pdf.save(nomeArquivo)
      }
    } catch (e) {
      // AbortError é a pessoa fechando a folha de compartilhamento sem
      // escolher nada — desistência, não erro (mesmo espírito de
      // `salvarArquivo` em motor/cliente.ts).
      if ((e as { name?: string })?.name !== 'AbortError') {
        setErro('Não consegui montar o PDF. Tente de novo.')
      }
    } finally {
      setExportando(false)
    }
  }

  const carregando = !dados || !fotosOrientadas || (!ocDataURL && !erroOC)

  return (
    <div className="orc-tela">
      <div className="orc-barra">
        <button type="button" className="orc-voltar" onClick={aoFechar}>← voltar</button>
        <b>O.C. {nota.ordem_numero ?? '—'} — recebimento de locação</b>
        <span className="orc-ficha-direita">
          <button type="button" className="orc-bt" disabled={carregando || exportando} onClick={() => void extrairOuEnviar()}>
            {exportando ? 'Montando...' : ehMobile ? 'Enviar' : 'Extrair PDF'}
          </button>
        </span>
      </div>

      {erro && <p className="erro">{erro}</p>}
      {erroOC && <p className="dica">Não consegui abrir a OC — o romaneio e as fotos continuam abaixo.</p>}

      <div className="loc-comp-corpo">
        {carregando && !erro ? (
          <p className="orc-vazio">abrindo os documentos…</p>
        ) : (
          folhas?.map((folha, i) => (
            <div key={i} className="loc-folha">
              {folha.map((item, j) => (
                <img
                  key={j}
                  src={item.url}
                  alt=""
                  className="loc-folha-item"
                  style={{
                    left: `${(item.xMM / A4_LARGURA_MM) * 100}%`,
                    top: `${(item.yMM / A4_ALTURA_MM) * 100}%`,
                    width: `${(item.larguraMM / A4_LARGURA_MM) * 100}%`,
                    height: `${(item.alturaMM / A4_ALTURA_MM) * 100}%`,
                  }}
                />
              ))}
            </div>
          ))
        )}
      </div>
    </div>
  )
}
