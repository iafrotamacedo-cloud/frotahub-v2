-- =============================================================================
-- 012 — o modo `alvos` do robô                                           rev 1
-- =============================================================================
--
-- POR QUE ESTA MIGRAÇÃO EXISTE
--
--   `robo_execucoes.modo` nasceu com um CHECK listando os três modos que havia:
--   levantamento, copia e atualizacao. E o robô abre a linha em
--   `robo_execucoes` ANTES de fazer qualquer trabalho — é assim que uma rodada
--   interrompida deixa rastro em vez de sumir.
--
--   Sem estender o CHECK, o modo novo morreria na PRIMEIRA linha, com um 400 do
--   PostgREST, antes de ler um único chamado. O código estaria certo e a rodada
--   falharia mesmo assim. Este é exatamente o tipo de erro que não aparece em
--   `go build` nem em `go vet`: mora no banco, não no programa.
--
-- O QUE O MODO NOVO FAZ
--
--   `alvos` lê chamados escolhidos a dedo, PELO NÚMERO, em vez de varrer uma
--   janela de tempo. Serve para buscar um punhado de chamados antigos sem
--   arrastar junto tudo o que foi criado entre eles e hoje — e, principalmente,
--   sem depender de um filtro para não gravar o resto: o que não está na lista
--   não chega a ser consultado.
--
-- É segura de rodar duas vezes.
-- =============================================================================

alter table robo_execucoes
  drop constraint if exists robo_execucoes_modo_check;

alter table robo_execucoes
  add constraint robo_execucoes_modo_check
  check (modo in ('levantamento', 'copia', 'atualizacao', 'alvos'));

insert into schema_migrations (versao, arquivo)
values ('012', '012_robo_alvos.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select pg_get_constraintdef(oid) from pg_constraint
--    where conname = 'robo_execucoes_modo_check';
--
-- PARA DESFAZER
--
--   alter table robo_execucoes drop constraint robo_execucoes_modo_check;
--   alter table robo_execucoes add constraint robo_execucoes_modo_check
--     check (modo in ('levantamento', 'copia', 'atualizacao'));
--   delete from schema_migrations where versao = '012';
-- =============================================================================
