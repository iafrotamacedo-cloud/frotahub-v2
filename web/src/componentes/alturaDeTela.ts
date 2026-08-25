// rev 1 — a lista termina no limite de baixo do campo de visão
//
// O PROBLEMA
//
//	A área de trabalho cresce com o conteúdo: uma lista de 500 linhas empurra a
//	página para baixo e a rolagem passa a ser da PÁGINA inteira. Aí o rodapé de
//	paginação e a barra de rolagem horizontal da tabela ficam lá embaixo, fora do
//	campo de visão — e para trocar de página é preciso rolar até o fim antes.
//
// A CONTA É MEDIDA, NÃO CHUTADA
//
//	Um `calc(100vh - 180px)` funciona na janela de quem escreveu e erra em todas
//	as outras: a barra de filtros muda de altura conforme a largura, o rodapé
//	também. Medir o topo do próprio elemento e o que existe abaixo dele acerta em
//	qualquer tamanho de janela.
import { useLayoutEffect, type RefObject } from 'react'

/**
 * Faz o elemento terminar exatamente na borda de baixo da janela.
 *
 * @param alvo      o painel que rola por dentro
 * @param recalcular quando refazer a conta (dados novos, tela trocada)
 */
export function useAlturaDeTela(alvo: RefObject<HTMLElement | null>, recalcular: unknown[] = []) {
  useLayoutEffect(() => {
    function ajustar() {
      const el = alvo.current
      if (!el) return
      const topo = el.getBoundingClientRect().top
      const conteudo = el.closest('.content') as HTMLElement | null
      const respiro = conteudo
        ? parseFloat(getComputedStyle(conteudo).paddingBottom || '24')
        : 24
      // É `maxHeight`, e não `height`: o painel é um item flex que cresce com o
      // conteúdo, e uma altura fixa brigaria com o flex — a lista curta ficaria
      // com um vazio embaixo. O teto deixa a lista curta encolher e só segura a
      // longa.
      //
      // O mínimo é generoso de propósito: numa janela muito baixa é melhor a
      // página rolar do que a lista virar uma fresta de três linhas.
      el.style.maxHeight = Math.max(280, window.innerHeight - topo - respiro - 18) + 'px'
    }
    ajustar()
    window.addEventListener('resize', ajustar)
    return () => window.removeEventListener('resize', ajustar)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, recalcular)
}
