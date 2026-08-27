-- =============================================================================
-- 040 — a loja tem o nosso nome, o nome do cliente e o CNPJ           rev 1
-- =============================================================================
--
-- POR QUE ISTO PRECISA EXISTIR
--
--   O relatório mensal que vai ao cliente identifica a loja como ELE a chama:
--   `LJ-02 - 2.7(OLIVEIRA PAIVA)`. Nós a chamamos `LOJA 02 - OLIVEIRA PAIVA`.
--   Aquele `2.7` é o CENTRO DE CUSTO dele e não existe em lugar nenhum do nosso
--   sistema — não dá para derivar do nosso nome, só para cadastrar.
--
--   Mandar o nome errado joga o custo no centro de custo errado, e é o tipo de
--   erro que só aparece quando o cliente contesta, semanas depois.
--
-- POR QUE NÃO É TABELA NOVA
--
--   `unidades` já tem `cnpj`, vazia nas 38 linhas. A correspondência é atributo
--   da unidade, não uma entidade à parte: uma tabela de-para separaria o que
--   anda junto e criaria um segundo lugar para a mesma loja existir.
--
-- POR QUE A CHAVE É `id_trilogo`, E NÃO O NOME
--
--   Nome muda — o cliente renomeia loja, alguém corrige um acento. O id do
--   Trílogo não. Casar por nome é plantar um de-para que quebra na primeira
--   correção ortográfica, em silêncio.
--
-- O CNPJ VAI SÓ COM OS 14 DÍGITOS
--
--   Mesma escolha da chave de acesso da NFe no resto do sistema. Máscara é
--   apresentação; guardada, ela vira duas grafias do mesmo número, e aí a
--   comparação passa a depender de qual delas alguém digitou.
--
-- AS 33 E AS 5
--
--   A lista de CNPJ do cliente tem 33 lojas. Nós temos 38 unidades. As cinco
--   sem CNPJ:
--
--     LOJA 07 - CRATO ................. fora do nosso escopo, 0 orçamentos
--     LOJA 12 - JUAZEIRO .............. fora do nosso escopo, 0 orçamentos
--     LOJA 25 - LAGOA SECA ............ fora do nosso escopo, 0 orçamentos
--     LOJA 31 - MERCADÃO NOVO JUAZEIRO  fora do nosso escopo, 0 orçamentos
--     ESCRITÓRIO ...................... NO escopo, e com cobrança em aberto
--
--   As quatro primeiras casam exatamente com `no_escopo = false`. Duas listas
--   feitas por gente diferente concordando é a melhor prova de que o
--   mapeamento está certo.
--
--   O ESCRITÓRIO é o buraco de verdade: está no escopo, tinha 6 orçamentos a
--   cobrar em 27/08/2026, e o cliente não mandou o CNPJ dele. Fica nulo, e a
--   conferência no rodapé aponta. Quando o número chegar, é um update de uma
--   linha.
--
-- DOIS NOMES ESTRANHOS QUE SÃO CÓPIA FIEL, E NÃO ERRO DE DIGITAÇÃO
--
--   `LJ-03 - 103.1(BENFICA)` — o 103 foge do padrão de todos os outros.
--   `LJ30 -  30.2(...)`      — DOIS espaços depois do traço.
--
--   Os dois vieram assim do arquivo do cliente. Se ele faz PROCV com esse
--   texto, "consertar" aqui quebraria a busca dele.
-- =============================================================================

alter table unidades
  add column if not exists nome_cliente text;

comment on column unidades.nome_cliente is
  'Como o CLIENTE chama esta loja, incluindo o código do centro de custo dele. '
  'É o que sai no relatório mensal — cópia fiel do cadastro dele, espaços '
  'duplos e tudo (migração 040).';

comment on column unidades.cnpj is
  'Só os 14 dígitos, sem máscara — mesma escolha da chave de acesso da NFe.';

update unidades u
   set nome_cliente = v.nome_cliente,
       cnpj         = coalesce(v.cnpj, u.cnpj),
       atualizado_em = now()
  from (values
  ( 62, 'LJ-01 - 1.9(MERCADÃO DUNAS)'           , '03720882000239'  ),  -- LOJA 01 - DUNAS
  (  1, 'LJ-02 - 2.7(OLIVEIRA PAIVA)'           , '03720882000310'  ),  -- LOJA 02 - OLIVEIRA PAIVA
  ( 63, 'LJ-03 - 103.1(BENFICA)'                , '03720882000409'  ),  -- LOJA 03 - BENFICA
  ( 66, 'LJ-04 - 4.3(PONTES VIEIRA)'            , '03720882000581'  ),  -- LOJA 04 - PONTES VIEIRA
  ( 67, 'LJ-05 - 5.1(COCÓ)'                     , '03720882001120'  ),  -- LOJA 05 - COCÓ
  ( 70, 'LJ-06 - 6.0(VIRGILIO TAVORA)'          , '03720882000662'  ),  -- LOJA 06 - VIRGÍLIO TÁVORA
  ( 94, 'LJ-07 - 7.8(MERCADÃO CRATO)'           , null              ),  -- LOJA 07 - CRATO
  ( 73, 'LJ-08 - 8.6(BARÃO DE STUDART)'         , '03720882001391'  ),  -- LOJA 08 - BARÃO DE STUDART
  ( 75, 'LJ-09 - 9.4(CAMBEBA)'                  , '03720882000743'  ),  -- LOJA 09 - CAMBEBA
  ( 79, 'LJ-10 - 10.8(PRAIA RACEMA)'            , '03720882000824'  ),  -- LOJA 10 - PRAIA
  (118, 'LJ-11 - 11.6(MERCADÃO WS)'             , '03720882001553'  ),  -- Loja 11 - WS
  ( 95, 'LJ-12 - 12.4(JUAZEIRO NORTE)'          , null              ),  -- LOJA 12 - JUAZEIRO
  ( 82, 'LJ-13 - 13.2(SANTOS DUMMONT)'          , '03720882001634'  ),  -- LOJA 13 - SANTOS DUMONT
  ( 84, 'LJ-14 - 14.0(RIOMAR)'                  , '03720882001987'  ),  -- LOJA 14 - RIOMAR PAPICU
  ( 85, 'LJ-15 - 15.9(ALPHAVILLE)'              , '03720882001804'  ),  -- LOJA 15 -ALPHAVILLE
  ( 86, 'LJ-16 - 16.7(MERCADÃO PK)'             , '03720882002010'  ),  -- LOJA 16 - RIOMAR PRESIDENTE KENNEDY
  (100, 'LJ-17 - 17.5(EDILSON BRASIL)'          , '03720882002100'  ),  -- LOJA 17 - EDILSON BRASIL
  ( 87, 'LJ-18 - 18.3(MERCADÃO CASTELÃO)'       , '03720882002282'  ),  -- LOJA 18 - CASTELÃO
  ( 88, 'LJ-19 - 19.1(DEL PASSEO)'              , '03720882002363'  ),  -- LOJA 19 - DEL PASEO
  ( 89, 'LJ-20 - 20.5(RUI BARBOSA)'             , '03720882002444'  ),  -- LOJA 20 - RUI BARBOSA
  ( 91, 'LJ-22 - 22.1(MIGUEL DIAS)'             , '03720882002797'  ),  -- LOJA 22 - MIGUEL DIAS
  ( 92, 'LJ-23 - 23.0(JULIO VENTURA)'           , '03720882002606'  ),  -- LOJA 23 - JÚLIO VENTURA
  ( 93, 'LJ-24 - 24.8(EUSÉBIO)'                 , '03720882002878'  ),  -- LOJA 24 - EUSÉBIO
  ( 96, 'LJ-25 - 25.6(LAGO SECA)'               , null              ),  -- LOJA 25 - LAGOA SECA
  (250, 'LJ-26 - 26.4(SOBRAL)'                  , '03720882003092'  ),  -- LOJA 26 - SOBRAL
  (170, 'LJ-27 - 27.2(MERCADÃO MARACANAU)'      , '03720882003254'  ),  -- LOJA 27 - MARACANAU
  (171, 'LJ-28 - 28.0(ABOLIÇÃO)'                , '03720882003173'  ),  -- LOJA 28 - ABOLIÇÃO
  (249, 'LJ29 - 29.9(MERCADÃO MONDUBIM)'        , '03720882003335'  ),  -- LOJA 29 - MONDUBIM
  (226, 'LJ30 -  30.2(MERCADÃO RUI BARBOSA)'    , '03720882003416'  ),  -- LOJA 30 - MERCADÃO RUI
  (300, 'LJ31 - 31.0(MERCADÃO NOVO JUAZEIRO)'   , null              ),  -- LOJA 31 - MERCADÃO NOVO JUAZEIRO
  (318, 'LJ35 - 35.3(MERCADÃO CAUCAIA)'         , '03720882003688'  ),  -- LOJA 35 - MERCADÃO CAUCAIA
  (337, 'LJ36 - 36.1(ANTONIO SALES)'            , '03720882003769'  ),  -- LOJA 36 - ANTÔNIO SALES
  (346, 'LJ37 - 37.0(MERCADÃO BORGUES DE MELO)' , '03720882003840'  ),  -- LOJA 37 - BORGES DE MELO
  (353, 'LJ38 - 38.8(PORTO DAS DUNAS)'          , '03720882004064'  ),  -- LOJA 38 - PORTO DAS DUNAS
  (377, 'LJ39 - 39.6(VILLAS JARDIM)'            , '03720882003920'  ),  -- LOJA 39 - VILLAS
  (123, 'CD - 82.5'                             , '03720882000158'  ),  -- CD
  ( 90, 'EMPORIO'                               , '03720882002525'  ),  -- EMPÓRIO
  (  3, 'ESCRITÓRIO - 74.4'                     , null              )  -- ESCRITÓRIO
  ) as v(id_trilogo, nome_cliente, cnpj)
 where u.id_trilogo = v.id_trilogo;

-- =============================================================================
-- A CONFERÊNCIA RODA AQUI DENTRO, E DERRUBA A MIGRAÇÃO SE NÃO FECHAR
--
--   Um de-para escrito à mão é exatamente o tipo de coisa que ninguém confere
--   depois. Se ele entrar torto, o erro só aparece na fatura do cliente.
-- =============================================================================

do $$
declare
  v_sem_nome  int;
  v_com_cnpj  int;
  v_repetido  int;
  v_torto     int;
begin
  select count(*) into v_sem_nome  from unidades where nome_cliente is null;
  select count(*) into v_com_cnpj  from unidades where cnpj is not null;
  select count(*) into v_torto     from unidades
   where cnpj is not null and (length(cnpj) <> 14 or cnpj !~ '^[0-9]+$');
  select count(*) into v_repetido  from (
    select nome_cliente from unidades where nome_cliente is not null
     group by nome_cliente having count(*) > 1) x;

  if v_sem_nome > 0 then
    raise exception 'ficaram % unidade(s) sem o nome do cliente — o de-para não '
                    'cobre todas, e a que faltar sai em branco no relatório', v_sem_nome;
  end if;
  if v_torto > 0 then
    raise exception '% CNPJ não tem 14 dígitos limpos', v_torto;
  end if;
  if v_repetido > 0 then
    raise exception '% nome(s) de cliente repetido(s) — duas lojas nossas apontando '
                    'para o mesmo centro de custo dele', v_repetido;
  end if;
  if v_com_cnpj <> 33 then
    raise exception 'esperava 33 CNPJ e achei % — a lista do cliente tem 33 lojas', v_com_cnpj;
  end if;

  raise notice '38 unidades com nome do cliente · 33 com CNPJ';
  raise notice 'sem CNPJ (esperado): CRATO, JUAZEIRO, LAGOA SECA e NOVO JUAZEIRO '
               '(fora do escopo) e o ESCRITÓRIO, que precisa do número';
end $$;

insert into schema_migrations (versao, arquivo)
values ('040', '040_a_loja_tem_o_nome_do_cliente.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select nome, nome_cliente,
--          case when cnpj is null then '—'
--               else regexp_replace(cnpj, '(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})',
--                                   '\1.\2.\3/\4-\5') end as cnpj
--     from unidades order by nome;
--
--   -- as sem CNPJ têm que ser as 4 fora do escopo + o escritório:
--   select nome, no_escopo from unidades where cnpj is null order by nome;
--
-- QUANDO O CNPJ DO ESCRITÓRIO CHEGAR
--   update unidades set cnpj = '0372088200XXXX' where id_trilogo = 3;
--
-- PARA DESFAZER
--   update unidades set nome_cliente = null, cnpj = null;
--   alter table unidades drop column nome_cliente;
-- =============================================================================
