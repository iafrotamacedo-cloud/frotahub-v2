-- =============================================================================
-- 031 — o valor da nota vira pergunta                                    rev 2
-- =============================================================================
--
-- REV 2: a rev 1 foi recusada pelo Postgres —
--   "cannot change name of view column motivo_conferencia to soma_dos_itens".
--   Eu tinha posto as colunas novas ANTES de `motivo_conferencia`, e
--   `create or replace view` só aceita coluna nova no FIM. A regra estava
--   escrita no comentário da 029 e eu a quebrei na migração seguinte.
--
-- O QUE ESTAVA ERRADO, E NÃO ERA A REDAÇÃO
--
--   A tela mostrava um PLACAR TÉCNICO como se fosse instrução. A confiança da
--   leitura soma partes: 0,35 pela chave de acesso, 0,35 pelos itens completos,
--   0,25 pela soma que fecha, 0,05 pela data. Faltando a chave, uma DANFE
--   perfeita — conta fechando no centavo — cai para 65% e vira "confira" para
--   sempre.
--
--   E a chave de acesso não é assunto de quem opera: são 44 dígitos que ninguém
--   vai digitar, que não entram no orçamento e que não impedem nada. Dizer "a
--   chave não foi lida" é contar da nossa contabilidade interna a quem só quer
--   saber se o número está certo.
--
--   Medido em 26/08/2026, sobre 79 notas lidas por máquina:
--     · 12 diziam "confira"
--     · 7 dessas 12 estavam perfeitas — alarme à toa
--     · 2 notas com problema real NÃO eram avisadas
--   Um placar que erra nos dois sentidos não se conserta com outra palavra.
--
-- O QUE ENTRA NO LUGAR
--
--   A pergunta que a pessoa consegue responder olhando o papel: "a IA leu
--   R$ 200,00 — é isso mesmo?". Ela responde sim ou não, e a resposta fica
--   gravada. Pergunta respondida não volta.
--
--   O gatilho é a ARITMÉTICA, não o placar: os itens somam o total? É a mesma
--   régua do `ContaFecha` do leitor, com o mesmo um por cento de tolerância,
--   para as duas nunca discordarem.
-- =============================================================================

alter table documentos
  add column if not exists valor_conferido_em  timestamptz,
  add column if not exists valor_conferido_por uuid;

comment on column documentos.valor_conferido_em is
  'Quando alguém olhou o papel e confirmou (ou corrigiu) o valor lido pela IA.';

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
        -- A CONTA QUE NÃO FECHA É PERGUNTA, NÃO AVISO
        --
        --   O que a pessoa pode conferir no papel é o VALOR. A chave de acesso,
        --   que derrubava o placar de confiança, ela nunca vai digitar e não
        --   muda nada no orçamento — dizer que faltou era contar da nossa
        --   contabilidade interna a quem só quer saber se o número está certo.
        --
        --   Uma vez respondida, a pergunta não volta: `valor_conferido_em`.
        WHEN d.status = 'lido'::text AND d.valor_conferido_em IS NULL
             AND NOT (d.valor_total IS NOT NULL AND d.valor_total > 0::numeric
                      AND COALESCE(i.quantos, 0::bigint) > 0
                      AND COALESCE(i.incompletos, 0::bigint) = 0
                      AND abs(COALESCE(i.soma, 0::numeric) - d.valor_total) <= d.valor_total * 0.01)
        THEN 'confirme o valor desta nota'
        ELSE NULL::text
    END AS motivo_conferencia,
    -- AS COLUNAS NOVAS VÊM NO FIM, E ISSO NÃO É ESTILO
    --
    --   `create or replace view` congela nome, ordem e tipo do que já existe;
    --   coluna nova só entra no fim. Pôr `soma_dos_itens` antes de
    --   `motivo_conferencia` faz o Postgres recusar com "cannot change name of
    --   view column" — foi o que aconteceu na primeira tentativa da 031.
    COALESCE(i.soma, 0::numeric) AS soma_dos_itens,
    COALESCE(i.incompletos, 0::bigint) AS itens_incompletos,
    -- A CONTA FECHA?
    --   Um por cento de tolerância cobre arredondamento e desconto de subtotal
    --   sem deixar passar item inventado — a mesma régua do `ContaFecha` do
    --   leitor, para as duas nunca discordarem.
    (d.valor_total IS NOT NULL AND d.valor_total > 0::numeric
      AND COALESCE(i.quantos, 0::bigint) > 0
      AND COALESCE(i.incompletos, 0::bigint) = 0
      AND abs(COALESCE(i.soma, 0::numeric) - d.valor_total) <= d.valor_total * 0.01) AS conta_fecha,
    d.valor_conferido_em
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
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos,
            sum(di.valor_total) AS soma,
            count(*) FILTER (WHERE di.descricao = ''::text OR di.quantidade <= 0::numeric
                               OR di.valor_unitario <= 0::numeric) AS incompletos
           FROM documento_itens di
          WHERE di.documento_id = d.id) i ON true
     LEFT JOIN documentos o ON o.id = d.duplicada_de;

insert into schema_migrations (versao, arquivo)
values ('031', '031_valor_vira_pergunta.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select count(*) filter (where conta_fecha)     as fecham,
--          count(*) filter (where not conta_fecha) as nao_fecham,
--          count(*) filter (where motivo_conferencia = 'confirme o valor desta nota') as a_perguntar
--     from documentos_lista where oculto_em is null and status = 'lido';
--
--   -- ninguém com a conta fechando pode estar sendo perguntado:
--   select count(*) from documentos_lista
--    where conta_fecha and motivo_conferencia = 'confirme o valor desta nota';
--   -- esperado: 0
--
-- PARA DESFAZER
--   Repita o `create or replace view` da 030 e:
--   alter table documentos drop column valor_conferido_em, drop column valor_conferido_por;
-- =============================================================================
