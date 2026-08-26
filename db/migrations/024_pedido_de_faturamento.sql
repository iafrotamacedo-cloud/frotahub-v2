-- =============================================================================
-- 024 — o pedido de faturamento ao fornecedor                            rev 1
-- =============================================================================
--
-- O LADO DE CÁ DO BALCÃO
--
--   A 017 tratou do que a gente COBRA do cliente. Esta trata do que a gente
--   DEVE ao fornecedor — e o mecanismo dele é diferente do nosso.
--
--   A Rodrigues não emite nota a cada compra. Ela emite um DAV (documento
--   auxiliar de venda, do SysPDV) e vai acumulando. De tempos em tempos nós
--   mandamos a ela a relação das DAVs em aberto, e ela emite UMA nota fiscal
--   cobrindo todas. Sem esse pedido, o DAV fica solto e ninguém cobra ninguém.
--
-- POR QUE PRECISA DE UMA TABELA, E NÃO DE UM `booleano` NA DAV
--
--   Porque a pergunta que aparece depois não é "esta DAV foi pedida?", é "o que
--   eu mandei para ela no dia tal, e o que voltou?". Uma coluna responde a
--   primeira e perde a segunda — e a segunda é a que resolve discussão.
--
--   É o mesmo desenho da 017: o CICLO guarda o corte que decidiu quem entrou.
--   Sem ele, daqui a um ano ninguém explica por que uma DAV de 25/08 ficou de
--   fora do pedido de 26/08.

create table if not exists pedidos_faturamento (
  id           uuid primary key default gen_random_uuid(),
  cliente_id   uuid not null references clientes(id) on delete restrict,
  -- Número sequencial por cliente, para a relação ter nome quando alguém
  -- perguntar "qual pedido?" no telefone.
  numero       integer not null,
  -- O CORTE. Entra quem foi emitido ATÉ aqui. É o campo que explica a lista
  -- seis meses depois.
  ate          date not null,
  fechado_em   timestamptz,
  enviado_em   timestamptz,
  observacao   text,
  criado_em    timestamptz not null default now(),
  criado_por   uuid references perfis(id) on delete set null
);

create unique index if not exists pedido_numero_por_cliente
  on pedidos_faturamento (cliente_id, numero);

-- ⚠ O VÍNCULO É NA DAV, E É ELE QUE IMPEDE PEDIR DUAS VEZES
--
--   Sem esta coluna, a mesma DAV entraria no pedido de hoje e no da semana que
--   vem — e a Rodrigues emitiria duas notas cobrando o mesmo material. O
--   critério da fila é `pedido_id is null`, exatamente como `fatura_id is null`
--   é o critério do lado do cliente.
alter table documentos
  add column if not exists pedido_id uuid references pedidos_faturamento(id) on delete set null;

comment on column documentos.pedido_id is
  'Em qual pedido de faturamento esta DAV foi mandada ao fornecedor. '
  'Nulo = ainda não foi pedida, e é esse o critério da fila.';

create index if not exists dav_a_pedir
  on documentos (cliente_id, emissao)
  where pedido_id is null and tipo = 'dav' and oculto_em is null;

create index if not exists dav_por_pedido
  on documentos (pedido_id) where pedido_id is not null;

-- -----------------------------------------------------------------------------
-- a lista dos pedidos, com o que cada um levou
-- -----------------------------------------------------------------------------
create or replace view pedidos_faturamento_lista as
 SELECT p.id,
    p.cliente_id,
    p.numero,
    p.ate,
    p.fechado_em,
    p.enviado_em,
    p.observacao,
    p.criado_em,
    COALESCE(d.quantas, 0::bigint) AS davs,
    COALESCE(d.total, 0::numeric) AS valor
   FROM pedidos_faturamento p
     LEFT JOIN LATERAL ( SELECT count(*) AS quantas, sum(x.valor_total) AS total
           FROM documentos x
          WHERE x.pedido_id = p.id) d ON true;

-- -----------------------------------------------------------------------------
-- a permissão
--
-- Herda de quem já podia faturar ao cliente: é a mesma pessoa que cuida de
-- dinheiro. Categoria nova nasce sem ela, como todas.
-- -----------------------------------------------------------------------------
insert into rotinas (codigo, nome, modulo, ordem) values
  ('CONTRATO_FINANCEIRO_PAGAR', 'A pagar — pedido de faturamento', 'manutencao', 327)
on conflict (codigo) do nothing;

insert into categoria_permissoes (categoria_id, rotina, pode)
select cp.categoria_id, 'CONTRATO_FINANCEIRO_PAGAR', cp.pode
  from categoria_permissoes cp
 where cp.rotina = 'CONTRATO_ORCAMENTOS_FATURAR'
on conflict (categoria_id, rotina) do nothing;

insert into schema_migrations (versao, arquivo)
values ('024', '024_pedido_de_faturamento.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- as DAVs em aberto (o que a tela vai mostrar):
--   select count(*), round(sum(valor_total),2) from documentos
--    where tipo='dav' and pedido_id is null and oculto_em is null;
--
--   -- e depois de fechar um pedido, elas somem da fila e aparecem nele:
--   select numero, ate, davs, valor from pedidos_faturamento_lista;
--
-- PARA DESFAZER
--   update documentos set pedido_id = null where pedido_id = '<id>';
--   delete from pedidos_faturamento where id = '<id>';
-- =============================================================================
