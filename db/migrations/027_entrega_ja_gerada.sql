-- =============================================================================
-- 027 — a entrega dos orçamentos já gerados                              rev 1
-- =============================================================================
--
-- O QUE FALTOU NA 026
--
--   A regra que devolve a linha de entrega ao direito — uma entrega custando o
--   total, no lugar de "12 unidades de serviço de entrega" — foi escrita no
--   ponto em que a NOTA vira linha de orçamento (`itensDo`). Isso conserta tudo
--   que for gerado daqui para frente e não encosta no que já existe.
--
--   E o que já existe está gravado: `orcamento_itens` guarda a linha como ela
--   foi decidida na geração. O PDF é desenhado a partir dela, não da nota. Ou
--   seja: os orçamentos gerados antes continuam imprimindo 10 × R$ 1,20, e vão
--   continuar para sempre, porque nada os relê.
--
--   Eu disse ao dono que "o PDF é montado na hora, não há o que voltar". Era
--   verdade para o DESENHO e falso para os VALORES. Esta migração é a metade que
--   faltou.
--
-- O DINHEIRO NÃO MUDA
--
--   12 × R$ 1,20 e 1 × R$ 14,40 somam o mesmo. `valor_total` não é tocado, e é
--   dele que sai o total do orçamento. Muda como a linha se lê.
--
-- O QUE ESTA MIGRAÇÃO NÃO TOCA, E POR QUÊ
--
--   Orçamentos do LEGADO (93 linhas, 82 já lançadas). O PDF deles está no
--   Dropbox e o anexo já subiu ao Trílogo no formato antigo. Reescrever a linha
--   aqui faria o nosso registro discordar do papel que já saiu — e o papel é o
--   que o cliente tem na mão. Registro que discorda do documento emitido é pior
--   que registro feio.
--
--   Orçamentos LANÇADOS, quaisquer que sejam. Mesma razão: o arquivo já está no
--   sistema do cliente.
-- =============================================================================

update orcamento_itens i
   set quantidade             = 1,
       valor_unitario_nota    = i.quantidade * i.valor_unitario_nota,
       valor_unitario_cobrado = i.valor_total
  from orcamentos o
 where o.id = i.orcamento_id
   and o.legado_id is null            -- só o que este sistema gerou
   and o.status = 'gerado'            -- nada que já saiu daqui
   and i.quantidade > 1
   and i.valor_unitario_nota = 1      -- a convenção do fornecedor, e só ela
   and upper(translate(i.descricao, 'ÇÃÁÉÍÓÚÂÊÔÕ', 'CAAEIOUAEOO')) like '%ENTREGA%';

insert into schema_migrations (versao, arquivo)
values ('027', '027_entrega_ja_gerada.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- nenhuma linha nossa, ainda por lançar, com entrega em quantidade:
--   select count(*) from orcamento_itens i join orcamentos o on o.id = i.orcamento_id
--    where o.legado_id is null and o.status = 'gerado' and i.quantidade > 1
--      and upper(translate(i.descricao,'ÇÃÁÉÍÓÚÂÊÔÕ','CAAEIOUAEOO')) like '%ENTREGA%';
--   -- esperado: 0
--
--   -- e o total de cada orçamento continua o mesmo da soma das linhas:
--   select count(*) from orcamentos o
--    where o.status = 'gerado' and o.legado_id is null
--      and o.valor <> (select coalesce(sum(valor_total),0) from orcamento_itens
--                       where orcamento_id = o.id);
--   -- esperado: 0
--
-- PARA DESFAZER
--   Não há volta automática: a quantidade original era o valor em reais, e ela
--   se recupera do próprio total (quantidade = valor_total / 1,20). Se precisar:
--
--   update orcamento_itens i set quantidade = round(i.valor_total / 1.20, 4),
--          valor_unitario_nota = 1, valor_unitario_cobrado = 1.20
--     from orcamentos o
--    where o.id = i.orcamento_id and o.legado_id is null and o.status = 'gerado'
--      and i.quantidade = 1
--      and upper(translate(i.descricao,'ÇÃÁÉÍÓÚÂÊÔÕ','CAAEIOUAEOO')) like '%ENTREGA%';
-- =============================================================================
