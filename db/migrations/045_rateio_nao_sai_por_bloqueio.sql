-- =============================================================================
-- 045 — rateio não sai da fila por bloqueio de teto                       rev 1
-- =============================================================================
--
-- O QUE O DONO REPORTOU EM 02/09/2026
--
--   "ESTOU COM PROBLEMA NAS NOTAS DE RATEIO...NAO CONSIGO TRATA-LAS, POR MAIS
--    QUE EU TROQUE OS TICKETS, ELA NAO VOLTA PRA TELA DE RATEIO"
--
--   Duas notas, medidas no banco antes deste conserto:
--
--     NOTAS PARA RATEIO.pdf — ticket 132290 estourou o teto (nota 898,20, teto 500,00)
--     f9109f04-...jpg       — ticket 126205 estourou o teto (nota 696,16, teto 500,00)
--
--   As duas com fila = 'rateio', status = 'lido', oculto_em nulo — e mesmo assim
--   `na_fila = false`. Não apareciam em NENHUMA das três abas de "Notas para
--   rateio" (fila / processadas / fora): não são 'usada', não estão ocultas, e
--   `na_fila` as excluía. Só sobrava "Correções › Passam do teto" — que oferece
--   desconto e aprovação do cliente, não o editor de tickets, que só existe na
--   tela de rateio.
--
-- A CAUSA — A MESMA LIÇÃO DA 030, ESQUECIDA NUM CASO NOVO
--
--   A 030 já tinha dito, com todas as letras: "nota de rateio fica na fila de
--   rateio até virar orçamento. Ponto." E a 038, ao reescrever `documentos_lista`
--   com a coluna `destino`, respeitou isso para 'sem-ticket' e 'sem-associacao' —
--   as duas vêm DEPOIS de `fila = 'rateio'` no CASE. Mas `bloqueio_motivo IS NOT
--   NULL THEN 'bloqueada'` ficou ANTES.
--
--   Resultado: nota de rateio com QUALQUER ticket estourando o teto vira
--   `destino = 'bloqueada'` em vez de `'rateio'`. E `na_fila` exclui de propósito
--   quem tem destino em ('sem-ticket','sem-associacao','bloqueada') depois de uma
--   tentativa de gerar — então a nota de rateio bloqueada evapora da própria fila,
--   exatamente como a nota sem ticket evaporava antes da 030.
--
-- O QUE MUDA, E O QUE NÃO MUDA
--
--   MUDA só a ORDEM de duas linhas do CASE: `fila = 'rateio'` passa a ser
--   perguntado ANTES de `bloqueio_motivo`. Nenhuma coluna nasce, nenhuma some,
--   nenhum outro destino muda de lugar.
--
--   NÃO muda o bloqueio em si: `documentos.bloqueio_motivo` continua gravado,
--   `desconto_bp`/`aprovacao_pedida` continuam funcionando do mesmo jeito, e
--   "Correções › Passam do teto" continua enxergando a nota — aquela tela filtra
--   direto por `bloqueio_motivo is not null`, não por `destino` (mesma escolha da
--   037: a nota aparece nos DOIS lugares, não some de nenhum).
--
--   NÃO precisa de conserto de dado: `gerar.go` já limpa `bloqueio_motivo`
--   sozinho quando a nota passa numa tentativa seguinte (linha "a nota passou: se
--   ela estava marcada de antes, a marca sai"). O problema nunca foi o bloqueio
--   ficar preso — foi a nota ficar invisível enquanto ele durava.
--
-- A GARANTIA QUE O DONO PEDIU
--
--   "PRECISO QUE A NOTA CORRIGIDA VOLTE PARA A FILA ORIGINAL DELA" — e a fila
--   dela nunca mudou: `documentos.fila` permanece 'rateio' do início ao fim deste
--   incidente. Trocar ticket (amarrarTickets / apagarTicket) nunca tocou essa
--   coluna. O que estava quebrado era só a VISIBILIDADE na tela — com o CASE
--   corrigido, a nota volta a aparecer em "Notas para rateio" (fila = 'rateio'),
--   que é a fila original dela, assim que `oculto_em` continuar nulo e o status
--   não virar 'usado'.
--
-- POR QUE O `with (security_invoker = true)` ESTÁ AQUI (P-35)
--
--   `create or replace view` APAGA as opções da view. Toda reescrita repete esta
--   cláusula, sem exceção.
-- =============================================================================

create or replace view documentos_lista
with (security_invoker = true) as
 SELECT d.id,
    d.cliente_id,
    d.fila,
    d.tipo,
    d.numero,
    d.dav_numero,
    d.chave_acesso,
    d.emitente_nome,
    d.emissao,
    d.valor_total,
    d.status,
    d.leitura_camada,
    d.leitura_confianca,
    d.nome_arquivo,
    d.arquivo_sha256,
    d.inserido_em,
    d.oculto_em,
    COALESCE(t.quantos, 0::bigint) AS tickets,
    COALESCE(t.lista, '{}'::integer[]) AS ticket_numeros,
    COALESCE(i.quantos, 0::bigint) AS itens,
    COALESCE(t.soltos, '{}'::integer[]) AS ticket_soltos,
    d.status = 'lido'::text AND COALESCE(t.quantos, 0::bigint) > 0
      AND COALESCE(array_length(t.soltos, 1), 0) = 0 AND d.duplicada_de IS NULL
      AND d.bloqueio_motivo IS NULL AND d.aprovacao_pedida IS NOT TRUE AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo,
    d.desconto_bp,
    d.desconto_em,
    d.aprovacao_pedida,
    d.aprovacao_pedida_em,
    CASE cl.destino
        WHEN 'usada'::text          THEN 'usada'::text
        WHEN 'sem-ticket'::text     THEN 'sem-ticket'::text
        WHEN 'sem-associacao'::text THEN 'sem-associacao'::text
        ELSE 'fila'::text
    END AS onde,
    CASE
        WHEN d.status = 'falhou'::text THEN COALESCE(NULLIF(d.leitura_erro, ''::text), 'a leitura desta nota falhou')
        WHEN d.duplicada_de IS NOT NULL THEN 'esta nota já entrou antes — confira antes de gerar'
        WHEN d.bloqueio_motivo IS NOT NULL THEN d.bloqueio_motivo
        WHEN d.aprovacao_pedida IS TRUE THEN 'aprovação pedida ao cliente — esperando a resposta'
        WHEN cl.destino = 'a-conferir'::text THEN 'confirme o valor desta nota'
        ELSE NULL::text
    END AS motivo_conferencia,
    COALESCE(i.soma, 0::numeric) AS soma_dos_itens,
    COALESCE(i.incompletos, 0::bigint) AS itens_incompletos,
    r.fecha AS conta_fecha,
    d.valor_conferido_em,
    cl.destino,
    -- ESTA NOTA APARECE NA FILA? (038, com a ordem consertada pela 045)
    (d.oculto_em IS NULL
     AND d.status <> 'usado'::text
     AND (d.geracao_tentada_em IS NULL
          OR cl.destino <> ALL (ARRAY['sem-ticket'::text, 'sem-associacao'::text, 'bloqueada'::text]))
    ) AS na_fila
   FROM documentos d
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos,
            array_agg(dt.ticket ORDER BY dt.ticket) AS lista,
            array_remove(array_agg(
                CASE
                    WHEN dt.chamado_id IS NULL THEN dt.ticket
                    ELSE NULL::integer
                END ORDER BY dt.ticket), NULL::integer) AS soltos
           FROM documento_tickets dt
          WHERE dt.documento_id = d.id) t ON true
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos,
            sum(di.valor_total) AS soma,
            count(*) FILTER (WHERE di.descricao = ''::text OR di.quantidade <= 0::numeric
                               OR di.valor_unitario <= 0::numeric) AS incompletos
           FROM documento_itens di
          WHERE di.documento_id = d.id) i ON true
     LEFT JOIN documentos o ON o.id = d.duplicada_de
     LEFT JOIN LATERAL ( SELECT (d.valor_total IS NOT NULL AND d.valor_total > 0::numeric
              AND COALESCE(i.quantos, 0::bigint) > 0
              AND COALESCE(i.incompletos, 0::bigint) = 0
              AND abs(COALESCE(i.soma, 0::numeric) - d.valor_total) <= d.valor_total * 0.01) AS fecha) r ON true
     LEFT JOIN LATERAL ( SELECT
        CASE
            WHEN d.oculto_em IS NOT NULL                    THEN 'excluida'::text
            WHEN d.status = 'usado'::text                   THEN 'usada'::text
            WHEN d.duplicada_de IS NOT NULL                 THEN 'repetida'::text
            WHEN d.status = 'falhou'::text                  THEN 'falhou'::text
            WHEN d.status <> 'lido'::text                   THEN 'lendo'::text
            -- A 045 MOVEU ESTA LINHA PARA CIMA
            --
            --   Estava depois de `bloqueio_motivo`, e por isso uma nota de rateio
            --   com um ticket estourando o teto virava 'bloqueada' em vez de
            --   continuar 'rateio' — e `na_fila` a excluía da própria fila dela.
            --   A 030 já tinha decidido: nota de rateio fica na fila até virar
            --   orçamento, ponto. Esta linha agora vem antes de qualquer motivo de
            --   bloqueio, do mesmo jeito que já vinha antes de 'sem-ticket' e
            --   'sem-associacao'.
            WHEN d.fila = 'rateio'::text                    THEN 'rateio'::text
            WHEN d.bloqueio_motivo IS NOT NULL              THEN 'bloqueada'::text
            WHEN d.fila = 'direto'::text                    THEN 'direto'::text
            WHEN COALESCE(t.quantos, 0::bigint) = 0         THEN 'sem-ticket'::text
            WHEN COALESCE(array_length(t.soltos, 1), 0) > 0 THEN 'sem-associacao'::text
            WHEN d.aprovacao_pedida IS TRUE                 THEN 'espera-cliente'::text
            WHEN d.valor_conferido_em IS NULL AND NOT r.fecha THEN 'a-conferir'::text
            WHEN d.status = 'lido'::text                    THEN 'pronta'::text
            ELSE 'nao-classificado'::text
        END AS destino) cl ON true;

comment on view documentos_lista is
  'A lista das notas. RODA COMO QUEM PERGUNTA (security_invoker) — se você '
  'reescrever esta view, REPITA a cláusula `with (security_invoker = true)`, '
  'senão a tranca cai em silêncio (migração 033). `destino` é a classificação '
  'completa; `onde` é uma projeção dela; `na_fila` diz se a nota ainda espera '
  'uma decisão sua na tela de Notas e DAVs (migração 038). Nota de rateio nunca '
  'vira ''bloqueada'', ''sem-ticket'' nem ''sem-associacao'' — continua ''rateio'' '
  'até virar orçamento, mesmo bloqueada por teto (migração 045).';

insert into schema_migrations (versao, arquivo)
values ('045', '045_rateio_nao_sai_por_bloqueio.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- as duas notas do incidente de 02/09/2026 voltam à fila de rateio:
--   select nome_arquivo, fila, destino, onde, na_fila, bloqueio_motivo
--     from documentos_lista
--    where id in ('8c85e686-4209-4436-b860-6d1160b4eda0',
--                 'f32d0811-ecda-45cf-8b51-7c2291f64bf0');
--   -- esperado: destino = 'rateio', onde = 'fila', na_fila = true,
--   --           bloqueio_motivo continua preenchido (a 045 não apaga bloqueio)
--
--   -- nenhuma nota de rateio no cliente aparece como bloqueada, sem ticket ou
--   -- sem associação — mesmo com ticket solto ou nota bloqueada:
--   select count(*) from documentos_lista
--    where fila = 'rateio' and destino in ('bloqueada','sem-ticket','sem-associacao');
--   -- esperado: 0
--
--   -- a fila de orçamento não mudou:
--   select destino, count(*) from documentos_lista
--    where fila = 'orcamento' and oculto_em is null group by destino order by destino;
--
--   -- e a tranca continua de pé:
--   select relname, reloptions from pg_class where relname = 'documentos_lista';
--   -- esperado: {security_invoker=true}
--
-- PARA DESFAZER
--   Repita o `create or replace view` da 038 (com o `with (security_invoker =
--   true)`), que devolve `bloqueio_motivo` para antes de `fila = 'rateio'`.
-- =============================================================================
