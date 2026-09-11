-- =============================================================================
-- 061 — PCO: destinatários do e-mail, e as duas rotinas do envio       rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (11/09/2026)
--
--   Envio do pacote de PCO por e-mail (Brevo), substituindo o envio manual
--   pelo Outlook que a skill `pco-organizer` fazia. Duas peças de permissão
--   SEPARADAS, e o dono foi explícito sobre isso: editar QUEM recebe o
--   e-mail é uma coisa; APERTAR O BOTÃO de enviar é outra. Um comprador pode
--   precisar enviar sem poder trocar os destinatários fixos do cliente.
--
-- POR QUE `pco_destinatarios` É TABELA, NÃO UMA LINHA EM `parametros`
--
--   `parametros` (migração 010) só guarda `numeric` — margem, teto, folga.
--   Uma lista de e-mails não cabe ali sem forçar o tipo da coluna para
--   texto/jsonb, o que quebraria o contrato numérico que as outras três
--   chaves já têm. Uma tabela própria, um e-mail por linha, é o desenho que
--   já prova certo no resto do sistema (fornecedores, categoria_permissoes)
--   — e permite ativar/desativar um destinatário sem apagar o registro
--   (CORE-05).
--
-- POR QUE NÃO TEM COLUNA "papel" (para/cc)
--
--   O dono pediu uma lista só, editável — não duas listas. Todo
--   destinatário ativo entra no "Para" do e-mail. Se um dia precisar de
--   Para/Cc separados, é uma coluna nova, não uma migração desfeita.
--
-- O SEGUNDO DESTINATÁRIO NASCE JÁ CADASTRADO
--
--   `ia.frotamacedo@gmail.com`, pedido explícito do dono para o início —
--   antes de apontar para o e-mail real do cliente, o envio já sai
--   confirmável por alguém do time.
-- =============================================================================

create table public.pco_destinatarios (
  id          uuid primary key default gen_random_uuid(),
  cliente_id  uuid not null references public.clientes(id),
  email       text not null,
  ativo       boolean not null default true,
  criado_em   timestamptz not null default now(),
  criado_por  uuid references public.perfis(id) on delete set null,

  unique (cliente_id, email)
);

comment on table public.pco_destinatarios is
  'Quem recebe o e-mail do pacote de PCO. Um e-mail por linha, ativo/inativo '
  '(CORE-05: tirar de circulação é marcar, não apagar) — nunca DELETE de '
  'verdade, para o histórico de quem cadastrou continuar de pé.';

alter table public.pco_destinatarios enable row level security;

create policy "destinatarios do meu cliente" on public.pco_destinatarios for select
  using (cliente_id = meu_cliente_id() and posso('COMPRAS_PCO_DESTINATARIOS'));

-- O destinatário inicial, pedido explícito do dono (11/09/2026): antes de
-- apontar para o e-mail real do cliente, alguém do time confirma o envio.
insert into public.pco_destinatarios (cliente_id, email)
select id, 'ia.frotamacedo@gmail.com' from public.clientes
on conflict (cliente_id, email) do nothing;

-- -----------------------------------------------------------------------------
-- as duas rotinas — separadas de propósito (ver cabeçalho)
-- -----------------------------------------------------------------------------

insert into public.rotinas (codigo, nome, modulo, ordem) values
  ('COMPRAS_PCO_DESTINATARIOS', 'PCO — editar os destinatários do e-mail', 'compras', 2),
  ('COMPRAS_PCO_ENVIAR', 'PCO — enviar o pedido ao cliente', 'compras', 3);

-- Permissão de saída: só CEO, mesmo padrão da 059 — quem for enviar/editar
-- destinatários no dia a dia precisa ganhar a rotina pela tela de Acesso.
insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo = 'ceo' and r.codigo in ('COMPRAS_PCO_DESTINATARIOS', 'COMPRAS_PCO_ENVIAR');

insert into schema_migrations (versao, arquivo)
values ('061', '061_pco_destinatarios_e_envio.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select relname, relrowsecurity from pg_class where relname = 'pco_destinatarios';
--   -- esperado: relrowsecurity = true
--
--   select email, ativo from pco_destinatarios;
--   -- esperado: 1 linha, ia.frotamacedo@gmail.com, ativo = true
--
--   select r.codigo, cp.pode, c.codigo as categoria
--     from rotinas r join categoria_permissoes cp on cp.rotina = r.codigo
--     join categorias c on c.id = cp.categoria_id
--    where r.codigo in ('COMPRAS_PCO_DESTINATARIOS','COMPRAS_PCO_ENVIAR');
--   -- esperado: 2 linhas, categoria 'ceo', pode = true
--
-- PARA DESFAZER
--   delete from categoria_permissoes where rotina in ('COMPRAS_PCO_DESTINATARIOS','COMPRAS_PCO_ENVIAR');
--   delete from rotinas where codigo in ('COMPRAS_PCO_DESTINATARIOS','COMPRAS_PCO_ENVIAR');
--   drop table public.pco_destinatarios;
--   delete from schema_migrations where versao = '061';
-- =============================================================================
