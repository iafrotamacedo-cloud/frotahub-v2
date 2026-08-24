// rev 1 — a letra que diminui sozinha para caber numa linha
//
// O PEDIDO
//   "onde o texto for grande, você diminui a letra automaticamente, para não
//   quebrar linhas."
//
// O JEITO ÓBVIO, E POR QUE ELE NÃO SERVE
//   O caminho natural é um laço: escreve, mede o elemento, se passou diminui um
//   pouco e mede de novo. Funciona — e trava. Cada leitura de `scrollWidth`
//   obriga o navegador a recalcular o layout ali mesmo; com 500 linhas, duas
//   colunas e uma dezena de tentativas por célula, são mais de dez mil
//   recálculos numa troca de página. A tabela congela por segundos.
//
// O JEITO DAQUI
//   A largura de um texto é PROPORCIONAL ao tamanho da letra na mesma fonte. Então
//   basta medir UMA vez, num canvas — que não toca no layout da página — e dividir:
//
//       tamanho = base × (largura disponível ÷ largura medida)
//
//   Uma conta por célula, nenhum recálculo de layout, e o mesmo resultado visual.
//   A largura da coluna também é lida uma vez só por coluna, e não por célula:
//   todas as células de uma coluna têm a mesma largura.

/**
 * O menor tamanho aceitável.
 *
 * Escolhido depois de ver a tela pronta (P-26): com o piso em 8,6px, uma
 * descrição de 240 caracteres encolhia até virar um borrão — e AINDA assim não
 * cabia. Encolher além deste ponto não resolve o problema, só troca "texto
 * cortado" por "texto ilegível e cortado".
 *
 * A regra que ficou: encolhe até 10,5px, que dá conta do texto que quase cabia
 * — o caso que o pedido tinha em mente. O que passa muito disso ganha reticências
 * e o texto inteiro no repouso do ponteiro; e completo mesmo, na ficha.
 */
const MINIMO = 10.5

let pincel: CanvasRenderingContext2D | null = null
let familia = ''

function contexto(): CanvasRenderingContext2D | null {
  if (pincel) return pincel
  if (typeof document === 'undefined') return null
  const tela = document.createElement('canvas')
  pincel = tela.getContext('2d')
  return pincel
}

/** A família de fontes real da página, para medir com a mesma letra que desenha. */
function familiaDaCasa(): string {
  if (familia) return familia
  if (typeof window === 'undefined') return 'sans-serif'
  const v = getComputedStyle(document.documentElement).getPropertyValue('--f').trim()
  familia = v || 'sans-serif'
  return familia
}

/**
 * O tamanho de letra que faz `texto` caber em `largura`, partindo de `base`.
 * Devolve `base` quando já cabe.
 */
export function tamanhoQueCabe(texto: string, largura: number, base: number, peso = 400): number {
  const ctx = contexto()
  if (!ctx || !texto || largura <= 0) return base

  ctx.font = `${peso} ${base}px ${familiaDaCasa()}`
  const medida = ctx.measureText(texto).width
  if (medida <= largura) return base

  // Arredondado para baixo em quartos de pixel: o navegador não desenha melhor
  // que isso, e passos menores só produziriam tamanhos diferentes entre células
  // que deveriam parecer iguais.
  const alvo = Math.floor((largura / medida) * base * 4) / 4
  return Math.max(MINIMO, alvo)
}

/**
 * Ajusta todas as células marcadas com `data-encolhe` dentro de `raiz`.
 *
 * A ordem importa: primeiro TODAS as leituras (uma por coluna), depois todas as
 * escritas. Ler e escrever alternadamente é o que provoca o recálculo em cascata
 * que este arquivo existe para evitar.
 */
export function ajustarCelulas(raiz: HTMLElement | null): void {
  if (!raiz) return
  const celulas = raiz.querySelectorAll<HTMLElement>('[data-encolhe]')
  if (celulas.length === 0) return

  // --- leituras
  const larguraDaColuna = new Map<string, number>()
  celulas.forEach(celula => {
    const coluna = celula.dataset.encolhe || '?'
    if (larguraDaColuna.has(coluna)) return
    const estilo = getComputedStyle(celula)
    const util = celula.clientWidth
      - parseFloat(estilo.paddingLeft || '0')
      - parseFloat(estilo.paddingRight || '0')
    larguraDaColuna.set(coluna, util)
  })

  // --- escritas
  celulas.forEach(celula => {
    const dentro = celula.firstElementChild as HTMLElement | null
    if (!dentro) return
    const largura = larguraDaColuna.get(celula.dataset.encolhe || '?') ?? 0
    const base = Number(celula.dataset.base || '13.2')
    const peso = Number(celula.dataset.peso || '400')
    const tamanho = tamanhoQueCabe(dentro.textContent || '', largura, base, peso)
    dentro.style.fontSize = tamanho === base ? '' : tamanho + 'px'
  })
}
