-- =============================================================================
-- 063 — PCO: o registro de OCs excluídas/substituídas depois do envio    rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (12/09/2026)
--
--   Explicou o processo de negócio primeiro: uma OC pode ser excluída por
--   desistência da compra "a qualquer momento, mesmo com a solicitação de
--   PCO já enviada" — e isso não é problema para o cliente final, porque o
--   PCO não faz fechamento financeiro, só um gabarito ("pode sobrar, só não
--   pode faltar"). Também pode ser SUBSTITUÍDA por uma OC diferente (nota
--   veio com valor diferente, CNPJ de fornecedor diferente, reaprovação no
--   Obra Prima, etc.) — nesse caso não precisa ser o mesmo número.
--
--   Pedido: "deve haver um registro de OCs excluídas e substituídas após
--   envio. pode criar um card novo em pco só com essa lista, mas sem opção
--   de restauração, apenas dados e filtros." — e o PDF, nesse caso
--   específico, continua acessível (diferente da substituição por
--   faturamento errado, onde o arquivo é apagado de vez).
--
-- SÓ DEPOIS DO ENVIO — ANTES DISSO CONTINUA MORRENDO SEM RASTRO
--
--   Exclusão/substituição de uma OC que nunca chegou a ser enviada (fila,
--   Processadas, Rejeitadas, PCO Pendentes) não grava nada aqui — seguem a
--   regra já combinada (`apagarOrdemDeVez`, migração/commit de 11/09): o ID
--   morre ali, sem exigir nota fiscal nenhuma pra dar continuidade. Esta
--   tabela é só para o caso em que o cliente JÁ recebeu o pedido de PCO.
--
-- É RETRATO, NÃO REFERÊNCIA VIVA
--
--   Fornecedor e faturamento são copiados como texto (não FK) de propósito:
--   é uma fotografia do que a OC era no momento em que saiu — não deve
--   mudar se o cadastro do fornecedor mudar depois. `substituta_id` aponta
--   pra OC nova quando existir, mas com `on delete set null` — se um dia
--   aquela OC nova também for excluída, o retrato não quebra, só perde o
--   link.
-- =============================================================================

create table public.ordens_compra_canceladas (
  id                 uuid primary key default gen_random_uuid(),
  cliente_id         uuid not null references public.clientes(id),
  tipo               text not null check (tipo in ('excluida','substituida')),

  -- retrato da OC removida, no momento em que saiu
  numero             text,
  obra_centro_custo  text,
  comprador_nome     text,
  comprador_cnpj     text,
  fornecedor_nome    text,
  fornecedor_cnpj    text,
  valor              numeric(14,2),
  nome_arquivo       text,
  arquivo_sha256     text references public.arquivos(sha256) on delete restrict,

  enviado_em         timestamptz,  -- o pco_enviado_em original
  removida_em        timestamptz not null default now(),
  removida_por       uuid references public.perfis(id),

  -- só quando tipo = 'substituida'
  substituta_numero  text,
  substituta_id      uuid references public.ordens_compra(id) on delete set null
);

comment on table public.ordens_compra_canceladas is
  'Retrato de uma OC excluída ou substituída DEPOIS de já ter sido enviada
  por PCO — nunca antes disso (aí ela só some, sem rastro, regra já
  combinada). Não é fila de trabalho: só leitura, sem opção de restaurar
  (pedido do dono, 12/09/2026). O PDF continua acessível — diferente da
  substituição por faturamento errado, este caso não apaga o arquivo.';

create index ordens_compra_canceladas_cliente_data
  on public.ordens_compra_canceladas (cliente_id, removida_em desc);

alter table public.ordens_compra_canceladas enable row level security;

create policy "OCs canceladas do meu cliente" on public.ordens_compra_canceladas for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_ORDENS_GERENCIAR'));

insert into schema_migrations (versao, arquivo)
values ('063', '063_ocs_excluidas_e_substituidas_apos_envio.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a tabela existe, RLS ligada:
--   select relname, relrowsecurity from pg_class
--    where relname = 'ordens_compra_canceladas';
--   -- esperado: relrowsecurity = true
--
--   -- apagar a OC substituta não derruba o retrato (só solta o link):
--   -- delete from ordens_compra where id = '<substituta_id de um retrato>';
--   -- select substituta_id from ordens_compra_canceladas where id = '<id>';
--   -- esperado: null, a linha do retrato continua existindo
--
-- PARA DESFAZER
--   drop index if exists ordens_compra_canceladas_cliente_data;
--   drop table public.ordens_compra_canceladas;
--   delete from public.schema_migrations where versao = '063';
-- =============================================================================
