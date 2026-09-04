// rev 2 — a ficha de um ticket de Serviço
//
// REAPROVEITA A FICHA DO TRÍLOGO POR COMPOSIÇÃO, NÃO RECONSTRUÇÃO
//
//	FichaChamado (trilogo/) já desenha o cabeçalho, a descrição, as fotos, os
//	custos e a linha do tempo — duplicar isso aqui seria manter dois layouts
//	da mesma coisa. O que muda por fila é só o painel de ação (acaoExtra,
//	prop nova em FichaChamado): inserir orçamento, lançar no Trílogo,
//	preencher PCO, anexar nota fiscal — ou nada, nas filas que avançam
//	sozinhas (Execução, Faturado).
import { useState } from 'react'
import { FichaChamado } from '../trilogo/FichaChamado'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { motor, enviarFormulario, ErroMotor, avisoDe } from '../../motor/cliente'
import type { Perfil } from '../../sessao/tipos'
import type { ItemLista, Status } from './tipos'
import { FormularioDeLancamento } from './FormularioDeLancamento'

export type Acao = 'inserir-orcamento' | 'lancar' | 'aprovar-rejeitar' | 'preencher-pco' | 'anexar-nf' | 'nenhuma'

interface Props {
  item: ItemLista
  perfil: Perfil
  acao: Acao
  voltar: () => void
  aoMudar: (recado: string) => void
}

export function FichaDoTicket({ item, perfil, acao, voltar, aoMudar }: Props) {
  const [vendo, setVendo] = useState<{ endereco: string; nome: string } | null>(null)

  function aoFeito(recado: string) {
    aoMudar(recado)
    voltar()
  }

  // O PDF ANEXADO ABRE NA TELA, NÃO EM DOWNLOAD
  //
  //	Mesmo padrão de orcamentos/FichaDoOrcamento.tsx: o orçamento ocupa a
  //	página inteira e o voltar devolve à ficha, não à lista — quem abriu
  //	estava conferindo o ticket.
  async function verOrcamento() {
    const r = await motor<{ url: string }>(`/servicos/kanban/${item.id}/arquivo?tipo=orcamento`)
    setVendo({
      endereco: r.url,
      nome: item.orcamento_arquivo_nome ?? `orcamento-${item.ticket}.pdf`,
    })
  }

  if (vendo) {
    return (
      <VisorDeDocumento
        endereco={vendo.endereco}
        nomeSugerido={vendo.nome}
        titulo={`Orçamento · ticket ${item.ticket}`}
        voltar={() => setVendo(null)}
      />
    )
  }

  return (
    <FichaChamado
      numero={String(item.ticket)}
      perfil={perfil}
      voltar={voltar}
      acaoExtra={<PainelDeAcao item={item} acao={acao} aoFeito={aoFeito} aoVerOrcamento={verOrcamento} />}
      permitirMarcarServico={false}
    />
  )
}

function PainelDeAcao({ item, acao, aoFeito, aoVerOrcamento }: {
  item: ItemLista
  acao: Acao
  aoFeito: (recado: string) => void
  aoVerOrcamento: () => Promise<void>
}) {
  switch (acao) {
    case 'inserir-orcamento':
      return <AnexarArquivo titulo="Inserir orçamento" campo="arquivo"
        caminho={`/servicos/kanban/${item.id}/arquivo-orcamento`}
        aoFeito={() => aoFeito('Orçamento anexado — o card avançou para "Feitos".')} />
    case 'lancar':
      return <Lancar item={item} aoFeito={aoFeito} aoVerOrcamento={aoVerOrcamento} />
    case 'aprovar-rejeitar':
      return <AprovarOuRejeitar item={item} aoFeito={aoFeito} />
    case 'preencher-pco':
      return <PreencherPCO item={item} aoFeito={aoFeito} />
    case 'anexar-nf':
      return <AnexarNotaFiscal item={item} aoFeito={aoFeito} />
    default:
      return (
        <p className="dica">
          Este serviço avança sozinho quando o Trílogo confirmar a próxima etapa —
          não há ação manual aqui.
        </p>
      )
  }
}

// ---------------------------------------------------------------------------

function AnexarArquivo({ titulo, campo, caminho, extraCampo, aoFeito }: {
  titulo: string
  campo: string
  caminho: string
  extraCampo?: { nome: string; rotulo: string }
  aoFeito: () => void
}) {
  const [arquivo, setArquivo] = useState<File | null>(null)
  const [extra, setExtra] = useState('')
  const [erro, setErro] = useState<string | null>(null)
  const [enviando, setEnviando] = useState(false)

  async function enviar() {
    if (!arquivo) return
    setEnviando(true)
    setErro(null)
    try {
      const forma = new FormData()
      forma.append(campo, arquivo, arquivo.name)
      if (extraCampo && extra.trim()) forma.append(extraCampo.nome, extra.trim())
      await enviarFormulario(caminho, forma)
      aoFeito()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui enviar o arquivo.')
      setEnviando(false)
    }
  }

  return (
    <div className="sv-form">
      <span className="sv-itens-titulo">{titulo}</span>
      {extraCampo && (
        <>
          <label htmlFor="af-extra">{extraCampo.rotulo}</label>
          <input id="af-extra" value={extra} onChange={e => setExtra(e.target.value)} disabled={enviando} />
        </>
      )}
      <input
        type="file" accept="application/pdf,image/*" disabled={enviando}
        onChange={e => setArquivo(e.target.files?.[0] ?? null)}
      />
      {erro && <div className="erro-caixa">{erro}</div>}
      <div className="jn-pe" style={{ marginTop: 12 }}>
        <button type="button" className="bt bt-forte" disabled={enviando || !arquivo} onClick={() => void enviar()}>
          {enviando ? 'Enviando...' : 'Enviar'}
        </button>
      </div>
    </div>
  )
}

function Lancar({ item, aoFeito, aoVerOrcamento }: {
  item: ItemLista
  aoFeito: (recado: string) => void
  aoVerOrcamento: () => Promise<void>
}) {
  const [abriu, setAbriu] = useState(false)
  const [abrindo, setAbrindo] = useState(false)
  const [erro, setErro] = useState<string | null>(null)

  async function ver() {
    setAbrindo(true)
    setErro(null)
    try {
      await aoVerOrcamento()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o orçamento.')
    } finally {
      setAbrindo(false)
    }
  }

  const verOrcamento = (
    <button type="button" className="bt bt-neutro" disabled={abrindo} onClick={() => void ver()}>
      {abrindo ? 'Abrindo...' : 'Visualizar orçamento'}
    </button>
  )

  if (!abriu) {
    return (
      <div className="sv-form">
        <p className="dica">Orçamento anexado — falta lançar a cotação e o orçamento no Trílogo.</p>
        {erro && <div className="erro-caixa">{erro}</div>}
        <div className="jn-pe" style={{ justifyContent: 'flex-start', marginTop: 12 }}>
          {verOrcamento}
          <button type="button" className="bt bt-forte" onClick={() => setAbriu(true)}>Lançar no Trílogo</button>
        </div>
      </div>
    )
  }
  return (
    <>
      {erro && <div className="erro-caixa">{erro}</div>}
      <div className="jn-pe" style={{ justifyContent: 'flex-start', marginTop: 0, marginBottom: 12 }}>
        {verOrcamento}
      </div>
      <FormularioDeLancamento
        itemID={item.id}
        aoFeito={aoFeito}
        aoCancelar={() => setAbriu(false)}
      />
    </>
  )
}

function AprovarOuRejeitar({ item, aoFeito }: { item: ItemLista; aoFeito: (recado: string) => void }) {
  const [erro, setErro] = useState<string | null>(null)
  const [agindo, setAgindo] = useState<Status | null>(null)

  async function mudar(novoStatus: Status, recado: string) {
    setAgindo(novoStatus)
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/status`, { metodo: 'POST', corpo: { status: novoStatus } })
      aoFeito(avisoDe(resposta) ?? recado)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui mudar o status.')
      setAgindo(null)
    }
  }

  return (
    <div className="sv-form">
      <p className="dica">
        Lançado no Trílogo — aguardando o cliente decidir. A detecção automática de
        aprovação ainda não existe; marque manualmente quando souber o resultado.
      </p>
      {erro && <div className="erro-caixa">{erro}</div>}
      <div className="jn-pe" style={{ justifyContent: 'flex-start', marginTop: 8 }}>
        <button type="button" className="bt bt-mini" disabled={!!agindo}
          onClick={() => void mudar('orcamento_aprovado', 'Orçamento aprovado.')}>
          {agindo === 'orcamento_aprovado' ? 'Marcando...' : 'Aprovado'}
        </button>
        <button type="button" className="bt bt-mini bt-perigo" disabled={!!agindo}
          onClick={() => void mudar('orcamento_rejeitado', 'Orçamento rejeitado.')}>
          {agindo === 'orcamento_rejeitado' ? 'Marcando...' : 'Rejeitado'}
        </button>
      </div>
    </div>
  )
}

function PreencherPCO({ item, aoFeito }: { item: ItemLista; aoFeito: (recado: string) => void }) {
  const [pco, setPco] = useState(item.pco_numero ?? '')
  const [erro, setErro] = useState<string | null>(null)
  const [enviando, setEnviando] = useState(false)

  async function enviar() {
    if (!pco.trim()) return
    setEnviando(true)
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/pco`, { metodo: 'POST', corpo: { pco: pco.trim() } })
      aoFeito(avisoDe(resposta) ?? 'PCO preenchido — o card foi para "A faturar".')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui preencher o PCO.')
      setEnviando(false)
    }
  }

  return (
    <div className="sv-form">
      <label htmlFor="pco-numero">Número do PCO</label>
      <input id="pco-numero" value={pco} onChange={e => setPco(e.target.value)} disabled={enviando} />
      {erro && <div className="erro-caixa">{erro}</div>}
      <div className="jn-pe" style={{ marginTop: 12 }}>
        <button type="button" className="bt bt-forte" disabled={enviando || !pco.trim()} onClick={() => void enviar()}>
          {enviando ? 'Salvando...' : 'Salvar PCO'}
        </button>
      </div>
    </div>
  )
}

function AnexarNotaFiscal({ item, aoFeito }: { item: ItemLista; aoFeito: (recado: string) => void }) {
  return (
    <AnexarArquivo
      titulo={`Anexar nota fiscal — PCO ${item.pco_numero ?? '—'}`}
      campo="arquivo"
      extraCampo={{ nome: 'numero', rotulo: 'Número da nota fiscal (opcional)' }}
      caminho={`/servicos/kanban/${item.id}/nota-fiscal`}
      aoFeito={() => aoFeito('Nota fiscal anexada — faturado.')}
    />
  )
}
