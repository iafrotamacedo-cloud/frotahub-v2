-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 004: ligar cada login ao seu cliente e à sua categoria
--
-- O QUE FAZ
--   A `perfis` passa a apontar para um cliente e para uma categoria, e perde a
--   coluna `nivel` — que agora é propriedade da categoria.
--
-- POR QUE O NÍVEL SAI DA PESSOA
--   Enquanto o nível ficava nos dois lugares, eles podiam discordar: alguém na
--   categoria "Administrativo" com nível builder gravado à mão. Com o nível
--   morando só na categoria, isso é impossível por construção (CORE-06).
--
-- ORDEM DAS COISAS
--   Cria a categoria builder → liga os perfis existentes → só então torna as
--   colunas obrigatórias. Nunca o contrário: coluna obrigatória antes de ter
--   valor derruba a migration com o banco pela metade.
--
-- DEPENDE DE: 003
-- -----------------------------------------------------------------------------

begin;

-- 1. As colunas nascem opcionais, para os perfis que já existem não quebrarem.
alter table perfis add column cliente_id   uuid references clientes(id) on delete restrict;
alter table perfis add column categoria_id uuid references categorias(id) on delete restrict;

-- 2. A categoria do builder. Protegida: não se exclui nem se edita permissão dela,
--    porque o builder passa sempre de qualquer forma — a matriz não se aplica.
insert into categorias (cliente_id, codigo, nome, nivel, protegida)
select c.id, 'builder', 'Builder', 'builder', true
from clientes c where c.slug = 'frota-macedo'
on conflict (cliente_id, codigo) do nothing;

-- 3. Liga os perfis que já existem ao cliente e à categoria correspondente ao
--    nível que eles tinham.
update perfis p
   set cliente_id   = c.id,
       categoria_id = cat.id
  from clientes c
  join categorias cat on cat.cliente_id = c.id and cat.codigo = 'builder'
 where c.slug = 'frota-macedo'
   and p.nivel = 'builder';

-- 4. Se sobrou algum perfil sem categoria, a migration PARA aqui em vez de
--    deixar o banco num estado que ninguém consegue explicar depois.
do $$
declare orfaos int;
begin
  select count(*) into orfaos from perfis where cliente_id is null or categoria_id is null;
  if orfaos > 0 then
    raise exception 'Existem % perfis sem cliente ou sem categoria. Resolva antes de continuar.', orfaos;
  end if;
end $$;

-- 5. Agora sim, obrigatórias.
alter table perfis alter column cliente_id   set not null;
alter table perfis alter column categoria_id set not null;

-- 6. O nível sai da pessoa.
alter table perfis drop column nivel;

-- 7. Índices para as buscas que a tela de usuários vai fazer. Existem desde já
--    porque toda lista nasce paginada e filtrada no banco (CORE-10).
create index perfis_por_cliente   on perfis (cliente_id, usuario);
create index perfis_por_categoria on perfis (categoria_id);

comment on column perfis.cliente_id is 'A empresa titular deste login.';
comment on column perfis.categoria_id is 'O grupo do login. O nível vem daqui.';


-- -----------------------------------------------------------------------------
-- 8. Quem sou eu — os atalhos que as políticas usam
--
-- Ficam aqui, e não na 003, porque dependem das colunas criadas acima. Existem
-- para o filtro por cliente ser escrito UMA vez (CORE-06) e valer em toda tabela,
-- em vez de ser repetido em cada política.
-- -----------------------------------------------------------------------------
create or replace function meu_cliente_id()
returns uuid language sql stable
as $$ select cliente_id from perfis where id = auth.uid() $$;

create or replace function minha_categoria_id()
returns uuid language sql stable
as $$ select categoria_id from perfis where id = auth.uid() $$;


-- -----------------------------------------------------------------------------
-- 9. As aberturas
--
-- Todas são de LEITURA. Escrever nestas tabelas é exclusividade do motor, que
-- usa a chave de serviço — o navegador não grava aqui de jeito nenhum.
-- -----------------------------------------------------------------------------

-- A pessoa enxerga o próprio cliente, e mais nenhum.
create policy "cada um vê o seu cliente"
  on clientes for select to authenticated
  using (id = meu_cliente_id());

-- As categorias do seu cliente. É daqui que a tela descobre o seu nível.
create policy "categorias do meu cliente"
  on categorias for select to authenticated
  using (cliente_id = meu_cliente_id());

-- Cada um enxerga as permissões da PRÓPRIA categoria — é o que monta o menu.
-- Ninguém descobre o que as outras categorias alcançam.
create policy "permissões da minha categoria"
  on categoria_permissoes for select to authenticated
  using (categoria_id = minha_categoria_id());


insert into schema_migrations (versao, arquivo) values ('004', '004_perfis_acesso.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop function if exists minha_categoria_id(), meu_cliente_id();
-- alter table perfis add column nivel nivel_acesso not null default 'comum';
-- update perfis p set nivel = c.nivel from categorias c where c.id = p.categoria_id;
-- drop index if exists perfis_por_categoria, perfis_por_cliente;
-- alter table perfis drop column categoria_id, drop column cliente_id;
-- delete from schema_migrations where versao = '004';
