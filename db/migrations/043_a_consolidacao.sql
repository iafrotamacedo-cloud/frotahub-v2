-- =============================================================================
-- 043 — a consolidação: nota × ticket, nos dois sentidos                 rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   Duas views de leitura para a tela Financeiro › Consolidação, que substitui
--   o "Balanço" que estava marcado como "em breve". Nenhuma tabela nova,
--   nenhuma coluna nova: é junção do que já existe.
--
-- POR QUE DUAS VIEWS, E NÃO UMA
--
--   Porque são duas PERGUNTAS, e elas não têm o mesmo grão.
--
--     `consolidacao_notas`   — uma linha por NOTA. "Comprei isto; quanto já
--                              voltou do cliente?"
--     `consolidacao_tickets` — uma linha por ORÇAMENTO. "Cobrei isto; de que
--                              nota saiu, e ela já foi paga ao fornecedor?"
--
--   Uma view só teria que escolher um dos dois grãos e a outra pergunta sairia
--   multiplicada: nota rateada em catorze tickets viraria catorze linhas de
--   nota, e a soma dos valores das notas passaria a mentir. Foi exatamente o
--   erro que apareceu na primeira medição desta tela — R$ 220.787,51 de
--   pendente onde o certo era R$ 59.185,21, porque dois `left join` se
--   multiplicaram. Sub-consulta por linha, e não junção, é o que impede isso.
--
-- O QUE A MEDIÇÃO DE 01/09/2026 MOSTROU, E QUE PRECISA ESTAR ESCRITO
--
--   Os 887 orçamentos são DOIS MUNDOS que ainda não se cruzam:
--
--     247 novos (gerados aqui)  — TODOS têm nota ligada. NENHUM entrou em
--                                 fatura ainda.
--     635 do legado (migrados)  — só 49 têm nota. 245 estão em fatura.
--
--   Logo `recebido` sai R$ 0,00 em toda nota, hoje, e não é defeito: é que
--   nenhum orçamento ligado a uma nota chegou ao relatório mensal. O número
--   deixa de ser zero no primeiro fechamento que incluir os orçamentos novos.
--
--   Por isso `lancado` também sai da view, ao lado. Ele é o marco que JÁ tem
--   número (762 orçamentos lançados), e é o que permite ver a tela viva
--   enquanto o outro não enche. A tela escolhe o que mostrar; a view entrega os
--   dois, para ninguém precisar de uma segunda consulta para saber onde o
--   dinheiro parou.
--
-- A NF DO OBRA PRIMA AINDA NÃO ESTÁ AQUI
--
--   `nf` é a NOSSA chave: `numero` quando o fornecedor emitiu nota, `dav_numero`
--   quando emitiu DAV. Medido em 01/09/2026: 25 notas com número próprio
--   (R$ 21.965,53) e 227 DAVs (R$ 29.188,75).
--
--   Para a SV o número bate com o do Obra Prima. Para a Rodrigues NÃO: as DAVs
--   viram uma nota-pacote que só existe lá. Enquanto o CSV do Obra Prima não
--   for importado, esta tela consolida o nosso lado contra si mesmo — o que
--   ainda pega orçamento não lançado e nota sem orçamento, mas NÃO pega a nota
--   que existe no Obra Prima e nunca entrou aqui. Esse é o furo que sobra, e
--   ele é conhecido: R$ 2.103,55 no pacote 9192 e R$ 448,06 no 9220.
--
-- ⚠ `create or replace view` NÃO É USADO AQUI
--   As duas views são novas, então nascem com `create view`. A tranca
--   `security_invoker = true` vai em cada uma (P-35): sem ela a view roda com os
--   direitos de quem a criou e fura o RLS de todo mundo.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- ABA 1 — OBRA PRIMA × FROTAHUB : uma linha por NOTA
-- -----------------------------------------------------------------------------
--
-- NF / VALOR / TICKETS / RECEBIDO / PENDENTE, mais o que a tela precisa para
-- abrir a lista de tickets sem uma segunda ida ao banco.
--
-- A nota oculta e a repetida ficam de fora: a primeira foi descartada por
-- alguém, a segunda é a mesma nota contada duas vezes — e somar as duas é como
-- o total do período passa a não bater com nada.

create view consolidacao_notas
with (security_invoker = true) as
select
    d.id                                                    as documento_id,
    d.cliente_id,
    coalesce(d.numero, d.dav_numero)                        as nf,
    case when d.numero is not null then 'NF' else 'DAV' end as tipo,
    d.emitente_nome                                         as fornecedor,
    d.emissao,
    d.valor_total                                           as valor,
    d.fila,
    d.status,
    coalesce(t.quantos, 0::bigint)                          as tickets,
    coalesce(t.lista, '{}'::integer[])                      as ticket_numeros,
    coalesce(o.quantos, 0::bigint)                          as orcamentos,
    coalesce(o.orcado, 0::numeric)                          as orcado,
    coalesce(o.recebido, 0::numeric)                        as recebido,
    coalesce(o.pendente, 0::numeric)                        as pendente,
    coalesce(o.lancado, 0::numeric)                         as lancado,
    d.inserido_em
  from documentos d
  -- OS TICKETS DA NOTA, CONTADOS UMA VEZ SÓ
  --   `documento_tickets` pode ter o mesmo ticket em duas linhas quando alguém
  --   corrige a associação; `distinct` é o que impede a nota de dizer que cobre
  --   três chamados quando cobre dois.
  left join lateral (
      select count(distinct dt.ticket)                              as quantos,
             array_agg(distinct dt.ticket order by dt.ticket)       as lista
        from documento_tickets dt
       where dt.documento_id = d.id
  ) t on true
  left join lateral (
      select count(*)                                                        as quantos,
             sum(orc.valor)                                                  as orcado,
             sum(orc.valor) filter (where orc.fatura_id is not null)         as recebido,
             sum(orc.valor) filter (where orc.fatura_id is null)             as pendente,
             sum(orc.valor) filter (where orc.lancado_em is not null)        as lancado
        from orcamento_documentos od
        join orcamentos orc on orc.id = od.orcamento_id
       where od.documento_id = d.id
         and orc.removido_em is null
  ) o on true
 where d.oculto_em is null
   and d.duplicada_de is null;

comment on view consolidacao_notas is
  'Uma linha por NOTA do fornecedor: quanto ela custou, quantos tickets cobre, '
  'quanto já entrou no relatório mensal (recebido) e quanto falta (pendente). '
  'RODA COMO QUEM PERGUNTA (security_invoker). `recebido` sai zero enquanto '
  'nenhum orçamento ligado a nota tiver entrado em fechamento — ver o cabeçalho '
  'da migração 043. `nf` é a NOSSA chave (número ou DAV), não a nota-pacote do '
  'Obra Prima.';

-- -----------------------------------------------------------------------------
-- ABA 2 — FROTAHUB × OBRA PRIMA : uma linha por ORÇAMENTO
-- -----------------------------------------------------------------------------
--
-- TICKET / VALOR / NF'S / RECEBIDO (S/N) / PAGO (S/N).
--
-- É por ORÇAMENTO e não por ticket porque um ticket pode ter mais de um: a
-- coluna `parte` existe justamente para isso. Agrupar por ticket somaria duas
-- compras diferentes numa linha só e perderia de qual nota veio cada uma.

create view consolidacao_tickets
with (security_invoker = true) as
select
    orc.id                                as orcamento_id,
    orc.cliente_id,
    orc.ticket,
    orc.parte,
    u.nome                                as loja,
    orc.conta,
    orc.valor,
    orc.valor_nota,
    -- AS NOTAS DO ORÇAMENTO, EM TEXTO
    --   Quase sempre é uma. Rateio ao contrário — duas notas para o mesmo
    --   ticket — existe e é legítimo, então a coluna é uma lista, não um campo.
    n.nfs,
    -- A NOTA FOI JOGADA FORA E O ORÇAMENTO FICOU DE PÉ
    --
    --	Medido em 01/09/2026: dois orçamentos vivos saíram de notas depois
    --	ocultadas — ticket 126861 (R$ 332,16, JÁ LANÇADO no Trílogo, NF 9214) e
    --	ticket 125371 (R$ 193,32, NF 9160). Somam os R$ 525,48 que faziam as
    --	duas abas discordarem.
    --
    --	A aba das notas não os enxerga, e está certa: a nota foi descartada, não
    --	deve entrar em soma nenhuma. Mas a cobrança ao cliente continua de pé, e
    --	ninguém tinha onde ver isso. É exatamente o buraco que a Consolidação
    --	existe para achar — então ele sai marcado, não escondido.
    coalesce(n.excluida, false)           as nota_excluida,
    (orc.fatura_id is not null)           as no_relatorio,
    orc.faturado,
    orc.pago,
    orc.status,
    orc.lancado_em,
    orc.rateio,
    -- O LEGADO PRECISA SER VISÍVEL, NÃO ESCONDIDO
    --   586 dos 887 orçamentos vieram da migração sem nota ligada. Se a coluna
    --   NF saísse vazia sem explicação, a tela pareceria com defeito. Marcado,
    --   o usuário sabe que é história, não erro.
    (orc.legado_id is not null)           as legado,
    orc.criado_em
  from orcamentos orc
  left join unidades u on u.id = orc.unidade_id
  left join lateral (
      select string_agg(distinct coalesce(d.numero, d.dav_numero), ', ') as nfs,
             bool_and(d.oculto_em is not null or d.duplicada_de is not null) as excluida
        from orcamento_documentos od
        join documentos d on d.id = od.documento_id
       where od.orcamento_id = orc.id
  ) n on true
 where orc.removido_em is null;

comment on view consolidacao_tickets is
  'Uma linha por ORÇAMENTO: o ticket, o valor cobrado, de que nota saiu, se já '
  'entrou no relatório mensal e se já foi pago ao fornecedor. RODA COMO QUEM '
  'PERGUNTA (security_invoker). `legado` marca o que veio da migração — esses '
  'não têm nota ligada, e a coluna NF sai vazia neles por isso. `nota_excluida` '
  'marca o orçamento cuja nota foi descartada depois: ele não aparece na aba '
  'das notas, mas a cobrança ao cliente continua de pé.';

-- -----------------------------------------------------------------------------
-- a rotina, e quem já pode ver
-- -----------------------------------------------------------------------------
--
-- Herda de CONTRATO_FINANCEIRO_PAGAR: quem controla o que se deve ao fornecedor
-- é quem tem motivo para conferir a consolidação. Nasce sem ninguém novo.

insert into rotinas (codigo, nome, modulo, ordem) values
  ('CONTRATO_FINANCEIRO_CONSOLIDACAO', 'Consolidação', 'manutencao', 328)
on conflict (codigo) do nothing;

insert into categoria_permissoes (categoria_id, rotina, pode)
select cp.categoria_id, 'CONTRATO_FINANCEIRO_CONSOLIDACAO', cp.pode
  from categoria_permissoes cp
 where cp.rotina = 'CONTRATO_FINANCEIRO_PAGAR'
on conflict (categoria_id, rotina) do nothing;

insert into schema_migrations (versao, arquivo)
values ('043', '043_a_consolidacao.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a tranca de pé nas duas (é o que a 033 ensinou a checar):
--   select relname, reloptions from pg_class
--    where relkind = 'v' and relname like 'consolidacao_%';
--   -- esperado: as DUAS com {security_invoker=true}
--
--   -- a soma das notas NÃO pode multiplicar por ticket:
--   select count(*), sum(valor) from consolidacao_notas;
--   -- esperado em 01/09/2026: 252 notas, R$ 51.154,28
--
   -- as duas abas contam o mesmo pendente, MENOS o que saiu de nota excluída:
--   select (select sum(pendente) from consolidacao_notas)                    as pela_nota
--        , (select sum(valor) from consolidacao_tickets
--            where not no_relatorio and nfs is not null
--              and not nota_excluida)                                        as pelo_ticket;
--   -- esperado: iguais (R$ 59.185,21 em 01/09/2026)
--
--   -- e a diferença tem nome e endereço:
--   select ticket, valor, nfs, status from consolidacao_tickets
--    where nota_excluida and not no_relatorio;
--   -- esperado: 126861 R$ 332,16 (NF 9214, lançado) e 125371 R$ 193,32 (NF 9160)
--   --           somando os R$ 525,48
--
--   -- o retrato dos dois mundos, que explica o `recebido` zerado:
--   select legado, count(*), count(*) filter (where nfs is not null) as com_nota
--        , count(*) filter (where no_relatorio) as em_fatura
--     from consolidacao_tickets group by legado;
--   -- esperado: legado=false → 247 / 247 com nota / 0 em fatura
--   --           legado=true  → 635 /  49 com nota / 245 em fatura
--
--   -- e a rotina tem que ter alcançado alguém:
--   select count(*) from categoria_permissoes
--    where rotina = 'CONTRATO_FINANCEIRO_CONSOLIDACAO' and pode;
--   -- esperado: o mesmo número de CONTRATO_FINANCEIRO_PAGAR
--
-- PARA DESFAZER
--   drop view consolidacao_notas, consolidacao_tickets;
--   delete from categoria_permissoes where rotina = 'CONTRATO_FINANCEIRO_CONSOLIDACAO';
--   delete from rotinas where codigo = 'CONTRATO_FINANCEIRO_CONSOLIDACAO';
--   delete from schema_migrations where versao = '043';
-- =============================================================================
