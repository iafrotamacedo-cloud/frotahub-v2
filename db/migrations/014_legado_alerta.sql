-- =============================================================================
-- 014 — a marca do que veio torto do legado                              rev 1
-- =============================================================================
--
-- A carga do sistema antigo traz 17 orçamentos que não fecham. Eles ENTRAM —
-- são lançamentos reais, cobrados de verdade, e esconder isso seria pior. Mas
-- entram MARCADOS, para que quem olhar a tela saiba que aquele número não foi
-- conferido por ninguém.
--
-- POR QUE UMA COLUNA, E NÃO UM RELATÓRIO À PARTE
--
--   Relatório à parte envelhece e some. A marca no próprio registro anda junto
--   com ele para sempre: a linha carrega a própria ressalva.
--
-- POR QUE LISTA FECHADA
--
--   O mesmo motivo de `leitura_camada` e de `status`: a tela vai desviar o
--   comportamento por este valor. Se qualquer texto passar, um erro de digitação
--   vira uma linha que nenhuma tela mostra — e o defeito fica invisível, que é
--   exatamente o contrário do que esta coluna existe para fazer.
--
-- É segura de rodar duas vezes.
-- =============================================================================

alter table orcamentos add column if not exists legado_alerta text;

alter table orcamentos drop constraint if exists orcamentos_legado_alerta_check;
alter table orcamentos add constraint orcamentos_legado_alerta_check
  check (legado_alerta is null or legado_alerta in (
    'sem_itens',        -- o orçamento existe e tem valor, mas a lista de itens
                        -- nunca foi gravada no sistema antigo. 10 casos.
    'itens_nao_somam'   -- a soma dos itens diverge do valor da nota. Medido: as
                        -- 7 divergências vão de 7 a 35 centavos — é o
                        -- arredondamento da leitura antiga, não valor errado.
  ));

comment on column orcamentos.legado_alerta is
  'Defeito conhecido herdado do sistema antigo. Nulo = a linha fecha. Nunca é escrito por nada que nasce aqui.';

-- A fila de "conferir um dia". Índice parcial: só as linhas marcadas ocupam
-- espaço, então listar os defeitos é instantâneo mesmo com o banco cheio.
create index if not exists orcamentos_com_alerta
  on orcamentos (cliente_id, legado_alerta)
  where legado_alerta is not null;

insert into schema_migrations (versao, arquivo)
values ('014', '014_legado_alerta.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select legado_alerta, count(*) from orcamentos group by 1;
--
-- PARA DESFAZER
--   drop index if exists orcamentos_com_alerta;
--   alter table orcamentos drop column if exists legado_alerta;
--   delete from schema_migrations where versao = '014';
-- =============================================================================
