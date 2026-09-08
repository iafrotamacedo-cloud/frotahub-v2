-- =============================================================================
-- 057 — Valor lido do PDF do orçamento, por IA                          rev 1
-- =============================================================================
--
-- SÓ REFERÊNCIA, NÃO O VALOR DE VERDADE
--
--   `orcamento_valor` (migração 050/052) é o valor de verdade — a soma dos
--   itens que a pessoa digitou e que foi lançada no Trílogo. Esta coluna nova
--   é OUTRA coisa: o que a IA (leitor.IA, mesma que lê nota fiscal do
--   contrato — ver interno/leitor) entendeu, lendo o PDF anexado, ANTES de
--   ninguém digitar nada. Serve só de pista discreta na tela de lançar
--   ("o PDF anexado indica R$ X"), pra quem está digitando os itens conferir
--   se bate — pedido do dono, 08/09/2026.
--
-- LEITURA BEST-EFFORT, NUNCA TRAVA O ANEXO
--
--   Sem chave de IA configurada, IA fora do ar, ou PDF que ela não entende: a
--   coluna fica nula e o anexo segue normal (ver servicos/documentos.go,
--   InserirArquivoDeOrcamento). Documento de Serviço não tem a régua de uma
--   nota fiscal (sem chave de acesso, sem itens de fornecedor) — é só o
--   `valor_total` da mesma leitura que a Orçamentos já usa.
--
-- É segura de rodar duas vezes.
-- =============================================================================

alter table servicos_orcamentos
  add column if not exists orcamento_arquivo_valor numeric(14,2);

comment on column servicos_orcamentos.orcamento_arquivo_valor is
  'O valor total que a IA leu no PDF anexado, só como referência discreta na '
  'tela de lançar — não é o valor lançado (orcamento_valor). Nulo quando a '
  'leitura não rolou (sem IA configurada, falha, ou PDF sem valor claro).';

-- servicos_lista (migração 054) precisa da coluna nova pra Planilha/listas
-- lerem sem um segundo pedido ao banco.
create or replace view servicos_lista
with (security_invoker = true) as
select
    so.id, so.cliente_id, so.chamado_id, so.ticket, so.conta, so.status,
    c.descricao as chamado_descricao, c.status_codigo as chamado_status_codigo,
    c.status as chamado_status, c.executado_em, c.vistoriado_em,
    u.nome as loja,
    so.cotacao_trilogo_id, so.orcamento_trilogo_id, so.orcamento_valor,
    so.orcamento_arquivo_sha256, so.orcamento_arquivo_nome, so.orcamento_arquivo_em,
    so.orcamento_aprovado_em, so.orcamento_rejeitado_em,
    so.pco_numero, so.pco_preenchido_em,
    so.nf_numero, so.nf_arquivo_sha256, so.nf_arquivo_nome, so.nf_arquivo_em,
    so.origem, so.entrou_em, so.atualizado_em, so.removido_em, so.removido_motivo,
    so.orcamento_arquivo_sha256 is not null as com_orcamento,
    so.pco_numero is not null as com_pco,
    so.nf_arquivo_sha256 is not null as com_nf,
    so.orcamento_aprovado_em is not null as esta_aprovado,
    so.orcamento_rejeitado_em is not null as esta_rejeitado,
    -- Nova (migração 057): vai no FIM de propósito. `CREATE OR REPLACE VIEW`
    -- recusa mudar posição/nome de coluna existente — só aceita ACRESCENTAR
    -- no fim (é o que travou na primeira tentativa desta migração).
    so.orcamento_arquivo_valor
  from servicos_orcamentos so
  join chamados c on c.id = so.chamado_id
  left join unidades u on u.id = c.unidade_id;

insert into schema_migrations (versao, arquivo)
values ('057', '057_valor_lido_do_orcamento_de_servico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select column_name from information_schema.columns
--    where table_name = 'servicos_orcamentos' and column_name = 'orcamento_arquivo_valor';
--   -- esperado: 1 linha
--
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname = 'servicos_lista';
--   -- esperado: {security_invoker=true}
--
-- PARA DESFAZER
--   create or replace view servicos_lista with (security_invoker = true) as
--     -- a definição da migração 054, sem orcamento_arquivo_valor
--   alter table servicos_orcamentos drop column orcamento_arquivo_valor;
--   delete from schema_migrations where versao = '057';
-- =============================================================================
