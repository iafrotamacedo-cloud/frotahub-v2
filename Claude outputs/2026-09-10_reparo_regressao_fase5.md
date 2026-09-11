# Reparo da regressão da "Fase 5: Inserir OC" — 10/09/2026

## O que aconteceu

Uma sessão paralela trabalhando na tela **Inserir OC** (Compras/Administrativo)
fez commit e push de três arquivos-chave do sistema — `web/src/App.tsx`,
`web/src/menu/arvore.ts` e `baleryan/cmd/baleryan/baleryan.go` — a partir de
uma cópia **desatualizada**, de antes dos módulos Serviços, Engenharia,
SESMT e DP, Consolidação, Estatísticas e Rogue Worker existirem.

O commit (`88aeee2`, "Fase 5: Inserir OC") não apagou nenhum desses módulos
do código-fonte — todos os arquivos em `telas/`, `menu/modulos/` e
`interno/modulos/` continuaram intactos. O que ele fez foi **sobrescrever os
três arquivos que ligam tudo** com uma versão anterior a esses módulos
existirem, derrubando de uma vez:

- **Frontend** (`App.tsx` + `arvore.ts`): Serviços, Engenharia, SESMT e DP,
  Consolidação (voltou a ser "Balanço" em breve), Estatísticas inteira, e o
  chat do Rogue Worker na barra lateral.
- **Backend** (`baleryan.go`): as rotas Go dos módulos `servicos`,
  `estatisticas`, `consolidacao`, `funcionarios`, `planejamento` e
  `rogueworker` pararam de ser registradas no `mux` — cada uma dessas rotas
  passou a responder **404 puro**, sem corpo JSON.

Uma tentativa de conserto por outra sessão (`a9556dc`) religou só a árvore de
menu do frontend, mas incompleta (só os três módulos "Fase 5", sem perceber
que Estatísticas e Consolidação também tinham sumido) — e foi revertida de
novo (`b1e8f2f`) por engano, piorando o estado.

### Por que o sintoma enganava

O front mostrava a mensagem genérica **"Alguma coisa deu errado do nosso
lado. Tente de novo."** em vez de um erro específico. Isso acontece porque
o motor (`web/src/motor/cliente.ts`) só sabe ler `{"erro": "..."}` do corpo
da resposta — uma resposta 404 sem JSON nenhum (a página padrão do
`http.ServeMux` do Go) cai direto na mensagem genérica, escondendo a causa
real (rota inexistente) atrás de um texto que parece "erro interno".

## Linha do tempo dos commits

| Commit | O que fez |
|---|---|
| `363c6d7` | Última versão **boa** conhecida antes da regressão (todos os módulos montados) |
| `376b5f3` | Administrativo entra no menu (frontend) — ainda uma versão boa |
| `88aeee2` | **"Fase 5: Inserir OC"** — sobrescreve `App.tsx`, `arvore.ts` e `baleryan.go` com uma base antiga; deploy no Render às 16:30 |
| `96d2109` | Tentativa de conserto de build, mas em cima da base já regredida |
| `a9556dc` | Religa Engenharia/Serviços/SESMT-DP no menu (incompleto — faltou Estatísticas/Consolidação) |
| `b1e8f2f` | Reverte `a9556dc` por engano — menu volta a ficar quebrado |
| `4c62103` | **Restauração 1**: `App.tsx` e `arvore.ts` reconstruídos a partir de `376b5f3`, com Inserir OC enxertado por cima |
| `607c36c` | **Restauração 2**: `baleryan.go` reconstruído a partir de `363c6d7`, com Administrativo enxertado por cima |

## O que foi restaurado, arquivo por arquivo

### `web/src/App.tsx`
- Voltaram os imports e os `case` de roteamento de `ServicosHub`,
  `Funcionarios`, `Obras`, `Consolidacao` e `Estatisticas`.
- Voltou o chat `ChatRogue` na barra lateral (estado `chatAberto`, botão
  "Rogue Worker").
- Voltou a div `#tp-acoes` no cabeçalho (o portal que o botão "Marcar
  chamado como Serviço" do Hub usa).
- Voltou a lista de telas "escuras" (`ehEscura`) incluindo
  `servicos-hub`, `consolidacao`, `a-pagar`, `faturar` — e ganhou
  `inserir-oc` também, pela mesma razão.
- **Mantido**: o import e o `case` de `InserirOC` (a peça nova e legítima).

### `web/src/menu/arvore.ts`
- Voltou a seção inteira de **Estatísticas** (doze telas, `est-*`).
- Voltou **Consolidação** como tela resolvida (não mais "Balanço" em breve).
- Voltaram os imports/registro de `engenhariaMenu`, `servicosMenu`,
  `sesmtDpMenu`.
- **Mantido**: `administrativoMenu` com Compras → Inserir OC + Equalizar
  Propostas, PCO, Notas fiscais — nada disso foi tocado.
- Estrutura de nesting conferida e preservada: "Dados do Trílogo" como
  irmão de "Contrato São Luiz" dentro de Manutenção (subiu um nível em
  commit anterior), e "Serviços" aninhado dentro de Manutenção (não no
  nível principal — isso foi uma correção de rota minha no meio do processo,
  não parte da regressão original).

### `baleryan/cmd/baleryan/baleryan.go`
- Voltaram a ser montados no `mux`: `servicos.NovoModulo`,
  `estatisticas.Novo`, `consolidacao.Novo`, `funcionarios.Novo`,
  `planejamento.Novo`, `rogueworker.Novo`.
- **Mantido**: `administrativo.Novo(...).Montar(mux)`.
- `Revisao` (que aparece em `GET /saude`) foi de "9" para "10", para
  distinguir do build quebrado.

## Verificação feita

- `npx tsc --noEmit` — limpo (só o erro pré-existente e não relacionado em
  `menu/modulos/administrativo.ts`, já resolvido antes desta sessão).
- `npx vite build` — build de produção completo, sem erro.
- `go build ./...` e `go vet ./...` no `baleryan` — limpos.
- Deploy manual disparado no Render (o auto-deploy por commit não estava
  disparando sozinho para nenhum dos commits desta sessão — mecanismo a
  investigar depois, não bloqueou o conserto).
- Logs do Render confirmados pós-deploy: `rev 10 · escutando na porta
  10000`, sem panic no boot.
- Logs do Supabase (`edge_logs`) usados para provar a causa raiz: as
  chamadas a `/servicos/painel` nunca geravam a consulta correspondente à
  view `servicos_painel` — confirmando que a rota nem chegava a existir no
  servidor antes do conserto.

## Como isso não devia ter acontecido (nota para o processo)

Várias sessões trabalhando em paralelo em módulos diferentes, cada uma
mexendo nos MESMOS três arquivos de "cola" (`App.tsx`, `arvore.ts`,
`baleryan.go`) para religar o próprio módulo — sem checar se a base local
estava atualizada antes de sobrescrever. Duas mitigações possíveis, para
decidir com o dono do sistema:

1. Antes de mexer nesses três arquivos, `git pull`/conferir o commit mais
   recente do `main` — não editar em cima de uma cópia antiga.
2. Reduzir o quanto esses arquivos precisam ser tocados por módulo novo —
   por exemplo, um registro de módulos mais declarativo (lista de módulos
   num único lugar, iterada automaticamente) tornaria uma sobrescrita
   acidental menos destrutiva, porque sobraria menos código pra "esquecer"
   de recolocar.
