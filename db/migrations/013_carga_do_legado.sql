-- =============================================================================
-- 013 — o que o modelo precisa para receber o legado                     rev 3
-- =============================================================================
--
-- Esta migration não move dado nenhum. Ela conserta três coisas que a 010
-- deixou passar e acrescenta a única peça que faltava para a carga do sistema
-- antigo ser REPETÍVEL.
--
--   1. `legado_id`        — a chave de idempotência que não existia
--   2. `leitura_camada`   — passa a aceitar 'legado'
--   3. o `unique` de ticket+parte vira PARCIAL  (bug vivo, não só da carga)
--   4. o índice de `orcamento_documentos` passa a impor o que prometia
--   5. o livro de migrations volta a bater com o banco
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. legado_id — como a carga sabe que já carregou
--
-- POR QUE NÃO DAVA PARA USAR O QUE JÁ EXISTIA
--
--   A trava de duplicidade da 010 é a chave de acesso da NFe. Ela é perfeita
--   para o que entra pelo site a partir de hoje — e inútil para o legado: as
--   linhas antigas não têm chave de acesso, a coluna nem existe no banco velho.
--
--   E, por decisão do dono (ago/2026), as compras da Rodrigues entram SEM
--   número de nota e SEM número de DAV. Some também o último candidato a chave
--   natural. Sem isto aqui, rodar a carga duas vezes duplica tudo.
--
--   O valor guardado é o `notas_orcamento.id` do sistema antigo — que lá é a
--   chave real do sistema, por decisão explícita: o ticket muda quando uma nota
--   sem-ticket é corrigida, o id não.
--
-- POR QUE UNIQUE POR CLIENTE, E NÃO GLOBAL
--
--   Dois clientes podem, em tese, ter legados independentes com ids que colidem.
--   O escopo do id é o cliente, então a trava também é.
-- -----------------------------------------------------------------------------
alter table documentos add column if not exists legado_id uuid;
alter table orcamentos add column if not exists legado_id uuid;

comment on column documentos.legado_id is
  'id da linha correspondente em notas_orcamento, no sistema antigo. Nulo em tudo que nasceu aqui.';
comment on column orcamentos.legado_id is
  'id da linha correspondente em notas_orcamento, no sistema antigo. É isto que faz a carga poder rodar de novo sem duplicar.';

create unique index if not exists documento_legado_unico
  on documentos (cliente_id, legado_id) where legado_id is not null;
create unique index if not exists orcamento_legado_unico
  on orcamentos (cliente_id, legado_id) where legado_id is not null;

-- O caminho velho do arquivo, preservado como estava. Não é para o programa
-- usar: é para uma pessoa conseguir achar o original no Dropbox se um dia
-- precisar conferir. O programa usa o sha256, que aponta para o R2.
alter table documentos add column if not exists legado_caminho text;
alter table orcamentos add column if not exists legado_caminho text;


-- -----------------------------------------------------------------------------
-- 2. leitura_camada aceita 'legado'
--
--   Chamar de 'manual' um dado que veio de uma carga automática seria mentir
--   sobre a origem — e a origem é justamente o que decide quanta confiança a
--   tela deve mostrar. 'legado' quer dizer: veio do sistema antigo, foi lido lá
--   por um caminho que não existe mais aqui, e ninguém reconferiu.
-- -----------------------------------------------------------------------------
alter table documentos drop constraint if exists documentos_leitura_camada_check;
alter table documentos add constraint documentos_leitura_camada_check
  check (leitura_camada in ('xml', 'texto', 'ocr', 'ia', 'manual', 'legado'));


-- -----------------------------------------------------------------------------
-- 3. ticket + parte: o unique vira PARCIAL
--
-- ISTO É BUG VIVO, NÃO É DETALHE DA CARGA
--
--   Do jeito que a 010 deixou, um orçamento REMOVIDO continua ocupando o par
--   (ticket, parte). Consequência prática: excluir um orçamento e gerar de novo
--   o mesmo ticket na mesma parte é impossível — e esse é exatamente o fluxo de
--   excluir/restaurar da Auditoria. O soft-delete existia, mas não liberava o
--   lugar.
--
--   No legado o problema é medido: 37 grupos, 82 linhas, com ticket+parte
--   repetido entre ativos e removidos. A carga de hoje não esbarra nisso porque
--   só traz lançados — mas a trava estaria errada do mesmo jeito.
-- -----------------------------------------------------------------------------
alter table orcamentos drop constraint if exists orcamentos_cliente_id_ticket_parte_key;

create unique index if not exists orcamento_ticket_parte_vivo
  on orcamentos (cliente_id, ticket, parte)
  where status <> 'removido';

comment on index orcamento_ticket_parte_vivo is
  'Removido não ocupa lugar: apagar um orçamento libera o par ticket+parte para ser gerado de novo.';


-- -----------------------------------------------------------------------------
-- 4. orcamento_documentos: o índice passa a impor o que prometia
--
-- O QUE ESTAVA ERRADO
--
--   A 010 criou `orcamento_nota_por_ticket` em (documento_id, orcamento_id) e
--   escreveu por cima o comentário "a mesma nota não vira orçamento duas vezes
--   no mesmo ticket". Só que a chave primária já é (orcamento_id, documento_id)
--   — o mesmo conjunto de colunas. O índice era uma cópia da PK com os campos
--   trocados de ordem: não impedia nada. A garantia anunciada nunca existiu.
--
-- POR QUE PRECISA DA COLUNA `ticket` AQUI
--
--   A regra fala de TICKET, e ticket mora em `orcamentos`. Índice único não
--   enxerga outra tabela. Então o ticket é copiado para cá, por gatilho — o
--   valor nunca é digitado por quem chama.
--
-- POR QUE SÃO DOIS GATILHOS, E NÃO UM
--
--   O primeiro carimba na hora de ligar a nota ao orçamento. Sozinho, ele
--   deixaria um furo: se alguém corrigir o ticket do orçamento depois, a cópia
--   daqui envelhece e o índice passa a proteger um ticket que não existe mais.
--   Isso foi medido num Postgres de verdade antes desta migration sair — o
--   update foi aceito e a linha ficou apontando para o ticket velho.
--
--   E ticket TROCA: é assim que uma nota que entrou sem ticket é corrigida.
--   Então o segundo gatilho propaga a correção para cá. Se a correção criar
--   duas vezes a mesma nota no mesmo ticket, o índice recusa o update inteiro —
--   que é exatamente o que se quer.
-- -----------------------------------------------------------------------------
drop index if exists orcamento_nota_por_ticket;

alter table orcamento_documentos add column if not exists ticket integer;

create or replace function orcamento_documento_carimba_ticket()
returns trigger language plpgsql as $$
begin
  select o.ticket into new.ticket from orcamentos o where o.id = new.orcamento_id;
  return new;
end;
$$;

drop trigger if exists carimba_ticket on orcamento_documentos;
create trigger carimba_ticket
  before insert or update of orcamento_id on orcamento_documentos
  for each row execute function orcamento_documento_carimba_ticket();

create or replace function orcamento_propaga_ticket()
returns trigger language plpgsql as $$
begin
  update orcamento_documentos set ticket = new.ticket where orcamento_id = new.id;
  return null;
end;
$$;

drop trigger if exists propaga_ticket on orcamentos;
create trigger propaga_ticket
  after update of ticket on orcamentos
  for each row when (old.ticket is distinct from new.ticket)
  execute function orcamento_propaga_ticket();

-- preenche o que já existe (hoje: nada, mas a migration precisa ser correta
-- se um dia rodar num banco que já tem linhas)
update orcamento_documentos od
   set ticket = o.ticket
  from orcamentos o
 where o.id = od.orcamento_id and od.ticket is distinct from o.ticket;

create unique index if not exists orcamento_nota_por_ticket
  on orcamento_documentos (documento_id, ticket);

comment on index orcamento_nota_por_ticket is
  'A MESMA nota não vira orçamento duas vezes no MESMO ticket. Agora é índice de verdade, não comentário.';


-- -----------------------------------------------------------------------------
-- 5. schema_migrations volta a bater com o banco
--
--   A 009, a 010 e a 011 rodaram, mas não se registraram: o livro pula de 008
--   para 012. Um livro de migrations com buraco é pior do que não ter livro —
--   ele diz que falta aplicar coisa que já está aplicada, e quem confiar nele
--   vai rodar de novo.
--
--   Isto não é chute. Antes de escrever estas três linhas, cada objeto que as
--   três migrations criam foi procurado no banco e encontrado: as 8 tabelas da
--   010, as views `chamados_lista`, `documentos_lista` e `orcamentos_painel`,
--   e os 6 índices da 009. Estão todos lá. O registro é o que estava faltando.
-- -----------------------------------------------------------------------------
insert into schema_migrations (versao, arquivo) values
  ('009', '009_trilogo_tela.sql'),
  ('010', '010_orcamentos.sql'),
  ('011', '011_rateio_travado.sql'),
  ('013', '013_carga_do_legado.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select indexdef from pg_indexes
--    where indexname in ('orcamento_ticket_parte_vivo','orcamento_nota_por_ticket',
--                        'documento_legado_unico','orcamento_legado_unico');
--
--   select pg_get_constraintdef(oid) from pg_constraint
--    where conname = 'documentos_leitura_camada_check';
--
-- PARA DESFAZER
--
--   drop index if exists orcamento_nota_por_ticket, orcamento_ticket_parte_vivo,
--                        documento_legado_unico, orcamento_legado_unico;
--   drop trigger if exists carimba_ticket on orcamento_documentos;
--   drop trigger if exists propaga_ticket on orcamentos;
--   drop function if exists orcamento_documento_carimba_ticket();
--   drop function if exists orcamento_propaga_ticket();
--   alter table orcamento_documentos drop column if exists ticket;
--   alter table documentos  drop column if exists legado_id, drop column if exists legado_caminho;
--   alter table orcamentos  drop column if exists legado_id, drop column if exists legado_caminho;
--   alter table orcamentos  add constraint orcamentos_cliente_id_ticket_parte_key
--                           unique (cliente_id, ticket, parte);
--   delete from schema_migrations where versao = '013';
--   (as linhas 009/010/011 são registro de fato consumado: não desfaça.)
-- =============================================================================
