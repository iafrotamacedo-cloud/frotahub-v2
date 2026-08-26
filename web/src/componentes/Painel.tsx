// rev 2 — o painel de barras verticais
//
// ESTE É O PADRÃO DE MENU DO PROGRAMA
//
//	Decisão do dono em 25/08/2026: "esse esquema de menu que projetei aqui deve
//	ser usado em todos os menus do programa". Por isso ele nasce como componente
//	genérico em `componentes/`, e não como parte da tela de Orçamentos. A tela
//	descreve as etapas; o desenho mora aqui, uma vez só (CORE-06).
//
// POR QUE UM CONTADOR DENTRO DO BOTÃO
//
//	Cinco botões bonitos continuam sendo cinco botões — o usuário clica para
//	descobrir se há trabalho. Com o número na frente, a tela deixa de ser um
//	menu e vira um painel: dá para ver o serviço do dia antes do primeiro
//	clique. É o que o dono pediu com "fugir do óbvio com a intenção de facilitar
//	a operação".
//
// NEM TODA ETAPA É FILA
//
//	"Planilhas de controle" é um livro-razão: mostrar "0 na fila" ali leria como
//	"não tem nada aqui", que é o contrário da verdade. Por isso o rótulo do
//	número vem de fora, e uma etapa pode ser `viva: false` — cinza e quieta —
//	sem que isso signifique problema.
import { useState, type ReactNode } from 'react'

export interface LinhaDaPrevia {
  /** O que a linha é. Aparece à esquerda e encolhe com reticências. */
  texto: string
  /** O número ou a data da direita. Curto. */
  fim?: string
}

export interface Etapa {
  chave: string
  titulo: string
  descricao: string
  icone: ReactNode
  /**
   * O contador. `null` vira traço ("não há nenhum" é diferente de "não sei
   * quantos"); AUSENTE tira o número da barra inteira.
   *
   * Menu de navegação não tem fila para contar — "Configurações" não tem
   * quantidade. Nesses casos a barra fica só com ícone, título e descrição, e
   * o espaço do número não vira um traço solto pedindo explicação.
   */
  numero?: number | null
  rotulo?: string
  rodape?: string
  faixa?: string
  previaTitulo?: string
  previa?: LinhaDaPrevia[]
  /** Quando vazio, a prévia mostra esta frase em vez de sumir. */
  previaVazia?: string
  /** Barra apagada, que não abre. É o "Em breve" da árvore de menus. */
  desabilitada?: boolean
  /** Selo pequeno no rodapé — hoje só o "Em breve". */
  selo?: string
  /**
   * Quando existe, a etapa NÃO abre ao clicar: ela se parte em quatro ao passar
   * o mouse, e quem abre é cada filho.
   */
  filhos?: EtapaFilha[]
}

export interface EtapaFilha {
  chave: string
  titulo: string
  descricao: string
  numero: number | null
  previa?: LinhaDaPrevia[]
  previaVazia?: string
}

interface Props {
  etapas: Etapa[]
  aoEscolher: (chave: string) => void
}

export function Painel({ etapas, aoEscolher }: Props) {
  // MENU DE NAVEGAÇÃO É OUTRA COISA DE UM PAINEL DE TRABALHO
  //
  //	Sem contador e sem prévia, não há o que preencher a barra — e o desenho do
  //	painel (título lá embaixo, número lá em cima) deixa um vazio enorme no meio
  //	que não quer dizer nada. Aqui o rótulo sobe para junto do ícone e a barra
  //	ganha uma largura de gente: duas opções não devem virar dois muros de 700px.
  const simples = etapas.every(e => e.numero === undefined && !e.filhos?.length)

  // Qual etapa está aberta em quatro. Estado, e não `:hover` puro no CSS,
  // porque o teclado também precisa abrir — e `:focus-within` sozinho fecharia
  // a coluna assim que o foco passasse para um dos sub-botões.
  const [aberta, setAberta] = useState<string | null>(null)

  return (
    <div className={'pn' + (simples ? ' simples' : '')}>
      {etapas.map(e => {
        const temFilhos = !!e.filhos?.length
        const estaAberta = temFilhos && aberta === e.chave
        const viva = (e.numero ?? 0) > 0
        const temContador = e.numero !== undefined

        return (
          <div
            key={e.chave}
            className={'pn-col' + (viva ? ' viva' : '') + (estaAberta ? ' aberta' : '')
              + (e.desabilitada ? ' apagada' : '')}
            style={{ ['--faixa' as string]: e.faixa ?? (viva ? 'var(--red-h)' : 'var(--d-line2)') }}
            onMouseEnter={() => temFilhos && setAberta(e.chave)}
            onMouseLeave={() => temFilhos && setAberta(null)}
            onFocus={() => temFilhos && setAberta(e.chave)}
          >
            <button
              type="button"
              className="pn-corpo"
              // A descrição saiu da TELA, não do sistema: ela continua sendo o
              // rótulo que um leitor de tela anuncia. Tirar o texto de dentro do
              // botão é limpeza; tirar o significado seria perda.
              aria-label={e.descricao ? `${e.titulo} — ${e.descricao}` : e.titulo}
              disabled={e.desabilitada}
              onClick={() => (temFilhos ? setAberta(estaAberta ? null : e.chave) : aoEscolher(e.chave))}
              // Quando a coluna está aberta, o corpo sai do caminho: deixá-lo
              // focável faria o Tab passar por um botão invisível.
              tabIndex={estaAberta ? -1 : 0}
            >
              <span className="pn-selo">{e.icone}</span>

              {temContador && (
                <span className="pn-medida">
                  <span className="pn-num">{e.numero === null ? '–' : formatar(e.numero!)}</span>
                  <span className="pn-rot" style={{ display: 'block' }}>{e.rotulo}</span>
                </span>
              )}

              {!simples && (
                <Previa titulo={e.previaTitulo} linhas={e.previa} vazia={e.previaVazia} numero={e.numero} />
              )}

              <h3>{e.titulo}</h3>

              {simples && <span className="pn-espaco" />}

              <span className="pn-rodape">
                <span>{e.selo ? <b className="pn-selo-breve">{e.selo}</b> : (e.rodape ?? '')}</span>
                {!e.desabilitada && <span className="pn-seta" aria-hidden>→</span>}
              </span>
            </button>

            {temFilhos && (
              <div className="pn-quatro">
                <div className="cab">
                  <h3>{e.titulo}</h3>
                </div>
                <div className="pn-grade">
                  {e.filhos!.map(f => (
                    <button
                      key={f.chave}
                      type="button"
                      className={'pn-sub4' + ((f.numero ?? 0) > 0 ? ' viva' : '')}
                      aria-label={f.descricao ? `${f.titulo} — ${f.descricao}` : f.titulo}
                      onClick={() => aoEscolher(f.chave)}
                      tabIndex={estaAberta ? 0 : -1}
                    >
                      <span className="n">{f.numero === null ? '–' : formatar(f.numero)}</span>
                      <span className="lin">
                        {f.previa?.length
                          ? f.previa.slice(0, 4).map((l, i) => (
                              <i key={i}><b>{l.texto}</b>{l.fim && <u>{l.fim}</u>}</i>
                            ))
                          : (f.numero ?? 0) === 0
                            ? <span className="nada">{f.previaVazia ?? 'nada aqui'}</span>
                            : null}
                      </span>
                      <span className="t">{f.titulo}</span>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

/**
 * A frase de "está vazio" só aparece quando o contador É zero.
 *
 * POR QUE ISSO IMPORTA
 *   Uma barra que mostra "3" e, logo abaixo, "nenhuma nota órfã" está se
 *   contradizendo na mesma tela. Quando há trabalho mas a prévia não chegou, o
 *   certo é não dizer nada — o número já disse a verdade.
 */
function Previa({ titulo, linhas, vazia, numero }: {
  titulo?: string
  linhas?: LinhaDaPrevia[]
  vazia?: string
  numero?: number | null
}) {
  // Sem prévia a coluna ainda precisa do espaço flexível, senão o título sobe
  // para o meio da barra e a etapa sem prévia fica desalinhada das outras.
  //
  // A classe é `pn-sem-previa`, e não `vazio`: `.vazio` já existe em telas.css
  // como caixa de estado vazio, com fundo BRANCO — e apareceu como um retângulo
  // branco no meio da barra escura. Classe curta em folha compartilhada é
  // colisão esperando acontecer; o prefixo do módulo resolve.
  if (!titulo) return <span className="pn-previa pn-sem-previa" />
  return (
    <span className="pn-previa">
      <span className="cap" style={{ display: 'block' }}>{titulo}</span>
      {linhas?.length
        ? linhas.slice(0, 9).map((l, i) => (
            <i key={i}><b>{l.texto}</b>{l.fim && <u>{l.fim}</u>}</i>
          ))
        : (numero ?? 0) === 0
          ? <span className="nada" style={{ display: 'block' }}>{vazia ?? 'nada por aqui'}</span>
          : null}
    </span>
  )
}

/** Milhar com ponto, como o resto do sistema escreve número. */
function formatar(n: number): string {
  return n.toLocaleString('pt-BR')
}
