-- =============================================================================
-- CONSERTO — a mesma nota cobrada duas vezes no mesmo ticket    27/08/2026
--            R$ 854,40
-- =============================================================================
--
-- NÃO É MIGRAÇÃO. Não muda esquema, não entra em `schema_migrations`, roda uma
-- vez. Conserta DADO — o estrago que o defeito da busca de candidatas deixou.
--
-- O QUE ACONTECEU
--
--   Duas notas fiscais entraram DUAS VEZES cada uma, por caminhos diferentes:
--   um scan solto (`CCF17082026_*.pdf`) e uma página de um PDF multipágina. São
--   arquivos diferentes, com sha256 diferente — a trava do ARQUIVO não vê nada,
--   e está certa: são arquivos diferentes mesmo.
--
--   A trava da NOTA deveria ter unido as duas na leitura, e falhou. A busca das
--   candidatas era exclusiva:
--
--     if chave != "" { procura SÓ por chave_acesso }
--     else           { procura por numero }
--
--   Num dos scans a IA não leu os 44 dígitos da chave; no outro leu. A cópia sem
--   chave nunca entrava na lista da que tinha chave, e `regras.MesmaNota` — que
--   diria "é a mesma" pelo número — nunca era chamada para aquele par.
--
--   Com três segundos entre uma e outra, nenhuma das duas achou a outra:
--   a primeira procurou por número e a segunda ainda não tinha número gravado;
--   a segunda procurou por chave e a primeira não tem chave.
--
-- O ESTRAGO, MEDIDO
--
--   NF 17936 · R$ 500,00 · ticket 126342 · dois orçamentos de R$ 600,00
--   NF 17937 · R$ 212,00 · ticket 112449 · dois orçamentos de R$ 254,40
--
--   Total cobrado a mais: R$ 854,40. NENHUM dos quatro foi lançado no Trílogo
--   (`trilogo_custo_id` nulo nos quatro) — conferido em 27/08/2026, e o script
--   recusa rodar se isso tiver mudado.
--
-- QUAL DAS DUAS FICA
--
--   A que chegou PRIMEIRO, que é a regra do sistema (`conferirRepetida` marca
--   quem chegou depois). Aqui isso mantém as partes 1, que já existem, e apaga
--   as partes 2.
--
--   Os valores são idênticos nos dois pares, então a escolha não mexe em
--   dinheiro nenhum: o que sai é a duplicata, não uma versão melhor da nota.
--
-- O QUE ESTE ARQUIVO FAZ, POR PAR
--
--   1. marca a cópia com `duplicada_de` apontando para a original — é o que a
--      leitura teria feito, e é o que faz a tela mostrar "repetida"
--   2. marca o orçamento da cópia como `removido`, com motivo escrito
--   3. tira o vínculo nota↔orçamento de circulação (`removido_em`), para o par
--      (documento, ticket) ficar livre se um dia precisar ser refeito
--   4. devolve a nota cópia de `usado` para `lido`: ela não virou orçamento
--      nenhum, e dizer que virou seria mentir na contabilidade
--
--   O orçamento NÃO é apagado de verdade. `removido` é reversível e deixa
--   rastro; um `delete` esconderia que isso aconteceu.
--
-- É IDEMPOTENTE — roda duas vezes sem efeito adicional.
-- =============================================================================

do $$
declare
  par record;
  v_original uuid;
  v_copia    uuid;
  v_orc      uuid;
  v_ticket   int;
  v_valor    numeric;
  v_feitos   int := 0;
begin
  for par in
    select * from (values
      -- (nota, arquivo que FICA, arquivo que sai)
      ('17936', 'CCF17082026_0006.pdf', 'nf frota macedo - 10.08_p3.pdf'),
      ('17937', 'CCF17082026_0004.pdf', 'TICKET_112449_NOTA_SN.pdf')
    ) as v(numero, arquivo_original, arquivo_copia)
  loop
    select id into v_original from documentos
     where numero = par.numero and nome_arquivo = par.arquivo_original;
    select id into v_copia from documentos
     where numero = par.numero and nome_arquivo = par.arquivo_copia;

    if v_original is null or v_copia is null then
      raise exception 'não achei o par da nota % (% / %)',
        par.numero, par.arquivo_original, par.arquivo_copia;
    end if;

    -- IDEMPOTÊNCIA: se a cópia já está marcada, este par já foi tratado.
    if exists (select 1 from documentos where id = v_copia and duplicada_de is not null) then
      raise notice 'nota % já estava tratada — pulando', par.numero;
      continue;
    end if;

    -- AS DUAS TÊM QUE CHEGAR NA MESMA ORDEM DE ANTES
    --   Se a cópia for a mais antiga, a regra do sistema apontaria o contrário e
    --   este script estaria invertendo original e cópia.
    if (select inserido_em from documentos where id = v_copia)
       <= (select inserido_em from documentos where id = v_original) then
      raise exception 'na nota %, o arquivo que eu ia marcar como cópia chegou ANTES '
                      'do outro — a ordem mudou desde a análise; confira antes de rodar',
                      par.numero;
    end if;

    select o.id, o.ticket, o.valor into v_orc, v_ticket, v_valor
      from orcamentos o
      join orcamento_documentos od on od.orcamento_id = o.id
     where od.documento_id = v_copia and o.status <> 'removido';

    if v_orc is null then
      raise exception 'não achei o orçamento vivo da cópia da nota %', par.numero;
    end if;

    -- TRAVA DE DINHEIRO: se já foi lançado no Trílogo, PARA.
    --   Apagar aqui um custo que existe lá deixaria os dois sistemas discordando
    --   — e o conserto passaria a ser no Trílogo, à mão, não neste arquivo.
    if exists (select 1 from orcamentos
                where id = v_orc and (trilogo_custo_id is not null or status = 'lancado')) then
      raise exception 'o orçamento % (ticket %) já foi lançado no Trílogo — este '
                      'script não mexe no que já virou custo lá; trate à mão',
                      v_orc, v_ticket;
    end if;

    -- 1) a cópia é cópia
    update documentos set duplicada_de = v_original where id = v_copia;

    -- 2) o orçamento dela sai de circulação, com o motivo escrito
    update orcamentos
       set status = 'removido',
           removido_em = now(),
           removido_motivo = 'duplicidade: a nota ' || par.numero || ' entrou duas vezes '
             || '(' || par.arquivo_original || ' e ' || par.arquivo_copia || ') e gerou '
             || 'dois orçamentos no ticket ' || v_ticket || '. Conserto de 27/08/2026.'
     where id = v_orc;

    -- 3) o vínculo sai de circulação, liberando o par (documento, ticket)
    update orcamento_documentos set removido_em = now()
     where orcamento_id = v_orc and removido_em is null;

    -- 4) a nota cópia não virou orçamento nenhum
    update documentos set status = 'lido' where id = v_copia;

    v_feitos := v_feitos + 1;
    raise notice 'nota % · ticket % · orçamento de R$ % removido (%)',
      par.numero, v_ticket, v_valor, par.arquivo_copia;
  end loop;

  raise notice '=== % par(es) tratado(s) ===', v_feitos;
end $$;

-- =============================================================================
-- COMO CONFERIR
--
--   -- cada ticket voltou a ter UM orçamento vivo:
--   select o.ticket, count(*) filter (where o.status <> 'removido') as vivos,
--          count(*) filter (where o.status = 'removido') as removidos,
--          sum(o.valor) filter (where o.status <> 'removido') as valor_certo
--     from orcamentos o where o.ticket in (126342, 112449) group by o.ticket;
--   -- esperado: 126342 -> 1 vivo, 1 removido, 600,00
--   --           112449 -> 1 vivo, 1 removido, 254,40
--
--   -- as cópias estão marcadas e voltaram para `lido`:
--   select numero, nome_arquivo, status, duplicada_de is not null as marcada_repetida
--     from documentos where numero in ('17936','17937') order by numero, inserido_em;
--
--   -- e não sobrou nenhum par de notas iguais sem marcação:
--   select count(*) from documentos a join documentos b
--     on b.cliente_id = a.cliente_id and b.id > a.id
--    and a.numero = b.numero and a.valor_total = b.valor_total
--   where a.oculto_em is null and b.oculto_em is null
--     and a.duplicada_de is null and b.duplicada_de is null;
--   -- esperado: 0
--
-- PARA DESFAZER
--   update documentos set duplicada_de = null, status = 'usado'
--    where nome_arquivo in ('nf frota macedo - 10.08_p3.pdf','TICKET_112449_NOTA_SN.pdf');
--   update orcamentos set status = 'gerado', removido_em = null, removido_motivo = null
--    where removido_motivo like 'duplicidade: a nota%';
--   update orcamento_documentos set removido_em = null
--    where orcamento_id in (select id from orcamentos where removido_motivo like 'duplicidade%');
-- =============================================================================
