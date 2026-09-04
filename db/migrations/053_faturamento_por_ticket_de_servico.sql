-- =============================================================================
-- 053 — PCO e nota fiscal, por ticket de Serviço                        rev 1
-- =============================================================================
--
-- O MESMO PCO DO FATURAMENTO DO CONTRATO, GRANULARIDADE DIFERENTE
--
--   `faturas.pco_numero` (migração 017) é o pedido de compra que o cliente
--   abre por CICLO (mês) × loja × conta — agrega dezenas de orçamentos numa
--   fatura só. Serviço não tem ciclo: cada ticket fatura sozinho, na hora que
--   terminar. É o MESMO conceito (confirmado com o dono, 04/09/2026) — por
--   isso a nomenclatura é igual (`pco_numero`, `nf_numero`) — só que direto em
--   `servicos_orcamentos`, uma linha por ticket, não numa tabela de ciclo.
--
-- AS DUAS TRANSIÇÕES DO CARTÃO FINANCEIRO
--
--   `aguardando_faturamento` continua sendo UM status só (migração 050) — os
--   dois sub-cards do front ("Aguardando PCO" / "A faturar") são o MESMO
--   status filtrado por `pco_numero is null`. A única transição de `status`
--   de verdade aqui é `aguardando_faturamento -> faturado`, disparada por
--   anexar a nota fiscal (`nf_arquivo_sha256` preenchido).
--
-- PCO PODE SER REESCRITO, NOTA FISCAL NÃO (por enquanto)
--
--   Digitar o PCO errado e corrigir é o caso comum — por isso não há coluna
--   "travando" o preenchimento. NF é o gatilho de uma mudança de status; se
--   precisar corrigir depois de anexada, é decisão futura (ver plano).
--
-- É segura de rodar duas vezes.
-- =============================================================================

alter table servicos_orcamentos
  add column if not exists pco_numero         text,
  add column if not exists pco_preenchido_em  timestamptz,
  add column if not exists pco_preenchido_por uuid references perfis(id) on delete set null,

  add column if not exists nf_numero          text,
  add column if not exists nf_arquivo_sha256  text references arquivos(sha256) on delete restrict,
  add column if not exists nf_arquivo_nome    text,
  add column if not exists nf_arquivo_em      timestamptz,
  add column if not exists nf_arquivo_por     uuid references perfis(id) on delete set null;

comment on column servicos_orcamentos.pco_numero is
  'O pedido de compra que o cliente abre para este ticket faturar — mesmo '
  'conceito de faturas.pco_numero (migração 017), granularidade por ticket. '
  'Preenchido separa "Aguardando PCO" de "A faturar" no front, sem mudar status.';
comment on column servicos_orcamentos.nf_arquivo_sha256 is
  'A nota fiscal, anexada — aponta para arquivos(sha256) (migração 007). '
  'Preenchida avança o status para faturado.';

insert into schema_migrations (versao, arquivo)
values ('053', '053_faturamento_por_ticket_de_servico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select column_name from information_schema.columns
--    where table_name = 'servicos_orcamentos' and column_name like 'pco_%' or column_name like 'nf_%';
--   -- esperado: inclui as 8 colunas novas
--
-- PARA DESFAZER
--   alter table servicos_orcamentos
--     drop column pco_numero, drop column pco_preenchido_em, drop column pco_preenchido_por,
--     drop column nf_numero, drop column nf_arquivo_sha256, drop column nf_arquivo_nome,
--     drop column nf_arquivo_em, drop column nf_arquivo_por;
--   delete from schema_migrations where versao = '053';
-- =============================================================================
