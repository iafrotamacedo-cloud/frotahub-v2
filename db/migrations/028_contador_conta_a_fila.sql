-- =============================================================================
-- 028 — o contador conta a fila                                          rev 1
-- =============================================================================
--
-- O QUE ESTAVA INCOERENTE
--
--   A tela de Notas e DAVs passou a mostrar só o que ainda dá trabalho: nota que
--   virou orçamento saiu da vista. O CONTADOR da barra, porém, continuou somando
--   tudo que não estava escondido.
--
--   O resultado é o pior tipo de número: o botão dizia 77 e a lista abria com
--   31. As outras 46 estavam resolvidas. Um contador em que não se confia é pior
--   que contador nenhum — quem o vê passa a conferir tudo na mão, que é
--   exatamente o trabalho que ele existia para poupar.
--
-- SÓ UMA EXPRESSÃO MUDA
--
--   `create or replace view` no Postgres não deixa mexer em nome, ordem nem tipo
--   de coluna — mas deixa trocar a EXPRESSÃO. A definição abaixo é a atual,
--   copiada do banco, com a condição `status <> 'usado'` acrescentada na
--   primeira contagem. Nada mais foi tocado.
-- =============================================================================

create or replace view orcamentos_painel as
 SELECT id AS cliente_id,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'orcamento'::text AND d.oculto_em IS NULL AND d.status <> 'usado'::text) AS notas_arquivos,
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
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'pode_lancar'::text) AS prontos_para_lancar,
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
          WHERE d.cliente_id = cl.id AND d.fila = 'direto'::text AND d.oculto_em IS NULL) AS notas_direto
   FROM clientes cl;

insert into schema_migrations (versao, arquivo)
values ('028', '028_contador_conta_a_fila.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select notas_arquivos from orcamentos_painel;
--   -- tem que bater com:
--   select count(*) from documentos
--    where fila = 'orcamento' and oculto_em is null and status <> 'usado';
--
-- PARA DESFAZER
--   Repita o `create or replace view` sem a condição `d.status <> 'usado'`.
-- =============================================================================
