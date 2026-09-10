// rev 1 — Administrativo > Compras > Inserir OC
//
// O PEDIDO (10/09/2026): "um botão para abrir o doc do PC e colocar na tela,
// outro botão para inserir, uma lista embaixo de todos os documentos que
// estão na fila." Mesmo padrão de inserção que Orçamentos já usa
// (`Arquivos.tsx`/`Insercao`) — sem fila dupla, sem tickets, sem geração: são
// duas coisas que a OC não tem.
//
// A LEITURA AINDA NÃO EXISTE
//
//	`numero`, `comprador_nome` e `total` só aparecem depois que o leitor do PDF
//	da OC for construído (próximo passo, não este — ver `baleryan/administrativo/
//	ordens.go`). Até lá toda OC fica em `status = 'inserido'`, e a lista mostra
//	isso sem fingir que sabe mais do que sabe.
import { useCallback, useEffect, useRef, useState } from 'react'
import { motor, enviarArquivos, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import { emReais, emDataHora, type OrdemDeCompra, type ResultadoDaInsercao } from './tipos'

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

export function InserirOC() {
  const [ordens, setOrdens] = useState<OrdemDeCompra[] | null>(null)
  const [erro, setErro] = useState('')
  const [recado, setRecado] = useState('')

  const carregar = useCallback(async () => {
    try {
      const r = await motor<{ ordens: OrdemDeCompra[] }>('/administrativo/compras/ordens')
      setOrdens(r.ordens)
    } catch (e) {
      setOrdens([])
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar a fila.')
    }
  }, [])

  useEffect(() => { void carregar() }, [carregar])

  async function abrirArquivo(o: OrdemDeCompra) {
    try {
      const r = await motor<{ url: string }>(`/administrativo/compras/ordens/${o.id}/arquivo`)
      window.open(r.url, '_blank', 'noopener')
    } catch (e) {
      setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir o arquivo.')
    }
  }

  return (
    <>
      <header className="hero hero-linha">
        <div>
          <h1>Inserir OC</h1>
          <p>A ordem de compra aprovada no Obra Prima, sempre em PDF digital. A leitura que monta o resto vem a seguir.</p>
        </div>
      </header>

      {recado && (
        <div className="recado" role="status">
          {recado}
          <button type="button" onClick={() => setRecado('')} aria-label="Fechar aviso">×</button>
        </div>
      )}
      {erro && <div className="erro-caixa">{erro}</div>}

      <Insercao
        aoTerminar={carregar}
        aoRecado={setRecado}
        aoFalhar={setErro}
      />

      {ordens === null ? (
        <Carregando texto="Carregando a fila..." />
      ) : ordens.length === 0 ? (
        <div className="vazio">Nada na fila. Insira o PDF de uma OC para começar.</div>
      ) : (
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
                      <div className="dica">
                        {o.numero ? `nº ${o.numero}` : ''}
                        {o.numero && o.comprador_nome ? ' · ' : ''}
                        {o.comprador_nome ?? ''}
                        {o.total ? ` · ${emReais(o.total)}` : ''}
                      </div>
                    )}
                    {o.erro_leitura && <div className="dica dica-alerta">{o.erro_leitura}</div>}
                  </td>
                  <td>{emDataHora(o.criado_em)}</td>
                  <td><span className={'pino ' + CLASSE_STATUS[o.status]}>{NOME_STATUS[o.status]}</span></td>
                  <td className="acoes">
                    <button type="button" className="bt bt-mini" onClick={() => void abrirArquivo(o)}>ver</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

// ---------------------------------------------------------------------------
// a barra de inserção — mesmo desenho de Orçamentos (Arquivos.tsx/Insercao):
// arrastar ou escolher, um botão Inserir, e o aviso de arquivo repetido com
// os dois nomes (o de agora e o de quem já estava), nunca só a contagem.
// ---------------------------------------------------------------------------

function Insercao({ aoTerminar, aoRecado, aoFalhar }: {
  aoTerminar: () => Promise<void>
  aoRecado: (s: string) => void
  aoFalhar: (s: string) => void
}) {
  const [escolhidos, setEscolhidos] = useState<File[]>([])
  const [enviando, setEnviando] = useState(false)
  const [sobre, setSobre] = useState(false)
  const [repetidos, setRepetidos] = useState<{ nome: string; como: string }[]>([])
  const campo = useRef<HTMLInputElement>(null)

  async function inserir() {
    if (!escolhidos.length || enviando) return
    setEnviando(true)
    aoFalhar('')
    try {
      const r = await enviarArquivos<{ arquivos: ResultadoDaInsercao[] }>(
        '/administrativo/compras/ordens', escolhidos)
      const ruins = r.arquivos.filter(a => a.erro)
      const repetidas = r.arquivos.filter(a => a.ja_existia)
      const novas = r.arquivos.filter(a => a.id && !a.ja_existia)

      if (ruins.length) {
        aoFalhar(ruins.map(a => `${a.nome}: ${a.erro}`).join(' · '))
      } else if (novas.length) {
        aoRecado(novas.length === 1 ? '1 ordem de compra inserida.' : `${novas.length} ordens de compra inseridas.`)
      }
      setRepetidos(repetidas.map(a => ({ nome: a.nome, como: a.ja_existia_como ?? '' })))
      setEscolhidos([])
      if (campo.current) campo.current.value = ''
      await aoTerminar()
    } catch (e) {
      aoFalhar(e instanceof ErroMotor ? e.message : 'Não consegui inserir os arquivos.')
    } finally {
      setEnviando(false)
    }
  }

  return (
    <div
      className={'adm-insercao' + (sobre ? ' sobre' : '')}
      onDragOver={e => { e.preventDefault(); setSobre(true) }}
      onDragLeave={() => setSobre(false)}
      onDrop={e => {
        e.preventDefault()
        setSobre(false)
        setEscolhidos(Array.from(e.dataTransfer.files))
      }}
    >
      {repetidos.length > 0 && (
        <div className="aviso-caixa">
          <b>{repetidos.length}</b>{' '}
          {repetidos.length === 1 ? 'arquivo não entrou' : 'arquivos não entraram'}
          {' '}porque {repetidos.length === 1 ? 'já estava' : 'já estavam'} na fila —
          {' '}mesmo arquivo, byte a byte. Nenhuma OC foi perdida.
          <ul style={{ margin: '6px 0 0', paddingLeft: 18 }}>
            {repetidos.map(x => (
              <li key={x.nome}>
                <b>{x.nome}</b>
                {x.como && x.como !== x.nome && <> é a mesma que <b>{x.como}</b></>}
              </li>
            ))}
          </ul>
        </div>
      )}

      <p>insira o PDF da OC direto pelo site — sempre digital, nunca foto</p>
      <div className="linha">
        <button type="button" className="bt bt-neutro" onClick={() => campo.current?.click()}>
          Escolher arquivos
        </button>
        <input
          ref={campo}
          type="file"
          multiple
          accept=".pdf"
          style={{ display: 'none' }}
          onChange={e => setEscolhidos(Array.from(e.target.files ?? []))}
        />
        <div className="caixa">
          {escolhidos.length === 0
            ? 'nenhum arquivo escolhido — ou arraste até aqui'
            : escolhidos.length === 1
              ? escolhidos[0].name
              : `${escolhidos[0].name} e mais ${escolhidos.length - 1}`}
        </div>
        <button
          type="button"
          className="bt bt-forte"
          disabled={!escolhidos.length || enviando}
          onClick={() => void inserir()}
        >
          {enviando ? 'Inserindo…' : 'Inserir'}
        </button>
      </div>
    </div>
  )
}
