-- =============================================================================
-- 050 — Candidatos e o Kanban de Serviço                                 rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   1. `servicos_candidatos` — a pré-triagem. Todo chamado NOVO que o Groq (ou
--      a heurística) julga "isto pode ser Serviço, não conserto" cai aqui,
--      pendente. Fica isolado da fila de verdade de propósito: nada aqui
--      escreve no Trílogo, é só sugestão pra um humano decidir.
--   2. `servicos_orcamentos` — o Kanban de verdade. Uma linha por chamado que
--      JÁ ENTROU na fila de Serviço (responsável = Serviço Instalações/Civil,
--      pelo gatilho automático OU pelo botão). O `status` é a coluna onde o
--      cartão está.
--   3. `robo_execucoes.modo` ganha 'candidatos' — o job agendado que classifica
--      candidatos e detecta o gatilho automático usa a MESMA trava de
--      concorrência e o mesmo relógio d'água que o robô do Trílogo já usa,
--      só que com `robo='servicos'`.
--   4. `servicos_inconsistencias` — a view que ALERTA (não bloqueia) quando um
--      chamado em Serviço recebeu custo pelo ciclo de Materiais do contrato.
--
-- POR QUE DUAS TABELAS, E NÃO UMA COM UM STATUS "candidato"
--
--   Porque são DUAS PERGUNTAS de dono diferente. Candidatos pergunta "será que
--   isto é Serviço?" — resposta de máquina, provisória, descartável. Kanban
--   pergunta "em que pé está ESTE serviço?" — fato, uma vez que o responsável
--   foi trocado de verdade no Trílogo. Misturar as duas faria um candidato
--   descartado (não é Serviço) e um serviço de verdade competirem pela mesma
--   coluna, e a tela teria que adivinhar qual é qual.
--
-- STATUS COMO TEXT + CHECK, NÃO ENUM
--
--   Mesma escolha de `orcamentos.status` (010_orcamentos.sql): trocar o
--   catálogo de status é um ALTER CONSTRAINT (ver a 012, que fez isto pra
--   `robo_execucoes.modo`), não uma migração de tipo.
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. servicos_candidatos
-- -----------------------------------------------------------------------------
--
-- `unique(chamado_id)`: um chamado só passa pela triagem UMA VEZ. Descartado é
-- definitivo (o dono foi claro: "não volta pra fila de candidatos") — e
-- resolvido (virou serviço de verdade pelo gatilho) também não teria por que
-- voltar a ser candidato de novo.

create table servicos_candidatos (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,
  chamado_id uuid not null references chamados(id) on delete cascade,

  -- Denormalizado por conveniência de leitura — mesmo padrão de
  -- `orcamentos.ticket` ao lado de `chamado_id` (010_orcamentos.sql).
  ticket integer not null,

  -- O texto que o Groq devolveu explicando o palpite. Curto, pra tela mostrar
  -- por que ELE achou que era Serviço — não é auditoria, é contexto.
  motivo text,

  status text not null default 'pendente'
    check (status in ('pendente', 'descartado', 'resolvido')),

  criado_em   timestamptz not null default now(),
  decidido_em timestamptz,
  -- Nulo quando `resolvido` chegou sozinho, pelo gatilho automático — só tem
  -- autor humano quando alguém decidiu na tela (descartar, ou mandar pra fila
  -- direto do card de Candidatos).
  decidido_por uuid,

  unique (chamado_id)
);

comment on table servicos_candidatos is
  'Pré-triagem de Serviço: chamados novos que o classificador (Groq) suspeita '
  'não serem manutenção comum. status=pendente aguarda decisão humana; '
  'descartado é definitivo; resolvido quer dizer que o chamado já entrou de '
  'verdade na fila de Serviço (servicos_orcamentos), por qualquer caminho.';

create index servicos_candidatos_pendentes
  on servicos_candidatos (cliente_id, criado_em desc)
  where status = 'pendente';


-- -----------------------------------------------------------------------------
-- 2. servicos_orcamentos — o Kanban
-- -----------------------------------------------------------------------------
--
-- ÍNDICE ÚNICO PARCIAL, NÃO UNIQUE NA COLUNA
--
--   Um chamado pode entrar em Serviço, ser reclassificado de volta pro
--   contrato (removido_em preenchido) e entrar de novo depois — o dono foi
--   claro que a troca de fila tem que valer nos dois sentidos, sempre. A
--   unicidade vale só ENQUANTO a linha está ativa; a antiga fica de pé como
--   histórico (nada é apagado).

create table servicos_orcamentos (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,
  chamado_id uuid not null references chamados(id) on delete cascade,

  ticket integer not null,
  conta  text not null check (conta in ('instalacoes', 'civil')),

  -- As dez colunas do Kanban, nos três cartões.
  --   Orçamento:  aguardando_orcamento -> orcamento_feito -> orcamento_lancado
  --               -> orcamento_aprovado | orcamento_rejeitado
  --   Execução:   aprovado_execucao -> em_execucao -> finalizado
  --   Financeiro: aguardando_faturamento -> faturado
  -- `orcamento_rejeitado` NÃO tem saída automática — fica parado até ação
  -- manual (um orçamento novo, ou reclassificar de volta pro contrato). É
  -- decisão do dono: "o fluxo tem que fechar sempre", mas fechar não é
  -- sinônimo de andar sozinho.
  status text not null default 'aguardando_orcamento' check (status in (
    'aguardando_orcamento', 'orcamento_feito', 'orcamento_lancado',
    'orcamento_aprovado', 'orcamento_rejeitado',
    'aprovado_execucao', 'em_execucao', 'finalizado',
    'aguardando_faturamento', 'faturado'
  )),

  -- Os ids do lado de lá — para achar de volta a cotação/orçamento no
  -- Trílogo sem ter que procurar (ver trilogo.Cotacoes / trilogo.Orcamento).
  cotacao_trilogo_id   integer,
  orcamento_trilogo_id integer,
  orcamento_valor      numeric(14,2),

  -- Como o chamado entrou: 'gatilho' (responsável mudou no Trílogo, direto ou
  -- pelo botão) ou 'candidato' (o dono aprovou um card de servicos_candidatos
  -- e o sistema mandou pra fila). Não muda o resultado, só explica a origem.
  origem text not null default 'gatilho' check (origem in ('gatilho', 'candidato')),

  entrou_em     timestamptz not null default now(),
  atualizado_em timestamptz not null default now(),

  removido_em     timestamptz,
  removido_por    uuid,
  removido_motivo text
);

comment on table servicos_orcamentos is
  'O Kanban de Serviço: uma linha por chamado que já está de verdade na fila '
  '(responsável = Serviço Instalações/Civil no Trílogo). status é a coluna. '
  'removido_em = reclassificado de volta pro contrato (a linha fica, como '
  'histórico — a fila ativa é sempre "where removido_em is null").';

create unique index servicos_orcamentos_ativo
  on servicos_orcamentos (chamado_id)
  where removido_em is null;

create index servicos_orcamentos_por_status
  on servicos_orcamentos (cliente_id, status, atualizado_em desc)
  where removido_em is null;

create trigger servicos_orcamentos_carimbo
  before update on servicos_orcamentos
  for each row execute function tocar_atualizado_em();


-- -----------------------------------------------------------------------------
-- 2.1 A fila de quem ainda não foi avaliado
-- -----------------------------------------------------------------------------
--
-- O job de Candidatos (cmd/servicos) pergunta "quem falta classificar?" — e a
-- resposta não é um filtro simples: é "chamados que NÃO estão em nenhuma das
-- duas tabelas de decisão, e são recentes o bastante para importar". Uma
-- view resolve isto num lugar só (CORE-06), em vez de o Go montar um NOT IN
-- por cima de duas consultas.
--
-- A DATA DE CORTE É OUTRA, DE PROPÓSITO
--
--   03/09/2026, e não a DataDeCorte geral do robô (01/07/2026, que é sobre
--   QUANDO A LEITURA DO TRÍLOGO COMEÇA). Esta é sobre a partir de quando o
--   Kanban de Serviço passa a valer — decisão do dono. Chamado antigo nunca
--   vira card de Candidatos, mesmo que o robô já o tenha lido há semanas.

create view servicos_a_classificar
with (security_invoker = true) as
select c.id as chamado_id, c.cliente_id, c.numero as ticket,
       c.descricao, c.conta, c.criado_em
  from chamados c
 where c.criado_em >= '2026-09-03'
   and not exists (
     select 1 from servicos_candidatos sc where sc.chamado_id = c.id
   )
   and not exists (
     select 1 from servicos_orcamentos so
      where so.chamado_id = c.id and so.removido_em is null
   );

comment on view servicos_a_classificar is
  'Chamados desde 03/09/2026 que ainda não passaram por nenhuma decisão de '
  'Serviço (nem candidato, nem já na fila de verdade) — é a fila que o job '
  'de Candidatos (cmd/servicos) classifica a cada rodada.';


-- -----------------------------------------------------------------------------
-- 3. O job agendado ganha um modo no relógio compartilhado
-- -----------------------------------------------------------------------------

alter table robo_execucoes
  drop constraint if exists robo_execucoes_modo_check;

alter table robo_execucoes
  add constraint robo_execucoes_modo_check
  check (modo in ('levantamento', 'copia', 'atualizacao', 'alvos', 'candidatos'));


-- -----------------------------------------------------------------------------
-- 4. servicos_inconsistencias — o alerta, não o bloqueio
-- -----------------------------------------------------------------------------
--
-- A decisão do dono foi clara: não dá pra travar o lançamento de custo do
-- lado do Trílogo, e travar do nosso lado seria travar sem o Trílogo saber —
-- o custo entraria lá do mesmo jeito se alguém usasse a tela deles direto.
-- Então isto MONITORA: todo orçamento do ciclo de contrato (`orcamentos`)
-- lançado DEPOIS de o chamado ter entrado na fila de Serviço, pra alguém
-- revisar e decidir o que fazer.

create view servicos_inconsistencias
with (security_invoker = true) as
select
    so.id                as servico_id,
    so.cliente_id,
    so.chamado_id,
    so.ticket,
    so.status             as status_do_servico,
    so.entrou_em          as servico_entrou_em,
    orc.id                as orcamento_id,
    orc.valor             as orcamento_valor,
    orc.status             as orcamento_status,
    orc.criado_em          as orcamento_criado_em
  from servicos_orcamentos so
  join orcamentos orc
    on orc.cliente_id = so.cliente_id
   and orc.ticket = so.ticket
   and orc.removido_em is null
 where so.removido_em is null
   and orc.criado_em > so.entrou_em;

comment on view servicos_inconsistencias is
  'ALERTA, não bloqueio (decisão do dono, 04/09/2026): tickets que estão na '
  'fila de Serviço e mesmo assim receberam orçamento pelo ciclo de Materiais '
  'do contrato DEPOIS de entrar. Não impede nada no Trílogo — só avisa pra '
  'alguém revisar a inconsistência.';


insert into schema_migrations (versao, arquivo)
values ('050', '050_candidatos_e_kanban_de_servico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select conname, pg_get_constraintdef(oid) from pg_constraint
--    where conname = 'robo_execucoes_modo_check';
--   -- esperado: inclui 'candidatos'
--
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname = 'servicos_inconsistencias';
--   -- esperado: {security_invoker=true}
--
--   -- as duas tabelas nascem vazias:
--   select count(*) from servicos_candidatos;
--   select count(*) from servicos_orcamentos;
--   -- esperado: 0 e 0
--
-- PARA DESFAZER
--   drop view servicos_inconsistencias;
--   drop view servicos_a_classificar;
--   alter table robo_execucoes drop constraint robo_execucoes_modo_check;
--   alter table robo_execucoes add constraint robo_execucoes_modo_check
--     check (modo in ('levantamento', 'copia', 'atualizacao', 'alvos'));
--   drop table servicos_orcamentos;
--   drop table servicos_candidatos;
--   delete from schema_migrations where versao = '050';
-- =============================================================================
