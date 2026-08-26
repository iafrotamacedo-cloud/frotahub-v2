-- 020 — o desconto do fornecedor, e a nota que não cabe no teto
--
-- O QUE ESTAVA ERRADO
--
--   `NF 9160`, de 26/08/2026: cinco itens somando R$ 514,60 e a nota dizendo
--   R$ 463,14. A diferença é exatamente 10% — desconto do fornecedor, não erro
--   de leitura.
--
--   Até aqui a margem era marcada em cima dos ITENS. Ou seja: 20% sobre
--   R$ 514,60, um custo que a empresa não teve. E do outro lado, uma nota cujo
--   bruto passa do teto era recusada mesmo quando o que se pagou por ela cabia
--   folgado.
--
-- AS TRÊS COLUNAS
--
--   `valor_nota_cheio` guarda o bruto ao lado do `valor_nota`, que passa a ser
--   o que se pagou. Os dois juntos, para que seis meses depois alguém consiga
--   ver que a nota valia R$ 700, que se pagou R$ 450, e que o orçamento saiu
--   por R$ 600 porque era o teto — sem reconstruir a conta de cabeça.
--
--   `ajustado_pelo_teto` é o carimbo interno: NOSSO, nunca do cliente. Ele não
--   entra no PDF que sai daqui.
--
--   `documentos.bloqueio_motivo` é a nota que não pode rodar, com a frase que
--   diz por quê. Bloqueio sem saída visível é o que faz o usuário criar
--   planilha paralela.

alter table orcamentos
  add column if not exists valor_nota_cheio numeric(14,2),
  add column if not exists ajustado_pelo_teto boolean not null default false;

comment on column orcamentos.valor_nota_cheio is
  'A soma dos itens como estão na nota, ANTES do desconto do fornecedor. '
  'valor_nota é o que se pagou. Iguais quando não houve desconto.';

comment on column orcamentos.ajustado_pelo_teto is
  'O bruto da nota passava do maior custo que cabe no teto, e o orçamento foi '
  'fechado no teto exato. Carimbo INTERNO — não aparece no documento do cliente.';

alter table documentos
  add column if not exists bloqueio_motivo text;

comment on column documentos.bloqueio_motivo is
  'Quando preenchida, esta nota não gera orçamento e a frase diz por quê. '
  'Hoje só o limite de teto a preenche. Nulo é o normal.';

-- A tela de notas precisa mostrar as bloqueadas para tratamento, e a geração
-- precisa pulá-las sem varrer a tabela.
create index if not exists documento_bloqueado
  on documentos (cliente_id, bloqueio_motivo)
  where bloqueio_motivo is not null;

-- ---------------------------------------------------------------------------
-- a tela precisa ENXERGAR o carimbo
-- ---------------------------------------------------------------------------
--
-- ⚠ `create or replace view` só ACRESCENTA coluna no fim: nome, ordem e tipo
--    das que já existem são imutáveis. As 33 colunas abaixo estão reproduzidas
--    VERBATIM da definição que está em produção, e as duas novas vão no fim.
--    Mexer na ordem aqui derruba a tela de lançamento inteira.

create or replace view orcamentos_lista as
 SELECT o.id,
    o.cliente_id,
    o.ticket,
    o.parte,
    o.conta,
    o.status,
    o.valor,
    o.valor_nota,
    o.reduzido_pelo_teto,
    o.valor_antes_do_teto,
    o.rateio,
    o.criado_em,
    o.lancado_em,
    o.faturado,
    o.pago,
    o.trilogo_custo_id,
    o.arquivo_pdf_sha256,
    u.nome AS loja,
    c.descricao AS chamado_descricao,
    ( SELECT string_agg(d.numero, ', '::text ORDER BY d.numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d ON d.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d.numero IS NOT NULL) AS notas,
    ( SELECT string_agg(d.dav_numero, ', '::text ORDER BY d.dav_numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d ON d.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d.dav_numero IS NOT NULL) AS davs,
    o.lancamento_bloqueio,
    o.lancamento_bloqueio_detalhe,
    o.lancamento_tentado_em,
    o.lancamento_tentativas,
    c.status AS ticket_status,
    c.status_codigo AS ticket_status_codigo,
    (c.status_codigo = ANY (ARRAY[1, 7])) AND r.ja_terminou AS reaberto,
        CASE
            WHEN (c.status_codigo = ANY (ARRAY[1, 7])) AND r.ja_terminou THEN r.ultima_frase
            ELSE NULL::text
        END AS motivo_reabertura,
        CASE
            WHEN o.status <> 'gerado'::text THEN NULL::text
            WHEN c.id IS NULL THEN 'sem_chamado'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6]) THEN 'pode_lancar'::text
            WHEN c.status_codigo = 3 THEN 'cliente'::text
            WHEN c.status_codigo = 1 AND r.ja_terminou THEN 'cliente'::text
            WHEN c.status_codigo = 1 THEN 'encarregados'::text
            WHEN c.status_codigo = 7 THEN 'encarregados'::text
            ELSE 'outro'::text
        END AS destino,
    ( SELECT a.avisado_em
           FROM ticket_avisos a
          WHERE a.cliente_id = o.cliente_id AND a.ticket = o.ticket
          ORDER BY a.avisado_em DESC
         LIMIT 1) AS avisado_em,
    o.unidade_id,
    o.fatura_id,
    o.valor_nota_cheio,
    o.ajustado_pelo_teto
   FROM orcamentos o
     LEFT JOIN unidades u ON u.id = o.unidade_id
     LEFT JOIN chamados c ON c.id = o.chamado_id
     LEFT JOIN LATERAL ( SELECT (EXISTS ( SELECT 1
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[3, 5, 6])))) AS ja_terminou,
            ( SELECT e.texto
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[1, 7])) AND COALESCE(e.texto, ''::text) <> ''::text
                  ORDER BY e.quando DESC
                 LIMIT 1) AS ultima_frase) r ON true;

-- ---------------------------------------------------------------------------
-- e a lista de NOTAS mostra o bloqueio
-- ---------------------------------------------------------------------------
--
-- Pelo mesmo motivo da outra: quem vai tratar precisa ler a frase, não
-- descobrir que a nota sumiu da fila. As 25 colunas da 019 estão reproduzidas
-- verbatim; `bloqueio_motivo` vai no fim, e `pronto_para_gerar` passa a exigir
-- que ela seja nula — nota bloqueada anunciada como pronta é a tela mentindo.

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
    d.status = 'lido'::text
      AND COALESCE(t.quantos, 0::bigint) > 0
      AND COALESCE(array_length(t.soltos, 1), 0) = 0
      AND d.duplicada_de IS NULL
      AND d.bloqueio_motivo IS NULL                    AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo
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

-- O REGISTRO DA VERSÃO
--
--   Faltava. As outras quatro desta leva o têm, e sem ele o `schema_migrations`
--   diria que a 020 nunca rodou — o que faria a próxima pessoa a olhar aquela
--   tabela concluir que o banco está atrás do código, e possivelmente rodá-la
--   de novo. As instruções aqui são idempotentes, então rodar de novo não
--   quebraria nada; mas confiar numa tabela de controle que mente é como se
--   perde a conta do que já foi aplicado.
insert into schema_migrations (versao, arquivo)
values ('020', '020_desconto_do_fornecedor.sql')
on conflict (versao) do nothing;
