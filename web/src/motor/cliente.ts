// rev 1 — a conversa do front com o baleryan
//
// Existe para que nenhuma tela precise saber montar cabeçalho, achar o token da
// sessão ou traduzir erro. Tela chama `motor(...)` e recebe o resultado ou uma
// mensagem pronta para mostrar (CORE-06).
//
// POR QUE ALGUMAS COISAS VÊM DAQUI E OUTRAS DIRETO DO BANCO
//   Ler o próprio perfil vai direto ao Supabase, porque é o que faz a tela abrir e
//   precisa ser instantâneo. Tudo que exige a chave de serviço — criar login,
//   trocar senha, mexer em permissão — só pode vir por aqui (CORE-09).
import { supabase } from '../supabase/cliente'

// O endereço entra na compilação. Se faltar, é erro de configuração do
// repositório, e a mensagem diz exatamente isso em vez de "falha de rede".
const BASE = (import.meta.env.VITE_MOTOR_URL ?? '').replace(/\/+$/, '')

export class ErroMotor extends Error {
  constructor(public status: number, mensagem: string) {
    super(mensagem)
    this.name = 'ErroMotor'
  }
}

interface Opcoes {
  metodo?: 'GET' | 'POST' | 'PATCH' | 'PUT'
  corpo?: unknown
}

export async function motor<T>(caminho: string, opcoes: Opcoes = {}): Promise<T> {
  if (!BASE) {
    throw new ErroMotor(0,
      'O endereço do motor não foi configurado nesta publicação (VITE_MOTOR_URL). Avise o responsável pelo sistema.')
  }

  const { data } = await supabase.auth.getSession()
  const token = data.session?.access_token
  if (!token) {
    throw new ErroMotor(401, 'A sua sessão expirou. Entre de novo.')
  }

  let resposta: Response
  try {
    resposta = await fetch(BASE + caminho, {
      method: opcoes.metodo ?? 'GET',
      headers: {
        Authorization: `Bearer ${token}`,
        ...(opcoes.corpo !== undefined ? { 'Content-Type': 'application/json' } : {}),
      },
      body: opcoes.corpo !== undefined ? JSON.stringify(opcoes.corpo) : undefined,
    })
  } catch {
    // Cai aqui quando o navegador nem conseguiu falar com o servidor.
    throw new ErroMotor(0, 'Não consegui falar com o servidor. Confira a conexão e tente de novo.')
  }

  const bruto = await resposta.text()
  let corpo: unknown = null
  if (bruto) {
    try { corpo = JSON.parse(bruto) } catch { corpo = null }
  }

  if (!resposta.ok) {
    // O motor escreve o erro para ser lido por gente (P-18). Se por algum motivo
    // não vier assim, a tela ainda diz alguma coisa útil.
    const msg = (corpo as { erro?: string } | null)?.erro
    throw new ErroMotor(resposta.status, msg || 'Alguma coisa deu errado do nosso lado. Tente de novo.')
  }

  return corpo as T
}

/** O aviso que algumas respostas trazem quando a ação deu certo mas o histórico não. */
export function avisoDe(resposta: unknown): string | null {
  const a = (resposta as { aviso?: string } | null)?.aviso
  return a ?? null
}
