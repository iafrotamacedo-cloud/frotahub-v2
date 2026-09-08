// rev 1 — a caixa de confirmação do sistema
//
// window.confirm() ABRE UM POPUP DO NAVEGADOR, NÃO DO FROTAHUB
//
//	"www.frotamacedo.com.br diz" com OK/Cancelar cinza — quebra a casca em
//	todo o resto do sistema (Janela.tsx, CORE-06). Este componente é a
//	mesma pergunta e o mesmo texto, só que dentro da Janela de sempre.
//
// SÍNCRONO VIROU ASSÍNCRONO
//
//	window.confirm() trava a função até o clique e devolve um booleano —
//	`if (!window.confirm(...)) return`. Uma Janela não bloqueia: quem
//	chama guarda a ação pendente em estado (useState) e só a executa
//	dentro de aoConfirmar, no lugar do que vinha depois do `if`.
import { Janela } from './Janela'

interface Props {
  titulo: string
  mensagem: string
  aoConfirmar: () => void
  aoFechar: () => void
  /** Ação destrutiva (rejeitar, excluir) — botão fica vermelho. */
  perigo?: boolean
  rotuloConfirmar?: string
}

export function Confirmar({ titulo, mensagem, aoConfirmar, aoFechar, perigo, rotuloConfirmar = 'Confirmar' }: Props) {
  return (
    <Janela titulo={titulo} aoFechar={aoFechar} largura={420}>
      <div className="jn-corpo">
        <p className="dica" style={{ whiteSpace: 'pre-line' }}>{mensagem}</p>
        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Cancelar</button>
          <button
            type="button" className={`bt ${perigo ? 'bt-perigo' : 'bt-forte'}`}
            onClick={() => { aoFechar(); aoConfirmar() }}
          >
            {rotuloConfirmar}
          </button>
        </div>
      </div>
    </Janela>
  )
}
