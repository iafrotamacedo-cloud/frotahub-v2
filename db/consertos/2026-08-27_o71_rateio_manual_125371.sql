-- =============================================================================
-- CONSERTO — o orçamento do ticket 125371 que ficou para trás     27/08/2026
-- =============================================================================
--
-- NÃO É MIGRAÇÃO. Não muda esquema, não entra em `schema_migrations`. É um
-- conserto de DADO, de um caso único, e roda uma vez só.
--
-- O QUE ACONTECEU
--
--   A NF 9160 foi rateada À MÃO, antes deste sistema existir, entre os tickets
--   125371 e 125372 — dois chamados irmãos da LOJA 02 OLIVEIRA PAIVA, abertos
--   no mesmo dia.
--
--   O 125372 recebeu a parte dele, virou orçamento no sistema antigo e foi
--   lançado no Trílogo (custo 36399, R$ 445,80). Veio para cá na carga do
--   legado e está correto.
--
--   O 125371 NUNCA teve orçamento. Nem no sistema antigo, nem aqui, nem custo
--   no Trílogo. A parte dele se perdeu no caminho.
--
-- A CONTA, ITEM A ITEM
--
--   A NF 9160 tem cinco itens, somando R$ 514,60:
--
--     TOR LAV LG 1/4V C70 1198 5/8"      4 × 83,90 = 335,60   -> foi ao 125372
--     PAINEL LED QUAD EMB 18W 6500K      1 × 17,90 =  17,90   -> foi ao 125372
--     CHUVEIRO CROMADO 4 FLAMINGO        4 × 29,90 = 119,60   -> FICOU PARA TRÁS
--     GRELHA ABRE FECHA INOX 100MM       4 ×  6,90 =  27,60   -> FICOU PARA TRÁS
--     GRELHA INOX 15X15 QUADRADA         1 × 13,90 =  13,90   -> FICOU PARA TRÁS
--
--   O que foi ao 125372:  335,60 + 17,90            = 353,50
--   O que ficou para trás: 119,60 + 27,60 + 13,90   = 161,10
--                                                     ------
--   A conta fecha:                                    514,60  = a nota inteira
--
--   O orçamento do 125371 é 161,10 × 1,20 = R$ 193,32.
--
-- POR QUE NÃO É `NT − O72` DIRETO
--
--   Porque o O72 traz uma FITA VEDA ROSCA de R$ 18,00 que NÃO está na NF 9160 —
--   ela veio de outra nota. `514,60 − 371,50` daria 143,10, descontando do
--   125371 dezoito reais que ele nunca recebeu. A conta correta é a dos ITENS,
--   que é o que o dono pediu, e é a única que fecha em 514,60.
--
--   Confirmado por ele em 27/08/2026: R$ 193,32.
--
-- O QUE ESTE ARQUIVO FAZ
--
--   1. cria o orçamento do 125371, parte 1, status `gerado`
--   2. grava os três itens que ficaram para trás, amarrados aos itens REAIS da
--      NF 9160 (`documento_item_id`) — o rastro fica de pé
--   3. amarra o orçamento à NF 9160 em `orcamento_documentos`
--   4. marca a NF 9160 como `usado`: ela está inteiramente contabilizada agora
--      (353,50 no legado + 161,10 aqui) e não tem mais nada a fazer na fila
--
--   O PDF NÃO precisa ser gerado aqui: o motor o monta na hora do lançamento, a
--   partir destes itens (`montarPDF`). Por isso os itens são o que importa.
--
-- É IDEMPOTENTE
--
--   Roda duas vezes sem duplicar nada. Se o orçamento do 125371 já existir, o
--   arquivo inteiro não faz nada e avisa.
--
-- PARA DESFAZER
--   ver o rodapé.
-- =============================================================================

do $$
declare
  v_cliente   uuid;
  v_chamado   uuid;
  v_unidade   uuid;
  v_conta     text;
  v_doc       uuid;
  v_orcamento uuid;
  v_soma      numeric;
begin
  -- ---------------------------------------------------------------------
  -- 1) de quem estamos falando, e as travas de segurança
  -- ---------------------------------------------------------------------
  select c.cliente_id, c.id, c.unidade_id, c.conta
    into v_cliente, v_chamado, v_unidade, v_conta
    from chamados c where c.numero = 125371;

  if v_chamado is null then
    raise exception 'o chamado 125371 não existe na base — nada a fazer';
  end if;

  select d.id into v_doc
    from documentos d where d.numero = '9160' and d.cliente_id = v_cliente;

  if v_doc is null then
    raise exception 'não achei a NF 9160 deste cliente';
  end if;

  -- IDEMPOTÊNCIA: se o 125371 já tem orçamento vivo, este arquivo já rodou.
  if exists (select 1 from orcamentos o
              where o.ticket = 125371 and o.cliente_id = v_cliente
                and o.status <> 'removido') then
    raise notice 'o ticket 125371 já tem orçamento — nada foi alterado';
    return;
  end if;

  -- TRAVA: o 125372 tem que estar como esperado. Se ele mudou desde a análise,
  -- a conta dos itens pode não ser mais essa — e aí é caso de olhar de novo,
  -- não de gravar por cima.
  if not exists (select 1 from orcamentos o
                  where o.ticket = 125372 and o.cliente_id = v_cliente
                    and o.valor = 445.80 and o.status = 'lancado') then
    raise exception 'o orçamento do 125372 não está em R$ 445,80 lançado — a conta '
                    'deste conserto foi calculada a partir dele; confira antes de rodar';
  end if;

  -- ---------------------------------------------------------------------
  -- 2) o orçamento que ficou para trás
  -- ---------------------------------------------------------------------
  insert into orcamentos (
      cliente_id, ticket, parte, chamado_id, unidade_id, conta,
      valor_nota, valor, margem_aplicada, teto_aplicado,
      reduzido_pelo_teto, ajustado_pelo_teto, rateio, status, criado_por)
  values (
      v_cliente, 125371, 1, v_chamado, v_unidade, v_conta,
      161.10, 193.32, 0.2000, 600.00,
      false, false,
      -- rateio = true: esta nota atende dois tickets, e é o que ela é.
      true, 'gerado',
      -- criado_por nulo: mesma decisão do legado — este registro não tem autor,
      -- ele repara uma ausência.
      null)
  returning id into v_orcamento;

  -- ---------------------------------------------------------------------
  -- 3) os três itens, amarrados aos itens REAIS da nota
  --
  --    `documento_item_id` é o que permite, daqui a um ano, abrir o orçamento e
  --    chegar na linha exata da nota que o originou. Sem ele, os valores viram
  --    números soltos que ninguém consegue reconferir.
  -- ---------------------------------------------------------------------
  insert into orcamento_itens (
      orcamento_id, ordem, descricao, unidade, quantidade,
      valor_unitario_nota, valor_unitario_cobrado, valor_total, documento_item_id)
  select v_orcamento,
         row_number() over (order by i.ordem),
         i.descricao, 'UN', i.quantidade,
         i.valor_unitario,
         round(i.valor_unitario * 1.20, 4),
         round(i.quantidade * i.valor_unitario * 1.20, 2),
         i.id
    from documento_itens i
   where i.documento_id = v_doc
     and i.descricao in ('CHUVEIRO CROMADO 4 FLAMINGO',
                         'GRELHA ABRE FECHA INOX QUADRADA 100MM - OVERTIME',
                         'GRELHA INOX 15X15 QUADRADA');

  -- A CONTA TEM QUE FECHAR, E O BANCO CONFERE — não o autor do script.
  select coalesce(sum(valor_total), 0) into v_soma
    from orcamento_itens where orcamento_id = v_orcamento;

  if v_soma <> 193.32 then
    raise exception 'os itens somaram % e o orçamento diz 193.32 — abortado sem gravar', v_soma;
  end if;

  -- ---------------------------------------------------------------------
  -- 4) o vínculo com a nota, e a nota encerrada
  -- ---------------------------------------------------------------------
  insert into orcamento_documentos (orcamento_id, documento_id)
  values (v_orcamento, v_doc);

  -- A NF 9160 está inteiramente contabilizada: 353,50 lá no legado (125372) e
  -- 161,10 aqui (125371). Ela sai da fila porque terminou, não porque sumiu.
  update documentos set status = 'usado' where id = v_doc;

  raise notice 'orçamento do 125371 criado: % — R$ 193,32, 3 itens, na fila de lançar', v_orcamento;
end $$;

-- =============================================================================
-- COMO CONFERIR
--
--   -- o orçamento nasceu e está na fila de lançar:
--   select ticket, parte, valor_nota, valor, status, destino, precisa_decisao, estado
--     from orcamentos_lista where ticket = 125371;
--   -- esperado: 161,10 · 193,32 · gerado · pode_lancar · false · pode-lancar
--
--   -- os três itens:
--   select ordem, descricao, quantidade, valor_unitario_nota, valor_unitario_cobrado, valor_total
--     from orcamento_itens i join orcamentos o on o.id = i.orcamento_id
--    where o.ticket = 125371 order by ordem;
--
--   -- A CONTA DA NOTA INTEIRA FECHA:
--   select (select sum(valor_total) from documento_itens i
--            join documentos d on d.id = i.documento_id where d.numero = '9160') as nota_toda,
--          371.50 - 18.00 as foi_ao_125372_desta_nota,
--          (select valor_nota from orcamentos where ticket = 125371) as ficou_no_125371;
--   -- esperado: 514,60 = 353,50 + 161,10
--
-- PARA DESFAZER
--   update documentos set status = 'lido' where numero = '9160';
--   delete from orcamentos where ticket = 125371 and legado_id is null;
--   -- (os itens e o vínculo saem em cascata)
-- =============================================================================
