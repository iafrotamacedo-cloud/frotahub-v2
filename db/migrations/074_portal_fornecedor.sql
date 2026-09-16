-- =============================================================================
-- 074 — o portal do fornecedor: só enviar nota/DAV, mais nada             rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (16/09/2026)
--
--   Um acesso pro PRÓPRIO fornecedor inserir as notas/DAVs que hoje só a
--   equipe interna lança em Manutenção › Contrato São Luiz › Orçamentos ›
--   "Notas e DAVs" (`CONTRATO_ORCAMENTOS_NOTAS`, migração 010). "O mais
--   simples possível": login entra direto na tela de enviar, sem menu, sem
--   lista, sem status do que já mandou — só um lugar de trocar a própria
--   senha, em Configurações › Minha conta (que já existe e não depende de
--   menu nenhum, `MinhaConta.tsx`).
--
-- POR QUE UM NÍVEL NOVO, E NÃO UMA CATEGORIA OPERACIONAL COMUM
--
--   Os cinco níveis de hoje (066_niveis_gerencial_supervisorio.sql) são
--   TODOS internos e formam uma cadeia de controle — cada um responde a
--   alguém acima, via vínculo hierárquico (068). O fornecedor não é gente da
--   casa: não responde a ninguém na hierarquia, e ninguém precisa responder
--   por ele. Encaixar como "operacional" ia forçá-lo pra dentro de uma
--   cadeia que não é dele. `fornecedor` fica de fora de propósito —
--   `nivelEncaixaAbaixoDe` (hierarquia.go) não reconhece este nível, então
--   ele nunca entra em vínculo hierárquico nenhum.
--
-- POR QUE UMA ROTINA NOVA, E NÃO REUSAR `CONTRATO_ORCAMENTOS_NOTAS`
--
--   Essa rotina de hoje libera LISTAR e VER a fila inteira — de todos os
--   fornecedores, não só de quem está logado (o motor não sabe "de quem" é
--   um documento, só "quem alcança a rotina"). Dar essa rotina ao fornecedor
--   deixaria ele ver e apagar os DAVs de QUALQUER outro fornecedor. A rotina
--   nova, `CONTRATO_ORCAMENTOS_NOTAS_FORNECEDOR`, só é aceita em UM lugar do
--   motor — `POST /orcamentos/documentos` — e só para a fila "orcamento"
--   (ver `quemInsereDocumento`, orcamentos/documentos.go). Ela nunca destrava
--   listar, ver, apagar ou gerar orçamento.
--
-- O QUE ESTA MIGRAÇÃO NÃO FAZ
--
--   Não cria a categoria "Fornecedor" nem nenhum login — ela não sabe quem é
--   o primeiro fornecedor a entrar. Isso é trabalho do Builder (ou do CEO,
--   que também pode criar categoria não-CEO) pela tela de Categorias, depois
--   que esta migração estiver no ar.
--
-- REGRA DO POSTGRES QUE ESTA MIGRAÇÃO RESPEITA (mesma nota da 066)
--
--   Um valor de enum recém-criado (`ADD VALUE`) não pode ser usado em
--   `insert`/`update` na MESMA transação em que nasceu. Esta migração não usa
--   `'fornecedor'` em nenhum insert/update — só cria o rótulo. Os inserts
--   abaixo (`rotinas`, `categoria_permissoes`) não têm nada a ver com o enum
--   `nivel_acesso`, então não esbarram nessa regra.
-- =============================================================================

alter type public.nivel_acesso add value 'fornecedor';

-- -----------------------------------------------------------------------------
-- Catálogo de rotinas — uma só, estreita de propósito (ver o cabeçalho acima).
-- 'manutencao' porque é onde "Notas e DAVs" mora hoje (mesmo modulo_menu da
-- 010/067) — não é um módulo de 1º nível novo, como Locações foi na 072.
-- -----------------------------------------------------------------------------

insert into public.rotinas (codigo, nome, modulo, modulo_menu, ordem) values
  ('CONTRATO_ORCAMENTOS_NOTAS_FORNECEDOR', 'Fornecedor — enviar nota/DAV (portal externo)', 'manutencao', 'manutencao', 329)
on conflict (codigo) do nothing;

-- Permissão de saída: só CEO, mesmo padrão da 059/072 — a categoria
-- "Fornecedor" ainda não existe, então é o Builder/CEO quem marca esta
-- rotina nela depois, pela tela de Acesso.
insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo = 'ceo' and r.codigo = 'CONTRATO_ORCAMENTOS_NOTAS_FORNECEDOR';

insert into schema_migrations (versao, arquivo)
values ('074', '074_portal_fornecedor.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--   select unnest(enum_range(null::nivel_acesso))::text;
--   -- espera: builder, ceo, gerencial, supervisorio, operacional, fornecedor
--   select codigo, nome, modulo_menu, ordem from rotinas
--     where codigo = 'CONTRATO_ORCAMENTOS_NOTAS_FORNECEDOR';
--
-- DEPOIS DESTA MIGRAÇÃO (fora do banco, pela tela)
--   1. Categorias → criar "Fornecedor", nível "fornecedor".
--   2. Permissões dessa categoria → marcar
--      "Fornecedor — enviar nota/DAV (portal externo)".
--   3. Usuários e Logins → criar o login do fornecedor nessa categoria.
--
-- PARA DESFAZER
--   delete from public.categoria_permissoes where rotina = 'CONTRATO_ORCAMENTOS_NOTAS_FORNECEDOR';
--   delete from public.rotinas where codigo = 'CONTRATO_ORCAMENTOS_NOTAS_FORNECEDOR';
--   -- 'fornecedor' NÃO tem como ser removido do enum (Postgres não tem
--   -- `DROP VALUE`) — inofensivo enquanto nenhuma categoria usar.
--   delete from public.schema_migrations where versao = '074';
-- =============================================================================
