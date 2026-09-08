-- =============================================================================
-- 056 — Engenharia > Planejamento: RLS e permissões                     rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI, E O QUE NÃO ENTRA
--
--   As sete tabelas (`calendarios`, `feriados`, `obras`, `calendario_excecoes`,
--   `cronogramas`, `eap_nos`, `eap_dependencias`), a extensão de `clientes`
--   (`tipo`, `tenant_id`, `razao_social` etc.) e as sete rotinas
--   `PLANEJAMENTO_*` já foram aplicadas neste banco por uma frente paralela
--   (repo `Projects\frotahub-v2`, hoje abandonado — ver decisão de 08/09/2026:
--   o `baleryan` é o único caminho daqui pra frente). Rodar `create table` de
--   novo aqui quebraria em "already exists" — por isso este arquivo não recria
--   nada disso.
--
--   O que faltava, e é o que este arquivo faz:
--
--     1. RLS estava LIGADO e SEM NENHUMA POLICY nas sete tabelas — ou seja,
--        bloqueado por padrão para qualquer leitura fora do service_role
--        (seguro, mas a tela de ninguém abriria via cliente direto no Supabase).
--        Entra a policy de leitura no mesmo padrão do resto da casa.
--
--     2. `clientes` já tem a policy "cada um vê o seu cliente" (id =
--        meu_cliente_id()), que cobre a própria linha do tenant — mas NÃO
--        cobre as linhas tipo='contratante', porque o id delas não é o do
--        tenant. Entra uma segunda policy para essas.
--
--     3. Nenhuma categoria tinha nenhuma das sete rotinas marcada em
--        `categoria_permissoes` — a matriz abria vazia (é o desenho: rotina
--        nasce, e quem libera é o builder pela tela de Acesso). Libero CEO por
--        padrão, do mesmo jeito que a 044 fez para SESMT/DP — dá pra ajustar
--        depois pela tela, sem migração nova.
--
-- POR QUE `clientes.tipo='contratante'` NÃO ENTRA NO `meu_cliente_id()`
--
--   `meu_cliente_id()` lê `perfis.cliente_id`, que sempre aponta para um
--   tenant. Uma obra contratante nunca é o cliente de ninguém que loga — é
--   dado QUE o tenant cadastra. A policy de leitura, então, não usa
--   `meu_cliente_id() = id`, usa `meu_cliente_id() = tenant_id`.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Leitura das sete tabelas de planejamento — mesmo padrão de sempre:
--    cliente_id do meu tenant + rotina liberada.
-- ---------------------------------------------------------------------------

create policy "calendarios do meu cliente" on public.calendarios for select
  using (cliente_id = meu_cliente_id() and posso('PLANEJAMENTO_CALENDARIO_GERENCIAR'));

create policy "feriados do meu cliente" on public.feriados for select
  using (cliente_id = meu_cliente_id() and posso('PLANEJAMENTO_CALENDARIO_GERENCIAR'));

create policy "obras do meu cliente" on public.obras for select
  using (cliente_id = meu_cliente_id() and posso('PLANEJAMENTO_OBRAS_DADOS'));

create policy "excecoes de calendario do meu cliente" on public.calendario_excecoes for select
  using (cliente_id = meu_cliente_id() and posso('PLANEJAMENTO_CALENDARIO_GERENCIAR'));

create policy "cronogramas das obras que eu vejo" on public.cronogramas for select
  using (
    exists (select 1 from obras o where o.id = cronogramas.obra_id and o.cliente_id = meu_cliente_id())
    and posso('PLANEJAMENTO_OBRAS_DADOS')
  );

create policy "nos eap das obras que eu vejo" on public.eap_nos for select
  using (
    exists (select 1 from obras o where o.id = eap_nos.obra_id and o.cliente_id = meu_cliente_id())
    and posso('PLANEJAMENTO_OBRAS_DADOS')
  );

create policy "dependencias das obras que eu vejo" on public.eap_dependencias for select
  using (
    exists (
      select 1 from cronogramas c join obras o on o.id = c.obra_id
      where c.id = eap_dependencias.cronograma_id and o.cliente_id = meu_cliente_id()
    )
    and posso('PLANEJAMENTO_OBRAS_DADOS')
  );

-- ---------------------------------------------------------------------------
-- 2. `clientes` tipo='contratante' — segunda policy, além da que já existe.
-- ---------------------------------------------------------------------------

create policy "contratantes do meu tenant" on public.clientes for select
  using (tipo = 'contratante' and tenant_id = meu_cliente_id() and posso('PLANEJAMENTO_OBRAS_DADOS'));

-- ---------------------------------------------------------------------------
-- 3. Permissões — CEO ganha as sete de saída. Builder não precisa de linha
--    (a exceção dele já cobre, em permissao.go). TST e DP ficam de fora: são
--    categorias de SESMT/DP, sem relação com planejamento de obra.
-- ---------------------------------------------------------------------------

insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo = 'ceo' and r.codigo like 'PLANEJAMENTO_%';

insert into schema_migrations (versao, arquivo)
values ('056', '056_engenharia_planejamento.sql')
on conflict (versao) do nothing;
