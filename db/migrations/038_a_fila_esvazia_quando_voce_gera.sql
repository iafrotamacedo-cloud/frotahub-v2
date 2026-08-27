-- =============================================================================
-- 038 — a fila esvazia quando VOCÊ gera                                  rev 1
-- =============================================================================
--
-- A REGRA COMPLETA, DITA PELO DONO EM 27/08/2026
--
--   insere        -> tudo vai para a fila (repetidas e tudo); a repetida ganha
--                    apenas a TAG
--   manda ler     -> tudo CONTINUA na fila, com a tag do estado
--   gera orçamento-> AÍ o registro muda de fila: vai para orçamentos gerados,
--                    vai para PENDÊNCIAS, ou PERMANECE na fila se for caso de
--                    duplicidade ou de falta de confiança na leitura
--
--   E, depois de gerar, sobre as quatro que sobraram na tela dele:
--   "eu nao quero eles nessa fila...eu quero eles apenas nas filas de pendencia"
--
-- O QUE A 037 ACERTOU E O QUE ELA DEIXOU PELA METADE
--
--   A 037 tirou a classificação de dentro do filtro da fila: nota sem ticket
--   parou de evaporar no instante em que era LIDA. Isso estava certo, e continua.
--
--   Mas ela travou a nota na fila para SEMPRE. Depois de gerar, as quatro que
--   não puderam virar orçamento — três com ticket que a base não conhece, uma
--   sem ticket nenhum — continuaram ali, misturadas com as que ainda não tinham
--   sido tentadas. A fila deixou de ser fila e virou depósito.
--
--   O que separa os dois momentos não é o ESTADO da nota: é se ela JÁ FOI
--   TENTADA. E isso o banco não sabia dizer — daí a coluna nova.
--
-- QUEM SAI E QUEM FICA, DEPOIS DE UMA TENTATIVA
--
--   SAI para Correções   quem tem uma frente própria lá, com ferramenta de
--                        conserto em lote:
--                          'sem-ticket'     -> Correções › Sem ticket
--                          'sem-associacao' -> Correções › Sem associação
--                          'bloqueada'      -> Correções › Passam do teto
--
--   FICA na fila         o que o dono nomeou — duplicidade e falta de confiança
--                        — e o que ainda não terminou de acontecer:
--                          'repetida', 'a-conferir', 'falhou', 'lendo',
--                          'pronta', 'espera-cliente', 'rateio', 'direto'
--
--   'repetida' nem chega a ser tentada: `documentosProntos` filtra
--   `duplicada_de is null`. Ela nunca ganha a marca, e por construção nunca sai
--   da fila por este caminho — que é exatamente o que ele pediu.
--
-- POR QUE UMA COLUNA E NÃO UM `status` NOVO
--
--   `status` responde "em que ponto da leitura esta nota está". Enfiar
--   "tentei gerar" ali misturaria duas linhas do tempo que andam separadas: uma
--   nota pode ser tentada, corrigida e tentada de novo sem que a leitura mude.
--   Coluna própria, com a hora, e o passado fica legível.
--
-- POR QUE O `with (security_invoker = true)` ESTÁ AQUI (P-35)
--
--   `create or replace view` APAGA as opções da view. Oito migrações seguidas
--   deixaram de repetir esta cláusula e as views passaram a entregar dados a
--   quem não tinha feito login — foi o que a 033 consertou.
-- =============================================================================

alter table documentos
  add column if not exists geracao_tentada_em timestamptz;

comment on column documentos.geracao_tentada_em is
  'Quando esta nota entrou numa rodada de "Gerar orçamentos" — tenha dado certo '
  'ou não. É o que separa "ainda não foi tentada" de "foi tentada e não passou", '
  'e é por isso que a fila esvazia sem nada sumir antes da hora (migração 038).';

create index if not exists documentos_geracao_tentada_idx
  on documentos (cliente_id, geracao_tentada_em)
  where geracao_tentada_em is null;

-- -----------------------------------------------------------------------------
-- 1) a lista das notas — ganha `na_fila` NO FIM
--
--    `create or replace view` congela nome, ordem e tipo do que já existe;
--    coluna nova só entra no fim. Foi o erro que fez a 031 ser recusada pelo
--    Postgres com "cannot change name of view column".
-- -----------------------------------------------------------------------------

create or replace view documentos_lista
with (security_invoker = true) as
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
    d.status = 'lido'::text AND COALESCE(t.quantos, 0::bigint) > 0
      AND COALESCE(array_length(t.soltos, 1), 0) = 0 AND d.duplicada_de IS NULL
      AND d.bloqueio_motivo IS NULL AND d.aprovacao_pedida IS NOT TRUE AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo,
    d.desconto_bp,
    d.desconto_em,
    d.aprovacao_pedida,
    d.aprovacao_pedida_em,
    CASE cl.destino
        WHEN 'usada'::text          THEN 'usada'::text
        WHEN 'sem-ticket'::text     THEN 'sem-ticket'::text
        WHEN 'sem-associacao'::text THEN 'sem-associacao'::text
        ELSE 'fila'::text
    END AS onde,
    CASE
        WHEN d.status = 'falhou'::text THEN COALESCE(NULLIF(d.leitura_erro, ''::text), 'a leitura desta nota falhou')
        WHEN d.duplicada_de IS NOT NULL THEN 'esta nota já entrou antes — confira antes de gerar'
        WHEN d.bloqueio_motivo IS NOT NULL THEN d.bloqueio_motivo
        WHEN d.aprovacao_pedida IS TRUE THEN 'aprovação pedida ao cliente — esperando a resposta'
        WHEN cl.destino = 'a-conferir'::text THEN 'confirme o valor desta nota'
        ELSE NULL::text
    END AS motivo_conferencia,
    COALESCE(i.soma, 0::numeric) AS soma_dos_itens,
    COALESCE(i.incompletos, 0::bigint) AS itens_incompletos,
    r.fecha AS conta_fecha,
    d.valor_conferido_em,
    cl.destino,
    -- -----------------------------------------------------------------------
    -- ESTA NOTA APARECE NA FILA?
    --
    --   A fila é o lugar de quem ainda espera a SUA decisão. Três condições, e
    --   a terceira é a que nasce nesta migração:
    --
    --   1. não foi tirada da fila à mão
    --   2. ainda não virou orçamento
    --   3. ou nunca foi tentada, ou é do tipo que fica mesmo depois de tentada
    --
    --   O item 3 é a regra do dono, em código: só sai depois de uma tentativa
    --   de geração quem tem uma tela de conserto esperando por ela. Duplicidade
    --   e falta de confiança ficam aqui, porque quem decide é você, e é aqui
    --   que você olha.
    -- -----------------------------------------------------------------------
    (d.oculto_em IS NULL
     AND d.status <> 'usado'::text
     AND (d.geracao_tentada_em IS NULL
          OR cl.destino <> ALL (ARRAY['sem-ticket'::text, 'sem-associacao'::text, 'bloqueada'::text]))
    ) AS na_fila
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
     LEFT JOIN documentos o ON o.id = d.duplicada_de
     LEFT JOIN LATERAL ( SELECT (d.valor_total IS NOT NULL AND d.valor_total > 0::numeric
              AND COALESCE(i.quantos, 0::bigint) > 0
              AND COALESCE(i.incompletos, 0::bigint) = 0
              AND abs(COALESCE(i.soma, 0::numeric) - d.valor_total) <= d.valor_total * 0.01) AS fecha) r ON true
     LEFT JOIN LATERAL ( SELECT
        CASE
            WHEN d.oculto_em IS NOT NULL                    THEN 'excluida'::text
            WHEN d.status = 'usado'::text                   THEN 'usada'::text
            WHEN d.duplicada_de IS NOT NULL                 THEN 'repetida'::text
            WHEN d.status = 'falhou'::text                  THEN 'falhou'::text
            WHEN d.status <> 'lido'::text                   THEN 'lendo'::text
            WHEN d.bloqueio_motivo IS NOT NULL              THEN 'bloqueada'::text
            WHEN d.fila = 'rateio'::text                    THEN 'rateio'::text
            WHEN d.fila = 'direto'::text                    THEN 'direto'::text
            WHEN COALESCE(t.quantos, 0::bigint) = 0         THEN 'sem-ticket'::text
            WHEN COALESCE(array_length(t.soltos, 1), 0) > 0 THEN 'sem-associacao'::text
            WHEN d.aprovacao_pedida IS TRUE                 THEN 'espera-cliente'::text
            WHEN d.valor_conferido_em IS NULL AND NOT r.fecha THEN 'a-conferir'::text
            WHEN d.status = 'lido'::text                    THEN 'pronta'::text
            ELSE 'nao-classificado'::text
        END AS destino) cl ON true;

comment on view documentos_lista is
  'A lista das notas. RODA COMO QUEM PERGUNTA (security_invoker) — se você '
  'reescrever esta view, REPITA a cláusula `with (security_invoker = true)`, '
  'senão a tranca cai em silêncio (migração 033). `destino` é a classificação '
  'completa; `onde` é uma projeção dela; `na_fila` diz se a nota ainda espera '
  'uma decisão sua na tela de Notas e DAVs (migração 038).';

-- -----------------------------------------------------------------------------
-- 2) o painel conta a MESMA fila que a lista mostra
--
--    Duas réguas para a mesma fila é como a tela mostra 31 e o botão diz 77.
--    Agora as duas leem `na_fila`.
-- -----------------------------------------------------------------------------

create or replace view orcamentos_painel
with (security_invoker = true) as
 SELECT id AS cliente_id,
    ( SELECT count(*) AS count
           FROM documentos_lista d
          WHERE d.cliente_id = cl.id AND d.fila = 'orcamento'::text AND d.na_fila) AS notas_arquivos,
    ( SELECT count(*) AS count
           FROM documentos_lista d
          WHERE d.cliente_id = cl.id AND d.fila = 'rateio'::text AND d.na_fila) AS rateio_sem_ticket,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text) AS a_lancar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND d.status = 'lido'::text
            AND d.fila = 'orcamento'::text AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS sem_ticket,
    ( SELECT count(*) AS count
           FROM documento_tickets t
             JOIN documentos d ON d.id = t.documento_id
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND t.chamado_id IS NULL
            AND d.fila <> 'direto'::text) AS sem_associacao,
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
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL
            AND array_length(dl.ticket_soltos, 1) > 0) AS notas_travadas,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND dl.pronto_para_gerar) AS prontas_para_gerar,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text
            AND o.lancamento_bloqueio IS NOT NULL) AS recusados,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'pode_lancar'::text
            AND NOT l.precisa_decisao) AS prontos_para_lancar,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'cliente'::text) AS esperando_cliente,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'encarregados'::text) AS esperando_equipe,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text
            AND o.fatura_id IS NULL AND o.faturamento_direto = false) AS a_faturar,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text
            AND o.fatura_id IS NULL AND o.faturamento_direto = false) AS valor_a_faturar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'direto'::text AND d.oculto_em IS NULL) AS notas_direto,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.precisa_decisao) AS esperando_decisao,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND d.status = 'lido'::text
            AND d.bloqueio_motivo IS NOT NULL) AS extrapoladas
   FROM clientes cl;

comment on view orcamentos_painel is
  'Os contadores do painel. RODA COMO QUEM PERGUNTA (security_invoker) — se '
  'você reescrever esta view, REPITA a cláusula `with (security_invoker = '
  'true)`, senão a tranca cai em silêncio (migração 033). `notas_arquivos` e '
  '`rateio_sem_ticket` leem `documentos_lista.na_fila`: a MESMA régua da lista.';

-- -----------------------------------------------------------------------------
-- 3) as quatro que ficaram na tela dele hoje já foram tentadas
--
--    Ele clicou em gerar e elas não passaram. Sem esta linha, elas continuariam
--    na fila até a próxima tentativa — e o conserto de hoje só valeria de
--    amanhã em diante. O carimbo é a hora da migração, que é a verdade mais
--    próxima que temos: a tentativa foi agora há pouco.
-- -----------------------------------------------------------------------------

update documentos d
   set geracao_tentada_em = now()
  from documentos_lista l
 where l.id = d.id
   and d.geracao_tentada_em is null
   and d.oculto_em is null
   and d.status = 'lido'
   and l.destino in ('sem-ticket', 'sem-associacao', 'bloqueada');

insert into schema_migrations (versao, arquivo)
values ('038', '038_a_fila_esvazia_quando_voce_gera.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- o painel e a lista contam a mesma coisa:
--   select (select notas_arquivos from orcamentos_painel) as painel,
--          (select count(*) from documentos_lista where fila='orcamento' and na_fila) as lista;
--   -- esperado: iguais
--
--   -- ninguém com tela de conserto própria ficou na fila depois de tentado:
--   select count(*) from documentos_lista
--    where na_fila and geracao_tentada_em is not null
--      and destino in ('sem-ticket','sem-associacao','bloqueada');
--   -- esperado: 0  (a coluna geracao_tentada_em vem de `documentos`)
--
--   -- e a tranca continua de pé nas duas views:
--   select relname, reloptions from pg_class
--    where relname in ('documentos_lista','orcamentos_painel');
--   -- esperado: {security_invoker=true} nas duas
--
-- PARA DESFAZER
--   Repita os dois `create or replace view` da 037 (e da 034, para a
--   documentos_lista), SEM ESQUECER o `with (security_invoker = true)`, e:
--   alter table documentos drop column geracao_tentada_em;
-- =============================================================================
