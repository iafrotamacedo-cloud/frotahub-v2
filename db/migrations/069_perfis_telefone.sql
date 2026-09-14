-- =============================================================================
-- 069 — perfis ganha telefone (15/09/2026)                                rev 1
-- =============================================================================
--
-- Pedido do dono: poder guardar o telefone de quem tem login, junto do resto
-- do cadastro (Usuários e Logins). Texto livre, sem obrigar formato nenhum —
-- mesma convenção de `fornecedores.telefone` (migração 059): quem digita sabe
-- melhor do que uma regex qual DDD ou extensão faz sentido pro próprio
-- telefone, e uma máscara rígida rejeitaria número de fora do Brasil sem
-- necessidade nenhuma.
begin;

alter table public.perfis add column telefone text;

insert into schema_migrations (versao, arquivo)
values ('069', '069_perfis_telefone.sql')
on conflict (versao) do nothing;

commit;

-- =============================================================================
-- COMO CONFERIR
--   select column_name, is_nullable from information_schema.columns
--    where table_name = 'perfis' and column_name = 'telefone';
--   -- esperado: is_nullable = 'YES'
--
-- COMO DESFAZER
--   begin;
--   alter table public.perfis drop column telefone;
--   delete from schema_migrations where versao = '069';
--   commit;
-- =============================================================================
