-- =============================================================================
-- 041 — o que ainda não foi cobrado do cliente                          rev 1
-- =============================================================================
--
-- A REGRA, NAS PALAVRAS DO DONO (27/08/2026)
--
--   "todo orçamento inserido e não colocado em planilhas anteriores deve ser
--    colocado na planilha de agora"
--
--   Não é uma janela de datas. É o que ficou para trás, venha de quando vier:
--   um orçamento lançado em julho que ninguém cobrou aparece no relatório de
--   dezembro — e tem que aparecer, porque a alternativa é ele nunca ser cobrado.
--
-- POR QUE A RÉGUA É `fatura_id IS NULL`, E POR QUE ELA JÁ ESTAVA CERTA
--
--   Medido contra a planilha de julho que ele enviou: dos 247 tickets dela, 245
--   já tinham `fatura_id` no sistema. Os 6 que faltavam não eram buraco — eram
--   PARTES 2 e 3 de tickets cuja parte 1 fora cobrada (93523, 120586, 124822,
--   126400, 126474). A planilha lista o valor da PARTE, não a soma do ticket.
--
--   A régua está certa parte a parte, e essas seis entram no próximo relatório,
--   corretamente. Os outros 2 (120530 e 126119) são os que ficaram de fora da
--   carga do legado por não terem custo no Trílogo.
--
-- POR QUE UMA VIEW E NÃO UM FILTRO NO MOTOR
--
--   Porque "o que ainda não foi cobrado" é a definição de uma dívida, e ela vai
--   ser lida por mais de um lugar: o relatório, o contador do painel, e amanhã
--   o fechamento. Escrita três vezes, ela discorda de si mesma — foi assim que
--   a tela mostrou 31 e o botão disse 77.
--
-- O NOME DA LOJA É O DO CLIENTE, NÃO O NOSSO
--
--   `nome_cliente` vem da migração 040. Sem ele a linha sai em branco e o
--   cliente não consegue lançar em centro de custo nenhum — por isso a view
--   entrega os DOIS nomes, e a tela avisa antes de o arquivo sair.
--
-- POR QUE O `with (security_invoker = true)` ESTÁ AQUI (P-35)
--
--   `create or replace view` APAGA as opções da view — e esta view carrega
--   dinheiro a cobrar de um cliente. Ver a migração 033.
-- =============================================================================

create or replace view orcamentos_a_cobrar
with (security_invoker = true) as
 SELECT o.id,
    o.cliente_id,
    o.ticket,
    o.parte,
    -- O VALOR É O COBRADO, E É O ÚNICO QUE SAI DAQUI
    --   `valor_nota` não entra nem como coluna oculta: esta view alimenta o
    --   arquivo que vai AO CLIENTE, e a diferença entre os dois é a margem.
    o.valor,
    o.criado_em,
    o.conta,
    o.unidade_id,
    u.nome         AS loja,
    u.nome_cliente AS loja_cliente,
    u.cnpj         AS loja_cnpj
   FROM orcamentos o
     LEFT JOIN unidades u ON u.id = o.unidade_id
  WHERE o.status = 'lancado'::text
    AND o.fatura_id IS NULL
    -- O FATURAMENTO DIRETO NÃO É NOSSA COBRANÇA
    --   Nessa fila o fornecedor cobra o cliente direto; nós só lançamos o custo
    --   no Trílogo. Pôr essas linhas no relatório seria cobrar duas vezes o
    --   mesmo material, uma por nós e outra pela Rodrigues.
    AND o.faturamento_direto = false;

comment on view orcamentos_a_cobrar is
  'Os orçamentos LANÇADOS no Trílogo que nenhuma planilha anterior levou ao '
  'cliente (`fatura_id is null`). RODA COMO QUEM PERGUNTA (security_invoker) — '
  'se você reescrever esta view, REPITA a cláusula `with (security_invoker = '
  'true)`, senão a tranca cai em silêncio (migração 033). Só o valor COBRADO '
  'sai daqui: `valor_nota` revelaria a margem (migração 041).';

insert into schema_migrations (versao, arquivo)
values ('041', '041_o_que_ainda_nao_foi_cobrado.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select count(*) as quantos, sum(valor) as total,
--          min(criado_em)::date as de, max(criado_em)::date as ate
--     from orcamentos_a_cobrar;
--   -- 27/08/2026: 408 orçamentos, R$ 63.808,80, de 31/07 a 27/08
--
--   -- nenhuma linha pode sair sem o nome do cliente:
--   select loja, count(*) from orcamentos_a_cobrar
--    where loja_cliente is null group by loja;
--   -- esperado: nada (depois da 040)
--
--   -- e o que JÁ foi cobrado não pode aparecer aqui:
--   select count(*) from orcamentos_a_cobrar a
--    join orcamentos o on o.id = a.id where o.fatura_id is not null;
--   -- esperado: 0
--
--   -- a tranca de pé:
--   select relname, reloptions from pg_class where relname='orcamentos_a_cobrar';
--   -- esperado: {security_invoker=true}
--
-- PARA DESFAZER
--   drop view orcamentos_a_cobrar;
-- =============================================================================
