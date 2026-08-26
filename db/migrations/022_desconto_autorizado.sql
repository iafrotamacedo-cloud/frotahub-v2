-- 022 — o desconto que alguém assina embaixo, e a nota que espera o cliente
--
-- AS TRÊS PORTAS DE SAÍDA DE UMA NOTA QUE NÃO CABE NO TETO
--
--   1. desconto     alguém abre mão de parte da margem para a nota fechar no
--                   teto. Exige confirmação e senha, e nunca passa de 20% —
--                   que é a margem inteira.
--   2. aprovação    o orçamento nasce com o valor CHEIO e fica parado, porque
--                   é ele que vai ao cliente. Aprovado, entra na fila de
--                   lançar.
--   3. rateio       a nota atende vários chamados e estava na gaveta errada.
--
--   (a quarta é excluir, que já existe e é `oculto_em`.)
--
-- POR QUE O DESCONTO MORA NA NOTA, E NÃO NO ORÇAMENTO
--
--   Porque ele é autorizado ANTES de o orçamento existir. E porque uma nota
--   rateada vira vários orçamentos: a autorização é uma só, e repeti-la em
--   cada pedaço faria três registros do mesmo ato — que é como se perde a
--   conta de quantas vezes alguém autorizou o quê.

alter table documentos
  add column if not exists desconto_bp integer not null default 0,
  add column if not exists desconto_em timestamptz,
  add column if not exists desconto_por uuid references perfis(id) on delete set null,
  add column if not exists aprovacao_pedida boolean not null default false,
  add column if not exists aprovacao_pedida_em timestamptz,
  add column if not exists aprovacao_pedida_por uuid references perfis(id) on delete set null;

-- ⚠ O LIMITE VIVE NO BANCO TAMBÉM, E NÃO SÓ NO GO
--   A regra dos 20% é conferida no motor antes de gravar. Mas uma trava que
--   existe num lugar só é uma trava que some no dia em que alguém escrever a
--   segunda rotina que grava aqui — e a segunda rotina sempre aparece.
alter table documentos drop constraint if exists desconto_dentro_da_margem;
alter table documentos add constraint desconto_dentro_da_margem
  check (desconto_bp >= 0 and desconto_bp <= 2000);

comment on column documentos.desconto_bp is
  'Desconto autorizado, em pontos-base do orçamento (385 = 3,85%). Zero é o normal. '
  'Teto de 2000 = 20%, que é a margem inteira: abaixo disso a empresa trabalharia pagando.';
comment on column documentos.aprovacao_pedida is
  'A nota foi mandada para aprovação do cliente. O orçamento nasce com o valor CHEIO '
  'e parado em aguardando_aprovacao — é ele que vai ao cliente, não a nota.';

create index if not exists documento_com_desconto
  on documentos (cliente_id, desconto_em)
  where desconto_bp > 0;

-- ---------------------------------------------------------------------------
-- a lista precisa mostrar o que já foi autorizado
-- ---------------------------------------------------------------------------
--
-- ⚠ As 26 colunas abaixo são a definição da 020, VERBATIM. `create or replace
--    view` só acrescenta no fim: nome, ordem e tipo das existentes são
--    imutáveis. As quatro novas vão depois.
--
--    `pronto_para_gerar` ganha mais um termo: uma nota que pediu aprovação não
--    está pronta para gerar pelo caminho normal — ela gera por outro, e
--    anunciá-la como pronta faria alguém processá-la duas vezes.

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
      AND d.bloqueio_motivo IS NULL
      AND d.aprovacao_pedida IS NOT TRUE                 AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo,
    -- daqui para baixo, o que a 022 acrescenta
    d.desconto_bp,
    d.desconto_em,
    d.aprovacao_pedida,
    d.aprovacao_pedida_em
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
values ('022', '022_desconto_autorizado.sql')
on conflict (versao) do nothing;
