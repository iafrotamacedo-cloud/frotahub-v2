-- =============================================================================
-- 010 — o módulo Orçamentos                                              rev 1
-- =============================================================================
--
-- Esta migration cria o modelo que quebra a god table do sistema antigo. Lá,
-- `notas_orcamento` era ao mesmo tempo fila de trabalho, registro fiscal, índice
-- de arquivo e estado financeiro, com os itens dentro de um jsonb. Aqui são
-- sete tabelas com uma responsabilidade cada.
--
-- O QUE ENTRA
--
--   parametros            teto e margem com VIGÊNCIA — regra de negócio no banco
--   documentos            a nota/DAV inserida pelo site
--   documento_itens       os itens, linha a linha (saem do jsonb)
--   documento_tickets     o rateio: quais tickets uma nota atende
--   orcamentos            ticket + parte, valor, status, lançamento
--   orcamento_itens       o que foi cobrado, com preço da nota e preço cobrado
--   orcamento_documentos  N↔N entre orçamento e nota
--   jobs                  a fila de leitura e de lançamento, no banco
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. Parâmetros de negócio — com vigência
--
-- POR QUE NÃO É CONSTANTE NO CÓDIGO
--
--   O teto de R$ 600 e a margem de 20% são decisões do cliente, não do
--   programa. No sistema antigo o teto tinha TRÊS implementações e a margem
--   ONZE, com dois arredondamentos que divergiam em centavos. Trazer isso para
--   uma tabela resolve dois problemas de uma vez: existe um valor só, e mudar
--   não é deploy.
--
--   A vigência é o que permite explicar um orçamento antigo. No dia em que o
--   teto virar R$ 800, os 509 orçamentos gerados sob a regra dos 600 continuam
--   corretos — porque a regra tem data.
-- -----------------------------------------------------------------------------
create table if not exists parametros (
  id             uuid primary key default gen_random_uuid(),
  cliente_id     uuid not null references clientes(id) on delete restrict,
  chave          text not null,
  valor          numeric(14,4) not null,
  vigencia_inicio date not null default current_date,
  vigencia_fim    date,
  observacao     text,
  criado_em      timestamptz not null default now(),
  criado_por     uuid references perfis(id) on delete set null,

  check (vigencia_fim is null or vigencia_fim >= vigencia_inicio)
);

comment on table parametros is
  'Regra de negócio com data. Mudar o teto é um insert, nunca um deploy.';

-- Só UM valor vigente por chave. Sem isto, dois "teto" abertos ao mesmo tempo
-- fariam o mesmo orçamento ter dois resultados dependendo da ordem da consulta.
create unique index if not exists parametro_vigente_unico
  on parametros (cliente_id, chave)
  where vigencia_fim is null;

create index if not exists parametros_por_chave
  on parametros (cliente_id, chave, vigencia_inicio desc);


-- -----------------------------------------------------------------------------
-- 2. Documentos — a nota fiscal ou o DAV que entrou pelo site
--
-- O ARQUIVO NÃO MORA AQUI
--   Mora em `arquivos`, endereçado pelo sha256 do próprio conteúdo (007). Aqui
--   fica a APARIÇÃO: quem inseriu, quando, em que fila, com que nome.
--
-- A ÁREA "HIDE" É UMA COLUNA
--   `oculto_em` preenchido = o arquivo saiu da lista. O byte continua no R2,
--   inteiro. É isso que faz o "desfazer" ser honesto: desfazer é um update, não
--   uma ressurreição. E é isso que faz o botão "orçamentos apagados" ser
--   trivial em vez de impossível.
-- -----------------------------------------------------------------------------
create table if not exists documentos (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,

  -- Em que fila a nota entrou. São dois caminhos diferentes de trabalho:
  --   orcamento  a nota traz o próprio ticket nas observações
  --   rateio     uma nota atende vários tickets, ditados pelo usuário
  fila text not null default 'orcamento' check (fila in ('orcamento', 'rateio')),

  tipo text check (tipo in ('nf', 'dav')),

  numero        text,
  serie         text,
  chave_acesso  text check (chave_acesso is null or char_length(chave_acesso) = 44),
  dav_numero    text,

  emitente_cnpj  text,
  emitente_nome  text,
  destinatario_cnpj text,

  emissao       date,
  valor_total   numeric(14,2),
  valor_frete   numeric(14,2) not null default 0,
  observacao    text,

  -- Como o conteúdo foi lido, e com quanta confiança. Nunca escondemos isso:
  -- valor lido por IA que ninguém conferiu não pode parecer valor digitado.
  leitura_camada text check (leitura_camada in ('xml', 'texto', 'ocr', 'ia', 'manual')),
  leitura_confianca numeric(4,3),
  leitura_bruta  jsonb,
  leitura_erro   text,

  status text not null default 'inserido'
    check (status in ('inserido', 'lendo', 'lido', 'falhou', 'usado')),

  nome_arquivo   text not null,
  arquivo_sha256 text references arquivos(sha256) on delete restrict,

  inserido_em  timestamptz not null default now(),
  inserido_por uuid references perfis(id) on delete set null,

  oculto_em  timestamptz,
  oculto_por uuid references perfis(id) on delete set null
);

comment on column documentos.oculto_em is
  'A área Hide. Preenchido = fora da lista. O arquivo continua inteiro no R2.';

create index if not exists documentos_da_fila
  on documentos (cliente_id, fila, inserido_em desc)
  where oculto_em is null;

create index if not exists documentos_ocultos
  on documentos (cliente_id, oculto_em desc)
  where oculto_em is not null;

-- A MESMA NOTA NÃO ENTRA DUAS VEZES
--   A chave de acesso da NFe tem 44 dígitos e é única no Brasil inteiro. Onde
--   ela existe, a duplicidade fica IMPOSSÍVEL — não "verificada".
create unique index if not exists documento_chave_unica
  on documentos (cliente_id, chave_acesso)
  where chave_acesso is not null and oculto_em is null;


-- -----------------------------------------------------------------------------
-- 3. Itens do documento — o que estava no jsonb
--
-- Com isto, auditoria vira `group by`. A view que explodia JSON some.
-- -----------------------------------------------------------------------------
create table if not exists documento_itens (
  id           uuid primary key default gen_random_uuid(),
  documento_id uuid not null references documentos(id) on delete cascade,
  ordem        integer not null,

  codigo         text,
  descricao      text not null,
  unidade        text,
  quantidade     numeric(14,4) not null default 1,
  valor_unitario numeric(14,4) not null default 0,
  valor_total    numeric(14,2) not null default 0,

  unique (documento_id, ordem)
);


-- -----------------------------------------------------------------------------
-- 4. Rateio — quais tickets esta nota atende
--
-- Uma linha por ticket. No sistema antigo isto era uma planilha que o usuário
-- preenchia à mão e o robô lia; aqui é tabela, e o "+" da tela escreve nela.
-- -----------------------------------------------------------------------------
create table if not exists documento_tickets (
  id           uuid primary key default gen_random_uuid(),
  documento_id uuid not null references documentos(id) on delete cascade,
  ticket       integer not null,
  chamado_id   uuid references chamados(id) on delete set null,
  incluido_em  timestamptz not null default now(),
  incluido_por uuid references perfis(id) on delete set null,

  unique (documento_id, ticket)
);

comment on column documento_tickets.chamado_id is
  'Nulo = o ticket foi digitado mas ainda não casou com a nossa base. É daqui que sai a fila "sem associação".';


-- -----------------------------------------------------------------------------
-- 5. Orçamentos
--
-- TICKET + PARTE
--   Um ticket pode ter vários orçamentos legítimos (notas diferentes). O que
--   não pode é a MESMA nota virar orçamento duas vezes. Então a identidade é
--   `ticket + parte`, e a trava de duplicidade é o par (ticket, documento).
--
-- O TETO GRAVADO
--   `teto_aplicado` guarda o valor que valia NO DIA. Sem isso, mudar o
--   parâmetro reescreveria a história: um orçamento de R$ 620 aprovado sob o
--   teto de R$ 800 pareceria uma violação para sempre.
-- -----------------------------------------------------------------------------
create table if not exists orcamentos (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,

  ticket     integer not null,
  parte      smallint not null default 1 check (parte >= 1),
  chamado_id uuid references chamados(id) on delete set null,
  unidade_id uuid references unidades(id) on delete set null,
  conta      text check (conta in ('instalacoes', 'civil')),

  -- Dinheiro é decimal, nunca ponto flutuante (P-12).
  valor_nota     numeric(14,2) not null default 0,
  valor          numeric(14,2) not null default 0,
  margem_aplicada numeric(6,4),
  teto_aplicado   numeric(14,2),

  -- Quando o teto obrigou a reduzir, os dois números ficam visíveis. Redução
  -- silenciosa é a diferença entre um sistema explicável e um sistema mágico.
  valor_antes_do_teto numeric(14,2),
  reduzido_pelo_teto  boolean not null default false,

  rateio boolean not null default false,

  status text not null default 'gerado' check (status in (
    'gerado',                -- existe, pronto para lançar
    'aguardando_aprovacao',  -- passou do teto: o cliente precisa aprovar
    'lancado',               -- está no Trílogo
    'removido'               -- soft-delete; nunca some de verdade
  )),

  aprovado_por   uuid references perfis(id) on delete set null,
  aprovado_em    timestamptz,
  aprovacao_nota text,

  -- O que o Trílogo devolveu no lançamento. Guardar o id deles é o que permite
  -- reconhecer o nosso próprio anexo quando o robô ler o chamado de volta.
  lancado_em          timestamptz,
  lancado_por         uuid references perfis(id) on delete set null,
  trilogo_custo_id    integer,
  trilogo_permalink   text,

  faturado    boolean not null default false,
  faturado_em timestamptz,
  pago        boolean not null default false,
  pago_em     timestamptz,

  arquivo_pdf_sha256 text references arquivos(sha256) on delete restrict,

  criado_em  timestamptz not null default now(),
  criado_por uuid references perfis(id) on delete set null,

  removido_em     timestamptz,
  removido_por    uuid references perfis(id) on delete set null,
  removido_motivo text,

  unique (cliente_id, ticket, parte)
);

comment on column orcamentos.faturado is
  'Escrito pelo módulo financeiro, que ainda não existe. No sistema antigo esta coluna nunca era escrita e a planilha reportava falso para sempre — aqui ela nasce sabendo que é assim.';

create index if not exists orcamentos_por_status
  on orcamentos (cliente_id, status, criado_em desc);

create index if not exists orcamentos_por_ticket
  on orcamentos (cliente_id, ticket);

create index if not exists orcamentos_a_lancar
  on orcamentos (cliente_id, criado_em)
  where status = 'gerado';

create index if not exists orcamentos_apagados
  on orcamentos (cliente_id, removido_em desc)
  where status = 'removido';


-- -----------------------------------------------------------------------------
-- 6. Itens do orçamento
--
-- Dois preços lado a lado, sempre: o que a nota cobrou e o que nós cobramos.
-- A margem fica DEMONSTRÁVEL sem estar escrita em lugar nenhum do documento.
-- -----------------------------------------------------------------------------
create table if not exists orcamento_itens (
  id           uuid primary key default gen_random_uuid(),
  orcamento_id uuid not null references orcamentos(id) on delete cascade,
  ordem        integer not null,

  descricao      text not null,
  unidade        text,
  quantidade     numeric(14,4) not null default 1,
  valor_unitario_nota    numeric(14,4) not null default 0,
  valor_unitario_cobrado numeric(14,4) not null default 0,
  valor_total            numeric(14,2) not null default 0,

  documento_item_id uuid references documento_itens(id) on delete set null,

  unique (orcamento_id, ordem)
);


-- -----------------------------------------------------------------------------
-- 7. Quais notas compõem este orçamento
--
-- N↔N. É o que faz o rateio deixar de ser gambiarra: uma nota entra em vários
-- orçamentos, cada um com a sua parcela.
-- -----------------------------------------------------------------------------
create table if not exists orcamento_documentos (
  orcamento_id uuid not null references orcamentos(id) on delete cascade,
  documento_id uuid not null references documentos(id) on delete restrict,
  parcela      numeric(14,2),
  primary key (orcamento_id, documento_id)
);

-- A MESMA NOTA NÃO VIRA ORÇAMENTO DUAS VEZES NO MESMO TICKET.
-- Índice, não verificação: a corrida entre duas requisições simultâneas deixa
-- de existir porque o banco recusa a segunda.
create unique index if not exists orcamento_nota_por_ticket
  on orcamento_documentos (documento_id, orcamento_id);


-- -----------------------------------------------------------------------------
-- 8. Fila de trabalho
--
-- O andamento mora no banco (P-03). O motor do plano gratuito adormece; se o
-- progresso estivesse na memória, uma leitura interrompida sumiria sem deixar
-- rastro e ninguém saberia que a nota nunca foi lida.
-- -----------------------------------------------------------------------------
create table if not exists jobs (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,

  tipo text not null check (tipo in ('ler_documento', 'gerar_orcamento', 'lancar_orcamento')),
  alvo_id uuid,

  status text not null default 'na_fila'
    check (status in ('na_fila', 'rodando', 'concluido', 'falhou')),

  tentativas smallint not null default 0,
  erro       text,
  detalhe    jsonb,

  criado_em    timestamptz not null default now(),
  comecou_em   timestamptz,
  terminou_em  timestamptz,

  -- Quem tomou o trabalho. Duas máquinas não pegam a mesma linha porque a
  -- tomada é um update condicional em status.
  tomado_por text
);

create index if not exists jobs_na_fila
  on jobs (cliente_id, tipo, criado_em)
  where status = 'na_fila';

create index if not exists jobs_presos
  on jobs (comecou_em)
  where status = 'rodando';


-- -----------------------------------------------------------------------------
-- 9. As visões das telas
--
-- A forma da lista mora no banco (CORE-06). Se amanhã a planilha de controle
-- ganhar uma coluna, ela nasce aqui e o motor não muda.
--
-- security_invoker = true — sem isto a visão passaria POR CIMA das políticas de
-- linha e viraria um segundo caminho para o dado, sem o filtro do titular.
-- -----------------------------------------------------------------------------

create or replace view documentos_lista
with (security_invoker = true) as
select d.id, d.cliente_id, d.fila, d.tipo, d.numero, d.dav_numero, d.chave_acesso,
       d.emitente_nome, d.emissao, d.valor_total, d.status,
       d.leitura_camada, d.leitura_confianca,
       d.nome_arquivo, d.arquivo_sha256, d.inserido_em, d.oculto_em,
       coalesce(t.quantos, 0)  as tickets,
       coalesce(t.lista, '{}') as ticket_numeros,
       coalesce(i.quantos, 0)  as itens
from documentos d
left join lateral (
  select count(*) as quantos, array_agg(dt.ticket order by dt.ticket) as lista
  from documento_tickets dt where dt.documento_id = d.id
) t on true
left join lateral (
  select count(*) as quantos from documento_itens di where di.documento_id = d.id
) i on true;


create or replace view orcamentos_lista
with (security_invoker = true) as
select o.id, o.cliente_id, o.ticket, o.parte, o.conta, o.status,
       o.valor, o.valor_nota, o.reduzido_pelo_teto, o.valor_antes_do_teto,
       o.rateio, o.criado_em, o.lancado_em, o.faturado, o.pago,
       o.trilogo_custo_id, o.arquivo_pdf_sha256,
       u.nome as loja,
       c.descricao as chamado_descricao,
       -- NOTA E DAV EM COLUNAS SEPARADAS
       --   A planilha de controle tem as duas colunas, e juntá-las aqui faria a
       --   tela ter que adivinhar qual é qual pelo formato do número — que é
       --   exatamente o tipo de adivinhação que produz coluna errada.
       (select string_agg(d.numero, ', ' order by d.numero)
          from orcamento_documentos od join documentos d on d.id = od.documento_id
         where od.orcamento_id = o.id and d.numero is not null) as notas,
       (select string_agg(d.dav_numero, ', ' order by d.dav_numero)
          from orcamento_documentos od join documentos d on d.id = od.documento_id
         where od.orcamento_id = o.id and d.dav_numero is not null) as davs
from orcamentos o
left join unidades u on u.id = o.unidade_id
left join chamados c on c.id = o.chamado_id;


-- O painel dos cinco botões. Um número por etapa, numa consulta só — a tela não
-- faz cinco chamadas para desenhar cinco contadores.
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
     where o.cliente_id = cl.id and o.status <> 'removido') as valor_total
from clientes cl;


-- -----------------------------------------------------------------------------
-- 10. Segurança de linha
--
-- Mesmo desenho da 007: o cliente titular filtra, e a rotina decide se abre.
-- -----------------------------------------------------------------------------
alter table parametros           enable row level security;
alter table documentos           enable row level security;
alter table documento_itens      enable row level security;
alter table documento_tickets    enable row level security;
alter table orcamentos           enable row level security;
alter table orcamento_itens      enable row level security;
alter table orcamento_documentos enable row level security;
alter table jobs                 enable row level security;

drop policy if exists "parametros do meu cliente" on parametros;
create policy "parametros do meu cliente"
  on parametros for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "documentos do meu cliente" on documentos;
create policy "documentos do meu cliente"
  on documentos for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "orcamentos do meu cliente" on orcamentos;
create policy "orcamentos do meu cliente"
  on orcamentos for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "jobs do meu cliente" on jobs;
create policy "jobs do meu cliente"
  on jobs for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "itens dos documentos que eu vejo" on documento_itens;
create policy "itens dos documentos que eu vejo"
  on documento_itens for select to authenticated
  using (exists (select 1 from documentos d
                  where d.id = documento_id and d.cliente_id = meu_cliente_id())
         and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "tickets dos documentos que eu vejo" on documento_tickets;
create policy "tickets dos documentos que eu vejo"
  on documento_tickets for select to authenticated
  using (exists (select 1 from documentos d
                  where d.id = documento_id and d.cliente_id = meu_cliente_id())
         and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "itens dos orcamentos que eu vejo" on orcamento_itens;
create policy "itens dos orcamentos que eu vejo"
  on orcamento_itens for select to authenticated
  using (exists (select 1 from orcamentos o
                  where o.id = orcamento_id and o.cliente_id = meu_cliente_id())
         and posso('CONTRATO_ORCAMENTOS'));

drop policy if exists "notas dos orcamentos que eu vejo" on orcamento_documentos;
create policy "notas dos orcamentos que eu vejo"
  on orcamento_documentos for select to authenticated
  using (exists (select 1 from orcamentos o
                  where o.id = orcamento_id and o.cliente_id = meu_cliente_id())
         and posso('CONTRATO_ORCAMENTOS'));


-- -----------------------------------------------------------------------------
-- 11. As rotinas no catálogo
--
-- Uma por botão. Assim a matriz de permissões consegue liberar "ver a planilha"
-- sem liberar "lançar no Trílogo" — que é uma distinção real na operação.
-- -----------------------------------------------------------------------------
insert into rotinas (codigo, nome, modulo, ordem) values
  ('CONTRATO_ORCAMENTOS',            'Orçamentos',            'manutencao', 320),
  ('CONTRATO_ORCAMENTOS_NOTAS',      'Notas e DAVs',          'manutencao', 321),
  ('CONTRATO_ORCAMENTOS_RATEIO',     'Notas para rateio',     'manutencao', 322),
  ('CONTRATO_ORCAMENTOS_LANCAR',     'Lançar orçamentos',     'manutencao', 323),
  ('CONTRATO_ORCAMENTOS_CORRECOES',  'Correções',             'manutencao', 324),
  ('CONTRATO_ORCAMENTOS_PLANILHAS',  'Planilhas de controle', 'manutencao', 325)
on conflict (codigo) do nothing;


-- -----------------------------------------------------------------------------
-- 12. Os parâmetros de partida
--
-- Os valores que já valiam no sistema antigo, agora com data. A vigência começa
-- no dia em que o módulo entrou no ar, e não antes: dizer que a regra vale
-- desde sempre seria inventar histórico.
-- -----------------------------------------------------------------------------
insert into parametros (cliente_id, chave, valor, observacao)
select c.id, p.chave, p.valor, p.observacao
from clientes c
cross join (values
  ('teto_lancamento', 600.0000,
   'Acima disto o orçamento precisa de aprovação do cliente.'),
  ('margem',            0.2000,
   'Acrescida ao valor unitário. Nunca aparece no documento.'),
  ('teto_folga_pct',    0.0500,
   'Passando do teto em ate isto, o valor é reduzido para o teto exato em vez de parar para aprovação.')
) as p(chave, valor, observacao)
where c.slug = 'frota-macedo'
  and not exists (
    select 1 from parametros x
     where x.cliente_id = c.id and x.chave = p.chave and x.vigencia_fim is null);


-- =============================================================================
-- PARA DESFAZER (só em desenvolvimento — em produção isto apaga trabalho):
--
--   drop view if exists orcamentos_painel, orcamentos_lista, documentos_lista;
--   drop table if exists jobs, orcamento_documentos, orcamento_itens, orcamentos,
--                        documento_tickets, documento_itens, documentos, parametros;
--   delete from rotinas where codigo like 'CONTRATO_ORCAMENTOS%';
-- =============================================================================
