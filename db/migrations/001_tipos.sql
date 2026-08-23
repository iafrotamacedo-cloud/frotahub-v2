-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 001: controle de migrations e tipos
--
-- O QUE FAZ
--   Cria o registro das próprias migrations e a lista de níveis de acesso.
--   Não cria tabela de dados — isso começa na 002.
--
-- RODAR DUAS VEZES
--   O arquivo inteiro está dentro de begin/commit: se der erro em qualquer ponto,
--   o Postgres desfaz tudo e não deixa nada pela metade.
--
-- COMO DESFAZER: no fim do arquivo.
-- -----------------------------------------------------------------------------

begin;

-- Registro do que já rodou (CORE-07). O hash detecta arquivo alterado depois de
-- aplicado — o que seria pior do que não ter aplicado.
create table if not exists schema_migrations (
  versao      text          primary key,
  arquivo     text          not null,
  hash_sha256 text          not null default '',
  aplicada_em timestamptz   not null default now()
);

comment on table schema_migrations is
  'Migrations já aplicadas. Não editar à mão.';

-- Nível de acesso do login. Lista fechada: o banco recusa qualquer outro valor.
-- O detalhe de quem alcança qual rotina virá com a tela de permissões; por ora o
-- sistema só precisa saber o nível.
create type nivel_acesso as enum ('builder', 'ceo', 'gerente', 'comum');

-- Mantém atualizado_em correto sem depender de o código lembrar.
create or replace function tocar_atualizado_em()
returns trigger
language plpgsql
as $$
begin
  new.atualizado_em := now();
  return new;
end;
$$;

alter table schema_migrations enable row level security;

insert into schema_migrations (versao, arquivo) values ('001', '001_tipos.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop function if exists tocar_atualizado_em();
-- drop type if exists nivel_acesso;
-- delete from schema_migrations where versao = '001';
