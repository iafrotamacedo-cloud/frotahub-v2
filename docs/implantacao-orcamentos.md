# Implantação do módulo Orçamentos

Fase 3 do FrotaHub 2. Sete passos, na ordem. Cada um tem uma prova — se a prova
não passar, pare ali: o passo seguinte só piora o diagnóstico.

---

## 0. Antes de tudo: as três pendências que já estavam abertas

Elas mordem primeiro, e duas delas escondem o próprio erro.

**a) `CORS_ORIGENS` no Render.** Sem isto o navegador responde "Failed to
fetch" e engole o erro real — e some o `Content-Disposition`, então a extração
baixa com o nome errado. No painel do Render, variável `CORS_ORIGENS` com o
endereço do front (`https://novo.frotamacedo.com.br`), sem barra no fim.

**b) O commit do cursor do robô.** Ainda não subiu.

**c) Rodar `atualizacao` uma vez pelo Actions** e conferir no log a linha
*"marca d'água avançou para ..."* com horários CRESCENTES. É a prova de que o
robô não anda mais para trás. Só depois disso religue o agendamento.

O item (c) importa para este módulo: os custos que o teto de R$ 600 vai somar
vêm de `chamado_custos`, que é o robô quem enche.

---

## 1. O banco

No SQL Editor do Supabase, cole e rode `db/migrations/010_orcamentos.sql`
inteiro. É seguro rodar duas vezes.

**Prova:**

```sql
select count(*) from parametros where vigencia_fim is null;   -- 3
select codigo from rotinas where codigo like 'CONTRATO_ORCAMENTOS%' order by 1;  -- 6 linhas
select * from orcamentos_painel;                              -- 1 linha, tudo zero
```

Se `parametros` vier vazio, o `where c.slug = 'frota-macedo'` não casou — confira
o slug do cliente em `clientes`.

---

## 2. As permissões

Configurações → Categorias → a categoria que vai usar o módulo. Marque as seis
rotinas novas. São separadas de propósito: "ver a planilha" e "lançar custo no
sistema do cliente" são responsabilidades diferentes, e na operação real nem
sempre é a mesma pessoa.

O builder passa sempre, independente da matriz.

---

## 3. As variáveis do motor (Render)

| variável | valor | o que acontece sem ela |
|---|---|---|
| `TRILOGO_EMPRESA_INSTALACOES` | `35` | já é o padrão |
| `TRILOGO_EMPRESA_CIVIL` | **falta levantar** | o lançamento da conta Civil é RECUSADO com a frase explicando |
| `GEMINI_API_KEY` | a chave | a nota entra sem itens, pedindo conferência |
| `GEMINI_MODELO` | `gemini-2.5-flash` | já é o padrão |

**As variáveis `DROPBOX_*` podem sair.** O Dropbox saiu do stack; o motor não as
lê mais, e em produção elas deixaram de ser obrigatórias.

### Como levantar o `TRILOGO_EMPRESA_CIVIL`

Logado na conta **Frota Civil**, abra um ticket que já tenha custo e chame:

```
GET https://web.api.trilogo.app/api/Ticket/GetTicketCosts/?ticketId=<numero>
```

O `company.id` da resposta é o número. Recusar em vez de chutar é proposital:
`CompanyId` errado lança o custo na empresa errada dentro do sistema do cliente.

---

## 4. O motor

Publique o serviço. `Revisao` passa a `8`.

**Prova:** `GET /saude` responde `"revisao": "8"` e o bloco `ligado` traz
`"ia": true` se a chave do Gemini estiver configurada. `dropbox` não aparece
mais.

---

## 5. O front

O `npm run build` já roda no GitHub Actions e publica no HostGator por FTPS.
Nada muda no processo.

**Prova:** o menu Manutenção → Contrato São Luiz mostra **Orçamentos** ao lado
de Dados do Trílogo. Abrindo, aparecem as cinco barras verticais em tema escuro,
com os contadores zerados e a prévia dizendo que não há nada — o que é verdade
num banco recém-migrado.

---

## 6. O leitor de notas (GitHub Actions)

O arquivo `.github/workflows/leitor-notas.yml` está no repositório, mas **o
GitHub não aceita workflow vindo de ferramenta remota** — cole o conteúdo pela
interface do GitHub, como fizemos com o robô do Trílogo.

Os segredos que ele usa já existem, menos `GEMINI_API_KEY`. Adicione em
Settings → Secrets and variables → Actions.

**Prova:** rode uma vez pelo botão (`workflow_dispatch`) com a fila vazia. O log
tem que dizer `=== FIM === 0 lidas · 0 falhas` e encerrar em segundos. Se
reclamar de ferramenta faltando, a mensagem já diz qual e como instalar.

---

## 7. A primeira nota de verdade

1. Orçamentos → **Notas e DAVs** → Escolher arquivos → uma nota → Inserir.
2. Se for **XML**, ela já entra lida, com a etiqueta verde "XML da nota".
   Se for **PDF**, entra como "na fila" e espera o leitor rodar.
3. Confira que o ticket foi achado (a etiqueta de leitura e, na tela de rateio,
   a coluna Tickets).
4. **Lançar orçamentos** → "Gerar orçamentos das notas lidas".
5. Confira o PDF gerado (botão `pdf`) ANTES de lançar.
6. Lançar. A tela mostra o número do custo que o Trílogo devolveu.

**Prova final:** abra o ticket no Trílogo e veja o custo lá, com o PDF anexado.

---

## O que este módulo NÃO faz ainda, de propósito

- **`faturado` e `pago`** existem e são sempre falso. Quem vai escrevê-los é o
  módulo financeiro. A tela diz isso em texto, em vez de mostrar uma coluna de
  traços que parece informação — que é exatamente o que o sistema antigo fazia.
- **Migração dos 509 orçamentos antigos** fica para a próxima fase, como
  combinado. O teto funciona sem ela: a soma vem de `chamado_custos`, que o robô
  já leu.
- **A leitura do Trílogo (`extract-invoice-data`)** existe e não é usada, por
  decisão do dono em 25/08/2026. A leitura é nossa.

---

## Se der errado

| sintoma | causa quase certa |
|---|---|
| "Failed to fetch" em tudo | `CORS_ORIGENS` |
| A extração baixa com nome errado | `CORS_ORIGENS` também (esconde o `Content-Disposition`) |
| "Não sei o id da empresa prestadora da conta civil" | `TRILOGO_EMPRESA_CIVIL` |
| Nota fica em "na fila" para sempre | o workflow do leitor não foi colado, ou falhou |
| Nota entra sem itens | sem `GEMINI_API_KEY` — é o comportamento esperado |
| "O custo FOI criado no Trílogo mas não consegui registrar aqui" | **não lance de novo.** Anote o número do custo e avise: é acerto manual de uma linha |
