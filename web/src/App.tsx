// rev 8 — a casca do FrotaHub
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
import { Orcamentos } from './telas/orcamentos/Orcamentos'
import { Painel } from './componentes/Painel'
import { etapasDoMenu } from './menu/etapas'
import { DadosTrilogo } from './telas/trilogo/DadosTrilogo'
import { useExpiracao } from './sessao/inatividade'

export default function App() {
  const { carregando, perfil, entrar, sair } = useSessao()

  // O menu é montado a partir do que ESTE login alcança, não da árvore inteira.
  // Fica memorizado porque a navegação depende dele: uma árvore nova a cada
  // desenho faria o caminho ser recalculado sem necessidade.
  const arvore = useMemo(
    () => arvoreVisivel(ehBuilder(perfil), perfil?.rotinas ?? []),
    [perfil],
  )

  // O caminho aberto mora no ENDEREÇO, não na memória da página. É o que faz o
  // botão "voltar" do navegador voltar uma tela em vez de sair do sistema.
  // [] é o início; [Manutenção] é o bloco; [Manutenção, Serviços] é a rotina.
  const { caminho, extra, navegar: irPara } = useNavegacao(arvore)

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

  // A barra lateral recolhida é uma escolha da pessoa, e ela não deve ter que
  // refazer essa escolha a cada visita. Fica no próprio navegador — é preferência
  // de quem está ali, não dado de sistema.
  const [recolhida, setRecolhida] = useState(() => {
    try { return localStorage.getItem('fh-menu-recolhido') === '1' } catch { return false }
  })
  useEffect(() => {
    try { localStorage.setItem('fh-menu-recolhido', recolhida ? '1' : '0') } catch { /* navegador sem armazenamento: só não lembra */ }
  }, [recolhida])
  const [expirou, setExpirou] = useState(false)

  // A sessão acaba por tempo: 3 h parada, 24 h no total. O relógio só corre com
  // alguém dentro — na tela de login não há sessão para expirar.
  const expirar = useCallback(() => { setExpirou(true); void sair() }, [sair])
  useExpiracao(!!perfil, expirar)

  if (carregando) return <div className="auth" />
  if (!perfil) return <Login entrar={entrar} expirou={expirou} />

  const atual = caminho[caminho.length - 1]

  // QUAIS TELAS SÃO ESCURAS
  //
  //	A regra do dono: moldura escura, listagem clara. Entram aqui a tela
  //	inicial, qualquer tela que só oferece caminhos, e as duas de trabalho —
  //	Orçamentos e Dados do Trílogo. Nelas o ÚNICO retângulo claro é a tabela, e
  //	é esse contraste que leva o olho direto ao dado.
  //
  //	Dados do Trílogo entrou por pedido do dono em 26/08/2026; antes ela era
  //	clara de ponta a ponta e a tabela não se destacava de nada.
  const ehEscura = caminho.length === 0 || !!atual?.sub?.length
    || atual?.tela === 'orcamentos' || atual?.tela === 'trilogo-dados'
  const iniciais = perfil.nome.trim().slice(0, 2).toUpperCase()

  function navegar(novo: ItemMenu[], sobra: string[] = []) {
    irPara(novo, sobra)
    setNavAberta(false)
    document.body.classList.remove('nav-aberta')
  }

  // O caminho até Minha conta é procurado na árvore, não escrito à mão: assim
  // ele continua certo se o item mudar de lugar amanhã.
  function irParaMinhaConta() {
    for (const bloco of arvore) {
      const filho = bloco.sub?.find(f => f.tela === 'minha-conta')
      if (filho) return navegar([bloco, filho])
      if (bloco.tela === 'minha-conta') return navegar([bloco])
    }
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
    <div className={'lay' + (recolhida ? ' recolhida' : '') + (ehEscura ? ' escura' : '')}>
      <div className="sd-back" onClick={alternarNav} />

      {/* O puxador vive FORA da barra: recolhida, a barra some, e ele precisa
          continuar ali para trazê-la de volta. */}
      <button
        className="sd-puxador"
        type="button"
        onClick={() => setRecolhida(r => !r)}
        title={recolhida ? 'Mostrar o menu' : 'Recolher o menu'}
        aria-label={recolhida ? 'Mostrar o menu' : 'Recolher o menu'}
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.1"
             strokeLinecap="round" strokeLinejoin="round">
          <path d={recolhida ? 'm9 5 7 7-7 7' : 'm15 5-7 7 7 7'} />
        </svg>
      </button>

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
          {/* Duas portas para a MESMA tela: o item em Configurações e este clique.
              Quem pensa "minhas configurações" acha no menu; quem pensa "minha
              conta" clica no próprio nome. Nenhuma das duas obriga a lembrar da
              outra (P-29). */}
          <button className="sd-eu" type="button" onClick={irParaMinhaConta} title="Minha conta">
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
          {/* VOLTAR EM TODA TELA
              Estar dentro de alguma coisa e não ter como sair dela obriga a
              pessoa a caçar o item no menu — ou a usar o botão do navegador,
              que nem todo mundo lembra que funciona aqui. */}
          {caminho.length > 0 && (
            <button
              type="button"
              className="tp-voltar"
              aria-label="Voltar"
              title="Voltar"
              onClick={() =>
                // Dentro de uma sub-tela, voltar FECHA a sub-tela. Só depois é
                // que sai do nível. Pular direto seria fazer um clique desfazer
                // dois passos.
                extra.length > 0 ? navegar(caminho) : navegar(caminho.slice(0, -1))
              }
            >
              ←
            </button>
          )}
          <div className="tp-info">
            {/* A MIGALHA É CLICÁVEL
                Ela já mostra o caminho inteiro; deixá-la inerte era desenhar um
                mapa e não deixar ninguém andar nele. */}
            <div className="crumb">
              <button type="button" className="crumb-link" onClick={() => navegar([])}>
                FrotaHub
              </button>
              {caminho.map((c, i) => (
                <span key={c.t}>
                  {' › '}
                  {i === caminho.length - 1 && extra.length === 0 ? (
                    // O último é onde já estamos: vira texto, não link. Link que
                    // não leva a lugar nenhum ensina a pessoa a não clicar.
                    <b className="crumb-aqui">{c.t}</b>
                  ) : (
                    <button
                      type="button"
                      className="crumb-link"
                      onClick={() => navegar(caminho.slice(0, i + 1))}
                    >
                      {c.t}
                    </button>
                  )}
                </span>
              ))}
            </div>
            {/* O TÍTULO SÓ APARECE ONDE NÃO HÁ HERO
                Antes, "Contrato São Luiz" saía TRÊS vezes na mesma tela: na
                migalha, aqui, e no título grande logo abaixo. Nas telas que têm
                hero, este some; nas que não têm, ele é o único título. */}
            {!atual?.sub?.length && (
              <div className="titulo">{atual ? atual.t : 'Início'}</div>
            )}
          </div>
        </header>

        {/* A tela de chamados é uma tabela larga: nela o limite de leitura
            confortável atrapalha mais do que ajuda. */}
        <main className={'content'
          + ((atual?.tela === 'trilogo-dados' || atual?.tela === 'orcamentos' || ehEscura) ? ' content-largo' : '')}>
          {caminho.length === 0 ? (
            <Inicio nome={perfil.nome} arvore={arvore} abrir={navegar} />
          ) : atual?.sub?.length ? (
            // O MESMO desenho de barras da tela inicial e do painel de
            // Orçamentos. Um esquema de menu só para o programa inteiro.
            <>
              <header className="hero">
                <h1>{atual.t}</h1>
                <p>{atual.desc}</p>
              </header>
              <Painel
                etapas={etapasDoMenu(atual.sub)}
                aoEscolher={rota => {
                  const filho = atual.sub!.find(f => f.rota === rota)
                  if (filho) navegar([...caminho, filho])
                }}
              />
            </>
          ) : atual?.tela === 'usuarios' ? (
            <Usuarios perfil={perfil} />
          ) : atual?.tela === 'categorias' ? (
            <Categorias />
          ) : atual?.tela === 'minha-conta' ? (
            <MinhaConta perfil={perfil} />
          ) : atual?.tela === 'orcamentos' ? (
            // A sub-tela também vem do endereço, pelo mesmo motivo do ticket
            // abaixo: voltar tem que fechar a sub-tela, não sair do sistema.
            <Orcamentos
              onde={extra[0]}
              abrir={onde => navegar(caminho, [onde])}
              voltar={() => navegar(caminho)}
            />
          ) : atual?.tela === 'trilogo-dados' ? (
            // O ticket aberto vem do ENDEREÇO, não de um estado escondido: é o
            // que faz o voltar do navegador fechar a ficha em vez de sair do
            // sistema, e faz um link para um chamado abrir aquele chamado.
            <DadosTrilogo
              ticket={extra[0]}
              abrir={numero => navegar(caminho, [String(numero)])}
              voltar={() => navegar(caminho)}
            />
          ) : (
            <EmBreve titulo={atual!.t} />
          )}
        </main>
      </div>

    </div>
  )
}
