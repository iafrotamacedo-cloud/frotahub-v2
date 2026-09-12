// rev 1 — excluir e substituir, compartilhados entre InserirOC e ListaDeOrdens
//
// POR QUE UM HOOK SÓ, USADO NAS DUAS TELAS
//
//	"Inserir OC" (fila/processadas/rejeitadas, por dentro do próprio seletor
//	de vista) e "OCs Inseridas" (a lista dedicada de Processadas/Rejeitadas)
//	são DUAS portas para o mesmo dado. Excluir e substituir têm a mesma
//	regra dos dois lados — duas cópias da lógica de exclusão divergiriam
//	sem ninguém perceber (CORE-06), então mora aqui uma vez só.
import { useRef, useState, type ChangeEvent } from 'react'
import { motor, enviarFormulario, ErroMotor } from '../../motor/cliente'
import type { OrdemDeCompra } from './tipos'

type AtualizarOrdens = (fn: (atual: OrdemDeCompra[] | null) => OrdemDeCompra[] | null) => void

export function useAcoesDaOrdem(setOrdens: AtualizarOrdens, setErro: (s: string) => void) {
  const [exclusao, setExclusao] = useState<OrdemDeCompra | null>(null)
  const [substituindo, setSubstituindo] = useState<OrdemDeCompra | null>(null)
  const [arquivoEscolhido, setArquivoEscolhido] = useState<File | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // EXCLUIR — a LINHA SOME NA HORA, A CHAMADA DE VERDADE ESPERA
  //
  //	O aviso de desfazer (`AvisoDesfazer.tsx`) é quem decide se a exclusão
  //	realmente acontece — "sim" nunca chega a chamar o motor.
  function pedirExclusao(o: OrdemDeCompra) {
    setOrdens(atual => atual ? atual.filter(x => x.id !== o.id) : atual)
    setExclusao(o)
  }

  function desfazerExclusao() {
    if (!exclusao) return
    const o = exclusao
    setExclusao(null)
    setOrdens(atual => (atual ? [o, ...atual] : atual))
  }

  async function confirmarExclusao() {
    if (!exclusao) return
    const o = exclusao
    setExclusao(null)
    try {
      await motor(`/administrativo/compras/ordens/${o.id}`, { metodo: 'DELETE' })
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui excluir esta ordem de compra.')
      setOrdens(atual => (atual ? [o, ...atual] : atual))
    }
  }

  // SUBSTITUIR — ESCOLHE O ARQUIVO, CONFIRMA, SÓ ENTÃO TROCA
  function pedirSubstituicao(o: OrdemDeCompra) {
    setSubstituindo(o)
    window.setTimeout(() => inputRef.current?.click(), 0)
  }

  function arquivoSelecionado(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    e.target.value = ''
    if (f) setArquivoEscolhido(f)
  }

  function cancelarSubstituicao() {
    setSubstituindo(null)
    setArquivoEscolhido(null)
  }

  async function confirmarSubstituicao() {
    if (!substituindo || !arquivoEscolhido) return
    const o = substituindo
    const arquivo = arquivoEscolhido
    setSubstituindo(null)
    setArquivoEscolhido(null)
    try {
      const forma = new FormData()
      forma.append('arquivo', arquivo, arquivo.name)
      await enviarFormulario(`/administrativo/compras/ordens/${o.id}/substituir`, forma)
      setOrdens(atual => (atual ? atual.filter(x => x.id !== o.id) : atual))
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui substituir esta ordem de compra.')
    }
  }

  return {
    exclusao, pedirExclusao, desfazerExclusao, confirmarExclusao,
    substituindo, arquivoEscolhido, inputRef,
    pedirSubstituicao, arquivoSelecionado, confirmarSubstituicao, cancelarSubstituicao,
  }
}
