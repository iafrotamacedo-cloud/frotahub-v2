-- =============================================================================
-- 075 — NF: receber por PDF, e separar "entregar" de "enviar ao cliente"  rev 1
-- =============================================================================
--
-- O QUE O DONO PEDIU (17/09/2026, obra piloto MSL Fátima)
--
--   Ao vivo, testando o recebimento de nota na obra, foram dois pedidos de
--   permissão além dos ajustes de tela do dia:
--
--   1. "opção de receber a NF inserindo a nota por PDF, com permissão
--      específica (eu decido quem pode)" — e depois, mudando de ideia sobre
--      quem alcança: "compras e adm tb devem poder fazer isso" (não só
--      gerencial pra cima, como tinha dito antes). Ou seja: por CATEGORIA,
--      sem trava de nível — o dono escolhe pela tela de Categorias, do
--      jeito que já escolhe pra toda rotina do sistema.
--
--   2. "gostaria de deixar bem definido a permissão para recebimento da
--      nota no escritório" — e hoje `COMPRAS_NF_ENTREGAR` (migração 064)
--      cobre DUAS coisas: confirmar que a nota chegou fisicamente no
--      escritório, E marcar que ela saiu no malote pro cliente. Não dá pra
--      "definir bem" uma permissão que na prática é duas. Separada do
--      mesmo jeito que RECEBER/ENTREGAR/CONFIGURAR_ACESSO já eram três
--      rotinas distintas desde a 064.
--
-- POR QUE `COMPRAS_NF_RECEBER_PDF` É ROTINA PRÓPRIA, NÃO UMA FLAG DENTRO DE
-- `COMPRAS_NF_RECEBER`
--
--   O armazém já sabe guardar PDF (`guardarArquivoNF`, olha a extensão) —
--   sem uma rotina própria, todo almoxarife com COMPRAS_NF_RECEBER ganharia
--   PDF de brinde só porque o formulário aceita qualquer arquivo. Separada,
--   o dono concede a quem ele quiser (obra OU escritório, Compras OU Adm),
--   sem depender de quem já recebe pela câmera.
--
-- POR QUE `COMPRAS_NF_ENTREGAR` NÃO MUDA DE CÓDIGO
--
--   Só o SIGNIFICADO fica mais estreito (passa a cobrir só a confirmação de
--   entrega no escritório). Manter o mesmo código evita órfão: quem já tinha
--   a permissão concedida (hoje só CEO, migração 064) continua com ela sem
--   precisar de nada novo. `COMPRAS_NF_ENVIAR_CLIENTE` é a rotina nova, pro
--   segundo passo — herda de quem já tinha ENTREGAR (linha abaixo), pra
--   ninguém perder capacidade que já usava; dali em diante são duas
--   concessões independentes.
--
-- =============================================================================

-- `modulo_menu` existe desde a 067 (categoria_modulos_e_bypass) — toda
-- rotina de compras/NF já usa 'administrativo' aqui (é onde o item mora no
-- menu, não o nome do pacote Go).
insert into public.rotinas (codigo, nome, modulo, modulo_menu, ordem) values
  ('COMPRAS_NF_RECEBER_PDF', 'Notas fiscais — receber por PDF (sem câmera)', 'compras', 'administrativo', 7),
  ('COMPRAS_NF_ENVIAR_CLIENTE', 'Notas fiscais — marcar enviada ao cliente (malote)', 'compras', 'administrativo', 8);

-- O NOME ANTIGO JÁ DENUNCIAVA O PROBLEMA: "confirmar entrega E envio" — as
-- duas coisas que esta migração está separando. Estreita o nome junto com o
-- código (que este continua sendo o mesmo, ver cabeçalho).
update public.rotinas set nome = 'Notas fiscais — confirmar entrega no escritório'
where codigo = 'COMPRAS_NF_ENTREGAR';

-- Quem já tinha COMPRAS_NF_ENTREGAR concedida herda COMPRAS_NF_ENVIAR_CLIENTE
-- — preserva o alcance de hoje; o dono ajusta as duas separadamente dali pra
-- frente, pela tela de Categorias.
insert into public.categoria_permissoes (categoria_id, rotina, pode)
select categoria_id, 'COMPRAS_NF_ENVIAR_CLIENTE', true
from public.categoria_permissoes
where rotina = 'COMPRAS_NF_ENTREGAR' and pode = true;

-- COMPRAS_NF_RECEBER_PDF não é concedida a ninguém por esta migração, de
-- propósito — mesmo espírito de COMPRAS_NF_RECEBER na 064: o dono ainda vai
-- escolher, pela tela de Categorias, quem alcança (ele já disse que quer
-- Compras e Administrativo, mas isso se faz na tela, não numa migração).

insert into schema_migrations (versao, arquivo)
values ('075', '075_nf_receber_pdf_e_split_entregar.sql');

-- =============================================================================
-- PARA VERIFICAR
--
--   select r.codigo, r.nome, r.ordem from rotinas r
--    where r.codigo in ('COMPRAS_NF_RECEBER_PDF','COMPRAS_NF_ENVIAR_CLIENTE');
--   -- esperado: as duas linhas
--
--   select c.codigo as categoria, cp.rotina, cp.pode
--     from categoria_permissoes cp join categorias c on c.id = cp.categoria_id
--    where cp.rotina in ('COMPRAS_NF_ENTREGAR','COMPRAS_NF_ENVIAR_CLIENTE')
--    order by c.codigo, cp.rotina;
--   -- esperado: toda categoria com ENTREGAR=true também aparece com
--   -- ENVIAR_CLIENTE=true (hoje, só 'ceo'); nenhuma linha de
--   -- COMPRAS_NF_RECEBER_PDF (ninguém tem ainda, de propósito)
--
-- PARA DESFAZER
--   update rotinas set nome = 'Notas fiscais — confirmar entrega e envio' where codigo = 'COMPRAS_NF_ENTREGAR';
--   delete from categoria_permissoes where rotina in ('COMPRAS_NF_RECEBER_PDF','COMPRAS_NF_ENVIAR_CLIENTE');
--   delete from rotinas where codigo in ('COMPRAS_NF_RECEBER_PDF','COMPRAS_NF_ENVIAR_CLIENTE');
--   delete from schema_migrations where versao = '075';
-- =============================================================================
