-- =============================================================================
-- 059 — Compras: a Ordem de Compra entra no sistema                      rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (10/09/2026)
--
--   "vamos começar a codar: criei os menus no sistema já (adm). preciso codar
--    primeiro a inserção [da OC — é a primeira entrada do sistema]. crie uma
--    tela de inserção parecida com as que já usamos no sistema: um botão para
--    abrir o doc do PC e colocar na tela, outro botão para inserir, uma lista
--    embaixo de todos os documentos que estão na fila. OC é sempre PDF
--    digital. leitura monta BD."
--
--   É o primeiro pedaço de código de verdade da Fase 5 (Administrativo). Tudo
--   antes disso foi levantamento — ver `claude/fase5-administrativo-
--   planejamento.md` no Projeto para o fluxo completo (Obra Prima → aprovação
--   → inclusão → PCO/medição → confirmação de obra → nota física), as
--   perguntas A1–A7/B1–B5 já respondidas, e o porquê de cada campo abaixo.
--
-- POR QUE ISTO NÃO É `documentos`
--
--   `documentos` (migração 010) já é o "PDF que vira linha no banco" para
--   nota/DAV — mas as 43 colunas dela são desenhadas em torno de ticket,
--   chamado e orçamento, que a OC não tem. Forçar a OC ali dentro seria virar
--   metade das colunas sempre nulas para um objeto de negócio diferente.
--   `ordens_compra` é nova, mas COPIA o padrão de `documentos` que já provou
--   funcionar: mesmo armazém de arquivo por sha256, mesma máquina de status
--   (inserido → lendo → lido → falhou), mesma tela de inserção (arrastar ou
--   escolher, botão Inserir, fila embaixo — ver `Arquivos.tsx`/`Insercao` em
--   Orçamentos, que esta tela nova replica).
--
-- A LEITURA VEM DEPOIS, NESTA MESMA MIGRAÇÃO NÃO
--
--   Por isso quase toda coluna de conteúdo (numero, data, comprador,
--   fornecedor, itens, totais) é OPCIONAL: a linha nasce só com o arquivo
--   (status `inserido`) e ganha o resto quando o motor lê o PDF (status vira
--   `lendo`, depois `lido` — ou `falhou`, com o motivo em `erro_leitura`). A
--   leitura de verdade (extrair os campos do PDF do Obra Prima) é o próximo
--   passo, não esta migração.
--
-- `cliente_id` AQUI É O TENANT, NÃO O CLIENTE FINAL DA OC
--
--   Confirmado lendo a 056: `clientes` é sempre o TENANT (`meu_cliente_id()`,
--   quem loga no FrotaHub — hoje só a Frota Macedo). O cliente final da OC
--   (Mercadinhos São Luiz / Distribuidora de Alimentos Fartura — mesmo grupo,
--   raiz de CNPJ `03720882`, ver A1 no planejamento) não ganha linha em
--   `clientes` nem em `unidades` nesta migração: ele é só o dado que valida a
--   OC (`comprador_nome`/`comprador_cnpj`), não um tenant novo. Se um dia
--   precisar de cadastro de verdade para o cliente final, é outra migração —
--   por ora a regra é fixa (raiz do CNPJ), como o dono decidiu em B1/A6.
--
-- A VALIDAÇÃO DA INCLUSÃO (B1, fechada em 30/08/2026)
--
--   1. `comprador_cnpj` tem que começar com `03720882` — é o CNPJ de quem
--      fatura direto, não o funcionário que cotou (esse é `comprador_interno`,
--      texto livre, só para registro — ex.: "Nadyson Ferreira").
--   2. Fornecedor precisa ter nome E CNPJ — por isso `fornecedores` tem as
--      duas colunas `not null`, e a OC referencia um fornecedor já cadastrado
--      (a tela de inserção cria o fornecedor na hora, se for novo, com os
--      dois dados obrigatórios — não é uma lista fechada de aprovados).
--   3. Nenhuma das duas regras impede a inclusão do ARQUIVO — só impede a OC
--      de sair de `lendo` para `lido`. Um PDF de OC inválida ainda entra na
--      fila e aparece como `falhou`, com o motivo — é a mesma filosofia da
--      `pco-organizer` (bloqueada não é descartada, é marcada).
--
-- POR QUE `fornecedores` NASCE AQUI, E NÃO ANTES
--
--   Não existia em lugar nenhum do modelo (só havia `emitente`, o NOSSO CNPJ,
--   e `unidades`, o cliente) — é o lado de quem a Frota Macedo compra. Nasce
--   junto com a primeira coisa que precisa dele.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- fornecedores — quem vende para a Frota Macedo
-- -----------------------------------------------------------------------------

create table public.fornecedores (
  id             uuid primary key default gen_random_uuid(),
  cliente_id     uuid not null references public.clientes(id),
  razao_social   text not null,
  cnpj           text not null,
  telefone       text,
  vendedor       text,
  email          text,
  endereco       text,
  criado_em      timestamptz not null default now(),
  atualizado_em  timestamptz not null default now(),
  unique (cliente_id, cnpj)
);

comment on table public.fornecedores is
  'Quem a Frota Macedo compra material. Razão social e CNPJ obrigatórios — é '
  'a validação da OC na inclusão (planejamento Fase 5, pergunta B1). Uma '
  'linha por CNPJ, por tenant — reinserir o mesmo CNPJ atualiza, não duplica '
  '(a tela resolve isso, não esta migração).';

create trigger fornecedores_carimbo before update on public.fornecedores
  for each row execute function tocar_atualizado_em();

-- -----------------------------------------------------------------------------
-- ordens_compra — o PDF da OC, e o que a leitura extrai dele
-- -----------------------------------------------------------------------------

create table public.ordens_compra (
  id                   uuid primary key default gen_random_uuid(),
  cliente_id           uuid not null references public.clientes(id),

  -- O ARQUIVO É O ÚNICO DADO OBRIGATÓRIO NA INSERÇÃO
  --   Tudo abaixo daqui é preenchido pela LEITURA, não pela tela de inserção.
  arquivo_sha256       text not null references public.arquivos(sha256) on delete restrict,
  status               text not null default 'inserido'
                         check (status in ('inserido','lendo','lido','falhou')),
  erro_leitura         text,

  -- Extraídos do PDF pela leitura (nulos até `status = 'lido'`).
  numero               text,
  data                 date,
  obra_centro_custo    text,
  cond_pgto            text,
  forma_pgto           text,
  previsao_entrega     date,
  comprador_interno    text,  -- quem cotou (funcionário) — texto livre, só registro
  comprador_nome       text,  -- quem fatura direto (cliente final)
  -- Raiz fixa do CNPJ que fatura direto (A1/A6/B1) — NULL passa (ainda não lido).
  comprador_cnpj       text check (comprador_cnpj ~ '^03720882'),
  fornecedor_id        uuid references public.fornecedores(id) on delete restrict,
  subtotal             numeric(14,2),
  desconto             numeric(14,2),
  frete                numeric(14,2),
  total                numeric(14,2),

  -- As duas rotinas diárias que consomem a OC depois de lida (planejamento
  -- Fase 5: solicitação de PCO por Brevo, e a medição por centro de custo).
  -- Nascem aqui vazias — quem preenche é a rotina agendada, não esta tela.
  pco_enviado_em       timestamptz,
  medicao_enviada_em   timestamptz,

  criado_por           uuid references public.perfis(id),
  criado_em            timestamptz not null default now(),
  atualizado_em        timestamptz not null default now()
);

comment on table public.ordens_compra is
  'A Ordem de Compra do Obra Prima, sempre inserida como PDF digital pelo '
  'comprador (nunca API — planejamento Fase 5, A2). Nasce só com o arquivo '
  '(`status = ''inserido''`); a leitura preenche o resto e decide `lido` ou '
  '`falhou`. `cliente_id` é o TENANT (Frota Macedo), não o cliente final da '
  'OC — esse é `comprador_nome`/`comprador_cnpj`, texto validado por CNPJ, '
  'não por cadastro (ver cabeçalho desta migração).';

-- DEDUP PELO NÚMERO, MAS SÓ DEPOIS DE LIDO
--   Não dá para `unique` direto na coluna: `numero` é nulo até a leitura, e
--   Postgres trata NULLs como distintos entre si mesmo em `unique` — então
--   isso já funcionaria sem o `where` — mas o índice parcial deixa a intenção
--   explícita: a garantia é "duas OCs lidas não têm o mesmo número", não
--   "duas linhas não têm o mesmo nulo".
create unique index ordens_compra_numero_unico
  on public.ordens_compra (cliente_id, numero)
  where numero is not null;

create trigger ordens_compra_carimbo before update on public.ordens_compra
  for each row execute function tocar_atualizado_em();

-- -----------------------------------------------------------------------------
-- ordens_compra_itens — os itens da OC, um por linha (mesmo padrão de
-- documento_itens — P-06)
-- -----------------------------------------------------------------------------

create table public.ordens_compra_itens (
  id               uuid primary key default gen_random_uuid(),
  ordem_compra_id  uuid not null references public.ordens_compra(id) on delete cascade,
  descricao        text not null,
  qtd              numeric(14,3) not null,
  unidade          text,
  valor_unit       numeric(14,2) not null,
  desconto         numeric(14,2) not null default 0,
  total            numeric(14,2) not null
);

comment on table public.ordens_compra_itens is
  'Os itens da OC, um por linha — a leitura apaga e reinsere tudo a cada '
  'releitura (mesmo padrão de documento_itens), nunca soma em cima.';

-- -----------------------------------------------------------------------------
-- RLS: mesmo padrão do resto do sistema — leitura do meu cliente (tenant) +
-- rotina liberada. A gravação de verdade é decidida no motor (a chave de
-- serviço ignora RLS); estas políticas cobrem quem um dia ler direto do
-- Supabase, e documentam a intenção de quem lê o esquema.
-- -----------------------------------------------------------------------------

alter table public.fornecedores enable row level security;
alter table public.ordens_compra enable row level security;
alter table public.ordens_compra_itens enable row level security;

create policy "fornecedores do meu cliente" on public.fornecedores for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_ORDENS_GERENCIAR'));

create policy "ordens de compra do meu cliente" on public.ordens_compra for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_ORDENS_GERENCIAR'));

create policy "itens das ordens que eu vejo" on public.ordens_compra_itens for select
  using (
    exists (
      select 1 from public.ordens_compra oc
       where oc.id = ordens_compra_itens.ordem_compra_id
         and oc.cliente_id = meu_cliente_id()
    )
    and posso('COMPRAS_ORDENS_GERENCIAR')
  );

-- Catálogo de rotinas do módulo (mesmo padrão <MODULO>_<AÇÃO> das migrações
-- 044/056 — Administrativo não é "CONTRATO_*": não é do Contrato São Luiz,
-- é departamento próprio, então o prefixo é o nome da área).
insert into public.rotinas (codigo, nome, modulo, ordem) values
  ('COMPRAS_ORDENS_GERENCIAR', 'Compras — inserir e ver ordens de compra', 'compras', 1);

-- Permissão de saída: só CEO, mesmo padrão da 044/056 (SESMT/DP e
-- Planejamento também nasceram assim — ajustar depois pela tela de Acesso,
-- sem migração nova).
--
-- ⚠ QUEM VAI USAR ESTA TELA NO DIA A DIA (o comprador, hoje Nadyson) PRECISA
-- GANHAR ESTA ROTINA NA CATEGORIA DELE PELA TELA DE ACESSO — esta migração
-- não sabe qual categoria ele é, então não libera por ele.
insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo = 'ceo' and r.codigo = 'COMPRAS_ORDENS_GERENCIAR';

insert into schema_migrations (versao, arquivo)
values ('059', '059_a_ordem_de_compra_entra_no_sistema.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- as três tabelas existem, RLS ligada:
--   select relname, relrowsecurity from pg_class
--    where relname in ('fornecedores','ordens_compra','ordens_compra_itens');
--   -- esperado: relrowsecurity = true nas três
--
--   -- a rotina nova está no catálogo e liberada para o CEO:
--   select r.codigo, cp.pode, c.codigo as categoria
--     from public.rotinas r
--     join public.categoria_permissoes cp on cp.rotina = r.codigo
--     join public.categorias c on c.id = cp.categoria_id
--    where r.codigo = 'COMPRAS_ORDENS_GERENCIAR';
--   -- esperado: 1 linha, categoria 'ceo', pode = true
--
--   -- o índice parcial não briga com múltiplas OCs ainda não lidas:
--   insert into ordens_compra (cliente_id, arquivo_sha256)
--     select id, (select sha256 from arquivos limit 1) from clientes limit 1;
--   -- rodar duas vezes não deve dar erro de unicidade (numero é nulo nas duas)
--
--   -- a raiz do CNPJ é validada só quando preenchida:
--   -- update ... set comprador_cnpj = '00000000000000' -- deve FALHAR (check)
--   -- update ... set comprador_cnpj = '03720882000199' -- deve PASSAR
--
-- PARA DESFAZER
--   drop table public.ordens_compra_itens;
--   drop index if exists ordens_compra_numero_unico;
--   drop table public.ordens_compra;
--   drop table public.fornecedores;
--   delete from public.categoria_permissoes where rotina = 'COMPRAS_ORDENS_GERENCIAR';
--   delete from public.rotinas where codigo = 'COMPRAS_ORDENS_GERENCIAR';
--   delete from public.schema_migrations where versao = '059';
-- =============================================================================
