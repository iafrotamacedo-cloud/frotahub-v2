// rev 1 — a confirmação em dois passos, para o que custa dinheiro
//
// POR QUE DOIS PASSOS, E NÃO UM
//
//	O primeiro mostra o NÚMERO: "dar desconto de 3,85%?". Ele existe para a
//	pessoa ver o tamanho do que está autorizando — um diálogo que só perguntasse
//	"confirma?" faria alguém autorizar 20% achando que eram 3%.
//
//	O segundo pede a senha. Não porque a senha protege o servidor — não protege,
//	e isso está escrito no motor —, mas porque ela quebra o automatismo. Quem
//	clica duas vezes por reflexo não digita uma senha por reflexo.
//
// A SENHA NÃO VIAJA PARA O NOSSO MOTOR
//
//	Ela vai direto ao mesmo serviço de autenticação que faz o login, do
//	navegador. O motor nunca a vê, nunca a registra e não teria onde guardá-la.
//	O que ele recebe é o pedido já autenticado pela sessão de sempre.
import { useState } from 'react'
import { supabase } from '../../supabase/cliente'

export function ConfirmarComSenha({ pergunta, aoConfirmar, fechar }: {
  /** A frase com o NÚMERO dentro. É ela que dá a dimensão do ato. */
  pergunta: string
  aoConfirmar: () => Promise<void>
  fechar: () => void
}) {
  const [passo, setPasso] = useState<'pergunta' | 'senha'>('pergunta')
  const [senha, setSenha] = useState('')
  const [erro, setErro] = useState('')
  const [ocupado, setOcupado] = useState(false)

  async function confirmar() {
    if (!senha) return
    setOcupado(true)
    setErro('')
    try {
      // QUEM CONFIRMA É QUEM ESTÁ LOGADO, E A SESSÃO JÁ SABE QUEM É
      //   Passar o usuário por propriedade obrigaria a arrastá-lo por duas
      //   telas que não têm nada com isso — e abriria a porta para alguém
      //   passar OUTRO usuário, transformando a confirmação numa formalidade.
      const { data: sessao } = await supabase.auth.getSession()
      const email = sessao.session?.user.email
      if (!email) {
        setErro('A sua sessão expirou. Entre de novo.')
        return
      }
      const { error } = await supabase.auth.signInWithPassword({ email, password: senha })
      if (error) {
        setErro('Senha incorreta.')
        return
      }
      // A senha some da memória no instante em que deixa de ser necessária.
      setSenha('')
      await aoConfirmar()
      fechar()
    } catch (e) {
      setErro(e instanceof Error ? e.message : 'Não consegui confirmar.')
    } finally {
      setOcupado(false)
    }
  }

  return (
    <div className="orc-janela" role="dialog" aria-modal>
      {/* O MESMO TAMANHO NOS DOIS PASSOS
          Pedido do dono: "abre outro, do mesmo tamanho". Uma caixa que muda de
          tamanho no meio da confirmação faz o botão de confirmar aparecer onde
          estava o de cancelar — e o dedo já está a caminho. */}
      <div className="orc-janela-caixa confirma">
        {passo === 'pergunta' ? (
          <>
            <h3>{pergunta}</h3>
            <p className="orc-dica">
              Confirmando, você vai precisar digitar a sua senha no passo seguinte.
            </p>
            <div className="confirma-botoes">
              <button type="button" className="orc-bt" onClick={fechar}>não</button>
              <button type="button" className="orc-bt forte" onClick={() => setPasso('senha')}>sim</button>
            </div>
          </>
        ) : (
          <>
            <h3>Digite a sua senha para confirmar</h3>
            <p className="orc-dica">{pergunta}</p>
            {erro && <p className="erro">{erro}</p>}
            <input
              type="password" autoComplete="current-password" autoFocus
              placeholder="••••••••" value={senha} disabled={ocupado}
              onChange={e => setSenha(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') void confirmar() }}
              aria-label="senha"
            />
            <div className="confirma-botoes">
              <button type="button" className="orc-bt" disabled={ocupado} onClick={fechar}>cancelar</button>
              <button type="button" className="orc-bt forte" disabled={ocupado || !senha}
                onClick={() => void confirmar()}>
                {ocupado ? 'confirmando…' : 'confirmar'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
