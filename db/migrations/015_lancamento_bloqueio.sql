-- =============================================================================
-- 015 — por que não lançou, e quem tem que resolver                      rev 1
-- =============================================================================
--
-- Hoje, quando o Trílogo recusa um lançamento, três coisas acontecem de errado:
-- a frase deles é jogada fora, nada é gravado, e o orçamento cai na área de
-- Correções junto com todos os outros que só ainda não foram tentados.
--
-- Esta migration dá o vocabulário para consertar isso:
--
--   1. a FLAG do bloqueio, em lista fechada, mais a frase crua do Trílogo
--   2. `ticket_avisos` — o registro de que um ticket já foi cobrado, e de quem
--   3. as views passam a calcular o DESTINO: quem resolve aquele orçamento
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. A flag do bloqueio
--
-- POR QUE LISTA FECHADA, E NÃO A FRASE DO TRÍLOGO
--
--   A tela precisa oferecer correções DIFERENTES conforme o motivo — e menu não
--   se monta a partir de texto livre. A categoria é o que o programa lê; a frase
--   é o que a pessoa lê. As duas coisas, lado a lado, porque nenhuma substitui a
--   outra: sem a categoria a tela não decide, sem a frase ninguém entende.
--
--   As seis categorias saíram de recusas OBSERVADAS em 25/08/2026, testando
--   contra o Trílogo de verdade — não de imaginação.
--
-- `lancamento_tentado_em` É O QUE SEPARA "FALHOU" DE "AINDA NÃO TENTEI"
--
--   É a distinção que a tela de Correções não tinha. Orçamento recém-nascido não
--   é uma correção: é trabalho na fila. Como a flag só é escrita quando uma
--   tentativa é RECUSADA, quem nunca foi tentado nunca aparece lá — por
--   construção, não por filtro que alguém precisa lembrar de escrever.
-- -----------------------------------------------------------------------------
alter table orcamentos add column if not exists lancamento_bloqueio          text;
alter table orcamentos add column if not exists lancamento_bloqueio_detalhe  text;
alter table orcamentos add column if not exists lancamento_tentado_em        timestamptz;
alter table orcamentos add column if not exists lancamento_tentativas        smallint not null default 0;

alter table orcamentos drop constraint if exists orcamentos_lancamento_bloqueio_check;
alter table orcamentos add constraint orcamentos_lancamento_bloqueio_check
  check (lancamento_bloqueio is null or lancamento_bloqueio in (
    'ticket_status',    -- "não é permitida para tickets com o status ..."
                        -- A mensagem deles MENTE: cita só 'Aberto' e 'Em
                        -- execução', mas recusa 'Arquivado' também — medido no
                        -- ticket 126211. Nunca decidir lendo a frase.
    'ticket_recusado',  -- o ticket não existe, ou não é da conta que tentou
    'teto',             -- entre gerar e lançar, o ticket recebeu outro custo
    'sem_empresa',      -- o CompanyId daquela conta não está configurado aqui
    'trilogo_fora',     -- login ou rede falharam: é temporário, não é defeito
    'desconhecido'      -- qualquer outra. A frase crua fica no detalhe.
  ));

comment on column orcamentos.lancamento_bloqueio is
  'Por que a última tentativa de lançar foi recusada. Nulo = nunca foi tentado, ou a última tentativa não foi recusada.';
comment on column orcamentos.lancamento_bloqueio_detalhe is
  'A resposta do Trílogo, palavra por palavra. Não traduzir: é ela que explica o caso que a categoria não cobre.';

-- A fila de Correções. Índice parcial: só os bloqueados ocupam espaço.
create index if not exists orcamentos_bloqueados
  on orcamentos (cliente_id, lancamento_bloqueio, lancamento_tentado_em desc)
  where lancamento_bloqueio is not null;


-- -----------------------------------------------------------------------------
-- 2. ticket_avisos — o que já foi cobrado, de quem
--
-- POR QUE TABELA, E NÃO COLUNA NO ORÇAMENTO
--
--   O destino é do TICKET, não do orçamento: um ticket com três orçamentos
--   parados manda os três na mesma cobrança, e o cliente recebe UMA linha. Uma
--   coluna em `orcamentos` obrigaria três marcas a concordarem entre si — e o
--   dia em que discordassem ninguém saberia qual valia.
--
--   Sendo tabela, cobrar duas vezes é duas linhas, e a segunda não apaga a
--   primeira. É isso que separa "já cobrei e não veio" de "nunca mandei".
--
-- `motivo` GUARDA O ESTADO DA ÉPOCA
--
--   O destino é calculado do status de agora e muda sozinho. Então o aviso
--   precisa gravar por que foi enviado NAQUELE dia — senão, meses depois, um
--   aviso mandado ao cliente por ticket arquivado apareceria sem sentido ao lado
--   de um ticket que hoje está aberto.
-- -----------------------------------------------------------------------------
create table if not exists ticket_avisos (
  id          uuid primary key default gen_random_uuid(),
  cliente_id  uuid not null references clientes(id) on delete restrict,
  ticket      integer not null,
  lista       text not null check (lista in ('cliente', 'encarregados')),
  motivo      text,
  quantos     integer,
  valor       numeric(14,2),
  avisado_em  timestamptz not null default now(),
  avisado_por uuid references perfis(id) on delete set null
);

comment on table ticket_avisos is
  'Cada vez que um ticket entrou numa lista enviada. Acrescenta, nunca sobrescreve.';

create index if not exists ticket_avisos_por_ticket
  on ticket_avisos (cliente_id, ticket, avisado_em desc);

alter table ticket_avisos enable row level security;

drop policy if exists "avisos do meu cliente" on ticket_avisos;
create policy "avisos do meu cliente"
  on ticket_avisos for select to authenticated
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_ORCAMENTOS'));


-- -----------------------------------------------------------------------------
-- 3. O destino, calculado numa view — não gravado
--
-- POR QUE NÃO É COLUNA
--
--   O destino sai do status do ticket, e o status muda sem nos avisar: medimos
--   7 dos 26 bloqueados destravando sozinhos em seis dias. Gravado, ele
--   envelheceria mentindo — o mesmo defeito da flag velha. Calculado, um ticket
--   que o cliente reabre hoje sai da lista dele e entra na dos encarregados
--   amanhã, sem ninguém mexer.
--
-- COMO 'ABERTO PELA PRIMEIRA VEZ' SE SEPARA DE 'REABERTO'
--
--   Os dois têm status 1. A diferença mora na timeline: reaberto é o que já
--   MUDOU de status alguma vez e voltou. Medido em 25/08/2026: dos 425 chamados
--   abertos, 16 tinham mudança de status e 409 não — e o texto dos 16 confirma
--   ("reaberto", "não foi executado", "problema não foi resolvido").
--
--   O tipo 2 é a abertura e existe uma vez por chamado; por isso ele é excluído.
--
-- QUEM RESOLVE CADA UM
--
--   Executado / Vistoriado  ->  ninguém: dá para lançar agora
--   Arquivado               ->  CLIENTE: só ele reabre
--   Aberto e já reaberto    ->  CLIENTE: o serviço voltou, ele precisa decidir
--   Aberto pela primeira vez->  ENCARREGADOS: é serviço nosso a fazer
--   Em execução             ->  ENCARREGADOS: idem
-- -----------------------------------------------------------------------------
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
       -- ------- daqui para baixo é o que a 015 acrescenta -------
       o.lancamento_bloqueio,
       o.lancamento_bloqueio_detalhe,
       o.lancamento_tentado_em,
       o.lancamento_tentativas,
       c.status        as ticket_status,
       c.status_codigo as ticket_status_codigo,
       r.reaberto,
       r.motivo_reabertura,
       case
         when o.status <> 'gerado'            then null
         when c.id is null                    then 'sem_chamado'
         when c.status_codigo in (5, 6)       then 'pode_lancar'
         when c.status_codigo = 3             then 'cliente'
         when c.status_codigo = 1 and r.reaberto then 'cliente'
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
     where e.chamado_id = c.id and e.tipo = 'status' and e.tipo_codigo <> 2
  ) as reaberto,
  (select e.texto from chamado_eventos e
    where e.chamado_id = c.id and e.tipo = 'status' and e.tipo_codigo <> 2
      and coalesce(e.texto, '') <> ''
    order by e.quando desc limit 1) as motivo_reabertura
) r on true;


-- O painel ganha os números que faltavam. Antes, `a_lancar` somava numa conta só
-- o que espera a gente e o que espera outra pessoa — e um número que mistura
-- duas perguntas não responde nenhuma.
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
  -- ------- daqui para baixo é o que a 015 acrescenta -------
  (select count(*) from orcamentos o
     where o.cliente_id = cl.id and o.status = 'gerado'
       and o.lancamento_bloqueio is not null) as recusados,
  (select count(*) from orcamentos_lista l
     where l.cliente_id = cl.id and l.destino = 'pode_lancar')   as prontos_para_lancar,
  (select count(*) from orcamentos_lista l
     where l.cliente_id = cl.id and l.destino = 'cliente')       as esperando_cliente,
  (select count(*) from orcamentos_lista l
     where l.cliente_id = cl.id and l.destino = 'encarregados')  as esperando_equipe
from clientes cl;


insert into schema_migrations (versao, arquivo)
values ('015', '015_lancamento_bloqueio.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select destino, count(*), sum(valor) from orcamentos_lista
--    where status = 'gerado' group by 1 order by 2 desc;
--
--   select lancamento_bloqueio, count(*) from orcamentos group by 1;
--
-- PARA DESFAZER
--
--   drop index if exists orcamentos_bloqueados;
--   drop table if exists ticket_avisos;
--   alter table orcamentos drop column if exists lancamento_bloqueio,
--     drop column if exists lancamento_bloqueio_detalhe,
--     drop column if exists lancamento_tentado_em,
--     drop column if exists lancamento_tentativas;
--   (as views voltam rodando a 010 de novo — ela é `create or replace`)
--   delete from schema_migrations where versao = '015';
-- =============================================================================
