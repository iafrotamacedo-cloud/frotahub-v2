-- =============================================================================
-- 029 — onde cada nota mora                                              rev 1
-- =============================================================================
--
-- O QUE ESTAVA ESPALHADO
--
--   A lista de Notas e DAVs, o contador do painel e as duas telas de Correções
--   perguntavam cada uma à sua maneira "esta nota é minha?". Três respostas para
--   a mesma pergunta é exatamente como nasceu o defeito de 26/08/2026: a lista
--   abrindo com 31 e o botão dizendo 77.
--
--   Agora a resposta é uma coluna: `onde`. Quem quiser saber, pergunta.
--
-- O PEDIDO QUE MOTIVOU
--
--   "notas que vão para sem ticket e não associadas também saem da fila e vão
--   para a fila do conserto. notas que exigem conferência ficam na fila, não vão
--   para lugar nenhum, mas têm que abrir na tela para conserto ali mesmo."
--
--   Então a fila deixa de ser "tudo que não virou orçamento" e passa a ser o que
--   se resolve ALI: o que ainda não foi lido, o que falhou na leitura, o que
--   está repetido, o que está bloqueado, e o que está pronto para gerar.
--
-- REPETIDA E BLOQUEADA FICAM NA FILA MESMO SEM TICKET
--
--   O impedimento delas não é o ticket. Mandá-las para "sem ticket" faria a
--   pessoa amarrar um número para descobrir, depois, que a nota nunca ia gerar
--   porque já entrou antes. A razão real tem que aparecer primeiro.
--
-- `create or replace view` SÓ ACRESCENTA COLUNA
--
--   Nome, ordem e tipo do que já existe ficam congelados; colunas novas entram
--   no fim. `onde` e `motivo_conferencia` são as duas últimas, e nada acima foi
--   tocado.
-- =============================================================================

create or replace view documentos_lista as
 SELECT d.id,
    d.cliente_id,
    d.fila,
    d.tipo,
    d.numero,
    d.dav_numero,
    d.chave_acesso,
    d.emitente_nome,
    d.emissao,
    d.valor_total,
    d.status,
    d.leitura_camada,
    d.leitura_confianca,
    d.nome_arquivo,
    d.arquivo_sha256,
    d.inserido_em,
    d.oculto_em,
    COALESCE(t.quantos, 0::bigint) AS tickets,
    COALESCE(t.lista, '{}'::integer[]) AS ticket_numeros,
    COALESCE(i.quantos, 0::bigint) AS itens,
    COALESCE(t.soltos, '{}'::integer[]) AS ticket_soltos,
    d.status = 'lido'::text AND COALESCE(t.quantos, 0::bigint) > 0 AND COALESCE(array_length(t.soltos, 1), 0) = 0 AND d.duplicada_de IS NULL AND d.bloqueio_motivo IS NULL AND d.aprovacao_pedida IS NOT TRUE AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo,
    d.desconto_bp,
    d.desconto_em,
    d.aprovacao_pedida,
    d.aprovacao_pedida_em,
    -- ONDE ESTA NOTA MORA — a regra escrita UMA vez
    --
    --   A lista, o contador do painel e as telas de Correções perguntavam cada
    --   uma à sua maneira "esta nota é minha?". Três respostas para a mesma
    --   pergunta é como nasce a tela que mostra 31 e o botão que diz 77.
    --
    --   'fila'           — está em Notas e DAVs e precisa de alguém aqui
    --   'sem-ticket'     — vive em Correções › Sem ticket
    --   'sem-associacao' — vive em Correções › Sem associação
    --   'usada'          — virou orçamento; saiu do caminho
    CASE
        WHEN d.status = 'usado'::text THEN 'usada'::text
        -- Ainda não lida, ou a leitura falhou: é trabalho da fila, não de
        -- Correções — não há ticket para consertar enquanto não se lê.
        WHEN d.status <> 'lido'::text THEN 'fila'::text
        -- Repetida e bloqueada ficam na fila mesmo sem ticket: o impedimento
        -- delas é outro, e mandá-las para "sem ticket" esconderia a razão real.
        WHEN d.duplicada_de IS NOT NULL THEN 'fila'::text
        WHEN d.bloqueio_motivo IS NOT NULL THEN 'fila'::text
        WHEN COALESCE(t.quantos, 0::bigint) = 0 THEN 'sem-ticket'::text
        WHEN COALESCE(array_length(t.soltos, 1), 0) > 0 THEN 'sem-associacao'::text
        ELSE 'fila'::text
    END AS onde,
    -- POR QUE ESTA NOTA PRECISA DE GENTE
    --
    --   A frase que a tela mostra acima do papel. Nasce aqui, e não no
    --   navegador, para que a lista, a tela cheia e qualquer relatório digam
    --   exatamente a mesma coisa.
    CASE
        WHEN d.status = 'falhou'::text THEN COALESCE(NULLIF(d.leitura_erro, ''::text), 'a leitura desta nota falhou')
        WHEN d.duplicada_de IS NOT NULL THEN 'esta nota já entrou antes — confira antes de gerar'
        WHEN d.bloqueio_motivo IS NOT NULL THEN d.bloqueio_motivo
        WHEN d.aprovacao_pedida IS TRUE THEN 'aprovação pedida ao cliente — esperando a resposta'
        ELSE NULL::text
    END AS motivo_conferencia
   FROM documentos d
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos,
            array_agg(dt.ticket ORDER BY dt.ticket) AS lista,
            array_remove(array_agg(
                CASE
                    WHEN dt.chamado_id IS NULL THEN dt.ticket
                    ELSE NULL::integer
                END ORDER BY dt.ticket), NULL::integer) AS soltos
           FROM documento_tickets dt
          WHERE dt.documento_id = d.id) t ON true
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos
           FROM documento_itens di
          WHERE di.documento_id = d.id) i ON true
     LEFT JOIN documentos o ON o.id = d.duplicada_de;

-- E o contador do painel passa a contar a MESMA coisa que a lista mostra.
-- A 028 já o tinha aproximado (tirando as usadas); agora ele usa a regra em vez
-- de repeti-la pela metade.
create or replace view orcamentos_painel as
 SELECT id AS cliente_id,
    ( SELECT count(*) AS count
           FROM documentos_lista d
          WHERE d.cliente_id = cl.id AND d.fila = 'orcamento'::text AND d.oculto_em IS NULL AND d.onde = 'fila'::text) AS notas_arquivos,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'rateio'::text AND d.oculto_em IS NULL AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS rateio_sem_ticket,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text) AS a_lancar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND d.status = 'lido'::text AND d.fila = 'orcamento'::text AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS sem_ticket,
    ( SELECT count(*) AS count
           FROM documento_tickets t
             JOIN documentos d ON d.id = t.documento_id
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND t.chamado_id IS NULL AND d.fila <> 'direto'::text) AS sem_associacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'aguardando_aprovacao'::text) AS aguardando_aprovacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'removido'::text) AS apagados,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS no_total,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS valor_total,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND array_length(dl.ticket_soltos, 1) > 0) AS notas_travadas,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND dl.pronto_para_gerar) AS prontas_para_gerar,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text AND o.lancamento_bloqueio IS NOT NULL) AS recusados,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'pode_lancar'::text) AS prontos_para_lancar,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'cliente'::text) AS esperando_cliente,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'encarregados'::text) AS esperando_equipe,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL AND o.faturamento_direto = false) AS a_faturar,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL AND o.faturamento_direto = false) AS valor_a_faturar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'direto'::text AND d.oculto_em IS NULL) AS notas_direto
   FROM clientes cl;

insert into schema_migrations (versao, arquivo)
values ('029', '029_onde_cada_nota_mora.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select onde, count(*) from documentos_lista
--    where oculto_em is null and fila = 'orcamento'
--    group by onde order by onde;
--
--   -- o contador tem que bater com a lista:
--   select (select notas_arquivos from orcamentos_painel) as no_botao,
--          (select count(*) from documentos_lista
--            where oculto_em is null and fila = 'orcamento' and onde = 'fila') as na_lista;
--
-- PARA DESFAZER
--   Repita o `create or replace view` de `documentos_lista` sem as duas últimas
--   colunas — o Postgres NÃO deixa remover coluna de view: é preciso
--   `drop view documentos_lista cascade` e recriar as dependentes.
-- =============================================================================
