// rev 4 — a casca do FrotaHub
//
// Junta as três peças e nada mais: a barra lateral com o menu, o cabeçalho com o
// caminho, e a área de trabalho. Cada rotina é um arquivo próprio em telas/ — este
// arquivo não cresce junto (CORE-16): ele só sabe QUAL abrir, nunca o que ela faz.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSessao } from './sessao/useSessao'
import { ehBuilder } from './sessao/tipos'
import { arvoreVisivel, type ItemMenu } from './menu/arvore'
import { useNavegacao } from './menu/navegacao'
import { Icone, Seta, Menu } from './componentes/Icone'
import { Marca } from './componentes/Marca'
import { Login } from './telas/Login'
import { Inicio } from './telas/Inicio'
import { EmBreve } from './telas/EmBreve'
import { Usuarios } from './telas/usuarios/Usuarios'
import { Categorias } from './telas/categorias/Categorias'
import { MinhaConta } from './telas/MinhaConta'
import { useExpiracao } from './sessao/inatividade'

export default function App() {
  const { carregando, perfil, entrar, sair } = useSessao()

  // O menu é montado a partir do que ESTE login alcança, não da árvore inteira.
  // Fica memorizado porque a navegação depende dele: uma árvore nova a cada
  // desenho faria o caminho ser recalculado sem necessidade.
  const arvore = useMemo(() => arvoreVisivel(ehBuilder(perfil)), [perfil])

  // O caminho aberto mora no ENDEREÇO, não na memória da página. É o que faz o
  // botão "voltar" do navegador voltar uma tela em vez de sair do sistema.
  // [] é o início; [Manutenção] é o bloco; [Manutenção, Serviços] é a rotina.
  const { caminho, navegar: irPara } = useNavegacao(arvore)

  const [abertos, setAbertos] = useState<string[]>([])

  // Chegar por endereço tem que abrir o grupo certo na barra lateral. Sem isto,
  // quem colasse `#/configuracoes/usuarios` veria a tela certa com o menu fechado,
  // e o item marcado como ativo escondido dentro de uma pasta.
  // Continua sendo possível fechar o grupo depois, à mão.
  const grupoAberto = caminho[0]?.t
  useEffect(() => {
    if (grupoAberto) setAbertos(a => (a.includes(grupoAberto) ? a : [...a, grupoAberto]))
  }, [grupoAberto])
  const [navAberta, setNavAberta] = useState(false)
  const [minhaConta, setMinhaConta] = useState(false)
  const [expirou, setExpirou] = useState(false)

  // A sessão acaba por tempo: 3 h parada, 24 h no total. O relógio só corre com
  // alguém dentro — na tela de login não há sessão para expirar.
  const expirar = useCallback(() => { setExpirou(true); void sair() }, [sair])
  useExpiracao(!!perfil, expirar)

  if (carregando) return <div className="auth" />
  if (!perfil) return <Login entrar={entrar} expirou={expirou} />

  const atual = caminho[caminho.length - 1]
  const iniciais = perfil.nome.trim().slice(0, 2).toUpperCase()

  function navegar(novo: ItemMenu[]) {
    irPara(novo)
    setNavAberta(false)
    document.body.classList.remove('nav-aberta')
  }

  function alternar(titulo: string) {
    setAbertos(a => (a.includes(titulo) ? a.filter(t => t !== titulo) : [...a, titulo]))
  }

  function alternarNav() {
    const aberta = !navAberta
    setNavAberta(aberta)
    document.body.classList.toggle('nav-aberta', aberta)
  }

  return (
    <div className="lay">
      <div className="sd-back" onClick={alternarNav} />

      <aside className="side">
        <div className="sd-topo">
          <button
            onClick={() => navegar([])}
            style={{ background: 'none', border: 0, padding: 0, cursor: 'pointer', textAlign: 'left' }}
            aria-label="Ir para o início"
          >
            <Marca assinatura />
          </button>
        </div>

        <nav className="sd-nav">
          <div className="nv-sec">Menu</div>

          {arvore.map(item => {
            const temFilhos = !!item.sub?.length
            const aberto = abertos.includes(item.t)
            const ativo = caminho[0]?.t === item.t && caminho.length === 1

            return (
              <div key={item.t}>
                <button
                  className={'nv-it' + (aberto ? ' aberto' : '') + (ativo ? ' ativo' : '') + (item.breve && !temFilhos ? ' breve' : '')}
                  onClick={() => (temFilhos ? alternar(item.t) : item.breve ? navegar([item]) : navegar([item]))}
                  type="button"
                >
                  <Icone nome={item.icone} />
                  <span className="lb">{item.t}</span>
                  {temFilhos ? <Seta /> : item.breve ? <span className="tag">breve</span> : null}
                </button>

                {temFilhos && aberto && (
                  <div className="nv-sub">
                    {item.sub!.map(filho => (
                      <button
                        key={filho.t}
                        className={'nv-it' + (filho.breve ? ' breve' : '') + (atual?.t === filho.t ? ' ativo' : '')}
                        onClick={() => navegar([item, filho])}
                        type="button"
                      >
                        <Icone nome={filho.icone} />
                        <span className="lb">{filho.t}</span>
                        {filho.breve && <span className="tag">breve</span>}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )
          })}
        </nav>

        <div className="sd-user">
          {/* O bloco inteiro abre Minha conta. É onde a pessoa procura a própria
              conta — e evita mais um item de menu para uma tela que se usa duas
              vezes por ano. */}
          <button className="sd-eu" type="button" onClick={() => setMinhaConta(true)} title="Minha conta">
            <span className="av">{iniciais}</span>
            <span className="i">
              <b>{perfil.nome}</b>
              <span>{perfil.nivel}</span>
            </span>
          </button>
          <button onClick={sair} type="button">Sair</button>
        </div>
      </aside>

      <div className="main">
        <header className="top">
          <button className="burger" onClick={alternarNav} type="button" aria-label="Abrir menu">
            <Menu />
          </button>
          <div className="tp-info">
            <div className="crumb">
              {caminho.length === 0 ? 'FrotaHub' : ['FrotaHub', ...caminho.map(c => c.t)].join(' › ')}
            </div>
            <div className="titulo">{atual ? atual.t : 'Início'}</div>
          </div>
        </header>

        <main className="content">
          {caminho.length === 0 ? (
            <Inicio nome={perfil.nome} arvore={arvore} abrir={navegar} />
          ) : atual?.sub?.length ? (
            <>
              <header className="hero">
                <h1>{atual.t}</h1>
                <p>{atual.desc}</p>
              </header>
              <div className="mods">
                {atual.sub.map(filho => (
                  <button
                    key={filho.t}
                    className={'mod' + (filho.breve ? ' breve' : '')}
                    onClick={() => navegar([...caminho, filho])}
                    type="button"
                  >
                    <div className="ic"><Icone nome={filho.icone} /></div>
                    <h3>{filho.t}</h3>
                    <p>{filho.desc}</p>
                    {filho.breve && <p style={{ marginTop: 10 }}><span className="selo">Em breve</span></p>}
                  </button>
                ))}
              </div>
            </>
          ) : atual?.tela === 'usuarios' ? (
            <Usuarios perfil={perfil} />
          ) : atual?.tela === 'categorias' ? (
            <Categorias />
          ) : (
            <EmBreve titulo={atual!.t} />
          )}
        </main>
      </div>

      {minhaConta && <MinhaConta perfil={perfil} aoFechar={() => setMinhaConta(false)} />}
    </div>
  )
}
