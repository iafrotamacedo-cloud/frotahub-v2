// rev 1 — criar e editar categoria
//
// O CÓDIGO NÃO SE EDITA depois de criado, pela mesma razão que o usuário de um
// login não se edita: ele é a identidade do registro no histórico. Mudar faria o
// rastro antigo falar de um código que não existe mais. O nome, esse sim, muda à
// vontade — é ele que aparece na tela.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import { NIVEIS, type Categoria } from './tipos'
import type { Nivel } from '../../sessao/tipos'

interface Props {
  categoria: Categoria | null // null = criar
  aoFechar: () => void
  aoSalvar: (aviso: string | null) => void
}

export function FormCategoria({ categoria, aoFechar, aoSalvar }: Props) {
  const editando = categoria !== null

  const [codigo, setCodigo] = useState(categoria?.codigo ?? '')
  const [nome, setNome] = useState(categoria?.nome ?? '')
  const [nivel, setNivel] = useState<Nivel>(categoria?.nivel ?? 'comum')
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    setErro(null)
    setSalvando(true)
    try {
      let r: unknown
      if (editando) {
        const mudou: Record<string, unknown> = {}
        if (nome.trim() !== categoria.nome) mudou.nome = nome.trim()
        if (nivel !== categoria.nivel) mudou.nivel = nivel
        if (Object.keys(mudou).length === 0) { aoFechar(); return }
        r = await motor(`/categorias/${categoria.id}`, { metodo: 'PATCH', corpo: mudou })
      } else {
        r = await motor('/categorias', {
          metodo: 'POST',
          corpo: { codigo: codigo.trim().toLowerCase(), nome: nome.trim(), nivel },
        })
      }
      aoSalvar(avisoDe(r))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar.')
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo={editando ? 'Editar categoria' : 'Nova categoria'}
      descricao={editando ? categoria.codigo : 'O grupo a que os logins pertencem. O nível vem daqui, não da pessoa.'}
      aoFechar={aoFechar}
    >
      <form className="jn-corpo" onSubmit={enviar}>
        {!editando && (
          <>
            <label htmlFor="c-codigo">Código</label>
            <input
              id="c-codigo" value={codigo} autoComplete="off"
              onChange={e => setCodigo(e.target.value.replace(/\s/g, '').toLowerCase())}
              placeholder="administrativo"
            />
            <p className="dica">Nome curto, sem espaço e sem acento. É a identidade da categoria no histórico, e não muda depois.</p>
          </>
        )}

        <label htmlFor="c-nome">Nome</label>
        <input id="c-nome" value={nome} onChange={e => setNome(e.target.value)} placeholder="Administrativo" />
        <p className="dica">É este que aparece nas telas.</p>

        <label htmlFor="c-nivel">Nível</label>
        <select id="c-nivel" value={nivel} onChange={e => setNivel(e.target.value as Nivel)}>
          {NIVEIS.map(n => <option key={n.valor} value={n.valor}>{n.rotulo}</option>)}
        </select>
        <p className="dica">
          Quem entra nesta categoria herda o nível. Ele não decide acesso sozinho —
          quem decide é a matriz de permissões.
        </p>

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando}>
            {salvando ? 'Salvando...' : editando ? 'Salvar' : 'Criar categoria'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
