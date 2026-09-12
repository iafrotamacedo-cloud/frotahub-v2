-- =============================================================================
-- 065 — nf_progresso_ordens: quanto de cada OC já tem nota fiscal          rev 1
-- =============================================================================
--
-- Por que uma view, e não uma coluna gravada em `ordens_compra`
--
--   "recebimento parcial, com várias NFs" (o dono, 12/09/2026) significa que
--   o total recebido é sempre a SOMA de `notas_fiscais.valor` daquele
--   momento — gravar isso numa coluna própria criaria uma segunda fonte que
--   precisaria ser mantida em sincronia toda vez que uma nota nasce, é
--   cancelada, ou uma OC é substituída. A view calcula na hora, sempre certa.
--
--   Só entram OCs que já passaram pelo PCO (`pco_enviado_em` preenchido) e
--   que não falharam a leitura (`status = 'lido'`) — o mesmo par de
--   condições de "pco-enviados" em `filtroDasOrdens` (ordens.go).
create or replace view public.nf_progresso_ordens as
select
  oc.id as ordem_compra_id,
  oc.cliente_id,
  oc.numero,
  oc.obra_centro_custo,
  oc.fornecedor_id,
  oc.total,
  coalesce(nf.recebido, 0) as recebido,
  oc.total - coalesce(nf.recebido, 0) as restante,
  (coalesce(nf.recebido, 0) >= oc.total) as completa,
  oc.pco_enviado_em
from public.ordens_compra oc
left join (
  select ordem_compra_id, sum(valor) as recebido
  from public.notas_fiscais
  where not cancelada
  group by ordem_compra_id
) nf on nf.ordem_compra_id = oc.id
where oc.status = 'lido' and oc.pco_enviado_em is not null;

comment on view public.nf_progresso_ordens is
  'Uma linha por OC enviada ao PCO, com o quanto já chegou de nota fiscal.
  "completa" é quem sai da fila "Aguardando NF" (nf.go). Não tem RLS própria
  — como toda consulta do backend já filtra cliente_id à mão (mesma
  disciplina do resto do módulo), e a chave de serviço do Supabase não passa
  por RLS mesmo, uma view não precisaria e não teria como ter policy própria.';

insert into schema_migrations (versao, arquivo)
values ('065', '065_nf_progresso_ordens_view.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select * from public.nf_progresso_ordens limit 5;
--
-- PARA DESFAZER
--   drop view public.nf_progresso_ordens;
--   delete from public.schema_migrations where versao = '065';
-- =============================================================================
