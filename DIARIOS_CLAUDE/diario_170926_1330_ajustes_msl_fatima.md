# Diário — 17/09/2026 13:30 — Ajustes na obra piloto (MSL Fátima)

## O que o Igor pediu

Ele estava ao vivo na obra piloto (MSL Fátima) usando o FrotaHub e foi listando,
um por um, o que via dando errado. Pediu pra eu **só registrar** cada item —
nada de codar ou vasculhar banco — até ele decidir seguir. Em dois dos três
itens ele pediu "investigue o pq" antes de eu simplesmente anotar. No fim,
mandou codar tudo que já tinha sido visto e escrever este diário.

## Item 1 — "Notas Fiscais → Acessos por obra" trava em "Acordando o servidor"

**O que parecia**: cold start do plano free do Render, só demorando demais.

**O que era de verdade** (investigado com os logs do próprio Render, serviço
`baleryan`): o backend respondia rápido — com **erro**. A rota
`GET /administrativo/nf/acessos` falhava toda vez:

```
não consegui listar os acessos concedidos: resposta do banco em
centro_custo_acessos?...&select=...,perfis(nome),centros_custo(...)
não é o que eu esperava: json: cannot unmarshal object into Go value of type []map[string]interface {}
```

Causa raiz: `centro_custo_acessos` (migração 064) tem **duas** foreign keys pra
`perfis` — `perfil_id` e `concedido_por`. O `select=...,perfis(nome),...` não
diz qual das duas usar pro embed, o PostgREST não desempata sozinho e devolve
um objeto de erro em vez da lista — o Go tenta decodificar isso como
`[]map[string]interface{}` e quebra.

Isso sozinho já devolveria um erro pro usuário. Mas tinha um **segundo bug**,
no front, escondendo esse erro: em `AcessosObraNF.tsx`, a tela só sai do
`<Carregando/>` quando os três estados (`obras`, `perfis`, `acessos`) deixam
de ser `null` — e como o `Promise.all` rejeita inteiro assim que uma das três
chamadas falha, nenhum dos três nunca é setado. O erro ficava guardado em
`erro`, mas a tela nunca chegava a olhar pra ele: girava pra sempre.

**Corrigido:**
- [acessos_obra.go](../baleryan/interno/modulos/administrativo/acessos_obra.go) —
  `perfis(nome)` → `perfis!perfil_id(nome)`, desambiguando o embed.
- [AcessosObraNF.tsx](../web/src/telas/administrativo/AcessosObraNF.tsx) — antes
  de checar os três estados `null`, checa se já tem `erro` guardado e mostra a
  mensagem em vez de ficar preso no `<Carregando/>`.

`go build ./...` e `tsc --noEmit` limpos nos dois. Não testado ao vivo (sem
acesso à obra/câmera daqui).

## Item 2 — Scanner de notas saindo torto

Investigado o código de detecção ([deteccao.ts](../web/src/componentes/scanner/deteccao.ts)).
Não é bug de coordenada — conferi que o requadro na tela e o recorte final
usam exatamente a mesma geometria do quadro cheio da câmera, sem descompasso.

É imprecisão inerente ao método: a detecção de cantos roda numa versão do
quadro reduzida a **240 px de largura** (de propósito — é Hough em TypeScript
puro, sem OpenCV, pra não pesar no 4G da obra), com resolução angular de
**1,5° por passo** e uma janela de voto de **±10,5°**. Esse erro pequeno, na
imagem de 240 px, é ampliado em ~9× quando o recorte em perspectiva é gerado
na resolução final (até 2200 px) — aí aparece como a folha ainda meio torta.
Piora com pouco contraste (mesa clara + nota clara) ou texto/tabela perto da
borda do papel.

**Não codado.** Não é um ajuste de uma linha — melhorar de verdade pede
aumentar a resolução de análise e/ou refinar o passo angular, o que pesa mais
no celular. Fica registrado pro Igor decidir o trade-off; não mexi.

## Item 3 — "Receber NF" abria a câmera direto, sem passar pelo formulário

No celular, clicar em "receber NF" na lista de Aguardando abria o scanner
(câmera) em tela cheia na hora — o almoxarife nem via antes de qual O.C. era
o recebimento. Era proposital na rev 4 (15/09), documentado no cabeçalho do
arquivo, mas o Igor pediu pra tirar: entrar primeiro no formulário, e o
scanner só abre por um botão dentro dele — que já existia
("Escanear a nota" / "Mais uma página").

**Corrigido**: [ReceberNF.tsx](../web/src/telas/administrativo/ReceberNF.tsx) —
`scannerAberto` nascia `useState(true)`, virou `useState(false)`. Só isso;
o botão que já existia no formulário é quem agora é o único jeito de abrir o
scanner. Fica igual ao padrão que `Devolver.tsx` e `ReceberLocacao.tsx` (
módulo de Locações) já usavam.

## Segunda rodada — mais 5 itens, mesma sessão

Depois dos 3 primeiros, o Igor listou mais 5 sem pausa pra investigar, e no
fim pediu pra codar essa etapa inteira também.

### Item 4 — "Aguardando NF": cada card precisa de um link que abre a OC

`GET /administrativo/compras/ordens/{id}/arquivo` (o que Compras usa pra ver
o PDF da OC) exige `COMPRAS_ORDENS_GERENCIAR` — rotina que o almoxarife que
recebe NF não tem. Reusar essa rota devolveria 403 bem na cara de quem o
link foi feito pra atender. Criei uma rota irmã,
`GET /administrativo/nf/ordens/{id}/arquivo`
([notas_fiscais.go](../baleryan/interno/modulos/administrativo/notas_fiscais.go)),
atrás de `COMPRAS_NF_RECEBER` e peneirada pela mesma regra de obra que já
protege a lista (`temAcessoAObra`). No front
([AguardandoNF.tsx](../web/src/telas/administrativo/AguardandoNF.tsx)): o
card inteiro no mobile é clicável (padrão já estabelecido em
`CartaoLinha.tsx` — "o cartão inteiro clica quando tem ficha atrás"), e no
desktop o número da O.C. virou um botão com cara de link
(`.bt-como-link`, novo em `telas.css`). Os dois abrem o mesmo
`VisorDeDocumento`.

### Item 5 — layout saindo mais largo que a tela no mobile

Sem celular físico aqui pra reproduzir, não dava pra caçar um culpado
específico com segurança. Varri o CSS atrás de `100vw`/larguras fixas — só
achei um caso (`.orc-desfaz`) e ele já tem `max-width:92vw` cobrindo, não é
o problema. Apliquei a trava padrão pra esse sintoma:
`html,body{overflow-x:hidden}` em [base.css](../web/src/estilos/base.css).
Nenhuma tela do sistema depende de rolagem horizontal do corpo (tabela larga
já rola por dentro, `.tabela-rolo`), então não quebra nada — mas é um
remédio geral, não a causa raiz confirmada. Se voltar a acontecer, precisa
de um celular de verdade pra achar o elemento exato.

### Item 6 — todo login deve cair na tela inicial

O endereço (`#/...`) é guardado no próprio navegador de propósito (permite
recarregar e continuar na mesma tela — ver `navegacao.ts`). Isso também
significa que uma aba com sessão expirada, ou um favorito colado direto
numa tela interna, reabre nessa tela depois do login — nunca no início.
Corrigido em [useSessao.ts](../web/src/sessao/useSessao.ts): `entrar()`
zera o endereço (`window.location.hash = ''`) só no momento em que o login
dá certo. Recarregar a página com sessão já ativa continua caindo no mesmo
lugar — só o login em si é que sempre reseta.

### Item 7 — Receber NF por PDF, com permissão própria

Pedido inicial: só gerencial pra cima. Depois, mudança: "compras e adm tb
devem poder fazer isso" — ou seja, por categoria, sem trava de nível.
Sugeri (e o Igor topou) uma rotina nova e separada de `COMPRAS_NF_RECEBER`,
`COMPRAS_NF_RECEBER_PDF`, concedida por categoria à escolha dele (não
automática pra ninguém). O armazém já sabia guardar PDF
(`guardarArquivoNF` olha a extensão); só faltava a trava de permissão — sem
ela, quem só recebe pela câmera ganharia PDF de brinde só porque o
formulário aceita qualquer arquivo. No front
([ReceberNF.tsx](../web/src/telas/administrativo/ReceberNF.tsx)): botão
"Enviar PDF" ao lado de "Escanear a nota", só aparece com a rotina; a
miniatura da página vira um quadro com ícone (PDF não renderiza em
`<img>`); a renumeração de páginas ao remover uma agora preserva a extensão
de cada arquivo (antes forçava `.jpg` em tudo — teria corrompido o tipo do
PDF).

### Item 8 — definir bem a permissão de recebimento no escritório

Achado ao investigar: `COMPRAS_NF_ENTREGAR` (migração 064) já cobria DUAS
coisas — confirmar que a nota chegou no escritório, E marcar que saiu no
malote pro cliente. O próprio nome da rotina no banco dizia isso
("confirmar entrega **e envio**"). Separei em duas, mesmo espírito de
RECEBER/ENTREGAR já serem rotinas distintas desde a 064:
`COMPRAS_NF_ENTREGAR` (código sem mudar, só o nome/significado — agora só
"confirmar entrega no escritório") e `COMPRAS_NF_ENVIAR_CLIENTE` (nova).
Quem já tinha ENTREGAR herdou ENVIAR_CLIENTE na migração, pra não perder
capacidade — dali em diante o Igor concede as duas separadamente pela tela
de Categorias.

### Migração 075 — aplicada em produção

[075_nf_receber_pdf_e_split_entregar.sql](../db/migrations/075_nf_receber_pdf_e_split_entregar.sql)
— duas rotinas novas, um rename, um INSERT que preserva quem já tinha
ENTREGAR. Aplicada direto no Supabase de produção (`frotahub-v2`,
`hltcngamdqabqlocufrv`) via MCP, mesmo padrão de migrações anteriores deste
repo (ex.: 072, "já aplicada em produção"). Na primeira tentativa faltou a
coluna `modulo_menu` (existe desde a 067, não usada no meu rascunho
inicial) — Postgres recusou a transação inteira, nada ficou pela metade;
corrigi e reapliquei com sucesso. Conferido depois: as duas rotinas
aparecem em `rotinas`, na ordem certa, e a tela de Categorias já lista
tudo dinamicamente (busca o catálogo do motor, não precisa de código novo
ali).

## Terceira rodada — NF com valor divergente da OC (item 9, o grande)

Feature nova, não um ajuste — o Igor explicou o problema de negócio com
calma (item faltando/sobrando, nota parcial já resolvida, produto por peso
com pequena divergência de balança) e pediu pra alinhar o desenho antes de
codar. Duas perguntas via `AskUserQuestion` (quem é RC/RO — RC já existia
no código, ligado a `LOCACOES_RENOVAR_OC`; RO não existia, confirmado como
"Responsável de Obra" = quem tem `COMPRAS_NF_RECEBER`; e se "corrigir a OC"
reusa o substituir de PDF que já existe — confirmado que sim) e uma proposta
escrita antes do primeiro `Write`.

**O achado técnico que mudou o desenho**: o substituir de OC que já existe
(`substituirOrdemPCO`, pra "nota veio com valor diferente do previsto" —
frase que já estava no cabeçalho de `substituicao.go` desde 12/09, quase
prevendo este pedido) APAGA a OC velha. Mas `notas_fiscais.ordem_compra_id`
tem `on delete restrict` — reusar aquilo tal como está quebraria a chave
estrangeira em qualquer OC que já tivesse nota recebida, que é exatamente o
caso de uso aqui. Resolvido migrando as notas fiscais pra OC nova ANTES de
apagar a velha (`corrigirOrdemComDivergencia`, novo).

**O desenho final:**
- Limiar de 3% (`LimiarDivergenciaOCPercent`), em centavos
  (`regras.Dinheiro`), nunca em float — dinheiro não divide em ponto
  flutuante (P-12 do próprio sistema).
- `avaliarDivergenciaOC` roda depois de toda NF recebida ou trocada: bate
  exato → nada muda; dentro de 3% e ainda não marcada → marca sozinha
  (`correcao_origem = 'automatica'`); fora de 3% → não mexe, fica esperando
  decisão. NUNCA desmarca uma correção manual sozinha — só um encaixe
  exato de valor, ou o RC clicando "voltar pra aguardando", desfazem.
- `nf_progresso_ordens.completa` mudou de `recebido >= total` (deixava
  sobra grande sumir da fila sozinha, sem ninguém decidir nada) para
  `recebido = total OR aguardando_correcao` — testado contra os 13 OCs
  reais de produção depois de aplicar, sem surpresa (só uma, a 019783, com
  1,38% de desvio — vai marcar sozinha na próxima nota recebida nela).
- 4 rotas novas: `POST nf/ordens/{id}/marcar-correcao` (RO força
  manualmente), `POST compras/ordens/{id}/voltar-aguardando` (RC desiste),
  `POST compras/ordens/{id}/corrigir` (RC sobe a OC certa — migra as notas,
  reseta status pra `recebida`, apaga a velha, a nova cai sozinha em
  Pendentes de envio pro reenvio de PCO), `POST nf/notas/{id}/trocar` (RC
  OU RO trocam só a nota, sem mexer na OC — sempre volta pra `recebida`,
  "nunca à frente disso" como pedido).
- Card novo "Correção de OC" em Compras
  ([CorrecaoDeOC.tsx](../web/src/telas/administrativo/CorrecaoDeOC.tsx)),
  reusando `VisorDeDocumento`/`Confirmar`/`CartaoLinha` — nada de
  componente novo de exibição, só a tela de listagem+ações.
  [AguardandoNF.tsx](../web/src/telas/administrativo/AguardandoNF.tsx) ganha
  "enviar p/ correção" (só quando já tem algo recebido).
  [ListaDeNF.tsx](../web/src/telas/administrativo/ListaDeNF.tsx) ganha
  "trocar" em todo lugar — inclusive pro almoxarife em modo
  `somenteLeitura`, porque quem já enxerga a lista já tem uma das três
  rotinas que `trocarNF` aceita.

**Migração 076** — aplicada em produção via MCP do Supabase, mesmo fluxo da
075. Bateu num detalhe do Postgres na primeira tentativa: `create or
replace view` recusa MUDAR a posição/nome de uma coluna existente —
coloquei `aguardando_correcao`/`correcao_origem` no meio do SELECT (antes
de "completa") e o banco achou que eu estava RENOMEANDO "completa".
Resolvido movendo as duas colunas novas pro fim do SELECT, depois de
`pco_enviado_em` — mesma regra que `nf_progresso_ordens` já seguia sem eu
ter percebido a razão até bater de frente com ela.

Nenhuma rotina nova de permissão pra RC: reusei `COMPRAS_ORDENS_GERENCIAR`
(já é quem gerencia OC em todo o resto do sistema) — RC não ganhou um
crachá novo, só mais uma capacidade dentro do que ele já tinha.

## Quarta rodada — testando ao vivo, "imagine comigo"

Antes de testar de verdade, o Igor pediu pra percorrer cenários em texto
("imagine comigo") — tracei um caso concreto (OC de 510, nota de 507 —
0,59%, dentro dos 3%: a nota vai pra Recebidas normal, a OC sai de
Aguardando NF e cai em Correção de OC, marcada automática) e ele foi
achando 6 ajustes em cima disso, testando ao vivo. Pediu pra só registrar
os 4 primeiros ("não faça nada agora, só guarde") e depois mandou codar
tudo de uma vez.

Nessa rodada vários arquivos (`ListaDeNF.tsx`, `TrocarNF.tsx`,
`notas_fiscais.go`, `NotasFiscais.tsx`, `modulo.go`) tinham mudado no disco
sem eu ter feito — outra sessão trabalhando em paralelo em Locações
(migração 077, `aguardando_nf_locacao`, campo `origem` na nota). Reli tudo
do zero antes de mexer, pra não pisar em cima — nada colidiu com o que eu
já tinha feito (correcao_oc.go, as rotas de correção) — os dois trabalhos
ficaram em partes diferentes dos mesmos arquivos.

### Ajuste 1 — NF de OC em correção não pode ser entregue no escritório

`avancarNF` (comum a `entregarNF` e `enviarNFAoCliente`) agora busca a OC
da nota antes de avançar e recusa (409) se `aguardando_correcao=true`. Vale
pras duas etapas, não só entrega — mandar ao cliente com a OC ainda errada
seria pior que só travar no escritório.

### Ajustes 2 e 3 — flag de correção + fornecedor na lista de NF

`comDadosDaOrdem` (usada pelas 3 listas — Recebidas/Entregues/Enviadas)
agora também busca `fornecedor_id`/`aguardando_correcao` da OC e reusa
`comNomeDoFornecedor` (a mesma função que Aguardando NF já usava — sem
duplicar a busca). A flag (`pino pino-err`, vermelha, "correção") aparece
do lado do número da nota sempre que a OC dela está na fila — some sozinha
no próximo carregamento da lista, porque é sempre a OC atual sendo lida,
nunca um estado congelado. Coluna de fornecedor nova na tabela, com classe
`.tabela-nf-apertada` pra caber (letra e preenchimento menores, só nesta
tabela).

### Ajuste 4 — não era um bug

Já tinha sido feito no item 4 da segunda rodada (link da OC em Aguardando
NF, desktop incluso). Sugeri duas explicações — cache/deploy atrasado, ou
confusão com a tela nova de `aguardando_locacao` que a outra sessão criou —
sem mexer em nada.

### Ajuste 5 — "← voltar" colado no notch/status bar

Achado: `.orc-tela` (a tela de documento — `VisorDeDocumento`,
`RepararOrdemOC`, `EditorDeOC`) é onde TODA visualização de documento do
sistema vive, e ela pede "foco" (`usePedirFoco`, `Foco.tsx`) — que esconde
a barra `.top` do app pra dar o espaço todo ao papel. Só que é a `.top`
quem reservava `env(safe-area-inset-top)`; sem ela, `.orc-tela` vira o
topo físico da tela, e o "← voltar" disputa espaço com o relógio/Dynamic
Island em celular com tela recortada — "alguns celulares" porque só os com
notch/ilha sofrem disso. `.lay.focada` já existia (App.tsx marcava a casca
assim que qualquer tela pedia foco) sem NENHUMA regra usando — gancho
pronto, só faltava a regra: `.lay.focada .orc-tela{padding-top:env(...)}`.
Não mexe em nenhuma das outras ~14 telas que usam `.orc-tela` só como
layout comum (essas continuam sob a `.top`, que já cuida da própria
folga).

### Ajuste 6 — gesto de arrastar (iPhone) pula pro menu

A causa: telas como `VisorDeDocumento` abrem por ESTADO local
(`setVendo(null)`), nunca mudam o endereço (`#/...`) — proposital, pra não
empilhar histórico a cada "ver" que a pessoa abre. O botão "← voltar" de
dentro cobre isso bem, mas o GESTO do sistema mexe direto no histórico do
navegador, que nunca soube que aquela tela abriu — ele pula ela inteira.
Corrigido com um hook novo,
[VoltarLocal.ts](../web/src/componentes/VoltarLocal.ts): ao abrir, empilha
uma entrada de histórico "muda" (mesmo endereço); o gesto consome ela
primeiro (dispara `popstate`, que a gente escuta e usa pra fechar a tela
igual ao botão); só then o próximo gesto volta a valer pra navegação de
verdade. Fechar por qualquer outro caminho desfaz a entrada muda sozinho —
com uma trava a mais: só desfaz se ninguém navegou pra outro lugar nesse
meio tempo (dá pra clicar no menu lateral no computador com um documento
aberto), senão desfaria a navegação que a pessoa acabou de fazer.

Aplicado só em `VisorDeDocumento.tsx` — é quem cobre toda visualização de
OC/NF testada hoje. `VisorDaNota.tsx` (Orçamentos, mais antigo, fecha por
um caminho diferente — não tem um `voltar()` único) ficou de fora por
enquanto; mesmo problema, mas precisa de mais uma olhada pra encaixar.

## O que ficou pendente

- Item 2 (scanner torto) segue sem correção — decisão de trade-off do dono.
- Item 5 (largura no mobile, primeira rodada) recebeu uma trava geral
  (`overflow-x:hidden`), não uma causa raiz confirmada — precisa de
  celular de verdade se voltar.
- `VisorDaNota.tsx` (Orçamentos) tem o mesmo problema do Ajuste 6 e não foi
  corrigido — fecha por um caminho diferente do `voltar()` único que
  `useVoltarLocal` espera.
- Item 9 (correção de OC): nada testado ao vivo com uma OC real de ponta a
  ponta (receber divergente → cair na fila → corrigir → conferir que o PCO
  realmente reenvia). O e-mail de PCO em si eu não toquei — a expectativa é
  que a OC nova, nascendo sem `pco_enviado_em`, entre no fluxo de envio já
  existente sem precisar de nada novo, mas isso é inferência de código, não
  teste de ponta a ponta.
- Nada desta sessão foi testado ao vivo na obra; só build (`go build`,
  `go vet`, `go test`) e `tsc --noEmit`, todos limpos. O teste que já
  falhava antes (`TestDesenharOC_RoundTripLer`, PDF de OC) continua
  falhando — confirmado que não é regressão de hoje (rodei com
  `git stash` antes de mexer em nada).
- Ninguém ainda tem `COMPRAS_NF_RECEBER_PDF` — o Igor concede pela tela de
  Categorias quando quiser (disse que quer Compras e Administrativo).
