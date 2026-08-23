// rev 1 — criar e editar login
//
// A MESMA janela serve para os dois, porque os campos são os mesmos. O que muda é
// o que ela permite: no login que já existe, o nome curto NÃO se edita.
//
// POR QUE O USUÁRIO NÃO SE EDITA
//   Ele é a identidade da pessoa no histórico e no login. Trocá-lo faria o rastro
//   antigo apontar para um nome que não existe mais. Quem precisar mudar cria
//   outro login e desativa o antigo — dois registros honestos em vez de um
//   registro reescrito.
//
// POR QUE A SENHA NÃO ESTÁ AQUI
//   Porque quem abre esta janela quase sempre quer corrigir um nome. Com a senha
//   no meio do formulário, um salvamento distraído tranca outra pessoa para fora.
//   Trocar senha é ação separada, de propósito.
import { useState, type FormEvent } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor, avisoDe } from '../../motor/cliente'
import type { Categoria, LinhaUsuario } from './tipos'

interface Props {
  usuario: LinhaUsuario | null // null = criar
  categorias: Categoria[]
  aoFechar: () => void
  aoSalvar: (aviso: string | null) => void
}

export function FormUsuario({ usuario, categorias, aoFechar, aoSalvar }: Props) {
  const editando = usuario !== null

  const [nome, setNome] = useState(usuario?.nome ?? '')
  const [curto, setCurto] = useState(usuario?.usuario ?? '')
  const [senha, setSenha] = useState('')
  const [categoriaId, setCategoriaId] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [salvando, setSalvando] = useState(false)

  // A categoria do dono do sistema fica fora da lista. Ela é a trava anti-tranca:
  // colocar alguém nela dá acesso total e permite desativar quem está criando.
  // Um segundo dono deve ser uma decisão deliberada, não um item de lista suspensa.
  const escolhiveis = categorias.filter(c => !c.protegida && c.ativo)

  async function enviar(e: FormEvent) {
    e.preventDefault()
    setErro(null)
    setSalvando(true)
    try {
      let resposta: unknown
      if (editando) {
        const mudou: Record<string, unknown> = {}
        if (nome.trim() !== usuario.nome) mudou.nome = nome.trim()
        if (categoriaId && categoriaId !== '') mudou.categoria_id = categoriaId
        if (Object.keys(mudou).length === 0) { aoFechar(); return }
        resposta = await motor(`/usuarios/${usuario.id}`, { metodo: 'PATCH', corpo: mudou })
      } else {
        resposta = await motor('/usuarios', {
          metodo: 'POST',
          corpo: { usuario: curto.trim().toLowerCase(), nome: nome.trim(), senha, categoria_id: categoriaId },
        })
      }
      aoSalvar(avisoDe(resposta))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui salvar.')
      setSalvando(false)
    }
  }

  return (
    <Janela
      titulo={editando ? 'Editar login' : 'Novo login'}
      descricao={editando ? usuario.usuario : 'A pessoa entra com este nome curto e a senha que você definir aqui.'}
      aoFechar={aoFechar}
    >
      <form className="jn-corpo" onSubmit={enviar}>
        {!editando && (
          <>
            <label htmlFor="u-curto">Usuário</label>
            <input
              id="u-curto" value={curto} autoComplete="off"
              onChange={e => setCurto(e.target.value.replace(/\s/g, '').toLowerCase())}
              placeholder="joao"
            />
            <p className="dica">Sem espaço e sem arroba. É o que a pessoa digita para entrar, e não muda depois.</p>
          </>
        )}

        <label htmlFor="u-nome">Nome</label>
        <input id="u-nome" value={nome} onChange={e => setNome(e.target.value)} placeholder="João da Silva" />

        <label htmlFor="u-cat">Categoria</label>
        {escolhiveis.length === 0 ? (
          <div className="vazio-inline">
            Nenhuma categoria disponível ainda. A tela de <b>Categorias</b> é a
            próxima a ser construída — até lá, não há grupo para colocar ninguém.
          </div>
        ) : (
          <select id="u-cat" value={categoriaId} onChange={e => setCategoriaId(e.target.value)}>
            <option value="">
              {editando ? `Manter — ${usuario.categorias?.nome ?? 'sem categoria'}` : 'Escolha...'}
            </option>
            {escolhiveis.map(c => (
              <option key={c.id} value={c.id}>{c.nome} · {c.nivel}</option>
            ))}
          </select>
        )}

        {!editando && (
          <>
            <label htmlFor="u-senha">Senha inicial</label>
            <input
              id="u-senha" type="password" value={senha} autoComplete="new-password"
              onChange={e => setSenha(e.target.value)}
            />
            <p className="dica">Mínimo de 8 caracteres. A pessoa pode trocar depois, em Minha conta.</p>
          </>
        )}

        {erro && <div className="erro-caixa">{erro}</div>}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button type="submit" className="bt bt-forte" disabled={salvando || (!editando && escolhiveis.length === 0)}>
            {salvando ? 'Salvando...' : editando ? 'Salvar' : 'Criar login'}
          </button>
        </div>
      </form>
    </Janela>
  )
}
