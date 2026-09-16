// rev 1 — o portal do fornecedor: só isto, e mais nada
//
// LOGIN DIRETO NESTA TELA, SEM CASCA NENHUMA (074_portal_fornecedor.sql)
//
//	Quem entra aqui é gente de fora — nível `fornecedor`, que não faz parte
//	da hierarquia interna do FrotaHub (ver o comentário em `sessao/tipos.ts`).
//	`App.tsx` desvia para este componente ANTES de montar a barra lateral e o
//	cabeçalho da casca normal: sem menu, sem outra tela alcançável, só o
//	envio de nota/DAV e a própria senha.
//
// SEM LISTA, SEM STATUS — DE PROPÓSITO (pedido do dono, 16/09/2026)
//
//	"Sem lista nem status, só enviar." O fornecedor manda o arquivo e pronto;
//	não acompanha se virou orçamento, se foi rejeitado, nada. Isso também
//	casa com o motor: a rotina que este login tem
//	(`CONTRATO_ORCAMENTOS_NOTAS_FORNECEDOR`) só destrava INSERIR — ver
//	`quemInsereDocumento` em `orcamentos/documentos.go`. Uma tela que
//	tentasse listar bateria numa porta que essa rotina não abre.
import { useRef, useState } from 'react'
import { Insercao } from '../orcamentos/Arquivos'
import { MinhaConta } from '../MinhaConta'
import { Marca } from '../../componentes/Marca'
import type { Perfil } from '../../sessao/tipos'

export function PortalFornecedor({ perfil, sair }: { perfil: Perfil; sair: () => void }) {
  const [trocandoSenha, setTrocandoSenha] = useState(false)

  return (
    <div className="pf-pagina">
      <header className="pf-topo">
        <Marca />
        <div className="pf-topo-dir">
          <span className="pf-quem">{perfil.nome}</span>
          {trocandoSenha ? (
            <button type="button" className="bt bt-neutro" onClick={() => setTrocandoSenha(false)}>
              ← voltar
            </button>
          ) : (
            <button type="button" className="bt bt-neutro" onClick={() => setTrocandoSenha(true)}>
              Trocar senha
            </button>
          )}
          <button type="button" className="bt bt-neutro" onClick={() => void sair()}>Sair</button>
        </div>
      </header>

      <main className="content pf-corpo">
        {trocandoSenha ? <MinhaConta perfil={perfil} /> : <EnviarDav />}
      </main>
    </div>
  )
}

function EnviarDav() {
  const [mensagem, setMensagem] = useState<{ ok: boolean; texto: string } | null>(null)
  // `aoFalhar` sempre chega ANTES de `aoTerminar`, na mesma chamada de
  // `inserir()` (Arquivos.tsx) — a ref segura esse valor para `aoTerminar`
  // saber, sem depender do estado do React ainda não ter sido aplicado.
  const ultimoErro = useRef('')

  return (
    <>
      <header className="hero">
        <h1>Enviar nota ou DAV</h1>
        <p>Escolha o arquivo e clique em inserir. É só isso.</p>
      </header>

      {mensagem && (
        <p className={mensagem.ok ? 'recado' : 'erro-caixa'} role="status">
          {mensagem.texto}
        </p>
      )}

      <Insercao
        fila="orcamento"
        aoFalhar={s => {
          ultimoErro.current = s
          if (s) setMensagem({ ok: false, texto: s })
        }}
        aoTerminar={async () => {
          if (!ultimoErro.current) setMensagem({ ok: true, texto: 'Enviado. Obrigado!' })
        }}
      />
    </>
  )
}
