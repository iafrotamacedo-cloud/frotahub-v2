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
// FATURAMENTO — só soma altura pro que vem depois; a partir de 11/09/2026
// faturamento errado não se edita mais aqui (vira substituição de arquivo,
// ver `substituicao.go` no motor), então a região não é exportada.
y += OC_FAIXA_H + 7.6 + OC_LINHA // nome
y += OC_LINHA + 25.5 // I.E. + espaço
// FORNECEDOR
y += OC_FAIXA_H + 4.6 + OC_LINHA // nome
const TOPO_FORN_CNPJ = y
y += OC_LINHA * 3 + 32.0 // tel, vend, e-mail + espaço → faixa obra
// OBRA
const TOPO_OBRA = y
// ENDEREÇO DE COBRANÇA — o rótulo "ENDEREÇO COBRANÇA:"/"Endereço:" fica logo
// depois da faixa da obra; a altura cobre o rótulo + até duas linhas de
// endereço quebrado (a maioria não quebra mais que isso).
const TOPO_ENDERECO_COBRANCA = TOPO_OBRA + OC_FAIXA_H + 2.1

/** Linha do CNPJ do fornecedor (valor à esq. da coluna Endereço). */
export const REGIAO_FORNECEDOR: RetanguloOC = retangulo(TOPO_FORN_CNPJ - 1, OC_LINHA + 2, 85, 355)

/** Rótulo + valor de ENDEREÇO COBRANÇA — metade direita da página, abaixo da
 *  faixa OBRA/CENTRO DE CUSTO. */
export const REGIAO_ENDERECO_COBRANCA: RetanguloOC = retangulo(TOPO_ENDERECO_COBRANCA, OC_LINHA * 3 + 10, 297, OC_X1)
