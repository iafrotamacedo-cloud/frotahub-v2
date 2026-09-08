// rev 1 — comparar dois moldes, sem carregar a rede
//
// PROPOSITALMENTE separado de `facial.ts`: este arquivo NÃO importa a
// `face-api.js` (que arrasta ~1 MB de TensorFlow.js junto). A tela de
// verificação no login precisa comparar o molde capturado contra o salvo, mas
// NÃO precisa carregar a rede de novo — quem capturou (`CapturaFacial`, via
// `facial.ts`) já carregou. Importar só isto daqui evita que todo mundo que
// abre a tela de login baixe TensorFlow.js antes mesmo de saber se vai usar
// verificação facial (CORE-01: o caminho mais barato primeiro).
export const LIMIAR_MESMO_ROSTO = 0.6

/** A distância euclidiana entre dois descritores — quanto menor, mais parecido. */
export function distancia(a: ArrayLike<number>, b: ArrayLike<number>): number {
  let soma = 0
  for (let i = 0; i < a.length; i++) {
    const d = a[i] - b[i]
    soma += d * d
  }
  return Math.sqrt(soma)
}

/** Compara a captura contra o molde salvo, usando o mesmo limiar do sistema inteiro. */
export function ehOMesmoRosto(capturado: ArrayLike<number>, salvo: ArrayLike<number>): boolean {
  return distancia(capturado, salvo) <= LIMIAR_MESMO_ROSTO
}
