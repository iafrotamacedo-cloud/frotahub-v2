-- =============================================================================
-- 025 — o vínculo do orçamento se solta                                  rev 1
-- =============================================================================
--
-- O QUE QUEBROU
--
--   `orcamento_documentos` tem um índice único em (documento_id, ticket). Ele
--   existe por um bom motivo: é o que impede a MESMA nota de virar orçamento
--   duas vezes no mesmo ticket, ou seja, a loja pagar o mesmo material duas
--   vezes.
--
--   Só que apagar um orçamento nunca soltou esse vínculo. O orçamento ficava
--   marcado como `removido` e a linha do vínculo continuava de pé, ocupando a
--   chave. Resultado: a nota ficava impossível de reorçar naquele ticket para
--   sempre — e a tentativa terminava pior que um "não", porque o orçamento novo
--   já tinha nascido quando o banco recusava o vínculo. Sobrava um órfão.
--
--   Aconteceu em 26/08/2026, refazendo o orçamento da DAV 19329 (ticket
--   131382): `duplicate key value violates unique constraint
--   "orcamento_nota_por_ticket"`.
--
-- POR QUE MARCAR E NÃO APAGAR A LINHA
--
--   Porque restaurar um orçamento apagado é uma frente que existe na tela de
--   Correções, e ela só é simples porque NADA é apagado de verdade: restaurar é
--   trocar um status de volta. Apagar a linha do vínculo tiraria a informação
--   de qual nota era — e aí restaurar devolveria um orçamento sem nota, que é
--   um registro pela metade com cara de inteiro.
--
--   Então a linha fica, marcada. E o índice passa a ignorar as marcadas: a
--   chave se libera sem que a memória se perca.
--
-- O QUE ISSO ABRE, E QUE É PROPOSITAL
--
--   Restaurar pode falhar agora — se a nota já foi reorçada naquele ticket
--   enquanto isso, a chave está ocupada por outro. É o certo: o banco recusa
--   em vez de deixar dois orçamentos vivos para o mesmo par. Quem chama trata
--   e explica.
-- =============================================================================

alter table orcamento_documentos
  add column if not exists removido_em timestamptz;

-- O índice novo primeiro, o velho depois: entre um comando e outro a trava
-- continua valendo. Trocar na ordem inversa abriria uma janela sem proteção.
create unique index if not exists orcamento_nota_por_ticket_vivo
  on orcamento_documentos (documento_id, ticket)
  where removido_em is null;

drop index if exists orcamento_nota_por_ticket;

-- Os vínculos de orçamentos JÁ removidos nascem soltos.
--
--   Sem isto a migração não conserta nada do que já está preso: as notas cujo
--   orçamento foi apagado antes de hoje continuariam impossíveis de reorçar,
--   e o defeito seguiria vivo justamente nos casos que o motivaram.
update orcamento_documentos od
   set removido_em = o.removido_em
  from orcamentos o
 where o.id = od.orcamento_id
   and o.status = 'removido'
   and o.removido_em is not null
   and od.removido_em is null;

insert into schema_migrations (versao, arquivo)
values ('025', '025_vinculo_do_orcamento_se_solta.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select count(*) filter (where removido_em is null)  as vivos,
--          count(*) filter (where removido_em is not null) as soltos
--     from orcamento_documentos;
--
--   select indexname from pg_indexes
--    where tablename = 'orcamento_documentos';
--   -- espera-se orcamento_documentos_pkey e orcamento_nota_por_ticket_vivo,
--   -- e NÃO orcamento_nota_por_ticket
--
-- PARA DESFAZER
--   create unique index orcamento_nota_por_ticket
--     on orcamento_documentos (documento_id, ticket);
--   drop index orcamento_nota_por_ticket_vivo;
--   alter table orcamento_documentos drop column removido_em;
--   -- só funciona se não houver par (documento_id, ticket) repetido
-- =============================================================================
