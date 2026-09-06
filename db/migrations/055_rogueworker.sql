-- =============================================================================
-- 055 — as duas tabelas da Rogue Worker                                  rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   A Rogue Worker cai no Groq quando nenhum comando pré-determinado reconhece
--   o pedido. Cada resolução dessas vira um CANDIDATO a comando fixo. Confirma
--   qualquer pessoa, na hora, na própria conversa — "isso ajudou?". Vira
--   comando de verdade só depois de N confirmações de pessoas DIFERENTES
--   (N mora no motor, hoje 3). A mesma pessoa confirmando várias vezes não
--   conta duas: a chave primária de `rogueworker_confirmacoes` é
--   (candidato_id, perfil_id).
--
-- POR QUE DUAS TABELAS, E NÃO UMA COM UM CONTADOR
--
--   Porque o contador esconderia QUEM confirmou, e aí a mesma pessoa
--   apertando "sim" três vezes promoveria sozinha. São duas perguntas: o que
--   foi resolvido (candidato) e quem topou lembrar (confirmação). Misturar as
--   duas faria a promoção depender de um número fácil de inflar.
--
-- NENHUMA ROTINA NOVA
--
--   A Rogue Worker reaproveita as rotinas que já existem. Estas tabelas não
--   entram no catálogo: quem conversa já está autenticado, e o que ela PODE
--   fazer é o que a categoria daquela pessoa já alcança no módulo tocado.
--
-- RLS LIGADA, NENHUMA POLÍTICA: o navegador não lê isto. Quem entrega é o
-- motor, depois de conferir quem está pedindo — o mesmo desenho de `historico`.
--
-- É segura de rodar duas vezes.
-- =============================================================================


create table if not exists rogueworker_candidatos (
  id uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id),
  pergunta_original text not null,
  rotina_resolvida text not null,
  comando_resolvido text not null,
  parametros jsonb not null default '{}',
  status text not null default 'candidato'
    check (status in ('candidato', 'promovido', 'descartado')),
  criado_em timestamptz not null default now()
);

comment on table rogueworker_candidatos is
  'Resolução da Rogue Worker que ainda não virou comando fixo. Promove sozinho '
  'quando N pessoas diferentes confirmam — ver rogueworker_confirmacoes.';

create index if not exists rogueworker_candidatos_por_cliente
  on rogueworker_candidatos (cliente_id, status, criado_em desc);


create table if not exists rogueworker_confirmacoes (
  candidato_id uuid not null references rogueworker_candidatos(id),
  perfil_id uuid not null references perfis(id),
  confirmado_em timestamptz not null default now(),
  primary key (candidato_id, perfil_id)
);

comment on table rogueworker_confirmacoes is
  'Quem disse que a resolução do candidato ajudou. A mesma pessoa não conta '
  'duas vezes — é a chave primária, não uma regra de aplicação.';

create index if not exists rogueworker_confirmacoes_por_candidato
  on rogueworker_confirmacoes (candidato_id);


alter table rogueworker_candidatos enable row level security;
alter table rogueworker_confirmacoes enable row level security;


insert into schema_migrations (versao, arquivo)
values ('055', '055_rogueworker.sql')
on conflict (versao) do nothing;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop table if exists rogueworker_confirmacoes;
-- drop table if exists rogueworker_candidatos;
-- delete from schema_migrations where versao = '055';
