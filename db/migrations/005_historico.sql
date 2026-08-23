-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 005: histórico
--
-- O QUE FAZ
--   Cria UMA tabela de histórico para o sistema inteiro, e a tranca contra
--   alteração e exclusão.
--
-- POR QUE UMA SÓ, E NÃO UMA POR MÓDULO
--   Porque histórico é sempre a mesma pergunta: quem fez, quando, com o quê, e o
--   que mudou. Uma tabela por módulo significaria repetir a mesma estrutura, o
--   mesmo código de gravação e a mesma tela de consulta a cada módulo novo.
--   Com uma tabela só, o próximo módulo declara a tenet dele e já grava.
--
--   Cuidado para não confundir: a tabela existir NÃO obriga ninguém a gravar.
--   Quem obriga é a tenet do módulo. Hoje só existe uma: MOD-USUARIOS-01.
--
-- ESTA TABELA NÃO ENTRA NO CATÁLOGO DE ROTINAS
--   Consultar histórico de login é coisa de builder, e builder não passa pela
--   matriz. Quando um módulo comum precisar mostrar histórico, a rotina dele é
--   que carrega a permissão.
--
-- DEPENDE DE: 003 (clientes), 004 (perfis com cliente_id)
-- -----------------------------------------------------------------------------

begin;

-- -----------------------------------------------------------------------------
-- 1. A tabela
-- -----------------------------------------------------------------------------
create table historico (
  -- Contador simples, e não uuid, de propósito: a tabela é só-inserção, ninguém
  -- precisa adivinhar um id daqui, e um bigint ocupa metade de um uuid. Numa
  -- tabela que só cresce, essa metade importa (CORE-01).
  id bigint generated always as identity primary key,

  -- De quem é o dado. Esta é a ÚNICA chave estrangeira da tabela, e ela fica
  -- porque só bloqueia — nunca altera a linha de histórico. Apagar uma empresa
  -- não pode apagar o rastro do que foi feito nela (CORE-11).
  cliente_id uuid not null references clientes(id) on delete restrict,

  -- Que parte do sistema gerou a linha. 'usuarios', e amanhã outros.
  modulo text not null check (modulo <> ''),

  -- Sobre QUEM/O QUÊ é o evento. Não tem chave estrangeira, e não pode ter:
  -- a coluna aponta ora para um login, ora para um serviço, ora para um
  -- contrato. É o preço de a tabela ser uma só, e é um preço consciente.
  registro_id uuid not null,

  -- O que aconteceu. Texto livre e não lista fechada, também de propósito: uma
  -- lista fechada obrigaria uma migration a cada verbo novo de cada módulo novo.
  -- Os verbos de cada módulo ficam escritos na tenet dele.
  acao text not null check (acao <> ''),

  -- Quem fez. SEM chave estrangeira, e isso foi aprendido no teste: com um
  -- "on delete set null" apontando para `perfis`, apagar um login fazia o banco
  -- tentar ALTERAR as linhas de histórico dele — e a tranca de imutabilidade
  -- (item 2) recusava. Resultado: quem já tinha feito alguma coisa no sistema
  -- não podia mais ser removido, sem que a mensagem de erro explicasse por quê.
  --
  -- A conclusão é mais geral do que o remendo: histórico é retrato do passado,
  -- não relação viva. Retrato não se atualiza quando o retratado muda. Por isso
  -- aqui só entra o número, e o `registro_id` acima segue a mesma regra.
  autor_id uuid not null,

  -- E o rótulo fica congelado no momento do evento. Se a pessoa for renomeada
  -- em 2028, o evento de 2026 continua dizendo o nome de 2026. Histórico que se
  -- atualiza sozinho mente sem avisar.
  autor_usuario text not null,

  quando timestamptz not null default now(),

  -- Só o que MUDOU, no formato {"campo": {"de": ..., "para": ...}}.
  -- Guardar a linha inteira a cada edição incharia a tabela e esconderia a
  -- informação útil no meio do resto (CORE-01).
  -- Fica nulo quando não há o que mostrar — troca de senha, por exemplo.
  mudancas jsonb
);

comment on table historico is
  'Rastro de tudo que os módulos declaram registrar. Só-inserção: ver trigger abaixo.';
comment on column historico.registro_id is
  'O registro afetado. Sem chave estrangeira porque aponta para tabelas diferentes conforme o módulo.';
comment on column historico.autor_id is
  'Quem executou. Sem chave estrangeira de propósito: histórico é retrato, não relação viva.';
comment on column historico.autor_usuario is
  'Nome de quem executou, congelado no momento do evento. Não atualizar nunca.';


-- -----------------------------------------------------------------------------
-- 2. A tranca
--
-- A tenet diz que histórico não se edita e não se apaga. Isso podia ficar só na
-- disciplina de quem escreve o código — mas regra que depende de lembrança um dia
-- é esquecida. Aqui ela é do banco: nem o motor, que usa a chave de serviço e
-- passa por cima das políticas, consegue furar.
--
-- São dois gatilhos porque TRUNCATE não dispara gatilho de linha. Sem o segundo,
-- um único comando limparia a tabela inteira sem esbarrar em nada.
-- -----------------------------------------------------------------------------
create or replace function historico_somente_insercao()
returns trigger language plpgsql as $$
begin
  raise exception
    'Histórico é só-inserção: % foi recusado. Corrigir histórico é reescrever o passado; se a linha está errada, grave o evento correto por cima.', tg_op;
end;
$$;

create trigger historico_imutavel
  before update or delete on historico
  for each row execute function historico_somente_insercao();

create trigger historico_imutavel_em_massa
  before truncate on historico
  for each statement execute function historico_somente_insercao();


-- -----------------------------------------------------------------------------
-- 3. Índice
--
-- Um só, e ele serve a pergunta que a tela realmente faz: "o histórico DESTE
-- registro, do mais novo para o mais velho". Índice é disco e é custo em toda
-- gravação; o segundo entra quando existir a tela que precisa dele (CORE-01).
-- -----------------------------------------------------------------------------
create index historico_por_registro
  on historico (cliente_id, modulo, registro_id, quando desc);


-- -----------------------------------------------------------------------------
-- 4. Fechadura
--
-- RLS ligada e NENHUMA política: o navegador não lê esta tabela de jeito nenhum.
-- Quem entrega histórico é o motor, depois de conferir quem está pedindo. É mais
-- apertado do que o resto do sistema de propósito — histórico é o lugar onde se
-- vê o que os outros fizeram, e isso não se abre por descuido (CORE-07).
-- -----------------------------------------------------------------------------
alter table historico enable row level security;


insert into schema_migrations (versao, arquivo) values ('005', '005_historico.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop trigger if exists historico_imutavel_em_massa on historico;
-- drop trigger if exists historico_imutavel on historico;
-- drop function if exists historico_somente_insercao();
-- drop table if exists historico;
-- delete from schema_migrations where versao = '005';
