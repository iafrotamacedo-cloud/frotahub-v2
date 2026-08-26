// rev 1 — a sugestão por semelhança, extraída de Correcoes.tsx
//
// A DICA QUE DEU ORIGEM A ESTA TELA
//
//	A primeira versão de "sem associação" abria pedindo o número certo do
//	ticket. O dono corrigiu: 90% das vezes o ticket está CERTO na nossa base e
//	ERRADO na nota — o fornecedor escreveu 13O328 com a letra O, ou inverteu
//	dois dígitos. O robô roda a cada duas horas e não atendemos ticket anterior
//	à data de corte, então "não está na base" é o caso raro, não o comum.
//
//	Por isso a tela sugere candidatos ANTES de pedir qualquer coisa: o usuário
//	confere e confirma. Digitar fica para quando nenhuma sugestão serve.
//
// POR QUE ELE SAIU DE `Correcoes.tsx`
//
//	Quando o tratamento passou a abrir a nota na tela (26/08/2026), quem precisa
//	sugerir candidatos é o visor, e o visor é importado por `Correcoes` — deixar
//	o componente lá dentro faria os dois arquivos se importarem em círculo.
//
//	E ele quase morreu na mudança: virou código sem uso quando reescrevi a tela,
//	e o compilador o apontou como "declarado e nunca lido". Apagar teria sido o
//	caminho fácil e errado — a sugestão por semelhança é melhor do que pedir o
//	número, e foi ela que o dono pediu no lugar da digitação.
import { useEffect, useState } from 'react'
import { motor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { emDataHora, contaPorExtenso, type Candidato } from './tipos'

/** A janela de sugestões — o coração da correção por semelhança. */
export function Candidatos({ documento, ticket, fechar, concluir }: {
  documento: string
  ticket: number
  fechar: () => void
  concluir: () => Promise<void>
}) {
  const [lista, setLista] = useState<Candidato[] | null>(null)
  const [dica, setDica] = useState('')
  const [erro, setErro] = useState('')
  const [manual, setManual] = useState('')

  useEffect(() => {
    void (async () => {
      try {
        const r = await motor<{ candidatos: Candidato[]; dica: string }>(
          '/orcamentos/correcoes/candidatos?ticket=' + ticket)
        setLista(r.candidatos)
        setDica(r.dica)
      } catch (e) {
        setErro(e instanceof Error ? e.message : 'Não consegui buscar sugestões.')
      }
    })()
  }, [ticket])

  async function corrigir(para: number) {
    try {
      await motor(`/orcamentos/documentos/${documento}/tickets`, {
        metodo: 'PATCH', corpo: { de: ticket, para },
      })
      await concluir()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui corrigir.')
    }
  }

  return (
    <div className="orc-janela" role="dialog" aria-modal>
      <div className="orc-janela-caixa">
        <h3>O fornecedor escreveu <b>{ticket}</b></h3>
        <p className="orc-dica">{dica}</p>

        {erro && <p className="erro">{erro}</p>}
        {!lista ? <Carregando /> : (
          <div className="orc-candidatos">
            {lista.map(c => (
              <button key={c.numero} type="button" className="orc-candidato" onClick={() => void corrigir(c.numero)}>
                <b>{c.numero}</b>
                <span className="loja">{c.loja} · {contaPorExtenso(c.conta)}</span>
                <span className="desc">{c.descricao}</span>
                <span className="porque">{c.porque} · aberto em {emDataHora(c.criado_em)}</span>
              </button>
            ))}
            {lista.length === 0 && <p className="orc-vazio">Nenhum chamado parecido.</p>}
          </div>
        )}

        <div className="orc-manual">
          <span>Nenhum destes?</span>
          <input
            inputMode="numeric"
            placeholder="digitar o número certo"
            value={manual}
            maxLength={6}
            onChange={e => setManual(e.target.value.replace(/\D/g, ''))}
          />
          <button type="button" className="orc-bt"
            disabled={!(Number(manual) >= 10000)}
            onClick={() => void corrigir(Number(manual))}>corrigir</button>
          <button type="button" className="orc-bt" onClick={fechar}>fechar</button>
        </div>
      </div>
    </div>
  )
}

