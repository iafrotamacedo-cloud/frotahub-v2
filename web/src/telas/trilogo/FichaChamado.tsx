// rev 1 — a ficha de um chamado
//
// A FOLHA A4
//   A ficha é desenhada como uma folha A4 de pé — 21 × 29,7 cm — e não como uma
//   página de internet que estica. Não é enfeite: um chamado é o documento que
//   alguém imprime, anexa a um e-mail ou mostra numa reunião. Com medida de
//   papel, o que se vê na tela é o que sai na impressora.
//
//   Daí as alturas fixas do cabeçalho: 1,2 cm na primeira linha e 1,0 cm na
//   segunda, como o dono pediu.
//
// A LINHA DO TEMPO FICA À DIREITA
//   Em altura cheia, rolando por dentro. A alternativa — embaixo, em duas
//   colunas — foi desenhada e descartada: ela faz a ficha passar de uma página e
//   quebra a leitura ao virar de coluna. Do jeito escolhido, a ficha cabe em UMA
//   folha tendo o chamado 5 ou 80 eventos.
import { useCallback, useEffect, useState } from 'react'
import { motor, ErroMotor } from '../../motor/cliente'
import { Carregando } from '../../componentes/Carregando'
import {
  classeDaPrioridade, classeDoStatus, contaPorExtenso, ehImagem, ehVideo, emReais,
  linhaDoTempo, quando, tamanhoLegivel,
  type Anexo, type Ficha,
} from './tipos'

interface Props {
  numero: string
  voltar: () => void
  anterior?: () => void
  proximo?: () => void
  posicao?: string
}

export function FichaChamado({ numero, voltar, anterior, proximo, posicao }: Props) {
  const [ficha, setFicha] = useState<Ficha | null>(null)
  const [erro, setErro] = useState<string | null>(null)
  const [aberta, setAberta] = useState<number | null>(null)

  useEffect(() => {
    let vivo = true
    setFicha(null)
    setErro(null)
    setAberta(null)
    motor<Ficha>(`/trilogo/chamados/${encodeURIComponent(numero)}`)
      .then(f => { if (vivo) setFicha(f) })
      .catch(e => {
        if (vivo) setErro(e instanceof ErroMotor ? e.message : 'Não consegui abrir este chamado.')
      })
    return () => { vivo = false }
  }, [numero])

  // Esc fecha o que estiver aberto: primeiro a foto, depois a ficha. É o reflexo
  // de todo mundo, e não custa nada atender.
  const aoTeclar = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') { aberta !== null ? setAberta(null) : voltar() }
  }, [aberta, voltar])

  useEffect(() => {
    window.addEventListener('keydown', aoTeclar)
    return () => window.removeEventListener('keydown', aoTeclar)
  }, [aoTeclar])

  const c = ficha?.chamado
  const midias = (ficha?.anexos ?? []).filter(a => a.colecao === 'anexo')
  const documentos = (ficha?.anexos ?? []).filter(a => a.colecao !== 'anexo')
  const fotos = midias.filter(ehImagem)
  const videos = midias.filter(ehVideo)
  const passos = linhaDoTempo(ficha?.eventos ?? [])
  const total = (ficha?.custos ?? []).reduce((s, k) => s + Number(k.valor || 0), 0)

  return (
    <>
      <div className="fi-barra">
        <button className="bt bt-neutro" type="button" onClick={voltar}>‹ Voltar para a lista</button>
        <span className="fi-migalha">Dados do Trílogo <b>›</b> Chamado {numero}</span>
        <div className="fi-espaco" />
        {(anterior || proximo) && (
          <div className="fi-passo">
            <button type="button" onClick={anterior} disabled={!anterior} aria-label="Chamado anterior">‹</button>
            <span>{posicao}</span>
            <button type="button" onClick={proximo} disabled={!proximo} aria-label="Próximo chamado">›</button>
          </div>
        )}
      </div>

      {erro && <div className="erro-caixa">{erro}</div>}

      {!ficha && !erro ? (
        <Carregando texto={`Abrindo o chamado ${numero}...`} />
      ) : !ficha ? null : (
        <div className="fi-mesa">
          <article className="fi-folha">
            {/* ---- primeira linha: 1,2 cm ---- */}
            <div className="fi-l1">
              <span className="fi-numero">{c!.numero}</span>
              <span className="fi-tracinho">—</span>
              <span className="fi-loja">{c!.loja}</span>
              <div className="fi-dir">
                <span className={'pino ' + classeDoStatus(c!.status)}>{c!.status || '—'}</span>
                <div className="fi-datinha">
                  <span className="fi-rotulo">Aberto em</span>
                  <b>{quando(c!.criado_em)}</b>
                </div>
              </div>
            </div>

            {/* ---- segunda linha: 1,0 cm ---- */}
            <div className="fi-l2">
              <span className={'tri-conta ' + (c!.conta === 'civil' ? 'ct-civil' : 'ct-inst')}>
                {contaPorExtenso(c!.conta)}
              </span>
              <div className="fi-dir">
                <div className="fi-datinha">
                  <span className="fi-rotulo">Responsável</span>
                  <b>{c!.responsavel || '—'}</b>
                </div>
                {c!.prioridade
                  ? <span className={'pino ' + classeDaPrioridade(c!.prioridade)}>{c!.prioridade}</span>
                  : <span className="pino pr-outra">sem prioridade</span>}
              </div>
            </div>

            <div className="fi-corpo">
              <div className="fi-esquerda">
                <section className="fi-bloco">
                  <span className="fi-rotulo">Descrição do chamado</span>
                  <div className="fi-desc">
                    {(c!.descricao || '—').split(/\n+/).map((p, i) => <p key={i}>{p}</p>)}
                  </div>
                </section>

                <section className="fi-bloco">
                  <span className="fi-rotulo">Ambiente</span>
                  <div className="fi-ambiente">
                    {(c!.ambiente || '—').split('>').map((parte, i, todas) => (
                      <span key={i}>
                        {i === 0 ? <b>{parte.trim()}</b> : parte.trim()}
                        {i < todas.length - 1 && <span className="fi-seta">›</span>}
                      </span>
                    ))}
                  </div>
                </section>

                <section className="fi-bloco">
                  <div className="fi-tit">
                    <span className="fi-rotulo">Fotos e vídeos</span>
                    <span className="fi-conta-arq">
                      {fotos.length} {fotos.length === 1 ? 'foto' : 'fotos'}
                      {videos.length > 0 && ` · ${videos.length} ${videos.length === 1 ? 'vídeo' : 'vídeos'}`}
                    </span>
                  </div>
                  {midias.length === 0 ? (
                    <p className="fi-nada">Nenhuma foto neste chamado.</p>
                  ) : (
                    <div className="fi-tira">
                      {midias.map((a, i) => <Miniatura key={a.id} anexo={a} aoAbrir={() => setAberta(i)} />)}
                    </div>
                  )}
                </section>

                <section className="fi-bloco">
                  <div className="fi-tit"><span className="fi-rotulo">Custos do ticket</span></div>
                  {ficha.custos.length === 0 ? (
                    <p className="fi-nada">Nenhum custo lançado.</p>
                  ) : (
                    <table className="fi-custos">
                      <thead>
                        <tr><th>Arquivo</th><th className="fi-tipo">Tipo</th><th className="fi-valor">Valor</th></tr>
                      </thead>
                      <tbody>
                        {ficha.custos.map(k => {
                          const doc = documentos.find(d => d.custo_id === k.id)
                          return (
                            <tr key={k.id}>
                              <td>
                                {doc
                                  ? <a href={doc.link} target="_blank" rel="noreferrer" className="fi-arquivo">{doc.nome}</a>
                                  : <span className="fi-semdoc">{k.numero_documento || 'sem arquivo'}</span>}
                              </td>
                              <td className="fi-tipo">{k.tipo || '—'}</td>
                              <td className="fi-valor">{emReais(k.valor)}</td>
                            </tr>
                          )
                        })}
                        <tr className="fi-total">
                          <td />
                          <td className="fi-tipo"><span className="fi-rotulo">Total</span></td>
                          <td className="fi-valor">R$ {emReais(total)}</td>
                        </tr>
                      </tbody>
                    </table>
                  )}
                </section>

                <footer className="fi-rodape">
                  {c!.tipo && <span>Tipo <b>{c!.tipo}</b></span>}
                  {c!.natureza && <span>Natureza <b>{c!.natureza}</b></span>}
                  <span>Prazo <b>{quando(c!.prazo, false)}</b></span>
                  <span className="fi-lido">Lido em <b>{quando(c!.lido_em ?? null)}</b></span>
                </footer>
              </div>

              <aside className="fi-trilho">
                <div className="fi-tit">
                  <span className="fi-rotulo">Linha do tempo</span>
                  <span className="fi-conta-arq">{ficha.eventos.length} eventos</span>
                </div>
                <div className="fi-tempo">
                  {passos.map((p, i) => (
                    <div key={i} className={'fi-ev' + (p.marco ? ' marco' : '')}>
                      <div className="fi-hora">{quando(p.quando)}</div>
                      <div className="fi-oque">{p.titulo}</div>
                      <div className="fi-quem">{p.autor}</div>
                      {p.texto && <div className="fi-texto">{p.texto}</div>}
                      {p.nota && <div className="fi-nota">{p.nota}</div>}
                    </div>
                  ))}
                </div>
              </aside>
            </div>
          </article>
        </div>
      )}

      {ficha && aberta !== null && (
        <TelaCheia
          anexos={midias}
          indice={aberta}
          aoTrocar={setAberta}
          aoFechar={() => setAberta(null)}
        />
      )}
    </>
  )
}

// ---------------------------------------------------------------------------

function Miniatura({ anexo, aoAbrir }: { anexo: Anexo; aoAbrir: () => void }) {
  // Vídeo não foi copiado para o nosso armazém — foi decisão: eram 86% do peso.
  // Então ele não vira miniatura, vira um cartão que leva ao Trílogo. A tela diz
  // isso com todas as letras, em vez de mostrar um quadrado que não abre.
  if (ehVideo(anexo)) {
    return (
      <a className="fi-mini fi-video" href={anexo.link} target="_blank" rel="noreferrer" title={anexo.nome}>
        <span className="fi-play" />
        <span className="fi-noTrilogo">ver no Trílogo</span>
        <span className="fi-legenda">{anexo.nome}</span>
      </a>
    )
  }
  return (
    <button className="fi-mini" type="button" onClick={aoAbrir} title={anexo.nome}>
      <img src={anexo.link} alt={anexo.nome} loading="lazy" />
      <span className="fi-legenda">{anexo.nome}</span>
    </button>
  )
}

function TelaCheia({ anexos, indice, aoTrocar, aoFechar }: {
  anexos: Anexo[]; indice: number; aoTrocar: (i: number) => void; aoFechar: () => void
}) {
  const atual = anexos[indice]

  const andar = useCallback((passo: number) => {
    const proximo = indice + passo
    if (proximo >= 0 && proximo < anexos.length) aoTrocar(proximo)
  }, [indice, anexos.length, aoTrocar])

  useEffect(() => {
    function tecla(e: KeyboardEvent) {
      if (e.key === 'ArrowRight') andar(1)
      if (e.key === 'ArrowLeft') andar(-1)
    }
    window.addEventListener('keydown', tecla)
    return () => window.removeEventListener('keydown', tecla)
  }, [andar])

  if (!atual) return null

  return (
    <div className="fi-veu" role="dialog" aria-modal="true">
      <div className="fi-veu-topo">
        <div>
          <div className="fi-veu-nome">{atual.nome}</div>
          <div className="fi-veu-sub">
            {[atual.autor, quando(atual.quando), tamanhoLegivel(atual.tamanho)].filter(Boolean).join(' · ')}
          </div>
        </div>
        <span className="fi-veu-conta">{indice + 1} de {anexos.length}</span>
        <a href={atual.link} target="_blank" rel="noreferrer" title="Abrir em outra aba">↗</a>
        <button type="button" onClick={aoFechar} aria-label="Fechar">×</button>
      </div>

      <div className="fi-palco">
        <button type="button" onClick={() => andar(-1)} disabled={indice === 0} aria-label="Anterior">‹</button>
        {ehVideo(atual)
          ? <div className="fi-so-link">Este vídeo não fica no nosso armazém. <a href={atual.link} target="_blank" rel="noreferrer">Abrir no Trílogo</a></div>
          : <img src={atual.link} alt={atual.nome} />}
        <button type="button" onClick={() => andar(1)} disabled={indice === anexos.length - 1} aria-label="Próxima">›</button>
      </div>

      <div className="fi-fita">
        {anexos.map((a, i) => (
          <button
            key={a.id}
            type="button"
            className={i === indice ? 'atual' : ''}
            onClick={() => aoTrocar(i)}
            aria-label={a.nome}
          >
            {ehVideo(a) ? <span className="fi-fita-video" /> : <img src={a.link} alt="" loading="lazy" />}
          </button>
        ))}
      </div>
    </div>
  )
}
