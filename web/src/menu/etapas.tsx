// rev 1 — a árvore de menus vira barras do painel
//
// POR QUE A CONVERSÃO MORA AQUI, E NÃO DENTRO DO `Painel`
//
//	O `Painel` não conhece o FrotaHub: ele desenha barras. Quem conhece a árvore
//	de menus é este arquivo. Assim o mesmo componente serve para o menu de
//	navegação (sem contador) e para o painel de trabalho de Orçamentos (com
//	contador, prévia e sub-botões), sem que um saiba do outro.
import { Icone } from '../componentes/Icone'
import type { Etapa } from '../componentes/Painel'
import type { ItemMenu } from './arvore'

/**
 * Transforma itens de menu em barras.
 *
 * Item de navegação NÃO tem contador: "Configurações" não tem quantidade. Por
 * isso `numero` fica ausente — e ausente é diferente de zero. Um traço enorme
 * no lugar do número faria a pessoa procurar o que ele quer dizer.
 */
export function etapasDoMenu(itens: ItemMenu[]): Etapa[] {
  return itens.map(item => ({
    chave: item.rota,
    titulo: item.t,
    descricao: item.desc ?? '',
    icone: <Icone nome={item.icone} />,
    desabilitada: item.breve,
    selo: item.breve ? 'Em breve' : undefined,
  }))
}
