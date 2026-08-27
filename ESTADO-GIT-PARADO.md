# Fila de entrega — GitHub parado

> **Este arquivo é cumulativo.** Ele é reescrito a cada rodada enquanto o GitHub
> estiver fora. Quando você disser **"voltou o git"**, eu devolvo daqui a lista
> completa de arquivos, os SQL, os YAML e a ordem exata do commit.

---

## O corte

| | |
|---|---|
| **Último push que entrou** | `12e57ba` — *Update leitor-notas.yml* |
| **Quando** | 26/08/2026, **12:03** (Fortaleza) |
| **Incidente do GitHub** | aberto 15:11 UTC = **12:11** — oito minutos depois |

Tudo listado abaixo é posterior a esse commit e **ainda não está no GitHub**.

**Os arquivos já estão no seu disco**, em `C:\Users\FROTAMACEDO\Desktop\FrotaHub-v2`.
Cada entrega foi escrita direto lá. Falta apenas `git add` + `commit` + `push`.

---

## 1. Arquivos

### Rodada 1 — a regra do desconto ligada, e o job órfão

| | arquivo | o que é |
|---|---|---|
| **novo** | `db/migrations/020_desconto_do_fornecedor.sql` | colunas + as duas views |
| mod | `baleryan/interno/modulos/orcamentos/gerar.go` | `planejarNota`: decide a nota inteira antes de gravar; `criarOrcamento` passa a só escrever |
| **novo** | `baleryan/interno/modulos/orcamentos/desconto_test.go` | as três faixas, o rateio por orçamento, o tudo-ou-nada |
| mod | `baleryan/cmd/leitor/main.go` | `recolherOrfaos`: devolve para a fila o trabalho preso há mais de 40 min |
| **novo** | `baleryan/cmd/leitor/orfao_test.go` | prova que o recolhimento é condicional e por tempo |

### Rodada 2 — o carimbo na tela

| | arquivo | o que é |
|---|---|---|
| mod | `web/src/telas/orcamentos/tipos.ts` | campos da 019 e da 020 |
| mod | `web/src/telas/orcamentos/Lancar.tsx` | "ajustada pelo teto · nota R$ X" embaixo do valor |
| mod | `web/src/telas/orcamentos/Arquivos.tsx` | selos `repetida` / `bloqueada` e a frase do motivo na linha |
| mod | `web/src/estilos/orcamentos.css` | cor para a linha de detalhe que precisa ser lida |

### Rodada 3 — a assinatura da NorthCore sai do front

| | arquivo | o que é |
|---|---|---|
| mod | `web/src/componentes/Marca.tsx` | a propriedade `assinatura` sai junto com o texto |
| mod | `web/src/App.tsx` | `<Marca />` no rodapé do menu |
| mod | `web/src/telas/Login.tsx` | `<Marca />` na tela de entrada |
| mod | `web/src/estilos/base.css` | as 5 regras órfãs da assinatura |

Não havia imagem: era texto. Conferido que não sobra menção no front, no
`index.html`, na pasta `public` nem no lado Go (PDF, planilha, e-mail).
O `docs/diario-de-bordo.md` continua citando o nome — é registro histórico.

### Rodada 4 — o tratamento das duas filas

| | arquivo | o que é |
|---|---|---|
| **novo** | `baleryan/interno/modulos/orcamentos/tratamento.go` | conferir, trocar de fila, trocar e apagar ticket |
| **novo** | `baleryan/interno/modulos/orcamentos/tratamento_test.go` | a ordem do trocar, o filtro do apagar, a avaliação que não para |
| **novo** | `web/src/telas/orcamentos/VisorDaNota.tsx` | a nota na tela com a barra de ação |
| **novo** | `web/src/telas/orcamentos/Candidatos.tsx` | extraído de `Correcoes.tsx` — quase morreu na mudança |
| mod | `baleryan/interno/modulos/orcamentos/gerar.go` | `avaliarPartes`, a conta que a tela e a geração compartilham |
| mod | `baleryan/interno/modulos/orcamentos/rotas.go` | os quatro endereços novos |
| mod | `baleryan/interno/banco/cliente.go` | `Apagar`, que recusa filtro vazio |
| mod | `web/src/telas/orcamentos/Correcoes.tsx` | as duas telas reescritas, com atualizar e reprocessar |
| mod | `web/src/telas/orcamentos/tipos.ts` | `Conferencia` e `ParteConferida` |
| mod | `web/src/estilos/orcamentos.css` | o visor, a barra e as linhas verde/vermelha |

**Sem migração.** Usa colunas que já existem.

### Rodada 5 — a nota adota o chamado que chegou depois

| | arquivo | o que é |
|---|---|---|
| **novo** | `db/migrations/021_nota_adota_chamado.sql` | gatilho + backfill: sem isto o "atualizar" nunca ficaria verde |

### Rodada 6 — as notas que passam do teto

| | arquivo | o que é |
|---|---|---|
| **novo** | `db/migrations/022_desconto_autorizado.sql` | colunas do desconto e da aprovação + `documentos_lista` |
| **novo** | `baleryan/interno/regras/desconto_autorizado_test.go` | o limite de 20%, o ticket cheio, as bordas |
| **novo** | `web/src/telas/orcamentos/ConfirmarComSenha.tsx` | os dois popups do mesmo tamanho |
| mod | `baleryan/interno/regras/orcamento.go` | `CalcularDesconto`, `ComDescontoAutorizado`, `ComAprovacaoDoCliente` |
| mod | `baleryan/interno/modulos/orcamentos/gerar.go` | os dois caminhos na avaliação, e o status parado restaurado |
| mod | `baleryan/interno/modulos/orcamentos/tratamento.go` | desconto, aprovação, lista das extrapoladas |
| mod | `baleryan/interno/modulos/orcamentos/tratamento_test.go` | as duas saídas provadas separadas |
| mod | `baleryan/interno/modulos/orcamentos/correcoes.go` | a lista entra no mesmo payload |
| mod | `baleryan/interno/modulos/orcamentos/rotas.go` | três endereços novos |
| mod | `web/src/telas/orcamentos/Correcoes.tsx` | a aba "Passam do teto" |
| mod | `web/src/telas/orcamentos/tipos.ts` | `Desconto` e os campos da 022 |
| mod | `web/src/estilos/orcamentos.css` | a caixa de confirmação |


### Rodada 7 — faturamento direto

| | arquivo | o que é |
|---|---|---|
| **novo** | `db/migrations/023_faturamento_direto.sql` | a fila `direto`, a coluna, os índices e `orcamentos_painel` |
| **novo** | `baleryan/interno/modulos/orcamentos/direto.go` | lista com filtros e o "mandar para lançar" |
| **novo** | `baleryan/interno/modulos/orcamentos/direto_test.go` | as duas travas de dinheiro |
| **novo** | `web/src/telas/orcamentos/Direto.tsx` | a tela |
| mod | `baleryan/interno/armazem/r2.go` | `Baixar`, para trazer a nota original de volta |
| mod | `baleryan/interno/modulos/orcamentos/lancar.go` | `arquivoParaLancar`: orçamento ou nota original |
| mod | `baleryan/interno/modulos/orcamentos/faturamento.go` | as duas consultas excluem as diretas |
| mod | `baleryan/interno/modulos/orcamentos/rotas.go` | dois endereços |
| mod | `web/src/telas/orcamentos/Orcamentos.tsx` | a barra nova no painel |
| mod | `web/src/telas/orcamentos/Arquivos.tsx` | `Insercao` exportada, para a tela nova usar |
| mod | `web/src/telas/orcamentos/tipos.ts` | `notas_direto` |
| mod | `web/src/estilos/orcamentos.css` | filtros e aviso da fila |

### Rodada 8 — "Faturar ao cliente" muda de menu

| | arquivo | o que é |
|---|---|---|
| mod | `web/src/menu/arvore.ts` | Financeiro › A receber deixa de ser "em breve" e ganha a rotina |
| mod | `web/src/App.tsx` | despacha a tela `faturar` |
| mod | `web/src/telas/orcamentos/Orcamentos.tsx` | a barra e o ícone saem do painel |

**Sem migração.** A rotina `CONTRATO_ORCAMENTOS_FATURAR` já existe desde a 017,
e as rotinas são planas — mudar de lugar no menu não muda quem pode o quê.

### Rodada 9 — a ficha do chamado voltou a rolar

| | arquivo | o que é |
|---|---|---|
| mod | `web/src/estilos/trilogo.css` | `.fi-mesa` rola, `.fi-folha` não encolhe, `.fi-esquerda` rola por dentro |

**Sem migração.** Só CSS. Medido em Chromium sem tela: antes o pé da folha era
inalcançável (`rolavel=false`), depois rola 529px e o rodapé aparece.

### Rodada 10 — A pagar › pedido de faturamento

| | arquivo | o que é |
|---|---|---|
| **novo** | `db/migrations/024_pedido_de_faturamento.sql` | tabela de pedidos, `documentos.pedido_id`, a view e a rotina |
| **novo** | `baleryan/interno/modulos/orcamentos/pedido.go` | fila, relação em Excel/PDF, fechar, reabrir, enviado |
| **novo** | `baleryan/interno/modulos/orcamentos/pedido_test.go` | o `pedido_id is null` e a DAV sem data |
| **novo** | `web/src/telas/financeiro/APagar.tsx` | a tela |
| mod | `baleryan/interno/modulos/orcamentos/rotas.go` | seis endereços |
| mod | `web/src/menu/arvore.ts` | A pagar deixa de ser "em breve" |
| mod | `web/src/App.tsx` | despacha a tela |
| mod | `web/src/estilos/orcamentos.css` | a barra do pedido |

**Total real: 40 arquivos — 19 novos, 21 modificados.**

> O número veio de `git diff --name-status` contra o corte, não de somar as
> tabelas: vários arquivos foram tocados em mais de uma rodada.

---

## 2. SQL

### 2.1 Migrações de esquema — nesta ordem

⚠️ **Na ordem: 020 → 021 → 022 → 023 → 024. Todas ANTES do push.**

#### `020_desconto_do_fornecedor.sql`

**Status: NÃO aplicada.** (Conferido no banco: as quatro colunas não existem.)

⚠️ **Tem que rodar ANTES do push.** O `criarOrcamento` novo grava
`valor_nota_cheio` e `ajustado_pelo_teto`. Código novo com banco velho faz
**toda geração de orçamento falhar**.

O arquivo faz três coisas, nesta ordem:

1. `orcamentos` ganha `valor_nota_cheio` e `ajustado_pelo_teto`;
   `documentos` ganha `bloqueio_motivo`
2. recria **`orcamentos_lista`** — 33 colunas verbatim + as 2 novas
3. recria **`documentos_lista`** — 25 colunas verbatim + `bloqueio_motivo`,
   e `pronto_para_gerar` passa a exigir que ela seja nula

Se alguma view der erro: **para e me mostra a mensagem.** Recriar view fora de
ordem derruba a tela de lançamento inteira.

#### `021_nota_adota_chamado.sql`

**Status: NÃO aplicada.**

Gatilho para o chamado que chega DEPOIS do ticket ser escrito na nota, mais o
backfill do que já estava esperando. Sem ela, o botão "atualizar" da tela de
tratamento relê o Trílogo, o chamado entra, e a linha continua vermelha —
porque a conferência lê o `chamado_id` guardado, não o número do ticket.

Conferido no banco de hoje: **não muda nenhuma linha agora**. O único ticket
solto (130709) é um chamado que o Trílogo realmente não trouxe. Ela entra
inerte e passa a impedir o caso a partir da próxima leitura.

#### `022_desconto_autorizado.sql`

**Status: NÃO aplicada.** Depois da 021.

Colunas do desconto autorizado (com `check desconto_bp <= 2000` no próprio
banco) e do pedido de aprovação, mais a `documentos_lista` recriada — 26
colunas verbatim, quatro novas no fim, e `pronto_para_gerar` passando a excluir
a nota que pediu aprovação.

#### `023_faturamento_direto.sql`

**Status: NÃO aplicada.** Por último.

A fila `direto` no CHECK de `documentos.fila`, a coluna `faturamento_direto` em
`orcamentos`, os índices parciais e a `orcamentos_painel` recriada — 18 colunas
verbatim, três expressões alteradas (`a_faturar`, `valor_a_faturar` e
`sem_associacao`) e um contador novo no fim.

#### `024_pedido_de_faturamento.sql`

**Status: NÃO aplicada.** Por último, depois da 023.

Cria `pedidos_faturamento`, a coluna `documentos.pedido_id` (o critério da fila),
a view `pedidos_faturamento_lista` e a rotina `CONTRATO_FINANCEIRO_PAGAR`, que
herda de quem já podia faturar ao cliente.

### 2.2 SQL de operação — recuperar a fila de leitura

**Status: NÃO rodado.** Não é migração: é conserto de dados, e só faz sentido
depois de o Actions voltar e o modelo novo ser aceito.

```sql
-- (a) as notas marcadas "falhou" não têm defeito: quem acabou foi a cota do
--     Gemini e o modelo aposentado. Devolve todas para "lido".
update documentos
   set status = 'lido', leitura_erro = null
 where oculto_em is null and status = 'falhou';

-- (b) refila o que ainda não tem o campo de observação reconhecido — são os
--     DAVs que perderam o ticket por causa do rótulo "Dados Complementares".
insert into jobs (cliente_id, tipo, alvo_id)
select d.cliente_id, 'ler_documento', d.id
  from documentos d
 where d.oculto_em is null
   and d.status in ('lido', 'inserido')
   and coalesce(d.leitura_bruta->>'observacao_do_campo', 'false') <> 'true'
   and not exists (select 1 from jobs j
                    where j.alvo_id = d.id and j.status = 'na_fila');
```

⚠️ **Antes de reler, um cuidado:** os itens são gravados com upsert por
`(documento, ordem)`. Se a leitura nova achar MENOS itens que a anterior, as
sobras da antiga continuam lá, órfãs. Para zerar antes de reler:

```sql
delete from documento_itens i
 using documentos d
 where i.documento_id = d.id
   and d.oculto_em is null
   and coalesce(d.leitura_bruta->>'observacao_do_campo', 'false') <> 'true';
```

---

## 3. YAML

**Nada pendente.** O `.github/workflows/leitor-notas.yml` que está no GitHub já é
a versão completa: passo do `poppler-utils`, sem Tesseract, sem OpenCV,
`GEMINI_MODELO: gemini-3.5-flash-lite` e `GEMINI_INTERVALO: ""`.
Entrou no commit `12e57ba`, antes do incidente.

---

## 4. Estado atual

### Repositório
- GitHub em `12e57ba`; seu disco tem 9 arquivos adiante disso, sem commit.
- Compila, `go vet` limpo, **suíte inteira verde**, `tsc` sem erro,
  `vite build` passando.

### Banco (`hltcngamdqabqlocufrv`)
- **019 aplicada** ✔  ·  **020 pendente** ✘
- documentos: **8 lidos**, **7 falhou** (todos recuperáveis pelo SQL 2.2a)
- jobs de leitura: 26 concluídos, **11 falhou**, nenhum na fila
- orçamentos: 72 gerados, 564 lançados

### Leitura de notas
- Modelo em uso: `gemini-3.5-flash-lite` — **ainda não foi testado**, a corrida
  ficou presa na fila do incidente.
- Modelos que morreram hoje: `gemini-2.5-flash` e `gemini-2.5-flash-lite`
  (fechados para contas novas); `gemini-3.6-flash` ficou sem cota (limite 20/dia).
- 6 DAVs continuam sem ticket por causa do rótulo — o conserto já está no
  GitHub, falta reler.

### Front
- O `dist` no HostGator **não** tem as telas novas. O carimbo só aparece depois
  de `npm run build` no `web` e do envio para `novo.frotamacedo.com.br`.
- O valor sai certo do banco mesmo sem o front novo.

### Página de manutenção
- No ar em `frotamacedo.com.br`, contagem apontando para **14h** de hoje.
- Se o Actions demorar, trocar o horário antes de a contagem zerar.

---

## 5. Fora desta fila (não depende do GitHub)

- **BackLog Conferência** — as 8 perguntas em aberto (Q2–Q9) seguem sem resposta.
- **Migração do legado** — 60 removidos (R$ 46.244,39), 84 notas pendentes,
  3 casos-problema (130559, 125691, 130320).
- **Notas já usadas** não foram carregadas no banco novo — pré-requisito para
  I3 e I4 do backlog.

---

*Última atualização: 26/08/2026, 14:55 (Fortaleza) — rodada 10.*
