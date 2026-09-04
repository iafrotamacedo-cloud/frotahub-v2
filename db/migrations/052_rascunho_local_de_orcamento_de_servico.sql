-- =============================================================================
-- 052 — o rascunho local do orçamento de Serviço                        rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   O redesenho do funil (dono, 04/09/2026) divide "criar orçamento" em duas
--   etapas que hoje são uma só (servicos/cotacoes.go, CriarOrcamento):
--
--     1. Pendentes -> Feitos: anexar o PDF do orçamento — SÓ pro nosso R2,
--        sem falar com o Trílogo ainda. É o rascunho.
--     2. Feitos -> Lançados: "lançar no Trílogo" de verdade — cria a cotação
--        e o orçamento lá, subindo o MESMO arquivo já anexado no passo 1.
--
--   Estas colunas guardam o passo 1. `cotacao_trilogo_id`/`orcamento_trilogo_id`
--   (migração 050) continuam sendo o passo 2 — não mudam de lugar nem de nome.
--
-- POR QUE `orcamento_aprovado_em`/`orcamento_rejeitado_em` JÁ NASCEM AQUI
--
--   Ficam nulas até a detecção automática de aprovação/rejeição existir (que
--   depende de um teste ao vivo no Trílogo ainda não feito — ver o plano).
--   Nascer agora evita uma quarta migração só para isso quando o teste
--   terminar; usar é opcional até lá.
--
-- É segura de rodar duas vezes.
-- =============================================================================

alter table servicos_orcamentos
  add column if not exists orcamento_arquivo_sha256 text references arquivos(sha256) on delete restrict,
  add column if not exists orcamento_arquivo_nome   text,
  add column if not exists orcamento_arquivo_em     timestamptz,
  add column if not exists orcamento_aprovado_em    timestamptz,
  add column if not exists orcamento_rejeitado_em   timestamptz;

comment on column servicos_orcamentos.orcamento_arquivo_sha256 is
  'O PDF do orçamento, anexado em Pendentes — rascunho local, antes de existir '
  'no Trílogo. Aponta para arquivos(sha256) (migração 007), o mesmo padrão de '
  'orcamentos/documentos.go.';
comment on column servicos_orcamentos.orcamento_aprovado_em is
  'Preenchida pela detecção automática de aprovação (ainda não construída — '
  'depende de teste ao vivo no Trílogo). Nula não quer dizer "rejeitado".';

insert into schema_migrations (versao, arquivo)
values ('052', '052_rascunho_local_de_orcamento_de_servico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select column_name from information_schema.columns
--    where table_name = 'servicos_orcamentos' and column_name like 'orcamento_%';
--   -- esperado: inclui as 5 colunas novas, além de orcamento_trilogo_id/orcamento_valor
--
-- PARA DESFAZER
--   alter table servicos_orcamentos
--     drop column orcamento_arquivo_sha256,
--     drop column orcamento_arquivo_nome,
--     drop column orcamento_arquivo_em,
--     drop column orcamento_aprovado_em,
--     drop column orcamento_rejeitado_em;
--   delete from schema_migrations where versao = '052';
-- =============================================================================
