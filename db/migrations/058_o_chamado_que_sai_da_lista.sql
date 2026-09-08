-- =============================================================================
-- 058 — o chamado que SAI da lista do Trílogo                            rev 1
-- =============================================================================
--
-- O DEFEITO QUE ISTO CONSERTA
--
--   O robô (trilogo/robo.go) só sabia ACRESCENTAR. Toda rodada lê a lista do
--   Trílogo e faz upsert do que achou; nada nunca olhava para o outro lado —
--   "o que estava aqui e não está mais lá?". Quando os Mercadinhos tiram um
--   chamado da nossa prestadora, ele some da lista das duas contas e continua
--   na nossa tela para sempre, indistinguível de um chamado nosso de verdade.
--   A lista só crescia. Pedido do dono, 08/09/2026.
--
-- POR QUE MARCAR, E NÃO APAGAR
--
--   O chamado que saiu tem HISTÓRIA: timeline, custos, anexos já copiados para
--   o armazém, e — o que mais pesa — pode ter nota, orçamento e faturamento
--   amarrados nele do lado de cá. Apagar a linha levaria tudo isso junto (as
--   filhas são `on delete cascade`) e transformaria "por que isso sumiu?" numa
--   pergunta sem resposta, que é exatamente o que o resto do sistema evita
--   (ver interno/banco/cliente.go, comentário de `Apagar`).
--
--   Então `saiu_em` é um CARIMBO, não uma exclusão: a linha continua inteira,
--   a ficha do ticket continua abrindo pelo número, e a lista deixa de
--   mostrá-la. Se o chamado voltar para a nossa prestadora, o robô limpa o
--   carimbo e ele reaparece — sem releitura de nada.
--
-- QUEM ESCREVE ESTA COLUNA
--
--   Só o robô, em trilogo/saida.go, e só depois de PERGUNTAR ao Trílogo, um a
--   um, se o chamado ainda é nosso. Sumir da lista é o indício; a resposta do
--   `GetTicketDetail` é a prova. Nenhuma tela escreve aqui.
--
-- É segura de rodar duas vezes.
-- =============================================================================

alter table chamados
  add column if not exists saiu_em timestamptz;

comment on column chamados.saiu_em is
  'Quando confirmamos que este chamado não é mais nosso no Trílogo (sumiu da '
  'lista das duas contas E o detalhe dele parou de responder). Nulo = está na '
  'lista. A linha fica inteira; só sai da vista.';


-- -----------------------------------------------------------------------------
-- A visão da lista ganha a coluna
-- -----------------------------------------------------------------------------
--
-- A visão não FILTRA os que saíram — ela EXPÕE o carimbo, e quem filtra é o
-- motor (trilogo/consulta.go, montarFiltro). O motivo é que a mesma visão
-- serve a três perguntas diferentes: a lista (esconde), a ficha aberta pelo
-- número (mostra sempre — quem digitou o ticket quer o ticket) e a vista dos
-- que saíram (mostra só eles). Uma visão que já decidisse por dentro não
-- conseguiria servir as três, e a ficha passaria a responder "não está na
-- nossa base" para um chamado que está.
--
-- Definição idêntica à da migração 009, mais `c.saiu_em`.

create or replace view chamados_lista
with (security_invoker = true) as
select
  c.id,
  c.cliente_id,
  c.numero,
  c.unidade_id,
  u.nome            as loja,
  c.conta,
  c.status,
  c.prioridade,
  c.descricao,
  c.ambiente,
  c.responsavel,
  c.criado_em,
  c.prazo,
  coalesce(k.total, 0)::numeric(12,2) as custo_total,
  coalesce(a.quantos, 0)              as anexos,
  -- Coluna nova entra no FIM, sempre: nome e posição do que já existia são
  -- congelados, e quem reescreve uma view no meio quebra quem lê por posição.
  -- Há um teste que cobra isso (orcamentos/migracoes_test.go).
  c.saiu_em
from chamados c
join unidades u on u.id = c.unidade_id
left join lateral (
  select sum(valor) as total from chamado_custos k2 where k2.chamado_id = c.id
) k on true
left join lateral (
  select count(*) as quantos from chamado_anexos a2 where a2.chamado_id = c.id
) a on true;

comment on view chamados_lista is
  'A lista de chamados como a tela precisa dela: com o nome da loja, a soma dos custos e a contagem de anexos já resolvidos. Roda com as permissões de quem lê.';


-- -----------------------------------------------------------------------------
-- Nenhum índice novo
-- -----------------------------------------------------------------------------
--
-- A tentação é criar um índice para `saiu_em`, já que ele passa a entrar em
-- toda consulta da lista. Não vale (CORE-01: um índice por pergunta que alguém
-- realmente faz):
--
--   - os índices da 009 já entregam a ORDEM, que é o que custava caro; o
--     `saiu_em is null` é uma conferência de uma coluna nula em linhas que o
--     banco já foi buscar;
--   - a esmagadora maioria das linhas tem `saiu_em` nulo, então um índice sobre
--     ele não separa quase nada — o planejador nem o usaria;
--   - e todo índice a mais é escrita mais lenta em TODA rodada do robô, que
--     grava centenas de chamados por vez.
--
-- Se um dia a base crescer a ponto de isso pesar, o índice certo é PARCIAL
-- (`where saiu_em is not null`), para servir à vista dos que saíram — que é a
-- pergunta rara. Medir antes.

insert into schema_migrations (versao, arquivo)
values ('058', '058_o_chamado_que_sai_da_lista.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select column_name from information_schema.columns
--    where table_name = 'chamados' and column_name = 'saiu_em';
--   -- esperado: 1 linha
--
--   select count(*) from chamados_lista where saiu_em is not null;
--   -- esperado: 0 antes da primeira rodada do robô depois desta migração
--
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname = 'chamados_lista';
--   -- esperado: {security_invoker=true}
--
-- DEPOIS DE UMA RODADA DE ATUALIZAÇÃO, quem saiu:
--
--   select numero, conta, status, criado_em, saiu_em
--     from chamados where saiu_em is not null order by saiu_em desc;
--
-- PARA DESFAZER
--   create or replace view chamados_lista with (security_invoker = true) as
--     -- a definição da migração 009, sem saiu_em
--   alter table chamados drop column saiu_em;
--   delete from schema_migrations where versao = '058';
-- =============================================================================
