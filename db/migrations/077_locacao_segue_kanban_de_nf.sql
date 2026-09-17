-- =============================================================================
-- 077 — Locação segue o kanban de Notas Fiscais, com um card próprio    rev 1
-- =============================================================================
--
-- O QUE O DONO EXPLICOU (17/09/2026)
--
--   A OC de locação não pode sumir do kanban raiz depois do recebimento —
--   "faturamento direto" (toda nota que a Frota Macedo repassa pro cliente
--   direto, sem passar por ticket/orçamento) é lei: TODA nota precisa andar
--   até "enviada ao cliente", locação incluída. O que a 072 fazia
--   (`destino_recebimento` tirando a OC de `nf_progresso_ordens` pra
--   sempre) estava errado — resolvia o "não confundir com NF de compra" na
--   base errada.
--
--   A diferença de locação pra uma nota comum: não existe NF física pra
--   escanear na obra. A prova de que o equipamento chegou é o romaneio
--   (`locacoes_recebimentos`, já existe). A NF de verdade da locadora não
--   chega na obra — chega por e-mail DIRETO pro ADM, que passa a ser
--   responsável assim que o romaneio sobe. Por isso a nota de locação nasce
--   num estado PRÓPRIO — `aguardando_nf_locacao` — que fica entre o
--   recebimento e "entregue no escritório", esperando o ADM anexar a NF
--   real. Anexar já é a confirmação de entrega (não tem papel pra carregar
--   fisicamente) — passa direto pra `entregue_escritorio`.
--
-- POR QUE `numero`/`valor` PASSAM A ADMITIR NULO
--
--   Uma nota de locação nasce sem número nem valor reais — só existem
--   quando o ADM anexa a NF. A constraint nova garante que isso só vale
--   PARA quem está em `aguardando_nf_locacao`: qualquer outra nota (compra,
--   ou locação já com a NF anexada) continua exigindo os dois preenchidos,
--   do jeito que sempre foi.
--
-- A VIEW `nf_progresso_ordens` GANHA UM TERCEIRO MOTIVO DE "completa"
--
--   Além de bater o valor exato (076) e estar marcada pra correção (076),
--   agora também sai da fila "Aguardando NF" quem já tem uma nota de
--   locação nascida (`origem = 'locacao'`) — não importa se ainda não tem
--   valor: o recebimento já aconteceu, o que falta (a NF real) se resolve
--   no kanban de NF, não em Aguardando NF de novo.
--
--   De passagem, esta migração corrige a view pra usar `with
--   (security_invoker = true)` — a 065 e a 076 recriaram sem essa cláusula
--   (ver TestReescreverViewNaoDerrubaOSecurityInvoker), o que a fazia
--   rodar com os privilégios de quem CRIOU a view, não de quem pergunta.
-- =============================================================================
begin;

alter table public.notas_fiscais
  add column origem text not null default 'compra' check (origem in ('compra','locacao'));

comment on column public.notas_fiscais.origem is
  '"compra" (padrão) nasce do escaneamento normal na obra. "locacao" nasce
  do romaneio de recebimento de um equipamento locado — sem número/valor
  reais até o ADM anexar a NF que a locadora manda por e-mail.';

alter table public.notas_fiscais alter column numero drop not null;
alter table public.notas_fiscais alter column valor drop not null;

alter table public.notas_fiscais drop constraint notas_fiscais_valor_check;
alter table public.notas_fiscais add constraint notas_fiscais_valor_check
  check (valor is null or valor > 0);

alter table public.notas_fiscais drop constraint notas_fiscais_status_check;
alter table public.notas_fiscais add constraint notas_fiscais_status_check
  check (status in ('recebida', 'aguardando_nf_locacao', 'entregue_escritorio', 'enviada_cliente'));

alter table public.notas_fiscais add constraint notas_fiscais_locacao_pendente_check
  check (status = 'aguardando_nf_locacao' or (numero is not null and valor is not null));

comment on constraint notas_fiscais_locacao_pendente_check on public.notas_fiscais is
  'Só quem está em aguardando_nf_locacao pode ter número/valor nulos — toda
  nota que já andou (recebida, entregue, enviada) continua exigindo os
  dois preenchidos, igual sempre foi.';

create or replace view public.nf_progresso_ordens
with (security_invoker = true) as
select
  oc.id as ordem_compra_id,
  oc.cliente_id,
  oc.numero,
  oc.obra_centro_custo,
  oc.fornecedor_id,
  oc.total,
  coalesce(nf.recebido, 0) as recebido,
  oc.total - coalesce(nf.recebido, 0) as restante,
  (
    (coalesce(nf.recebido, 0) = oc.total)
    or oc.aguardando_correcao
    or exists (
      select 1 from public.notas_fiscais nfloc
       where nfloc.ordem_compra_id = oc.id
         and nfloc.origem = 'locacao'
         and not nfloc.cancelada
    )
  ) as completa,
  oc.pco_enviado_em,
  oc.aguardando_correcao,
  oc.correcao_origem
from public.ordens_compra oc
left join (
  select ordem_compra_id, sum(valor) as recebido
  from public.notas_fiscais
  where not cancelada and valor is not null
  group by ordem_compra_id
) nf on nf.ordem_compra_id = oc.id
where oc.status = 'lido' and oc.pco_enviado_em is not null;

comment on view public.nf_progresso_ordens is
  'Uma linha por OC enviada ao PCO, com o quanto já chegou de nota fiscal.
  "completa" (quem sai de "Aguardando NF") é: bate exato, ou está marcada
  pra correção (076), ou já tem uma nota de locação nascida (077, mesmo
  sem valor ainda — o que falta se resolve no kanban de NF, não aqui).';

insert into schema_migrations (versao, arquivo)
values ('077', '077_locacao_segue_kanban_de_nf.sql');

commit;

-- =============================================================================
-- COMO CONFERIR
--   select column_name, is_nullable from information_schema.columns
--    where table_name = 'notas_fiscais' and column_name in ('numero','valor','origem');
--   -- esperado: numero/valor is_nullable = YES, origem = NO (tem default)
--
--   select conname from pg_constraint where conrelid = 'public.notas_fiscais'::regclass;
--   -- esperado: notas_fiscais_locacao_pendente_check presente
--
-- PARA DESFAZER
--   begin;
--   alter table public.notas_fiscais drop constraint notas_fiscais_locacao_pendente_check;
--   alter table public.notas_fiscais drop constraint notas_fiscais_status_check;
--   alter table public.notas_fiscais add constraint notas_fiscais_status_check
--     check (status in ('recebida','entregue_escritorio','enviada_cliente'));
--   alter table public.notas_fiscais drop constraint notas_fiscais_valor_check;
--   alter table public.notas_fiscais add constraint notas_fiscais_valor_check check (valor > 0);
--   alter table public.notas_fiscais alter column numero set not null;
--   alter table public.notas_fiscais alter column valor set not null;
--   alter table public.notas_fiscais drop column origem;
--   -- recriar a view como estava na 076 (ver aquele arquivo)
--   delete from public.schema_migrations where versao = '077';
--   commit;
-- =============================================================================
