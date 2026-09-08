-- =============================================================================
-- 057 — biometria facial: verificação de rosto no login                  rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   Uma tabela só: `perfil_biometria_facial`, o molde do rosto de quem tem
--   login (`perfis`). É a base para o pedido do dono do sistema — "reconhecimento
--   para toda necessidade de auth" — mas esta migração cobre só o primeiro uso:
--   um segundo fator opcional no login, autoatendido por cada login em Minha
--   conta, do mesmo jeito que a própria senha (P-29).
--
--   Funcionário de obra (`funcionarios`) não loga — se o rosto vier a servir
--   para ponto ou conferência de documento, é tabela nova, quando esse desenho
--   existir. Misturar os dois papéis aqui seria adivinhar o requisito de algo
--   que ainda não foi pedido.
--
-- POR QUE O MOLDE FICA NO BANCO, E NÃO SÓ NO NAVEGADOR
--
--   O molde ("descritor") é o que a rede de reconhecimento facial calcula a
--   partir do rosto: 128 números que não dá para reverter para uma foto. Ele
--   precisa estar em algum lugar comum a todo aparelho de onde a pessoa loga —
--   celular do fiscal, computador do escritório — e o banco já é isso.
--
-- POR QUE `real[]`, E NÃO UMA COLUNA POR NÚMERO NEM JSONB
--
--   São sempre 128 posições, sempre float, sempre lidos e escritos inteiros —
--   nunca um pedaço só. `real[]` deixa a restrição de tamanho ser do BANCO
--   (P-04), sem depender de quem escreve o JSON lembrar de mandar os 128.
--
-- POR QUE A LEITURA VAI DIRETO AO SUPABASE, E A ESCRITA PASSA PELO MOTOR
--
--   Mesma régua de `perfis`: o QUE a pessoa é (aqui, o próprio molde, para
--   comparar no navegador contra a captura da câmera) é lido direto, porque
--   precisa ser instantâneo e a política de linha já limita a UMA linha, a
--   própria. CRIAR ou APAGAR o molde é subir de conta de segurança — o mesmo
--   nível de trocar a senha — e por isso só o motor grava, com a chave de
--   serviço, deixando rastro (MOD-USUARIOS-01: `cadastrou_biometria_facial`,
--   `removeu_biometria_facial`). Sem política de INSERT/UPDATE/DELETE aqui,
--   de propósito — só o SELECT abre para o dono da linha (CORE-08: na dúvida,
--   nega).
--
-- POR QUE NÃO HÁ HISTÓRICO DO CONTEÚDO DO MOLDE
--
--   Do mesmo jeito que senha nunca entra no histórico: o molde não é "dado",
--   é segredo de acesso. O que fica registrado é o FATO — cadastrou, removeu —
--   nunca os 128 números.
-- =============================================================================

create table public.perfil_biometria_facial (
  perfil_id uuid primary key references perfis(id) on delete cascade,
  cliente_id uuid not null references clientes(id) on delete restrict,
  descritor real[] not null,
  criado_em timestamptz not null default now(),
  atualizado_em timestamptz not null default now(),
  constraint perfil_biometria_facial_128 check (array_length(descritor, 1) = 128)
);
comment on table public.perfil_biometria_facial is
  'O molde do rosto de quem tem login, para o segundo fator opcional no login. Um por perfil — reenviar substitui. Ver perfis para quem loga; ver MOD-USUARIOS-01 para o rastro de cadastrar/remover.';
comment on column public.perfil_biometria_facial.descritor is
  '128 números calculados pela rede de reconhecimento facial no navegador — não dá para reconstruir uma foto a partir deles.';

create trigger perfil_biometria_facial_carimbo before update on public.perfil_biometria_facial
  for each row execute function tocar_atualizado_em();

alter table public.perfil_biometria_facial enable row level security;

-- Só o dono da linha lê — e só lê, porque é assim que ele compara a própria
-- captura da câmera contra o molde salvo. Sem política de escrita: quem grava
-- é o motor, pela chave de serviço, que ignora RLS e é o único que gera rastro.
create policy "a propria biometria facial" on public.perfil_biometria_facial for select
  using (perfil_id = auth.uid());

insert into schema_migrations (versao, arquivo)
values ('057', '057_biometria_facial.sql')
on conflict (versao) do nothing;
