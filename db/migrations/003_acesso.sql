-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 003: cliente, categorias e permissão por rotina
--
-- O QUE FAZ
--   Cria as quatro tabelas que respondem "quem pode o quê". Elas nascem fechadas;
--   quem as abre é a 004, depois que a `perfis` souber a que cliente pertence.
--     clientes             — a empresa titular de cada dado
--     categorias           — o grupo a que um login pertence
--     rotinas              — o catálogo do que existe no sistema
--     categoria_permissoes — a matriz: para cada categoria, quais rotinas
--
-- COMO O ACESSO FUNCIONA
--   Cada login pertence a uma categoria. A categoria tem uma lista de rotinas
--   liberadas. Abrir uma rotina significa que a sua categoria a tem marcada.
--   Existem DUAS exceções, e só duas: o builder passa sempre, e só o builder
--   cria ou edita login. Nada além disso decide acesso.
--
-- O CATÁLOGO NASCE VAZIO
--   Nenhuma rotina é cadastrada aqui. Cada rotina se registra na migration do
--   módulo que a constrói. Assim catálogo e realidade nunca divergem: não existe
--   permissão para algo que não existe.
--
-- DEPENDE DE: 001 (nivel_acesso), 002 (perfis)
-- -----------------------------------------------------------------------------

begin;

-- -----------------------------------------------------------------------------
-- 1. Clientes
--
-- Todo dado de negócio pertence a um cliente titular, e o acesso é filtrado por
-- ele (CORE-11). O filtro mora na política do banco, nunca na consulta — é isso
-- que garante que ele valha mesmo quando alguém esquece de filtrar.
-- -----------------------------------------------------------------------------
create table clientes (
  id        uuid primary key default gen_random_uuid(),
  nome      text not null,
  slug      text not null unique,        -- identificador curto, sem acento
  ativo     boolean not null default true,
  criado_em timestamptz not null default now()
);

insert into clientes (nome, slug) values ('Frota Macedo Engenharia', 'frota-macedo');


-- -----------------------------------------------------------------------------
-- 2. Categorias
--
-- O grupo a que um login pertence. Existe para não ter que marcar permissão
-- pessoa por pessoa: configura-se a categoria uma vez e todos herdam.
--
-- O NÍVEL É PROPRIEDADE DA CATEGORIA, não da pessoa. Quem entra na categoria
-- herda. Assim os dois nunca discordam.
--
-- Hoje só dois níveis têm uso: 'builder' e 'comum'. O tipo aceita outros, que
-- entram quando existir gente para eles.
-- -----------------------------------------------------------------------------
create table categorias (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete cascade,
  codigo     text not null,                    -- 'builder', 'administrativo'
  nome       text not null,                    -- como aparece na tela
  nivel      nivel_acesso not null default 'comum',
  protegida  boolean not null default false,   -- não pode ser excluída nem editada
  criado_em  timestamptz not null default now(),
  unique (cliente_id, codigo)
);

comment on column categorias.protegida is
  'Categoria de sistema: não pode ser excluída, e as permissões dela não se editam.';


-- -----------------------------------------------------------------------------
-- 3. Catálogo de rotinas
--
-- O que EXISTE no sistema. Não tem cliente: a lista de rotinas é da plataforma,
-- e é a mesma para todos.
--
-- A chave estrangeira da matriz aponta para cá — então é impossível dar permissão
-- a uma rotina inventada.
-- -----------------------------------------------------------------------------
create table rotinas (
  codigo    text primary key,        -- 'CONFIG_USUARIOS'
  nome      text not null,           -- 'Usuários e Logins'
  modulo    text not null,           -- 'Configurações'
  ordem     integer not null default 0,
  criado_em timestamptz not null default now()
);


-- -----------------------------------------------------------------------------
-- 4. A matriz
-- -----------------------------------------------------------------------------
create table categoria_permissoes (
  categoria_id uuid not null references categorias(id) on delete cascade,
  rotina       text not null references rotinas(codigo) on delete cascade,
  pode         boolean not null default true,
  primary key (categoria_id, rotina)
);


-- -----------------------------------------------------------------------------
-- 5. Fechaduras
--
-- Tudo nasce fechado (CORE-07). Aqui só se abre o catálogo de rotinas, que não
-- tem dado de cliente nenhum — é apenas a lista do que existe no sistema.
--
-- As demais aberturas ficam na 004, porque dependem de a `perfis` já saber a que
-- cliente e a que categoria cada login pertence.
-- -----------------------------------------------------------------------------
alter table clientes             enable row level security;
alter table categorias           enable row level security;
alter table rotinas              enable row level security;
alter table categoria_permissoes enable row level security;

create policy "catálogo visível a quem entrou"
  on rotinas for select to authenticated
  using (true);


insert into schema_migrations (versao, arquivo) values ('003', '003_acesso.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop table if exists categoria_permissoes, rotinas, categorias, clientes cascade;
-- delete from schema_migrations where versao = '003';
