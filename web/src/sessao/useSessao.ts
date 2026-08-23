// rev 1 — a sessão do usuário
//
// Uma responsabilidade só: dizer QUEM está logado (CORE-17). O que essa pessoa PODE
// fazer é assunto de permissão, que entra quando as rotinas existirem.
import { useCallback, useEffect, useState } from 'react'
import { supabase, usuarioParaEmail } from '../supabase/cliente'
import type { Perfil } from './tipos'

interface Estado {
  carregando: boolean
  perfil: Perfil | null
}

export function useSessao() {
  const [estado, setEstado] = useState<Estado>({ carregando: true, perfil: null })

  const carregarPerfil = useCallback(async (userId: string): Promise<Perfil | null> => {
    // A segurança de linha do banco garante que cada um só enxerga a própria linha.
    const { data, error } = await supabase
      .from('perfis')
      .select('id, usuario, nome, nivel, ativo')
      .eq('id', userId)
      .maybeSingle()

    if (error || !data) return null
    return data as Perfil
  }, [])

  useEffect(() => {
    let vivo = true

    // Sessão guardada de um acesso anterior.
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
    // Mensagem escrita para ser lida por quem usa (CORE-24).
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
    setEstado({ carregando: false, perfil })
    return null
  }, [carregarPerfil])

  const sair = useCallback(async () => {
    await supabase.auth.signOut()
    setEstado({ carregando: false, perfil: null })
  }, [])

  return { ...estado, entrar, sair }
}
