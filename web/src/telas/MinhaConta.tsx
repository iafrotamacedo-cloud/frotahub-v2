// rev 2 — a própria conta
//
// A única tela de Configurações que NÃO é do builder. Todo login chega aqui, e é
// daqui que cada um cuida da própria senha sem depender de ninguém (P-29).
//
// O QUE MUDOU NA REVISÃO 2
//   Era uma janela sobreposta, aberta só clicando no próprio nome na barra lateral.
//   Virou TELA, com lugar no menu. O motivo é de operação: quem procura a própria
//   conta procura em Configurações — e quem não é builder não via Configurações
//   nenhuma, porque tudo lá dentro era do dono do sistema.
//
//   As duas portas continuam existindo: o item no menu e o clique no próprio nome.
//   Uma implementação só, dois caminhos até ela (CORE-06).
//
// POR QUE PEDIR A SENHA ATUAL
//   O cenário real é o computador da obra deixado aberto. Sem essa confirmação,
//   quem passar pela cadeira troca a senha de quem esqueceu de sair, e o dono do
//   login perde a própria conta.
//
// Quem confere a senha atual é o Supabase, não o motor — o motor não guarda senha
// nenhuma (CORE-09).
import { useState, type FormEvent } from 'react'
import { motor, ErroMotor, avisoDe } from '../motor/cliente'
import type { Perfil } from '../sessao/tipos'

const SENHA_MINIMA = 8

export function MinhaConta({ perfil }: { perfil: Perfil }) {
  const [atual, setAtual] = useState('')
  const [nova, setNova] = useState('')
  const [repetida, setRepetida] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [pronto, setPronto] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  // As conferências que dá para fazer ANTES de incomodar o servidor ficam aqui, e
  // aparecem enquanto a pessoa digita — em vez de só depois de clicar em salvar.
  const curta = nova.length > 0 && nova.length < SENHA_MINIMA
  const diferentes = repetida.length > 0 && nova !== repetida
  const repetiuAAtual = nova.length > 0 && nova === atual
  const podeSalvar = !!atual && nova.length >= SENHA_MINIMA && nova === repetida && !repetiuAAtual

  async function enviar(e: FormEvent) {
    e.preventDefault()
    setErro(null)
    setPronto(null)
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
    <>
      <header className="hero">
        <h1>Minha conta</h1>
        <p>Os seus dados no FrotaHub e a troca da sua senha.</p>
      </header>

      <div className="cartoes">
        <section className="cartao">
          <h2>Seus dados</h2>
          <dl className="ficha">
            <div><dt>Nome</dt><dd>{perfil.nome}</dd></div>
            <div><dt>Usuário</dt><dd><code>{perfil.usuario}</code></dd></div>
            <div><dt>Categoria</dt><dd>{perfil.categoriaNome || '—'}</dd></div>
            <div><dt>Nível</dt><dd style={{ textTransform: 'capitalize' }}>{perfil.nivel}</dd></div>
          </dl>
          {/* Dizer QUEM muda cada coisa poupa a pergunta e a espera por uma
              resposta que nunca vem (P-29). */}
          <p className="dica">
            Nome, usuário e categoria são definidos por quem administra o sistema.
            A senha é sua.
          </p>
        </section>

        <section className="cartao">
          <h2>Trocar a senha</h2>
          <form onSubmit={enviar}>
            <label htmlFor="mc-atual">Senha atual</label>
            <input
              id="mc-atual" type="password" autoComplete="current-password"
              value={atual} onChange={e => setAtual(e.target.value)}
            />

            <label htmlFor="mc-nova">Senha nova</label>
            <input
              id="mc-nova" type="password" autoComplete="new-password"
              value={nova} onChange={e => setNova(e.target.value)}
            />
            <p className={'dica' + (curta || repetiuAAtual ? ' dica-alerta' : '')}>
              {repetiuAAtual
                ? 'A senha nova tem que ser diferente da atual.'
                : `Mínimo de ${SENHA_MINIMA} caracteres.`}
            </p>

            <label htmlFor="mc-rep">Repita a senha nova</label>
            <input
              id="mc-rep" type="password" autoComplete="new-password"
              value={repetida} onChange={e => setRepetida(e.target.value)}
            />
            {diferentes && <p className="dica dica-alerta">As duas não são iguais.</p>}

            {erro && <div className="erro-caixa">{erro}</div>}
            {pronto && <div className="recado" role="status">{pronto}</div>}

            <div className="jn-pe">
              <button type="submit" className="bt bt-forte" disabled={salvando || !podeSalvar}>
                {salvando ? 'Salvando...' : 'Trocar senha'}
              </button>
            </div>
          </form>
        </section>
      </div>
    </>
  )
}
