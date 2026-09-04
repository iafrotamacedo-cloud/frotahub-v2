-- =============================================================================
-- 051 — origem='manual' em servicos_orcamentos                          rev 1
-- =============================================================================
--
-- A migração 050 previa só duas origens: 'gatilho' (a varredura automática
-- achou o responsável já trocado) e 'candidato' (alguém aprovou um card de
-- servicos_candidatos). Faltava a terceira: o botão "mandar para fila de
-- serviços" agora grava a linha do Kanban NA HORA (servicos/kanban.go,
-- MarcarComoServico) em vez de esperar a próxima varredura alcançar — então
-- precisa do próprio rótulo, não emprestar 'gatilho' (que numa auditoria
-- futura ia parecer que ninguém clicou em nada).
--
-- É segura de rodar duas vezes.
-- =============================================================================

alter table servicos_orcamentos
  drop constraint if exists servicos_orcamentos_origem_check;

alter table servicos_orcamentos
  add constraint servicos_orcamentos_origem_check
  check (origem in ('gatilho', 'candidato', 'manual'));

insert into schema_migrations (versao, arquivo)
values ('051', '051_origem_manual_de_servico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select conname, pg_get_constraintdef(oid) from pg_constraint
--    where conname = 'servicos_orcamentos_origem_check';
--   -- esperado: inclui 'manual'
--
-- PARA DESFAZER
--   alter table servicos_orcamentos drop constraint servicos_orcamentos_origem_check;
--   alter table servicos_orcamentos add constraint servicos_orcamentos_origem_check
--     check (origem in ('gatilho', 'candidato'));
--   delete from schema_migrations where versao = '051';
-- =============================================================================
