-- =============================================================================
-- 042 — as estatísticas do contrato                                     rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   A tela de Estatísticas foi construída fora do sistema, como um bloco que
--   roda offline a partir de um RETRATO do banco (um `dados.js` de 780 KB,
--   tirado em 30/08/2026). Esta migração é o que aposenta esse retrato: as nove
--   views abaixo entregam exatamente as mesmas nove tabelas, com as mesmas
--   colunas e na mesma ordem, lendo o banco vivo.
--
--   Nada é guardado duas vezes. Não há tabela nova, não há cópia, não há coluna
--   nova em lugar nenhum: são NOVE VIEWS sobre o que já existe. O retrato
--   morre junto com esta migração.
--
-- POR QUE VIEWS, E NÃO A CONTA NO SQL
--
--   Porque a conta da estatística — mediana, tempo entre marcos, índice contra
--   o plano de preventiva — mora no JavaScript da tela, e num lugar só
--   (CORE-06). Ela foi escrita e conferida contra o retrato; se metade dela
--   descesse para o SQL, passariam a existir duas versões da mesma regra, e
--   elas discordariam no primeiro mês.
--
--   O que estas views fazem é só JUNTAR e DAR FORMATO: trocar `unidade_id`
--   (uuid) pelo número da loja no Trílogo, trocar `chamado_id` pelo número do
--   chamado, e resolver as três contagens que a tela não teria como fazer sem
--   puxar tabela inteira só para contar linha.
--
-- POR QUE O NÚMERO DO TRÍLOGO É A CHAVE DA LOJA
--
--   O retrato identificava a loja pela POSIÇÃO dela no array — a loja era "26"
--   porque era a 26ª linha. Isso funciona num arquivo parado e quebra no
--   primeiro dia em que uma unidade nova entra no meio da lista: todo chamado
--   passa a apontar para a loja errada, em silêncio, e o número continua
--   fazendo sentido na tela.
--
--   `id_trilogo` é estável, é inteiro, e já é a identidade da unidade do lado
--   de lá. As 38 unidades têm o seu, e os 38 são distintos (conferido em
--   30/08/2026).
--
-- POR QUE A DESCRIÇÃO DO CHAMADO NÃO SAI DAQUI
--
--   Ela pesava 114,4 KB — 14% do retrato inteiro — e a tela a usava para
--   desenhar DEZ linhas: a lista dos chamados mais antigos da fila. Mandar a
--   descrição de 1.719 chamados para escrever 10 é carregar o mesmo dado duas
--   vezes pelo sistema, e a decisão do dono em 30/08/2026 foi não carregar.
--
--   As dez linhas viram link para Dados do Trílogo, que já mostra o chamado
--   inteiro. O dado fica num lugar só.
--
-- POR QUE O ORÇAMENTO REMOVIDO CONTINUA SAINDO
--
--   Porque a tela conta quantos foram removidos, e `status = 'removido'` é
--   como ela sabe. Conferido em 30/08/2026: `status = 'removido'` e
--   `removido_em is not null` são a MESMA linha, sempre — os 5 removidos, e
--   nenhum vivo. Filtrar aqui esconderia o número que a tela existe para
--   mostrar.
--
-- POR QUE TODA VIEW CARREGA `with (security_invoker = true)` (P-35)
--
--   Porque sem a cláusula a view roda como a DONA das tabelas, e a dona não é
--   filtrada por política nenhuma — a chave `publishable` viaja dentro do
--   JavaScript do site. Foi o buraco que a migração 033 fechou, e ele volta
--   sozinho em qualquer `create or replace view` que esqueça a cláusula.
--
--   Estas nove entregam a operação inteira do contrato: chamados, custos,
--   margem e faturamento. São exatamente as que não podem vazar.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. As lojas
--
-- `id` é o número do Trílogo, e é ele que todas as outras views usam para dizer
-- de que loja é cada linha.
-- -----------------------------------------------------------------------------
create or replace view estatisticas_unidades
with (security_invoker = true) as
select u.cliente_id,
       u.id_trilogo as id,
       u.nome,
       u.cidade,
       u.no_escopo
  from unidades u;

comment on view estatisticas_unidades is
  'As lojas, como a tela de Estatísticas as identifica: `id` é o número do '
  'Trílogo (estável), não a posição numa lista. RODA COMO QUEM PERGUNTA '
  '(security_invoker) — se reescrever, REPITA a cláusula (migração 033).';


-- -----------------------------------------------------------------------------
-- 2. Os chamados
--
-- Sem a `descricao`: ver o cabeçalho.
-- -----------------------------------------------------------------------------
create or replace view estatisticas_chamados
with (security_invoker = true) as
select c.cliente_id,
       c.numero,
       u.id_trilogo as unidade,
       c.conta,
       c.status,
       c.prioridade,
       c.ambiente,
       c.responsavel,
       c.criado_em,
       c.prazo
  from chamados c
  left join unidades u on u.id = c.unidade_id;

comment on view estatisticas_chamados is
  'Os chamados para a tela de Estatísticas. NÃO traz a `descricao` de '
  'propósito: ela pesava 14% da resposta para desenhar 10 linhas, e essas 10 '
  'linhas passaram a levar para Dados do Trílogo (30/08/2026). RODA COMO QUEM '
  'PERGUNTA (security_invoker) — se reescrever, REPITA a cláusula.';


-- -----------------------------------------------------------------------------
-- 3. A linha do tempo
--
-- SÓ os eventos de MUDANÇA DE STATUS. A tabela tem 22.934 linhas; destas,
-- 6.290 mudam status (conferido em 30/08/2026). As outras 16.644 são
-- comentário, anexo e afins — a tela não olha para nenhuma delas, e mandá-las
-- quadruplicaria a resposta para nada.
--
-- É desta view que sai TODO tempo medido pela tela. A coluna `executado_em` da
-- tabela `chamados` está vazia em todos os chamados; quem sabe quando o chamado
-- foi executado é o primeiro evento de status 5.
-- -----------------------------------------------------------------------------
create or replace view estatisticas_eventos
with (security_invoker = true) as
select c.cliente_id,
       c.numero as chamado,
       e.status_codigo as sc,
       e.quando
  from chamado_eventos e
  join chamados c on c.id = e.chamado_id
 where e.status_codigo is not null;

comment on view estatisticas_eventos is
  'A linha do tempo dos chamados, só as mudanças de STATUS (as demais são '
  'comentário e anexo). A tela toma sempre o PRIMEIRO instante de cada status, '
  'para chamado reaberto não contar duas vezes. RODA COMO QUEM PERGUNTA '
  '(security_invoker) — se reescrever, REPITA a cláusula.';


-- -----------------------------------------------------------------------------
-- 4. Os custos lançados no Trílogo
-- -----------------------------------------------------------------------------
create or replace view estatisticas_custos
with (security_invoker = true) as
select c.cliente_id,
       c.numero as chamado,
       k.tipo,
       k.valor,
       k.criado_em
  from chamado_custos k
  join chamados c on c.id = k.chamado_id;

comment on view estatisticas_custos is
  'Os custos lançados nos chamados do Trílogo. `criado_em` vem vazio em boa '
  'parte deles (600 de 718 em 30/08/2026) — a tela datou esses pela execução '
  'do chamado, e diz na tela quantos foram datados assim. RODA COMO QUEM '
  'PERGUNTA (security_invoker) — se reescrever, REPITA a cláusula.';


-- -----------------------------------------------------------------------------
-- 5. Os orçamentos
--
-- Todos, inclusive os removidos: ver o cabeçalho.
-- -----------------------------------------------------------------------------
create or replace view estatisticas_orcamentos
with (security_invoker = true) as
select o.cliente_id,
       o.ticket,
       u.id_trilogo as unidade,
       o.conta,
       o.valor_nota,
       o.valor,
       o.ajustado_pelo_teto,
       o.rateio,
       o.status,
       o.lancado_em,
       o.faturado,
       o.pago,
       o.pago_em,
       o.criado_em,
       o.lancamento_bloqueio,
       o.faturamento_direto,
       o.fatura_id
  from orcamentos o
  left join unidades u on u.id = o.unidade_id;

comment on view estatisticas_orcamentos is
  'Os orçamentos para a tela de Estatísticas, INCLUSIVE os removidos — a tela '
  'conta quantos foram, e `status = ''removido''` é como ela sabe. '
  '`valor_nota` é o que se paga ao fornecedor e `valor` o que se cobra do '
  'cliente; a margem é a diferença. RODA COMO QUEM PERGUNTA '
  '(security_invoker) — se reescrever, REPITA a cláusula: esta view carrega a '
  'margem do contrato inteiro.';


-- -----------------------------------------------------------------------------
-- 6. As notas e DAVs
--
-- As três contagens do fim são o motivo de esta view existir. Sem elas a tela
-- teria que puxar `documento_tickets` e `orcamento_documentos` inteiras só para
-- contar linha, e a resposta cresceria sem nenhum número novo aparecer.
--
-- `duplicada_de` vira SIM/NÃO: a tela conta quantas são repetidas, não de qual
-- nota cada uma é cópia — e o uuid da outra nota não tem o que fazer aqui.
-- -----------------------------------------------------------------------------
create or replace view estatisticas_documentos
with (security_invoker = true) as
select d.cliente_id,
       d.fila,
       d.tipo,
       d.emissao,
       d.valor_total,
       d.status,
       d.inserido_em,
       d.oculto_em,
       d.bloqueio_motivo,
       (d.duplicada_de is not null) as duplicada_de,
       (select count(*) from documento_tickets t
         where t.documento_id = d.id) as tickets,
       (select count(*) from documento_tickets t
         where t.documento_id = d.id and t.chamado_id is null) as tickets_soltos,
       (select count(*) from orcamento_documentos od
          join orcamentos o on o.id = od.orcamento_id
         where od.documento_id = d.id
           and od.removido_em is null
           and o.removido_em is null) as orcamentos
  from documentos d;

comment on view estatisticas_documentos is
  'As notas e DAVs para a tela de Estatísticas. As três contagens do fim '
  'definem as pendências: sem ticket = `tickets = 0` e não usada; ticket não '
  'associado = `tickets_soltos > 0`. `duplicada_de` é SIM/NÃO, não o uuid da '
  'outra nota. RODA COMO QUEM PERGUNTA (security_invoker) — se reescrever, '
  'REPITA a cláusula.';


-- -----------------------------------------------------------------------------
-- 7. Os ciclos de faturamento
-- -----------------------------------------------------------------------------
create or replace view estatisticas_ciclos
with (security_invoker = true) as
select c.cliente_id,
       c.id,
       c.competencia,
       c.ate,
       c.enviado_em,
       c.fechado_em
  from faturamento_ciclos c;

comment on view estatisticas_ciclos is
  'Os ciclos de faturamento ao cliente. RODA COMO QUEM PERGUNTA '
  '(security_invoker) — se reescrever, REPITA a cláusula.';


-- -----------------------------------------------------------------------------
-- 8. As faturas
--
-- `valor_faturado` é a soma dos orçamentos amarrados à fatura. Ele não é
-- coluna de tabela nenhuma de propósito: a verdade sobre o que foi cobrado é a
-- lista de orçamentos, e um total gravado à parte é um número que envelhece
-- calado quando um orçamento entra ou sai.
--
-- `valor_recebido` é o outro lado, e esse é digitado: é o que o cliente pagou.
-- -----------------------------------------------------------------------------
create or replace view estatisticas_faturas
with (security_invoker = true) as
select f.cliente_id,
       f.id,
       f.ciclo_id,
       u.id_trilogo as unidade,
       f.conta,
       f.pco_numero,
       f.nf_numero,
       f.nf_em,
       f.recebido_em,
       f.valor_recebido,
       coalesce((select sum(o.valor) from orcamentos o
                  where o.fatura_id = f.id
                    and o.removido_em is null), 0) as valor_faturado
  from faturas f
  left join unidades u on u.id = f.unidade_id;

comment on view estatisticas_faturas is
  'As faturas ao cliente. `valor_faturado` é SOMADO dos orçamentos ligados à '
  'fatura, e não guardado em coluna: total gravado à parte envelhece calado. '
  '`valor_recebido` é o que voltou do cliente. RODA COMO QUEM PERGUNTA '
  '(security_invoker) — se reescrever, REPITA a cláusula.';


-- -----------------------------------------------------------------------------
-- 9. Os parâmetros
--
-- Margem e teto, com vigência (P-08). A tela mostra qual estava valendo.
-- -----------------------------------------------------------------------------
create or replace view estatisticas_parametros
with (security_invoker = true) as
select p.cliente_id,
       p.chave,
       p.valor,
       p.vigencia_inicio,
       p.vigencia_fim
  from parametros p;

comment on view estatisticas_parametros is
  'Margem e teto com a vigência de cada um (P-08). RODA COMO QUEM PERGUNTA '
  '(security_invoker) — se reescrever, REPITA a cláusula.';


-- -----------------------------------------------------------------------------
-- 10. A rotina no catálogo
--
-- Rotina nova sem ninguém marcado é um item de menu que não abre para pessoa
-- nenhuma: o motor recusa antes de olhar o dado, e a tela some (P-17). Por isso
-- ela nasce marcada para toda categoria que hoje enxerga os Dados do Trílogo —
-- que é o público exato desta tela: quem acompanha a operação do contrato.
--
-- Não herda de CONTRATO_ORCAMENTOS: quem só olha chamado não precisa passar a
-- ver margem por causa de uma tela nova.
-- -----------------------------------------------------------------------------
insert into rotinas (codigo, nome, modulo, ordem) values
  ('CONTRATO_ESTATISTICAS', 'Estatísticas', 'manutencao', 330)
on conflict (codigo) do nothing;

insert into categoria_permissoes (categoria_id, rotina, pode)
select cp.categoria_id, 'CONTRATO_ESTATISTICAS', cp.pode
  from categoria_permissoes cp
 where cp.rotina = 'CONTRATO_TRILOGO_DADOS'
on conflict (categoria_id, rotina) do nothing;


insert into schema_migrations (versao, arquivo)
values ('042', '042_as_estatisticas_do_contrato.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a tranca de pé nas nove (é o que a 033 ensinou a checar):
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname like 'estatisticas_%'
--    order by relname;
--   -- esperado: as NOVE com {security_invoker=true}
--
--   -- as contagens, contra o retrato de 30/08/2026 11:12:
--   select (select count(*) from estatisticas_unidades)   as unidades    -- 38
--        , (select count(*) from estatisticas_chamados)   as chamados    -- 1.719
--        , (select count(*) from estatisticas_eventos)    as eventos     -- 6.265+
--        , (select count(*) from estatisticas_custos)     as custos      -- 718
--        , (select count(*) from estatisticas_orcamentos) as orcamentos  -- 819
--        , (select count(*) from estatisticas_documentos) as documentos  -- 249
--        , (select count(*) from estatisticas_faturas)    as faturas     -- 54
--        , (select count(*) from estatisticas_ciclos)     as ciclos      -- 1
--        , (select count(*) from estatisticas_parametros) as parametros; -- 3
--   -- (eventos e orçamentos crescem todo dia; os outros são o piso)
--
--   -- nenhuma loja pode sair sem número do Trílogo, senão o chamado dela some
--   -- de toda estatística por loja:
--   select count(*) from estatisticas_chamados where unidade is null;
--   -- esperado: 0
--
--   -- a descrição NÃO pode voltar:
--   select count(*) from information_schema.columns
--    where table_name = 'estatisticas_chamados' and column_name = 'descricao';
--   -- esperado: 0
--
--   -- e a rotina tem que ter alcançado alguém:
--   select count(*) from categoria_permissoes
--    where rotina = 'CONTRATO_ESTATISTICAS' and pode;
--   -- esperado: o mesmo número de CONTRATO_TRILOGO_DADOS
--
-- PARA DESFAZER
--   drop view estatisticas_unidades, estatisticas_chamados, estatisticas_eventos,
--             estatisticas_custos, estatisticas_orcamentos, estatisticas_documentos,
--             estatisticas_ciclos, estatisticas_faturas, estatisticas_parametros;
--   delete from categoria_permissoes where rotina = 'CONTRATO_ESTATISTICAS';
--   delete from rotinas where codigo = 'CONTRATO_ESTATISTICAS';
--   delete from schema_migrations where versao = '042';
-- =============================================================================
