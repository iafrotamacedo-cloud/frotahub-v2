-- rev 1 -----------------------------------------------------------------------
-- FrotaHub — migration 008: quais arquivos a gente guarda
--
-- O QUE FAZ
--   Acrescenta a marca `copiar` nas aparições de arquivo, e desliga os vídeos.
--
-- O PROBLEMA QUE ISTO RESOLVE
--   Sem esta coluna, "ainda não copiei" e "nunca vou copiar" são a MESMA coisa no
--   banco: `arquivo_sha256` nulo nos dois casos. Consequências, as duas ruins:
--   o robô de cópia tentaria os vídeos para sempre, a cada rodada; e ninguém, daqui
--   a um ano, saberia dizer se um arquivo sem cópia é decisão ou pendência.
--
-- POR QUE OS VÍDEOS FICAM DE FORA
--   O levantamento mediu tudo, arquivo por arquivo: são 8,7 GB, e **678 vídeos
--   carregam 7,3 GB — 86% do peso, em 13% dos arquivos**. Sem eles, o acervo cabe
--   em 1,2 GB, contra os 10 GB do plano gratuito.
--
--   O vídeo não se perde: a aparição continua guardando o endereço dele no
--   Trílogo, e a tela abre por lá.
--
--   O preço, dito com todas as letras: esses vídeos passam a depender de o
--   Trílogo continuar servindo os arquivos. Se um dia esse contrato acabar, eles
--   vão junto. Fotos, PDFs e áudios — que são o que se consulta no dia a dia —
--   ficam conosco.
--
-- COMO MUDAR DE IDEIA DEPOIS
--   É um UPDATE. Ligar os vídeos de volta:
--     update chamado_anexos set copiar = true where extensao in ('mp4','mov',…);
--   e rodar o robô no modo `copia`. Nada precisa ser relido do Trílogo.
--
-- DEPENDE DE: 007
-- -----------------------------------------------------------------------------

begin;

alter table chamado_anexos
  add column copiar boolean not null default true;

comment on column chamado_anexos.copiar is
  'Falso = fica só o endereço no Trílogo, o conteúdo não vem para o nosso armazém.';

-- Desliga o que já está catalogado. O robô cuida do que entrar daqui para frente.
update chamado_anexos
   set copiar = false
 where lower(extensao) in ('mp4', 'mov', 'avi', '3gp', 'm4v', 'mkv', 'wmv', 'webm');

-- O índice da fila de cópia passa a ignorar o que nunca vai ser copiado. Sem
-- isto, a fila carregaria 678 vídeos que ela nunca resolve.
drop index if exists anexos_a_copiar;
create index anexos_a_copiar
  on chamado_anexos (chamado_id)
  where arquivo_sha256 is null and copiar;

insert into schema_migrations (versao, arquivo) values ('008', '008_anexos_copiar.sql')
on conflict (versao) do nothing;

commit;

-- COMO DESFAZER ---------------------------------------------------------------
-- drop index if exists anexos_a_copiar;
-- alter table chamado_anexos drop column copiar;
-- create index anexos_a_copiar on chamado_anexos (chamado_id) where arquivo_sha256 is null;
-- delete from schema_migrations where versao = '008';
