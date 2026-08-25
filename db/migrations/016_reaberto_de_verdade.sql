-- =============================================================================
-- 016 — "reaberto" estava marcando quem nunca foi reaberto             rev 1
-- =============================================================================
--
-- DEFEITO DA 015, ACHADO CONFERINDO A SAÍDA CONTRA O BANCO
--
--   A 015 definiu reaberto como "existe evento de mudança de status". Isso é
--   verdade para QUALQUER chamado que já andou alguma vez — inclusive um que só
--   foi executado, vistoriado e arquivado, sem nunca ter voltado.
--
--   Medido: a lista do cliente marcava 17 de 17 como reabertos, sendo que 10
--   eram arquivados. A situação sairia escrita como "Arquivado (reaberto)" numa
--   planilha enviada AO CLIENTE.
--
--   O roteamento não estava errado — o `destino` testa arquivado antes de olhar
--   reaberto. Errado estava o RÓTULO, e ele é o que a pessoa lê.
--
-- A DEFINIÇÃO CERTA
--
--   Reaberto é o chamado que JÁ TINHA TERMINADO e voltou: existe um evento que o
--   colocou em Executado (5), Vistoriado (6) ou Arquivado (3), e hoje ele está
--   em Aberto (1) ou Em execução (7).
--
--   Os eventos carregam o código do status de destino — 5.273 dos 5.275 eventos
--   de status têm o campo preenchido. É esse campo que a 015 não usou.
--
--   Pela regra nova, a base inteira tem 17 reabertos: 15 em Aberto e 2 em Em
--   execução. Nenhum arquivado, nenhum vistoriado.
--
-- É segura de rodar duas vezes.
-- =============================================================================

create or replace view orcamentos_lista
with (security_invoker = true) as
select o.id, o.cliente_id, o.ticket, o.parte, o.conta, o.status,
       o.valor, o.valor_nota, o.reduzido_pelo_teto, o.valor_antes_do_teto,
       o.rateio, o.criado_em, o.lancado_em, o.faturado, o.pago,
       o.trilogo_custo_id, o.arquivo_pdf_sha256,
       u.nome as loja,
       c.descricao as chamado_descricao,
       (select string_agg(d.numero, ', ' order by d.numero)
          from orcamento_documentos od join documentos d on d.id = od.documento_id
         where od.orcamento_id = o.id and d.numero is not null) as notas,
       (select string_agg(d.dav_numero, ', ' order by d.dav_numero)
          from orcamento_documentos od join documentos d on d.id = od.documento_id
         where od.orcamento_id = o.id and d.dav_numero is not null) as davs,
       o.lancamento_bloqueio,
       o.lancamento_bloqueio_detalhe,
       o.lancamento_tentado_em,
       o.lancamento_tentativas,
       c.status        as ticket_status,
       c.status_codigo as ticket_status_codigo,
       -- AQUI ESTÁ O CONSERTO: além de já ter terminado alguma vez, o chamado
       -- precisa estar HOJE de volta em Aberto ou Em execução.
       (c.status_codigo in (1, 7) and r.ja_terminou) as reaberto,
       case when c.status_codigo in (1, 7) and r.ja_terminou
            then r.ultima_frase end as motivo_reabertura,
       case
         when o.status <> 'gerado'            then null
         when c.id is null                    then 'sem_chamado'
         when c.status_codigo in (5, 6)       then 'pode_lancar'
         when c.status_codigo = 3             then 'cliente'
         when c.status_codigo = 1 and r.ja_terminou then 'cliente'
         when c.status_codigo = 1             then 'encarregados'
         when c.status_codigo = 7             then 'encarregados'
         else 'outro'
       end as destino,
       (select a.avisado_em from ticket_avisos a
         where a.cliente_id = o.cliente_id and a.ticket = o.ticket
         order by a.avisado_em desc limit 1) as avisado_em
from orcamentos o
left join unidades u on u.id = o.unidade_id
left join chamados c on c.id = o.chamado_id
left join lateral (
  select exists (
    select 1 from chamado_eventos e
     where e.chamado_id = c.id and e.tipo = 'status'
       and e.status_codigo in (3, 5, 6)
  ) as ja_terminou,
  -- A frase de quem mandou o chamado de volta. É ela que muda a conversa com o
  -- cliente: em vez de "reabriram", a lista diz "reabriram porque o serviço não
  -- foi executado".
  (select e.texto from chamado_eventos e
    where e.chamado_id = c.id and e.tipo = 'status'
      and e.status_codigo in (1, 7)
      and coalesce(e.texto, '') <> ''
    order by e.quando desc limit 1) as ultima_frase
) r on true;


insert into schema_migrations (versao, arquivo)
values ('016', '016_reaberto_de_verdade.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select ticket_status, count(*), count(*) filter (where reaberto) as reabertos
--     from orcamentos_lista where status = 'gerado' group by 1;
--
--   Arquivado e Vistoriado têm que dar ZERO reabertos.
--
-- PARA DESFAZER
--   rodar a 015 de novo — ela é `create or replace`.
-- =============================================================================
