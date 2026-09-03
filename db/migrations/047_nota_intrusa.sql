-- =============================================================================
-- 047 — a nota que não deveria contar                                    rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (03/09/2026)
--
--   Ao ver os furos da 046 (notas do CSV sem ticket associado), apareceu uma
--   categoria diferente: nota que EXISTE no relatório do Obra Prima mas não é
--   gasto de manutenção nenhum — o exemplo real é a "5925 D" da RJ Consultoria
--   (Exames Médicos). Ela nunca vai ter ticket, e não deveria contar na
--   consolidação. Pedido, verbatim: "coloque a flag 'intruso' e não podemos
--   contabilizar na consolidação... uma opção de marcar a nota como intrusa
--   sempre que eu quiser (no final da linha)".
--
-- MARCA, NÃO APAGA
--
--   A nota continua em `obra_prima_notas` e continua na tela — só passa a
--   carregar um selo. Apagar a linha esconderia o rastro (o CSV importado de
--   verdade trouxe aquilo) e brigaria com o upsert por (doc,parc): reimportar
--   o mesmo CSV recriaria a linha do zero, perdendo a marca se ela morasse
--   ali. Por isso a marca mora numa tabela À PARTE
--   (`obra_prima_nota_intrusa`, chave só cliente_id+num): reimportar o CSV
--   nunca toca nela, e "marcar sempre que eu quiser" vira alternar uma linha
--   que existe ou não — sem migração de dado nenhuma no meio.
--
-- QUEM SOMA DECIDE, A VIEW SÓ AVISA
--
--   Mesma regra da 043 e da 046 ("este módulo não calcula nada"): a view
--   devolve `intrusa` cru; é a TELA que decide excluir a nota das somas do
--   rodapé quando `intrusa = true`. A nota continua NA LISTA (com o selo),
--   porque o dono pediu para poder desmarcar "sempre que quiser" — uma nota
--   escondida da tela não tem como ser desmarcada.
-- =============================================================================

create table public.obra_prima_nota_intrusa (
    cliente_id    uuid not null references public.clientes(id),
    num           text not null,
    marcado_em    timestamptz not null default now(),
    marcado_por   uuid references public.perfis(id),
    primary key (cliente_id, num)
);

comment on table public.obra_prima_nota_intrusa is
  'Marca manual: nota do Obra Prima que existe no CSV mas não é gasto de '
  'manutenção (ex.: exames médicos) e não deve contar na consolidação. '
  'Presença da linha = marcada. Tabela à parte de obra_prima_notas para '
  'sobreviver à reimportação do CSV (upsert por doc+parc nunca toca aqui).';

alter table public.obra_prima_nota_intrusa enable row level security;

create policy "marcas de intrusa do meu cliente" on public.obra_prima_nota_intrusa for select
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_FINANCEIRO_CONSOLIDACAO'));

-- -----------------------------------------------------------------------------
-- ABA 1, de novo — a mesma view da 046, só com `intrusa` a mais
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
    -- MARGEM SAI CRUA, A TELA FORMATA (mesma regra da 046)
    case when coalesce(o.orcado, 0) > 0
         then (o.orcado - n.valor) / o.orcado
         else null end                                        as margem,
    -- INTRUSA SAI CRUA TAMBÉM — ver cabeçalho desta migração
    (i.num is not null)                                       as intrusa,
    n.importado_em
  from (
      -- UMA LINHA POR NOTA, NÃO POR PARCELA (mesma regra da 046)
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
  left join lateral (
      select count(distinct pt.ticket)                              as quantos,
             array_agg(distinct pt.ticket order by pt.ticket)        as lista
        from public.obra_prima_ticket pt
       where pt.cliente_id = n.cliente_id
         and pt.num = n.num
  ) tk on true
  left join lateral (
      select sum(orc.valor) as orcado
        from public.orcamentos orc
       where orc.cliente_id = n.cliente_id
         and orc.removido_em is null
         and orc.ticket = any(coalesce(tk.lista, '{}'::integer[]))
  ) o on true
  left join public.obra_prima_nota_intrusa i
    on i.cliente_id = n.cliente_id and i.num = n.num;

comment on view consolidacao_notas is
  'Uma linha por NOTA DO OBRA PRIMA (Núm.): o que ela custou (`valor`), '
  'quantos tickets alguém já amarrou a ela (`tickets`, manual — ver '
  'obra_prima_ticket), quanto isso já virou orçamento aqui (`orcado`), a '
  'margem entre os dois, e se foi marcada como `intrusa` (manual — ver '
  'obra_prima_nota_intrusa: nota que não é gasto de manutenção e não deve '
  'contar na consolidação). RODA COMO QUEM PERGUNTA (security_invoker). '
  'Substituiu a versão da migração 046 — ver cabeçalho da 047.';

insert into public.schema_migrations (versao, arquivo)
values ('047', '047_nota_intrusa.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a tranca de pé (P-35):
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname = 'consolidacao_notas';
--   -- esperado: {security_invoker=true}
--
--   -- marcar/desmarcar não muda nada além da própria linha:
--   select count(*) from consolidacao_notas;
--   -- esperado: igual a antes desta migração (247 notas em 03/09/2026,
--   -- do Documentos_a_pagar_7.csv ainda não importado — confira o que o
--   -- banco tiver no momento)
--
-- PARA DESFAZER
--   drop view consolidacao_notas;
--   -- restaura a versão da 046 rodando o "create view" de lá de novo
--   -- (sem a coluna `intrusa`);
--   drop table public.obra_prima_nota_intrusa;
--   delete from public.schema_migrations where versao = '047';
-- =============================================================================
