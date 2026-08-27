-- =============================================================================
-- 037 — a fila não perde nota                                            rev 1
-- =============================================================================
--
-- A REGRA, DITA PELO DONO EM 27/08/2026, DEPOIS DE EU ERRAR TRÊS VEZES
--
--   "nao pode sair nada da fila quando inseri ou depois que le....nada"
--
--   E o fluxo inteiro, nas palavras dele:
--
--     insere        -> tudo vai para a fila (repetidas e tudo); a repetida
--                      ganha apenas a TAG
--     manda ler     -> tudo CONTINUA na fila, com a tag do estado (leitura boa,
--                      repetida, sem ticket, não associado, ...)
--     gera orçamento-> só AÍ o registro muda de fila
--
-- O QUE ACONTECIA
--
--   Ele inseriu 9 notas, mandou ler, e a lista caiu para 8. Nenhuma sumiu do
--   sistema: a DAV 19425 não tem ticket escrito na observação, e tanto a lista
--   quanto o contador filtravam por `onde = 'fila'` — que manda a nota sem
--   ticket para Correções.
--
--   Do lado de dentro isso é uma classificação correta. Do lado de fora é uma
--   nota que evaporou depois de uma ação que deveria apenas LER. Uma fila que
--   muda de tamanho sozinha ensina o usuário a desconfiar dela, e quem
--   desconfia da fila confere tudo na mão — que é o trabalho que este sistema
--   existe para tirar dele.
--
-- O QUE MUDA, E O QUE NÃO MUDA
--
--   NÃO muda a classificação. `destino` e `onde` continuam iguais, e por isso
--   Correções, o Fechamento e as contas de 034 continuam iguais. A nota passa a
--   aparecer nos DOIS lugares: na fila, para ninguém achar que sumiu, e em
--   Correções, que é onde estão as ferramentas de conserto.
--
--   MUDA quem os contadores da fila contam: tudo o que entrou e ainda não virou
--   orçamento. A mesma frase que o motor usa na lista (a constante `NaFila`, em
--   `interno/modulos/orcamentos/documentos.go`). Duas réguas para a mesma fila é
--   como a tela e o botão discordam.
--
-- MEDIDO NO BANCO DE VERDADE, ANTES DE ESCREVER ISTO
--
--   notas_arquivos    hoje  8  ->  12   (8 prontas + 3 sem associação + 1 sem ticket)
--   rateio_sem_ticket hoje  0  ->   2
--
--   As 4 que reaparecem não são novidade nem erro: são as que foram sumindo da
--   vista desde 26/08. Elas voltam com o selo do motivo na linha.
--
-- POR QUE O `with (security_invoker = true)` ESTÁ AQUI (P-35)
--
--   `create or replace view` APAGA as opções da view. Oito migrações seguidas
--   deixaram de repetir esta cláusula e as views passaram a entregar dados a
--   quem não tinha feito login — foi o que a 033 consertou. Toda reescrita
--   repete, sem exceção.
-- =============================================================================

create or replace view orcamentos_painel
with (security_invoker = true) as
 SELECT id AS cliente_id,
    -- A FILA DE NOTAS E DAVs — tudo o que entrou e não virou orçamento
    --   Era `d.onde = 'fila'`, que escondia a nota sem ticket e a não
    --   associada. Ver o cabeçalho.
    ( SELECT count(*) AS count
           FROM documentos_lista d
          WHERE d.cliente_id = cl.id AND d.fila = 'orcamento'::text
            AND d.oculto_em IS NULL AND d.status <> 'usado'::text) AS notas_arquivos,
    -- A FILA DE RATEIO — mesma régua
    --   Era "não tem nenhum ticket", o que fazia a nota sumir do contador no
    --   instante em que alguém amarrava o primeiro ticket — bem no meio do
    --   trabalho que aquela tela existe para fazer.
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'rateio'::text
            AND d.oculto_em IS NULL AND d.status <> 'usado'::text) AS rateio_sem_ticket,
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
  '`rateio_sem_ticket` contam TUDO o que entrou e não virou orçamento: nada '
  'sai da fila antes de gerar (migração 037).';

insert into schema_migrations (versao, arquivo)
values ('037', '037_a_fila_nao_perde_nota.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- o contador do painel tem que bater com a lista, nota por nota:
--   select (select notas_arquivos from orcamentos_painel) as painel,
--          (select count(*) from documentos_lista
--            where fila='orcamento' and oculto_em is null and status <> 'usado') as lista;
--   -- esperado: painel = lista (12 = 12 em 27/08/2026)
--
--   -- e a tranca continua de pé:
--   select relname, reloptions from pg_class where relname = 'orcamentos_painel';
--   -- esperado: {security_invoker=true}
--
-- PARA DESFAZER
--   Repita o `create or replace view` da 036, SEM ESQUECER o
--   `with (security_invoker = true)`.
-- =============================================================================
