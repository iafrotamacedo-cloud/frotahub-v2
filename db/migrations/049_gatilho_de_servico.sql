-- =============================================================================
-- 049 — a rotina do gatilho de Serviço                                   rev 1
-- =============================================================================
--
-- O QUE ENTRA AQUI
--
--   Só a rotina no catálogo. Nenhuma tabela nova: as rotas de mudar/remover
--   responsável e criar/excluir cotação/orçamento (rotas_servico.go) falam
--   direto com o Trílogo, do mesmo jeito que o robô já fala — não há estado
--   pra guardar aqui ainda (isso é o Kanban de Serviços, que fica para depois).
--
-- POR QUE UMA ROTINA PRÓPRIA, E NÃO EMPRESTAR CONTRATO_TRILOGO_DADOS
--
--   CONTRATO_TRILOGO_DADOS é leitura — quem tem só essa rotina consegue ABRIR
--   a tela de Dados do Trílogo e nada mais. As rotas deste arquivo ESCREVEM no
--   sistema do cliente: mudam o responsável de um chamado, criam e apagam
--   orçamento. Misturar as duas faria "ver os dados" virar, sem ninguém pedir,
--   "mexer nos dados do cliente" — e é exatamente o furo que o resto do
--   catálogo (CONTRATO_FUNCIONARIOS_DADOS × _DADO_COMPLETO, _DOCUMENTOS ×
--   _APROVAR) sempre separou.
--
-- É segura de rodar duas vezes.
-- =============================================================================

insert into rotinas (codigo, nome, modulo, ordem) values
  ('CONTRATO_SERVICO_GERENCIAR', 'Serviço (fila, cotação e orçamento)', 'manutencao', 340)
on conflict (codigo) do nothing;

-- Nasce para quem já vê os dados do Trílogo — ponto de partida razoável, não
-- decisão final: a matriz de permissões continua aberta pra apertar depois.
insert into categoria_permissoes (categoria_id, rotina, pode)
select cp.categoria_id, 'CONTRATO_SERVICO_GERENCIAR', cp.pode
  from categoria_permissoes cp
 where cp.rotina = 'CONTRATO_TRILOGO_DADOS'
on conflict (categoria_id, rotina) do nothing;

insert into schema_migrations (versao, arquivo)
values ('049', '049_gatilho_de_servico.sql')
on conflict (versao) do nothing;

-- =============================================================================
-- COMO CONFERIR
--
--   select * from rotinas where codigo = 'CONTRATO_SERVICO_GERENCIAR';
--   -- esperado: 1 linha, modulo='manutencao', ordem=340
--
--   select count(*) from categoria_permissoes where rotina = 'CONTRATO_SERVICO_GERENCIAR';
--   -- esperado: o mesmo número de CONTRATO_TRILOGO_DADOS
--
-- PARA DESFAZER
--   delete from categoria_permissoes where rotina = 'CONTRATO_SERVICO_GERENCIAR';
--   delete from rotinas where codigo = 'CONTRATO_SERVICO_GERENCIAR';
--   delete from schema_migrations where versao = '049';
-- =============================================================================
