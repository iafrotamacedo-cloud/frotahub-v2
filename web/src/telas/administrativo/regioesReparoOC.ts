// rev 2 — caixas do reparo híbrido, calibradas no layout 019731 (documento_pdf.go)
//
// A4 retrato: 595 × 842 pt. `topo` no Go = distância do topo da página até a
// faixa/linha. Os retângulos abaixo ficam em PONTOS PDF (não mais em % de
// CSS) — desde a troca para VisorDePdfComOverlay.tsx (canvas + pdf.js), a
// caixa precisa da MESMA escala que o canvas usa pra desenhar, não de um
// percentual fixo de um contêiner proporcional. Ver `pixelsDoRetangulo`.

export const A4_LARGURA = 595
export const A4_ALTURA = 842

const OC_X0 = 19.44
const OC_X1 = 576
const OC_FAIXA_H = 18.03
const OC_LINHA = 14.4

export interface RetanguloOC {
  topo: number
  esquerda: number
  largura: number
  altura: number
}

function retangulo(topo: number, altura: number, esq = OC_X0, dir = OC_X1): RetanguloOC {
  return { topo, esquerda: esq, largura: dir - esq, altura }
}

/** Retângulo em pontos PDF → posição em pixel na escala atual do canvas. */
export function pixelsDoRetangulo(r: RetanguloOC, escala: number) {
  return {
    top: r.topo * escala,
    left: r.esquerda * escala,
    width: r.largura * escala,
    height: r.altura * escala,
  }
}

// Traço de blocosDaOC(e, topo=114.44) — ver documento_pdf.go
const TOPO_LETREIRO_FIM = 114.44
let y = TOPO_LETREIRO_FIM
// DADOS DA ORDEM
y += OC_FAIXA_H + 5.6 + OC_LINHA + OC_LINHA + 2.2 + 19.4
// RESPONSÁVEL
//
//	Só UMA linha de altura aqui (Nome/Comprador → Email), não duas — era um
//	OC_LINHA a mais, que empurrava Faturamento/Fornecedor/Obra ~14,4pt pra
//	baixo do lugar real. Achado renderizando OCs_Teste/OC_20021_* com
//	PyMuPDF e buscando o texto de verdade: "DADOS DO FATURAMENTO" está em
//	y≈260,45, não y≈273,1 (11/09/2026).
y += OC_FAIXA_H + 5.9 + OC_LINHA + 31.9
// FATURAMENTO
const TOPO_FATURAMENTO = y
y += OC_FAIXA_H + 7.6 + OC_LINHA // nome
const TOPO_FAT_CNPJ = y
y += OC_LINHA + 25.5 // I.E. + espaço
// FORNECEDOR
const TOPO_FORNECEDOR = y
y += OC_FAIXA_H + 4.6 + OC_LINHA // nome
const TOPO_FORN_CNPJ = y
y += OC_LINHA * 3 + 32.0 // tel, vend, e-mail + espaço → faixa obra
// OBRA
const TOPO_OBRA = y

/** Bloco inteiro DADOS DO FATURAMENTO (faixa + nome + CNPJ + I.E.). */
export const REGIAO_FATURAMENTO: RetanguloOC = retangulo(TOPO_FATURAMENTO, TOPO_FORNECEDOR - TOPO_FATURAMENTO)

/** Linha do CNPJ do fornecedor (valor à esq. da coluna Endereço). */
export const REGIAO_FORNECEDOR: RetanguloOC = retangulo(TOPO_FORN_CNPJ - 1, OC_LINHA + 2, 85, 355)

/** Faixa OBRA/CENTRO DE CUSTO + valor. */
export const REGIAO_OBRA: RetanguloOC = retangulo(TOPO_OBRA, OC_FAIXA_H + 1, OC_X0, 470)

/** Só a linha do CNPJ de faturamento (refino visual opcional). */
export const REGIAO_FAT_CNPJ: RetanguloOC = retangulo(TOPO_FAT_CNPJ - 1, OC_LINHA + 2, 85, 355)
