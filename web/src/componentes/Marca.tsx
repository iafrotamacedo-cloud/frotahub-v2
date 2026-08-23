// rev 1 — a marca do FrotaHub
//
// A figura vem do logo da Frota Macedo, redesenhada em vetor para ficar nítida em
// qualquer tamanho. O nome ao lado repete o tratamento do logo original — a primeira
// palavra pesada, a segunda leve — e a assinatura da NorthCore vai embaixo.
export function Marca({ assinatura = false }: { assinatura?: boolean }) {
  return (
    <div className="marca">
      <img src="/marca.svg" alt="" aria-hidden="true" />
      <div>
        <div className="nome"><b>Frota</b><span>Hub</span></div>
        {assinatura && (
          <span className="assin"><i>by</i> <b>NorthCore</b></span>
        )}
      </div>
    </div>
  )
}
