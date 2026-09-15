-- =============================================================================
-- 071 — Paginas da NF + leitura ERA READ congelada
-- =============================================================================
--
-- Uma nota pode ter varias paginas escaneadas (DANFE de duas folhas, etc.).
-- A pagina 1 continua em notas_fiscais.arquivo_sha256; as seguintes vao na
-- tabela abaixo. Cada pagina guarda o JSON leitura_era do ERA READ no dia
-- do escaneamento.

alter table public.notas_fiscais
  add column if not exists leitura_era jsonb;

comment on column public.notas_fiscais.leitura_era is
  'Leitura ERA READ da primeira pagina, congelada no recebimento.';

create table if not exists public.notas_fiscais_paginas (
  id               uuid primary key default gen_random_uuid(),
  cliente_id       uuid not null references public.clientes(id),
  nota_fiscal_id   uuid not null references public.notas_fiscais(id) on delete cascade,
  pagina           int not null check (pagina >= 2),
  arquivo_sha256   text not null references public.arquivos(sha256) on delete restrict,
  leitura_era      jsonb,
  criado_em        timestamptz not null default now(),
  unique (nota_fiscal_id, pagina)
);

comment on table public.notas_fiscais_paginas is
  'Paginas 2+ de uma nota fiscal — cada escaneamento salvo de uma vez.';

create index if not exists notas_fiscais_paginas_nota
  on public.notas_fiscais_paginas (nota_fiscal_id);

alter table public.notas_fiscais_paginas enable row level security;

create policy "paginas nf do meu cliente" on public.notas_fiscais_paginas for select
  using (cliente_id = (select cliente_id from public.perfis where id = auth.uid()));
