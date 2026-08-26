-- 021 — a NOTA também adota o chamado que chegou depois
--
-- O DEFEITO, E COMO ELE APARECEU
--
--   A 018 fez os ORÇAMENTOS adotarem o chamado que chega depois. Ninguém fez o
--   mesmo pelas NOTAS — e é nelas que o problema começa.
--
--   Quando um ticket é amarrado a uma nota, o motor procura o chamado naquele
--   instante. Se ele ainda não veio do Trílogo, a linha nasce com
--   `chamado_id = null`: é a nota "sem associação". Duas horas depois o robô
--   traz o chamado, e a linha da nota CONTINUA nula. Para sempre.
--
--   Nada no sistema voltava para religá-la. A nota ficava travada num problema
--   que já tinha deixado de existir, e o único jeito de destravar era alguém
--   mexer no ticket de novo — reescrevendo o mesmo número que já estava certo.
--
-- POR QUE ISSO FICOU URGENTE EM 26/08/2026
--
--   Porque o botão "atualizar" da tela de tratamento nasceu neste dia, e ele
--   faz exatamente isto: relê o Trílogo e reconfere as notas. Sem esta adoção,
--   ele releria, o chamado entraria, e a linha continuaria VERMELHA — porque a
--   conferência lê o `chamado_id` guardado, não o número do ticket.
--
--   O botão pareceria quebrado. E o pior: pareceria quebrado dizendo a verdade
--   sobre um dado errado, que é o tipo de defeito que ninguém consegue
--   diagnosticar olhando a tela.
--
-- E O NÚMERO VELHO NUNCA SOBREVIVE AO CONSERTO
--
--   Esta é a garantia que o dono pediu. A geração lê os tickets da nota na hora
--   de gerar (`documento_tickets`, sempre fresco), e não uma cópia guardada. O
--   que faltava era o elo `ticket -> chamado` acompanhar a realidade. Agora
--   acompanha, dos dois lados: a nota e o orçamento.

create or replace function chamado_adota_notas()
returns trigger language plpgsql as $$
begin
  -- `documento_tickets` não tem cliente_id: ele vem pela nota. O `exists`
  -- garante que um chamado de um cliente nunca adote a nota de outro — a mesma
  -- numeração pode existir em dois contratos.
  update documento_tickets dt
     set chamado_id = new.id
   where dt.ticket     = new.numero
     and dt.chamado_id is null
     and exists (
       select 1 from documentos d
        where d.id = dt.documento_id
          and d.cliente_id = new.cliente_id
          and d.oculto_em is null);
  return null;
end;
$$;

comment on function chamado_adota_notas() is
  'Ao entrar um chamado, liga a ele os tickets de nota daquele número que estavam soltos. '
  'Sem isto, uma nota fica em "sem associação" para sempre, mesmo depois de o chamado chegar.';

drop trigger if exists adota_notas on chamados;
create trigger adota_notas
  after insert on chamados
  for each row
  execute function chamado_adota_notas();

-- -----------------------------------------------------------------------------
-- A adoção dos que já estavam esperando
--
-- O gatilho vale para quem chegar daqui em diante. Estes já estão no banco,
-- soltos, esperando desde o dia em que o ticket foi escrito.
-- -----------------------------------------------------------------------------
update documento_tickets dt
   set chamado_id = c.id
  from documentos d, chamados c
 where d.id          = dt.documento_id
   and c.numero      = dt.ticket
   and c.cliente_id  = d.cliente_id
   and dt.chamado_id is null
   and d.oculto_em   is null;

insert into schema_migrations (versao, arquivo)
values ('021', '021_nota_adota_chamado.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- não pode sobrar ticket solto cujo chamado JÁ existe na base:
--   select dt.ticket
--     from documento_tickets dt
--     join documentos d on d.id = dt.documento_id
--    where dt.chamado_id is null
--      and d.oculto_em is null
--      and exists (select 1 from chamados c
--                   where c.numero = dt.ticket and c.cliente_id = d.cliente_id);
--   -> nenhuma linha
--
--   -- e os que continuam soltos são os que realmente não existem:
--   select dt.ticket from documento_tickets dt
--     join documentos d on d.id = dt.documento_id
--    where dt.chamado_id is null and d.oculto_em is null;
--   -> só tickets que o Trílogo ainda não trouxe
--
-- PARA DESFAZER
--   drop trigger if exists adota_notas on chamados;
--   drop function if exists chamado_adota_notas();
--   (a adoção já feita não se desfaz — e não deveria: ela corrigiu um dado errado.)
-- =============================================================================
