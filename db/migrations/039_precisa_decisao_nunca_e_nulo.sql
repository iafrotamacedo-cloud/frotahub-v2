-- =============================================================================
-- 039 — o orçamento novo volta a aparecer na fila de lançar             rev 1
-- =============================================================================
--
-- O QUE ACONTECEU, EM 27/08/2026
--
--   Ele gerou mais de 40 orçamentos e o cartão do painel continuou dizendo
--   "1 PODEM SUBIR · 150 estão parados". Os 39 recém-nascidos não apareceram
--   nem no cartão nem na fila de lançamento. Nenhum estava travado: eles
--   simplesmente não eram VISTOS.
--
-- A CAUSA, E ELA É UMA LINHA
--
--   `precisa_decisao` nasceu na migração 035 assim:
--
--     o.status = 'gerado' AND (o.lancamento_bloqueio = ANY (ARRAY['teto', ...]))
--
--   Em SQL, comparar com NULL não devolve `false`: devolve **NULL**.
--   `NULL = ANY (ARRAY[...])` é NULL, e `true AND NULL` é NULL. Então todo
--   orçamento com `lancamento_bloqueio` nulo — que é TODO orçamento recém
--   gerado, antes da primeira tentativa de lançamento — recebia
--   `precisa_decisao = NULL` em vez de `false`.
--
--   E quem lê essa coluna pergunta pela negativa:
--
--     · o painel:  WHERE destino = 'pode_lancar' AND NOT precisa_decisao
--     · a tela:    &precisa_decisao=is.false
--
--   `NOT NULL` é NULL, e NULL não passa em `WHERE`. `is.false` também não casa
--   com NULL. O orçamento sumia dos dois ao mesmo tempo, sem erro, sem log e
--   sem nada na tela — a espécie de falha mais difícil de notar que existe.
--
-- POR QUE ISTO NÃO APARECEU ANTES
--
--   Porque `lancamento_bloqueio` deixa de ser nulo assim que alguém TENTA
--   lançar. Todos os orçamentos que existiam quando a 035 subiu já tinham
--   passado por uma tentativa: 88 com `ticket_status`, 3 com
--   `possivel_duplicata`, 2 com `teto`. Nenhum com nulo.
--
--   Medido hoje, depois de ele gerar: 58 orçamentos com `lancamento_bloqueio`
--   nulo, **todos os 58 de hoje**, e nos 58 a expressão devolve NULL.
--
--   O defeito precisava de alguém gerar e olhar o painel ANTES de tentar
--   lançar. Foi exatamente o que ele fez.
--
-- O CONSERTO
--
--   `COALESCE(o.lancamento_bloqueio, '')`. Bloqueio ausente vira string vazia,
--   que não está na lista, e a expressão devolve `false` — que é a verdade:
--   este orçamento NÃO precisa de decisão nenhuma.
--
--   Nada mais muda. As 40 colunas continuam iguais, na mesma ordem, com os
--   mesmos tipos: só a expressão de UMA delas foi reescrita.
--
-- POR QUE O `with (security_invoker = true)` ESTÁ AQUI (P-35)
--
--   `create or replace view` APAGA as opções da view. Oito migrações seguidas
--   deixaram de repetir esta cláusula e as views passaram a entregar dados a
--   quem não tinha feito login — foi o que a 033 consertou.
-- =============================================================================

create or replace view orcamentos_lista
with (security_invoker = true) as
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
    ( SELECT string_agg(d_1.numero, ', '::text ORDER BY d_1.numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d_1 ON d_1.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d_1.numero IS NOT NULL) AS notas,
    ( SELECT string_agg(d_1.dav_numero, ', '::text ORDER BY d_1.dav_numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d_1 ON d_1.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d_1.dav_numero IS NOT NULL) AS davs,
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
    o.ajustado_pelo_teto,
        CASE
            WHEN o.status = 'removido'::text THEN 'apagado'::text
            WHEN o.status = 'lancado'::text THEN 'lancado'::text
            WHEN o.status = 'aguardando_aprovacao'::text THEN 'aguardando-aprovacao'::text
            WHEN d.precisa THEN 'espera-decisao'::text
            WHEN c.id IS NULL THEN 'sem-chamado'::text
            WHEN c.status_codigo = 3 THEN 'espera-cliente'::text
            WHEN c.status_codigo = 1 AND r.ja_terminou THEN 'espera-cliente'::text
            WHEN c.status_codigo = ANY (ARRAY[1, 7]) THEN 'espera-equipe'::text
            WHEN (c.status_codigo = ANY (ARRAY[5, 6])) AND o.lancamento_bloqueio IS NOT NULL THEN 'recusado-ja-liberou'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6]) THEN 'pode-lancar'::text
            ELSE 'nao-classificado'::text
        END AS estado,
    d.precisa AS precisa_decisao
   FROM orcamentos o
     LEFT JOIN unidades u ON u.id = o.unidade_id
     LEFT JOIN chamados c ON c.id = o.chamado_id
     LEFT JOIN LATERAL ( SELECT o.status = 'gerado'::text
              -- O `COALESCE` É O CONSERTO INTEIRO DESTA MIGRAÇÃO
              --   Sem ele, `NULL = ANY (ARRAY[...])` devolve NULL, e
              --   `true AND NULL` devolve NULL. Veja o cabeçalho.
              AND (COALESCE(o.lancamento_bloqueio, ''::text) = ANY (ARRAY['teto'::text, 'possivel_duplicata'::text]))
            AS precisa) d ON true
     LEFT JOIN LATERAL ( SELECT (EXISTS ( SELECT 1
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[3, 5, 6])))) AS ja_terminou,
            ( SELECT e.texto
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[1, 7])) AND COALESCE(e.texto, ''::text) <> ''::text
                  ORDER BY e.quando DESC
                 LIMIT 1) AS ultima_frase) r ON true;

comment on view orcamentos_lista is
  'A lista dos orçamentos. RODA COMO QUEM PERGUNTA (security_invoker) — se você '
  'reescrever esta view, REPITA a cláusula `with (security_invoker = true)`, '
  'senão a tranca cai em silêncio (migração 033). `precisa_decisao` NUNCA pode '
  'ser nulo: quem a lê pergunta pela negativa (`NOT precisa_decisao`), e NULL '
  'faz a linha sumir do painel e da tela ao mesmo tempo (migração 039).';

insert into schema_migrations (versao, arquivo)
values ('039', '039_precisa_decisao_nunca_e_nulo.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- ninguém pode ter `precisa_decisao` nulo:
--   select count(*) from orcamentos_lista where precisa_decisao is null;
--   -- esperado: 0
--
--   -- e a fila de lançar tem que voltar a enxergar os recém-gerados:
--   select count(*) from orcamentos_lista
--    where destino = 'pode_lancar' and not precisa_decisao;
--   -- esperado: 40 em 27/08/2026 (era 1)
--
--   -- a tranca continua de pé:
--   select relname, reloptions from pg_class where relname = 'orcamentos_lista';
--   -- esperado: {security_invoker=true}
--
-- PARA DESFAZER
--   Repita o `create or replace view` da 035, SEM ESQUECER o
--   `with (security_invoker = true)`. (Mas aí o defeito volta.)
-- =============================================================================
