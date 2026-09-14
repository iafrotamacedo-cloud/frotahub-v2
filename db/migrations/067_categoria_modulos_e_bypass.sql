-- =============================================================================
-- 067 — o acesso "tudo do módulo" que o Builder libera pro CEO             rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (14/09/2026)
--
--   Segunda peça da hierarquia de 5 níveis (a primeira foi a 066, o rename do
--   enum): o CEO ganha acesso total a todo MÓDULO que o Builder liberar pra
--   ele — não rotina por rotina, como todo mundo hoje. "Módulo" aqui são as
--   5-6 grandes seções do menu (Administrativo, Manutenção, Engenharia,
--   SESMT/DP, Configurações), não o `modulo` mais fino que já existe em
--   `rotinas` (que hoje vale `compras`/`manutencao`/`planejamento`/`rh` — só
--   `manutencao` bate com uma seção do menu; por isso esta migração cria uma
--   coluna SEPARADA, em vez de reaproveitar essa).
--
--   E é bypass DE VERDADE — quem tem o módulo liberado não passa pela matriz
--   `categoria_permissoes` pra essas rotinas. Mas com uma trava: tudo que for
--   NOVO no catálogo nasce OCULTO pro bypass, mesmo dentro de um módulo já
--   liberado — o Builder precisa liberar rotina por rotina, na primeira vez
--   que ela aparece. Só o módulo já aberto não é carta branca pro futuro.
--
-- A FONTE ÚNICA DA VERDADE
--
--   Hoje "o que esta categoria alcança" tem UM lugar: `categoria_permissoes`.
--   A partir de agora tem duas fontes que precisam concordar sempre — a
--   matriz de sempre, e o bypass de módulo — por isso nasce uma VIEW,
--   `categoria_rotinas_efetivas`, que é a união das duas. Ela substitui
--   `categoria_permissoes` em TODO lugar que hoje pergunta "esta categoria
--   tem esta rotina?": `useSessao.ts` (o menu, lido direto do Supabase),
--   `permissao.go` (o motor, que decide de verdade — P-29) e a função
--   `posso()` (usada em política de RLS de 14 migrations — sem atualizar
--   ela também, o bypass funcionaria no menu e no motor mas ficaria
--   INVISÍVEL pra qualquer leitura direta do Supabase protegida por RLS,
--   furo sutil, não só inconsistência visual).
--
--   Builder continua bypassando as três do jeito de sempre — esta migração
--   não mexe nisso, só troca o segundo `exists(...)` de cada uma.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. `rotinas` ganha onde ela mora no MENU, e se já está liberada pro bypass
-- -----------------------------------------------------------------------------

alter table public.rotinas add column modulo_menu text;
alter table public.rotinas add column liberada_para_bypass boolean not null default false;

comment on column public.rotinas.modulo_menu is
  'Em qual das 5-6 grandes seções do menu esta rotina vive, pro bypass de
  módulo do CEO — vocabulário fechado, NADA a ver com a coluna `modulo`
  (essa é só agrupamento visual da tela de Permissões, mais fina).';
comment on column public.rotinas.liberada_para_bypass is
  'Falso até o Builder decidir o contrário. Módulo liberado NÃO basta: cada
  rotina, uma vez, precisa ser marcada aqui — é o "nasce oculto" valendo pra
  sempre, mesmo dentro de módulo já aberto.';

-- Backfill das 29 rotinas de hoje, pela tabela de correspondência que o dono
-- confirmou — `compras`→administrativo, `manutencao`→manutencao (só esta
-- coincide de nome), `planejamento`→engenharia, `rh`→sesmt-dp. Nada mapeia
-- pra `configuracoes` ainda — não existe rotina de lá hoje.
update public.rotinas set modulo_menu = case modulo
  when 'compras'      then 'administrativo'
  when 'manutencao'   then 'manutencao'
  when 'planejamento' then 'engenharia'
  when 'rh'           then 'sesmt-dp'
end;

do $$
declare sem_modulo int;
begin
  select count(*) into sem_modulo from public.rotinas where modulo_menu is null;
  if sem_modulo > 0 then
    raise exception 'Existem % rotinas sem modulo_menu — o `case` do backfill não cobriu tudo.', sem_modulo;
  end if;
end $$;

alter table public.rotinas alter column modulo_menu set not null;
alter table public.rotinas add constraint rotinas_modulo_menu_valido
  check (modulo_menu in ('administrativo', 'manutencao', 'engenharia', 'sesmt-dp', 'configuracoes'));

-- -----------------------------------------------------------------------------
-- 2. `categoria_modulos_liberados` — quais módulos cada categoria CEO recebeu
--
-- Presença de linha = liberado, mesmo idioma de `centro_custo_acessos` (064).
-- Só o Builder escreve aqui (é ele quem libera módulo, não o próprio CEO) —
-- a escrita passa pelo motor com a chave de serviço, como sempre.
-- -----------------------------------------------------------------------------

create table public.categoria_modulos_liberados (
  id           uuid primary key default gen_random_uuid(),
  cliente_id   uuid not null references public.clientes(id),
  categoria_id uuid not null references public.categorias(id) on delete cascade,
  modulo       text not null
    check (modulo in ('administrativo', 'manutencao', 'engenharia', 'sesmt-dp', 'configuracoes')),
  liberado_por uuid references public.perfis(id),
  criado_em    timestamptz not null default now(),
  unique (categoria_id, modulo)
);

comment on table public.categoria_modulos_liberados is
  'Um módulo liberado pro Builder = bypass de verdade nas rotinas daquele
  módulo que já estiverem `liberada_para_bypass` (ver categoria_rotinas_efetivas).';

alter table public.categoria_modulos_liberados enable row level security;

-- Só builder/ceo enxergam esta configuração — não é dado de trabalho, é
-- controle de acesso, do mesmo jeito que só quem tem
-- COMPRAS_NF_CONFIGURAR_ACESSO vê `centro_custo_acessos`. Aqui não há
-- rotina natural pra isso (é conceito de NÍVEL, não de rotina), por isso
-- a função de apoio é sobre nível, não sobre `posso()`.
create or replace function sou_builder_ou_ceo()
returns boolean language sql stable
as $$
  select exists(
    select 1 from public.perfis p join public.categorias c on c.id = p.categoria_id
     where p.id = auth.uid() and c.nivel in ('builder', 'ceo')
  )
$$;

create policy "builder ou ceo vê os módulos liberados do cliente"
  on public.categoria_modulos_liberados for select to authenticated
  using (cliente_id = meu_cliente_id() and sou_builder_ou_ceo());

-- -----------------------------------------------------------------------------
-- 3. A view — fonte única de "o que esta categoria alcança de verdade"
-- -----------------------------------------------------------------------------

create view public.categoria_rotinas_efetivas
with (security_invoker = true) as
select cp.categoria_id, cp.rotina
  from public.categoria_permissoes cp
 where cp.pode
union
select cml.categoria_id, r.codigo as rotina
  from public.categoria_modulos_liberados cml
  join public.rotinas r
    on r.modulo_menu = cml.modulo
   and r.liberada_para_bypass;

comment on view public.categoria_rotinas_efetivas is
  'A UNIÃO de duas formas de uma categoria alcançar uma rotina: marcada direto
  na matriz (categoria_permissoes), ou herdada pelo bypass de módulo do CEO
  (categoria_modulos_liberados + rotinas.liberada_para_bypass). Todo consumidor
  de "esta categoria tem esta rotina" lê DAQUI, nunca mais direto de
  categoria_permissoes — useSessao.ts, permissao.go e a função posso() abaixo.';

-- -----------------------------------------------------------------------------
-- 4. `posso()` passa a consultar a view, não a matriz direto
--
-- Mesma assinatura, mesmo comportamento pro builder — só o segundo
-- `exists(...)` muda de alvo.
-- -----------------------------------------------------------------------------

create or replace function posso(codigo text)
returns boolean language sql stable
as $$
  select
    exists (
      select 1 from perfis p
        join categorias c on c.id = p.categoria_id
       where p.id = auth.uid() and c.nivel = 'builder'
    )
    or exists (
      select 1 from categoria_rotinas_efetivas cre
       where cre.categoria_id = minha_categoria_id()
         and cre.rotina = codigo
    );
$$;

comment on function posso(text) is
  'Verdadeiro se o login atual alcança a rotina — direto pela matriz, ou pelo
  bypass de módulo do CEO. O builder passa sempre.';

insert into schema_migrations (versao, arquivo)
values ('067', '067_categoria_modulos_e_bypass.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select modulo_menu, liberada_para_bypass, count(*)
--     from rotinas group by 1, 2 order by 1;
--   -- as 29 rotinas de hoje têm modulo_menu preenchido, liberada_para_bypass
--   -- todas em falso
--
--   -- provar o bypass: liberar Administrativo pra uma categoria de teste e
--   -- marcar uma rotina como elegível, depois conferir que ela aparece em
--   -- categoria_rotinas_efetivas sem estar em categoria_permissoes
--   insert into categoria_modulos_liberados (cliente_id, categoria_id, modulo)
--     select cliente_id, id, 'administrativo' from categorias where codigo = 'ceo';
--   update rotinas set liberada_para_bypass = true where codigo = 'COMPRAS_ORDENS_GERENCIAR';
--   select * from categoria_rotinas_efetivas cre
--     join categorias c on c.id = cre.categoria_id
--    where c.codigo = 'ceo' and cre.rotina = 'COMPRAS_ORDENS_GERENCIAR';
--   -- espera 1 linha, mesmo sem nenhuma linha correspondente em categoria_permissoes
--
-- PARA DESFAZER
--   drop function if exists posso(text);
--   -- (recriar a versão da 007/064, que lia categoria_permissoes direto)
--   drop view if exists categoria_rotinas_efetivas;
--   drop table if exists categoria_modulos_liberados;
--   drop function if exists sou_builder_ou_ceo();
--   alter table rotinas drop constraint if exists rotinas_modulo_menu_valido;
--   alter table rotinas drop column if exists modulo_menu;
--   alter table rotinas drop column if exists liberada_para_bypass;
--   delete from schema_migrations where versao = '067';
-- =============================================================================
