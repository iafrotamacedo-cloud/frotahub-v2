-- =============================================================================
-- 030 — a fila de rateio tem regra própria                               rev 1
-- =============================================================================
--
-- O QUE EU QUEBREI NA 029
--
--   A 029 escreveu a coluna `onde` pensando na fila `orcamento`, onde uma nota
--   sem ticket é um problema a resolver em Correções. Só que ela vale para TODAS
--   as filas — e na fila `rateio` a mesma condição significa o contrário.
--
--   Ali a nota chega justamente por NÃO ter ticket: ela atende vários chamados e
--   quem os dita é o usuário, na própria lista. Classificá-la como "sem-ticket"
--   a tirou da única tela onde o trabalho dela é feito. E não a mandou para
--   lugar nenhum: Correções › Sem ticket conta só a fila `orcamento`.
--
--   Sumiu do sistema sem sumir do banco, que é o pior jeito de sumir.
--
--   A nota da SV (`NF RATEIO SV 659629.pdf`, R$ 1.454,22) foi a que apareceu.
--
-- A REGRA CERTA
--
--   Nota de rateio fica na fila de rateio até virar orçamento. Ponto. O ticket
--   solto dela continua aparecendo em Correções › Sem associação, como sempre
--   apareceu — mas ela não sai da lista onde é trabalhada.
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
        -- NA FILA DE RATEIO, "SEM TICKET" É O ESTADO NORMAL
        --
        --   Ali a nota chega JUSTAMENTE por não ter ticket: quem os dita é o
        --   usuário, na própria lista, pelo botão de amarrar. Mandá-la para
        --   Correções › Sem ticket a tiraria da única tela onde o trabalho dela
        --   é feito — e Correções nem a receberia, porque aquela frente conta só
        --   a fila `orcamento`.
        --
        --   A 029 aplicou a esta fila uma regra desenhada para a outra, e a nota
        --   da SV sumiu da lista de rateio. Nota de rateio fica na fila de
        --   rateio até virar orçamento.
        WHEN d.fila = 'rateio'::text THEN 'fila'::text
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

insert into schema_migrations (versao, arquivo)
values ('030', '030_rateio_tem_regra_propria.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select nome_arquivo, status, tickets, onde
--     from documentos_lista
--    where fila = 'rateio' and oculto_em is null;
--   -- todas têm que estar em 'fila' — inclusive a que tem 0 tickets
--
--   -- e a fila de orçamento não pode ter mudado:
--   select onde, count(*) from documentos_lista
--    where fila = 'orcamento' and oculto_em is null group by onde order by onde;
--
-- PARA DESFAZER
--   Repita o `create or replace view` da 029.
-- =============================================================================
