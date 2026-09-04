-- =============================================================================
-- 048 — a margem vira sobre o valor da nota, não sobre o orçado          rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (03/09/2026)
--
--   "preciso de inverter o percentual exibido: margem = (orcamento - valor)/
--   valor" — verbatim. Desde a migração 047 (e a 046 antes dela) a conta
--   dividia pelo ORÇADO:
--
--       margem = (orcado - valor) / orcado
--
--   e passa a dividir pelo VALOR DA NOTA:
--
--       margem = (orcado - valor) / valor
--
--   Não é só trocar o denominador por outro que dá o mesmo número: são
--   contas DIFERENTES, que respondem perguntas diferentes. A antiga dizia
--   "que fatia do que cobramos do cliente é lucro". A nova diz "quanto a
--   mais do que pagamos ao fornecedor cobramos do cliente" — a mesma
--   pergunta que a razão de 20% do resto do sistema já faz (ver
--   `financeiro-desenho.md`: "o orçamento sai de valor / valor_nota_cheio =
--   1,2000 exato"). Uma nota de R$100 orçada em R$120: pela conta antiga a
--   margem era 16,67%; pela nova é 20% — o número que bate com a margem
--   padrão do contrato.
--
-- A GUARDA CONTINUA SENDO "TEM ORÇADO", NÃO "VALOR DIFERENTE DE ZERO"
--
--   `null` continua significando "ainda não dá para calcular" (nenhum
--   ticket associado a esta nota — ver o cabeçalho da 046/047), não "a
--   margem é zero". Trocar o divisor não muda ESSA regra: uma nota sem
--   orçado nenhum continua saindo null, mesmo que o valor dela exista. O
--   `nullif` no denominador é só uma segunda trava, para o caso (hoje não
--   visto na base) de uma nota entrar com Bruto zerado — divisão por zero
--   vira erro de banco, e uma tela de dinheiro não pode cair por causa de
--   uma coluna decorativa.
--
-- QUEM SOMA DECIDE, A VIEW SÓ AVISA (mesma regra da 043/046/047)
-- =============================================================================

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
    -- MARGEM SAI CRUA, A TELA FORMATA (mesma regra da 046/047)
    -- MUDOU NA 048: divide pelo VALOR DA NOTA, não mais pelo orçado — ver o
    -- cabeçalho desta migração.
    case when coalesce(o.orcado, 0) > 0
         then (o.orcado - n.valor) / nullif(n.valor, 0)
         else null end                                        as margem,
    -- INTRUSA SAI CRUA TAMBÉM (migração 047)
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
  'margem — (orcado - valor) / valor, desde a migração 048 — e se foi '
  'marcada como `intrusa` (manual — ver obra_prima_nota_intrusa: nota que '
  'não é gasto de manutenção e não deve contar na consolidação). RODA COMO '
  'QUEM PERGUNTA (security_invoker). Substituiu a versão da migração 047 — '
  'ver cabeçalho da 048.';

insert into schema_migrations (versao, arquivo)
values ('048', '048_margem_sobre_valor.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a tranca de pé (P-35):
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname = 'consolidacao_notas';
--   -- esperado: {security_invoker=true}
--
--   -- a conta mudou de divisor, não de sinal nem de guarda:
--   select nf, valor, orcado, margem from consolidacao_notas
--    where orcado > 0
--    order by importado_em desc limit 5;
--   -- confira à mão: margem = (orcado - valor) / valor
--
-- PARA DESFAZER
--   drop view consolidacao_notas;
--   -- restaura a versão da 047 rodando o "create view" de lá de novo
--   -- (margem dividindo por `o.orcado`, sem o `nullif`);
--   delete from public.schema_migrations where versao = '048';
-- =============================================================================
