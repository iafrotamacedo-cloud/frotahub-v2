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
import { MODULOS, type Categoria, type Matriz, type Rotina } from './tipos'

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

  // O PAINEL DE MÓDULOS SÓ EXISTE NUMA CATEGORIA CEO
  //
  //	E só quem chega aqui olhando uma categoria CEO é o builder — o próprio
  //	CEO nunca abre a permissão de outro CEO (o backend recusa antes,
  //	foraDoAlcanceDoCEO em acesso.go). Por isso não precisa de uma segunda
  //	checagem "sou builder" aqui: o nível da categoria já basta.
  const ehCeo = categoria.nivel === 'ceo'
  const [modulos, setModulos] = useState<Set<string>>(new Set())
  const [modulosOriginal, setModulosOriginal] = useState<Set<string>>(new Set())
  const [salvandoModulos, setSalvandoModulos] = useState(false)

  useEffect(() => {
    let vivo = true
    motor<Matriz>(`/categorias/${categoria.id}/permissoes`)
      .then(r => {
        if (!vivo) return
        setDados(r)
        setMarcadas(new Set(r.permitidas))
        setOriginal(new Set(r.permitidas))
        setModulos(new Set(r.modulos_liberados ?? []))
        setModulosOriginal(new Set(r.modulos_liberados ?? []))
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

  const mudouModulos = useMemo(() => {
    if (modulos.size !== modulosOriginal.size) return true
    for (const m of modulos) if (!modulosOriginal.has(m)) return true
    return false
  }, [modulos, modulosOriginal])

  function alternarModulo(valor: string) {
    setModulos(atual => {
      const nova = new Set(atual)
      if (nova.has(valor)) nova.delete(valor)
      else nova.add(valor)
      return nova
    })
  }

  async function salvarModulos() {
    setErro(null)
    setSalvandoModulos(true)
    try {
      const r = await motor<{ liberados?: number; revogados?: number }>(
        `/categorias/${categoria.id}/modulos-liberados`,
        { metodo: 'PUT', corpo: { modulos: [...modulos] } },
      )
      setModulosOriginal(new Set(modulos))
      const partes: string[] = []
      if (r.liberados) partes.push(`${r.liberados} liberado(s)`)
      if (r.revogados) partes.push(`${r.revogados} revogado(s)`)
      aoSalvar(avisoDe(r), partes.length ? `Módulos: ${partes.join(' e ')}.` : 'Nada mudou.')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar os módulos liberados.')
    } finally {
      setSalvandoModulos(false)
    }
  }

  // Ao contrário da matriz e dos módulos — que só valem depois de um clique
  // em "salvar" — esta troca vale na hora: é um catálogo global (a mesma
  // rotina, pra toda categoria), não um estado desta janela em particular.
  async function alternarBypass(codigo: string, novoValor: boolean) {
    setErro(null)
    setDados(atual => atual && {
      ...atual,
      rotinas: atual.rotinas.map(r => r.codigo === codigo ? { ...r, liberada_para_bypass: novoValor } : r),
    })
    try {
      await motor(`/rotinas/${codigo}`, { metodo: 'PATCH', corpo: { liberada_para_bypass: novoValor } })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar esta rotina.')
      // Desfaz na tela — a chamada falhou, o banco não mudou.
      setDados(atual => atual && {
        ...atual,
        rotinas: atual.rotinas.map(r => r.codigo === codigo ? { ...r, liberada_para_bypass: !novoValor } : r),
      })
    }
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

        {dados && ehCeo && (
          <fieldset className="matriz">
            <legend>Módulos liberados (acesso total do builder)</legend>
            <p className="dica">
              Marcar um módulo dá a este CEO TODA rotina dele que já estiver liberada pro
              bypass (coluna "bypass" abaixo) — sem precisar marcar rotina por rotina. O que
              for novo continua oculto até você liberar a rotina em si, mesmo com o módulo já
              marcado aqui.
            </p>
            {MODULOS.map(m => (
              <label className="mx-linha" key={m.valor}>
                <input type="checkbox" checked={modulos.has(m.valor)} onChange={() => alternarModulo(m.valor)} />
                <span className="mx-nome">{m.rotulo}</span>
              </label>
            ))}
            <div className="jn-pe" style={{ padding: 0, marginTop: 10 }}>
              <button
                type="button" className="bt bt-forte" onClick={() => void salvarModulos()}
                disabled={salvandoModulos || !mudouModulos}
              >
                {salvandoModulos ? 'Salvando...' : 'Salvar módulos'}
              </button>
            </div>
          </fieldset>
        )}

        {dados && !dados.ignora_matriz && porModulo.map(([modulo, rotinas]) => (
          <fieldset className="matriz" key={modulo}>
            <legend>{modulo}</legend>
            {rotinas.map(r => (
              // Dois <label> irmãos, nunca um dentro do outro — aninhado, o
              // clique no checkbox de bypass também dispara o da esquerda
              // (o navegador ativa o PRIMEIRO input de um <label> ao clicar
              // em qualquer parte dele).
              <div className="mx-linha-grupo" key={r.codigo}>
                <label className="mx-linha">
                  <input
                    type="checkbox"
                    checked={marcadas.has(r.codigo)}
                    onChange={() => alternar(r.codigo)}
                  />
                  <span className="mx-nome">{r.nome}</span>
                  <code>{r.codigo}</code>
                </label>
                {ehCeo && (
                  <label className="mx-bypass" title="Liberada pro bypass de módulo do CEO — vale pra toda categoria, não só esta.">
                    <input
                      type="checkbox"
                      checked={!!r.liberada_para_bypass}
                      onChange={() => void alternarBypass(r.codigo, !r.liberada_para_bypass)}
                    />
                    bypass
                  </label>
                )}
              </div>
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
