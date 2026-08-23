// rev 1 — o rastro de um login (MOD-USUARIOS-01)
//
// Lê e não escreve: histórico não se corrige. Se uma linha está errada, o que se
// faz é gravar o evento certo por cima — a tranca é do banco, não desta tela.
import { useEffect, useState } from 'react'
import { Janela } from '../../componentes/Janela'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from './Carregando'
import type { Evento, LinhaUsuario, Par } from './tipos'

const VERBO: Record<string, string> = {
  criou: 'criou o login',
  alterou: 'alterou',
  desativou: 'desativou',
  reativou: 'reativou',
  trocou_senha: 'trocou a senha',
}

const CAMPO: Record<string, string> = {
  usuario: 'Usuário',
  nome: 'Nome',
  categoria: 'Categoria',
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

export function Historico({ usuario, aoFechar }: { usuario: LinhaUsuario; aoFechar: () => void }) {
  const [eventos, setEventos] = useState<Evento[] | null>(null)
  const [erro, setErro] = useState<string | null>(null)

  useEffect(() => {
    let vivo = true
    motor<{ historico: Evento[] }>(`/usuarios/${usuario.id}/historico`)
      .then(r => { if (vivo) setEventos(r.historico) })
      .catch(e => { if (vivo) setErro(e instanceof ErroMotor ? e.message : 'Não consegui carregar o histórico.') })
    return () => { vivo = false }
  }, [usuario.id])

  return (
    <Janela titulo="Histórico" descricao={`${usuario.nome} · ${usuario.usuario}`} aoFechar={aoFechar} largura={560}>
      <div className="jn-corpo">
        {erro && <div className="erro-caixa">{erro}</div>}
        {!erro && eventos === null && <Carregando texto="Carregando o histórico..." />}

        {eventos !== null && eventos.length === 0 && (
          <div className="vazio-inline">
            Nada registrado ainda. Este login foi criado antes de o histórico existir,
            ou ninguém mexeu nele desde então.
          </div>
        )}

        {eventos !== null && eventos.length > 0 && (
          <ol className="linha-tempo">
            {eventos.map(ev => (
              <li key={ev.id}>
                <div className="lt-topo">
                  <b>{ev.autor_usuario}</b> {VERBO[ev.acao] ?? ev.acao}
                  <span className="lt-quando">{quando(ev.quando)}</span>
                </div>
                {ev.mudancas && Object.keys(ev.mudancas).length > 0 && (
                  <ul className="lt-mud">
                    {Object.entries(ev.mudancas).map(([campo, par]: [string, Par]) => (
                      <li key={campo}>
                        <span className="c">{CAMPO[campo] ?? campo}</span>
                        {par.de === null || par.de === undefined
                          ? <> <b>{valor(par.para)}</b></>
                          : <> <s>{valor(par.de)}</s> → <b>{valor(par.para)}</b></>}
                      </li>
                    ))}
                  </ul>
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
