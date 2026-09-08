// rev 1 — a ficha de documentos de um funcionário
//
// A tela cruza DUAS listas: o catálogo inteiro de tipos de documento (o mesmo
// para todo mundo) e o que ESTE funcionário já enviou. Um tipo sem linha
// correspondente é "pendente" — nunca foi enviado, e a tela mostra o campo de
// envio direto, sem precisar de um estado "criar documento vazio" no banco.
//
// REENVIAR SUBSTITUI (ver baleryan/documentos.go): escolher um arquivo novo
// para um tipo que já tem documento troca a linha e volta para "enviado" —
// por isso o campo de envio aparece SEMPRE, mesmo quando já tem aprovado.
import { useCallback, useEffect, useState } from 'react'
import { Janela } from '../../componentes/Janela'
import { Carregando } from '../../componentes/Carregando'
import { motor, enviarArquivos, ErroMotor, avisoDe } from '../../motor/cliente'
import type { Perfil } from '../../sessao/tipos'
import type { Funcionario, TipoDocumento, FuncionarioDocumento } from './tipos'

const RUBRICAS: Record<string, string> = {
  pessoal: 'Pessoal',
  admissional: 'Admissional',
  saude: 'Saúde (ASO)',
  nr: 'Normas Regulamentadoras',
  contratual: 'Contratual',
}

const NOME_STATUS: Record<string, string> = {
  pendente: 'Pendente',
  enviado: 'Aguardando conferência',
  aprovado: 'Aprovado',
  reprovado: 'Reprovado',
  vencido: 'Vencido',
}

function alcanca(perfil: Perfil, rotina: string): boolean {
  return perfil.nivel === 'builder' || perfil.rotinas.includes(rotina)
}

interface Props {
  funcionario: Funcionario
  perfil: Perfil
  aoFechar: () => void
  aoMudar: () => void
}

export function DocumentosFuncionario({ funcionario, perfil, aoFechar, aoMudar }: Props) {
  const [tipos, setTipos] = useState<TipoDocumento[] | null>(null)
  const [docs, setDocs] = useState<FuncionarioDocumento[]>([])
  const [erro, setErro] = useState<string | null>(null)
  const [recado, setRecado] = useState<string | null>(null)
  const [enviando, setEnviando] = useState<string | null>(null) // tipo_documento_id em voo
  const [agindo, setAgindo] = useState<string | null>(null) // documento_id em voo

  const podeEnviar = alcanca(perfil, 'CONTRATO_FUNCIONARIOS_DOCUMENTOS')
  const podeAprovar = alcanca(perfil, 'CONTRATO_FUNCIONARIOS_APROVAR')

  const carregar = useCallback(async () => {
    setErro(null)
    try {
      const [t, d] = await Promise.all([
        motor<{ tipos_documento: TipoDocumento[] }>('/tipos-documento'),
        motor<{ documentos: FuncionarioDocumento[] }>(`/funcionarios/${funcionario.id}/documentos`),
      ])
      setTipos(t.tipos_documento)
      setDocs(d.documentos)
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar os documentos.')
      setTipos([])
    }
  }, [funcionario.id])

  useEffect(() => { void carregar() }, [carregar])

  const porTipo = new Map(docs.map(d => [d.tipo_documento_id, d]))

  async function enviar(tipoId: string, arquivo: File, dataValidade: string) {
    setEnviando(tipoId)
    setErro(null)
    try {
      let caminho = `/funcionarios/${funcionario.id}/documentos?tipo_documento_id=${encodeURIComponent(tipoId)}`
      if (dataValidade) caminho += `&data_validade=${encodeURIComponent(dataValidade)}`
      const resposta = await enviarArquivos(caminho, [arquivo])
      setRecado(avisoDe(resposta) ?? 'Documento enviado — aguardando conferência.')
      await carregar()
      aoMudar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui enviar o arquivo.')
    } finally {
      setEnviando(null)
    }
  }

  async function aprovar(doc: FuncionarioDocumento) {
    setAgindo(doc.id)
    setErro(null)
    try {
      const resposta = await motor(`/documentos/${doc.id}/aprovar`, { metodo: 'POST' })
      setRecado(avisoDe(resposta) ?? 'Documento aprovado.')
      await carregar()
      aoMudar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui aprovar.')
    } finally {
      setAgindo(null)
    }
  }

  async function reprovar(doc: FuncionarioDocumento) {
    const motivo = window.prompt('Por que este documento está sendo reprovado?')
    if (!motivo || !motivo.trim()) return
    setAgindo(doc.id)
    setErro(null)
    try {
      const resposta = await motor(`/documentos/${doc.id}/reprovar`, { metodo: 'POST', corpo: { motivo: motivo.trim() } })
      setRecado(avisoDe(resposta) ?? 'Documento reprovado.')
      await carregar()
      aoMudar()
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui reprovar.')
    } finally {
      setAgindo(null)
    }
  }

  return (
    <Janela
      titulo={`Documentos — ${funcionario.nome_completo}`}
      descricao={funcionario.funcoes?.nome ?? 'Sem função definida'}
      aoFechar={aoFechar}
      largura={780}
    >
      <div className="jn-corpo">
        {recado && (
          <div className="recado" role="status">
            {recado}
            <button type="button" onClick={() => setRecado(null)} aria-label="Fechar aviso">×</button>
          </div>
        )}
        {erro && <div className="erro-caixa">{erro}</div>}

        {tipos === null ? (
          <Carregando texto="Carregando os documentos..." />
        ) : tipos.length === 0 ? (
          <div className="vazio">Nenhum tipo de documento cadastrado no catálogo.</div>
        ) : (
          <div className="tabela-rolo">
            <table className="tabela">
              <thead>
                <tr>
                  <th>Documento</th>
                  <th>Situação</th>
                  <th>Validade</th>
                  {podeEnviar && <th>Enviar</th>}
                  {podeAprovar && <th className="acoes-col">Conferência</th>}
                </tr>
              </thead>
              <tbody>
                {Object.entries(
                  tipos.reduce<Record<string, TipoDocumento[]>>((grupos, t) => {
                    (grupos[t.categoria] ??= []).push(t)
                    return grupos
                  }, {})
                ).map(([categoria, doCategoria]) => (
                  <FragmentoCategoria
                    key={categoria}
                    categoria={categoria}
                    tipos={doCategoria}
                    porTipo={porTipo}
                    podeEnviar={podeEnviar}
                    podeAprovar={podeAprovar}
                    enviando={enviando}
                    agindo={agindo}
                    onEnviar={enviar}
                    onAprovar={aprovar}
                    onReprovar={reprovar}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Janela>
  )
}

// Um <> por categoria dentro do <tbody> — cabeçalho de grupo, e as linhas.
function FragmentoCategoria(props: {
  categoria: string
  tipos: TipoDocumento[]
  porTipo: Map<string, FuncionarioDocumento>
  podeEnviar: boolean
  podeAprovar: boolean
  enviando: string | null
  agindo: string | null
  onEnviar: (tipoId: string, arquivo: File, dataValidade: string) => void
  onAprovar: (doc: FuncionarioDocumento) => void
  onReprovar: (doc: FuncionarioDocumento) => void
}) {
  const { categoria, tipos, porTipo, podeEnviar, podeAprovar, enviando, agindo, onEnviar, onAprovar, onReprovar } = props
  return (
    <>
      <tr className="grupo-linha">
        <td colSpan={2 + (podeEnviar ? 1 : 0) + (podeAprovar ? 1 : 0)}>
          {RUBRICAS[categoria] ?? categoria}
        </td>
      </tr>
      {tipos.map(t => {
        const doc = porTipo.get(t.id)
        const status = doc?.status ?? 'pendente'
        const vencido = !!doc?.data_validade && doc.data_validade < new Date().toISOString().slice(0, 10) && status === 'aprovado'
        return (
          <tr key={t.id}>
            <td>{t.nome}</td>
            <td>
              <span className={'pino ' + corDoStatus(vencido ? 'vencido' : status)}>
                {NOME_STATUS[vencido ? 'vencido' : status]}
              </span>
              {doc?.motivo_reprovacao && <p className="dica dica-alerta">{doc.motivo_reprovacao}</p>}
            </td>
            <td>{doc?.data_validade ?? '—'}</td>
            {podeEnviar && (
              <td>
                <CampoEnvio
                  temValidade={t.tem_validade}
                  ocupado={enviando === t.id}
                  onEnviar={(arquivo, validade) => onEnviar(t.id, arquivo, validade)}
                />
              </td>
            )}
            {podeAprovar && (
              <td className="acoes">
                {doc && status === 'enviado' ? (
                  <>
                    <button type="button" className="bt bt-mini" disabled={agindo === doc.id} onClick={() => onAprovar(doc)}>
                      Aprovar
                    </button>
                    <button type="button" className="bt bt-mini bt-perigo" disabled={agindo === doc.id} onClick={() => onReprovar(doc)}>
                      Reprovar
                    </button>
                  </>
                ) : (
                  <span className="dica">—</span>
                )}
              </td>
            )}
          </tr>
        )
      })}
    </>
  )
}

function corDoStatus(status: string): string {
  switch (status) {
    case 'aprovado':
      return 'pino-ok'
    case 'reprovado':
    case 'vencido':
      return 'pino-err'
    case 'enviado':
      return 'pino-warn'
    default:
      return 'pino-off'
  }
}

function CampoEnvio({
  temValidade,
  ocupado,
  onEnviar,
}: {
  temValidade: boolean
  ocupado: boolean
  onEnviar: (arquivo: File, dataValidade: string) => void
}) {
  const [validade, setValidade] = useState('')
  return (
    <div className="campo-envio">
      {temValidade && (
        <input
          type="date"
          value={validade}
          onChange={e => setValidade(e.target.value)}
          aria-label="Validade do documento"
          disabled={ocupado}
        />
      )}
      <input
        type="file"
        disabled={ocupado}
        aria-label="Escolher arquivo"
        onChange={e => {
          const arquivo = e.target.files?.[0]
          if (arquivo) onEnviar(arquivo, validade)
          e.target.value = ''
        }}
      />
      {ocupado && <span className="dica">Enviando...</span>}
    </div>
  )
}
