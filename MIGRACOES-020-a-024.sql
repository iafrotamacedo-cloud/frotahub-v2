-- =============================================================================
-- FrotaHub — as cinco migrações da parada do GitHub, NA ORDEM
--
--   020  desconto do fornecedor        colunas + orcamentos_lista + documentos_lista
--   021  a nota adota o chamado        gatilho em `chamados` + backfill
--   022  desconto autorizado           colunas + documentos_lista
--   023  faturamento direto            fila 'direto' + orcamentos_painel
--   024  pedido de faturamento         tabela, pedido_id, view e rotina
--
-- ⚠ RODE COM O SISTEMA PARADO
--
--   Em 26/08/2026 a primeira tentativa morreu com `40P01: deadlock detected`.
--   Os dois lados eram: esta migração, segurando `chamados` para trocar o
--   gatilho da 021; e o robô do Trílogo, segurando o armazenamento (`buckets`)
--   e querendo LER `chamados` para copiar anexos. Cada um esperando o outro.
--
--   Nada foi aplicado — o editor do Supabase envolve o script numa transação, e
--   o deadlock desfez tudo. Mas o certo é não chegar lá.
--
--   ANTES DE RODAR, confira na aba Actions que não há nada em andamento:
--     · Leitor de notas    dispara aos :05 e :35 de CADA hora
--     · Robô do Trílogo    dispara às horas ímpares em UTC (14h, 16h, 18h aqui)
--
-- AS DUAS LINHAS ABAIXO SÃO O QUE IMPEDE A ESPERA ETERNA
--
--   Sem `lock_timeout`, uma instrução que precisa de trava exclusiva espera
--   para sempre — e só descobre o problema quando o Postgres detecta o ciclo,
--   que foi o que aconteceu. Com ele, se a trava não estiver livre em dez
--   segundos a migração desiste na hora, com mensagem clara, e nada é
--   aplicado. Falhar rápido e explicado é melhor que travar e adivinhar.
--
--   `statement_timeout` cobre o outro caso: uma instrução que PEGOU a trava e
--   está demorando demais por outro motivo. Dez minutos é folga enorme para
--   este script — se bater nisso, alguma coisa está errada e é melhor parar.
-- =============================================================================

set lock_timeout      = '10s';
set statement_timeout = '10min';


-- >>>>>>>>>>>>>>>>>>>>>>>>>>  020_desconto_do_fornecedor.sql  <<<<<<<<<<<<<<<<<<<<<<<<<<

-- 020 — o desconto do fornecedor, e a nota que não cabe no teto
--
-- O QUE ESTAVA ERRADO
--
--   `NF 9160`, de 26/08/2026: cinco itens somando R$ 514,60 e a nota dizendo
--   R$ 463,14. A diferença é exatamente 10% — desconto do fornecedor, não erro
--   de leitura.
--
--   Até aqui a margem era marcada em cima dos ITENS. Ou seja: 20% sobre
--   R$ 514,60, um custo que a empresa não teve. E do outro lado, uma nota cujo
--   bruto passa do teto era recusada mesmo quando o que se pagou por ela cabia
--   folgado.
--
-- AS TRÊS COLUNAS
--
--   `valor_nota_cheio` guarda o bruto ao lado do `valor_nota`, que passa a ser
--   o que se pagou. Os dois juntos, para que seis meses depois alguém consiga
--   ver que a nota valia R$ 700, que se pagou R$ 450, e que o orçamento saiu
--   por R$ 600 porque era o teto — sem reconstruir a conta de cabeça.
--
--   `ajustado_pelo_teto` é o carimbo interno: NOSSO, nunca do cliente. Ele não
--   entra no PDF que sai daqui.
--
--   `documentos.bloqueio_motivo` é a nota que não pode rodar, com a frase que
--   diz por quê. Bloqueio sem saída visível é o que faz o usuário criar
--   planilha paralela.

alter table orcamentos
  add column if not exists valor_nota_cheio numeric(14,2),
  add column if not exists ajustado_pelo_teto boolean not null default false;

comment on column orcamentos.valor_nota_cheio is
  'A soma dos itens como estão na nota, ANTES do desconto do fornecedor. '
  'valor_nota é o que se pagou. Iguais quando não houve desconto.';

comment on column orcamentos.ajustado_pelo_teto is
  'O bruto da nota passava do maior custo que cabe no teto, e o orçamento foi '
  'fechado no teto exato. Carimbo INTERNO — não aparece no documento do cliente.';

alter table documentos
  add column if not exists bloqueio_motivo text;

comment on column documentos.bloqueio_motivo is
  'Quando preenchida, esta nota não gera orçamento e a frase diz por quê. '
  'Hoje só o limite de teto a preenche. Nulo é o normal.';

-- A tela de notas precisa mostrar as bloqueadas para tratamento, e a geração
-- precisa pulá-las sem varrer a tabela.
create index if not exists documento_bloqueado
  on documentos (cliente_id, bloqueio_motivo)
  where bloqueio_motivo is not null;

-- ---------------------------------------------------------------------------
-- a tela precisa ENXERGAR o carimbo
-- ---------------------------------------------------------------------------
--
-- ⚠ `create or replace view` só ACRESCENTA coluna no fim: nome, ordem e tipo
--    das que já existem são imutáveis. As 33 colunas abaixo estão reproduzidas
--    VERBATIM da definição que está em produção, e as duas novas vão no fim.
--    Mexer na ordem aqui derruba a tela de lançamento inteira.

create or replace view orcamentos_lista as
 SELECT o.id,
    o.cliente_id,
    o.ticket,
    o.parte,
    o.conta,
    o.status,
    o.valor,
    o.valor_nota,
    o.reduzido_pelo_teto,
    o.valor_antes_do_teto,
    o.rateio,
    o.criado_em,
    o.lancado_em,
    o.faturado,
    o.pago,
    o.trilogo_custo_id,
    o.arquivo_pdf_sha256,
    u.nome AS loja,
    c.descricao AS chamado_descricao,
    ( SELECT string_agg(d.numero, ', '::text ORDER BY d.numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d ON d.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d.numero IS NOT NULL) AS notas,
    ( SELECT string_agg(d.dav_numero, ', '::text ORDER BY d.dav_numero) AS string_agg
           FROM orcamento_documentos od
             JOIN documentos d ON d.id = od.documento_id
          WHERE od.orcamento_id = o.id AND d.dav_numero IS NOT NULL) AS davs,
    o.lancamento_bloqueio,
    o.lancamento_bloqueio_detalhe,
    o.lancamento_tentado_em,
    o.lancamento_tentativas,
    c.status AS ticket_status,
    c.status_codigo AS ticket_status_codigo,
    (c.status_codigo = ANY (ARRAY[1, 7])) AND r.ja_terminou AS reaberto,
        CASE
            WHEN (c.status_codigo = ANY (ARRAY[1, 7])) AND r.ja_terminou THEN r.ultima_frase
            ELSE NULL::text
        END AS motivo_reabertura,
        CASE
            WHEN o.status <> 'gerado'::text THEN NULL::text
            WHEN c.id IS NULL THEN 'sem_chamado'::text
            WHEN c.status_codigo = ANY (ARRAY[5, 6]) THEN 'pode_lancar'::text
            WHEN c.status_codigo = 3 THEN 'cliente'::text
            WHEN c.status_codigo = 1 AND r.ja_terminou THEN 'cliente'::text
            WHEN c.status_codigo = 1 THEN 'encarregados'::text
            WHEN c.status_codigo = 7 THEN 'encarregados'::text
            ELSE 'outro'::text
        END AS destino,
    ( SELECT a.avisado_em
           FROM ticket_avisos a
          WHERE a.cliente_id = o.cliente_id AND a.ticket = o.ticket
          ORDER BY a.avisado_em DESC
         LIMIT 1) AS avisado_em,
    o.unidade_id,
    o.fatura_id,
    o.valor_nota_cheio,
    o.ajustado_pelo_teto
   FROM orcamentos o
     LEFT JOIN unidades u ON u.id = o.unidade_id
     LEFT JOIN chamados c ON c.id = o.chamado_id
     LEFT JOIN LATERAL ( SELECT (EXISTS ( SELECT 1
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[3, 5, 6])))) AS ja_terminou,
            ( SELECT e.texto
                   FROM chamado_eventos e
                  WHERE e.chamado_id = c.id AND e.tipo = 'status'::text AND (e.status_codigo = ANY (ARRAY[1, 7])) AND COALESCE(e.texto, ''::text) <> ''::text
                  ORDER BY e.quando DESC
                 LIMIT 1) AS ultima_frase) r ON true;

-- ---------------------------------------------------------------------------
-- e a lista de NOTAS mostra o bloqueio
-- ---------------------------------------------------------------------------
--
-- Pelo mesmo motivo da outra: quem vai tratar precisa ler a frase, não
-- descobrir que a nota sumiu da fila. As 25 colunas da 019 estão reproduzidas
-- verbatim; `bloqueio_motivo` vai no fim, e `pronto_para_gerar` passa a exigir
-- que ela seja nula — nota bloqueada anunciada como pronta é a tela mentindo.

create or replace view documentos_lista as
 SELECT d.id,
    d.cliente_id,
    d.fila,
    d.tipo,
    d.numero,
    d.dav_numero,
    d.chave_acesso,
    d.emitente_nome,
    d.emissao,
    d.valor_total,
    d.status,
    d.leitura_camada,
    d.leitura_confianca,
    d.nome_arquivo,
    d.arquivo_sha256,
    d.inserido_em,
    d.oculto_em,
    COALESCE(t.quantos, 0::bigint) AS tickets,
    COALESCE(t.lista, '{}'::integer[]) AS ticket_numeros,
    COALESCE(i.quantos, 0::bigint) AS itens,
    COALESCE(t.soltos, '{}'::integer[]) AS ticket_soltos,
    d.status = 'lido'::text
      AND COALESCE(t.quantos, 0::bigint) > 0
      AND COALESCE(array_length(t.soltos, 1), 0) = 0
      AND d.duplicada_de IS NULL
      AND d.bloqueio_motivo IS NULL                    AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo
   FROM documentos d
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos,
            array_agg(dt.ticket ORDER BY dt.ticket) AS lista,
            array_remove(array_agg(
                CASE
                    WHEN dt.chamado_id IS NULL THEN dt.ticket
                    ELSE NULL::integer
                END ORDER BY dt.ticket), NULL::integer) AS soltos
           FROM documento_tickets dt
          WHERE dt.documento_id = d.id) t ON true
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos
           FROM documento_itens di
          WHERE di.documento_id = d.id) i ON true
     LEFT JOIN documentos o ON o.id = d.duplicada_de;

-- O REGISTRO DA VERSÃO
--
--   Faltava. As outras quatro desta leva o têm, e sem ele o `schema_migrations`
--   diria que a 020 nunca rodou — o que faria a próxima pessoa a olhar aquela
--   tabela concluir que o banco está atrás do código, e possivelmente rodá-la
--   de novo. As instruções aqui são idempotentes, então rodar de novo não
--   quebraria nada; mas confiar numa tabela de controle que mente é como se
--   perde a conta do que já foi aplicado.
insert into schema_migrations (versao, arquivo)
values ('020', '020_desconto_do_fornecedor.sql')
on conflict (versao) do nothing;

-- >>>>>>>>>>>>>>>>>>>>>>>>>>  021_nota_adota_chamado.sql  <<<<<<<<<<<<<<<<<<<<<<<<<<

-- 021 — a NOTA também adota o chamado que chegou depois
--
-- O DEFEITO, E COMO ELE APARECEU
--
--   A 018 fez os ORÇAMENTOS adotarem o chamado que chega depois. Ninguém fez o
--   mesmo pelas NOTAS — e é nelas que o problema começa.
--
--   Quando um ticket é amarrado a uma nota, o motor procura o chamado naquele
--   instante. Se ele ainda não veio do Trílogo, a linha nasce com
--   `chamado_id = null`: é a nota "sem associação". Duas horas depois o robô
--   traz o chamado, e a linha da nota CONTINUA nula. Para sempre.
--
--   Nada no sistema voltava para religá-la. A nota ficava travada num problema
--   que já tinha deixado de existir, e o único jeito de destravar era alguém
--   mexer no ticket de novo — reescrevendo o mesmo número que já estava certo.
--
-- POR QUE ISSO FICOU URGENTE EM 26/08/2026
--
--   Porque o botão "atualizar" da tela de tratamento nasceu neste dia, e ele
--   faz exatamente isto: relê o Trílogo e reconfere as notas. Sem esta adoção,
--   ele releria, o chamado entraria, e a linha continuaria VERMELHA — porque a
--   conferência lê o `chamado_id` guardado, não o número do ticket.
--
--   O botão pareceria quebrado. E o pior: pareceria quebrado dizendo a verdade
--   sobre um dado errado, que é o tipo de defeito que ninguém consegue
--   diagnosticar olhando a tela.
--
-- E O NÚMERO VELHO NUNCA SOBREVIVE AO CONSERTO
--
--   Esta é a garantia que o dono pediu. A geração lê os tickets da nota na hora
--   de gerar (`documento_tickets`, sempre fresco), e não uma cópia guardada. O
--   que faltava era o elo `ticket -> chamado` acompanhar a realidade. Agora
--   acompanha, dos dois lados: a nota e o orçamento.

create or replace function chamado_adota_notas()
returns trigger language plpgsql as $$
begin
  -- `documento_tickets` não tem cliente_id: ele vem pela nota. O `exists`
  -- garante que um chamado de um cliente nunca adote a nota de outro — a mesma
  -- numeração pode existir em dois contratos.
  update documento_tickets dt
     set chamado_id = new.id
   where dt.ticket     = new.numero
     and dt.chamado_id is null
     and exists (
       select 1 from documentos d
        where d.id = dt.documento_id
          and d.cliente_id = new.cliente_id
          and d.oculto_em is null);
  return null;
end;
$$;

comment on function chamado_adota_notas() is
  'Ao entrar um chamado, liga a ele os tickets de nota daquele número que estavam soltos. '
  'Sem isto, uma nota fica em "sem associação" para sempre, mesmo depois de o chamado chegar.';

drop trigger if exists adota_notas on chamados;
create trigger adota_notas
  after insert on chamados
  for each row
  execute function chamado_adota_notas();

-- -----------------------------------------------------------------------------
-- A adoção dos que já estavam esperando
--
-- O gatilho vale para quem chegar daqui em diante. Estes já estão no banco,
-- soltos, esperando desde o dia em que o ticket foi escrito.
-- -----------------------------------------------------------------------------
update documento_tickets dt
   set chamado_id = c.id
  from documentos d, chamados c
 where d.id          = dt.documento_id
   and c.numero      = dt.ticket
   and c.cliente_id  = d.cliente_id
   and dt.chamado_id is null
   and d.oculto_em   is null;

insert into schema_migrations (versao, arquivo)
values ('021', '021_nota_adota_chamado.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- não pode sobrar ticket solto cujo chamado JÁ existe na base:
--   select dt.ticket
--     from documento_tickets dt
--     join documentos d on d.id = dt.documento_id
--    where dt.chamado_id is null
--      and d.oculto_em is null
--      and exists (select 1 from chamados c
--                   where c.numero = dt.ticket and c.cliente_id = d.cliente_id);
--   -> nenhuma linha
--
--   -- e os que continuam soltos são os que realmente não existem:
--   select dt.ticket from documento_tickets dt
--     join documentos d on d.id = dt.documento_id
--    where dt.chamado_id is null and d.oculto_em is null;
--   -> só tickets que o Trílogo ainda não trouxe
--
-- PARA DESFAZER
--   drop trigger if exists adota_notas on chamados;
--   drop function if exists chamado_adota_notas();
--   (a adoção já feita não se desfaz — e não deveria: ela corrigiu um dado errado.)
-- =============================================================================

-- >>>>>>>>>>>>>>>>>>>>>>>>>>  022_desconto_autorizado.sql  <<<<<<<<<<<<<<<<<<<<<<<<<<

-- 022 — o desconto que alguém assina embaixo, e a nota que espera o cliente
--
-- AS TRÊS PORTAS DE SAÍDA DE UMA NOTA QUE NÃO CABE NO TETO
--
--   1. desconto     alguém abre mão de parte da margem para a nota fechar no
--                   teto. Exige confirmação e senha, e nunca passa de 20% —
--                   que é a margem inteira.
--   2. aprovação    o orçamento nasce com o valor CHEIO e fica parado, porque
--                   é ele que vai ao cliente. Aprovado, entra na fila de
--                   lançar.
--   3. rateio       a nota atende vários chamados e estava na gaveta errada.
--
--   (a quarta é excluir, que já existe e é `oculto_em`.)
--
-- POR QUE O DESCONTO MORA NA NOTA, E NÃO NO ORÇAMENTO
--
--   Porque ele é autorizado ANTES de o orçamento existir. E porque uma nota
--   rateada vira vários orçamentos: a autorização é uma só, e repeti-la em
--   cada pedaço faria três registros do mesmo ato — que é como se perde a
--   conta de quantas vezes alguém autorizou o quê.

alter table documentos
  add column if not exists desconto_bp integer not null default 0,
  add column if not exists desconto_em timestamptz,
  add column if not exists desconto_por uuid references perfis(id) on delete set null,
  add column if not exists aprovacao_pedida boolean not null default false,
  add column if not exists aprovacao_pedida_em timestamptz,
  add column if not exists aprovacao_pedida_por uuid references perfis(id) on delete set null;

-- ⚠ O LIMITE VIVE NO BANCO TAMBÉM, E NÃO SÓ NO GO
--   A regra dos 20% é conferida no motor antes de gravar. Mas uma trava que
--   existe num lugar só é uma trava que some no dia em que alguém escrever a
--   segunda rotina que grava aqui — e a segunda rotina sempre aparece.
alter table documentos drop constraint if exists desconto_dentro_da_margem;
alter table documentos add constraint desconto_dentro_da_margem
  check (desconto_bp >= 0 and desconto_bp <= 2000);

comment on column documentos.desconto_bp is
  'Desconto autorizado, em pontos-base do orçamento (385 = 3,85%). Zero é o normal. '
  'Teto de 2000 = 20%, que é a margem inteira: abaixo disso a empresa trabalharia pagando.';
comment on column documentos.aprovacao_pedida is
  'A nota foi mandada para aprovação do cliente. O orçamento nasce com o valor CHEIO '
  'e parado em aguardando_aprovacao — é ele que vai ao cliente, não a nota.';

create index if not exists documento_com_desconto
  on documentos (cliente_id, desconto_em)
  where desconto_bp > 0;

-- ---------------------------------------------------------------------------
-- a lista precisa mostrar o que já foi autorizado
-- ---------------------------------------------------------------------------
--
-- ⚠ As 26 colunas abaixo são a definição da 020, VERBATIM. `create or replace
--    view` só acrescenta no fim: nome, ordem e tipo das existentes são
--    imutáveis. As quatro novas vão depois.
--
--    `pronto_para_gerar` ganha mais um termo: uma nota que pediu aprovação não
--    está pronta para gerar pelo caminho normal — ela gera por outro, e
--    anunciá-la como pronta faria alguém processá-la duas vezes.

create or replace view documentos_lista as
 SELECT d.id,
    d.cliente_id,
    d.fila,
    d.tipo,
    d.numero,
    d.dav_numero,
    d.chave_acesso,
    d.emitente_nome,
    d.emissao,
    d.valor_total,
    d.status,
    d.leitura_camada,
    d.leitura_confianca,
    d.nome_arquivo,
    d.arquivo_sha256,
    d.inserido_em,
    d.oculto_em,
    COALESCE(t.quantos, 0::bigint) AS tickets,
    COALESCE(t.lista, '{}'::integer[]) AS ticket_numeros,
    COALESCE(i.quantos, 0::bigint) AS itens,
    COALESCE(t.soltos, '{}'::integer[]) AS ticket_soltos,
    d.status = 'lido'::text
      AND COALESCE(t.quantos, 0::bigint) > 0
      AND COALESCE(array_length(t.soltos, 1), 0) = 0
      AND d.duplicada_de IS NULL
      AND d.bloqueio_motivo IS NULL
      AND d.aprovacao_pedida IS NOT TRUE                 AS pronto_para_gerar,
    d.duplicada_de,
    o.nome_arquivo AS duplicada_de_nome,
    o.inserido_em AS duplicada_de_em,
    d.bloqueio_motivo,
    -- daqui para baixo, o que a 022 acrescenta
    d.desconto_bp,
    d.desconto_em,
    d.aprovacao_pedida,
    d.aprovacao_pedida_em
   FROM documentos d
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos,
            array_agg(dt.ticket ORDER BY dt.ticket) AS lista,
            array_remove(array_agg(
                CASE
                    WHEN dt.chamado_id IS NULL THEN dt.ticket
                    ELSE NULL::integer
                END ORDER BY dt.ticket), NULL::integer) AS soltos
           FROM documento_tickets dt
          WHERE dt.documento_id = d.id) t ON true
     LEFT JOIN LATERAL ( SELECT count(*) AS quantos
           FROM documento_itens di
          WHERE di.documento_id = d.id) i ON true
     LEFT JOIN documentos o ON o.id = d.duplicada_de;

insert into schema_migrations (versao, arquivo)
values ('022', '022_desconto_autorizado.sql')
on conflict (versao) do nothing;

-- >>>>>>>>>>>>>>>>>>>>>>>>>>  023_faturamento_direto.sql  <<<<<<<<<<<<<<<<<<<<<<<<<<

-- 023 — faturamento direto: a nota que o fornecedor cobra do cliente
--
-- O QUE ESTA FILA É
--
--   Há notas que o fornecedor fatura DIRETO ao cliente. Elas passam por nós só
--   para controle e estatística: nós lançamos o custo no Trílogo, porque o
--   chamado precisa saber quanto custou — e nunca as cobramos, porque não é
--   nosso o dinheiro que entra.
--
--   Por isso elas não têm orçamento no sentido comum. Não levam margem, não
--   passam pelo teto, não são rateadas e não entram em espelho de faturamento.
--   O que sobe para o Trílogo é o ARQUIVO ORIGINAL da nota, e o valor é o dela,
--   limpo.
--
-- POR QUE UMA TERCEIRA FILA, E NÃO UMA COLUNA NA TABELA DE SEMPRE
--
--   `documentos.fila` já separava `orcamento` de `rateio`, e tudo o que filtra
--   nota no sistema — o painel, a geração, as correções — filtra por ela. Uma
--   coluna nova ao lado exigiria lembrar de acrescentá-la em cada um desses
--   lugares, e o dia em que alguém esquecesse de um seria o dia em que uma nota
--   de faturamento direto entraria num espelho de cobrança.
--
--   Com a fila, quem não a conhece simplesmente não a vê. É o comportamento
--   certo por omissão, e não por vigilância.

alter table documentos drop constraint if exists documentos_fila_check;
alter table documentos add constraint documentos_fila_check
  check (fila = any (array['orcamento'::text, 'rateio'::text, 'direto'::text]));

comment on column documentos.fila is
  'orcamento = vira orçamento nosso · rateio = uma nota, vários chamados · '
  'direto = o fornecedor fatura ao cliente; nós só lançamos o custo e NUNCA cobramos.';

-- ---------------------------------------------------------------------------
-- o custo lançado, do lado dos orçamentos
-- ---------------------------------------------------------------------------
--
-- Ele mora em `orcamentos` porque a fila de lançamento é essa, e duplicar a
-- máquina de lançar para uma variante seria manter duas rotinas que sobem
-- arquivo para o Trílogo — e uma delas envelheceria.
--
-- ⚠ A COLUNA É O QUE O MANTÉM FORA DA COBRANÇA
--   `fatura_id is null` era o único critério da fila de faturar. Uma nota de
--   faturamento direto tem `fatura_id` nulo para sempre — ela entraria em todo
--   espelho, todo mês, e seria cobrada de um cliente que já pagou o fornecedor.
alter table orcamentos
  add column if not exists faturamento_direto boolean not null default false;

comment on column orcamentos.faturamento_direto is
  'Custo de nota faturada DIRETO ao cliente pelo fornecedor. Lançamos no Trílogo '
  'com o arquivo original e o valor limpo, e NUNCA entra em faturamento.';

-- O índice parcial da fila de faturar precisa excluí-las também: sem isso o
-- índice continuaria apontando para linhas que a consulta nunca mais quer.
drop index if exists orcamento_a_faturar;
create index if not exists orcamento_a_faturar
  on orcamentos (cliente_id, criado_em)
  where fatura_id is null and status <> 'removido' and faturamento_direto = false;

create index if not exists orcamento_direto
  on orcamentos (cliente_id, criado_em)
  where faturamento_direto = true;

-- ---------------------------------------------------------------------------
-- o painel: a fila de faturar passa a excluí-las, e a nova ganha contador
-- ---------------------------------------------------------------------------
--
-- ⚠ As 18 colunas abaixo são a definição em produção, VERBATIM e na ordem.
--    `create or replace view` só acrescenta no fim. O que MUDA são três
--    expressões — isso o Postgres permite, e é o ponto desta migração:
--
--    `a_faturar` e `valor_a_faturar`  passam a exigir faturamento_direto = false
--    `sem_associacao`                 deixa de olhar a fila `direto`, onde
--                                     ticket sem chamado é o esperado: não
--                                     conferimos o Trílogo nessas, por decisão.

create or replace view orcamentos_painel as
 SELECT id AS cliente_id,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'orcamento'::text AND d.oculto_em IS NULL) AS notas_arquivos,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'rateio'::text AND d.oculto_em IS NULL AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS rateio_sem_ticket,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text) AS a_lancar,
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND d.status = 'lido'::text AND d.fila = 'orcamento'::text AND NOT (EXISTS ( SELECT 1
                   FROM documento_tickets t
                  WHERE t.documento_id = d.id))) AS sem_ticket,
    ( SELECT count(*) AS count
           FROM documento_tickets t
             JOIN documentos d ON d.id = t.documento_id
          WHERE d.cliente_id = cl.id AND d.oculto_em IS NULL AND t.chamado_id IS NULL
            AND d.fila <> 'direto'::text) AS sem_associacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'aguardando_aprovacao'::text) AS aguardando_aprovacao,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'removido'::text) AS apagados,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS no_total,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text) AS valor_total,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND array_length(dl.ticket_soltos, 1) > 0) AS notas_travadas,
    ( SELECT count(*) AS count
           FROM documentos_lista dl
          WHERE dl.cliente_id = cl.id AND dl.oculto_em IS NULL AND dl.pronto_para_gerar) AS prontas_para_gerar,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status = 'gerado'::text AND o.lancamento_bloqueio IS NOT NULL) AS recusados,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'pode_lancar'::text) AS prontos_para_lancar,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'cliente'::text) AS esperando_cliente,
    ( SELECT count(*) AS count
           FROM orcamentos_lista l
          WHERE l.cliente_id = cl.id AND l.destino = 'encarregados'::text) AS esperando_equipe,
    ( SELECT count(*) AS count
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL
            AND o.faturamento_direto = false) AS a_faturar,
    ( SELECT COALESCE(sum(o.valor), 0::numeric) AS "coalesce"
           FROM orcamentos o
          WHERE o.cliente_id = cl.id AND o.status <> 'removido'::text AND o.fatura_id IS NULL
            AND o.faturamento_direto = false) AS valor_a_faturar,
    -- daqui para baixo, o que a 023 acrescenta
    ( SELECT count(*) AS count
           FROM documentos d
          WHERE d.cliente_id = cl.id AND d.fila = 'direto'::text AND d.oculto_em IS NULL) AS notas_direto
   FROM clientes cl;

insert into schema_migrations (versao, arquivo)
values ('023', '023_faturamento_direto.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- nenhuma nota de faturamento direto pode aparecer na fila de faturar:
--   select count(*) from orcamentos
--    where fatura_id is null and status <> 'removido' and faturamento_direto;
--   -> o número existe, mas ele NÃO entra em a_faturar:
--   select a_faturar from orcamentos_painel limit 1;
--
--   -- e a fila nova aceita o valor novo:
--   insert into documentos (cliente_id, fila, nome_arquivo, arquivo_sha256, status)
--   values ('<cliente>', 'direto', 'teste.pdf', repeat('0',64), 'inserido');
--   -> aceita (e depois apague este teste)
-- =============================================================================

-- >>>>>>>>>>>>>>>>>>>>>>>>>>  024_pedido_de_faturamento.sql  <<<<<<<<<<<<<<<<<<<<<<<<<<

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

-- =============================================================================
-- CONFERÊNCIA — rode isto DEPOIS, e espere ver 5 linhas
-- =============================================================================
-- select versao, arquivo from schema_migrations
--  where versao in ('020','021','022','023','024') order by versao;
--
-- E as quatro views recriadas têm que responder:
-- select count(*) from documentos_lista;
-- select count(*) from orcamentos_lista;
-- select count(*) from orcamentos_painel;
-- select count(*) from pedidos_faturamento_lista;
-- =============================================================================
