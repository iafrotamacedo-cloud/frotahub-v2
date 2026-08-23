-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 006: categoria também se desativa
--
-- O QUE FAZ
--   Acrescenta a marca de ativo na tabela de categorias.
--
-- POR QUE AGORA
--   Porque a tela de categorias vai nascer nesta fase, e alguém vai querer tirar
--   uma categoria de circulação. Pela CORE-05, tirar de circulação é MARCAR, não
--   apagar: um login antigo pode apontar para ela, e o histórico com certeza
--   aponta. Apagar deixaria rastro órfão.
--
--   A tabela nasceu sem esta coluna na 003. Acrescentar agora, com uma linha no
--   banco, é trivial; acrescentar depois, com quinze categorias em uso e telas
--   escritas em cima delas, não é.
--
-- DEPENDE DE: 003
-- -----------------------------------------------------------------------------

begin;

alter table categorias add column ativo boolean not null default true;

comment on column categorias.ativo is
  'Categoria fora de circulação continua existindo, e o histórico dela continua de pé (CORE-05).';

insert into schema_migrations (versao, arquivo) values ('006', '006_categorias_ativo.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- alter table categorias drop column ativo;
-- delete from schema_migrations where versao = '006';
