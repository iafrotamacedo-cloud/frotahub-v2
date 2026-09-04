# Contrato da API do Trílogo — responsável, cotação e orçamento de Serviço

Levantado em **04/09/2026** por captura de rede na tela real, ticket 135613
(LOJA 28 - ABOLIÇÃO, conta Frota Instalações). O responsável foi trocado e
devolvido ao estado original **quatro vezes seguidas**; uma cotação foi
criada, um orçamento com mão de obra + material + 2 PDFs de teste (R$0,01
cada) foi inserido, conferido pela releitura e excluído — o ticket voltou a
R$0,00, sem responsável.

Este documento é irmão de `api-trilogo.md` (que cobre `CreateTicketCost`, o
ciclo de Materiais do contrato) — aqui é outro ciclo, o de **Serviço**:

```
Responsável = "Serviço X" (gatilho)
        │
        ▼
   Cotação (pedido, sem preço)
        │  CriarCotacao
        ▼
   Orçamento (resposta, com preço + anexo)
        │  CriarOrcamentoServico
        ▼
   "Marcar como vencedor" NA TELA do Trílogo — decisão do responsável
   MSLZ, não automatizada aqui.
```

## Bases

Mesmas de `api-trilogo.md`: `https://web.api.trilogo.app/api/` para dados,
`https://upload.api.trilogo.app/` para arquivos. Autenticação: header
`Authorization: Bearer <token>`.

## 1. Trocar o Responsável Executante

```
PUT https://web.api.trilogo.app/api/Ticket/ChangeServiceCompanyAssignee
Content-Type: application/json

{ "serviceCompanyAssigneeId": 67277, "ticketId": 135613 }

200 -> (corpo vazio)
```

Para remover, manda `null`:

```
{ "serviceCompanyAssigneeId": null, "ticketId": 135613 }
```

A timeline do chamado registra dois tipos de evento diferentes (não o
mesmo "alteração" para os dois sentidos):

| código do evento | rótulo | quando |
|---|---|---|
| 102 | `responsavel` | `ALTERAÇÃO DE RESPONSÁVEL EXECUTANTE` — "Novo responsável: X" |
| 103 | `responsavel` | `REMOÇÃO DE RESPONSÁVEL EXECUTANTE` — "Responsável removido: X" |

(já mapeados em `tipos.go`, `rotuloEvento`)

### Os dois valores especiais

| id | nome dentro do Trílogo | rótulo no FrotaHub | empresa prestadora | status |
|---|---|---|---|---|
| 67277 | "Serviço Instalações" (literal) | Serviço Instalações | FROTA - INSTALAÇÕES (serviceCompanyId 35) | existe, testado |
| 67264 | "Romario Santos" (pessoa) | **Serviço Civil** | FROTA - CIVIL (serviceCompanyId 72) | existe — corrigido em 04/09/2026 |

**Correção (04/09/2026):** o levantamento original concluiu, errado, que
"Serviço Civil" não existia — a busca por esse TEXTO não achava nada porque
não é assim que o registro se chama lá dentro. **Romario Santos**
(encarregado do setor Civil) é quem faz esse papel: é o nome real, dentro
dos 8 responsáveis de `serviceCompanyId=72` (Assis Sousa, Carlos Eduardo,
Frota Civil, contingenciacivil@..., hmartins85@..., Maquiel Gomes, Paulo
Roberto, **Romario Santos**), que o dono confirmou ser o equivalente civil.

**O nome cru nunca deve aparecer na nossa tela.** Mostrar "Romario Santos"
confundiria quem opera, fazendo parecer um técnico específico e não o
gatilho da fila de Serviço — o FrotaHub sempre traduz esse id para "Serviço
Civil" (`trilogo.RotuloResponsavelServico`, `config.Trilogo.ResponsavelServicoCivil = 67264`).

O id do responsável é buscável por:

```
GET https://web.api.trilogo.app/api/ticket/GetTicketServiceCompanyAssignees
    ?term=servi&offset=0&limit=20&serviceCompanyId=35   (ou 72, para Civil)
```

## 2. Buscar a empresa prestadora certa não basta sozinho

Ligado ao achado acima: a lista de responsáveis é **escopada pela empresa
prestadora do ticket** — um ticket cuja `serviceCompany` é FROTA-INSTALAÇÕES
só mostra "Serviço Instalações" nessa busca (testado: buscar "civil" nesse
mesmo ticket devolveu "Nenhum usuário encontrado"). Não existe uma lista
única e global de responsáveis.

## 3. Criar uma cotação

```
POST https://web.api.trilogo.app/api/Quotation/CreateQuotation
Content-Type: application/json

{ "ticketId": 135613,
  "description": "...",
  "comment": "",
  "isAdditionalItemsAllowed": true,
  "isBudgetAttachmentRequired": false,
  "showTicketAttachments": true,
  "products": [],
  "services": [],
  "uploadedFiles": [] }

200 -> { "id": 270242 }
```

**Não existe endpoint de exclusão de cotação.** Conferido na tela: o menu
"..." de um Orçamento tem "Excluir"; a Cotação em volta dele, não. Uma
cotação criada por engano fica lá para sempre, vazia, `status=1` ("aberta").
(Ficaram 4 cotações de teste no ticket 135613 por causa disso — inofensivas,
R$0,00, descritas como "TESTE FROTAHUB - IGNORAR".)

## 4. Criar um orçamento (a resposta, com preço)

Mão de obra e material entram **juntos, no mesmo orçamento** — não é um
orçamento por tipo. É esse o "orçamento único de MDO+material" do desenho de
negócio.

```
POST https://web.api.trilogo.app/api/Budget/CreateBudget
Content-Type: application/json

{ "quotationId": 270225,
  "supplierId": 23927,
  "contact": null,
  "deadlineDate": null,
  "comment": null,
  "payment": { "rateType": 1, "method": 2, "installments": 1,
               "rateValue": null, "ratePercentage": null },
  "shipping": { "deadline": 1, "deadlineUnit": "3", "cost": 0 },
  "products": [ { "isAvailable": true, "unitPrice": 0.01, "amount": 1,
                   "measurementUnit": 1, "description": "2 CFP (MATERIAL DE COLAGEM)" } ],
  "services": [ { "isAvailable": true, "description": " MÃO DE OBRA",
                   "unitPrice": 0.01, "amount": 1, "timeUnit": 1 } ],
  "uploadedFiles": [ { "filename": "...", "permalink": "..." } ] }

200 -> { "id": 304505, "supplierId": 23927, "contact": null }
```

**Devolve o id direto** — diferente de `CreateTicketCost`, que não devolve
nada e obriga reler para descobrir o id (ver `lancamento.go`).

`measurementUnit` (materiais) e `timeUnit` (mão de obra) só foram testados
com o valor-padrão da tela (**1** = Unid. / Minutos). A tela tem outras
opções (Kg, Horas, ...) cujos códigos não foram levantados.

### Tabelas de código

| `supplierId` | conta | status |
|---|---|---|
| 23927 | FROTA - INSTALAÇÕES | levantado (04/09/2026, ticket 135613) |
| ? | FROTA - CIVIL | **não levantado** |

⚠️ **Outro espaço de números.** Não confundir com o `CompanyId` de
`CreateTicketCost` (35 / 72, ver `api-trilogo.md`) — descobertos em telas e
datas diferentes, nada garante que coincidam. `supplierId` mora em
`config.Trilogo.FornecedorInstalacoes` / `FornecedorCivil`.

## 5. Ler as cotações e orçamentos de um ticket

```
GET https://web.api.trilogo.app/api/Quotation/GetQuotationsTicketDetails?ticketId=135613

200 -> { "quotations": [
  { "id": 270242, "description": "...", "creationDate": "2026-09-04T07:44:24",
    "status": 2, "winningBudgetId": null, "comment": "",
    "hasBudget": true, "budgetRequestSent": false,
    "isBudgetAttachmentRequired": false, "isAdditionalItemsAllowed": true,
    "approvedValue": null, "idealPrice": null, "idealPriceIsWinning": false,
    "uId": "...", "requestedSuppliers": [],
    "budgets": [
      { "id": 304505, "creationDate": "2026-09-04T07:48:24",
        "lastChangeDate": null, "deadlineDate": null, "contact": null,
        "shippingCost": 0, "rateType": null, "ratePercentage": null,
        "rateValue": null, "authorId": 0, "approvedValue": null,
        "totalValue": 0.01, "isWinning": false, "viaMarketplace": false,
        "supplier": { "id": 23927, "name": "FROTA - INSTALAÇÕES" },
        "attachments": [], "winningBudgetItems": [] } ] } ] }
```

`status` da cotação: **1** = aberta, sem orçamento nenhum; **2** = em
cotação, com pelo menos um orçamento (muda sozinho ao criar o primeiro
orçamento — bate com o rótulo "ABERTA" → "EM COTAÇÃO" na tela). Outros
códigos (aprovada? rejeitada?) não foram vistos — nenhum orçamento de teste
chegou a ser "marcado como vencedor".

`winningBudgetId` e `approvedValue` na Cotação, e `isWinning` /
`approvedValue` no Orçamento, são presumivelmente como a aprovação aparece
depois de alguém marcar um orçamento como vencedor na tela — **não
confirmado por captura**, porque não testamos essa ação (é decisão do
responsável MSLZ).

## 6. Excluir um orçamento

```
DELETE https://web.api.trilogo.app/api/Budget/DeleteBudget/304505

200 -> (corpo vazio)
```

Só o orçamento — ver item 3 sobre a cotação não ter exclusão.

## 7. Aprovação — levantado em 04/09/2026, pelo lado do CLIENTE, sem nenhum teste ao vivo

**Como foi levantado**: não criamos nenhuma cotação de teste nem marcamos nada
como vencedor — a conta do cliente (Mercadinhos São Luiz, login que aparece
como "F" na tela) já tinha cotações de verdade, aprovadas em datas reais.
Bastou abrir a tela "Cotações" dela (`/external-user-quotations`, um
endereço NOVO, do lado do cliente — diferente de tudo que tínhamos
documentado até aqui, que era só o lado do fornecedor) e capturar a rede.
**Zero orçamento de teste criado ou apagado nesta rodada.**

### A lista — a fonte mais simples

```
GET https://web.api.trilogo.app/api/Quotation/GetQuotationSupplierGrid?limit=20&Status=1,2,3,4

200 -> {
  "totalRecords": 655,
  "list": [
    { "id": 270348, "description": "ORÇAMENTO DE COMPRA DE MATERIAL ",
      "creationDate": "2026-09-04T11:57:32", "ticketId": 135589,
      "companyName": "LOJA 35 - MERCADÃO CAUCAIA",
      "budgetCreationDate": "2026-09-04T11:57:56",
      "budgetDeadlineDate": null,
      "budgetApprovedDate": "2026-09-04T12:38:24",
      "viaMarketplace": false, "status": 3 },
    ...
  ]
}
```

`Status=1,2,3,4` no pedido confirma que existem **quatro** códigos possíveis
— só dois foram observados de verdade:

- **`status: 2`** — "Em análise" na tela. `budgetApprovedDate: null`.
- **`status: 3`** — "Orçamento aprovado" na tela. `budgetApprovedDate`
  preenchido, com a data/hora exata da aprovação.

`status: 1` e `status: 4` **nunca apareceram** em nenhuma das páginas
conferidas (paginamos por uma amostra de ~40 dos 655 registros, todos
`status` 2 ou 3). A leitura mais provável, pela ordem do enum e pelo padrão
já visto no resto do Trílogo (1 = aberta, sem orçamento — bate com o
`status` 1 da Cotação documentado no item acima): `1` = cotação aberta, sem
orçamento ainda; `4` = rejeitado. **Nenhum dos dois está confirmado por
captura** — é a mesma régua de honestidade do resto deste documento.

### O detalhe — a mesma informação, de outro ângulo

```
GET https://web.api.trilogo.app/api/Quotation/GetQuotationSupplierDetail?quotationId=270348

200 -> {
  "id": 270348, "description": "ORÇAMENTO DE COMPRA DE MATERIAL ",
  "creationDate": "2026-09-04T08:57:32",
  "ticket": { "id": 135589, ... },
  "budget": { "id": 304624, "total": 6981.28, "creationDate": "2026-09-04T08:57:56" },
  "statuses": [
    { "status": 1, "creationDate": "2026-09-04T08:57:32", "isCompleted": true },
    { "status": 2, "creationDate": "2026-09-04T08:57:56", "isCompleted": true },
    { "status": 3, "creationDate": "2026-09-04T09:38:24", "isCompleted": true }
  ],
  "supplier": { "id": 23927, "name": "FROTA - INSTALAÇÕES" },
  "viaMarketplace": false
}
```

**Diferente do `Cotacao`/`Orcamento` do item 5** (que veio do lado do
fornecedor, `GetQuotationsTicketDetails`, e tem `winningBudgetId`/`isWinning`
— campos que **continuam nunca vistos preenchidos**): este endpoint devolve
uma **linha do tempo** (`statuses`), um degrau por etapa alcançada, cada um
com a própria data. Aqui os três degraus batem exatamente com os campos da
lista: 1 = cotação criada, 2 = orçamento criado, 3 = aprovado (a data do
degrau 3, `09:38:24`, bate com `budgetApprovedDate` da lista, só que em
UTC — a lista mostra `12:38:24`, 3h à frente, o fuso de Fortaleza).

**Isto muda o desenho da detecção automática** (ver o plano do redesenho do
módulo Serviços): não é preciso presumir `winningBudgetId`/`isWinning` do
lado do fornecedor — dá para perguntar `GetQuotationSupplierDetail` com o
próprio `cotacao_trilogo_id` que já gravamos, e olhar se `statuses` contém
`status: 3`. Falta confirmar uma coisa antes de programar: **se a conta do
FORNECEDOR (Frota Instalações/Civil — a que o robô já usa) também alcança
este endereço**, já que desta vez ele foi visto pelo login do CLIENTE
(Mercadinhos São Luiz). O nome do endereço ("QuotationSupplierDetail") e o
campo `supplier` dentro da resposta sugerem que sim — mas isso só se
confirma logando como fornecedor, o que não foi feito nesta rodada (evitar
trocar de conta na sessão do dono sem avisar).

## O que este levantamento provou

1. **O gatilho de entrada na fila de Serviço é o mesmo, automático ou
   manual**: ambos passam por `ChangeServiceCompanyAssignee`. O botão
   "mandar para fila de serviços" (dentro do FrotaHub) precisa chamar essa
   mesma rota — não existe um endereço separado "botão".
2. **"Serviço Civil" existe, mas com outro nome dentro do Trílogo** — é a
   pessoa Romario Santos (id 67264), não um registro literal "Serviço
   Civil". O FrotaHub traduz; o Trílogo nunca vê a palavra "Serviço" nesse
   caso. Ver item 1.
3. **Mão de obra e material são o MESMO orçamento**, não dois. Confirma o
   desenho de negócio ("orçamento único de MDO+material").
4. **A cotação nunca se apaga.** Criar uma por engano é definitivo.
5. **Aprovação tem endereço próprio, do lado do cliente** —
   `GetQuotationSupplierDetail`/`GetQuotationSupplierGrid`, `status: 3` +
   `budgetApprovedDate`/degrau `statuses` — capturado com dado real, sem
   precisar criar nem apagar nada (item 7).

## O que ainda falta levantar

- O `supplierId` da conta Civil (`config.Trilogo.FornecedorCivil` fica zero
  até lá).
- Os códigos de `measurementUnit`/`timeUnit` além do padrão (1).
- **O que significa `status` 1 e 4** em `GetQuotationSupplierGrid`/
  `GetQuotationSupplierDetail` — `1` é provavelmente "sem orçamento ainda" e
  `4` provavelmente "rejeitado", mas nenhum dos dois apareceu nos ~40
  registros conferidos (item 7). Rejeição especificamente **nunca foi
  observada** — se a hipótese "não distinguível de 'ainda não decidiu'"
  estiver certa, a detecção automática de rejeição pode não dar para
  construir.
- **Se a conta do FORNECEDOR (Frota Instalações/Civil) alcança
  `GetQuotationSupplierDetail`** — só testado pelo login do cliente até
  agora (item 7). Isto decide se a detecção automática de aprovação
  (`servicos/deteccao.go`) consegue rodar sozinha com a sessão que o robô já
  usa, ou se precisa de outra credencial.
- Se existe endpoint de edição de orçamento (a tela tem "Editar" no mesmo
  menu de "Excluir").
