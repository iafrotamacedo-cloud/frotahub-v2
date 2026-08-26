-- =============================================================================
-- 017 — o que já foi FATURADO AO CLIENTE, e o que já voltou dele        rev 1
-- =============================================================================
--
-- O QUE ESTA MIGRAÇÃO NÃO É
--
--   `orcamentos.faturado` e `orcamentos.pago` já existem e são do LADO DO
--   FORNECEDOR: a nota que compramos, o dinheiro que saiu. Nada aqui mexe
--   nelas. O que falta é o outro lado do balcão — a nota que EMITIMOS e o
--   dinheiro que ENTROU. Enquanto os dois lados não existirem separados, a
--   pergunta "quanto o cliente me deve" não tem resposta no banco.
--
-- COMO O CLIENTE FATURA (a regra dele, não a nossa)
--
--   Nós mandamos UMA planilha por mês. Ele abre um PCO (pedido de compra em
--   obra) para cada conta/loja/período, e cada PCO vira uma nota fiscal. São
--   34 lojas × 2 contas = até 68 por mês — mas "célula vazia não gera fatura":
--   loja sem orçamento naquele mês não vira nota. Em julho foram 54 células;
--   em agosto, 59.
--
-- TRÊS PEÇAS
--
--   faturamento_ciclos   o mês. Guarda o CORTE (`ate`) — foi ele que decidiu
--                        quem entrou. Sem isso, daqui a um ano ninguém explica
--                        por que um orçamento de 31/07 foi cobrado em agosto.
--
--   faturas              uma por (ciclo, loja, conta). É o PCO/NF, e é onde
--                        mora o que o cliente PAGOU.
--
--   orcamentos.fatura_id qual nota levou aquele orçamento. Nulo = ainda não
--                        foi cobrado — e é exatamente essa a fila do próximo
--                        mês, sem ninguém precisar lembrar de uma data.
--
-- POR QUE `fatura_id` E NÃO UM `ciclo_id` TAMBÉM
--
--   A fatura já sabe o ciclo dela. Duas colunas seriam duas verdades sobre a
--   mesma coisa, e o dia em que discordassem ninguém saberia qual valia
--   (CORE-06). O ciclo se alcança pela fatura.
--
-- POR QUE A FATURA GUARDA VALOR RECEBIDO MAS NÃO VALOR FATURADO
--
--   O faturado é a soma dos orçamentos dela — calcular é a única forma de ele
--   nunca discordar da composição. Já o RECEBIDO pode ser diferente do
--   faturado: pagamento parcial, glosa, desconto. É justamente essa diferença
--   que o controle existe para mostrar, então ela precisa caber no banco.
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. O ciclo — o mês, e o corte que o definiu
-- -----------------------------------------------------------------------------
create table if not exists faturamento_ciclos (
  id          uuid primary key default gen_random_uuid(),
  cliente_id  uuid not null references clientes(id) on delete restrict,
  competencia text not null,
  -- O corte. Entra no ciclo quem foi criado ANTES deste instante e ainda não
  -- tinha fatura. Guardado porque é o critério, não o resultado.
  ate         timestamptz not null,
  enviado_em  timestamptz,
  fechado_em  timestamptz,
  observacao  text,
  criado_em   timestamptz not null default now(),
  criado_por  uuid references perfis(id) on delete set null
);

comment on table faturamento_ciclos is
  'O mês de faturamento ao cliente. `ate` é o corte que decidiu quem entrou.';
comment on column faturamento_ciclos.competencia is
  'AAAA-MM. É o nome do período para gente, não o critério — o critério é `ate`.';

create unique index if not exists ciclo_por_competencia
  on faturamento_ciclos (cliente_id, competencia);


-- -----------------------------------------------------------------------------
-- 2. A fatura — uma por loja e conta dentro do ciclo
-- -----------------------------------------------------------------------------
create table if not exists faturas (
  id             uuid primary key default gen_random_uuid(),
  cliente_id     uuid not null references clientes(id) on delete restrict,
  ciclo_id       uuid not null references faturamento_ciclos(id) on delete cascade,
  unidade_id     uuid not null references unidades(id) on delete restrict,
  -- A mesma lista fechada de `orcamentos.conta`. Não há tabela de contas: são
  -- duas, e uma tabela de duas linhas seria cerimônia sem dono.
  conta          text not null check (conta in ('instalacoes', 'civil')),
  -- Os números vêm DEPOIS, do lado do cliente: primeiro ele abre o PCO, e só
  -- então a nota é emitida. Nascer nulo é o estado normal, não uma falha.
  pco_numero     text,
  nf_numero      text,
  nf_em          date,
  recebido_em    date,
  valor_recebido numeric(14,2),
  observacao     text,
  criado_em      timestamptz not null default now()
);

comment on table faturas is
  'Uma nota por loja e conta no ciclo — o PCO do cliente. O valor faturado é a soma dos orçamentos que apontam para ela.';

-- Duas faturas para a mesma loja, conta e ciclo seria cobrar duas vezes.
create unique index if not exists fatura_unica_no_ciclo
  on faturas (ciclo_id, unidade_id, conta);

create index if not exists fatura_por_recebimento
  on faturas (cliente_id, recebido_em);


-- -----------------------------------------------------------------------------
-- 3. O elo — e a fila do mês que vem
--
-- Nulo é o estado que importa: é ele que forma a próxima planilha. O índice é
-- parcial ao contrário do costume aqui — o que se procura é o vazio.
-- -----------------------------------------------------------------------------
alter table orcamentos add column if not exists fatura_id uuid;

do $$
begin
  if not exists (select 1 from pg_constraint where conname = 'orcamentos_fatura_id_fkey') then
    alter table orcamentos
      add constraint orcamentos_fatura_id_fkey
      foreign key (fatura_id) references faturas(id) on delete set null;
  end if;
end $$;

comment on column orcamentos.fatura_id is
  'A nota que levou este orçamento ao cliente. Nulo = ainda não foi cobrado. Não confundir com `faturado`/`pago`, que são do lado do FORNECEDOR.';

create index if not exists orcamento_a_faturar
  on orcamentos (cliente_id, criado_em)
  where fatura_id is null and status <> 'removido';

create index if not exists orcamento_por_fatura
  on orcamentos (fatura_id)
  where fatura_id is not null;


-- -----------------------------------------------------------------------------
-- 4. RLS
-- -----------------------------------------------------------------------------
alter table faturamento_ciclos enable row level security;
alter table faturas            enable row level security;

drop policy if exists "ciclos do meu cliente" on faturamento_ciclos;
create policy "ciclos do meu cliente"
  on faturamento_ciclos for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "faturas do meu cliente" on faturas;
create policy "faturas do meu cliente"
  on faturas for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_ORCAMENTOS'));


-- -----------------------------------------------------------------------------
-- 5. A view ganha duas colunas no fim
--
-- `create or replace view` no Postgres só ACRESCENTA coluna: nome e ordem das
-- que já existem estão congelados. Por isso este bloco é a 016 inteira, letra
-- por letra, com `unidade_id` e `fatura_id` no fim. Copiar a 015 — que era o
-- que eu tinha na cabeça — foi exatamente o erro que derrubou a 015 rev 1.
-- -----------------------------------------------------------------------------
create or replace view orcamentos_lista
with (security_invoker = true) as
select o.id, o.cliente_id, o.ticket, o.parte, o.conta, o.status,
       o.valor, o.valor_nota, o.reduzido_pelo_teto, o.valor_antes_do_teto,
       o.rateio, o.criado_em, o.lancado_em, o.faturado, o.pago,
       o.trilogo_custo_id, o.arquivo_pdf_sha256,
       u.nome as loja,
       c.descricao as chamado_descricao,
       (select string_agg(d.numero, ', ' order by d.numero)
          from orcamento_documentos od join documentos d on d.id = od.documento_id
         where od.orcamento_id = o.id and d.numero is not null) as notas,
       (select string_agg(d.dav_numero, ', ' order by d.dav_numero)
          from orcamento_documentos od join documentos d on d.id = od.documento_id
         where od.orcamento_id = o.id and d.dav_numero is not null) as davs,
       o.lancamento_bloqueio,
       o.lancamento_bloqueio_detalhe,
       o.lancamento_tentado_em,
       o.lancamento_tentativas,
       c.status        as ticket_status,
       c.status_codigo as ticket_status_codigo,
       (c.status_codigo in (1, 7) and r.ja_terminou) as reaberto,
       case when c.status_codigo in (1, 7) and r.ja_terminou
            then r.ultima_frase end as motivo_reabertura,
       case
         when o.status <> 'gerado'            then null
         when c.id is null                    then 'sem_chamado'
         when c.status_codigo in (5, 6)       then 'pode_lancar'
         when c.status_codigo = 3             then 'cliente'
         when c.status_codigo = 1 and r.ja_terminou then 'cliente'
         when c.status_codigo = 1             then 'encarregados'
         when c.status_codigo = 7             then 'encarregados'
         else 'outro'
       end as destino,
       (select a.avisado_em from ticket_avisos a
         where a.cliente_id = o.cliente_id and a.ticket = o.ticket
         order by a.avisado_em desc limit 1) as avisado_em,
       -- ---- a 017 ----
       o.unidade_id,
       o.fatura_id
from orcamentos o
left join unidades u on u.id = o.unidade_id
left join chamados c on c.id = o.chamado_id
left join lateral (
  select exists (
    select 1 from chamado_eventos e
     where e.chamado_id = c.id and e.tipo = 'status'
       and e.status_codigo in (3, 5, 6)
  ) as ja_terminou,
  (select e.texto from chamado_eventos e
    where e.chamado_id = c.id and e.tipo = 'status'
      and e.status_codigo in (1, 7)
      and coalesce(e.texto, '') <> ''
    order by e.quando desc limit 1) as ultima_frase
) r on true;


-- -----------------------------------------------------------------------------
-- 6. A conferência das faturas — o que foi cobrado e o que voltou
--
-- Uma linha por fatura, com a soma vinda dos orçamentos. É esta view que
-- responde "quanto o cliente ainda me deve".
-- -----------------------------------------------------------------------------
create or replace view faturas_lista
with (security_invoker = true) as
select f.id, f.cliente_id, f.ciclo_id,
       cl.competencia, cl.enviado_em, cl.fechado_em,
       f.unidade_id, u.nome as loja, f.conta,
       f.pco_numero, f.nf_numero, f.nf_em,
       f.recebido_em, f.valor_recebido, f.observacao,
       coalesce(s.quantos, 0)   as orcamentos,
       coalesce(s.valor, 0)     as valor,
       (f.recebido_em is not null) as recebida,
       coalesce(s.valor, 0) - coalesce(f.valor_recebido, 0) as diferenca
from faturas f
join faturamento_ciclos cl on cl.id = f.ciclo_id
join unidades u on u.id = f.unidade_id
left join lateral (
  select count(*) as quantos, sum(o.valor) as valor
    from orcamentos o
   where o.fatura_id = f.id and o.status <> 'removido'
) s on true;


-- -----------------------------------------------------------------------------
-- 7. A rotina — e quem já pode usá-la
--
-- Rotina nova sem ninguém marcado é um botão que não abre para pessoa nenhuma:
-- o motor recusa antes de olhar o dado, e a tela some. Por isso ela já nasce
-- marcada para toda categoria que hoje enxerga as planilhas de controle — que
-- é o público exato deste botão.
-- -----------------------------------------------------------------------------
insert into rotinas (codigo, nome, modulo, ordem) values
  ('CONTRATO_ORCAMENTOS_FATURAR', 'Faturar ao cliente', 'manutencao', 326)
on conflict (codigo) do nothing;

insert into categoria_permissoes (categoria_id, rotina, pode)
select cp.categoria_id, 'CONTRATO_ORCAMENTOS_FATURAR', cp.pode
  from categoria_permissoes cp
 where cp.rotina = 'CONTRATO_ORCAMENTOS_PLANILHAS'
on conflict (categoria_id, rotina) do nothing;


-- -----------------------------------------------------------------------------
-- 8. O painel ganha o contador do faturamento
--
-- Mesma regra da view acima: `create or replace` só ACRESCENTA. Este bloco é o
-- painel da 015 inteiro, na mesma ordem, com duas colunas no fim.
-- -----------------------------------------------------------------------------
create or replace view orcamentos_painel
with (security_invoker = true) as
select cl.id as cliente_id,
  (select count(*) from documentos d
     where d.cliente_id = cl.id and d.fila = 'orcamento' and d.oculto_em is null) as notas_arquivos,
  (select count(*) from documentos d
     where d.cliente_id = cl.id and d.fila = 'rateio' and d.oculto_em is null
       and not exists (select 1 from documento_tickets t where t.documento_id = d.id)) as rateio_sem_ticket,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'gerado') as a_lancar,
  (select count(*) from documentos d
     where d.cliente_id = cl.id and d.oculto_em is null and d.status = 'lido'
       and d.fila = 'orcamento'
       and not exists (select 1 from documento_tickets t where t.documento_id = d.id)) as sem_ticket,
  (select count(*) from documento_tickets t
     join documentos d on d.id = t.documento_id
     where d.cliente_id = cl.id and d.oculto_em is null and t.chamado_id is null) as sem_associacao,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'aguardando_aprovacao') as aguardando_aprovacao,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'removido') as apagados,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status <> 'removido') as no_total,
  (select coalesce(sum(o.valor), 0) from orcamentos o
     where o.cliente_id = cl.id and o.status <> 'removido') as valor_total,

  -- ---- as duas da 011. Reproduzidas aqui porque `create or replace view` exige
  -- ---- a lista INTEIRA, na mesma ordem: acrescentar é permitido, mexer não.
  -- ---- Foi assim que a rev 1 falhou — eu tinha copiado o painel da 010, que
  -- ---- não tem estas duas, e a 015 tentou renomear `notas_travadas`.

  (select count(*) from documentos_lista dl
     where dl.cliente_id = cl.id and dl.oculto_em is null
       and array_length(dl.ticket_soltos, 1) > 0) as notas_travadas,
  (select count(*) from documentos_lista dl
     where dl.cliente_id = cl.id and dl.oculto_em is null and dl.pronto_para_gerar) as prontas_para_gerar,

  -- ------- daqui para baixo é o que a 015 acrescenta -------
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'gerado'
       and o.lancamento_bloqueio is not null) as recusados,
  (select count(*) from orcamentos_lista l
     where l.cliente_id = cl.id and l.destino = 'pode_lancar')   as prontos_para_lancar,
  (select count(*) from orcamentos_lista l
     where l.cliente_id = cl.id and l.destino = 'cliente')       as esperando_cliente,
  (select count(*) from orcamentos_lista l
     where l.cliente_id = cl.id and l.destino = 'encarregados')  as esperando_equipe,

  -- ------- daqui para baixo é o que a 017 acrescenta -------
  -- A fila do faturamento é uma coluna VAZIA, não um mês: em julho a planilha
  -- fechou no dia 29 e a leva do dia 31 rolou para agosto. Se o critério fosse
  -- o mês, aqueles 39 orçamentos teriam sumido do faturamento para sempre.
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status <> 'removido'
       and o.fatura_id is null) as a_faturar,
  (select coalesce(sum(o.valor), 0) from orcamentos o
     where o.cliente_id = cl.id and o.status <> 'removido'
       and o.fatura_id is null) as valor_a_faturar
from clientes cl;


insert into schema_migrations (versao, arquivo)
values ('017', '017_faturamento_do_cliente.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select count(*) from orcamentos where fatura_id is null and status <> 'removido';
--     -> 636 antes da carga de julho, 391 depois
--
--   select * from faturas_lista order by competencia, loja, conta;
--
-- PARA DESFAZER
--   update orcamentos set fatura_id = null;
--   drop view faturas_lista;
--   drop table faturas cascade;  drop table faturamento_ciclos cascade;
--   alter table orcamentos drop column fatura_id;
--   (a orcamentos_lista volta rodando a 016 — mas ela é `create or replace`,
--    e replace NÃO remove coluna: para tirar as duas do fim, `drop view` antes.)
-- =============================================================================
