-- =============================================================================
-- 032 — 'possivel_duplicata' entra na lista de bloqueios permitidos
-- =============================================================================
--
-- O QUE ACONTECEU, MEDIDO EM 26/08/2026
--
--   A trava de duplicidade entrou no motor e funcionou na primeira tentativa: no
--   lote de 56, o orçamento do ticket 125199 (R$ 33,36) foi barrado porque o
--   custo nº 38062, do mesmo valor, já estava no Trílogo desde 19/08. A tela
--   mostrou a frase certa e o lançamento não aconteceu.
--
--   Mas o BANCO recusou a marca. `orcamentos_lancamento_bloqueio_check`, escrito
--   na 015, lista os bloqueios que podem existir — e 'possivel_duplicata' não
--   estava nela. O `bloquear` engole o próprio erro de propósito (para não trocar
--   a mensagem do Trílogo por uma mensagem do banco), então o insucesso foi só
--   para o log:
--
--     select count(*) from orcamentos where lancamento_bloqueio='possivel_duplicata'
--     -- 0, depois de a trava ter barrado de verdade
--
-- POR QUE ISSO NÃO É DETALHE
--
--   Sem a marca, o orçamento barrado não aparece em Correções › Recusados. A
--   trava protege o lançamento de HOJE e não deixa rastro nenhum para amanhã:
--   quem abrir a fila amanhã vê um orçamento gerado, sem nenhuma indicação de
--   que ele já foi apontado como repetido, e clica em lançar.
--
--   Uma conferência que barra sem registrar é uma conferência que precisa ser
--   feita de novo toda vez, por uma pessoa que talvez não estivesse lá.
--
-- A LIÇÃO, PARA A PRÓXIMA FLAG
--
--   Esta coluna é FECHADA por `check`. Bloqueio novo no Go exige migração junto,
--   no mesmo pacote. O teste `TestTodaFlagDeBloqueioEhPermitidaNoBanco` passou a
--   ler esta lista e comparar com as flags do `lancar.go` — se as duas se
--   separarem outra vez, ele quebra antes de subir.
-- =============================================================================

alter table orcamentos drop constraint if exists orcamentos_lancamento_bloqueio_check;
alter table orcamentos add constraint orcamentos_lancamento_bloqueio_check
  check (lancamento_bloqueio is null or lancamento_bloqueio in (
    'ticket_status',      -- "não é permitida para tickets com o status ..."
                          -- A mensagem deles MENTE: cita só 'Aberto' e 'Em
                          -- execução', mas recusa 'Arquivado' também — medido no
                          -- ticket 126211. Nunca decidir lendo a frase.
    'ticket_recusado',    -- o ticket não existe, ou não é da conta que tentou
    'teto',               -- entre gerar e lançar, o ticket recebeu outro custo
    'possivel_duplicata', -- o ticket JÁ TEM um custo deste mesmo valor lá
    'sem_empresa',        -- o CompanyId daquela conta não está configurado aqui
    'trilogo_fora',       -- login ou rede falharam: é temporário, não é defeito
    'desconhecido'        -- qualquer outra. A frase crua fica no detalhe.
  ));

insert into schema_migrations (versao, arquivo)
values ('032', '032_duplicata_e_um_bloqueio.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   -- a lista aceita o valor novo:
--   update orcamentos set lancamento_bloqueio = 'possivel_duplicata'
--    where false;               -- não muda nada; só o `check` é validado
--
--   -- e, depois de tentar lançar o 125199 de novo, ele passa a aparecer:
--   select ticket, parte, lancamento_bloqueio, lancamento_bloqueio_detalhe
--     from orcamentos where lancamento_bloqueio = 'possivel_duplicata';
--
-- PARA DESFAZER
--   Repita o bloco `check` da 015 (o mesmo, sem 'possivel_duplicata') — mas
--   apague antes as marcas que já existirem, ou o `check` recusa a si mesmo:
--   update orcamentos set lancamento_bloqueio = null
--    where lancamento_bloqueio = 'possivel_duplicata';
-- =============================================================================
