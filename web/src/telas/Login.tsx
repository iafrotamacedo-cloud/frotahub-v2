// rev 2 — a tela de entrada
//
// Não existe autocadastro nem "esqueci minha senha": quem cria login é o administrador.
import { useState } from 'react'
import { Marca } from '../componentes/Marca'

interface Props {
  entrar: (usuario: string, senha: string) => Promise<string | null>
  /** Verdadeiro quando a pessoa caiu aqui por tempo, não por ter clicado em Sair. */
  expirou?: boolean
}

export function Login({ entrar, expirou }: Props) {
  const [usuario, setUsuario] = useState('')
  const [senha, setSenha] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [enviando, setEnviando] = useState(false)

  async function submeter(e: React.FormEvent) {
    e.preventDefault()
    if (enviando) return
    setEnviando(true)
    setErro(null)
    const problema = await entrar(usuario, senha)
    if (problema) {
      setErro(problema)
      setEnviando(false)
    }
    // Se deu certo, a sessão troca a tela — não há o que fazer aqui.
  }

  return (
    <div className="auth">
      <aside className="auth-marca">
        <Marca />
        <h2>Gestão integrada da <em>Frota Macedo Engenharia</em></h2>
        <div className="ft">Administrativo · Manutenção · Engenharia · SESMT — num só lugar.</div>
      </aside>

      <main className="auth-form">
        <form className="auth-card" onSubmit={submeter}>
          <h3>Entrar</h3>
          <p className="d">Use o usuário e a senha fornecidos pelo administrador.</p>

          {expirou && (
            <div className="aviso-caixa" role="status">
              A sua sessão foi encerrada por tempo. Entre de novo para continuar.
            </div>
          )}

          <label htmlFor="usuario">Usuário</label>
          <input
            id="usuario" type="text" autoComplete="username" placeholder="seu usuário"
            value={usuario} onChange={e => setUsuario(e.target.value)}
            autoFocus required
          />

          <label htmlFor="senha">Senha</label>
          <input
            id="senha" type="password" autoComplete="current-password" placeholder="••••••••"
            value={senha} onChange={e => setSenha(e.target.value)}
            required
          />

          <button className="btn" type="submit" disabled={enviando}>
            {enviando ? 'Entrando…' : 'Entrar'}
          </button>

          {erro && <div className="err" role="alert">{erro}</div>}
        </form>
      </main>
    </div>
  )
}
