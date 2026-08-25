-- =============================================================================
-- 011 — a nota rateada não passa com ticket solto                        rev 2
-- =============================================================================
--
-- POR QUE ISTO É CORREÇÃO, E NÃO PREFERÊNCIA
--
--   Numa nota rateada o valor de cada orçamento DEPENDE de quantos tickets a
--   nota atende. Se um dos tickets não existe na nossa base e a geração segue
--   com os que sobraram, todos os orçamentos daquela nota saem com o valor
--   errado — e saem parecendo certos, porque cada um fecha a própria conta.
--
--   O erro é silencioso e caro: só apareceria meses depois, conferindo nota por
--   nota. A regra passa a ser: a nota só é processada quando TODOS os tickets
--   dela existem.
--
-- A COLUNA NOVA VAI NO FIM, E ISSO NÃO É ESTILO
--
--   `create or replace view` no Postgres só sabe ACRESCENTAR coluna no fim. Ele
--   compara posição por posição; uma coluna nova no meio faz a de trás parecer
--   renomeada, e ele recusa:
--
--       42P16: cannot change name of view column "itens" to "ticket_soltos"
--
--   A rev 1 desta migration fazia exatamente isso. A alternativa seria
--   `drop view ... cascade`, que derruba junto tudo que depende da visão — e o
--   painel depende. Acrescentar no fim resolve sem derrubar nada, e a ordem das
--   colunas não importa para quem lê: o motor e a tela leem por NOME.
--
-- É segura de rodar duas vezes.
-- =============================================================================

create or replace view documentos_lista
with (security_invoker = true) as
select d.id, d.cliente_id, d.fila, d.tipo, d.numero, d.dav_numero, d.chave_acesso,
       d.emitente_nome, d.emissao, d.valor_total, d.status,
       d.leitura_camada, d.leitura_confianca,
       d.nome_arquivo, d.arquivo_sha256, d.inserido_em, d.oculto_em,
       coalesce(t.quantos, 0)  as tickets,
       coalesce(t.lista, '{}') as ticket_numeros,
       coalesce(i.quantos, 0)  as itens,

       -- ---- daqui para baixo é o que a 011 acrescenta ----

       -- Os tickets que o usuário escreveu e a nossa base não conhece. É esta
       -- lista que a tela pinta de âmbar, e é ela que trava a geração.
       coalesce(t.soltos, '{}') as ticket_soltos,

       -- Pronta = lida, com pelo menos um ticket, e nenhum ticket solto.
       (d.status = 'lido'
        and coalesce(t.quantos, 0) > 0
        and coalesce(array_length(t.soltos, 1), 0) = 0) as pronto_para_gerar
from documentos d
left join lateral (
  select count(*) as quantos,
         array_agg(dt.ticket order by dt.ticket) as lista,
         array_remove(array_agg(
           case when dt.chamado_id is null then dt.ticket end
           order by dt.ticket), null) as soltos
  from documento_tickets dt where dt.documento_id = d.id
) t on true
left join lateral (
  select count(*) as quantos from documento_itens di where di.documento_id = d.id
) i on true;


-- O painel também passa a contar as notas travadas e as prontas: são números
-- que o usuário precisa ver ANTES de clicar em gerar e não achar nada.
-- Mesma regra de ordem: as duas colunas novas vão no FIM.
create or replace view orcamentos_painel
with (security_invoker = true) as
select cl.id as cliente_id,
  (select count(*) from documentos d
     where d.cliente_id = cl.id and d.fila = 'orcamento' and d.oculto_em is null) as notas_arquivos,
  (select count(*) from documentos d
     where d.cliente_id = cl.id and d.fila = 'rateio' and d.oculto_em is null
       and not exists (select 1 from documento_tickets t where t.documento_id = d.id)) as rateio_sem_ticket,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'gerado') as a_lancar,
  (select count(*) from documentos d
     where d.cliente_id = cl.id and d.oculto_em is null and d.status = 'lido'
       and d.fila = 'orcamento'
       and not exists (select 1 from documento_tickets t where t.documento_id = d.id)) as sem_ticket,
  (select count(*) from documento_tickets t
     join documentos d on d.id = t.documento_id
     where d.cliente_id = cl.id and d.oculto_em is null and t.chamado_id is null) as sem_associacao,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'aguardando_aprovacao') as aguardando_aprovacao,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'removido') as apagados,
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status <> 'removido') as no_total,
  (select coalesce(sum(o.valor), 0) from orcamentos o
     where o.cliente_id = cl.id and o.status <> 'removido') as valor_total,

  -- ---- daqui para baixo é o que a 011 acrescenta ----

  (select count(*) from documentos_lista dl
     where dl.cliente_id = cl.id and dl.oculto_em is null
       and array_length(dl.ticket_soltos, 1) > 0) as notas_travadas,
  (select count(*) from documentos_lista dl
     where dl.cliente_id = cl.id and dl.oculto_em is null and dl.pronto_para_gerar) as prontas_para_gerar
from clientes cl;

-- =============================================================================
-- CONFERÊNCIA
--   select nome_arquivo, ticket_numeros, ticket_soltos, pronto_para_gerar
--     from documentos_lista where oculto_em is null order by inserido_em desc;
--
--   select notas_travadas, prontas_para_gerar from orcamentos_painel;
-- =============================================================================
