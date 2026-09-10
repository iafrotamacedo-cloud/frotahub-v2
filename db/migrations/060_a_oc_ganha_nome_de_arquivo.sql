-- =============================================================================
-- 060 — Compras: a OC ganha nome de arquivo                              rev 1
-- =============================================================================
--
-- A 059 ESQUECEU A PRÓPRIA COLUNA QUE A FILA PRECISA PARA IDENTIFICAR A LINHA
--
--   `numero` só existe depois de lida (é a leitura do PDF que preenche —
--   ainda não construída, ver cabeçalho da 059). Até lá, o único jeito de
--   alguém reconhecer "essa é a OC que acabei de subir" numa lista é o nome
--   do arquivo escolhido no PC — exatamente o papel que `documentos.nome_
--   arquivo` já cumpre em Orçamentos. A 059 copiou o resto do padrão de
--   `documentos` (arquivo por sha256, máquina de status) e deixou esta de
--   fora.
--
--   Sem esta coluna, a tela de inserção (o pedido do dono em 10/09/2026: "uma
--   lista embaixo de todos os documentos que estão na fila") não tem o que
--   mostrar em cada linha antes da leitura existir.
--
-- POR QUE `NOT NULL` DIRETO, SEM VALOR-PADRÃO
--
--   Não existe nenhuma linha em `ordens_compra` em produção ainda (a 059
--   subiu sem nenhuma inserção até este momento) — a coluna nasce obrigatória
--   de uma vez, sem precisar de um valor-padrão temporário nem de um UPDATE
--   de arrumação depois.
-- =============================================================================

alter table public.ordens_compra add column nome_arquivo text not null;

comment on column public.ordens_compra.nome_arquivo is
  'O nome do arquivo escolhido na inserção (ex.: OrdemDeCompra_019566.pdf). '
  'É o que identifica a linha na fila antes da leitura preencher `numero`.';

insert into schema_migrations (versao, arquivo)
values ('060', '060_a_oc_ganha_nome_de_arquivo.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select column_name, is_nullable from information_schema.columns
--    where table_name = 'ordens_compra' and column_name = 'nome_arquivo';
--   -- esperado: is_nullable = 'NO'
--
-- PARA DESFAZER
--   alter table public.ordens_compra drop column nome_arquivo;
--   delete from public.schema_migrations where versao = '060';
-- =============================================================================
