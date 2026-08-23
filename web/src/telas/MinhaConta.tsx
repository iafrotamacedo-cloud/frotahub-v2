// rev 1 — a própria conta
//
// A única tela deste bloco que não é do builder: aqui a pessoa troca a PRÓPRIA
// senha, e para isso precisa digitar a atual.
//
// POR QUE PEDIR A SENHA ATUAL
//   O cenário real é o computador da obra deixado aberto. Sem essa confirmação,
//   quem passar pela cadeira troca a senha de quem esqueceu de sair, e o dono do
//   login perde a própria conta.
//
// Quem confere a senha atual é o Supabase, não o motor — o motor não guarda senha
// nenhuma (CORE-09).
import { useState, type FormEvent } from 'react'
import { Janela } from '../componentes/Janela'
import { motor, ErroMotor, avisoDe } from '../motor/cliente'
import type { Perfil } from '../sessao/tipos'

interface Props {
  perfil: Perfil
  aoFechar: () => void
}

export function MinhaConta({ perfil, aoFechar }: Props) {
  const [atual, setAtual] = useState('')
  const [nova, setNova] = useState('')
  const [repetida, setRepetida] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [pronto, setPronto] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    if (nova !== repetida) {
      setErro('As duas senhas novas não são iguais.')
      return
    }
    setErro(null)
    setSalvando(true)
    try {
      const r = await motor('/minha-conta/senha', {
        metodo: 'POST',
        corpo: { senha_atual: atual, senha_nova: nova },
      })
      setPronto(avisoDe(r) ?? 'Senha trocada. Use a nova da próxima vez que entrar.')
      setAtual(''); setNova(''); setRepetida('')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui trocar a senha.')
    } finally {
      setSalvando(false)
    }
  }

  return (
    <Janela titulo="Minha conta" descricao={`${perfil.nome} · ${perfil.usuario}`} aoFechar={aoFechar} largura={420}>
      <form className="jn-corpo" onSubmit={enviar}>
        <dl className="ficha">
          <div><dt>Usuário</dt><dd><code>{perfil.usuario}</code></dd></div>
          <div><dt>Categoria</dt><dd>{perfil.categoriaNome || '—'}</dd></div>
          <div><dt>Nível</dt><dd style={{ textTransform: 'capitalize' }}>{perfil.nivel}</dd></div>
        </dl>

        <label htmlFor="mc-atual">Senha atual</label>
        <input id="mc-atual" type="password" autoComplete="current-password" value={atual} onChange={e => setAtual(e.target.value)} />

        <label htmlFor="mc-nova">Senha nova</label>
        <input id="mc-nova" type="password" autoComplete="new-password" value={nova} onChange={e => setNova(e.target.value)} />

        <label htmlFor="mc-rep">Repita a senha nova</label>
        <input id="mc-rep" type="password" autoComplete="new-password" value={repetida} onChange={e => setRepetida(e.target.value)} />

        {erro && <div className="erro-caixa">{erro}</div>}
        {pronto && <div className="recado" role="status">{pronto}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Fechar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando || !atual || nova.length < 8}>
            {salvando ? 'Salvando...' : 'Trocar senha'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
