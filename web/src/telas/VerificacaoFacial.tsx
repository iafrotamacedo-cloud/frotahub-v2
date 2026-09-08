// rev 1 — o segundo fator do login
//
// A senha já confirmou — falta o rosto. Mesma casca visual do login (P-19: a
// identidade da casa é a mesma em todo lugar), porque para quem está entrando
// isto ainda É o login, só que numa segunda etapa.
//
// A comparação roda no NAVEGADOR de quem está entrando: o molde salvo (o
// "descritor") já chegou até aqui pela política de linha (só o dono lê o
// próprio), e o que a câmera captura nunca sai daqui — nada disto passa pelo
// motor (CORE-09).
import { lazy, Suspense, useState } from 'react'
import { Marca } from '../componentes/Marca'
import { ehOMesmoRosto } from '../reconhecimento/distancia'
import type { PendenteFacial } from '../sessao/useSessao'

// Adiado: só quem CHEGA nesta tela (quem ativou o segundo fator) paga o peso
// de carregar a `face-api.js` — ninguém mais baixa isso ao abrir o login.
const CapturaFacial = lazy(() => import('../reconhecimento/CapturaFacial').then(m => ({ default: m.CapturaFacial })))

interface Props {
  pendente: PendenteFacial
  confirmar: (sucesso: boolean) => void
}

export function VerificacaoFacial({ pendente, confirmar }: Props) {
  const [erro, setErro] = useState<string | null>(null)

  function aoCapturar(descritor: Float32Array) {
    if (ehOMesmoRosto(descritor, pendente.descritor)) {
      setErro(null)
      confirmar(true)
      return
    }
    setErro('Não reconheci esse rosto. Tente de novo, com boa luz e olhando para a câmera.')
  }

  return (
    <div className="auth">
      <aside className="auth-marca">
        <Marca />
        <h2>Gestão integrada da <em>Frota Macedo Engenharia</em></h2>
        <div className="ft">Administrativo · Manutenção · Engenharia · SESMT — num só lugar.</div>
      </aside>

      <main className="auth-form">
        <div className="auth-card">
          <h3>Confirme que é você, {pendente.perfil.nome.split(' ')[0]}</h3>
          <p className="d">A senha está certa. Falta olhar para a câmera para confirmar o seu rosto.</p>

          <Suspense fallback={<div className="captura-facial-carregando" style={{ position: 'static', color: 'var(--gray)' }}>Preparando a câmera…</div>}>
            <CapturaFacial rotuloBotao="Confirmar meu rosto" aoCapturar={aoCapturar} />
          </Suspense>

          {erro && <div className="erro-caixa">{erro}</div>}

          <button
            type="button"
            className="bt bt-neutro"
            style={{ width: '100%', marginTop: 14 }}
            onClick={() => confirmar(false)}
          >
            Cancelar e voltar ao login
          </button>
        </div>
      </main>
    </div>
  )
}
