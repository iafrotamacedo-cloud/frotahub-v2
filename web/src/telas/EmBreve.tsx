// rev 1 — a tela de rotina ainda não construída
//
// Existe para que o menu possa mostrar o sistema inteiro desde o começo, sem
// prometer o que ainda não funciona.
import { Relogio } from '../componentes/Icone'

export function EmBreve({ titulo }: { titulo: string }) {
  return (
    <div className="breve-caixa">
      <div className="ic"><Relogio /></div>
      <h2>{titulo}</h2>
      <p>Esta parte ainda não foi construída. Ela já aparece aqui para dar a medida do
        que vem pela frente — quando estiver pronta, abre normalmente por este mesmo caminho.</p>
    </div>
  )
}
