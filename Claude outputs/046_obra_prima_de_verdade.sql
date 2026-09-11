-- =============================================================================
-- 046 — a nota do Obra Prima entra de verdade                            rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU
--
--   "vamos ajustar a tela de consolidacao agora.
--    1) Obra prima x FrotaHub
--    1.1) NF: nota integrada no sistema via csv
--    1.2) valor: valor da NF
--    1.3) tickets: todos os tickets associados àquela nota
--    1.4) orçado: o total orçado (somatorio dos orcamentos dos tickets daquela
--         nota). orçado apenas.
--    1.5) margem: (orçado - valor)/(orçado) (%)"
--
-- O FURO QUE A 043 JÁ TINHA MARCADO
--
--   O cabeçalho da migração 043 foi honesto: "a NF do Obra Prima ainda não está
--   aqui — enquanto o CSV não for importado, esta tela consolida o nosso lado
--   contra si mesmo... o furo que sobra é conhecido: R$ 2.103,55 no pacote 9192
--   e R$ 448,06 no 9220". É esse furo que esta migração fecha: a partir de
--   agora `consolidacao_notas` (ABA 1) lê a NOTA DE VERDADE — a que o Obra
--   Prima emitiu — em vez de reconstruir a mesma informação a partir dos
--   documentos que o FrotaHub já tinha lido sozinho.
--
--   Isso muda o GRÃO da view: de "uma linha por `documentos.id`" para "uma
--   linha por `Núm.` do Obra Prima" — que é exatamente a nota-pacote da
--   Rodrigues (9192, 9220, 9270...) que não existe 1:1 como `documentos` aqui.
--   `recebido`/`pendente` saem da view: não fazem sentido nesse grão novo, e o
--   lugar delas já é coberto por `orcado`/`margem`, que é o que o dono pediu.
--
-- O CSV, MEDIDO NA AMOSTRA REAL (Documentos_a_pagar_7.csv, 02/09/2026)
--
--   157 linhas, 154 notas distintas (3 notas vêm parceladas: 660575 em 3
--   parcelas, 19702 em 2). Colunas úteis: `Doc.` (nº interno do Obra Prima),
--   `Núm.` (nº da nota do FORNECEDOR — a chave desta view), `Fornecedor`,
--   `Bruto (R$)`, `Situação`, `Data Pgto.`, `Vlr. pago (R$)`, `Desc.`.
--
--   `Bruto (R$)` é o valor do DOCUMENTO — não da parcela. Medido nas duas
--   notas parceladas da amostra: as 3 linhas da 660575 trazem R$ 2.100,00
--   nas três, e as 2 linhas da 19702 trazem R$ 475,00 nas duas. Somar por
--   linha inflaria a nota parcelada por 2x ou 3x — por isso `valor` agrupa por
--   `num` e usa `max(bruto)`, e não `sum(bruto)`.
--
-- A GARANTIA CONTRA O VALOR QUE DISCORDA DE SI MESMO
--
--   Se um dia uma nota parcelada chegar com `Bruto` diferente entre as
--   parcelas — a fonte mudou de comportamento, ou a nota foi corrigida no meio
--   do parcelamento —, agrupar e pegar `max()` estaria ESCOLHENDO um valor
--   errado em silêncio. Por isso a garantia mora no IMPORTADOR (Go,
--   `obra_prima.go`), não na view: `parseObraPrima` recusa o arquivo INTEIRO
--   se achar duas linhas do mesmo `Núm.` com `Bruto` diferente, citando a nota
--   e os dois valores. A view, depois disso, pode confiar no `max()`.
--
-- A ASSOCIAÇÃO NOTA×TICKET, NESTA PRIMEIRA CARGA, É DE QUEM SABE
--
--   O CSV não traz CNPJ nem ticket — só o número da nota do fornecedor. Para a
--   Rodrigues esse número é uma nota-pacote (9192/9220/9270) que cobre vários
--   tickets ao mesmo tempo, e não existe regra automática ainda que resolva
--   isso (é o "depois, te mostro o sistema como será" combinado com o dono).
--   `obra_prima_ticket` existe para guardar essa associação manual — uma linha
--   por (nota, ticket) — sem tela própria por enquanto: as linhas desta
--   primeira carga entram por SQL direto, com o dono ditando quais são.
--
-- ⚠ `consolidacao_notas` MUDA DE FORMA (não é `create or replace`)
--   Postgres não deixa trocar a lista de colunas de uma view com
--   `create or replace` — e aqui a lista muda de verdade (sai `documento_id`,
--   `recebido`, `pendente`; entra `orcado`, `margem`). Por isso: `drop view` e
--   `create view` de novo, com `security_invoker = true` de novo (P-35 — a
--   tranca não sobrevive ao replace nem ao drop/create, tem que ser escrita
--   toda vez). `consolidacao_tickets` (ABA 2) NÃO muda nesta migração — o
--   pedido foi só do item 1, "Obra prima x FrotaHub".
-- =============================================================================

-- -----------------------------------------------------------------------------
-- obra_prima_notas — o que o CSV trouxe, uma linha por parcela
-- -----------------------------------------------------------------------------

create table public.obra_prima_notas (
    id             uuid primary key default gen_random_uuid(),
    cliente_id     uuid not null references public.clientes(id),
    doc            text not null,   -- "Doc.": nº interno do Obra Prima para este lançamento
    tipo           text,            -- "Tipo": Provisão, etc.
    num            text not null,   -- "Núm.": nº da nota do FORNECEDOR — a chave da consolidação
    parc           text not null default '1/1', -- "Parc.": "1/1", "2/3"...
    obra           text,            -- "Obra": centro de custo, texto livre do Obra Prima
    fornecedor     text not null,
    vencimento     date,
    bruto          numeric(14,2) not null,
    liquido        numeric(14,2),
    data_pagamento date,
    valor_pago     numeric(14,2),
    situacao       text,            -- "Liquidado" / "Aberto" / o que o Obra Prima mandar
    descricao      text,            -- "Desc.": às vezes traz os tickets escritos à mão
    arquivo        text not null,   -- nome do CSV de origem — auditoria de qual carga trouxe a linha
    importado_em   timestamptz not null default now(),
    atualizado_em  timestamptz not null default now(),
    unique (cliente_id, doc, parc)
);

comment on table public.obra_prima_notas is
  'Uma linha por PARCELA do relatório "documentos a pagar" do Obra Prima. '
  '`bruto` é o valor do DOCUMENTO inteiro (repetido em cada parcela, não '
  'somado) — ver o cabeçalho da migração 046. Reimportar o mesmo CSV atualiza '
  'as linhas (upsert por cliente_id+doc+parc), nunca duplica.';

create trigger obra_prima_notas_carimbo before update on public.obra_prima_notas
  for each row execute function tocar_atualizado_em();

-- -----------------------------------------------------------------------------
-- obra_prima_ticket — a associação manual nota × ticket (primeira carga)
-- -----------------------------------------------------------------------------

create table public.obra_prima_ticket (
    id           uuid primary key default gen_random_uuid(),
    cliente_id   uuid not null references public.clientes(id),
    num          text not null,   -- a mesma chave de obra_prima_notas.num
    ticket       integer not null,
    observacao   text,            -- de onde veio a certeza ("informado pelo dono, carga 1")
    criado_por   uuid references public.perfis(id),
    criado_em    timestamptz not null default now(),
    unique (cliente_id, num, ticket)
);

comment on table public.obra_prima_ticket is
  'Associação nota-do-Obra-Prima × ticket, feita À MÃO enquanto não existe '
  'mecanismo automático (nota-pacote da Rodrigues não bate 1:1 com documento '
  'nosso). Sem tela própria ainda — linhas entram por SQL direto, ditadas pelo '
  'dono. Uma nota pode cobrir vários tickets; um ticket pode aparecer em mais '
  'de uma nota (rateio ao contrário).';

-- -----------------------------------------------------------------------------
-- RLS: mesmo padrão do resto do sistema (documentado na 044) — a chave de
-- serviço do motor ignora RLS; estas políticas cobrem quem um dia ler direto
-- do Supabase, e documentam a intenção de quem lê o esquema.
-- -----------------------------------------------------------------------------

alter table public.obra_prima_notas enable row level security;
alter table public.obra_prima_ticket enable row level security;

create policy "notas do obra prima do meu cliente" on public.obra_prima_notas for select
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_FINANCEIRO_CONSOLIDACAO'));

create policy "associações do obra prima do meu cliente" on public.obra_prima_ticket for select
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_FINANCEIRO_CONSOLIDACAO'));

-- -----------------------------------------------------------------------------
-- ABA 1, de novo — OBRA PRIMA × FROTAHUB : agora uma linha por NOTA DE VERDADE
-- -----------------------------------------------------------------------------

drop view if exists consolidacao_notas;

create view consolidacao_notas
with (security_invoker = true) as
select
    n.cliente_id,
    n.num                                                     as nf,
    n.fornecedor,
    n.valor,
    n.situacao,
    n.parcelas,
    coalesce(tk.quantos, 0::bigint)                           as tickets,
    coalesce(tk.lista, '{}'::integer[])                       as ticket_numeros,
    coalesce(o.orcado, 0::numeric)                            as orcado,
    -- MARGEM SAI CRUA, A TELA FORMATA
    --   (orçado - valor) / orçado, sem multiplicar por 100 nem arredondar —
    --   é a mesma regra da migração 043: "este módulo não calcula nada (...)
    --   totalizar e formatar mora na tela". `null` (não zero) quando não há
    --   orçado ainda: zero diria "margem zero", que é uma afirmação, não a
    --   ausência de uma.
    case when coalesce(o.orcado, 0) > 0
         then (o.orcado - n.valor) / o.orcado
         else null end                                        as margem,
    n.importado_em
  from (
      -- UMA LINHA POR NOTA, NÃO POR PARCELA
      --   `max(bruto)` confia que todas as parcelas da mesma nota concordam no
      --   valor — quem garante isso é o importador Go (obra_prima.go), que
      --   recusa o arquivo inteiro antes de deixar uma divergência chegar
      --   aqui. Ver o cabeçalho desta migração.
      select cliente_id,
             num,
             max(bruto)                                        as valor,
             max(fornecedor)                                   as fornecedor,
             max(situacao)                                      as situacao,
             count(*)                                           as parcelas,
             max(importado_em)                                  as importado_em
        from public.obra_prima_notas
       group by cliente_id, num
  ) n
  -- OS TICKETS DA NOTA, CONTADOS UMA VEZ SÓ
  left join lateral (
      select count(distinct pt.ticket)                              as quantos,
             array_agg(distinct pt.ticket order by pt.ticket)        as lista
        from public.obra_prima_ticket pt
       where pt.cliente_id = n.cliente_id
         and pt.num = n.num
  ) tk on true
  -- O ORÇADO SOMA orcamentos DE VERDADE, PELO TICKET — NÃO PELA NOTA
  --   O pedido do dono foi explícito: "o total orçado (somatório dos
  --   orçamentos dos tickets daquela nota). orçado apenas." — soma o que foi
  --   orçado para o ticket, esteja ele ligado a esta nota, a outra, ou a
  --   nenhuma no FrotaHub.
  left join lateral (
      select sum(orc.valor) as orcado
        from public.orcamentos orc
       where orc.cliente_id = n.cliente_id
         and orc.removido_em is null
         and orc.ticket = any(coalesce(tk.lista, '{}'::integer[]))
  ) o on true;

comment on view consolidacao_notas is
  'Uma linha por NOTA DO OBRA PRIMA (Núm.): o que ela custou (`valor`, do '
  '`Bruto` do CSV), quantos tickets alguém já amarrou a ela (`tickets`, '
  'manual — ver obra_prima_ticket), quanto isso já virou orçamento aqui '
  '(`orcado`) e a margem entre os dois. RODA COMO QUEM PERGUNTA '
  '(security_invoker). Substituiu a versão da migração 043, que comparava o '
  'FrotaHub contra si mesmo por falta do CSV — ver cabeçalho da 046.';

insert into public.schema_migrations (versao, arquivo)
values ('046', '046_obra_prima_de_verdade.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a tranca de pé (P-35):
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname = 'consolidacao_notas';
--   -- esperado: {security_invoker=true}
--
--   -- depois de importar Documentos_a_pagar_7.csv (a amostra usada aqui):
--   select count(*), sum(valor) from consolidacao_notas;
--   -- esperado: 154 notas, R$ 84.849,01
--
--   -- nenhuma nota deveria discordar de si mesma entre parcelas — o
--   -- importador já barra isso, mas conferir direto no banco não custa nada:
--   select num, count(distinct bruto) as brutos_diferentes
--     from obra_prima_notas
--    group by cliente_id, num
--   having count(distinct bruto) > 1;
--   -- esperado: nenhuma linha
--
--   -- a aba 2 (consolidacao_tickets) não mudou nesta migração:
--   select count(*), sum(valor) from consolidacao_tickets;
--   -- esperado: igual ao medido na 043 (01/09/2026: 883 vivos)
--
-- PARA DESFAZER
--   drop view consolidacao_notas;
--   -- restaura a versão da 043 rodando o "create view" de lá de novo;
--   drop table public.obra_prima_ticket;
--   drop table public.obra_prima_notas;
--   delete from public.schema_migrations where versao = '046';
-- =============================================================================
