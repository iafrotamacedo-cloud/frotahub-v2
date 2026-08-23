// rev 1 — a conexão com o Supabase (banco e login)
//
// Sobre estas duas constantes estarem escritas aqui, num repositório público:
// elas SÃO públicas por natureza. A chave "publishable" é feita para viajar dentro
// do navegador de todo usuário — não há como escondê-la de quem abre o site.
// Quem protege os dados é a segurança de linha do banco (CORE-19), não o segredo
// desta chave. A chave de serviço, essa sim secreta, nunca sai do servidor (CORE-21).
import { createClient } from '@supabase/supabase-js'

export const SUPABASE_URL = 'https://hltcngamdqabqlocufrv.supabase.co'
export const SUPABASE_CHAVE_PUBLICA = 'sb_publishable_IyCf5yioEo-Ry8q5mB2boA__ekxvPIu'

// Domínio usado para transformar um usuário curto ("builder") num e-mail, que é o
// que o Supabase espera. Ninguém digita e-mail para entrar no FrotaHub.
export const DOMINIO_LOGIN = 'frotahub.local'

export function usuarioParaEmail(entrada: string): string {
  const u = entrada.trim().toLowerCase()
  return u.includes('@') ? u : `${u}@${DOMINIO_LOGIN}`
}

export const supabase = createClient(SUPABASE_URL, SUPABASE_CHAVE_PUBLICA, {
  auth: {
    persistSession: true,
    autoRefreshToken: true,
  },
})
