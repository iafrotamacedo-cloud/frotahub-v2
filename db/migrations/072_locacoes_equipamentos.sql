-- =============================================================================
-- 072 — Locações: o modelo de dados do módulo                            rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (15-16/09/2026)
--
--   A Frota Macedo aluga equipamento para as obras (andaime, betoneira,
--   gerador...) e hoje isso não tem controle nenhum: ninguém sabe quando um
--   equipamento vence, se foi renovado com OC de verdade ou se foi devolvido
--   com prova. A OC de locação entra pela MESMA esteira de Compras/PCO — zero
--   mudança ali. A bifurcação acontece no recebimento: na fila "Aguardando
--   NF" (`nf_progresso_ordens`), em vez de escanear a nota, o almoxarife
--   escolhe "Locação" — a OC não tem nota (é aluguel, não compra), então a
--   prova de entrada é o romaneio, não a NF.
--
--   Decisões fechadas na sessão de planejamento (ver o plano salvo em
--   Locações, 16/09/2026):
--     - Unidade de controle é o EQUIPAMENTO (um por item da OC), não a OC.
--     - Prazo não vem do PDF — o almoxarife define no recebimento (default
--       1 mês). Quantidade recebida é ajustável por item, com fotos.
--     - Intertravamento "opção A": Devolver/Renovar por equipamento, um
--       bloqueia o outro enquanto o outro está em andamento.
--     - Renovação em DUAS etapas: a hierarquia da obra PEDE
--       (`locacoes_renovacoes`, estado pendente), o RC CONCLUI inserindo a OC
--       de renovação de verdade — o pedido em si nunca cobre o período, só a
--       OC concluída cria o próximo `locacoes_periodos`.
--     - Devolução exige romaneio de devolução + fotos do estado de saída.
--     - Faturamento (mês cheio × proporcional, com override por
--       fornecedor/equipamento) é fase futura — aqui só nascem os campos que
--       tornam essa conta possível depois.
--
-- POR QUE TABELAS NOVAS, E NÃO DENTRO DE `notas_fiscais`
--
--   A prova de entrada de uma locação (romaneio) e a de uma compra (NF) são
--   coisas diferentes, e depois do recebimento a locação vive um ciclo que a
--   compra nunca teve — vencimento, renovação, devolução. Forçar isso dentro
--   de `notas_fiscais` seria metade das colunas sempre nulas para um objeto
--   de negócio diferente (mesmo raciocínio da 059 sobre `documentos`).
--
-- POR QUE O CONTROLE É O EQUIPAMENTO, E A OC SÓ A COBERTURA
--
--   Dentro da MESMA OC, um equipamento pode ser devolvido no fim do mês e
--   outro renovado — não dá pra tratar a OC como a unidade que vence.
--   `locacoes_equipamentos` é a linha do tempo de cada equipamento;
--   `locacoes_periodos` é a cobertura (a OC original é o período 1, cada
--   renovação é o período seguinte, sempre CONTÍGUO — início = fim do
--   anterior, garantido pelo motor, não por constraint de banco).
--
-- `ordens_compra.destino_recebimento` — A MESMA VIEW, SEM DUPLICAR ESTADO
--
--   `nf_progresso_ordens` (065) já é quem decide o que aparece em
--   "Aguardando NF". Uma coluna nova na própria OC ('nf' ou 'locacao') e um
--   filtro a mais na view bastam para uma OC recebida como locação (ou a OC
--   de renovação, quando a Fase 4 chegar) nunca mais aparecer ali — sem
--   segunda fonte de verdade sobre "essa OC é locação".
-- =============================================================================
begin;

-- -----------------------------------------------------------------------------
-- ordens_compra e fornecedores ganham as colunas que faltam
-- -----------------------------------------------------------------------------

alter table public.ordens_compra
  add column if not exists destino_recebimento text
    check (destino_recebimento in ('nf','locacao'));

comment on column public.ordens_compra.destino_recebimento is
  'Nula até o recebimento decidir. "nf" ou "locacao" tiram a OC de
  "Aguardando NF" (ver a view nf_progresso_ordens abaixo) — uma OC de
  renovação (Fase 4) já nasce com "locacao".';

alter table public.fornecedores
  add column if not exists regra_faturamento text not null default 'padrao'
    check (regra_faturamento in ('padrao','sempre_proporcional','sempre_mes_cheio'));

comment on column public.fornecedores.regra_faturamento is
  'Override da regra "mês cheio no 1º período, proporcional depois" por
  locadora — "padrao" segue a regra geral. Fase 5 (faturamento) lê isto;
  antes disso a coluna só existe para não precisar de outra migração depois.';

-- `with (security_invoker = true)`: sem repetir a cláusula aqui, um
-- `create or replace view` derruba a tranca que a view já deveria ter e ela
-- passa a rodar como dona das tabelas, ignorando as políticas de quem
-- pergunta (ver o cabeçalho de orcamentos/migracoes_test.go,
-- TestReescreverViewNaoDerrubaOSecurityInvoker).
create or replace view public.nf_progresso_ordens
with (security_invoker = true) as
select
  oc.id as ordem_compra_id,
  oc.cliente_id,
  oc.numero,
  oc.obra_centro_custo,
  oc.fornecedor_id,
  oc.total,
  coalesce(nf.recebido, 0) as recebido,
  oc.total - coalesce(nf.recebido, 0) as restante,
  (coalesce(nf.recebido, 0) >= oc.total) as completa,
  oc.pco_enviado_em
from public.ordens_compra oc
left join (
  select ordem_compra_id, sum(valor) as recebido
  from public.notas_fiscais
  where not cancelada
  group by ordem_compra_id
) nf on nf.ordem_compra_id = oc.id
where oc.status = 'lido' and oc.pco_enviado_em is not null
  and coalesce(oc.destino_recebimento, 'nf') = 'nf';

comment on view public.nf_progresso_ordens is
  'Uma linha por OC enviada ao PCO, com o quanto já chegou de nota fiscal.
  "completa" é quem sai da fila "Aguardando NF" (nf.go) — e, desde a 072,
  destino_recebimento = ''locacao'' tira a OC da view inteira, do mesmo jeito:
  quem escolheu "Locação" no recebimento não deveria ver essa OC pedindo NF
  de novo. Sem RLS própria (mesma disciplina do resto do módulo).';

-- -----------------------------------------------------------------------------
-- locacoes_recebimentos — um por ato de recebimento (a "porta de entrada")
-- -----------------------------------------------------------------------------

create table public.locacoes_recebimentos (
  id                uuid primary key default gen_random_uuid(),
  cliente_id        uuid not null references public.clientes(id),
  ordem_compra_id   uuid not null unique references public.ordens_compra(id) on delete restrict,

  data_recebimento  date not null,
  recebido_por      uuid references public.perfis(id),

  romaneio_sha256   text not null references public.arquivos(sha256) on delete restrict,
  nf_numero         text,
  nf_sha256         text references public.arquivos(sha256) on delete restrict,
  observacao        text,

  criado_em         timestamptz not null default now()
);

comment on table public.locacoes_recebimentos is
  'O recebimento que decidiu "isto é locação" — uma OC só pode ser recebida
  como locação uma vez (unique em ordem_compra_id). O romaneio é a prova de
  entrada (obrigatório, é o que substitui a NF); a NF em si é opcional aqui —
  às vezes vem junto com o equipamento mesmo sem ser o documento de entrada.';

create table public.locacoes_recebimentos_paginas (
  id               uuid primary key default gen_random_uuid(),
  cliente_id       uuid not null references public.clientes(id),
  recebimento_id   uuid not null references public.locacoes_recebimentos(id) on delete cascade,
  pagina           int not null check (pagina >= 2),
  arquivo_sha256   text not null references public.arquivos(sha256) on delete restrict,
  criado_em        timestamptz not null default now(),
  unique (recebimento_id, pagina)
);

comment on table public.locacoes_recebimentos_paginas is
  'Páginas 2+ do romaneio — mesmo desenho de notas_fiscais_paginas (071):
  a página 1 mora em locacoes_recebimentos.romaneio_sha256.';

-- -----------------------------------------------------------------------------
-- locacoes_equipamentos — a unidade de controle (um por item da OC)
-- -----------------------------------------------------------------------------

create table public.locacoes_equipamentos (
  id                     uuid primary key default gen_random_uuid(),
  cliente_id             uuid not null references public.clientes(id),
  recebimento_id         uuid not null references public.locacoes_recebimentos(id) on delete restrict,
  ordem_compra_id        uuid not null references public.ordens_compra(id) on delete restrict,
  ordem_compra_item_id   uuid references public.ordens_compra_itens(id) on delete set null,

  fornecedor_id          uuid references public.fornecedores(id),
  obra_centro_custo      text,
  descricao              text not null,
  unidade                text,

  qtd_recebida           numeric(14,3) not null check (qtd_recebida > 0),
  qtd_ativa              numeric(14,3) not null check (qtd_ativa >= 0),
  valor_unit             numeric(14,2) not null,

  periodicidade          text not null check (periodicidade in ('mensal','quinzenal','semanal')),
  data_inicio            date not null,
  vencimento_atual       date not null,
  estado                 text not null default 'ativo' check (estado in ('ativo','encerrado')),

  -- Override da regra de faturamento a nível de equipamento — sobrepõe o do
  -- fornecedor (fornecedores.regra_faturamento) para um caso negociado à
  -- parte. NULL = herda do fornecedor. Fase 5 lê isto.
  regra_faturamento      text check (regra_faturamento in ('padrao','sempre_proporcional','sempre_mes_cheio')),

  criado_em              timestamptz not null default now(),
  atualizado_em          timestamptz not null default now()
);

comment on table public.locacoes_equipamentos is
  'A unidade que vence, renova e devolve — não a OC. Nasce no recebimento com
  qtd_ativa = qtd_recebida e estado ativo; qtd_ativa cai a cada devolução
  parcial (Fase 3) e chega a zero quando encerra. vencimento_atual é
  recalculado a cada renovação concluída (Fase 4) — a cobertura de verdade,
  período a período, mora em locacoes_periodos.';

create index locacoes_equipamentos_vencimento
  on public.locacoes_equipamentos (cliente_id, estado, vencimento_atual);

create trigger locacoes_equipamentos_carimbo before update on public.locacoes_equipamentos
  for each row execute function tocar_atualizado_em();

-- -----------------------------------------------------------------------------
-- locacoes_periodos — a cobertura de cada equipamento, período a período
-- -----------------------------------------------------------------------------

create table public.locacoes_periodos (
  id                uuid primary key default gen_random_uuid(),
  cliente_id        uuid not null references public.clientes(id),
  equipamento_id    uuid not null references public.locacoes_equipamentos(id) on delete cascade,
  ordem_compra_id   uuid not null references public.ordens_compra(id) on delete restrict,

  numero            int not null check (numero >= 1),
  tipo              text not null check (tipo in ('original','renovacao')),
  inicio            date not null,
  fim               date not null,
  qtd               numeric(14,3) not null,
  valor_unit        numeric(14,2) not null,

  criado_por        uuid references public.perfis(id),
  criado_em         timestamptz not null default now(),

  unique (equipamento_id, numero),
  check (fim > inicio)
);

comment on table public.locacoes_periodos is
  'Um período = uma OC cobrindo o equipamento por um intervalo. numero=1 é o
  recebimento original; cada renovação concluída (Fase 4) soma um período
  novo, com inicio = fim do anterior — a contiguidade é responsabilidade do
  motor (ver locacoes/renovacao.go quando existir), não uma constraint aqui,
  porque o Postgres não valida "o próximo começa onde o anterior parou" sem
  travar a linha inteira a cada insert.';

-- -----------------------------------------------------------------------------
-- locacoes_renovacoes — o pedido da obra e a conclusão do RC (Fase 4)
-- -----------------------------------------------------------------------------

create table public.locacoes_renovacoes (
  id                uuid primary key default gen_random_uuid(),
  cliente_id        uuid not null references public.clientes(id),
  equipamento_id    uuid not null references public.locacoes_equipamentos(id) on delete cascade,

  periodicidade     text not null check (periodicidade in ('mensal','quinzenal','semanal')),
  estado            text not null default 'pendente' check (estado in ('pendente','concluida','cancelada')),

  pedida_em         timestamptz not null default now(),
  pedida_por        uuid references public.perfis(id),

  ordem_compra_id   uuid references public.ordens_compra(id),
  periodo_id        uuid references public.locacoes_periodos(id),
  concluida_em      timestamptz,
  concluida_por     uuid references public.perfis(id),

  cancelada_em      timestamptz,
  motivo            text
);

comment on table public.locacoes_renovacoes is
  'O pedido nasce de quem tem LOCACOES_DECIDIR (a hierarquia da obra —
  almoxarife, encarregado, engenheiro) e fica pendente até o RC
  (LOCACOES_RENOVAR_OC) inserir a OC de renovação e concluir. Um pedido
  pendente NÃO cobre o equipamento — só a conclusão cria o próximo
  locacoes_periodos e empurra vencimento_atual. Existe pra sinalizar o RC
  (card "Renovações pendentes") e pra travar Devolver enquanto está em
  aberto (intertravamento, Fase 3/4).';

create unique index locacoes_renovacoes_pendente_unica
  on public.locacoes_renovacoes (equipamento_id)
  where estado = 'pendente';

-- -----------------------------------------------------------------------------
-- locacoes_devolucoes — a prova de que o relógio parou (Fase 3)
-- -----------------------------------------------------------------------------

create table public.locacoes_devolucoes (
  id                       uuid primary key default gen_random_uuid(),
  cliente_id               uuid not null references public.clientes(id),
  equipamento_id           uuid not null references public.locacoes_equipamentos(id) on delete cascade,

  data_devolucao           date not null,
  qtd                      numeric(14,3) not null check (qtd > 0),
  romaneio_sha256          text not null references public.arquivos(sha256) on delete restrict,
  registrado_por           uuid references public.perfis(id),

  -- A OC de desmobilização (frete de volta), quando existir — vínculo
  -- deixado pelo dono para decidir depois (pendência do plano); sem tela até
  -- ele pedir.
  ordem_compra_frete_id    uuid references public.ordens_compra(id),
  observacao               text,

  criado_em                timestamptz not null default now()
);

comment on table public.locacoes_devolucoes is
  'Sem romaneio de devolução, não existe devolução — é a defesa contra a
  locadora continuar cobrando um equipamento que já voltou. qtd parcial
  reduz locacoes_equipamentos.qtd_ativa; ao chegar a zero, o equipamento
  encerra (Fase 3).';

create table public.locacoes_devolucoes_paginas (
  id               uuid primary key default gen_random_uuid(),
  cliente_id       uuid not null references public.clientes(id),
  devolucao_id     uuid not null references public.locacoes_devolucoes(id) on delete cascade,
  pagina           int not null check (pagina >= 2),
  arquivo_sha256   text not null references public.arquivos(sha256) on delete restrict,
  criado_em        timestamptz not null default now(),
  unique (devolucao_id, pagina)
);

-- -----------------------------------------------------------------------------
-- locacoes_fotos — fotos do equipamento, no recebimento e na devolução
-- -----------------------------------------------------------------------------

create table public.locacoes_fotos (
  id                uuid primary key default gen_random_uuid(),
  cliente_id        uuid not null references public.clientes(id),
  equipamento_id    uuid not null references public.locacoes_equipamentos(id) on delete cascade,
  evento            text not null check (evento in ('recebimento','devolucao')),
  devolucao_id      uuid references public.locacoes_devolucoes(id),
  arquivo_sha256    text not null references public.arquivos(sha256) on delete restrict,
  criado_em         timestamptz not null default now()
);

comment on table public.locacoes_fotos is
  'Uma linha por foto — mesmo desenho de notas_fiscais_fotos_material (070).
  "recebimento" é a evidência do que chegou (obrigatória por item, 1+);
  "devolucao" é o estado do equipamento na saída (defesa contra cobrança de
  avaria), ligada à linha de locacoes_devolucoes correspondente.';

-- -----------------------------------------------------------------------------
-- RLS — leitura do meu cliente + rotina; a escrita de verdade passa pelo
-- motor com a chave de serviço (mesma disciplina do resto do sistema).
-- -----------------------------------------------------------------------------

alter table public.locacoes_recebimentos enable row level security;
alter table public.locacoes_recebimentos_paginas enable row level security;
alter table public.locacoes_equipamentos enable row level security;
alter table public.locacoes_periodos enable row level security;
alter table public.locacoes_renovacoes enable row level security;
alter table public.locacoes_devolucoes enable row level security;
alter table public.locacoes_devolucoes_paginas enable row level security;
alter table public.locacoes_fotos enable row level security;

create policy "recebimentos de locacao do meu cliente" on public.locacoes_recebimentos for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

create policy "paginas de recebimento do meu cliente" on public.locacoes_recebimentos_paginas for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

create policy "equipamentos locados do meu cliente" on public.locacoes_equipamentos for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

create policy "periodos de locacao do meu cliente" on public.locacoes_periodos for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

create policy "renovacoes de locacao do meu cliente" on public.locacoes_renovacoes for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

create policy "devolucoes de locacao do meu cliente" on public.locacoes_devolucoes for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

create policy "paginas de devolucao do meu cliente" on public.locacoes_devolucoes_paginas for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

create policy "fotos de locacao do meu cliente" on public.locacoes_fotos for select
  using (cliente_id = meu_cliente_id() and posso('LOCACOES_MONITORAR'));

-- -----------------------------------------------------------------------------
-- Locações vira um módulo de 1º nível — o vocabulário fechado de
-- `modulo_menu` (067, bypass de módulo do CEO) e
-- `categoria_modulos_liberados.modulo` (mesma migração) ainda não conhecem
-- este nome: as duas checks precisam aprender "locacoes" antes das rotinas
-- abaixo poderem usá-lo. `modulo_menu` aqui é o grupo do MENU (mesmo nível
-- de administrativo/manutencao/engenharia), diferente da coluna `modulo`
-- (mais fina, só agrupamento visual — ver o comentário da 067).
-- -----------------------------------------------------------------------------

alter table public.rotinas drop constraint rotinas_modulo_menu_valido;
alter table public.rotinas add constraint rotinas_modulo_menu_valido
  check (modulo_menu in ('administrativo', 'manutencao', 'engenharia', 'sesmt-dp', 'configuracoes', 'locacoes'));

alter table public.categoria_modulos_liberados drop constraint categoria_modulos_liberados_modulo_check;
alter table public.categoria_modulos_liberados add constraint categoria_modulos_liberados_modulo_check
  check (modulo in ('administrativo', 'manutencao', 'engenharia', 'sesmt-dp', 'configuracoes', 'locacoes'));

-- -----------------------------------------------------------------------------
-- Rotinas novas — as quatro do módulo, todas liberadas por padrão só ao CEO
-- (mesmo espírito da 059/064: quem usa no dia a dia ganha pela tela de
-- Acesso, a migração não sabe quem é o almoxarife ou o RC). `liberada_para_
-- bypass` fica no default `false` (067) — o Builder decide depois se libera
-- o módulo inteiro pro bypass do CEO.
-- -----------------------------------------------------------------------------

insert into public.rotinas (codigo, nome, modulo, modulo_menu, ordem) values
  ('LOCACOES_RECEBER',    'Locações — receber equipamento (almoxarife)', 'locacoes', 'locacoes', 1),
  ('LOCACOES_MONITORAR',  'Locações — ver o monitoramento',              'locacoes', 'locacoes', 2),
  ('LOCACOES_DECIDIR',    'Locações — devolver e pedir renovação',       'locacoes', 'locacoes', 3),
  ('LOCACOES_RENOVAR_OC', 'Locações — concluir renovação com OC (RC)',   'locacoes', 'locacoes', 4);

insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo = 'ceo' and r.codigo in
  ('LOCACOES_RECEBER','LOCACOES_MONITORAR','LOCACOES_DECIDIR','LOCACOES_RENOVAR_OC');

insert into schema_migrations (versao, arquivo)
values ('072', '072_locacoes_equipamentos.sql')
on conflict (versao) do nothing;

commit;

-- =============================================================================
-- COMO CONFERIR
--
--   -- as oito tabelas existem, RLS ligada:
--   select relname, relrowsecurity from pg_class
--    where relname like 'locacoes_%';
--   -- esperado: relrowsecurity = true nas oito
--
--   -- as quatro rotinas estão no catálogo, o CEO já tem as quatro:
--   select r.codigo, cp.pode from public.rotinas r
--     left join public.categoria_permissoes cp
--       on cp.rotina = r.codigo and cp.categoria_id = (select id from categorias where codigo='ceo')
--    where r.codigo like 'LOCACOES_%';
--   -- esperado: 4 linhas, pode = true nas 4
--
--   -- a view não mostra mais uma OC recebida como locação:
--   -- update ordens_compra set destino_recebimento = 'locacao' where id = '<uma-oc-de-teste>';
--   -- select * from nf_progresso_ordens where ordem_compra_id = '<uma-oc-de-teste>';
--   -- esperado: nenhuma linha
--
-- PARA DESFAZER
--   begin;
--   drop table public.locacoes_fotos;
--   drop table public.locacoes_devolucoes_paginas;
--   drop table public.locacoes_devolucoes;
--   drop index if exists locacoes_renovacoes_pendente_unica;
--   drop table public.locacoes_renovacoes;
--   drop table public.locacoes_periodos;
--   drop index if exists locacoes_equipamentos_vencimento;
--   drop table public.locacoes_equipamentos;
--   drop table public.locacoes_recebimentos_paginas;
--   drop table public.locacoes_recebimentos;
--   delete from public.categoria_permissoes where rotina like 'LOCACOES_%';
--   delete from public.rotinas where codigo like 'LOCACOES_%';
--   alter table public.categoria_modulos_liberados drop constraint categoria_modulos_liberados_modulo_check;
--   alter table public.categoria_modulos_liberados add constraint categoria_modulos_liberados_modulo_check
--     check (modulo in ('administrativo', 'manutencao', 'engenharia', 'sesmt-dp', 'configuracoes'));
--   alter table public.rotinas drop constraint rotinas_modulo_menu_valido;
--   alter table public.rotinas add constraint rotinas_modulo_menu_valido
--     check (modulo_menu in ('administrativo', 'manutencao', 'engenharia', 'sesmt-dp', 'configuracoes'));
--   create or replace view public.nf_progresso_ordens as
--   select
--     oc.id as ordem_compra_id, oc.cliente_id, oc.numero, oc.obra_centro_custo,
--     oc.fornecedor_id, oc.total, coalesce(nf.recebido, 0) as recebido,
--     oc.total - coalesce(nf.recebido, 0) as restante,
--     (coalesce(nf.recebido, 0) >= oc.total) as completa, oc.pco_enviado_em
--   from public.ordens_compra oc
--   left join (
--     select ordem_compra_id, sum(valor) as recebido from public.notas_fiscais
--     where not cancelada group by ordem_compra_id
--   ) nf on nf.ordem_compra_id = oc.id
--   where oc.status = 'lido' and oc.pco_enviado_em is not null;
--   alter table public.fornecedores drop column regra_faturamento;
--   alter table public.ordens_compra drop column destino_recebimento;
--   delete from public.schema_migrations where versao = '072';
--   commit;
-- =============================================================================
