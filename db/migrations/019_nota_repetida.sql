-- 019 — a nota repetida ganha nome, e para de virar orçamento
--
-- O QUE ESTAVA ACONTECENDO
--
--   A trava de duplicidade existia em Go desde o começo (`regras.MesmaNota`),
--   com teste e tudo, e NUNCA foi chamada de lugar nenhum. Estava escrita e
--   morta.
--
--   O que protegia o sistema era só o sha256 do arquivo: subir o MESMO arquivo
--   duas vezes não cria dois documentos. Isso cobre o dedo escorregando no
--   Explorer, e não cobre o caso real — a mesma nota chegando como arquivo
--   diferente: a foto e o PDF dela, dois escaneamentos, o mesmo PDF renomeado.
--   Shas diferentes, dois documentos, dois orçamentos, e a loja pagando o mesmo
--   material duas vezes.
--
-- POR QUE UMA COLUNA, E NÃO UM `delete`
--
--   Porque "esta é repetida" é uma CONCLUSÃO, e conclusão de máquina sobre
--   dinheiro precisa poder ser desfeita por gente. Guardando de QUEM ela é
--   repetida, a tela mostra as duas lado a lado e o usuário decide. Apagar
--   sozinho seria a máquina tendo a última palavra sobre uma nota que talvez
--   fosse legítima.
--
--   E é `on delete set null` de propósito: se a original for removida um dia, a
--   cópia deixa de ser cópia — vira a nota que sobrou, e volta a poder rodar.

alter table documentos
  add column if not exists duplicada_de uuid references documentos(id) on delete set null;

comment on column documentos.duplicada_de is
  'Quando preenchida, esta nota é a MESMA que a apontada — mesma chave de acesso, '
  'ou mesmo número e valor. Ela não gera orçamento, e a tela mostra as duas juntas '
  'para uma pessoa decidir. Nulo é o normal.';

-- O índice serve à tela ("me mostre as repetidas") e ao filtro da geração, que
-- exige `duplicada_de is null` em toda busca de nota pronta.
create index if not exists documento_repetido
  on documentos (cliente_id, duplicada_de)
  where duplicada_de is not null;

-- A BUSCA QUE A TRAVA FAZ, TODA VEZ QUE UMA NOTA É LIDA
--
--   Sem isto, procurar "outra nota com esta chave" varre a tabela inteira a
--   cada leitura. Parcial porque nota sem chave não entra nesta busca — ela cai
--   na comparação por número e valor, que tem o índice logo abaixo.
create index if not exists documento_por_chave
  on documentos (cliente_id, chave_acesso)
  where chave_acesso is not null and oculto_em is null;

create index if not exists documento_por_numero
  on documentos (cliente_id, numero, valor_total)
  where numero is not null and oculto_em is null;

-- ---------------------------------------------------------------------------
-- a lista da tela precisa ENXERGAR a marca
-- ---------------------------------------------------------------------------
--
-- ⚠ `create or replace view` no Postgres só ACRESCENTA coluna no fim: nome,
--    ordem e tipo das que já existem são imutáveis. Por isso as 22 colunas
--    abaixo estão reproduzidas VERBATIM, na ordem exata da 016, e as novas vão
--    no fim. Trocar uma vírgula de lugar aqui derruba a tela inteira.
--
--    O que MUDA é a expressão de `pronto_para_gerar` — isso o Postgres permite,
--    porque o nome e o tipo continuam os mesmos. E precisa mudar: uma nota
--    repetida que a tela anuncia como "pronta para gerar" é a tela contando uma
--    mentira sobre dinheiro.

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
      AND d.duplicada_de IS NULL                       AS pronto_para_gerar,
    -- daqui para baixo, o que a 019 acrescenta
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em  AS duplicada_de_em
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
