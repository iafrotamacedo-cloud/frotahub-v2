// rev 1 — o padrão de ver documento
//
// A REGRA DA CASA, DITA EM 26/08/2026
//
//	"Toda vez que houver visualização de documento, tem que ser nesse padrão."
//	O padrão é o da nota em correção: o documento ocupa a tela, a casca do
//	programa sai do caminho, e sobra uma barra com o voltar e o que se faz ali.
//
//	Antes, ver um orçamento era BAIXAR o orçamento. Cada olhada deixava um
//	arquivo na pasta de Downloads, e conferir cinco orçamentos sujava a pasta com
//	cinco cópias que ninguém queria guardar.
//
// O DOWNLOAD CONTINUA, MAS COMO ESCOLHA
//
//	Dentro da visualização há "salvar como", e ele SEMPRE pergunta a pasta —
//	estes documentos são arquivados por loja e por mês, e a pasta padrão do
//	navegador nunca é o lugar certo.
//
//	O arquivo salvo é o MESMO que está na tela: ele foi buscado uma vez e fica
//	na mão. Baixar de novo do servidor abriria a porta para a tela mostrar uma
//	versão e o disco receber outra.
import { useEffect, useState, type ReactNode } from 'react'
import { arquivoDoMotor, salvarArquivo } from '../motor/cliente'
import { usePedirFoco } from './Foco'

export function VisorDeDocumento({
  titulo, caminho, endereco, nomeSugerido, voltar, acoes,
}: {
  /** O que a barra diz que é este documento. */
  titulo: ReactNode
  /** A rota do motor que devolve o arquivo — vai com o cabeçalho de sessão. */
  caminho?: string
  /**
   * Um endereço que JÁ vale por si — o link temporário do armazém, por exemplo.
   *
   * São dois caminhos porque são duas naturezas: o que o motor MONTA na hora
   * (orçamento, pedido, extração) precisa do token; o que já está GUARDADO no
   * armazém vem com a assinatura na própria URL. Um só dos dois é preenchido.
   */
  endereco?: string
  /** Nome oferecido no "salvar como". Sem ele, vale o que o motor sugeriu. */
  nomeSugerido?: string
  voltar: () => void
  /** Botões próprios da tela, antes do salvar. */
  acoes?: ReactNode
}) {
  usePedirFoco()

  const [arq, setArq] = useState<{ url: string; blob: Blob | null; nome: string } | null>(null)
  const [erro, setErro] = useState('')

  useEffect(() => {
    // O endereço pronto não passa por busca nenhuma: ele já é exibível, e
    // baixá-lo aqui só para mostrar seria pedir o arquivo duas vezes.
    if (endereco) {
      setArq({ url: endereco, blob: null, nome: nomeSugerido ?? 'documento' })
      return
    }
    if (!caminho) return

    let vivo = true
    let local = ''
    void (async () => {
      try {
        const a = await arquivoDoMotor(caminho)
        local = a.url
        if (!vivo) {
          URL.revokeObjectURL(a.url)
          return
        }
        setArq(a)
      } catch (e) {
        if (vivo) setErro(e instanceof Error ? e.message : 'Não consegui abrir o documento.')
      }
    })()
    // Sem isto cada documento aberto fica preso na memória da aba até ela
    // fechar. Quem confere vinte numa tarde carrega vinte.
    return () => {
      vivo = false
      if (local) URL.revokeObjectURL(local)
    }
  }, [caminho, endereco, nomeSugerido])

  // SALVAR NÃO PEDE O ARQUIVO DE NOVO — quando ele já está na mão.
  //   O que veio do motor já é blob. O que veio do armazém é um endereço, e aí
  //   sim é preciso buscar; se o armazém recusar a busca de outra origem, o
  //   botão diz isso em vez de não fazer nada.
  async function salvar() {
    if (!arq) return
    try {
      const blob = arq.blob ?? await (await fetch(arq.url)).blob()
      await salvarArquivo(blob, nomeSugerido ?? arq.nome)
    } catch {
      setErro('Não consegui salvar este arquivo. Ele continua à vista aqui.')
    }
  }

  return (
    <div className="orc-tela">
      <div className="orc-barra">
        <button type="button" className="orc-voltar" onClick={voltar}>← voltar</button>
        <b>{titulo}</b>
        <span className="orc-ficha-direita">
          {acoes}
          <button
            type="button"
            className="orc-bt"
            disabled={!arq}
            onClick={() => void salvar()}
          >
            salvar como…
          </button>
        </span>
      </div>

      {erro && <p className="erro">{erro}</p>}

      <div className="orc-visor-doc">
        {!arq
          ? <p className="orc-vazio">abrindo o documento…</p>
          : ehImagem(arq.url, nomeSugerido)
            ? <img src={arq.url} alt="documento" />
            // `toolbar=0` e `navpanes=0` tiram a régua e a coluna de miniaturas
            // do leitor do navegador: numa folha só elas comem largura, e a
            // largura aqui é onde os valores estão.
            : <iframe src={arq.url + '#toolbar=0&navpanes=0&view=FitH'} title="documento" />}
      </div>
    </div>
  )
}

// ehImagem decide pelo nome, e não pelo endereço: o link do armazém vem com
// assinatura na query e termina em qualquer coisa.
function ehImagem(url: string, nome?: string): boolean {
  const alvo = (nome ?? url).toLowerCase()
  return /\.(jpe?g|png|webp|gif|heic|bmp)(\?|$)/.test(alvo)
}
