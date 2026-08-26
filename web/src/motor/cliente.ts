// rev 3 — a conversa do front com o baleryan
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
  /**
   * O corpo inteiro da resposta que falhou.
   *
   * POR QUE ELE VIAJA JUNTO
   *   Alguns erros do motor não são só uma frase: o bloqueio do teto vem com o
   *   quanto o ticket já tem, o quanto é este orçamento e as saídas possíveis.
   *   Se o cliente ficasse só com a mensagem, a tela mostraria "passou do teto"
   *   e jogaria fora justamente a parte que resolve.
   */
  constructor(public status: number, mensagem: string, public corpo?: unknown) {
    super(mensagem)
    this.name = 'ErroMotor'
  }
}

interface Opcoes {
  metodo?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
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
    throw new ErroMotor(resposta.status, msg || 'Alguma coisa deu errado do nosso lado. Tente de novo.', corpo)
  }

  return corpo as T
}

/** O aviso que algumas respostas trazem quando a ação deu certo mas o histórico não. */
export function avisoDe(resposta: unknown): string | null {
  const a = (resposta as { aviso?: string } | null)?.aviso
  return a ?? null
}

/**
 * Baixa um arquivo gerado pelo motor.
 *
 * POR QUE NÃO É UM LINK DIRETO
 *   Um `<a href="...">` não carrega o cabeçalho de autorização, e a saída fácil
 *   seria pendurar o token no endereço. Token em endereço vai parar no histórico
 *   do navegador, no log do servidor e no primeiro link que alguém colar numa
 *   conversa (CORE-09).
 *
 *   Então o arquivo é buscado como qualquer outra chamada, com o cabeçalho, e
 *   entregue ao navegador como um endereço temporário que vive na memória e
 *   morre em seguida.
 */
/**
 * Pede um arquivo ao motor, com o cabeçalho de sessão.
 *
 * Existe separada porque BAIXAR e MOSTRAR compartilham tudo até aqui — a
 * autenticação, o tratamento de erro, a frase que o motor devolve. Duas cópias
 * disso um dia discordariam sobre o que é um erro.
 */
async function pedirArquivo(caminho: string): Promise<Response> {
  if (!BASE) {
    throw new ErroMotor(0, 'O endereço do motor não foi configurado nesta publicação (VITE_MOTOR_URL).')
  }
  const { data } = await supabase.auth.getSession()
  const token = data.session?.access_token
  if (!token) throw new ErroMotor(401, 'A sua sessão expirou. Entre de novo.')

  let resposta: Response
  try {
    resposta = await fetch(BASE + caminho, { headers: { Authorization: `Bearer ${token}` } })
  } catch {
    throw new ErroMotor(0, 'Não consegui falar com o servidor. Confira a conexão e tente de novo.')
  }

  if (!resposta.ok) {
    // Quando dá errado, o motor responde JSON — e a mensagem dele é escrita
    // para ser lida (P-18).
    let msg = 'Não consegui gerar o arquivo.'
    try { msg = (JSON.parse(await resposta.text()) as { erro?: string }).erro || msg } catch { /* corpo não era JSON */ }
    throw new ErroMotor(resposta.status, msg)
  }
  return resposta
}

export async function baixarDoMotor(caminho: string): Promise<void> {
  const resposta = await pedirArquivo(caminho)

  const nome = nomeDoCabecalho(resposta.headers.get('Content-Disposition')) ?? 'extracao'
  const blob = await resposta.blob()
  const endereco = URL.createObjectURL(blob)
  try {
    const a = document.createElement('a')
    a.href = endereco
    a.download = nome
    document.body.appendChild(a)
    a.click()
    a.remove()
  } finally {
    // Sem isto o arquivo fica preso na memória da aba até ela fechar.
    setTimeout(() => URL.revokeObjectURL(endereco), 1000)
  }
}

function nomeDoCabecalho(cabecalho: string | null): string | null {
  if (!cabecalho) return null
  const utf8 = /filename\*=UTF-8''([^;]+)/i.exec(cabecalho)
  if (utf8) { try { return decodeURIComponent(utf8[1]) } catch { /* segue para o simples */ } }
  const simples = /filename="?([^";]+)"?/i.exec(cabecalho)
  return simples ? simples[1] : null
}

/**
 * Envia arquivos para o motor.
 *
 * POR QUE NÃO PASSA PELO `motor()`
 *   Porque ali o corpo vira JSON e o cabeçalho diz `application/json`. Um
 *   arquivo dentro de JSON precisaria virar base64 — 33% mais bytes subindo, e
 *   o motor teria que decodificar na memória antes de saber o tamanho.
 *
 *   Com `FormData` o navegador monta o multipart e escolhe o próprio limite de
 *   fronteira. Definir `Content-Type` à mão aqui QUEBRA o envio, porque o
 *   limite não entraria junto — é o erro clássico, e por isso o cabeçalho está
 *   ausente de propósito.
 */
export async function enviarArquivos<T>(caminho: string, arquivos: File[]): Promise<T> {
  if (!BASE) {
    throw new ErroMotor(0, 'O endereço do motor não foi configurado nesta publicação (VITE_MOTOR_URL).')
  }
  const { data } = await supabase.auth.getSession()
  const token = data.session?.access_token
  if (!token) throw new ErroMotor(401, 'A sua sessão expirou. Entre de novo.')

  const forma = new FormData()
  for (const a of arquivos) forma.append('arquivos', a, a.name)

  let resposta: Response
  try {
    resposta = await fetch(BASE + caminho, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: forma,
    })
  } catch {
    throw new ErroMotor(0, 'Não consegui enviar os arquivos. Confira a conexão e tente de novo.')
  }

  const bruto = await resposta.text()
  let corpo: unknown = null
  if (bruto) { try { corpo = JSON.parse(bruto) } catch { corpo = null } }

  if (!resposta.ok) {
    const msg = (corpo as { erro?: string } | null)?.erro
    throw new ErroMotor(resposta.status, msg || 'Não consegui enviar os arquivos.', corpo)
  }
  return corpo as T
}

/**
 * Traz um arquivo do motor como endereço de objeto, para MOSTRAR na tela.
 *
 * POR QUE NÃO DÁ PARA APONTAR UM `iframe` DIRETO PARA O MOTOR
 *   A rota exige o cabeçalho `Authorization`, e um `iframe` não manda cabeçalho
 *   nenhum — ele só pede a URL. Pôr o token na query resolveria e criaria um
 *   problema pior: o token ficaria no histórico do navegador e em qualquer log
 *   de acesso pelo caminho.
 *
 *   Então o arquivo vem por `fetch`, com o cabeçalho, e vira um endereço local
 *   que só existe dentro desta aba.
 *
 * QUEM CHAMA PRECISA DEVOLVER
 *   O endereço prende o arquivo na memória da aba até alguém o revogar. Em tela
 *   que troca de registro isso vaza um PDF por visita — por isso a devolução é
 *   do chamador, no `useEffect`, e não daqui.
 */
export async function arquivoDoMotor(caminho: string): Promise<string> {
  const resposta = await pedirArquivo(caminho)
  return URL.createObjectURL(await resposta.blob())
}
