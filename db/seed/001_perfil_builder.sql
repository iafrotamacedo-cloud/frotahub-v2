-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — carga inicial: o perfil do builder
--
-- QUANDO RODAR
--   DEPOIS de criar o usuário no painel do Supabase, em Authentication → Users →
--   Add user, com:
--       e-mail  builder@frotahub.local
--       senha   a que você escolher
--       "Auto Confirm User" LIGADO  (senão o login recusa por e-mail não confirmado)
--
--   Este arquivo não cria login nem guarda senha: ele só liga o usuário que você
--   criou a um perfil dentro do FrotaHub.
--
-- PODE RODAR DE NOVO
--   Sim. Se o perfil já existir, ele é apenas atualizado.
-- -----------------------------------------------------------------------------

insert into perfis (id, usuario, nome, nivel, ativo)
select u.id, 'builder', 'Igor Tostes', 'builder', true
from auth.users u
where u.email = 'builder@frotahub.local'
on conflict (id) do update
  set usuario = excluded.usuario,
      nome    = excluded.nome,
      nivel   = excluded.nivel,
      ativo   = excluded.ativo;

-- Confere: tem que devolver exatamente uma linha.
select p.usuario, p.nome, p.nivel, p.ativo, u.email
from perfis p join auth.users u on u.id = p.id;
