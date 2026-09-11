// rev 1 — a tabela de Ordens de Compra, compartilhada
//
// EXTRAÍDA DE `InserirOC.tsx` (rev 2) QUANDO "OCs Inseridas" GANHOU TELA
// PRÓPRIA
//
//	A mesma tabela (arquivo, data, selo de leitura, ações) aparecia em dois
//	lugares: a fila de "Inserir OC" e as listas de "Processadas"/"Rejeitadas"
//	do novo hub. Duas cópias da mesma linha são duas chances de uma delas
//	ficar para trás quando o selo ganhar uma cor nova (CORE-06).
//
//	`onLer` é opcional: passe-o só onde o botão "ler"/"ler de novo" faz
//	sentido (a fila). Nas listas de Processadas/Rejeitadas a ação já
//	aconteceu — a tabela ali é só para ver.
//
//	`onEnviar` (11/09/2026) é o mesmo desenho, para o botão de PCO: passe-o só
//	na lista de "Pendentes de envio" — cada linha ali já É elegível (a vista
//	só traz `status=lido AND pco_enviado_em is null`), então o botão aparece
//	em toda linha, sem precisar checar status de novo aqui.
import {
  emReais, emDataHora, type OrdemDeCompra,
} from './tipos'

const NOME_STATUS: Record<OrdemDeCompra['status'], string> = {
  inserido: 'na fila',
  lendo: 'lendo…',
  lido: 'lida',
  falhou: 'falhou',
}

const CLASSE_STATUS: Record<OrdemDeCompra['status'], string> = {
  inserido: 'pino-off',
  lendo: 'pino-warn',
  lido: 'pino-ok',
  falhou: 'pino-err',
}

export function TabelaDeOrdens({ ordens, onVer, onLer, lendoId, loteRodando, onEnviar, enviandoId }: {
  ordens: OrdemDeCompra[]
  onVer: (o: OrdemDeCompra) => void
  /** Ausente = tabela só de consulta, sem botão de ler. */
  onLer?: (id: string) => void
  lendoId?: string | null
  loteRodando?: boolean
  /** Ausente = sem botão de enviar (Enviados, Processadas, Rejeitadas). */
  onEnviar?: (id: string) => void
  enviandoId?: string | null
}) {
  return (
    <div className="tabela-rolo">
      <table className="tabela">
        <thead>
          <tr>
            <th>Arquivo</th>
            <th>Inserida em</th>
            <th>Leitura</th>
            <th className="acoes-col"></th>
          </tr>
        </thead>
        <tbody>
          {ordens.map(o => (
            <tr key={o.id}>
              <td>
                {o.nome_arquivo}
                {(o.numero || o.comprador_nome || o.total) && (
                  <div className="adm-dica">
                    {o.numero ? `nº ${o.numero}` : ''}
                    {o.numero && o.comprador_nome ? ' · ' : ''}
                    {o.comprador_nome ?? ''}
                    {o.total ? ` · ${emReais(o.total)}` : ''}
                  </div>
                )}
                {o.erro_leitura && <div className="adm-dica adm-dica-alerta">{o.erro_leitura}</div>}
              </td>
              <td>{emDataHora(o.criado_em)}</td>
              <td><span className={'pino ' + CLASSE_STATUS[o.status]}>{NOME_STATUS[o.status]}</span></td>
              <td className="acoes">
                {onLer && (o.status === 'inserido' || o.status === 'falhou' || o.status === 'lendo') && (
                  <button
                    type="button"
                    className="bt bt-mini"
                    disabled={lendoId === o.id || loteRodando}
                    onClick={() => onLer(o.id)}
                  >
                    {lendoId === o.id
                      ? 'lendo…'
                      : o.status === 'falhou'
                        ? 'ler de novo'
                        : o.status === 'lendo'
                          ? 'retomar'
                          : 'ler'}
                  </button>
                )}
                {onEnviar && (
                  <button
                    type="button"
                    className="bt bt-mini"
                    disabled={enviandoId === o.id || enviandoId === '*'}
                    onClick={() => onEnviar(o.id)}
                  >
                    {enviandoId === o.id ? 'enviando…' : 'enviar'}
                  </button>
                )}
                <button type="button" className="bt bt-mini" onClick={() => onVer(o)}>ver</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
