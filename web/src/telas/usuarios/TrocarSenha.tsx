// rev 1 — trocar a senha de outra pessoa
//
// Ação separada do formulário de dados, e com confirmação por escrito, porque o
// efeito é imediato e não se desfaz: a pessoa perde o acesso até receber a senha
// nova. Escrever duas vezes evita o erro de digitação que ninguém percebe na hora.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import type { LinhaUsuario } from './tipos'

interface Props {
  usuario: LinhaUsuario
  aoFechar: () => void
  aoSalvar: (aviso: string | null) => void
}

export function TrocarSenha({ usuario, aoFechar, aoSalvar }: Props) {
  const [senha, setSenha] = useState('')
  const [repetida, setRepetida] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    if (senha !== repetida) {
      setErro('As duas senhas não são iguais.')
      return
    }
    setErro(null)
    setSalvando(true)
    try {
      const r = await motor(`/usuarios/${usuario.id}/senha`, { metodo: 'POST', corpo: { senha } })
      aoSalvar(avisoDe(r))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui trocar a senha.')
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo="Trocar senha"
      descricao={`${usuario.nome} · ${usuario.usuario}`}
      aoFechar={aoFechar}
      largura={420}
    >
      <form className="jn-corpo" onSubmit={enviar}>
        <div className="aviso-caixa">
          A senha antiga para de funcionar na hora. Combine a nova com a pessoa antes de salvar.
        </div>

        <label htmlFor="s-nova">Senha nova</label>
        <input id="s-nova" type="password" autoComplete="new-password" value={senha} onChange={e => setSenha(e.target.value)} />

        <label htmlFor="s-rep">Repita a senha</label>
        <input id="s-rep" type="password" autoComplete="new-password" value={repetida} onChange={e => setRepetida(e.target.value)} />

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando || senha.length < 8}>
            {salvando ? 'Salvando...' : 'Trocar senha'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
