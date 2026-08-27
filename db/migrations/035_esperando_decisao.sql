-- =============================================================================
-- 035 — o orçamento que só uma pessoa destrava sai da fila
-- =============================================================================
--
-- O QUE ACONTECEU, MEDIDO EM 27/08/2026
--
--   A fila de Lançar mostrava 5 "podem subir". O botão dizia "Lançar todos que
--   podem subir (5)". O resultado foi **0 de 5 subiram**:
--
--     125199-2  o ticket já tem R$ 33,36 lançado (nº 38062) — pode ser duplicata
--     130633    o ticket já tem R$ 233,76 lançado (nº 38502) — pode ser duplicata
--     126998-2  o ticket já tem R$ 143,28 lançado (nº 37671) — pode ser duplicata
--     130396-6  passou do teto: R$ 578,56 já no ticket + R$ 598,86 deste
--     125973-2  passou do teto: R$ 557,40 já no ticket + R$ 83,88 deste
--
--   As duas travas fizeram exatamente o que deviam. Quem errou foi a FILA, que
--   contou como pronto o que precisa de uma pessoa decidindo.
--
-- A DIFERENÇA QUE ESTA MIGRAÇÃO ESCREVE NO BANCO
--
--   Nem todo bloqueio é igual, e a distinção não é de gravidade — é de QUEM
--   DESFAZ:
--
--     destrava sozinho    `ticket_status`, `trilogo_fora`. O chamado anda, a
--                         rede volta. Tentar de novo é o certo, e o lote deve
--                         continuar tentando. (Medido: 7 de 26 destravaram
--                         sozinhos em seis dias.)
--
--     só gente destrava   `teto`, `possivel_duplicata`. O ticket vai continuar
--                         com o mesmo custo lançado; o teto vai continuar sendo
--                         R$ 600. Tentar de novo dá o mesmo resultado, para
--                         sempre.
--
--   Os segundos não são fila: são uma pergunta esperando resposta. Vão para
--   Pendências, ao lado das outras duas listas de "esperando alguém".
--
-- POR QUE UMA COLUNA, E NÃO UM `if` NA TELA
--
--   Ontem essa lista de dois nomes nasceu escrita no TypeScript. Hoje ela
--   precisa ser respondida também pelo motor (para filtrar no banco, CORE-10) e
--   pelo balanço (para o Fechamento não contar como "pode lançar" o que a tela
--   mostra em outro lugar). Três lugares com a mesma lista é como duas delas um
--   dia discordam (CORE-06).
--
--   Ela passa a morar aqui, e os três leem.
--
-- ⚠ `create or replace view` APAGA AS OPÇÕES DA VIEW E SÓ ACRESCENTA NO FIM
--   As duas reescritas abaixo repetem `with (security_invoker = true)` e põem a
--   coluna nova depois da última. Ver a 033 e a 034.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1) a lista dos orçamentos ganha `precisa_decisao`
-- -----------------------------------------------------------------------------

create or replace view orcamentos_lista
with (security_invoker = true) as
 SELECT o.id,
    o.cliente_id,
    o.ticket,
    o.parte,
    o.conta,
    o.status,
    o.valor,
    o.valor_nota,
    o.reduzido_pelo_teto,
    o.valor_antes_do_teto,
    o.rateio,
    o.criado_em,
    o.lancado_em,
    o.faturado,
    o.pago,
    o.trilogo_custo_id,
    o.arquivo_pdf_sha256,
    u.nome AS loja,
    c.descricao AS chamado_descricao,
    ( SELECT string_agg(d.numero, ', '::text ORDER BY d.numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d ON d.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d.numero IS NOT NULL) AS notas,
    ( SELECT string_agg(d.dav_numero, ', '::text ORDER BY d.dav_numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d ON d.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d.dav_numero IS NOT NULL) AS davs,
    o.lancamento_bloqueio,
    o.lancamento_bloqueio_detalhe,
    o.lancamento_tentado_em,
    o.lancamento_tentativas,
    c.status AS ticket_status,
    c.status_codigo AS ticket_status_codigo,
    (c.status_codigo = ANY (ARRAY[1, 7])) AND r.ja_terminou AS reaberto,
        CASE
            WHEN (c.status_codigo = ANY (ARRAY[1, 7])) AND r.ja_terminou THEN r.ultima_frase
            ELSE NULL::text
        END AS motivo_reabertura,
        CASE
            WHEN o.status <> 'gerado'::text THEN NULL::text
            WHEN c.id IS NULL THEN 'sem_chamado'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6]) THEN 'pode_lancar'::text
            WHEN c.status_codigo = 3 THEN 'cliente'::text
            WHEN c.status_codigo = 1 AND r.ja_terminou THEN 'cliente'::text
            WHEN c.status_codigo = 1 THEN 'encarregados'::text
            WHEN c.status_codigo = 7 THEN 'encarregados'::text
            ELSE 'outro'::text
        END AS destino,
    ( SELECT a.avisado_em
           FROM ticket_avisos a
          WHERE a.cliente_id = o.cliente_id AND a.ticket = o.ticket
          ORDER BY a.avisado_em DESC
         LIMIT 1) AS avisado_em,
    o.unidade_id,
    o.fatura_id,
    o.valor_nota_cheio,
    o.ajustado_pelo_teto,
    -- O ESTADO — agora com a espera por decisão em linha própria
    --
    --   Antes ela caía em 'recusado-ja-liberou', junto com quem só precisava de
    --   uma nova tentativa. Somar as duas é o que fazia a tela dizer que 5
    --   podiam subir quando nenhuma podia.
        CASE
            WHEN o.status = 'removido'::text             THEN 'apagado'::text
            WHEN o.status = 'lancado'::text              THEN 'lancado'::text
            WHEN o.status = 'aguardando_aprovacao'::text THEN 'aguardando-aprovacao'::text
            WHEN d.precisa                               THEN 'espera-decisao'::text
            WHEN c.id IS NULL                            THEN 'sem-chamado'::text
            WHEN c.status_codigo = 3                     THEN 'espera-cliente'::text
            WHEN c.status_codigo = 1 AND r.ja_terminou   THEN 'espera-cliente'::text
            WHEN c.status_codigo = ANY (ARRAY[1, 7])     THEN 'espera-equipe'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6]) AND o.lancamento_bloqueio IS NOT NULL
                                                         THEN 'recusado-ja-liberou'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6])     THEN 'pode-lancar'::text
            ELSE 'nao-classificado'::text
        END AS estado,
    -- ---------------------------------------------------------------------
    -- PRECISA DE UMA PESSOA?
    --
    --   `teto` e `possivel_duplicata` não andam sozinhos: o ticket vai
    --   continuar com o mesmo custo lançado, e o teto vai continuar sendo o
    --   mesmo. Tentar de novo dá o mesmo resultado para sempre.
    --
    --   `ticket_status` e `trilogo_fora` ficam de FORA de propósito — aqueles
    --   destravam quando o mundo anda, e o lote deve continuar tentando.
    --   Trazê-los para cá seria condená-los a um clique manual eterno.
    --
    --   Só vale para orçamento em pé: um lançado ou apagado não espera decisão
    --   de ninguém, mesmo carregando a marca da última tentativa.
    -- ---------------------------------------------------------------------
    d.precisa AS precisa_decisao
   FROM orcamentos o
     LEFT JOIN unidades u ON u.id = o.unidade_id
     LEFT JOIN chamados c ON c.id = o.chamado_id
     LEFT JOIN LATERAL ( SELECT (o.status = 'gerado'::text
              AND o.lancamento_bloqueio = ANY (ARRAY['teto'::text, 'possivel_duplicata'::text]))
              AS precisa) d ON true
     LEFT JOIN LATERAL ( SELECT (EXISTS ( SELECT 1
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[3, 5, 6])))) AS ja_terminou,
            ( SELECT e.texto
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[1, 7])) AND COALESCE(e.texto, ''::text) <> ''::text
                  ORDER BY e.quando DESC
                 LIMIT 1) AS ultima_frase) r ON true;

comment on view orcamentos_lista is
  'A lista dos orçamentos. RODA COMO QUEM PERGUNTA — repita '
  '`with (security_invoker = true)` em qualquer reescrita. Ver migração 033. '
  '`destino` = quem tem que agir; `estado` = onde está; `precisa_decisao` = '
  'travado por algo que só uma pessoa desfaz (teto, duplicidade).';

-- -----------------------------------------------------------------------------
-- 2) o rótulo do novo estado, no balanço
-- -----------------------------------------------------------------------------

create or replace view fechamento_orcamentos
with (security_invoker = true) as
  select o.cliente_id,
         case o.estado
           when 'pode-lancar'          then 1 when 'espera-decisao'      then 2
           when 'recusado-ja-liberou'  then 3 when 'espera-equipe'       then 4
           when 'espera-cliente'       then 5 when 'sem-chamado'         then 6
           when 'aguardando-aprovacao' then 7 when 'lancado'             then 8
           when 'apagado'              then 9
           else 99 end as ordem,
         o.estado,
         case o.estado
           when 'pode-lancar'          then 'pode lançar agora'
           when 'espera-decisao'       then 'esperando uma decisão sua — teto ou duplicidade'
           when 'recusado-ja-liberou'  then 'foi recusado antes, mas o chamado já andou'
           when 'espera-equipe'        then 'parado: pendência nossa'
           when 'espera-cliente'       then 'parado: pendência do cliente'
           when 'sem-chamado'          then 'parado: chamado não encontrado'
           when 'aguardando-aprovacao' then 'esperando a aprovação do cliente'
           when 'lancado'              then 'lançado no Trílogo'
           when 'apagado'              then 'apagado'
           else 'NÃO CLASSIFICADO — avise o suporte'
         end as rotulo,
         count(*) as quantos,
         COALESCE(sum(o.valor), 0::numeric) as valor
    from orcamentos_lista o
   group by o.cliente_id, o.estado;

-- -----------------------------------------------------------------------------
-- 3) o painel para de prometer o que não entrega
-- -----------------------------------------------------------------------------
--
-- `prontos_para_lancar` contava os 5 que não podiam subir. A expressão muda; o
-- nome e o tipo continuam, que é o que o `create or replace view` exige.

create or replace view orcamentos_painel
with (security_invoker = true) as
 SELECT id AS cliente_id,
    ( SELECT count(*) AS count
           FROM documentos_lista d
          WHERE d.cliente_id = cl.id AND d.fila = 'orcamento'::text AND d.oculto_em IS NULL AND d.onde = 'fila'::text) AS notas_arquivos,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'rateio'::text AND d.oculto_em IS NULL AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS rateio_sem_ticket,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text) AS a_lancar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND d.status = 'lido'::text AND d.fila = 'orcamento'::text AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS sem_ticket,
    ( SELECT count(*) AS count
           FROM documento_tickets t
             JOIN documentos d ON d.id = t.documento_id
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND t.chamado_id IS NULL AND d.fila <> 'direto'::text) AS sem_associacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'aguardando_aprovacao'::text) AS aguardando_aprovacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'removido'::text) AS apagados,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS no_total,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS valor_total,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND array_length(dl.ticket_soltos, 1) > 0) AS notas_travadas,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND dl.pronto_para_gerar) AS prontas_para_gerar,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text AND o.lancamento_bloqueio IS NOT NULL) AS recusados,
    -- SÓ O QUE O BOTÃO VAI CONSEGUIR SUBIR
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'pode_lancar'::text
            AND NOT l.precisa_decisao) AS prontos_para_lancar,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'cliente'::text) AS esperando_cliente,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'encarregados'::text) AS esperando_equipe,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL AND o.faturamento_direto = false) AS a_faturar,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL AND o.faturamento_direto = false) AS valor_a_faturar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'direto'::text AND d.oculto_em IS NULL) AS notas_direto,
    -- A terceira lista de Pendências. Coluna nova, no fim.
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.precisa_decisao) AS esperando_decisao
   FROM clientes cl;

comment on view orcamentos_painel is
  'Os contadores do painel. RODA COMO QUEM PERGUNTA — repita '
  '`with (security_invoker = true)` em qualquer reescrita. Ver migração 033. '
  '`prontos_para_lancar` conta só o que o botão de lote consegue subir.';

insert into schema_migrations (versao, arquivo)
values ('035', '035_esperando_decisao.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR — e os números medidos em 27/08/2026, antes desta migração
--
--   select prontos_para_lancar, esperando_decisao, esperando_equipe,
--          esperando_cliente, a_lancar
--     from orcamentos_painel;
--   -- esperado: 0 · 5 · 41 · 47 · 93     (antes: prontos dizia 5)
--
--   select ordem, rotulo, quantos, valor::numeric(12,2)
--     from fechamento_orcamentos order by ordem;
--   -- 'esperando uma decisão sua' aparece com 5
--
--   -- e a conta continua fechando:
--   select (select count(*) from orcamentos) = (select sum(quantos) from fechamento_orcamentos);
--   -- esperado: t
--
--   -- ninguém no balde do desconhecido:
--   select * from fechamento_orcamentos where ordem = 99;   -- nenhuma linha
--
--   -- e a tranca de pé:
--   select relname, reloptions from pg_class
--    where relkind='v' and relnamespace='public'::regnamespace order by relname;
--
-- PARA DESFAZER
--   Repita os três `create or replace view` da 034 — sem esquecer o
--   `with (security_invoker = true)` em cada um.
-- =============================================================================
