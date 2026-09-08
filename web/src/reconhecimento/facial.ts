// rev 2 — o motor do reconhecimento facial
//
// Uma peça só, para toda tela que um dia precisar de rosto como fator de acesso
// (CORE-06): hoje é o login opcional em Minha conta; amanhã pode ser ponto ou
// conferência de documento — quem vier depois reaproveita isto, não reescreve.
//
// POR QUE RODA NO NAVEGADOR, E NÃO NUM SERVIÇO DE FORA
//
//   Duas razões, e as duas são tenets do sistema. CORE-01: o caminho mais barato
//   que resolve vem primeiro — aqui o mais barato é o único que existe, porque
//   uma rede local não cobra por captura, ao contrário de um serviço de nuvem de
//   reconhecimento facial. CORE-09: a foto do rosto nunca precisaria sair do
//   aparelho de quem está entrando — mandar para um serviço de fora seria abrir
//   uma porta que não existe hoje.
//
// O QUE FICA GRAVADO NÃO É A FOTO
//
//   `face-api.js` reduz o rosto detectado a 128 números (o "descritor"). Não dá
//   para reconstruir uma foto a partir deles — é o que viaja para o banco
//   (migração 057), nunca a imagem em si.
//
// POR QUE `distancia`/`ehOMesmoRosto` SAÍRAM DAQUI NA REVISÃO 2
//
//   Este arquivo importa `face-api.js`, que carrega ~1 MB de TensorFlow.js.
//   Comparar dois moldes já capturados não precisa da rede — só de aritmética.
//   Quem só compara (a tela de login) importa `./distancia`, e nunca baixa o
//   peso todo por causa de uma subtração. Quem CAPTURA (este arquivo) já
//   precisa da rede mesmo, então reexporta as duas por conveniência.
import * as faceapi from 'face-api.js'

export { distancia, ehOMesmoRosto, LIMIAR_MESMO_ROSTO } from './distancia'

// Onde os pesos da rede vivem — arquivos estáticos, publicados junto com o
// resto do site (o mesmo HostGator de sempre). Nada disso é chamada de rede
// para fora do domínio do FrotaHub.
const CAMINHO_MODELOS = '/modelos-face'

let carregamento: Promise<void> | null = null

/** Carrega os três modelos uma vez só; chamadas seguintes reaproveitam a mesma promessa. */
export function carregarModelos(): Promise<void> {
  if (!carregamento) {
    carregamento = Promise.all([
      faceapi.nets.tinyFaceDetector.loadFromUri(CAMINHO_MODELOS),
      faceapi.nets.faceLandmark68Net.loadFromUri(CAMINHO_MODELOS),
      faceapi.nets.faceRecognitionNet.loadFromUri(CAMINHO_MODELOS),
    ]).then(() => undefined)
  }
  return carregamento
}

/**
 * Acha UM rosto no vídeo e devolve o descritor dele — ou `null` se não achou
 * nenhum rosto, ou achou mais de um (P-18: quem chama decide a frase; aqui só
 * o fato).
 */
export async function descritorDoVideo(video: HTMLVideoElement): Promise<Float32Array | null> {
  await carregarModelos()
  const deteccoes = await faceapi
    .detectAllFaces(video, new faceapi.TinyFaceDetectorOptions())
    .withFaceLandmarks()
    .withFaceDescriptors()
  if (deteccoes.length !== 1) return null
  return deteccoes[0].descriptor
}
