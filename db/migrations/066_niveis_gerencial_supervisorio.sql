-- =============================================================================
-- 066 — nivel_acesso: gerente vira gerencial, comum vira operacional,       rev 1
--        e entra supervisório entre os dois
-- =============================================================================
--
-- O QUE O DONO PEDIU (14/09/2026)
--
--   Depois de uma conversa inteira definindo a hierarquia de verdade do
--   FrotaHub, ficaram 5 níveis, não mais 4: Builder (como está), CEO
--   (controlado só pelo builder), Gerencial — ex-"Gerente" — (controlado
--   pelo CEO), Supervisório — nível novo — (controlado pelo Gerencial) e
--   Operacional — ex-"Comum" — (não gerencia ninguém, vínculo pode vir de
--   qualquer nível acima, não só do vizinho direto).
--
--   Esta migração só troca os RÓTULOS do enum e acrescenta o nível novo —
--   nenhuma linha muda de categoria, nenhum dado é reescrito. É rename de
--   catálogo, não um `update`: uma categoria hoje marcada "gerente" passa a
--   se chamar "gerencial" sozinha, sem tocar `categoria_id` de ninguém.
--
--   De propósito SOZINHA nesta migração, sem mexer em mais nada: é a única
--   parte de toda a mudança de hierarquia que toca dado de produção já
--   existente (por relabeling, não por escrita). O resto — bypass de módulo
--   do CEO, vínculo hierárquico — vem em migrações separadas depois que este
--   rename estiver no ar e confirmado.
--
--   Confirmado ao vivo antes de rodar: existe hoje 1 categoria `builder`, 1
--   `ceo`, 2 `comum` e 0 `gerente` — o rename não deixa ninguém "gerente"
--   pra trás.
--
-- SOBRE A ORDEM DAS PALAVRAS NO ENUM
--
--   `supervisorio` entra fisicamente depois de `gerencial` no catálogo do
--   Postgres só por legibilidade (`\dT+ nivel_acesso` lê na ordem certa) —
--   nada no código hoje compara nível por ordem (`<`, `>`, `order by nivel`),
--   então isso não é uma garantia de comportamento, só organização.
--
-- REGRA DO POSTGRES QUE ESTA MIGRAÇÃO RESPEITA
--
--   Um valor de enum recém-criado (`ADD VALUE`) não pode ser usado em
--   `insert`/`update` na MESMA transação em que nasceu. Esta migração não
--   insere nem atualiza nenhuma linha usando `'supervisorio'` — só cria o
--   rótulo. Nenhuma migração futura deve tentar usar `'supervisorio'` num
--   `insert`/`update` misturado com um `ALTER TYPE ... ADD VALUE` no mesmo
--   arquivo.
-- =============================================================================

alter type public.nivel_acesso rename value 'gerente' to 'gerencial';
alter type public.nivel_acesso rename value 'comum' to 'operacional';
alter type public.nivel_acesso add value 'supervisorio' after 'gerencial';

insert into schema_migrations (versao, arquivo)
values ('066', '066_niveis_gerencial_supervisorio.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select unnest(enum_range(null::nivel_acesso))::text;
--   -- espera: builder, ceo, gerencial, supervisorio, operacional
--   select nivel, count(*) from categorias group by nivel order by 1;
--   -- as categorias que eram 'gerente'/'comum' aparecem já como
--   -- 'gerencial'/'operacional', mesma contagem de antes
--
-- PARA DESFAZER
--   alter type public.nivel_acesso rename value 'gerencial' to 'gerente';
--   alter type public.nivel_acesso rename value 'operacional' to 'comum';
--   -- 'supervisorio' NÃO tem como ser removido do enum (Postgres não tem
--   -- `DROP VALUE`) — se nenhuma linha tiver usado ainda, fica inofensivo,
--   -- só um rótulo sobrando no catálogo.
--   delete from public.schema_migrations where versao = '066';
-- =============================================================================
