-- =============================================================================
-- carga 5 — a fatura de julho/2026, o começo do controle do lado do cliente
-- =============================================================================
--
-- POR QUE JULHO É O MARCO ZERO
--
--   Julho é o único mês já faturado AO CLIENTE e pago por ele. Tudo que veio
--   antes está fora do controle porque nunca houve controle. Registrar julho é
--   o que faz agosto ser o primeiro mês medido pelo sistema, e não mais uma
--   planilha digitada à mão.
--
-- O CORTE SAIU DA PLANILHA, NÃO DE UM PALPITE
--
--   A planilha enviada ao cliente tem 247 linhas e R$ 35.334,37. Cruzando
--   linha a linha com o banco por (ticket, valor):
--
--     245 linhas casam com orçamentos nossos ............... R$ 34.849,09
--       2 linhas não existem no nosso banco ................ R$    485,28
--      39 orçamentos nossos ficaram DE FORA da planilha .... R$  4.463,41
--
--   E os 39 de fora são, todos os 39, de 31/07. A data mais recente que entrou
--   na planilha é 29/07. Ou seja: a fatura de julho fechou antes da leva do
--   dia 31, e ela rola para agosto. O corte é 30/07 — medido, não escolhido.
--
--   245 + 2 = 247 ✔   34.849,09 + 428,40 + 56,88 = 35.334,37 ✔
--
-- AS DUAS LINHAS QUE FALTAM
--
--   Tickets 120530 (R$ 428,40) e 126119 (R$ 56,88). São dois dos três casos que
--   ficaram fora da migração por não terem custo no Trílogo — e mesmo assim
--   foram cobrados do cliente e pagos. Não estão sendo inventados aqui: quando
--   entrarem no banco, entram no ciclo de julho, e a fatura fecha em 35.334,37.
--   Até lá, julho vale 34.849,09 no nosso lado e a diferença está escrita na
--   observação do ciclo, onde alguém a encontra.
--
-- O QUE ESTA CARGA NÃO INVENTA
--
--   Número de PCO, número de nota e data de recebimento nascem NULOS. Julho foi
--   pago — mas as datas e os números nunca foram registrados em lugar nenhum, e
--   preencher com data de hoje seria criar um registro falso num controle
--   financeiro. Ficam para a tela.
--
-- Roda quantas vezes quiser: o ciclo é único por competência, a fatura é única
-- por (ciclo, loja, conta), e o carimbo só toca em orçamento sem fatura.
-- =============================================================================

begin;

-- 1. O ciclo -----------------------------------------------------------------
insert into faturamento_ciclos (cliente_id, competencia, ate, fechado_em, observacao)
select o.cliente_id,
       '2026-07',
       timestamptz '2026-07-30 00:00:00+00',
       now(),
       'Faturado à mão, fora do sistema. A planilha enviada tinha 247 linhas e '
       || 'R$ 35.334,37; 245 delas (R$ 34.849,09) são os orçamentos deste ciclo. '
       || 'As outras duas — tickets 120530 (R$ 428,40) e 126119 (R$ 56,88) — foram '
       || 'cobradas e pagas, mas ainda não existem no banco: não têm custo no '
       || 'Trílogo e ficaram fora da migração. Foi pago integralmente; os números '
       || 'de PCO, de nota e as datas de recebimento não foram registrados na época.'
  from orcamentos o
 group by o.cliente_id
on conflict (cliente_id, competencia) do nothing;


-- 2. As faturas — uma por loja e conta que teve orçamento até o corte --------
--    Loja sem orçamento no mês não vira nota: é a regra "célula vazia não gera
--    fatura", escrita como a ausência de linha e não como uma linha zerada.
insert into faturas (cliente_id, ciclo_id, unidade_id, conta)
select distinct o.cliente_id, c.id, o.unidade_id, o.conta
  from orcamentos o
  join faturamento_ciclos c
    on c.cliente_id = o.cliente_id and c.competencia = '2026-07'
 where o.status <> 'removido'
   and o.fatura_id is null
   and o.criado_em < c.ate
   and o.unidade_id is not null
   and o.conta is not null
on conflict (ciclo_id, unidade_id, conta) do nothing;


-- 3. O carimbo ---------------------------------------------------------------
update orcamentos o
   set fatura_id = f.id
  from faturamento_ciclos c
  join faturas f on f.ciclo_id = c.id
 where c.cliente_id = o.cliente_id
   and c.competencia = '2026-07'
   and f.unidade_id = o.unidade_id
   and f.conta      = o.conta
   and o.status <> 'removido'
   and o.fatura_id is null
   and o.criado_em < c.ate;

commit;

-- =============================================================================
-- COMO CONFERIR — os quatro números que têm que bater
--
--   select count(*) as orcamentos, round(sum(valor),2) as valor,
--          count(distinct fatura_id) as faturas
--     from orcamentos
--    where fatura_id is not null;
--   -> 245 | 34849.09 | 54
--
--   select count(*) as a_faturar, round(sum(valor),2) as valor
--     from orcamentos where fatura_id is null and status <> 'removido';
--   -> 391 | 62456.60
--
--   -- e nenhum orçamento de 30/07 em diante pode ter entrado em julho:
--   select count(*) from orcamentos o
--     join faturas f on f.id = o.fatura_id
--     join faturamento_ciclos c on c.id = f.ciclo_id
--    where c.competencia = '2026-07' and o.criado_em >= c.ate;
--   -> 0
-- =============================================================================
