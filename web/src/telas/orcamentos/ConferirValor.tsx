// rev 1 — a pergunta do valor, com o papel do lado
//
// O PEDIDO, VERBATIM (26/08/2026)
//
//	"notas com esse problema precisam de um comparativo, ou seja: a IA leu 200,00
//	 reais, tem mesmo 200,00 aí <s/n>... tem que ser assim"
//
//	E ele está certo. Antes disto a tela dizia "confira" — e, quando passou a
//	dizer o quê, dizia "a chave de acesso não foi lida": específico e inútil.
//	Ninguém digita uma chave de 44 dígitos, ela não entra no orçamento e não
//	impede nada.
//
//	O que a pessoa consegue conferir olhando o papel é o VALOR. Então o sistema
//	pergunta o valor, e ela responde sim ou não.
//
// POR QUE OS DOIS NÚMEROS APARECEM
//
//	A soma dos itens ao lado do total lido é o que EXPLICA a pergunta. Sem ela, a
//	pessoa não sabe se o problema é o total ou é um item — e sem saber, confirma
//	no automático, que é o mesmo que não perguntar.
import { useState } from 'react'
import { motor } from '../../motor/cliente'
import { VisorDaNota, type Acoes } from './VisorDaNota'
import { emReais, oQueConferir, type Documento } from './tipos'

export function ConferirValor({
  documento, acoes, aoResponder,
}: {
  documento: Documento
  acoes: Acoes
  /** Chamado depois que a resposta foi gravada, para a lista se refazer. */
  aoResponder: () => Promise<void>
}) {
  const [corrigindo, setCorrigindo] = useState(false)
  const [digitado, setDigitado] = useState('')
  const [ocupado, setOcupado] = useState(false)
  const [erro, setErro] = useState('')

  async function responder(confere: boolean, valor?: number) {
    setOcupado(true)
    try {
      await motor(`/orcamentos/documentos/${documento.id}/valor`,
        { metodo: 'POST', corpo: { confere, valor: valor ?? 0 } })
      setErro('')
      await aoResponder()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui gravar a resposta.')
    } finally {
      setOcupado(false)
    }
  }

  function enviarCorrecao() {
    // Vírgula é como se digita dinheiro aqui. Recusar por causa dela seria
    // ensinar o usuário a falar como o programa.
    const n = Number(digitado.replace(/\./g, '').replace(',', '.'))
    if (!Number.isFinite(n) || n <= 0) {
      setErro('Escreva o valor da nota, como está no papel.')
      return
    }
    void responder(false, n)
  }

  const pergunta = (
    <div className="conf-valor">
      <p className="conf-pergunta">
        A leitura entendeu que esta nota vale <b>{emReais(documento.valor_total)}</b>.
        {' '}É isso mesmo?
      </p>

      {/* O PORQUÊ DA PERGUNTA, EM NÚMEROS
          Sem isto a pessoa não sabe se o problema é o total ou é um item — e
          sem saber, confirma no automático. */}
      <p className="conf-porque">
        {oQueConferir(documento)}
        {documento.itens > 0 && documento.soma_dos_itens !== undefined && (
          <> — os {documento.itens} {documento.itens === 1 ? 'item soma' : 'itens somam'}{' '}
            <b>{emReais(documento.soma_dos_itens)}</b></>
        )}
      </p>

      {erro && <p className="erro">{erro}</p>}

      {!corrigindo ? (
        <div className="conf-botoes">
          <button type="button" className="orc-bt forte" disabled={ocupado}
            onClick={() => void responder(true)}>
            Sim, vale {emReais(documento.valor_total)}
          </button>
          <button type="button" className="orc-bt" disabled={ocupado}
            onClick={() => { setDigitado(''); setCorrigindo(true) }}>
            Não, o valor é outro
          </button>
        </div>
      ) : (
        <div className="conf-botoes">
          <input
            autoFocus inputMode="decimal" placeholder="valor da nota"
            value={digitado} disabled={ocupado}
            onChange={e => setDigitado(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') enviarCorrecao() }}
            aria-label="valor correto da nota"
          />
          <button type="button" className="orc-bt forte" disabled={ocupado || !digitado}
            onClick={enviarCorrecao}>gravar este valor</button>
          <button type="button" className="orc-bt" disabled={ocupado}
            onClick={() => { setCorrigindo(false); setErro('') }}>voltar</button>
        </div>
      )}
    </div>
  )

  return (
    <VisorDaNota
      documento={documento.id}
      nome={documento.nome_arquivo}
      valor={documento.valor_total}
      tickets={documento.ticket_numeros ?? []}
      modo="conferencia"
      cabeca={pergunta}
      acoes={acoes}
    />
  )
}
