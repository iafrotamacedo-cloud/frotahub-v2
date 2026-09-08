// rev 5 — a sessão do usuário
//
// Uma responsabilidade só: dizer QUEM está logado.
//
// Por que o perfil é lido direto do banco, e não do motor: para a tela abrir, basta
// saber quem entrou — e o banco responde na hora. Se dependesse do motor, o primeiro
// acesso do dia esperaria o serviço acordar antes de mostrar qualquer coisa. O motor
// entra quando a ação exige servidor (criar login, por exemplo).
//
// A segurança de linha garante que cada um só enxerga a própria linha.
//
// O QUE MUDOU NA REVISÃO 5 — o segundo fator opcional
//
//   Quem ativou a verificação facial (Minha conta) tem o login em DUAS etapas:
//   a senha confere primeiro, e só DEPOIS o rosto. Enquanto o rosto não é
//   confirmado, `perfil` continua nulo e `pendenteFacial` carrega quem está
//   tentando entrar — é o que faz a tela trocar para a câmera em vez de abrir
//   o sistema.
//
//   `entrarEmAndamento` existe por causa de uma corrida: `signInWithPassword`
//   dispara o mesmo evento (`SIGNED_IN`) que o `onAuthStateChange` já escuta
//   para refletir login feito em OUTRA aba. Sem a trava, as duas rotas
//   correriam juntas — o listener abriria o sistema direto, por cima da
//   checagem de rosto que `entrar` ainda não tinha terminado de fazer. A trava
//   só desarma quando `entrar` já decidiu o destino final (login pronto, ou
//   esperando o rosto) — daí para frente, o listener volta a cuidar de
//   sincronizar entre abas.
import { useCallback, useEffect, useRef, useState } from 'react'
import { supabase, usuarioParaEmail } from '../supabase/cliente'
import type { Nivel, Perfil } from './tipos'
import { limparMarcasDeSessao, marcarInicioDeSessao } from './inatividade'

interface LinhaPerfil {
  id: string
  usuario: string
  nome: string
  ativo: boolean
  cliente_id: string
  categoria_id: string
  categorias: { nome: string; nivel: Nivel } | null
}

/** Quem está tentando entrar, esperando confirmar o próprio rosto. */
export interface PendenteFacial {
  perfil: Perfil
  descritor: number[]
}

interface Estado {
  carregando: boolean
  perfil: Perfil | null
  pendenteFacial: PendenteFacial | null
}

export function useSessao() {
  const [estado, setEstado] = useState<Estado>({ carregando: true, perfil: null, pendenteFacial: null })
  const entrarEmAndamento = useRef(false)

  const carregarPerfil = useCallback(async (userId: string): Promise<Perfil | null> => {
    const { data, error } = await supabase
      .from('perfis')
      .select('id, usuario, nome, ativo, cliente_id, categoria_id, categorias(nome, nivel)')
      .eq('id', userId)
      .maybeSingle<LinhaPerfil>()

    if (error || !data) return null

    // As rotinas vêm do MESMO lugar que o perfil: o banco, direto, sem passar
    // pelo motor. Se dependessem dele, o menu só apareceria depois de o serviço
    // acordar no Render — e o primeiro acesso do dia abriria sem menu nenhum.
    //
    // A segurança de linha já limita a consulta à categoria de quem perguntou;
    // o filtro explícito está aqui para quem lê o código não precisar ir
    // conferir a política para entender o que volta.
    const { data: permissoes } = await supabase
      .from('categoria_permissoes')
      .select('rotina')
      .eq('categoria_id', data.categoria_id)
      .eq('pode', true)

    return {
      id: data.id,
      usuario: data.usuario,
      nome: data.nome,
      ativo: data.ativo,
      clienteId: data.cliente_id,
      categoriaId: data.categoria_id,
      categoriaNome: data.categorias?.nome ?? '',
      nivel: data.categorias?.nivel ?? 'comum',
      rotinas: (permissoes ?? []).map(p => p.rotina as string),
    }
  }, [])

  useEffect(() => {
    let vivo = true

    supabase.auth.getSession().then(async ({ data }) => {
      if (!vivo || entrarEmAndamento.current) return
      const uid = data.session?.user?.id
      const perfil = uid ? await carregarPerfil(uid) : null
      if (vivo && !entrarEmAndamento.current) setEstado({ carregando: false, perfil, pendenteFacial: null })
    })

    // Entrar e sair em qualquer aba refletem aqui — exceto enquanto ESTA aba
    // está no meio de um `entrar()` próprio, que pode ainda estar esperando a
    // confirmação do rosto (ver a nota da revisão 5, no topo do arquivo).
    const { data: sub } = supabase.auth.onAuthStateChange(async (_evento, sessao) => {
      if (!vivo || entrarEmAndamento.current) return
      const uid = sessao?.user?.id
      const perfil = uid ? await carregarPerfil(uid) : null
      if (vivo && !entrarEmAndamento.current) setEstado({ carregando: false, perfil, pendenteFacial: null })
    })

    return () => { vivo = false; sub.subscription.unsubscribe() }
  }, [carregarPerfil])

  const entrar = useCallback(async (usuario: string, senha: string): Promise<string | null> => {
    entrarEmAndamento.current = true
    try {
      const { data, error } = await supabase.auth.signInWithPassword({
        email: usuarioParaEmail(usuario),
        password: senha,
      })
      // Mensagem escrita para ser lida por quem usa (P-18).
      if (error) return 'Usuário ou senha inválidos.'

      const uid = data.user?.id
      const perfil = uid ? await carregarPerfil(uid) : null
      if (!perfil) {
        await supabase.auth.signOut()
        return 'Este login existe, mas ainda não tem perfil no FrotaHub. Fale com o administrador.'
      }
      if (!perfil.ativo) {
        await supabase.auth.signOut()
        return 'Este login está desativado.'
      }

      // Segundo fator opcional: quem ativou (Minha conta) tem um molde salvo.
      // Achou o molde → o login não termina ainda, a tela troca para a
      // câmera. `confirmarBiometria` é quem completa (ou desfaz) a partir daqui.
      const { data: bio } = await supabase
        .from('perfil_biometria_facial')
        .select('descritor')
        .eq('perfil_id', uid as string)
        .maybeSingle<{ descritor: number[] }>()

      if (bio?.descritor) {
        setEstado({ carregando: false, perfil: null, pendenteFacial: { perfil, descritor: bio.descritor } })
        return null
      }

      // Os carimbos de sessão nascem aqui, no único ponto em que alguém entra.
      marcarInicioDeSessao()
      setEstado({ carregando: false, perfil, pendenteFacial: null })
      return null
    } finally {
      entrarEmAndamento.current = false
    }
  }, [carregarPerfil])

  // Fecha a etapa do rosto: `sucesso` abre o sistema, `!sucesso` desfaz o
  // login inteiro — a senha certa sozinha não basta para quem tem o segundo
  // fator ativado.
  const confirmarBiometria = useCallback(async (sucesso: boolean) => {
    const pendente = estado.pendenteFacial
    if (!pendente) return
    if (sucesso) {
      marcarInicioDeSessao()
      setEstado({ carregando: false, perfil: pendente.perfil, pendenteFacial: null })
    } else {
      await supabase.auth.signOut()
      setEstado({ carregando: false, perfil: null, pendenteFacial: null })
    }
  }, [estado.pendenteFacial])

  const sair = useCallback(async () => {
    limparMarcasDeSessao()
    await supabase.auth.signOut()
    setEstado({ carregando: false, perfil: null, pendenteFacial: null })
  }, [])

  return { ...estado, entrar, confirmarBiometria, sair }
}
