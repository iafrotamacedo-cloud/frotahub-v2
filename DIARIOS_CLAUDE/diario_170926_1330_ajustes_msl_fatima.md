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

## O que ficou pendente

- Item 2 (scanner torto) segue sem correção — decisão de trade-off do dono.
- Nenhum dos três itens foi testado ao vivo na obra; só build/typecheck limpos.
