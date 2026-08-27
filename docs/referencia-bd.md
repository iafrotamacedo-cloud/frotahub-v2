# Referência do banco — FrotaHub v2

> Levantada **direto do banco** em 27/08/2026, e não da memória.
> Projeto `hltcngamdqabqlocufrv` · PostgreSQL 17.6 · região us-east-1.
>
> ⚠ O projeto `faalgfbugvekbuhhtatt` é o do sistema **antigo, em produção**.
> Nunca escrever nele.

---

## Como ler este arquivo

Ele responde três perguntas, nesta ordem: **o que existe**, **o que o banco
impõe sozinho**, e **o que já rodou**. O detalhe do *porquê* de cada mudança mora
no comentário da migração que a fez — as migrações deste projeto são longas de
propósito, e são o registro de verdade.

---

## 1. As migrações

**34 arquivos, 001 a 034.** Todas aplicadas, com uma ressalva histórica:

| Migração | O que trouxe |
|---|---|
| 001–006 | tipos, perfis, acesso (categorias, rotinas, matriz), histórico |
| 007–008 | o espelho do Trílogo: chamados, eventos, custos, anexos, arquivos |
| 009 | a tela "Dados do Trílogo" |
| **010** | **o módulo Orçamentos** — documentos, itens, tickets, orçamentos, parâmetros |
| 011 | a nota rateada não passa com ticket solto |
| 012 | o modo `alvos` do robô |
| 013–014 | a carga do legado, e a marca do que veio torto |
| 015 | por que não lançou, e quem tem que resolver |
| 016 | "reaberto" estava marcando quem nunca foi reaberto |
| 017 | o que já foi faturado ao cliente, e o que voltou dele |
| 018 | o orçamento que nasceu antes do chamado |
| 019 | a nota repetida ganha nome, e para de virar orçamento |
| 020 | o desconto do fornecedor, e a nota que não cabe no teto |
| 021 | a nota também adota o chamado que chegou depois |
| 022 | o desconto que alguém assina embaixo |
| 023 | faturamento direto |
| 024 | o pedido de faturamento ao fornecedor |
| 025 | o vínculo do orçamento se solta |
| 026 | o orçamento vira documento (emitente, logo) |
| 027 | a entrega dos orçamentos já gerados |
| 028 | o contador conta a fila |
| 029 | onde cada nota mora |
| 030 | a fila de rateio tem regra própria |
| 031 | o valor da nota vira pergunta |
| 032 | `possivel_duplicata` entra na lista de bloqueios |
| **033** | **as views voltam a respeitar quem está perguntando** ⚠ |
| **034** | **a conta fecha, e dá para provar** |

**A ressalva histórica:** a **019 rodou e nunca se registrou** — o arquivo dela não
tem o `insert` do rodapé, então `schema_migrations` pulava do 018 para o 020
enquanto a coluna e os índices dela estavam no banco. A 033 escreve a linha que
faltava, e o teste `TodaMigracaoSeRegistra` impede que se repita (**P-36**).

**As armadilhas do `create or replace view`, as duas medidas na pele:**

1. **Só acrescenta coluna no fim.** Nome, ordem e tipo do que já existe são
   congelados. Pôr coluna nova no meio é recusado com *"cannot change name of
   view column"* — foi como a primeira 031 morreu.
2. **Apaga as opções da view.** Sem repetir `with (security_invoker = true)`, a
   tranca cai em silêncio (**P-35**). Ver a seção 3.

---

## 2. As tabelas

### O núcleo — acesso e titularidade

| Tabela | O que guarda |
|---|---|
| `clientes` | o titular de cada dado (CORE-11) |
| `perfis` | quem cada login é. **Não guarda senha** — quem guarda é a `auth.users` |
| `categorias` | o grupo de um login; **é aqui que mora o nível** |
| `rotinas` · `categoria_permissoes` | o catálogo e a matriz |
| `historico` | o rastro. **Só-inserção, garantido por gatilho** |

### O espelho do Trílogo

| Tabela | Chave de dedup |
|---|---|
| `unidades` | cliente + id do Trílogo · carrega `no_escopo` |
| `chamados` | cliente + **número** (a conta é atributo, não identidade) |
| `chamado_eventos` | chamado + chave (id, ou impressão digital para o evento sem id) |
| `chamado_custos` | chamado + id do custo |
| `chamado_anexos` | chamado + coleção + id do anexo |
| `arquivos` | **o sha256 do conteúdo** |
| `robo_execucoes` | o andamento de cada rodada, e a **marca d'água** |

### Os orçamentos

| Tabela | O que guarda |
|---|---|
| `documentos` | a nota ou DAV lida. 43 colunas — é a tabela mais trabalhada do sistema |
| `documento_itens` | os itens dela, um por linha (P-06) |
| `documento_tickets` | os tickets escritos na nota; `chamado_id` nulo = ticket **solto** |
| `orcamentos` | ticket, parte, valor, status, lançamento, faturamento, legado |
| `orcamento_itens` | preço da nota e preço cobrado, lado a lado |
| `orcamento_documentos` | o vínculo — é o que faz o **rateio** ser caso normal |
| `parametros` | margem, teto, folga, **com vigência** (P-08) |
| `faturamento_ciclos` · `faturas` | o que foi cobrado do cliente e o que voltou |
| `pedidos_faturamento` | a relação de DAVs mandada ao fornecedor |
| `ticket_avisos` | quando cada lista de pendência foi cobrada |
| `emitente` | razão social, CNPJ, endereço e a marca do PDF |
| `jobs` | tarefa demorada é linha, não memória (P-03) |

### Os números de hoje (27/08/2026)

```
documentos            91      orcamentos           703
documento_itens      208      orcamento_itens    2.377
documento_tickets     92      orcamento_documentos  66
chamados           1.643      chamado_eventos   21.046
arquivos           5.320      chamado_anexos     6.459
```

---

## 3. As views — e a tranca que elas precisam carregar

| View | Para quê |
|---|---|
| `documentos_lista` | a lista das notas, com `onde`, `motivo_conferencia`, `conta_fecha` e **`destino`** |
| `orcamentos_lista` | a lista dos orçamentos, com `destino` (quem age) e **`estado`** (onde está) |
| `orcamentos_painel` | os contadores das barras |
| `chamados_lista` · `faturas_lista` · `pedidos_faturamento_lista` | as outras listas |
| **`fechamento_notas`** | toda nota em um destino só |
| **`fechamento_orcamentos`** | todo orçamento em um estado só |
| **`fechamento_ponte`** | nada se perdeu entre a nota e o orçamento |

**⚠ TODA view carrega `with (security_invoker = true)`, e isso não é enfeite.**

Ela manda a consulta rodar com os privilégios de **quem pergunta**. Sem ela, a
view roda como a dona das tabelas — e a dona não é filtrada por política nenhuma.

Medido em 27/08/2026, rodando como `anon` (o papel de quem **não** fez login):

```
set local role anon;
select (select count(*) from documentos)        -- 0    a tabela protege
     , (select count(*) from documentos_lista)  -- 91   a view entregava
     , (select count(*) from orcamentos_lista)  -- 703  a view entregava
```

A chave `publishable` viaja dentro do JavaScript do site. Consertado pela 033.

**Como conferir, e vale rodar de vez em quando:**

```sql
select relname, reloptions from pg_class
 where relkind = 'v' and relnamespace = 'public'::regnamespace order by relname;
-- TODAS têm que aparecer com {security_invoker=true}
```

---

## 4. O que o banco impõe sozinho (P-04)

**Listas fechadas (`check`)** — campo de situação só aceita os valores da lista:

| Coluna | Valores |
|---|---|
| `documentos.status` | `inserido`, `lendo`, `lido`, `falhou`, `usado` |
| `documentos.fila` | `orcamento`, `rateio`, `direto` |
| `documentos.tipo` | `nf`, `dav` |
| `documentos.leitura_camada` | `xml`, `texto`, `ocr`, `ia`, `manual`, `legado` |
| `orcamentos.status` | `gerado`, `aguardando_aprovacao`, `lancado`, `removido` |
| `orcamentos.conta` | `instalacoes`, `civil` |
| `orcamentos.lancamento_bloqueio` | `ticket_status`, `ticket_recusado`, `teto`, `possivel_duplicata`, `sem_empresa`, `trilogo_fora`, `desconhecido` |
| `documentos.desconto_bp` | 0 a 2000 |

> **Atenção ao último:** uma flag nova no Go sem a migração junto **não dá erro
> visível** — o `bloquear` engole o próprio erro de propósito, e a marca
> simplesmente não é gravada. Foi o que aconteceu com `possivel_duplicata` em
> 26/08. O teste `TodaFlagDeBloqueioEhPermitidaNoBanco` lê as duas fontes.

**Índices únicos que impedem duplicata:**

- `orcamento_nota_por_ticket_vivo` — uma nota, um ticket, **um orçamento vivo**
  (parcial em `removido_em is null`, para o apagado soltar a chave)
- `chamados` por cliente + número · `arquivos` pelo sha256
- `documento_por_chave` · `documento_por_numero` — a busca da trava de repetição

**Gatilhos:**

| Gatilho | O que faz |
|---|---|
| `historico_imutavel` (+ `_em_massa`) | **recusa** update, delete e truncate no histórico |
| `adota_notas` · `adota_orcamentos` | chamado que chega adota o que estava órfão |
| `carimba_ticket` · `propaga_ticket` | o ticket do vínculo acompanha o do orçamento |
| `perfis_carimbo` · `unidades_carimbo` | `atualizado_em` |

**O que o banco NÃO impõe, e devia (pendência):**

- **CORE-05** — "nada é apagado de vez" só está garantido no `historico`. Nas
  demais tabelas é disciplina do código, e `pedidos_faturamento` é apagado de
  verdade ao reabrir um pedido.
- A regra do `Encaixar` (a soma dos itens é o valor do orçamento) só existe em Go.

---

## 5. RLS — quem enxerga o quê

**Todas as 27 tabelas têm RLS ligada.** As políticas são **só de leitura**: não
existe nenhuma política de `insert`, `update` ou `delete`, então **o navegador não
escreve em tabela nenhuma**. Toda gravação passa pelo motor, que usa a chave de
serviço.

Quatro tabelas têm RLS ligada e **zero políticas**, de propósito — leitura fechada
para todos, e só o motor alcança: `arquivos`, `historico`, `schema_migrations`,
`emitente`, `pedidos_faturamento`.

A função `posso('CODIGO_DA_ROTINA')` é usada nas políticas do módulo de
orçamentos: esconder o item na barra lateral seria teatro, bastaria pedir a
tabela direto.

---

## 6. As duas contas que têm que fechar

```sql
select ordem, rotulo, quantas from fechamento_notas      order by ordem;
select ordem, rotulo, quantos from fechamento_orcamentos order by ordem;
select * from fechamento_ponte;

-- e o que prova que fecham:
select (select count(*) from documentos)            as notas,
       (select sum(quantas) from fechamento_notas)  as somadas,
       (select count(*) from orcamentos)            as orcamentos,
       (select sum(quantos) from fechamento_orcamentos) as somados;

-- ninguém pode cair no balde do desconhecido:
select * from fechamento_notas      where ordem = 99;
select * from fechamento_orcamentos where ordem = 99;
-- esperado: nenhuma linha
```

**Medido em 27/08/2026:** notas 91 = 91 ✓ · orçamentos 703 = 703 ✓ · ponte com os
três órfãos zerados ✓.

---

## 7. Como rodar as migrações do zero (P-24)

```bash
createdb prova
psql -d prova -c "create schema auth;
  create table auth.users (id uuid primary key default gen_random_uuid(), email text);
  create or replace function auth.uid() returns uuid language sql stable as
    \$\$ select nullif(current_setting('request.jwt.claim.sub', true), '')::uuid \$\$;"
for f in db/migrations/*.sql; do psql -v ON_ERROR_STOP=1 -q -d prova -f "$f" || break; done
```

As 34 rodam limpas nessa ordem, conferido em 27/08/2026 num PostgreSQL 16.
