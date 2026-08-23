-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 002: quem entra no sistema
--
-- O QUE FAZ
--   Cria a tabela `perfis`, que diz quem cada login é DENTRO do FrotaHub.
--
-- O QUE NÃO FAZ
--   Não guarda senha. Quem guarda senha é o Supabase, na tabela `auth.users`, que
--   já existe no projeto — não somos nós que a criamos. Esta tabela apenas aponta
--   para lá. Login apagado no Supabase leva o perfil junto (`on delete cascade`).
--
--   Também não cria categorias nem matriz de permissões: isso entra junto com a
--   tela de configuração de logins, que ainda não é para existir.
--
-- SEGURANÇA
--   A tabela nasce com leitura fechada (CORE-19) e ganha UMA política: cada pessoa
--   enxerga a própria linha, e mais nada. Ninguém lista os outros usuários pelo
--   navegador. Escrita não tem política nenhuma — só o servidor grava aqui.
--
-- DEPENDE DE: 001 (nivel_acesso, tocar_atualizado_em)
-- -----------------------------------------------------------------------------

begin;

create table perfis (
  id              uuid primary key references auth.users(id) on delete cascade,
  usuario         text not null unique,      -- o que a pessoa digita para entrar
  nome            text not null,             -- como aparece na tela
  nivel           nivel_acesso not null default 'comum',
  ativo           boolean not null default true,
  criado_em       timestamptz not null default now(),
  atualizado_em   timestamptz not null default now()
);

comment on table perfis is
  'Quem cada login é dentro do FrotaHub. A senha fica no Supabase (auth.users).';
comment on column perfis.usuario is
  'Nome curto de entrada. O front transforma em e-mail acrescentando o domínio.';

create trigger perfis_carimbo
  before update on perfis
  for each row execute function tocar_atualizado_em();

-- Fechadura ligada; sem política, ninguém lê nada.
alter table perfis enable row level security;

-- Única exceção: cada um lê a própria linha. É o que permite a tela mostrar o nome
-- e o nível de quem entrou, sem expor a lista de usuários a ninguém.
create policy "cada um lê o seu perfil"
  on perfis for select
  to authenticated
  using (id = auth.uid());

insert into schema_migrations (versao, arquivo) values ('002', '002_perfis.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop table if exists perfis cascade;
-- delete from schema_migrations where versao = '002';
