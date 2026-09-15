-- =============================================================================
-- 070 — Notas Fiscais: fotos do material recebido (15/09/2026)            rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU
--
--   Na tela de receber NF (mobile, almoxarife na obra), separar "escanear a
--   nota" de "fotografar o material que chegou" — são duas evidências
--   diferentes, e uma nota pode vir com várias fotos do material. Só câmera
--   nas duas, nada de escolher arquivo da galeria (por enquanto).
--
-- POR QUE UMA TABELA NOVA, E NÃO UM ARRAY EM `notas_fiscais`
--
--   `notas_fiscais.arquivo_sha256` é uma referência só, porque a nota é UM
--   documento. As fotos do material são VÁRIAS, e cada uma já é um arquivo
--   próprio guardado em `arquivos` (dedup por sha256, mesma receita de
--   `guardarArquivoNF`) — uma linha por foto é o mesmo desenho de qualquer
--   relação um-para-muitos deste banco, e evita a UPDATE-em-array que
--   `notas_fiscais_carimbo` (trigger de `atualizado_em`) nunca precisaria
--   disparar de verdade.
begin;

create table public.notas_fiscais_fotos_material (
  id               uuid primary key default gen_random_uuid(),
  cliente_id       uuid not null references public.clientes(id),
  nota_fiscal_id   uuid not null references public.notas_fiscais(id) on delete cascade,
  arquivo_sha256   text not null references public.arquivos(sha256) on delete restrict,
  criado_em        timestamptz not null default now()
);

comment on table public.notas_fiscais_fotos_material is
  'Fotos do material recebido, tiradas pelo almoxarife ao lado da nota
  fiscal — evidência do que chegou, separada do documento da nota em si.
  Zero ou várias por nota fiscal; apagar a nota (on delete cascade) apaga
  as linhas daqui, mas nunca o arquivo em si (arquivo_sha256 é restrict,
  igual a notas_fiscais.arquivo_sha256).';

create index notas_fiscais_fotos_material_nota
  on public.notas_fiscais_fotos_material (nota_fiscal_id);

alter table public.notas_fiscais_fotos_material enable row level security;

create policy "fotos de material do meu cliente" on public.notas_fiscais_fotos_material for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_ORDENS_GERENCIAR'));

insert into schema_migrations (versao, arquivo)
values ('070', '070_nf_fotos_material.sql')
on conflict (versao) do nothing;

commit;

-- =============================================================================
-- COMO CONFERIR
--   select relname, relrowsecurity from pg_class
--    where relname = 'notas_fiscais_fotos_material';
--   -- esperado: relrowsecurity = true
--
-- COMO DESFAZER
--   begin;
--   drop table public.notas_fiscais_fotos_material;
--   delete from schema_migrations where versao = '070';
--   commit;
-- =============================================================================
