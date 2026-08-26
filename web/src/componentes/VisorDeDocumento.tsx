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
import { arquivoDoMotor, salvarArquivo, type ArquivoDoMotor } from '../motor/cliente'
import { usePedirFoco } from './Foco'

export function VisorDeDocumento({
  titulo, caminho, nomeSugerido, voltar, acoes,
}: {
  /** O que a barra diz que é este documento. */
  titulo: ReactNode
  /** A rota do motor que devolve o arquivo. */
  caminho: string
  /** Nome oferecido no "salvar como". Sem ele, vale o que o motor sugeriu. */
  nomeSugerido?: string
  voltar: () => void
  /** Botões próprios da tela, antes do salvar. */
  acoes?: ReactNode
}) {
  usePedirFoco()

  const [arq, setArq] = useState<ArquivoDoMotor | null>(null)
  const [erro, setErro] = useState('')

  useEffect(() => {
    let vivo = true
    let endereco = ''
    void (async () => {
      try {
        const a = await arquivoDoMotor(caminho)
        endereco = a.url
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
      if (endereco) URL.revokeObjectURL(endereco)
    }
  }, [caminho])

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
            onClick={() => { if (arq) void salvarArquivo(arq.blob, nomeSugerido ?? arq.nome) }}
          >
            salvar como…
          </button>
        </span>
      </div>

      {erro && <p className="erro">{erro}</p>}

      <div className="orc-visor-doc">
        {!arq
          ? <p className="orc-vazio">abrindo o documento…</p>
          // `toolbar=0` e `navpanes=0` tiram a régua e a coluna de miniaturas do
          // leitor do navegador: numa folha só elas comem largura, e a largura
          // aqui é onde os valores estão.
          : <iframe src={arq.url + '#toolbar=0&navpanes=0&view=FitH'} title="documento" />}
      </div>
    </div>
  )
}
