// rev 1 — achar a folha dentro do quadro da câmera, e recortá-la reta
//
// O QUE ISTO FAZ, EM UMA LINHA
//
//	Dado um quadro pequeno da câmera (≈ 240 px de largura), devolve os
//	quatro cantos da folha de papel que aparece nele — o "requadro" que o
//	app de Notas da Apple desenha por cima da imagem — e, na hora da
//	captura, recorta a foto em alta resolução por esses cantos, tirando a
//	perspectiva (a folha sai reta, como num scanner de mesa).
//
// SEM OPENCV, DE PROPÓSITO
//
//	O opencv.js tem ~8 MB; o almoxarife está na obra, no 4G. Tudo aqui é
//	TypeScript puro sobre `ImageData`: cinza → borrão → Sobel → Hough de
//	retas → quatro retas que fecham um quadrilátero. Roda em ~10 ms por
//	quadro num celular mediano, o que dá de sobra pra 8–10 quadros/s.
//
// POR QUE HOUGH, E NÃO CONTORNO
//
//	Achar contornos (o caminho do OpenCV) exige rastrear bordas pixel a
//	pixel e aproximar polígonos — muita conta e muito código. A folha é
//	um retângulo: quatro retas longas. A transformada de Hough acha retas
//	longas mesmo com a borda interrompida por uma mão ou uma sombra, e o
//	texto dentro da folha vira ruído curto, que não acumula voto. A única
//	esperteza é votar só na direção do gradiente de cada pixel de borda
//	(±10°), o que corta o custo em 10× e limpa o acumulador.

export interface Ponto { x: number; y: number }

/** Os quatro cantos, sempre nesta ordem: cima-esq, cima-dir, baixo-dir, baixo-esq. */
export type Quad = [Ponto, Ponto, Ponto, Ponto]

// ---------------------------------------------------------------------------
// cinza + borrão + gradiente
// ---------------------------------------------------------------------------

export function paraCinza(rgba: Uint8ClampedArray, w: number, h: number): Float32Array {
  const g = new Float32Array(w * h)
  for (let i = 0, j = 0; i < g.length; i++, j += 4) {
    g[i] = 0.299 * rgba[j] + 0.587 * rgba[j + 1] + 0.114 * rgba[j + 2]
  }
  return g
}

/** Borrão gaussiano 5×5 separável (1 4 6 4 1 / 16). */
function borrar(src: Float32Array, w: number, h: number): Float32Array {
  const tmp = new Float32Array(w * h)
  const out = new Float32Array(w * h)
  const k = [1, 4, 6, 4, 1]
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      let s = 0
      for (let i = -2; i <= 2; i++) {
        const xx = Math.min(w - 1, Math.max(0, x + i))
        s += src[y * w + xx] * k[i + 2]
      }
      tmp[y * w + x] = s / 16
    }
  }
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      let s = 0
      for (let i = -2; i <= 2; i++) {
        const yy = Math.min(h - 1, Math.max(0, y + i))
        s += tmp[yy * w + x] * k[i + 2]
      }
      out[y * w + x] = s / 16
    }
  }
  return out
}

interface Bordas {
  mag: Float32Array   // magnitude do gradiente
  ang: Float32Array   // direção do gradiente (rad, 0..π — a normal da reta)
}

function sobel(g: Float32Array, w: number, h: number): Bordas {
  const mag = new Float32Array(w * h)
  const ang = new Float32Array(w * h)
  for (let y = 1; y < h - 1; y++) {
    for (let x = 1; x < w - 1; x++) {
      const i = y * w + x
      const gx = -g[i - w - 1] + g[i - w + 1] - 2 * g[i - 1] + 2 * g[i + 1] - g[i + w - 1] + g[i + w + 1]
      const gy = -g[i - w - 1] - 2 * g[i - w] - g[i - w + 1] + g[i + w - 1] + 2 * g[i + w] + g[i + w + 1]
      mag[i] = Math.hypot(gx, gy)
      let a = Math.atan2(gy, gx)
      if (a < 0) a += Math.PI
      if (a >= Math.PI) a -= Math.PI
      ang[i] = a
    }
  }
  return { mag, ang }
}

// ---------------------------------------------------------------------------
// Hough
// ---------------------------------------------------------------------------

interface Reta { theta: number; rho: number; votos: number }

const PASSO_THETA = Math.PI / 120 // 1,5°
const N_THETA = 120

/**
 * Acha as retas mais votadas do mapa de bordas. Cada pixel de borda vota só
 * nos ângulos próximos da própria direção de gradiente.
 */
function hough(b: Bordas, w: number, h: number): Reta[] {
  const diag = Math.ceil(Math.hypot(w, h))
  const nRho = 2 * diag + 1
  const acc = new Float32Array(N_THETA * nRho)

  // Limiar adaptativo: só o que está bem acima da média de borda vota.
  let soma = 0, n = 0
  for (let i = 0; i < b.mag.length; i++) { if (b.mag[i] > 0) { soma += b.mag[i]; n++ } }
  const media = n ? soma / n : 0
  const limiar = Math.max(24, media * 2.2)

  const cosT = new Float32Array(N_THETA), sinT = new Float32Array(N_THETA)
  for (let t = 0; t < N_THETA; t++) { cosT[t] = Math.cos(t * PASSO_THETA); sinT[t] = Math.sin(t * PASSO_THETA) }

  const janela = 7 // ±7 bins = ±10,5°
  for (let y = 1; y < h - 1; y++) {
    for (let x = 1; x < w - 1; x++) {
      const i = y * w + x
      const m = b.mag[i]
      if (m < limiar) continue
      const t0 = Math.round(b.ang[i] / PASSO_THETA)
      for (let dt = -janela; dt <= janela; dt++) {
        let t = t0 + dt
        // θ vive em [0, π): ao passar da borda, o bin equivalente é θ∓π, e o
        // ρ desse bin sai da própria fórmula (troca de sinal sozinho).
        if (t < 0) t += N_THETA
        else if (t >= N_THETA) t -= N_THETA
        const rho = Math.round(x * cosT[t] + y * sinT[t]) + diag
        if (rho >= 0 && rho < nRho) acc[t * nRho + rho] += 1
      }
    }
  }

  // Picos: máximo local numa vizinhança de ±3 thetas × ±4 rhos.
  let maximo = 0
  for (let i = 0; i < acc.length; i++) if (acc[i] > maximo) maximo = acc[i]
  const minimoVotos = Math.max(0.18 * maximo, 0.12 * Math.min(w, h))
  const picos: Reta[] = []
  for (let t = 0; t < N_THETA; t++) {
    for (let r = 0; r < nRho; r++) {
      const v = acc[t * nRho + r]
      if (v < minimoVotos) continue
      let ehPico = true
      for (let dt = -3; dt <= 3 && ehPico; dt++) {
        let tt = t + dt
        let esp = 0
        if (tt < 0) { tt += N_THETA; esp = 1 }
        else if (tt >= N_THETA) { tt -= N_THETA; esp = 1 }
        for (let dr = -4; dr <= 4; dr++) {
          if (dt === 0 && dr === 0) continue
          // Ao cruzar 0/π o rho troca de sinal.
          const rr = esp ? (2 * diag - (r + dr)) : (r + dr)
          if (rr < 0 || rr >= nRho) continue
          const o = acc[tt * nRho + rr]
          if (o > v || (o === v && (dt < 0 || (dt === 0 && dr < 0)))) { ehPico = false; break }
        }
      }
      if (ehPico) picos.push({ theta: t * PASSO_THETA, rho: r - diag, votos: v })
    }
  }
  picos.sort((a, c) => c.votos - a.votos)
  return picos.slice(0, 16)
}

// ---------------------------------------------------------------------------
// de retas a quadrilátero
// ---------------------------------------------------------------------------

/** Diferença angular entre duas direções de reta (0..π/2). */
function difAngulo(a: number, b: number): number {
  let d = Math.abs(a - b) % Math.PI
  if (d > Math.PI / 2) d = Math.PI - d
  return d
}

function intersecao(a: Reta, b: Reta): Ponto | null {
  const ca = Math.cos(a.theta), sa = Math.sin(a.theta)
  const cb = Math.cos(b.theta), sb = Math.sin(b.theta)
  const det = ca * sb - sa * cb
  if (Math.abs(det) < 1e-6) return null
  return {
    x: (a.rho * sb - b.rho * sa) / det,
    y: (ca * b.rho - cb * a.rho) / det,
  }
}

function areaPoligono(p: Ponto[]): number {
  let s = 0
  for (let i = 0; i < p.length; i++) {
    const a = p[i], b = p[(i + 1) % p.length]
    s += a.x * b.y - b.x * a.y
  }
  return Math.abs(s) / 2
}

/** Ordena quatro pontos quaisquer em cima-esq, cima-dir, baixo-dir, baixo-esq. */
export function ordenarQuad(pts: Ponto[]): Quad {
  const cx = pts.reduce((s, p) => s + p.x, 0) / 4
  const cy = pts.reduce((s, p) => s + p.y, 0) / 4
  // Com y crescendo para baixo, atan2 crescente é o sentido HORÁRIO na tela.
  const ordem = [...pts].sort((a, b) => Math.atan2(a.y - cy, a.x - cx) - Math.atan2(b.y - cy, b.x - cx))
  // Começa pelo canto mais "cima-esquerda" (menor x+y) e segue no horário:
  // cima-esq → cima-dir → baixo-dir → baixo-esq.
  let ini = 0
  for (let i = 1; i < 4; i++) if (ordem[i].x + ordem[i].y < ordem[ini].x + ordem[ini].y) ini = i
  return [ordem[ini], ordem[(ini + 1) % 4], ordem[(ini + 2) % 4], ordem[(ini + 3) % 4]]
}

function anguloInterno(a: Ponto, v: Ponto, b: Ponto): number {
  const ax = a.x - v.x, ay = a.y - v.y, bx = b.x - v.x, by = b.y - v.y
  const c = (ax * bx + ay * by) / (Math.hypot(ax, ay) * Math.hypot(bx, by) || 1)
  return Math.acos(Math.max(-1, Math.min(1, c)))
}

/**
 * Detecta a folha num quadro em cinza (w×h pequeno). Devolve os cantos no
 * espaço deste quadro, ou null se não houver um retângulo convincente.
 */
export function detectarDocumento(cinza: Float32Array, w: number, h: number): Quad | null {
  const bordas = sobel(borrar(cinza, w, h), w, h)
  const retas = hough(bordas, w, h)
  if (retas.length < 4) return null

  const areaQuadro = w * h
  const margem = 0.12 * Math.max(w, h)
  const minLado = 0.22 * Math.min(w, h)
  let melhor: { quad: Quad; nota: number } | null = null

  const n = retas.length
  for (let i = 0; i < n; i++) {
    for (let j = i + 1; j < n; j++) {
      // Par 1: quase paralelas e separadas.
      if (difAngulo(retas[i].theta, retas[j].theta) > Math.PI / 7) continue
      for (let k = 0; k < n; k++) {
        if (k === i || k === j) continue
        // Par 2 é ~perpendicular ao par 1.
        if (difAngulo(retas[i].theta, retas[k].theta) < Math.PI / 3) continue
        for (let l = k + 1; l < n; l++) {
          if (l === i || l === j) continue
          if (difAngulo(retas[k].theta, retas[l].theta) > Math.PI / 7) continue

          const p1 = intersecao(retas[i], retas[k]), p2 = intersecao(retas[i], retas[l])
          const p3 = intersecao(retas[j], retas[k]), p4 = intersecao(retas[j], retas[l])
          if (!p1 || !p2 || !p3 || !p4) continue
          const pts = [p1, p2, p3, p4]
          if (pts.some(p => p.x < -margem || p.y < -margem || p.x > w + margem || p.y > h + margem)) continue
          const quad = ordenarQuad(pts)
          const area = areaPoligono(quad)
          if (area < 0.12 * areaQuadro || area > 0.98 * areaQuadro) continue
          // Lados mínimos e cantos entre 55° e 125°: descarta "quadriláteros" degenerados.
          let ok = true
          for (let c = 0; c < 4 && ok; c++) {
            const a = quad[c], b = quad[(c + 1) % 4], z = quad[(c + 3) % 4]
            if (Math.hypot(b.x - a.x, b.y - a.y) < minLado) ok = false
            const ang = anguloInterno(z, a, b)
            if (ang < Math.PI * 55 / 180 || ang > Math.PI * 125 / 180) ok = false
          }
          if (!ok) continue
          const votos = retas[i].votos + retas[j].votos + retas[k].votos + retas[l].votos
          const nota = votos * Math.sqrt(area / areaQuadro)
          if (!melhor || nota > melhor.nota) melhor = { quad, nota }
        }
      }
    }
  }
  return melhor ? melhor.quad : null
}

// ---------------------------------------------------------------------------
// recorte em perspectiva (homografia de 4 pontos, amostragem bilinear)
// ---------------------------------------------------------------------------

/** Resolve a homografia que leva (0,0),(W,0),(W,H),(0,H) nos cantos `q`. */
function homografiaPara(q: Quad, W: number, H: number): Float64Array {
  const src: Ponto[] = [{ x: 0, y: 0 }, { x: W, y: 0 }, { x: W, y: H }, { x: 0, y: H }]
  // 8 equações, 8 incógnitas (h33 = 1).
  const A: number[][] = []
  const B: number[] = []
  for (let i = 0; i < 4; i++) {
    const { x, y } = src[i], { x: u, y: v } = q[i]
    A.push([x, y, 1, 0, 0, 0, -u * x, -u * y]); B.push(u)
    A.push([0, 0, 0, x, y, 1, -v * x, -v * y]); B.push(v)
  }
  // Eliminação de Gauss com pivô parcial.
  const n = 8
  for (let c = 0; c < n; c++) {
    let piv = c
    for (let r = c + 1; r < n; r++) if (Math.abs(A[r][c]) > Math.abs(A[piv][c])) piv = r
    ;[A[c], A[piv]] = [A[piv], A[c]]; [B[c], B[piv]] = [B[piv], B[c]]
    const d = A[c][c] || 1e-12
    for (let r = c + 1; r < n; r++) {
      const f = A[r][c] / d
      if (!f) continue
      for (let k = c; k < n; k++) A[r][k] -= f * A[c][k]
      B[r] -= f * B[c]
    }
  }
  const hv = new Float64Array(9)
  for (let r = n - 1; r >= 0; r--) {
    let s = B[r]
    for (let k = r + 1; k < n; k++) s -= A[r][k] * hv[k]
    hv[r] = s / (A[r][r] || 1e-12)
  }
  hv[8] = 1
  return hv
}

/**
 * Recorta `origem` pelos cantos `quad` (nas coordenadas da própria origem)
 * e devolve um canvas com a folha reta. O tamanho de saída segue o tamanho
 * real dos lados no quadro, limitado a `ladoMax`.
 */
export function recortarPerspectiva(origem: ImageData, quad: Quad, ladoMax: number): HTMLCanvasElement {
  const [tl, tr, br, bl] = quad
  let W = Math.round(Math.max(Math.hypot(tr.x - tl.x, tr.y - tl.y), Math.hypot(br.x - bl.x, br.y - bl.y)))
  let H = Math.round(Math.max(Math.hypot(bl.x - tl.x, bl.y - tl.y), Math.hypot(br.x - tr.x, br.y - tr.y)))
  const f = ladoMax / Math.max(W, H)
  if (f < 1) { W = Math.round(W * f); H = Math.round(H * f) }
  W = Math.max(8, W); H = Math.max(8, H)

  const hm = homografiaPara(quad, W, H)
  const sw = origem.width, sh = origem.height, sp = origem.data
  const saida = document.createElement('canvas')
  saida.width = W; saida.height = H
  const ctx = saida.getContext('2d')!
  const img = ctx.createImageData(W, H)
  const d = img.data
  let o = 0
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++, o += 4) {
      const den = hm[6] * x + hm[7] * y + 1
      const u = (hm[0] * x + hm[1] * y + hm[2]) / den
      const v = (hm[3] * x + hm[4] * y + hm[5]) / den
      const x0 = Math.floor(u), y0 = Math.floor(v)
      if (x0 < 0 || y0 < 0 || x0 >= sw - 1 || y0 >= sh - 1) {
        d[o] = d[o + 1] = d[o + 2] = 255; d[o + 3] = 255
        continue
      }
      const fx = u - x0, fy = v - y0
      const i00 = (y0 * sw + x0) * 4, i10 = i00 + 4, i01 = i00 + sw * 4, i11 = i01 + 4
      const w00 = (1 - fx) * (1 - fy), w10 = fx * (1 - fy), w01 = (1 - fx) * fy, w11 = fx * fy
      d[o] = sp[i00] * w00 + sp[i10] * w10 + sp[i01] * w01 + sp[i11] * w11
      d[o + 1] = sp[i00 + 1] * w00 + sp[i10 + 1] * w10 + sp[i01 + 1] * w01 + sp[i11 + 1] * w11
      d[o + 2] = sp[i00 + 2] * w00 + sp[i10 + 2] * w10 + sp[i01 + 2] * w01 + sp[i11 + 2] * w11
      d[o + 3] = 255
    }
  }
  ctx.putImageData(img, 0, 0)
  return saida
}

/** Escala um quad de um espaço (w1×h1) para outro (w2×h2). */
export function escalarQuad(q: Quad, w1: number, h1: number, w2: number, h2: number): Quad {
  const fx = w2 / w1, fy = h2 / h1
  return q.map(p => ({ x: p.x * fx, y: p.y * fy })) as Quad
}

/** Distância média entre cantos correspondentes — a medida de "parou de mexer". */
export function distanciaQuad(a: Quad, b: Quad): number {
  let s = 0
  for (let i = 0; i < 4; i++) s += Math.hypot(a[i].x - b[i].x, a[i].y - b[i].y)
  return s / 4
}

/** Média ponderada entre o quad anterior e o novo (suaviza o tremor da mão). */
export function suavizarQuad(anterior: Quad | null, novo: Quad, peso: number): Quad {
  if (!anterior) return novo
  return novo.map((p, i) => ({
    x: anterior[i].x + (p.x - anterior[i].x) * peso,
    y: anterior[i].y + (p.y - anterior[i].y) * peso,
  })) as Quad
}
