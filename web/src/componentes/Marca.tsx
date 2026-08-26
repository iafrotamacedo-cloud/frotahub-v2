// rev 3 — a marca do FrotaHub
//
// A figura é a marca do logo original da Frota Macedo, recortada do arquivo em alta
// resolução com o fundo transparente — os degradês e o relevo são os mesmos do logo
// da empresa, não uma imitação. O nome ao lado repete o tratamento do original: a
// primeira palavra pesada, a segunda leve.
//
// A ASSINATURA DA NORTHCORE SAIU EM 26/08/2026
//
//	Decisão do dono. A propriedade `assinatura` saiu junto, e não ficou como
//	`assinatura={false}` em lugar nenhum: propriedade que ninguém mais usa é
//	convite para alguém religá-la sem saber que foi desligada de propósito. O
//	CSS dela também foi removido, pelo mesmo motivo — regra órfã em folha
//	compartilhada é peso morto que sobrevive a todas as limpezas seguintes.
export function Marca() {
  return (
    <div className="marca">
      <img src="/marca.png" alt="" aria-hidden="true" />
      <div>
        <div className="nome"><b>Frota</b><span>Hub</span></div>
      </div>
    </div>
  )
}
