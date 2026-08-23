// rev 1 — quando a sessão acaba
//
// Duas contas, decididas pelo dono do sistema:
//   • 3 horas PARADO  → pede senha de novo
//   • 24 horas no total → pede senha de novo, mesmo em uso contínuo
//
// LEIA ISTO ANTES DE CONFIAR NESTE ARQUIVO
//   Cronômetro de navegador é conveniência, não tranca. Ele fecha a tela, mas a
//   credencial guardada continuaria valendo se alguém a pegasse por fora. A trava
//   de verdade é a configuração de sessão do Supabase, no painel — esta peça
//   existe para que a pessoa seja levada à tela de login de forma limpa, em vez de
//   descobrir que expirou por um erro no meio de um clique.
//
// POR QUE OS CARIMBOS FICAM GUARDADOS NO NAVEGADOR
//   Um cronômetro só na memória morre junto com a aba. Quem fechasse o navegador
//   às 18h e voltasse às 8h da manhã encontraria a sessão aberta, porque nenhum
//   cronômetro chegou a disparar. Com os carimbos guardados, a conta é feita
//   também na hora de abrir.
import { useEffect } from 'react'

export const MINUTOS_PARADO = 180
export const HORAS_TOTAL = 24

const CHAVE_ULTIMO = 'frotahub.ultimo-uso'
const CHAVE_INICIO = 'frotahub.inicio-sessao'

// O armazenamento do navegador falha em janela anônima e com bloqueio de dados.
// Falhar aqui não pode derrubar o sistema: sem carimbo, vale o cronômetro em
// memória, que é o comportamento de antes.
function ler(chave: string): number {
  try {
    const v = window.localStorage.getItem(chave)
    return v ? Number(v) : 0
  } catch { return 0 }
}

function gravar(chave: string, valor: number) {
  try { window.localStorage.setItem(chave, String(valor)) } catch { /* segue sem carimbo */ }
}

export function marcarInicioDeSessao() {
  const agora = Date.now()
  gravar(CHAVE_INICIO, agora)
  gravar(CHAVE_ULTIMO, agora)
}

export function limparMarcasDeSessao() {
  try {
    window.localStorage.removeItem(CHAVE_INICIO)
    window.localStorage.removeItem(CHAVE_ULTIMO)
  } catch { /* nada a limpar */ }
}

/** Já passou do prazo? Responde na hora, sem esperar cronômetro. */
export function sessaoVencida(): boolean {
  const agora = Date.now()
  const ultimo = ler(CHAVE_ULTIMO)
  const inicio = ler(CHAVE_INICIO)
  if (ultimo && agora - ultimo > MINUTOS_PARADO * 60_000) return true
  if (inicio && agora - inicio > HORAS_TOTAL * 3_600_000) return true
  return false
}

const SINAIS = ['pointerdown', 'keydown', 'wheel', 'touchstart'] as const

/**
 * Observa o uso e chama `aoExpirar` quando um dos dois prazos vence.
 *
 * `ligado` existe para o relógio não correr na tela de login: lá não há sessão
 * para expirar.
 */
export function useExpiracao(ligado: boolean, aoExpirar: () => void) {
  useEffect(() => {
    if (!ligado) return

    let alarme = 0

    // Depois de encerrar, nada mais toca nos carimbos. Sem esta trava, o próprio
    // rearme gravava um "usei agora" logo após a saída, e a marca de sessão ficava
    // no navegador de quem já tinha sido deslogado. Apareceu no teste.
    let encerrado = false

    function conferir() {
      if (encerrado) return
      if (sessaoVencida()) {
        encerrado = true
        window.clearTimeout(alarme)
        aoExpirar()
      }
    }

    function rearmar() {
      if (encerrado) return
      gravar(CHAVE_ULTIMO, Date.now())
      window.clearTimeout(alarme)
      alarme = window.setTimeout(conferir, MINUTOS_PARADO * 60_000)
    }

    function aoVoltar() {
      // Voltar para a aba é o momento certo de conferir: enquanto ela estava
      // escondida, o cronômetro pode nem ter rodado.
      if (document.visibilityState === 'visible') conferir()
    }

    conferir()
    rearmar()
    for (const s of SINAIS) window.addEventListener(s, rearmar, { passive: true })
    document.addEventListener('visibilitychange', aoVoltar)

    return () => {
      window.clearTimeout(alarme)
      for (const s of SINAIS) window.removeEventListener(s, rearmar)
      document.removeEventListener('visibilitychange', aoVoltar)
    }
  }, [ligado, aoExpirar])
}
