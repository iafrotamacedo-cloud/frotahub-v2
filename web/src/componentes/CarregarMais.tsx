// rev 1 — o Padrão 3 do mobile: paginação vira "Carregar mais" (14/09/2026)
//
// HOJE EXISTEM 3 JEITOS DIFERENTES DE PAGINAR (Usuários/Funcionários,
// `Paginacao` de Arquivos.tsx, e a reimplementação solta de DadosTrilogo) —
// esta peça não tenta unificar os três no PC, só dá a cada tela um jeito
// comum de mostrar "tem mais" no mobile, qualquer que seja a forma de
// paginação por trás.
interface Props {
  temMais: boolean
  carregando?: boolean
  onClick: () => void
}

export function CarregarMais({ temMais, carregando, onClick }: Props) {
  if (!temMais) return null
  return (
    <button type="button" className="bt bt-neutro cm-carregar-mais" onClick={onClick} disabled={carregando}>
      {carregando ? 'Carregando...' : 'Carregar mais'}
    </button>
  )
}
