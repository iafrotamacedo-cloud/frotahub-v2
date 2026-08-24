-- =============================================================================
-- 009 — a tela "Dados do Trílogo"                                        rev 1
-- =============================================================================
--
-- A 007 criou as tabelas e disse, com todas as letras, que NÃO cadastraria a
-- rotina no catálogo enquanto a tela não existisse — para o catálogo nunca
-- prometer uma porta que não abre. A tela agora existe, e é esta migration que
-- cumpre aquela promessa.
--
-- Três coisas aqui, e só três:
--
--   1. a rotina CONTRATO_TRILOGO_DADOS entra no catálogo
--   2. a visão `chamados_lista` — a forma da lista mora no banco, não no motor
--   3. os índices que a lista vai usar
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. A rotina no catálogo
-- -----------------------------------------------------------------------------
--
-- A partir daqui, `posso('CONTRATO_TRILOGO_DADOS')` para de responder falso para
-- todo mundo: passa a responder o que a matriz de permissões disser. O builder
-- continua passando sempre — é a exceção que impede o dono de se trancar para
-- fora (CORE-08 tem um irmão: na dúvida nega, mas nunca tranque o dono).
--
-- ATENÇÃO, e isto não é detalhe: esta é a PRIMEIRA linha do catálogo. Enquanto
-- ele estava vazio, a tela de Categorias não tinha o que listar e a matriz de
-- permissões aparecia em branco — não estava quebrada, estava vazia. Depois
-- desta migration ela passa a ter uma linha para marcar.

insert into rotinas (codigo, nome, modulo, ordem)
values ('CONTRATO_TRILOGO_DADOS', 'Dados do Trílogo', 'manutencao', 310)
on conflict (codigo) do nothing;


-- -----------------------------------------------------------------------------
-- 2. A visão da lista
-- -----------------------------------------------------------------------------
--
-- POR QUE UMA VISÃO, E NÃO UMA CONSULTA MONTADA NO MOTOR
--
--   A lista precisa de três coisas que não estão na tabela `chamados`: o NOME da
--   loja (está em `unidades`), a SOMA dos custos e a CONTAGEM de anexos. Dá para
--   montar isso no Go, com três consultas e uma costura na memória. Dá também
--   para pedir agregação ao PostgREST, que muda de sintaxe entre versões.
--
--   As duas saídas espalham a mesma regra por dois lugares. A visão põe a forma
--   da lista num lugar só (CORE-06): o motor lê `chamados_lista` e filtra; se
--   amanhã a lista ganhar uma coluna, ela nasce aqui e o motor não muda.
--
-- security_invoker = true — sem isto, a visão rodaria com os poderes de quem a
-- criou e passaria POR CIMA das políticas de linha das tabelas de baixo. Seria
-- um segundo caminho para o dado, sem o filtro do cliente titular (CORE-11).
-- Com invoker, quem lê a visão carrega as próprias permissões.

create or replace view chamados_lista
with (security_invoker = true) as
select
  c.id,
  c.cliente_id,
  c.numero,
  c.unidade_id,
  u.nome            as loja,
  c.conta,
  c.status,
  c.prioridade,
  c.descricao,
  c.ambiente,
  c.responsavel,
  c.criado_em,
  c.prazo,
  coalesce(k.total, 0)::numeric(12,2) as custo_total,
  coalesce(a.quantos, 0)              as anexos
from chamados c
join unidades u on u.id = c.unidade_id
left join lateral (
  select sum(valor) as total from chamado_custos k2 where k2.chamado_id = c.id
) k on true
left join lateral (
  select count(*) as quantos from chamado_anexos a2 where a2.chamado_id = c.id
) a on true;

comment on view chamados_lista is
  'A lista de chamados como a tela precisa dela: com o nome da loja, a soma dos custos e a contagem de anexos já resolvidos. Roda com as permissões de quem lê.';


-- -----------------------------------------------------------------------------
-- 3. Os índices da lista
-- -----------------------------------------------------------------------------
--
-- MEDIDO, NÃO SUPOSTO (24/08/2026, sobre os 1.377 chamados reais):
--
--   sem índice que sirva à ordenação ....... 211 ms
--   com índice que já entrega a ordem ......   8 ms
--
-- E o interessante não é o número, é o PORQUÊ. Sem um índice que já venha
-- ordenado, o banco calcula a soma de custos e a contagem de anexos das 1.377
-- linhas para depois jogar 877 fora. Com o índice, ele lê em ordem, para na
-- linha 500 e faz a conta só dessas — o plano mostra `loops=500` no lugar de
-- `loops=1377`.
--
-- Daí a forma de cada índice: filtro PRIMEIRO, `criado_em desc` DEPOIS. Um
-- índice só em `status` acharia as linhas, mas obrigaria a ordenar tudo de novo,
-- e o trabalho voltaria a crescer com o tamanho da tabela. Do jeito abaixo, o
-- custo da página é o tamanho da página — 500 linhas custam 500 linhas, com
-- 1.377 chamados ou com 50 mil.
--
-- Não crio índice para (cliente_id, numero): a restrição de unicidade da 007 já
-- criou um, e um segundo igual só ocuparia espaço e atrasaria toda escrita.

create index if not exists chamados_cliente_criado
  on chamados (cliente_id, criado_em desc);

create index if not exists chamados_cliente_status_criado
  on chamados (cliente_id, status, criado_em desc);

create index if not exists chamados_cliente_conta_criado
  on chamados (cliente_id, conta, criado_em desc);

create index if not exists chamados_cliente_unidade_criado
  on chamados (cliente_id, unidade_id, criado_em desc);

create index if not exists chamados_cliente_prioridade_criado
  on chamados (cliente_id, prioridade, criado_em desc);

-- A ficha abre a linha do tempo em ordem cronológica. A unicidade da 007 é por
-- (chamado_id, chave), que serve para achar o chamado mas não entrega a ordem.
create index if not exists eventos_do_chamado
  on chamado_eventos (chamado_id, quando);
