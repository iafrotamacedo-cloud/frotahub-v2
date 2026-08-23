// rev 2 — o rastro de um registro
//
// UMA janela para o sistema inteiro (CORE-06). A tabela de histórico é
// compartilhada entre os módulos, e a tela de ler também: o que muda de um módulo
// para o outro é só o caminho e o vocabulário.
//
// Lê e não escreve: histórico não se corrige. Se uma linha está errada, o que se
// faz é gravar o evento certo por cima — e a tranca é do banco, não desta tela.
import { useEffect, useState } from 'react'
import { Janela } from './Janela'
import { Carregando } from './Carregando'
import { motor, ErroMotor } from '../motor/cliente'

export interface Par {
  de: unknown
  para: unknown
}

export interface Evento {
  id: number
  acao: string
  autor_usuario: string
  quando: string
  mudancas: Record<string, Par> | null
}

const VERBOS: Record<string, string> = {
  criou: 'criou',
  alterou: 'alterou',
  desativou: 'desativou',
  reativou: 'reativou',
  trocou_senha: 'trocou a senha',
  alterou_permissoes: 'mexeu nas permissões',
}

const CAMPOS: Record<string, string> = {
  usuario: 'Usuário',
  nome: 'Nome',
  categoria: 'Categoria',
  codigo: 'Código',
  nivel: 'Nível',
  ativo: 'Situação',
}

function valor(v: unknown): string {
  if (v === null || v === undefined || v === '') return '—'
  if (v === true) return 'ativo'
  if (v === false) return 'inativo'
  return String(v)
}

function quando(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
}

/**
 * As mudanças de permissão são booleanos, mas "de inativo para ativo" não é como
 * se fala de uma rotina liberada. Este caso ganha texto próprio.
 */
function Permissoes({ mudancas }: { mudancas: Record<string, Par> }) {
  const liberadas = Object.keys(mudancas).filter(k => mudancas[k].para === true)
  const retiradas = Object.keys(mudancas).filter(k => mudancas[k].para === false)
  return (
    <ul className="lt-mud">
      {liberadas.length > 0 && (
        <li><span className="c">Liberou</span> <b>{liberadas.join(', ')}</b></li>
      )}
      {retiradas.length > 0 && (
        <li><span className="c">Retirou</span> <b>{retiradas.join(', ')}</b></li>
      )}
    </ul>
  )
}

interface Props {
  /** O caminho no motor, por exemplo `/usuarios/{id}/historico`. */
  caminho: string
  titulo: string
  descricao: string
  aoFechar: () => void
}

export function Historico({ caminho, titulo, descricao, aoFechar }: Props) {
  const [eventos, setEventos] = useState<Evento[] | null>(null)
  const [erro, setErro] = useState<string | null>(null)

  useEffect(() => {
    let vivo = true
    motor<{ historico: Evento[] }>(caminho)
      .then(r => { if (vivo) setEventos(r.historico) })
      .catch(e => { if (vivo) setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o histórico.') })
    return () => { vivo = false }
  }, [caminho])

  return (
    <Janela titulo={titulo} descricao={descricao} aoFechar={aoFechar} largura={560}>
      <div className="jn-corpo">
        {erro && <div className="erro-caixa">{erro}</div>}
        {!erro && eventos === null && <Carregando texto="Carregando o histórico..." />}

        {eventos !== null && eventos.length === 0 && (
          <div className="vazio-inline">
            Nada registrado ainda. Este registro nasceu antes de o histórico existir,
            ou ninguém mexeu nele desde então.
          </div>
        )}

        {eventos !== null && eventos.length > 0 && (
          <ol className="linha-tempo">
            {eventos.map(ev => (
              <li key={ev.id}>
                <div className="lt-topo">
                  <b>{ev.autor_usuario}</b> {VERBOS[ev.acao] ?? ev.acao}
                  <span className="lt-quando">{quando(ev.quando)}</span>
                </div>

                {ev.mudancas && Object.keys(ev.mudancas).length > 0 && (
                  ev.acao === 'alterou_permissoes'
                    ? <Permissoes mudancas={ev.mudancas} />
                    : (
                      <ul className="lt-mud">
                        {Object.entries(ev.mudancas).map(([campo, par]) => (
                          <li key={campo}>
                            <span className="c">{CAMPOS[campo] ?? campo}</span>
                            {par.de === null || par.de === undefined
                              ? <> <b>{valor(par.para)}</b></>
                              : <> <s>{valor(par.de)}</s> → <b>{valor(par.para)}</b></>}
                          </li>
                        ))}
                      </ul>
                    )
                )}
              </li>
            ))}
          </ol>
        )}

        <div className="jn-pe">
          <button type="button" className="bt bt-neutro" onClick={aoFechar}>Fechar</button>
        </div>
      </div>
    </Janela>
  )
}
