-- =============================================================================
-- 018 — o orçamento que nasceu antes do chamado fica órfão para sempre   rev 1
-- =============================================================================
--
-- O QUE ACONTECEU HOJE
--
--   Cinco orçamentos (R$ 1.513,44) apareciam como "chamado não encontrado".
--   Os chamados existiam no Trílogo — só eram velhos demais para a janela do
--   robô, que lê de 01/07/2026 para cá. Rodamos o modo `alvos` com 116391,
--   116919, 120779 e 121505; os quatro entraram no banco, com 140 eventos.
--
--   E os cinco orçamentos continuaram órfãos.
--
-- POR QUE
--
--   `orcamentos.chamado_id` é resolvido UMA VEZ, quando o orçamento nasce. Se o
--   chamado ainda não existe naquele instante, a coluna fica nula — e nada
--   nunca mais volta para preenchê-la. O robô grava chamados; ele não sabe que
--   existe alguém esperando por eles.
--
--   É o mesmo defeito da 013, de outro ângulo: lá o ticket denormalizado
--   envelhecia mentindo; aqui o elo simplesmente não chega a existir. Nos dois
--   casos, o conserto é o banco fazer sozinho o que se esperava que alguém
--   lembrasse de fazer.
--
-- O CONSERTO É UM GATILHO, NÃO UM UPDATE
--
--   Um UPDATE hoje resolve estes cinco e deixa a armadilha armada para o
--   próximo. O gatilho faz o chamado ADOTAR, ao entrar, todo orçamento órfão
--   com o número dele. Não é o robô que passa a saber disso — é o banco, que é
--   onde a regra vale para qualquer caminho de escrita (CORE-06).
--
--   A adoção não toca em orçamento que já tem chamado: `chamado_id is null` é a
--   condição inteira. Um orçamento apontando para o chamado errado não é
--   problema que este gatilho resolva, e sobrescrever seria pior que deixar.
--
-- É segura de rodar duas vezes.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- 1. O gatilho — o chamado adota os órfãos ao chegar
-- -----------------------------------------------------------------------------
create or replace function chamado_adota_orcamentos()
returns trigger language plpgsql as $$
begin
  update orcamentos o
     set chamado_id = new.id
   where o.cliente_id = new.cliente_id
     and o.ticket     = new.numero
     and o.chamado_id is null;
  return null;
end;
$$;

comment on function chamado_adota_orcamentos() is
  'Ao entrar um chamado, liga a ele os orçamentos daquele ticket que estavam sem chamado.';

drop trigger if exists adota_orcamentos on chamados;
create trigger adota_orcamentos
  after insert on chamados
  for each row
  execute function chamado_adota_orcamentos();


-- -----------------------------------------------------------------------------
-- 2. A adoção dos que já estavam esperando
--
-- O gatilho só vale para quem chegar daqui para frente. Estes quatro chamados
-- já entraram hoje de manhã, antes de ele existir.
-- -----------------------------------------------------------------------------
update orcamentos o
   set chamado_id = c.id
  from chamados c
 where c.cliente_id = o.cliente_id
   and c.numero     = o.ticket
   and o.chamado_id is null
   and o.status <> 'removido';


insert into schema_migrations (versao, arquivo)
values ('018', '018_orcamento_adota_chamado.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select count(*) as orfaos from orcamentos
--    where chamado_id is null and status <> 'removido';
--   -> 0
--
--   select ticket, parte, valor, ticket_status, destino
--     from orcamentos_lista
--    where ticket in (116391, 116919, 120779, 121505) and status = 'gerado'
--    order by ticket, parte;
--   -> 116391 Em execução (encarregados) · 116919 Arquivado (cliente)
--      120779 Vistoriado (PODE LANÇAR)  · 121505 Aberto (encarregados)
--
-- PARA DESFAZER
--   drop trigger if exists adota_orcamentos on chamados;
--   drop function if exists chamado_adota_orcamentos();
--   (a adoção já feita não se desfaz sozinha — e não deveria.)
-- =============================================================================
