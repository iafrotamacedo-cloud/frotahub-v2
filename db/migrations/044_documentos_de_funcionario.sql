-- =============================================================================
-- 044 — documentos de funcionário: SESMT e DP                           rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   O primeiro módulo de RH do FrotaHub: cadastro de funcionário da obra e o
--   controle dos documentos que a lei exige dele — pessoais, admissionais, ASO
--   (PCMSO) e os certificados de NR (18, 6, 35, 33, 10, 12, 11). Cinco tabelas
--   novas, nenhuma toca no que já existe.
--
-- POR QUE FUNCIONÁRIO NÃO É `perfis`
--
--   `perfis` é quem ENTRA no sistema — tem login, senha, categoria. O peão da
--   obra não loga em lugar nenhum; o que existe dele aqui é o CPF, o RG, a
--   função e a pilha de documentos que precisa estar em dia antes de uma
--   fiscalização. São dois conceitos diferentes, e por isso duas tabelas
--   diferentes — misturar os dois faria `perfis` carregar colunas que só fazem
--   sentido para 5% das linhas.
--
-- POR QUE `funcao_documento_requisitos`, E NÃO UMA LISTA FIXA NO CÓDIGO
--
--   Nem todo funcionário precisa do mesmo documento: um Pedreiro pode precisar
--   de NR-35 (trabalho em altura) e um Servente não. A obrigatoriedade é dado,
--   não regra escrita no Go — é isso que permite a empresa ajustar sem esperar
--   deploy (P-08), do mesmo jeito que `parametros` já faz para valor.
--
-- POR QUE O ARQUIVO REAPROVEITA `arquivos`, E NÃO GANHA TABELA PRÓPRIA
--
--   `arquivos` já é o registro genérico por sha256 — "um conteúdo, uma linha".
--   Um documento de funcionário é só mais um arquivo nesse mesmo armazém,
--   endereçado do mesmo jeito (CORE-01, CORE-04): duplicar continua impossível
--   por construção, e o motor ganha zero código novo de upload — reaproveita
--   `armazem.Enviar`, que já existe e já é testado.
--
-- SESMT E TST/DP NÃO SÃO NÍVEL, SÃO CATEGORIA
--
--   O nível de acesso (`nivel_acesso`) continua sendo só builder/ceo/gerente/
--   comum — mexer nesse enum for a das quatro é reabrir uma decisão que já foi
--   tomada em outra migração. TST (Técnico em Segurança do Trabalho) e DP
--   (Departamento Pessoal) entram como CATEGORIAS novas, nível `comum`, com a
--   própria linha na matriz de permissões — exatamente como qualquer outra
--   categoria que o builder crie pela tela de Acesso.
--
-- QUEM VÊ O QUÊ
--
--   CEO, Builder e DP alcançam os dados completos, inclusive CPF/RG sem
--   máscara (CONTRATO_FUNCIONARIOS_DADO_COMPLETO) — é gente de admissão, que
--   precisa do documento inteiro. TST aprova e envia documento (é o trabalho
--   dele: conferir o certificado de NR), mas não precisa do CPF completo para
--   isso — o motor mascara para quem não tem essa rotina.
--
-- =============================================================================

create table public.funcoes (
  id uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,
  nome text not null,
  criado_em timestamptz not null default now(),
  unique (cliente_id, nome)
);

create table public.tipos_documento (
  id uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,
  nome text not null,
  categoria text not null check (categoria in ('pessoal','admissional','saude','nr','contratual')),
  tem_validade boolean not null default false,
  validade_dias_padrao int,
  dias_alerta int not null default 30,
  criado_em timestamptz not null default now(),
  unique (cliente_id, nome)
);

create table public.funcao_documento_requisitos (
  id uuid primary key default gen_random_uuid(),
  funcao_id uuid not null references funcoes(id) on delete cascade,
  tipo_documento_id uuid not null references tipos_documento(id) on delete cascade,
  obrigatorio boolean not null default true,
  unique (funcao_id, tipo_documento_id)
);

create table public.funcionarios (
  id uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references clientes(id) on delete restrict,
  funcao_id uuid references funcoes(id) on delete restrict,
  unidade_id uuid references unidades(id) on delete set null,
  nome_completo text not null,
  cpf text not null,
  rg text,
  pis_nis text,
  ctps_numero text,
  data_nascimento date,
  data_admissao date,
  data_demissao date,
  status text not null default 'ativo' check (status in ('ativo','inativo','desligado')),
  criado_em timestamptz not null default now(),
  atualizado_em timestamptz not null default now(),
  unique (cliente_id, cpf)
);
comment on table public.funcionarios is
  'Quem trabalha na obra — não loga no sistema. Ver perfis para quem loga.';

create table public.funcionario_documentos (
  id uuid primary key default gen_random_uuid(),
  funcionario_id uuid not null references funcionarios(id) on delete cascade,
  tipo_documento_id uuid not null references tipos_documento(id) on delete restrict,
  arquivo_sha256 text references arquivos(sha256) on delete restrict,
  status text not null default 'pendente' check (status in ('pendente','enviado','aprovado','reprovado','vencido')),
  data_emissao date,
  data_validade date,
  aprovado_por uuid references perfis(id),
  aprovado_em timestamptz,
  motivo_reprovacao text,
  criado_por uuid references perfis(id),
  criado_em timestamptz not null default now(),
  atualizado_em timestamptz not null default now(),
  unique (funcionario_id, tipo_documento_id)
);
comment on table public.funcionario_documentos is
  'Um documento por (funcionário, tipo). Reenviar substitui a linha — a versão anterior some junto com o sha256 antigo, que continua no armazém.';

create trigger funcionarios_carimbo before update on public.funcionarios
  for each row execute function tocar_atualizado_em();
create trigger funcionario_documentos_carimbo before update on public.funcionario_documentos
  for each row execute function tocar_atualizado_em();

-- RLS: mesmo padrão do resto do sistema — leitura do meu cliente + rotina
-- liberada. A escrita de verdade é decidida no motor (a chave de serviço
-- ignora RLS), então estas políticas cobrem quem um dia ler direto do
-- Supabase, e documentam a intenção para quem ler o esquema.
alter table public.funcoes enable row level security;
alter table public.tipos_documento enable row level security;
alter table public.funcao_documento_requisitos enable row level security;
alter table public.funcionarios enable row level security;
alter table public.funcionario_documentos enable row level security;

create policy "funcoes do meu cliente" on public.funcoes for select
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_FUNCIONARIOS_DADOS'));

create policy "tipos de documento do meu cliente" on public.tipos_documento for select
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_FUNCIONARIOS_DADOS'));

create policy "requisitos das funcoes que eu vejo" on public.funcao_documento_requisitos for select
  using (
    exists (select 1 from funcoes f where f.id = funcao_documento_requisitos.funcao_id and f.cliente_id = meu_cliente_id())
    and posso('CONTRATO_FUNCIONARIOS_DADOS')
  );

create policy "funcionarios do meu cliente" on public.funcionarios for select
  using (cliente_id = meu_cliente_id() and posso('CONTRATO_FUNCIONARIOS_DADOS'));

create policy "documentos dos funcionarios que eu vejo" on public.funcionario_documentos for select
  using (
    exists (select 1 from funcionarios fu where fu.id = funcionario_documentos.funcionario_id and fu.cliente_id = meu_cliente_id())
    and posso('CONTRATO_FUNCIONARIOS_DOCUMENTOS')
  );

-- Catálogo de rotinas do módulo (mesmo padrão CONTRATO_<MODULO>_<AÇÃO> de
-- Orçamentos e Estatísticas).
insert into public.rotinas (codigo, nome, modulo, ordem) values
  ('CONTRATO_FUNCIONARIOS_DADOS', 'Funcionários — dados', 'sesmt-dp', 1),
  ('CONTRATO_FUNCIONARIOS_DOCUMENTOS', 'Funcionários — documentos', 'sesmt-dp', 2),
  ('CONTRATO_FUNCIONARIOS_APROVAR', 'Funcionários — aprovar/reprovar documento', 'sesmt-dp', 3),
  ('CONTRATO_FUNCIONARIOS_DADO_COMPLETO', 'Funcionários — ver CPF/RG completo (sem máscara)', 'sesmt-dp', 4);

-- Categorias novas: TST (Técnico em Segurança do Trabalho) e DP (Departamento
-- Pessoal). Nível comum — não passam pela exceção do builder, entram na
-- matriz como qualquer categoria criada pela tela de Acesso.
insert into public.categorias (cliente_id, codigo, nome, nivel)
select id, 'tst', 'TST', 'comum'::nivel_acesso from public.clientes where slug = 'frota-macedo'
union all
select id, 'dp', 'DP', 'comum'::nivel_acesso from public.clientes where slug = 'frota-macedo';

-- Permissões: CEO e DP têm acesso total (inclusive dado completo e
-- aprovação). TST aprova/edita documento mas não vê CPF/RG sem máscara.
-- Builder não precisa de linha: a exceção do builder já cobre (permissao.go).
insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo in ('ceo','dp')
  and r.codigo in ('CONTRATO_FUNCIONARIOS_DADOS','CONTRATO_FUNCIONARIOS_DOCUMENTOS','CONTRATO_FUNCIONARIOS_APROVAR','CONTRATO_FUNCIONARIOS_DADO_COMPLETO');

insert into public.categoria_permissoes (categoria_id, rotina, pode)
select c.id, r.codigo, true
from public.categorias c, public.rotinas r
where c.codigo = 'tst'
  and r.codigo in ('CONTRATO_FUNCIONARIOS_DADOS','CONTRATO_FUNCIONARIOS_DOCUMENTOS','CONTRATO_FUNCIONARIOS_APROVAR');

-- Catálogo inicial de tipos de documento (NRs e exigências de construção
-- civil, levantado com a legislação vigente). `funcoes` e a matriz de
-- `funcao_documento_requisitos` ficam vazias de propósito — quem sabe o que
-- cada cargo da obra exige é o DP, pela tela, não esta migração.
insert into public.tipos_documento (cliente_id, nome, categoria, tem_validade, validade_dias_padrao, dias_alerta)
select id, v.nome, v.categoria, v.tem_validade, v.validade_dias_padrao, v.dias_alerta
from public.clientes, (values
  ('RG', 'pessoal', false, null::int, 30),
  ('CPF', 'pessoal', false, null::int, 30),
  ('CTPS (Carteira de Trabalho Digital)', 'pessoal', false, null::int, 30),
  ('PIS/NIS', 'pessoal', false, null::int, 30),
  ('Comprovante de residência', 'pessoal', false, null::int, 30),
  ('Título de eleitor', 'pessoal', false, null::int, 30),
  ('Certificado de reservista', 'pessoal', false, null::int, 30),
  ('CNH', 'pessoal', true, 1825, 60),
  ('Contrato de trabalho assinado', 'admissional', false, null::int, 30),
  ('Ficha de registro de empregado', 'admissional', false, null::int, 30),
  ('ASO Admissional', 'saude', false, null::int, 30),
  ('ASO Periódico', 'saude', true, 365, 30),
  ('ASO Demissional', 'saude', false, null::int, 30),
  ('ASO Mudança de Função', 'saude', false, null::int, 30),
  ('ASO Retorno ao Trabalho', 'saude', false, null::int, 30),
  ('Certificado NR-18 (Integração)', 'nr', true, 365, 30),
  ('Ficha de entrega de EPI (NR-6)', 'nr', false, null::int, 30),
  ('Certificado NR-35 (Trabalho em Altura)', 'nr', true, 730, 45),
  ('Certificado NR-33 (Espaço Confinado)', 'nr', true, 365, 30),
  ('Certificado NR-10 (Eletricidade)', 'nr', true, 730, 45),
  ('Certificado NR-12 (Máquinas e Equipamentos)', 'nr', true, 730, 45),
  ('Certificado NR-11 (Operação de Equipamentos)', 'nr', true, 730, 45),
  ('Ordem de Serviço (OS) assinada', 'contratual', false, null::int, 30)
) as v(nome, categoria, tem_validade, validade_dias_padrao, dias_alerta)
where clientes.slug = 'frota-macedo';

insert into schema_migrations (versao, arquivo)
values ('044', '044_documentos_de_funcionario.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- STATUS: JÁ APLICADA em 02/09/2026, antes desta migração ganhar arquivo —
-- as cinco tabelas, as quatro rotinas, as duas categorias e as onze
-- permissões já existem no banco (conferido: get_advisors sem aviso novo).
-- Este arquivo é o registro do que já está lá, para o histórico não ficar
-- com um buraco. Rodar de novo falharia em "already exists" — é o esperado,
-- exceto o próprio insert acima, que é `on conflict do nothing` de propósito.
-- =============================================================================
