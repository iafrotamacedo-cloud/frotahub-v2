-- =============================================================================
-- 054 — o painel de cards e a planilha de controle de Serviço            rev 1
-- =============================================================================
--
-- DUAS VIEWS, NENHUMA TABELA NOVA
--
--   servicos_painel   uma linha por cliente — alimenta os 6 cards do hub
--                      (contagem + o split de Faturamento em "Aguardando
--                      PCO"/"A faturar", que não é status novo, é filtro por
--                      pco_numero — ver migração 053).
--
--   servicos_lista     a fonte ÚNICA das 9 telas de lista e da exportação da
--                       Planilha de controle. Uma fonte só, de propósito: a
--                       tela e o PDF/Excel nunca podem discordar do que é
--                       "atrasado" ou "com PCO" (CORE-06).
--
-- security_invoker = true NAS DUAS, sem exceção — mesma regra de todas as
-- views deste sistema desde a migração 033 (ver TestReescreverViewNaoDerruba
-- OSecurityInvoker, baleryan/interno/modulos/orcamentos/migracoes_test.go).
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. servicos_painel
-- -----------------------------------------------------------------------------

create view servicos_painel
with (security_invoker = true) as
select
  c.id as cliente_id,

  coalesce(cand.pendentes, 0)              as candidatos_pendentes,
  coalesce(so.aguardando_orcamento, 0)     as orcamentos_pendentes,
  coalesce(so.orcamento_feito, 0)          as orcamento_feito,
  coalesce(so.orcamento_lancado, 0)        as orcamento_lancado,
  coalesce(so.aprovado_execucao, 0)        as execucao_aberto,
  coalesce(so.em_execucao, 0)              as execucao_em_curso,
  coalesce(so.finalizado, 0)               as execucao_executado,
  coalesce(so.aguardando_pco, 0)           as faturamento_aguardando_pco,
  coalesce(so.a_faturar, 0)                as faturamento_a_faturar,
  coalesce(so.faturado, 0)                 as faturamento_faturado,
  coalesce(so.total_ativos, 0)             as total_ativos

from clientes c
left join (
  select cliente_id, count(*) as pendentes
    from servicos_candidatos
   where status = 'pendente'
   group by cliente_id
) cand on cand.cliente_id = c.id
left join (
  select
    cliente_id,
    count(*) filter (where status = 'aguardando_orcamento')                              as aguardando_orcamento,
    count(*) filter (where status = 'orcamento_feito')                                   as orcamento_feito,
    count(*) filter (where status = 'orcamento_lancado')                                 as orcamento_lancado,
    count(*) filter (where status = 'aprovado_execucao')                                 as aprovado_execucao,
    count(*) filter (where status = 'em_execucao')                                       as em_execucao,
    count(*) filter (where status = 'finalizado')                                        as finalizado,
    count(*) filter (where status = 'aguardando_faturamento' and pco_numero is null)     as aguardando_pco,
    count(*) filter (where status = 'aguardando_faturamento' and pco_numero is not null) as a_faturar,
    count(*) filter (where status = 'faturado')                                          as faturado,
    count(*)                                                                             as total_ativos
    from servicos_orcamentos
   where removido_em is null
   group by cliente_id
) so on so.cliente_id = c.id;

comment on view servicos_painel is
  'Uma linha por cliente — alimenta os 6 cards do hub de Serviço. Aguardando '
  'PCO/A faturar são o MESMO status (aguardando_faturamento), separados aqui '
  'só por pco_numero — ver migração 053.';


-- -----------------------------------------------------------------------------
-- 2. servicos_lista
-- -----------------------------------------------------------------------------
--
-- FLAGS CALCULADAS EM SQL, NÃO EM GO NEM NO FRONT
--
--   `com_pco`/`com_nf`/`aprovado`/`rejeitado` são fatos derivados de colunas
--   já existentes — calcular uma vez aqui evita que a tela de lista e a
--   exportação em PDF/Excel discordem sobre o que "com PCO" significa.

create view servicos_lista
with (security_invoker = true) as
select
  so.id, so.cliente_id, so.chamado_id, so.ticket, so.conta, so.status,

  c.descricao      as chamado_descricao,
  c.status_codigo  as chamado_status_codigo,
  c.status         as chamado_status,
  c.executado_em,
  c.vistoriado_em,
  u.nome           as loja,

  so.cotacao_trilogo_id, so.orcamento_trilogo_id, so.orcamento_valor,
  so.orcamento_arquivo_sha256, so.orcamento_arquivo_nome, so.orcamento_arquivo_em,
  so.orcamento_aprovado_em, so.orcamento_rejeitado_em,

  so.pco_numero, so.pco_preenchido_em,
  so.nf_numero, so.nf_arquivo_sha256, so.nf_arquivo_nome, so.nf_arquivo_em,

  so.origem, so.entrou_em, so.atualizado_em,
  so.removido_em, so.removido_motivo,

  (so.orcamento_arquivo_sha256 is not null)  as com_orcamento,
  (so.pco_numero is not null)                as com_pco,
  (so.nf_arquivo_sha256 is not null)         as com_nf,
  (so.orcamento_aprovado_em is not null)     as esta_aprovado,
  (so.orcamento_rejeitado_em is not null)    as esta_rejeitado

  from servicos_orcamentos so
  join chamados  c on c.id = so.chamado_id
  left join unidades u on u.id = c.unidade_id;

comment on view servicos_lista is
  'Fonte única das telas de lista e da exportação da Planilha de controle — '
  'inclui histórico (removido_em preenchido também aparece; quem filtra '
  'decide se quer só os ativos).';


insert into schema_migrations (versao, arquivo)
values ('054', '054_paineis_e_planilha_de_servico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname in ('servicos_painel', 'servicos_lista');
--   -- esperado: {security_invoker=true} nas duas
--
--   select * from servicos_painel;
--   -- esperado: uma linha por cliente, tudo zero se ainda não há serviço
--
-- PARA DESFAZER
--   drop view servicos_lista;
--   drop view servicos_painel;
--   delete from schema_migrations where versao = '054';
-- =============================================================================
