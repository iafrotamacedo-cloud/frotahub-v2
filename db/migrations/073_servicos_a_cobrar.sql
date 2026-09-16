-- =============================================================================
-- 073 — o que ainda não foi cobrado do cliente, em Serviço              rev 1
-- =============================================================================
--
-- O MESMO CONCEITO DE orcamentos_a_cobrar (migração 041), DO OUTRO LADO
--
--   Lá a régua era `status = 'lancado' AND fatura_id IS NULL`: tudo que foi
--   lançado no Trílogo e nenhuma planilha anterior levou ao cliente.
--
--   Aqui não existe `fatura_id` — o card de Faturamento de Serviço já separa
--   "Aguardando PCO" de "A faturar" pelo PREENCHIMENTO de `pco_numero`
--   (migração 053, ver o comentário de PreencherPCO em kanban.go e a view
--   servicos_painel, migração 054). Então a régua nasce pronta:
--
--     status = 'aguardando_faturamento' AND pco_numero IS NULL
--
--   — exatamente o recorte que já vira o sub-card "Aguardando PCO" no hub.
--   O relatório mensal é essa mesma fila, no modelo de planilha que vai ao
--   cliente; o dia em que ele responde com o PCO, o número entra em
--   `pco_numero` (tela de Faturamento) e a linha sai daqui sozinha — não tem
--   "fechar competência" aqui, porque a saída da fila já é o próprio PCO.
--
-- A DATA É `atualizado_em`, NÃO `entrou_em`
--
--   `entrou_em` é de quando o chamado virou Serviço — pode ser meses antes de
--   ficar pronto para faturar. `atualizado_em` é carimbada por um gatilho a
--   cada UPDATE da linha (migração 050, tocar_atualizado_em); para uma linha
--   ainda em aguardando_faturamento/sem PCO, ela é o instante em que o card
--   entrou nessa fila (a transição finalizado → aguardando_faturamento) — a
--   melhor aproximação disponível de "desde quando isto está esperando".
--
-- O NOME DA LOJA É O DO CLIENTE, NÃO O NOSSO (mesma razão da 041/040)
--
--   `nome_cliente` vem de `unidades` (migração 040) — mesma tabela que
--   orcamentos_a_cobrar já usa. Sem ele a linha sai com LOJA em branco.
--
-- POR QUE O `with (security_invoker = true)` ESTÁ AQUI (P-35)
--
--   `create or replace view` apaga as opções da view — e esta view carrega
--   dinheiro a cobrar de um cliente. Ver a migração 033.
--
-- É segura de rodar duas vezes.
-- =============================================================================

create or replace view servicos_a_cobrar
with (security_invoker = true) as
select
  so.id,
  so.cliente_id,
  so.ticket,
  so.conta,
  so.orcamento_valor as valor,
  so.atualizado_em   as data_relatorio,
  u.nome             as loja,
  u.nome_cliente     as loja_cliente
from servicos_orcamentos so
join chamados c on c.id = so.chamado_id
left join unidades u on u.id = c.unidade_id
where so.status = 'aguardando_faturamento'
  and so.pco_numero is null
  and so.removido_em is null;

comment on view servicos_a_cobrar is
  'Os serviços vistoriados (aguardando_faturamento) que ainda não têm PCO do '
  'cliente — a mesma fila do sub-card "Aguardando PCO" do hub de Serviço, no '
  'recorte que alimenta o Relatório mensal. RODA COMO QUEM PERGUNTA '
  '(security_invoker) — se você reescrever esta view, REPITA a cláusula '
  '`with (security_invoker = true)`, senão a tranca cai em silêncio (migração '
  '033).';

insert into schema_migrations (versao, arquivo)
values ('073', '073_servicos_a_cobrar.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select count(*) as quantos, sum(valor) as total,
--          min(data_relatorio) as de, max(data_relatorio) as ate
--     from servicos_a_cobrar;
--
--   -- nenhuma linha pode sair sem o nome do cliente:
--   select loja, count(*) from servicos_a_cobrar
--    where loja_cliente is null group by loja;
--   -- esperado: nada (mesma cobertura de unidades da 040)
--
--   -- o que já tem PCO não pode aparecer aqui:
--   select count(*) from servicos_a_cobrar sc
--     join servicos_orcamentos so on so.id = sc.id where so.pco_numero is not null;
--   -- esperado: 0
--
--   -- a tranca de pé:
--   select relname, reloptions from pg_class where relname='servicos_a_cobrar';
--   -- esperado: {security_invoker=true}
--
-- PARA DESFAZER
--   drop view servicos_a_cobrar;
--   delete from schema_migrations where versao = '073';
-- =============================================================================
