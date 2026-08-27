// rev 1 — Fechamento: as duas contas, e a prova de que elas fecham
//
// POR QUE ESTA TELA EXISTE
//
//	"tem 93 não lançados, 23 de pendência, 68 parados. nada bate com nada."
//
//	Os três números estavam certos. Eram respostas para três perguntas
//	diferentes, e nenhuma tela mostrava as três juntas — então a única coisa que
//	dava para concluir, olhando de fora, era que o sistema não fechava. Um
//	sistema que não dá para conferir é um sistema em que não se confia, mesmo
//	quando ele está certo.
//
// SÃO DUAS CONTAS, LIGADAS POR UMA PONTE
//
//	A conta escrita pelo dono misturava notas e orçamentos, e por isso nunca
//	fecharia: uma nota vira VÁRIOS orçamentos (as partes), e um orçamento de
//	rateio nasce de VÁRIAS notas. O que fecha é isto:
//
//	  1. toda NOTA que entrou está em exatamente um destino
//	  2. todo ORÇAMENTO que existe está em exatamente um estado
//	  3. e nada se perdeu entre a nota e o orçamento — a ponte
//
// A DIFERENÇA É O NÚMERO MAIS IMPORTANTE DA TELA
//
//	Cada conta traz o total das linhas E o universo contado direto na tabela,
//	por fora. Se os dois baterem, a diferença é zero e a tela diz isso em verde.
//	Se um dia não baterem, a diferença aparece em vermelho, com o tamanho exato
//	do buraco — em vez de o sistema fingir que fecha.
//
//	É por isso que o universo não é a soma das linhas: somando, fecharia sempre.
import { useCallback, useEffect, useState } from 'react'
import { motor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { BarraDeVolta } from './Arquivos'
import { emReais, type Fechamento as Dados, type LinhaDoFechamento } from './tipos'

export function Fechamento({ voltar }: { voltar: () => void }) {
  const [dados, setDados] = useState<Dados | null>(null)
  const [erro, setErro] = useState('')

  const carregar = useCallback(async () => {
    try {
      setDados(await motor<Dados>('/orcamentos/fechamento'))
      setErro('')
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui carregar o fechamento.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  return (
    <div className="orc-tela">
      <BarraDeVolta voltar={voltar} titulo="Fechamento" />
      {erro && <p className="erro">{erro}</p>}
      {!dados ? <Carregando /> : (
        <div className="orc-lista">
          <div className="orc-lista-cab">
            <h2>As duas contas</h2>
            <em>
              Toda nota que entrou está em um destino só. Todo orçamento está em um
              estado só. Se as somas baterem com o que existe na base, o sistema fecha.
            </em>
          </div>

          <Conta
            titulo="As notas que entraram"
            unidade="notas"
            conta={dados.notas}
            explica="Cada linha é um caminho que uma nota pode ter tomado. Uma nota está em uma linha e só."
          />

          <Conta
            titulo="Os orçamentos que existem"
            unidade="orçamentos"
            conta={dados.orcamentos}
            explica="Uma nota pode virar mais de um orçamento — são as partes do mesmo ticket. Por isso esta conta é separada da de cima."
          />

          <Ponte dados={dados} />
        </div>
      )}
    </div>
  )
}

function Conta({ titulo, unidade, conta, explica }: {
  titulo: string
  unidade: string
  conta: Dados['notas']
  explica: string
}) {
  const fecha = conta.diferenca === 0
  // Sem linha nenhuma, "0 = 0" é verdade e não é informação. Dizer que está
  // vazio é mais honesto do que exibir uma tabela em branco com um selo verde.
  const vazia = conta.universo === 0

  return (
    <section className="orc-conta">
      <div className="orc-conta-cab">
        <h3>{titulo}</h3>
        <em>{explica}</em>
      </div>

      {vazia ? (
        <p className="orc-vazio">Nenhuma {unidade.replace(/s$/, '')} ainda.</p>
      ) : (
        <div className="orc-rolagem">
          <table className="orc-tabela orc-conta-tabela">
            <thead>
              <tr>
                <th>Onde está</th>
                <th style={{ textAlign: 'right' }}>Quantas</th>
                <th style={{ textAlign: 'right' }}>Valor</th>
              </tr>
            </thead>
            <tbody>
              {conta.linhas.map(l => <Linha key={l.chave} l={l} />)}
            </tbody>
            <tfoot>
              <tr className="orc-conta-total">
                <td>somando as linhas</td>
                <td style={{ textAlign: 'right' }}>{conta.total}</td>
                <td style={{ textAlign: 'right' }}>{emReais(conta.valor)}</td>
              </tr>
              {/* O UNIVERSO VEM DA TABELA, POR FORA DA SOMA
                  Se ele viesse da soma das linhas, fecharia sempre — inclusive
                  quando alguma nota escapasse da classificação. */}
              <tr className="orc-conta-total">
                <td>{unidade} na base</td>
                <td style={{ textAlign: 'right' }}>{conta.universo}</td>
                <td />
              </tr>
              <tr className={fecha ? 'orc-conta-fecha' : 'orc-conta-furo'}>
                <td>{fecha ? 'a conta fecha' : 'FALTAM NA CONTA'}</td>
                <td style={{ textAlign: 'right' }}>{fecha ? '✓' : conta.diferenca}</td>
                <td />
              </tr>
            </tfoot>
          </table>
        </div>
      )}
    </section>
  )
}

function Linha({ l }: { l: LinhaDoFechamento }) {
  // O BALDE DO DESCONHECIDO GRITA
  //   `ordem = 99` é o `else` das views: um estado que ninguém previu. Ele
  //   existe justamente para APARECER — sumir dentro de outra linha é como o
  //   erro de prioridade sobreviveu anos no sistema antigo.
  const desconhecido = l.ordem === 99
  return (
    <tr className={desconhecido ? 'orc-conta-furo' : undefined}>
      <td>
        {l.rotulo}
        <div className="orc-sub">{l.chave}</div>
      </td>
      <td style={{ textAlign: 'right' }}>{l.quantos}</td>
      <td style={{ textAlign: 'right' }}>{l.valor ? emReais(l.valor) : '–'}</td>
    </tr>
  )
}

/** A ponte entre as duas contas.
 *
 *  Os três "órfãos" têm que ser sempre zero. Se um dia não forem, alguma
 *  gravação falhou pelo meio — a nota foi marcada como usada e o vínculo não
 *  nasceu, ou o contrário. É o tipo de coisa que, sem esta linha, só apareceria
 *  meses depois, como um orçamento sem nota atrás. */
function Ponte({ dados }: { dados: Dados }) {
  const p = dados.ponte
  const ok = p.usadas_sem_vinculo === 0
    && p.vinculo_sem_nota_usada === 0
    && p.vinculo_de_orcamento_apagado === 0
  return (
    <section className="orc-conta">
      <div className="orc-conta-cab">
        <h3>A ponte entre as duas</h3>
        <em>Nota que virou orçamento tem que ter vínculo vivo, e todo vínculo vivo tem que ter nota. Os três de baixo são sempre zero.</em>
      </div>
      <div className="orc-rolagem">
        <table className="orc-tabela orc-conta-tabela">
          <tbody>
            <tr><td>notas que viraram orçamento</td><td style={{ textAlign: 'right' }}>{p.notas_usadas}</td></tr>
            <tr><td>vínculos vivos</td><td style={{ textAlign: 'right' }}>{p.vinculos_vivos}</td></tr>
            <tr className={p.usadas_sem_vinculo ? 'orc-conta-furo' : undefined}>
              <td>usadas sem vínculo</td><td style={{ textAlign: 'right' }}>{p.usadas_sem_vinculo}</td></tr>
            <tr className={p.vinculo_sem_nota_usada ? 'orc-conta-furo' : undefined}>
              <td>vínculo sem nota usada</td><td style={{ textAlign: 'right' }}>{p.vinculo_sem_nota_usada}</td></tr>
            <tr className={p.vinculo_de_orcamento_apagado ? 'orc-conta-furo' : undefined}>
              <td>vínculo preso a orçamento apagado</td><td style={{ textAlign: 'right' }}>{p.vinculo_de_orcamento_apagado}</td></tr>
          </tbody>
          <tfoot>
            <tr className={ok ? 'orc-conta-fecha' : 'orc-conta-furo'}>
              <td>{ok ? 'nada se perdeu no caminho' : 'ALGUMA COISA SE PERDEU — avise o suporte'}</td>
              <td style={{ textAlign: 'right' }}>{ok ? '✓' : '!'}</td>
            </tr>
          </tfoot>
        </table>
      </div>
    </section>
  )
}
