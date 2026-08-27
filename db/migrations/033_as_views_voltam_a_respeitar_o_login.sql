-- =============================================================================
-- 033 — as views voltam a respeitar quem está perguntando          ⚠ URGENTE
-- =============================================================================
--
-- O QUE ESTÁ ACONTECENDO AGORA, MEDIDO EM 27/08/2026
--
--   Rodando como `anon` — o papel de quem NÃO fez login — dentro do próprio
--   banco:
--
--     set local role anon;
--     select (select count(*) from documentos)        -- 0    ✅ a tabela protege
--          , (select count(*) from documentos_lista)  -- 91   ❌ a view entrega
--          , (select count(*) from orcamentos)        -- 0    ✅
--          , (select count(*) from orcamentos_lista)  -- 703  ❌
--          , (select count(*) from orcamentos_painel) -- 1    ❌
--
--   As tabelas estão trancadas. As VIEWS não. E a chave `publishable` do
--   Supabase viaja dentro do JavaScript do site — ela é pública por natureza.
--   Ou seja: hoje, qualquer pessoa com o endereço do banco e essa chave lê o
--   número, o fornecedor, o ticket, a loja e o VALOR de toda nota e de todo
--   orçamento, e o painel financeiro inteiro. Sem login.
--
-- POR QUE ISSO ACONTECEU — E NÃO FOI DESCUIDO DE UMA PESSOA SÓ
--
--   A 010, a 015, a 016 e a 017 criaram as views com a cláusula certa:
--
--     create view ... with (security_invoker = true) as ...
--
--   `security_invoker` manda a view rodar com os privilégios de QUEM PERGUNTA,
--   e não de quem a criou. Sem ela, a view roda como a dona das tabelas — que,
--   sendo dona, não é filtrada por política nenhuma.
--
--   A armadilha: **`create or replace view` NÃO preserva essa cláusula.** Ele
--   substitui as opções da view pelo que vier no `with`; sem `with`, elas
--   voltam ao padrão, que é `security_invoker = false`.
--
--   Da 019 em diante, TODA reescrita dessas três views veio sem a cláusula —
--   019, 020, 022, 023, 028, 029, 030, 031. Cada uma dessas migrações estava
--   mexendo noutra coisa (uma coluna nova, uma expressão), e a tranca caiu
--   junto, calada. Nenhum erro, nenhum aviso: a view continuou respondendo
--   normalmente, só que para todo mundo.
--
--   Repare que `chamados_lista` e `faturas_lista` escaparam — não porque
--   alguém as protegeu, mas porque ninguém precisou reescrevê-las depois.
--
-- POR QUE `alter view`, E NÃO REESCREVER AS VIEWS
--
--   Reescrever as quatro exigiria reproduzir centenas de linhas de `select`
--   VERBATIM, e um `create or replace view` que erre nome, ordem ou tipo de
--   uma coluna é recusado — ou, pior, passa e muda a tela. `alter view ... set`
--   mexe SÓ na opção, sem tocar no corpo. É a mudança mínima que resolve.
-- =============================================================================

alter view documentos_lista          set (security_invoker = true);
alter view orcamentos_lista          set (security_invoker = true);
alter view orcamentos_painel         set (security_invoker = true);
alter view pedidos_faturamento_lista set (security_invoker = true);

-- A view de pedidos ainda não vazou porque a tabela está vazia. Entra aqui
-- junto: esperar ela ter dado para então protegê-la é escolher o pior momento.

comment on view documentos_lista is
  'A lista das notas. RODA COMO QUEM PERGUNTA (security_invoker) — se você '
  'reescrever esta view, REPITA a cláusula `with (security_invoker = true)`, '
  'senão a tranca cai em silêncio. Ver migração 033.';
comment on view orcamentos_lista is
  'A lista dos orçamentos. RODA COMO QUEM PERGUNTA — repita '
  '`with (security_invoker = true)` em qualquer reescrita. Ver migração 033.';
comment on view orcamentos_painel is
  'Os contadores do painel. RODA COMO QUEM PERGUNTA — repita '
  '`with (security_invoker = true)` em qualquer reescrita. Ver migração 033.';

-- -----------------------------------------------------------------------------
-- E, DE QUEBRA, A TABELA QUE DIZ O QUE JÁ RODOU VOLTA A DIZER A VERDADE
-- -----------------------------------------------------------------------------
--
--   A 019 rodou: a coluna `duplicada_de` e os três índices dela estão no banco,
--   conferidos hoje. O que ela NUNCA fez foi escrever a própria linha aqui —
--   o arquivo dela não tem o `insert` do rodapé. Resultado: `schema_migrations`
--   pula do 018 para o 020, e a tabela que existe para responder "o que já
--   rodou?" responde errado.
--
--   Escrever a linha agora é registrar um fato, não fingir um. A migração
--   aconteceu; só o recibo faltava.
insert into schema_migrations (versao, arquivo)
values ('019', '019_nota_repetida.sql')
on conflict (versao) do nothing;

insert into schema_migrations (versao, arquivo)
values ('033', '033_as_views_voltam_a_respeitar_o_login.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR — e esta conferência vale a pena rodar de verdade
--
--   -- 1) as seis views têm que aparecer com a opção ligada:
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relnamespace = 'public'::regnamespace
--    order by relname;
--   -- esperado: TODAS com {security_invoker=true}
--
--   -- 2) a prova pelo lado de quem não fez login:
--   set local role anon;
--   select (select count(*) from documentos_lista)  as notas,
--          (select count(*) from orcamentos_lista)  as orcamentos,
--          (select count(*) from orcamentos_painel) as painel;
--   reset role;
--   -- esperado agora: 0, 0, 0
--
--   -- 3) e a lista de migrações não pula mais número:
--   select versao from schema_migrations order by versao;
--   -- esperado: 001..019, 020..033, sem buracos
--
--   -- 4) e o sistema continua funcionando: entre no site e abra Notas e DAVs.
--   --    O motor usa a chave de serviço e não é afetado; o que muda é o que o
--   --    NAVEGADOR alcança direto.
--
-- PARA DESFAZER (não faça — isto reabre o vazamento)
--   alter view documentos_lista set (security_invoker = false);
-- =============================================================================
