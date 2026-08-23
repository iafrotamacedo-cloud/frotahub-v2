// rev 1 — o endereço do navegador é o lugar onde a pessoa está
//
// O PROBLEMA QUE ISTO RESOLVE
//   Antes, a tela aberta vivia só na memória da página. O endereço nunca mudava,
//   então o navegador não sabia que alguém tinha andado dentro do sistema: apertar
//   "voltar" saía do FrotaHub e ia para o site anterior. É o tipo de coisa que faz
//   perder o que se estava vendo por um reflexo — e todo mundo tem esse reflexo.
//
//   Com o caminho no endereço, "voltar" volta uma tela, "avançar" avança, recarregar
//   a página cai no mesmo lugar, e um endereço colado numa conversa abre onde deve.
//
// POR QUE COM `#` NO ENDEREÇO
//   `novo.frotamacedo.com.br/#/configuracoes/usuarios` em vez de
//   `novo.frotamacedo.com.br/configuracoes/usuarios`.
//
//   O sistema é um arquivo só, servido pelo HostGator. Com endereço sem `#`, quem
//   recarregasse a página numa tela interna receberia um 404 do servidor, porque
//   aquela pasta não existe lá — só existe dentro do programa. Consertar isso pede
//   uma regra de reescrita no servidor, mais uma peça para configurar e mais uma
//   coisa para quebrar em silêncio.
//
//   O `#` não vai ao servidor. Funciona sem configurar nada. O preço é estético, e
//   é reversível: no dia em que a regra do servidor existir, some daqui e não muda
//   mais nada.
import { useCallback, useEffect, useState } from 'react'
import type { ItemMenu } from './arvore'

export function caminhoParaEndereco(caminho: ItemMenu[]): string {
  return '#/' + caminho.map(i => i.rota).join('/')
}

/**
 * Lê o endereço e devolve o caminho correspondente DENTRO da árvore recebida.
 *
 * Como a árvore já vem filtrada pelo que este login alcança, um endereço para uma
 * tela que a pessoa não pode ver simplesmente não resolve, e ela cai no início.
 * A proteção sai de graça: não é preciso conferir permissão outra vez aqui.
 */
export function enderecoParaCaminho(endereco: string, arvore: ItemMenu[]): ItemMenu[] {
  const partes = endereco.replace(/^#\/?/, '').split('/').filter(Boolean)
  const caminho: ItemMenu[] = []
  let nivel = arvore

  for (const parte of partes) {
    const achado = nivel.find(i => i.rota === parte)
    if (!achado) return []
    caminho.push(achado)
    nivel = achado.sub ?? []
  }
  return caminho
}

/**
 * O caminho aberto, guardado no endereço.
 *
 * O estado aqui é o TEXTO do endereço, não a lista de itens de menu. Assim, quando
 * a árvore muda — alguém entra, alguém sai, um login com outro alcance — o caminho
 * é recalculado sozinho a partir do mesmo endereço, em vez de continuar apontando
 * para objetos de menu antigos.
 */
export function useNavegacao(arvore: ItemMenu[]) {
  const [endereco, setEndereco] = useState(() => window.location.hash)

  useEffect(() => {
    function mudou() { setEndereco(window.location.hash) }
    window.addEventListener('hashchange', mudou)
    // Garante que o primeiro desenho já reflita o endereço, inclusive quando a
    // página abriu direto numa tela interna.
    mudou()
    return () => window.removeEventListener('hashchange', mudou)
  }, [])

  const caminho = enderecoParaCaminho(endereco, arvore)

  const navegar = useCallback((novo: ItemMenu[]) => {
    const alvo = caminhoParaEndereco(novo)
    if (window.location.hash === alvo) return
    // Trocar o endereço é o que cria a entrada no histórico do navegador. O
    // `hashchange` volta para cá e atualiza a tela — um caminho só (CORE-06).
    window.location.hash = alvo
  }, [])

  return { caminho, navegar }
}
