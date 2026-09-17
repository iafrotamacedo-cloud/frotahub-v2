-- =============================================================================
-- 076 — Correção de OC: nota com valor divergente vira fila própria      rev 1
-- =============================================================================
--
-- O QUE O DONO EXPLICOU (17/09/2026, obra piloto MSL Fátima)
--
--   NF com valor diferente do total da OC acontece por três motivos: item
--   faltando (raro sobrando), a nota é parcial mesmo (já bem resolvido,
--   `nf_progresso_ordens`), ou o produto é cobrado por peso e a OC não bate
--   exatamente com o que a balança do fornecedor deu. No terceiro caso (e no
--   primeiro), quem corrige é o RC — sobe a OC corrigida no Obra Prima e
--   substitui aqui, do MESMO jeito que já existe pra faturamento errado
--   (`substituicao.go`) — só que agora com nota(s) fiscal(is) já recebidas
--   na OC velha, que precisam migrar pra nova em vez de travar a exclusão
--   (FK `on delete restrict`).
--
--   Um percentual (3%, "cobre 95% dos casos") decide o que é automático:
--   divergência dentro da faixa sai IMEDIATAMENTE de "Aguardando NF" pra
--   "Correção de OC", sem ninguém apertar nada. Fora da faixa, a OC continua
--   em Aguardando NF até a equipe da obra (RO) decidir, na tela, mandar pra
--   correção mesmo assim — nunca sai sozinha.
--
-- POR QUE NÃO É SÓ UM CAMPO CALCULADO
--
--   Ao contrário de "completa" (nf_progresso_ordens, sempre recalculada),
--   "está em correção" PRECISA ser um estado gravado: alguém pode "voltar
--   pra fila inicial" (RC julga que era parcial mesmo, ou o fornecedor
--   troca a nota) — se fosse só um cálculo em cima do valor recebido, a OC
--   voltaria pra correção sozinha no próximo cálculo, e o "voltar" nunca
--   pegaria. `correcao_origem` registra se foi o sistema ou uma pessoa que
--   decidiu, pra `avaliarDivergenciaOC` (o cálculo automático) NUNCA
--   sobrescrever uma decisão manual.
--
-- A MUDANÇA EM "completa" (nf_progresso_ordens)
--
--   Era `recebido >= total` — qualquer sobra grande completava a OC
--   SOZINHA e silenciosa, sem ninguém decidir nada. Passa a ser
--   `recebido = total` (bate exato, sem drama nenhum) OU `aguardando_correcao`
--   (passou pela fila, com ou sem correção ainda). Fora dessas duas
--   condições, a OC continua visível em Aguardando NF — nunca mais some
--   sozinha por estar "meio recebida demais".
-- =============================================================================

alter table public.ordens_compra
  add column aguardando_correcao boolean not null default false,
  add column correcao_origem text check (correcao_origem in ('automatica','manual')),
  add column correcao_marcada_em timestamptz,
  add column correcao_marcada_por uuid references public.perfis(id);

comment on column public.ordens_compra.aguardando_correcao is
  'true = a NF recebida diverge do total da OC (dentro ou fora dos 3%
  automáticos) e está na fila "Correção de OC", em Compras. O RC corrige
  subindo a OC certa (migra a(s) NF(s) pra ela) ou volta pra Aguardando NF.';

create index ordens_compra_aguardando_correcao
  on public.ordens_compra (cliente_id) where aguardando_correcao;

-- `create or replace view` só aceita ACRESCENTAR coluna no fim — colocar
-- `aguardando_correcao`/`correcao_origem` no meio (antes de "completa")
-- faria o Postgres achar que "completa" estava sendo RENOMEADA. Por isso as
-- duas colunas novas vão depois de `pco_enviado_em`, não perto do que elas
-- afetam.
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
  (
    (coalesce(nf.recebido, 0) = oc.total)
    or oc.aguardando_correcao
  ) as completa,
  oc.pco_enviado_em,
  oc.aguardando_correcao,
  oc.correcao_origem
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
  "completa" é quem sai da fila "Aguardando NF" (nf.go) — bate exato OU está
  marcada pra correção (migração 076); nunca mais só "recebido >= total",
  que deixava sobra grande sumir da fila sem ninguém decidir nada.';

insert into schema_migrations (versao, arquivo)
values ('076', '076_correcao_de_oc.sql');

-- =============================================================================
-- COMO CONFERIR
--   select ordem_compra_id, total, recebido, aguardando_correcao, completa
--     from nf_progresso_ordens limit 5;
--
-- PARA DESFAZER
--   create or replace view public.nf_progresso_ordens as
--   select oc.id as ordem_compra_id, oc.cliente_id, oc.numero, oc.obra_centro_custo,
--     oc.fornecedor_id, oc.total, coalesce(nf.recebido, 0) as recebido,
--     oc.total - coalesce(nf.recebido, 0) as restante,
--     (coalesce(nf.recebido, 0) >= oc.total) as completa, oc.pco_enviado_em
--   from public.ordens_compra oc
--   left join (select ordem_compra_id, sum(valor) as recebido from public.notas_fiscais
--     where not cancelada group by ordem_compra_id) nf on nf.ordem_compra_id = oc.id
--   where oc.status = 'lido' and oc.pco_enviado_em is not null;
--   drop index if exists ordens_compra_aguardando_correcao;
--   alter table public.ordens_compra
--     drop column aguardando_correcao, drop column correcao_origem,
--     drop column correcao_marcada_em, drop column correcao_marcada_por;
--   delete from schema_migrations where versao = '076';
-- =============================================================================
