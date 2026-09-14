// rev 1 — o organograma de um login (14/09/2026)
//
// SÓ O DIAGRAMA, NADA DE PAINEL SEPARADO
//
//	Pedido do dono: nada de "esquerda mostra, direita comanda" — a própria
//	árvore é o comando. Cada linha que liga dois nós ganha um botão "+"; ele
//	abre um popup pra escolher quem entra ali no meio, e a árvore redesenha
//	sozinha (o `motor` já devolveu a cadeia nova).
//
// O DEFAULT NUNCA É GRAVADO
//
//	Antes de qualquer configuração, a cadeia é sempre "CEO > este login" —
//	mas o nó do CEO chega marcado `implicito`, porque `vinculos_hierarquicos`
//	não tem nenhuma linha ainda (ver o cabeçalho de `hierarquia.go`). É só
//	texto ("ainda não configurado"), não muda o que o "+" faz.
//
// CADA "+" MANDA "QUEM ESTÁ ACIMA" JUNTO
//
//	Não só "insira X aqui" — o motor confere que a linha clicada ainda é a
//	de verdade (`acima_id`) antes de gravar. Duas pessoas mexendo ao mesmo
//	tempo não conseguem se pisar em silêncio.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { Janela } from '../../componentes/Janela'
import type { NoCadeia, PerfilLeve } from './tipos'

interface Props {
  perfilId: string
}

// Espelha nivelEncaixaAbaixoDe (hierarquia.go) — só pra já mostrar a lista
// certa no popup; quem decide de verdade continua sendo o motor.
function encaixaAbaixoDe(nivel: string, acima: string): boolean {
  switch (nivel) {
    case 'gerencial': return acima === 'ceo'
    case 'supervisorio': return acima === 'gerencial'
    case 'operacional': return acima === 'ceo' || acima === 'gerencial' || acima === 'supervisorio'
    default: return false
  }
}

const ROTULO_NIVEL: Record<string, string> = {
  builder: 'Builder', ceo: 'CEO', gerencial: 'Gerencial', supervisorio: 'Supervisório', operacional: 'Operacional',
}

export function VinculoHierarquico({ perfilId }: Props) {
  const [cadeia, setCadeia] = useState<NoCadeia[] | null>(null)
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [linhaAberta, setLinhaAberta] = useState<{ acimaId: string; qId: string; acimaNivel: string } | null>(null)
  const [removendo, setRemovendo] = useState<string | null>(null)

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const r = await motor<{ cadeia: NoCadeia[] }>(`/perfis/${perfilId}/hierarquia`)
      setCadeia(r.cadeia)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a hierarquia.')
    }
  }, [perfilId])

  useEffect(() => { void carregar() }, [carregar])

  async function remover(id: string) {
    setRemovendo(id)
    setErro(null)
    try {
      await motor(`/perfis/${id}/hierarquia`, { metodo: 'DELETE' })
      setRecado('Vínculo removido — voltou a herdar o padrão.')
      await carregar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui remover este vínculo.')
    } finally {
      setRemovendo(null)
    }
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Vínculo hierárquico</h1>
          <p>
            {cadeia ? `${cadeia[cadeia.length - 1]?.nome} · ${cadeia[cadeia.length - 1]?.usuario}` : 'Carregando...'}
          </p>
        </div>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      {cadeia === null ? (
        <Carregando texto="Montando o organograma..." />
      ) : (
        <div className="vh-arvore">
          {cadeia.map((no, i) => (
            <div key={no.id} className="vh-grupo">
              <div className="vh-no">
                <span className="vh-nivel">{ROTULO_NIVEL[no.nivel] ?? no.nivel}</span>
                <b className="vh-nome">{no.nome}</b>
                <span className="vh-meta">
                  <code>{no.usuario}</code> · {no.categoria_nome}
                  {no.implicito && ' · ainda não configurado'}
                </span>
                {i > 0 && i < cadeia.length - 1 && !no.implicito && (
                  <button
                    type="button" className="bt bt-mini bt-perigo vh-remover"
                    disabled={removendo === no.id}
                    onClick={() => void remover(no.id)}
                  >
                    {removendo === no.id ? 'Removendo...' : 'Remover deste ponto'}
                  </button>
                )}
              </div>

              {i < cadeia.length - 1 && (
                <div className="vh-conector">
                  <span className="vh-linha" aria-hidden />
                  <button
                    type="button"
                    className="vh-mais"
                    title="Inserir alguém entre estes dois"
                    onClick={() => setLinhaAberta({ acimaId: no.id, qId: cadeia[i + 1].id, acimaNivel: no.nivel })}
                  >
                    +
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {linhaAberta && (
        <PopupInserir
          acimaId={linhaAberta.acimaId}
          qId={linhaAberta.qId}
          acimaNivel={linhaAberta.acimaNivel}
          aoFechar={() => setLinhaAberta(null)}
          aoInserir={() => {
            setLinhaAberta(null)
            setRecado('Inserido na cadeia.')
            void carregar()
          }}
        />
      )}
    </>
  )
}

function PopupInserir({ acimaId, qId, acimaNivel, aoFechar, aoInserir }: {
  acimaId: string
  qId: string
  acimaNivel: string
  aoFechar: () => void
  aoInserir: () => void
}) {
  const [candidatos, setCandidatos] = useState<PerfilLeve[] | null>(null)
  const [escolhido, setEscolhido] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [enviando, setEnviando] = useState(false)

  useEffect(() => {
    let vivo = true
    motor<{ perfis: PerfilLeve[] }>('/perfis')
      .then(r => { if (vivo) setCandidatos(r.perfis.filter(p => encaixaAbaixoDe(p.nivel, acimaNivel) && p.id !== qId)) })
      .catch(e => { if (vivo) setErro(e instanceof ErroMotor ? e.message : 'Não consegui listar quem pode entrar aqui.') })
    return () => { vivo = false }
  }, [acimaNivel, qId])

  async function inserir() {
    if (!escolhido) return
    setErro(null)
    setEnviando(true)
    try {
      await motor(`/perfis/${qId}/hierarquia`, {
        metodo: 'PUT',
        corpo: { acima_id: acimaId, novo_superior_id: escolhido },
      })
      aoInserir()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui inserir.')
    } finally {
      setEnviando(false)
    }
  }

  return (
    <Janela titulo="Inserir na cadeia" descricao="Quem entra logo abaixo do nó de cima?" aoFechar={aoFechar}>
      <div className="jn-corpo">
        {erro && <div className="erro-caixa">{erro}</div>}

        {candidatos === null ? (
          <Carregando texto="Carregando quem pode entrar aqui..." />
        ) : candidatos.length === 0 ? (
          <div className="vazio-inline">
            Ninguém com o nível certo já responde a quem está acima desta linha — a pessoa
            precisa já se reportar a ele antes de poder ser inserida aqui.
          </div>
        ) : (
          <>
            <label htmlFor="vh-candidato">Pessoa</label>
            <select id="vh-candidato" value={escolhido} onChange={e => setEscolhido(e.target.value)}>
              <option value="">Escolha...</option>
              {candidatos.map(c => (
                <option key={c.id} value={c.id}>
                  {c.nome} · {ROTULO_NIVEL[c.nivel] ?? c.nivel}
                </option>
              ))}
            </select>
          </>
        )}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          {candidatos !== null && candidatos.length > 0 && (
            <button type="button" className="bt bt-forte" onClick={() => void inserir()} disabled={!escolhido || enviando}>
              {enviando ? 'Inserindo...' : 'Inserir'}
            </button>
          )}
        </div>
      </div>
    </Janela>
  )
}
