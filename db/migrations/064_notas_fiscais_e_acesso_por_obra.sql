-- =============================================================================
-- 064 — Notas Fiscais: o Bloco A (recebimento) e o acesso por obra       rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (12/09/2026)
--
--   Depois de fechar o filtro/reparo de faturamento, começou a próxima fase:
--   toda OC que anda o ciclo inteiro (chega a "Enviados" e não é excluída)
--   vira uma nota fiscal. Descreveu o kanban da nota — recebida pelo
--   almoxarife na obra (mobile, com foto) → entregue no escritório
--   (autenticado por login) → enviada ao cliente (malote, marcação manual)
--   — e confirmou dois pontos que moldam esta migração:
--
--   1. "não é permitido uma nota fiscal para várias OCs, mas é permitido o
--      recebimento parcial, com várias NFs... para uma mesma OC" — então
--      `notas_fiscais.ordem_compra_id` é obrigatório e único-por-nota, e o
--      estado da OC (aguardando/parcial/completa) é CALCULADO pela soma das
--      notas, não uma coluna gravada — evita as duas fontes discordarem.
--
--   2. O almoxarife só pode agir nas obras que alguém de nível superior
--      liberou pra ele — "deve haver uma configuração onde auths de nível
--      superior decidem quais obras ele pode ver/editar". `centros_custo`
--      (migração 062, hoje travada em modo teste — `aprenderCentrosCusto`
--      em ler.go) já é "a lista oficial de obras que o sistema conhece",
--      então o vínculo é `perfil ↔ centros_custo`, não um cadastro à parte.
--
--   O "Bloco B" (documento de Protocolo — numeração, agrupamento por dia,
--   vencimento/forma de pagamento) fica pra depois, por pedido explícito do
--   dono ("foca no A") — por isso `notas_fiscais` não tem `protocolo_id`
--   nem campo de pagamento nenhum aqui.
--
-- POR QUE A NOTA NÃO TEM STATUS "AGUARDANDO"
--
--   Uma linha em `notas_fiscais` só existe a partir do momento em que o
--   almoxarife escaneia — antes disso não há nota, há só uma OC esperando.
--   Por isso o `check` de status começa em `recebida`, não antes.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- notas_fiscais — o retrato de cada NF recebida, uma por linha
-- -----------------------------------------------------------------------------

create table public.notas_fiscais (
  id                       uuid primary key default gen_random_uuid(),
  cliente_id               uuid not null references public.clientes(id),
  ordem_compra_id          uuid not null references public.ordens_compra(id) on delete restrict,

  numero                   text not null,
  valor                    numeric(14,2) not null check (valor > 0),
  arquivo_sha256           text references public.arquivos(sha256) on delete restrict,

  status                   text not null default 'recebida'
                             check (status in ('recebida','entregue_escritorio','enviada_cliente')),
  cancelada                boolean not null default false,
  motivo_cancelamento      text,

  recebida_em              timestamptz not null default now(),
  recebida_por             uuid references public.perfis(id),
  entregue_escritorio_em   timestamptz,
  entregue_confirmado_por  uuid references public.perfis(id),
  enviada_cliente_em       timestamptz,
  enviada_confirmado_por   uuid references public.perfis(id),

  criado_em                timestamptz not null default now(),
  atualizado_em             timestamptz not null default now()
);

comment on table public.notas_fiscais is
  'Cada nota fiscal recebida contra uma OC — várias por OC são permitidas
  (recebimento parcial), uma nota nunca cobre mais de uma OC. Nasce sempre
  com status "recebida" (é o almoxarife escaneando na hora) e anda sozinha
  pela própria jornada até "enviada_cliente" (malote), sem esperar a OC
  fechar. "cancelada" cobre o caso real de fornecedor cancelar a NF depois
  de já recebida (visto num protocolo de verdade, 12/09/2026).';

create index notas_fiscais_ordem_compra
  on public.notas_fiscais (ordem_compra_id) where not cancelada;

create index notas_fiscais_status
  on public.notas_fiscais (cliente_id, status) where not cancelada;

create trigger notas_fiscais_carimbo before update on public.notas_fiscais
  for each row execute function tocar_atualizado_em();

-- -----------------------------------------------------------------------------
-- centro_custo_acessos — quais obras cada perfil pode receber NF
-- -----------------------------------------------------------------------------

create table public.centro_custo_acessos (
  id               uuid primary key default gen_random_uuid(),
  cliente_id       uuid not null references public.clientes(id),
  perfil_id        uuid not null references public.perfis(id) on delete cascade,
  centro_custo_id  uuid not null references public.centros_custo(id) on delete cascade,
  concedido_por    uuid references public.perfis(id),
  criado_em        timestamptz not null default now(),
  unique (perfil_id, centro_custo_id)
);

comment on table public.centro_custo_acessos is
  'Vínculo perfil ↔ obra — só quem tem uma linha aqui (concedida por alguém
  de nível superior, tela de configuração própria) pode receber nota fiscal
  para aquela obra. Depende de `centros_custo` estar alimentada de verdade
  (hoje travada em `aprenderCentrosCusto = false`, ler.go) — sem isso, não
  há obra nenhuma pra conceder.';

alter table public.notas_fiscais enable row level security;
alter table public.centro_custo_acessos enable row level security;

create policy "notas fiscais do meu cliente" on public.notas_fiscais for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_ORDENS_GERENCIAR'));

create policy "acessos por obra do meu cliente" on public.centro_custo_acessos for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_ORDENS_GERENCIAR'));

-- -----------------------------------------------------------------------------
-- Rotinas novas
-- -----------------------------------------------------------------------------
--
--   COMPRAS_NF_RECEBER: só o almoxarife — a rotina estreita que o dono
--   pediu ("permissão específica apenas para as responsabilidades dele").
--   Não é concedida a nenhuma categoria por esta migração de propósito: o
--   dono ainda vai criar a categoria "Almoxarife" pela tela de Categorias
--   que já existe, do mesmo jeito que a 059 deixou pro comprador.
--
--   COMPRAS_NF_ENTREGAR: confirmar entrega física no escritório + marcar
--   envio ao cliente (malote) — perfil de escritório/compras, não a obra.
--
--   COMPRAS_NF_CONFIGURAR_ACESSO: conceder/revogar obra a um perfil — "auth
--   de nível superior", por isso só CEO por padrão (mesmo espírito da
--   rotina de Destinatários do PCO, migração 061: rotina normal, não
--   soBuilder, mas concedida com mais cuidado).
insert into public.rotinas (codigo, nome, modulo, ordem) values
  ('COMPRAS_NF_RECEBER', 'Notas fiscais — receber (almoxarife)', 'compras', 4),
  ('COMPRAS_NF_ENTREGAR', 'Notas fiscais — confirmar entrega e envio', 'compras', 5),
  ('COMPRAS_NF_CONFIGURAR_ACESSO', 'Notas fiscais — configurar acesso por obra', 'compras', 6);

insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo = 'ceo' and r.codigo in ('COMPRAS_NF_ENTREGAR', 'COMPRAS_NF_CONFIGURAR_ACESSO');

insert into schema_migrations (versao, arquivo)
values ('064', '064_notas_fiscais_e_acesso_por_obra.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- as duas tabelas existem, RLS ligada:
--   select relname, relrowsecurity from pg_class
--    where relname in ('notas_fiscais','centro_custo_acessos');
--   -- esperado: relrowsecurity = true nas duas
--
--   -- as três rotinas estão no catálogo, e o CEO já tem duas delas:
--   select r.codigo, cp.pode from public.rotinas r
--     left join public.categoria_permissoes cp
--       on cp.rotina = r.codigo and cp.categoria_id = (select id from categorias where codigo='ceo')
--    where r.codigo like 'COMPRAS_NF_%';
--   -- esperado: COMPRAS_NF_RECEBER com pode=null (ninguém tem ainda, de
--   -- propósito), os outros dois com pode=true
--
-- PARA DESFAZER
--   drop table public.centro_custo_acessos;
--   drop index if exists notas_fiscais_status;
--   drop index if exists notas_fiscais_ordem_compra;
--   drop table public.notas_fiscais;
--   delete from public.categoria_permissoes where rotina like 'COMPRAS_NF_%';
--   delete from public.rotinas where codigo like 'COMPRAS_NF_%';
--   delete from public.schema_migrations where versao = '064';
-- =============================================================================
