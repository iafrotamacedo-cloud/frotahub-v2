-- 023 — faturamento direto: a nota que o fornecedor cobra do cliente
--
-- O QUE ESTA FILA É
--
--   Há notas que o fornecedor fatura DIRETO ao cliente. Elas passam por nós só
--   para controle e estatística: nós lançamos o custo no Trílogo, porque o
--   chamado precisa saber quanto custou — e nunca as cobramos, porque não é
--   nosso o dinheiro que entra.
--
--   Por isso elas não têm orçamento no sentido comum. Não levam margem, não
--   passam pelo teto, não são rateadas e não entram em espelho de faturamento.
--   O que sobe para o Trílogo é o ARQUIVO ORIGINAL da nota, e o valor é o dela,
--   limpo.
--
-- POR QUE UMA TERCEIRA FILA, E NÃO UMA COLUNA NA TABELA DE SEMPRE
--
--   `documentos.fila` já separava `orcamento` de `rateio`, e tudo o que filtra
--   nota no sistema — o painel, a geração, as correções — filtra por ela. Uma
--   coluna nova ao lado exigiria lembrar de acrescentá-la em cada um desses
--   lugares, e o dia em que alguém esquecesse de um seria o dia em que uma nota
--   de faturamento direto entraria num espelho de cobrança.
--
--   Com a fila, quem não a conhece simplesmente não a vê. É o comportamento
--   certo por omissão, e não por vigilância.

alter table documentos drop constraint if exists documentos_fila_check;
alter table documentos add constraint documentos_fila_check
  check (fila = any (array['orcamento'::text, 'rateio'::text, 'direto'::text]));

comment on column documentos.fila is
  'orcamento = vira orçamento nosso · rateio = uma nota, vários chamados · '
  'direto = o fornecedor fatura ao cliente; nós só lançamos o custo e NUNCA cobramos.';

-- ---------------------------------------------------------------------------
-- o custo lançado, do lado dos orçamentos
-- ---------------------------------------------------------------------------
--
-- Ele mora em `orcamentos` porque a fila de lançamento é essa, e duplicar a
-- máquina de lançar para uma variante seria manter duas rotinas que sobem
-- arquivo para o Trílogo — e uma delas envelheceria.
--
-- ⚠ A COLUNA É O QUE O MANTÉM FORA DA COBRANÇA
--   `fatura_id is null` era o único critério da fila de faturar. Uma nota de
--   faturamento direto tem `fatura_id` nulo para sempre — ela entraria em todo
--   espelho, todo mês, e seria cobrada de um cliente que já pagou o fornecedor.
alter table orcamentos
  add column if not exists faturamento_direto boolean not null default false;

comment on column orcamentos.faturamento_direto is
  'Custo de nota faturada DIRETO ao cliente pelo fornecedor. Lançamos no Trílogo '
  'com o arquivo original e o valor limpo, e NUNCA entra em faturamento.';

-- O índice parcial da fila de faturar precisa excluí-las também: sem isso o
-- índice continuaria apontando para linhas que a consulta nunca mais quer.
drop index if exists orcamento_a_faturar;
create index if not exists orcamento_a_faturar
  on orcamentos (cliente_id, criado_em)
  where fatura_id is null and status <> 'removido' and faturamento_direto = false;

create index if not exists orcamento_direto
  on orcamentos (cliente_id, criado_em)
  where faturamento_direto = true;

-- ---------------------------------------------------------------------------
-- o painel: a fila de faturar passa a excluí-las, e a nova ganha contador
-- ---------------------------------------------------------------------------
--
-- ⚠ As 18 colunas abaixo são a definição em produção, VERBATIM e na ordem.
--    `create or replace view` só acrescenta no fim. O que MUDA são três
--    expressões — isso o Postgres permite, e é o ponto desta migração:
--
--    `a_faturar` e `valor_a_faturar`  passam a exigir faturamento_direto = false
--    `sem_associacao`                 deixa de olhar a fila `direto`, onde
--                                     ticket sem chamado é o esperado: não
--                                     conferimos o Trílogo nessas, por decisão.

create or replace view orcamentos_painel as
 SELECT id AS cliente_id,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'orcamento'::text AND d.oculto_em IS NULL) AS notas_arquivos,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'rateio'::text AND d.oculto_em IS NULL AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS rateio_sem_ticket,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text) AS a_lancar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND d.status = 'lido'::text AND d.fila = 'orcamento'::text AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS sem_ticket,
    ( SELECT count(*) AS count
           FROM documento_tickets t
             JOIN documentos d ON d.id = t.documento_id
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND t.chamado_id IS NULL
            AND d.fila <> 'direto'::text) AS sem_associacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'aguardando_aprovacao'::text) AS aguardando_aprovacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'removido'::text) AS apagados,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS no_total,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS valor_total,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND array_length(dl.ticket_soltos, 1) > 0) AS notas_travadas,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND dl.pronto_para_gerar) AS prontas_para_gerar,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text AND o.lancamento_bloqueio IS NOT NULL) AS recusados,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'pode_lancar'::text) AS prontos_para_lancar,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'cliente'::text) AS esperando_cliente,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'encarregados'::text) AS esperando_equipe,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL
            AND o.faturamento_direto = false) AS a_faturar,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL
            AND o.faturamento_direto = false) AS valor_a_faturar,
    -- daqui para baixo, o que a 023 acrescenta
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'direto'::text AND d.oculto_em IS NULL) AS notas_direto
   FROM clientes cl;

insert into schema_migrations (versao, arquivo)
values ('023', '023_faturamento_direto.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- nenhuma nota de faturamento direto pode aparecer na fila de faturar:
--   select count(*) from orcamentos
--    where fatura_id is null and status <> 'removido' and faturamento_direto;
--   -> o número existe, mas ele NÃO entra em a_faturar:
--   select a_faturar from orcamentos_painel limit 1;
--
--   -- e a fila nova aceita o valor novo:
--   insert into documentos (cliente_id, fila, nome_arquivo, arquivo_sha256, status)
--   values ('<cliente>', 'direto', 'teste.pdf', repeat('0',64), 'inserido');
--   -> aceita (e depois apague este teste)
-- =============================================================================
