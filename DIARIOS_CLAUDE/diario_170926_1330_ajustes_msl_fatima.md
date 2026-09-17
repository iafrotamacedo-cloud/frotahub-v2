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

## O que ficou pendente

- Item 2 (scanner torto) segue sem correção — decisão de trade-off do dono.
- Item 5 (largura no mobile) recebeu uma trava geral (`overflow-x:hidden`),
  não uma causa raiz confirmada — precisa de celular de verdade se voltar.
- Nada desta sessão foi testado ao vivo na obra; só build (`go build`,
  `go vet`, `go test`) e `tsc --noEmit`, todos limpos. O teste que já
  falhava antes (`TestDesenharOC_RoundTripLer`, PDF de OC) continua
  falhando — confirmado que não é regressão de hoje (rodei com
  `git stash` antes de mexer em nada).
- Ninguém ainda tem `COMPRAS_NF_RECEBER_PDF` — o Igor concede pela tela de
  Categorias quando quiser (disse que quer Compras e Administrativo).
