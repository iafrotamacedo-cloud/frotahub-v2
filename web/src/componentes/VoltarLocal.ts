// rev 1 — o gesto nativo de voltar fecha a tela local, não pula ela (17/09/2026)
//
// O PROBLEMA (achado na obra piloto MSL Fátima, testando no iPhone)
//
//	`VisorDeDocumento` e outras telas de foco (`Foco.tsx`) abrem por ESTADO
//	local — `setVendo(null)`/`voltar()` — sem nunca mudar o endereço
//	(`#/...`). É proposital (`navegacao.ts`): empilhar uma entrada de
//	histórico pra cada "ver" que a pessoa abre encheria o botão voltar do
//	navegador de telas inúteis. O "← voltar" DE DENTRO da tela cobre isso
//	bem — mas o GESTO do sistema (arrastar da borda esquerda, no iPhone; o
//	botão físico voltar, no Android) não passa por aquele botão: ele mexe
//	direto no histórico do navegador, que nunca soube que essa tela abriu.
//	O gesto pula ela inteira e sai pra fora de onde a pessoa estava — no
//	teste, caiu direto no menu em vez de fechar só o documento.
//
// A SOLUÇÃO — UMA ENTRADA "MUDA" NO HISTÓRICO
//
//	Ao abrir, empilha uma entrada de histórico que aponta pro MESMO
//	endereço — a URL visível não muda nada. O gesto de voltar consome essa
//	entrada primeiro: dispara `popstate`, que a gente escuta e usa pra
//	fechar a tela, exatamente como clicar no "← voltar" de dentro dela. Só
//	DEPOIS disso o próximo gesto volta a valer pra navegação de verdade.
//
//	Fechar por outro caminho (o próprio botão, "salvar", um erro) desfaz a
//	entrada muda sozinho — senão "voltar" duas vezes seguidas seria preciso
//	pra sair de verdade. A ÚNICA exceção é fechar porque a pessoa navegou
//	pra OUTRO lugar enquanto a tela estava aberta (dá pra clicar no menu
//	lateral no computador mesmo com um documento em foco) — aí o endereço já
//	mudou, e desfazer a entrada muda desfaria a navegação que ela acabou de
//	fazer. Confere o endereço antes de desfazer, por isso.
import { useEffect, useRef } from 'react'

export function useVoltarLocal(fechar: () => void): void {
  const fecharRef = useRef(fechar)
  fecharRef.current = fechar

  useEffect(() => {
    let consumida = false
    const enderecoAoAbrir = window.location.href
    window.history.pushState({ telaLocal: true }, '')

    function aoGestoDeVoltar() {
      consumida = true
      fecharRef.current()
    }
    window.addEventListener('popstate', aoGestoDeVoltar)

    return () => {
      window.removeEventListener('popstate', aoGestoDeVoltar)
      if (!consumida && window.location.href === enderecoAoAbrir) {
        window.history.back()
      }
    }
  }, [])
}
