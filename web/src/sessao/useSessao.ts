// rev 3 — a sessão do usuário
//
// Uma responsabilidade só: dizer QUEM está logado.
//
// Por que o perfil é lido direto do banco, e não do motor: para a tela abrir, basta
// saber quem entrou — e o banco responde na hora. Se dependesse do motor, o primeiro
// acesso do dia esperaria o serviço acordar antes de mostrar qualquer coisa. O motor
// entra quando a ação exige servidor (criar login, por exemplo).
//
// A segurança de linha garante que cada um só enxerga a própria linha.
import { useCallback, useEffect, useState } from 'react'
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

interface Estado {
  carregando: boolean
  perfil: Perfil | null
}

export function useSessao() {
  const [estado, setEstado] = useState<Estado>({ carregando: true, perfil: null })

  const carregarPerfil = useCallback(async (userId: string): Promise<Perfil | null> => {
    const { data, error } = await supabase
      .from('perfis')
      .select('id, usuario, nome, ativo, cliente_id, categoria_id, categorias(nome, nivel)')
      .eq('id', userId)
      .maybeSingle<LinhaPerfil>()

    if (error || !data) return null

    return {
      id: data.id,
      usuario: data.usuario,
      nome: data.nome,
      ativo: data.ativo,
      clienteId: data.cliente_id,
      categoriaId: data.categoria_id,
      categoriaNome: data.categorias?.nome ?? '',
      nivel: data.categorias?.nivel ?? 'comum',
    }
  }, [])

  useEffect(() => {
    let vivo = true

    supabase.auth.getSession().then(async ({ data }) => {
      if (!vivo) return
      const uid = data.session?.user?.id
      const perfil = uid ? await carregarPerfil(uid) : null
      if (vivo) setEstado({ carregando: false, perfil })
    })

    // Entrar e sair em qualquer aba refletem aqui.
    const { data: sub } = supabase.auth.onAuthStateChange(async (_evento, sessao) => {
      if (!vivo) return
      const uid = sessao?.user?.id
      const perfil = uid ? await carregarPerfil(uid) : null
      if (vivo) setEstado({ carregando: false, perfil })
    })

    return () => { vivo = false; sub.subscription.unsubscribe() }
  }, [carregarPerfil])

  const entrar = useCallback(async (usuario: string, senha: string): Promise<string | null> => {
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
    // Os carimbos de sessão nascem aqui, no único ponto em que alguém entra.
    marcarInicioDeSessao()
    setEstado({ carregando: false, perfil })
    return null
  }, [carregarPerfil])

  const sair = useCallback(async () => {
    limparMarcasDeSessao()
    await supabase.auth.signOut()
    setEstado({ carregando: false, perfil: null })
  }, [])

  return { ...estado, entrar, sair }
}
