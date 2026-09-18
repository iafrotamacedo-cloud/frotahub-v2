// rev 1 — a composição de uma NF de locação: layout e captura (18/09/2026)
//
// O QUE O DONO PEDIU
//
//	Ver, em tela cheia, os três documentos que provam o recebimento de um
//	equipamento locado — a OC, o romaneio, as fotos — como se fossem folhas
//	A4 empilhadas. As fotos entram numa grade que aproveita a folha:
//
//	  paisagem sozinha → 3 por página (cada uma, 1/3 da altura)
//	  retrato sozinho  → 4 por página, 2×2
//	  misto            → retratos sempre emparelhados lado a lado (uma
//	                      fileira de meia página); cada paisagem é uma
//	                      fileira de 1/3 de página; enche a página com
//	                      fileiras até não caber mais uma inteira. Sobrou
//	                      1 retrato sem par? Fileira de meia página, só
//	                      do lado esquerdo — o direito fica em branco.
//
//	No celular não tem grade — item 3 é só uma foto embaixo da outra, sem
//	girar nenhuma (mais fácil de olhar na tela pequena, e evita adivinhar
//	orientação errado).
//
// POR QUE UM MÓDULO SÓ, SEM REACT
//
//	`montarFolhasDeFotos` decide a mesma coisa pra tela (que orientação cada
//	`<img>` mostrar, em que posição) e pro PDF exportado (`extrairPDF`,
//	`VisualizarComposicaoLocacao.tsx`) — se fossem dois cálculos, um dia
//	divergiriam sem ninguém perceber (CORE-06). Fica puro, testável sem
//	montar componente nenhum.
//
// `pdfjs-dist` ENTRA ESTÁTICO, NÃO SOB DEMANDA
//
//	`FolhaPdfCanvas.tsx` já importa estático — o pacote inteiro já viaja no
//	primeiro carregamento de qualquer tela que abra um PDF (reparo de OC,
//	por exemplo). Um `import()` aqui não adiaria nada de verdade (o bundler
//	avisa disso), só trocaria uma linha clara por uma promise sem ganho.
import * as pdfjsLib from 'pdfjs-dist'
import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorkerUrl

export const A4_LARGURA_MM = 210
export const A4_ALTURA_MM = 297

/** Um item posicionado numa folha A4, em milímetros. */
export interface ItemDaFolha {
  url: string
  xMM: number
  yMM: number
  larguraMM: number
  alturaMM: number
}

/** Uma folha é só a lista do que cai nela. */
export type Folha = ItemDaFolha[]

export interface FotoOrientada {
  url: string
  /** true = mais alta que larga. */
  retrato: boolean
}

/**
 * Empacota fotos (já classificadas retrato/paisagem) em folhas A4, seguindo
 * a regra do cabeçalho. Greedy: cada foto vira uma "fileira" (paisagem = a
 * folha inteira de largura, 1/3 de altura; retrato = a metade da largura,
 * junto do próximo retrato, 1/2 de altura) e as fileiras enchem a página
 * até não caber mais uma inteira.
 */
export function montarFolhasDeFotos(fotos: FotoOrientada[]): Folha[] {
  interface Fileira { alturaMM: number; larguraItemMM: number; urls: string[] }
  const fileiras: Fileira[] = []
  let pendente: FotoOrientada | null = null

  for (const foto of fotos) {
    if (!foto.retrato) {
      fileiras.push({ alturaMM: A4_ALTURA_MM / 3, larguraItemMM: A4_LARGURA_MM, urls: [foto.url] })
      continue
    }
    if (pendente) {
      fileiras.push({ alturaMM: A4_ALTURA_MM / 2, larguraItemMM: A4_LARGURA_MM / 2, urls: [pendente.url, foto.url] })
      pendente = null
    } else {
      pendente = foto
    }
  }
  // Retrato sem par: fileira de meia página, só a metade esquerda ocupada.
  if (pendente) {
    fileiras.push({ alturaMM: A4_ALTURA_MM / 2, larguraItemMM: A4_LARGURA_MM / 2, urls: [pendente.url] })
  }

  const folhas: Folha[] = []
  let atual: Folha = []
  let alturaUsada = 0
  const FOLGA_MM = 0.5 // arredondamento de ponto flutuante, não abre página à toa
  for (const fileira of fileiras) {
    if (atual.length > 0 && alturaUsada + fileira.alturaMM > A4_ALTURA_MM + FOLGA_MM) {
      folhas.push(atual)
      atual = []
      alturaUsada = 0
    }
    fileira.urls.forEach((url, i) => {
      atual.push({ url, xMM: i * fileira.larguraItemMM, yMM: alturaUsada, larguraMM: fileira.larguraItemMM, alturaMM: fileira.alturaMM })
    })
    alturaUsada += fileira.alturaMM
  }
  if (atual.length > 0) folhas.push(atual)
  return folhas
}

/** No celular, cada foto é a própria folha — sem grade, sem girar. */
export function montarFolhasDeFotosMobile(fotos: FotoOrientada[]): Folha[] {
  return fotos.map(f => [{ url: f.url, xMM: 0, yMM: 0, larguraMM: A4_LARGURA_MM, alturaMM: A4_ALTURA_MM }])
}

/**
 * Busca uma imagem (foto ou página de romaneio) e devolve os bytes já como
 * JPEG, prontos pro jsPDF — `fetch` + canvas, não `<img crossorigin>`: o
 * link do armazém é de outra origem, e um `<img>` com `crossorigin` exige
 * que o R2 responda CORS pro CARREGAMENTO da imagem (um modo de pedido
 * diferente do GET comum). `fetch` já é provado funcionando pra estes
 * mesmos links (`VisorDeDocumento.salvar`), e o blob local nunca tem
 * problema de origem — o `<img>` que lê a orientação aqui aponta pra ele,
 * não pro link de fora.
 */
export async function carregarComoJPEG(url: string): Promise<{ dataURL: string; retrato: boolean }> {
  const resposta = await fetch(url)
  if (!resposta.ok) throw new Error('não consegui baixar a imagem')
  const blob = await resposta.blob()
  const blobURL = URL.createObjectURL(blob)
  try {
    const img = await new Promise<HTMLImageElement>((resolve, reject) => {
      const el = new Image()
      el.onload = () => resolve(el)
      el.onerror = () => reject(new Error('não consegui abrir a imagem'))
      el.src = blobURL
    })
    const canvas = document.createElement('canvas')
    canvas.width = img.naturalWidth
    canvas.height = img.naturalHeight
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('sem contexto de canvas')
    // Fundo branco antes de desenhar: PNG com transparência viraria preto
    // dentro do PDF (JPEG não tem canal alfa).
    ctx.fillStyle = '#fff'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(img, 0, 0)
    return {
      dataURL: canvas.toDataURL('image/jpeg', 0.9),
      retrato: img.naturalHeight > img.naturalWidth,
    }
  } finally {
    URL.revokeObjectURL(blobURL)
  }
}

/**
 * Renderiza a página 1 de um PDF (a OC) num canvas próprio e devolve como
 * JPEG — mesma receita de `FolhaPdfCanvas.tsx` (pdf.js), numa resolução
 * fixa (não precisa medir container: aqui é só pra virar bytes de imagem).
 */
export async function ocComoJPEG(url: string): Promise<string> {
  const doc = await pdfjsLib.getDocument(url).promise
  const pagina = await doc.getPage(1)
  // ~150 dpi numa A4 retrato — nítido o bastante pra ler, sem pesar o PDF final.
  const ESCALA_ALVO_PX_LARGURA = 1240
  const base = pagina.getViewport({ scale: 1 })
  const escala = ESCALA_ALVO_PX_LARGURA / base.width
  const viewport = pagina.getViewport({ scale: escala })
  const canvas = document.createElement('canvas')
  canvas.width = viewport.width
  canvas.height = viewport.height
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('sem contexto de canvas')
  ctx.fillStyle = '#fff'
  ctx.fillRect(0, 0, canvas.width, canvas.height)
  await pagina.render({ canvasContext: ctx, viewport }).promise
  return canvas.toDataURL('image/jpeg', 0.9)
}
