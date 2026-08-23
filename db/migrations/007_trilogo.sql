-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 007: os dados do Trílogo
--
-- O QUE FAZ
--   Cria as tabelas onde o robô do Trílogo grava: unidades, chamados, timeline,
--   custos, anexos e o registro das rodadas.
--
-- O QUE ELA NÃO FAZ
--   Não cadastra rotina no catálogo. A tela de "Dados do Trílogo" ainda não existe;
--   quando existir, a migration dela registra a rotina. Assim o catálogo nunca
--   promete uma tela que não abre.
--
-- AS TRÊS DECISÕES QUE MOLDARAM ESTE ARQUIVO
--
--   1. A CHAVE DO CHAMADO É O NÚMERO, NÃO "NÚMERO + CONTA".
--      A timeline do Trílogo tem o evento "Nova empresa prestadora": um chamado
--      TROCA de conta ao longo da vida. Com "número + conta" na chave, essa troca
--      criaria dois registros do mesmo chamado, os dois parecendo válidos. A conta
--      é atributo, não identidade. *O robô antigo erra nisto hoje.*
--
--   2. GUARDA-SE O CÓDIGO CRU DO TRÍLOGO JUNTO COM O RÓTULO.
--      Conferindo código contra tela, descobri que o robô antigo tem a prioridade
--      TROCADA (grava 1=Baixa quando 1 é Média). Guardando o número junto, um erro
--      de tradução se corrige com um UPDATE — sem reler 1.430 chamados no Trílogo.
--
--   3. O ARQUIVO E A APARIÇÃO DELE SÃO COISAS DIFERENTES.
--      `arquivos` é o conteúdo, identificado pelo sha256. `chamado_anexos` é cada
--      vez que aquele conteúdo aparece num chamado. É isto que impede o orçamento
--      de duplicar quando nós o subimos ao Trílogo e o lemos de volta: vira uma
--      aparição nova apontando para o arquivo que já existe.
--
-- DEPENDE DE: 003 (clientes, categorias, categoria_permissoes), 004 (perfis)
-- -----------------------------------------------------------------------------

begin;

-- -----------------------------------------------------------------------------
-- 0. "Eu alcanço esta rotina?"
--
-- Existe para a permissão chegar ao DADO, e não só ao menu. Sem isto, esconder o
-- item da barra lateral seria teatro: bastaria pedir a tabela direto.
--
-- Enquanto a rotina não existir no catálogo, isto devolve falso para todo mundo —
-- menos o builder, que passa sempre. É o "nasce fechado" (CORE-07) funcionando
-- sozinho, sem ninguém precisar lembrar de fechar.
-- -----------------------------------------------------------------------------
create or replace function posso(codigo text)
returns boolean language sql stable
as $$
  select
    exists (
      select 1 from perfis p
        join categorias c on c.id = p.categoria_id
       where p.id = auth.uid() and c.nivel = 'builder'
    )
    or exists (
      select 1 from categoria_permissoes cp
       where cp.categoria_id = minha_categoria_id()
         and cp.rotina = codigo
         and cp.pode
    );
$$;

comment on function posso(text) is
  'Verdadeiro se o login atual alcança a rotina. O builder passa sempre.';


-- -----------------------------------------------------------------------------
-- 1. Unidades — as lojas, o CD, o escritório
--
-- SOBRE O "FORA DO ESCOPO"
--   Quatro unidades não são atendidas pela Frota Macedo. O robô antigo as excluía
--   procurando as palavras "juazeiro", "lagoa seca" e "crato" no nome — o que
--   excluiria por acidente qualquer loja futura que tivesse essas palavras.
--
--   Aqui é uma COLUNA, e a exclusão é pelo id do Trílogo, que é exato. No dia em
--   que Crato voltar ao contrato, é um clique no banco, não um envio de código
--   (P-08: parâmetro de negócio mora no banco).
-- -----------------------------------------------------------------------------
create table unidades (
  id            uuid primary key default gen_random_uuid(),
  cliente_id    uuid not null references clientes(id) on delete restrict,
  id_trilogo    integer not null,
  nome          text not null,
  cidade        text,
  uf            text,
  endereco      text,
  cnpj          text,
  no_escopo     boolean not null default true,
  criado_em     timestamptz not null default now(),
  atualizado_em timestamptz not null default now(),
  unique (cliente_id, id_trilogo)
);

comment on column unidades.no_escopo is
  'Falso = a Frota Macedo não atende esta unidade; os chamados dela não entram.';

create trigger unidades_carimbo
  before update on unidades
  for each row execute function tocar_atualizado_em();


-- -----------------------------------------------------------------------------
-- 2. Chamados
-- -----------------------------------------------------------------------------
create table chamados (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,

  -- O número do Trílogo. É a identidade, sozinho — ver a decisão 1 lá em cima.
  numero     integer not null,

  unidade_id uuid references unidades(id) on delete restrict,

  -- Qual das duas contas atende HOJE. Muda de valor quando o Trílogo troca a
  -- empresa prestadora; por isso não entra na chave.
  conta      text not null check (conta in ('instalacoes', 'civil')),

  descricao  text,

  -- Código cru + rótulo, lado a lado — ver a decisão 2.
  status_codigo     smallint,
  status            text,
  prioridade_codigo smallint,
  prioridade        text,
  tipo_codigo       smallint,
  tipo              text,

  natureza      text,
  tipo_predial  text,
  ambiente      text,      -- o caminho completo: "LOJA 13 > Área interna > ..."
  ambiente_id   integer,

  criado_por     text,
  criado_por_id  integer,
  responsavel    text,
  prestadora     text,
  prestadora_cnpj text,

  criado_em      timestamptz,   -- quando nasceu no Trílogo
  prazo          date,

  -- A MARCA D'ÁGUA da leitura rotineira. O Trílogo mexe neste campo a cada
  -- evento do chamado, então ele responde "mudou alguma coisa desde a última vez?"
  -- sem precisar comparar o chamado inteiro.
  alterado_em    timestamptz,

  -- Marcos que o Trílogo já entrega prontos — servem às métricas sem cálculo.
  executado_em      timestamptz,
  vistoriado_em     timestamptz,
  concluido_em      timestamptz,
  tempo_em_execucao numeric(12,4),

  tags     text,
  lido_em  timestamptz not null default now(),

  unique (cliente_id, numero)
);

comment on column chamados.conta is
  'A conta que atende hoje. É atributo, não identidade: o chamado pode trocar.';
comment on column chamados.alterado_em is
  'dateOfLastChange do Trílogo. É por ele que a leitura rotineira sabe o que mudou.';

-- Um índice por pergunta que alguém realmente faz, e nada além (CORE-01):
--   (a) "o que mudou desde a última leitura?"  — o robô, a cada rodada
--   (b) "os chamados desta loja, do mais novo"  — a tela
create index chamados_por_alteracao on chamados (cliente_id, alterado_em desc);
create index chamados_por_unidade   on chamados (cliente_id, unidade_id, criado_em desc);


-- -----------------------------------------------------------------------------
-- 3. Timeline
--
-- SOBRE A COLUNA `chave`
--   Quase todo evento do Trílogo tem id próprio. UM TIPO NÃO TEM: vem com id = 0.
--   Se a chave fosse só o id, esses eventos entrariam de novo a cada leitura, para
--   sempre. Para eles a identidade é uma impressão digital do próprio conteúdo
--   (tipo + data-hora + autor + texto), gravada aqui com o prefixo "h:".
-- -----------------------------------------------------------------------------
create table chamado_eventos (
  id         bigint generated always as identity primary key,
  chamado_id uuid not null references chamados(id) on delete cascade,

  chave      text not null,

  tipo_codigo   smallint,     -- recordType
  tipo          text,         -- 'status', 'anexo', 'prioridade', 'prestadora', ...
  status_codigo smallint,     -- para qual status foi, quando o evento é de status
  status        text,

  quando   timestamptz not null,
  autor    text,
  autor_id integer,
  texto    text,

  unique (chamado_id, chave)
);

comment on column chamado_eventos.chave is
  'Id do evento no Trílogo; ou "h:"+impressão digital, para os eventos que vêm sem id.';

-- A timeline é sempre lida do mais novo para o mais velho, de um chamado por vez.
create index eventos_por_chamado on chamado_eventos (chamado_id, quando desc);


-- -----------------------------------------------------------------------------
-- 4. Custos
-- -----------------------------------------------------------------------------
create table chamado_custos (
  id         uuid primary key default gen_random_uuid(),
  chamado_id uuid not null references chamados(id) on delete cascade,
  id_trilogo integer not null,

  tipo_codigo smallint,
  tipo        text,             -- 'Mão de obra', 'Materiais', ...

  -- Dinheiro é decimal, nunca ponto flutuante (P-12).
  valor          numeric(14,2),
  valor_servico  numeric(14,2),
  valor_material numeric(14,2),

  numero_documento text,
  empresa          text,
  criado_em        timestamptz,

  unique (chamado_id, id_trilogo)
);


-- -----------------------------------------------------------------------------
-- 5. Arquivos — o CONTEÚDO, uma vez só
--
-- A chave é o sha256 do próprio conteúdo. Dois arquivos idênticos são um arquivo,
-- por construção — não por lembrança de quem escreveu o código (P-04).
--
-- A linha só nasce quando o arquivo é REALMENTE copiado para o R2. Na passada de
-- levantamento, que não baixa nada, existem apenas as aparições (tabela abaixo)
-- com o tamanho lido do cabeçalho.
-- -----------------------------------------------------------------------------
create table arquivos (
  sha256     text primary key check (char_length(sha256) = 64),
  cliente_id uuid not null references clientes(id) on delete restrict,
  tamanho    bigint not null,
  tipo       text,
  chave_r2   text not null,
  copiado_em timestamptz not null default now()
);

comment on table arquivos is
  'Um conteúdo, uma linha. O caminho no R2 deriva do sha256 — duplicar é impossível.';


-- -----------------------------------------------------------------------------
-- 6. Anexos — cada APARIÇÃO de um arquivo num chamado
--
-- É AQUI QUE O ORÇAMENTO DEIXA DE DUPLICAR
--   Ciclo temido: o FrotaHub gera o orçamento, guarda no R2, sobe no Trílogo, e o
--   robô lê de volta. Sem esta separação, o mesmo PDF entraria duas vezes.
--
--   Com ela: o robô vê um anexo novo, calcula o sha256, reconhece o conteúdo, e
--   grava só uma APARIÇÃO nova apontando para o arquivo que já existe.
--
--   E existe um atalho antes disso: quando o FrotaHub sobe um arquivo, ele guarda
--   o id que o Trílogo devolveu. Na leitura seguinte o robô reconhece o id e NEM
--   BAIXA. O sha256 fica como rede de segurança, para o caso de alguém subir o
--   mesmo arquivo pela tela do Trílogo, onde não temos o id.
--
-- DUAS COLEÇÕES DIFERENTES
--   No Trílogo, foto e vídeo ficam em `attachments`; os nossos ORÇAMENTOS ficam
--   em `invoiceFiles`, dentro do custo. São listas separadas, e os ids podem
--   coincidir entre elas — por isso a coleção entra na chave.
-- -----------------------------------------------------------------------------
create table chamado_anexos (
  id         uuid primary key default gen_random_uuid(),
  chamado_id uuid not null references chamados(id) on delete cascade,

  colecao    text not null check (colecao in ('anexo', 'orcamento')),
  id_trilogo integer not null,
  custo_id   uuid references chamado_custos(id) on delete cascade,

  nome        text,
  extensao    text,
  url_origem  text not null,

  -- Vem do cabeçalho HTTP, sem baixar o arquivo. É o que responde "quantos GB?"
  -- antes de gastar o primeiro byte de R2.
  tamanho bigint,
  tipo    text,

  autor    text,
  autor_id integer,
  quando   timestamptz,

  -- De onde veio. 'sistema-antigo' marca os orçamentos que o robô em Python subiu
  -- (dá para reconhecer: têm nome de arquivo temporário).
  origem text not null default 'trilogo'
    check (origem in ('trilogo', 'frotahub', 'sistema-antigo')),

  -- Nulo enquanto o conteúdo não foi copiado para o R2.
  arquivo_sha256 text references arquivos(sha256) on delete restrict,

  visto_em timestamptz not null default now(),

  unique (chamado_id, colecao, id_trilogo)
);

-- "O que ainda falta copiar?" é a pergunta da passada 2, e ela roda em lote.
create index anexos_a_copiar on chamado_anexos (chamado_id) where arquivo_sha256 is null;


-- -----------------------------------------------------------------------------
-- 7. Rodadas do robô
--
-- O andamento mora no BANCO, não na memória do servidor (P-03). O motor do plano
-- gratuito adormece e reinicia; se o progresso estivesse só na memória, uma
-- rodada interrompida não saberia de onde continuar.
--
-- A marca d'água só avança em rodada CONCLUÍDA. Se avançasse durante, uma rodada
-- interrompida pularia para sempre os chamados que ela não chegou a processar.
-- -----------------------------------------------------------------------------
create table robo_execucoes (
  id         uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,

  robo  text not null,                       -- 'trilogo'
  modo  text not null check (modo in ('levantamento', 'copia', 'atualizacao')),

  situacao text not null default 'rodando'
    check (situacao in ('rodando', 'concluida', 'falhou', 'interrompida')),

  disparado_por text,                        -- o usuário, ou 'agendador'
  autor_id      uuid,

  janela_de   date,                          -- de onde a leitura começou
  marca_dagua timestamptz,                   -- até onde leu (só vale se concluída)

  chamados_lidos    integer not null default 0,
  chamados_gravados integer not null default 0,
  eventos_gravados  integer not null default 0,
  arquivos_vistos   integer not null default 0,
  bytes_vistos      bigint  not null default 0,
  arquivos_copiados integer not null default 0,
  bytes_copiados    bigint  not null default 0,

  erro        text,
  comecou_em  timestamptz not null default now(),
  terminou_em timestamptz
);

create index execucoes_recentes on robo_execucoes (cliente_id, robo, comecou_em desc);


-- -----------------------------------------------------------------------------
-- 8. Fechaduras
--
-- Tudo nasce fechado (CORE-07). Estas tabelas abrem só para LEITURA, e só para
-- quem alcança a rotina — que hoje não existe, então só o builder passa.
--
-- Escrever é exclusividade do motor, que usa a chave de serviço. O navegador não
-- grava aqui de jeito nenhum.
-- -----------------------------------------------------------------------------
alter table unidades        enable row level security;
alter table chamados        enable row level security;
alter table chamado_eventos enable row level security;
alter table chamado_custos  enable row level security;
alter table arquivos        enable row level security;
alter table chamado_anexos  enable row level security;
alter table robo_execucoes  enable row level security;

create policy "unidades do meu cliente"
  on unidades for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_TRILOGO_DADOS'));

create policy "chamados do meu cliente"
  on chamados for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_TRILOGO_DADOS'));

create policy "rodadas do meu cliente"
  on robo_execucoes for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_TRILOGO_DADOS'));

-- As tabelas filhas herdam a permissão do chamado a que pertencem: não existe
-- caso em que alguém possa ver o evento sem poder ver o chamado.
create policy "eventos dos chamados que eu vejo"
  on chamado_eventos for select to authenticated
  using (exists (select 1 from chamados c
                  where c.id = chamado_id
                    and c.cliente_id = meu_cliente_id())
         and posso('CONTRATO_TRILOGO_DADOS'));

create policy "custos dos chamados que eu vejo"
  on chamado_custos for select to authenticated
  using (exists (select 1 from chamados c
                  where c.id = chamado_id
                    and c.cliente_id = meu_cliente_id())
         and posso('CONTRATO_TRILOGO_DADOS'));

create policy "anexos dos chamados que eu vejo"
  on chamado_anexos for select to authenticated
  using (exists (select 1 from chamados c
                  where c.id = chamado_id
                    and c.cliente_id = meu_cliente_id())
         and posso('CONTRATO_TRILOGO_DADOS'));

-- `arquivos` fica SEM política de leitura, de propósito. Ela guarda o caminho no
-- R2 de todo arquivo do sistema; quem precisa de um arquivo pede ao motor, que
-- confere o acesso e devolve um endereço temporário (P-20). Ninguém varre o
-- acervo inteiro pelo navegador.


-- -----------------------------------------------------------------------------
-- 9. As quatro unidades fora do escopo
--
-- São as ÚNICAS semeadas aqui. As outras 34 o robô cria sozinho quando as
-- encontra — semear 38 linhas à mão seria trabalho repetido, com risco de erro de
-- digitação e de ficar velho no dia em que abrir uma loja nova.
--
-- Estas quatro precisam existir ANTES, porque a regra depende delas: sem a linha,
-- o robô não teria como saber que elas ficam de fora. Os ids foram conferidos no
-- Trílogo em 23/08/2026.
-- -----------------------------------------------------------------------------
insert into unidades (cliente_id, id_trilogo, nome, cidade, uf, no_escopo)
select c.id, u.id_trilogo, u.nome, u.cidade, 'CE', false
from clientes c,
     (values (94,  'LOJA 07 - CRATO',                  'CRATO'),
             (95,  'LOJA 12 - JUAZEIRO',               'JUAZEIRO DO NORTE'),
             (96,  'LOJA 25 - LAGOA SECA',             'JUAZEIRO DO NORTE'),
             (300, 'LOJA 31 - MERCADÃO NOVO JUAZEIRO', 'JUAZEIRO DO NORTE')
     ) as u(id_trilogo, nome, cidade)
where c.slug = 'frota-macedo'
on conflict (cliente_id, id_trilogo) do nothing;


insert into schema_migrations (versao, arquivo) values ('007', '007_trilogo.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop table if exists robo_execucoes, chamado_anexos, arquivos,
--                      chamado_custos, chamado_eventos, chamados, unidades cascade;
-- drop function if exists posso(text);
-- delete from schema_migrations where versao = '007';
