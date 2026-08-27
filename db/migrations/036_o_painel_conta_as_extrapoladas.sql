-- =============================================================================
-- 036 — o painel passa a contar as notas que passam do teto
-- =============================================================================
--
-- POR QUE ESTA COLUNA NASCE AGORA
--
--   A frente "Passam do teto" existe em Correções desde a 020 e NUNCA teve
--   entrada no painel — quem não abrisse a tela não sabia que ela existia. No
--   lugar dela, o painel listava "Recusados", uma frente que saiu em 27/08.
--   Ou seja: o menu apontava para uma tela que não existe mais e escondia uma
--   que existe.
--
--   Ao trazê-la para o painel, ela precisa de um número. E número de painel não
--   pode ser escrito à mão no navegador: hoje seria 0 e estaria certo; no dia em
--   que a primeira nota estourar o teto, o cartão continuaria dizendo 0 e a tela
--   dentro dele diria 3. É exatamente a família de defeito que esta semana
--   inteira foi gasta fechando.
--
-- O QUE ELA CONTA
--
--   Nota lida, em circulação, com `bloqueio_motivo` — que é a marca que a 020
--   grava quando a nota não cabe no teto do ticket. É a mesma condição que a
--   tela de Correções usa, e por isso ela mora aqui, uma vez só (CORE-06).
--
-- ⚠ `create or replace view` APAGA AS OPÇÕES E SÓ ACRESCENTA NO FIM
--   A cláusula `with (security_invoker = true)` está repetida abaixo, e a
--   coluna nova vem depois de `esperando_decisao`. Ver 033 e 035.
-- =============================================================================

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
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.precisa_decisao) AS esperando_decisao,
    -- A frente "Passam do teto", que existia sem número. Coluna nova, no fim.
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND d.status = 'lido'::text
            AND d.bloqueio_motivo IS NOT NULL) AS extrapoladas
   FROM clientes cl;

comment on view orcamentos_painel is
  'Os contadores do painel. RODA COMO QUEM PERGUNTA — repita '
  '`with (security_invoker = true)` em qualquer reescrita. Ver migração 033. '
  '`prontos_para_lancar` conta só o que o botão de lote consegue subir.';

insert into schema_migrations (versao, arquivo)
values ('036', '036_o_painel_conta_as_extrapoladas.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select extrapoladas from orcamentos_painel;
--   -- tem que bater com:
--   select count(*) from documentos
--    where oculto_em is null and status = 'lido' and bloqueio_motivo is not null;
--
--   select relname, reloptions from pg_class
--    where relkind='v' and relnamespace='public'::regnamespace order by relname;
--   -- todas com {security_invoker=true}
--
-- PARA DESFAZER
--   Repita o `create or replace view` da 035 — sem esquecer a cláusula `with`.
-- =============================================================================
