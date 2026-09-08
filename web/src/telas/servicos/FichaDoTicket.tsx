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
import { Confirmar } from '../../componentes/Confirmar'
import { VisorDeDocumento } from '../../componentes/VisorDeDocumento'
import { motor, enviarFormulario, ErroMotor, avisoDe } from '../../motor/cliente'
import { emReais } from '../orcamentos/tipos'
import type { Perfil } from '../../sessao/tipos'
import type { ItemLista } from './tipos'
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
  const [trocando, setTrocando] = useState(false)
  const [abrindo, setAbrindo] = useState(false)
  const [excluindo, setExcluindo] = useState(false)
  const [confirmandoExcluir, setConfirmandoExcluir] = useState(false)
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

  // EXCLUIR O PDF ≠ TROCAR O PDF
  //
  //	Trocar mantém o card em Feitos, só o arquivo muda. Excluir tira o
  //	rascunho inteiro e devolve o card pra Pendentes — mesma dupla
  //	apagarArquivoDeOrcamento/soltarRegistroDeArquivo de Reclassificar (ver
  //	documentos.go, ExcluirArquivoDeOrcamento), só que sem sair de Serviço.
  async function excluirArquivo() {
    setExcluindo(true)
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/arquivo-orcamento`, { metodo: 'DELETE' })
      aoFeito(avisoDe(resposta) ?? 'PDF excluído — voltou para "Pendentes".')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui excluir o arquivo.')
      setExcluindo(false)
    }
  }

  const verOrcamento = (
    <button type="button" className="bt bt-neutro" disabled={abrindo} onClick={() => void ver()}>
      {abrindo ? 'Abrindo...' : 'Visualizar orçamento'}
    </button>
  )

  // TROCAR O PDF É REANEXAR — o motor já aceita (InserirArquivoDeOrcamento
  // permite reescrever enquanto o card está em orcamento_feito). Reaproveita
  // o MESMO AnexarArquivo da fila "Pendentes", só muda o título.
  if (trocando) {
    return (
      <>
        <AnexarArquivo
          titulo="Trocar o PDF do orçamento" campo="arquivo"
          caminho={`/servicos/kanban/${item.id}/arquivo-orcamento`}
          aoFeito={() => aoFeito('PDF do orçamento trocado.')}
        />
        <button type="button" className="bt bt-mini bt-neutro" style={{ marginTop: 8 }}
          onClick={() => setTrocando(false)}>
          Cancelar
        </button>
      </>
    )
  }

  if (!abriu) {
    return (
      <div className="sv-form">
        <p className="dica">
          Orçamento anexado — falta lançar a cotação e o orçamento no Trílogo.
          {item.orcamento_arquivo_valor != null && (
            <span className="sv-valor-lido"> O PDF indica {emReais(item.orcamento_arquivo_valor)}.</span>
          )}
        </p>
        {erro && <div className="erro-caixa">{erro}</div>}
        <div className="jn-pe" style={{ justifyContent: 'flex-start', marginTop: 12 }}>
          {verOrcamento}
          <button type="button" className="bt bt-neutro" disabled={excluindo} onClick={() => setTrocando(true)}>Trocar PDF</button>
          <button type="button" className="bt bt-neutro" disabled={excluindo} onClick={() => setConfirmandoExcluir(true)}>
            {excluindo ? 'Excluindo...' : 'Excluir PDF'}
          </button>
          <button type="button" className="bt bt-forte" disabled={excluindo} onClick={() => setAbriu(true)}>Lançar no Trílogo</button>
        </div>
        {confirmandoExcluir && (
          <Confirmar
            titulo={`Excluir o PDF anexado do ticket ${item.ticket}?`}
            mensagem='O card volta para "Pendentes", sem orçamento nenhum.'
            perigo
            aoConfirmar={() => void excluirArquivo()}
            aoFechar={() => setConfirmandoExcluir(false)}
          />
        )}
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
        valorLido={item.orcamento_arquivo_valor}
        aoFeito={aoFeito}
        aoCancelar={() => setAbriu(false)}
      />
    </>
  )
}

function AprovarOuRejeitar({ item, aoFeito }: { item: ItemLista; aoFeito: (recado: string) => void }) {
  const [erro, setErro] = useState<string | null>(null)
  const [agindo, setAgindo] = useState<'aprovado' | 'rejeitado' | 'excluindo' | null>(null)
  const [confirmando, setConfirmando] = useState<'rejeitar' | 'retirar' | null>(null)

  async function aprovar() {
    setAgindo('aprovado')
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/status`, {
        metodo: 'POST', corpo: { status: 'aprovado_execucao' },
      })
      aoFeito(avisoDe(resposta) ?? 'Orçamento aprovado — foi para Execução.')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui aprovar.')
      setAgindo(null)
    }
  }

  async function rejeitar() {
    setAgindo('rejeitado')
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/rejeitar`, { metodo: 'POST' })
      aoFeito(avisoDe(resposta) ?? 'Orçamento rejeitado — voltou para o contrato.')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui rejeitar.')
      setAgindo(null)
    }
  }

  // RETIRAR COTAÇÃO ≠ REJEITAR
  //
  //	Rejeitar tira o ticket de Serviço inteiro, de volta pro contrato.
  //	Retirar cotação só desfaz O LANÇAMENTO — apaga a cotação/orçamento no
  //	Trílogo e devolve o card pra "Feitos", com o MESMO PDF ainda anexado,
  //	pronto pra lançar de novo com os itens certos (ver cotacoes.go,
  //	ExcluirOrcamento — mesma ação exposta também na lista de Lançados,
  //	ListaDeServicos.tsx).
  async function retirarCotacao() {
    setAgindo('excluindo')
    setErro(null)
    try {
      const resposta = await motor(`/servicos/kanban/${item.id}/orcamentos`, { metodo: 'DELETE' })
      aoFeito(avisoDe(resposta) ?? 'Cotação retirada — voltou para "Feitos".')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui retirar a cotação.')
      setAgindo(null)
    }
  }

  return (
    <div className="sv-form">
      <p className="dica">
        Lançado no Trílogo — aguardando o cliente. Aprovado vai para Execução.
        Rejeitado devolve o ticket ao contrato. Retirar cotação desfaz só o
        lançamento, pra corrigir e lançar de novo.
      </p>
      {erro && <div className="erro-caixa">{erro}</div>}
      <div className="jn-pe" style={{ justifyContent: 'flex-start', marginTop: 8 }}>
        <button type="button" className="bt bt-mini" disabled={!!agindo}
          onClick={() => void aprovar()}>
          {agindo === 'aprovado' ? 'Marcando...' : 'Aprovado'}
        </button>
        <button type="button" className="bt bt-mini bt-neutro" disabled={!!agindo}
          onClick={() => setConfirmando('retirar')}>
          {agindo === 'excluindo' ? 'Retirando...' : 'Retirar cotação'}
        </button>
        <button type="button" className="bt bt-mini bt-perigo" disabled={!!agindo}
          onClick={() => setConfirmando('rejeitar')}>
          {agindo === 'rejeitado' ? 'Rejeitando...' : 'Rejeitar orçamento'}
        </button>
      </div>
      {confirmando === 'rejeitar' && (
        <Confirmar
          titulo={`Rejeitar o orçamento do ticket ${item.ticket}?`}
          mensagem="O ticket volta para o contrato. O orçamento some no Trílogo e o PDF fica guardado — se o chamado voltar para Serviço, cai em Feitos para lançar de novo."
          perigo
          aoConfirmar={() => void rejeitar()}
          aoFechar={() => setConfirmando(null)}
        />
      )}
      {confirmando === 'retirar' && (
        <Confirmar
          titulo={`Retirar a cotação do ticket ${item.ticket} no Trílogo?`}
          mensagem='Apaga a cotação e o orçamento lá. O ticket continua em Serviço, volta para "Feitos" com o mesmo PDF, pronto pra lançar de novo.'
          aoConfirmar={() => void retirarCotacao()}
          aoFechar={() => setConfirmando(null)}
        />
      )}
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
