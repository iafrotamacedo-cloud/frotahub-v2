// rev 2 — a marca do FrotaHub
//
// A figura é a marca do logo original da Frota Macedo, recortada do arquivo em alta
// resolução com o fundo transparente — os degradês e o relevo são os mesmos do logo
// da empresa, não uma imitação. O nome ao lado repete o tratamento do original (a
// primeira palavra pesada, a segunda leve) e a assinatura da NorthCore vai embaixo.
export function Marca({ assinatura = false }: { assinatura?: boolean }) {
  return (
    <div className="marca">
      <img src="/marca.png" alt="" aria-hidden="true" />
      <div>
        <div className="nome"><b>Frota</b><span>Hub</span></div>
        {assinatura && (
          <span className="assin"><i>by</i> <b>NorthCore</b></span>
        )}
      </div>
    </div>
  )
}
