// rev 1 — a matriz: o que esta categoria alcança
//
// A tela manda a lista COMPLETA do que deve ficar marcado, e o motor calcula a
// diferença. É mais simples de acertar do que mandar "marque isto, desmarque
// aquilo": se dois builders salvarem quase junto, o último grava um estado inteiro
// coerente, em vez de metade de dois estados.
//
// SOBRE O QUADRO ABRIR VAZIO
//   O catálogo de rotinas nasce sem nada, e cada rotina só se cadastra quando o
//   módulo dela é construído. Enquanto não houver módulo de negócio, não há o que
//   marcar — e isso é o desenho funcionando, não tela quebrada. A mensagem diz isso
//   com todas as letras, para ninguém procurar defeito onde não tem.
import { useEffect, useMemo, useState } from 'react'
import { Janela } from '../../componentes/Janela'
import { Carregando } from '../../componentes/Carregando'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import type { Categoria, Matriz, Rotina } from './tipos'

interface Props {
  categoria: Categoria
  aoFechar: () => void
  aoSalvar: (aviso: string | null, recado: string) => void
}

export function Permissoes({ categoria, aoFechar, aoSalvar }: Props) {
  const [dados, setDados] = useState<Matriz | null>(null)
  const [marcadas, setMarcadas] = useState<Set<string>>(new Set())
  const [original, setOriginal] = useState<Set<string>>(new Set())
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  useEffect(() => {
    let vivo = true
    motor<Matriz>(`/categorias/${categoria.id}/permissoes`)
      .then(r => {
        if (!vivo) return
        setDados(r)
        setMarcadas(new Set(r.permitidas))
        setOriginal(new Set(r.permitidas))
      })
      .catch(e => { if (vivo) setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a matriz.') })
    return () => { vivo = false }
  }, [categoria.id])

  // As rotinas vêm do motor já ordenadas; aqui só se agrupa por módulo para o
  // quadro ficar legível quando forem dezenas.
  const porModulo = useMemo(() => {
    const mapa = new Map<string, Rotina[]>()
    for (const r of dados?.rotinas ?? []) {
      const lista = mapa.get(r.modulo) ?? []
      lista.push(r)
      mapa.set(r.modulo, lista)
    }
    return [...mapa.entries()]
  }, [dados])

  const mudou = useMemo(() => {
    if (marcadas.size !== original.size) return true
    for (const c of marcadas) if (!original.has(c)) return true
    return false
  }, [marcadas, original])

  function alternar(codigo: string) {
    setMarcadas(atual => {
      const nova = new Set(atual)
      if (nova.has(codigo)) nova.delete(codigo)
      else nova.add(codigo)
      return nova
    })
  }

  async function salvar() {
    setErro(null)
    setSalvando(true)
    try {
      const r = await motor<{ liberadas?: number; retiradas?: number }>(
        `/categorias/${categoria.id}/permissoes`,
        { metodo: 'PUT', corpo: { rotinas: [...marcadas] } },
      )
      const partes: string[] = []
      if (r.liberadas) partes.push(`${r.liberadas} liberada(s)`)
      if (r.retiradas) partes.push(`${r.retiradas} retirada(s)`)
      aoSalvar(avisoDe(r), partes.length ? `Permissões salvas: ${partes.join(' e ')}.` : 'Nada mudou.')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar as permissões.')
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo="Permissões"
      descricao={`${categoria.nome} · nível ${categoria.nivel}`}
      aoFechar={aoFechar}
      largura={560}
    >
      <div className="jn-corpo">
        {erro && <div className="erro-caixa">{erro}</div>}
        {!erro && dados === null && <Carregando texto="Carregando a matriz..." />}

        {dados?.ignora_matriz && (
          <div className="aviso-caixa">
            Esta é a categoria do dono do sistema. Ela alcança tudo por construção — marcar
            rotinas aqui não teria efeito nenhum, e por isso o quadro está desabilitado.
          </div>
        )}

        {dados && !dados.ignora_matriz && dados.rotinas.length === 0 && (
          <div className="vazio-inline">
            O catálogo de rotinas ainda está vazio, e isso é esperado: cada rotina se
            cadastra quando o módulo dela é construído. Assim não existe permissão para
            algo que não existe. Quando Manutenção e Serviços forem feitos, eles aparecem
            aqui sozinhos.
          </div>
        )}

        {dados && !dados.ignora_matriz && porModulo.map(([modulo, rotinas]) => (
          <fieldset className="matriz" key={modulo}>
            <legend>{modulo}</legend>
            {rotinas.map(r => (
              <label className="mx-linha" key={r.codigo}>
                <input
                  type="checkbox"
                  checked={marcadas.has(r.codigo)}
                  onChange={() => alternar(r.codigo)}
                />
                <span className="mx-nome">{r.nome}</span>
                <code>{r.codigo}</code>
              </label>
            ))}
          </fieldset>
        ))}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Fechar</button>
          {dados && !dados.ignora_matriz && dados.rotinas.length > 0 && (
            <button type="button" className="bt bt-forte" onClick={() => void salvar()} disabled={salvando || !mudou}>
              {salvando ? 'Salvando...' : 'Salvar permissões'}
            </button>
          )}
        </div>
      </div>
    </Janela>
  )
}
