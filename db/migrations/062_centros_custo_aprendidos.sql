-- =============================================================================
-- 062 — Compras: o cadastro de centros de custo, alimentado sozinho     rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (12/09/2026)
--
--   "precisamos pensar agora em como alimentar o BD com centros de custos e
--    dados de faturamento que forem passando para o sistema" — e, depois de
--    perguntado se isso deveria virar um filtro novo: "na vdd, não precisa de
--    validação, só quero que o sistema guarde os que ja passam por la".
--
-- MESMO PADRÃO DE `fornecedores` (migração 059) — NÃO É CADASTRO MANUAL
--
--   `fornecedores` nasce e cresce sozinho: toda vez que uma OC é lida com um
--   fornecedor válido, `resolverFornecedor` grava (UPSERT por CNPJ) sem
--   ninguém digitar nada numa tela de cadastro. `centros_custo` segue a
--   MESMA receita — `resolverCentroCusto` grava (UPSERT por
--   `obra_centro_custo`) toda vez que uma OC lida traz um centro de custo e
--   um CNPJ de faturamento que já passou no filtro de raiz.
--
-- SÓ ARMAZENAR, DE PROPÓSITO NENHUM FILTRO NOVO NEM CHECK
--
--   O dono foi explícito: isto é referência, não bloqueio. Uma OC nunca é
--   aceita ou rejeitada por causa desta tabela — ela só registra o que JÁ
--   passou pelos filtros que existem (`leitura.go`). Por isso não tem
--   coluna "confirmado" nem trigger de validação: é o último CNPJ que
--   passou para aquele centro de custo, sem julgamento.
--
-- TROCA SEMPRE SILENCIOSA (decisão do dono)
--
--   Se o CNPJ de um centro de custo já conhecido mudar (ex.: a obra nova que
--   fatura pela matriz temporariamente e um dia ganha CNPJ próprio), o
--   UPSERT troca sem avisar ninguém — mesmo comportamento de `fornecedores`
--   hoje. Não é uma omissão: foi perguntado e respondido assim.
-- =============================================================================

create table public.centros_custo (
  id             uuid primary key default gen_random_uuid(),
  cliente_id     uuid not null references public.clientes(id),
  obra_centro_custo text not null,
  comprador_nome text,
  comprador_cnpj text,
  criado_em      timestamptz not null default now(),
  atualizado_em  timestamptz not null default now(),
  unique (cliente_id, obra_centro_custo)
);

comment on table public.centros_custo is
  'Aprendido sozinho pela leitura de OCs (mesmo padrão de fornecedores,
  migração 059) — cada centro de custo e o último CNPJ/nome de faturamento
  que passou no filtro de raiz para ele. Referência, não validação: nada
  aqui bloqueia OC nenhuma (pedido do dono, 12/09/2026). Reinserir o mesmo
  centro de custo atualiza a linha, não duplica — e a troca é sempre
  silenciosa, mesmo quando o CNPJ de um centro já conhecido muda.';

create trigger centros_custo_carimbo before update on public.centros_custo
  for each row execute function tocar_atualizado_em();

alter table public.centros_custo enable row level security;

create policy "centros de custo do meu cliente" on public.centros_custo for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_ORDENS_GERENCIAR'));

insert into schema_migrations (versao, arquivo)
values ('062', '062_centros_custo_aprendidos.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a tabela existe, RLS ligada:
--   select relname, relrowsecurity from pg_class where relname = 'centros_custo';
--   -- esperado: relrowsecurity = true
--
--   -- reinserir o mesmo centro de custo atualiza, não duplica:
--   insert into centros_custo (cliente_id, obra_centro_custo, comprador_nome, comprador_cnpj)
--     values ('<id>', 'MSL VILLAS - AQUIRAZ', 'MERCADINHOS SÃO LUIZ VILLAS', '03720882003920')
--     on conflict (cliente_id, obra_centro_custo) do update
--       set comprador_nome = excluded.comprador_nome, comprador_cnpj = excluded.comprador_cnpj;
--   -- rodar duas vezes com CNPJ diferente deve deixar só 1 linha, com o CNPJ mais novo
--
-- PARA DESFAZER
--   drop table public.centros_custo;
--   delete from public.schema_migrations where versao = '062';
-- =============================================================================
