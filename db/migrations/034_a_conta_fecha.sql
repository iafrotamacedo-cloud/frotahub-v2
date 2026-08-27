-- =============================================================================
-- 034 — a conta fecha, e dá para provar
-- =============================================================================
--
-- A PERGUNTA DO DONO, EM 27/08/2026
--
--   "tem 93 não lançados, 23 de pendência, 68 parados. nada bate com nada."
--
--   Fui medir. Os três números estão CERTOS — e são respostas para três
--   perguntas diferentes, que nenhuma tela jamais mostrou lado a lado:
--
--     93 = a fila inteira (tudo que está `gerado`)
--     68 = travados pelo status do chamado (27 do cliente + 41 nossos)
--     23 = os que já foram TENTADOS e o Trílogo recusou
--
--   E 93 = 68 + 22 + 3. Bate. O que não existia era o lugar onde isso
--   aparecesse somado.
--
--   Pior: dos 23 "recusados", 22 aparecem como `pode_lancar` — a recusa é de
--   ontem e o espelho do chamado diz que hoje pode. A lista de Recusados estava
--   mostrando HISTÓRIA, e quem a lia procurava PROBLEMA.
--
-- O QUE ESTA MIGRAÇÃO FAZ
--
--   Dá a cada nota UM destino e a cada orçamento UM estado, calculados num
--   lugar só, e publica os dois balanços somados em duas views. Depois disto,
--   "não bate" deixa de ser uma sensação e vira um número que a tela mostra.
--
-- POR QUE SÃO DUAS CONTAS, E NÃO UMA
--
--   O dono escreveu a conta assim:
--     notas que entram = orçamentos lançados + não lançados por pendência do
--     cliente + ... + não gerados por não ter ticket + ... + exclusões
--
--   Ela mistura duas unidades, e por isso NUNCA fecharia como está escrita:
--   uma nota pode virar VÁRIOS orçamentos (as partes de um mesmo ticket), e um
--   orçamento de rateio nasce de VÁRIAS notas. Hoje mesmo: 66 notas usadas
--   ↔ 66 vínculos, mas 703 orçamentos no total, porque 636 vieram do legado.
--
--   O que fecha — e é o que o dono quer de verdade — são DUAS contas ligadas:
--
--     1) toda NOTA que entrou está em exatamente um destino
--     2) todo ORÇAMENTO que existe está em exatamente um estado
--     3) e a ponte entre elas: nota usada ↔ vínculo vivo ↔ orçamento
--
--   As três foram medidas hoje e as três fecham (números no rodapé).
--
-- A REGRA MORA NUMA JUNÇÃO, NÃO NUMA COLUNA (CORE-06)
--
--   `onde`, `motivo_conferencia`, `conta_fecha` e o novo `destino` respondem
--   variações da MESMA pergunta. Antes, cada um tinha a sua própria expressão
--   copiada — foi assim que `conta_fecha` e o `ContaFecha` do leitor passaram a
--   discordar. Agora a classificação é feita UMA vez, numa junção lateral, e as
--   colunas são projeções dela. Discordar virou impossível por construção.
--
-- ⚠ `create or replace view` APAGA AS OPÇÕES DA VIEW
--
--   Sem repetir `with (security_invoker = true)`, a view volta a rodar como a
--   dona das tabelas e ignora as políticas — foi exatamente o que aconteceu da
--   019 até a 031, e o que a 033 acabou de consertar. A cláusula está nas duas
--   reescritas abaixo, e não é enfeite.
--
-- ⚠ E SÓ ACRESCENTA COLUNA NO FIM
--
--   Nome, ordem e tipo do que já existe são congelados. As 36 colunas de
--   `documentos_lista` e as 40 de `orcamentos_lista` estão reproduzidas na
--   ordem exata; o que é novo vai depois da última.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1) as notas — cada uma em um destino só
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
    d.status = 'lido'::text AND COALESCE(t.quantos, 0::bigint) > 0 AND COALESCE(array_length(t.soltos, 1), 0) = 0 AND d.duplicada_de IS NULL AND d.bloqueio_motivo IS NULL AND d.aprovacao_pedida IS NOT TRUE AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo,
    d.desconto_bp,
    d.desconto_em,
    d.aprovacao_pedida,
    d.aprovacao_pedida_em,
    -- `onde` VIROU UMA PROJEÇÃO DE `destino`
    --
    --   Ela responde a mesma pergunta com menos detalhe: as telas de Correções
    --   só precisam saber se a nota mora aqui ou lá. Derivando-a do `destino`,
    --   as duas não podem mais discordar — que foi o defeito que a 029 criou e
    --   a 030 remendou.
    CASE cl.destino
        WHEN 'usada'::text          THEN 'usada'::text
        WHEN 'sem-ticket'::text     THEN 'sem-ticket'::text
        WHEN 'sem-associacao'::text THEN 'sem-associacao'::text
        ELSE 'fila'::text
    END AS onde,
    -- POR QUE ESTA NOTA PRECISA DE GENTE — a frase que a tela mostra
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
    -- ---------------------------------------------------------------------
    -- O DESTINO — a coluna que faz a conta fechar
    --
    --   Treze valores, e a ordem do `CASE` É a regra: a primeira condição que
    --   casar ganha, e por isso cada nota cai em um só. Ler de cima para baixo
    --   é ler a regra de negócio.
    --
    --   O `ELSE` NÃO É ENFEITE. Se amanhã nascer um estado que ninguém previu,
    --   ele aparece como 'nao-classificado' na tela de Fechamento em vez de
    --   sumir dentro de 'pronta'. É a mesma escolha do `codigo N` do robô: o
    --   desconhecido tem que ser VISÍVEL. Uma conta que fecha escondendo o que
    --   não entendeu não é uma conta que fecha.
    -- ---------------------------------------------------------------------
    cl.destino
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
     -- A CONTA FECHA? — escrita UMA vez, lida por três colunas.
     --   Um por cento de tolerância cobre arredondamento e desconto de subtotal
     --   sem deixar passar item inventado.
     LEFT JOIN LATERAL ( SELECT (d.valor_total IS NOT NULL AND d.valor_total > 0::numeric
              AND COALESCE(i.quantos, 0::bigint) > 0
              AND COALESCE(i.incompletos, 0::bigint) = 0
              AND abs(COALESCE(i.soma, 0::numeric) - d.valor_total) <= d.valor_total * 0.01) AS fecha) r ON true
     LEFT JOIN LATERAL ( SELECT
        CASE
            -- Saiu de circulação: qualquer outro estado dela virou passado.
            WHEN d.oculto_em IS NOT NULL                    THEN 'excluida'::text
            -- Chegou ao fim do caminho: virou orçamento.
            WHEN d.status = 'usado'::text                   THEN 'usada'::text
            -- Barrada ANTES de virar orçamento — é a trava de duplicidade.
            WHEN d.duplicada_de IS NOT NULL                 THEN 'repetida'::text
            WHEN d.status = 'falhou'::text                  THEN 'falhou'::text
            -- AINDA NÃO LIDA — e a condição é NEGATIVA de propósito
            --
            --   A primeira versão desta migração listou os status de entrada
            --   ('novo', 'lendo'). O status de entrada se chama 'inserido', e
            --   'novo' não existe: uma nota recém-inserida caía até o fim do
            --   `CASE` e era classificada como 'sem-ticket' — o que é verdade e
            --   não é o ponto, porque ela ainda nem foi lida. O Postgres de
            --   prova pegou isso antes de sair daqui (P-24).
            --
            --   Perguntar "não é 'lido'?" cobre 'inserido', 'lendo', e o status
            --   que alguém inventar amanhã sem lembrar de vir aqui.
            WHEN d.status <> 'lido'::text                   THEN 'lendo'::text
            WHEN d.bloqueio_motivo IS NOT NULL              THEN 'bloqueada'::text
            -- NA FILA DE RATEIO, NÃO TER TICKET É O ESTADO NORMAL
            --   Quem os dita é o usuário, na própria lista. Mandá-la para
            --   'sem-ticket' a tiraria da única tela onde o trabalho dela é
            --   feito — foi o defeito que a 029 causou e a 030 remendou.
            WHEN d.fila = 'rateio'::text                    THEN 'rateio'::text
            -- O fornecedor cobra do cliente; nós só lançamos o custo.
            WHEN d.fila = 'direto'::text                    THEN 'direto'::text
            WHEN COALESCE(t.quantos, 0::bigint) = 0         THEN 'sem-ticket'::text
            WHEN COALESCE(array_length(t.soltos, 1), 0) > 0 THEN 'sem-associacao'::text
            WHEN d.aprovacao_pedida IS TRUE                 THEN 'espera-cliente'::text
            -- A aritmética não fecha e ninguém confirmou o valor ainda.
            WHEN d.valor_conferido_em IS NULL AND NOT r.fecha THEN 'a-conferir'::text
            WHEN d.status = 'lido'::text                    THEN 'pronta'::text
            ELSE 'nao-classificado'::text
        END AS destino) cl ON true;

comment on view documentos_lista is
  'A lista das notas. RODA COMO QUEM PERGUNTA (security_invoker) — se você '
  'reescrever esta view, REPITA a cláusula `with (security_invoker = true)`, '
  'senão a tranca cai em silêncio. Ver migração 033. A coluna `destino` é a '
  'classificação completa; `onde` é uma projeção dela.';

-- -----------------------------------------------------------------------------
-- 2) os orçamentos — cada um em um estado só
-- -----------------------------------------------------------------------------
--
-- `destino` (que já existia) responde QUEM TEM QUE AGIR.
-- `estado` (que nasce aqui) responde ONDE O ORÇAMENTO ESTÁ.
-- São perguntas diferentes, e por isso são duas colunas — não uma disputa.

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
    o.ajustado_pelo_teto,
    -- ---------------------------------------------------------------------
    -- O ESTADO — oito valores, e a soma deles é o total. Sempre.
    --
    --   'recusado-ja-liberou' existe por causa de um caso medido: 22 dos 23
    --   orçamentos da lista de Recusados já podiam subir de novo, porque o
    --   chamado andou depois da recusa. Somá-los aos travados de verdade é o
    --   que fazia a conta parecer errada. Eles são história, não problema, e
    --   agora têm linha própria.
    --
    --   O `ELSE` visível, pelo mesmo motivo da outra view.
    -- ---------------------------------------------------------------------
        CASE
            WHEN o.status = 'removido'::text             THEN 'apagado'::text
            WHEN o.status = 'lancado'::text              THEN 'lancado'::text
            WHEN o.status = 'aguardando_aprovacao'::text THEN 'aguardando-aprovacao'::text
            WHEN c.id IS NULL                            THEN 'sem-chamado'::text
            WHEN c.status_codigo = 3                     THEN 'espera-cliente'::text
            WHEN c.status_codigo = 1 AND r.ja_terminou   THEN 'espera-cliente'::text
            WHEN c.status_codigo = ANY (ARRAY[1, 7])     THEN 'espera-equipe'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6]) AND o.lancamento_bloqueio IS NOT NULL
                                                         THEN 'recusado-ja-liberou'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6])     THEN 'pode-lancar'::text
            ELSE 'nao-classificado'::text
        END AS estado
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

comment on view orcamentos_lista is
  'A lista dos orçamentos. RODA COMO QUEM PERGUNTA — repita '
  '`with (security_invoker = true)` em qualquer reescrita. Ver migração 033. '
  '`destino` = quem tem que agir; `estado` = onde o orçamento está.';

-- -----------------------------------------------------------------------------
-- 3) os dois balanços, prontos para a tela
-- -----------------------------------------------------------------------------
--
-- Cada linha traz `ordem` porque a sequência é a leitura: de cima para baixo
-- conta-se a história de uma nota, do que entrou ao que virou orçamento.
-- Deixar a tela ordenar por nome poria 'a-conferir' antes de 'lendo'.

create or replace view fechamento_notas
with (security_invoker = true) as
  select d.cliente_id,
         case d.destino
           when 'lendo'          then 1 when 'falhou'          then 2
           when 'a-conferir'     then 3 when 'sem-ticket'      then 4
           when 'sem-associacao' then 5 when 'bloqueada'       then 6
           when 'repetida'       then 7 when 'espera-cliente'  then 8
           when 'rateio'         then 9 when 'direto'          then 10
           when 'pronta'         then 11 when 'usada'          then 12
           when 'excluida'       then 13 else 99 end as ordem,
         d.destino,
         case d.destino
           when 'lendo'          then 'ainda não lida'
           when 'falhou'         then 'a leitura falhou'
           when 'a-conferir'     then 'esperando alguém confirmar o valor'
           when 'sem-ticket'     then 'nenhum ticket encontrado'
           when 'sem-associacao' then 'ticket escrito não bate com a base'
           when 'bloqueada'      then 'bloqueada'
           when 'repetida'       then 'repetida — barrada antes de virar orçamento'
           when 'espera-cliente' then 'aprovação pedida ao cliente'
           when 'rateio'         then 'na fila de rateio, esperando os tickets'
           when 'direto'         then 'faturamento direto'
           when 'pronta'         then 'pronta, ainda não gerada'
           when 'usada'          then 'virou orçamento'
           when 'excluida'       then 'excluída'
           else 'NÃO CLASSIFICADA — avise o suporte'
         end as rotulo,
         count(*) as quantas,
         COALESCE(sum(d.valor_total), 0::numeric) as valor
    from documentos_lista d
   group by d.cliente_id, d.destino;

create or replace view fechamento_orcamentos
with (security_invoker = true) as
  select o.cliente_id,
         case o.estado
           when 'pode-lancar'          then 1 when 'recusado-ja-liberou' then 2
           when 'espera-equipe'        then 3 when 'espera-cliente'      then 4
           when 'sem-chamado'          then 5 when 'aguardando-aprovacao' then 6
           when 'lancado'              then 7 when 'apagado'             then 8
           else 99 end as ordem,
         o.estado,
         case o.estado
           when 'pode-lancar'          then 'pode lançar agora'
           when 'recusado-ja-liberou'  then 'foi recusado antes, mas o chamado já andou'
           when 'espera-equipe'        then 'parado: pendência nossa'
           when 'espera-cliente'       then 'parado: pendência do cliente'
           when 'sem-chamado'          then 'parado: chamado não encontrado'
           when 'aguardando-aprovacao' then 'esperando a aprovação do cliente'
           when 'lancado'              then 'lançado no Trílogo'
           when 'apagado'              then 'apagado'
           else 'NÃO CLASSIFICADO — avise o suporte'
         end as rotulo,
         count(*) as quantos,
         COALESCE(sum(o.valor), 0::numeric) as valor
    from orcamentos_lista o
   group by o.cliente_id, o.estado;

-- A PONTE ENTRE AS DUAS CONTAS
--
--   Uma linha só, com os números que provam que nada se perdeu no caminho da
--   nota até o orçamento. Os três "órfãos" têm que ser SEMPRE zero; se um dia
--   não forem, alguma gravação falhou pelo meio (é o risco do `_ =` que engole
--   erro em `gerar.go`).
create or replace view fechamento_ponte
with (security_invoker = true) as
  select d.cliente_id,
         count(*) filter (where d.destino = 'usada') as notas_usadas,
         ( select count(*) from orcamento_documentos od
            where od.removido_em is null ) as vinculos_vivos,
         ( select count(*) from documentos x
            where x.cliente_id = d.cliente_id and x.status = 'usado' and x.oculto_em is null
              and not exists (select 1 from orcamento_documentos od
                               where od.documento_id = x.id and od.removido_em is null)
         ) as usadas_sem_vinculo,
         ( select count(*) from documentos x
            where x.cliente_id = d.cliente_id and x.status <> 'usado' and x.oculto_em is null
              and exists (select 1 from orcamento_documentos od
                           where od.documento_id = x.id and od.removido_em is null)
         ) as vinculo_sem_nota_usada,
         ( select count(*) from orcamento_documentos od
             join orcamentos o on o.id = od.orcamento_id
            where od.removido_em is null and o.status = 'removido'
         ) as vinculo_de_orcamento_apagado
    from documentos_lista d
   group by d.cliente_id;

insert into schema_migrations (versao, arquivo)
values ('034', '034_a_conta_fecha.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR — e os números medidos em 27/08/2026, antes desta migração
--
--   select ordem, rotulo, quantas, valor from fechamento_notas order by ordem;
--     esperado hoje:  lendo 0 · falhou 2 · a-conferir 0 · sem-ticket 4 ·
--                     sem-associacao 5 · repetida 0 · rateio 1 · usada 66 ·
--                     excluida 13   →  TOTAL 91
--
--   select ordem, rotulo, quantos, valor from fechamento_orcamentos order by ordem;
--     esperado hoje:  pode-lancar 3 · recusado-ja-liberou 22 · espera-equipe 41 ·
--                     espera-cliente 27 · lancado 609 · apagado 1
--                                                     →  TOTAL 703
--
--   -- As duas somas TÊM que bater com o universo:
--   select (select count(*) from documentos) as notas,
--          (select sum(quantas) from fechamento_notas) as somadas,
--          (select count(*) from orcamentos) as orcamentos,
--          (select sum(quantos) from fechamento_orcamentos) as somados;
--
--   -- Ninguém pode cair no balde do desconhecido:
--   select * from fechamento_notas where ordem = 99;
--   select * from fechamento_orcamentos where ordem = 99;
--   -- esperado: nenhuma linha
--
--   -- E a ponte, com os três órfãos zerados:
--   select * from fechamento_ponte;
--
--   -- A tranca da 033 continua de pé depois desta migração:
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relnamespace = 'public'::regnamespace order by relname;
--   -- esperado: TODAS com {security_invoker=true}
--
-- PARA DESFAZER
--   drop view fechamento_ponte, fechamento_orcamentos, fechamento_notas;
--   e repita os `create or replace view` da 031 e da 020 — SEM esquecer o
--   `with (security_invoker = true)`.
-- =============================================================================
