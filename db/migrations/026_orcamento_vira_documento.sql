-- =============================================================================
-- 026 - o orcamento vira documento                                       rev 1
-- =============================================================================
--
-- O QUE ESTAVA ERRADO
--
--   O PDF do orcamento reusava `relatorio.Tabela`, a mesma listagem que gera as
--   extracoes de planilha. Saia em A4 deitada, com cabecalho de relatorio e
--   rodape "gerado em" - um despejo de dados no lugar de um documento que vai
--   ao cliente. O sistema antigo entregava capa com marca, dados do prestador e
--   do tomador, valor por extenso, observacoes e linha de aceite.
--
-- POR QUE UMA TABELA, E NAO CONSTANTES NO CODIGO
--
--   Razao social, CNPJ, endereco e forma de pagamento mudam sem que ninguem
--   avise o programador - mudam por decisao de escritorio. Presas no binario,
--   cada troca vira commit, build e deploy para corrigir um telefone.
--
--   E sao POR CLIENTE. Hoje ha um so, e a tentacao e fixar tudo; amanha o mesmo
--   motor emite para outro contratante e o documento sairia com o CNPJ errado -
--   o tipo de erro que ninguem ve ate chegar na contabilidade de terceiros.
--
-- A MARCA MORA AQUI TAMBEM
--
--   E um JPEG de 3,8 KB guardado em base64. Poderia ir embutida no binario com
--   `go:embed`, e seria menos linha - mas ai a marca seria a mesma para todo
--   cliente, e troca-la exigiria recompilar. Guardada aqui, o documento de cada
--   contratante sai com a marca dele.
-- =============================================================================

create table if not exists emitente (
  cliente_id        uuid primary key references clientes(id) on delete cascade,
  razao_social      text not null,
  cnpj              text not null,
  endereco          text not null,
  contato           text not null,
  forma_pagamento   text not null,
  validade_dias     integer not null default 7,
  observacoes       text[] not null default '{}',
  -- A marca em base64. `text` e nao `bytea` porque quem le e o PostgREST, que
  -- devolveria bytea como hexadecimal escapado - mais bytes e mais conversao
  -- para o mesmo fim.
  marca_jpeg_base64 text,
  atualizado_em     timestamptz not null default now()
);

comment on table emitente is
  'Quem assina o orçamento: os dados de capa do documento que vai ao cliente.';

insert into emitente (
  cliente_id, razao_social, cnpj, endereco, contato, forma_pagamento,
  validade_dias, observacoes, marca_jpeg_base64
)
select c.id,
  'FROTA MACEDO ENGENHARIA LTDA',
  '27.363.223/0001-70',
  'Eng. Heitor de Oliveira Albuquerque, 295 — Cidade dos Funcionários — Fortaleza/CE',
  '(85) 2181-1386 - frotamacedoengenharia@gmail.com',
  'Transferência Bancária 30 dias',
  7,
  array[
    'Orçamento válido por 7 (sete) dias corridos a partir da data de emissão.',
    'Os valores acima incluem material e serviço de entrega.'
  ],
  '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'
from clientes c
on conflict (cliente_id) do nothing;

insert into schema_migrations (versao, arquivo)
values ('026', '026_orcamento_vira_documento.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select razao_social, cnpj, validade_dias,
--          length(marca_jpeg_base64) as marca_bytes,
--          array_length(observacoes, 1) as observacoes
--     from emitente;
--
-- PARA DESFAZER
--   drop table emitente;
--   delete from schema_migrations where versao = '026';
-- =============================================================================
