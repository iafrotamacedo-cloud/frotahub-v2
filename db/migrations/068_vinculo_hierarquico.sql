-- =============================================================================
-- 068 — vinculos_hierarquicos: quem responde pra quem                      rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (14/09/2026)
--
--   Terceira e última peça da hierarquia de 5 níveis (066 renomeou os
--   níveis, 067 deu ao CEO o bypass de módulo). Esta é a que ele mais quer
--   testar: uma pessoa de nível Gerencial gerencia Supervisório, e
--   Supervisório gerencia Operacional — mas não pela categoria inteira de
--   uma vez, por um VÍNCULO entre duas pessoas específicas. Quem configura
--   esse vínculo é só CEO e Builder, numa tela de organograma (frontend,
--   fora desta migração) alcançada por um botão em Usuários e Logins.
--
-- É ÁRVORE, NÃO GRAFO
--
--   Cada perfil tem NO MÁXIMO um superior direto — por isso `unique
--   (perfil_id)`. Sem essa restrição, "quem é o chefe de fulano" deixaria de
--   ter resposta única, e a tela de organograma não saberia qual caminho
--   desenhar.
--
-- O QUE NÃO TEM LINHA AQUI
--
--   Antes de qualquer configuração, o default é "CEO > este login" — mas
--   isso é CALCULADO (`hierarquia.go`, cadeiaDe), não gravado. Gravar um
--   default pra cada login existente encheria a tabela de linhas idênticas
--   que ninguém decidiu de verdade, e o dia em que o CEO mudar (raro, mas
--   possível) exigiria reescrever todas elas em vez de continuarem
--   corretas sozinhas.
--
-- ESTA RODADA SÓ CONSTRÓI O VÍNCULO EM SI
--
--   Usar esse vínculo pra restringir o que aparece em OUTRAS telas (ex: um
--   Gerencial só ver os Supervisórios dele na lista de Usuários) fica pra
--   depois, por pedido explícito — hoje é só o dado e a tela de configurar.
-- =============================================================================

create table public.vinculos_hierarquicos (
  id           uuid primary key default gen_random_uuid(),
  cliente_id   uuid not null references public.clientes(id),
  perfil_id    uuid not null references public.perfis(id) on delete cascade,
  superior_id  uuid not null references public.perfis(id) on delete restrict,
  definido_por uuid references public.perfis(id),
  criado_em    timestamptz not null default now(),
  unique (perfil_id),
  check (perfil_id <> superior_id)
);

comment on table public.vinculos_hierarquicos is
  'O superior DIRETO de cada perfil — no máximo um por perfil (unique
  perfil_id), por isso é árvore e não grafo. Quem não tem linha aqui responde
  implicitamente ao único CEO do cliente (calculado em hierarquia.go,
  cadeiaDe — nunca gravado). Escrita exclusiva do motor: só CEO e Builder
  chegam nas rotas que mexem aqui (permissao.ExigeBuilderOuCEO).';
comment on column public.vinculos_hierarquicos.perfil_id is
  'Quem é gerenciado.';
comment on column public.vinculos_hierarquicos.superior_id is
  'Quem gerencia — o degrau imediatamente acima na cadeia deste perfil.';

alter table public.vinculos_hierarquicos enable row level security;

-- Mesma ideia de `centro_custo_acessos` (064): não é dado de trabalho, é
-- controle de acesso, então só quem administra a hierarquia enxerga — não
-- existe rotina natural pra isso (é conceito de NÍVEL), por isso reaproveita
-- `sou_builder_ou_ceo()` que a 067 já criou.
create policy "builder ou ceo vê os vínculos do cliente"
  on public.vinculos_hierarquicos for select to authenticated
  using (cliente_id = meu_cliente_id() and sou_builder_ou_ceo());

insert into schema_migrations (versao, arquivo)
values ('068', '068_vinculo_hierarquico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select relname, relrowsecurity from pg_class where relname = 'vinculos_hierarquicos';
--   -- espera relrowsecurity = true
--
--   -- ninguém tem vínculo ainda logo após a migração:
--   select count(*) from vinculos_hierarquicos;
--   -- espera 0
--
-- PARA DESFAZER
--   drop table public.vinculos_hierarquicos;
--   delete from schema_migrations where versao = '068';
-- =============================================================================
